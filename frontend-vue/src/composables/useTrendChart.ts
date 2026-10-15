import { ref, computed, type ComputedRef } from 'vue'
import { computeCeiling, buildSmoothSpline, formatMetricRate } from '../components/overview/hud/trendMath'

export interface TrendPoint {
  time: string
  cpu: number
  mem: number
  disk: number
  reqs: number
}

export interface UseTrendChartOptions {
  trendHistory: ComputedRef<TrendPoint[]>
  effectiveHttpRps: ComputedRef<number>
}

export function useTrendChart(options: UseTrendChartOptions) {
  const { trendHistory, effectiveHttpRps } = options

  const latestCpu = computed(() => {
    const h = trendHistory.value
    if (!h.length) return 0
    return h[h.length - 1].cpu || 0
  })

  const latestMem = computed(() => {
    const h = trendHistory.value
    if (!h.length) return 0
    return h[h.length - 1].mem || 0
  })

  const latestReqs = computed(() => {
    const h = trendHistory.value
    if (!h.length) return effectiveHttpRps.value || 0
    return h[h.length - 1].reqs || 0
  })

  const trendChartMaxReqs = computed(() => {
    const rawMax = Math.max(0, ...trendHistory.value.map((p) => p.reqs || 0), Math.round(effectiveHttpRps.value || 0))
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
    const h = trendHistory.value
    if (hoveredIndex.value === null || hoveredIndex.value < 0 || hoveredIndex.value >= h.length) {
      return null
    }
    return h[hoveredIndex.value]
  })

  const hoveredElapsed = computed<string>(() => {
    if (hoveredIndex.value === null) return ''
    const offset = (trendHistory.value.length - 1 - hoveredIndex.value) * 10
    if (offset === 0) return 'Now (Live)'
    const mins = Math.floor(offset / 60)
    const secs = offset % 60
    if (mins === 0) return `${secs}s ago`
    return `${mins}m ${secs > 0 ? secs + 's' : ''} ago`
  })

  const hoverCoords = computed(() => {
    const h = trendHistory.value
    if (hoveredIndex.value === null || !h.length) return null
    const step = h.length > 1 ? 988 / (h.length - 1) : 0
    const idx = hoveredIndex.value
    const pt = h[idx]
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
    hoveredIndex.value = Math.max(0, Math.min(history.length - 1, idx))
    tooltipPos.value = { x: mouseX, y: mouseY }
  }

  function handleChartLeave() {
    hoveredIndex.value = null
  }

  return {
    latestCpu,
    latestMem,
    latestReqs,
    trendChartMaxReqs,
    trendChartCpuPath,
    trendChartCpuArea,
    trendChartMemPath,
    trendChartMemArea,
    trendChartReqsPath,
    trendChartReqsArea,
    trendTimeAxisMarkers,
    hoveredPoint,
    hoveredElapsed,
    hoverCoords,
    tooltipStyle,
    handleChartHover,
    handleChartLeave,
    formatMetricRate,
  }
}
