<script setup lang="ts">
import { ref, computed } from 'vue'
import type { NodeMetrics, TpsSnapshot } from '../../../api/overview'
import ModalDrawer from '../../ui/ModalDrawer.vue'

interface TrendPoint {
  time: string
  cpu: number
  mem: number
  disk: number
  reqs: number
}

export interface DeepDiveServiceItem {
  service_name: string
  node_name: string
  requests_per_sec: number
  traffic_percent: number
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  total_rx_bytes?: number
  total_tx_bytes?: number
  cpu_percent: number
  memory_used_mb: number
  status: 'healthy' | 'degraded' | 'down'
  container_count: number
}

interface Props {
  show: boolean
  trendHistory: TrendPoint[]
  nodes: NodeMetrics[]
  tpsData?: TpsSnapshot | null
  dockerContainers?: any[]
}

const props = withDefaults(defineProps<Props>(), {
  show: false,
  tpsData: null,
  dockerContainers: () => [],
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:show', value: boolean): void
}>()

function handleClose() {
  emit('close')
  emit('update:show', false)
}

// Modal Chart Series Toggles
const modalShowCpu = ref(true)
const modalShowMem = ref(true)
const modalShowReqs = ref(true)

// Modal Chart Hover Crosshair & Tooltip
const hoveredModalIndex = ref<number | null>(null)
const modalTooltipPos = ref({ x: 0, y: 0 })

// Modal Services Table State
const modalServiceSearch = ref('')
const modalServiceSortBy = ref<'name' | 'node' | 'traffic' | 'rps' | 'bandwidth' | 'cpu' | 'mem' | 'status'>('traffic')
const modalServiceSortOrder = ref<'asc' | 'desc'>('desc')

// Summary Statistics
const peakThroughput = computed(() => {
  if (!props.trendHistory.length) return 0
  const maxReq = Math.max(...props.trendHistory.map(p => p.reqs || 0))
  const tpsMax = props.tpsData?.http?.requests_per_sec || 0
  return Math.max(maxReq, tpsMax)
})

const avgThroughput = computed(() => {
  if (!props.trendHistory.length) return props.tpsData?.http?.requests_per_sec || 0
  const sum = props.trendHistory.reduce((acc, p) => acc + (p.reqs || 0), 0)
  return sum / props.trendHistory.length
})

const peakCpu = computed(() => {
  if (!props.trendHistory.length) return 0
  return Math.max(...props.trendHistory.map(p => p.cpu || 0))
})

const avgCpu = computed(() => {
  if (!props.trendHistory.length) return 0
  const sum = props.trendHistory.reduce((acc, p) => acc + (p.cpu || 0), 0)
  return sum / props.trendHistory.length
})

const peakRam = computed(() => {
  if (!props.trendHistory.length) return 0
  return Math.max(...props.trendHistory.map(p => p.mem || 0))
})

const avgRam = computed(() => {
  if (!props.trendHistory.length) return 0
  const sum = props.trendHistory.reduce((acc, p) => acc + (p.mem || 0), 0)
  return sum / props.trendHistory.length
})

// Modal High-Res Chart Computeds
const maxModalReqs = computed(() => {
  return Math.max(10, ...props.trendHistory.map(p => p.reqs || 0), Math.round(props.tpsData?.http?.requests_per_sec || 0))
})

const modalChartCpuPath = computed(() => {
  const history = props.trendHistory
  if (history.length < 2) return ''
  const step = 650 / (history.length - 1)
  return history.map((h, i) => {
    const x = 50 + i * step
    const y = 190 - (Math.min(100, Math.max(0, h.cpu)) / 100) * 170
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

const modalChartCpuArea = computed(() => {
  if (!modalChartCpuPath.value) return ''
  const history = props.trendHistory
  const firstX = 50
  const lastX = 50 + (history.length - 1) * (650 / (history.length - 1))
  return `${modalChartCpuPath.value} L ${lastX.toFixed(1)} 190 L ${firstX} 190 Z`
})

const modalChartMemPath = computed(() => {
  const history = props.trendHistory
  if (history.length < 2) return ''
  const step = 650 / (history.length - 1)
  return history.map((h, i) => {
    const x = 50 + i * step
    const y = 190 - (Math.min(100, Math.max(0, h.mem)) / 100) * 170
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

const modalChartMemArea = computed(() => {
  if (!modalChartMemPath.value) return ''
  const history = props.trendHistory
  const firstX = 50
  const lastX = 50 + (history.length - 1) * (650 / (history.length - 1))
  return `${modalChartMemPath.value} L ${lastX.toFixed(1)} 190 L ${firstX} 190 Z`
})

const modalChartReqsPath = computed(() => {
  const history = props.trendHistory
  if (history.length < 2) return ''
  const step = 650 / (history.length - 1)
  const maxR = maxModalReqs.value
  return history.map((h, i) => {
    const x = 50 + i * step
    const y = 190 - (Math.min(maxR, Math.max(0, h.reqs)) / maxR) * 170
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
})

const modalChartReqsArea = computed(() => {
  if (!modalChartReqsPath.value) return ''
  const history = props.trendHistory
  const firstX = 50
  const lastX = 50 + (history.length - 1) * (650 / (history.length - 1))
  return `${modalChartReqsPath.value} L ${lastX.toFixed(1)} 190 L ${firstX} 190 Z`
})

const hoveredModalPoint = computed<TrendPoint | null>(() => {
  if (hoveredModalIndex.value === null || hoveredModalIndex.value < 0 || hoveredModalIndex.value >= props.trendHistory.length) {
    return null
  }
  return props.trendHistory[hoveredModalIndex.value]
})

const hoveredModalElapsed = computed<string>(() => {
  if (hoveredModalIndex.value === null) return ''
  const offsetFromEnd = (props.trendHistory.length - 1 - hoveredModalIndex.value) * 10
  if (offsetFromEnd === 0) return 'Now (Live)'
  const mins = Math.floor(offsetFromEnd / 60)
  const secs = offsetFromEnd % 60
  if (mins === 0) return `${secs}s ago`
  return `${mins}m ${secs > 0 ? secs + 's' : ''} ago`
})

const modalHoverCoords = computed(() => {
  if (hoveredModalIndex.value === null || !props.trendHistory.length) return null
  const history = props.trendHistory
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
  if (hoveredModalIndex.value === null) return { display: 'none' }
  const { x, y } = modalTooltipPos.value
  const isRightSide = x > 380
  return {
    left: isRightSide ? `${x - 240}px` : `${x + 16}px`,
    top: `${Math.max(10, y - 40)}px`,
    pointerEvents: 'none' as const,
  }
})

const modalTimeAxisMarkers = computed(() => {
  const history = props.trendHistory
  if (history.length < 2) return []
  const count = Math.min(5, history.length)
  const markers = []
  const step = 650 / (history.length - 1)
  for (let i = 0; i < count; i++) {
    const idx = Math.round((i / (count - 1)) * (history.length - 1))
    const pt = history[idx]
    if (pt) {
      markers.push({
        x: 50 + idx * step,
        time: pt.time,
      })
    }
  }
  return markers
})

function handleModalChartHover(event: MouseEvent) {
  const history = props.trendHistory
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
}

function handleModalChartLeave() {
  hoveredModalIndex.value = null
}

// Services Breakdown Computation
const allClusterServices = computed<DeepDiveServiceItem[]>(() => {
  const nodeMap = new Map<string, string>()
  for (const n of props.nodes) {
    nodeMap.set(n.node_id.toLowerCase(), n.node_name)
    nodeMap.set(n.node_name.toLowerCase(), n.node_name)
  }

  let list: DeepDiveServiceItem[] = []

  if (props.tpsData?.services && props.tpsData.services.length > 0) {
    list = props.tpsData.services.map(s => {
      const nId = (s.node_id || '').toLowerCase()
      const nName = (s.node_name || '').toLowerCase()
      const resolvedNodeName = nodeMap.get(nId) || nodeMap.get(nName) || s.node_name || s.node_id || 'k8smaster'
      return {
        service_name: s.service_name,
        node_name: resolvedNodeName,
        requests_per_sec: s.requests_per_sec || 0,
        traffic_percent: 0,
        rx_bytes_per_sec: s.rx_bytes_per_sec || 0,
        tx_bytes_per_sec: s.tx_bytes_per_sec || 0,
        total_rx_bytes: s.total_rx_bytes || 0,
        total_tx_bytes: s.total_tx_bytes || 0,
        cpu_percent: s.cpu_percent || 0,
        memory_used_mb: s.memory_used_mb || 0,
        status: (s.status as any) || 'healthy',
        container_count: s.container_count || 1,
      }
    })
  } else if (props.dockerContainers && props.dockerContainers.length > 0) {
    list = props.dockerContainers.map(c => {
      const nId = (c.node_id || '').toLowerCase()
      const resolvedNodeName = nodeMap.get(nId) || c.node_id || 'k8smaster'
      const names = c.names || (c.name ? [c.name] : ['unknown-service'])
      const cleanName = (names[0] || 'service').replace(/^\//, '')
      return {
        service_name: cleanName,
        node_name: resolvedNodeName,
        requests_per_sec: 0,
        traffic_percent: 0,
        rx_bytes_per_sec: 0,
        tx_bytes_per_sec: 0,
        total_rx_bytes: 0,
        total_tx_bytes: 0,
        cpu_percent: c.cpu_percent || 0,
        memory_used_mb: c.memory_usage ? Math.round(c.memory_usage / (1024 * 1024)) : 0,
        status: c.state === 'running' ? 'healthy' : 'degraded',
        container_count: 1,
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
    } else {
      trafficShare = list.length > 0 ? 100 / list.length : 0
    }
    return {
      ...s,
      traffic_percent: trafficShare,
    }
  })
})

const filteredAndSortedClusterServices = computed(() => {
  let list = [...allClusterServices.value]

  // Filter
  const q = modalServiceSearch.value.toLowerCase().trim()
  if (q) {
    list = list.filter(s =>
      s.service_name.toLowerCase().includes(q) ||
      s.node_name.toLowerCase().includes(q)
    )
  }

  // Sort
  const sortKey = modalServiceSortBy.value
  const isAsc = modalServiceSortOrder.value === 'asc'
  const multiplier = isAsc ? 1 : -1

  list.sort((a, b) => {
    if (sortKey === 'name') {
      return a.service_name.localeCompare(b.service_name) * multiplier
    }
    if (sortKey === 'node') {
      return a.node_name.localeCompare(b.node_name) * multiplier
    }
    if (sortKey === 'traffic') {
      return (a.traffic_percent - b.traffic_percent) * multiplier
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

function formatBytes(bytes?: number, decimals = 1): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i] || 'B'}`
}

function formatIoRate(bytesPerSec?: number): string {
  if (!bytesPerSec || bytesPerSec <= 0 || isNaN(bytesPerSec)) return '0 B/s'
  return `${formatBytes(bytesPerSec)}/s`
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    maxWidth="1140px"
    title="Cluster Ingress & Request Saturation Deep-Dive"
    subtitle="High-resolution time-series saturation curves and multi-service traffic contributor breakdown"
    @close="handleClose"
  >
    <div class="deep-dive-body">
      <!-- SECTION 1: Historical Summary Statistics Cards -->
      <section class="modal-summary-grid">
        <!-- Card 1: Peak Throughput -->
        <div class="deep-stat-card glass-panel">
          <div class="deep-stat-header">
            <span class="deep-stat-title">Peak Gateway Throughput</span>
            <span class="hud-icon">⚡</span>
          </div>
          <div class="deep-stat-body">
            <span class="deep-stat-value text-cyan font-mono font-bold">
              {{ peakThroughput >= 100 ? Math.round(peakThroughput).toLocaleString() : (peakThroughput > 0 ? peakThroughput.toFixed(1) : '0.0') }}
            </span>
            <span class="deep-stat-unit">req / sec</span>
          </div>
          <div class="deep-stat-sub font-mono">
            <span>Window High (5-min buffer)</span>
          </div>
        </div>

        <!-- Card 2: Average Throughput -->
        <div class="deep-stat-card glass-panel">
          <div class="deep-stat-header">
            <span class="deep-stat-title">Average Gateway Throughput</span>
            <span class="hud-icon">🌐</span>
          </div>
          <div class="deep-stat-body">
            <span class="deep-stat-value text-emerald font-mono font-bold">
              {{ avgThroughput >= 100 ? Math.round(avgThroughput).toLocaleString() : (avgThroughput > 0 ? avgThroughput.toFixed(1) : '0.0') }}
            </span>
            <span class="deep-stat-unit">req / sec</span>
          </div>
          <div class="deep-stat-sub font-mono">
            <span>Cluster-wide mean rate</span>
          </div>
        </div>

        <!-- Card 3: Peak CPU Saturation -->
        <div class="deep-stat-card glass-panel">
          <div class="deep-stat-header">
            <span class="deep-stat-title">Peak CPU Saturation</span>
            <span class="hud-icon">🔥</span>
          </div>
          <div class="deep-stat-body">
            <span
              class="deep-stat-value font-mono font-bold"
              :class="peakCpu >= 85 ? 'text-rose' : peakCpu >= 65 ? 'text-amber' : 'text-violet'"
            >
              {{ Math.round(peakCpu) }}%
            </span>
            <span class="deep-stat-unit">Avg {{ Math.round(avgCpu) }}%</span>
          </div>
          <div class="deep-stat-sub font-mono">
            <span>5-minute envelope max</span>
          </div>
        </div>

        <!-- Card 4: Average RAM Usage -->
        <div class="deep-stat-card glass-panel">
          <div class="deep-stat-header">
            <span class="deep-stat-title">Peak Memory Pressure</span>
            <span class="hud-icon">🧠</span>
          </div>
          <div class="deep-stat-body">
            <span
              class="deep-stat-value font-mono font-bold"
              :class="peakRam >= 85 ? 'text-rose' : peakRam >= 65 ? 'text-amber' : 'text-cyan'"
            >
              {{ Math.round(peakRam) }}%
            </span>
            <span class="deep-stat-unit">Avg {{ Math.round(avgRam) }}%</span>
          </div>
          <div class="deep-stat-sub font-mono">
            <span>Cluster-wide memory mean</span>
          </div>
        </div>
      </section>

      <!-- SECTION 2: High-Resolution Expanded Multi-Series Chart -->
      <section class="modal-chart-section glass-panel">
        <div class="modal-chart-top-bar">
          <div class="chart-title-group">
            <h4 class="modal-section-title">
              <span>📈 High-Resolution Telemetry Overlay</span>
            </h4>
            <span class="trend-chart-subtitle">
              Time-synchronized CPU %, RAM %, and Throughput RPS (30 rolling samples)
            </span>
          </div>

          <!-- Series Toggles -->
          <div class="series-toggles-group">
            <button
              type="button"
              class="series-toggle-btn"
              :class="{ 'toggle-active cpu-active': modalShowCpu }"
              @click="modalShowCpu = !modalShowCpu"
            >
              <span class="toggle-dot bg-violet"></span>
              <span>CPU %</span>
            </button>

            <button
              type="button"
              class="series-toggle-btn"
              :class="{ 'toggle-active mem-active': modalShowMem }"
              @click="modalShowMem = !modalShowMem"
            >
              <span class="toggle-dot bg-cyan"></span>
              <span>RAM %</span>
            </button>

            <button
              type="button"
              class="series-toggle-btn"
              :class="{ 'toggle-active reqs-active': modalShowReqs }"
              @click="modalShowReqs = !modalShowReqs"
            >
              <span class="toggle-dot bg-emerald"></span>
              <span>Req/s</span>
            </button>
          </div>
        </div>

        <!-- Expanded SVG Chart Canvas with Hover Crosshair & Tooltip -->
        <div
          class="modal-svg-canvas-box"
          @mousemove="handleModalChartHover"
          @mouseleave="handleModalChartLeave"
        >
          <svg viewBox="0 0 760 210" preserveAspectRatio="none" class="modal-expanded-svg">
            <defs>
              <linearGradient id="modalCpuGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#8b5cf6" stop-opacity="0.3" />
                <stop offset="100%" stop-color="#8b5cf6" stop-opacity="0.0" />
              </linearGradient>
              <linearGradient id="modalMemGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.25" />
                <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
              </linearGradient>
              <linearGradient id="modalReqsGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#10b981" stop-opacity="0.25" />
                <stop offset="100%" stop-color="#10b981" stop-opacity="0.0" />
              </linearGradient>
            </defs>

            <!-- Horizontal Grid Lines & Left/Right Y-Axis Labels -->
            <!-- 100% -->
            <line x1="50" y1="20" x2="700" y2="20" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3 3" />
            <text x="42" y="24" text-anchor="end" class="svg-axis-label">100%</text>
            <text x="708" y="24" text-anchor="start" class="svg-axis-label text-emerald">{{ maxModalReqs }}</text>

            <!-- 75% -->
            <line x1="50" y1="62.5" x2="700" y2="62.5" stroke="rgba(255,255,255,0.04)" stroke-dasharray="3 3" />
            <text x="42" y="66" text-anchor="end" class="svg-axis-label">75%</text>
            <text x="708" y="66" text-anchor="start" class="svg-axis-label text-emerald">{{ Math.round(maxModalReqs * 0.75) }}</text>

            <!-- 50% -->
            <line x1="50" y1="105" x2="700" y2="105" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3 3" />
            <text x="42" y="109" text-anchor="end" class="svg-axis-label">50%</text>
            <text x="708" y="109" text-anchor="start" class="svg-axis-label text-emerald">{{ Math.round(maxModalReqs * 0.5) }}</text>

            <!-- 25% -->
            <line x1="50" y1="147.5" x2="700" y2="147.5" stroke="rgba(255,255,255,0.04)" stroke-dasharray="3 3" />
            <text x="42" y="151" text-anchor="end" class="svg-axis-label">25%</text>
            <text x="708" y="151" text-anchor="start" class="svg-axis-label text-emerald">{{ Math.round(maxModalReqs * 0.25) }}</text>

            <!-- 0% Baseline -->
            <line x1="50" y1="190" x2="700" y2="190" stroke="rgba(255,255,255,0.12)" />
            <text x="42" y="194" text-anchor="end" class="svg-axis-label">0%</text>
            <text x="708" y="194" text-anchor="start" class="svg-axis-label text-emerald">0</text>

            <!-- X-Axis Timestamps -->
            <g class="modal-time-markers">
              <text
                v-for="marker in modalTimeAxisMarkers"
                :key="marker.x"
                :x="marker.x"
                y="205"
                text-anchor="middle"
                class="svg-axis-label"
              >
                {{ marker.time }}
              </text>
            </g>

            <!-- Series 1: CPU Area & Line -->
            <path v-if="modalShowCpu && modalChartCpuArea" :d="modalChartCpuArea" fill="url(#modalCpuGrad)" />
            <path v-if="modalShowCpu && modalChartCpuPath" :d="modalChartCpuPath" fill="none" stroke="#8b5cf6" stroke-width="2.5" />

            <!-- Series 2: RAM Area & Line -->
            <path v-if="modalShowMem && modalChartMemArea" :d="modalChartMemArea" fill="url(#modalMemGrad)" />
            <path v-if="modalShowMem && modalChartMemPath" :d="modalChartMemPath" fill="none" stroke="#06b6d4" stroke-width="2.5" />

            <!-- Series 3: Throughput Area & Line -->
            <path v-if="modalShowReqs && modalChartReqsArea" :d="modalChartReqsArea" fill="url(#modalReqsGrad)" />
            <path v-if="modalShowReqs && modalChartReqsPath" :d="modalChartReqsPath" fill="none" stroke="#10b981" stroke-width="2.5" />

            <!-- Interactive Hover Crosshair & Series Markers -->
            <g v-if="modalHoverCoords">
              <!-- Vertical Guide Line -->
              <line
                :x1="modalHoverCoords.x"
                y1="15"
                :x2="modalHoverCoords.x"
                y2="190"
                stroke="#38bdf8"
                stroke-width="1.5"
                stroke-dasharray="3 3"
              />

              <!-- CPU Point Marker -->
              <circle
                v-if="modalShowCpu"
                :cx="modalHoverCoords.x"
                :cy="modalHoverCoords.yCpu"
                r="4.5"
                fill="#8b5cf6"
                stroke="#ffffff"
                stroke-width="2"
              />

              <!-- RAM Point Marker -->
              <circle
                v-if="modalShowMem"
                :cx="modalHoverCoords.x"
                :cy="modalHoverCoords.yMem"
                r="4.5"
                fill="#06b6d4"
                stroke="#ffffff"
                stroke-width="2"
              />

              <!-- Reqs Point Marker -->
              <circle
                v-if="modalShowReqs"
                :cx="modalHoverCoords.x"
                :cy="modalHoverCoords.yReq"
                r="4.5"
                fill="#10b981"
                stroke="#ffffff"
                stroke-width="2"
              />
            </g>
          </svg>

          <!-- Expanded Floating Tooltip Box -->
          <div v-if="hoveredModalPoint" class="modal-rich-tooltip" :style="modalTooltipStyle">
            <div class="tooltip-time-header">
              <span class="tooltip-time-icon">🕒</span>
              <span class="tooltip-time-text font-mono font-bold">{{ hoveredModalPoint.time }}</span>
              <span class="tooltip-time-elapsed">({{ hoveredModalElapsed }})</span>
            </div>

            <div class="tooltip-metrics-list">
              <div class="tooltip-metric-row" v-if="modalShowCpu">
                <span class="metric-indicator bg-violet"></span>
                <span class="metric-name">Avg Cluster CPU:</span>
                <span class="metric-val font-mono text-violet font-bold">{{ Math.round(hoveredModalPoint.cpu) }}%</span>
              </div>

              <div class="tooltip-metric-row" v-if="modalShowMem">
                <span class="metric-indicator bg-cyan"></span>
                <span class="metric-name">Avg Cluster RAM:</span>
                <span class="metric-val font-mono text-cyan font-bold">{{ Math.round(hoveredModalPoint.mem) }}%</span>
              </div>

              <div class="tooltip-metric-row" v-if="modalShowReqs">
                <span class="metric-indicator bg-emerald"></span>
                <span class="metric-name">Throughput:</span>
                <span class="metric-val font-mono text-emerald font-bold">{{ Math.round(hoveredModalPoint.reqs) }} req/s</span>
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
              <span>📦 Ranked Service & Workload Contributors</span>
            </h4>
            <span class="badge badge-cyan font-mono">
              {{ filteredAndSortedClusterServices.length }} Active Services
            </span>
          </div>

          <!-- Search & Filters -->
          <div class="breakdown-actions-bar">
            <div class="table-search-input-wrap">
              <span class="search-icon">🔍</span>
              <input
                type="text"
                v-model="modalServiceSearch"
                placeholder="Filter services or nodes..."
                class="table-search-field font-mono"
              />
              <button
                v-if="modalServiceSearch"
                class="btn-clear-search"
                @click="modalServiceSearch = ''"
              >
                ✕
              </button>
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
</template>

<style scoped>
.deep-dive-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* SECTION 1: Modal Summary Statistics Grid */
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
  color: var(--text-secondary, #94a3b8);
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
  color: var(--text-muted, #64748b);
}

.deep-stat-sub {
  font-size: 10.5px;
  color: var(--text-muted, #64748b);
}

/* Modal Chart Section */
.modal-chart-section {
  padding: 18px 20px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.08);
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

.trend-chart-subtitle {
  font-size: 11.5px;
  color: var(--text-secondary, #94a3b8);
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
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-muted, #94a3b8);
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

/* Modal Breakdown Table Section */
.modal-breakdown-section {
  padding: 18px 20px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.08);
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
  color: var(--text-muted, #94a3b8);
  pointer-events: none;
}

.table-search-field {
  padding: 6px 30px 6px 30px;
  font-size: 12px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid rgba(255, 255, 255, 0.1);
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
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
  cursor: pointer;
}

.btn-clear-search:hover {
  color: #fff;
}

.breakdown-table-wrapper {
  overflow-x: auto;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
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
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.breakdown-table th {
  padding: 10px 14px;
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted, #94a3b8);
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

.node-indicator-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
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
  color: var(--text-muted, #94a3b8);
}

.node-cell-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-secondary, #94a3b8);
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
  color: var(--text-muted, #94a3b8);
}

.font-mono { font-family: var(--font-mono, monospace); }
.font-bold { font-weight: 700; }
.cursor-pointer { cursor: pointer; }
.text-cyan { color: #06b6d4; }
.text-violet { color: #8b5cf6; }
.text-emerald { color: #10b981; }
.text-rose { color: #f43f5e; }
.text-amber { color: #f59e0b; }
.text-muted { color: #64748b; }
.bg-cyan { background-color: #06b6d4; }
.bg-violet { background-color: #8b5cf6; }
.bg-emerald { background-color: #10b981; }
.bg-rose { background-color: #f43f5e; }
.bg-amber { background-color: #f59e0b; }

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
.badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.badge-rose { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }
.badge-cyan { background: rgba(6, 182, 212, 0.15); color: #22d3ee; border: 1px solid rgba(6, 182, 212, 0.3); }

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

.btn-secondary {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-primary, #f8fafc);
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.2);
}

.btn-sm {
  padding: 4px 10px;
  font-size: 12px;
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
</style>\n