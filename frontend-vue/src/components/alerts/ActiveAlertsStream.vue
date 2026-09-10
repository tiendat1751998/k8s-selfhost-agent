<script setup lang="ts">
import type { AlertHistory } from '../../api/management'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  alerts: AlertHistory[]
}>()

const emit = defineEmits<{
  (e: 'silence', alert: AlertHistory): void
  (e: 'resolve', alert: AlertHistory): void
  (e: 'telemetry', alert: AlertHistory): void
}>()
</script>

<template>
  <div class="active-alerts-stream">
    <div class="stream-banner-header">
      <div class="stream-pulse-title">
        <span class="pulse-dot"></span>
        <span>Active Firing Alerts Stream ({{ alerts.length }})</span>
      </div>
      <span class="text-xs text-muted font-mono">Real-time SRE Telemetry</span>
    </div>

    <div v-if="alerts.length === 0" class="empty-list glass-panel">
      ✨ All metric thresholds nominal. No active anomalies currently firing.
    </div>

    <div v-else class="stream-cards-list">
      <div v-for="alert in alerts" :key="alert.ID" class="stream-card glass-panel animate-fade-in">
        <div class="stream-card-main">
          <div class="stream-card-meta">
            <StatusBadge status="danger" label="FIRING" size="sm" />
            <span class="font-mono text-cyan font-bold">{{ alert.ID }}</span>
            <span class="font-mono text-muted truncate">Rule: {{ alert.RuleID }}</span>
            <span class="font-mono text-rose font-bold">Val: {{ alert.Value }}</span>
          </div>
          <div class="stream-card-title truncate" :title="alert.Message">{{ alert.Message }}</div>
          <small class="text-muted font-mono text-xs truncate">
            Triggered at {{ new Date(alert.CreatedAt).toLocaleTimeString() }} ({{ new Date(alert.CreatedAt).toLocaleDateString() }})
          </small>
        </div>

        <div class="stream-actions">
          <button 
            class="btn btn-secondary btn-sm" 
            title="Silence alert for 1 hour"
            @click="emit('silence', alert)"
          >
            <span>🔕 Silence</span>
          </button>
          <button 
            class="btn btn-primary btn-sm" 
            title="Resolve alert anomaly"
            @click="emit('resolve', alert)"
          >
            <span>⚡ Resolve</span>
          </button>
          <button 
            class="btn btn-cyan btn-sm" 
            title="Inspect related telemetry and runbook"
            @click="emit('telemetry', alert)"
          >
            <span>🔍 Details</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
