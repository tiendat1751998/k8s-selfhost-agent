<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'

interface Props {
  data: number[]
  color?: string
  width?: number
  height?: number
}

const props = withDefaults(defineProps<Props>(), { color: '#06b6d4', height: 28 })
const containerRef = ref<HTMLDivElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)
let resizeObserver: ResizeObserver | null = null
let rafId: number | null = null

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

function draw() {
  const canvas = canvasRef.value
  const container = containerRef.value
  if (!canvas || !container) return
  const w = props.width ?? Math.max(10, Math.floor(container.getBoundingClientRect().width))
  const h = props.height
  const dpr = window.devicePixelRatio || 1
  canvas.width = w * dpr
  canvas.height = h * dpr
  canvas.style.width = `${w}px`
  canvas.style.height = `${h}px`

  const ctx = canvas.getContext('2d')
  if (!ctx) return
  ctx.scale(dpr, dpr)
  ctx.clearRect(0, 0, w, h)
  const pts = props.data
  if (!pts || pts.length === 0) return

  const pad = 3
  const drawH = h - pad * 2
  const minVal = Math.min(...pts)
  const maxVal = Math.max(...pts)
  const range = maxVal === minVal ? 1 : maxVal - minVal
  const coords = pts.map((val, idx) => ({
    x: pts.length === 1 ? w / 2 : (idx / (pts.length - 1)) * w,
    y: pad + (1 - (val - minVal) / range) * drawH
  }))

  const grad = ctx.createLinearGradient(0, 0, 0, h)
  grad.addColorStop(0, hexToRgba(props.color, 0.35))
  grad.addColorStop(1, hexToRgba(props.color, 0.02))

  drawCurve(ctx, coords)
  ctx.lineTo(coords[coords.length - 1].x, h)
  ctx.lineTo(coords[0].x, h)
  ctx.closePath()
  ctx.fillStyle = grad
  ctx.fill()

  drawCurve(ctx, coords)
  ctx.strokeStyle = props.color
  ctx.lineWidth = 1.5
  ctx.stroke()
}

function queueDraw() {
  if (rafId !== null) cancelAnimationFrame(rafId)
  rafId = requestAnimationFrame(draw)
}

watch(() => [props.data, props.color, props.width, props.height], queueDraw, { deep: true })
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
  <div ref="containerRef" class="canvas-sparkline-wrapper">
    <canvas ref="canvasRef" class="canvas-sparkline"></canvas>
  </div>
</template>

<style scoped>
.canvas-sparkline-wrapper { display: inline-block; width: 100%; position: relative; line-height: 0; vertical-align: middle; }
.canvas-sparkline { display: block; }
</style>
