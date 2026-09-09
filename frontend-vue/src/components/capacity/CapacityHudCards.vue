<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'
import type { HudMetricItem } from '../../composables/useCapacityForecast'

defineProps<{
  saturation: HudMetricItem
  exhaustion: HudMetricItem
  binPacking: HudMetricItem
  headroom: HudMetricItem
}>()
</script>

<template>
  <div class="capacity-hud-grid">
    <MetricCard
      title="Cluster Saturation"
      :value="saturation.value"
      :trend="saturation.trend"
      :trend-type="saturation.trendType"
      :badge="saturation.badge"
      :badge-color="saturation.badgeColor"
      :subtitle="saturation.subtitle"
      icon="⚡"
    />
    <MetricCard
      title="Days to Exhaustion"
      :value="exhaustion.value"
      :trend="exhaustion.trend"
      :trend-type="exhaustion.trendType"
      :badge="exhaustion.badge"
      :badge-color="exhaustion.badgeColor"
      :subtitle="exhaustion.subtitle"
      icon="⏳"
    />
    <MetricCard
      title="Bin-Packing Efficiency"
      :value="binPacking.value"
      :trend="binPacking.trend"
      :trend-type="binPacking.trendType"
      :badge="binPacking.badge"
      :badge-color="binPacking.badgeColor"
      :subtitle="binPacking.subtitle"
      icon="📦"
    />
    <MetricCard
      title="Safe Headroom"
      :value="headroom.value"
      :trend="headroom.trend"
      :trend-type="headroom.trendType"
      :badge="headroom.badge"
      :badge-color="headroom.badgeColor"
      :subtitle="headroom.subtitle"
      icon="🛡️"
    />
  </div>
</template>