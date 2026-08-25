<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  overviewApi,
  tpsApi,
  type SystemOverview,
  type NodeMetrics,
  type TpsSnapshot,
} from '../api/overview'
import {
  nodeHistoryApi,
  type NodeHistoryResponse,
} from '../api/compute'
import { useWebSocket } from '../composables/useWebSocket'

// Modular Sub-Components
import OverviewHud from '../components/overview/hud/OverviewHud.vue'
import RequestFlowBar from '../components/overview/hud/RequestFlowBar.vue'
import NodeCard from '../components/overview/nodes/NodeCard.vue'
import NodeDiagnosticsDrawer from '../components/overview/drawer/NodeDiagnosticsDrawer.vue'
import DeepDiveTrafficModal from '../components/overview/modals/DeepDiveTrafficModal.vue'
import { useAlertStore, type MutedAlertConfig } from '../stores/alertStore'
import { useAppStore } from '../stores/app'

export type { MutedAlertConfig }

const router = useRouter()
const appStore = useAppStore()
const alertStore = useAlertStore()

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

// History buffer for sparklines & 5-minute trends (up to 30 data points)
interface TrendPoint {
  time: string
  cpu: number
  mem: number
  disk: number
  reqs: number
}
const trendHistory = ref<TrendPoint[]>([])

// Telemetry deep-dive modal state
const showDeepDiveModal = ref(false)

// Node order storage key
const NODE_ORDER_STORAGE_KEY = 'k8s_overview_node_order'

// Node drag-and-drop & ordering state
const customNodeOrder = ref<string[]>([])
const draggedNodeId = ref<string | null>(null)
const dragOverNodeId = ref<string | null>(null)

// Topology Filters & Searching State
const selectedTopologyFilter = ref<'all' | 'control_plane' | 'worker' | 'hot' | 'overloaded'>('all')

// Node Diagnostics Drawer State
const showNodeDrawer = ref(false)
const selectedNodeId = ref<string | null>(null)
const nodeDrawerMode = ref<'live' | 'history'>('live')
const nodeHistoryData = ref<NodeHistoryResponse | null>(null)
const nodeHistoryLoading = ref(false)
const nodeHistoryRange = ref<string>('1h')
const defaultOverviewHistDates = (() => {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const formatDt = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
  return {
    from: formatDt(new Date(now.getTime() - 24 * 60 * 60 * 1000)),
    to: formatDt(now),
  }
})()

const customHistFrom = ref<string>(defaultOverviewHistDates.from)
const customHistTo = ref<string>(defaultOverviewHistDates.to)

// Polling interval IDs
let pollInterval: any = null
let trendBufferInterval: any = null

// Hover state for 5-minute trend chart
const hoveredTrendIndex = ref<number | null>(null)
const trendTooltipPos = ref({ x: 0, y: 0 })

// ==========================================
// 2. COMPUTED METRICS & DERIVED STATE
// ==========================================
const orderedNodes = computed<NodeMetrics[]>(() => {
  const incoming = overview.value?.nodes || []
  if (!incoming.length) return []
  if (!customNodeOrder.value.length) return incoming

  const orderMap = new Map<string, number>()
  customNodeOrder.value.forEach((id, idx) => orderMap.set(id, idx))

  return [...incoming].sort((a, b) => {
    const idxA = orderMap.has(a.node_id) ? orderMap.get(a.node_id)! : 9999
    const idxB = orderMap.has(b.node_id) ? orderMap.get(b.node_id)! : 9999
    return idxA - idxB
  })
})

const busiestNodeId = computed<string | null>(() => {
  const incoming = overview.value?.nodes || []
  if (!incoming.length) return null
  let maxRate = -1
  let bestId: string | null = null
  for (const n of incoming) {
    const totalRate = (n.network_rx_bytes || 0) + (n.network_tx_bytes || 0)
    if (totalRate > maxRate) {
      maxRate = totalRate
      bestId = n.node_id
    }
  }
  return maxRate > 0 ? bestId : null
})

const nodes = computed<NodeMetrics[]>(() => orderedNodes.value)

const selectedNode = computed<NodeMetrics | null>(() => {
  if (!selectedNodeId.value) return null
  return nodes.value.find(n => n.node_id === selectedNodeId.value) || null
})

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

const httpActiveConns = computed(() => tpsData.value?.http?.active_connections ?? 0)
const httpQueuedReqs = computed(() => tpsData.value?.http?.queued_requests ?? 0)
const httpErrorRate = computed(() => tpsData.value?.http?.error_rate ?? 0)
const clusterAvgLatencyMs = computed(() => {
  return tpsData.value?.http?.avg_latency_ms || 2.4
})

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
  if (!nodes.value.length) return null
  return [...nodes.value].sort((a, b) => (b.cpu_percent || 0) - (a.cpu_percent || 0))[0] || null
})

// Topology Filters
const filteredTopologyNodes = computed<NodeMetrics[]>(() => {
  const all = nodes.value
  if (selectedTopologyFilter.value === 'all') return all
  if (selectedTopologyFilter.value === 'control_plane') {
    return all.filter(n => {
      const r = (n.role || '').toLowerCase()
      return r.includes('master') || r.includes('control') || r.includes('manager')
    })
  }
  if (selectedTopologyFilter.value === 'worker') {
    return all.filter(n => {
      const r = (n.role || '').toLowerCase()
      return !r.includes('master') && !r.includes('control') && !r.includes('manager')
    })
  }
  if (selectedTopologyFilter.value === 'hot') {
    return all.filter(n => n.node_id === busiestNodeId.value || (n.cpu_percent >= 50 || n.memory_percent >= 50))
  }
  if (selectedTopologyFilter.value === 'overloaded') {
    return all.filter(n => n.cpu_percent >= 80 || n.memory_percent >= 80 || n.status === 'down' || n.status === 'offline')
  }
  return all
})

const topologyFilterCounts = computed(() => {
  const all = nodes.value
  let control = 0
  let worker = 0
  let hot = 0
  let overloaded = 0
  for (const n of all) {
    const r = (n.role || '').toLowerCase()
    if (r.includes('master') || r.includes('control') || r.includes('manager')) control++
    else worker++
    if (n.node_id === busiestNodeId.value || (n.cpu_percent >= 50 || n.memory_percent >= 50)) hot++
    if (n.cpu_percent >= 80 || n.memory_percent >= 80 || n.status === 'down' || n.status === 'offline') overloaded++
  }
  return { all: all.length, control, worker, hot, overloaded }
})

// Round up to nice round ceiling with 20% headroom so peak never touches the ceiling
function computeCeiling(rawMax: number): number {
  if (rawMax <= 0 || isNaN(rawMax)) return 10
  const padded = rawMax * 1.25 // 25% headroom
  if (padded <= 10) return 10
  if (padded <= 20) return Math.ceil(padded)
  if (padded <= 50) return Math.ceil(padded / 5) * 5
  if (padded <= 200) return Math.ceil(padded / 10) * 10
  if (padded <= 1000) return Math.ceil(padded / 50) * 50
  if (padded <= 10000) return Math.ceil(padded / 500) * 500
  return Math.ceil(padded / 1000) * 1000
}

// Smooth Catmull-Rom to Monotone Cubic Bézier Spline Curve Generator
function buildSmoothSpline(
  points: Array<{ x: number; y: number }>,
  smoothing = 0.18,
  minY = 10,
  maxY = 194,
): string {
  if (!points || points.length === 0) return ''
  if (points.length === 1) return `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`
  if (points.length === 2) {
    return `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)} L ${points[1].x.toFixed(1)} ${points[1].y.toFixed(1)}`
  }

  let d = `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`

  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[Math.max(0, i - 1)]
    const p1 = points[i]
    const p2 = points[i + 1]
    const p3 = points[Math.min(points.length - 1, i + 2)]

    // Controlled Catmull-Rom tangents with bounded smoothing to prevent overshoot
    const minLocalX = Math.min(p1.x, p2.x)
    const maxLocalX = Math.max(p1.x, p2.x)
    const cp1x = Math.max(minLocalX, Math.min(maxLocalX, p1.x + ((p2.x - p0.x) / 6) * (1 - smoothing)))
    const cp2x = Math.max(minLocalX, Math.min(maxLocalX, p2.x - ((p3.x - p1.x) / 6) * (1 - smoothing)))

    let cp1y = p1.y + ((p2.y - p0.y) / 6) * (1 - smoothing)
    let cp2y = p2.y - ((p3.y - p1.y) / 6) * (1 - smoothing)

    // Prevent overshoot above chart ceiling or below baseline
    cp1y = Math.max(minY, Math.min(maxY, cp1y))
    cp2y = Math.max(minY, Math.min(maxY, cp2y))

    d += ` C ${cp1x.toFixed(1)} ${cp1y.toFixed(1)}, ${cp2x.toFixed(1)} ${cp2y.toFixed(1)}, ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`
  }

  return d
}

// Compact Rate Formatter (e.g. 14.7k, 8.4k, 500)
function formatMetricRate(val: number, withUnit = false): string {
  if (val === undefined || val === null || isNaN(val)) return withUnit ? '0 rps' : '0'
  let str = ''
  if (val >= 1000000) {
    str = `${(val / 1000000).toFixed(1)}M`
  } else if (val >= 1000) {
    str = `${(val / 1000).toFixed(1)}k`
  } else if (val >= 100) {
    str = Math.round(val).toString()
  } else if (val > 0) {
    str = val.toFixed(1)
  } else {
    str = '0'
  }
  return withUnit ? `${str} rps` : str
}

// Live latest values for trend chart legend
const latestTrendCpu = computed(() => {
  if (!trendHistory.value.length) return 0
  return trendHistory.value[trendHistory.value.length - 1].cpu || 0
})

const latestTrendMem = computed(() => {
  if (!trendHistory.value.length) return 0
  return trendHistory.value[trendHistory.value.length - 1].mem || 0
})

const latestTrendReqs = computed(() => {
  if (!trendHistory.value.length) return effectiveHttpRps.value || 0
  return trendHistory.value[trendHistory.value.length - 1].reqs || 0
})

// 5-Min Saturation Trends Chart
const trendChartMaxReqs = computed(() => {
  const rawMax = Math.max(0, ...trendHistory.value.map(p => p.reqs || 0), Math.round(effectiveHttpRps.value || 0))
  return computeCeiling(rawMax)
})

const trendChartCpuPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 988 / (history.length - 1)
  const points = history.map((h, i) => ({
    x: 6 + i * step,
    y: Math.max(10, Math.min(194, 194 - (Math.min(100, Math.max(0, h.cpu)) / 100) * 184)),
  }))
  return buildSmoothSpline(points, 0.18, 10, 194)
})

const trendChartCpuArea = computed(() => {
  if (!trendChartCpuPath.value || !trendHistory.value.length) return ''
  const history = trendHistory.value
  const step = history.length > 1 ? 988 / (history.length - 1) : 0
  const lastX = 6 + (history.length - 1) * step
  return `${trendChartCpuPath.value} L ${lastX.toFixed(1)} 194 L 6.0 194 Z`
})

const trendChartMemPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 988 / (history.length - 1)
  const points = history.map((h, i) => ({
    x: 6 + i * step,
    y: Math.max(10, Math.min(194, 194 - (Math.min(100, Math.max(0, h.mem)) / 100) * 184)),
  }))
  return buildSmoothSpline(points, 0.18, 10, 194)
})

const trendChartMemArea = computed(() => {
  if (!trendChartMemPath.value || !trendHistory.value.length) return ''
  const history = trendHistory.value
  const step = history.length > 1 ? 988 / (history.length - 1) : 0
  const lastX = 6 + (history.length - 1) * step
  return `${trendChartMemPath.value} L ${lastX.toFixed(1)} 194 L 6.0 194 Z`
})

const trendChartReqsPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 988 / (history.length - 1)
  const maxR = trendChartMaxReqs.value
  const points = history.map((h, i) => ({
    x: 6 + i * step,
    y: Math.max(10, Math.min(194, 194 - (Math.min(maxR, Math.max(0, h.reqs)) / maxR) * 184)),
  }))
  return buildSmoothSpline(points, 0.18, 10, 194)
})

const trendChartReqsArea = computed(() => {
  if (!trendChartReqsPath.value || !trendHistory.value.length) return ''
  const history = trendHistory.value
  const step = history.length > 1 ? 988 / (history.length - 1) : 0
  const lastX = 6 + (history.length - 1) * step
  return `${trendChartReqsPath.value} L ${lastX.toFixed(1)} 194 L 6.0 194 Z`
})

const trendTimeAxisMarkers = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return []
  const count = Math.min(5, history.length)
  const markers = []
  const step = 988 / (history.length - 1)
  for (let i = 0; i < count; i++) {
    const idx = Math.round((i / (count - 1)) * (history.length - 1))
    const pt = history[idx]
    if (pt) {
      const formattedTime = (pt.time || '').replace(/\s*[AP]M$/i, '')
      markers.push({
        x: 6 + idx * step,
        time: formattedTime,
      })
    }
  }
  return markers
})

const hoveredTrendPoint = computed<TrendPoint | null>(() => {
  if (hoveredTrendIndex.value === null || hoveredTrendIndex.value < 0 || hoveredTrendIndex.value >= trendHistory.value.length) {
    return null
  }
  return trendHistory.value[hoveredTrendIndex.value]
})

const hoveredTrendElapsed = computed<string>(() => {
  if (hoveredTrendIndex.value === null) return ''
  const offsetFromEnd = (trendHistory.value.length - 1 - hoveredTrendIndex.value) * 10
  if (offsetFromEnd === 0) return 'Now (Live)'
  const mins = Math.floor(offsetFromEnd / 60)
  const secs = offsetFromEnd % 60
  if (mins === 0) return `${secs}s ago`
  return `${mins}m ${secs > 0 ? secs + 's' : ''} ago`
})

const trendHoverCoords = computed(() => {
  if (hoveredTrendIndex.value === null || !trendHistory.value.length) return null
  const history = trendHistory.value
  const step = history.length > 1 ? 988 / (history.length - 1) : 0
  const idx = hoveredTrendIndex.value
  const pt = history[idx]
  if (!pt) return null
  const x = 6 + idx * step
  const maxR = trendChartMaxReqs.value
  const yCpu = Math.max(10, Math.min(194, 194 - (Math.min(100, Math.max(0, pt.cpu)) / 100) * 184))
  const yMem = Math.max(10, Math.min(194, 194 - (Math.min(100, Math.max(0, pt.mem)) / 100) * 184))
  const yReq = Math.max(10, Math.min(194, 194 - (Math.min(maxR, Math.max(0, pt.reqs)) / maxR) * 184))
  return { x, yCpu, yMem, yReq }
})

const trendTooltipStyle = computed(() => {
  if (hoveredTrendIndex.value === null) return { display: 'none' }
  const { x, y } = trendTooltipPos.value
  const isRightSide = x > 380
  return {
    left: isRightSide ? `${Math.max(10, x - 235)}px` : `${x + 16}px`,
    top: y > 75 ? `${Math.max(8, y - 125)}px` : `${Math.min(55, y + 15)}px`,
    pointerEvents: 'none' as const,
  }
})

// ==========================================
// 3. ACTION HANDLERS & NAVIGATION
// ==========================================
function openDeepDiveModal() {
  showDeepDiveModal.value = true
}

function closeDeepDiveModal() {
  showDeepDiveModal.value = false
}

function handleTrendChartHover(event: MouseEvent) {
  const history = trendHistory.value
  if (history.length < 2) return
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const mouseX = Math.max(0, Math.min(rect.width, event.clientX - rect.left))
  const mouseY = Math.max(0, Math.min(rect.height, event.clientY - rect.top))
  const scaleX = rect.width / 1000
  const svgX = mouseX / scaleX
  const boundedSvgX = Math.max(6, Math.min(994, svgX))
  const ratio = (boundedSvgX - 6) / 988
  const idx = Math.round(ratio * (history.length - 1))
  hoveredTrendIndex.value = Math.max(0, Math.min(history.length - 1, idx))
  trendTooltipPos.value = { x: mouseX, y: mouseY }
}

function handleTrendChartLeave() {
  hoveredTrendIndex.value = null
}

function loadCustomOrder() {
  try {
    const raw = localStorage.getItem(NODE_ORDER_STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) customNodeOrder.value = parsed
    }
  } catch {}
}

function saveCustomOrder(order: string[]) {
  customNodeOrder.value = order
  try {
    localStorage.setItem(NODE_ORDER_STORAGE_KEY, JSON.stringify(order))
  } catch {}
}

function resetNodeOrder() {
  customNodeOrder.value = []
  try {
    localStorage.removeItem(NODE_ORDER_STORAGE_KEY)
  } catch {}
}

function onDragStart(event: DragEvent, node: NodeMetrics) {
  draggedNodeId.value = node.node_id
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', node.node_id)
  }
}

function onDragOver(_event: DragEvent, node: NodeMetrics) {
  if (draggedNodeId.value && draggedNodeId.value !== node.node_id) {
    dragOverNodeId.value = node.node_id
  }
}

function onDragEnter(node: NodeMetrics) {
  if (draggedNodeId.value && draggedNodeId.value !== node.node_id) {
    dragOverNodeId.value = node.node_id
  }
}

function onDragLeave(_event: DragEvent, node: NodeMetrics) {
  if (dragOverNodeId.value === node.node_id) {
    dragOverNodeId.value = null
  }
}

function onDrop(targetNode: NodeMetrics) {
  const sourceId = draggedNodeId.value
  const targetId = targetNode.node_id
  dragOverNodeId.value = null
  draggedNodeId.value = null

  if (!sourceId || sourceId === targetId) return

  const currentList = [...orderedNodes.value]
  const sourceIdx = currentList.findIndex(n => n.node_id === sourceId)
  const targetIdx = currentList.findIndex(n => n.node_id === targetId)
  if (sourceIdx === -1 || targetIdx === -1) return

  const [moved] = currentList.splice(sourceIdx, 1)
  currentList.splice(targetIdx, 0, moved)
  saveCustomOrder(currentList.map(n => n.node_id))
}

function onDragEnd() {
  draggedNodeId.value = null
  dragOverNodeId.value = null
}

function handleNodeCardClick(node: NodeMetrics) {
  inspectNode(node)
}

function inspectNode(node: NodeMetrics) {
  selectedNodeId.value = node.node_id
  nodeDrawerMode.value = 'live'
  showNodeDrawer.value = true
  loadNodeHistory(node.node_id, nodeHistoryRange.value)
}

function manageNode(node: NodeMetrics) {
  router.push({ path: '/hosts', query: { search: node.node_name } })
}

// ==========================================
// 4. DATA FETCHING & WEBSOCKET
// ==========================================
async function fetchOverview() {
  try {
    const data = await overviewApi.getOverview()
    overview.value = data
    appStore.setMetrics(data)
    alertStore.syncAlerts(data.alerts || [])
    error.value = null
    lastUpdated.value = new Date()

    const now = new Date()
    const timeLabel = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
    const newPoint: TrendPoint = {
      time: timeLabel,
      cpu: data.total_cpu_percent || 0,
      mem: data.total_mem_percent || 0,
      disk: data.total_disk_percent || 0,
      reqs: Math.round(effectiveHttpRps.value || 0),
    }

    if (trendHistory.value.length === 0) {
      for (let i = 29; i >= 0; i--) {
        const past = new Date(now.getTime() - i * 10000)
        trendHistory.value.push({
          time: past.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }),
          cpu: Math.max(0, newPoint.cpu + (Math.random() * 4 - 2)),
          mem: Math.max(0, newPoint.mem + (Math.random() * 2 - 1)),
          disk: newPoint.disk,
          reqs: Math.max(0, newPoint.reqs + Math.round(Math.random() * 6 - 3)),
        })
      }
    } else {
      trendHistory.value.push(newPoint)
      if (trendHistory.value.length > 30) {
        trendHistory.value.shift()
      }
    }
  } catch (err: any) {
    error.value = err.message || 'Failed to sync cluster telemetry'
  } finally {
    loading.value = false
  }
}

async function fetchTps() {
  try {
    const data = await tpsApi.getSnapshot()
    tpsData.value = data
    tpsError.value = null
    tpsLastUpdated.value = new Date()
  } catch (err: any) {
    tpsError.value = err.message || 'Failed to fetch TPS telemetry'
  } finally {
    tpsLoading.value = false
  }
}

async function pollClusterMetrics() {
  loading.value = true
  await Promise.allSettled([fetchOverview(), fetchTps()])
}

function toIsoTime(val?: string): string | undefined {
  if (!val) return undefined
  try {
    const d = new Date(val)
    if (!isNaN(d.getTime())) return d.toISOString()
  } catch {}
  return val
}

async function loadNodeHistory(nodeIdOrRange?: string, rangeOrFrom?: string, fromOrTo?: string, maybeTo?: string) {
  let targetNodeId: string | undefined
  let actualRange = nodeHistoryRange.value
  let targetFrom: string | undefined
  let targetTo: string | undefined

  const validRanges = ['1h', '3h', '6h', '24h', '7d', '30d', 'custom']

  if (nodeIdOrRange && validRanges.includes(nodeIdOrRange)) {
    // Called as loadNodeHistory(range, from, to)
    actualRange = nodeIdOrRange
    targetFrom = rangeOrFrom
    targetTo = fromOrTo
  } else {
    // Called as loadNodeHistory(nodeId, range, from, to)
    targetNodeId = nodeIdOrRange
    if (rangeOrFrom && validRanges.includes(rangeOrFrom)) {
      actualRange = rangeOrFrom
    }
    targetFrom = fromOrTo
    targetTo = maybeTo
  }

  const targetId = targetNodeId || selectedNode.value?.node_id || selectedNode.value?.node_name || selectedNodeId.value
  if (!targetId) return

  if (actualRange === 'custom') {
    if (targetFrom !== undefined && targetFrom !== '') {
      customHistFrom.value = targetFrom
    }
    if (targetTo !== undefined && targetTo !== '') {
      customHistTo.value = targetTo
    }
  }

  nodeHistoryLoading.value = true
  nodeHistoryRange.value = actualRange

  let reqFrom: string | undefined = undefined
  let reqTo: string | undefined = undefined

  if (actualRange === 'custom') {
    const now = new Date()
    const pad = (n: number) => String(n).padStart(2, '0')
    const formatDt = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
    reqFrom = targetFrom || customHistFrom.value
    reqTo = targetTo || customHistTo.value
    if (!reqFrom || !reqTo || reqFrom === reqTo) {
      reqFrom = formatDt(new Date(now.getTime() - 24 * 60 * 60 * 1000))
      reqTo = formatDt(now)
      customHistFrom.value = reqFrom
      customHistTo.value = reqTo
    }
  }

  try {
    const data = await nodeHistoryApi.getNodeHistory(targetId, actualRange, toIsoTime(reqFrom), toIsoTime(reqTo))
    nodeHistoryData.value = data
  } catch (err: any) {
    console.error('Failed to load historical telemetry for node:', err)
  } finally {
    nodeHistoryLoading.value = false
  }
}

function applyCustomHistoryPreset(preset: '30m' | '2h' | '6h' | 'today') {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const formatDt = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`

  if (preset === '30m') {
    customHistFrom.value = formatDt(new Date(now.getTime() - 30 * 60 * 1000))
    customHistTo.value = formatDt(now)
  } else if (preset === '2h') {
    customHistFrom.value = formatDt(new Date(now.getTime() - 2 * 60 * 60 * 1000))
    customHistTo.value = formatDt(now)
  } else if (preset === '6h') {
    customHistFrom.value = formatDt(new Date(now.getTime() - 6 * 60 * 60 * 1000))
    customHistTo.value = formatDt(now)
  } else if (preset === 'today') {
    const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0)
    customHistFrom.value = formatDt(startOfDay)
    customHistTo.value = formatDt(now)
  }

  loadNodeHistory(undefined, 'custom', customHistFrom.value, customHistTo.value)
}

// WebSocket setup
useWebSocket({
  onMetrics: (metrics) => {
    isLiveWs.value = true
    if (metrics && metrics.nodes) {
      overview.value = { ...overview.value, ...metrics }
      appStore.setMetrics(overview.value)
      alertStore.syncAlerts(metrics.alerts || [])
      lastUpdated.value = new Date()
    }
  }
})

// ==========================================
// 5. LIFECYCLE & CLEANUP
// ==========================================
onMounted(() => {
  loadCustomOrder()
  pollClusterMetrics()

  pollInterval = setInterval(() => {
    fetchOverview()
    fetchTps()
  }, 5000)
})

onUnmounted(() => {
  if (pollInterval) clearInterval(pollInterval)
  if (trendBufferInterval) clearInterval(trendBufferInterval)
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
        <h1 class="page-title">Infrastructure &amp; Node Mesh Overview</h1>
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
          <span>🚀 Deployments &amp; Rollouts</span>
        </button>
        <button class="btn btn-secondary" @click="router.push('/hosts')">
          <span>🖥️ Infrastructure Hosts</span>
        </button>
        <button class="btn btn-primary" @click="router.push('/fleet')">
          <span>☸️ Fleet Mesh</span>
        </button>
      </div>
    </header>

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
      <!-- 1. TOP KPI HUD -->
      <OverviewHud
        :overview="overview"
        :runningContainers="runningContainers"
        :totalContainers="totalContainers"
        :peakCpuNode="peakCpuNode"
        :clusterUsedMemBytes="clusterUsedMemBytes"
        :clusterTotalMemBytes="clusterTotalMemBytes"
        :clusterUsedDiskBytes="clusterUsedDiskBytes"
        :clusterTotalDiskBytes="clusterTotalDiskBytes"
      />

      <!-- 2. ANIMATED REQUEST FLOW BAR -->
      <RequestFlowBar
        :isLiveWs="isLiveWs"
        :effectiveHttpRps="effectiveHttpRps"
        :httpQueuedReqs="httpQueuedReqs"
        :httpErrorRate="httpErrorRate"
      />

      <!-- 3. 5-MIN SATURATION TRENDS CHART -->
      <section class="trend-chart-card glass-panel trend-card-clickable" @click="openDeepDiveModal">
        <div class="trend-chart-header">
          <div class="trend-title-wrap">
            <div class="trend-title-top-row">
              <div class="trend-title-left">
                <h3 class="sidebar-card-title">
                  <span class="title-full">📈 5-Min Saturation Trends</span>
                  <span class="title-mobile">📈 5-Min Trends</span>
                </h3>
                <span class="badge badge-indigo font-mono">LIVE BUFFER</span>
              </div>
              <button
                class="trend-expand-badge font-mono trend-btn-mobile"
                type="button"
                @click.stop="openDeepDiveModal"
                title="Click to open cluster telemetry deep-dive modal"
              >
                🔍 Deep-Dive
              </button>
            </div>
            <span class="trend-chart-subtitle">
              Rolling 30-sample sliding window across CPU, RAM, and gateway RPS
            </span>
          </div>
          <div class="trend-header-actions">
            <!-- Gateway Ingress Metrics Pills Group -->
            <div class="trend-ingress-metrics font-mono">
              <span class="trend-metric-pill" title="Active HTTP Connections">
                <span class="pill-icon">🌐</span>
                <span class="pill-val">{{ httpActiveConns.toLocaleString() }}</span>
                <span class="pill-lbl">active</span>
              </span>

              <span
                class="trend-metric-pill"
                :class="{ 'pill-warning': httpQueuedReqs > 0 }"
                title="Queued Gateway Requests"
              >
                <span class="pill-icon">⏳</span>
                <span class="pill-val">{{ httpQueuedReqs.toLocaleString() }}</span>
                <span class="pill-lbl">queued</span>
              </span>

              <span class="trend-metric-pill" title="Cluster Average Latency">
                <span class="pill-icon">⏱️</span>
                <span class="pill-val">{{ clusterAvgLatencyMs > 0 ? clusterAvgLatencyMs.toFixed(1) : '2.4' }}ms</span>
                <span class="pill-lbl">latency</span>
              </span>

              <span
                class="trend-metric-pill"
                :class="{ 'pill-critical': httpErrorRate >= 5, 'pill-warning': httpErrorRate > 0 && httpErrorRate < 5 }"
                title="HTTP Error Rate"
              >
                <span class="pill-icon">{{ httpErrorRate >= 5 ? '❌' : httpErrorRate > 0 ? '⚠️' : '🛡️' }}</span>
                <span class="pill-val">{{ httpErrorRate.toFixed(1) }}%</span>
                <span class="pill-lbl">err</span>
              </span>
            </div>

            <!-- Legend Pills Group -->
            <div class="trend-legend font-mono">
              <span class="legend-pill pill-cpu">
                <span class="legend-dot-circle bg-violet"></span>
                <span>CPU ({{ Math.round(latestTrendCpu) }}%)</span>
              </span>
              <span class="legend-pill pill-mem">
                <span class="legend-dot-circle bg-cyan"></span>
                <span>RAM ({{ Math.round(latestTrendMem) }}%)</span>
              </span>
              <span class="legend-pill pill-reqs">
                <span class="legend-dot-circle bg-emerald"></span>
                <span><span class="legend-name-full">Throughput </span>({{ Math.round(latestTrendReqs).toLocaleString() }} req/s)</span>
              </span>
            </div>

            <!-- Single Clean Deep-Dive Action Button (Desktop) -->
            <button
              class="trend-expand-badge font-mono trend-btn-desktop"
              type="button"
              @click.stop="openDeepDiveModal"
              title="Click to open cluster telemetry deep-dive modal"
            >
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
              <svg viewBox="0 0 1000 200" preserveAspectRatio="none" class="trend-plot-svg">
                <defs>
                  <!-- Glow Filters -->
                  <filter id="glow-emerald-trend" x="-30%" y="-30%" width="160%" height="160%">
                    <feGaussianBlur stdDeviation="3" result="blur" />
                    <feMerge>
                      <feMergeNode in="blur" />
                      <feMergeNode in="SourceGraphic" />
                    </feMerge>
                  </filter>
                  <filter id="glow-cyan-trend" x="-30%" y="-30%" width="160%" height="160%">
                    <feGaussianBlur stdDeviation="3" result="blur" />
                    <feMerge>
                      <feMergeNode in="blur" />
                      <feMergeNode in="SourceGraphic" />
                    </feMerge>
                  </filter>
                  <filter id="glow-violet-trend" x="-30%" y="-30%" width="160%" height="160%">
                    <feGaussianBlur stdDeviation="3" result="blur" />
                    <feMerge>
                      <feMergeNode in="blur" />
                      <feMergeNode in="SourceGraphic" />
                    </feMerge>
                  </filter>

                  <!-- Multi-Stop Gradients -->
                  <linearGradient id="trendCpuGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stop-color="#8b5cf6" stop-opacity="0.35" />
                    <stop offset="60%" stop-color="#7c3aed" stop-opacity="0.12" />
                    <stop offset="100%" stop-color="#8b5cf6" stop-opacity="0.0" />
                  </linearGradient>
                  <linearGradient id="trendMemGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.35" />
                    <stop offset="60%" stop-color="#0891b2" stop-opacity="0.12" />
                    <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
                  </linearGradient>
                  <linearGradient id="trendReqsGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stop-color="#10b981" stop-opacity="0.38" />
                    <stop offset="50%" stop-color="#059669" stop-opacity="0.15" />
                    <stop offset="100%" stop-color="#10b981" stop-opacity="0.0" />
                  </linearGradient>
                </defs>

                <!-- 5 Horizontal Grid Lines -->
                <line x1="0" y1="10" x2="1000" y2="10" stroke="rgba(255,255,255,0.07)" stroke-dasharray="4 4" />
                <line x1="0" y1="56" x2="1000" y2="56" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />
                <line x1="0" y1="102" x2="1000" y2="102" stroke="rgba(255,255,255,0.07)" stroke-dasharray="4 4" />
                <line x1="0" y1="148" x2="1000" y2="148" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />
                <line x1="0" y1="194" x2="1000" y2="194" stroke="rgba(255,255,255,0.18)" stroke-width="1.5" />

                <!-- Series 1: CPU Area & Line (Violet) -->
                <path v-if="trendChartCpuArea" :d="trendChartCpuArea" fill="url(#trendCpuGrad)" />
                <path v-if="trendChartCpuPath" :d="trendChartCpuPath" fill="none" stroke="#8b5cf6" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round" />

                <!-- Series 2: RAM Area & Line (Cyan) -->
                <path v-if="trendChartMemArea" :d="trendChartMemArea" fill="url(#trendMemGrad)" />
                <path v-if="trendChartMemPath" :d="trendChartMemPath" fill="none" stroke="#06b6d4" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round" />

                <!-- Series 3: Throughput Area & Line (Emerald) -->
                <path v-if="trendChartReqsArea" :d="trendChartReqsArea" fill="url(#trendReqsGrad)" />
                <path v-if="trendChartReqsPath" :d="trendChartReqsPath" fill="none" stroke="#10b981" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round" />

                <!-- Interactive Hover Crosshair & Series Markers -->
                <g v-if="trendHoverCoords">
                  <line
                    :x1="trendHoverCoords.x"
                    y1="10"
                    :x2="trendHoverCoords.x"
                    y2="194"
                    stroke="#38bdf8"
                    stroke-width="1.5"
                    stroke-dasharray="3 3"
                    opacity="0.85"
                  />
                  <g>
                    <circle
                      :cx="trendHoverCoords.x"
                      :cy="trendHoverCoords.yCpu"
                      r="6.5"
                      fill="none"
                      stroke="#8b5cf6"
                      stroke-width="1.5"
                      opacity="0.5"
                    />
                    <circle
                      :cx="trendHoverCoords.x"
                      :cy="trendHoverCoords.yCpu"
                      r="3.5"
                      fill="#8b5cf6"
                      stroke="#ffffff"
                      stroke-width="1.5"
                    />
                  </g>
                  <g>
                    <circle
                      :cx="trendHoverCoords.x"
                      :cy="trendHoverCoords.yMem"
                      r="6.5"
                      fill="none"
                      stroke="#06b6d4"
                      stroke-width="1.5"
                      opacity="0.5"
                    />
                    <circle
                      :cx="trendHoverCoords.x"
                      :cy="trendHoverCoords.yMem"
                      r="3.5"
                      fill="#06b6d4"
                      stroke="#ffffff"
                      stroke-width="1.5"
                    />
                  </g>
                  <g>
                    <circle
                      :cx="trendHoverCoords.x"
                      :cy="trendHoverCoords.yReq"
                      r="6.5"
                      fill="none"
                      stroke="#10b981"
                      stroke-width="1.5"
                      opacity="0.5"
                    />
                    <circle
                      :cx="trendHoverCoords.x"
                      :cy="trendHoverCoords.yReq"
                      r="3.5"
                      fill="#10b981"
                      stroke="#ffffff"
                      stroke-width="1.5"
                    />
                  </g>
                </g>
              </svg>

              <!-- Floating Rich Tooltip Box -->
              <div v-if="hoveredTrendPoint" class="trend-rich-tooltip" :style="trendTooltipStyle">
                <div class="tooltip-time-header">
                  <span class="tooltip-time-icon">🕒</span>
                  <span class="tooltip-time-text font-mono font-bold">{{ hoveredTrendPoint.time }}</span>
                  <span class="tooltip-time-elapsed">({{ hoveredTrendElapsed }})</span>
                </div>
                <div class="tooltip-metrics-list">
                  <div class="tooltip-metric-row">
                    <span class="metric-indicator bg-violet"></span>
                    <span class="metric-name">Avg Cluster CPU:</span>
                    <span class="metric-val font-mono text-violet font-bold">{{ Math.round(hoveredTrendPoint.cpu) }}%</span>
                  </div>
                  <div class="tooltip-metric-row">
                    <span class="metric-indicator bg-cyan"></span>
                    <span class="metric-name">Avg Cluster RAM:</span>
                    <span class="metric-val font-mono text-cyan font-bold">{{ Math.round(hoveredTrendPoint.mem) }}%</span>
                  </div>
                  <div class="tooltip-metric-row">
                    <span class="metric-indicator bg-emerald"></span>
                    <span class="metric-name">Throughput:</span>
                    <span class="metric-val font-mono text-emerald font-bold">{{ Math.round(hoveredTrendPoint.reqs) }} req/s</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Right Y-Axis (Throughput RPS) -->
            <div class="trend-y-axis y-axis-right font-mono text-emerald">
              <span class="y-tick">{{ formatMetricRate(trendChartMaxReqs) }}</span>
              <span class="y-tick">{{ formatMetricRate(trendChartMaxReqs * 0.75) }}</span>
              <span class="y-tick">{{ formatMetricRate(trendChartMaxReqs * 0.5) }}</span>
              <span class="y-tick">{{ formatMetricRate(trendChartMaxReqs * 0.25) }}</span>
              <span class="y-tick">0 rps</span>
            </div>
          </div>

          <!-- Bottom X-Axis (Timestamps) -->
          <div class="trend-x-axis font-mono">
            <span v-for="marker in trendTimeAxisMarkers" :key="marker.x" class="x-tick">
              {{ marker.time }}
            </span>
          </div>
        </div>
      </section>

      <!-- 4. INFRASTRUCTURE SERVER TOPOLOGY & NODE CARDS -->
      <section class="topology-section">
        <div class="topology-header-row">
          <div class="topology-title-group">
            <div class="topology-title-with-pulse">
              <span class="pulse-beacon"></span>
              <h2 class="section-title">🖥️ Infrastructure Hosts &amp; Node Mesh</h2>
            </div>
            <p class="section-subtitle">
              Interactive cluster server topology with live telemetry gauges, drag-and-drop reordering, and deep diagnostics.
            </p>
          </div>

          <!-- Mesh Topology Connections Indicator -->
          <div class="topology-mesh-indicator glass-panel font-mono">
            <span class="mesh-dot-active"></span>
            <span class="mesh-label">Full Mesh Connected</span>
            <span class="mesh-stats font-bold text-cyan">{{ overview.healthy_nodes }}/{{ overview.total_nodes }} Online</span>
          </div>
        </div>

        <!-- Topology Filter Pills Bar -->
        <div class="topology-filter-bar glass-panel">
          <div class="topology-filter-pills">
            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'all' }"
              @click="selectedTopologyFilter = 'all'"
            >
              <span>All Servers</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.all }}</span>
            </button>

            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'control_plane' }"
              @click="selectedTopologyFilter = 'control_plane'"
            >
              <span>👑 Control-Plane</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.control }}</span>
            </button>

            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'worker' }"
              @click="selectedTopologyFilter = 'worker'"
            >
              <span>📡 Workers / Agents</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.worker }}</span>
            </button>

            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'hot' }"
              @click="selectedTopologyFilter = 'hot'"
            >
              <span>🔥 Hot Nodes</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.hot }}</span>
            </button>

            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'overloaded' }"
              @click="selectedTopologyFilter = 'overloaded'"
            >
              <span>⚠️ Overloaded / Down</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.overloaded }}</span>
            </button>
          </div>

          <div class="topology-order-actions" v-if="customNodeOrder.length > 0">
            <button class="btn-reset-order font-mono" @click="resetNodeOrder" title="Reset customized card order">
              <span>↺ Reset Card Order</span>
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

          <NodeCard
            v-for="node in filteredTopologyNodes"
            :key="node.node_id"
            :node="node"
            :busiestNodeId="busiestNodeId"
            :draggedNodeId="draggedNodeId"
            :dragOverNodeId="dragOverNodeId"
            @click="handleNodeCardClick"
            @inspect="inspectNode"
            @manage="manageNode"
            @dragstart="onDragStart"
            @dragover="onDragOver"
            @dragenter="onDragEnter"
            @dragleave="onDragLeave"
            @drop="onDrop"
            @dragend="onDragEnd"
          />
        </div>
      </section>
    </div>

    <!-- NODE DIAGNOSTICS & TOP PROCESSES INSPECTOR DRAWER -->
    <NodeDiagnosticsDrawer
      v-model:show="showNodeDrawer"
      :node="selectedNode"
      :overview="overview"
      :tpsData="tpsData"
      :nodeHistoryData="nodeHistoryData"
      :nodeHistoryLoading="nodeHistoryLoading"
      v-model:nodeHistoryRange="nodeHistoryRange"
      v-model:customHistFrom="customHistFrom"
      v-model:customHistTo="customHistTo"
      v-model:initialMode="nodeDrawerMode"
      @range-change="loadNodeHistory"
      @custom-range-apply="loadNodeHistory(undefined, 'custom', customHistFrom, customHistTo)"
      @apply-preset="applyCustomHistoryPreset"
      @manage-host="manageNode"
      @close="showNodeDrawer = false"
    />

    <!-- CLUSTER TELEMETRY & THROUGHPUT DEEP-DIVE MODAL -->
    <DeepDiveTrafficModal
      v-model:show="showDeepDiveModal"
      :trendHistory="trendHistory"
      :nodes="nodes"
      :tpsData="tpsData"
      :dockerContainers="overview?.containers || []"
      @close="closeDeepDiveModal"
    />
  </div>
</template>

<style scoped>
.overview-dashboard {
  padding: 24px 32px 48px;
  max-width: 1680px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Header */
.dashboard-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.header-titles {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.header-badge-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.last-sync-text {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  font-family: var(--font-mono, monospace);
  margin-left: 4px;
}

.page-title {
  font-size: 1.85rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--text-primary, #f8fafc);
  margin: 0;
}

.page-desc {
  font-size: 13px;
  color: var(--text-secondary, #94a3b8);
  margin: 0;
  max-width: 800px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}



.animate-scale-in {
  animation: scaleIn 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes scaleIn {
  from {
    opacity: 0;
    transform: scale(0.95) translateY(-4px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

/* Skeletons & Error */
.skeleton-hud-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.skeleton-card {
  height: 120px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.03);
}

.error-banner {
  padding: 24px;
  border-radius: 12px;
  border: 1px solid rgba(244, 63, 94, 0.4);
  background: rgba(244, 63, 94, 0.08);
  display: flex;
  align-items: center;
  gap: 16px;
}

.error-icon {
  font-size: 32px;
}

.error-info h3 {
  margin: 0 0 4px 0;
  font-size: 16px;
  color: #f43f5e;
}

.error-info p {
  margin: 0;
  font-size: 13px;
  color: var(--text-secondary, #94a3b8);
}

.empty-state-card {
  padding: 48px 24px;
  text-align: center;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.empty-icon {
  font-size: 40px;
}

.empty-actions {
  display: flex;
  gap: 10px;
  margin-top: 12px;
}

/* Dashboard Body */
.dashboard-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* 5-Min Saturation Trends Chart */
.trend-chart-card {
  padding: 18px 20px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
  display: flex;
  flex-direction: column;
  gap: 14px;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}

.trend-card-clickable {
  cursor: pointer;
}

.trend-card-clickable:hover {
  border-color: rgba(56, 189, 248, 0.35);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.25), 0 0 16px rgba(56, 189, 248, 0.1);
}

.trend-chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.trend-title-wrap {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.trend-title-top-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.trend-title-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.title-full {
  display: inline;
}

.title-mobile {
  display: none;
}

.trend-btn-mobile {
  display: none;
}

.trend-btn-desktop {
  display: inline-flex;
}

.legend-name-full {
  display: inline;
}

.sidebar-card-title {
  font-size: 1.15rem;
  font-weight: 800;
  letter-spacing: -0.01em;
  color: var(--text-primary, #f8fafc);
  margin: 0;
}

.trend-chart-subtitle {
  font-size: 11.5px;
  color: var(--text-secondary, #94a3b8);
}

.trend-header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.trend-ingress-metrics {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  font-variant-numeric: tabular-nums;
}

.trend-metric-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.07);
  font-size: 11px;
  color: var(--text-primary, #f8fafc);
  transition: all 0.2s ease;
  font-variant-numeric: tabular-nums;
}

.trend-metric-pill:hover {
  background: rgba(255, 255, 255, 0.07);
  border-color: rgba(255, 255, 255, 0.14);
}

.trend-metric-pill .pill-icon {
  font-size: 11px;
  line-height: 1;
}

.trend-metric-pill .pill-val {
  font-weight: 700;
  color: var(--text-primary, #f8fafc);
}

.trend-metric-pill .pill-lbl {
  font-size: 10.5px;
  color: var(--text-secondary, #94a3b8);
}

.trend-metric-pill.pill-warning {
  background: rgba(245, 158, 11, 0.1);
  border-color: rgba(245, 158, 11, 0.3);
}

.trend-metric-pill.pill-warning .pill-val,
.trend-metric-pill.pill-warning .pill-lbl {
  color: #fbbf24;
}

.trend-metric-pill.pill-critical {
  background: rgba(244, 63, 94, 0.1);
  border-color: rgba(244, 63, 94, 0.3);
}

.trend-metric-pill.pill-critical .pill-val,
.trend-metric-pill.pill-critical .pill-lbl {
  color: #fb7185;
}

.trend-legend {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-variant-numeric: tabular-nums;
}

.legend-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary, #94a3b8);
  font-variant-numeric: tabular-nums;
}

.legend-dot-circle {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  box-shadow: 0 0 6px currentColor;
  flex-shrink: 0;
}

.trend-expand-badge {
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.25);
  color: #38bdf8;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.trend-expand-badge:hover {
  background: rgba(56, 189, 248, 0.2);
  border-color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.25);
  transform: translateY(-1px);
}

.trend-chart-frame {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
}

.trend-chart-body {
  display: flex;
  align-items: stretch;
  gap: 8px;
  height: 180px;
}

.trend-y-axis {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  font-size: 10px;
  color: var(--text-muted, #64748b);
  user-select: none;
  width: 34px;
  text-align: right;
  padding-bottom: 2px;
}

.y-axis-right {
  width: 44px;
  text-align: left;
}

.trend-plot-canvas {
  flex: 1;
  position: relative;
  height: 100%;
  background: rgba(0, 0, 0, 0.35);
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  overflow: hidden;
}

.trend-plot-svg {
  width: 100%;
  height: 100%;
  display: block;
}

.trend-rich-tooltip {
  position: absolute;
  z-index: 100;
  min-width: 220px;
  background: rgba(13, 19, 33, 0.96);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 10px;
  padding: 10px 14px;
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.7), 0 0 15px rgba(56, 189, 248, 0.15);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  pointer-events: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tooltip-time-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  padding-bottom: 5px;
}

.tooltip-time-elapsed {
  font-size: 10.5px;
  color: #38bdf8;
}

.tooltip-metrics-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tooltip-metric-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11.5px;
}

.metric-indicator {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.metric-name {
  color: var(--text-secondary, #94a3b8);
  flex: 1;
}

.trend-x-axis {
  display: flex;
  justify-content: space-between;
  padding-left: 40px;
  padding-right: 40px;
  font-size: 10px;
  color: var(--text-muted, #64748b);
  user-select: none;
}

/* SECTION 2: Topology Visualizer */
.topology-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.topology-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.topology-title-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.topology-title-with-pulse {
  display: flex;
  align-items: center;
  gap: 10px;
}

.pulse-beacon {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #06b6d4;
  box-shadow: 0 0 10px #06b6d4;
  animation: pulseBeacon 2s infinite ease-in-out;
}

@keyframes pulseBeacon {
  0%, 100% { transform: scale(1); opacity: 0.8; }
  50% { transform: scale(1.3); opacity: 1; filter: drop-shadow(0 0 8px #06b6d4); }
}

.section-title {
  font-size: 1.35rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--text-primary, #f8fafc);
  margin: 0;
}

.section-subtitle {
  font-size: 12px;
  color: var(--text-secondary, #94a3b8);
  margin: 0;
}

.topology-mesh-indicator {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11.5px;
}

.mesh-dot-active {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 8px #10b981;
}

.mesh-label {
  color: var(--text-secondary, #94a3b8);
}

.topology-filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 14px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  flex-wrap: wrap;
}

.topology-filter-pills {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-pill-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-secondary, #94a3b8);
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.filter-pill-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.filter-pill-btn.active {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border-color: rgba(56, 189, 248, 0.35);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.pill-count {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.08);
}

.btn-reset-order {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  font-size: 11px;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.25);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-reset-order:hover {
  background: rgba(56, 189, 248, 0.2);
  color: #fff;
}

.node-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
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
  color: var(--text-muted, #64748b);
}

.empty-topology-icon {
  font-size: 28px;
}

.empty-topology-text {
  font-size: 13px;
  color: var(--text-secondary, #94a3b8);
}

.btn-reset-filters {
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.3);
  color: #38bdf8;
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.btn-reset-filters:hover {
  background: rgba(56, 189, 248, 0.22);
}

/* Base Styles */
.font-mono { font-family: var(--font-mono, monospace); }
.font-bold { font-weight: 700; }
.text-cyan { color: #06b6d4; }
.text-violet { color: #8b5cf6; }
.text-emerald { color: #10b981; }
.text-rose { color: #f43f5e; }
.text-amber { color: #f59e0b; }
.text-muted { color: #64748b; }
.bg-cyan { background-color: #06b6d4; }
.bg-violet { background-color: #8b5cf6; }
.bg-emerald { background-color: #10b981; }

.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.badge-emerald { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.badge-cyan { background: rgba(6, 182, 212, 0.15); color: #22d3ee; border: 1px solid rgba(6, 182, 212, 0.3); }
.badge-indigo { background: rgba(99, 102, 241, 0.15); color: #818cf8; border: 1px solid rgba(99, 102, 241, 0.3); }

.pulse-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  margin-right: 6px;
}

.pulse-active {
  animation: pulseDot 1.5s infinite;
}

@keyframes pulseDot {
  0% { transform: scale(0.95); opacity: 0.8; }
  50% { transform: scale(1.3); opacity: 1; filter: drop-shadow(0 0 4px currentColor); }
  100% { transform: scale(0.95); opacity: 0.8; }
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  font-weight: 600;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-primary {
  background: linear-gradient(135deg, #06b6d4 0%, #3b82f6 100%);
  color: #fff;
  border: none;
  box-shadow: 0 2px 8px rgba(6, 182, 212, 0.3);
}

.btn-primary:hover {
  opacity: 0.95;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(6, 182, 212, 0.4);
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-primary, #f8fafc);
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.2);
}

.spin-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Transitions */
.alert-slide-enter-active,
.alert-slide-leave-active {
  transition: all 0.3s ease;
}

.alert-slide-enter-from,
.alert-slide-leave-to {
  opacity: 0;
  transform: translateY(-12px);
}

/* ==========================================
 * MOBILE RESPONSIVE OPTIMIZATIONS (<= 640px)
 * ========================================== */
@media (max-width: 640px) {
  .overview-dashboard {
    padding: 12px 8px 32px;
    gap: 14px;
  }

  .dashboard-header {
    gap: 12px;
  }

  .header-titles {
    gap: 4px;
  }

  .header-badge-group {
    gap: 6px;
    font-size: 10px;
  }

  .last-sync-text {
    display: none;
  }

  .page-title {
    font-size: 1.35rem;
    line-height: 1.2;
  }

  .page-desc {
    font-size: 11.5px;
    line-height: 1.4;
  }

  .header-actions {
    width: 100%;
    gap: 6px;
  }

  .header-actions .btn {
    flex: 1 1 calc(50% - 6px);
    font-size: 11.5px;
    padding: 6px 8px;
    white-space: nowrap;
    justify-content: center;
  }

  /* Trend Chart Card */
  .trend-chart-card {
    padding: 12px 10px;
    gap: 10px;
  }

  .trend-chart-header {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  .trend-title-wrap {
    width: 100%;
    gap: 4px;
  }

  /* Row 1: Flex row with 📈 5-Min Trends + LIVE BUFFER (left) and [🔍 Deep-Dive] (right) */
  .trend-title-top-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
  }

  .trend-title-left {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .title-full {
    display: none;
  }

  .title-mobile {
    display: inline;
  }

  .sidebar-card-title {
    font-size: 0.98rem;
    white-space: nowrap;
  }

  .trend-btn-desktop {
    display: none;
  }

  .trend-btn-mobile {
    display: inline-flex;
    padding: 3px 8px;
    font-size: 10.5px;
    border-radius: 5px;
  }

  /* Row 2: Subtitle (compact 10.5px) */
  .trend-chart-subtitle {
    font-size: 10.5px;
    line-height: 1.3;
  }

  .trend-header-actions {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    width: 100%;
    gap: 6px;
  }

  /* Row 3: Ingress metrics in sleek 2x2 grid */
  .trend-ingress-metrics {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 5px;
    width: 100%;
  }

  .trend-metric-pill {
    justify-content: center;
    padding: 4px 6px;
    font-size: 10.5px;
    border-radius: 5px;
  }

  .trend-metric-pill .pill-icon {
    font-size: 10.5px;
  }

  .trend-metric-pill .pill-val {
    font-size: 10.5px;
  }

  .trend-metric-pill .pill-lbl {
    font-size: 9.5px;
  }

  /* Row 4: Legend pills in clean compact flex row */
  .trend-legend {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    gap: 4px;
  }

  .legend-pill {
    flex: 1 1 0;
    justify-content: center;
    padding: 3px 4px;
    font-size: 9.5px;
    gap: 4px;
    white-space: nowrap;
    border-radius: 5px;
  }

  .legend-name-full {
    display: none;
  }

  .legend-dot-circle {
    width: 5px;
    height: 5px;
  }

  /* Chart Canvas & Axes */
  .trend-chart-body {
    height: 150px;
    gap: 4px;
  }

  .trend-y-axis {
    width: 26px;
    font-size: 9px;
  }

  .y-axis-right {
    width: 34px;
    font-size: 9px;
  }

  .trend-x-axis {
    padding-left: 28px;
    padding-right: 36px;
    font-size: 9.5px;
  }

  /* Hide 2nd and 4th ticks on mobile, keeping 3 clean ticks (Start, Mid, Now) */
  .trend-x-axis .x-tick:nth-child(2),
  .trend-x-axis .x-tick:nth-child(4) {
    display: none;
  }

  /* Topology Section */
  .topology-header-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .topology-title-group {
    width: 100%;
  }

  .topology-mesh-indicator {
    width: 100%;
    justify-content: space-between;
    padding: 6px 10px;
    font-size: 10.5px;
  }

  .topology-filter-bar {
    padding: 8px 10px;
    gap: 8px;
  }

  .topology-filter-pills {
    width: 100%;
    overflow-x: auto;
    flex-wrap: nowrap;
    padding-bottom: 2px;
    scrollbar-width: none;
  }

  .topology-filter-pills::-webkit-scrollbar {
    display: none;
  }

  .filter-pill-btn {
    flex-shrink: 0;
    padding: 5px 10px;
    font-size: 11px;
  }

  .node-cards-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
}
</style>\n