<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  overviewApi,
  tpsApi,
  type SystemOverview,
  type NodeMetrics,
  type NetworkInterface,
  type ContainerMetrics,
  type MetricAlert,
  type ProcessMetric,
  type TpsSnapshot,
  type TpsServiceMetrics,
} from '../api/overview'
import { useWebSocket } from '../composables/useWebSocket'
import CircularGauge from '../components/ui/CircularGauge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import { formatContainerName, formatImageName } from '../utils/dockerFormat'

const router = useRouter()

// ==========================================
// 1. STATE MANAGEMENT
// ==========================================
const overview = ref<SystemOverview | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const lastUpdated = ref<Date>(new Date())
const isLiveWs = ref(false)

// TPS (Transactions Per Second) Metrics State
const tpsData = ref<TpsSnapshot | null>(null)
const tpsLoading = ref(true)
const tpsError = ref<string | null>(null)
const tpsLastUpdated = ref<Date | null>(null)
let tpsAbortController: AbortController | null = null

// History buffer for sparklines & 5-minute trends (up to 30 data points)
interface TrendPoint {
  time: string
  cpu: number
  mem: number
  disk: number
  reqs: number
}
const trendHistory = ref<TrendPoint[]>([])

// Node diagnostics inspector drawer
const selectedNodeId = ref<string | null>(null)
const showNodeDrawer = ref(false)
const processSearch = ref('')
const processSortBy = ref<'cpu' | 'mem' | 'disk' | 'name' | 'pid'>('cpu')

// Container filtering & selection
const containerSearch = ref('')
const selectedNodeFilter = ref<string>('all')
const containerSortBy = ref<'cpu' | 'memory' | 'name'>('cpu')
const selectedContainer = ref<ContainerMetrics | null>(null)
const showContainerDrawer = ref(false)

// Topology filtering & selection
type TopologyFilter = 'all' | 'online' | 'offline'
const selectedTopologyFilter = ref<TopologyFilter>('all')

// Dismissed alert IDs (session-level)
const dismissedAlerts = ref<Set<string>>(new Set())

// Polling fallback timer
let fallbackInterval: ReturnType<typeof setInterval> | null = null
let abortController: AbortController | null = null

// Live request activity simulation based on reqs/s
interface ActivityLog {
  id: string
  time: string
  method: string
  path: string
  status: number
  duration: number
}
const recentActivity = ref<ActivityLog[]>([])

// ==========================================
// 2. DATA FETCHING & WEBSOCKET
// ==========================================
async function fetchOverview() {
  if (abortController) {
    abortController.abort()
  }
  abortController = new AbortController()

  try {
    const data = await overviewApi.getOverview()
    if (data) {
      updateOverviewState(data)
      error.value = null
    }
  } catch (err: unknown) {
    if (err instanceof Error && err.name === 'AbortError') return
    if (!overview.value) {
      error.value = err instanceof Error ? err.message : 'Failed to retrieve cluster telemetry'
    }
  } finally {
    loading.value = false
  }
}

async function fetchTpsMetrics() {
  if (tpsAbortController) {
    tpsAbortController.abort()
  }
  tpsAbortController = new AbortController()

  try {
    const data = await tpsApi.getSnapshot()
    if (data) {
      tpsData.value = data
      tpsError.value = null
      tpsLastUpdated.value = new Date()
    }
  } catch (err: unknown) {
    if (err instanceof Error && err.name === 'AbortError') return
    tpsError.value = err instanceof Error ? err.message : 'Failed to retrieve throughput metrics'
  } finally {
    tpsLoading.value = false
  }
}


// ==========================================
// 1b. NODE ORDERING & PERSISTENCE
// ==========================================
const NODE_ORDER_STORAGE_KEY = 'k8s_overview_node_order'

function loadSavedNodeOrder(): string[] {
  try {
    const raw = localStorage.getItem(NODE_ORDER_STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) {
        return parsed.filter((item): item is string => typeof item === 'string' && item.length > 0)
      }
    }
  } catch (err) {
    console.warn('Failed to parse saved node order from localStorage:', err)
  }
  return []
}

function saveSavedNodeOrder(order: string[]) {
  try {
    localStorage.setItem(NODE_ORDER_STORAGE_KEY, JSON.stringify(order))
  } catch (err) {
    console.warn('Failed to save node order to localStorage:', err)
  }
}

const nodeOrderKeys = ref<string[]>(loadSavedNodeOrder())
const hasCustomNodeOrder = computed(() => {
  return nodeOrderKeys.value.length > 0 && localStorage.getItem(NODE_ORDER_STORAGE_KEY) !== null
})

// Drag and drop state
const draggedNodeId = ref<string | null>(null)
const dragOverNodeId = ref<string | null>(null)
let hasDragged = false

function onDragStart(e: DragEvent, node: NodeMetrics) {
  hasDragged = true
  draggedNodeId.value = node.node_id
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', node.node_id)
  }
}

function onDragOver(e: DragEvent, node: NodeMetrics) {
  if (!draggedNodeId.value || draggedNodeId.value === node.node_id) return
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'move'
  }
  dragOverNodeId.value = node.node_id
}

function onDragEnter(node: NodeMetrics) {
  if (!draggedNodeId.value || draggedNodeId.value === node.node_id) return
  dragOverNodeId.value = node.node_id
}

function onDragLeave(e: DragEvent, node: NodeMetrics) {
  const currentTarget = e.currentTarget as HTMLElement | null
  const relatedTarget = e.relatedTarget as Node | null
  if (currentTarget && relatedTarget && !currentTarget.contains(relatedTarget)) {
    if (dragOverNodeId.value === node.node_id) {
      dragOverNodeId.value = null
    }
  }
}

function onDrop(targetNode: NodeMetrics) {
  if (draggedNodeId.value && draggedNodeId.value !== targetNode.node_id) {
    reorderNodes(draggedNodeId.value, targetNode.node_id)
  }
  draggedNodeId.value = null
  dragOverNodeId.value = null
}

function onDragEnd() {
  draggedNodeId.value = null
  dragOverNodeId.value = null
  setTimeout(() => {
    hasDragged = false
  }, 50)
}

function reorderNodes(draggedId: string, targetId: string) {
  if (!draggedId || !targetId || draggedId === targetId) return

  // Build full order list including all current nodes in case any was missing
  const currentNodes = overview.value?.nodes || []
  const order = [...nodeOrderKeys.value]
  for (const n of currentNodes) {
    const k = n.node_id || n.node_name
    if (k && !order.includes(k)) {
      order.push(k)
    }
  }

  const fromIndex = order.indexOf(draggedId)
  const toIndex = order.indexOf(targetId)

  if (fromIndex !== -1 && toIndex !== -1) {
    order.splice(fromIndex, 1)
    order.splice(toIndex, 0, draggedId)
    nodeOrderKeys.value = order
    saveSavedNodeOrder(order)
  }
}

function resetNodeOrder() {
  localStorage.removeItem(NODE_ORDER_STORAGE_KEY)
  const currentNodes = overview.value?.nodes || []
  nodeOrderKeys.value = currentNodes.map(n => n.node_id || n.node_name).filter(Boolean)
}

function handleNodeCardClick(node: NodeMetrics) {
  if (hasDragged) return
  inspectNode(node)
}

function updateOverviewState(data: SystemOverview) {
  overview.value = data
  lastUpdated.value = new Date()

  // Maintain stable ordering: add any newly discovered nodes to the end without changing existing node positions
  const currentKeySet = new Set(nodeOrderKeys.value)
  let orderModified = false
  for (const n of data.nodes || []) {
    const key = n.node_id || n.node_name
    if (key && !currentKeySet.has(key)) {
      nodeOrderKeys.value.push(key)
      currentKeySet.add(key)
      orderModified = true
    }
  }
  if (orderModified && nodeOrderKeys.value.length > 0) {
    saveSavedNodeOrder(nodeOrderKeys.value)
  }

  // Push to trend history
  const timeStr = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  const newPoint: TrendPoint = {
    time: timeStr,
    cpu: Math.round(data.total_cpu_percent || 0),
    mem: Math.round(data.total_mem_percent || 0),
    disk: Math.round(data.total_disk_percent || 0),
    reqs: Math.round(data.requests_per_sec || 0),
  }

  trendHistory.value.push(newPoint)
  if (trendHistory.value.length > 30) {
    trendHistory.value.shift()
  }
}

// WebSocket Connection with metrics callback
useWebSocket({
  onMetrics: (metrics: SystemOverview) => {
    isLiveWs.value = true
    updateOverviewState(metrics)
    loading.value = false
    error.value = null
  },
})

// ==========================================
// 3. COMPUTED METRICS & DERIVED STATE
// ==========================================
const orderedNodes = computed<NodeMetrics[]>(() => {
  const incoming = overview.value?.nodes || []
  if (incoming.length === 0) return []

  const nodeMap = new Map<string, NodeMetrics>()
  for (const n of incoming) {
    const key = n.node_id || n.node_name
    nodeMap.set(key, n)
  }

  const result: NodeMetrics[] = []
  const seen = new Set<string>()

  // 1. Follow saved / current order
  for (const key of nodeOrderKeys.value) {
    const node = nodeMap.get(key)
    if (node) {
      result.push(node)
      seen.add(key)
    }
  }

  // 2. Append any incoming nodes not in saved order to the end
  for (const n of incoming) {
    const key = n.node_id || n.node_name
    if (!seen.has(key)) {
      result.push(n)
      seen.add(key)
    }
  }

  return result
})

const nodes = computed<NodeMetrics[]>(() => orderedNodes.value)
const totalContainers = computed(() => overview.value?.total_containers || 0)
const runningContainers = computed(() => overview.value?.running_containers || 0)

const activeAlerts = computed<MetricAlert[]>(() => {
  const alerts = overview.value?.alerts || []
  return alerts.filter(a => !dismissedAlerts.value.has(`${a.node_id}-${a.type}`))
})

// Topology Filter Counts & Predicate
function isNodeDegradedOrOffline(n: NodeMetrics): boolean {
  const isDown = n.status === 'down' || n.status === 'disconnected' || n.status === 'error' || n.status === 'offline'
  const isOverloaded = (n.cpu_percent || 0) >= 80 || (n.memory_percent || 0) >= 80 || (n.disk_percent || 0) >= 85
  const hasAlert = activeAlerts.value.some(a => a.node_id === n.node_id)
  return isDown || isOverloaded || hasAlert
}

const onlineActiveCount = computed(() => {
  return nodes.value.filter(n => !isNodeDegradedOrOffline(n)).length
})

const offlineDegradedCount = computed(() => {
  return nodes.value.filter(n => isNodeDegradedOrOffline(n)).length
})

// Filtered Topology Nodes based on selected filter pill
const filteredTopologyNodes = computed<NodeMetrics[]>(() => {
  if (selectedTopologyFilter.value === 'online') {
    return nodes.value.filter(n => !isNodeDegradedOrOffline(n))
  }
  if (selectedTopologyFilter.value === 'offline') {
    return nodes.value.filter(n => isNodeDegradedOrOffline(n))
  }
  return nodes.value
})

// Determine the node receiving the highest traffic
const busiestNodeId = computed<string | null>(() => {
  if (nodes.value.length === 0) return null
  let maxTraffic = -1
  let busiest: string | null = null
  for (const n of nodes.value) {
    const traffic = (n.network_rx_bytes || 0) + (n.network_tx_bytes || 0)
    if (traffic > maxTraffic) {
      maxTraffic = traffic
      busiest = n.node_id
    }
  }
  return busiest
})

// Selected Node for Diagnostics Drawer (syncs live with incoming metrics)
const selectedNode = computed<NodeMetrics | null>(() => {
  if (!selectedNodeId.value) return null
  return nodes.value.find(n => n.node_id === selectedNodeId.value) || null
})

// Filtered network interfaces for selected node (excluding lo and veth*)
const filteredNodeInterfaces = computed<NetworkInterface[]>(() => {
  const ifaces = selectedNode.value?.network_interfaces || []
  return ifaces.filter(iface => {
    const name = (iface.name || '').toLowerCase()
    if (name === 'lo' || name.startsWith('lo:') || name.startsWith('veth')) {
      return false
    }
    return true
  })
})

// Filtered and sorted services running on selected node
const nodeServices = computed<TpsServiceMetrics[]>(() => {
  if (!selectedNode.value || !tpsData.value?.services) return []
  const nodeId = (selectedNode.value.node_id || '').toLowerCase().trim()
  const nodeName = (selectedNode.value.node_name || '').toLowerCase().trim()
  const isSingleNode = (overview.value?.nodes?.length || 0) === 1

  return tpsData.value.services.filter(s => {
    const sNodeId = (s.node_id || '').toLowerCase().trim()
    const sNodeName = (s.node_name || '').toLowerCase().trim()

    if (sNodeId && (sNodeId === nodeId || sNodeId === nodeName)) return true
    if (sNodeName && (sNodeName === nodeName || sNodeName === nodeId)) return true
    if (sNodeName && nodeName) {
      if (nodeName === sNodeName || nodeName.includes(sNodeName) || sNodeName.includes(nodeName)) return true
      const normS = sNodeName.replace(/^(k8s|node-)/, '')
      const normN = nodeName.replace(/^(k8s|node-)/, '')
      if (normS && normN && (normS === normN || normS.includes(normN) || normN.includes(normS))) return true
    }
    if (isSingleNode && (!s.node_id || sNodeId === nodeId || sNodeId === nodeName)) return true
    return false
  }).sort((a, b) => {
    if (b.cpu_percent !== a.cpu_percent) {
      return b.cpu_percent - a.cpu_percent
    }
    return b.memory_used_mb - a.memory_used_mb
  })
})

// Filtered and sorted processes for selected node
const filteredNodeProcesses = computed<ProcessMetric[]>(() => {
  if (!selectedNode.value?.top_processes) return []
  let list = [...selectedNode.value.top_processes]

  if (processSearch.value.trim()) {
    const q = processSearch.value.toLowerCase().trim()
    list = list.filter(p =>
      p.name.toLowerCase().includes(q) ||
      (p.command_line && p.command_line.toLowerCase().includes(q)) ||
      (p.user && p.user.toLowerCase().includes(q)) ||
      String(p.pid).includes(q)
    )
  }

  if (processSortBy.value === 'cpu') {
    list.sort((a, b) => (b.cpu_percent || 0) - (a.cpu_percent || 0))
  } else if (processSortBy.value === 'mem') {
    list.sort((a, b) => (b.memory_bytes || 0) - (a.memory_bytes || 0))
  } else if (processSortBy.value === 'disk') {
    list.sort((a, b) => ((b.read_bytes_per_sec || 0) + (b.write_bytes_per_sec || 0)) - ((a.read_bytes_per_sec || 0) + (a.write_bytes_per_sec || 0)))
  } else if (processSortBy.value === 'pid') {
    list.sort((a, b) => a.pid - b.pid)
  } else if (processSortBy.value === 'name') {
    list.sort((a, b) => a.name.localeCompare(b.name))
  }

  return list
})

// Filtered and sorted containers
const filteredContainers = computed<ContainerMetrics[]>(() => {
  let list = overview.value?.containers || []

  if (selectedNodeFilter.value !== 'all') {
    list = list.filter(c => c.node_id === selectedNodeFilter.value)
  }

  if (containerSearch.value.trim()) {
    const q = containerSearch.value.toLowerCase().trim()
    list = list.filter(c => {
      const formatted = formatContainerName(c.container_name)
      const formattedImg = formatImageName(c.image)
      return (
        c.container_name.toLowerCase().includes(q) ||
        formatted.serviceName.toLowerCase().includes(q) ||
        (formatted.slotBadgeText && formatted.slotBadgeText.toLowerCase().includes(q)) ||
        c.image.toLowerCase().includes(q) ||
        formattedImg.display.toLowerCase().includes(q) ||
        c.node_id.toLowerCase().includes(q)
      )
    })
  }

  const sorted = [...list]
  if (containerSortBy.value === 'cpu') {
    sorted.sort((a, b) => (b.cpu_percent || 0) - (a.cpu_percent || 0))
  } else if (containerSortBy.value === 'memory') {
    sorted.sort((a, b) => (b.memory_percent || 0) - (a.memory_percent || 0))
  } else if (containerSortBy.value === 'name') {
    sorted.sort((a, b) => a.container_name.localeCompare(b.container_name))
  }

  return sorted
})

// ==========================================
// 4. FORMATTING & HELPER FUNCTIONS
// ==========================================
function inspectNode(node: NodeMetrics) {
  selectedNodeId.value = node.node_id
  processSearch.value = ''
  processSortBy.value = 'cpu'
  showNodeDrawer.value = true
}

function formatBytes(bytes?: number, decimals = 1): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i] || 'B'}`
}

function formatPercent(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0%'
  return `${Math.round(val * 10) / 10}%`
}

function formatUptime(uptimeSeconds?: number): string {
  if (!uptimeSeconds || uptimeSeconds <= 0) return 'Active'
  const days = Math.floor(uptimeSeconds / 86400)
  const hours = Math.floor((uptimeSeconds % 86400) / 3600)
  const minutes = Math.floor((uptimeSeconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

function formatLoadAvg(loadAvg?: [number, number, number] | number[]): string {
  if (!loadAvg || !Array.isArray(loadAvg) || loadAvg.length === 0) return '0.45, 0.32, 0.28'
  return loadAvg.map(n => (typeof n === 'number' ? n.toFixed(2) : String(n))).join(', ')
}

function formatOsSummary(node?: NodeMetrics | null): string {
  if (!node) return 'Linux | amd64'
  const distro = node.os_distro || (node.os ? node.os.toUpperCase() : 'Linux')
  const kernel = node.kernel_version ? `Linux ${node.kernel_version}` : (node.os || 'Linux')
  const arch = node.arch || 'amd64'
  return `${distro} | ${kernel} | ${arch}`
}

function formatIoRate(bytesPerSec?: number): string {
  if (!bytesPerSec || bytesPerSec <= 0 || isNaN(bytesPerSec)) return '0 B/s'
  return `${formatBytes(bytesPerSec)}/s`
}

function getUtilizationColor(percent: number): string {
  if (percent >= 80) return 'rose'
  if (percent >= 60) return 'amber'
  return 'emerald'
}

function getProcessCpuColor(percent: number): string {
  if (percent >= 70) return 'rose'
  if (percent >= 30) return 'amber'
  return 'emerald'
}

function getProcessStateBadgeClass(state: string): string {
  const s = (state || '').toLowerCase()
  if (s.includes('run') || s === 'r') return 'badge-emerald'
  if (s.includes('disk') || s === 'd') return 'badge-amber'
  if (s.includes('sleep') || s === 's' || s === 'i') return 'badge-indigo'
  if (s.includes('zombie') || s === 'z' || s.includes('stop') || s === 't') return 'badge-rose'
  return 'badge-cyan'
}

function getProcessStateLabel(state: string): string {
  const s = (state || '').toLowerCase()
  if (s.includes('run') || s === 'r') return 'Running'
  if (s.includes('disk') || s === 'd') return 'Disk Sleep'
  if (s.includes('sleep') || s === 's' || s === 'i') return 'Sleeping'
  if (s.includes('zombie') || s === 'z') return 'Zombie'
  if (s.includes('stop') || s === 't') return 'Stopped'
  return state || 'Active'
}

function getNodeCardClass(node: NodeMetrics): string {
  if (node.status === 'down') return 'node-down'
  if (node.cpu_percent >= 80 || node.memory_percent >= 80) return 'node-overloaded'
  if (node.cpu_percent >= 60 || node.memory_percent >= 60) return 'node-warning'
  return 'node-healthy'
}

function getAlertRecommendation(alert: MetricAlert): string {
  const t = alert.type.toLowerCase()
  if (t.includes('cpu')) {
    return 'Consider scaling horizontally or migrating heavy containers'
  }
  if (t.includes('mem') || t.includes('memory')) {
    return 'Check for memory leaks, increase node RAM'
  }
  if (t.includes('disk')) {
    return 'Clean up unused images/volumes, expand storage'
  }
  if (t.includes('down') || t.includes('node')) {
    return 'Check node connectivity, run diagnostics'
  }
  return 'Review resource quotas and cluster workload placement'
}

function dismissAlert(alert: MetricAlert) {
  dismissedAlerts.value.add(`${alert.node_id}-${alert.type}`)
}

function inspectContainer(c: ContainerMetrics) {
  selectedContainer.value = c
  showContainerDrawer.value = true
}

function getNodeName(nodeId: string): string {
  const n = nodes.value.find(node => node.node_id === nodeId)
  return n ? n.node_name : nodeId
}

// Sparkline SVG coordinates generator
const sparklinePoints = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const maxReqs = Math.max(...history.map(h => h.reqs), 10)
  const width = 120
  const height = 32
  const step = width / (history.length - 1)

  return history
    .map((h, i) => {
      const x = i * step
      const y = height - (h.reqs / maxReqs) * (height - 4) - 2
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
})

// Trend Chart SVG Points for 5-minute CPU / RAM
const trendChartCpuPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const width = 300
  const height = 80
  const step = width / (history.length - 1)

  return history
    .map((h, i) => {
      const x = i * step
      const y = height - (Math.min(100, h.cpu) / 100) * (height - 10) - 5
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const trendChartMemPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const width = 300
  const height = 80
  const step = width / (history.length - 1)

  return history
    .map((h, i) => {
      const x = i * step
      const y = height - (Math.min(100, h.mem) / 100) * (height - 10) - 5
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

function getHttpLatencyColor(latencyMs?: number): string {
  if (latencyMs === undefined || latencyMs === null || isNaN(latencyMs)) return 'text-muted'
  if (latencyMs > 500) return 'text-rose'
  if (latencyMs > 100) return 'text-amber'
  return 'text-emerald'
}

function getHttpErrorRateClass(errorRate?: number): string {
  if (errorRate === undefined || errorRate === null || isNaN(errorRate)) return 'badge-muted'
  if (errorRate > 5) return 'badge-rose'
  return 'badge-emerald'
}

function getDbCacheHitClass(hitRatio?: number): string {
  if (hitRatio === undefined || hitRatio === null || isNaN(hitRatio)) return 'badge-muted'
  const pct = hitRatio <= 1.0 ? hitRatio * 100 : hitRatio
  if (pct >= 95) return 'badge-emerald'
  if (pct >= 90) return 'badge-amber'
  return 'badge-rose'
}

function formatCacheHitRatio(ratio?: number): string {
  if (ratio === undefined || ratio === null || isNaN(ratio)) return '--'
  const pct = ratio <= 1.0 ? ratio * 100 : ratio
  return pct.toFixed(1) + '%'
}


// ==========================================
// 5. LIFECYCLE & CLEANUP
// ==========================================
onMounted(() => {
  fetchOverview()
  fetchTpsMetrics()

  // Polling fallback every 5 seconds if WebSocket is delayed
  fallbackInterval = setInterval(() => {
    fetchOverview()
    fetchTpsMetrics()
  }, 5000)
})

onUnmounted(() => {
  if (fallbackInterval) {
    clearInterval(fallbackInterval)
    fallbackInterval = null
  }
  if (abortController) {
    abortController.abort()
    abortController = null
  }
  if (tpsAbortController) {
    tpsAbortController.abort()
    tpsAbortController = null
  }
})

</script>

<template>
  <div class="overview-dashboard animate-fade-in">
    <!-- Header Bar -->
    <header class="dashboard-header">
      <div class="header-titles">
        <div class="header-badge-group">
          <span class="badge" :class="isLiveWs ? 'badge-emerald' : 'badge-cyan'">
            <span class="pulse-dot" :class="{ 'pulse-active': isLiveWs }"></span>
            {{ isLiveWs ? 'LIVE WEBSOCKET STREAM' : 'TELEMETRY SYNC' }}
          </span>
          <span class="badge badge-indigo">Auto-Refresh 5s</span>
          <span class="last-sync-text">Updated: {{ lastUpdated.toLocaleTimeString() }}</span>
        </div>
        <h1 class="page-title">Infrastructure & Node Mesh Overview</h1>
        <p class="page-desc">
          Real-time cluster topology, container saturation metrics, dynamic resource gauges, and autonomous threshold alerting.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" @click="() => { fetchOverview(); fetchTpsMetrics() }" :disabled="loading">
          <span class="btn-icon" :class="{ 'spin-icon': loading || tpsLoading }">🔄</span>
          <span>Refresh</span>
        </button>
        <button class="btn btn-secondary" @click="router.push('/deployments')">
          <span>🚀 Deployments & Rollouts</span>
        </button>
        <button class="btn btn-secondary" @click="router.push('/hosts')">
          <span>🖥️ Infrastructure Hosts</span>
        </button>
        <button class="btn btn-primary" @click="router.push('/fleet')">
          <span>☸️ Fleet Mesh</span>
        </button>
      </div>
    </header>

    <!-- SECTION 4: SLIDE-DOWN ALERT BANNERS -->
    <transition-group name="alert-slide" tag="section" class="alerts-section" v-if="activeAlerts.length > 0">
      <div
        v-for="alert in activeAlerts"
        :key="`${alert.node_id}-${alert.type}`"
        class="alert-banner glass-panel"
        :class="alert.value >= 90 || alert.type === 'node_down' ? 'alert-critical' : 'alert-warning'"
      >
        <div class="alert-icon-box">
          <span class="alert-emoji">⚠️</span>
        </div>
        <div class="alert-content">
          <div class="alert-title-row">
            <span class="alert-node-tag">{{ alert.node_name || alert.node_id }}</span>
            <span class="alert-type-tag">{{ alert.type.toUpperCase() }}</span>
            <span class="alert-value-tag">{{ Math.round(alert.value) }}% (Threshold: {{ alert.threshold }}%)</span>
          </div>
          <p class="alert-message">
            <strong>{{ alert.message }}:</strong> {{ getAlertRecommendation(alert) }}
          </p>
        </div>
        <div class="alert-actions">
          <button
            class="btn-alert-action"
            @click="router.push({ path: '/hosts', query: { search: alert.node_name || alert.node_id } })"
            title="Open host in Infrastructure Registry"
          >
            <span>⚙️ Open in Registry</span>
          </button>
          <button class="btn-alert-dismiss" @click="dismissAlert(alert)" title="Dismiss Alert">
            ✕
          </button>
        </div>
      </div>
    </transition-group>

    <!-- LOADING SKELETON -->
    <div v-if="loading && !overview" class="skeleton-hud-grid">
      <div v-for="i in 4" :key="i" class="skeleton-card glass-panel shimmer"></div>
    </div>

    <!-- ERROR STATE -->
    <div v-else-if="error && !overview" class="error-banner glass-panel">
      <div class="error-icon">⚠️</div>
      <div class="error-info">
        <h3>Telemetry Connection Interrupted</h3>
        <p>{{ error }}</p>
      </div>
      <button class="btn btn-primary" @click="fetchOverview">Retry Telemetry Sync</button>
    </div>

    <!-- EMPTY STATE -->
    <div v-else-if="nodes.length === 0 && !loading" class="empty-state-card glass-panel">
      <div class="empty-icon">🖥️</div>
      <h2>No Infrastructure Servers Connected</h2>
      <p>Deploy k8s-agent (port 9100) on your hosts or attach compute clusters to enable real-time telemetry streaming.</p>
      <div class="empty-actions">
        <button class="btn btn-primary" @click="router.push('/hosts')">Manage Infrastructure Hosts</button>
        <button class="btn btn-secondary" @click="router.push('/settings')">Configure Settings</button>
      </div>
    </div>

    <!-- MAIN DASHBOARD CONTENT -->
    <div v-else-if="overview" class="dashboard-body">
      <!-- SECTION 1: SUMMARY HUD (TOP BAR) -->
      <section class="summary-hud-grid">
        <!-- Card 1: Nodes -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Nodes Online</span>
            <span
              class="status-indicator-dot"
              :class="overview.healthy_nodes === overview.total_nodes ? 'status-green' : 'status-amber'"
            ></span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value">{{ overview.healthy_nodes }} <span class="hud-total">/ {{ overview.total_nodes }}</span></span>
            <span class="badge" :class="overview.healthy_nodes === overview.total_nodes ? 'badge-emerald' : 'badge-amber'">
              {{ overview.healthy_nodes === overview.total_nodes ? '100% HEALTHY' : 'DEGRADED' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill bg-emerald"
              :style="{ width: `${overview.total_nodes ? (overview.healthy_nodes / overview.total_nodes) * 100 : 0}%` }"
            ></div>
          </div>
        </div>

        <!-- Card 2: Containers -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Containers</span>
            <span class="hud-icon">📦</span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value">{{ runningContainers }} <span class="hud-total">/ {{ totalContainers }}</span></span>
            <span class="badge badge-cyan">RUNNING</span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill bg-cyan"
              :style="{ width: `${totalContainers ? (runningContainers / totalContainers) * 100 : 0}%` }"
            ></div>
          </div>
        </div>

        <!-- Card 3: Avg CPU -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Avg CPU Saturation</span>
            <span class="hud-badge-tag" :class="`text-${getUtilizationColor(overview.total_cpu_percent)}`">
              {{ formatPercent(overview.total_cpu_percent) }}
            </span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value">{{ Math.round(overview.total_cpu_percent) }}%</span>
            <span class="badge" :class="`badge-${getUtilizationColor(overview.total_cpu_percent)}`">
              {{ overview.total_cpu_percent >= 80 ? 'CRITICAL' : overview.total_cpu_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill"
              :class="`bg-${getUtilizationColor(overview.total_cpu_percent)}`"
              :style="{ width: `${Math.min(100, overview.total_cpu_percent)}%` }"
            ></div>
          </div>
        </div>

        <!-- Card 4: Avg RAM -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Avg Memory Saturation</span>
            <span class="hud-badge-tag" :class="`text-${getUtilizationColor(overview.total_mem_percent)}`">
              {{ formatPercent(overview.total_mem_percent) }}
            </span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value">{{ Math.round(overview.total_mem_percent) }}%</span>
            <span class="badge" :class="`badge-${getUtilizationColor(overview.total_mem_percent)}`">
              {{ overview.total_mem_percent >= 80 ? 'CRITICAL' : overview.total_mem_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill"
              :class="`bg-${getUtilizationColor(overview.total_mem_percent)}`"
              :style="{ width: `${Math.min(100, overview.total_mem_percent)}%` }"
            ></div>
          </div>
        </div>

        <!-- Card 5: Request Throughput with Sparkline -->
        <div class="hud-card glass-panel hud-card-throughput">
          <div class="hud-card-top">
            <span class="hud-label">Throughput</span>
            <span class="badge badge-violet">LIVE EDGE</span>
          </div>
          <div class="hud-value-row">
            <div class="throughput-details">
              <span class="hud-value">{{ Math.round(overview.requests_per_sec) }}</span>
              <span class="hud-unit">req/s</span>
            </div>
            <!-- SVG Sparkline -->
            <div class="sparkline-container" v-if="sparklinePoints">
              <svg viewBox="0 0 120 32" class="sparkline-svg">
                <polyline
                  fill="none"
                  stroke="#8b5cf6"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  :points="sparklinePoints"
                />
              </svg>
            </div>
          </div>
          <div class="hud-card-footer-text">
            <span>Aggregated API gateway ingress stream</span>
          </div>
        </div>
      </section>

      <!-- SECTION 2: INFRASTRUCTURE SERVER TOPOLOGY -->
      <section class="topology-section">
        <div class="section-title-bar">
          <div class="section-title-group">
            <h2 class="section-title">Infrastructure Server Topology</h2>
            <span class="section-subtitle">Real-time k8s-agent compute nodes & hardware gauge telemetry</span>
          </div>
          <div class="topology-legend">
            <span class="legend-item"><span class="legend-dot bg-emerald"></span> &lt;60% Nominal</span>
            <span class="legend-item"><span class="legend-dot bg-amber"></span> 60-80% Elevated</span>
            <span class="legend-item"><span class="legend-dot bg-rose"></span> &gt;80% Overload</span>
            <span class="legend-item"><span class="legend-dot bg-gray"></span> Offline</span>
          </div>
        </div>

        <!-- Mesh Topology Connections Indicator -->
        <div class="mesh-interconnect-bar">
          <div class="mesh-pill">
            <span class="mesh-icon">⚡</span>
            <span>Mesh Interconnect Active</span>
            <span class="mesh-nodes-count">({{ nodes.length }} servers linked)</span>
          </div>
          <div class="mesh-line-stream">
            <div class="mesh-pulse-line"></div>
          </div>
        </div>

        <!-- Topology Filter Pills Bar -->
        <div class="topology-filter-bar">
          <div class="filter-pills-group">
            <button
              class="filter-pill-btn"
              :class="{ 'filter-pill-active': selectedTopologyFilter === 'all' }"
              @click="selectedTopologyFilter = 'all'"
            >
              <span class="filter-pill-icon">🌐</span>
              <span>All Infrastructure Servers</span>
              <span class="filter-pill-count">({{ nodes.length }})</span>
            </button>

            <button
              class="filter-pill-btn"
              :class="{ 'filter-pill-active': selectedTopologyFilter === 'online' }"
              @click="selectedTopologyFilter = 'online'"
            >
              <span class="filter-pill-icon">🟢</span>
              <span>Online / Active</span>
              <span class="filter-pill-count">({{ onlineActiveCount }})</span>
            </button>

            <button
              class="filter-pill-btn"
              :class="[
                { 'filter-pill-active': selectedTopologyFilter === 'offline' },
                offlineDegradedCount > 0 ? 'filter-pill-alert' : ''
              ]"
              @click="selectedTopologyFilter = 'offline'"
            >
              <span class="filter-pill-icon">🚨</span>
              <span>Offline / Degraded</span>
              <span class="filter-pill-count">({{ offlineDegradedCount }})</span>
            </button>
          </div>

          <div class="filter-meta-text">
            <span>Showing {{ filteredTopologyNodes.length }} of {{ nodes.length }} servers</span>
            <button
              v-if="hasCustomNodeOrder"
              class="btn-reset-order"
              @click="resetNodeOrder"
              title="Reset nodes to default ordering"
            >
              ↺ Reset Layout
            </button>
          </div>
        </div>

        <!-- Node Cards Grid -->
        <div class="node-cards-grid">
          <div
            v-if="filteredTopologyNodes.length === 0"
            class="empty-topology-state glass-panel"
          >
            <span class="empty-topology-icon">🔍</span>
            <span class="empty-topology-text">No servers match the selected filter "{{ selectedTopologyFilter }}".</span>
            <button class="btn-reset-filters" @click="selectedTopologyFilter = 'all'">Show All Servers</button>
          </div>

          <div
            v-for="node in filteredTopologyNodes"
            :key="node.node_id"
            class="node-card glass-panel cursor-pointer"
            :class="[
              getNodeCardClass(node),
              {
                'is-busiest': node.node_id === busiestNodeId,
                'is-dragging': draggedNodeId === node.node_id,
                'is-drag-over': dragOverNodeId === node.node_id && draggedNodeId !== node.node_id,
              }
            ]"
            draggable="true"
            @dragstart="onDragStart($event, node)"
            @dragover.prevent="onDragOver($event, node)"
            @dragenter.prevent="onDragEnter(node)"
            @dragleave="onDragLeave($event, node)"
            @drop.prevent="onDrop(node)"
            @dragend="onDragEnd"
            @click="handleNodeCardClick(node)"
          >
            <!-- Card Top: Name, Source Badge, Role & Status -->
            <div class="node-card-header">
              <div class="node-identity">
                <span class="drag-handle" title="Drag to reorder server card">⋮⋮</span>
                <span
                  class="node-status-dot"
                  :class="node.status === 'ready' || node.status === 'online' ? 'status-green' : 'status-red'"
                ></span>
                <span class="node-name" :title="node.node_name">{{ node.node_name }}</span>
              </div>
              <div class="node-badge-row">
                <!-- OS Distro Badge -->
                <span class="badge badge-slate font-mono" :title="formatOsSummary(node)">
                  🐧 {{ node.os_distro || (node.os ? node.os.toUpperCase() : 'Linux') }}
                </span>
                <!-- Source Badge -->
                <span class="badge badge-indigo">
                  📡 K8S-AGENT
                </span>
                <span v-if="node.node_id === busiestNodeId" class="badge badge-amber badge-traffic-pulse" title="Highest traffic node">
                  🔥 HOT NODE
                </span>
              </div>
            </div>

            <!-- Gauge Cluster: CPU / RAM / Disk -->
            <div class="node-gauges-cluster">
              <div class="gauge-col">
                <CircularGauge
                  :percent="node.cpu_percent"
                  label="CPU"
                  :size="52"
                  :strokeWidth="3.5"
                  :disabled="node.status === 'down' || node.status === 'offline' || node.status === 'disconnected'"
                />
              </div>
              <div class="gauge-col">
                <CircularGauge
                  :percent="node.memory_percent"
                  label="RAM"
                  :size="52"
                  :strokeWidth="3.5"
                  :disabled="node.status === 'down' || node.status === 'offline' || node.status === 'disconnected'"
                />
              </div>
              <div class="gauge-col">
                <CircularGauge
                  :percent="node.disk_percent"
                  label="DISK"
                  :size="52"
                  :strokeWidth="3.5"
                  :disabled="node.status === 'down' || node.status === 'offline' || node.status === 'disconnected'"
                />
              </div>
            </div>

            <!-- Node Resource Detail Stats -->
            <div class="node-meta-grid">
              <div class="meta-item">
                <span class="meta-label">Memory</span>
                <span class="meta-val">{{ formatBytes(node.memory_used) }} / {{ formatBytes(node.memory_total) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Disk Storage</span>
                <span class="meta-val">{{ formatBytes(node.disk_used) }} / {{ formatBytes(node.disk_total) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Network I/O</span>
                <span class="meta-val font-mono">
                  ↑ {{ formatBytes(node.network_tx_bytes) }}/s  ↓ {{ formatBytes(node.network_rx_bytes) }}/s
                </span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Processes</span>
                <span class="meta-val">{{ node.running_count || node.container_count || 0 }}</span>
              </div>
            </div>

            <!-- Footer Actions: Direct Navigation & Process Inspection -->
            <div class="node-card-footer">
              <div class="node-footer-actions">
                <button
                  class="btn-node-action btn-inspect-node"
                  @click.stop="inspectNode(node)"
                  title="Inspect real-time telemetry, hardware saturation, and top processes"
                >
                  <span>🔍 Inspect Telemetry & Apps</span>
                </button>
                <button
                  class="btn-node-action btn-manage-host"
                  @click.stop="router.push({ path: '/hosts', query: { search: node.node_name } })"
                  title="Manage server in Infrastructure Registry"
                >
                  <span>⚙️ Manage</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- SECTION: REAL-TIME THROUGHPUT MONITOR (TPS) -->
      <section class="tps-monitor-section">
        <div class="section-title-bar">
          <div class="section-title-group">
            <h2 class="section-title">📊 Real-Time Throughput Monitor</h2>
            <span class="section-subtitle">
              Unified ingress, edge gateway, database transactions & messaging
            </span>
          </div>
          <div class="tps-header-meta">
            <span class="badge badge-indigo">
              <span class="pulse-dot"></span>
              Auto-Refresh 5s
            </span>
            <span class="tps-sync-time" v-if="tpsLastUpdated">
              Updated: {{ tpsLastUpdated.toLocaleTimeString() }}
            </span>
          </div>
        </div>

        <!-- 4 CATEGORY CARDS GRID -->
        <div class="tps-cards-grid">
          <!-- Card 1: Network I/O -->
          <div class="tps-card glass-panel">
            <div class="tps-card-top">
              <div class="tps-card-identity">
                <span class="tps-icon">🌐</span>
                <span class="tps-label">Network I/O</span>
              </div>
              <span class="badge badge-cyan">WIRE STREAM</span>
            </div>

            <div class="tps-highlight-row">
              <div class="tps-big-stat">
                <span class="tps-big-num font-mono">
                  {{ tpsData?.network ? formatBytes(tpsData.network.total_rx_bytes_per_sec + tpsData.network.total_tx_bytes_per_sec) : '--' }}
                </span>
                <span class="tps-big-unit">/s total</span>
              </div>
            </div>

            <div class="tps-metrics-block">
              <div class="tps-metric-row">
                <span class="tps-metric-key">Total Rx</span>
                <span class="tps-metric-val text-cyan font-mono">
                  ↓ {{ tpsData?.network ? formatBytes(tpsData.network.total_rx_bytes_per_sec) + '/s' : '--' }}
                </span>
              </div>
              <div class="tps-metric-row">
                <span class="tps-metric-key">Total Tx</span>
                <span class="tps-metric-val text-violet font-mono">
                  ↑ {{ tpsData?.network ? formatBytes(tpsData.network.total_tx_bytes_per_sec) + '/s' : '--' }}
                </span>
              </div>
            </div>

            <!-- Sparkline / Bar indicator -->
            <div class="tps-indicator-wrap">
              <div class="tps-ratio-bar">
                <div
                  class="tps-bar-rx"
                  :style="{
                    width: `${
                      tpsData?.network && (tpsData.network.total_rx_bytes_per_sec + tpsData.network.total_tx_bytes_per_sec > 0)
                        ? (tpsData.network.total_rx_bytes_per_sec / (tpsData.network.total_rx_bytes_per_sec + tpsData.network.total_tx_bytes_per_sec)) * 100
                        : 50
                    }%`
                  }"
                  title="Rx Bandwidth Share"
                ></div>
                <div
                  class="tps-bar-tx"
                  :style="{
                    width: `${
                      tpsData?.network && (tpsData.network.total_rx_bytes_per_sec + tpsData.network.total_tx_bytes_per_sec > 0)
                        ? (tpsData.network.total_tx_bytes_per_sec / (tpsData.network.total_rx_bytes_per_sec + tpsData.network.total_tx_bytes_per_sec)) * 100
                        : 50
                    }%`
                  }"
                  title="Tx Bandwidth Share"
                ></div>
              </div>
              <div class="tps-ratio-labels font-mono">
                <span>Rx Share</span>
                <span>Tx Share</span>
              </div>
            </div>
          </div>

          <!-- Card 2: HTTP Gateway (Traefik) -->
          <div class="tps-card glass-panel">
            <div class="tps-card-top">
              <div class="tps-card-identity">
                <span class="tps-icon">🔗</span>
                <span class="tps-label">HTTP Gateway (Traefik)</span>
              </div>
              <span
                class="badge"
                :class="getHttpErrorRateClass(tpsData?.http?.error_rate)"
              >
                {{ tpsData?.http ? tpsData.http.error_rate.toFixed(1) + '% Err' : '--' }}
              </span>
            </div>

            <div class="tps-highlight-row">
              <div class="tps-big-stat">
                <span class="tps-big-num font-mono">
                  {{ tpsData?.http ? tpsData.http.requests_per_sec.toFixed(1) : '--' }}
                </span>
                <span class="tps-big-unit">req/s</span>
              </div>
              <div class="tps-badge-group">
                <span class="tps-sub-badge font-mono">
                  ⚡ {{ tpsData?.http?.active_connections ?? '--' }} conn
                </span>
              </div>
            </div>

            <div class="tps-metrics-block">
              <div class="tps-metric-row">
                <span class="tps-metric-key">Active Connections</span>
                <span class="tps-metric-val font-mono">
                  {{ tpsData?.http?.active_connections ?? '--' }}
                </span>
              </div>
              <div class="tps-metric-row">
                <span class="tps-metric-key">Error Rate</span>
                <span
                  class="tps-metric-val font-mono"
                  :class="(tpsData?.http?.error_rate ?? 0) > 5 ? 'text-rose font-bold' : 'text-emerald'"
                >
                  {{ tpsData?.http ? tpsData.http.error_rate.toFixed(1) + '%' : '--' }}
                </span>
              </div>
              <div class="tps-metric-row">
                <span class="tps-metric-key">Avg Latency</span>
                <span
                  class="tps-metric-val font-mono"
                  :class="getHttpLatencyColor(tpsData?.http?.avg_latency_ms)"
                >
                  {{ tpsData?.http?.avg_latency_ms != null ? tpsData.http.avg_latency_ms.toFixed(1) + 'ms' : '--' }}
                </span>
              </div>
            </div>
          </div>

          <!-- Card 3: Database (PostgreSQL) -->
          <div class="tps-card glass-panel">
            <div class="tps-card-top">
              <div class="tps-card-identity">
                <span class="tps-icon">🗄️</span>
                <span class="tps-label">Database (PostgreSQL)</span>
              </div>
              <span
                class="badge"
                :class="getDbCacheHitClass(tpsData?.database?.cache_hit_ratio)"
              >
                {{ formatCacheHitRatio(tpsData?.database?.cache_hit_ratio) }} Hit
              </span>
            </div>

            <div class="tps-highlight-row">
              <div class="tps-big-stat">
                <span class="tps-big-num font-mono">
                  {{ tpsData?.database ? tpsData.database.transactions_per_sec.toFixed(1) : '--' }}
                </span>
                <span class="tps-big-unit">tx/s</span>
              </div>
              <div class="tps-badge-group">
                <span class="tps-sub-badge font-mono">
                  🔌 {{ tpsData?.database?.active_connections ?? '--' }} conn
                </span>
              </div>
            </div>

            <div class="tps-metrics-block">
              <div class="tps-metric-row">
                <span class="tps-metric-key">Reads</span>
                <span class="tps-metric-val text-cyan font-mono">
                  {{ tpsData?.database ? tpsData.database.reads_per_sec.toFixed(1) + ' rows/s' : '--' }}
                </span>
              </div>
              <div class="tps-metric-row">
                <span class="tps-metric-key">Writes</span>
                <span class="tps-metric-val text-amber font-mono">
                  {{ tpsData?.database ? tpsData.database.writes_per_sec.toFixed(1) + ' rows/s' : '--' }}
                </span>
              </div>
              <div class="tps-metric-row">
                <span class="tps-metric-key">Cache Hit</span>
                <span
                  class="tps-metric-val font-mono"
                  :class="
                    ((tpsData?.database?.cache_hit_ratio ?? 0) <= 1.0 ? (tpsData?.database?.cache_hit_ratio ?? 0) * 100 : (tpsData?.database?.cache_hit_ratio ?? 0)) >= 95
                      ? 'text-emerald'
                      : ((tpsData?.database?.cache_hit_ratio ?? 0) <= 1.0 ? (tpsData?.database?.cache_hit_ratio ?? 0) * 100 : (tpsData?.database?.cache_hit_ratio ?? 0)) >= 90
                      ? 'text-amber'
                      : 'text-rose'
                  "
                >
                  {{ formatCacheHitRatio(tpsData?.database?.cache_hit_ratio) }}
                </span>
              </div>
            </div>
          </div>

          <!-- Card 4: Messaging (NATS) -->
          <div class="tps-card glass-panel">
            <div class="tps-card-top">
              <div class="tps-card-identity">
                <span class="tps-icon">📨</span>
                <span class="tps-label">Messaging (NATS)</span>
              </div>
              <span class="badge" :class="tpsData?.messaging?.connections ? 'badge-emerald' : 'badge-muted'">PUB/SUB</span>
            </div>

            <div class="tps-highlight-row">
              <div class="tps-big-stat">
                <span class="tps-big-num font-mono">
                  {{ tpsData?.messaging ? (tpsData.messaging.in_msgs_per_sec + tpsData.messaging.out_msgs_per_sec).toFixed(1) : '--' }}
                </span>
                <span class="tps-big-unit">msg/s</span>
              </div>
              <div class="tps-badge-group">
                <span class="tps-sub-badge font-mono">
                  👥 {{ tpsData?.messaging?.connections != null ? (tpsData.messaging.connections > 0 ? tpsData.messaging.connections + ' conn' : '0 conn') : 'N/A' }}
                </span>
              </div>
            </div>

            <div class="tps-metrics-block">
              <div class="tps-metric-row">
                <span class="tps-metric-key">In Rate</span>
                <span class="tps-metric-val text-cyan font-mono">
                  {{ tpsData?.messaging ? tpsData.messaging.in_msgs_per_sec.toFixed(1) + ' msg/s' : '--' }}
                </span>
              </div>
              <div class="tps-metric-row">
                <span class="tps-metric-key">Out Rate</span>
                <span class="tps-metric-val text-violet font-mono">
                  {{ tpsData?.messaging ? tpsData.messaging.out_msgs_per_sec.toFixed(1) + ' msg/s' : '--' }}
                </span>
              </div>
              <div class="tps-metric-row">
                <span class="tps-metric-key">Bandwidth</span>
                <span class="tps-metric-val font-mono">
                  <span class="text-cyan">↓{{ tpsData?.messaging ? formatBytes(tpsData.messaging.in_bytes_per_sec) : '--' }}</span>
                  <span class="text-muted"> / </span>
                  <span class="text-violet">↑{{ tpsData?.messaging ? formatBytes(tpsData.messaging.out_bytes_per_sec) : '--' }}</span>
                </span>
              </div>
            </div>
          </div>
        </div>


      </section>

      <!-- SECTION 3 & 5: SPLIT CONTAINER GRID & LIVE ACTIVITY / TRENDS -->
      <div class="dashboard-split-layout">

        <!-- SECTION 3: CONTAINER GRID -->
        <section class="container-grid-section glass-panel">
          <div class="split-header">
            <div class="split-title-group">
              <div class="flex items-center gap-2">
                <h2 class="section-title">Active Workload Containers</h2>
                <button class="btn btn-secondary btn-xs" @click="router.push('/deployments')" title="Open Deployment & Workload Orchestrator">
                  <span>🚀 Manage Workloads ➔</span>
                </button>
              </div>
              <span class="section-subtitle">
                Showing {{ filteredContainers.length }} workloads
                <span v-if="selectedNodeFilter !== 'all'">on node "{{ getNodeName(selectedNodeFilter) }}"</span>
              </span>
            </div>

            <!-- Filters -->
            <div class="container-filters">
              <div class="search-box">
                <span class="search-icon">🔍</span>
                <input
                  v-model="containerSearch"
                  type="text"
                  placeholder="Search container or image..."
                  class="input-search"
                />
                <button v-if="containerSearch" class="btn-clear-search" @click="containerSearch = ''">✕</button>
              </div>

              <!-- Node Filter Selector -->
              <select v-model="selectedNodeFilter" class="select-filter">
                <option value="all">All Nodes ({{ nodes.length }})</option>
                <option v-for="node in nodes" :key="node.node_id" :value="node.node_id">
                  {{ node.node_name }} ({{ node.container_count }})
                </option>
              </select>

              <!-- Sort Selector -->
              <select v-model="containerSortBy" class="select-filter">
                <option value="cpu">Sort: CPU (Hottest First)</option>
                <option value="memory">Sort: Memory</option>
                <option value="name">Sort: Name</option>
              </select>
            </div>
          </div>

          <!-- Containers List / Table -->
          <div class="containers-table-wrapper">
            <table class="containers-table" v-if="filteredContainers.length > 0">
              <thead>
                <tr>
                  <th>Container Name</th>
                  <th>Image</th>
                  <th>Node</th>
                  <th>Status</th>
                  <th>CPU Usage</th>
                  <th>RAM Usage</th>
                  <th>Network</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="c in filteredContainers"
                  :key="c.container_id"
                  class="container-row"
                  @click="inspectContainer(c)"
                >
                  <td class="col-name" :title="c.container_name">
                    <div class="container-name-row">
                      <span class="container-primary-name">{{ formatContainerName(c.container_name).serviceName }}</span>
                      <span
                        v-if="formatContainerName(c.container_name).slotBadgeText"
                        class="slot-badge font-mono"
                        :title="`Full Task Name: ${c.container_name}`"
                      >
                        {{ formatContainerName(c.container_name).slotBadgeText }}
                      </span>
                    </div>
                    <span class="container-id-sub">{{ c.container_id.slice(0, 12) }}</span>
                  </td>
                  <td class="col-image" :title="c.image">
                    <span class="image-text">{{ formatImageName(c.image).display }}</span>
                  </td>
                  <td class="col-node">
                    <span class="badge badge-cyan">{{ getNodeName(c.node_id) }}</span>
                  </td>
                  <td class="col-state">
                    <span
                      class="badge"
                      :class="c.state === 'running' ? 'badge-emerald' : 'badge-rose'"
                    >
                      {{ c.state.toUpperCase() }}
                    </span>
                  </td>
                  <td class="col-cpu">
                    <div class="usage-metric-cell">
                      <span class="usage-text">{{ formatPercent(c.cpu_percent) }}</span>
                      <div class="mini-bar-track">
                        <div
                          class="mini-bar-fill"
                          :class="`bg-${getUtilizationColor(c.cpu_percent)}`"
                          :style="{ width: `${Math.min(100, c.cpu_percent)}%` }"
                        ></div>
                      </div>
                    </div>
                  </td>
                  <td class="col-ram">
                    <div class="usage-metric-cell">
                      <span class="usage-text">{{ formatBytes(c.memory_used) }}</span>
                      <div class="mini-bar-track">
                        <div
                          class="mini-bar-fill"
                          :class="`bg-${getUtilizationColor(c.memory_percent)}`"
                          :style="{ width: `${Math.min(100, c.memory_percent)}%` }"
                        ></div>
                      </div>
                    </div>
                  </td>
                  <td class="col-net font-mono">
                    ↑{{ formatBytes(c.network_tx) }} ↓{{ formatBytes(c.network_rx) }}
                  </td>
                  <td class="col-action">
                    <button class="btn-inspect" @click.stop="inspectContainer(c)">
                      Inspect
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>

            <div v-else class="empty-filter-state">
              <span>No containers match current filters.</span>
              <button class="btn-reset-filters" @click="containerSearch = ''; selectedNodeFilter = 'all'">
                Reset Filters
              </button>
            </div>
          </div>
        </section>

        <!-- SECTION 5: LIVE ACTIVITY FEED & 5-MIN TRENDS -->
        <aside class="trends-activity-sidebar">
          <!-- 5-Minute Trend Chart Card -->
          <div class="trend-chart-card glass-panel">
            <div class="trend-chart-header">
              <h3 class="sidebar-card-title">5-Min Saturation Trends</h3>
              <div class="trend-legend">
                <span class="legend-line cpu-legend">CPU</span>
                <span class="legend-line mem-legend">RAM</span>
              </div>
            </div>

            <!-- SVG Multi-Line Chart -->
            <div class="trend-svg-box">
              <svg viewBox="0 0 300 80" class="trend-svg" preserveAspectRatio="none">
                <!-- Grid Lines -->
                <line x1="0" y1="20" x2="300" y2="20" stroke="rgba(255,255,255,0.05)" stroke-dasharray="2,2" />
                <line x1="0" y1="45" x2="300" y2="45" stroke="rgba(255,255,255,0.05)" stroke-dasharray="2,2" />
                <line x1="0" y1="70" x2="300" y2="70" stroke="rgba(255,255,255,0.05)" stroke-dasharray="2,2" />

                <!-- Memory Path (Cyan) -->
                <path
                  v-if="trendChartMemPath"
                  :d="trendChartMemPath"
                  fill="none"
                  stroke="#06b6d4"
                  stroke-width="2"
                  stroke-linecap="round"
                />

                <!-- CPU Path (Violet) -->
                <path
                  v-if="trendChartCpuPath"
                  :d="trendChartCpuPath"
                  fill="none"
                  stroke="#8b5cf6"
                  stroke-width="2"
                  stroke-linecap="round"
                />
              </svg>
            </div>
            <div class="trend-footer-stats">
              <div class="stat-pair">
                <span class="stat-k">Current CPU</span>
                <span class="stat-v text-violet">{{ formatPercent(overview.total_cpu_percent) }}</span>
              </div>
              <div class="stat-pair">
                <span class="stat-k">Current RAM</span>
                <span class="stat-v text-cyan">{{ formatPercent(overview.total_mem_percent) }}</span>
              </div>
              <div class="stat-pair">
                <span class="stat-k">Storage</span>
                <span class="stat-v text-emerald">{{ formatPercent(overview.total_disk_percent) }}</span>
              </div>
            </div>
          </div>

          <!-- Live Ingress Activity Feed -->
          <div class="activity-feed-card glass-panel">
            <div class="activity-header">
              <div class="activity-title-group">
                <span class="live-dot pulse-active"></span>
                <h3 class="sidebar-card-title">Live Ingress Activity</h3>
              </div>
              <span class="badge badge-emerald">Streaming</span>
            </div>

            <div class="activity-items-list">
              <div
                v-if="recentActivity.length === 0"
                class="empty-activity text-muted"
                style="padding: 24px; text-align: center; font-size: 13px;"
              >
                No recent activity
              </div>
              <div
                v-else
                v-for="item in recentActivity"
                :key="item.id"
                class="activity-log-row animate-fade-in"
              >
                <div class="activity-meta">
                  <span class="activity-method" :class="`method-${item.method.toLowerCase()}`">{{ item.method }}</span>
                  <span class="activity-path" :title="item.path">{{ item.path }}</span>
                </div>
                <div class="activity-timing">
                  <span class="activity-status" :class="item.status === 200 ? 'status-ok' : 'status-err'">
                    {{ item.status }}
                  </span>
                  <span class="activity-dur">{{ item.duration }}ms</span>
                  <span class="activity-time">{{ item.time }}</span>
                </div>
              </div>
            </div>
          </div>
        </aside>
      </div>
    </div>

    <!-- CONTAINER DETAILS MODAL DRAWER -->
    <ModalDrawer
      :show="showContainerDrawer"
      mode="drawer"
      title="Container Metric Details"
      @close="showContainerDrawer = false"
      @update:show="showContainerDrawer = $event"
    >
      <div v-if="selectedContainer" class="drawer-container-details">
        <div class="drawer-header-card glass-panel">
          <div class="drawer-title-row">
            <div class="drawer-title-wrap">
              <h3 class="drawer-container-title" :title="selectedContainer.container_name">
                {{ formatContainerName(selectedContainer.container_name).serviceName }}
              </h3>
              <span
                v-if="formatContainerName(selectedContainer.container_name).slotBadgeText"
                class="slot-badge font-mono"
                :title="`Full Task Name: ${selectedContainer.container_name}`"
              >
                {{ formatContainerName(selectedContainer.container_name).slotBadgeText }}
              </span>
            </div>
            <span class="badge" :class="selectedContainer.state === 'running' ? 'badge-emerald' : 'badge-rose'">
              {{ selectedContainer.state.toUpperCase() }}
            </span>
          </div>
          <span class="drawer-sub-image font-mono" :title="selectedContainer.image">
            {{ formatImageName(selectedContainer.image).display }}
          </span>
        </div>

        <div class="drawer-metrics-grid">
          <div class="drawer-metric-box glass-panel">
            <span class="box-label">CPU Saturation</span>
            <span class="box-value" :class="`text-${getUtilizationColor(selectedContainer.cpu_percent)}`">
              {{ formatPercent(selectedContainer.cpu_percent) }}
            </span>
            <div class="mini-bar-track">
              <div
                class="mini-bar-fill"
                :class="`bg-${getUtilizationColor(selectedContainer.cpu_percent)}`"
                :style="{ width: `${Math.min(100, selectedContainer.cpu_percent)}%` }"
              ></div>
            </div>
          </div>

          <div class="drawer-metric-box glass-panel">
            <span class="box-label">Memory Used / Limit</span>
            <span class="box-value text-cyan">
              {{ formatBytes(selectedContainer.memory_used) }}
            </span>
            <span class="box-sub">Limit: {{ formatBytes(selectedContainer.memory_limit) }} ({{ formatPercent(selectedContainer.memory_percent) }})</span>
          </div>
        </div>

        <div class="drawer-meta-list glass-panel">
          <div class="drawer-meta-row" v-if="formatContainerName(selectedContainer.container_name).isSwarmTask">
            <span class="dm-label">Full Swarm Task</span>
            <span class="dm-val font-mono">{{ selectedContainer.container_name }}</span>
          </div>
          <div class="drawer-meta-row">
            <span class="dm-label">Container ID</span>
            <span class="dm-val font-mono">{{ selectedContainer.container_id }}</span>
          </div>
          <div class="drawer-meta-row">
            <span class="dm-label">Host Node ID</span>
            <span class="dm-val">{{ getNodeName(selectedContainer.node_id) }} ({{ selectedContainer.node_id }})</span>
          </div>
          <div class="drawer-meta-row">
            <span class="dm-label">Network Ingress (RX)</span>
            <span class="dm-val font-mono">{{ formatBytes(selectedContainer.network_rx) }}</span>
          </div>
          <div class="drawer-meta-row">
            <span class="dm-label">Network Egress (TX)</span>
            <span class="dm-val font-mono">{{ formatBytes(selectedContainer.network_tx) }}</span>
          </div>
        </div>

        <div class="drawer-actions">
          <button class="btn btn-secondary w-full" @click="router.push('/hosts')">
            Manage in Infrastructure Hosts
          </button>
          <button class="btn btn-secondary w-full" @click="router.push('/logs')">
            Stream Container Logs
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- NODE DIAGNOSTICS & TOP PROCESSES INSPECTOR DRAWER -->
    <ModalDrawer
      :show="showNodeDrawer"
      mode="drawer"
      placement="right"
      maxWidth="820px"
      :title="`Node Diagnostics: ${selectedNode?.node_name || 'Host Telemetry'}`"
      subtitle="Real-time telemetry, hardware saturation, and live process inspection"
      @close="showNodeDrawer = false"
      @update:show="showNodeDrawer = $event"
    >
      <div v-if="selectedNode" class="node-diagnostics-drawer">
        <!-- 1. Header Information Panel -->
        <div class="node-drawer-header glass-panel">
          <div class="header-top-row">
            <div class="node-primary-info">
              <div class="node-title-group">
                <span
                  class="node-status-dot-large"
                  :class="selectedNode.status === 'ready' || selectedNode.status === 'online' ? 'status-green' : 'status-red'"
                ></span>
                <h3 class="node-drawer-title">{{ selectedNode.node_name }}</h3>
                <span class="node-id-subtag font-mono">{{ selectedNode.node_id }}</span>
              </div>
              <div class="node-badge-pills">
                <span
                  class="badge"
                  :class="selectedNode.status === 'ready' || selectedNode.status === 'online' ? 'badge-emerald' : 'badge-rose'"
                >
                  {{ selectedNode.status === 'ready' || selectedNode.status === 'online' ? '● ONLINE' : '● OFFLINE' }}
                </span>
                <span class="badge badge-indigo">
                  📡 {{ selectedNode.role ? selectedNode.role.toUpperCase() : 'K8S-AGENT' }}
                </span>
                <span class="badge badge-cyan" v-if="selectedNode.source">
                  {{ selectedNode.source.toUpperCase() }}
                </span>
                <span class="badge badge-slate font-mono" :title="formatOsSummary(selectedNode)">
                  🐧 {{ formatOsSummary(selectedNode) }}
                </span>
              </div>
            </div>

            <div class="node-quick-stats">
              <div class="quick-stat-item">
                <span class="qs-label">OS DISTRO</span>
                <span class="qs-val" :title="selectedNode.os_distro || selectedNode.os || 'Linux'">{{ selectedNode.os_distro || selectedNode.os || 'Linux' }}</span>
              </div>
              <div class="quick-stat-item">
                <span class="qs-label">KERNEL VERSION</span>
                <span class="qs-val font-mono" :title="selectedNode.kernel_version || 'Linux'">{{ selectedNode.kernel_version || 'Linux' }}</span>
              </div>
              <div class="quick-stat-item">
                <span class="qs-label">ARCHITECTURE</span>
                <span class="qs-val font-mono">{{ selectedNode.arch || 'amd64' }}</span>
              </div>
              <div class="quick-stat-item">
                <span class="qs-label">UPTIME</span>
                <span class="qs-val font-mono">{{ formatUptime(selectedNode.uptime_seconds || selectedNode.uptime) }}</span>
              </div>
              <div class="quick-stat-item">
                <span class="qs-label">LOAD AVG</span>
                <span class="qs-val font-mono">{{ formatLoadAvg(selectedNode.load_avg || selectedNode.load_average) }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 2. Hardware Telemetry HUD -->
        <div class="hardware-hud-section">
          <div class="hud-section-header">
            <h4 class="hud-section-heading">
              <span class="heading-icon">📊</span> Hardware Saturation Telemetry
            </h4>
            <span class="badge badge-indigo">LIVE METRICS</span>
          </div>

          <div class="hardware-gauges-grid">
            <!-- CPU HUD -->
            <div class="hw-gauge-card glass-panel">
              <div class="hw-gauge-top">
                <span class="hw-gauge-label">CPU LOAD</span>
                <span class="hw-gauge-value" :class="`text-${getUtilizationColor(selectedNode.cpu_percent)}`">
                  {{ formatPercent(selectedNode.cpu_percent) }}
                </span>
              </div>
              <div class="hw-progress-track">
                <div
                  class="hw-progress-fill"
                  :class="`bg-${getUtilizationColor(selectedNode.cpu_percent)}`"
                  :style="{ width: `${Math.min(100, selectedNode.cpu_percent)}%` }"
                ></div>
              </div>
              <div class="hw-gauge-sub">
                <span>Saturation:</span>
                <strong :class="`text-${getUtilizationColor(selectedNode.cpu_percent)}`">
                  {{ selectedNode.cpu_percent >= 80 ? 'CRITICAL' : selectedNode.cpu_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
                </strong>
              </div>
            </div>

            <!-- RAM HUD -->
            <div class="hw-gauge-card glass-panel">
              <div class="hw-gauge-top">
                <span class="hw-gauge-label">MEMORY USAGE</span>
                <span class="hw-gauge-value text-cyan">
                  {{ formatPercent(selectedNode.memory_percent) }}
                </span>
              </div>
              <div class="hw-progress-track">
                <div
                  class="hw-progress-fill bg-cyan"
                  :style="{ width: `${Math.min(100, selectedNode.memory_percent)}%` }"
                ></div>
              </div>
              <div class="hw-gauge-sub">
                <span>{{ formatBytes(selectedNode.memory_used) }} / {{ formatBytes(selectedNode.memory_total) }}</span>
              </div>
            </div>

            <!-- DISK HUD -->
            <div class="hw-gauge-card glass-panel">
              <div class="hw-gauge-top">
                <span class="hw-gauge-label">DISK STORAGE</span>
                <span class="hw-gauge-value text-emerald">
                  {{ formatPercent(selectedNode.disk_percent) }}
                </span>
              </div>
              <div class="hw-progress-track">
                <div
                  class="hw-progress-fill bg-emerald"
                  :style="{ width: `${Math.min(100, selectedNode.disk_percent)}%` }"
                ></div>
              </div>
              <div class="hw-gauge-sub">
                <span>{{ formatBytes(selectedNode.disk_used) }} / {{ formatBytes(selectedNode.disk_total) }}</span>
              </div>
            </div>

            <!-- NETWORK I/O HUD -->
            <div class="hw-gauge-card glass-panel">
              <div class="hw-gauge-top">
                <span class="hw-gauge-label">NETWORK I/O</span>
                <span class="badge badge-violet font-mono">BANDWIDTH</span>
              </div>
              <div class="hw-net-rates font-mono">
                <div class="net-rate-item">
                  <span class="net-icon text-cyan">↓ RX:</span>
                  <span class="net-val">{{ formatIoRate(selectedNode.network_rx_bytes) }}</span>
                </div>
                <div class="net-rate-item">
                  <span class="net-icon text-violet">↑ TX:</span>
                  <span class="net-val">{{ formatIoRate(selectedNode.network_tx_bytes) }}</span>
                </div>
              </div>
              <div class="hw-gauge-sub">
                <span>Containers: {{ selectedNode.running_count || selectedNode.container_count || 0 }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 3. 📡 Network Interface Throughput Section -->
        <div class="node-net-interfaces-section glass-panel">
          <div class="hud-section-header">
            <div class="node-section-title-group">
              <h4 class="hud-section-heading">
                <span class="heading-icon">📡</span> Network Interface Throughput
              </h4>
              <span class="badge badge-cyan font-mono" v-if="filteredNodeInterfaces.length > 0">
                {{ filteredNodeInterfaces.length }} {{ filteredNodeInterfaces.length === 1 ? 'interface' : 'interfaces' }}
              </span>
            </div>
            <span class="badge badge-violet font-mono">BANDWIDTH</span>
          </div>

          <div class="interfaces-table-wrapper" v-if="filteredNodeInterfaces.length > 0">
            <table class="interfaces-table">
              <thead>
                <tr>
                  <th class="th-iface">Interface</th>
                  <th class="th-rx">↓ Rx</th>
                  <th class="th-tx">↑ Tx</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="iface in filteredNodeInterfaces" :key="iface.name" class="iface-row">
                  <td class="col-iface-name font-mono">
                    <span class="iface-tag font-mono">{{ iface.name }}</span>
                  </td>
                  <td class="col-iface-rx font-mono">
                    <span class="text-cyan">↓ {{ formatIoRate(iface.rx_bytes_per_sec) }}</span>
                  </td>
                  <td class="col-iface-tx font-mono">
                    <span class="text-violet">↑ {{ formatIoRate(iface.tx_bytes_per_sec) }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-else class="node-section-empty">
            <span class="text-muted text-xs">No active network interfaces detected for this node</span>
          </div>
        </div>

        <!-- 4. 📦 Apps & Services on this Node Section -->
        <div class="node-services-section glass-panel">
          <div class="hud-section-header">
            <div class="node-section-title-group">
              <h4 class="hud-section-heading">
                <span class="heading-icon">📦</span> Apps & Services on this Node
              </h4>
              <span class="badge badge-cyan font-mono" v-if="nodeServices.length > 0">
                {{ nodeServices.length }} {{ nodeServices.length === 1 ? 'service' : 'services' }}
              </span>
            </div>
            <span class="badge badge-indigo font-mono">THROUGHPUT</span>
          </div>

          <div class="services-table-wrapper" v-if="nodeServices.length > 0">
            <table class="node-services-table">
              <thead>
                <tr>
                  <th class="th-svc-name">Service</th>
                  <th class="th-svc-cpu">CPU</th>
                  <th class="th-svc-mem">Memory</th>
                  <th class="th-svc-rx">↓ Rx</th>
                  <th class="th-svc-tx">↑ Tx</th>
                  <th class="th-svc-status">Status</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="svc in nodeServices" :key="svc.service_name + (svc.node_id || '')" class="node-svc-row">
                  <td class="col-svc-name">
                    <div class="svc-name-cell">
                      <span
                        class="node-indicator-dot"
                        :class="svc.status === 'healthy' ? 'bg-emerald' : svc.status === 'degraded' ? 'bg-amber' : 'bg-rose'"
                      ></span>
                      <span class="svc-title font-mono">{{ svc.service_name }}</span>
                    </div>
                  </td>
                  <td class="col-svc-cpu font-mono">
                    <span :class="svc.cpu_percent > 80 ? 'text-rose' : svc.cpu_percent > 50 ? 'text-amber' : 'text-emerald'">
                      {{ svc.cpu_percent.toFixed(1) }}%
                    </span>
                  </td>
                  <td class="col-svc-mem font-mono">
                    <span :class="svc.memory_percent > 85 ? 'text-rose' : svc.memory_percent > 70 ? 'text-amber' : 'text-cyan'">
                      {{ svc.memory_used_mb >= 1024 ? (svc.memory_used_mb / 1024).toFixed(1) + ' GB' : svc.memory_used_mb.toFixed(0) + ' MB' }}
                    </span>
                  </td>
                  <td class="col-svc-rx font-mono">
                    <span class="text-cyan">↓ {{ formatIoRate(svc.rx_bytes_per_sec) }}</span>
                  </td>
                  <td class="col-svc-tx font-mono">
                    <span class="text-violet">↑ {{ formatIoRate(svc.tx_bytes_per_sec) }}</span>
                  </td>
                  <td class="col-svc-status">
                    <span
                      class="badge"
                      :class="svc.status === 'healthy' ? 'badge-emerald' : svc.status === 'degraded' ? 'badge-amber' : 'badge-rose'"
                    >
                      {{ svc.status === 'healthy' ? '● HEALTHY' : svc.status === 'degraded' ? '● DEGRADED' : '● DOWN' }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-else class="node-section-empty">
            <span class="text-muted text-xs">No containerized services detected on this node</span>
          </div>
        </div>

        <!-- 5. 🔥 Top Resource-Consuming Apps & Processes Table -->
        <div class="processes-section glass-panel">
          <div class="processes-header">
            <div class="proc-title-group">
              <div class="proc-title-row">
                <h4 class="proc-section-heading">🔥 Top Resource-Consuming Apps & Processes</h4>
                <span class="badge badge-indigo font-mono" v-if="filteredNodeProcesses.length > 0">
                  {{ filteredNodeProcesses.length }} active
                </span>
              </div>
              <p class="proc-section-desc">
                Real-time operating system PID telemetry mapped to workload containers and system daemons.
              </p>
            </div>

            <div class="proc-controls">
              <div class="proc-search-box">
                <span class="search-icon">🔍</span>
                <input
                  v-model="processSearch"
                  type="text"
                  placeholder="Filter by name, PID, user, cmd..."
                  class="input-proc-search"
                />
                <button v-if="processSearch" class="btn-clear-search" @click="processSearch = ''">✕</button>
              </div>

              <div class="proc-sort-group">
                <select v-model="processSortBy" class="select-proc-sort">
                  <option value="cpu">Sort: CPU % (High to Low)</option>
                  <option value="mem">Sort: Memory</option>
                  <option value="disk">Sort: Disk I/O Rate</option>
                  <option value="name">Sort: Process Name</option>
                  <option value="pid">Sort: PID</option>
                </select>
              </div>
            </div>
          </div>

          <!-- Processes Table -->
          <div class="processes-table-wrapper" v-if="filteredNodeProcesses.length > 0">
            <table class="processes-table">
              <thead>
                <tr>
                  <th class="th-pid">PID</th>
                  <th class="th-app">App / Command</th>
                  <th class="th-user">User</th>
                  <th class="th-cpu">CPU %</th>
                  <th class="th-mem">Memory</th>
                  <th class="th-disk">Disk I/O</th>
                  <th class="th-state">State</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="proc in filteredNodeProcesses"
                  :key="proc.pid"
                  class="proc-row"
                  :class="{ 'proc-row-hot': proc.cpu_percent >= 70 }"
                >
                  <!-- PID -->
                  <td class="col-proc-pid">
                    <span class="pid-tag font-mono">#{{ proc.pid }}</span>
                  </td>

                  <!-- App / Command -->
                  <td class="col-proc-app">
                    <div class="proc-app-info" :title="proc.command_line ? `${proc.name}\nCommand: ${proc.command_line}` : proc.name">
                      <span class="proc-name">{{ proc.name }}</span>
                      <span class="proc-cmd-line font-mono" v-if="proc.command_line">
                        {{ proc.command_line }}
                      </span>
                    </div>
                  </td>

                  <!-- User -->
                  <td class="col-proc-user">
                    <span class="user-badge font-mono">{{ proc.user || 'root' }}</span>
                  </td>

                  <!-- CPU % -->
                  <td class="col-proc-cpu">
                    <div class="proc-cpu-box">
                      <div class="proc-cpu-val-row">
                        <span class="proc-cpu-val" :class="`text-${getProcessCpuColor(proc.cpu_percent)}`">
                          {{ formatPercent(proc.cpu_percent) }}
                        </span>
                        <span v-if="proc.cpu_percent >= 70" class="badge badge-rose badge-hot-pulse">
                          HOT
                        </span>
                      </div>
                      <div class="proc-mini-track">
                        <div
                          class="proc-mini-fill"
                          :class="`bg-${getProcessCpuColor(proc.cpu_percent)}`"
                          :style="{ width: `${Math.min(100, proc.cpu_percent)}%` }"
                        ></div>
                      </div>
                    </div>
                  </td>

                  <!-- Memory -->
                  <td class="col-proc-mem">
                    <div class="proc-mem-box">
                      <span class="proc-mem-val">{{ formatBytes(proc.memory_bytes) }}</span>
                      <span class="proc-mem-pct font-mono" v-if="proc.memory_percent">
                        ({{ formatPercent(proc.memory_percent) }})
                      </span>
                    </div>
                  </td>

                  <!-- Disk I/O -->
                  <td class="col-proc-disk font-mono">
                    <div class="proc-io-rates">
                      <span class="io-read">R: {{ formatIoRate(proc.read_bytes_per_sec) }}</span>
                      <span class="io-sep">·</span>
                      <span class="io-write">W: {{ formatIoRate(proc.write_bytes_per_sec) }}</span>
                    </div>
                  </td>

                  <!-- State -->
                  <td class="col-proc-state">
                    <span class="badge" :class="getProcessStateBadgeClass(proc.state)">
                      {{ getProcessStateLabel(proc.state) }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Clean Cybernetic Empty State -->
          <div v-else class="proc-empty-state">
            <div class="proc-empty-icon">📡</div>
            <h5 class="proc-empty-title">
              {{ processSearch ? 'No Processes Matching Filter' : 'No Process Telemetry Streamed' }}
            </h5>
            <p class="proc-empty-desc" v-if="processSearch">
              No active processes matched "{{ processSearch }}". Try searching for a different process name or PID.
            </p>
            <p class="proc-empty-desc" v-else>
              Node telemetry is online but process inspector stream is pending or waiting for agent collector broadcast. Ensure <code>k8s-agent</code> v2.4+ is installed and running with process collector enabled (port 9100).
            </p>
            <div class="proc-empty-actions" v-if="processSearch">
              <button class="btn btn-secondary btn-sm" @click="processSearch = ''">
                Clear Search Filter
              </button>
            </div>
          </div>
        </div>

        <!-- 4. Drawer Footer Actions -->
        <div class="node-drawer-footer">
          <button
            class="btn btn-secondary"
            @click="router.push({ path: '/hosts', query: { search: selectedNode.node_name } })"
          >
            <span>⚙️ Manage in Registry</span>
          </button>
          <button
            class="btn btn-primary"
            @click="router.push('/incidents')"
          >
            <span>🤖 AI Incident Diagnostics</span>
          </button>
        </div>
      </div>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.overview-dashboard {
  display: flex;
  flex-direction: column;
  gap: 24px;
  width: 100%;
  padding-bottom: 40px;
}

/* Header */
.dashboard-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.header-badge-group {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.pulse-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
  display: inline-block;
}

.pulse-active {
  animation: pulse-ring 1.8s infinite;
}

@keyframes pulse-ring {
  0% { transform: scale(0.95); opacity: 0.8; box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7); }
  70% { transform: scale(1.1); opacity: 1; box-shadow: 0 0 0 6px rgba(16, 185, 129, 0); }
  100% { transform: scale(0.95); opacity: 0.8; box-shadow: 0 0 0 0 rgba(16, 185, 129, 0); }
}

.last-sync-text {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.page-title {
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--text-primary);
}

.page-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 800px;
  margin-top: 4px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.spin-icon {
  display: inline-block;
  animation: spin 1s infinite linear;
}

@keyframes spin {
  100% { transform: rotate(360deg); }
}

/* SECTION 4: Alerts */
.alerts-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.alert-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 18px;
  border-radius: 12px;
  gap: 14px;
  transition: all 0.3s ease;
}

.alert-critical {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.35);
  box-shadow: 0 0 20px rgba(244, 63, 94, 0.15);
}

.alert-warning {
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.35);
  box-shadow: 0 0 20px rgba(245, 158, 11, 0.15);
}

.alert-icon-box {
  font-size: 20px;
  display: flex;
  align-items: center;
}

.alert-content {
  flex: 1;
}

.alert-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 2px;
}

.alert-node-tag {
  font-weight: 700;
  font-size: 13px;
  color: var(--text-primary);
}

.alert-type-tag {
  font-size: 10px;
  font-weight: 800;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.1);
  font-family: var(--font-mono);
}

.alert-value-tag {
  font-size: 11px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

.alert-message {
  font-size: 12px;
  color: var(--text-secondary);
}

.alert-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.btn-alert-action {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.16);
  color: var(--text-primary);
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.btn-alert-action:hover {
  background: rgba(255, 255, 255, 0.18);
  border-color: var(--border-accent);
  color: #ffffff;
  transform: translateY(-1px);
}

.btn-alert-dismiss {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 14px;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  transition: color 0.2s ease;
}

.btn-alert-dismiss:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.1);
}

/* SECTION 1: Summary HUD */
.summary-hud-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 16px;
}

.hud-card {
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  border-radius: 14px;
}

.hud-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hud-label {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
}

.status-indicator-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-green { background: #10b981; box-shadow: 0 0 8px rgba(16, 185, 129, 0.6); }
.status-amber { background: #f59e0b; box-shadow: 0 0 8px rgba(245, 158, 11, 0.6); }
.status-red { background: #f43f5e; box-shadow: 0 0 8px rgba(244, 63, 94, 0.6); }

.hud-value-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.hud-value {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--text-primary);
  font-family: var(--font-sans);
}

.hud-total {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-muted);
}

.hud-badge-tag {
  font-size: 11px;
  font-weight: 700;
  font-family: var(--font-mono);
}

.hud-progress-track {
  width: 100%;
  height: 4px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.hud-progress-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.5s ease;
}

.hud-card-throughput {
  grid-column: span 1;
}

.throughput-details {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.hud-unit {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 600;
}

.sparkline-container {
  width: 100px;
  height: 28px;
}

.sparkline-svg {
  width: 100%;
  height: 100%;
}

.hud-card-footer-text {
  font-size: 10px;
  color: var(--text-muted);
}

/* SECTION 2: Topology Visualizer */
.topology-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-title-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.section-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
}

.section-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  display: block;
}

.topology-legend {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 11px;
  color: var(--text-secondary);
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.bg-emerald { background: #10b981; }
.bg-amber { background: #f59e0b; }
.bg-rose { background: #f43f5e; }
.bg-cyan { background: #06b6d4; }
.bg-gray { background: #64748b; }

.mesh-interconnect-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 16px;
  background: rgba(11, 15, 25, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
}

.mesh-pill {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-cyan);
  white-space: nowrap;
}

.mesh-nodes-count {
  color: var(--text-muted);
}

.mesh-line-stream {
  flex: 1;
  height: 2px;
  background: rgba(255, 255, 255, 0.05);
  position: relative;
  overflow: hidden;
  border-radius: 2px;
}

.mesh-pulse-line {
  position: absolute;
  top: 0;
  left: -20%;
  width: 30%;
  height: 100%;
  background: linear-gradient(90deg, transparent, var(--accent-cyan), transparent);
  animation: mesh-stream 3s infinite linear;
}

@keyframes mesh-stream {
  0% { left: -30%; }
  100% { left: 100%; }
}

/* Topology Filter Pills Bar */
.topology-filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.filter-pills-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-pill-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.filter-pill-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
  border-color: var(--border-accent);
  transform: translateY(-1px);
}

.filter-pill-active {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
  color: var(--accent-cyan);
  box-shadow: 0 0 12px rgba(6, 182, 212, 0.2);
}

.filter-pill-active:hover {
  background: rgba(6, 182, 212, 0.22);
  color: var(--accent-cyan);
}

.filter-pill-alert {
  border-color: rgba(244, 63, 94, 0.4);
  background: rgba(244, 63, 94, 0.08);
  color: #fb7185;
}

.filter-pill-alert.filter-pill-active {
  background: rgba(244, 63, 94, 0.2);
  border-color: #f43f5e;
  color: #fb7185;
  box-shadow: 0 0 14px rgba(244, 63, 94, 0.3);
}

.filter-pill-count {
  font-family: var(--font-mono);
  font-size: 11px;
  opacity: 0.85;
}

.filter-meta-text {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.btn-reset-order {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  font-size: 11px;
  font-family: inherit;
  color: var(--accent-cyan, #38bdf8);
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.25);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-reset-order:hover {
  background: rgba(56, 189, 248, 0.2);
  border-color: rgba(56, 189, 248, 0.5);
  color: #fff;
}

.empty-topology-state {
  padding: 36px 20px;
  text-align: center;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  grid-column: 1 / -1;
  color: var(--text-muted);
}

.empty-topology-icon {
  font-size: 28px;
}

.empty-topology-text {
  font-size: 13px;
  color: var(--text-secondary);
}

/* Node Cards Grid */
.node-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.drag-handle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted, #94a3b8);
  font-size: 13px;
  letter-spacing: -2px;
  cursor: grab;
  padding: 2px 4px;
  border-radius: 4px;
  transition: color 0.15s ease, background-color 0.15s ease, opacity 0.15s ease;
  user-select: none;
  opacity: 0.5;
}

.node-card:hover .drag-handle {
  opacity: 0.95;
  color: var(--text-primary, #f8fafc);
  background: rgba(255, 255, 255, 0.06);
}

.drag-handle:hover {
  opacity: 1;
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
}

.drag-handle:active {
  cursor: grabbing;
}

.node-card {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  position: relative;
  transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              box-shadow 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              border-color 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              opacity 0.2s ease;
  cursor: grab;
}

.node-card:hover {
  transform: translateY(-2px);
  border-color: rgba(56, 189, 248, 0.3);
}

.node-card:active {
  cursor: grabbing;
}

.node-card.is-dragging {
  opacity: 0.35;
  transform: scale(0.97);
  border: 2px dashed rgba(56, 189, 248, 0.6) !important;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  cursor: grabbing;
}

.node-card.is-drag-over {
  border-color: #38bdf8 !important;
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.5), 0 8px 25px rgba(56, 189, 248, 0.25) !important;
  transform: translateY(-4px) scale(1.02);
  background: rgba(56, 189, 248, 0.08);
}

.node-overloaded {
  border-color: rgba(244, 63, 94, 0.4);
  box-shadow: 0 4px 20px rgba(244, 63, 94, 0.12);
}

.node-warning {
  border-color: rgba(245, 158, 11, 0.3);
}

.node-down {
  opacity: 0.6;
  filter: grayscale(0.7);
}

.is-busiest {
  box-shadow: 0 0 25px rgba(245, 158, 11, 0.18);
  border-color: rgba(245, 158, 11, 0.4);
}

.badge-traffic-pulse {
  animation: pulse-traffic 2s infinite ease-in-out;
}

@keyframes pulse-traffic {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.05); opacity: 0.85; }
}

.node-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.node-identity {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.node-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.node-name {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-badge-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.node-gauges-cluster {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 10px 0;
  border-top: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
  background: rgba(0, 0, 0, 0.15);
  border-radius: 8px;
}

.gauge-col {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.node-meta-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 12px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.meta-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.meta-val {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
}

.node-card-footer {
  margin-top: auto;
}

.node-footer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.btn-node-action {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 6px 10px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.btn-node-action:hover {
  background: rgba(255, 255, 255, 0.09);
  color: var(--text-primary);
  border-color: var(--border-accent);
  transform: translateY(-1px);
}

.btn-manage-host {
  width: 100%;
}

.btn-manage-host:hover {
  border-color: #818cf8;
  color: #a5b4fc;
  background: rgba(99, 102, 241, 0.12);
}

.btn-node-filter {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-node-filter:hover {
  background: rgba(56, 189, 248, 0.1);
  color: var(--text-primary);
  border-color: var(--border-accent);
}

.btn-node-filter-active {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
  color: var(--accent-cyan);
}

.count-pill {
  padding: 1px 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.1);
  font-family: var(--font-mono);
  font-size: 10px;
}

/* SECTION 3 & 5: Split Layout */
.dashboard-split-layout {
  display: grid;
  grid-template-columns: 1fr 340px;
  gap: 20px;
  align-items: start;
}

@media (max-width: 1100px) {
  .dashboard-split-layout {
    grid-template-columns: 1fr;
  }
}

.container-grid-section {
  padding: 20px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.split-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 14px;
}

.container-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.search-box {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 10px;
  font-size: 12px;
  color: var(--text-muted);
}

.input-search {
  padding: 7px 28px 7px 30px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid var(--border-subtle);
  color: var(--text-primary);
  font-size: 12px;
  width: 180px;
  outline: none;
  transition: all 0.2s ease;
}

.input-search:focus {
  border-color: var(--border-accent);
  width: 220px;
}

.btn-clear-search {
  position: absolute;
  right: 8px;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
}

.select-filter {
  padding: 7px 10px;
  border-radius: 8px;
  background: rgba(11, 15, 25, 0.9);
  border: 1px solid var(--border-subtle);
  color: var(--text-primary);
  font-size: 12px;
  outline: none;
  cursor: pointer;
}

.containers-table-wrapper {
  overflow-x: auto;
}

.containers-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}

.containers-table th {
  padding: 10px 12px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border-subtle);
}

.containers-table td {
  padding: 10px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  vertical-align: middle;
}

.container-row {
  cursor: pointer;
  transition: background 0.15s ease;
}

.container-row:hover {
  background: rgba(255, 255, 255, 0.03);
}

.col-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.container-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.container-primary-name {
  font-weight: 600;
  color: var(--text-primary);
}

.slot-badge {
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  font-weight: 500;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(56, 189, 248, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.25);
  line-height: 1.3;
  letter-spacing: 0.02em;
  white-space: nowrap;
}

.drawer-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.container-id-sub {
  font-size: 10px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.col-image {
  max-width: 160px;
}

.image-text {
  display: block;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 11px;
}

.usage-metric-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 90px;
}

.usage-text {
  font-size: 11px;
  font-weight: 600;
  font-family: var(--font-mono);
}

.mini-bar-track {
  width: 100%;
  height: 4px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.mini-bar-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.3s ease;
}

.btn-inspect {
  padding: 4px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-inspect:hover {
  background: var(--accent-cyan);
  color: var(--text-inverse);
  border-color: var(--accent-cyan);
}

.empty-filter-state {
  padding: 30px;
  text-align: center;
  color: var(--text-muted);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.btn-reset-filters {
  background: none;
  border: 1px solid var(--border-subtle);
  color: var(--accent-cyan);
  padding: 4px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 11px;
}

/* SECTION 5: Sidebar Trends & Live Ingress */
.trends-activity-sidebar {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.sidebar-card-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-primary);
}

.trend-chart-card {
  padding: 16px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.trend-chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.trend-legend {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 10px;
  font-weight: 700;
}

.legend-line {
  display: flex;
  align-items: center;
  gap: 4px;
}

.cpu-legend::before {
  content: '';
  width: 10px;
  height: 2px;
  background: #8b5cf6;
  display: inline-block;
}

.mem-legend::before {
  content: '';
  width: 10px;
  height: 2px;
  background: #06b6d4;
  display: inline-block;
}

.trend-svg-box {
  width: 100%;
  height: 80px;
  background: rgba(0, 0, 0, 0.2);
  border-radius: 8px;
  overflow: hidden;
}

.trend-svg {
  width: 100%;
  height: 100%;
}

.trend-footer-stats {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}

.stat-pair {
  display: flex;
  flex-direction: column;
}

.stat-k {
  font-size: 9px;
  text-transform: uppercase;
  color: var(--text-muted);
}

.stat-v {
  font-size: 12px;
  font-weight: 700;
  font-family: var(--font-mono);
}

.text-violet { color: #8b5cf6; }
.text-cyan { color: #06b6d4; }
.text-emerald { color: #10b981; }
.text-amber { color: #f59e0b; }
.text-rose { color: #f43f5e; }

.activity-feed-card {
  padding: 16px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.activity-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.activity-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.live-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #10b981;
}

.activity-items-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 280px;
  overflow-y: auto;
}

.activity-log-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 8px;
  background: rgba(0, 0, 0, 0.2);
  border-radius: 6px;
  font-size: 11px;
}

.activity-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.activity-method {
  font-size: 9px;
  font-weight: 800;
  padding: 1px 4px;
  border-radius: 3px;
  font-family: var(--font-mono);
}

.method-get { background: rgba(56, 189, 248, 0.2); color: #38bdf8; }
.method-post { background: rgba(16, 185, 129, 0.2); color: #10b981; }
.method-delete { background: rgba(244, 63, 94, 0.2); color: #f43f5e; }

.activity-path {
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 120px;
  font-family: var(--font-mono);
}

.activity-timing {
  display: flex;
  align-items: center;
  gap: 6px;
  font-family: var(--font-mono);
  font-size: 10px;
}

.activity-status {
  font-weight: 700;
}

.status-ok { color: #10b981; }
.status-err { color: #f43f5e; }
.activity-dur { color: var(--text-muted); }
.activity-time { color: var(--text-muted); font-size: 9px; }

/* Skeleton & Empty States */
.skeleton-hud-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.skeleton-card {
  height: 110px;
  border-radius: 14px;
}

.shimmer {
  background: linear-gradient(90deg, rgba(255,255,255,0.03) 25%, rgba(255,255,255,0.08) 50%, rgba(255,255,255,0.03) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.6s infinite;
}

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

.empty-state-card {
  padding: 60px 20px;
  text-align: center;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.empty-icon {
  font-size: 48px;
}

.empty-actions {
  display: flex;
  gap: 12px;
  margin-top: 8px;
}

.error-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px;
  border-radius: 14px;
  border: 1px solid rgba(244, 63, 94, 0.4);
  background: rgba(244, 63, 94, 0.08);
}

.error-icon { font-size: 32px; }
.error-info h3 { font-size: 15px; color: #f43f5e; }
.error-info p { font-size: 12px; color: var(--text-secondary); }

/* Drawer Details */
.drawer-container-details {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px 0;
}

.drawer-header-card {
  padding: 16px;
  border-radius: 12px;
}

.drawer-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.drawer-container-title {
  font-size: 16px;
  font-weight: 700;
}

.drawer-sub-image {
  font-size: 12px;
  color: var(--text-muted);
  font-family: var(--font-mono);
  word-break: break-all;
}

.drawer-metrics-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.drawer-metric-box {
  padding: 14px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.box-label { font-size: 10px; text-transform: uppercase; color: var(--text-muted); }
.box-value { font-size: 20px; font-weight: 800; font-family: var(--font-mono); }
.box-sub { font-size: 10px; color: var(--text-muted); }

.drawer-meta-list {
  padding: 14px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.drawer-meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}

.dm-label { color: var(--text-muted); }
.dm-val { color: var(--text-primary); font-weight: 600; }

.drawer-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.w-full { width: 100%; }

/* Buttons & Badges */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.btn-primary {
  background: var(--grad-cyan);
  color: var(--text-inverse);
  box-shadow: 0 2px 10px rgba(6, 182, 212, 0.3);
}

.btn-primary:hover {
  filter: brightness(1.1);
  box-shadow: 0 4px 14px rgba(6, 182, 212, 0.4);
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.05);
  border-color: var(--border-subtle);
  color: var(--text-primary);
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: var(--border-accent);
}

.badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 700;
  font-family: var(--font-mono);
  letter-spacing: 0.02em;
}

.badge-emerald { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.badge-rose { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }
.badge-cyan { background: rgba(6, 182, 212, 0.15); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.3); }
.badge-indigo { background: rgba(99, 102, 241, 0.15); color: #818cf8; border: 1px solid rgba(99, 102, 241, 0.3); }
.badge-violet { background: rgba(139, 92, 246, 0.15); color: #a78bfa; border: 1px solid rgba(139, 92, 246, 0.3); }

/* Transitions */
.alert-slide-enter-active, .alert-slide-leave-active {
  transition: all 0.3s ease;
}
.alert-slide-enter-from, .alert-slide-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

/* ==========================================
   NODE DIAGNOSTICS & PROCESS INSPECTOR DRAWER
   ========================================== */
.cursor-pointer {
  cursor: pointer;
}

.btn-inspect-node {
  background: rgba(6, 182, 212, 0.08);
  border-color: rgba(6, 182, 212, 0.25);
  color: var(--accent-cyan);
}

.btn-inspect-node:hover {
  background: rgba(6, 182, 212, 0.18);
  border-color: var(--accent-cyan);
  color: #38bdf8;
  box-shadow: 0 0 10px rgba(6, 182, 212, 0.2);
}

.node-diagnostics-drawer {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 4px 0 16px;
}

/* Header Summary Card */
.node-drawer-header {
  padding: 18px 20px;
  border-radius: 14px;
}

.header-top-row {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

@media (min-width: 640px) {
  .header-top-row {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
  }
}

.node-primary-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.node-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.node-status-dot-large {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.node-drawer-title {
  font-size: 20px;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--text-primary);
  margin: 0;
}

.node-id-subtag {
  font-size: 11px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.06);
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid var(--border-subtle);
}

.node-badge-pills {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.node-quick-stats {
  display: flex;
  align-items: center;
  gap: 16px;
  background: rgba(0, 0, 0, 0.25);
  padding: 10px 16px;
  border-radius: 10px;
  border: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.quick-stat-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.qs-label {
  font-size: 9px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.qs-val {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
}

/* Hardware Telemetry HUD */
.hardware-hud-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.hud-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hud-section-heading {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  gap: 6px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin: 0;
}

.heading-icon {
  font-size: 14px;
}

.hardware-gauges-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px;
}

.hw-gauge-card {
  padding: 14px 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.hw-gauge-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hw-gauge-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.hw-gauge-value {
  font-size: 18px;
  font-weight: 800;
  font-family: var(--font-mono);
}

.hw-progress-track {
  width: 100%;
  height: 5px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.hw-progress-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.4s ease;
}

.hw-gauge-sub {
  font-size: 10px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.hw-net-rates {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 2px 0;
}

.net-rate-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
}

.net-icon {
  font-weight: 700;
  font-size: 10px;
}

.net-val {
  color: var(--text-primary);
  font-weight: 600;
}

/* Network Interface & Services Sections in Node Drawer */
.node-net-interfaces-section,
.node-services-section {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.node-section-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.interfaces-table-wrapper,
.services-table-wrapper {
  overflow-x: auto;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.2);
}

.interfaces-table,
.node-services-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}

.interfaces-table thead tr,
.node-services-table thead tr {
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid var(--border-subtle);
}

.interfaces-table th,
.node-services-table th {
  padding: 10px 12px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  white-space: nowrap;
}

.interfaces-table tbody tr,
.node-services-table tbody tr {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: background 0.15s ease;
}

.interfaces-table tbody tr:last-child,
.node-services-table tbody tr:last-child {
  border-bottom: none;
}

.interfaces-table tbody tr:hover,
.node-services-table tbody tr:hover {
  background: rgba(255, 255, 255, 0.04);
}

.interfaces-table td,
.node-services-table td {
  padding: 10px 12px;
  vertical-align: middle;
}

.col-iface-name {
  width: 40%;
}

.col-iface-rx,
.col-iface-tx {
  width: 30%;
}

.iface-tag {
  font-size: 11px;
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid var(--border-subtle);
}

.col-svc-name {
  min-width: 160px;
}

.svc-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.svc-title {
  font-weight: 700;
  color: var(--text-primary);
  font-size: 12px;
}

.col-svc-cpu,
.col-svc-mem,
.col-svc-rx,
.col-svc-tx {
  white-space: nowrap;
}

.col-svc-status {
  white-space: nowrap;
  width: 110px;
}

.node-section-empty {
  padding: 18px;
  text-align: center;
  background: rgba(0, 0, 0, 0.15);
  border-radius: 8px;
  border: 1px dashed var(--border-subtle);
}

/* Processes Section */
.processes-section {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.processes-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.proc-title-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.proc-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.proc-section-heading {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.proc-section-desc {
  font-size: 12px;
  color: var(--text-muted);
  margin: 0;
}

.proc-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.proc-search-box {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 220px;
}

.input-proc-search {
  width: 100%;
  padding: 8px 32px 8px 34px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 12px;
  transition: all 0.2s ease;
}

.input-proc-search:focus {
  outline: none;
  border-color: var(--accent-cyan);
  background: rgba(0, 0, 0, 0.5);
  box-shadow: 0 0 12px rgba(6, 182, 212, 0.25);
}

.proc-sort-group {
  display: flex;
  align-items: center;
}

.select-proc-sort {
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  outline: none;
  transition: all 0.2s ease;
}

.select-proc-sort:focus,
.select-proc-sort:hover {
  border-color: var(--accent-cyan);
  color: var(--text-primary);
}

/* Processes Table */
.processes-table-wrapper {
  overflow-x: auto;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.2);
}

.processes-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}

.processes-table thead tr {
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid var(--border-subtle);
}

.processes-table th {
  padding: 10px 12px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  white-space: nowrap;
}

.processes-table tbody tr {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: background 0.15s ease;
}

.processes-table tbody tr:last-child {
  border-bottom: none;
}

.processes-table tbody tr:hover {
  background: rgba(255, 255, 255, 0.04);
}

.proc-row-hot {
  background: rgba(244, 63, 94, 0.04);
}

.proc-row-hot:hover {
  background: rgba(244, 63, 94, 0.08) !important;
}

.processes-table td {
  padding: 10px 12px;
  vertical-align: middle;
}

.col-proc-pid {
  white-space: nowrap;
  width: 70px;
}

.pid-tag {
  font-size: 11px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
  border: 1px solid var(--border-subtle);
}

.col-proc-app {
  max-width: 220px;
  min-width: 140px;
}

.proc-app-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  cursor: help;
}

.proc-name {
  font-weight: 700;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.proc-cmd-line {
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.col-proc-user {
  white-space: nowrap;
  width: 80px;
}

.user-badge {
  font-size: 10px;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
}

.col-proc-cpu {
  min-width: 110px;
}

.proc-cpu-box {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.proc-cpu-val-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.proc-cpu-val {
  font-weight: 700;
  font-family: var(--font-mono);
  font-size: 11px;
}

.badge-hot-pulse {
  font-size: 8px;
  padding: 1px 4px;
  animation: pulse-hot 1.2s infinite ease-in-out;
}

@keyframes pulse-hot {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.08); opacity: 0.8; }
}

.proc-mini-track {
  width: 100%;
  height: 3px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.proc-mini-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.3s ease;
}

.col-proc-mem {
  white-space: nowrap;
  min-width: 100px;
}

.proc-mem-box {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.proc-mem-val {
  font-weight: 600;
  color: var(--text-primary);
  font-family: var(--font-mono);
}

.proc-mem-pct {
  font-size: 10px;
  color: var(--text-muted);
}

.col-proc-disk {
  white-space: nowrap;
  font-size: 11px;
  min-width: 140px;
}

.proc-io-rates {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--text-secondary);
}

.io-read { color: #38bdf8; }
.io-sep { color: var(--text-muted); }
.io-write { color: #a78bfa; }

.col-proc-state {
  white-space: nowrap;
  width: 90px;
}

/* Empty State */
.proc-empty-state {
  padding: 36px 20px;
  text-align: center;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.2);
  border: 1px dashed var(--border-medium);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.proc-empty-icon {
  font-size: 32px;
  filter: drop-shadow(0 0 8px rgba(6, 182, 212, 0.4));
}

.proc-empty-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.proc-empty-desc {
  font-size: 12px;
  color: var(--text-muted);
  max-width: 460px;
  margin: 0;
  line-height: 1.5;
}

.proc-empty-desc code {
  font-family: var(--font-mono);
  background: rgba(255, 255, 255, 0.08);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--accent-cyan);
}

.proc-empty-actions {
  margin-top: 4px;
}

.btn-sm {
  padding: 5px 12px;
  font-size: 11px;
}

/* Drawer Footer */
.node-drawer-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--border-subtle);
}

/* ==========================================
   REAL-TIME THROUGHPUT MONITOR (TPS) STYLES
   ========================================== */
.tps-monitor-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
}

.tps-header-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.tps-sync-time {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.tps-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}

.tps-card {
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  border-radius: 14px;
  transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              border-color 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              box-shadow 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.tps-card:hover {
  transform: translateY(-2px);
  border-color: var(--border-medium);
  box-shadow: 0 8px 24px -6px rgba(0, 0, 0, 0.4);
}

.tps-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.tps-card-identity {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tps-icon {
  font-size: 16px;
}

.tps-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.tps-highlight-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.tps-big-stat {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.tps-big-num {
  font-size: 22px;
  font-weight: 800;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

.tps-big-unit {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 600;
}

.tps-badge-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tps-sub-badge {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.04);
  padding: 2px 8px;
  border-radius: 6px;
  border: 1px solid var(--border-subtle);
}

.tps-metrics-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 4px;
  border-top: 1px solid var(--border-subtle);
}

.tps-metric-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  gap: 8px;
}

.tps-metric-key {
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 500;
}

.tps-metric-val {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 12px;
}

.tps-indicator-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 2px;
}

.tps-ratio-bar {
  display: flex;
  width: 100%;
  height: 6px;
  border-radius: 999px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.06);
}

.tps-bar-rx {
  background: linear-gradient(90deg, #06b6d4, #38bdf8);
  transition: width 0.4s ease;
  min-width: 4px;
}

.tps-bar-tx {
  background: linear-gradient(90deg, #8b5cf6, #a855f7);
  transition: width 0.4s ease;
  min-width: 4px;
}

.tps-ratio-labels {
  display: flex;
  justify-content: space-between;
  font-size: 10px;
  color: var(--text-muted);
}

.node-indicator-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
</style>

