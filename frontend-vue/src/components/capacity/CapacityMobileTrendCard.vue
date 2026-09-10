<script setup lang="ts">
import { computed } from 'vue'
import type { CapacityForecast } from '../../api/governance'
import type { HudMetricItem } from '../../composables/useCapacityForecast'

const props = defineProps<{
  forecasts: CapacityForecast[]
  exhaustion?: HudMetricItem
}>()

const cpuForecast = computed(() => props.forecasts.find(f => f.resource_type.toLowerCase() === 'cpu'))
const memForecast = computed(() => props.forecasts.find(f => ['memory', 'ram'].includes(f.resource_type.toLowerCase())))

function getY(val: number): number {
  const clamped = Math.max(0, Math.min(100, val))
  return 54 - (clamped / 100) * 44
}

const cpuPoints = computed(() => {
  const current = cpuForecast.value?.current_usage ?? 64.2
  const d7 = cpuForecast.value?.forecast_7d ?? 68.5
  const d30 = cpuForecast.value?.forecast_30d ?? 76.1
  const d90 = cpuForecast.value?.forecast_90d ?? 86.4
  return [
    { x: 18, y: getY(current), val: current },
    { x: 110, y: getY(d7), val: d7 },
    { x: 210, y: getY(d30), val: d30 },
    { x: 322, y: getY(d90), val: d90 },
  ]
})

const memPoints = computed(() => {
  const current = memForecast.value?.current_usage ?? 58.7
  const d7 = memForecast.value?.forecast_7d ?? 61.2
  const d30 = memForecast.value?.forecast_30d ?? 67.9
  const d90 = memForecast.value?.forecast_90d ?? 78.3
  return [
    { x: 18, y: getY(current), val: current },
    { x: 110, y: getY(d7), val: d7 },
    { x: 210, y: getY(d30), val: d30 },
    { x: 322, y: getY(d90), val: d90 },
  ]
})

function getBezierPath(pts: { x: number; y: number }[]): string {
  if (pts.length < 2) return ''
  let d = `M ${pts[0].x},${pts[0].y}`
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i]
    const p1 = pts[i + 1]
    const mx = (p0.x + p1.x) / 2
    d += ` C ${mx},${p0.y} ${mx},${p1.y} ${p1.x},${p1.y}`
  }
  return d
}

function getAreaPath(pts: { x: number; y: number }[]): string {
  const line = getBezierPath(pts)
  if (!line) return ''
  return `${line} L ${pts[pts.length - 1].x},58 L ${pts[0].x},58 Z`
}

const isCritical = computed(() => {
  const cpu90 = cpuPoints.value[3]?.val ?? 0
  const mem90 = memPoints.value[3]?.val ?? 0
  return cpu90 >= 85 || mem90 >= 85
})

const isWarning = computed(() => {
  const cpu90 = cpuPoints.value[3]?.val ?? 0
  const mem90 = memPoints.value[3]?.val ?? 0
  return (cpu90 >= 75 || mem90 >= 75) && !isCritical.value
})
</script>

<template>
  <div class="mobile-trend-card glass-panel" role="region" aria-label="Mobile Capacity Projection Trend">
    <div class="mobile-trend-header">
      <div class="mobile-trend-title-box">
        <span
          class="pulse-dot"
          :class="isCritical ? 'pulse-dot-rose' : isWarning ? 'pulse-dot-amber' : 'pulse-dot-cyan'"
        ></span>
        <span class="mobile-trend-title">Forecast Runways</span>
        <span class="mobile-trend-runway font-mono">
          ⏳ {{ exhaustion?.value || '> 90d' }}
        </span>
      </div>
      <div class="mobile-trend-badges font-mono">
        <span class="badge badge-cyan" style="padding: 1px 5px; font-size: 9px;">
          CPU {{ (cpuPoints[3]?.val ?? 0).toFixed(0) }}% @90d
        </span>
        <span class="badge badge-emerald" style="padding: 1px 5px; font-size: 9px;">
          RAM {{ (memPoints[3]?.val ?? 0).toFixed(0) }}% @90d
        </span>
      </div>
    </div>

    <!-- Cubic Bezier SVG Curves -->
    <div class="mobile-trend-svg-box">
      <svg class="mobile-trend-svg" viewBox="0 0 340 60" preserveAspectRatio="none">
        <defs>
          <linearGradient id="cpuGradMobile" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#38bdf8" stop-opacity="0.3" />
            <stop offset="100%" stop-color="#38bdf8" stop-opacity="0.0" />
          </linearGradient>
          <linearGradient id="memGradMobile" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#34d399" stop-opacity="0.25" />
            <stop offset="100%" stop-color="#34d399" stop-opacity="0.0" />
          </linearGradient>
        </defs>

        <!-- Threshold Reference Lines (80% warning, 90% hard eviction) -->
        <line x1="18" y1="18.8" x2="322" y2="18.8" stroke="rgba(245,158,11,0.25)" stroke-dasharray="3,3" />
        <line x1="18" y1="14.4" x2="322" y2="14.4" stroke="rgba(244,63,94,0.3)" stroke-dasharray="3,3" />

        <!-- Memory Curve Area & Line -->
        <path :d="getAreaPath(memPoints)" fill="url(#memGradMobile)" />
        <path :d="getBezierPath(memPoints)" fill="none" stroke="#34d399" stroke-width="2" />
        <circle v-for="(p, i) in memPoints" :key="`m-${i}`" :cx="p.x" :cy="p.y" r="2.5" fill="#34d399" />

        <!-- CPU Curve Area & Line -->
        <path :d="getAreaPath(cpuPoints)" fill="url(#cpuGradMobile)" />
        <path :d="getBezierPath(cpuPoints)" fill="none" stroke="#38bdf8" stroke-width="2.2" />
        <circle v-for="(p, i) in cpuPoints" :key="`c-${i}`" :cx="p.x" :cy="p.y" r="3" fill="#38bdf8" />
      </svg>
    </div>

    <!-- Timeline Footer -->
    <div class="mobile-trend-footer font-mono">
      <span>Now ({{ (cpuPoints[0]?.val ?? 0).toFixed(0) }}%)</span>
      <span>+7d</span>
      <span>+30d</span>
      <span>+90d ({{ (cpuPoints[3]?.val ?? 0).toFixed(0) }}%)</span>
    </div>
  </div>
</template>
