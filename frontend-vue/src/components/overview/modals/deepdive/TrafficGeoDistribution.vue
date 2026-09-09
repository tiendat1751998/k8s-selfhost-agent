<script setup lang="ts">
import { computed } from 'vue'
import type { NodeMetrics, TpsSnapshot } from '../../../../api/overview'

export interface TrendPoint {
  time: string
  cpu: number
  mem: number
  disk: number
  reqs: number
}

interface Props {
  trendHistory: TrendPoint[]
  nodes: NodeMetrics[]
  tpsData?: TpsSnapshot | null
}

const props = withDefaults(defineProps<Props>(), {
  trendHistory: () => [],
  nodes: () => [],
  tpsData: null,
})

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
</script>

<template>
  <section class="modal-summary-grid">
    <!-- Card 1: Peak Throughput -->
    <div class="deep-stat-card card-peak-reqs glass-panel">
      <div class="deep-stat-header">
        <span class="deep-stat-title">
          <span class="stat-title-full">Peak Gateway Throughput</span>
          <span class="stat-title-mobile">Peak Gateway</span>
        </span>
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
    <div class="deep-stat-card card-avg-reqs glass-panel">
      <div class="deep-stat-header">
        <span class="deep-stat-title">
          <span class="stat-title-full">Average Gateway Throughput</span>
          <span class="stat-title-mobile">Avg Gateway</span>
        </span>
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
    <div class="deep-stat-card card-cpu glass-panel">
      <div class="deep-stat-header">
        <span class="deep-stat-title">
          <span class="stat-title-full">Peak CPU Saturation</span>
          <span class="stat-title-mobile">Peak CPU</span>
        </span>
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
    <div class="deep-stat-card card-mem glass-panel">
      <div class="deep-stat-header">
        <span class="deep-stat-title">
          <span class="stat-title-full">Peak Memory Pressure</span>
          <span class="stat-title-mobile">Peak RAM</span>
        </span>
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
</template>

<style scoped>
@import '../../../../assets/styles/components/deep-dive-traffic.css';
</style>
