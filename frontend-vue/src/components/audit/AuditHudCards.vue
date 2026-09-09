<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'

defineProps<{
  metrics: {
    totalEvents: number
    securityMutations: number
    administrativeActions: number
    policyDenials: number
    signedPercentage?: number
    securityViolations?: number
  }
  loading?: boolean
}>()
</script>

<template>
  <div class="audit-hud-grid">
    <MetricCard
      title="Total Audit Events"
      :value="loading ? '...' : metrics.totalEvents"
      badge="TRAIL LOGS"
      badge-color="cyan"
      subtitle="Audit events in current window"
      icon="📋"
    />
    <MetricCard
      title="Security Mutations"
      :value="loading ? '...' : metrics.securityMutations"
      badge="MUTATIONS"
      badge-color="amber"
      subtitle="Cluster & workload state changes"
      icon="⚡"
    />
    <MetricCard
      title="Administrative Actions"
      :value="loading ? '...' : metrics.administrativeActions"
      badge="ELEVATED"
      badge-color="violet"
      subtitle="RBAC grants & privilege bindings"
      icon="🛡️"
    />
    <MetricCard
      title="Policy Denials"
      :value="loading ? '...' : metrics.policyDenials"
      :trend="metrics.policyDenials === 0 ? '0 Blocked' : `${metrics.policyDenials} Denied`"
      :trend-type="metrics.policyDenials === 0 ? 'positive' : 'negative'"
      :badge="metrics.policyDenials === 0 ? 'SECURE' : 'ACTION REQ'"
      :badge-color="metrics.policyDenials === 0 ? 'emerald' : 'rose'"
      subtitle="Policy enforcement blocked actions"
      icon="🚫"
    />
  </div>
</template>
