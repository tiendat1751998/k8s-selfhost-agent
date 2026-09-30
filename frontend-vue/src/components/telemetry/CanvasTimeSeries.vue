<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useTimeSync } from '../../composables/useTimeSync'

export interface TimeSeriesItem {
  name: string
  data: [number, number][]
  color: string
}

export interface ThresholdItem {
  value: number
  color: string
  label?: string
}

interface Props {
  series: TimeSeriesItem[]
  unit: string
  height?: number
  syncGroup?: string
  min?: number
  max?: number
  thresholds?: ThresholdItem[]
}

const props = withDefaults(defineProps<Props>(), {
  height: 160,
  syncGroup: 'default',
  thresholds: () => [],
})

const emit = defineEmits<{ (e: 'timeHover', timestamp: number | null): void }>()
const { activeHoverTime, activeHoverGroup, setHoverTime, clearHoverTime } = useTimeSync()

const containerRef = ref<HTMLDivElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)
let resizeObserver: ResizeObserver | null = null
let rafId: number | null = null

const padLeft = 44, padRight = 14, padTop = 14, padBottom = 20

function hexToRgba(hex: string, alpha: number): string {
  if (!hex.startsWith('#')) return hex
  let c = hex.slice(1)
  if (c.length === 3) c = c.split('').map(x => x + x).join('')
  const num = parseInt(c, 16)
  return isNaN(num) || c.length !== 6 ? hex : `rgba(${(num >> 16) & 255}, ${(num >> 8) & 255}, ${num & 255}, ${alpha})`
}

function drawCurve(ctx: CanvasRenderingContext2D, pts: { x: number; y: number }[]) {
  ctx.beginPath()
  ctx.moveTo(pts[0].x, pts[0].y)
  for (let i = 0; i < pts.length - 1; i++) {
    const midX = (pts[i].x + pts[i + 1].x) / 2
    ctx.bezierCurveTo(midX, pts[i].y, midX, pts[i + 1].y, pts[i + 1].x, pts[i + 1].y)
  }
}

const timeBounds = computed(() => {
  let tMin = Infinity, tMax = -Infinity
  for (const s of props.series) {
    for (const [t] of s.data) {
      if (t < tMin) tMin = t
      if (t > tMax) tMax = t
    }
  }
  if (!isFinite(tMin) || !isFinite(tMax) || tMin === tMax) {
    const now = Date.now()
    return { min: now - 60000, max: now }
  }
  return { min: tMin, max: tMax }
})

const valBounds = computed(() => {
  let vMin = Infinity, vMax = -Infinity
  for (const s of props.series) {
    for (const [, v] of s.data) {
      if (v < vMin) vMin = v
      if (v > vMax) vMax = v
    }
  }
  for (const th of props.thresholds) {
    if (th.value < vMin) vMin = th.value
    if (th.value > vMax) vMax = th.value
  }
  if (!isFinite(vMin) || !isFinite(vMax)) return { min: props.min ?? 0, max: props.max ?? 100 }
  const effMin = props.min !== undefined ? props.min : Math.min(0, vMin)
  const effMax = props.max !== undefined ? props.max : (vMax === effMin ? effMin + 10 : vMax * 1.08)
  return { min: effMin, max: effMax }
})

const isHoverActive = computed(() => {
  if (activeHoverTime.value === null) return false
  if (activeHoverGroup.value && activeHoverGroup.value !== props.syncGroup) return false
  const t = activeHoverTime.value
  return t >= timeBounds.value.min && t <= timeBounds.value.max
})

const hoverData = computed(() => {
  if (!isHoverActive.value || activeHoverTime.value === null) return null
  const target = activeHoverTime.value
  const { min: tMin, max: tMax } = timeBounds.value
  const containerW = containerRef.value ? containerRef.value.getBoundingClientRect().width : 300
  const chartW = containerW - padLeft - padRight
  const frac = chartW > 0 ? (target - tMin) / (tMax - tMin) : 0
  const pixelX = padLeft + Math.max(0, Math.min(chartW, frac * chartW))

  const items = props.series.map(s => {
    if (s.data.length === 0) return { name: s.name, color: s.color, value: null, y: 0 }
    let closest = s.data[0]
    let minDiff = Math.abs(closest[0] - target)
    for (let i = 1; i < s.data.length; i++) {
      const diff = Math.abs(s.data[i][0] - target)
      if (diff < minDiff) { minDiff = diff; closest = s.data[i] }
    }
    const { min: vMin, max: vMax } = valBounds.value
    const chartH = props.height - padTop - padBottom
    const yFrac = vMax === vMin ? 0.5 : (closest[1] - vMin) / (vMax - vMin)
    return { name: s.name, color: s.color, value: closest[1], y: padTop + (1 - Math.max(0, Math.min(1, yFrac))) * chartH }
  })
  const d = new Date(target)
  const timeStr = `${d.toTimeString().split(' ')[0]}.${String(d.getMilliseconds()).padStart(3, '0').slice(0, 2)}`
  return { pixelX, timeStr, items }
})

function draw() {
  const canvas = canvasRef.value, container = containerRef.value
  if (!canvas || !container) return
  const w = Math.max(100, Math.floor(container.getBoundingClientRect().width)), h = props.height
  const dpr = window.devicePixelRatio || 1
  canvas.width = w * dpr; canvas.height = h * dpr
  canvas.style.width = `${w}px`; canvas.style.height = `${h}px`
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  ctx.scale(dpr, dpr); ctx.clearRect(0, 0, w, h)

  const chartW = w - padLeft - padRight, chartH = h - padTop - padBottom
  const { min: tMin, max: tMax } = timeBounds.value
  const { min: vMin, max: vMax } = valBounds.value
  const vRange = vMax === vMin ? 1 : vMax - vMin, tRange = tMax === tMin ? 1 : tMax - tMin

  // Grid and Y labels
  ctx.lineWidth = 1; ctx.font = '10px monospace'; ctx.fillStyle = '#64748b'; ctx.textAlign = 'right'; ctx.textBaseline = 'middle'
  for (let i = 0; i <= 3; i++) {
    const yVal = vMin + (i / 3) * vRange
    const yPos = padTop + (1 - i / 3) * chartH
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.06)'; ctx.setLineDash([])
    ctx.beginPath(); ctx.moveTo(padLeft, yPos); ctx.lineTo(w - padRight, yPos); ctx.stroke()
    const lbl = yVal >= 1000 ? (yVal / 1000).toFixed(1) + 'k' : yVal.toFixed(yVal % 1 === 0 ? 0 : 1)
    ctx.fillText(`${lbl}${props.unit}`, padLeft - 5, yPos)
  }

  // Threshold lines
  for (const th of props.thresholds) {
    if (th.value >= vMin && th.value <= vMax) {
      const thY = padTop + (1 - (th.value - vMin) / vRange) * chartH
      ctx.strokeStyle = hexToRgba(th.color, 0.75); ctx.lineWidth = 1; ctx.setLineDash([4, 4])
      ctx.beginPath(); ctx.moveTo(padLeft, thY); ctx.lineTo(w - padRight, thY); ctx.stroke()
      if (th.label) { ctx.fillStyle = th.color; ctx.textAlign = 'left'; ctx.fillText(th.label, padLeft + 4, thY - 5) }
    }
  }

  // Series curves
  for (const s of props.series) {
    if (s.data.length === 0) continue
    const pts = s.data.map(([t, v]) => ({
      x: padLeft + Math.max(0, Math.min(chartW, ((t - tMin) / tRange) * chartW)),
      y: padTop + (1 - Math.max(0, Math.min(1, (v - vMin) / vRange))) * chartH
    }))
    const grad = ctx.createLinearGradient(0, padTop, 0, padTop + chartH)
    grad.addColorStop(0, hexToRgba(s.color, 0.3)); grad.addColorStop(1, hexToRgba(s.color, 0.01))

    drawCurve(ctx, pts)
    ctx.lineTo(pts[pts.length - 1].x, padTop + chartH); ctx.lineTo(pts[0].x, padTop + chartH); ctx.closePath()
    ctx.fillStyle = grad; ctx.fill()

    drawCurve(ctx, pts)
    ctx.strokeStyle = s.color; ctx.lineWidth = 1.75; ctx.setLineDash([]); ctx.stroke()
  }

  // Crosshair
  if (hoverData.value) {
    const { pixelX, items } = hoverData.value
    ctx.save(); ctx.beginPath(); ctx.setLineDash([3, 3]); ctx.strokeStyle = 'rgba(255, 255, 255, 0.5)'; ctx.lineWidth = 1
    ctx.moveTo(pixelX, padTop); ctx.lineTo(pixelX, padTop + chartH); ctx.stroke()
    for (const it of items) {
      if (it.value !== null) {
        ctx.beginPath(); ctx.arc(pixelX, it.y, 4, 0, Math.PI * 2); ctx.fillStyle = it.color; ctx.fill()
        ctx.lineWidth = 2; ctx.strokeStyle = '#0f172a'; ctx.stroke()
      }
    }
    ctx.restore()
  }
}

function queueDraw() {
  if (rafId !== null) cancelAnimationFrame(rafId)
  rafId = requestAnimationFrame(draw)
}

function handleMouseMove(e: MouseEvent) {
  const container = containerRef.value
  if (!container) return
  const rect = container.getBoundingClientRect()
  const mouseX = e.clientX - rect.left, chartW = rect.width - padLeft - padRight
  if (mouseX < padLeft || mouseX > padLeft + chartW) { clearHoverTime(); emit('timeHover', null); return }
  const frac = (mouseX - padLeft) / chartW
  const { min: tMin, max: tMax } = timeBounds.value
  const targetTime = Math.round(tMin + frac * (tMax - tMin))
  setHoverTime(targetTime, props.syncGroup)
  emit('timeHover', targetTime)
}

function handleMouseLeave() {
  if (activeHoverGroup.value === props.syncGroup) { clearHoverTime(); emit('timeHover', null) }
}

watch([() => props.series, () => props.height, () => props.thresholds, activeHoverTime], queueDraw, { deep: true })
onMounted(() => {
  queueDraw()
  if (containerRef.value && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(queueDraw)
    resizeObserver.observe(containerRef.value)
  }
})
onBeforeUnmount(() => {
  if (rafId !== null) cancelAnimationFrame(rafId)
  if (resizeObserver) resizeObserver.disconnect()
})
</script>

<template>
  <div
    ref="containerRef"
    class="canvas-timeseries-container"
    @mousemove="handleMouseMove"
    @mouseleave="handleMouseLeave"
  >
    <canvas ref="canvasRef" class="canvas-timeseries"></canvas>
    <div
      v-if="hoverData"
      class="crosshair-tooltip-pill"
      :style="{ left: `${hoverData.pixelX}px`, top: '8px' }"
      :class="{ 'pill-right-aligned': hoverData.pixelX > (containerRef?.clientWidth || 300) - 130 }"
    >
      <div class="pill-time font-mono">{{ hoverData.timeStr }}</div>
      <div class="pill-metrics font-mono">
        <div v-for="it in hoverData.items" :key="it.name" class="pill-item">
          <span class="pill-dot" :style="{ backgroundColor: it.color }"></span>
          <span class="pill-name">{{ it.name }}:</span>
          <span class="pill-val font-semibold">{{ it.value !== null ? it.value.toFixed(1) : '--' }}{{ unit }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.canvas-timeseries-container { position: relative; width: 100%; user-select: none; overflow: hidden; border-radius: 6px; background: rgba(15, 23, 42, 0.45); }
.canvas-timeseries { display: block; width: 100%; }
.crosshair-tooltip-pill {
  position: absolute; pointer-events: none; transform: translateX(-50%);
  background: rgba(15, 23, 42, 0.94); border: 1px solid rgba(56, 189, 248, 0.4);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.5); backdrop-filter: blur(8px);
  padding: 4px 8px; border-radius: 4px; font-size: 11px; line-height: 1.25; white-space: nowrap; z-index: 10;
}
.crosshair-tooltip-pill.pill-right-aligned { transform: translateX(-95%); }
.pill-time { color: #94a3b8; font-size: 10px; margin-bottom: 2px; }
.pill-metrics { display: flex; flex-direction: column; gap: 2px; }
.pill-item { display: flex; align-items: center; gap: 4px; }
.pill-dot { width: 6px; height: 6px; border-radius: 50%; display: inline-block; }
.pill-name { color: #cbd5e1; }
.pill-val { color: #f8fafc; }
</style>
