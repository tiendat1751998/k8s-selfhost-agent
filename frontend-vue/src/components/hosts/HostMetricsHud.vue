<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'

defineProps<{
  totalHosts: number
  connectedHosts: number
  disconnectedHosts: number
  errorHosts: number
  typeCounts: Record<string, number>
}>()
</script>

<template>
  <div class="metrics-grid">
    <MetricCard
      title="Total Hosts"
      :value="totalHosts"
      subtitle="Across 7 multi-type categories"
      icon="🖥️"
      badge="FLEET NODES"
      badge-color="cyan"
    />
    <MetricCard
      title="Connected / Online"
      :value="connectedHosts"
      :subtitle="`${totalHosts > 0 ? Math.round((connectedHosts / totalHosts) * 100) : 100}% Availability Rate`"
      icon="🟢"
      badge="HEALTHY"
      badge-color="emerald"
      trend="Active Mesh"
      trend-type="positive"
    />
    <MetricCard
      title="Offline / Degraded"
      :value="disconnectedHosts + errorHosts"
      :subtitle="`${errorHosts} error, ${disconnectedHosts} disconnected`"
      icon="🚨"
      badge="ALERTS"
      badge-color="rose"
      :trend="errorHosts > 0 ? 'Requires Action' : 'All Clear'"
      :trend-type="errorHosts > 0 ? 'negative' : 'positive'"
    />
    <MetricCard
      title="Swarm & K8s Agents"
      :value="(typeCounts['agent'] || 0) + (typeCounts['k8s'] || 0) + (typeCounts['docker'] || 0)"
      :subtitle="`${typeCounts['k8s'] || 0} k8s · ${typeCounts['docker'] || 0} docker · ${typeCounts['agent'] || 0} agent`"
      icon="☸️"
      badge="CLUSTERS"
      badge-color="violet"
    />
  </div>
</template>
