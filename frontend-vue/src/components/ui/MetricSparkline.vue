<script setup lang="ts">
import { computed, useId } from 'vue'

export interface MetricSparklineProps {
  data: number[]
  width?: number | string
  height?: number | string
  color?: string
  strokeWidth?: number | string
  fill?: boolean
}

const props = withDefaults(defineProps<MetricSparklineProps>(), {
  width: 36,
  height: 12,
  color: '#10b981',
  strokeWidth: 1.5,
  fill: false,
})

const uid = useId()
const gradientId = `sparkline-grad-${uid.replace(/:/g, '')}`

const numWidth = computed(() => {
  const w = typeof props.width === 'number' ? props.width : parseFloat(String(props.width))
  return Number.isFinite(w) && w > 0 ? w : 36
})

const numHeight = computed(() => {
  const h = typeof props.height === 'number' ? props.height : parseFloat(String(props.height))
  return Number.isFinite(h) && h > 0 ? h : 12
})

const numStroke = computed(() => {
  const s = typeof props.strokeWidth === 'number' ? props.strokeWidth : parseFloat(String(props.strokeWidth))
  return Number.isFinite(s) && s > 0 ? s : 1.5
})

interface Point {
  x: number
  y: number
}

const points = computed<Point[]>(() => {
  const raw = props.data
  if (!Array.isArray(raw) || raw.length === 0) return []

  const valid = raw.filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
  if (valid.length === 0) return []

  const w = numWidth.value
  const h = numHeight.value
  const stroke = numStroke.value

  // Vertical padding to ensure line strokes are not clipped at top or bottom
  const padY = Math.max(0.75, stroke / 2)
  const availableH = Math.max(1, h - padY * 2)

  // Single data point: centered horizontal baseline
  if (valid.length === 1) {
    const midY = Number((h / 2).toFixed(2))
    return [
      { x: 0, y: midY },
      { x: w, y: midY },
    ]
  }

  const min = Math.min(...valid)
  const max = Math.max(...valid)

  // All equal values: centered horizontal baseline
  if (max === min) {
    const midY = Number((h / 2).toFixed(2))
    return [
      { x: 0, y: midY },
      { x: w, y: midY },
    ]
  }

  const range = max - min
  const n = valid.length

  return valid.map((val, idx) => {
    const x = Number(((idx / (n - 1)) * w).toFixed(2))
    const ratio = (val - min) / range
    // Inverted for SVG coordinate system (higher value = lower y coordinate)
    const y = Number((h - padY - ratio * availableH).toFixed(2))
    return { x, y }
  })
})

const linePath = computed(() => {
  if (points.value.length === 0) return ''
  const [first, ...rest] = points.value
  return `M ${first.x} ${first.y} ` + rest.map((p) => `L ${p.x} ${p.y}`).join(' ')
})

const areaPath = computed(() => {
  if (!props.fill || points.value.length === 0) return ''
  const h = numHeight.value
  const first = points.value[0]
  const last = points.value[points.value.length - 1]
  return `${linePath.value} L ${last.x} ${h} L ${first.x} ${h} Z`
})
</script>

<template>
  <svg
    class="metric-sparkline"
    :width="numWidth"
    :height="numHeight"
    :viewBox="`0 0 ${numWidth} ${numHeight}`"
    fill="none"
    role="img"
    aria-hidden="true"
  >
    <defs v-if="fill && gradientId">
      <linearGradient :id="gradientId" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" :stop-color="color" stop-opacity="0.3" />
        <stop offset="100%" :stop-color="color" stop-opacity="0.0" />
      </linearGradient>
    </defs>

    <!-- Translucent Area Gradient -->
    <path
      v-if="fill && areaPath"
      :d="areaPath"
      :fill="`url(#${gradientId})`"
      stroke="none"
    />

    <!-- Sparkline Stroke Path -->
    <path
      v-if="linePath"
      :d="linePath"
      :stroke="color"
      :stroke-width="numStroke"
      stroke-linecap="round"
      stroke-linejoin="round"
      fill="none"
    />
  </svg>
</template>

<style scoped>
.metric-sparkline {
  display: inline-block;
  vertical-align: middle;
  flex-shrink: 0;
  overflow: visible;
  line-height: 0;
}
</style>
