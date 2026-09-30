<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

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
  <div class="fleet-kpi-bar glass-panel desktop-only font-mono">
    <div class="kpi-item">
      <BaseIcon name="globe" size="xs" class="kpi-icon text-cyan" />
      <span class="kpi-value text-primary font-bold">{{ totalControlPlanes }}</span>
      <span class="kpi-label text-muted">Control Planes</span>
      <span class="kpi-detail text-muted font-xs">({{ k8sClusterCount }} K8s · {{ hasSwarm ? '1 Swarm' : '0 Swarm' }})</span>
    </div>
    <span class="kpi-sep">·</span>
    <div class="kpi-item">
      <BaseIcon name="shield" size="xs" class="kpi-icon text-emerald" />
      <span class="kpi-value text-emerald font-bold">{{ totalControlPlanes > 0 ? `${healthyCount}/${totalControlPlanes}` : '0/0' }}</span>
      <span class="kpi-label text-muted">Healthy</span>
    </div>
    <span class="kpi-sep">·</span>
    <div class="kpi-item">
      <BaseIcon name="server" size="xs" class="kpi-icon text-violet" />
      <span class="kpi-value text-primary font-bold">{{ totalNodes }}</span>
      <span class="kpi-label text-muted">Nodes</span>
      <span class="kpi-detail text-muted font-xs">({{ k8sNodes }} K8s · {{ swarmNodes }} Swarm)</span>
    </div>
    <span class="kpi-sep">·</span>
    <div class="kpi-item">
      <BaseIcon name="cloud" size="xs" class="kpi-icon text-cyan" />
      <span class="kpi-value text-primary font-bold">{{ cloudProviders.length }}</span>
      <span class="kpi-label text-muted">Engine Providers</span>
      <span v-if="cloudProviders.length > 0" class="kpi-detail text-muted font-xs">({{ cloudProviders.join(', ') }})</span>
    </div>
  </div>
</template>
