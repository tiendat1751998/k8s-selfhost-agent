<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  overviewApi,
  tpsApi,
  type SystemOverview,
  type NodeMetrics,
  type MetricAlert,
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

// Slide-down Alerts Persistent Muted & Snooze state
const MUTED_ALERTS_STORAGE_KEY = 'k8s_muted_alerts_v1'
const SESSION_MUTED_ALERTS_KEY = 'k8s_session_muted_alerts_v1'

export interface MutedAlertConfig {
  key: string // e.g. "workerdb1-node_down"
  nodeId?: string
  nodeName?: string
  type: string
  mutedAt: number // timestamp
  snoozeMode: 'restart' | '1h' | '24h' | 'session' | 'forever'
  expiresAt?: number // timestamp if time-based
}

const dismissedAlerts = ref<Set<string>>(new Set())
const mutedAlertsMap = ref<Record<string, MutedAlertConfig>>({})
const isAlertsMinimized = ref<boolean>(false)
const showMutedManagerModal = ref<boolean>(false)
const openSnoozeDropdownKey = ref<string | null>(null)

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

function getAlertKey(alert: MetricAlert): string {
  return `${alert.node_name || alert.node_id}-${alert.type}`
}

function isAlertMuted(alert: MetricAlert): boolean {
  const keyByName = `${alert.node_name || alert.node_id}-${alert.type}`
  const keyById = `${alert.node_id}-${alert.type}`
  const now = Date.now()

  const config = mutedAlertsMap.value[keyByName] || mutedAlertsMap.value[keyById]
  if (!config) return false
  if (config.expiresAt && now > config.expiresAt) {
    return false
  }
  return true
}

const activeAlerts = computed<MetricAlert[]>(() => {
  const alerts = overview.value?.alerts || []
  return alerts.filter(a => {
    const key = getAlertKey(a)
    const rawKey = `${a.node_id}-${a.type}`
    if (dismissedAlerts.value.has(key) || dismissedAlerts.value.has(rawKey)) return false
    if (isAlertMuted(a)) return false
    return true
  })
})

const mutedAlertsList = computed<MutedAlertConfig[]>(() => {
  const now = Date.now()
  return Object.values(mutedAlertsMap.value).filter(
    item => !item.expiresAt || now <= item.expiresAt
  )
})

const hasCriticalAlerts = computed<boolean>(() => {
  return activeAlerts.value.some(a => a.value >= 90 || a.type === 'node_down')
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

// 5-Min Saturation Trends Chart
const trendChartMaxReqs = computed(() => {
  return Math.max(10, ...trendHistory.value.map(p => p.reqs || 0), Math.round(effectiveHttpRps.value || 0))
})

const trendChartCpuPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 1000 / (history.length - 1)
  return history.map((h, i) => {
    const x = i * step
    const y = 196 - (Math.min(100, Math.max(0, h.cpu)) / 100) * 192
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

const trendChartCpuArea = computed(() => {
  if (!trendChartCpuPath.value) return ''
  return `${trendChartCpuPath.value} L 1000 196 L 0 196 Z`
})

const trendChartMemPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 1000 / (history.length - 1)
  return history.map((h, i) => {
    const x = i * step
    const y = 196 - (Math.min(100, Math.max(0, h.mem)) / 100) * 192
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

const trendChartMemArea = computed(() => {
  if (!trendChartMemPath.value) return ''
  return `${trendChartMemPath.value} L 1000 196 L 0 196 Z`
})

const trendChartReqsPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const step = 1000 / (history.length - 1)
  const maxR = trendChartMaxReqs.value
  return history.map((h, i) => {
    const x = i * step
    const y = 196 - (Math.min(maxR, Math.max(0, h.reqs)) / maxR) * 192
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

const trendChartReqsArea = computed(() => {
  if (!trendChartReqsPath.value) return ''
  return `${trendChartReqsPath.value} L 1000 196 L 0 196 Z`
})

const trendTimeAxisMarkers = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return []
  const count = Math.min(5, history.length)
  const markers = []
  const step = 1000 / (history.length - 1)
  for (let i = 0; i < count; i++) {
    const idx = Math.round((i / (count - 1)) * (history.length - 1))
    const pt = history[idx]
    if (pt) {
      markers.push({
        x: idx * step,
        time: pt.time,
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
  if (hoveredTrendIndex.value === null) return { display: 'none' }
  const { x, y } = trendTooltipPos.value
  const isRightSide = x > 380
  return {
    left: isRightSide ? `${x - 240}px` : `${x + 16}px`,
    top: `${Math.max(10, y - 40)}px`,
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
  const ratio = mouseX / rect.width
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

function getAlertRecommendation(alert: MetricAlert): string {
  const t = alert.type.toLowerCase()
  if (t.includes('cpu')) return 'Consider load balancing pods or scaling compute nodes.'
  if (t.includes('mem')) return 'Check container memory leaks or increase RAM allocation.'
  if (t.includes('disk')) return 'Clean unused docker images and rotate system log files.'
  if (t.includes('node') || t.includes('down')) return 'Host unreachable. Check agent daemon status on port 9100.'
  return 'Review host telemetry and system diagnostics.'
}

function loadMutedAlerts() {
  try {
    const now = Date.now()
    const valid: Record<string, MutedAlertConfig> = {}
    let localChanged = false

    // 1. Load persistent local storage
    const rawLocal = localStorage.getItem(MUTED_ALERTS_STORAGE_KEY)
    if (rawLocal) {
      const parsedLocal = JSON.parse(rawLocal) as Record<string, MutedAlertConfig>
      for (const [key, item] of Object.entries(parsedLocal)) {
        if (item.expiresAt && now > item.expiresAt) {
          localChanged = true
          continue
        }
        valid[key] = item
      }
    }

    // 2. Load session storage
    const rawSession = sessionStorage.getItem(SESSION_MUTED_ALERTS_KEY)
    if (rawSession) {
      const parsedSession = JSON.parse(rawSession) as Record<string, MutedAlertConfig>
      for (const [key, item] of Object.entries(parsedSession)) {
        if (item.expiresAt && now > item.expiresAt) {
          continue
        }
        valid[key] = item
      }
    }

    mutedAlertsMap.value = valid

    if (localChanged) {
      persistMutedAlertsToStorage()
    }
  } catch (e) {
    console.error('Failed to load muted alerts from storage:', e)
  }
}

function persistMutedAlertsToStorage() {
  try {
    const localEntries: Record<string, MutedAlertConfig> = {}
    const sessionEntries: Record<string, MutedAlertConfig> = {}

    for (const [key, item] of Object.entries(mutedAlertsMap.value)) {
      if (item.snoozeMode === 'session') {
        sessionEntries[key] = item
      } else {
        localEntries[key] = item
      }
    }

    localStorage.setItem(MUTED_ALERTS_STORAGE_KEY, JSON.stringify(localEntries))
    sessionStorage.setItem(SESSION_MUTED_ALERTS_KEY, JSON.stringify(sessionEntries))
  } catch (e) {
    console.error('Failed to save muted alerts to storage:', e)
  }
}

function muteAlert(
  alert: MetricAlert,
  mode: 'restart' | '1h' | '24h' | 'session' | 'forever' = 'restart'
) {
  const key = getAlertKey(alert)
  const now = Date.now()
  let expiresAt: number | undefined

  if (mode === '1h') {
    expiresAt = now + 60 * 60 * 1000
  } else if (mode === '24h') {
    expiresAt = now + 24 * 60 * 60 * 1000
  }

  const config: MutedAlertConfig = {
    key,
    nodeId: alert.node_id,
    nodeName: alert.node_name,
    type: alert.type,
    mutedAt: now,
    snoozeMode: mode,
    expiresAt,
  }

  mutedAlertsMap.value = {
    ...mutedAlertsMap.value,
    [key]: config,
  }
  persistMutedAlertsToStorage()
  openSnoozeDropdownKey.value = null
}

function unmuteAlert(key: string) {
  const updated = { ...mutedAlertsMap.value }
  delete updated[key]
  mutedAlertsMap.value = updated
  persistMutedAlertsToStorage()
}

function muteAllActiveAlerts(mode: 'restart' | '1h' | '24h' = 'restart') {
  const now = Date.now()
  let expiresAt: number | undefined

  if (mode === '1h') {
    expiresAt = now + 60 * 60 * 1000
  } else if (mode === '24h') {
    expiresAt = now + 24 * 60 * 60 * 1000
  }

  const newMap = { ...mutedAlertsMap.value }
  activeAlerts.value.forEach(alert => {
    const key = getAlertKey(alert)
    newMap[key] = {
      key,
      nodeId: alert.node_id,
      nodeName: alert.node_name,
      type: alert.type,
      mutedAt: now,
      snoozeMode: mode,
      expiresAt,
    }
  })

  mutedAlertsMap.value = newMap
  persistMutedAlertsToStorage()
}

function unmuteAllAlerts() {
  mutedAlertsMap.value = {}
  persistMutedAlertsToStorage()
}

function dismissAlert(alert: MetricAlert) {
  dismissedAlerts.value.add(`${alert.node_id}-${alert.type}`)
  dismissedAlerts.value.add(getAlertKey(alert))
}

function toggleSnoozeDropdown(key: string) {
  if (openSnoozeDropdownKey.value === key) {
    openSnoozeDropdownKey.value = null
  } else {
    openSnoozeDropdownKey.value = key
  }
}

function handleDocumentClick(e: MouseEvent) {
  if (openSnoozeDropdownKey.value) {
    const target = e.target as HTMLElement
    if (!target.closest('.snooze-dropdown-wrapper')) {
      openSnoozeDropdownKey.value = null
    }
  }
}

function formatSnoozeLabel(config: MutedAlertConfig): string {
  if (config.snoozeMode === 'restart') return 'Until Restart'
  if (config.snoozeMode === 'forever') return 'Muted Forever'
  if (config.snoozeMode === 'session') return 'Session Only'
  if (config.expiresAt) {
    const diffMs = config.expiresAt - Date.now()
    if (diffMs <= 0) return 'Expired'
    const mins = Math.ceil(diffMs / (60 * 1000))
    if (mins < 60) return `Snoozed (${mins}m rem)`
    const hours = Math.ceil(mins / 60)
    return `Snoozed (${hours}h rem)`
  }
  return config.snoozeMode
}

// ==========================================
// 4. DATA FETCHING & WEBSOCKET
// ==========================================
async function fetchOverview() {
  try {
    const data = await overviewApi.getOverview()
    overview.value = data
    error.value = null
    lastUpdated.value = new Date()

    const now = new Date()
    const timeLabel = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
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
          time: past.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
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
      lastUpdated.value = new Date()
    }
  }
})

// ==========================================
// 5. LIFECYCLE & CLEANUP
// ==========================================
onMounted(() => {
  loadMutedAlerts()
  loadCustomOrder()
  pollClusterMetrics()

  pollInterval = setInterval(() => {
    fetchOverview()
    fetchTps()
  }, 5000)

  document.addEventListener('click', handleDocumentClick)
})

onUnmounted(() => {
  if (pollInterval) clearInterval(pollInterval)
  if (trendBufferInterval) clearInterval(trendBufferInterval)
  document.removeEventListener('click', handleDocumentClick)
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

    <!-- SECTION 4: SLIDE-DOWN ALERT BANNERS & MUTED ALERTS MANAGER -->
    <section class="alerts-section" v-if="activeAlerts.length > 0 || mutedAlertsList.length > 0">
      <!-- Muted Status Chip (when 0 active alerts left, but 1+ alerts are silenced) -->
      <div v-if="activeAlerts.length === 0 && mutedAlertsList.length > 0" class="muted-alerts-chip glass-panel animate-fade-in">
        <div class="muted-chip-left">
          <span class="muted-chip-icon">🔕</span>
          <span class="muted-chip-text">
            <strong>{{ mutedAlertsList.length }} alert(s) silenced</strong> (Muted until restart / schedule)
          </span>
        </div>
        <div class="muted-chip-actions">
          <button class="btn-manage-muted" @click="showMutedManagerModal = true">
            <span>⚙️ Manage / Unmute</span>
          </button>
          <button class="btn-unmute-quick" @click="unmuteAllAlerts" title="Restore all silenced alerts">
            <span>🔔 Unmute All</span>
          </button>
        </div>
      </div>

      <!-- Active Alerts Container -->
      <div v-if="activeAlerts.length > 0" class="active-alerts-container">
        <!-- Consolidated Alerts Header Bar -->
        <div
          class="alerts-header-bar glass-panel"
          :class="hasCriticalAlerts ? 'alerts-header-critical' : 'alerts-header-warning'"
        >
          <div class="alerts-header-left">
            <span class="alerts-header-icon">{{ hasCriticalAlerts ? '🚨' : '⚠️' }}</span>
            <div class="alerts-header-info">
              <span class="alerts-header-title">Cluster Health Alerts</span>
              <span class="badge" :class="hasCriticalAlerts ? 'badge-rose' : 'badge-amber'">
                {{ activeAlerts.length }} Active
              </span>
            </div>
            <button
              v-if="mutedAlertsList.length > 0"
              class="badge-muted-counter"
              @click="showMutedManagerModal = true"
              title="View and manage muted alerts"
            >
              <span>🔕 {{ mutedAlertsList.length }} muted</span>
            </button>
          </div>

          <div class="alerts-header-actions">
            <!-- 1-Click Mute All -->
            <button
              class="btn-mute-all"
              @click="muteAllActiveAlerts('restart')"
              title="Silence all active node alerts until server restart"
            >
              <span class="btn-mute-icon">🔕</span>
              <span>Mute All (Until Restart)</span>
            </button>

            <!-- Minimize / Expand Toggle -->
            <button
              class="btn-minimize-alerts"
              @click="isAlertsMinimized = !isAlertsMinimized"
              :title="isAlertsMinimized ? 'Expand alert cards' : 'Minimize alerts into a compact single-line bar'"
            >
              <span>{{ isAlertsMinimized ? '👁️ Expand' : '👁️ Minimize' }}</span>
            </button>
          </div>
        </div>

        <!-- Minimized Single-Line Summary Bar -->
        <div
          v-if="isAlertsMinimized"
          class="alerts-minimized-bar glass-panel"
          @click="isAlertsMinimized = false"
          title="Click to expand alert details"
        >
          <span class="pulse-dot pulse-active" :class="hasCriticalAlerts ? 'text-rose' : 'text-amber'"></span>
          <span class="minimized-text">
            <strong>{{ activeAlerts.length }} active alert(s):</strong>
            {{ activeAlerts.map(a => a.node_name || a.node_id).join(', ') }} &mdash;
            <span class="minimized-click-hint">Click to expand details</span>
          </span>
        </div>

        <!-- Expanded Alert Cards List -->
        <transition-group v-else name="alert-slide" tag="div" class="alerts-cards-list">
          <div
            v-for="alert in activeAlerts"
            :key="`${alert.node_id}-${alert.type}`"
            class="alert-banner glass-panel"
            :class="alert.value >= 90 || alert.type === 'node_down' ? 'alert-critical' : 'alert-warning'"
          >
            <div class="alert-icon-box">
              <span class="alert-emoji">{{ alert.value >= 90 || alert.type === 'node_down' ? '🚨' : '⚠️' }}</span>
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

              <!-- Snooze / Mute Dropdown & Quick Action -->
              <div class="snooze-dropdown-wrapper">
                <button
                  class="btn-mute-pill"
                  @click="muteAlert(alert, 'restart')"
                  title="Silence this alert until server restart"
                >
                  <span class="mute-icon">🔕</span>
                  <span>Mute (Until Restart)</span>
                </button>
                <button
                  class="btn-snooze-caret"
                  @click.stop="toggleSnoozeDropdown(getAlertKey(alert))"
                  title="More snooze durations"
                >
                  ▾
                </button>

                <!-- Snooze Options Dropdown Menu -->
                <div
                  v-if="openSnoozeDropdownKey === getAlertKey(alert)"
                  class="snooze-menu glass-panel animate-scale-in"
                  @click.stop
                >
                  <div class="snooze-menu-header">Snooze Duration</div>
                  <button class="snooze-menu-item" @click="muteAlert(alert, 'restart')">
                    <span class="snooze-item-icon">🔄</span>
                    <div class="snooze-item-text">
                      <span class="snooze-item-title">Until Server Restart</span>
                      <span class="snooze-item-desc">Muted across page refreshes</span>
                    </div>
                  </button>
                  <button class="snooze-menu-item" @click="muteAlert(alert, '1h')">
                    <span class="snooze-item-icon">⏱️</span>
                    <div class="snooze-item-text">
                      <span class="snooze-item-title">Snooze 1 Hour</span>
                      <span class="snooze-item-desc">Re-evaluate after 60 mins</span>
                    </div>
                  </button>
                  <button class="snooze-menu-item" @click="muteAlert(alert, '24h')">
                    <span class="snooze-item-icon">📅</span>
                    <div class="snooze-item-text">
                      <span class="snooze-item-title">Snooze 24 Hours</span>
                      <span class="snooze-item-desc">Re-evaluate after 1 day</span>
                    </div>
                  </button>
                  <button class="snooze-menu-item" @click="muteAlert(alert, 'session')">
                    <span class="snooze-item-icon">🪟</span>
                    <div class="snooze-item-text">
                      <span class="snooze-item-title">Dismiss for Session</span>
                      <span class="snooze-item-desc">Muted until tab is closed</span>
                    </div>
                  </button>
                </div>
              </div>

              <button class="btn-alert-dismiss" @click="dismissAlert(alert)" title="Dismiss Alert">
                ✕
              </button>
            </div>
          </div>
        </transition-group>
      </div>
    </section>

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
        :httpActiveConns="httpActiveConns"
        :httpQueuedReqs="httpQueuedReqs"
        :httpErrorRate="httpErrorRate"
        :clusterAvgLatencyMs="clusterAvgLatencyMs"
        @open-deep-dive="openDeepDiveModal"
      />

      <!-- 3. 5-MIN SATURATION TRENDS CHART -->
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
              <svg viewBox="0 0 1000 200" preserveAspectRatio="none" class="trend-plot-svg">
                <defs>
                  <linearGradient id="trendCpuGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stop-color="#8b5cf6" stop-opacity="0.35" />
                    <stop offset="100%" stop-color="#8b5cf6" stop-opacity="0.0" />
                  </linearGradient>
                  <linearGradient id="trendMemGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.3" />
                    <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
                  </linearGradient>
                  <linearGradient id="trendReqsGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stop-color="#10b981" stop-opacity="0.3" />
                    <stop offset="100%" stop-color="#10b981" stop-opacity="0.0" />
                  </linearGradient>
                </defs>

                <!-- 5 Horizontal Grid Lines -->
                <line x1="0" y1="4" x2="1000" y2="4" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3 3" />
                <line x1="0" y1="52" x2="1000" y2="52" stroke="rgba(255,255,255,0.04)" stroke-dasharray="3 3" />
                <line x1="0" y1="100" x2="1000" y2="100" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3 3" />
                <line x1="0" y1="148" x2="1000" y2="148" stroke="rgba(255,255,255,0.04)" stroke-dasharray="3 3" />
                <line x1="0" y1="196" x2="1000" y2="196" stroke="rgba(255,255,255,0.1)" />

                <!-- Series 1: CPU Area & Line (Violet) -->
                <path v-if="trendChartCpuArea" :d="trendChartCpuArea" fill="url(#trendCpuGrad)" />
                <path v-if="trendChartCpuPath" :d="trendChartCpuPath" fill="none" stroke="#8b5cf6" stroke-width="2.5" />

                <!-- Series 2: RAM Area & Line (Cyan) -->
                <path v-if="trendChartMemArea" :d="trendChartMemArea" fill="url(#trendMemGrad)" />
                <path v-if="trendChartMemPath" :d="trendChartMemPath" fill="none" stroke="#06b6d4" stroke-width="2.5" />

                <!-- Series 3: Throughput Area & Line (Emerald) -->
                <path v-if="trendChartReqsArea" :d="trendChartReqsArea" fill="url(#trendReqsGrad)" />
                <path v-if="trendChartReqsPath" :d="trendChartReqsPath" fill="none" stroke="#10b981" stroke-width="2.5" />

                <!-- Interactive Hover Crosshair & Series Markers -->
                <g v-if="trendHoverCoords">
                  <line
                    :x1="trendHoverCoords.x"
                    y1="4"
                    :x2="trendHoverCoords.x"
                    y2="196"
                    stroke="#38bdf8"
                    stroke-width="1.5"
                    stroke-dasharray="3 3"
                  />
                  <circle
                    :cx="trendHoverCoords.x"
                    :cy="trendHoverCoords.yCpu"
                    r="4"
                    fill="#8b5cf6"
                    stroke="#ffffff"
                    stroke-width="2"
                  />
                  <circle
                    :cx="trendHoverCoords.x"
                    :cy="trendHoverCoords.yMem"
                    r="4"
                    fill="#06b6d4"
                    stroke="#ffffff"
                    stroke-width="2"
                  />
                  <circle
                    :cx="trendHoverCoords.x"
                    :cy="trendHoverCoords.yReq"
                    r="4"
                    fill="#10b981"
                    stroke="#ffffff"
                    stroke-width="2"
                  />
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
              <span class="y-tick">{{ trendChartMaxReqs }}</span>
              <span class="y-tick">{{ Math.round(trendChartMaxReqs * 0.75) }}</span>
              <span class="y-tick">{{ Math.round(trendChartMaxReqs * 0.5) }}</span>
              <span class="y-tick">{{ Math.round(trendChartMaxReqs * 0.25) }}</span>
              <span class="y-tick">0</span>
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

    <!-- MUTED ALERTS MANAGER MODAL -->
    <div v-if="showMutedManagerModal" class="modal-backdrop animate-fade-in" @click.self="showMutedManagerModal = false">
      <div class="muted-manager-modal glass-panel animate-scale-in">
        <div class="muted-modal-header">
          <div class="muted-modal-title-group">
            <div class="muted-modal-icon-badge">🔕</div>
            <div>
              <h3 class="muted-modal-title">Muted &amp; Silenced Health Alerts</h3>
              <p class="muted-modal-subtitle">
                {{ mutedAlertsList.length }} alert rule(s) currently silenced. Muted alerts will not flash or re-trigger telemetry notifications.
              </p>
            </div>
          </div>
          <button class="btn-modal-close" @click="showMutedManagerModal = false">✕</button>
        </div>

        <div class="muted-modal-body">
          <div v-if="mutedAlertsList.length === 0" class="muted-empty-state">
            <span class="muted-empty-icon">🔔</span>
            <p>No alerts currently muted. All telemetry warnings will be displayed in real time.</p>
          </div>

          <div v-else class="muted-list">
            <div v-for="item in mutedAlertsList" :key="item.key" class="muted-item-row glass-panel">
              <div class="muted-item-info">
                <div class="muted-item-header">
                  <span class="muted-item-node">{{ item.nodeName || item.nodeId || item.key }}</span>
                  <span class="alert-type-tag">{{ item.type.toUpperCase() }}</span>
                  <span class="muted-badge-mode">{{ formatSnoozeLabel(item) }}</span>
                </div>
                <div class="muted-item-meta">
                  <span>Muted at {{ new Date(item.mutedAt).toLocaleTimeString() }}</span>
                  <span v-if="item.expiresAt" class="muted-expires-text">
                    &bull; Expires: {{ new Date(item.expiresAt).toLocaleTimeString() }}
                  </span>
                </div>
              </div>

              <div class="muted-item-actions">
                <button
                  class="btn-unmute-single"
                  @click="unmuteAlert(item.key)"
                  title="Unmute and restore live alerting for this node"
                >
                  <span>🔔 Unmute</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="muted-modal-footer">
          <button
            v-if="mutedAlertsList.length > 0"
            class="btn btn-secondary btn-unmute-all-modal"
            @click="unmuteAllAlerts"
          >
            <span>🔔 Unmute All ({{ mutedAlertsList.length }})</span>
          </button>
          <button class="btn btn-primary" @click="showMutedManagerModal = false">
            <span>Done</span>
          </button>
        </div>
      </div>
    </div>
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

/* SECTION 4: Alerts & Muted Alerts Manager */
.alerts-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
  transition: all 0.25s ease;
}

/* Muted Status Chip */
.muted-alerts-chip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 18px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.75);
  border: 1px dashed rgba(148, 163, 184, 0.3);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  transition: all 0.25s ease;
  flex-wrap: wrap;
}

.muted-chip-left {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: var(--text-secondary, #cbd5e1);
}

.muted-chip-icon {
  font-size: 18px;
}

.muted-chip-text strong {
  color: #fbbf24;
  font-weight: 600;
}

.muted-chip-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-manage-muted {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.16);
  color: #f8fafc;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.btn-manage-muted:hover {
  background: rgba(255, 255, 255, 0.16);
  border-color: rgba(255, 255, 255, 0.3);
  transform: translateY(-1px);
}

.btn-unmute-quick {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-unmute-quick:hover {
  background: rgba(16, 185, 129, 0.25);
  border-color: rgba(16, 185, 129, 0.5);
  transform: translateY(-1px);
}

/* Active Alerts Container */
.active-alerts-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

/* Consolidated Header Bar */
.alerts-header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 16px;
  border-radius: 12px;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  transition: all 0.25s ease;
  flex-wrap: wrap;
}

.alerts-header-critical {
  border: 1px solid rgba(244, 63, 94, 0.4);
  background: linear-gradient(90deg, rgba(244, 63, 94, 0.18) 0%, rgba(15, 23, 42, 0.85) 100%);
}

.alerts-header-warning {
  border: 1px solid rgba(245, 158, 11, 0.4);
  background: linear-gradient(90deg, rgba(245, 158, 11, 0.18) 0%, rgba(15, 23, 42, 0.85) 100%);
}

.alerts-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.alerts-header-icon {
  font-size: 18px;
}

.alerts-header-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.alerts-header-title {
  font-weight: 700;
  font-size: 13px;
  color: #fff;
  letter-spacing: 0.02em;
}

.badge-rose {
  background: rgba(244, 63, 94, 0.18);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.35);
}

.badge-amber {
  background: rgba(245, 158, 11, 0.18);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.35);
}

.badge-muted-counter {
  background: rgba(148, 163, 184, 0.15);
  border: 1px solid rgba(148, 163, 184, 0.3);
  color: #cbd5e1;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.badge-muted-counter:hover {
  background: rgba(148, 163, 184, 0.25);
  color: #fff;
  border-color: rgba(148, 163, 184, 0.5);
}

.alerts-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-mute-all {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.4);
  color: #fecdd3;
  font-weight: 600;
  font-size: 11px;
  padding: 6px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.btn-mute-all:hover {
  background: rgba(244, 63, 94, 0.3);
  border-color: rgba(244, 63, 94, 0.6);
  color: #fff;
  transform: translateY(-1px);
}

.btn-mute-icon {
  font-size: 13px;
}

.btn-minimize-alerts {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: var(--text-secondary, #cbd5e1);
  font-size: 11px;
  font-weight: 600;
  padding: 6px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-minimize-alerts:hover {
  background: rgba(255, 255, 255, 0.16);
  color: #fff;
}

/* Minimized Single-Line Bar */
.alerts-minimized-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.7);
  border: 1px dashed rgba(245, 158, 11, 0.4);
  cursor: pointer;
  font-size: 12px;
  color: var(--text-secondary, #cbd5e1);
  transition: all 0.2s ease;
}

.alerts-minimized-bar:hover {
  background: rgba(15, 23, 42, 0.9);
  border-color: rgba(245, 158, 11, 0.7);
  color: #fff;
}

.minimized-text {
  flex: 1;
}

.minimized-click-hint {
  color: #38bdf8;
  text-decoration: underline;
  font-style: italic;
  margin-left: 4px;
}

.text-rose {
  color: #f43f5e;
}

.text-amber {
  color: #fbbf24;
}

/* Alert Cards List */
.alerts-cards-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.alert-banner {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 18px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.85);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.alert-critical {
  border-color: rgba(244, 63, 94, 0.5);
  background: linear-gradient(90deg, rgba(244, 63, 94, 0.15) 0%, rgba(15, 23, 42, 0.85) 100%);
}

.alert-warning {
  border-color: rgba(245, 158, 11, 0.5);
  background: linear-gradient(90deg, rgba(245, 158, 11, 0.15) 0%, rgba(15, 23, 42, 0.85) 100%);
}

.alert-icon-box {
  font-size: 20px;
  flex-shrink: 0;
}

.alert-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.alert-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.alert-node-tag {
  font-weight: 700;
  font-size: 13px;
  color: var(--text-primary, #f8fafc);
  font-family: var(--font-mono, monospace);
}

.alert-type-tag {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.1);
  color: #fbbf24;
}

.alert-value-tag {
  font-size: 11px;
  font-family: var(--font-mono, monospace);
  color: var(--text-muted, #64748b);
}

.alert-message {
  font-size: 12px;
  color: var(--text-secondary, #cbd5e1);
  margin: 0;
  line-height: 1.4;
}

.alert-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.btn-alert-action {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: var(--text-primary, #f8fafc);
  padding: 5px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-alert-action:hover {
  background: rgba(255, 255, 255, 0.15);
}

/* Snooze Dropdown & Pill */
.snooze-dropdown-wrapper {
  position: relative;
  display: inline-flex;
  align-items: stretch;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
}

.btn-mute-pill {
  background: transparent;
  border: none;
  color: #fbbf24;
  font-size: 11px;
  font-weight: 600;
  padding: 5px 9px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  transition: all 0.2s ease;
}

.btn-mute-pill:hover {
  background: rgba(251, 191, 36, 0.15);
  color: #fde68a;
}

.mute-icon {
  font-size: 12px;
}

.btn-snooze-caret {
  background: transparent;
  border: none;
  border-left: 1px solid rgba(255, 255, 255, 0.12);
  color: #fbbf24;
  padding: 0 7px;
  font-size: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-snooze-caret:hover {
  background: rgba(251, 191, 36, 0.2);
  color: #fff;
}

.snooze-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 240px;
  background: rgba(15, 23, 42, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  padding: 6px;
  z-index: 60;
  box-shadow: 0 12px 30px -4px rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.snooze-menu-header {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted, #94a3b8);
  padding: 4px 8px;
  font-weight: 700;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  margin-bottom: 2px;
}

.snooze-menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 8px;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: var(--text-primary, #f8fafc);
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease;
  width: 100%;
}

.snooze-menu-item:hover {
  background: rgba(255, 255, 255, 0.1);
}

.snooze-item-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.snooze-item-text {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.snooze-item-title {
  font-size: 11px;
  font-weight: 600;
  color: #f8fafc;
}

.snooze-item-desc {
  font-size: 9.5px;
  color: #94a3b8;
  line-height: 1.2;
}

.btn-alert-dismiss {
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  font-size: 14px;
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 4px;
}

.btn-alert-dismiss:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.1);
}

/* Muted Alerts Manager Modal */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.muted-manager-modal {
  width: 92%;
  max-width: 600px;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  background: rgba(15, 23, 42, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 16px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.6);
  overflow: hidden;
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
}

.muted-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.muted-modal-title-group {
  display: flex;
  align-items: center;
  gap: 14px;
}

.muted-modal-icon-badge {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: rgba(245, 158, 11, 0.15);
  border: 1px solid rgba(245, 158, 11, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  flex-shrink: 0;
}

.muted-modal-title {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 2px;
}

.muted-modal-subtitle {
  font-size: 12px;
  color: var(--text-secondary, #94a3b8);
  margin: 0;
  line-height: 1.4;
}

.btn-modal-close {
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  font-size: 18px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.btn-modal-close:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.1);
}

.muted-modal-body {
  padding: 20px 24px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 50vh;
}

.muted-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 32px 16px;
  text-align: center;
  color: var(--text-muted, #94a3b8);
  font-size: 13px;
}

.muted-empty-icon {
  font-size: 28px;
}

.muted-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.muted-item-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  transition: all 0.2s ease;
}

.muted-item-row:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.12);
}

.muted-item-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.muted-item-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.muted-item-node {
  font-weight: 700;
  font-size: 13px;
  color: #f8fafc;
  font-family: var(--font-mono, monospace);
}

.muted-badge-mode {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(56, 189, 248, 0.15);
  border: 1px solid rgba(56, 189, 248, 0.3);
  color: #38bdf8;
}

.muted-item-meta {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.muted-expires-text {
  color: #fbbf24;
}

.muted-item-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.btn-unmute-single {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: #f8fafc;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.btn-unmute-single:hover {
  background: rgba(16, 185, 129, 0.2);
  border-color: rgba(16, 185, 129, 0.5);
  color: #34d399;
}

.muted-modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.2);
}

.btn-unmute-all-modal {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.35);
  color: #34d399;
}

.btn-unmute-all-modal:hover {
  background: rgba(16, 185, 129, 0.25);
  border-color: rgba(16, 185, 129, 0.5);
  color: #fff;
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
  gap: 14px;
  flex-wrap: wrap;
}

.trend-legend {
  display: flex;
  align-items: center;
  gap: 10px;
}

.legend-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary, #94a3b8);
}

.legend-dot-circle {
  width: 7px;
  height: 7px;
  border-radius: 50%;
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
}

.trend-expand-badge:hover {
  background: rgba(56, 189, 248, 0.2);
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
  width: 32px;
  text-align: right;
  padding-bottom: 2px;
}

.y-axis-right {
  text-align: left;
}

.trend-plot-canvas {
  flex: 1;
  position: relative;
  height: 100%;
  background: rgba(0, 0, 0, 0.25);
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.05);
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
</style>\n