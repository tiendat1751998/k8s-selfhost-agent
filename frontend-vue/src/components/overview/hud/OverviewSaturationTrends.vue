<script setup lang="ts">
import { computed, ref } from 'vue'

export interface TrendPoint {
  time: string
  cpu: number
  mem: number
  disk: number
  reqs: number
}

interface Props {
  trendHistory: TrendPoint[]
  effectiveHttpRps?: number
  httpActiveConns?: number
  httpQueuedReqs?: number
  clusterAvgLatencyMs?: number
  httpErrorRate?: number
}

const props = withDefaults(defineProps<Props>(), {
  effectiveHttpRps: 0,
  httpActiveConns: 0,
  httpQueuedReqs: 0,
  clusterAvgLatencyMs: 0,
  httpErrorRate: 0,
})

const emit = defineEmits<{
  (e: 'openDeepDive'): void
}>()

function computeCeiling(rawMax: number): number {
  if (rawMax <= 0 || isNaN(rawMax)) return 10
  const padded = rawMax * 1.25
  if (padded <= 10) return 10
  if (padded <= 20) return Math.ceil(padded)
  if (padded <= 50) return Math.ceil(padded / 5) * 5
  if (padded <= 200) return Math.ceil(padded / 10) * 10
  if (padded <= 1000) return Math.ceil(padded / 50) * 50
  if (padded <= 10000) return Math.ceil(padded / 500) * 500
  return Math.ceil(padded / 1000) * 1000
}

function buildSmoothSpline(points: Array<{ x: number; y: number }>, smoothing = 0.18, minY = 10, maxY = 194): string {
  if (!points || points.length === 0) return ''
  if (points.length === 1) return `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`
  if (points.length === 2) return `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)} L ${points[1].x.toFixed(1)} ${points[1].y.toFixed(1)}`

  let d = `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[Math.max(0, i - 1)]
    const p1 = points[i]
    const p2 = points[i + 1]
    const p3 = points[Math.min(points.length - 1, i + 2)]

    const minLocalX = Math.min(p1.x, p2.x), maxLocalX = Math.max(p1.x, p2.x)
    const cp1x = Math.max(minLocalX, Math.min(maxLocalX, p1.x + ((p2.x - p0.x) / 6) * (1 - smoothing)))
    const cp2x = Math.max(minLocalX, Math.min(maxLocalX, p2.x - ((p3.x - p1.x) / 6) * (1 - smoothing)))
    const cp1y = Math.max(minY, Math.min(maxY, p1.y + ((p2.y - p0.y) / 6) * (1 - smoothing)))
    const cp2y = Math.max(minY, Math.min(maxY, p2.y - ((p3.y - p1.y) / 6) * (1 - smoothing)))

    d += ` C ${cp1x.toFixed(1)} ${cp1y.toFixed(1)}, ${cp2x.toFixed(1)} ${cp2y.toFixed(1)}, ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`
  }
  return d
}

function formatMetricRate(val: number, withUnit = false): string {
  if (val === undefined || val === null || isNaN(val)) return withUnit ? '0 rps' : '0'
  let str = '0'
  if (val >= 1000000) str = `${(val / 1000000).toFixed(1)}M`
  else if (val >= 1000) str = `${(val / 1000).toFixed(1)}k`
  else if (val >= 100) str = Math.round(val).toString()
  else if (val > 0) str = val.toFixed(1)
  return withUnit ? `${str} rps` : str
}

const latestCpu = computed(() => {
  if (!props.trendHistory.length) return 0
  return props.trendHistory[props.trendHistory.length - 1].cpu || 0
})

const latestMem = computed(() => {
  if (!props.trendHistory.length) return 0
  return props.trendHistory[props.trendHistory.length - 1].mem || 0
})

const latestReqs = computed(() => {
  if (!props.trendHistory.length) return props.effectiveHttpRps || 0
  return props.trendHistory[props.trendHistory.length - 1].reqs || 0
})

const trendChartMaxReqs = computed(() => {
  const rawMax = Math.max(0, ...props.trendHistory.map(p => p.reqs || 0), Math.round(props.effectiveHttpRps || 0))
  return computeCeiling(rawMax)
})

const trendChartCpuPath = computed(() => {
  const history = props.trendHistory
  if (history.length < 2) return ''
  const step = 988 / (history.length - 1)
  const points = history.map((h, i) => ({
    x: 6 + i * step,
    y: Math.max(10, Math.min(194, 194 - (Math.min(100, Math.max(0, h.cpu)) / 100) * 184)),
  }))
  return buildSmoothSpline(points, 0.18, 10, 194)
})

const trendChartCpuArea = computed(() => {
  if (!trendChartCpuPath.value || !props.trendHistory.length) return ''
  const history = props.trendHistory
  const step = history.length > 1 ? 988 / (history.length - 1) : 0
  const lastX = 6 + (history.length - 1) * step
  return `${trendChartCpuPath.value} L ${lastX.toFixed(1)} 194 L 6.0 194 Z`
})

const trendChartMemPath = computed(() => {
  const history = props.trendHistory
  if (history.length < 2) return ''
  const step = 988 / (history.length - 1)
  const points = history.map((h, i) => ({
    x: 6 + i * step,
    y: Math.max(10, Math.min(194, 194 - (Math.min(100, Math.max(0, h.mem)) / 100) * 184)),
  }))
  return buildSmoothSpline(points, 0.18, 10, 194)
})

const trendChartMemArea = computed(() => {
  if (!trendChartMemPath.value || !props.trendHistory.length) return ''
  const history = props.trendHistory
  const step = history.length > 1 ? 988 / (history.length - 1) : 0
  const lastX = 6 + (history.length - 1) * step
  return `${trendChartMemPath.value} L ${lastX.toFixed(1)} 194 L 6.0 194 Z`
})

const trendChartReqsPath = computed(() => {
  const history = props.trendHistory
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
  if (!trendChartReqsPath.value || !props.trendHistory.length) return ''
  const history = props.trendHistory
  const step = history.length > 1 ? 988 / (history.length - 1) : 0
  const lastX = 6 + (history.length - 1) * step
  return `${trendChartReqsPath.value} L ${lastX.toFixed(1)} 194 L 6.0 194 Z`
})

const trendTimeAxisMarkers = computed(() => {
  const history = props.trendHistory
  if (history.length < 2) return []
  const count = Math.min(5, history.length)
  const markers = []
  const step = 988 / (history.length - 1)
  for (let i = 0; i < count; i++) {
    const idx = Math.round((i / (count - 1)) * (history.length - 1))
    const pt = history[idx]
    if (pt) {
      markers.push({
        x: 6 + idx * step,
        time: (pt.time || '').replace(/\s*[AP]M$/i, ''),
      })
    }
  }
  return markers
})

const hoveredIndex = ref<number | null>(null)
const tooltipPos = ref({ x: 0, y: 0 })

const hoveredPoint = computed<TrendPoint | null>(() => {
  if (hoveredIndex.value === null || hoveredIndex.value < 0 || hoveredIndex.value >= props.trendHistory.length) {
    return null
  }
  return props.trendHistory[hoveredIndex.value]
})

const hoveredElapsed = computed<string>(() => {
  if (hoveredIndex.value === null) return ''
  const offset = (props.trendHistory.length - 1 - hoveredIndex.value) * 10
  if (offset === 0) return 'Now (Live)'
  const mins = Math.floor(offset / 60)
  const secs = offset % 60
  if (mins === 0) return `${secs}s ago`
  return `${mins}m ${secs > 0 ? secs + 's' : ''} ago`
})

const hoverCoords = computed(() => {
  if (hoveredIndex.value === null || !props.trendHistory.length) return null
  const history = props.trendHistory
  const step = history.length > 1 ? 988 / (history.length - 1) : 0
  const idx = hoveredIndex.value
  const pt = history[idx]
  if (!pt) return null
  const x = 6 + idx * step
  const maxR = trendChartMaxReqs.value
  const yCpu = Math.max(10, Math.min(194, 194 - (Math.min(100, Math.max(0, pt.cpu)) / 100) * 184))
  const yMem = Math.max(10, Math.min(194, 194 - (Math.min(100, Math.max(0, pt.mem)) / 100) * 184))
  const yReq = Math.max(10, Math.min(194, 194 - (Math.min(maxR, Math.max(0, pt.reqs)) / maxR) * 184))
  return { x, yCpu, yMem, yReq }
})

const tooltipStyle = computed(() => {
  if (hoveredIndex.value === null) return { display: 'none' }
  const { x, y } = tooltipPos.value
  const isRightSide = x > 380
  return {
    left: isRightSide ? `${Math.max(10, x - 235)}px` : `${x + 16}px`,
    top: y > 75 ? `${Math.max(8, y - 125)}px` : `${Math.min(55, y + 15)}px`,
    pointerEvents: 'none' as const,
  }
})

function handleChartHover(event: MouseEvent) {
  const history = props.trendHistory
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
  hoveredIndex.value = Math.max(0, Math.min(history.length - 1, idx))
  tooltipPos.value = { x: mouseX, y: mouseY }
}

function handleChartLeave() {
  hoveredIndex.value = null
}
</script>

<template>
  <section class="trend-chart-card glass-panel trend-card-clickable" @click="emit('openDeepDive')">
    <div class="trend-chart-header">
      <div class="trend-title-wrap">
        <div class="trend-title-top-row">
          <div class="trend-title-left">
            <h3 class="sidebar-card-title">
              <span class="title-full">📈 5-Min Saturation Trends</span>
              <span class="title-mobile">📈 Trends</span>
            </h3>
            <span class="badge badge-muted font-mono">LIVE BUFFER</span>
          </div>
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
            <span class="legend-dot-circle bg-blue"></span>
            <span>CPU ({{ Math.round(latestCpu) }}%)</span>
          </span>
          <span class="legend-pill pill-mem">
            <span class="legend-dot-circle bg-sky"></span>
            <span>RAM ({{ Math.round(latestMem) }}%)</span>
          </span>
          <span class="legend-pill pill-reqs">
            <span class="legend-dot-circle bg-emerald"></span>
            <span><span class="legend-name-full">Throughput </span>({{ Math.round(latestReqs).toLocaleString() }} req/s)</span>
          </span>
        </div>

        <!-- Single Clean Deep-Dive Action Button (Desktop) -->
        <button
          class="trend-expand-badge font-mono trend-btn-desktop"
          type="button"
          @click.stop="emit('openDeepDive')"
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
          @mousemove="handleChartHover"
          @mouseleave="handleChartLeave"
        >
          <svg viewBox="0 0 1000 200" preserveAspectRatio="none" class="trend-plot-svg">
            <defs>
              <linearGradient id="trendCpuGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.08" />
                <stop offset="100%" stop-color="#3b82f6" stop-opacity="0.0" />
              </linearGradient>
              <linearGradient id="trendMemGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#0284c7" stop-opacity="0.08" />
                <stop offset="100%" stop-color="#0284c7" stop-opacity="0.0" />
              </linearGradient>
              <linearGradient id="trendReqsGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#10b981" stop-opacity="0.08" />
                <stop offset="100%" stop-color="#10b981" stop-opacity="0.0" />
              </linearGradient>
            </defs>

            <!-- 5 Horizontal Grid Lines -->
            <line x1="0" y1="10" x2="1000" y2="10" stroke="rgba(255,255,255,0.06)" stroke-dasharray="4 4" />
            <line x1="0" y1="56" x2="1000" y2="56" stroke="rgba(255,255,255,0.04)" stroke-dasharray="4 4" />
            <line x1="0" y1="102" x2="1000" y2="102" stroke="rgba(255,255,255,0.06)" stroke-dasharray="4 4" />
            <line x1="0" y1="148" x2="1000" y2="148" stroke="rgba(255,255,255,0.04)" stroke-dasharray="4 4" />
            <line x1="0" y1="194" x2="1000" y2="194" stroke="rgba(255,255,255,0.12)" stroke-width="1" />

            <!-- Series 1: CPU Area & Line (Enterprise Sapphire Blue #3b82f6) -->
            <path v-if="trendChartCpuArea" :d="trendChartCpuArea" fill="url(#trendCpuGrad)" />
            <path v-if="trendChartCpuPath" :d="trendChartCpuPath" fill="none" stroke="#3b82f6" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />

            <!-- Series 2: RAM Area & Line (Slate/Sky #0284c7) -->
            <path v-if="trendChartMemArea" :d="trendChartMemArea" fill="url(#trendMemGrad)" />
            <path v-if="trendChartMemPath" :d="trendChartMemPath" fill="none" stroke="#0284c7" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />

            <!-- Series 3: Throughput Area & Line (Refined Emerald #10b981) -->
            <path v-if="trendChartReqsArea" :d="trendChartReqsArea" fill="url(#trendReqsGrad)" />
            <path v-if="trendChartReqsPath" :d="trendChartReqsPath" fill="none" stroke="#10b981" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />

            <!-- Interactive Hover Crosshair & Series Markers -->
            <g v-if="hoverCoords">
              <line
                :x1="hoverCoords.x"
                y1="10"
                :x2="hoverCoords.x"
                y2="194"
                stroke="rgba(255, 255, 255, 0.25)"
                stroke-width="1"
                stroke-dasharray="3 3"
              />
              <g>
                <circle :cx="hoverCoords.x" :cy="hoverCoords.yCpu" r="5" fill="#161f30" stroke="#3b82f6" stroke-width="2" />
              </g>
              <g>
                <circle :cx="hoverCoords.x" :cy="hoverCoords.yMem" r="5" fill="#161f30" stroke="#0284c7" stroke-width="2" />
              </g>
              <g>
                <circle :cx="hoverCoords.x" :cy="hoverCoords.yReq" r="5" fill="#161f30" stroke="#10b981" stroke-width="2" />
              </g>
            </g>
          </svg>

          <!-- Floating Rich Tooltip Box -->
          <div v-if="hoveredPoint" class="trend-rich-tooltip" :style="tooltipStyle">
            <div class="tooltip-time-header">
              <span class="tooltip-time-icon">🕒</span>
              <span class="tooltip-time-text font-mono font-bold">{{ hoveredPoint.time }}</span>
              <span class="tooltip-time-elapsed">({{ hoveredElapsed }})</span>
            </div>
            <div class="tooltip-metrics-list">
              <div class="tooltip-metric-row">
                <span class="metric-indicator bg-blue"></span>
                <span class="metric-name">Avg Cluster CPU:</span>
                <span class="metric-val font-mono text-blue font-bold">{{ Math.round(hoveredPoint.cpu) }}%</span>
              </div>
              <div class="tooltip-metric-row">
                <span class="metric-indicator bg-sky"></span>
                <span class="metric-name">Avg Cluster RAM:</span>
                <span class="metric-val font-mono text-sky font-bold">{{ Math.round(hoveredPoint.mem) }}%</span>
              </div>
              <div class="tooltip-metric-row">
                <span class="metric-indicator bg-emerald"></span>
                <span class="metric-name">Throughput:</span>
                <span class="metric-val font-mono text-emerald font-bold">{{ Math.round(hoveredPoint.reqs) }} req/s</span>
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
</template>

<style scoped src="../../../assets/styles/overview-saturation-trends.css"></style>