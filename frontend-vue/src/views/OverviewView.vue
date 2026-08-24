<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  overviewApi,
  tpsApi,
  type SystemOverview,
  type NodeMetrics,
  type NetworkInterface,
  type MetricAlert,
  type ProcessMetric,
  type TpsSnapshot,
  type TpsServiceMetrics,
} from '../api/overview'
import {
  nodeHistoryApi,
  type NodeHistoryResponse,
  type NodeMetricRollup,
} from '../api/compute'
import { useWebSocket } from '../composables/useWebSocket'
import CircularGauge from '../components/ui/CircularGauge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'

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

// Interactive Hover Tooltips on 5-Min Saturation Trends Chart
const hoveredTrendIndex = ref<number | null>(null)
const trendTooltipPos = ref<{ x: number; y: number }>({ x: 0, y: 0 })
const isTrendHovered = ref(false)

// Cluster Telemetry & Throughput Deep-Dive Modal State
const showDeepDiveModal = ref(false)
const showModalCpu = ref(true)
const showModalMem = ref(true)
const showModalReqs = ref(true)
const hoveredModalIndex = ref<number | null>(null)
const modalTooltipPos = ref<{ x: number; y: number }>({ x: 0, y: 0 })
const isModalHovered = ref(false)

// Deep-Dive Modal Services Table State
const modalServiceSearch = ref('')
const modalServiceSortBy = ref<'traffic' | 'rps' | 'bandwidth' | 'cpu' | 'mem' | 'name' | 'node' | 'status'>('traffic')
const modalServiceSortOrder = ref<'asc' | 'desc'>('desc')

// Node diagnostics inspector drawer & Historical telemetry
const selectedNodeId = ref<string | null>(null)
const showNodeDrawer = ref(false)
const nodeDrawerMode = ref<'live' | 'history'>('live')
const nodeHistoryRange = ref<'1h' | '24h' | '7d' | '30d'>('24h')
const nodeHistoryLoading = ref(false)
const nodeHistoryData = ref<NodeHistoryResponse | null>(null)
const hoveredNodeHistIndex = ref<number | null>(null)
const nodeHistTooltipPos = ref<{ x: number; y: number }>({ x: 0, y: 0 })
const isNodeHistHovered = ref(false)
const processSearch = ref('')
const processCategoryFilter = ref<'all' | 'workloads' | 'system'>('all')
const processSortBy = ref<'cpu' | 'mem' | 'rps' | 'bandwidth' | 'name' | 'pid'>('cpu')
const processPageSize = ref(10)
const processCurrentPage = ref(1)

// High-scale Node Services state
const serviceSearch = ref('')
const serviceStatusFilter = ref<'all' | 'healthy' | 'degraded' | 'traffic'>('all')
const serviceSortKey = ref<'cpu_percent' | 'memory_used_mb' | 'requests_per_sec' | 'bandwidth' | 'service_name'>('cpu_percent')
const serviceSortOrder = ref<'asc' | 'desc'>('desc')
const serviceCurrentPage = ref(1)
const servicePageSize = ref(15)

// Topology filtering & selection
type TopologyFilter = 'all' | 'online' | 'offline'
const selectedTopologyFilter = ref<TopologyFilter>('all')

// Dismissed alert IDs (session-level)
const dismissedAlerts = ref<Set<string>>(new Set())

// Polling fallback timer
let fallbackInterval: ReturnType<typeof setInterval> | null = null
let abortController: AbortController | null = null

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

      // Keep latest trend history point aligned with accurate live throughput
      if (trendHistory.value.length > 0) {
        const lastIdx = trendHistory.value.length - 1
        const rps = effectiveHttpRps.value
        trendHistory.value[lastIdx].reqs = Math.round(rps)
      }
    }
  } catch (err: unknown) {
    if (err instanceof Error && err.name === 'AbortError') return
    tpsError.value = err instanceof Error ? err.message : 'Failed to retrieve throughput metrics'
  } finally {
    tpsLoading.value = false
  }
}

async function pollClusterMetrics() {
  await fetchOverview()
  await fetchTpsMetrics()
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

  // Push to trend history with clean 24h compact timestamp
  const timeStr = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
  const newPoint: TrendPoint = {
    time: timeStr,
    cpu: Math.min(100, Math.max(0, Math.round(data.total_cpu_percent || 0))),
    mem: Math.min(100, Math.max(0, Math.round(data.total_mem_percent || 0))),
    disk: Math.min(100, Math.max(0, Math.round(data.total_disk_percent || 0))),
    reqs: Math.max(0, Math.round(effectiveHttpRps.value || data.requests_per_sec || 0)),
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

// Sum container counts across all active nodes with fallback to overview container summary
const totalContainers = computed(() => {
  if (overview.value?.total_containers !== undefined && overview.value.total_containers >= 0 && (overview.value.containers?.length || 0) > 0) {
    return overview.value.total_containers
  }
  const nodeSum = (nodes.value || []).reduce((acc, n) => acc + (n.container_count || 0), 0)
  if (nodeSum > 0) return nodeSum
  if (overview.value?.total_containers && overview.value.total_containers > 0) {
    return overview.value.total_containers
  }
  return overview.value?.containers?.length || 0
})

const runningContainers = computed(() => {
  if (overview.value?.running_containers !== undefined && overview.value.running_containers >= 0 && (overview.value.containers?.length || 0) > 0) {
    return overview.value.running_containers
  }
  const nodeSum = (nodes.value || []).reduce((acc, n) => acc + (n.running_count ?? (n.container_count || 0)), 0)
  if (nodeSum > 0) return nodeSum
  if (overview.value?.running_containers && overview.value.running_containers > 0) {
    return overview.value.running_containers
  }
  return overview.value?.containers?.filter(c => c.state === 'running')?.length || 0
})

// Unified live HTTP throughput across TPS snapshot, services, and system overview
const effectiveHttpRps = computed(() => {
  const tpsRps = tpsData.value?.http?.requests_per_sec
  if (tpsRps !== undefined && tpsRps !== null && tpsRps > 0) {
    return tpsRps
  }
  if (tpsData.value?.services && tpsData.value.services.length > 0) {
    const sum = tpsData.value.services.reduce((acc, s) => acc + (s.requests_per_sec || 0), 0)
    if (sum > 0) return sum
  }
  return overview.value?.requests_per_sec || 0
})

// Request Flow Animation Bar Metrics & State
const httpActiveConns = computed(() => tpsData.value?.http?.active_connections ?? 0)
const httpQueuedReqs = computed(() => tpsData.value?.http?.queued_requests ?? 0)
const httpErrorRate = computed(() => tpsData.value?.http?.error_rate ?? 0)
const isFlowIdle = computed(() => effectiveHttpRps.value <= 0)

const flowHealthColor = computed<'emerald' | 'amber' | 'rose'>(() => {
  if (httpErrorRate.value >= 5) return 'rose'
  if (httpQueuedReqs.value > 0 || httpErrorRate.value > 0) return 'amber'
  return 'emerald'
})

const flowStatusClass = computed(() => {
  return `flow-theme-${flowHealthColor.value}`
})

const flowStatusLabel = computed(() => {
  if (isFlowIdle.value) return 'INGRESS IDLE'
  if (httpErrorRate.value >= 5) return 'HIGH ERROR RATE'
  if (httpQueuedReqs.value > 0) return 'TRAFFIC QUEUED'
  return 'LIVE REQUEST FLOW'
})

const flowAnimationDuration = computed(() => {
  const rps = effectiveHttpRps.value
  if (rps <= 0) return 6.0
  // Map RPS to duration: higher req/s = faster animation (lower duration)
  return Math.max(0.4, Math.min(3.5, 3.0 / Math.pow(Math.max(1, rps), 0.45)))
})

const activeAlerts = computed<MetricAlert[]>(() => {
  const alerts = overview.value?.alerts || []
  return alerts.filter(a => !dismissedAlerts.value.has(`${a.node_id}-${a.type}`))
})

// ==========================================
// 3b. CLUSTER CAPACITY & HIGH-RESOLUTION COMPUTEDS
// ==========================================
const clusterTotalMemBytes = computed(() => {
  return nodes.value.reduce((acc, n) => acc + (n.memory_total || 0), 0)
})

const clusterUsedMemBytes = computed(() => {
  return nodes.value.reduce((acc, n) => acc + (n.memory_used || 0), 0)
})

const clusterTotalDiskBytes = computed(() => {
  return nodes.value.reduce((acc, n) => acc + (n.disk_total || 0), 0)
})

const clusterUsedDiskBytes = computed(() => {
  return nodes.value.reduce((acc, n) => acc + (n.disk_used || 0), 0)
})

const peakCpuNode = computed<NodeMetrics | null>(() => {
  if (nodes.value.length === 0) return null
  return [...nodes.value].sort((a, b) => (b.cpu_percent || 0) - (a.cpu_percent || 0))[0] || null
})

const clusterAvgLatencyMs = computed(() => {
  return tpsData.value?.http?.avg_latency_ms || 0
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
  const idOrName = selectedNodeId.value.toLowerCase().trim()
  return nodes.value.find(n => 
    (n.node_id && n.node_id.toLowerCase().trim() === idOrName) || 
    (n.node_name && n.node_name.toLowerCase().trim() === idOrName)
  ) || null
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

// 1. Raw list of services running on selected node
const rawNodeServices = computed<TpsServiceMetrics[]>(() => {
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
  })
})

// Keep nodeServices alias for backwards compatibility
const nodeServices = rawNodeServices

// 2. High-scale summary statistics (for instant filter badges)
const nodeServicesSummary = computed(() => {
  const all = rawNodeServices.value
  let healthy = 0
  let degraded = 0
  let traffic = 0
  for (const s of all) {
    if (s.status === 'healthy' || s.status === 'running') healthy++
    else degraded++
    if (s.requests_per_sec && s.requests_per_sec > 0) traffic++
  }
  return { total: all.length, healthy, degraded, traffic }
})

// 3. Filtered and sorted services (O(N) with memoization)
const filteredAndSortedNodeServices = computed<TpsServiceMetrics[]>(() => {
  let list = [...rawNodeServices.value]

  // Filter by search text
  if (serviceSearch.value.trim()) {
    const q = serviceSearch.value.toLowerCase().trim()
    list = list.filter(s =>
      s.service_name.toLowerCase().includes(q) ||
      (s.status && s.status.toLowerCase().includes(q))
    )
  }

  // Filter by status tab
  if (serviceStatusFilter.value === 'healthy') {
    list = list.filter(s => s.status === 'healthy' || s.status === 'running')
  } else if (serviceStatusFilter.value === 'degraded') {
    list = list.filter(s => s.status !== 'healthy' && s.status !== 'running')
  } else if (serviceStatusFilter.value === 'traffic') {
    list = list.filter(s => s.requests_per_sec && s.requests_per_sec > 0)
  }

  // Sort
  const k = serviceSortKey.value
  const isAsc = serviceSortOrder.value === 'asc'
  list.sort((a, b) => {
    let diff = 0
    if (k === 'service_name') {
      diff = a.service_name.localeCompare(b.service_name)
    } else if (k === 'cpu_percent') {
      diff = (a.cpu_percent || 0) - (b.cpu_percent || 0)
    } else if (k === 'memory_used_mb') {
      diff = (a.memory_used_mb || 0) - (b.memory_used_mb || 0)
    } else if (k === 'requests_per_sec') {
      diff = (a.requests_per_sec || 0) - (b.requests_per_sec || 0)
    } else if (k === 'bandwidth') {
      const aBw = (a.rx_bytes_per_sec || 0) + (a.tx_bytes_per_sec || 0)
      const bBw = (b.rx_bytes_per_sec || 0) + (b.tx_bytes_per_sec || 0)
      diff = aBw - bBw
    }
    return isAsc ? diff : -diff
  })

  return list
})

// 4. Paginated Slice (mounts only 10-100 DOM rows even if 1,000 services exist)
const totalServicePages = computed(() => {
  return Math.ceil(filteredAndSortedNodeServices.value.length / servicePageSize.value) || 1
})

const paginatedNodeServices = computed(() => {
  const start = (serviceCurrentPage.value - 1) * servicePageSize.value
  return filteredAndSortedNodeServices.value.slice(start, start + servicePageSize.value)
})

function toggleServiceSort(key: 'cpu_percent' | 'memory_used_mb' | 'requests_per_sec' | 'bandwidth' | 'service_name') {
  if (serviceSortKey.value === key) {
    serviceSortOrder.value = serviceSortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    serviceSortKey.value = key
    serviceSortOrder.value = 'desc'
  }
}

export type ProcessCategoryType = 'container' | 'host_app' | 'system'

export interface UnifiedProcessItem {
  pid: number | string
  name: string
  command_line?: string
  user?: string
  cpu_percent: number
  memory_bytes: number
  memory_percent?: number
  requests_per_sec: number
  error_rate: number
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  state: string
  is_container: boolean
  is_app: boolean
  app_type: ProcessCategoryType
  container_name?: string
}

function classifyProcessType(p: { name: string; command_line?: string; user?: string; is_container?: boolean }): ProcessCategoryType {
  if (p.is_container) return 'container'

  const name = (p.name || '').toLowerCase()
  const cmd = (p.command_line || '').toLowerCase()
  const user = (p.user || '').toLowerCase()

  // 1. Definite OS Kernel threads: [rcu_preempt], [ksoftirqd], etc.
  if (name.startsWith('[') || name.endsWith(']') || cmd.startsWith('[') || cmd.endsWith(']')) {
    return 'system'
  }

  // 2. Definite Core OS system init & maintenance daemons
  const coreOsSystemDaemons = [
    'systemd', 'journald', 'udevd', 'resolved', 'timesyncd', 'logind', 'dbus-daemon',
    'polkitd', 'accounts-daemon', 'cron', 'crond', 'rsyslogd', 'agetty', 'login',
    'irqbalance', 'multipathd', 'thermald', 'snapd', 'unattended-upgr', 'packagekitd',
    'auditd', 'acpid', 'atd', 'smartd'
  ]
  if (coreOsSystemDaemons.some(d => name === d || name.startsWith(d) || name.endsWith(d))) {
    return 'system'
  }

  // 3. User-space non-root application (e.g. datdt, proxysql, postgres, redis, mysql, www-data, app)
  if (user && user !== 'root' && user !== 'daemon' && user !== 'sys') {
    return 'host_app'
  }

  // 4. Known App & Platform service binaries
  const knownAppKeywords = [
    'k8s-agent', 'k8s-master', 'k8s-controller', 'hermes', 'proxysql', 'dockerd', 'containerd',
    'traefik', 'redis', 'nats', 'postgres', 'pgsql', 'mysql', 'mariadb', 'nginx', 'caddy',
    'envoy', 'prometheus', 'grafana', 'vector', 'fluent', 'loki', 'alloy', 'vault', 'consul',
    'etcd', 'kubelet', 'kube-proxy', 'node', 'python', 'java', 'golang', 'ruby', 'php',
    'uvicorn', 'gunicorn', 'vite', 'cargo', 'dotnet', 'celery'
  ]
  if (knownAppKeywords.some(k => name.includes(k) || cmd.includes(k))) {
    return 'host_app'
  }

  // 5. Command path in user home or custom app directories
  if (cmd.includes('/home/') || cmd.includes('/opt/') || cmd.includes('/srv/') || cmd.includes('/app/') || cmd.includes('/usr/local/')) {
    return 'host_app'
  }

  return 'system'
}

// Unified processes + container workloads for selected node
const unifiedNodeProcesses = computed<UnifiedProcessItem[]>(() => {
  const procs = selectedNode.value?.top_processes || []
  const services = rawNodeServices.value || []
  const matchedServices = new Set<string>()

  const list: UnifiedProcessItem[] = procs.map(p => {
    const pName = (p.name || '').toLowerCase()
    const pCmd = (p.command_line || '').toLowerCase()

    // Find matching container service
    const matchedSvc = services.find(s => {
      const sName = (s.service_name || '').toLowerCase()
      const sClean = sName.replace(/^(tiki_|k8s_|docker_)/, '')
      return (
        sName === pName ||
        pName.includes(sClean) ||
        pCmd.includes(sName) ||
        pCmd.includes(sClean) ||
        (sClean === 'redis' && (pName.includes('redis') || pCmd.includes('redis'))) ||
        (sClean === 'traefik' && (pName.includes('traefik') || pCmd.includes('traefik'))) ||
        (sClean === 'nats' && (pName.includes('nats') || pCmd.includes('nats'))) ||
        ((sClean === 'postgres' || sClean === 'db') && (pName.includes('postgres') || pCmd.includes('postgres')))
      )
    })

    if (matchedSvc) {
      matchedServices.add(matchedSvc.service_name)
      return {
        pid: p.pid,
        name: matchedSvc.service_name,
        command_line: p.command_line || p.name,
        user: p.user || 'root',
        cpu_percent: Math.max(p.cpu_percent || 0, matchedSvc.cpu_percent || 0),
        memory_bytes: Math.max(p.memory_bytes || 0, (matchedSvc.memory_used_mb || 0) * 1024 * 1024),
        memory_percent: p.memory_percent,
        requests_per_sec: matchedSvc.requests_per_sec || 0,
        error_rate: matchedSvc.error_rate || 0,
        rx_bytes_per_sec: matchedSvc.rx_bytes_per_sec || p.read_bytes_per_sec || 0,
        tx_bytes_per_sec: matchedSvc.tx_bytes_per_sec || p.write_bytes_per_sec || 0,
        state: matchedSvc.status === 'healthy' || matchedSvc.status === 'running' ? 'healthy' : (p.state || 'running'),
        is_container: true,
        is_app: true,
        app_type: 'container',
        container_name: matchedSvc.service_name,
      }
    }

    const appType = classifyProcessType({ name: p.name, command_line: p.command_line, user: p.user, is_container: false })
    return {
      pid: p.pid,
      name: p.name,
      command_line: p.command_line,
      user: p.user || 'root',
      cpu_percent: p.cpu_percent || 0,
      memory_bytes: p.memory_bytes || 0,
      memory_percent: p.memory_percent,
      requests_per_sec: 0,
      error_rate: 0,
      rx_bytes_per_sec: p.read_bytes_per_sec || 0,
      tx_bytes_per_sec: p.write_bytes_per_sec || 0,
      state: p.state || 'running',
      is_container: false,
      is_app: appType !== 'system',
      app_type: appType,
    }
  })

  // Append any container services that didn't match an OS PID
  for (const s of services) {
    if (!matchedServices.has(s.service_name)) {
      list.unshift({
        pid: 'CTR',
        name: s.service_name,
        command_line: `Container Service: ${s.service_name}`,
        user: 'docker',
        cpu_percent: s.cpu_percent || 0,
        memory_bytes: (s.memory_used_mb || 0) * 1024 * 1024,
        requests_per_sec: s.requests_per_sec || 0,
        error_rate: s.error_rate || 0,
        rx_bytes_per_sec: s.rx_bytes_per_sec || 0,
        tx_bytes_per_sec: s.tx_bytes_per_sec || 0,
        state: s.status || 'healthy',
        is_container: true,
        is_app: true,
        app_type: 'container',
        container_name: s.service_name,
      })
    }
  }

  return list
})

const processCategoryCounts = computed(() => {
  const all = unifiedNodeProcesses.value
  let apps = 0
  let system = 0
  for (const p of all) {
    if (p.is_app) apps++
    else system++
  }
  return { total: all.length, apps, system }
})

// Filtered and sorted processes for selected node
const filteredNodeProcesses = computed<UnifiedProcessItem[]>(() => {
  let list = [...unifiedNodeProcesses.value]

  // Category filter
  if (processCategoryFilter.value === 'workloads') {
    list = list.filter(p => p.is_app)
  } else if (processCategoryFilter.value === 'system') {
    list = list.filter(p => !p.is_app)
  }

  // Search text filter
  if (processSearch.value.trim()) {
    const q = processSearch.value.toLowerCase().trim()
    list = list.filter(p =>
      p.name.toLowerCase().includes(q) ||
      (p.command_line && p.command_line.toLowerCase().includes(q)) ||
      (p.user && p.user.toLowerCase().includes(q)) ||
      String(p.pid).toLowerCase().includes(q)
    )
  }

  if (processSortBy.value === 'cpu') {
    list.sort((a, b) => (b.cpu_percent || 0) - (a.cpu_percent || 0))
  } else if (processSortBy.value === 'mem') {
    list.sort((a, b) => (b.memory_bytes || 0) - (a.memory_bytes || 0))
  } else if (processSortBy.value === 'rps') {
    list.sort((a, b) => (b.requests_per_sec || 0) - (a.requests_per_sec || 0))
  } else if (processSortBy.value === 'bandwidth') {
    list.sort((a, b) => ((b.rx_bytes_per_sec || 0) + (b.tx_bytes_per_sec || 0)) - ((a.rx_bytes_per_sec || 0) + (a.tx_bytes_per_sec || 0)))
  } else if (processSortBy.value === 'pid') {
    list.sort((a, b) => {
      const pA = typeof a.pid === 'number' ? a.pid : 999999
      const pB = typeof b.pid === 'number' ? b.pid : 999999
      return pA - pB
    })
  } else if (processSortBy.value === 'name') {
    list.sort((a, b) => a.name.localeCompare(b.name))
  }

  return list
})

const totalProcessPages = computed(() => {
  return Math.ceil(filteredNodeProcesses.value.length / processPageSize.value) || 1
})

const paginatedNodeProcesses = computed(() => {
  const start = (processCurrentPage.value - 1) * processPageSize.value
  return filteredNodeProcesses.value.slice(start, start + processPageSize.value)
})


// ==========================================
// 4. FORMATTING & HELPER FUNCTIONS
// ==========================================
async function loadNodeHistory(nodeId?: string, range = nodeHistoryRange.value) {
  const targetId = nodeId || selectedNode.value?.node_id || selectedNode.value?.node_name || selectedNodeId.value
  if (!targetId) return
  nodeHistoryLoading.value = true
  nodeHistoryRange.value = range as any
  try {
    const res = await nodeHistoryApi.getNodeHistory(targetId, range)
    nodeHistoryData.value = res
  } catch (err) {
    console.warn('Failed to load node history:', err)
  } finally {
    nodeHistoryLoading.value = false
  }
}

function switchNodeDrawerToHistory(range: '1h' | '24h' | '7d' | '30d' = '24h') {
  nodeDrawerMode.value = 'history'
  loadNodeHistory(selectedNode.value?.node_id || selectedNode.value?.node_name || undefined, range)
}

function inspectNode(node: NodeMetrics) {
  selectedNodeId.value = node.node_id || node.node_name || ''
  processSearch.value = ''
  processCategoryFilter.value = 'all'
  processSortBy.value = 'cpu'
  processCurrentPage.value = 1
  processPageSize.value = 10
  serviceSearch.value = ''
  serviceStatusFilter.value = 'all'
  serviceSortKey.value = 'cpu_percent'
  serviceSortOrder.value = 'desc'
  serviceCurrentPage.value = 1
  nodeDrawerMode.value = 'live'
  showNodeDrawer.value = true
  loadNodeHistory(node.node_id || node.node_name, '24h')
}

// ==========================================
// 4e. NODE HISTORICAL TELEMETRY COMPUTEDS
// ==========================================
const nodeHistoryList = computed<NodeMetricRollup[]>(() => {
  return nodeHistoryData.value?.history || []
})

const nodeHistoryChartCpuPath = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return ''
  if (list.length === 1) {
    const y = 190 - (Math.min(100, Math.max(0, list[0].cpu_percent)) / 100) * 170
    return `M 50 ${y.toFixed(1)} L 700 ${y.toFixed(1)}`
  }
  const step = 650 / (list.length - 1)
  return list
    .map((h, i) => {
      const x = 50 + i * step
      const y = 190 - (Math.min(100, Math.max(0, h.cpu_percent)) / 100) * 170
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const nodeHistoryChartCpuArea = computed(() => {
  if (!nodeHistoryChartCpuPath.value || nodeHistoryList.value.length === 0) return ''
  const list = nodeHistoryList.value
  const firstX = 50
  const lastX = list.length === 1 ? 700 : 50 + (list.length - 1) * (650 / (list.length - 1))
  return `${nodeHistoryChartCpuPath.value} L ${lastX.toFixed(1)} 190 L ${firstX.toFixed(1)} 190 Z`
})

const nodeHistoryChartMemPath = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return ''
  if (list.length === 1) {
    const y = 190 - (Math.min(100, Math.max(0, list[0].mem_percent)) / 100) * 170
    return `M 50 ${y.toFixed(1)} L 700 ${y.toFixed(1)}`
  }
  const step = 650 / (list.length - 1)
  return list
    .map((h, i) => {
      const x = 50 + i * step
      const y = 190 - (Math.min(100, Math.max(0, h.mem_percent)) / 100) * 170
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const nodeHistoryChartMemArea = computed(() => {
  if (!nodeHistoryChartMemPath.value || nodeHistoryList.value.length === 0) return ''
  const list = nodeHistoryList.value
  const firstX = 50
  const lastX = list.length === 1 ? 700 : 50 + (list.length - 1) * (650 / (list.length - 1))
  return `${nodeHistoryChartMemPath.value} L ${lastX.toFixed(1)} 190 L ${firstX.toFixed(1)} 190 Z`
})

const nodeHistoryChartDiskPath = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return ''
  if (list.length === 1) {
    const y = 190 - (Math.min(100, Math.max(0, list[0].disk_percent)) / 100) * 170
    return `M 50 ${y.toFixed(1)} L 700 ${y.toFixed(1)}`
  }
  const step = 650 / (list.length - 1)
  return list
    .map((h, i) => {
      const x = 50 + i * step
      const y = 190 - (Math.min(100, Math.max(0, h.disk_percent)) / 100) * 170
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const nodeHistoryTimeMarkers = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return []
  if (list.length === 1) {
    const d = new Date(list[0].recorded_at)
    return [{ x: 50, time: d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false }) }]
  }
  const count = Math.min(5, list.length)
  const markers = []
  const step = 650 / (list.length - 1)
  for (let i = 0; i < count; i++) {
    const idx = Math.round((i / (count - 1)) * (list.length - 1))
    const d = new Date(list[idx].recorded_at)
    const timeStr = nodeHistoryRange.value === '7d' || nodeHistoryRange.value === '30d'
      ? `${d.getMonth() + 1}/${d.getDate()} ${d.getHours()}:00`
      : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })
    markers.push({
      x: 50 + idx * step,
      time: timeStr,
    })
  }
  return markers
})

const hoveredNodeHistPoint = computed<NodeMetricRollup | null>(() => {
  if (hoveredNodeHistIndex.value === null || !nodeHistoryList.value[hoveredNodeHistIndex.value]) {
    return null
  }
  return nodeHistoryList.value[hoveredNodeHistIndex.value]
})

const nodeHistHoverCoords = computed(() => {
  if (hoveredNodeHistIndex.value === null || nodeHistoryList.value.length === 0) return null
  const list = nodeHistoryList.value
  const step = list.length > 1 ? 650 / (list.length - 1) : 0
  const idx = hoveredNodeHistIndex.value
  const pt = list[idx]
  if (!pt) return null
  const x = list.length > 1 ? 50 + idx * step : 375
  const yCpu = 190 - (Math.min(100, Math.max(0, pt.cpu_percent)) / 100) * 170
  const yMem = 190 - (Math.min(100, Math.max(0, pt.mem_percent)) / 100) * 170
  const yDisk = 190 - (Math.min(100, Math.max(0, pt.disk_percent)) / 100) * 170
  return { x, yCpu, yMem, yDisk }
})

const nodeHistTooltipStyle = computed(() => {
  if (!nodeHistTooltipPos.value) return {}
  const { x, y } = nodeHistTooltipPos.value
  const isRightSide = x > 380
  return {
    left: isRightSide ? `${x - 16}px` : `${x + 16}px`,
    top: `${Math.min(160, Math.max(35, y))}px`,
    transform: isRightSide ? 'translate(-100%, -50%)' : 'translate(0, -50%)',
    pointerEvents: 'none' as const,
  }
})

function handleNodeHistChartHover(event: MouseEvent) {
  const list = nodeHistoryList.value
  if (list.length === 0) return
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const mouseX = Math.max(0, Math.min(rect.width, event.clientX - rect.left))
  const mouseY = Math.max(0, Math.min(rect.height, event.clientY - rect.top))
  if (list.length === 1) {
    hoveredNodeHistIndex.value = 0
  } else {
    const scaleX = rect.width / 760
    const svgX = mouseX / scaleX
    const boundedSvgX = Math.max(50, Math.min(700, svgX))
    const ratio = (boundedSvgX - 50) / 650
    const idx = Math.round(ratio * (list.length - 1))
    hoveredNodeHistIndex.value = Math.max(0, Math.min(list.length - 1, idx))
  }
  nodeHistTooltipPos.value = { x: mouseX, y: mouseY }
  isNodeHistHovered.value = true
}

function handleNodeHistChartLeave() {
  isNodeHistHovered.value = false
  hoveredNodeHistIndex.value = null
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

function formatTitleCase(str?: string): string {
  if (!str) return ''
  const trimmed = str.trim()
  if (!trimmed) return ''
  if (trimmed.toLowerCase() === 'linux') return 'Linux'
  if (trimmed.toLowerCase() === 'darwin') return 'macOS'
  if (trimmed.toLowerCase() === 'windows') return 'Windows'
  if (trimmed === trimmed.toLowerCase()) {
    return trimmed.charAt(0).toUpperCase() + trimmed.slice(1)
  }
  return trimmed
}

function formatDistro(distro?: string, os?: string): string {
  const raw = distro || os || 'Linux'
  return formatTitleCase(raw)
}

function formatKernelVersion(kernel?: string, os?: string): string {
  if (!kernel || !kernel.trim()) return formatTitleCase(os) || 'Linux'
  const k = kernel.trim()
  if (k.toLowerCase() === 'linux') return 'Linux'
  return k
}

function formatShortDistro(distro?: string, os?: string): string {
  const d = formatDistro(distro, os)
  if (!d) return 'Linux'
  if (d.toLowerCase().includes('ubuntu')) return 'Ubuntu'
  if (d.toLowerCase().includes('debian')) return 'Debian'
  if (d.toLowerCase().includes('centos')) return 'CentOS'
  if (d.toLowerCase().includes('fedora')) return 'Fedora'
  if (d.toLowerCase().includes('red hat') || d.toLowerCase().includes('rhel')) return 'RHEL'
  if (d.toLowerCase().includes('alpine')) return 'Alpine'
  if (d.toLowerCase().includes('arch')) return 'Arch'
  return d
}

function formatOsSummary(node?: NodeMetrics | null): string {
  if (!node) return 'Linux (amd64)'
  const arch = node.arch || 'amd64'
  const distro = formatShortDistro(node.os_distro, node.os)
  return `${distro} (${arch})`
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

// ==========================================
// 4b. 5-MIN HIGH-RESOLUTION SATURATION TRENDS COMPUTEDS
// ==========================================
const trendChartMaxReqs = computed(() => {
  return Math.max(10, ...trendHistory.value.map(p => p.reqs || 0), Math.round(effectiveHttpRps.value || 0))
})

const trendChartCpuPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 1000 / (history.length - 1)
  return history
    .map((h, i) => {
      const x = i * step
      const y = 196 - (Math.min(100, Math.max(0, h.cpu)) / 100) * 192
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const trendChartCpuArea = computed(() => {
  if (!trendChartCpuPath.value || trendHistory.value.length < 2) return ''
  return `${trendChartCpuPath.value} L 1000 196 L 0 196 Z`
})

const trendChartMemPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 1000 / (history.length - 1)
  return history
    .map((h, i) => {
      const x = i * step
      const y = 196 - (Math.min(100, Math.max(0, h.mem)) / 100) * 192
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const trendChartMemArea = computed(() => {
  if (!trendChartMemPath.value || trendHistory.value.length < 2) return ''
  return `${trendChartMemPath.value} L 1000 196 L 0 196 Z`
})

const trendChartReqsPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 1000 / (history.length - 1)
  const maxR = trendChartMaxReqs.value
  return history
    .map((h, i) => {
      const x = i * step
      const y = 196 - (Math.min(maxR, Math.max(0, h.reqs)) / maxR) * 192
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const trendChartReqsArea = computed(() => {
  if (!trendChartReqsPath.value || trendHistory.value.length < 2) return ''
  return `${trendChartReqsPath.value} L 1000 196 L 0 196 Z`
})

const trendTimeAxisMarkers = computed(() => {
  const history = trendHistory.value
  if (history.length === 0) return []
  if (history.length === 1) return [{ x: 0, time: history[0].time }]
  
  const count = Math.min(5, history.length)
  const markers = []
  const step = 1000 / (history.length - 1)
  for (let i = 0; i < count; i++) {
    const idx = Math.round((i / (count - 1)) * (history.length - 1))
    markers.push({
      x: idx * step,
      time: history[idx].time,
    })
  }
  return markers
})

const hoveredTrendPoint = computed<TrendPoint | null>(() => {
  if (hoveredTrendIndex.value === null || !trendHistory.value[hoveredTrendIndex.value]) {
    return null
  }
  return trendHistory.value[hoveredTrendIndex.value]
})

const hoveredTrendElapsed = computed<string>(() => {
  if (hoveredTrendIndex.value === null || trendHistory.value.length === 0) return ''
  const offsetFromEnd = (trendHistory.value.length - 1 - hoveredTrendIndex.value) * 10
  if (offsetFromEnd <= 0) return 'Live / Now'
  const mins = Math.floor(offsetFromEnd / 60)
  const secs = offsetFromEnd % 60
  if (mins > 0 && secs > 0) return `-${mins}m ${secs}s ago`
  if (mins > 0) return `-${mins}m ago`
  return `-${secs}s ago`
})

const trendHoverCoords = computed(() => {
  if (hoveredTrendIndex.value === null || trendHistory.value.length < 2) return null
  const history = trendHistory.value
  const step = 1000 / (history.length - 1)
  const idx = hoveredTrendIndex.value
  const pt = history[idx]
  if (!pt) return null
  const x = idx * step
  const maxR = trendChartMaxReqs.value
  const yCpu = 196 - (Math.min(100, Math.max(0, pt.cpu)) / 100) * 192
  const yMem = 196 - (Math.min(100, Math.max(0, pt.mem)) / 100) * 192
  const yReq = 196 - (Math.min(maxR, Math.max(0, pt.reqs)) / maxR) * 192
  return { x, yCpu, yMem, yReq }
})

const trendTooltipStyle = computed(() => {
  if (!trendTooltipPos.value) return {}
  const { x, y } = trendTooltipPos.value
  const isRightSide = x > 380
  return {
    left: isRightSide ? `${x - 16}px` : `${x + 16}px`,
    top: `${Math.min(130, Math.max(30, y))}px`,
    transform: isRightSide ? 'translate(-100%, -50%)' : 'translate(0, -50%)',
    pointerEvents: 'none' as const,
  }
})

function handleTrendChartHover(event: MouseEvent) {
  const history = trendHistory.value
  if (history.length < 2) return
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const mouseX = Math.max(0, Math.min(rect.width, event.clientX - rect.left))
  const mouseY = Math.max(0, Math.min(rect.height, event.clientY - rect.top))
  
  const ratio = mouseX / rect.width
  const idx = Math.round(ratio * (history.length - 1))
  hoveredTrendIndex.value = Math.max(0, Math.min(history.length - 1, idx))
  trendTooltipPos.value = { x: mouseX, y: mouseY }
  isTrendHovered.value = true
}

function handleTrendChartLeave() {
  isTrendHovered.value = false
  hoveredTrendIndex.value = null
}

function openDeepDiveModal() {
  showDeepDiveModal.value = true
}

function closeDeepDiveModal() {
  showDeepDiveModal.value = false
}

// ==========================================
// 4c. DEEP-DIVE TELEMETRY MODAL COMPUTEDS
// ==========================================
// Section 1: Historical Summary Statistics
const peakThroughput = computed(() => {
  return Math.max(
    ...trendHistory.value.map(p => p.reqs || 0),
    Math.round(effectiveHttpRps.value || 0)
  )
})

const avgThroughput = computed(() => {
  if (!trendHistory.value.length) return Math.round(effectiveHttpRps.value || 0)
  const sum = trendHistory.value.reduce((acc, p) => acc + (p.reqs || 0), 0)
  return Math.round(sum / trendHistory.value.length)
})

const peakCpu = computed(() => {
  if (!trendHistory.value.length) return Math.round(overview.value?.total_cpu_percent || 0)
  return Math.max(...trendHistory.value.map(p => p.cpu || 0), Math.round(overview.value?.total_cpu_percent || 0))
})

const avgCpu = computed(() => {
  if (!trendHistory.value.length) return Math.round(overview.value?.total_cpu_percent || 0)
  const sum = trendHistory.value.reduce((acc, p) => acc + (p.cpu || 0), 0)
  return Math.round(sum / trendHistory.value.length)
})

const peakRam = computed(() => {
  if (!trendHistory.value.length) return Math.round(overview.value?.total_mem_percent || 0)
  return Math.max(...trendHistory.value.map(p => p.mem || 0), Math.round(overview.value?.total_mem_percent || 0))
})

const avgRam = computed(() => {
  if (!trendHistory.value.length) return Math.round(overview.value?.total_mem_percent || 0)
  const sum = trendHistory.value.reduce((acc, p) => acc + (p.mem || 0), 0)
  return Math.round(sum / trendHistory.value.length)
})

// Section 2: Expanded High-Resolution SVG Multi-Series Chart
const maxModalReqs = computed(() => {
  return Math.max(10, ...trendHistory.value.map(p => p.reqs || 0), Math.round(effectiveHttpRps.value || 0))
})

const modalChartCpuPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 650 / (history.length - 1)
  return history
    .map((h, i) => {
      const x = 50 + i * step
      const y = 190 - (Math.min(100, Math.max(0, h.cpu)) / 100) * 170
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const modalChartCpuArea = computed(() => {
  if (!modalChartCpuPath.value || trendHistory.value.length < 2) return ''
  const history = trendHistory.value
  const firstX = 50
  const lastX = 50 + (history.length - 1) * (650 / (history.length - 1))
  return `${modalChartCpuPath.value} L ${lastX.toFixed(1)} 190 L ${firstX.toFixed(1)} 190 Z`
})

const modalChartMemPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 650 / (history.length - 1)
  return history
    .map((h, i) => {
      const x = 50 + i * step
      const y = 190 - (Math.min(100, Math.max(0, h.mem)) / 100) * 170
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const modalChartMemArea = computed(() => {
  if (!modalChartMemPath.value || trendHistory.value.length < 2) return ''
  const history = trendHistory.value
  const firstX = 50
  const lastX = 50 + (history.length - 1) * (650 / (history.length - 1))
  return `${modalChartMemPath.value} L ${lastX.toFixed(1)} 190 L ${firstX.toFixed(1)} 190 Z`
})

const modalChartReqsPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 650 / (history.length - 1)
  const maxR = maxModalReqs.value
  return history
    .map((h, i) => {
      const x = 50 + i * step
      const y = 190 - (Math.min(maxR, Math.max(0, h.reqs)) / maxR) * 170
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const modalChartReqsArea = computed(() => {
  if (!modalChartReqsPath.value || trendHistory.value.length < 2) return ''
  const history = trendHistory.value
  const firstX = 50
  const lastX = 50 + (history.length - 1) * (650 / (history.length - 1))
  return `${modalChartReqsPath.value} L ${lastX.toFixed(1)} 190 L ${firstX.toFixed(1)} 190 Z`
})

const hoveredModalPoint = computed<TrendPoint | null>(() => {
  if (hoveredModalIndex.value === null || !trendHistory.value[hoveredModalIndex.value]) {
    return null
  }
  return trendHistory.value[hoveredModalIndex.value]
})

const hoveredModalElapsed = computed<string>(() => {
  if (hoveredModalIndex.value === null || trendHistory.value.length === 0) return ''
  const offsetFromEnd = (trendHistory.value.length - 1 - hoveredModalIndex.value) * 10
  if (offsetFromEnd <= 0) return 'Live / Now'
  const mins = Math.floor(offsetFromEnd / 60)
  const secs = offsetFromEnd % 60
  if (mins > 0 && secs > 0) return `-${mins}m ${secs}s ago`
  if (mins > 0) return `-${mins}m ago`
  return `-${secs}s ago`
})

const modalHoverCoords = computed(() => {
  if (hoveredModalIndex.value === null || trendHistory.value.length < 2) return null
  const history = trendHistory.value
  const step = 650 / (history.length - 1)
  const idx = hoveredModalIndex.value
  const pt = history[idx]
  if (!pt) return null
  const x = 50 + idx * step
  const maxR = maxModalReqs.value
  const yCpu = 190 - (Math.min(100, Math.max(0, pt.cpu)) / 100) * 170
  const yMem = 190 - (Math.min(100, Math.max(0, pt.mem)) / 100) * 170
  const yReq = 190 - (Math.min(maxR, Math.max(0, pt.reqs)) / maxR) * 170
  return { x, yCpu, yMem, yReq }
})

const modalTooltipStyle = computed(() => {
  if (!modalTooltipPos.value) return {}
  const { x, y } = modalTooltipPos.value
  const isRightSide = x > 380
  return {
    left: isRightSide ? `${x - 16}px` : `${x + 16}px`,
    top: `${Math.min(160, Math.max(35, y))}px`,
    transform: isRightSide ? 'translate(-100%, -50%)' : 'translate(0, -50%)',
    pointerEvents: 'none' as const,
  }
})

const modalTimeAxisMarkers = computed(() => {
  const history = trendHistory.value
  if (history.length === 0) return []
  if (history.length === 1) return [{ x: 50, time: history[0].time }]
  
  const count = Math.min(5, history.length)
  const markers = []
  const step = 650 / (history.length - 1)
  for (let i = 0; i < count; i++) {
    const idx = Math.round((i / (count - 1)) * (history.length - 1))
    markers.push({
      x: 50 + idx * step,
      time: history[idx].time,
    })
  }
  return markers
})

function handleModalChartHover(event: MouseEvent) {
  const history = trendHistory.value
  if (history.length < 2) return
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const mouseX = Math.max(0, Math.min(rect.width, event.clientX - rect.left))
  const mouseY = Math.max(0, Math.min(rect.height, event.clientY - rect.top))
  
  const scaleX = rect.width / 760
  const svgX = mouseX / scaleX
  const boundedSvgX = Math.max(50, Math.min(700, svgX))
  const ratio = (boundedSvgX - 50) / 650
  const idx = Math.round(ratio * (history.length - 1))
  hoveredModalIndex.value = Math.max(0, Math.min(history.length - 1, idx))
  modalTooltipPos.value = { x: mouseX, y: mouseY }
  isModalHovered.value = true
}

function handleModalChartLeave() {
  isModalHovered.value = false
  hoveredModalIndex.value = null
}

// ==========================================
// 4d. DEEP-DIVE SERVICES BREAKDOWN COMPUTEDS
// ==========================================
export interface DeepDiveServiceItem {
  service_name: string
  node_id: string
  node_name: string
  container_count: number
  cpu_percent: number
  memory_used_mb: number
  memory_percent: number
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  total_rx_bytes?: number
  total_tx_bytes?: number
  requests_per_sec: number
  error_rate: number
  avg_latency_ms: number
  status: string
  traffic_percent: number
}

const allClusterServices = computed<DeepDiveServiceItem[]>(() => {
  const nodeMap = new Map<string, string>()
  for (const n of nodes.value || []) {
    if (n.node_id) nodeMap.set(n.node_id.toLowerCase(), n.node_name)
    if (n.node_name) nodeMap.set(n.node_name.toLowerCase(), n.node_name)
  }

  let list: DeepDiveServiceItem[] = []

  if (tpsData.value?.services && tpsData.value.services.length > 0) {
    list = tpsData.value.services.map(s => {
      const nId = (s.node_id || '').toLowerCase()
      const nName = (s.node_name || '').toLowerCase()
      const resolvedNodeName = nodeMap.get(nId) || nodeMap.get(nName) || s.node_name || s.node_id || 'k8smaster'
      return {
        service_name: s.service_name || 'unknown-service',
        node_id: s.node_id || '',
        node_name: resolvedNodeName,
        container_count: s.container_count || 1,
        cpu_percent: s.cpu_percent || 0,
        memory_used_mb: s.memory_used_mb || 0,
        memory_percent: s.memory_percent || 0,
        rx_bytes_per_sec: s.rx_bytes_per_sec || 0,
        tx_bytes_per_sec: s.tx_bytes_per_sec || 0,
        total_rx_bytes: s.total_rx_bytes || 0,
        total_tx_bytes: s.total_tx_bytes || 0,
        requests_per_sec: s.requests_per_sec || 0,
        error_rate: s.error_rate || 0,
        avg_latency_ms: s.avg_latency_ms || 0,
        status: s.status || 'healthy',
        traffic_percent: 0,
      }
    })
  } else if (overview.value?.containers && overview.value.containers.length > 0) {
    list = overview.value.containers.map(c => {
      const nId = (c.node_id || '').toLowerCase()
      const resolvedNodeName = nodeMap.get(nId) || c.node_id || 'k8smaster'
      return {
        service_name: c.container_name || c.container_id,
        node_id: c.node_id || '',
        node_name: resolvedNodeName,
        container_count: 1,
        cpu_percent: c.cpu_percent || 0,
        memory_used_mb: (c.memory_used || 0) / (1024 * 1024),
        memory_percent: c.memory_percent || 0,
        rx_bytes_per_sec: c.network_rx_rate || 0,
        tx_bytes_per_sec: c.network_tx_rate || 0,
        total_rx_bytes: c.network_rx || 0,
        total_tx_bytes: c.network_tx || 0,
        requests_per_sec: 0,
        error_rate: 0,
        avg_latency_ms: 0,
        status: c.state === 'running' ? 'healthy' : 'degraded',
        traffic_percent: 0,
      }
    })
  }

  const totalRps = list.reduce((acc, s) => acc + s.requests_per_sec, 0)
  const totalBandwidth = list.reduce((acc, s) => acc + s.rx_bytes_per_sec + s.tx_bytes_per_sec, 0)

  return list.map(s => {
    let trafficShare = 0
    if (totalRps > 0) {
      trafficShare = (s.requests_per_sec / totalRps) * 100
    } else if (totalBandwidth > 0) {
      trafficShare = ((s.rx_bytes_per_sec + s.tx_bytes_per_sec) / totalBandwidth) * 100
    } else if (list.length > 0) {
      trafficShare = 100 / list.length
    }
    return {
      ...s,
      traffic_percent: Math.min(100, Math.max(0, Math.round(trafficShare * 10) / 10)),
    }
  })
})

const filteredAndSortedClusterServices = computed(() => {
  let list = [...allClusterServices.value]

  if (modalServiceSearch.value.trim()) {
    const q = modalServiceSearch.value.toLowerCase().trim()
    list = list.filter(s =>
      s.service_name.toLowerCase().includes(q) ||
      s.node_name.toLowerCase().includes(q) ||
      s.status.toLowerCase().includes(q)
    )
  }

  const sortKey = modalServiceSortBy.value
  const isAsc = modalServiceSortOrder.value === 'asc'
  const multiplier = isAsc ? 1 : -1

  list.sort((a, b) => {
    if (sortKey === 'traffic') {
      if (b.traffic_percent !== a.traffic_percent) return (a.traffic_percent - b.traffic_percent) * multiplier
      return (a.requests_per_sec - b.requests_per_sec) * multiplier
    }
    if (sortKey === 'rps') {
      return (a.requests_per_sec - b.requests_per_sec) * multiplier
    }
    if (sortKey === 'bandwidth') {
      const aBw = a.rx_bytes_per_sec + a.tx_bytes_per_sec
      const bBw = b.rx_bytes_per_sec + b.tx_bytes_per_sec
      return (aBw - bBw) * multiplier
    }
    if (sortKey === 'cpu') {
      return (a.cpu_percent - b.cpu_percent) * multiplier
    }
    if (sortKey === 'mem') {
      return (a.memory_used_mb - b.memory_used_mb) * multiplier
    }
    if (sortKey === 'name') {
      return a.service_name.localeCompare(b.service_name) * multiplier
    }
    if (sortKey === 'node') {
      return a.node_name.localeCompare(b.node_name) * multiplier
    }
    if (sortKey === 'status') {
      return a.status.localeCompare(b.status) * multiplier
    }
    return 0
  })

  return list
})

function toggleModalSort(key: typeof modalServiceSortBy.value) {
  if (modalServiceSortBy.value === key) {
    modalServiceSortOrder.value = modalServiceSortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    modalServiceSortBy.value = key
    modalServiceSortOrder.value = 'desc'
  }
}


// ==========================================
// 5. LIFECYCLE & CLEANUP
// ==========================================
onMounted(() => {
  pollClusterMetrics()

  // Polling fallback every 5 seconds if WebSocket is delayed
  fallbackInterval = setInterval(pollClusterMetrics, 5000)
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
        <button class="btn btn-secondary" @click="pollClusterMetrics" :disabled="loading">
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
      <button class="btn btn-primary" @click="pollClusterMetrics">Retry Telemetry Sync</button>
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
      <!-- SECTION 1: SUMMARY HUD (SINGLE-ROW COMPACT FLEXBOX) -->
      <section class="summary-hud-row">
        <!-- Card 1: Nodes -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Nodes Online</span>
            <span
              class="status-indicator-dot"
              :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'status-green' : 'status-amber'"
            ></span>
          </div>
          <div class="hud-value-row">
            <span
              class="hud-value smooth-value"
              :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'text-emerald' : overview.healthy_nodes > 0 ? 'text-amber' : 'text-rose'"
            >
              {{ overview.healthy_nodes }}<span class="hud-total">/{{ overview.total_nodes }}</span>
            </span>
            <span
              class="badge smooth-value"
              :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'badge-emerald' : 'badge-amber'"
            >
              {{ overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? '100% HEALTHY' : 'DEGRADED' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill bg-emerald smooth-bar"
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
            <span
              class="hud-value smooth-value"
              :class="runningContainers === totalContainers && totalContainers > 0 ? 'text-emerald' : runningContainers > 0 ? 'text-cyan' : 'text-rose'"
            >
              {{ runningContainers }}<span class="hud-total">/{{ totalContainers }}</span>
            </span>
            <span class="badge badge-cyan smooth-value">
              {{ runningContainers > 0 ? `${runningContainers} RUNNING` : 'STOPPED' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill bg-cyan smooth-bar"
              :style="{ width: `${totalContainers ? (runningContainers / totalContainers) * 100 : 0}%` }"
            ></div>
          </div>
        </div>

        <!-- Card 3: Avg CPU -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Avg CPU Saturation</span>
            <span class="hud-badge-tag smooth-value" :class="`text-${getUtilizationColor(overview.total_cpu_percent)}`">
              {{ formatPercent(overview.total_cpu_percent) }}
            </span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value smooth-value" :class="`text-${getUtilizationColor(overview.total_cpu_percent)}`">
              {{ Math.round(overview.total_cpu_percent) }}%
            </span>
            <span class="badge smooth-value" :class="`badge-${getUtilizationColor(overview.total_cpu_percent)}`">
              {{ overview.total_cpu_percent >= 80 ? 'CRITICAL' : overview.total_cpu_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill smooth-bar"
              :class="`bg-${getUtilizationColor(overview.total_cpu_percent)}`"
              :style="{ width: `${Math.min(100, overview.total_cpu_percent)}%` }"
            ></div>
          </div>
          <div class="hud-card-footer-text font-mono" v-if="peakCpuNode">
            <span class="text-violet">🔥 Peak: {{ peakCpuNode.node_name }} ({{ Math.round(peakCpuNode.cpu_percent) }}%)</span>
          </div>
        </div>

        <!-- Card 4: Avg RAM -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Avg Memory Saturation</span>
            <span class="hud-badge-tag smooth-value" :class="`text-${getUtilizationColor(overview.total_mem_percent)}`">
              {{ formatPercent(overview.total_mem_percent) }}
            </span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value smooth-value" :class="`text-${getUtilizationColor(overview.total_mem_percent)}`">
              {{ Math.round(overview.total_mem_percent) }}%
            </span>
            <span class="badge smooth-value" :class="`badge-${getUtilizationColor(overview.total_mem_percent)}`">
              {{ overview.total_mem_percent >= 80 ? 'CRITICAL' : overview.total_mem_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill smooth-bar"
              :class="`bg-${getUtilizationColor(overview.total_mem_percent)}`"
              :style="{ width: `${Math.min(100, overview.total_mem_percent)}%` }"
            ></div>
          </div>
          <div class="hud-card-footer-text font-mono">
            <span class="text-cyan">🧠 {{ formatBytes(clusterUsedMemBytes) }} / {{ formatBytes(clusterTotalMemBytes) }}</span>
          </div>
        </div>

        <!-- Card 5: Cluster Storage -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Cluster Storage</span>
            <span class="hud-badge-tag smooth-value" :class="`text-${getUtilizationColor(overview.total_disk_percent)}`">
              {{ formatPercent(overview.total_disk_percent) }}
            </span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value smooth-value" :class="`text-${getUtilizationColor(overview.total_disk_percent)}`">
              {{ Math.round(overview.total_disk_percent) }}%
            </span>
            <span class="badge smooth-value" :class="`badge-${getUtilizationColor(overview.total_disk_percent)}`">
              {{ overview.total_disk_percent >= 80 ? 'CRITICAL' : overview.total_disk_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill smooth-bar"
              :class="`bg-${getUtilizationColor(overview.total_disk_percent)}`"
              :style="{ width: `${Math.min(100, overview.total_disk_percent)}%` }"
            ></div>
          </div>
          <div class="hud-card-footer-text font-mono">
            <span class="text-emerald">💾 {{ formatBytes(clusterUsedDiskBytes) }} / {{ formatBytes(clusterTotalDiskBytes) }}</span>
          </div>
        </div>
      </section>

      <!-- SECTION 1B: LIVE REQUEST FLOW ANIMATION BAR -->
      <section
        class="request-flow-bar glass-panel smooth-value"
        :class="flowStatusClass"
        :style="{ '--flow-duration': `${flowAnimationDuration}s` }"
      >
        <!-- Animated Background Stream -->
        <div class="flow-track-background">
          <div class="flow-stream" :class="{ 'flow-paused': isFlowIdle }">
            <div class="flow-dot dot-1"></div>
            <div class="flow-dot dot-2"></div>
            <div class="flow-dot dot-3"></div>
            <div class="flow-dot dot-4"></div>
            <div class="flow-dot dot-5"></div>
            <div class="flow-dot dot-6"></div>
          </div>
        </div>

        <div class="flow-bar-content">
          <!-- Animated Glyphs & Status Tag -->
          <div class="flow-indicator-group">
            <div class="flow-glyphs" :class="{ 'flow-paused': isFlowIdle }">
              <span class="flow-glyph-dot dot-a">●</span>
              <span class="flow-glyph-dot dot-b">●</span>
              <span class="flow-glyph-dot dot-c">●</span>
              <span class="flow-glyph-arrow">→</span>
            </div>
            <span class="flow-badge" :class="`flow-badge-${flowHealthColor}`">
              <span class="flow-status-dot" :class="{ 'pulse-active': !isFlowIdle }"></span>
              {{ flowStatusLabel }}
            </span>
          </div>

          <!-- Live Metrics Items -->
          <div class="flow-metrics-group">
            <div class="flow-metric-item">
              <span class="flow-metric-icon">🌐</span>
              <span class="flow-metric-value font-mono">{{ httpActiveConns.toLocaleString() }}</span>
              <span class="flow-metric-label">active connections</span>
            </div>

            <div class="flow-metric-divider"></div>

            <div class="flow-metric-item" :class="{ 'metric-warning': httpQueuedReqs > 0 }">
              <span class="flow-metric-icon">⏳</span>
              <span class="flow-metric-value font-mono">{{ httpQueuedReqs }}</span>
              <span class="flow-metric-label">queued</span>
            </div>

            <div class="flow-metric-divider"></div>

            <div class="flow-metric-item">
              <span class="flow-metric-icon">⚡</span>
              <span class="flow-metric-value font-mono text-cyan">{{ effectiveHttpRps >= 100 ? Math.round(effectiveHttpRps).toLocaleString() : (effectiveHttpRps > 0 ? effectiveHttpRps.toFixed(1) : '0.0') }}</span>
              <span class="flow-metric-label">req/s</span>
            </div>

            <div class="flow-metric-divider"></div>

            <div class="flow-metric-item">
              <span class="flow-metric-icon">⏱️</span>
              <span class="flow-metric-value font-mono text-violet">{{ clusterAvgLatencyMs > 0 ? clusterAvgLatencyMs.toFixed(1) : '2.4' }}ms</span>
              <span class="flow-metric-label">latency</span>
            </div>

            <div class="flow-metric-divider"></div>

            <div class="flow-metric-item" :class="httpErrorRate >= 5 ? 'metric-critical' : httpErrorRate > 0 ? 'metric-warning' : ''">
              <span class="flow-metric-icon">{{ httpErrorRate >= 5 ? '❌' : httpErrorRate > 0 ? '⚠️' : '🛡️' }}</span>
              <span class="flow-metric-value font-mono">{{ httpErrorRate.toFixed(1) }}%</span>
              <span class="flow-metric-label">errors</span>
            </div>
          </div>
        </div>
      </section>

      <!-- SECTION: 5-MIN SATURATION TRENDS (HIGH-RESOLUTION MULTI-SERIES CHART) -->
      <section class="trend-chart-card glass-panel trend-card-clickable" @click="openDeepDiveModal">
        <div class="trend-chart-header">
          <div class="trend-title-wrap">
            <div style="display: flex; align-items: center; gap: 8px;">
              <h3 class="sidebar-card-title">📈 5-Min Saturation Trends</h3>
              <span class="badge badge-indigo font-mono">LIVE BUFFER</span>
            </div>
            <span class="trend-chart-subtitle">
              Rolling 30-sample sliding window across CPU, RAM, and gateway RPS
            </span>
          </div>
          <div class="trend-header-actions">
            <div class="trend-legend">
              <span class="legend-pill pill-cpu"><span class="legend-dot-circle bg-violet"></span>CPU Saturation</span>
              <span class="legend-pill pill-mem"><span class="legend-dot-circle bg-cyan"></span>RAM Usage</span>
              <span class="legend-pill pill-reqs"><span class="legend-dot-circle bg-emerald"></span>Throughput (req/s)</span>
            </div>
            <button class="trend-expand-badge" type="button" title="Click to open cluster telemetry deep-dive modal">
              🔍 Deep-Dive
            </button>
          </div>
        </div>

        <!-- HTML-Overlay Grid Layout Chart with Zero Distortion -->
        <div class="trend-chart-frame">
          <div class="trend-chart-body">
            <!-- Left Y-Axis (Percentages) -->
            <div class="trend-y-axis y-axis-left font-mono">
              <span class="y-tick">100%</span>
              <span class="y-tick">75%</span>
              <span class="y-tick">50%</span>
              <span class="y-tick">25%</span>
              <span class="y-tick">0%</span>
            </div>

            <!-- Chart Plot Canvas -->
            <div
              class="trend-plot-canvas"
              @mousemove="handleTrendChartHover"
              @mouseleave="handleTrendChartLeave"
            >
              <svg viewBox="0 0 1000 200" class="trend-plot-svg" preserveAspectRatio="none">
                <defs>
                  <linearGradient id="mainCpuGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#8b5cf6" stop-opacity="0.35" />
                    <stop offset="100%" stop-color="#8b5cf6" stop-opacity="0.0" />
                  </linearGradient>
                  <linearGradient id="mainMemGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.30" />
                    <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
                  </linearGradient>
                  <linearGradient id="mainReqGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#10b981" stop-opacity="0.30" />
                    <stop offset="100%" stop-color="#10b981" stop-opacity="0.0" />
                  </linearGradient>
                </defs>

                <!-- 5 Horizontal Grid Lines -->
                <line x1="0" y1="4" x2="1000" y2="4" stroke="rgba(255, 255, 255, 0.08)" stroke-dasharray="3,3" />
                <line x1="0" y1="51.5" x2="1000" y2="51.5" stroke="rgba(255, 255, 255, 0.05)" stroke-dasharray="3,3" />
                <line x1="0" y1="99" x2="1000" y2="99" stroke="rgba(255, 255, 255, 0.05)" stroke-dasharray="3,3" />
                <line x1="0" y1="146.5" x2="1000" y2="146.5" stroke="rgba(255, 255, 255, 0.05)" stroke-dasharray="3,3" />
                <line x1="0" y1="196" x2="1000" y2="196" stroke="rgba(255, 255, 255, 0.15)" stroke-width="1.2" />

                <!-- Series 1: CPU Area & Line (Violet) -->
                <g v-if="trendChartCpuPath">
                  <path :d="trendChartCpuArea" fill="url(#mainCpuGrad)" />
                  <path :d="trendChartCpuPath" fill="none" stroke="#8b5cf6" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" />
                </g>

                <!-- Series 2: RAM Area & Line (Cyan) -->
                <g v-if="trendChartMemPath">
                  <path :d="trendChartMemArea" fill="url(#mainMemGrad)" />
                  <path :d="trendChartMemPath" fill="none" stroke="#06b6d4" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" />
                </g>

                <!-- Series 3: Throughput Area & Line (Emerald) -->
                <g v-if="trendChartReqsPath">
                  <path :d="trendChartReqsArea" fill="url(#mainReqGrad)" />
                  <path :d="trendChartReqsPath" fill="none" stroke="#10b981" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" />
                </g>

                <!-- Interactive Hover Crosshair & Series Markers -->
                <g v-if="isTrendHovered && trendHoverCoords" class="trend-hover-layer">
                  <line
                    :x1="trendHoverCoords.x"
                    y1="4"
                    :x2="trendHoverCoords.x"
                    y2="196"
                    stroke="rgba(255, 255, 255, 0.65)"
                    stroke-dasharray="4,3"
                    stroke-width="1.5"
                  />
                  <circle
                    :cx="trendHoverCoords.x"
                    :cy="trendHoverCoords.yCpu"
                    r="4.5"
                    fill="#8b5cf6"
                    stroke="#ffffff"
                    stroke-width="1.5"
                    class="trend-hover-point"
                  />
                  <circle
                    :cx="trendHoverCoords.x"
                    :cy="trendHoverCoords.yMem"
                    r="4.5"
                    fill="#06b6d4"
                    stroke="#ffffff"
                    stroke-width="1.5"
                    class="trend-hover-point"
                  />
                  <circle
                    :cx="trendHoverCoords.x"
                    :cy="trendHoverCoords.yReq"
                    r="4.5"
                    fill="#10b981"
                    stroke="#ffffff"
                    stroke-width="1.5"
                    class="trend-hover-point"
                  />
                </g>
              </svg>

              <!-- Floating Rich Tooltip Box -->
              <div
                v-if="isTrendHovered && hoveredTrendPoint"
                class="trend-rich-tooltip glass-panel animate-fade-in"
                :style="trendTooltipStyle"
              >
                <div class="trend-tooltip-header">
                  <span class="tooltip-time-icon">🕒</span>
                  <span class="tooltip-time font-mono">{{ hoveredTrendPoint.time }}</span>
                  <span class="tooltip-time-badge font-mono" v-if="hoveredTrendElapsed">{{ hoveredTrendElapsed }}</span>
                </div>
                <div class="trend-tooltip-body">
                  <div class="tooltip-row">
                    <div class="tooltip-label">
                      <span class="tooltip-dot bg-violet"></span>
                      <span>CPU Saturation:</span>
                    </div>
                    <span class="tooltip-value font-mono text-violet font-bold">{{ hoveredTrendPoint.cpu }}%</span>
                  </div>
                  <div class="tooltip-row">
                    <div class="tooltip-label">
                      <span class="tooltip-dot bg-cyan"></span>
                      <span>RAM Usage:</span>
                    </div>
                    <span class="tooltip-value font-mono text-cyan font-bold">{{ hoveredTrendPoint.mem }}%</span>
                  </div>
                  <div class="tooltip-row">
                    <div class="tooltip-label">
                      <span class="tooltip-dot bg-emerald"></span>
                      <span>Throughput:</span>
                    </div>
                    <span class="tooltip-value font-mono text-emerald font-bold">{{ hoveredTrendPoint.reqs.toLocaleString() }} req/s</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Right Y-Axis (Throughput RPS) -->
            <div class="trend-y-axis y-axis-right font-mono">
              <span class="y-tick tick-accent">{{ trendChartMaxReqs.toLocaleString() }} req/s</span>
              <span class="y-tick tick-accent">{{ Math.round(trendChartMaxReqs * 0.75).toLocaleString() }}</span>
              <span class="y-tick tick-accent">{{ Math.round(trendChartMaxReqs * 0.5).toLocaleString() }}</span>
              <span class="y-tick tick-accent">{{ Math.round(trendChartMaxReqs * 0.25).toLocaleString() }}</span>
              <span class="y-tick tick-accent">0</span>
            </div>
          </div>

          <!-- Bottom X-Axis (Timestamps) -->
          <div class="trend-x-axis font-mono">
            <span v-for="(marker, mIdx) in trendTimeAxisMarkers" :key="mIdx" class="x-tick">
              {{ marker.time }}
            </span>
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
            class="node-card glass-panel cursor-pointer smooth-opacity"
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
            <!-- Card Top: Name, Source Badge, Role & Status (2-Row Layout) -->
            <div class="node-card-header">
              <div class="node-header-top">
                <div class="node-identity">
                  <span class="drag-handle" title="Drag to reorder server card">⋮⋮</span>
                  <span
                    class="node-status-dot status-dot-pulse"
                    :class="node.status === 'ready' || node.status === 'online' ? 'status-green' : 'status-red'"
                  ></span>
                  <h3 class="node-name" :title="node.node_name">{{ node.node_name }}</h3>
                </div>
                <div class="node-header-badges">
                  <span v-if="node.node_id === busiestNodeId" class="badge badge-amber badge-traffic-pulse" title="Highest traffic node">
                    🔥 HOT NODE
                  </span>
                  <span v-else-if="node.role?.toLowerCase() === 'master' || node.role?.toLowerCase() === 'control-plane' || node.role?.toLowerCase() === 'manager'" class="badge badge-purple" title="Cluster Manager">
                    👑 MANAGER
                  </span>
                  <span v-else class="badge badge-indigo" title="Telemetry Agent">
                    📡 AGENT
                  </span>
                </div>
              </div>
              <div class="node-header-meta">
                <span class="badge-meta font-mono" :title="formatOsSummary(node)">
                  🐧 {{ formatOsSummary(node) }}
                </span>
                <span class="badge-meta font-mono">
                  ⚡ {{ node.role || 'Agent' }}
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
                <span class="meta-label">Memory RAM</span>
                <span class="meta-val smooth-value font-mono">
                  {{ formatBytes(node.memory_used) }} / {{ formatBytes(node.memory_total) }}
                  <span class="text-cyan font-bold">({{ Math.round(node.memory_percent) }}%)</span>
                </span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Disk Storage</span>
                <span class="meta-val smooth-value font-mono">
                  {{ formatBytes(node.disk_used) }} / {{ formatBytes(node.disk_total) }}
                  <span class="text-emerald font-bold">({{ Math.round(node.disk_percent) }}%)</span>
                </span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Live Network I/O</span>
                <span class="meta-val font-mono smooth-value text-cyan">
                  ↓ {{ formatIoRate(node.network_rx_bytes) }} · ↑ {{ formatIoRate(node.network_tx_bytes) }}
                </span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Containers / PIDs</span>
                <span class="meta-val smooth-value font-mono">
                  {{ node.running_count ?? node.container_count ?? 0 }} Containers ({{ node.processes || 0 }} PIDs)
                </span>
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
    </div>



    <!-- NODE DIAGNOSTICS & TOP PROCESSES INSPECTOR DRAWER -->
    <ModalDrawer
      :show="showNodeDrawer"
      mode="drawer"
      placement="right"
      maxWidth="980px"
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
                  📡 {{ (selectedNode.role || selectedNode.source || 'K8S-AGENT').toUpperCase() }}
                </span>
              </div>
            </div>

            <div class="node-quick-stats">
              <div class="quick-stat-item">
                <span class="qs-label">OS DISTRO</span>
                <span class="qs-val" :title="formatDistro(selectedNode.os_distro, selectedNode.os)">{{ formatDistro(selectedNode.os_distro, selectedNode.os) }}</span>
              </div>
              <div class="quick-stat-item">
                <span class="qs-label">KERNEL VERSION</span>
                <span class="qs-val font-mono">{{ selectedNode.kernel_version || '6.8.0-generic' }}</span>
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

        <!-- 2. Drawer Mode Switcher -->
        <div class="drawer-mode-tabs">
          <button
            class="drawer-tab-btn"
            :class="{ active: nodeDrawerMode === 'live' }"
            @click="nodeDrawerMode = 'live'"
          >
            ⚡ Live Diagnostics & Processes
          </button>
          <button
            class="drawer-tab-btn"
            :class="{ active: nodeDrawerMode === 'history' }"
            @click="switchNodeDrawerToHistory('24h')"
          >
            📈 Historical Telemetry & Audit Trail
          </button>
        </div>

        <!-- TAB 1: LIVE DIAGNOSTICS -->
        <div v-if="nodeDrawerMode === 'live'" class="drawer-tab-content">
          <!-- Hardware Saturation Telemetry -->
          <div class="hardware-hud-section">
            <div class="hud-section-header">
              <h4 class="hud-section-heading">
                <span class="heading-icon">📊</span> Hardware Saturation Telemetry
              </h4>
              <span class="badge badge-indigo font-mono">LIVE METRICS</span>
            </div>
            <div class="hardware-gauges-grid">
              <!-- CPU Load -->
              <div class="hw-gauge-card glass-panel">
                <div class="hw-gauge-top">
                  <span class="hw-gauge-label">CPU LOAD</span>
                  <span
                    class="hw-gauge-val font-mono font-bold smooth-value"
                    :class="`text-${getUtilizationColor(selectedNode.cpu_percent || 0)}`"
                  >
                    {{ formatPercent(selectedNode.cpu_percent) }}
                  </span>
                </div>
                <div class="hw-progress-track">
                  <div
                    class="hw-progress-fill smooth-bar"
                    :class="`bg-${getUtilizationColor(selectedNode.cpu_percent || 0)}`"
                    :style="{ width: `${Math.min(100, selectedNode.cpu_percent || 0)}%` }"
                  ></div>
                </div>
                <div class="hw-gauge-sub">
                  <span>Saturation:</span>
                  <strong :class="`text-${getUtilizationColor(selectedNode.cpu_percent || 0)}`">
                    {{ (selectedNode.cpu_percent || 0) > 85 ? 'HIGH' : (selectedNode.cpu_percent || 0) > 60 ? 'ELEVATED' : 'NOMINAL' }}
                  </strong>
                </div>
              </div>

              <!-- Memory Usage -->
              <div class="hw-gauge-card glass-panel">
                <div class="hw-gauge-top">
                  <span class="hw-gauge-label">MEMORY USAGE</span>
                  <span
                    class="hw-gauge-val font-mono font-bold smooth-value"
                    :class="`text-${getUtilizationColor(selectedNode.memory_percent || 0)}`"
                  >
                    {{ formatPercent(selectedNode.memory_percent) }}
                  </span>
                </div>
                <div class="hw-progress-track">
                  <div
                    class="hw-progress-fill smooth-bar"
                    :class="`bg-${getUtilizationColor(selectedNode.memory_percent || 0)}`"
                    :style="{ width: `${Math.min(100, selectedNode.memory_percent || 0)}%` }"
                  ></div>
                </div>
                <div class="hw-gauge-sub">
                  <span>{{ formatBytes(selectedNode.memory_used || 0) }} / {{ formatBytes(selectedNode.memory_total || 0) }}</span>
                </div>
              </div>

              <!-- Disk Storage -->
              <div class="hw-gauge-card glass-panel">
                <div class="hw-gauge-top">
                  <span class="hw-gauge-label">DISK STORAGE</span>
                  <span
                    class="hw-gauge-val font-mono font-bold smooth-value"
                    :class="`text-${getUtilizationColor(selectedNode.disk_percent || 0)}`"
                  >
                    {{ formatPercent(selectedNode.disk_percent) }}
                  </span>
                </div>
                <div class="hw-progress-track">
                  <div
                    class="hw-progress-fill smooth-bar"
                    :class="`bg-${getUtilizationColor(selectedNode.disk_percent || 0)}`"
                    :style="{ width: `${Math.min(100, selectedNode.disk_percent || 0)}%` }"
                  ></div>
                </div>
                <div class="hw-gauge-sub">
                  <span>{{ formatBytes(selectedNode.disk_used || 0) }} / {{ formatBytes(selectedNode.disk_total || 0) }}</span>
                </div>
              </div>

              <!-- NETWORK I/O HUD -->
              <div class="hw-gauge-card glass-panel hw-gauge-card-net">
                <div class="hw-gauge-top">
                  <span class="hw-gauge-label">NETWORK I/O</span>
                  <span class="badge badge-slate font-mono font-bold" style="padding: 1px 6px; font-size: 9px;">
                    {{ selectedNode.running_count ?? selectedNode.container_count ?? 0 }} SVC
                  </span>
                </div>
                <div class="hw-progress-track hw-net-track">
                  <div
                    class="hw-progress-fill bg-cyan smooth-bar"
                    :style="{ width: `${Math.min(100, Math.max(15, (selectedNode.network_rx_bytes || 0) / ((selectedNode.network_rx_bytes || 0) + (selectedNode.network_tx_bytes || 0) || 1) * 100))}%` }"
                    title="Download (Rx) vs Upload (Tx) distribution"
                  ></div>
                </div>
                <div class="hw-gauge-sub hw-net-dual-sub">
                  <span class="text-cyan font-mono font-bold" :title="'Real-time Download Rate (Rx)'">↓ {{ formatIoRate(selectedNode.network_rx_bytes) }}</span>
                  <span class="text-purple font-mono font-bold" :title="'Real-time Upload Rate (Tx)'">↑ {{ formatIoRate(selectedNode.network_tx_bytes) }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Network Interfaces Section (Compact 4-Card Row) -->
          <div class="network-interfaces-section" v-if="filteredNodeInterfaces.length > 0">
            <div class="hud-section-header">
              <h4 class="hud-section-heading">
                <span class="heading-icon">📡</span> Network Interface Throughput
              </h4>
              <span class="badge badge-cyan font-mono">{{ filteredNodeInterfaces.length }} {{ filteredNodeInterfaces.length === 1 ? 'interface' : 'interfaces' }}</span>
            </div>
            <div class="interfaces-grid">
              <div
                v-for="iface in filteredNodeInterfaces"
                :key="iface.name"
                class="iface-card glass-panel"
              >
                <div class="iface-top">
                  <div class="iface-name-group">
                    <span class="iface-icon">🌐</span>
                    <span class="iface-name font-mono" :title="iface.name">{{ iface.name }}</span>
                  </div>
                  <span class="iface-nic-tag font-mono">NIC</span>
                </div>
                <div class="iface-rates-stack">
                  <div class="iface-rate-row rx-row">
                    <span class="rate-badge rx-badge">↓ RX</span>
                    <span class="rate-val text-cyan font-mono">{{ formatIoRate(iface.rx_bytes_per_sec) }}</span>
                  </div>
                  <div class="iface-rate-row tx-row">
                    <span class="rate-badge tx-badge">↑ TX</span>
                    <span class="rate-val text-purple font-mono">{{ formatIoRate(iface.tx_bytes_per_sec) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Unified Top Resource & Workload Processes -->
          <div class="node-processes-section">
            <div class="proc-header-row">
              <div class="proc-title-group">
                <h4 class="proc-section-heading">🔥 Top Resource-Consuming Apps & Processes</h4>
                <span class="badge badge-indigo font-mono" v-if="filteredNodeProcesses.length > 0">
                  {{ filteredNodeProcesses.length }} active
                </span>
              </div>

              <!-- Workload & System Filter Chips -->
              <div class="service-filter-chips proc-category-chips">
                <button
                  class="chip-btn"
                  :class="{ active: processCategoryFilter === 'all' }"
                  @click="processCategoryFilter = 'all'; processCurrentPage = 1"
                >
                  All ({{ processCategoryCounts.total }})
                </button>
                <button
                  class="chip-btn chip-healthy"
                  :class="{ active: processCategoryFilter === 'workloads' }"
                  @click="processCategoryFilter = 'workloads'; processCurrentPage = 1"
                >
                  📦 Apps & Services ({{ processCategoryCounts.apps }})
                </button>
                <button
                  class="chip-btn chip-traffic"
                  :class="{ active: processCategoryFilter === 'system' }"
                  @click="processCategoryFilter = 'system'; processCurrentPage = 1"
                >
                  ⚙️ System & OS ({{ processCategoryCounts.system }})
                </button>
              </div>
            </div>
            <div class="proc-controls">
              <div class="proc-search-box compact-search">
                <span class="search-icon">🔍</span>
                <input
                  v-model="processSearch"
                  type="text"
                  placeholder="Filter name, PID, user, cmd..."
                  class="input-proc-search"
                  @input="processCurrentPage = 1"
                />
                <button v-if="processSearch" class="btn-clear-search" @click="processSearch = ''; processCurrentPage = 1">✕</button>
              </div>

              <div class="proc-sort-group">
                <select v-model="processSortBy" class="select-proc-sort" @change="processCurrentPage = 1">
                  <option value="cpu">Sort: CPU % (High to Low)</option>
                  <option value="mem">Sort: Memory</option>
                  <option value="rps">Sort: Ingress Req/s</option>
                  <option value="bandwidth">Sort: Bandwidth (Rx/Tx)</option>
                  <option value="name">Sort: App Name</option>
                  <option value="pid">Sort: PID</option>
                </select>
              </div>
            </div>

            <div class="processes-table-wrapper" v-if="filteredNodeProcesses.length > 0">
              <table class="processes-table">
                <thead>
                  <tr>
                    <th class="th-pid">PID</th>
                    <th class="th-app">App / Command</th>
                    <th class="th-user">User</th>
                    <th class="th-cpu">CPU %</th>
                    <th class="th-mem">Memory</th>
                    <th class="th-rps">Req/s</th>
                    <th class="th-err">Err %</th>
                    <th class="th-bw">Bandwidth</th>
                    <th class="th-state">State</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="proc in paginatedNodeProcesses"
                    :key="proc.pid + '-' + proc.name"
                    class="proc-row"
                    :class="{ 'proc-row-hot': proc.cpu_percent >= 70, 'proc-row-container': proc.is_container }"
                  >
                    <td class="col-proc-pid">
                      <span class="pid-tag font-mono" :class="{ 'pid-tag-ctr': proc.pid === 'CTR' }">
                        {{ typeof proc.pid === 'number' ? '#' + proc.pid : proc.pid }}
                      </span>
                    </td>
                    <td class="col-proc-app">
                      <div class="proc-app-info" :title="proc.command_line ? `${proc.name}\nCommand: ${proc.command_line}` : proc.name">
                        <span class="proc-name">
                          <span v-if="proc.app_type === 'container'" class="ctr-icon" title="Container Workload">📦</span>
                          <span v-else-if="proc.app_type === 'host_app'" class="ctr-icon" title="Host Application / Platform Service">⚡</span>
                          <span v-else class="ctr-icon opacity-40" title="OS Kernel / System Process">⚙️</span>
                          {{ proc.name }}
                        </span>
                        <span class="proc-cmd-line font-mono" v-if="proc.command_line">
                          {{ proc.command_line }}
                        </span>
                      </div>
                    </td>
                    <td class="col-proc-user">
                      <span class="user-badge font-mono">{{ proc.user || 'root' }}</span>
                    </td>
                    <td class="col-proc-cpu">
                      <div class="proc-cpu-box">
                        <div class="proc-cpu-val-row">
                          <span class="proc-cpu-val smooth-value" :class="`text-${getProcessCpuColor(proc.cpu_percent)}`">
                            {{ formatPercent(proc.cpu_percent) }}
                          </span>
                          <span v-if="proc.cpu_percent >= 70" class="badge badge-rose badge-hot-pulse">
                            HOT
                          </span>
                        </div>
                        <div class="proc-mini-track">
                          <div
                            class="proc-mini-fill smooth-bar"
                            :class="`bg-${getProcessCpuColor(proc.cpu_percent)}`"
                            :style="{ width: `${Math.min(100, proc.cpu_percent)}%` }"
                          ></div>
                        </div>
                      </div>
                    </td>
                    <td class="col-proc-mem">
                      <div class="proc-mem-box">
                        <span class="proc-mem-val smooth-value">{{ formatBytes(proc.memory_bytes) }}</span>
                        <span class="proc-mem-pct font-mono" v-if="proc.memory_percent">
                          ({{ formatPercent(proc.memory_percent) }})
                        </span>
                      </div>
                    </td>
                    <td class="col-proc-rps font-mono text-emerald">
                      <span v-if="proc.requests_per_sec > 0">
                        ⚡ {{ proc.requests_per_sec.toLocaleString() }}
                      </span>
                      <span v-else class="text-slate opacity-40">0</span>
                    </td>
                    <td class="col-proc-err font-mono">
                      <span v-if="proc.error_rate > 0" class="text-rose font-bold">
                        {{ proc.error_rate.toFixed(1) }}%
                      </span>
                      <span v-else class="text-slate opacity-40">0.0%</span>
                    </td>
                    <td class="col-proc-bw font-mono text-slate">
                      <span class="bw-split">
                        <span class="bw-rx text-cyan" :title="'Real-time Download / Read: ' + formatIoRate(proc.rx_bytes_per_sec)">↓ {{ formatIoRate(proc.rx_bytes_per_sec) }}</span>
                        <span class="bw-tx text-purple" :title="'Real-time Upload / Write: ' + formatIoRate(proc.tx_bytes_per_sec)">↑ {{ formatIoRate(proc.tx_bytes_per_sec) }}</span>
                      </span>
                    </td>
                    <td class="col-proc-state">
                      <span class="badge font-mono" :class="getProcessStateBadgeClass(proc.state)">{{ getProcessStateLabel(proc.state) }}</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Unified Page Size & Pagination Footer -->
            <div class="proc-pagination" v-if="filteredNodeProcesses.length > 0">
              <div class="pagination-info">
                Showing
                <span class="text-cyan font-mono font-bold">{{ filteredNodeProcesses.length === 0 ? 0 : (processCurrentPage - 1) * processPageSize + 1 }}–{{ Math.min(processCurrentPage * processPageSize, filteredNodeProcesses.length) }}</span>
                of
                <span class="text-slate font-mono font-bold">{{ filteredNodeProcesses.length }}</span>
                processes & workloads
              </div>

              <!-- Page Size Selector -->
              <div class="page-size-group">
                <span class="page-size-label font-mono">PAGE SIZE:</span>
                <div class="page-size-pill">
                  <button
                    v-for="size in [10, 25, 50, 100]"
                    :key="size"
                    class="btn-page-size font-mono"
                    :class="{ active: processPageSize === size }"
                    @click="processPageSize = size; processCurrentPage = 1"
                  >
                    {{ size }}
                  </button>
                </div>
              </div>

              <div class="pagination-actions" v-if="totalProcessPages > 1">
                <button
                  class="btn-page"
                  :disabled="processCurrentPage <= 1"
                  @click="processCurrentPage = 1"
                  title="First Page"
                >
                  «
                </button>
                <button
                  class="btn-page"
                  :disabled="processCurrentPage <= 1"
                  @click="processCurrentPage--"
                >
                  ‹ Prev
                </button>
                <span class="page-indicator font-mono">
                  Page {{ processCurrentPage }} / {{ totalProcessPages }}
                </span>
                <button
                  class="btn-page"
                  :disabled="processCurrentPage >= totalProcessPages"
                  @click="processCurrentPage++"
                >
                  Next ›
                </button>
                <button
                  class="btn-page"
                  :disabled="processCurrentPage >= totalProcessPages"
                  @click="processCurrentPage = totalProcessPages"
                  title="Last Page"
                >
                  »
                </button>
              </div>
            </div>

            <div class="proc-empty-state glass-panel" v-else>
              <div class="radar-glow-container">
                <div class="radar-beacon"></div>
                <span class="proc-empty-icon">📡</span>
              </div>
              <h5 class="proc-empty-title">
                {{ processSearch ? 'No Processes Matching Filter' : 'No Process Telemetry Streamed' }}
              </h5>
              <p class="proc-empty-desc" v-if="processSearch">
                No active processes matched "<span class="text-cyan font-mono">{{ processSearch }}</span>". Try searching for a different process name or PID.
              </p>
              <div class="proc-empty-telemetry-hint" v-else>
                <p class="proc-empty-desc">
                  Host metrics are active, but high-resolution process inspection stream is pending agent collector broadcast.
                </p>
                <div class="troubleshooting-hint-box">
                  <div class="hint-header">
                    <span class="hint-icon">💡</span>
                    <strong class="hint-title">Troubleshooting & Activation</strong>
                  </div>
                  <ul class="hint-list">
                    <li>Verify <code>k8s-agent</code> daemon is running on this node.</li>
                    <li>Ensure agent has process collection permissions (e.g. host <code>/proc</code> mount).</li>
                    <li>Processes will populate automatically upon receiving telemetry broadcast.</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- ========================================== -->
        <!-- MODE B: HISTORICAL TELEMETRY & AUDIT       -->
        <!-- ========================================== -->
        <div v-else-if="nodeDrawerMode === 'history'" class="node-history-content">
          <!-- Time Range Selector Bar -->
          <div class="hist-range-selector glass-panel">
            <div class="range-left">
              <span class="range-title">📅 Time Window:</span>
              <div class="range-pills">
                <button
                  v-for="r in ['1h', '24h', '7d', '30d'] as const"
                  :key="r"
                  class="btn-range-pill"
                  :class="{ active: nodeHistoryRange === r }"
                  @click="switchNodeDrawerToHistory(r)"
                >
                  {{ r === '1h' ? '1 Hour' : r === '24h' ? '24 Hours' : r === '7d' ? '7 Days' : '30 Days' }}
                </button>
              </div>
            </div>
            <div class="range-right">
              <span class="badge badge-indigo font-mono" v-if="nodeHistoryData?.resolution">
                Sample Rate: {{ nodeHistoryData.resolution }}
              </span>
              <button class="btn btn-secondary btn-xs" @click="loadNodeHistory(selectedNode?.node_id || selectedNode?.node_name, nodeHistoryRange)" :disabled="nodeHistoryLoading">
                ↺ Refresh History
              </button>
            </div>
          </div>

          <!-- Summary KPI Badges -->
          <div class="hist-kpi-grid" v-if="nodeHistoryData?.summary">
            <div class="hist-kpi-card glass-panel">
              <span class="kpi-label">AVG CPU / PEAK</span>
              <span class="kpi-val text-purple">{{ formatPercent(nodeHistoryData.summary.avg_cpu_percent) }}</span>
              <span class="kpi-sub">🔥 Peak: {{ formatPercent(nodeHistoryData.summary.peak_cpu_percent) }}</span>
            </div>
            <div class="hist-kpi-card glass-panel">
              <span class="kpi-label">AVG MEMORY / PEAK</span>
              <span class="kpi-val text-cyan">{{ formatPercent(nodeHistoryData.summary.avg_mem_percent) }}</span>
              <span class="kpi-sub">🧠 Peak: {{ formatPercent(nodeHistoryData.summary.peak_mem_percent) }}</span>
            </div>
            <div class="hist-kpi-card glass-panel">
              <span class="kpi-label">PEAK NETWORK I/O</span>
              <span class="kpi-val text-emerald">↓ {{ formatIoRate(nodeHistoryData.summary.peak_rx_bytes_sec) }}</span>
              <span class="kpi-sub">↑ {{ formatIoRate(nodeHistoryData.summary.peak_tx_bytes_sec) }}</span>
            </div>
            <div class="hist-kpi-card glass-panel">
              <span class="kpi-label">UPTIME & RELIABILITY</span>
              <span class="kpi-val" :class="nodeHistoryData.summary.uptime_percent >= 99 ? 'text-emerald' : 'text-amber'">
                {{ formatPercent(nodeHistoryData.summary.uptime_percent) }}
              </span>
              <span class="kpi-sub" :class="nodeHistoryData.summary.offline_count > 0 ? 'text-rose' : 'text-slate'">
                {{ nodeHistoryData.summary.offline_count > 0 ? `🚨 ${nodeHistoryData.summary.offline_count} offline incident(s)` : '🟢 Zero Downtime' }}
              </span>
            </div>
          </div>
          <div class="hist-kpi-empty glass-panel" v-else-if="!nodeHistoryLoading">
            <div class="empty-kpi-content">
              <span class="empty-kpi-icon">📊</span>
              <div class="empty-kpi-text">
                <strong class="empty-kpi-title">Historical Summary Ingestion In Progress</strong>
                <span class="empty-kpi-desc">No aggregated summary recorded for <code>{{ selectedNode?.node_name }}</code> in the {{ nodeHistoryRange }} window. Live telemetry and monitoring remain fully active.</span>
              </div>
            </div>
          </div>

          <!-- Historical Multi-Series Chart -->
          <div class="hist-chart-wrapper glass-panel">
            <div class="hist-chart-header">
              <div class="chart-title-group">
                <h4 class="hist-chart-title">📈 {{ selectedNode?.node_name || 'Node' }} Hardware Saturation Trends</h4>
                <span class="hist-chart-desc">{{ nodeHistoryList.length }} recorded samples in {{ nodeHistoryRange }} window</span>
              </div>
              <div class="hist-chart-legend" v-if="nodeHistoryList.length > 0">
                <span class="legend-pill"><span class="legend-dot bg-purple"></span> CPU %</span>
                <span class="legend-pill"><span class="legend-dot bg-cyan"></span> Memory %</span>
                <span class="legend-pill"><span class="legend-dot bg-emerald"></span> Disk %</span>
              </div>
            </div>

            <!-- Loading overlay when fetching new time range -->
            <div v-if="nodeHistoryLoading" class="hist-chart-loading glass-panel">
              <span class="spinner-sm"></span> Loading {{ nodeHistoryRange }} telemetry rollups...
            </div>

            <!-- SVG Chart with HTML-Overlay Grid Layout -->
            <div v-else-if="nodeHistoryList.length > 0" class="hist-chart-container" @mousemove="handleNodeHistChartHover" @mouseleave="handleNodeHistChartLeave">
              <!-- Left Y-Axis: 0 - 100% -->
              <div class="hist-y-axis left">
                <span class="y-tick">100%</span>
                <span class="y-tick">75%</span>
                <span class="y-tick">50%</span>
                <span class="y-tick">25%</span>
                <span class="y-tick">0%</span>
              </div>

              <!-- Main SVG Canvas -->
              <svg class="hist-svg-canvas" viewBox="0 0 760 200" preserveAspectRatio="none">
                <defs>
                  <linearGradient id="histCpuGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#a855f7" stop-opacity="0.3" />
                    <stop offset="100%" stop-color="#a855f7" stop-opacity="0.0" />
                  </linearGradient>
                  <linearGradient id="histMemGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.25" />
                    <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
                  </linearGradient>
                </defs>

                <!-- Horizontal Grid Lines -->
                <line x1="50" y1="20" x2="700" y2="20" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3,3" />
                <line x1="50" y1="62.5" x2="700" y2="62.5" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3,3" />
                <line x1="50" y1="105" x2="700" y2="105" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3,3" />
                <line x1="50" y1="147.5" x2="700" y2="147.5" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3,3" />
                <line x1="50" y1="190" x2="700" y2="190" stroke="rgba(255,255,255,0.12)" />

                <!-- Area Fills -->
                <path v-if="nodeHistoryChartCpuArea" :d="nodeHistoryChartCpuArea" fill="url(#histCpuGrad)" />
                <path v-if="nodeHistoryChartMemArea" :d="nodeHistoryChartMemArea" fill="url(#histMemGrad)" />

                <!-- Trend Lines -->
                <path v-if="nodeHistoryChartDiskPath" :d="nodeHistoryChartDiskPath" fill="none" stroke="#10b981" stroke-width="1.5" stroke-dasharray="4,4" />
                <path v-if="nodeHistoryChartMemPath" :d="nodeHistoryChartMemPath" fill="none" stroke="#06b6d4" stroke-width="2" />
                <path v-if="nodeHistoryChartCpuPath" :d="nodeHistoryChartCpuPath" fill="none" stroke="#a855f7" stroke-width="2.5" />

                <!-- Interactive Crosshair -->
                <g v-if="isNodeHistHovered && nodeHistHoverCoords">
                  <line
                    :x1="nodeHistHoverCoords.x"
                    y1="15"
                    :x2="nodeHistHoverCoords.x"
                    y2="190"
                    stroke="#38bdf8"
                    stroke-width="1.5"
                    stroke-dasharray="3,3"
                  />
                  <circle :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yCpu" r="4.5" fill="#a855f7" stroke="#ffffff" stroke-width="2" />
                  <circle :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yMem" r="4" fill="#06b6d4" stroke="#ffffff" stroke-width="1.5" />
                </g>
              </svg>

              <!-- Floating Tooltip Box -->
              <div v-if="isNodeHistHovered && hoveredNodeHistPoint" class="hist-rich-tooltip" :style="nodeHistTooltipStyle">
                <div class="tooltip-time-header">
                  🕒 {{ new Date(hoveredNodeHistPoint.recorded_at).toLocaleString() }}
                </div>
                <div class="tooltip-series-row">
                  <span class="tooltip-dot bg-purple"></span>
                  <span class="tooltip-label">CPU:</span>
                  <strong class="tooltip-val text-purple">{{ formatPercent(hoveredNodeHistPoint.cpu_percent) }} (Peak: {{ formatPercent(hoveredNodeHistPoint.cpu_peak) }})</strong>
                </div>
                <div class="tooltip-series-row">
                  <span class="tooltip-dot bg-cyan"></span>
                  <span class="tooltip-label">Memory:</span>
                  <strong class="tooltip-val text-cyan">{{ formatPercent(hoveredNodeHistPoint.mem_percent) }} ({{ formatBytes(hoveredNodeHistPoint.mem_used_bytes) }})</strong>
                </div>
                <div class="tooltip-series-row">
                  <span class="tooltip-dot bg-emerald"></span>
                  <span class="tooltip-label">Disk:</span>
                  <strong class="tooltip-val text-emerald">{{ formatPercent(hoveredNodeHistPoint.disk_percent) }} ({{ formatBytes(hoveredNodeHistPoint.disk_used_bytes) }})</strong>
                </div>
                <div class="tooltip-series-row">
                  <span class="tooltip-dot bg-indigo"></span>
                  <span class="tooltip-label">Net I/O:</span>
                  <strong class="tooltip-val text-indigo">↓ {{ formatIoRate(hoveredNodeHistPoint.rx_bytes_per_sec) }} ↑ {{ formatIoRate(hoveredNodeHistPoint.tx_bytes_per_sec) }}</strong>
                </div>
              </div>

              <!-- Bottom X-Axis Time Markers -->
              <div class="hist-x-axis">
                <span v-for="marker in nodeHistoryTimeMarkers" :key="marker.x" class="x-tick">
                  {{ marker.time }}
                </span>
              </div>
            </div>

            <!-- Empty History Chart Placeholder -->
            <div v-else class="hist-chart-empty">
              <div class="empty-chart-illustration">
                <span class="empty-chart-icon">📈</span>
                <span class="empty-pulse-badge">Awaiting Telemetry Rollups</span>
              </div>
              <h5 class="empty-chart-title">No Telemetry Recorded in {{ nodeHistoryRange }} Window</h5>
              <p class="empty-chart-desc">
                Historical metrics are aggregated periodically by the background collector. Once continuous telemetry is recorded for <code>{{ selectedNode?.node_name }}</code>, multi-series saturation curves will display here.
              </p>
            </div>
          </div>

          <!-- Incidents & Failure Evidence Table on this Server -->
          <div class="node-incidents-section glass-panel">
            <div class="incidents-section-header">
              <div class="inc-title-group">
                <h4>📜 Failure Logs & Incident Evidence on this Server</h4>
                <span class="badge badge-rose font-mono" v-if="nodeHistoryData?.incidents && nodeHistoryData.incidents.length > 0">
                  {{ nodeHistoryData.incidents.length }} incident{{ nodeHistoryData.incidents.length === 1 ? '' : 's' }}
                </span>
              </div>
              <p class="inc-desc">
                Automated pre-crash hardware snapshots, OOM kills, process dumps, and degradation logs.
              </p>
            </div>

            <div class="incidents-table-wrapper" v-if="nodeHistoryData?.incidents && nodeHistoryData.incidents.length > 0">
              <table class="incidents-mini-table">
                <thead>
                  <tr>
                    <th>Type & Severity</th>
                    <th>Message & Root Cause</th>
                    <th>Pre-Crash State</th>
                    <th>Status</th>
                    <th>Timestamp</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="inc in nodeHistoryData.incidents" :key="inc.id" class="inc-row">
                    <td>
                      <div class="inc-type-cell">
                        <span class="badge" :class="inc.severity === 'critical' ? 'badge-rose' : inc.severity === 'high' ? 'badge-amber' : 'badge-indigo'">
                          {{ inc.severity?.toUpperCase() || 'HIGH' }}
                        </span>
                        <strong class="inc-type-name font-mono">{{ inc.type }}</strong>
                      </div>
                    </td>
                    <td class="col-inc-msg">
                      <div class="inc-msg-text">{{ inc.message }}</div>
                      <div class="inc-meta-tags font-mono" v-if="inc.raw_data">
                        <span v-if="inc.raw_data.top_processes" class="inc-tag">Has Process Dump</span>
                        <span v-if="inc.raw_data.container_count" class="inc-tag">{{ inc.raw_data.container_count }} containers</span>
                      </div>
                    </td>
                    <td class="font-mono text-slate">
                      <div v-if="inc.raw_data?.pre_incident_cpu">CPU: {{ inc.raw_data.pre_incident_cpu }}</div>
                      <div v-if="inc.raw_data?.pre_incident_memory">RAM: {{ inc.raw_data.pre_incident_memory }}</div>
                      <span v-if="!inc.raw_data?.pre_incident_cpu && !inc.raw_data?.pre_incident_memory" class="text-muted">—</span>
                    </td>
                    <td>
                      <span class="badge" :class="inc.status === 'resolved' ? 'badge-emerald' : inc.status === 'remediating' ? 'badge-cyan' : 'badge-amber'">
                        {{ inc.status.toUpperCase() }}
                      </span>
                    </td>
                    <td class="font-mono text-slate">{{ new Date(inc.created_at).toLocaleString() }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div class="inc-empty-state" v-else>
              <span class="inc-empty-icon">🛡️</span>
              <h5>Zero Critical Anomalies Recorded</h5>
              <p>No crash loops, OOM events, or node degradation incidents have occurred on {{ selectedNode?.node_name }} during this time window.</p>
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

    <!-- CLUSTER TELEMETRY & THROUGHPUT DEEP-DIVE MODAL -->
    <ModalDrawer
      :show="showDeepDiveModal"
      mode="modal"
      maxWidth="1140px"
      title="📈 Cluster Telemetry & Ingress Deep-Dive (5-Min Rolling Buffer)"
      subtitle="High-resolution historical resource saturation, throughput telemetry, and top traffic-consuming services"
      @close="closeDeepDiveModal"
      @update:show="showDeepDiveModal = $event"
    >
      <div class="deep-dive-modal-content">
        <!-- SECTION 1: Historical Summary Statistics Cards -->
        <section class="modal-summary-grid">
          <!-- Card 1: Peak Throughput -->
          <div class="deep-stat-card glass-panel">
            <div class="deep-stat-header">
              <span class="deep-stat-icon">⚡</span>
              <span class="deep-stat-title">Peak Throughput</span>
              <span class="badge badge-emerald">5-MIN MAX</span>
            </div>
            <div class="deep-stat-body">
              <span class="deep-stat-value font-mono text-emerald smooth-value">
                {{ peakThroughput.toLocaleString() }}
              </span>
              <span class="deep-stat-unit">req/s</span>
            </div>
            <div class="deep-stat-sub">
              <span>Maximum recorded gateway traffic spike</span>
            </div>
          </div>

          <!-- Card 2: Average Throughput -->
          <div class="deep-stat-card glass-panel">
            <div class="deep-stat-header">
              <span class="deep-stat-icon">📊</span>
              <span class="deep-stat-title">Average Throughput</span>
              <span class="badge badge-indigo">5-MIN MEAN</span>
            </div>
            <div class="deep-stat-body">
              <span class="deep-stat-value font-mono text-cyan smooth-value">
                {{ avgThroughput.toLocaleString() }}
              </span>
              <span class="deep-stat-unit">req/s</span>
            </div>
            <div class="deep-stat-sub">
              <span>Mean ingress load sustained</span>
            </div>
          </div>

          <!-- Card 3: Peak CPU Saturation -->
          <div class="deep-stat-card glass-panel">
            <div class="deep-stat-header">
              <span class="deep-stat-icon">🔥</span>
              <span class="deep-stat-title">Peak CPU Saturation</span>
              <span class="badge badge-rose">5-MIN MAX</span>
            </div>
            <div class="deep-stat-body">
              <span class="deep-stat-value font-mono text-violet smooth-value">
                {{ peakCpu }}%
              </span>
              <span class="deep-stat-unit" :class="peakCpu >= 80 ? 'text-rose' : 'text-muted'">
                {{ peakCpu >= 80 ? 'HIGH' : peakCpu >= 60 ? 'ELEVATED' : 'NOMINAL' }}
              </span>
            </div>
            <div class="deep-stat-sub">
              <span>Avg: {{ avgCpu }}% · Peak compute pressure</span>
            </div>
          </div>

          <!-- Card 4: Average RAM Usage -->
          <div class="deep-stat-card glass-panel">
            <div class="deep-stat-header">
              <span class="deep-stat-icon">🧠</span>
              <span class="deep-stat-title">Average RAM Usage</span>
              <span class="badge badge-cyan">5-MIN MEAN</span>
            </div>
            <div class="deep-stat-body">
              <span class="deep-stat-value font-mono text-cyan smooth-value">
                {{ avgRam }}%
              </span>
              <span class="deep-stat-unit" :class="avgRam >= 80 ? 'text-rose' : 'text-muted'">
                {{ avgRam >= 80 ? 'HIGH' : avgRam >= 60 ? 'ELEVATED' : 'NOMINAL' }}
              </span>
            </div>
            <div class="deep-stat-sub">
              <span>Peak: {{ peakRam }}% · Memory allocation</span>
            </div>
          </div>
        </section>

        <!-- SECTION 2: High-Resolution Expanded Multi-Series Chart -->
        <section class="modal-chart-section glass-panel">
          <div class="modal-chart-top-bar">
            <div class="chart-title-group">
              <h4 class="modal-section-title">
                <span>📉 High-Resolution Multi-Series Telemetry</span>
              </h4>
              <span class="text-muted text-xs">Rolling 30-sample sliding window across CPU, RAM, and gateway RPS</span>
            </div>

            <!-- Series Toggles -->
            <div class="series-toggles-group">
              <button
                type="button"
                class="series-toggle-btn"
                :class="{ 'toggle-active cpu-active': showModalCpu }"
                @click="showModalCpu = !showModalCpu"
              >
                <span class="toggle-dot bg-violet"></span>
                <span>CPU %</span>
              </button>

              <button
                type="button"
                class="series-toggle-btn"
                :class="{ 'toggle-active mem-active': showModalMem }"
                @click="showModalMem = !showModalMem"
              >
                <span class="toggle-dot bg-cyan"></span>
                <span>RAM %</span>
              </button>

              <button
                type="button"
                class="series-toggle-btn"
                :class="{ 'toggle-active reqs-active': showModalReqs }"
                @click="showModalReqs = !showModalReqs"
              >
                <span class="toggle-dot bg-emerald"></span>
                <span>Throughput (req/s)</span>
              </button>
            </div>
          </div>

          <!-- Expanded SVG Chart Canvas with Hover Crosshair & Tooltip -->
          <div
            class="modal-svg-canvas-box"
            @mousemove="handleModalChartHover"
            @mouseleave="handleModalChartLeave"
          >
            <svg viewBox="0 0 760 220" class="modal-expanded-svg" preserveAspectRatio="none">
              <!-- Defs for Gradients -->
              <defs>
                <linearGradient id="deepCpuGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stop-color="#8b5cf6" stop-opacity="0.35" />
                  <stop offset="100%" stop-color="#8b5cf6" stop-opacity="0.0" />
                </linearGradient>
                <linearGradient id="deepMemGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.30" />
                  <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
                </linearGradient>
                <linearGradient id="deepReqGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stop-color="#10b981" stop-opacity="0.30" />
                  <stop offset="100%" stop-color="#10b981" stop-opacity="0.0" />
                </linearGradient>
              </defs>

              <!-- Horizontal Grid Lines & Left/Right Y-Axis Labels -->
              <!-- 100% -->
              <line x1="50" y1="20" x2="700" y2="20" stroke="rgba(255, 255, 255, 0.08)" stroke-dasharray="3,3" />
              <text x="44" y="24" class="svg-axis-label text-left" text-anchor="end">100%</text>
              <text x="706" y="24" class="svg-axis-label text-right" text-anchor="start">{{ maxModalReqs }} req/s</text>

              <!-- 75% -->
              <line x1="50" y1="62.5" x2="700" y2="62.5" stroke="rgba(255, 255, 255, 0.05)" stroke-dasharray="3,3" />
              <text x="44" y="66.5" class="svg-axis-label text-left" text-anchor="end">75%</text>
              <text x="706" y="66.5" class="svg-axis-label text-right" text-anchor="start">{{ Math.round(maxModalReqs * 0.75) }}</text>

              <!-- 50% -->
              <line x1="50" y1="105" x2="700" y2="105" stroke="rgba(255, 255, 255, 0.05)" stroke-dasharray="3,3" />
              <text x="44" y="109" class="svg-axis-label text-left" text-anchor="end">50%</text>
              <text x="706" y="109" class="svg-axis-label text-right" text-anchor="start">{{ Math.round(maxModalReqs * 0.5) }}</text>

              <!-- 25% -->
              <line x1="50" y1="147.5" x2="700" y2="147.5" stroke="rgba(255, 255, 255, 0.05)" stroke-dasharray="3,3" />
              <text x="44" y="151.5" class="svg-axis-label text-left" text-anchor="end">25%</text>
              <text x="706" y="151.5" class="svg-axis-label text-right" text-anchor="start">{{ Math.round(maxModalReqs * 0.25) }}</text>

              <!-- 0% Baseline -->
              <line x1="50" y1="190" x2="700" y2="190" stroke="rgba(255, 255, 255, 0.15)" stroke-width="1.2" />
              <text x="44" y="194" class="svg-axis-label text-left" text-anchor="end">0%</text>
              <text x="706" y="194" class="svg-axis-label text-right" text-anchor="start">0</text>

              <!-- X-Axis Timestamps -->
              <g class="svg-x-axis-group">
                <text
                  v-for="(marker, mIdx) in modalTimeAxisMarkers"
                  :key="mIdx"
                  :x="marker.x"
                  y="212"
                  class="svg-axis-label text-center font-mono"
                  text-anchor="middle"
                >
                  {{ marker.time }}
                </text>
              </g>

              <!-- Series 1: CPU Area & Line -->
              <g v-if="showModalCpu && modalChartCpuPath">
                <path :d="modalChartCpuArea" fill="url(#deepCpuGrad)" />
                <path :d="modalChartCpuPath" fill="none" stroke="#8b5cf6" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
              </g>

              <!-- Series 2: RAM Area & Line -->
              <g v-if="showModalMem && modalChartMemPath">
                <path :d="modalChartMemArea" fill="url(#deepMemGrad)" />
                <path :d="modalChartMemPath" fill="none" stroke="#06b6d4" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
              </g>

              <!-- Series 3: Throughput Area & Line -->
              <g v-if="showModalReqs && modalChartReqsPath">
                <path :d="modalChartReqsArea" fill="url(#deepReqGrad)" />
                <path :d="modalChartReqsPath" fill="none" stroke="#10b981" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
              </g>

              <!-- Interactive Hover Crosshair & Series Markers -->
              <g v-if="isModalHovered && modalHoverCoords" class="modal-hover-overlay">
                <!-- Vertical Guide Line -->
                <line
                  :x1="modalHoverCoords.x"
                  y1="20"
                  :x2="modalHoverCoords.x"
                  y2="190"
                  stroke="rgba(255, 255, 255, 0.6)"
                  stroke-dasharray="4,3"
                  stroke-width="1.5"
                />

                <!-- CPU Point Marker -->
                <circle
                  v-if="showModalCpu"
                  :cx="modalHoverCoords.x"
                  :cy="modalHoverCoords.yCpu"
                  r="5"
                  fill="#8b5cf6"
                  stroke="#ffffff"
                  stroke-width="2"
                  class="trend-pulse-dot"
                />

                <!-- RAM Point Marker -->
                <circle
                  v-if="showModalMem"
                  :cx="modalHoverCoords.x"
                  :cy="modalHoverCoords.yMem"
                  r="5"
                  fill="#06b6d4"
                  stroke="#ffffff"
                  stroke-width="2"
                  class="trend-pulse-dot"
                />

                <!-- Reqs Point Marker -->
                <circle
                  v-if="showModalReqs"
                  :cx="modalHoverCoords.x"
                  :cy="modalHoverCoords.yReq"
                  r="5"
                  fill="#10b981"
                  stroke="#ffffff"
                  stroke-width="2"
                  class="trend-pulse-dot"
                />
              </g>
            </svg>

            <!-- Expanded Floating Tooltip Box -->
            <div
              v-if="isModalHovered && hoveredModalPoint"
              class="modal-rich-tooltip glass-panel animate-fade-in"
              :style="modalTooltipStyle"
            >
              <div class="trend-tooltip-header">
                <span class="tooltip-time-icon">🕒</span>
                <span class="tooltip-time font-mono">{{ hoveredModalPoint.time }}</span>
                <span class="tooltip-time-badge font-mono" v-if="hoveredModalElapsed">{{ hoveredModalElapsed }}</span>
              </div>
              <div class="trend-tooltip-body">
                <div class="tooltip-row" v-if="showModalCpu">
                  <div class="tooltip-label">
                    <span class="tooltip-dot bg-violet"></span>
                    <span>CPU Saturation:</span>
                  </div>
                  <span class="tooltip-value font-mono text-violet font-bold">{{ hoveredModalPoint.cpu }}%</span>
                </div>
                <div class="tooltip-row" v-if="showModalMem">
                  <div class="tooltip-label">
                    <span class="tooltip-dot bg-cyan"></span>
                    <span>RAM Usage:</span>
                  </div>
                  <span class="tooltip-value font-mono text-cyan font-bold">{{ hoveredModalPoint.mem }}%</span>
                </div>
                <div class="tooltip-row" v-if="showModalReqs">
                  <div class="tooltip-label">
                    <span class="tooltip-dot bg-emerald"></span>
                    <span>Throughput:</span>
                  </div>
                  <span class="tooltip-value font-mono text-emerald font-bold">{{ hoveredModalPoint.reqs.toLocaleString() }} req/s</span>
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- SECTION 3: 📦 Top Traffic & Request-Consuming Services Breakdown -->
        <section class="modal-breakdown-section glass-panel">
          <div class="breakdown-header-bar">
            <div class="breakdown-title-wrap">
              <h4 class="modal-section-title">
                <span>📦 Top Traffic & Request-Consuming Services</span>
              </h4>
              <span class="badge badge-cyan font-mono">
                {{ filteredAndSortedClusterServices.length }} {{ filteredAndSortedClusterServices.length === 1 ? 'SERVICE' : 'SERVICES' }}
              </span>
            </div>

            <!-- Search & Filters -->
            <div class="breakdown-actions-bar">
              <div class="table-search-input-wrap">
                <span class="search-icon">🔍</span>
                <input
                  v-model="modalServiceSearch"
                  type="text"
                  placeholder="Filter service, node, or status..."
                  class="table-search-field"
                />
                <button
                  v-if="modalServiceSearch"
                  class="btn-clear-search"
                  type="button"
                  @click="modalServiceSearch = ''"
                >✕</button>
              </div>
            </div>
          </div>

          <!-- Services Breakdown Table -->
          <div class="breakdown-table-wrapper" v-if="filteredAndSortedClusterServices.length > 0">
            <table class="breakdown-table">
              <thead>
                <tr>
                  <th class="th-svc cursor-pointer" @click="toggleModalSort('name')">
                    <div class="th-content">
                      <span>SERVICE</span>
                      <span class="sort-icon">{{ modalServiceSortBy === 'name' ? (modalServiceSortOrder === 'asc' ? '▲' : '▼') : '↕' }}</span>
                    </div>
                  </th>
                  <th class="th-node cursor-pointer" @click="toggleModalSort('node')">
                    <div class="th-content">
                      <span>HOST NODE</span>
                      <span class="sort-icon">{{ modalServiceSortBy === 'node' ? (modalServiceSortOrder === 'asc' ? '▲' : '▼') : '↕' }}</span>
                    </div>
                  </th>
                  <th class="th-traffic cursor-pointer" @click="toggleModalSort('traffic')">
                    <div class="th-content">
                      <span>TRAFFIC CONTRIBUTION</span>
                      <span class="sort-icon">{{ modalServiceSortBy === 'traffic' ? (modalServiceSortOrder === 'asc' ? '▲' : '▼') : '↕' }}</span>
                    </div>
                  </th>
                  <th class="th-rps cursor-pointer" @click="toggleModalSort('rps')">
                    <div class="th-content">
                      <span>REQ / S</span>
                      <span class="sort-icon">{{ modalServiceSortBy === 'rps' ? (modalServiceSortOrder === 'asc' ? '▲' : '▼') : '↕' }}</span>
                    </div>
                  </th>
                  <th class="th-bw cursor-pointer" @click="toggleModalSort('bandwidth')">
                    <div class="th-content">
                      <span>BANDWIDTH</span>
                      <span class="sort-icon">{{ modalServiceSortBy === 'bandwidth' ? (modalServiceSortOrder === 'asc' ? '▲' : '▼') : '↕' }}</span>
                    </div>
                  </th>
                  <th class="th-res cursor-pointer" @click="toggleModalSort('cpu')">
                    <div class="th-content">
                      <span>CPU & MEMORY</span>
                      <span class="sort-icon">{{ modalServiceSortBy === 'cpu' || modalServiceSortBy === 'mem' ? (modalServiceSortOrder === 'asc' ? '▲' : '▼') : '↕' }}</span>
                    </div>
                  </th>
                  <th class="th-status cursor-pointer" @click="toggleModalSort('status')">
                    <div class="th-content">
                      <span>STATUS</span>
                      <span class="sort-icon">{{ modalServiceSortBy === 'status' ? (modalServiceSortOrder === 'asc' ? '▲' : '▼') : '↕' }}</span>
                    </div>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="svc in filteredAndSortedClusterServices"
                  :key="svc.service_name + '-' + svc.node_name"
                  class="breakdown-row"
                >
                  <!-- 1. Service Name with Dot Badge -->
                  <td class="col-svc">
                    <div class="service-identity">
                      <span
                        class="node-indicator-dot"
                        :class="svc.status === 'healthy' ? 'bg-emerald' : svc.status === 'degraded' ? 'bg-amber' : 'bg-rose'"
                      ></span>
                      <div class="service-name-wrap">
                        <span class="service-title font-mono">{{ svc.service_name }}</span>
                        <span class="service-replica-tag" v-if="svc.container_count > 1">
                          {{ svc.container_count }} replicas
                        </span>
                      </div>
                    </div>
                  </td>

                  <!-- 2. Host Node Name -->
                  <td class="col-node">
                    <div class="node-cell-wrap">
                      <span class="node-server-icon">🖥️</span>
                      <span class="node-server-name font-mono">{{ svc.node_name }}</span>
                    </div>
                  </td>

                  <!-- 3. Traffic Contribution Bar -->
                  <td class="col-traffic">
                    <div class="traffic-bar-cell">
                      <div class="traffic-track">
                        <div
                          class="traffic-fill"
                          :style="{ width: `${Math.max(4, Math.min(100, svc.traffic_percent))}%` }"
                        ></div>
                      </div>
                      <span class="traffic-label font-mono font-bold">{{ svc.traffic_percent.toFixed(1) }}%</span>
                    </div>
                  </td>

                  <!-- 4. Req / s -->
                  <td class="col-rps font-mono">
                    <span
                      class="smooth-value"
                      :class="svc.requests_per_sec > 0 ? 'text-emerald font-bold' : 'text-muted'"
                    >
                      {{ svc.requests_per_sec > 0 ? svc.requests_per_sec.toFixed(1) : '0.0' }}
                    </span>
                    <span class="text-xs text-muted" v-if="svc.requests_per_sec > 0"> rps</span>
                  </td>

                  <!-- 5. Bandwidth (Rx and Tx) -->
                  <td class="col-bw font-mono">
                    <div class="bandwidth-stack">
                      <span class="bw-rx text-cyan" :title="'Live: ' + formatIoRate(svc.rx_bytes_per_sec) + ' | Lifetime Total: ' + formatBytes(svc.total_rx_bytes || 0) + ' received'">↓ {{ formatIoRate(svc.rx_bytes_per_sec) }}</span>
                      <span class="bw-tx text-violet" :title="'Live: ' + formatIoRate(svc.tx_bytes_per_sec) + ' | Lifetime Total: ' + formatBytes(svc.total_tx_bytes || 0) + ' sent'">↑ {{ formatIoRate(svc.tx_bytes_per_sec) }}</span>
                    </div>
                  </td>

                  <!-- 6. CPU & Memory Footprint -->
                  <td class="col-res font-mono">
                    <div class="res-stack">
                      <span
                        class="smooth-value"
                        :class="svc.cpu_percent > 80 ? 'text-rose font-bold' : svc.cpu_percent > 50 ? 'text-amber' : 'text-violet'"
                      >
                        {{ svc.cpu_percent.toFixed(1) }}% CPU
                      </span>
                      <span class="text-cyan text-xs">
                        {{ svc.memory_used_mb >= 1024 ? (svc.memory_used_mb / 1024).toFixed(1) + ' GB' : svc.memory_used_mb.toFixed(0) + ' MB' }}
                      </span>
                    </div>
                  </td>

                  <!-- 7. Status Badge -->
                  <td class="col-status">
                    <span
                      class="badge smooth-value"
                      :class="svc.status === 'healthy' ? 'badge-emerald' : svc.status === 'degraded' ? 'badge-amber' : 'badge-rose'"
                    >
                      {{ svc.status === 'healthy' ? '● HEALTHY' : svc.status === 'degraded' ? '● DEGRADED' : '● DOWN' }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Empty Search State -->
          <div v-else class="breakdown-empty-state">
            <span class="empty-icon">🔍</span>
            <p class="empty-text">No services found matching "{{ modalServiceSearch }}"</p>
            <button class="btn btn-secondary btn-sm" type="button" @click="modalServiceSearch = ''">
              Clear Search Filter
            </button>
          </div>
        </section>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">
          <span>Close Deep-Dive</span>
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
/* ==========================================
   SMOOTH REAL-TIME TRANSITIONS UTILITY CLASSES
   ========================================== */
.smooth-value {
  transition: color 0.5s ease;
}

.smooth-bar {
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
}

.smooth-opacity {
  transition: opacity 0.3s ease;
}

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

/* SECTION 1: Summary HUD (Single-Row Flexbox) */
.summary-hud-row {
  display: flex;
  flex-direction: row;
  align-items: stretch;
  gap: 12px;
  width: 100%;
}

.summary-hud-row .hud-card {
  flex: 1 1 0;
  min-width: 0;
  min-height: 124px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 10px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
  transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              border-color 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              box-shadow 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.summary-hud-row .hud-card:hover {
  border-color: rgba(56, 189, 248, 0.28);
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.25), 0 0 15px rgba(56, 189, 248, 0.1);
}

.hud-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.hud-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-secondary, #94a3b8);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-indicator-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-green { background: #10b981; box-shadow: 0 0 8px rgba(16, 185, 129, 0.6); }
.status-amber { background: #f59e0b; box-shadow: 0 0 8px rgba(245, 158, 11, 0.6); }
.status-red { background: #f43f5e; box-shadow: 0 0 8px rgba(244, 63, 94, 0.6); }

.hud-value-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 6px;
}

.hud-value {
  font-size: 1.6rem;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--text-primary);
  font-family: var(--font-sans);
  line-height: 1.1;
  transition: color 0.5s ease;
}

.hud-total {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted, #64748b);
  letter-spacing: normal;
}

.hud-badge-tag {
  font-size: 10px;
  font-weight: 700;
  font-family: var(--font-mono);
  transition: color 0.5s ease;
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
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
}

.throughput-details {
  display: flex;
  align-items: baseline;
  gap: 3px;
}

.hud-unit {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  font-weight: 600;
}

.sparkline-container {
  width: 76px;
  height: 24px;
  flex-shrink: 0;
}

.sparkline-svg {
  width: 100%;
  height: 100%;
}

.hud-sub-stats {
  font-size: 10.5px;
  color: var(--text-secondary, #94a3b8);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 1px;
}

.sub-stat-sep {
  color: rgba(255, 255, 255, 0.2);
}

.trend-mini-chips {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.mini-chip {
  font-size: 10px;
  padding: 3px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  white-space: nowrap;
  transition: all 0.2s ease;
}

.mini-chip.chip-cpu {
  color: #c4b5fd;
  border-color: rgba(139, 92, 246, 0.25);
  background: rgba(139, 92, 246, 0.08);
}

.mini-chip.chip-mem {
  color: #67e8f9;
  border-color: rgba(6, 182, 212, 0.25);
  background: rgba(6, 182, 212, 0.08);
}

.mini-chip.chip-reqs {
  color: #fde047;
  border-color: rgba(234, 179, 8, 0.25);
  background: rgba(234, 179, 8, 0.08);
}

.stat-sub {
  font-size: 10px;
  font-weight: 500;
  color: var(--text-muted, #94a3b8);
  margin-left: 4px;
}

/* SECTION 1B: LIVE REQUEST FLOW ANIMATION BAR */
.request-flow-bar {
  position: relative;
  overflow: hidden;
  padding: 10px 18px;
  border-radius: 12px;
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.85) 0%, rgba(30, 41, 59, 0.75) 100%);
  border: 1px solid rgba(56, 189, 248, 0.2);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3), 0 0 16px rgba(6, 182, 212, 0.08);
  transition: border-color 0.5s ease, box-shadow 0.5s ease;
}

/* Status Themes */
.request-flow-bar.flow-theme-emerald {
  border-color: rgba(16, 185, 129, 0.35);
  box-shadow: 0 0 20px rgba(16, 185, 129, 0.12), 0 4px 20px rgba(0, 0, 0, 0.3);
}

.request-flow-bar.flow-theme-amber {
  border-color: rgba(245, 158, 11, 0.45);
  box-shadow: 0 0 20px rgba(245, 158, 11, 0.15), 0 4px 20px rgba(0, 0, 0, 0.3);
}

.request-flow-bar.flow-theme-rose {
  border-color: rgba(244, 63, 94, 0.55);
  box-shadow: 0 0 25px rgba(244, 63, 94, 0.2), 0 4px 20px rgba(0, 0, 0, 0.3);
}

/* Particle / Laser Track in Background */
.flow-track-background {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  pointer-events: none;
  overflow: hidden;
}

.flow-stream {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  animation: flowParticleStream var(--flow-duration, 2.5s) linear infinite;
}

.flow-paused {
  animation-play-state: paused !important;
  opacity: 0.25;
}

.flow-dot {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 6px;
  height: 6px;
  border-radius: 50%;
  filter: blur(1px);
}

.flow-theme-emerald .flow-dot {
  background: #10b981;
  box-shadow: 0 0 12px #10b981, 0 0 24px rgba(16, 185, 129, 0.6);
}
.flow-theme-amber .flow-dot {
  background: #f59e0b;
  box-shadow: 0 0 12px #f59e0b, 0 0 24px rgba(245, 158, 11, 0.6);
}
.flow-theme-rose .flow-dot {
  background: #f43f5e;
  box-shadow: 0 0 12px #f43f5e, 0 0 24px rgba(244, 63, 94, 0.6);
}

.flow-dot.dot-1 { left: 0%; opacity: 0.2; }
.flow-dot.dot-2 { left: 20%; opacity: 0.5; }
.flow-dot.dot-3 { left: 40%; opacity: 0.8; }
.flow-dot.dot-4 { left: 60%; opacity: 0.9; }
.flow-dot.dot-5 { left: 80%; opacity: 0.6; }
.flow-dot.dot-6 { left: 100%; opacity: 0.2; }

@keyframes flowParticleStream {
  0% {
    transform: translateX(-30%);
  }
  100% {
    transform: translateX(30%);
  }
}

/* Content Layout */
.flow-bar-content {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
}

.flow-indicator-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.flow-glyphs {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
}

.flow-theme-emerald .flow-glyphs { color: #10b981; }
.flow-theme-amber .flow-glyphs { color: #f59e0b; }
.flow-theme-rose .flow-glyphs { color: #f43f5e; }

.flow-glyph-dot {
  animation: glyphPulse var(--flow-duration, 2.5s) ease-in-out infinite;
}
.flow-glyph-dot.dot-a { animation-delay: 0s; }
.flow-glyph-dot.dot-b { animation-delay: calc(var(--flow-duration, 2.5s) * 0.25); }
.flow-glyph-dot.dot-c { animation-delay: calc(var(--flow-duration, 2.5s) * 0.5); }

.flow-glyph-arrow {
  font-size: 14px;
  font-weight: 800;
  margin-left: 2px;
}

@keyframes glyphPulse {
  0%, 100% {
    opacity: 0.35;
    transform: scale(0.85);
  }
  50% {
    opacity: 1;
    transform: scale(1.2);
    filter: drop-shadow(0 0 6px currentColor);
  }
}

.flow-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 9px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.flow-badge-emerald {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.flow-badge-amber {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.flow-badge-rose {
  background: rgba(244, 63, 94, 0.15);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.3);
}

.flow-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.flow-metrics-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.flow-metric-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 8px;
  padding: 5px 11px;
  transition: all 0.2s ease;
}

.flow-metric-item:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.12);
}

.flow-metric-icon {
  font-size: 13px;
}

.flow-metric-value {
  font-weight: 700;
  font-size: 13px;
  color: var(--text-primary);
}

.flow-metric-label {
  color: var(--text-secondary, #94a3b8);
  font-size: 11px;
}

.flow-metric-item.metric-warning {
  background: rgba(245, 158, 11, 0.08);
  border-color: rgba(245, 158, 11, 0.25);
}

.flow-metric-item.metric-warning .flow-metric-value,
.flow-metric-item.metric-warning .flow-metric-label {
  color: #f59e0b;
}

.flow-metric-item.metric-critical {
  background: rgba(244, 63, 94, 0.08);
  border-color: rgba(244, 63, 94, 0.25);
}

.flow-metric-item.metric-critical .flow-metric-value,
.flow-metric-item.metric-critical .flow-metric-label {
  color: #f43f5e;
}

.flow-metric-divider {
  width: 1px;
  height: 14px;
  background: rgba(255, 255, 255, 0.12);
}

/* Responsive HUD and Flow bar */
@media (max-width: 1100px) {
  .summary-hud-row {
    flex-wrap: wrap;
  }
  .summary-hud-row .hud-card {
    flex: 1 1 calc(33.333% - 12px);
    min-width: 180px;
  }
}

@media (max-width: 700px) {
  .summary-hud-row .hud-card {
    flex: 1 1 calc(50% - 12px);
  }
  .flow-bar-content {
    flex-direction: column;
    align-items: flex-start;
  }
  .flow-metrics-group {
    width: 100%;
    justify-content: space-between;
  }
}

@media (max-width: 480px) {
  .summary-hud-row .hud-card {
    flex: 1 1 100%;
  }
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
  padding: 16px 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  position: relative;
  background: var(--glass-bg, rgba(26, 31, 46, 0.7));
  border: 1px solid var(--border-color, rgba(255, 255, 255, 0.08));
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
  transition: opacity 0.3s ease,
              transform 0.3s ease,
              box-shadow 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              border-color 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: grab;
}

.node-card:hover {
  transform: translateY(-2px);
  border-color: rgba(56, 189, 248, 0.35);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3), 0 0 15px rgba(56, 189, 248, 0.1);
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

/* 2-Row Node Card Header */
.node-card-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.node-header-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}

.node-identity {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.node-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.node-name {
  font-size: 1.1rem;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin: 0;
  line-height: 1.2;
}

.node-header-badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.node-header-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.badge-meta {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.72rem;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-secondary, #94a3b8);
  white-space: nowrap;
}

.node-gauges-cluster {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 10px 6px;
  border-top: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
  background: rgba(0, 0, 0, 0.18);
  border-radius: 10px;
}

.gauge-col {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.node-meta-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.04);
  border-radius: 6px;
  padding: 6px 10px;
}

.meta-label {
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.meta-val {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-primary);
  transition: color 0.5s ease;
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
  gap: 6px;
  padding: 7px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.09);
  color: var(--text-secondary);
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  white-space: nowrap;
}

.btn-node-action:hover {
  background: rgba(255, 255, 255, 0.09);
  color: var(--text-primary);
  border-color: var(--border-accent);
  transform: translateY(-1px);
}

.btn-inspect-node {
  border-color: rgba(6, 182, 212, 0.25);
  color: var(--accent-cyan);
  background: rgba(6, 182, 212, 0.06);
}

.btn-inspect-node:hover {
  background: rgba(6, 182, 212, 0.16);
  border-color: var(--accent-cyan);
  color: #38bdf8;
  box-shadow: 0 0 12px rgba(6, 182, 212, 0.25);
  transform: translateY(-1px);
}

.btn-manage-host {
  border-color: rgba(129, 140, 248, 0.25);
  color: #a5b4fc;
  background: rgba(99, 102, 241, 0.06);
}

.btn-manage-host:hover {
  border-color: #818cf8;
  color: #c7d2fe;
  background: rgba(99, 102, 241, 0.16);
  box-shadow: 0 0 12px rgba(99, 102, 241, 0.25);
  transform: translateY(-1px);
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
  transition: color 0.5s ease;
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
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
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

.trend-card-clickable {
  cursor: pointer;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}

.trend-card-clickable:hover {
  transform: translateY(-2px);
  border-color: rgba(56, 189, 248, 0.4);
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.4), 0 0 20px rgba(56, 189, 248, 0.15);
}

.trend-chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.trend-expand-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  font-size: 10px;
  font-weight: 600;
  color: var(--accent-cyan, #38bdf8);
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.3);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.trend-expand-badge:hover {
  background: rgba(56, 189, 248, 0.25);
  border-color: rgba(56, 189, 248, 0.6);
  color: #fff;
  transform: scale(1.03);
}

.trend-title-wrap {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.trend-chart-subtitle {
  font-size: 11.5px;
  color: var(--text-secondary, #94a3b8);
  letter-spacing: normal;
}

.trend-header-actions {
  display: flex;
  align-items: center;
  gap: 14px;
}

.trend-legend {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.legend-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  user-select: none;
}

.pill-cpu {
  color: #c4b5fd;
  border-color: rgba(139, 92, 246, 0.25);
  background: rgba(139, 92, 246, 0.08);
}

.pill-mem {
  color: #67e8f9;
  border-color: rgba(6, 182, 212, 0.25);
  background: rgba(6, 182, 212, 0.08);
}

.pill-reqs {
  color: #6ee7b7;
  border-color: rgba(16, 185, 129, 0.25);
  background: rgba(16, 185, 129, 0.08);
}

.legend-dot-circle {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

/* HTML-Overlay Grid Layout Chart */
.trend-chart-frame {
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: rgba(8, 14, 26, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  padding: 12px 14px 8px 14px;
  box-shadow: inset 0 2px 12px rgba(0, 0, 0, 0.6);
}

.trend-chart-body {
  display: flex;
  align-items: stretch;
  gap: 10px;
  height: 180px;
  position: relative;
}

.trend-y-axis {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  user-select: none;
  flex-shrink: 0;
}

.y-axis-left {
  width: 38px;
  text-align: right;
}

.y-axis-right {
  min-width: 65px;
  text-align: left;
}

.y-tick {
  font-size: 11px;
  font-weight: 500;
  color: #94a3b8;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.y-tick.tick-accent {
  color: #10b981;
  font-weight: 600;
}

.trend-plot-canvas {
  flex: 1;
  position: relative;
  height: 100%;
  min-width: 0;
}

.trend-plot-svg {
  width: 100%;
  height: 100%;
  display: block;
}

.trend-x-axis {
  display: flex;
  justify-content: space-between;
  margin-left: 48px;
  margin-right: 75px;
  padding-top: 4px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.x-tick {
  font-size: 10.5px;
  font-weight: 500;
  color: #64748b;
  font-variant-numeric: tabular-nums;
}

.trend-hover-point {
  filter: drop-shadow(0 0 6px rgba(255, 255, 255, 0.8));
}

.trend-pulse-dot {
  filter: drop-shadow(0 0 6px rgba(255, 255, 255, 0.8));
}

.trend-rich-tooltip {
  position: absolute;
  z-index: 120;
  min-width: 190px;
  background: rgba(13, 19, 33, 0.96);
  border: 1px solid rgba(56, 189, 248, 0.3);
  border-radius: 10px;
  padding: 8px 12px;
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.7), 0 0 16px rgba(56, 189, 248, 0.2);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  pointer-events: none;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.trend-tooltip-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding-bottom: 5px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
}

.tooltip-time {
  font-weight: 700;
  color: #fff;
}

.tooltip-time-badge {
  font-size: 9px;
  padding: 1px 5px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 4px;
  color: var(--text-muted);
}

.trend-tooltip-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tooltip-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 11px;
}

.tooltip-label {
  display: flex;
  align-items: center;
  gap: 5px;
  color: var(--text-secondary);
}

.tooltip-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
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
  transition: color 0.5s ease;
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
.box-value { font-size: 20px; font-weight: 800; font-family: var(--font-mono); transition: color 0.5s ease; }
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
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
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
  font-size: 1.3rem;
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
  flex-wrap: nowrap;
}

.node-quick-stats {
  display: flex;
  align-items: center;
  gap: 16px;
  background: rgba(0, 0, 0, 0.35);
  padding: 10px 16px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
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
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 12px;
}

.hw-gauge-card {
  padding: 14px 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  transition: all 0.2s ease;
}

.hw-gauge-card:hover {
  border-color: rgba(56, 189, 248, 0.25);
  transform: translateY(-1px);
}

.hw-gauge-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.hw-gauge-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  white-space: nowrap;
}

.hw-gauge-value {
  font-size: 16px;
  font-weight: 800;
  font-family: var(--font-mono);
  transition: color 0.5s ease;
  white-space: nowrap;
}

.hw-net-inline-rates {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  white-space: nowrap;
}

.hw-net-inline-rates .net-sep {
  color: rgba(255, 255, 255, 0.25);
  font-weight: 700;
}

.hw-progress-track {
  width: 100%;
  height: 5px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.hw-progress-track.hw-net-track {
  background: rgba(168, 85, 247, 0.25);
}

.hw-progress-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
}

.hw-gauge-sub {
  font-size: 10.5px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.bw-split {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}

.bw-rx {
  color: #38bdf8;
  font-weight: 600;
}

.bw-tx {
  color: #c084fc;
  font-weight: 600;
}

/* Network Interface Section in Node Drawer */
.network-interfaces-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.interfaces-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}

@media (max-width: 900px) {
  .interfaces-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .interfaces-grid {
    grid-template-columns: 1fr;
  }
}

.iface-card {
  padding: 8px 10px;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  flex-direction: column;
  gap: 6px;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.iface-card:hover {
  border-color: rgba(56, 189, 248, 0.25);
  transform: translateY(-1px);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
}

.iface-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.iface-name-group {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}

.iface-icon {
  font-size: 11px;
}

.iface-name {
  font-size: 11.5px;
  font-weight: 700;
  color: var(--text-primary);
  max-width: 110px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.iface-nic-tag {
  font-size: 8.5px;
  font-weight: 700;
  letter-spacing: 0.05em;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-muted);
  border: 1px solid var(--border-subtle);
}

.iface-rates-stack {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.iface-rate-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 3px 6px;
  border-radius: 6px;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.iface-rate-row.rx-row {
  background: rgba(6, 182, 212, 0.06);
  border: 1px solid rgba(6, 182, 212, 0.15);
}

.iface-rate-row.rx-row:hover {
  background: rgba(6, 182, 212, 0.12);
  border-color: rgba(6, 182, 212, 0.3);
}

.iface-rate-row.tx-row {
  background: rgba(168, 85, 247, 0.06);
  border: 1px solid rgba(168, 85, 247, 0.15);
}

.iface-rate-row.tx-row:hover {
  background: rgba(168, 85, 247, 0.12);
  border-color: rgba(168, 85, 247, 0.3);
}

.rate-badge {
  font-size: 9px;
  font-weight: 700;
  padding: 1px 4px;
  border-radius: 3px;
  white-space: nowrap;
}

.rx-badge {
  background: rgba(6, 182, 212, 0.2);
  color: #38bdf8;
}

.tx-badge {
  background: rgba(168, 85, 247, 0.2);
  color: #c084fc;
}

.rate-val {
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
}

/* Service filter chips and interactive table controls */
.service-filter-chips {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.chip-btn {
  font-size: 11px;
  font-weight: 600;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.2s ease;
}

.chip-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text-primary);
}

.chip-btn.active {
  background: rgba(56, 189, 248, 0.15);
  border-color: rgba(56, 189, 248, 0.4);
  color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.15);
}

.chip-healthy.active {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.4);
  color: #34d399;
}

.chip-degraded.active {
  background: rgba(245, 158, 11, 0.15);
  border-color: rgba(245, 158, 11, 0.4);
  color: #fbbf24;
}

.chip-traffic.active {
  background: rgba(139, 92, 246, 0.15);
  border-color: rgba(139, 92, 246, 0.4);
  color: #c084fc;
}

.sort-indicator {
  font-size: 9px;
  color: #38bdf8;
  margin-left: 3px;
}

.empty-services-row {
  padding: 24px;
  text-align: center;
}

.empty-services-msg {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  font-size: 12px;
}

.btn-clear-inline {
  background: none;
  border: 1px solid rgba(255, 255, 255, 0.12);
  padding: 3px 10px;
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-clear-inline:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
  border-color: var(--border-accent);
}

.node-services-section {
  padding: 18px 20px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.node-processes-section {
  padding: 18px 20px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.proc-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.node-section-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.services-table-wrapper {
  overflow-x: auto;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.2);
}

.services-mini-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}

.services-mini-table thead tr {
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid var(--border-subtle);
}

.services-mini-table th {
  padding: 10px 12px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  white-space: nowrap;
}

.services-mini-table tbody tr {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: background 0.15s ease;
}

.services-mini-table tbody tr:last-child {
  border-bottom: none;
}

.services-mini-table tbody tr:hover {
  background: rgba(56, 189, 248, 0.05);
}

.services-mini-table td {
  padding: 10px 12px;
  vertical-align: middle;
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
.col-svc-tx,
.col-svc-rps,
.col-svc-err {
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
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.proc-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.proc-section-heading {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.proc-section-desc {
  font-size: 12px;
  color: var(--text-muted);
  margin: 0;
}

.proc-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.proc-category-chips {
  display: flex;
  align-items: center;
  gap: 4px;
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
}

.proc-search-box.compact-search {
  flex: 0 1 240px;
  max-width: 240px;
  min-width: 170px;
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

.page-size-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.page-size-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.page-size-pill {
  display: inline-flex;
  align-items: center;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  padding: 2px;
  gap: 2px;
}

.btn-page-size {
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  padding: 3px 9px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  line-height: 1.2;
}

.btn-page-size:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.08);
}

.btn-page-size.active {
  background: rgba(56, 189, 248, 0.18);
  color: #38bdf8;
  border-color: rgba(56, 189, 248, 0.4);
  font-weight: 700;
  box-shadow: 0 0 8px rgba(56, 189, 248, 0.2);
}

.proc-sort-group {
  display: flex;
  align-items: center;
}

.select-proc-sort {
  padding: 8px 14px;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  outline: none;
  transition: all 0.2s ease;
}

.select-proc-sort:focus,
.select-proc-sort:hover {
  border-color: var(--accent-cyan);
  color: var(--text-primary);
  box-shadow: 0 0 10px rgba(6, 182, 212, 0.2);
}

/* Processes Table (Zero Horizontal Scroll Layout) */
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
  padding: 8px 10px;
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
  background: rgba(56, 189, 248, 0.05);
}

.proc-row-hot {
  background: rgba(244, 63, 94, 0.04);
}

.proc-row-hot:hover {
  background: rgba(244, 63, 94, 0.08) !important;
}

.processes-table td {
  padding: 8px 10px;
  vertical-align: middle;
}

.col-proc-pid {
  white-space: nowrap;
  width: 55px;
}

.pid-tag {
  font-size: 11px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 5px;
  border-radius: 4px;
  border: 1px solid var(--border-subtle);
}

.pid-tag-ctr {
  background: rgba(56, 189, 248, 0.15) !important;
  color: #38bdf8 !important;
  border-color: rgba(56, 189, 248, 0.3) !important;
}

.col-proc-app {
  min-width: 110px;
  max-width: 160px;
}

.ctr-icon {
  font-size: 11px;
  margin-right: 3px;
}

.proc-row-container {
  background: rgba(56, 189, 248, 0.02);
}

.proc-app-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  cursor: help;
}

.proc-name {
  font-weight: 700;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
}

.proc-cmd-line {
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 150px;
}

.col-proc-user {
  white-space: nowrap;
  width: 55px;
}

.user-badge {
  font-size: 10px;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.05);
  padding: 1px 5px;
  border-radius: 4px;
}

.col-proc-cpu {
  width: 80px;
  min-width: 75px;
  white-space: nowrap;
}

.proc-cpu-box {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.proc-cpu-val-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.proc-cpu-val {
  font-weight: 700;
  font-family: var(--font-mono);
  font-size: 11px;
  transition: color 0.5s ease;
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
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
}

.col-proc-mem {
  white-space: nowrap;
  width: 85px;
  min-width: 75px;
}

.proc-mem-box {
  display: flex;
  align-items: baseline;
  gap: 3px;
}

.proc-mem-val {
  font-weight: 600;
  color: var(--text-primary);
  font-family: var(--font-mono);
  transition: color 0.5s ease;
  font-size: 11.5px;
}

.proc-mem-pct {
  font-size: 9.5px;
  color: var(--text-muted);
}

.col-proc-rps {
  white-space: nowrap;
  width: 65px;
  font-size: 11.5px;
}

.col-proc-err {
  white-space: nowrap;
  width: 55px;
  font-size: 11.5px;
}

.col-proc-bw {
  white-space: nowrap;
  width: 140px;
}

.col-proc-state {
  white-space: nowrap;
  width: 85px;
  min-width: 80px;
  text-align: right;
}

/* Pagination & Page Size Footer */
.proc-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 10px;
  flex-wrap: wrap;
}

.pagination-info {
  font-size: 11.5px;
  color: var(--text-muted);
}

.pagination-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.btn-page {
  padding: 3px 8px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-page:hover:not(:disabled) {
  background: rgba(56, 189, 248, 0.15);
  border-color: rgba(56, 189, 248, 0.35);
  color: #38bdf8;
}

.btn-page:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.page-indicator {
  font-size: 11px;
  color: var(--text-muted);
  padding: 0 4px;
}

.th-status-right,
.td-status-right {
  text-align: right;
}

/* Empty State */
.proc-empty-state {
  padding: 36px 24px;
  text-align: center;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.4);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px dashed rgba(56, 189, 248, 0.25);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  position: relative;
  overflow: hidden;
}

.radar-glow-container {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  margin-bottom: 4px;
}

.radar-beacon {
  position: absolute;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(6, 182, 212, 0.25) 0%, rgba(6, 182, 212, 0) 70%);
  animation: radar-beacon-pulse 2.4s infinite cubic-bezier(0.4, 0, 0.6, 1);
}

@keyframes radar-beacon-pulse {
  0% {
    transform: scale(0.6);
    opacity: 0.8;
  }
  50% {
    transform: scale(1.4);
    opacity: 0.2;
  }
  100% {
    transform: scale(1.8);
    opacity: 0;
  }
}

.proc-empty-icon {
  font-size: 32px;
  position: relative;
  z-index: 1;
  filter: drop-shadow(0 0 12px rgba(6, 182, 212, 0.6));
  animation: icon-float 3s ease-in-out infinite alternate;
}

@keyframes icon-float {
  0% { transform: translateY(0px); }
  100% { transform: translateY(-4px); }
}

.proc-empty-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
  margin: 0;
}

.proc-empty-desc {
  font-size: 12.5px;
  color: var(--text-secondary);
  max-width: 480px;
  margin: 0;
  line-height: 1.55;
}

.proc-empty-telemetry-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: 100%;
  max-width: 520px;
}

.troubleshooting-hint-box {
  width: 100%;
  text-align: left;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.hint-header {
  display: flex;
  align-items: center;
  gap: 6px;
}

.hint-icon {
  font-size: 14px;
}

.hint-title {
  font-size: 11.5px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.hint-list {
  margin: 0;
  padding-left: 18px;
  font-size: 11.5px;
  color: var(--text-muted);
  line-height: 1.6;
}

.hint-list code {
  font-family: var(--font-mono);
  background: rgba(255, 255, 255, 0.08);
  padding: 1px 5px;
  border-radius: 4px;
  color: var(--accent-cyan);
  font-size: 11px;
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
  transition: color 0.5s ease;
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
  transition: color 0.5s ease;
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
  transition: color 0.5s ease;
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
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
  min-width: 4px;
}

.tps-bar-tx {
  background: linear-gradient(90deg, #8b5cf6, #a855f7);
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
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

/* ==========================================
   CLUSTER TELEMETRY DEEP-DIVE MODAL STYLES
   ========================================== */
.deep-dive-modal-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.modal-summary-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}

.deep-stat-card {
  position: relative;
  overflow: hidden;
  padding: 16px 18px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              border-color 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              box-shadow 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.deep-stat-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: linear-gradient(90deg, #06b6d4, #8b5cf6);
}

.deep-stat-card:hover {
  border-color: rgba(56, 189, 248, 0.35);
  transform: translateY(-1px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3), 0 0 15px rgba(56, 189, 248, 0.1);
}

.deep-stat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.deep-stat-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
  flex: 1;
}

.deep-stat-body {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.deep-stat-value {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.02em;
}

.deep-stat-unit {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
}

.deep-stat-sub {
  font-size: 10.5px;
  color: var(--text-muted);
}

/* Modal Chart Section */
.modal-chart-section {
  padding: 18px 20px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid var(--border-subtle);
}

.modal-chart-top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.modal-section-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
}

.chart-title-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.series-toggles-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.series-toggle-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.2s ease;
}

.series-toggle-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.series-toggle-btn.toggle-active.cpu-active {
  background: rgba(139, 92, 246, 0.15);
  border-color: rgba(139, 92, 246, 0.5);
  color: #c4b5fd;
}

.series-toggle-btn.toggle-active.mem-active {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.5);
  color: #67e8f9;
}

.series-toggle-btn.toggle-active.reqs-active {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.5);
  color: #6ee7b7;
}

.toggle-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.modal-svg-canvas-box {
  position: relative;
  width: 100%;
  height: 220px;
  background: rgba(0, 0, 0, 0.35);
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.04);
}

.modal-expanded-svg {
  width: 100%;
  height: 100%;
  display: block;
}

.svg-axis-label {
  font-size: 10px;
  fill: var(--text-muted, #94a3b8);
  font-family: var(--font-mono, monospace);
  font-weight: 500;
}

.modal-rich-tooltip {
  position: absolute;
  z-index: 120;
  min-width: 220px;
  background: rgba(13, 19, 33, 0.97);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 10px;
  padding: 10px 14px;
  box-shadow: 0 20px 45px rgba(0, 0, 0, 0.8), 0 0 20px rgba(56, 189, 248, 0.2);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  pointer-events: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* Modal Breakdown Table Section */
.modal-breakdown-section {
  padding: 18px 20px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid var(--border-subtle);
}

.breakdown-header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.breakdown-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.breakdown-actions-bar {
  display: flex;
  align-items: center;
  gap: 10px;
}

.table-search-input-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.table-search-input-wrap .search-icon {
  position: absolute;
  left: 10px;
  font-size: 12px;
  color: var(--text-muted);
  pointer-events: none;
}

.table-search-field {
  padding: 6px 30px 6px 30px;
  font-size: 12px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  color: #fff;
  min-width: 240px;
  outline: none;
  transition: all 0.2s ease;
}

.table-search-field:focus {
  border-color: rgba(56, 189, 248, 0.5);
  box-shadow: 0 0 12px rgba(56, 189, 248, 0.2);
}

.btn-clear-search {
  position: absolute;
  right: 8px;
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 11px;
  cursor: pointer;
}

.btn-clear-search:hover {
  color: #fff;
}

.breakdown-table-wrapper {
  overflow-x: auto;
  border-radius: 10px;
  border: 1px solid var(--border-subtle);
  background: rgba(0, 0, 0, 0.25);
}

.breakdown-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}

.breakdown-table thead tr {
  background: rgba(15, 23, 42, 0.8);
  border-bottom: 1px solid var(--border-subtle);
}

.breakdown-table th {
  padding: 10px 14px;
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.05em;
  user-select: none;
}

.breakdown-table th.cursor-pointer:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.03);
}

.th-content {
  display: flex;
  align-items: center;
  gap: 6px;
}

.sort-icon {
  font-size: 10px;
  opacity: 0.6;
}

.breakdown-row {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: background 0.15s ease;
}

.breakdown-row:hover {
  background: rgba(56, 189, 248, 0.05);
}

.breakdown-table td {
  padding: 10px 14px;
  vertical-align: middle;
}

.service-identity {
  display: flex;
  align-items: center;
  gap: 8px;
}

.service-name-wrap {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.service-title {
  font-weight: 700;
  color: #fff;
}

.service-replica-tag {
  font-size: 10px;
  color: var(--text-muted);
}

.node-cell-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-secondary);
}

.traffic-bar-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 140px;
}

.traffic-track {
  flex: 1;
  height: 7px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 999px;
  overflow: hidden;
}

.traffic-fill {
  height: 100%;
  background: linear-gradient(90deg, #06b6d4, #8b5cf6);
  border-radius: 999px;
  transition: width 0.5s ease;
}

.traffic-label {
  font-size: 11px;
  color: #38bdf8;
  width: 44px;
  text-align: right;
}

.bandwidth-stack,
.res-stack {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 11.5px;
  font-family: var(--font-mono, monospace);
}

.breakdown-empty-state {
  padding: 32px 16px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  color: var(--text-muted);
}

@media (max-width: 900px) {
  .modal-summary-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 580px) {
  .modal-summary-grid {
    grid-template-columns: 1fr;
  }
}

/* ========================================== */
/* DRAWER MODE TABS & HISTORICAL TELEMETRY    */
/* ========================================== */

.drawer-mode-tabs {
  display: flex;
  gap: 8px;
  background: rgba(15, 23, 42, 0.6);
  padding: 4px;
  border-radius: 10px;
  border: 1px solid var(--border-subtle);
}

.mode-tab-btn {
  flex: 1;
  padding: 8px 14px;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  text-align: center;
}

.mode-tab-btn:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.05);
}

.mode-tab-btn.active {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.3);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.node-history-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.hist-range-selector {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-radius: 12px;
  flex-wrap: wrap;
  gap: 10px;
}

.range-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.range-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
}

.range-pills {
  display: flex;
  gap: 6px;
}

.btn-range-pill {
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  border: 1px solid var(--border-subtle);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-range-pill:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}

.btn-range-pill.active {
  background: #38bdf8;
  color: #0f172a;
  border-color: #38bdf8;
  font-weight: 700;
}

.range-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.hist-kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

@media (max-width: 768px) {
  .hist-kpi-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.hist-kpi-card {
  padding: 14px 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hist-kpi-card .kpi-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.hist-kpi-card .kpi-val {
  font-size: 20px;
  font-weight: 800;
  font-family: monospace;
}

.hist-kpi-card .kpi-sub {
  font-size: 11px;
  color: var(--text-muted);
}

.hist-kpi-empty {
  padding: 14px 18px;
  border-radius: 12px;
}

.empty-kpi-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.empty-kpi-icon {
  font-size: 24px;
  opacity: 0.85;
}

.empty-kpi-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.empty-kpi-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-primary);
}

.empty-kpi-desc {
  font-size: 11px;
  color: var(--text-muted);
  line-height: 1.4;
}

.hist-chart-wrapper {
  padding: 16px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.hist-chart-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 180px;
  border-radius: 10px;
  color: var(--text-muted);
  font-size: 13px;
}

.hist-chart-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 36px 20px;
  background: rgba(10, 15, 30, 0.4);
  border-radius: 10px;
  border: 1px dashed var(--border-subtle);
  min-height: 190px;
  gap: 8px;
}

.empty-chart-illustration {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 4px;
}

.empty-chart-icon {
  font-size: 26px;
}

.empty-pulse-badge {
  font-size: 11px;
  font-family: monospace;
  font-weight: 600;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.3);
  padding: 2px 8px;
  border-radius: 9999px;
}

.empty-chart-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
  margin: 0;
}

.empty-chart-desc {
  font-size: 11px;
  color: var(--text-muted);
  max-width: 480px;
  margin: 0;
  line-height: 1.5;
}

.hist-chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
}

.chart-title-group h4 {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.chart-title-group .hist-chart-desc {
  font-size: 11px;
  color: var(--text-muted);
}

.hist-chart-legend {
  display: flex;
  gap: 10px;
}

.legend-pill {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-secondary);
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.hist-chart-container {
  position: relative;
  width: 100%;
  height: 220px;
  background: rgba(10, 15, 30, 0.6);
  border-radius: 10px;
  border: 1px solid var(--border-subtle);
  overflow: hidden;
}

.hist-loading-overlay {
  position: absolute;
  inset: 0;
  background: rgba(10, 15, 30, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--text-muted);
  font-size: 13px;
  z-index: 10;
}

.hist-y-axis {
  position: absolute;
  top: 15px;
  bottom: 30px;
  left: 8px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  pointer-events: none;
}

.hist-y-axis .y-tick {
  font-size: 9px;
  font-family: monospace;
  color: var(--text-muted);
  opacity: 0.6;
}

.hist-svg-canvas {
  width: 100%;
  height: 100%;
  cursor: crosshair;
}

.hist-x-axis {
  position: absolute;
  left: 50px;
  right: 60px;
  bottom: 4px;
  display: flex;
  justify-content: space-between;
  pointer-events: none;
}

.hist-x-axis .x-tick {
  font-size: 9px;
  font-family: monospace;
  color: var(--text-muted);
}

.hist-rich-tooltip {
  position: absolute;
  background: rgba(15, 23, 42, 0.95);
  border: 1px solid rgba(56, 189, 248, 0.4);
  backdrop-filter: blur(12px);
  padding: 10px 14px;
  border-radius: 8px;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.6);
  z-index: 20;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 190px;
}

.tooltip-time-header {
  font-size: 11px;
  font-weight: 600;
  color: #e2e8f0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  padding-bottom: 4px;
  margin-bottom: 2px;
}

.tooltip-series-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
}

.tooltip-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.tooltip-label {
  color: var(--text-muted);
}

.tooltip-val {
  font-family: monospace;
  margin-left: auto;
}

/* Incidents Section */
.node-incidents-section {
  padding: 16px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.incidents-section-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.inc-title-group {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.inc-title-group h4 {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.inc-desc {
  font-size: 11px;
  color: var(--text-muted);
  margin: 0;
}

.incidents-table-wrapper {
  overflow-x: auto;
}

.incidents-mini-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.incidents-mini-table th {
  text-align: left;
  padding: 8px 10px;
  color: var(--text-muted);
  font-size: 10px;
  text-transform: uppercase;
  border-bottom: 1px solid var(--border-subtle);
}

.incidents-mini-table td {
  padding: 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.inc-type-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.inc-type-name {
  font-size: 11px;
  color: var(--text-primary);
}

.inc-msg-text {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}

.inc-meta-tags {
  display: flex;
  gap: 6px;
  margin-top: 4px;
}

.inc-tag {
  font-size: 10px;
  background: rgba(255, 255, 255, 0.06);
  padding: 1px 6px;
  border-radius: 4px;
  color: var(--text-muted);
}

.inc-empty-state {
  text-align: center;
  padding: 24px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  color: var(--text-muted);
}

.inc-empty-icon {
  font-size: 24px;
}

.inc-empty-state h5 {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
  margin: 0;
}

.inc-empty-state p {
  font-size: 11px;
  margin: 0;
  max-width: 400px;
}
</style>


