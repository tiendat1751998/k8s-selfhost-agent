<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import { useInfraHosts } from '../../composables/useInfraHosts'
import { api } from '../../api/client'
import { fleetApi } from '../../api/fleet'

export interface LogTarget {
  type: 'all' | 'node' | 'service'
  id: string
  name: string
  icon?: string
}

export interface TargetNodeItem {
  id: string
  name: string
  icon: string
  role: string
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
const emit = defineEmits<{ (e: 'update:modelValue', target: LogTarget): void; (e: 'select', target: LogTarget): void }>()

const { hosts } = useInfraHosts()
const apiServices = ref<TargetServiceItem[]>([])
const targetSearch = ref('')
const nodesExpanded = ref(true)
const servicesExpanded = ref(true)

function getServiceIcon(name: string): string {
  const lower = name.toLowerCase()
  if (lower.includes('traefik') || lower.includes('ingress') || lower.includes('gateway')) return '🚦'
  if (lower.includes('postg') || lower.includes('sql') || lower.includes('mysql') || lower.includes('redis') || lower.includes('db')) return '🐘'
  if (lower.includes('nats') || lower.includes('kafka') || lower.includes('queue') || lower.includes('mq')) return '⚡'
  if (lower.includes('agent')) return '🛰️'
  if (lower.includes('docker') || lower.includes('containerd')) return '🐳'
  if (lower.includes('auth') || lower.includes('vault') || lower.includes('security')) return '🔒'
  if (lower.includes('monitor') || lower.includes('prom') || lower.includes('grafana')) return '📊'
  return '⚙️'
}

async function fetchClusterServices() {
  try {
    // Verify active cluster exists before querying or query fleet
    let targetCluster = props.cluster
    if (!targetCluster) {
      try {
        const clusters = await fleetApi.list()
        if (clusters && clusters.length > 0) {
          targetCluster = clusters[0].name || clusters[0].id || ''
        }
      } catch {
        // Fleet list failed or offline; continue without active cluster
      }
    }

    if (!targetCluster) {
      // No verified active cluster; retain default services and logs discovery without firing 404
      return
    }

    const res = await api.get<any>(`/k8s/${encodeURIComponent(targetCluster)}/resources`, { kind: 'Service' })
    const items = Array.isArray(res) ? res : (res?.data || res?.items || [])
    if (Array.isArray(items) && items.length > 0) {
      apiServices.value = items.map((s: any) => {
        const name = s.metadata?.name || s.name || String(s)
        return {
          id: name,
          name,
          icon: getServiceIcon(name),
          type: s.spec?.type || 'Service',
        }
      })
    }
  } catch {
    // Gracefully catch HTTP 404 and endpoint errors without unhandled console errors
    // Baseline defaults and dynamic log stream discovery provide complete coverage
  }
}

watch(
  () => props.cluster,
  () => {
    fetchClusterServices()
  }
)

onMounted(() => {
  fetchClusterServices()
})

// Dynamic host nodes: loaded from useInfraHosts() API + dynamic log discovery + fallback defaults
const dynamicHostNodes = computed<TargetNodeItem[]>(() => {
  const map = new Map<string, TargetNodeItem>()

  if (hosts.value && hosts.value.length > 0) {
    for (const h of hosts.value) {
      const name = h.name || h.id
      const isMaster = name.toLowerCase().includes('master') || (h.host_role?.toLowerCase().includes('control') ?? false)
      const icon = isMaster ? '👑' : (h.host_type === 'database' ? '🗄️' : '🖥️')
      const role = h.host_role || (isMaster ? 'Control Plane' : 'Worker Node')
      map.set(name.toLowerCase(), { id: name, name, icon, role })
    }
  } else {
    // Real node defaults (fixing typo 'k8smater' -> 'master')
    const defaults: TargetNodeItem[] = [
      { id: 'master', name: 'master', icon: '👑', role: 'Control Plane' },
      { id: 'worker1', name: 'worker1', icon: '🖥️', role: 'Worker Node' },
      { id: 'worker2', name: 'worker2', icon: '🖥️', role: 'Worker Node' },
      { id: 'worker3', name: 'worker3', icon: '🖥️', role: 'Worker Node' },
      { id: 'masterdb', name: 'masterdb', icon: '🗄️', role: 'Primary DB' },
      { id: 'workerdb1', name: 'workerdb1', icon: '🗄️', role: 'Replica DB' },
    ]
    for (const d of defaults) {
      map.set(d.id.toLowerCase(), d)
    }
  }

  // Also collect any unique nodes present in incoming log stream
  for (const log of props.logs) {
    if (log.node && !map.has(log.node.toLowerCase())) {
      const isMaster = log.node.toLowerCase().includes('master')
      map.set(log.node.toLowerCase(), {
        id: log.node,
        name: log.node,
        icon: isMaster ? '👑' : '🖥️',
        role: isMaster ? 'Control Plane' : 'Host Node',
      })
    }
  }

  return Array.from(map.values())
})

// Dynamic services: baseline defaults + API /k8s/default/resources?kind=Service + dynamically collected from props.logs
const dynamicServices = computed<TargetServiceItem[]>(() => {
  const map = new Map<string, TargetServiceItem>()

  const defaultServices: TargetServiceItem[] = [
    { id: 'traefik', name: 'traefik', icon: '🚦', type: 'Ingress Proxy' },
    { id: 'postgres', name: 'postgres', icon: '🐘', type: 'Stateful DB' },
    { id: 'nats', name: 'nats', icon: '⚡', type: 'Message Broker' },
    { id: 'k8s-agent', name: 'k8s-agent', icon: '🛰️', type: 'Cluster Agent' },
    { id: 'standalone', name: 'standalone', icon: '⚙️', type: 'Core Daemon' },
    { id: 'docker', name: 'docker', icon: '🐳', type: 'Container Engine' },
  ]
  for (const s of defaultServices) {
    map.set(s.id.toLowerCase(), s)
  }

  for (const s of apiServices.value) {
    map.set(s.id.toLowerCase(), s)
  }

  // Dynamically collect unique services/workloads from incoming log entries
  for (const log of props.logs) {
    const svc = log.service || log.container
    if (svc && !map.has(svc.toLowerCase())) {
      map.set(svc.toLowerCase(), {
        id: svc,
        name: svc,
        icon: getServiceIcon(svc),
        type: 'Workload',
      })
    }
  }

  return Array.from(map.values())
})

// Quick filter search applied across targets
const filteredHostNodes = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  if (!q) return dynamicHostNodes.value
  return dynamicHostNodes.value.filter(n => n.name.toLowerCase().includes(q) || n.role.toLowerCase().includes(q))
})

const filteredServices = computed(() => {
  const q = targetSearch.value.trim().toLowerCase()
  if (!q) return dynamicServices.value
  return dynamicServices.value.filter(s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q))
})

function selectTarget(target: LogTarget) {
  emit('update:modelValue', target)
  emit('select', target)
}

function getNodeCount(nodeId: string): number {
  const q = nodeId.toLowerCase()
  return props.logs.filter(l => (l.node && l.node.toLowerCase().includes(q)) || (l.pod && l.pod.toLowerCase().includes(q)) || (l.namespace && l.namespace.toLowerCase().includes(q))).length
}

function getServiceCount(serviceId: string): number {
  const q = serviceId.toLowerCase()
  return props.logs.filter(l => (l.service && l.service.toLowerCase().includes(q)) || (l.container && l.container.toLowerCase().includes(q)) || (l.pod && l.pod.toLowerCase().includes(q)) || (l.namespace && l.namespace.toLowerCase().includes(q))).length
}
</script>

<template>
  <aside class="log-target-tree glass-panel" aria-label="Log Stream Target Hierarchy">
    <div class="tree-header">
      <div class="tree-title"><span class="tree-icon">🌲</span><span>Log Targets</span></div>
      <span class="tree-badge font-mono">{{ logs.length }} logs</span>
    </div>

    <!-- Quick Filter Search Input -->
    <div class="tree-search-bar">
      <span class="tree-search-ico" aria-hidden="true">🔍</span>
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
      >✕</button>
    </div>

    <div class="tree-content">
      <!-- All Cluster Logs -->
      <button
        type="button"
        role="treeitem"
        :aria-selected="modelValue.type === 'all'"
        class="tree-item tree-root-item"
        :class="{ active: modelValue.type === 'all' }"
        @click="selectTarget({ type: 'all', id: 'all', name: 'All Cluster Logs', icon: '🌐' })"
      >
        <span class="tree-accent-bar"></span>
        <span class="item-icon">🌐</span>
        <div class="item-meta">
          <span class="item-name">All Cluster Logs</span>
          <span class="item-sub font-mono">Unified aggregator</span>
        </div>
        <span class="item-count font-mono" :class="{ 'has-logs': logs.length > 0 }">{{ logs.length }}</span>
      </button>

      <!-- Host Nodes Section -->
      <div class="tree-section">
        <div class="section-toggle" @click="nodesExpanded = !nodesExpanded">
          <span class="section-caret" :class="{ 'caret-down': nodesExpanded }">▸</span>
          <span class="section-label">Host Nodes</span>
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
            @click="selectTarget({ type: 'node', id: node.id, name: node.name, icon: node.icon })"
          >
            <span class="tree-accent-bar"></span>
            <span class="item-icon">{{ node.icon }}</span>
            <div class="item-meta">
              <span class="item-name font-mono">{{ node.name }}</span>
              <span class="item-sub font-mono">{{ node.role }}</span>
            </div>
            <span class="item-count font-mono" :class="{ 'has-logs': getNodeCount(node.id) > 0 }">{{ getNodeCount(node.id) }}</span>
          </button>
        </div>
      </div>

      <!-- Services & Containers Section -->
      <div class="tree-section">
        <div class="section-toggle" @click="servicesExpanded = !servicesExpanded">
          <span class="section-caret" :class="{ 'caret-down': servicesExpanded }">▸</span>
          <span class="section-label">Services & Containers</span>
          <span class="section-count font-mono">{{ filteredServices.length }}</span>
        </div>
        <div v-show="servicesExpanded" class="section-items">
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
            <span class="item-icon">{{ svc.icon }}</span>
            <div class="item-meta">
              <span class="item-name font-mono">{{ svc.name }}</span>
              <span class="item-sub font-mono">{{ svc.type }}</span>
            </div>
            <span class="item-count font-mono" :class="{ 'has-logs': getServiceCount(svc.id) > 0 }">{{ getServiceCount(svc.id) }}</span>
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>
