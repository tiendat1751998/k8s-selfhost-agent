import { ref, type Ref, type ComputedRef } from 'vue'
import { nodeHistoryApi, type NodeHistoryResponse } from '../api/compute'
import type { NodeMetrics } from '../api/overview'

export function useNodeDiagnostics(
  selectedNode: ComputedRef<NodeMetrics | null> | Ref<NodeMetrics | null>,
  selectedNodeId: Ref<string | null>,
) {
  const showNodeDrawer = ref(false)
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
      actualRange = nodeIdOrRange
      targetFrom = rangeOrFrom
      targetTo = fromOrTo
    } else {
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
      if (targetFrom !== undefined && targetFrom !== '') customHistFrom.value = targetFrom
      if (targetTo !== undefined && targetTo !== '') customHistTo.value = targetTo
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
    } catch (err: unknown) {
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

  function inspectNode(node: NodeMetrics) {
    selectedNodeId.value = node.node_id
    nodeDrawerMode.value = 'live'
    showNodeDrawer.value = true
    loadNodeHistory(node.node_id, nodeHistoryRange.value)
  }

  return {
    showNodeDrawer,
    nodeDrawerMode,
    nodeHistoryData,
    nodeHistoryLoading,
    nodeHistoryRange,
    customHistFrom,
    customHistTo,
    loadNodeHistory,
    applyCustomHistoryPreset,
    inspectNode,
  }
}
