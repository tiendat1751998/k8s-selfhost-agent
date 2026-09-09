import { ref, computed, watch } from 'vue'
import type { NodeMetrics, TpsSnapshot } from '../../../api/overview'
import type { NodeMetricRollup, NodeHistoryResponse } from '../../../api/compute'
import {
  HIST_Y_BOTTOM,
  HIST_Y_HEIGHT,
  HIST_X_LEFT,
  HIST_X_RIGHT,
  HIST_X_WIDTH,
  formatDt,
  getSafeDefaultDates,
  hPeak,
  buildLinePath,
  buildAreaPath,
  buildCpuEnvelope,
  buildSpikeMarkers,
  buildTimeMarkers,
} from './nodeChartMath'

export interface HistoricalChartProps {
  node: NodeMetrics
  tpsData?: TpsSnapshot | null
  nodeHistoryData?: NodeHistoryResponse | null
  nodeHistoryLoading?: boolean
  nodeHistoryRange?: string
  customHistFrom?: string
  customHistTo?: string
}

export type HistoricalChartEmit = {
  (e: 'update:nodeHistoryRange', range: string): void
  (e: 'update:customHistFrom', val: string): void
  (e: 'update:customHistTo', val: string): void
  (e: 'range-change', range: string, from?: string, to?: string): void
  (e: 'custom-range-apply'): void
  (e: 'apply-preset', preset: '30m' | '2h' | '6h' | 'today'): void
  (e: 'sync-point-in-time', point: NodeMetricRollup, suspectApp?: string): void
}

export function useNodeHistoricalChart(props: HistoricalChartProps, emit: HistoricalChartEmit) {
  const showHistCpu = ref(true)
  const showHistMem = ref(true)
  const showHistDisk = ref(true)
  const showHistPeakEnvelope = ref(true)
  const histViewMode = ref<'both' | 'avg' | 'peak'>('both')
  const showCustomHistoryPicker = ref(false)

  const safeInit = getSafeDefaultDates()
  const customHistoryFrom = ref(props.customHistFrom || safeInit.from)
  const customHistoryTo = ref(props.customHistTo || safeInit.to)

  watch(() => props.customHistFrom, (val) => {
    if (val) customHistoryFrom.value = val
    else if (!customHistoryFrom.value) customHistoryFrom.value = getSafeDefaultDates().from
  })
  watch(() => props.customHistTo, (val) => {
    if (val) customHistoryTo.value = val
    else if (!customHistoryTo.value) customHistoryTo.value = getSafeDefaultDates().to
  })

  const hoveredNodeHistIndex = ref<number | null>(null)
  const isNodeHistHovered = ref(false)
  const nodeHistTooltipPos = ref({ x: 0, y: 0 })

  const nodeHistoryWindowBadge = computed(() => {
    if (props.nodeHistoryRange === 'custom') return 'CUSTOM WINDOW'
    return `PAST ${(props.nodeHistoryRange || '1h').toUpperCase()}`
  })

  function toggleCustomHistoryPicker() {
    showCustomHistoryPicker.value = !showCustomHistoryPicker.value
    if (showCustomHistoryPicker.value) {
      if (!customHistoryFrom.value || !customHistoryTo.value || customHistoryFrom.value === customHistoryTo.value) {
        const dates = getSafeDefaultDates()
        customHistoryFrom.value = dates.from
        customHistoryTo.value = dates.to
      }
    }
  }

  function switchNodeDrawerToHistory(range: string) {
    showCustomHistoryPicker.value = false
    emit('update:nodeHistoryRange', range)
    emit('range-change', range)
  }

  function applyPreset(preset: '30m' | '2h' | '6h' | 'today') {
    const now = new Date()
    if (preset === '30m') {
      customHistoryFrom.value = formatDt(new Date(now.getTime() - 30 * 60 * 1000))
      customHistoryTo.value = formatDt(now)
    } else if (preset === '2h') {
      customHistoryFrom.value = formatDt(new Date(now.getTime() - 2 * 60 * 60 * 1000))
      customHistoryTo.value = formatDt(now)
    } else if (preset === '6h') {
      customHistoryFrom.value = formatDt(new Date(now.getTime() - 6 * 60 * 60 * 1000))
      customHistoryTo.value = formatDt(now)
    } else if (preset === 'today') {
      const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0)
      customHistoryFrom.value = formatDt(startOfDay)
      customHistoryTo.value = formatDt(now)
    }

    emit('update:customHistFrom', customHistoryFrom.value)
    emit('update:customHistTo', customHistoryTo.value)
    emit('range-change', 'custom', customHistoryFrom.value, customHistoryTo.value)
  }

  const nodeHistoryList = computed<NodeMetricRollup[]>(() => {
    const base = props.nodeHistoryData?.history || []
    if (base.length === 0) {
      if (!props.node) return []
      const now = new Date()
      return [{
        id: 'live',
        tenant_id: 'default',
        node_id: props.node.node_id,
        node_name: props.node.node_name,
        cpu_percent: props.node.cpu_percent || 0,
        cpu_peak: props.node.cpu_percent || 0,
        mem_used_bytes: props.node.memory_used || 0,
        mem_total_bytes: props.node.memory_total || 0,
        mem_percent: props.node.memory_percent || 0,
        disk_used_bytes: props.node.disk_used || 0,
        disk_total_bytes: props.node.disk_total || 0,
        disk_percent: props.node.disk_percent || 0,
        rx_bytes_per_sec: props.node.network_rx_bytes || 0,
        tx_bytes_per_sec: props.node.network_tx_bytes || 0,
        process_count: props.node.processes || 0,
        container_count: props.node.container_count || 0,
        status: props.node.status || 'ready',
        resolution: 'live',
        recorded_at: now.toISOString(),
      }]
    }

    if (props.node && base.length > 0) {
      const last = base[base.length - 1]
      const lastTime = new Date(last.recorded_at).getTime()
      if (Date.now() - lastTime > 15000 && props.nodeHistoryRange === '1h') {
        return [...base, {
          id: 'live-tail',
          tenant_id: 'default',
          node_id: props.node.node_id,
          node_name: props.node.node_name,
          cpu_percent: props.node.cpu_percent || 0,
          cpu_peak: props.node.cpu_percent || 0,
          mem_used_bytes: props.node.memory_used || 0,
          mem_total_bytes: props.node.memory_total || 0,
          mem_percent: props.node.memory_percent || 0,
          disk_used_bytes: props.node.disk_used || 0,
          disk_total_bytes: props.node.disk_total || 0,
          disk_percent: props.node.disk_percent || 0,
          rx_bytes_per_sec: props.node.network_rx_bytes || 0,
          tx_bytes_per_sec: props.node.network_tx_bytes || 0,
          process_count: props.node.processes || 0,
          container_count: props.node.container_count || 0,
          status: props.node.status || 'ready',
          resolution: 'live',
          recorded_at: new Date().toISOString(),
        }]
      }
    }

    return base
  })

  const nodeHistoryChartCpuPath = computed(() => buildLinePath(nodeHistoryList.value, h => h.cpu_percent))
  const nodeHistoryChartCpuPeakPath = computed(() => buildLinePath(nodeHistoryList.value, h => Math.max(hPeak(h), h.cpu_percent)))
  const nodeHistoryChartCpuEnvelope = computed(() => buildCpuEnvelope(nodeHistoryList.value))
  const nodeHistorySpikeMarkers = computed(() => buildSpikeMarkers(nodeHistoryList.value))
  const nodeHistoryChartCpuArea = computed(() => buildAreaPath(nodeHistoryChartCpuPath.value, nodeHistoryList.value))
  const nodeHistoryChartMemPath = computed(() => buildLinePath(nodeHistoryList.value, h => h.mem_percent))
  const nodeHistoryChartMemArea = computed(() => buildAreaPath(nodeHistoryChartMemPath.value, nodeHistoryList.value))
  const nodeHistoryChartDiskPath = computed(() => buildLinePath(nodeHistoryList.value, h => h.disk_percent))
  const nodeHistoryTimeMarkers = computed(() => buildTimeMarkers(nodeHistoryList.value, props.nodeHistoryRange))

  const hoveredNodeHistPoint = computed<NodeMetricRollup | null>(() => {
    if (hoveredNodeHistIndex.value === null || hoveredNodeHistIndex.value < 0 || hoveredNodeHistIndex.value >= nodeHistoryList.value.length) {
      return null
    }
    return nodeHistoryList.value[hoveredNodeHistIndex.value]
  })

  const hoveredPointSuspect = computed<{ name: string; reason: string } | null>(() => {
    if (!hoveredNodeHistPoint.value) return null
    const pt = hoveredNodeHistPoint.value
    const pointTime = new Date(pt.recorded_at).getTime()

    if (!isNaN(pointTime) && props.nodeHistoryData?.incidents && props.nodeHistoryData.incidents.length > 0) {
      const windowMs = 15 * 60 * 1000
      for (const inc of props.nodeHistoryData.incidents) {
        const incTimeStr = (inc as any).recorded_at || inc.created_at || (inc as any).timestamp
        if (!incTimeStr) continue
        const incTime = new Date(incTimeStr).getTime()
        if (!isNaN(incTime) && Math.abs(incTime - pointTime) <= windowMs) {
          const sName = (inc as any).service_name || inc.pod_name || (inc as any).name
          const sTitle = (inc as any).title || inc.type || inc.message || 'Incident Event'
          if (sName) return { name: sName, reason: sTitle }
        }
      }
    }

    const nodeId = (props.node?.node_id || '').toLowerCase().trim()
    const nodeName = (props.node?.node_name || '').toLowerCase().trim()

    const nodeServices = (props.tpsData?.services || []).filter(s => {
      const sNodeId = (s.node_id || '').toLowerCase().trim()
      const sNodeName = (s.node_name || '').toLowerCase().trim()
      if (sNodeId && (sNodeId === nodeId || sNodeId === nodeName)) return true
      if (sNodeName && (sNodeName === nodeName || sNodeName === nodeId)) return true
      return false
    })

    interface OffenderCandidate {
      name: string
      cpu_percent: number
      mem_percent: number
      score: number
    }
    const candidates: OffenderCandidate[] = []

    for (const proc of props.node?.top_processes || []) {
      const pName = (proc.name || '').trim()
      if (pName && !pName.startsWith('[')) {
        const cpu = proc.cpu_percent || 0
        const mem = proc.memory_percent || 0
        candidates.push({ name: pName, cpu_percent: cpu, mem_percent: mem, score: Math.max(cpu, mem) })
      }
    }

    for (const s of nodeServices) {
      const sName = (s.service_name || '').trim()
      if (sName && !sName.startsWith('[')) {
        const cpu = s.cpu_percent || 0
        const mem = s.memory_percent || 0
        candidates.push({ name: sName, cpu_percent: cpu, mem_percent: mem, score: Math.max(cpu, mem) })
      }
    }

    if (candidates.length > 0) {
      candidates.sort((a, b) => b.score - a.score)
      const top = candidates[0]
      if (top && top.name) {
        return { name: top.name, reason: `CPU: ${Math.round(top.cpu_percent || 0)}%` }
      }
    }

    return null
  })

  const nodeHistHoverCoords = computed(() => {
    if (hoveredNodeHistIndex.value === null || !nodeHistoryList.value.length) return null
    const list = nodeHistoryList.value
    const step = list.length > 1 ? HIST_X_WIDTH / (list.length - 1) : 0
    const idx = hoveredNodeHistIndex.value
    const pt = list[idx]
    if (!pt) return null
    const x = list.length > 1 ? HIST_X_LEFT + idx * step : 385
    const peakVal = Math.max(hPeak(pt), pt.cpu_percent)
    const yCpu = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, pt.cpu_percent)) / 100) * HIST_Y_HEIGHT
    const yCpuPeak = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, peakVal)) / 100) * HIST_Y_HEIGHT
    const yMem = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, pt.mem_percent)) / 100) * HIST_Y_HEIGHT
    const yDisk = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, pt.disk_percent)) / 100) * HIST_Y_HEIGHT
    return { x, yCpu, yCpuPeak, yMem, yDisk }
  })

  const nodeHistTooltipStyle = computed(() => {
    if (hoveredNodeHistIndex.value === null) return { display: 'none' }
    const { x, y } = nodeHistTooltipPos.value
    const isRightSide = x > 380
    return {
      left: isRightSide ? `${x - 250}px` : `${x + 16}px`,
      top: `${Math.max(10, y - 40)}px`,
      pointerEvents: 'auto' as const,
    }
  })

  function handleNodeHistChartHover(event: MouseEvent) {
    const list = nodeHistoryList.value
    if (list.length === 0) return
    const target = event.currentTarget as HTMLElement
    if (!target) return
    const targetElement = event.target as HTMLElement
    if (targetElement && (targetElement.closest('.spike-pin-item') || targetElement.closest('.hist-rich-tooltip'))) {
      return
    }

    const rect = target.getBoundingClientRect()
    const mouseX = Math.max(0, Math.min(rect.width, event.clientX - rect.left))
    const mouseY = Math.max(0, Math.min(rect.height, event.clientY - rect.top))

    if (list.length === 1) {
      hoveredNodeHistIndex.value = 0
    } else {
      const scaleX = rect.width / 760
      const svgX = mouseX / scaleX
      const boundedSvgX = Math.max(HIST_X_LEFT, Math.min(HIST_X_RIGHT, svgX))
      const ratio = (boundedSvgX - HIST_X_LEFT) / HIST_X_WIDTH
      const idx = Math.round(ratio * (list.length - 1))
      hoveredNodeHistIndex.value = Math.max(0, Math.min(list.length - 1, idx))
    }
    isNodeHistHovered.value = true
    nodeHistTooltipPos.value = { x: mouseX, y: mouseY }
  }

  function handleNodeHistChartLeave(event?: MouseEvent) {
    if (event) {
      const related = event.relatedTarget as HTMLElement
      if (related && related.closest && related.closest('.hist-rich-tooltip')) {
        return
      }
    }
    isNodeHistHovered.value = false
    hoveredNodeHistIndex.value = null
  }

  function loadNodeHistory(_nodeId?: string, range = '1h', from?: string, to?: string) {
    let targetFrom = from !== undefined ? from : customHistoryFrom.value
    let targetTo = to !== undefined ? to : customHistoryTo.value

    if (range === 'custom') {
      if (!targetFrom || !targetTo || targetFrom === targetTo) {
        const dates = getSafeDefaultDates()
        targetFrom = dates.from
        targetTo = dates.to
        customHistoryFrom.value = targetFrom
        customHistoryTo.value = targetTo
      }
    }

    emit('update:nodeHistoryRange', range)
    emit('update:customHistFrom', targetFrom)
    emit('update:customHistTo', targetTo)
    emit('range-change', range, targetFrom, targetTo)
    if (range === 'custom') {
      emit('custom-range-apply')
    }
  }

  function syncLogsToPointInTime(point?: NodeMetricRollup | null) {
    const pt = point || hoveredNodeHistPoint.value
    if (pt) {
      const suspectName = hoveredPointSuspect.value?.name
      emit('sync-point-in-time', pt, suspectName)
    }
  }

  return {
    showHistCpu,
    showHistMem,
    showHistDisk,
    showHistPeakEnvelope,
    histViewMode,
    showCustomHistoryPicker,
    customHistoryFrom,
    customHistoryTo,
    hoveredNodeHistIndex,
    isNodeHistHovered,
    nodeHistTooltipPos,
    nodeHistoryWindowBadge,
    nodeHistoryList,
    nodeHistoryChartCpuPath,
    nodeHistoryChartCpuPeakPath,
    nodeHistoryChartCpuEnvelope,
    nodeHistorySpikeMarkers,
    nodeHistoryChartCpuArea,
    nodeHistoryChartMemPath,
    nodeHistoryChartMemArea,
    nodeHistoryChartDiskPath,
    nodeHistoryTimeMarkers,
    hoveredNodeHistPoint,
    hoveredPointSuspect,
    nodeHistHoverCoords,
    nodeHistTooltipStyle,
    toggleCustomHistoryPicker,
    switchNodeDrawerToHistory,
    applyPreset,
    handleNodeHistChartHover,
    handleNodeHistChartLeave,
    loadNodeHistory,
    syncLogsToPointInTime,
  }
}
