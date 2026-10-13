<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import { useInfraHosts } from '../../composables/useInfraHosts'
import { api } from '../../api/client'
import { dockerApi } from '../../api/docker'
import { k8sApi } from '../../api/k8s'
import BaseIcon from '../ui/BaseIcon.vue'
import LogTargetItem from './LogTargetItem.vue'
import { isNoiseLog, classifyLogTarget } from '../../utils/logTargetCategorizer'

export interface LogTarget {
  type: 'service' | 'node' | 'all'
  id: string
  name: string
  icon?: string
  namespace?: string
}

export interface TargetNodeItem {
  id: string
  name: string
  icon: string
  role: string
  systemd?: boolean
}

export interface TargetServiceItem {
  id: string
  name: string
  icon: string
  type: string
  category: 'app' | 'systemd'
}

export interface TargetWorkloadItem {
  id: string
  name: string
  icon: string
  kind: string
  namespace?: string
}

const props = withDefaults(
  defineProps<{
    modelValue: LogTarget
    logs: LogEntry[]
    cluster?: string
  }>(),
  { cluster: '' }
)
const emit = defineEmits<{
  (e: 'update:modelValue', target: LogTarget): void
  (e: 'select', target: LogTarget): void
}>()

const { hosts } = useInfraHosts()
const apiServices = ref<TargetServiceItem[]>([])
const apiWorkloads = ref<TargetWorkloadItem[]>([])
const targetSearch = ref('')
const workloadsExpanded = ref(true)
const appsExpanded = ref(true)
const systemdExpanded = ref(true)
const nodesExpanded = ref(true)

async function fetchDynamicServices() {
  const foundNames = new Set<string>()
  const items: TargetServiceItem[] = []
  const workloads: TargetWorkloadItem[] = []

  function addService(name?: string | null, defaultType?: string) {
    if (!name || isNoiseLog(name) || foundNames.has(name.toLowerCase())) return
    foundNames.add(name.toLowerCase())
    const c = classifyLogTarget(name, defaultType)
    items.push({ id: name, name, icon: c.icon, type: c.type, category: c.category })
  }

  function addWorkload(name?: string | null, kind = 'Deployment', namespace?: string) {
    if (!name || isNoiseLog(name)) return
    const key = `wl:${name.toLowerCase()}`
    if (foundNames.has(key)) return
    foundNames.add(key)
    workloads.push({ id: name, name, icon: 'layers', kind, namespace })
  }

  // 1. Try GET /logs/targets or /logs/services
  try {
    const res = await api.get<{ targets?: string[]; services?: string[] } | string[]>('/logs/targets')
    const rawList = Array.isArray(res) ? res : (res?.targets || res?.services || [])
    for (const item of rawList) {
      addService(typeof item === 'string' ? item : (item as { name?: string })?.name || String(item))
    }
  } catch {
    try {
      const res = await api.get<{ services?: string[] } | string[]>('/logs/services')
      const rawList = Array.isArray(res) ? res : (res?.services || [])
      for (const item of rawList) {
        addService(typeof item === 'string' ? item : (item as { name?: string })?.name || String(item))
      }
    } catch {}
  }

  // 2. Try GET /docker/services and /docker/containers
  try {
    const [dockerServices, dockerContainers] = await Promise.allSettled([
      dockerApi.listServices(),
      dockerApi.listContainers(),
    ])
    if (dockerServices.status === 'fulfilled' && Array.isArray(dockerServices.value)) {
      for (const s of dockerServices.value) addService(s.name || s.id, 'Swarm Service')
    }
    if (dockerContainers.status === 'fulfilled' && Array.isArray(dockerContainers.value)) {
      for (const c of dockerContainers.value) addService((c.name || c.id).replace(/^\//, ''), c.state || 'Container')
    }
  } catch {}

  // 3. Try GET /deployments for active k8s workloads
  try {
    const deployRes = await api.get<{ data?: any[] } | any[]>('/deployments')
    const deploys = Array.isArray(deployRes) ? deployRes : (deployRes?.data || [])
    for (const d of deploys) {
      if (d && d.name) addWorkload(d.name, d.kind || 'Deployment', d.namespace)
    }
  } catch {}

  // 4. Fallback cluster resources if cluster specified
  if (props.cluster) {
    try {
      const [svcRes, podRes, deployRes] = await Promise.allSettled([
        k8sApi.listResources(props.cluster, 'services'),
        k8sApi.listResources(props.cluster, 'pods'),
        k8sApi.listResources(props.cluster, 'deployments'),
      ])
      if (deployRes.status === 'fulfilled' && Array.isArray(deployRes.value)) {
        for (const d of deployRes.value) addWorkload(d.metadata?.name || (d as { name?: string }).name, 'Deployment', d.metadata?.namespace)
      }
      if (svcRes.status === 'fulfilled' && Array.isArray(svcRes.value)) {
        for (const s of svcRes.value) addService(s.metadata?.name || (s as { name?: string }).name, 'Service')
      }
      if (podRes.status === 'fulfilled' && Array.isArray(podRes.value)) {
        for (const p of podRes.value) addService(p.metadata?.name || (p as { name?: string }).name, 'Pod')
      }
    } catch {}
  }

  if (items.length > 0) apiServices.value = items
  if (workloads.length > 0) apiWorkloads.value = workloads
}

const dynamicWorkloads = computed<TargetWorkloadItem[]>(() => {
  const map = new Map<string, TargetWorkloadItem>()
  for (const w of apiWorkloads.value) map.set(w.id.toLowerCase(), w)
  return Array.from(map.values())
})

const dynamicServices = computed<TargetServiceItem[]>(() => {
  const map = new Map<string, TargetServiceItem>()
  for (const s of apiServices.value) {
    if (!isNoiseLog(s.name)) map.set(s.id.toLowerCase(), s)
  }
  for (const log of props.logs) {
    const svc = log.service || log.container || log.attributes?.app || log.attributes?.service
    if (svc && !map.has(svc.toLowerCase()) && !isNoiseLog(svc)) {
      const classified = classifyLogTarget(svc)
      map.set(svc.toLowerCase(), {
        id: svc,
        name: svc,
        icon: classified.icon,
        type: classified.type,
        category: classified.category,
      })
    }
  }
  return Array.from(map.values())
})

const dynamicAppServices = computed(() => dynamicServices.value.filter(s => s.category === 'app'))
const dynamicSystemdServices = computed(() => dynamicServices.value.filter(s => s.category === 'systemd'))

const dynamicHostNodes = computed<TargetNodeItem[]>(() => {
  const map = new Map<string, TargetNodeItem>()
  if (hosts.value && hosts.value.length > 0) {
    for (const h of hosts.value) {
      const name = h.name || h.id || h.endpoint
      const isMaster = name.toLowerCase().includes('mater') || name.toLowerCase().includes('master')
      map.set(name.toLowerCase(), { id: name, name, icon: 'server', role: isMaster ? 'Control Plane · SystemD' : 'Worker Node · SystemD', systemd: true })
    }
  }
  for (const log of props.logs) {
    const node = log.node || log.attributes?.node
    if (node && !map.has(node.toLowerCase())) {
      map.set(node.toLowerCase(), { id: node, name: node, icon: 'server', role: 'Host Node · SystemD', systemd: true })
    }
  }
  return Array.from(map.values())
})

const filteredK8sWorkloads = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  return q ? dynamicWorkloads.value.filter(w => w.name.toLowerCase().includes(q) || w.kind.toLowerCase().includes(q)) : dynamicWorkloads.value
})

const filteredAppServices = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  return q ? dynamicAppServices.value.filter(s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q)) : dynamicAppServices.value
})

const filteredSystemdServices = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  return q ? dynamicSystemdServices.value.filter(s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q)) : dynamicSystemdServices.value
})

const filteredHostNodes = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  return q ? dynamicHostNodes.value.filter(n => n.name.toLowerCase().includes(q) || n.role.toLowerCase().includes(q)) : dynamicHostNodes.value
})

function selectTarget(target: LogTarget) {
  emit('update:modelValue', target)
  emit('select', target)
}

const targetLogCounts = computed(() => {
  const serviceCounts = new Map<string, number>()
  const nodeCounts = new Map<string, number>()
  for (const l of props.logs) {
    const sTargets = [l.service, l.container, l.attributes?.app, l.attributes?.service, l.pod, l.namespace].filter(Boolean) as string[]
    for (const key of new Set(sTargets.map(s => s.toLowerCase()))) {
      serviceCounts.set(key, (serviceCounts.get(key) || 0) + 1)
    }
    const nTargets = [l.node, l.attributes?.node, l.pod, l.namespace].filter(Boolean) as string[]
    for (const key of new Set(nTargets.map(n => n.toLowerCase()))) {
      nodeCounts.set(key, (nodeCounts.get(key) || 0) + 1)
    }
  }
  return { serviceCounts, nodeCounts }
})

function getNodeCount(nodeId: string): number {
  return targetLogCounts.value.nodeCounts.get(nodeId.toLowerCase()) || 0
}

function getServiceCount(serviceId: string): number {
  return targetLogCounts.value.serviceCounts.get(serviceId.toLowerCase()) || 0
}

function autoSelectDefaultTarget() {
  if (props.modelValue && props.modelValue.type !== 'all' && props.modelValue.id && props.modelValue.id !== 'all') {
    return
  }
  if (dynamicWorkloads.value.length > 0) {
    const first = dynamicWorkloads.value[0]
    selectTarget({ type: 'service', id: first.id, name: first.name, icon: first.icon, namespace: first.namespace })
  } else if (dynamicAppServices.value.length > 0) {
    const first = dynamicAppServices.value[0]
    selectTarget({ type: 'service', id: first.id, name: first.name, icon: first.icon })
  } else if (dynamicSystemdServices.value.length > 0) {
    const first = dynamicSystemdServices.value[0]
    selectTarget({ type: 'service', id: first.id, name: first.name, icon: first.icon })
  } else if (dynamicHostNodes.value.length > 0) {
    const first = dynamicHostNodes.value[0]
    selectTarget({ type: 'node', id: first.id, name: first.name, icon: 'server' })
  }
}

watch(() => props.cluster, () => { fetchDynamicServices() })
watch([dynamicWorkloads, dynamicServices, dynamicHostNodes], () => { autoSelectDefaultTarget() }, { immediate: true })

onMounted(async () => {
  await fetchDynamicServices()
  autoSelectDefaultTarget()
})
</script>

<template>
  <aside class="log-target-tree glass-panel" aria-label="Log Stream Target Hierarchy">
    <div class="tree-header">
      <div class="tree-title">
        <span class="tree-icon"><BaseIcon name="layers" size="sm" /></span>
        <span>Log Targets</span>
      </div>
      <span class="tree-badge font-mono">{{ logs.length }} logs</span>
    </div>

    <!-- Quick Filter Search Input -->
    <div class="tree-search-bar">
      <span class="tree-search-ico" aria-hidden="true"><BaseIcon name="search" size="sm" /></span>
      <input
        v-model="targetSearch"
        type="text"
        placeholder="Filter targets..."
        class="tree-search-input font-mono"
        aria-label="Filter targets"
      />
      <button
        v-if="targetSearch"
        type="button"
        class="tree-search-clear"
        aria-label="Clear target filter"
        @click="targetSearch = ''"
      ><BaseIcon name="x" size="xs" /></button>
    </div>

    <div class="tree-content">
      <div
        v-if="filteredK8sWorkloads.length === 0 && filteredAppServices.length === 0 && filteredSystemdServices.length === 0 && filteredHostNodes.length === 0"
        class="tree-empty-state font-mono"
        style="padding: 24px 16px; text-align: center; color: var(--text-muted); font-size: 12px; display: flex; flex-direction: column; align-items: center; gap: 8px;"
      >
        <BaseIcon name="layers" size="md" />
        <span>No log targets discovered</span>
      </div>

      <!-- Section 0: Kubernetes Workloads -->
      <div v-if="filteredK8sWorkloads.length > 0" class="tree-section">
        <div class="section-toggle" @click="workloadsExpanded = !workloadsExpanded">
          <span class="section-caret" :class="{ 'caret-down': workloadsExpanded }">&#9656;</span>
          <BaseIcon name="layers" size="xs" />
          <span class="section-label">Kubernetes Workloads</span>
          <span class="section-count font-mono">{{ filteredK8sWorkloads.length }}</span>
        </div>
        <div v-show="workloadsExpanded" class="section-items">
          <LogTargetItem
            v-for="wl in filteredK8sWorkloads"
            :key="wl.id"
            :id="wl.id"
            :name="wl.name"
            :sub="wl.kind + (wl.namespace ? ' · ' + wl.namespace : '')"
            :icon="wl.icon"
            :count="getServiceCount(wl.id)"
            :active="modelValue.type === 'service' && modelValue.id === wl.id"
            @click="selectTarget({ type: 'service', id: wl.id, name: wl.name, icon: wl.icon, namespace: wl.namespace })"
          />
        </div>
      </div>

      <!-- Section 1: Running Apps (Containers) -->
      <div class="tree-section">
        <div class="section-toggle" @click="appsExpanded = !appsExpanded">
          <span class="section-caret" :class="{ 'caret-down': appsExpanded }">&#9656;</span>
          <BaseIcon name="box" size="xs" />
          <span class="section-label">Running Apps (Containers)</span>
          <span class="section-count font-mono">{{ filteredAppServices.length }}</span>
        </div>
        <div v-show="appsExpanded" class="section-items">
          <LogTargetItem
            v-for="svc in filteredAppServices"
            :key="svc.id"
            :id="svc.id"
            :name="svc.name"
            :sub="svc.type"
            :icon="svc.icon || 'box'"
            :count="getServiceCount(svc.id)"
            :active="modelValue.type === 'service' && modelValue.id === svc.id"
            @click="selectTarget({ type: 'service', id: svc.id, name: svc.name, icon: svc.icon })"
          />
        </div>
      </div>

      <!-- Section 2: System Services (SystemD) -->
      <div class="tree-section">
        <div class="section-toggle" @click="systemdExpanded = !systemdExpanded">
          <span class="section-caret" :class="{ 'caret-down': systemdExpanded }">&#9656;</span>
          <BaseIcon name="cpu" size="xs" />
          <span class="section-label">System Services (SystemD)</span>
          <span class="section-count font-mono">{{ filteredSystemdServices.length }}</span>
        </div>
        <div v-show="systemdExpanded" class="section-items">
          <LogTargetItem
            v-for="svc in filteredSystemdServices"
            :key="svc.id"
            :id="svc.id"
            :name="svc.name"
            :sub="svc.type"
            :icon="svc.icon || 'cpu'"
            :count="getServiceCount(svc.id)"
            :active="modelValue.type === 'service' && modelValue.id === svc.id"
            @click="selectTarget({ type: 'service', id: svc.id, name: svc.name, icon: svc.icon })"
          />
        </div>
      </div>

      <!-- Section 3: Node Systems (Log Node) -->
      <div class="tree-section">
        <div class="section-toggle" @click="nodesExpanded = !nodesExpanded">
          <span class="section-caret" :class="{ 'caret-down': nodesExpanded }">&#9656;</span>
          <BaseIcon name="server" size="xs" />
          <span class="section-label">Node Systems (Log Node)</span>
          <span class="section-count font-mono">{{ filteredHostNodes.length }}</span>
        </div>
        <div v-show="nodesExpanded" class="section-items">
          <LogTargetItem
            v-for="node in filteredHostNodes"
            :key="node.id"
            :id="node.id"
            :name="node.name"
            :sub="node.role"
            :icon="node.icon || 'server'"
            :count="getNodeCount(node.id)"
            :active="modelValue.type === 'node' && modelValue.id === node.id"
            @click="selectTarget({ type: 'node', id: node.id, name: node.name, icon: 'server' })"
          />
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>
