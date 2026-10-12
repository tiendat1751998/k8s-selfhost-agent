<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import { useInfraHosts } from '../../composables/useInfraHosts'
import { api } from '../../api/client'
import { dockerApi } from '../../api/docker'
import { k8sApi } from '../../api/k8s'
import BaseIcon from '../ui/BaseIcon.vue'
import {
  isNoiseLog,
  classifyLogTarget,
} from '../../utils/logTargetCategorizer'

export interface LogTarget {
  type: 'service' | 'node' | 'all'
  id: string
  name: string
  icon?: string
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

const props = withDefaults(
  defineProps<{
    modelValue: LogTarget
    logs: LogEntry[]
    cluster?: string
  }>(),
  {
    cluster: '',
  }
)
const emit = defineEmits<{
  (e: 'update:modelValue', target: LogTarget): void
  (e: 'select', target: LogTarget): void
}>()

const { hosts } = useInfraHosts()
const apiServices = ref<TargetServiceItem[]>([])
const targetSearch = ref('')
const appsExpanded = ref(true)
const systemdExpanded = ref(true)
const nodesExpanded = ref(true)

// Dynamic target discovery only — zero static fallback mocks
const DEFAULT_APPS: readonly TargetServiceItem[] = []
const DEFAULT_SYSTEMD: readonly TargetServiceItem[] = []
const DEFAULT_NODES: readonly TargetNodeItem[] = []

async function fetchDynamicServices() {
  const foundNames = new Set<string>()
  const items: TargetServiceItem[] = []

  function addService(name?: string | null, defaultType?: string) {
    if (!name || isNoiseLog(name) || foundNames.has(name.toLowerCase())) return
    foundNames.add(name.toLowerCase())
    const c = classifyLogTarget(name, defaultType)
    items.push({ id: name, name, icon: c.icon, type: c.type, category: c.category })
  }

  // 1. Try GET /logs/services (centralized logging or agent services)
  try {
    const res = await api.get<{ services?: string[] } | string[]>('/logs/services')
    const rawList = Array.isArray(res) ? res : (res?.services || [])
    for (const item of rawList) {
      const name = typeof item === 'string' ? item : (item as { name?: string })?.name || String(item)
      addService(name)
    }
  } catch {
    // Graceful fallback to docker services
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
  } catch {
    // Graceful fallback
  }

  // 3. Fallback cluster resources if cluster specified
  if (props.cluster) {
    try {
      const [svcRes, podRes] = await Promise.allSettled([
        k8sApi.listResources(props.cluster, 'services'),
        k8sApi.listResources(props.cluster, 'pods'),
      ])
      if (svcRes.status === 'fulfilled' && Array.isArray(svcRes.value)) {
        for (const s of svcRes.value) addService(s.metadata?.name || (s as { name?: string }).name, 'Service')
      }
      if (podRes.status === 'fulfilled' && Array.isArray(podRes.value)) {
        for (const p of podRes.value) addService(p.metadata?.name || (p as { name?: string }).name, 'Pod')
      }
    } catch {
      // Graceful fallback
    }
  }

  if (items.length > 0) {
    apiServices.value = items
  }
}

// All discovered dynamic services (containers + systemd units)
const dynamicServices = computed<TargetServiceItem[]>(() => {
  const map = new Map<string, TargetServiceItem>()

  // Production running apps defaults
  for (const app of DEFAULT_APPS) map.set(app.id.toLowerCase(), app)

  // System services defaults (systemd)
  for (const sys of DEFAULT_SYSTEMD) {
    if (!map.has(sys.id.toLowerCase())) map.set(sys.id.toLowerCase(), sys)
  }

  // API discovered services (filter out noise)
  for (const s of apiServices.value) {
    if (isNoiseLog(s.name)) continue
    map.set(s.id.toLowerCase(), s)
  }

  // Dynamically collect unique running apps/containers & systemd units from incoming logs
  for (const log of props.logs) {
    const svc = log.service || log.container || log.attributes?.app || log.attributes?.service
    if (svc && !map.has(svc.toLowerCase())) {
      if (isNoiseLog(svc)) continue
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

// Section 1: Running Apps (Container Workloads)
const dynamicAppServices = computed<TargetServiceItem[]>(() => {
  return dynamicServices.value.filter(s => s.category === 'app')
})

// Section 2: System Services (SystemD Daemons)
const dynamicSystemdServices = computed<TargetServiceItem[]>(() => {
  return dynamicServices.value.filter(s => s.category === 'systemd')
})

// Section 3: Node Systems (Log Node / SystemD)
const dynamicHostNodes = computed<TargetNodeItem[]>(() => {
  const map = new Map<string, TargetNodeItem>()

  if (hosts.value && hosts.value.length > 0) {
    for (const h of hosts.value) {
      const name = h.name || h.id || h.endpoint
      const isMaster = name.toLowerCase().includes('mater') || name.toLowerCase().includes('master')
      const role = isMaster ? 'Control Plane · SystemD' : 'Worker Node · SystemD'
      map.set(name.toLowerCase(), { id: name, name, icon: 'server', role, systemd: true })
    }
  }

  // Host node defaults
  for (const d of DEFAULT_NODES) {
    if (!map.has(d.id.toLowerCase())) map.set(d.id.toLowerCase(), d)
  }

  // Dynamically collect unique nodes present in incoming log entries
  for (const log of props.logs) {
    const node = log.node || log.attributes?.node
    if (node && !map.has(node.toLowerCase())) {
      map.set(node.toLowerCase(), { id: node, name: node, icon: 'server', role: 'Host Node · SystemD', systemd: true })
    }
  }

  return Array.from(map.values())
})

// Filter search applied across targets
const filteredAppServices = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  if (!q) return dynamicAppServices.value
  return dynamicAppServices.value.filter(s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q))
})

const filteredSystemdServices = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  if (!q) return dynamicSystemdServices.value
  return dynamicSystemdServices.value.filter(s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q))
})

const filteredHostNodes = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  if (!q) return dynamicHostNodes.value
  return dynamicHostNodes.value.filter(n => n.name.toLowerCase().includes(q) || n.role.toLowerCase().includes(q))
})

function selectTarget(target: LogTarget) {
  emit('update:modelValue', target)
  emit('select', target)
}

function getNodeCount(nodeId: string): number {
  const q = nodeId.toLowerCase()
  return props.logs.filter(l =>
    (l.node && l.node.toLowerCase().includes(q)) ||
    (l.attributes?.node && l.attributes.node.toLowerCase().includes(q)) ||
    (l.pod && l.pod.toLowerCase().includes(q)) ||
    (l.namespace && l.namespace.toLowerCase().includes(q))
  ).length
}

function getServiceCount(serviceId: string): number {
  const q = serviceId.toLowerCase()
  return props.logs.filter(l =>
    (l.service && l.service.toLowerCase().includes(q)) ||
    (l.container && l.container.toLowerCase().includes(q)) ||
    (l.attributes?.app && l.attributes.app.toLowerCase().includes(q)) ||
    (l.attributes?.service && l.attributes.service.toLowerCase().includes(q)) ||
    (l.pod && l.pod.toLowerCase().includes(q)) ||
    (l.namespace && l.namespace.toLowerCase().includes(q))
  ).length
}

// Default selection when page opens: first available container app, systemd service, or node.
function autoSelectDefaultTarget() {
  if (props.modelValue && props.modelValue.type !== 'all' && props.modelValue.id && props.modelValue.id !== 'all') {
    return
  }
  if (dynamicAppServices.value.length > 0) {
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

watch(
  () => props.cluster,
  () => {
    fetchDynamicServices()
  }
)

watch(
  [dynamicServices, dynamicHostNodes],
  () => {
    autoSelectDefaultTarget()
  },
  { immediate: true }
)

onMounted(async () => {
  await fetchDynamicServices()
  autoSelectDefaultTarget()
})
</script>

<template>
  <aside class="log-target-tree glass-panel" aria-label="Log Stream Target Hierarchy">
    <div class="tree-header">
      <div class="tree-title">
        <span class="tree-icon">
          <BaseIcon name="layers" size="sm" />
        </span>
        <span>Log Targets</span>
      </div>
      <span class="tree-badge font-mono">{{ logs.length }} logs</span>
    </div>

    <!-- Quick Filter Search Input -->
    <div class="tree-search-bar">
      <span class="tree-search-ico" aria-hidden="true">
        <BaseIcon name="search" size="sm" />
      </span>
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
        v-if="filteredAppServices.length === 0 && filteredSystemdServices.length === 0 && filteredHostNodes.length === 0"
        class="tree-empty-state font-mono"
        style="padding: 24px 16px; text-align: center; color: var(--text-muted); font-size: 12px; display: flex; flex-direction: column; align-items: center; gap: 8px;"
      >
        <BaseIcon name="layers" size="md" />
        <span>No log targets discovered</span>
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
          <button
            v-for="svc in filteredAppServices"
            :key="svc.id"
            type="button"
            role="treeitem"
            :aria-selected="modelValue.type === 'service' && modelValue.id === svc.id"
            class="tree-item"
            :class="{ active: modelValue.type === 'service' && modelValue.id === svc.id }"
            @click="selectTarget({ type: 'service', id: svc.id, name: svc.name, icon: svc.icon })"
          >
            <span class="tree-accent-bar"></span>
            <span class="item-icon">
              <BaseIcon :name="svc.icon || 'box'" size="sm" />
            </span>
            <div class="item-meta">
              <span class="item-name font-mono">{{ svc.name }}</span>
              <span class="item-sub font-mono">{{ svc.type }}</span>
            </div>
            <span class="item-count font-mono" :class="{ 'has-logs': getServiceCount(svc.id) > 0 }">{{ getServiceCount(svc.id) }}</span>
          </button>
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
          <button
            v-for="svc in filteredSystemdServices"
            :key="svc.id"
            type="button"
            role="treeitem"
            :aria-selected="modelValue.type === 'service' && modelValue.id === svc.id"
            class="tree-item"
            :class="{ active: modelValue.type === 'service' && modelValue.id === svc.id }"
            @click="selectTarget({ type: 'service', id: svc.id, name: svc.name, icon: svc.icon })"
          >
            <span class="tree-accent-bar"></span>
            <span class="item-icon">
              <BaseIcon :name="svc.icon || 'cpu'" size="sm" />
            </span>
            <div class="item-meta">
              <span class="item-name font-mono">{{ svc.name }}</span>
              <span class="item-sub font-mono">{{ svc.type }}</span>
            </div>
            <span class="item-count font-mono" :class="{ 'has-logs': getServiceCount(svc.id) > 0 }">{{ getServiceCount(svc.id) }}</span>
          </button>
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
          <button
            v-for="node in filteredHostNodes"
            :key="node.id"
            type="button"
            role="treeitem"
            :aria-selected="modelValue.type === 'node' && modelValue.id === node.id"
            class="tree-item"
            :class="{ active: modelValue.type === 'node' && modelValue.id === node.id }"
            @click="selectTarget({ type: 'node', id: node.id, name: node.name, icon: 'server' })"
          >
            <span class="tree-accent-bar"></span>
            <span class="item-icon">
              <BaseIcon name="server" size="sm" />
            </span>
            <div class="item-meta">
              <span class="item-name font-mono">{{ node.name }}</span>
              <span class="item-sub font-mono">{{ node.role }}</span>
            </div>
            <span class="item-count font-mono" :class="{ 'has-logs': getNodeCount(node.id) > 0 }">{{ getNodeCount(node.id) }}</span>
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>
