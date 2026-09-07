<script setup lang="ts">
import { ref, computed } from 'vue'
import type { TpsSnapshot } from '../../../../api/overview'
import type { TrendPoint } from './TrafficGeoDistribution.vue'
import { computeCeiling, buildSmoothSpline, formatMetricRate } from './trafficChartMath'

interface Props {
  trendHistory: TrendPoint[]
  tpsData?: TpsSnapshot | null
}

const props = withDefaults(defineProps<Props>(), {
  trendHistory: () => [],
  tpsData: null,
})

const modalShowCpu = ref(true)
const modalShowMem = ref(true)
const modalShowReqs = ref(true)
const hoveredModalIndex = ref<number | null>(null)
const modalTooltipPos = ref({ x: 0, y: 0 })

const latestCpu = computed(() => props.trendHistory.length ? props.trendHistory[props.trendHistory.length - 1].cpu || 0 : 0)
const latestMem = computed(() => props.trendHistory.length ? props.trendHistory[props.trendHistory.length - 1].mem || 0 : 0)
const latestReqs = computed(() => {
  if (!props.trendHistory.length) return props.tpsData?.http?.requests_per_sec || 0
  return props.trendHistory[props.trendHistory.length - 1].reqs || 0
})

const maxModalReqs = computed(() => {
  const rawMax = Math.max(0, ...props.trendHistory.map(p => p.reqs || 0), Math.round(props.tpsData?.http?.requests_per_sec || 0))
  return computeCeiling(rawMax)
})

const modalPeakReqsCoord = computed(() => {
  const history = props.trendHistory
  if (!history.length) return null
  let maxVal = -1, maxIdx = 0
  for (let i = 0; i < history.length; i++) {
    const r = history[i].reqs || 0
    if (r > maxVal) { maxVal = r; maxIdx = i }
  }
  if (maxVal <= 0) return null
  const step = history.length > 1 ? 640 / (history.length - 1) : 0
  const maxR = maxModalReqs.value
  return { x: 50 + maxIdx * step, y: Math.max(25, Math.min(190, 190 - (Math.min(maxR, maxVal) / maxR) * 165)), value: maxVal, index: maxIdx }
})

function makePath(getValue: (p: TrendPoint) => number, maxVal = 100) {
  const h = props.trendHistory
  if (h.length < 2) return ''
  const step = 640 / (h.length - 1)
  return buildSmoothSpline(h.map((pt, i) => ({ x: 50 + i * step, y: Math.max(25, Math.min(190, 190 - (Math.min(maxVal, Math.max(0, getValue(pt))) / maxVal) * 165)) })))
}
function makeArea(path: string) {
  const h = props.trendHistory
  if (!path || !h.length) return ''
  const lastX = 50 + (h.length - 1) * (h.length > 1 ? 640 / (h.length - 1) : 0)
  return `${path} L ${lastX.toFixed(1)} 190 L 50.0 190 Z`
}

const modalChartCpuPath = computed(() => makePath(p => p.cpu, 100))
const modalChartCpuArea = computed(() => makeArea(modalChartCpuPath.value))
const modalChartMemPath = computed(() => makePath(p => p.mem, 100))
const modalChartMemArea = computed(() => makeArea(modalChartMemPath.value))
const modalChartReqsPath = computed(() => makePath(p => p.reqs, maxModalReqs.value))
const modalChartReqsArea = computed(() => makeArea(modalChartReqsPath.value))

const hoveredModalPoint = computed<TrendPoint | null>(() => {
  if (hoveredModalIndex.value === null || hoveredModalIndex.value < 0 || hoveredModalIndex.value >= props.trendHistory.length) return null
  return props.trendHistory[hoveredModalIndex.value]
})

const hoveredModalElapsed = computed<string>(() => {
  if (hoveredModalIndex.value === null) return ''
  const offset = (props.trendHistory.length - 1 - hoveredModalIndex.value) * 10
  if (offset === 0) return 'Now (Live)'
  const mins = Math.floor(offset / 60)
  const secs = offset % 60
  return mins === 0 ? `${secs}s ago` : `${mins}m ${secs > 0 ? secs + 's' : ''} ago`
})

const modalHoverCoords = computed(() => {
  if (hoveredModalIndex.value === null || !props.trendHistory.length) return null
  const pt = props.trendHistory[hoveredModalIndex.value]
  if (!pt) return null
  const x = 50 + hoveredModalIndex.value * (props.trendHistory.length > 1 ? 640 / (props.trendHistory.length - 1) : 0)
  const maxR = maxModalReqs.value
  return {
    x,
    yCpu: Math.max(25, Math.min(190, 190 - (Math.min(100, Math.max(0, pt.cpu)) / 100) * 165)),
    yMem: Math.max(25, Math.min(190, 190 - (Math.min(100, Math.max(0, pt.mem)) / 100) * 165)),
    yReq: Math.max(25, Math.min(190, 190 - (Math.min(maxR, Math.max(0, pt.reqs)) / maxR) * 165)),
  }
})

const modalTooltipStyle = computed(() => {
  if (hoveredModalIndex.value === null) return { display: 'none' }
  const { x, y } = modalTooltipPos.value
  return {
    left: x > 380 ? `${Math.max(10, x - 240)}px` : `${x + 16}px`,
    top: y > 95 ? `${Math.max(10, y - 130)}px` : `${Math.min(90, y + 15)}px`,
    pointerEvents: 'none' as const,
  }
})

const modalTimeAxisMarkers = computed(() => {
  const h = props.trendHistory
  if (h.length < 2) return []
  const count = Math.min(5, h.length)
  const step = 640 / (h.length - 1)
  return Array.from({ length: count }, (_, i) => {
    const idx = Math.round((i / (count - 1)) * (h.length - 1))
    return { x: 50 + idx * step, time: h[idx]?.time || '' }
  })
})

function handleModalChartHover(event: MouseEvent) {
  if (props.trendHistory.length < 2) return
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  const mouseX = Math.max(0, Math.min(rect.width, event.clientX - rect.left))
  const mouseY = Math.max(0, Math.min(rect.height, event.clientY - rect.top))
  const ratio = (Math.max(50, Math.min(690, mouseX / (rect.width / 760))) - 50) / 640
  hoveredModalIndex.value = Math.max(0, Math.min(props.trendHistory.length - 1, Math.round(ratio * (props.trendHistory.length - 1))))
  modalTooltipPos.value = { x: mouseX, y: mouseY }
}
</script>

<template>
  <section class="modal-chart-section glass-panel">
    <div class="modal-chart-top-bar">
      <div class="chart-title-group">
        <h4 class="modal-section-title">
          <span class="chart-title-full">📈 High-Resolution Telemetry Overlay</span>
          <span class="chart-title-mobile">📈 Telemetry Spline</span>
        </h4>
        <span class="trend-chart-subtitle">Time-synchronized CPU %, RAM %, and Throughput RPS (30 rolling samples)</span>
      </div>

      <div class="series-toggles-group">
        <button type="button" class="series-toggle-btn" :class="{ 'toggle-active cpu-active': modalShowCpu, 'toggle-dimmed': !modalShowCpu }" @click="modalShowCpu = !modalShowCpu" title="Toggle CPU saturation spline">
          <span class="toggle-dot bg-violet"></span>
          <span class="toggle-name">CPU %</span>
          <span class="toggle-val font-mono">({{ Math.round(latestCpu) }}%)</span>
        </button>
        <button type="button" class="series-toggle-btn" :class="{ 'toggle-active mem-active': modalShowMem, 'toggle-dimmed': !modalShowMem }" @click="modalShowMem = !modalShowMem" title="Toggle RAM usage spline">
          <span class="toggle-dot bg-cyan"></span>
          <span class="toggle-name">RAM %</span>
          <span class="toggle-val font-mono">({{ Math.round(latestMem) }}%)</span>
        </button>
        <button type="button" class="series-toggle-btn" :class="{ 'toggle-active reqs-active': modalShowReqs, 'toggle-dimmed': !modalShowReqs }" @click="modalShowReqs = !modalShowReqs" title="Toggle Throughput spline">
          <span class="toggle-dot bg-emerald"></span>
          <span class="toggle-name"><span class="btn-lbl-full">Gateway Throughput</span><span class="btn-lbl-mobile">Gateway</span></span>
          <span class="toggle-val font-mono">({{ Math.round(latestReqs).toLocaleString() }} req/s)</span>
        </button>
      </div>
    </div>

    <div class="modal-svg-canvas-box" @mousemove="handleModalChartHover" @mouseleave="hoveredModalIndex = null">
      <svg viewBox="0 0 760 210" preserveAspectRatio="none" class="modal-expanded-svg">
        <defs>
          <linearGradient id="modalCpuGrad" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#8b5cf6" stop-opacity="0.35" /><stop offset="60%" stop-color="#7c3aed" stop-opacity="0.12" /><stop offset="100%" stop-color="#8b5cf6" stop-opacity="0.0" />
          </linearGradient>
          <linearGradient id="modalMemGrad" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.35" /><stop offset="60%" stop-color="#0891b2" stop-opacity="0.12" /><stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
          </linearGradient>
          <linearGradient id="modalReqsGrad" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#10b981" stop-opacity="0.38" /><stop offset="50%" stop-color="#059669" stop-opacity="0.15" /><stop offset="100%" stop-color="#10b981" stop-opacity="0.0" />
          </linearGradient>
        </defs>

        <!-- Grid Lines -->
        <g v-for="g in [{ y: 25, pct: '100%', rps: formatMetricRate(maxModalReqs) }, { y: 66.25, pct: '75%', rps: formatMetricRate(maxModalReqs * 0.75) }, { y: 107.5, pct: '50%', rps: formatMetricRate(maxModalReqs * 0.5) }, { y: 148.75, pct: '25%', rps: formatMetricRate(maxModalReqs * 0.25) }]" :key="g.pct">
          <line x1="50" :y1="g.y" x2="690" :y2="g.y" stroke="rgba(255,255,255,0.06)" stroke-dasharray="4 4" />
          <text x="42" :y="g.y + 4" text-anchor="end" class="svg-axis-label">{{ g.pct }}</text>
          <text x="698" :y="g.y + 4" text-anchor="start" class="svg-axis-label text-emerald">{{ g.rps }}</text>
        </g>
        <line x1="50" y1="190" x2="690" y2="190" stroke="rgba(255,255,255,0.18)" stroke-width="1.5" />
        <text x="42" y="194" text-anchor="end" class="svg-axis-label">0%</text>
        <text x="698" y="194" text-anchor="start" class="svg-axis-label text-emerald">0 rps</text>

        <!-- Timestamps -->
        <text v-for="m in modalTimeAxisMarkers" :key="m.x" :x="m.x" y="205" text-anchor="middle" class="svg-axis-label">{{ m.time }}</text>

        <!-- Curves -->
        <path v-if="modalShowCpu && modalChartCpuArea" :d="modalChartCpuArea" fill="url(#modalCpuGrad)" />
        <path v-if="modalShowCpu && modalChartCpuPath" :d="modalChartCpuPath" fill="none" stroke="#8b5cf6" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round" />
        <path v-if="modalShowMem && modalChartMemArea" :d="modalChartMemArea" fill="url(#modalMemGrad)" />
        <path v-if="modalShowMem && modalChartMemPath" :d="modalChartMemPath" fill="none" stroke="#06b6d4" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round" />
        <path v-if="modalShowReqs && modalChartReqsArea" :d="modalChartReqsArea" fill="url(#modalReqsGrad)" />
        <path v-if="modalShowReqs && modalChartReqsPath" :d="modalChartReqsPath" fill="none" stroke="#10b981" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round" />

        <!-- Peak Beacon -->
        <g v-if="modalShowReqs && modalPeakReqsCoord">
          <line :x1="modalPeakReqsCoord.x" :y1="modalPeakReqsCoord.y" :x2="modalPeakReqsCoord.x" y2="190" stroke="rgba(16, 185, 129, 0.2)" stroke-width="1" stroke-dasharray="2 2" />
          <circle :cx="modalPeakReqsCoord.x" :cy="modalPeakReqsCoord.y" r="6" fill="none" stroke="#10b981" stroke-width="1.2" opacity="0.6" class="peak-pulse-ring" />
          <circle :cx="modalPeakReqsCoord.x" :cy="modalPeakReqsCoord.y" r="3" fill="#10b981" stroke="#ffffff" stroke-width="1.5" />
        </g>

        <!-- Hover Crosshair -->
        <g v-if="modalHoverCoords">
          <line :x1="modalHoverCoords.x" y1="25" :x2="modalHoverCoords.x" y2="190" stroke="#38bdf8" stroke-width="1.5" stroke-dasharray="3 3" opacity="0.85" />
          <g v-for="s in [{ show: modalShowCpu, y: modalHoverCoords.yCpu, c: '#8b5cf6' }, { show: modalShowMem, y: modalHoverCoords.yMem, c: '#06b6d4' }, { show: modalShowReqs, y: modalHoverCoords.yReq, c: '#10b981' }]" :key="s.c">
            <circle v-if="s.show" :cx="modalHoverCoords.x" :cy="s.y" r="7" fill="none" :stroke="s.c" stroke-width="1.5" opacity="0.5" />
            <circle v-if="s.show" :cx="modalHoverCoords.x" :cy="s.y" r="4" :fill="s.c" stroke="#ffffff" stroke-width="2" />
          </g>
        </g>
      </svg>

      <!-- Floating Tooltip -->
      <div v-if="hoveredModalPoint" class="modal-rich-tooltip" :style="modalTooltipStyle">
        <div class="tooltip-time-header">
          <span>🕒</span>
          <span class="font-mono font-bold">{{ hoveredModalPoint.time }}</span>
          <span class="tooltip-time-elapsed">({{ hoveredModalElapsed }})</span>
        </div>
        <div class="tooltip-metrics-list">
          <div v-if="modalShowCpu" class="tooltip-metric-row"><span class="metric-indicator bg-violet"></span><span class="metric-name">Avg Cluster CPU:</span><span class="font-mono text-violet font-bold">{{ Math.round(hoveredModalPoint.cpu) }}%</span></div>
          <div v-if="modalShowMem" class="tooltip-metric-row"><span class="metric-indicator bg-cyan"></span><span class="metric-name">Avg Cluster RAM:</span><span class="font-mono text-cyan font-bold">{{ Math.round(hoveredModalPoint.mem) }}%</span></div>
          <div v-if="modalShowReqs" class="tooltip-metric-row"><span class="metric-indicator bg-emerald"></span><span class="metric-name">Throughput:</span><span class="font-mono text-emerald font-bold">{{ Math.round(hoveredModalPoint.reqs) }} req/s</span></div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
@import '../../../../assets/styles/components/deep-dive-traffic.css';
</style>
