<script setup lang="ts">
defineProps<{
  totalIncidents: number
  criticalCount: number
  analyzingCount: number
  resolvedCount: number
}>()
</script>

<template>
  <div class="micro-telemetry-strip font-mono" role="status" aria-label="Incident Telemetry Summary">
    <span class="telemetry-item tel-total">⚡ {{ totalIncidents }} inc</span>
    <span class="telemetry-sep">·</span>
    <span class="telemetry-item tel-crit">🔥 {{ criticalCount }} crit</span>
    <span class="telemetry-sep">·</span>
    <span class="telemetry-item tel-act">🤖 {{ analyzingCount }} act</span>
    <span class="telemetry-sep">·</span>
    <span class="telemetry-item tel-res">🛡️ {{ resolvedCount }} res</span>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/incidents.css';
</style>
