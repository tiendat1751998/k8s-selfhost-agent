<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import { useInfraHosts } from '../../composables/useInfraHosts'
import { api } from '../../api/client'
import { dockerApi } from '../../api/docker'
import BaseIcon from '../ui/BaseIcon.vue'

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
const nodesExpanded = ref(true)

function getServiceIcon(name: string): string {
  const lower = name.toLowerCase()
  if (lower.includes('traefik') || lower.includes('ingress') || lower.includes('gateway') || lower.includes('nginx')) return 'radio'
  if (lower.includes('postg') || lower.includes('sql') || lower.includes('mysql') || lower.includes('redis') || lower.includes('db')) return 'database'
  if (lower.includes('nats') || lower.includes('kafka') || lower.includes('queue') || lower.includes('mq')) return 'zap'
  if (lower.includes('agent')) return 'cloud'
  if (lower.includes('docker') || lower.includes('containerd')) return 'box'
  if (lower.includes('auth') || lower.includes('vault') || lower.includes('security')) return 'lock'
  if (lower.includes('monitor') || lower.includes('prom') || lower.includes('grafana')) return 'activity'
  return 'sliders'
}

async function fetchDynamicServices() {
  const foundNames = new Set<string>()
  const items: TargetServiceItem[] = []

  // 1. Try GET /logs/services (centralized logging or agent services)
  try {
    const res = await api.get<{ services?: string[] } | string[]>('/logs/services')
    const rawList = Array.isArray(res) ? res : (res?.services || [])
    for (const item of rawList) {
      const name = typeof item === 'string' ? item : (item as { name?: string })?.name || String(item)
      if (name && !foundNames.has(name.toLowerCase())) {
        foundNames.add(name.toLowerCase())
        items.push({
          id: name,
          name,
          icon: getServiceIcon(name),
          type: 'App Container',
        })
      }
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
      for (const s of dockerServices.value) {
        const name = s.name || s.id
        if (name && !foundNames.has(name.toLowerCase())) {
          foundNames.add(name.toLowerCase())
          items.push({
            id: name,
            name,
            icon: getServiceIcon(name),
            type: 'Swarm Service',
          })
        }
      }
    }

    if (dockerContainers.status === 'fulfilled' && Array.isArray(dockerContainers.value)) {
      for (const c of dockerContainers.value) {
        const name = (c.name || c.id).replace(/^\//, '')
        if (name && !foundNames.has(name.toLowerCase())) {
          foundNames.add(name.toLowerCase())
          items.push({
            id: name,
            name,
            icon: getServiceIcon(name),
            type: c.state || 'Container',
          })
        }
      }
    }
  } catch {
    // Graceful fallback
  }

  // 3. Fallback cluster resources if cluster specified
  if (props.cluster) {
    try {
      const res = await api.get<any>(`/k8s/${encodeURIComponent(props.cluster)}/resources`, { kind: 'Service' })
      const k8sItems = Array.isArray(res) ? res : (res?.data || res?.items || [])
      if (Array.isArray(k8sItems)) {
        for (const s of k8sItems) {
          const name = s.metadata?.name || s.name
          if (name && !foundNames.has(name.toLowerCase())) {
            foundNames.add(name.toLowerCase())
            items.push({
              id: name,
              name,
              icon: getServiceIcon(name),
              type: s.spec?.type || 'Service',
            })
          }
        }
      }
    } catch {
      // Graceful fallback
    }
  }

  if (items.length > 0) {
    apiServices.value = items
  }
}

// Section 1: Running Apps (Log App)
// Lists actual running application containers: postgres_db, tiki_redis, my-nginx, nats, traefik, etc.
const dynamicServices = computed<TargetServiceItem[]>(() => {
  const map = new Map<string, TargetServiceItem>()

  // Production running apps defaults
  const defaultApps: TargetServiceItem[] = [
    { id: 'postgres_db', name: 'postgres_db', icon: 'database', type: 'Database' },
    { id: 'tiki_redis', name: 'tiki_redis', icon: 'database', type: 'Cache Store' },
    { id: 'my-nginx', name: 'my-nginx', icon: 'radio', type: 'Web Server' },
    { id: 'nats', name: 'nats', icon: 'zap', type: 'Message Broker' },
    { id: 'traefik', name: 'traefik', icon: 'radio', type: 'Edge Proxy' },
  ]
  for (const app of defaultApps) {
    map.set(app.id.toLowerCase(), app)
  }

  // API discovered services
  for (const s of apiServices.value) {
    map.set(s.id.toLowerCase(), s)
  }

  // Dynamically collect unique running apps/containers from incoming logs
  for (const log of props.logs) {
    const svc = log.service || log.container || log.attributes?.app || log.attributes?.service
    if (svc && !map.has(svc.toLowerCase())) {
      map.set(svc.toLowerCase(), {
        id: svc,
        name: svc,
        icon: getServiceIcon(svc),
        type: 'App Container',
      })
    }
  }

  return Array.from(map.values())
})

// Section 2: Node Systems (Log Node / SystemD)
// Lists host nodes: k8smater133, 10.10.10.60, 10.10.10.80.
// When a node is selected, views host OS & systemd journal logs.
const dynamicHostNodes = computed<TargetNodeItem[]>(() => {
  const map = new Map<string, TargetNodeItem>()

  if (hosts.value && hosts.value.length > 0) {
    for (const h of hosts.value) {
      const name = h.name || h.id || h.endpoint
      const isMaster = name.toLowerCase().includes('mater') || name.toLowerCase().includes('master')
      const role = isMaster ? 'Control Plane · SystemD' : 'Worker Node · SystemD'
      map.set(name.toLowerCase(), {
        id: name,
        name,
        icon: 'server',
        role,
        systemd: true,
      })
    }
  }

  // Host node defaults specified in requirements: k8smater133, 10.10.10.60, 10.10.10.80
  const defaultNodes: TargetNodeItem[] = [
    { id: 'k8smater133', name: 'k8smater133', icon: 'server', role: 'Control Plane · SystemD', systemd: true },
    { id: '10.10.10.60', name: '10.10.10.60', icon: 'server', role: 'Worker Node · SystemD', systemd: true },
    { id: '10.10.10.80', name: '10.10.10.80', icon: 'server', role: 'Worker Node · SystemD', systemd: true },
  ]
  for (const d of defaultNodes) {
    if (!map.has(d.id.toLowerCase())) {
      map.set(d.id.toLowerCase(), d)
    }
  }

  // Dynamically collect unique nodes present in incoming log entries
  for (const log of props.logs) {
    const node = log.node || log.attributes?.node
    if (node && !map.has(node.toLowerCase())) {
      map.set(node.toLowerCase(), {
        id: node,
        name: node,
        icon: 'server',
        role: 'Host Node · SystemD',
        systemd: true,
      })
    }
  }

  return Array.from(map.values())
})

// Filter search applied across targets
const filteredServices = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  if (!q) return dynamicServices.value
  return dynamicServices.value.filter(s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q))
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

// Default selection when page opens: first available running app or first available node.
function autoSelectDefaultTarget() {
  if (props.modelValue && props.modelValue.type !== 'all' && props.modelValue.id && props.modelValue.id !== 'all') {
    return
  }
  if (dynamicServices.value.length > 0) {
    const first = dynamicServices.value[0]
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
      <!-- Section 1: Running Apps (Log App) -->
      <div class="tree-section">
        <div class="section-toggle" @click="appsExpanded = !appsExpanded">
          <span class="section-caret" :class="{ 'caret-down': appsExpanded }">&#9656;</span>
          <BaseIcon name="box" size="xs" />
          <span class="section-label">Running Apps (Log App)</span>
          <span class="section-count font-mono">{{ filteredServices.length }}</span>
        </div>
        <div v-show="appsExpanded" class="section-items">
          <button
            v-for="svc in filteredServices"
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

      <!-- Section 2: Node Systems (Log Node / SystemD) -->
      <div class="tree-section">
        <div class="section-toggle" @click="nodesExpanded = !nodesExpanded">
          <span class="section-caret" :class="{ 'caret-down': nodesExpanded }">&#9656;</span>
          <BaseIcon name="server" size="xs" />
          <span class="section-label">Node Systems (Log Node / SystemD)</span>
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
