<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'

defineProps<{
  totalMonthlyCost: number
  totalDailyCost: number
  wasteSaved: number
  totalWastedCost: number
  wastePercentage: number
  projectedCost: number
  spotRatio: number
  spotSavings: number
}>()
</script>

<template>
  <div class="metrics-grid">
    <MetricCard
      title="Total Monthly Spend"
      :value="`$${totalMonthlyCost.toLocaleString()}`"
      :trend="`$${totalDailyCost.toLocaleString()} / day`"
      trend-type="neutral"
      badge="RUN-RATE"
      badge-color="emerald"
      subtitle="Aggregated multi-cluster billing"
      icon="💵"
    />
    <MetricCard
      title="Waste Saved & Recovered"
      :value="`$${wasteSaved.toLocaleString()}`"
      :trend="totalWastedCost > 0 ? `$${totalWastedCost.toLocaleString()} actionable` : 'Zero Idle Spend'"
      :trend-type="totalWastedCost > 0 ? 'positive' : 'neutral'"
      :badge="totalWastedCost > 0 ? 'SAVINGS TARGET' : 'OPTIMIZED'"
      badge-color="emerald"
      subtitle="Right-sizing & idle reclaim"
      icon="📉"
    />
    <MetricCard
      title="Monthly Spend Forecast"
      :value="`$${projectedCost.toLocaleString()}`"
      trend="+4.2% projected"
      trend-type="neutral"
      badge="30D FORECAST"
      badge-color="cyan"
      subtitle="P95 trajectory model"
      icon="📈"
    />
    <MetricCard
      title="Spot Instance Ratio"
      :value="`${spotRatio}%`"
      :trend="`$${spotSavings.toLocaleString()} saved / mo`"
      trend-type="positive"
      badge="SPOT SAVINGS"
      badge-color="violet"
      subtitle="Preemptible & spot node workloads"
      icon="⚡"
    />
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/cost.css';
</style>
