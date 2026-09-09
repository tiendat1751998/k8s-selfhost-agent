<template>
  <div class="metrics-grid">
    <MetricCard
      title="Overall Score"
      :value="`${overallScore.toFixed(1)}%`"
      :trend="overallScore >= 80 ? 'Passing Audit' : 'Action Required'"
      :trend-type="overallScore >= 80 ? 'positive' : 'negative'"
      badge="HUD AGGREGATE"
      badge-color="emerald"
      subtitle="Across all regulatory standards"
      icon="📊"
    />
    <MetricCard
      title="Passing Controls"
      :value="`${passingControls} / ${totalControls}`"
      trend="Controls Passed"
      trend-type="positive"
      badge="COMPLIANT"
      badge-color="cyan"
      subtitle="Automated checks validated"
      icon="✅"
    />
    <MetricCard
      title="Critical Failures"
      :value="criticalFailures"
      :trend="criticalFailures > 0 ? 'Blocks Deployments' : 'Zero Blocking Defects'"
      :trend-type="criticalFailures > 0 ? 'negative' : 'positive'"
      :badge="criticalFailures === 0 ? 'CLEAN' : `${criticalFailures} CRITICAL`"
      :badge-color="criticalFailures === 0 ? 'emerald' : 'rose'"
      subtitle="Policy rules failing audit gate"
      icon="🚨"
    />
    <MetricCard
      title="Automated Audit Status"
      :value="auditStatus === 'running' ? 'Scanning...' : 'Automated (Live)'"
      :trend="lastScanTime ? `Last: ${lastScanTime}` : 'Continuous validation'"
      trend-type="neutral"
      :badge="auditStatus === 'running' ? 'ACTIVE' : 'READY'"
      badge-color="violet"
      subtitle="Regulatory posture engine"
      icon="⚡"
    />
  </div>
</template>

<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'
import type { AuditRunStatus } from '../../composables/useCompliance'

defineProps<{
  overallScore: number
  passingControls: number
  totalControls: number
  criticalFailures: number
  auditStatus: AuditRunStatus
  lastScanTime?: string
}>()
</script>
