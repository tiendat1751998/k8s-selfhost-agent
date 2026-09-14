<script setup lang="ts">
import { computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import type { HudMetricItem } from '../../composables/useCapacityForecast'

const props = defineProps<{
  saturation: HudMetricItem
  exhaustion: HudMetricItem
  binPacking: HudMetricItem
  headroom: HudMetricItem
}>()

const saturationPercent = computed(() => {
  if (!props.saturation?.value) return 0
  const match = props.saturation.value.match(/([\d.]+)/)
  return match ? Math.min(100, Math.max(0, parseFloat(match[1]))) : 0
})

const saturationBarColor = computed(() => {
  const pct = saturationPercent.value
  if (pct >= 85) return 'bg-rose'
  if (pct >= 70) return 'bg-amber'
  return 'bg-cyan'
})

const exhaustionPercent = computed(() => {
  if (!props.exhaustion?.value) return 100
  if (props.exhaustion.value.includes('>')) return 100
  const match = props.exhaustion.value.match(/(\d+)/)
  if (!match) return 75
  const days = parseInt(match[1], 10)
  return Math.min(100, Math.max(10, Math.round((days / 180) * 100)))
})

const exhaustionBarColor = computed(() => {
  if (props.exhaustion?.badgeColor === 'rose') return 'bg-rose'
  if (props.exhaustion?.badgeColor === 'amber') return 'bg-amber'
  return 'bg-emerald'
})

const binPackingPercent = computed(() => {
  if (!props.binPacking?.value) return 75
  const match = props.binPacking.value.match(/([\d.]+)/)
  return match ? Math.min(100, Math.max(0, parseFloat(match[1]))) : 75
})

const headroomPercent = computed(() => {
  if (!props.headroom?.value) return 30
  const match = props.headroom.value.match(/([\d.]+)/)
  return match ? Math.min(100, Math.max(0, parseFloat(match[1]))) : 30
})

const headroomBarColor = computed(() => {
  return headroomPercent.value < 20 ? 'bg-amber' : 'bg-emerald'
})
</script>

<template>
  <div class="capacity-hud-grid">
    <!-- Card 1: Cluster Saturation -->
    <div class="capacity-kpi-card glass-panel" title="Cluster Saturation: Aggregate compute & memory load">
      <div class="kpi-card-header">
        <div class="kpi-card-title-group">
          <BaseIcon name="zap" size="xs" class="text-cyan" />
          <span class="kpi-card-title">Cluster Saturation</span>
        </div>
        <span class="badge kpi-badge" :class="`badge-${saturation?.badgeColor || 'emerald'}`">
          {{ saturation?.badge || 'NOMINAL' }}
        </span>
      </div>
      <div class="kpi-card-body">
        <span class="kpi-card-value font-mono">{{ saturation?.value || '0%' }}</span>
        <span class="kpi-card-trend font-mono" :class="`trend-${saturation?.trendType || 'neutral'}`">
          {{ saturation?.trend || 'Safe Limits' }}
        </span>
      </div>
      <div class="kpi-card-gauge">
        <div class="kpi-gauge-track">
          <div
            class="kpi-gauge-fill"
            :class="saturationBarColor"
            :style="{ width: `${saturationPercent}%` }"
          ></div>
        </div>
      </div>
    </div>

    <!-- Card 2: Days to Exhaustion -->
    <div class="capacity-kpi-card glass-panel" title="Days to Exhaustion: Predicted capacity exhaustion runway">
      <div class="kpi-card-header">
        <div class="kpi-card-title-group">
          <BaseIcon name="clock" size="xs" class="text-amber" />
          <span class="kpi-card-title">Days to Exhaustion</span>
        </div>
        <span class="badge kpi-badge" :class="`badge-${exhaustion?.badgeColor || 'emerald'}`">
          {{ exhaustion?.badge || 'HEALTHY' }}
        </span>
      </div>
      <div class="kpi-card-body">
        <span class="kpi-card-value font-mono">{{ exhaustion?.value || '--' }}</span>
        <span class="kpi-card-trend font-mono" :class="`trend-${exhaustion?.trendType || 'neutral'}`">
          {{ exhaustion?.trend || 'Stable Runway' }}
        </span>
      </div>
      <div class="kpi-card-gauge">
        <div class="kpi-gauge-track">
          <div
            class="kpi-gauge-fill"
            :class="exhaustionBarColor"
            :style="{ width: `${exhaustionPercent}%` }"
          ></div>
        </div>
      </div>
    </div>

    <!-- Card 3: Bin-Packing Efficiency -->
    <div class="capacity-kpi-card glass-panel" title="Bin-Packing Efficiency: Weighted pod-to-allocatable density">
      <div class="kpi-card-header">
        <div class="kpi-card-title-group">
          <BaseIcon name="box" size="xs" class="text-cyan" />
          <span class="kpi-card-title">Bin-Packing Efficiency</span>
        </div>
        <span class="badge kpi-badge" :class="`badge-${binPacking?.badgeColor || 'cyan'}`">
          {{ binPacking?.badge || 'HIGH DENSITY' }}
        </span>
      </div>
      <div class="kpi-card-body">
        <span class="kpi-card-value font-mono">{{ binPacking?.value || '0%' }}</span>
        <span class="kpi-card-trend font-mono" :class="`trend-${binPacking?.trendType || 'neutral'}`">
          {{ binPacking?.trend || 'Optimal Density' }}
        </span>
      </div>
      <div class="kpi-card-gauge">
        <div class="kpi-gauge-track">
          <div
            class="kpi-gauge-fill bg-cyan"
            :style="{ width: `${binPackingPercent}%` }"
          ></div>
        </div>
      </div>
    </div>

    <!-- Card 4: Safe Headroom -->
    <div class="capacity-kpi-card glass-panel" title="Safe Headroom: Guaranteed burst buffer before evictions">
      <div class="kpi-card-header">
        <div class="kpi-card-title-group">
          <BaseIcon name="shield" size="xs" class="text-emerald" />
          <span class="kpi-card-title">Safe Headroom</span>
        </div>
        <span class="badge kpi-badge" :class="`badge-${headroom?.badgeColor || 'emerald'}`">
          {{ headroom?.badge || 'SAFE BUFFER' }}
        </span>
      </div>
      <div class="kpi-card-body">
        <span class="kpi-card-value font-mono">{{ headroom?.value || '0%' }}</span>
        <span class="kpi-card-trend font-mono" :class="`trend-${headroom?.trendType || 'neutral'}`">
          {{ headroom?.trend || 'Buffer Intact' }}
        </span>
      </div>
      <div class="kpi-card-gauge">
        <div class="kpi-gauge-track">
          <div
            class="kpi-gauge-fill"
            :class="headroomBarColor"
            :style="{ width: `${headroomPercent}%` }"
          ></div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/capacity.css';
</style>