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
.micro-telemetry-strip {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 20px;
  min-height: 20px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  gap: 6px;
  user-select: none;
  letter-spacing: -0.01em;
  padding: 0 4px;
}
.telemetry-item {
  display: inline-flex;
  align-items: center;
  line-height: 1;
}
.tel-total { color: #38bdf8; }
.tel-crit { color: #f43f5e; }
.tel-act { color: #c084fc; }
.tel-res { color: #34d399; }
.telemetry-sep {
  color: rgba(148, 163, 184, 0.4);
  font-weight: 400;
}
</style>
