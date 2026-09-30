<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
defineProps<{
  totalIncidents: number
  criticalCount: number
  analyzingCount: number
  resolvedCount: number
}>()
</script>

<template>
  <div class="micro-telemetry-strip font-mono" role="status" aria-label="Incident Telemetry Summary">
    <span class="telemetry-item tel-total"><BaseIcon name="zap" size="xs" /> {{ totalIncidents }} inc</span>
    <span class="telemetry-sep">·</span>
    <span class="telemetry-item tel-crit"><BaseIcon name="flame" size="xs" /> {{ criticalCount }} crit</span>
    <span class="telemetry-sep">·</span>
    <span class="telemetry-item tel-act"><BaseIcon name="bot" size="xs" /> {{ analyzingCount }} act</span>
    <span class="telemetry-sep">·</span>
    <span class="telemetry-item tel-res"><BaseIcon name="shield" size="xs" /> {{ resolvedCount }} res</span>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/incidents.css';
</style>
