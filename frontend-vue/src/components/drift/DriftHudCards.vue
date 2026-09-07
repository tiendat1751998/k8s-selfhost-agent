<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'

interface Props {
  driftedCount: number
  criticalCount: number
  remediatedTodayCount: number
  gitReposTracked: number
}

defineProps<Props>()
</script>

<template>
  <div class="metrics-grid">
    <!-- Card 1: Drifted Resources -->
    <MetricCard
      title="Drifted Resources"
      :value="driftedCount"
      :badge="driftedCount === 0 ? 'SYNCHRONIZED' : 'DRIFT DETECTED'"
      :badge-color="driftedCount === 0 ? 'emerald' : 'rose'"
      subtitle="Uncommitted live cluster mutations"
      icon="⚡"
    />

    <!-- Card 2: Critical Out-of-Sync -->
    <MetricCard
      title="Critical Out-of-Sync"
      :value="criticalCount"
      :badge="criticalCount === 0 ? 'ZERO CRITICAL' : 'REQUIRES SYNC'"
      :badge-color="criticalCount === 0 ? 'emerald' : 'amber'"
      :trend="criticalCount === 0 ? 'Compliant' : 'Reconciliation Pending'"
      :trend-type="criticalCount === 0 ? 'positive' : 'negative'"
      subtitle="Workloads with mutated replicas/security"
      icon="🚨"
    />

    <!-- Card 3: Remediated Today -->
    <MetricCard
      title="Remediated Today"
      :value="remediatedTodayCount"
      badge="AUTO-RECONCILED"
      badge-color="cyan"
      subtitle="Cryptographically restored to Git"
      icon="🔄"
    />

    <!-- Card 4: Git Repos Tracked -->
    <MetricCard
      title="Git Repos Tracked"
      :value="gitReposTracked"
      badge="GITOPS SOURCE"
      badge-color="violet"
      subtitle="Flux / ArgoCD monitored sources"
      icon="🐙"
    />
  </div>
</template>