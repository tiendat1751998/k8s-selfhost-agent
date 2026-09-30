<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  totalHosts: number
  connectedHosts: number
  disconnectedHosts: number
  errorHosts: number
  typeCounts: Record<string, number>
}>()
</script>

<template>
  <div class="hosts-metric-strip font-mono" role="status" aria-label="Host Fleet Overview">
    <div class="strip-item">
      <span class="pulse-dot pulse-dot-cyan"></span>
      <span class="strip-val font-bold">{{ totalHosts }} Total Hosts</span>
    </div>
    <span class="strip-sep">·</span>
    <div class="strip-item">
      <BaseIcon name="shield" size="xs" class="text-emerald" />
      <span class="text-emerald font-semibold">{{ connectedHosts }} Online</span>
    </div>
    <span class="strip-sep">·</span>
    <div class="strip-item">
      <BaseIcon name="alert-triangle" size="xs" :class="(disconnectedHosts + errorHosts) > 0 ? 'text-rose' : 'text-slate'" />
      <span :class="(disconnectedHosts + errorHosts) > 0 ? 'text-rose font-semibold' : 'text-slate'">
        {{ disconnectedHosts + errorHosts }} Degraded
      </span>
      <span v-if="errorHosts > 0" class="strip-sub text-muted">({{ errorHosts }} err)</span>
    </div>
    <span class="strip-sep">·</span>
    <div class="strip-item">
      <BaseIcon name="cpu" size="xs" class="text-slate" />
      <span>{{ (typeCounts['agent'] || 0) + (typeCounts['k8s'] || 0) + (typeCounts['docker'] || 0) }} Agents</span>
      <span class="strip-sub text-muted">({{ typeCounts['k8s'] || 0 }} k8s, {{ typeCounts['docker'] || 0 }} swarm, {{ typeCounts['agent'] || 0 }} agent)</span>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/infra-hosts.css';
</style>
