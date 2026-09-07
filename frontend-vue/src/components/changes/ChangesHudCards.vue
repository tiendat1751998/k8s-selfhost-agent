<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'

defineProps<{
  totalChanges: number
  totalRollbacks: number
  configDrifts: number
  highRiskMutations: number
  pendingReviews?: number
  activeWindowsCount?: number
}>()
</script>

<template>
  <div class="changes-hud-grid">
    <MetricCard
      title="Total Changes (24h)"
      :value="totalChanges"
      icon="📋"
      badge="Live Sync"
      badgeColor="cyan"
      trend="Operational Audit Activity"
      trendType="positive"
    />
    <MetricCard
      title="Rollback Points"
      :value="totalRollbacks"
      icon="↺"
      badge="Safety Checkpoints"
      badgeColor="amber"
      trend="Reversible Revisions"
      trendType="neutral"
    />
    <MetricCard
      title="Config Drifts"
      :value="configDrifts"
      icon="⚠️"
      :badge="configDrifts > 0 ? 'Drift Alert' : 'In Sync'"
      :badgeColor="configDrifts > 0 ? 'rose' : 'emerald'"
      :trend="configDrifts > 0 ? 'GitOps Desynchronization' : 'Desired State Matching'"
      :trendType="configDrifts > 0 ? 'negative' : 'positive'"
    />
    <MetricCard
      title="High-Risk Mutations"
      :value="highRiskMutations"
      icon="⚡"
      :badge="highRiskMutations > 0 ? 'Action Required' : 'Guarded'"
      :badgeColor="highRiskMutations > 0 ? 'rose' : 'cyan'"
      trend="Emergency & Critical RFCs"
      :trendType="highRiskMutations > 0 ? 'negative' : 'neutral'"
    />
  </div>
</template>
