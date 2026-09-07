<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'

defineProps<{
  rulesCount: number
  activeRulesCount: number
  executions24hCount: number
  healingSuccessRate: number
  savedEngineeringHours: string | number
}>()
</script>

<template>
  <div class="metrics-grid">
    <MetricCard
      title="Active Workflows"
      :value="rulesCount"
      badge="ARMED"
      badge-color="cyan"
      :subtitle="`${activeRulesCount} active monitoring workflows`"
      icon="⚡"
    />
    <MetricCard
      title="Executions (24h)"
      :value="executions24hCount"
      badge="EXECUTED"
      badge-color="emerald"
      subtitle="Self-healing & automated dispatches"
      icon="🤖"
    />
    <MetricCard
      title="Success Rate"
      :value="`${healingSuccessRate}%`"
      :trend="healingSuccessRate >= 90 ? 'High Reliability' : 'Review Errors'"
      :trend-type="healingSuccessRate >= 90 ? 'positive' : 'negative'"
      badge="AUTO-RESOLVED"
      badge-color="emerald"
      subtitle="Closed without human escalation"
      icon="🛡️"
    />
    <MetricCard
      title="Saved Engineering Hours"
      :value="`${savedEngineeringHours}h`"
      badge="ROI SAVINGS"
      badge-color="violet"
      subtitle="DevOps incident triage toil averted"
      icon="⏱️"
    />
  </div>
</template>
