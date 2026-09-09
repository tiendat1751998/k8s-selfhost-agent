<script setup lang="ts">
import MetricCard from '../ui/MetricCard.vue'

defineProps<{
  totalControlPlanes: number
  k8sClusterCount: number
  hasSwarm: boolean
  healthyCount: number
  totalNodes: number
  k8sNodes: number
  swarmNodes: number
  cloudProviders: string[]
}>()
</script>

<template>
  <div class="metrics-grid">
    <MetricCard
      title="Fleet Control Planes"
      :value="totalControlPlanes"
      :subtitle="`${k8sClusterCount} K8s · ${hasSwarm ? '1 Docker Swarm' : '0 Swarm'}`"
      icon="🌐"
      badge="TOPOLOGY"
      badge-color="cyan"
    />
    <MetricCard
      title="Healthy Control Planes"
      :value="totalControlPlanes > 0 ? `${healthyCount}/${totalControlPlanes}` : '0/0'"
      subtitle="Clusters passing control plane health checks"
      icon="🛡️"
      badge="HEALTH"
      badge-color="emerald"
      trend="Continuous Probing"
      trend-type="positive"
    />
    <MetricCard
      title="Total Fleet Nodes"
      :value="totalNodes"
      :subtitle="`${k8sNodes} K8s nodes · ${swarmNodes} Swarm nodes`"
      icon="🖥️"
      badge="COMPUTE"
      badge-color="violet"
    />
    <MetricCard
      title="Active Engine Providers"
      :value="cloudProviders.length"
      :subtitle="cloudProviders.length > 0 ? cloudProviders.join(', ') : 'No engines attached'"
      icon="☁️"
      badge="HYBRID"
      badge-color="cyan"
    />
  </div>
</template>
