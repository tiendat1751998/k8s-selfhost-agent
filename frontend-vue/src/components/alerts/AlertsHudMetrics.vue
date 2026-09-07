<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'

defineProps<{
  firingCount: number
  criticalCount: number
  silencedCount: number
  mtta: string | null
}>()
</script>

<template>
  <div class="metrics-grid">
    <MetricCard 
      title="Active Firing" 
      :value="firingCount" 
      :trend="firingCount > 0 ? 'Requires SRE Action' : 'All thresholds nominal'" 
      :trendType="firingCount > 0 ? 'negative' : 'positive'" 
    />
    <MetricCard 
      title="Critical P1" 
      :value="criticalCount" 
      :trend="criticalCount > 0 ? 'Page SRE On-Call' : 'Zero critical incidents'" 
      :trendType="criticalCount > 0 ? 'negative' : 'positive'" 
    />
    <MetricCard 
      title="Silenced Rules" 
      :value="silencedCount" 
      :trend="silencedCount > 0 ? `${silencedCount} rules/alerts silenced` : 'No active silences'" 
      trendType="neutral" 
    />
    <MetricCard 
      title="Mean Time to Acknowledge" 
      :value="mtta || '—'" 
      :trend="mtta ? 'Real MTTA Calculated' : 'No acknowledged alerts'" 
      :trendType="mtta ? 'positive' : 'neutral'" 
    />
  </div>
</template>
