<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import CyberDateTimePicker from '../../ui/CyberDateTimePicker.vue'
import type { NodeMetrics, TpsSnapshot } from '../../../api/overview'
import type { NodeMetricRollup, NodeHistoryResponse } from '../../../api/compute'

interface Props {
  node: NodeMetrics
  tpsData?: TpsSnapshot | null
  nodeHistoryData?: NodeHistoryResponse | null
  nodeHistoryLoading?: boolean
  nodeHistoryRange?: string
  customHistFrom?: string
  customHistTo?: string
}

const props = withDefaults(defineProps<Props>(), {
  tpsData: null,
  nodeHistoryData: null,
  nodeHistoryLoading: false,
  nodeHistoryRange: '1h',
  customHistFrom: '',
  customHistTo: '',
})

const emit = defineEmits<{
  (e: 'update:nodeHistoryRange', range: string): void
  (e: 'update:customHistFrom', val: string): void
  (e: 'update:customHistTo', val: string): void
  (e: 'range-change', range: string, from?: string, to?: string): void
  (e: 'custom-range-apply'): void
  (e: 'apply-preset', preset: '30m' | '2h' | '6h' | 'today'): void
  (e: 'sync-point-in-time', point: NodeMetricRollup, suspectApp?: string): void
}>()

// Series visibility toggles
const showHistCpu = ref(true)
const showHistMem = ref(true)
const showHistDisk = ref(true)
const showHistPeakEnvelope = ref(true)
const histViewMode = ref<'both' | 'avg' | 'peak'>('both')
const showCustomHistoryPicker = ref(false)

const pad = (n: number) => String(n).padStart(2, '0')
const formatDt = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`

function getSafeDefaultDates() {
  const now = new Date()
  return {
    from: formatDt(new Date(now.getTime() - 24 * 60 * 60 * 1000)),
    to: formatDt(now),
  }
}

const safeInit = getSafeDefaultDates()
const customHistoryFrom = ref(props.customHistFrom || safeInit.from)
const customHistoryTo = ref(props.customHistTo || safeInit.to)

watch(() => props.customHistFrom, (val) => {
  if (val) {
    customHistoryFrom.value = val
  } else if (!customHistoryFrom.value) {
    customHistoryFrom.value = getSafeDefaultDates().from
  }
})
watch(() => props.customHistTo, (val) => {
  if (val) {
    customHistoryTo.value = val
  } else if (!customHistoryTo.value) {
    customHistoryTo.value = getSafeDefaultDates().to
  }
})

// Chart Hover State
const hoveredNodeHistIndex = ref<number | null>(null)
const isNodeHistHovered = ref(false)
const nodeHistTooltipPos = ref({ x: 0, y: 0 })

// Chart Coordinates
const HIST_Y_BOTTOM = 180
const HIST_Y_HEIGHT = 160 // 180 - 20
const HIST_X_LEFT = 45
const HIST_X_RIGHT = 725
const HIST_X_WIDTH = 680 // 725 - 45

const nodeHistoryWindowBadge = computed(() => {
  if (props.nodeHistoryRange === 'custom') {
    return 'CUSTOM WINDOW'
  }
  return `PAST ${props.nodeHistoryRange.toUpperCase()}`
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

// Synthetic / Live Rollup List
const nodeHistoryList = computed<NodeMetricRollup[]>(() => {
  const base = props.nodeHistoryData?.history || []
  if (base.length === 0) {
    if (!props.node) return []
    const now = new Date()
    const liveSample: NodeMetricRollup = {
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
    }
    return [liveSample]
  }

  if (props.node && base.length > 0) {
    const last = base[base.length - 1]
    const lastTime = new Date(last.recorded_at).getTime()
    const nowTime = Date.now()
    if (nowTime - lastTime > 15000 && props.nodeHistoryRange === '1h') {
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

const nodeHistoryChartCpuPath = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return ''
  if (list.length === 1) {
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, list[0].cpu_percent)) / 100) * HIST_Y_HEIGHT
    return `M ${HIST_X_LEFT} ${y.toFixed(1)} L ${HIST_X_RIGHT} ${y.toFixed(1)}`
  }
  const step = HIST_X_WIDTH / (list.length - 1)
  return list.map((h, i) => {
    const x = HIST_X_LEFT + i * step
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, h.cpu_percent)) / 100) * HIST_Y_HEIGHT
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

const nodeHistoryChartCpuPeakPath = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return ''
  if (list.length === 1) {
    const peakVal = Math.max(hPeak(list[0]), list[0].cpu_percent)
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, peakVal)) / 100) * HIST_Y_HEIGHT
    return `M ${HIST_X_LEFT} ${y.toFixed(1)} L ${HIST_X_RIGHT} ${y.toFixed(1)}`
  }
  const step = HIST_X_WIDTH / (list.length - 1)
  return list.map((h, i) => {
    const x = HIST_X_LEFT + i * step
    const peakVal = Math.max(hPeak(h), h.cpu_percent)
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, peakVal)) / 100) * HIST_Y_HEIGHT
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

function hPeak(h: NodeMetricRollup): number {
  return h.cpu_peak !== undefined && h.cpu_peak !== null ? h.cpu_peak : h.cpu_percent
}

const nodeHistoryChartCpuEnvelope = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return ''
  if (list.length === 1) {
    const peakVal = Math.max(hPeak(list[0]), list[0].cpu_percent)
    const avgVal = list[0].cpu_percent
    const yPeak = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, peakVal)) / 100) * HIST_Y_HEIGHT
    const yAvg = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, avgVal)) / 100) * HIST_Y_HEIGHT
    return `M ${HIST_X_LEFT} ${yPeak.toFixed(1)} L ${HIST_X_RIGHT} ${yPeak.toFixed(1)} L ${HIST_X_RIGHT} ${yAvg.toFixed(1)} L ${HIST_X_LEFT} ${yAvg.toFixed(1)} Z`
  }
  const step = HIST_X_WIDTH / (list.length - 1)
  const peakPoints = list.map((h, i) => {
    const x = HIST_X_LEFT + i * step
    const peakVal = Math.max(hPeak(h), h.cpu_percent)
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, peakVal)) / 100) * HIST_Y_HEIGHT
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
  const avgPointsRev = [...list].reverse().map((h, i) => {
    const origIdx = list.length - 1 - i
    const x = HIST_X_LEFT + origIdx * step
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, h.cpu_percent)) / 100) * HIST_Y_HEIGHT
    return `L ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
  return `${peakPoints} ${avgPointsRev} Z`
})

const nodeHistorySpikeMarkers = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return []
  const step = list.length > 1 ? HIST_X_WIDTH / (list.length - 1) : 0
  const markers: Array<{ index: number; x: number; y: number; time: string; peak: number; avg: number }> = []

  list.forEach((h, i) => {
    const peak = Math.max(hPeak(h), h.cpu_percent)
    const avg = h.cpu_percent || 0
    if (peak >= 75 || (peak - avg >= 20 && peak >= 50)) {
      const x = list.length > 1 ? HIST_X_LEFT + i * step : (HIST_X_LEFT + HIST_X_RIGHT) / 2
      const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, peak)) / 100) * HIST_Y_HEIGHT
      const d = new Date(h.recorded_at)
      const timeStr = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
      markers.push({
        index: i,
        x,
        y,
        time: timeStr,
        peak,
        avg,
      })
    }
  })
  return markers
})

const nodeHistoryChartCpuArea = computed(() => {
  if (!nodeHistoryChartCpuPath.value) return ''
  const list = nodeHistoryList.value
  const firstX = HIST_X_LEFT
  const lastX = list.length === 1 ? HIST_X_RIGHT : HIST_X_LEFT + (list.length - 1) * (HIST_X_WIDTH / (list.length - 1))
  return `${nodeHistoryChartCpuPath.value} L ${lastX.toFixed(1)} ${HIST_Y_BOTTOM} L ${firstX} ${HIST_Y_BOTTOM} Z`
})

const nodeHistoryChartMemPath = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return ''
  if (list.length === 1) {
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, list[0].mem_percent)) / 100) * HIST_Y_HEIGHT
    return `M ${HIST_X_LEFT} ${y.toFixed(1)} L ${HIST_X_RIGHT} ${y.toFixed(1)}`
  }
  const step = HIST_X_WIDTH / (list.length - 1)
  return list.map((h, i) => {
    const x = HIST_X_LEFT + i * step
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, h.mem_percent)) / 100) * HIST_Y_HEIGHT
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

const nodeHistoryChartMemArea = computed(() => {
  if (!nodeHistoryChartMemPath.value) return ''
  const list = nodeHistoryList.value
  const firstX = HIST_X_LEFT
  const lastX = list.length === 1 ? HIST_X_RIGHT : HIST_X_LEFT + (list.length - 1) * (HIST_X_WIDTH / (list.length - 1))
  return `${nodeHistoryChartMemPath.value} L ${lastX.toFixed(1)} ${HIST_Y_BOTTOM} L ${firstX} ${HIST_Y_BOTTOM} Z`
})

const nodeHistoryChartDiskPath = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return ''
  if (list.length === 1) {
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, list[0].disk_percent)) / 100) * HIST_Y_HEIGHT
    return `M ${HIST_X_LEFT} ${y.toFixed(1)} L ${HIST_X_RIGHT} ${y.toFixed(1)}`
  }
  const step = HIST_X_WIDTH / (list.length - 1)
  return list.map((h, i) => {
    const x = HIST_X_LEFT + i * step
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, h.disk_percent)) / 100) * HIST_Y_HEIGHT
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

const nodeHistoryTimeMarkers = computed(() => {
  const list = nodeHistoryList.value
  if (list.length === 0) return []
  if (list.length === 1) {
    const d = new Date(list[0].recorded_at)
    return [{ x: (HIST_X_LEFT + HIST_X_RIGHT) / 2, time: d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }]
  }
  const count = Math.min(6, list.length)
  const markers = []
  const step = HIST_X_WIDTH / (list.length - 1)
  const firstD = new Date(list[0].recorded_at)
  const lastD = new Date(list[list.length - 1].recorded_at)
  const spanMs = Math.abs(lastD.getTime() - firstD.getTime())
  const isMultiDay = spanMs > 24 * 60 * 60 * 1000 || props.nodeHistoryRange === '7d' || props.nodeHistoryRange === '30d'

  for (let i = 0; i < count; i++) {
    const idx = Math.round((i / (count - 1)) * (list.length - 1))
    const d = new Date(list[idx].recorded_at)
    const timeStr = isMultiDay
      ? `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:00`
      : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    markers.push({
      x: HIST_X_LEFT + idx * step,
      time: timeStr,
    })
  }
  return markers
})

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

  // 1. Check if an incident in props.nodeHistoryData?.incidents occurred within ±15 minutes of hoveredNodeHistPoint.recorded_at
  if (!isNaN(pointTime) && props.nodeHistoryData?.incidents && props.nodeHistoryData.incidents.length > 0) {
    const windowMs = 15 * 60 * 1000
    for (const inc of props.nodeHistoryData.incidents) {
      const incTimeStr = (inc as any).recorded_at || inc.created_at || (inc as any).timestamp
      if (!incTimeStr) continue
      const incTime = new Date(incTimeStr).getTime()
      if (!isNaN(incTime) && Math.abs(incTime - pointTime) <= windowMs) {
        const sName = (inc as any).service_name || inc.pod_name || (inc as any).name
        const sTitle = (inc as any).title || inc.type || inc.message || 'Incident Event'
        if (sName) {
          return { name: sName, reason: sTitle }
        }
      }
    }
  }

  // 2. Otherwise, check props.node?.top_processes and props.tpsData?.services for workloads running on this node:
  // Pick the workload/process with the highest CPU or Memory usage (ignoring kernel daemons starting with `[`).
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
      candidates.push({
        name: pName,
        cpu_percent: cpu,
        mem_percent: mem,
        score: Math.max(cpu, mem),
      })
    }
  }

  for (const s of nodeServices) {
    const sName = (s.service_name || '').trim()
    if (sName && !sName.startsWith('[')) {
      const cpu = s.cpu_percent || 0
      const mem = s.memory_percent || 0
      candidates.push({
        name: sName,
        cpu_percent: cpu,
        mem_percent: mem,
        score: Math.max(cpu, mem),
      })
    }
  }

  if (candidates.length > 0) {
    candidates.sort((a, b) => b.score - a.score)
    const top = candidates[0]
    if (top && top.name) {
      return {
        name: top.name,
        reason: `CPU: ${Math.round(top.cpu_percent || 0)}%`,
      }
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
  return `${Math.round(val)}%`
}

function formatIoRate(bytesPerSec?: number): string {
  if (!bytesPerSec || bytesPerSec <= 0 || isNaN(bytesPerSec)) return '0 B/s'
  return `${formatBytes(bytesPerSec)}/s`
}
</script>

<template>
  <div class="node-historical-container">
    <!-- 1. Top 4 KPI Summary Cards (ALWAYS visible at top) -->
    <div class="hist-kpi-grid">
      <!-- Card 1: Realtime CPU & Peak -->
      <div class="hist-kpi-card glass-panel">
        <span class="kpi-label">REALTIME CPU / PEAK</span>
        <span class="kpi-val text-violet">{{ formatPercent(node?.cpu_percent) }}</span>
        <span class="kpi-sub font-mono">
          🔥 Peak: {{ formatPercent(nodeHistoryData?.summary?.peak_cpu_percent || node?.cpu_percent) }} <span class="text-slate">| Avg: {{ formatPercent(nodeHistoryData?.summary?.avg_cpu_percent || node?.cpu_percent) }}</span>
        </span>
      </div>

      <!-- Card 2: Realtime RAM & Peak -->
      <div class="hist-kpi-card glass-panel">
        <span class="kpi-label">REALTIME RAM / PEAK</span>
        <span class="kpi-val text-cyan">{{ formatPercent(node?.memory_percent) }}</span>
        <span class="kpi-sub font-mono">
          🧠 {{ formatBytes(node?.memory_used) }} / {{ formatBytes(node?.memory_total) }} <span class="text-slate">(Peak: {{ formatPercent(nodeHistoryData?.summary?.peak_mem_percent || node?.memory_percent) }})</span>
        </span>
      </div>

      <!-- Card 3: Live Network I/O -->
      <div class="hist-kpi-card glass-panel">
        <span class="kpi-label">LIVE NETWORK I/O</span>
        <div class="hist-kpi-net-row font-mono">
          <div class="net-pill net-pill-rx">
            <span class="net-pill-arrow">↓</span>
            <span class="net-pill-val">{{ formatIoRate(node?.network_rx_bytes || 0) }}</span>
          </div>
          <div class="net-pill net-pill-tx">
            <span class="net-pill-arrow">↑</span>
            <span class="net-pill-val">{{ formatIoRate(node?.network_tx_bytes || 0) }}</span>
          </div>
        </div>
        <span class="kpi-sub font-mono">
          ⚡ Peak: <span class="text-emerald">↓ {{ formatIoRate(nodeHistoryData?.summary?.peak_rx_bytes_sec || node?.network_rx_bytes) }}</span> <span class="text-slate">·</span> <span class="text-cyan">↑ {{ formatIoRate(nodeHistoryData?.summary?.peak_tx_bytes_sec || node?.network_tx_bytes) }}</span>
        </span>
      </div>

      <!-- Card 4: Uptime & Disk Usage -->
      <div class="hist-kpi-card glass-panel">
        <span class="kpi-label">UPTIME & DISK USAGE</span>
        <span class="kpi-val text-emerald">
          {{ formatPercent(nodeHistoryData?.summary?.uptime_percent || 99) }}
        </span>
        <span class="kpi-sub font-mono">
          💾 Disk: {{ formatPercent(node?.disk_percent) }} <span class="text-slate">({{ formatBytes(node?.disk_used) }} / {{ formatBytes(node?.disk_total) }})</span>
        </span>
      </div>
    </div>

    <!-- 2. Historical Multi-Series Chart -->
    <div class="hist-chart-wrapper glass-panel">
      <!-- 2a. Chart Header with Title, Badge, and Series Toggles -->
      <div class="hist-chart-header">
        <div class="chart-title-group">
          <h4 class="hist-chart-title">📈 {{ node?.node_name || 'Node' }} Hardware Saturation Trends</h4>
          <span class="hist-chart-desc badge-history-window font-mono">{{ nodeHistoryWindowBadge }}</span>
        </div>

        <!-- Interactive Series Toggles with Live Values -->
        <div class="series-toggles-group" v-if="nodeHistoryList.length > 0">
          <button
            type="button"
            class="series-toggle-btn"
            :class="{ 'toggle-active cpu-active': showHistCpu }"
            @click="showHistCpu = !showHistCpu"
            title="Click to toggle CPU curve"
          >
            <span class="toggle-dot bg-violet"></span>
            <span>CPU: <strong>{{ formatPercent(node?.cpu_percent) }}</strong></span>
          </button>

          <button
            type="button"
            class="series-toggle-btn"
            :class="{ 'toggle-active peak-active': showHistPeakEnvelope }"
            @click="showHistPeakEnvelope = !showHistPeakEnvelope"
            title="Toggle Peak Spike Envelope layer"
          >
            <span class="toggle-dot bg-rose"></span>
            <span>🔥 Peak Envelope: <strong>{{ showHistPeakEnvelope ? 'ON' : 'OFF' }}</strong></span>
          </button>

          <button
            type="button"
            class="series-toggle-btn"
            :class="{ 'toggle-active mem-active': showHistMem }"
            @click="showHistMem = !showHistMem"
            title="Click to toggle RAM curve"
          >
            <span class="toggle-dot bg-cyan"></span>
            <span>RAM: <strong>{{ formatPercent(node?.memory_percent) }}</strong></span>
          </button>

          <button
            type="button"
            class="series-toggle-btn"
            :class="{ 'toggle-active disk-active reqs-active': showHistDisk }"
            @click="showHistDisk = !showHistDisk"
            title="Click to toggle Disk curve"
          >
            <span class="toggle-dot bg-emerald"></span>
            <span>Disk: <strong>{{ formatPercent(node?.disk_percent) }}</strong></span>
          </button>
        </div>
      </div>

      <!-- 2b. Integrated Time Range & Sample Bar -->
      <div class="hist-chart-time-bar">
        <div class="chart-time-pills">
          <button
            v-for="r in ['1h', '3h', '6h', '24h', '7d', '30d'] as const"
            :key="r"
            class="btn-chart-range-pill"
            :class="{ active: nodeHistoryRange === r && !showCustomHistoryPicker }"
            @click="switchNodeDrawerToHistory(r)"
          >
            {{ r }}
          </button>
          <button
            class="btn-chart-range-pill"
            :class="{ active: nodeHistoryRange === 'custom' || showCustomHistoryPicker }"
            @click="toggleCustomHistoryPicker"
          >
            📅 Custom
          </button>
        </div>
        <div class="range-right">
          <span class="badge badge-indigo font-mono" v-if="nodeHistoryData?.resolution">
            Sample Rate: {{ nodeHistoryData.resolution }}
          </span>
          <button class="btn btn-secondary btn-xs" @click="emit('range-change', nodeHistoryRange)" :disabled="nodeHistoryLoading">
            ↺ Refresh
          </button>
        </div>
      </div>

      <!-- 2c. Inline Glassmorphic Custom History Toolbar -->
      <div v-if="showCustomHistoryPicker || nodeHistoryRange === 'custom'" class="custom-range-bar glass-panel animate-fadeIn">
        <div class="custom-range-inputs">
          <div class="range-field">
            <label class="range-label font-mono">FROM:</label>
            <CyberDateTimePicker
              v-model="customHistoryFrom"
              placeholder="From datetime"
              @apply="loadNodeHistory(node?.node_id || node?.node_name, 'custom', customHistoryFrom, customHistoryTo)"
            />
          </div>
          <div class="range-field">
            <label class="range-label font-mono">TO:</label>
            <CyberDateTimePicker
              v-model="customHistoryTo"
              placeholder="To datetime"
              @apply="loadNodeHistory(node?.node_id || node?.node_name, 'custom', customHistoryFrom, customHistoryTo)"
            />
          </div>
        </div>

        <div class="custom-range-presets">
          <span class="preset-label font-mono">PRESETS:</span>
          <button type="button" class="btn-preset-chip font-mono" @click="applyPreset('30m')">Last 30m</button>
          <button type="button" class="btn-preset-chip font-mono" @click="applyPreset('2h')">Last 2h</button>
          <button type="button" class="btn-preset-chip font-mono" @click="applyPreset('6h')">Last 6h</button>
          <button type="button" class="btn-preset-chip font-mono" @click="applyPreset('today')">Today</button>
        </div>

        <div class="custom-range-actions">
          <button
            type="button"
            class="btn-apply-range font-mono"
            :disabled="nodeHistoryLoading"
            @click="loadNodeHistory(node?.node_id || node?.node_name, 'custom', customHistoryFrom, customHistoryTo)"
          >
            <span class="glow-dot"></span>
            <span>{{ nodeHistoryLoading ? 'Applying...' : 'Apply Window' }}</span>
          </button>
        </div>
      </div>

      <!-- 2d. SVG Chart Canvas / Loading / Empty -->
      <div v-if="nodeHistoryLoading" class="hist-chart-loading glass-panel">
        <span class="spinner-sm"></span> Loading {{ nodeHistoryRange }} telemetry rollups...
      </div>

      <div v-else-if="nodeHistoryList.length > 0" class="hist-chart-container" @mousemove="handleNodeHistChartHover" @mouseleave="handleNodeHistChartLeave">
        <!-- Main SVG Canvas -->
        <svg class="hist-svg-canvas" viewBox="0 0 760 200" preserveAspectRatio="none">
          <defs>
            <linearGradient id="histCpuGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="#a855f7" stop-opacity="0.35" />
              <stop offset="100%" stop-color="#a855f7" stop-opacity="0.0" />
            </linearGradient>
            <linearGradient id="histCpuPeakEnvelopeGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="#f43f5e" stop-opacity="0.45" />
              <stop offset="50%" stop-color="#e879f9" stop-opacity="0.25" />
              <stop offset="100%" stop-color="#a855f7" stop-opacity="0.05" />
            </linearGradient>
            <linearGradient id="histMemGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.3" />
              <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
            </linearGradient>
          </defs>

          <!-- Y-Axis Percentage Labels inside SVG (Fixed exact alignment) -->
          <g class="hist-svg-y-labels" font-size="9" text-anchor="end">
            <text x="44" y="23" fill="#fb7185" font-weight="600">100%</text>
            <text x="44" y="63" fill="#fbbf24" font-weight="600">75%</text>
            <text x="44" y="103" fill="#38bdf8" font-weight="600">50%</text>
            <text x="44" y="143" fill="#94a3b8">25%</text>
            <text x="44" y="183" fill="#64748b">0%</text>
          </g>

          <!-- Horizontal Grid Lines (Locked to exact scale: 20=100%, 60=75%, 100=50%, 140=25%, 180=0%) -->
          <line x1="50" y1="20" x2="720" y2="20" stroke="rgba(244,63,94,0.2)" stroke-dasharray="3,3" />
          <line x1="50" y1="60" x2="720" y2="60" stroke="rgba(245,158,11,0.2)" stroke-dasharray="3,3" />
          <line x1="50" y1="100" x2="720" y2="100" stroke="rgba(56,189,248,0.12)" stroke-dasharray="3,3" />
          <line x1="50" y1="140" x2="720" y2="140" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3,3" />
          <line x1="50" y1="180" x2="720" y2="180" stroke="rgba(255,255,255,0.12)" />

          <!-- Area Fills -->
          <path
            v-if="showHistCpu && showHistPeakEnvelope && (histViewMode === 'both' || histViewMode === 'peak') && nodeHistoryChartCpuEnvelope"
            :d="nodeHistoryChartCpuEnvelope"
            fill="url(#histCpuPeakEnvelopeGrad)"
            class="chart-envelope-area"
          />
          <path v-if="showHistCpu && (histViewMode === 'both' || histViewMode === 'avg') && nodeHistoryChartCpuArea" :d="nodeHistoryChartCpuArea" fill="url(#histCpuGrad)" />
          <path v-if="showHistMem && nodeHistoryChartMemArea" :d="nodeHistoryChartMemArea" fill="url(#histMemGrad)" />

          <!-- Trend Lines -->
          <path v-if="showHistDisk && nodeHistoryChartDiskPath" :d="nodeHistoryChartDiskPath" fill="none" stroke="#10b981" stroke-width="1.5" stroke-dasharray="4,4" />
          <path v-if="showHistMem && nodeHistoryChartMemPath" :d="nodeHistoryChartMemPath" fill="none" stroke="#06b6d4" stroke-width="2" />
          <path
            v-if="showHistCpu && (histViewMode === 'both' || histViewMode === 'peak') && (showHistPeakEnvelope || histViewMode === 'peak') && nodeHistoryChartCpuPeakPath"
            :d="nodeHistoryChartCpuPeakPath"
            fill="none"
            stroke="#e879f9"
            stroke-width="1.2"
            stroke-dasharray="3,2"
            class="chart-peak-line"
          />
          <path v-if="showHistCpu && (histViewMode === 'both' || histViewMode === 'avg') && nodeHistoryChartCpuPath" :d="nodeHistoryChartCpuPath" fill="none" stroke="#a855f7" stroke-width="2" />

          <!-- Glowing Spike Pins on Prominent Spikes -->
          <g v-if="showHistCpu && (showHistPeakEnvelope || histViewMode === 'peak')" class="spike-markers-group">
            <g
              v-for="marker in nodeHistorySpikeMarkers"
              :key="'spike-' + marker.index"
              class="spike-pin-item"
              @mouseenter="hoveredNodeHistIndex = marker.index; isNodeHistHovered = true"
              @click.stop="syncLogsToPointInTime(nodeHistoryList[marker.index])"
            >
              <!-- Outer pulsing ring for critical spike >= 85% -->
              <circle
                v-if="marker.peak >= 85"
                :cx="marker.x"
                :cy="marker.y"
                r="7"
                fill="none"
                stroke="#f43f5e"
                stroke-width="1.5"
                class="spike-marker-pulse"
              />
              <!-- Core dot -->
              <circle
                :cx="marker.x"
                :cy="marker.y"
                :r="marker.peak >= 85 ? 4 : 3"
                :fill="marker.peak >= 85 ? '#f43f5e' : '#e879f9'"
                stroke="#ffffff"
                stroke-width="1.2"
              />
            </g>
          </g>

          <!-- Interactive Crosshair -->
          <g v-if="isNodeHistHovered && nodeHistHoverCoords">
            <line
              :x1="nodeHistHoverCoords.x"
              y1="15"
              :x2="nodeHistHoverCoords.x"
              y2="185"
              stroke="#38bdf8"
              stroke-width="1.5"
              stroke-dasharray="3,3"
            />
            <circle v-if="showHistCpu && (histViewMode === 'both' || histViewMode === 'avg')" :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yCpu" r="4.5" fill="#a855f7" stroke="#ffffff" stroke-width="2" />
            <circle v-if="showHistCpu && (showHistPeakEnvelope || histViewMode === 'peak') && hoveredNodeHistPoint && (hoveredNodeHistPoint.cpu_peak || 0) > hoveredNodeHistPoint.cpu_percent" :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yCpuPeak" r="4" fill="#e879f9" stroke="#ffffff" stroke-width="1.5" />
            <circle v-if="showHistMem" :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yMem" r="4" fill="#06b6d4" stroke="#ffffff" stroke-width="1.5" />
            <circle v-if="showHistDisk" :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yDisk" r="3.5" fill="#10b981" stroke="#ffffff" stroke-width="1.5" />
          </g>
        </svg>

        <!-- Floating Tooltip Box (Color-Matched with Line Legends) -->
        <div v-if="isNodeHistHovered && hoveredNodeHistPoint" class="hist-rich-tooltip" :style="nodeHistTooltipStyle">
          <div class="tooltip-time-header">
            🕒 {{ new Date(hoveredNodeHistPoint.recorded_at).toLocaleString() }}
          </div>
          <div class="tooltip-series-row" v-if="showHistCpu">
            <span class="tooltip-dot dot-violet"></span>
            <span class="tooltip-label">CPU:</span>
            <strong class="tooltip-val text-violet">
              {{ formatPercent(hoveredNodeHistPoint.cpu_percent) }}
              <span v-if="(hoveredNodeHistPoint.cpu_peak || 0) > hoveredNodeHistPoint.cpu_percent" class="tooltip-peak-highlight">
                (🔥 Peak: {{ formatPercent(hoveredNodeHistPoint.cpu_peak) }})
              </span>
            </strong>
          </div>
          <div v-if="showHistCpu && (hoveredNodeHistPoint.cpu_peak || 0) >= 85" class="tooltip-spike-alert">
            <span class="spike-alert-tag">⚠️ Critical Spike: {{ formatPercent(hoveredNodeHistPoint.cpu_peak) }}</span>
          </div>
          <div v-if="hoveredPointSuspect" class="tooltip-suspect-row">
            <span class="tooltip-suspect-tag">
              🔥 Top Offender: <strong class="text-rose">{{ hoveredPointSuspect.name }}</strong> ({{ hoveredPointSuspect.reason }})
            </span>
          </div>
          <div class="tooltip-series-row" v-if="showHistMem">
            <span class="tooltip-dot dot-cyan"></span>
            <span class="tooltip-label">Memory:</span>
            <strong class="tooltip-val text-cyan">{{ formatPercent(hoveredNodeHistPoint.mem_percent) }} ({{ formatBytes(hoveredNodeHistPoint.mem_used_bytes) }})</strong>
          </div>
          <div class="tooltip-series-row" v-if="showHistDisk">
            <span class="tooltip-dot dot-emerald"></span>
            <span class="tooltip-label">Disk:</span>
            <strong class="tooltip-val text-emerald">{{ formatPercent(hoveredNodeHistPoint.disk_percent) }} ({{ formatBytes(hoveredNodeHistPoint.disk_used_bytes) }})</strong>
          </div>
          <div class="tooltip-series-row">
            <span class="tooltip-dot dot-indigo"></span>
            <span class="tooltip-label">Net I/O:</span>
            <strong class="tooltip-val text-indigo">↓ {{ formatIoRate(hoveredNodeHistPoint.rx_bytes_per_sec) }} ↑ {{ formatIoRate(hoveredNodeHistPoint.tx_bytes_per_sec) }}</strong>
          </div>
          <button
            type="button"
            class="btn-pit-sync font-mono"
            @click.stop="syncLogsToPointInTime(hoveredNodeHistPoint)"
            title="Sync live and historical failure logs to this point in time (±15 minutes)"
          >
            <span>🔍</span> View Logs at this time
          </button>
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
          Historical metrics are aggregated periodically by the background collector. Once continuous telemetry is recorded for <code>{{ node?.node_name }}</code>, multi-series saturation curves will display here.
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.node-historical-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
}

/* 4 Summary KPI Badges */
.hist-kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 0;
}

@media (max-width: 900px) {
  .hist-kpi-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .hist-kpi-grid {
    grid-template-columns: 1fr;
  }
}

.hist-kpi-card {
  padding: 14px 16px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  min-height: 96px;
  gap: 6px;
  transition: all 0.2s ease;
}

.hist-kpi-card:hover {
  border-color: rgba(56, 189, 248, 0.25);
  transform: translateY(-1px);
}

.hist-kpi-card .kpi-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted, #94a3b8);
}

.hist-kpi-card .kpi-val {
  font-size: 20px;
  font-weight: 800;
  font-family: var(--font-mono, monospace);
  line-height: 1.2;
}

.hist-kpi-card .kpi-sub {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.hist-kpi-net-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 2px 0;
}

.net-pill {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 3px 6px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
  min-width: 0;
}

.net-pill-rx {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.35);
  color: #34d399;
}

.net-pill-tx {
  background: rgba(6, 182, 212, 0.15);
  border: 1px solid rgba(6, 182, 212, 0.35);
  color: #22d3ee;
}

.net-pill-arrow {
  font-size: 11px;
  font-weight: 800;
}

.net-pill-val {
  font-size: 11px;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Historical Chart Wrapper */
.hist-chart-wrapper {
  padding: 16px 18px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.hist-chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
}

.chart-title-group h4 {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary, #f8fafc);
  margin: 0;
}

.chart-title-group .hist-chart-desc {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.badge-history-window {
  font-size: 11px;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.1);
  padding: 2px 8px;
  border-radius: 6px;
  border: 1px solid rgba(56, 189, 248, 0.25);
  display: inline-block;
  margin-top: 3px;
}

/* Series Toggles */
.series-toggles-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.series-toggle-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  color: var(--text-muted, #94a3b8);
  font-size: 11.5px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.series-toggle-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary, #f8fafc);
}

.series-toggle-btn.toggle-active.cpu-active {
  background: rgba(168, 85, 247, 0.15);
  border-color: rgba(168, 85, 247, 0.4);
  color: #c084fc;
}

.series-toggle-btn.toggle-active.peak-active {
  background: rgba(244, 63, 94, 0.15);
  border-color: rgba(244, 63, 94, 0.4);
  color: #fb7185;
}

.series-toggle-btn.toggle-active.mem-active {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.4);
  color: #38bdf8;
}

.series-toggle-btn.toggle-active.reqs-active,
.series-toggle-btn.toggle-active.disk-active {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.4);
  color: #34d399;
}

.toggle-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.bg-violet { background-color: #a855f7; }
.bg-rose { background-color: #f43f5e; }
.bg-cyan { background-color: #06b6d4; }
.bg-emerald { background-color: #10b981; }

/* Time Range Bar */
.hist-chart-time-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  padding: 8px 0;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  margin: 4px 0 8px 0;
}

.chart-time-pills {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.btn-chart-range-pill {
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11.5px;
  font-weight: 600;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-secondary, #94a3b8);
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-chart-range-pill:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary, #f8fafc);
  border-color: rgba(56, 189, 248, 0.3);
}

.btn-chart-range-pill.active {
  background: rgba(56, 189, 248, 0.2);
  color: #38bdf8;
  border-color: #38bdf8;
  font-weight: 700;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.2);
}

.range-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* Custom Range Toolbar */
.custom-range-bar {
  position: relative;
  z-index: 50;
  overflow: visible;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 10px 14px;
  background: rgba(15, 23, 42, 0.7);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(56, 189, 248, 0.25);
  border-radius: 8px;
  margin: 4px 0 8px 0;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
}

.custom-range-inputs {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.range-field {
  display: flex;
  align-items: center;
  gap: 6px;
}

.range-label {
  font-size: 10.5px;
  font-weight: 700;
  color: #38bdf8;
  letter-spacing: 0.05em;
}

.input-datetime {
  color-scheme: dark;
  background: rgba(0, 0, 0, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  color: #38bdf8;
  font-size: 11.5px;
  padding: 5px 8px;
  outline: none;
  transition: all 0.2s ease;
}

.input-datetime:focus {
  border-color: #38bdf8;
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.2);
  background: rgba(0, 0, 0, 0.7);
}

.custom-range-presets {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.preset-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted, #94a3b8);
}

.btn-preset-chip {
  padding: 3px 8px;
  font-size: 11px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: var(--text-secondary, #94a3b8);
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-preset-chip:hover {
  background: rgba(56, 189, 248, 0.15);
  border-color: rgba(56, 189, 248, 0.4);
  color: #38bdf8;
}

.custom-range-actions {
  display: flex;
  align-items: center;
}

.btn-apply-range {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  background: linear-gradient(135deg, rgba(14, 165, 233, 0.3) 0%, rgba(56, 189, 248, 0.15) 100%);
  border: 1px solid rgba(56, 189, 248, 0.6);
  border-radius: 6px;
  color: #38bdf8;
  font-size: 11.5px;
  font-weight: 700;
  cursor: pointer;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.25);
  transition: all 0.2s ease;
}

.btn-apply-range:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(14, 165, 233, 0.5) 0%, rgba(56, 189, 248, 0.3) 100%);
  border-color: #38bdf8;
  box-shadow: 0 0 14px rgba(56, 189, 248, 0.45);
  transform: translateY(-1px);
}

.btn-apply-range:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.glow-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #38bdf8;
  box-shadow: 0 0 6px #38bdf8;
}

/* Loading & Empty States */
.hist-chart-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 180px;
  border-radius: 10px;
  color: var(--text-muted, #94a3b8);
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
  border: 1px dashed var(--border-subtle, rgba(255, 255, 255, 0.1));
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
  color: var(--text-secondary, #94a3b8);
  margin: 0;
}

.empty-chart-desc {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  max-width: 480px;
  margin: 0;
  line-height: 1.5;
}

/* SVG Chart Container */
.hist-chart-container {
  position: relative;
  width: 100%;
  height: 220px;
  background: rgba(10, 15, 30, 0.6);
  border-radius: 10px;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  overflow: hidden;
}

.hist-svg-canvas {
  width: 100%;
  height: 100%;
  cursor: crosshair;
}

.hist-svg-y-labels text {
  font-family: var(--font-mono, monospace);
  user-select: none;
}

.hist-x-axis {
  position: absolute;
  left: 50px;
  right: 40px;
  bottom: 4px;
  display: flex;
  justify-content: space-between;
  pointer-events: none;
}

.hist-x-axis .x-tick {
  font-size: 9px;
  font-family: monospace;
  color: var(--text-muted, #64748b);
}

/* Hover Tooltip */
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
  gap: 5px;
  min-width: 200px;
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
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-violet {
  background-color: #a855f7 !important;
  box-shadow: 0 0 6px rgba(168, 85, 247, 0.7);
}

.dot-cyan {
  background-color: #06b6d4 !important;
  box-shadow: 0 0 6px rgba(6, 182, 212, 0.7);
}

.dot-emerald {
  background-color: #10b981 !important;
  box-shadow: 0 0 6px rgba(16, 185, 129, 0.7);
}

.dot-indigo {
  background-color: #6366f1 !important;
  box-shadow: 0 0 6px rgba(99, 102, 241, 0.7);
}

.text-violet {
  color: #c084fc !important;
}

.text-cyan {
  color: #38bdf8 !important;
}

.text-emerald {
  color: #34d399 !important;
}

.text-indigo {
  color: #818cf8 !important;
}

.text-slate {
  color: #64748b !important;
}

.tooltip-label {
  color: var(--text-muted, #94a3b8);
}

.tooltip-val {
  font-family: monospace;
  margin-left: auto;
}

.btn-pit-sync {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  margin-top: 6px;
  padding: 5px 10px;
  background: rgba(56, 189, 248, 0.15);
  border: 1px solid rgba(56, 189, 248, 0.4);
  border-radius: 6px;
  color: #38bdf8;
  font-size: 10.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  pointer-events: auto;
}

.btn-pit-sync:hover {
  background: rgba(56, 189, 248, 0.3);
  border-color: #38bdf8;
  color: #ffffff;
  box-shadow: 0 0 8px rgba(56, 189, 248, 0.4);
  transform: translateY(-1px);
}

.chart-peak-line {
  filter: drop-shadow(0 0 3px rgba(232, 121, 249, 0.6));
  pointer-events: none;
}

.chart-envelope-area {
  transition: opacity 0.3s ease;
  pointer-events: none;
}

.spike-marker-pulse {
  animation: spikePulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
  transform-origin: center;
}

@keyframes spikePulse {
  0%, 100% {
    r: 6;
    opacity: 0.85;
    stroke-width: 1.5;
  }
  50% {
    r: 9;
    opacity: 0.2;
    stroke-width: 2.5;
  }
}

.spike-pin-item {
  cursor: pointer;
}

.tooltip-peak-highlight {
  color: #f472b6 !important;
  font-weight: 700;
  margin-left: 3px;
}

.tooltip-spike-alert {
  margin-top: 2px;
  display: flex;
}

.spike-alert-tag {
  font-size: 10px;
  font-weight: 700;
  color: #fecdd3;
  background: rgba(225, 29, 72, 0.3);
  border: 1px solid rgba(244, 63, 94, 0.5);
  padding: 2px 6px;
  border-radius: 4px;
  letter-spacing: 0.02em;
}

.tooltip-suspect-row {
  margin-top: 3px;
  display: flex;
}

.tooltip-suspect-tag {
  font-size: 10.5px;
  font-weight: 600;
  color: #fda4af;
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.4);
  padding: 2px 7px;
  border-radius: 4px;
  letter-spacing: 0.01em;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  word-break: break-word;
}

.text-rose {
  color: #fb7185;
}

.btn-xs {
  padding: 3px 8px;
  font-size: 11px;
}
</style>
