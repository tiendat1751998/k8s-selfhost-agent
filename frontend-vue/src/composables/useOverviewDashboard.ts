import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  overviewApi,
  tpsApi,
  type SystemOverview,
  type NodeMetrics,
  type TpsSnapshot,
} from '../api/overview'
import { useWebSocket } from './useWebSocket'
import { useAlertStore } from '../stores/alertStore'
import { useAppStore } from '../stores/app'
import { useNodeDiagnostics } from './useNodeDiagnostics'

export interface TrendPoint {
  time: string
  cpu: number
  mem: number
  disk: number
  reqs: number
}

export type TopologyFilterType = 'all' | 'control_plane' | 'worker' | 'hot' | 'overloaded'

const NODE_ORDER_STORAGE_KEY = 'k8s_overview_node_order'

export function useOverviewDashboard() {
  const router = useRouter()
  const appStore = useAppStore()
  const alertStore = useAlertStore()

  // 1. Primary Telemetry State
  const overview = ref<SystemOverview | null>(null)
  const loading = ref(true)
  const error = ref<string | null>(null)
  const lastUpdated = ref<Date>(new Date())
  const isLiveWs = ref(false)

  // TPS State
  const tpsData = ref<TpsSnapshot | null>(null)
  const tpsLoading = ref(true)
  const tpsError = ref<string | null>(null)
  const tpsLastUpdated = ref<Date | null>(null)

  // Rolling 30-sample history for saturation trends
  const trendHistory = ref<TrendPoint[]>([])

  // Modal & Ordering State
  const showDeepDiveModal = ref(false)
  const customNodeOrder = ref<string[]>([])
  const draggedNodeId = ref<string | null>(null)
  const dragOverNodeId = ref<string | null>(null)
  const selectedTopologyFilter = ref<TopologyFilterType>('all')
  const selectedNodeId = ref<string | null>(null)

  // Polling Interval
  let pollInterval: ReturnType<typeof setInterval> | null = null

  // 2. Computed Metrics & Topology Aggregations
  const orderedNodes = computed<NodeMetrics[]>(() => {
    const raw = overview.value?.nodes || []
    if (!raw.length) return []
    const incoming = raw.map(n => ({
      ...n,
      node_name: n.node_name === 'k8smater' ? 'k8smaster' : n.node_name
    }))
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
    const incoming = nodes.value
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

  // Diagnostics Composable Delegation
  const diagnostics = useNodeDiagnostics(selectedNode, selectedNodeId)

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
  const clusterAvgLatencyMs = computed(() => tpsData.value?.http?.avg_latency_ms ?? 0)

  const clusterTotalMemBytes = computed(() => nodes.value.reduce((acc, n) => acc + (n.memory_total || 0), 0))
  const clusterUsedMemBytes = computed(() => nodes.value.reduce((acc, n) => acc + (n.memory_used || 0), 0))
  const clusterTotalDiskBytes = computed(() => nodes.value.reduce((acc, n) => acc + (n.disk_total || 0), 0))
  const clusterUsedDiskBytes = computed(() => nodes.value.reduce((acc, n) => acc + (n.disk_used || 0), 0))

  const peakCpuNode = computed<NodeMetrics | null>(() => {
    if (!nodes.value.length) return null
    return [...nodes.value].sort((a, b) => (b.cpu_percent || 0) - (a.cpu_percent || 0))[0] || null
  })

  const clusterHealthScore = computed<number>(() => {
    if (!overview.value || !nodes.value.length) return 100
    const totalN = overview.value.total_nodes || nodes.value.length
    const healthyN = overview.value.healthy_nodes ?? totalN
    const nodeRatio = totalN > 0 ? (healthyN / totalN) * 50 : 50
    const errPenalty = Math.min(25, (httpErrorRate.value || 0) * 5)
    const cpuPenalty = (overview.value.total_cpu_percent || 0) > 85 ? 15 : 0
    const memPenalty = (overview.value.total_mem_percent || 0) > 90 ? 10 : 0
    return Math.max(0, Math.round(nodeRatio + 50 - errPenalty - cpuPenalty - memPenalty))
  })

  const isClusterDegraded = computed<boolean>(() => {
    if (!overview.value) return false
    const hasUnhealthyNodes = overview.value.healthy_nodes < overview.value.total_nodes
    const hasCriticalAlerts = alertStore.hasCriticalAlerts || alertStore.hasNodeDown
    const isHighError = httpErrorRate.value >= 5
    return hasUnhealthyNodes || hasCriticalAlerts || isHighError
  })

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

  // 3. Quick Action Dispatchers & Order Management
  function openDeepDiveModal() {
    showDeepDiveModal.value = true
  }

  function closeDeepDiveModal() {
    showDeepDiveModal.value = false
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
    diagnostics.inspectNode(node)
  }

  function manageNode(node: NodeMetrics) {
    router.push({ path: '/hosts', query: { search: node.node_name } })
  }

  function navigateTo(path: string) {
    router.push(path)
  }

  // 4. Data Fetching & WebSockets
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

      trendHistory.value.push(newPoint)
      if (trendHistory.value.length > 30) {
        trendHistory.value.shift()
      }
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to sync cluster telemetry'
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
    } catch (err: unknown) {
      tpsError.value = err instanceof Error ? err.message : 'Failed to fetch TPS telemetry'
    } finally {
      tpsLoading.value = false
    }
  }

  async function pollClusterMetrics() {
    loading.value = true
    await Promise.allSettled([fetchOverview(), fetchTps()])
  }

  // Live WebSocket sync
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

  // 5. Lifecycle Management
  onMounted(() => {
    loadCustomOrder()
    pollClusterMetrics()

    pollInterval = setInterval(() => {
      fetchOverview()
      fetchTps()
    }, 5000)
  })

  onUnmounted(() => {
    if (pollInterval) {
      clearInterval(pollInterval)
      pollInterval = null
    }
  })

  return {
    // State
    overview,
    loading,
    error,
    lastUpdated,
    isLiveWs,
    tpsData,
    tpsLoading,
    tpsError,
    tpsLastUpdated,
    trendHistory,
    showDeepDiveModal,
    selectedNodeId,
    customNodeOrder,
    draggedNodeId,
    dragOverNodeId,
    selectedTopologyFilter,

    // Diagnostics State & Methods (Delegated)
    showNodeDrawer: diagnostics.showNodeDrawer,
    nodeDrawerMode: diagnostics.nodeDrawerMode,
    nodeHistoryData: diagnostics.nodeHistoryData,
    nodeHistoryLoading: diagnostics.nodeHistoryLoading,
    nodeHistoryRange: diagnostics.nodeHistoryRange,
    customHistFrom: diagnostics.customHistFrom,
    customHistTo: diagnostics.customHistTo,
    loadNodeHistory: diagnostics.loadNodeHistory,
    applyCustomHistoryPreset: diagnostics.applyCustomHistoryPreset,
    inspectNode: diagnostics.inspectNode,

    // Computed
    orderedNodes,
    busiestNodeId,
    nodes,
    selectedNode,
    totalContainers,
    runningContainers,
    effectiveHttpRps,
    httpActiveConns,
    httpQueuedReqs,
    httpErrorRate,
    clusterAvgLatencyMs,
    clusterTotalMemBytes,
    clusterUsedMemBytes,
    clusterTotalDiskBytes,
    clusterUsedDiskBytes,
    peakCpuNode,
    clusterHealthScore,
    isClusterDegraded,
    filteredTopologyNodes,
    topologyFilterCounts,

    // Methods / Actions
    openDeepDiveModal,
    closeDeepDiveModal,
    loadCustomOrder,
    saveCustomOrder,
    resetNodeOrder,
    onDragStart,
    onDragOver,
    onDragEnter,
    onDragLeave,
    onDrop,
    onDragEnd,
    handleNodeCardClick,
    manageNode,
    navigateTo,
    fetchOverview,
    fetchTps,
    pollClusterMetrics,
  }
}
