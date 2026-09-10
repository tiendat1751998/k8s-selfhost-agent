<script setup lang="ts">
import type { AlertHistory } from '../../api/management'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  alerts: AlertHistory[]
}>()

const emit = defineEmits<{
  (e: 'acknowledge', alert: AlertHistory): void
  (e: 'silence', alert: AlertHistory): void
  (e: 'telemetry', alert: AlertHistory): void
}>()
</script>

<template>
  <div class="alerts-mobile-stream">
    <div v-if="alerts.length === 0" class="mobile-empty-alerts glass-panel">
      <span>🚨 No active firing alerts. All services operating normally.</span>
    </div>

    <div 
      v-else
      v-for="alert in alerts" 
      :key="alert.ID" 
      class="mobile-alert-card animate-fade-in"
    >
      <div class="mobile-card-content">
        <div class="mobile-card-headline">
          <StatusBadge 
            :status="alert.Status === 'firing' ? 'danger' : alert.Status === 'acknowledged' ? 'warning' : 'healthy'" 
            :label="alert.Status.toUpperCase()" 
            size="sm" 
          />
          <span class="font-mono text-cyan text-xs font-bold">{{ alert.ID }}</span>
          <span class="font-mono text-rose text-xs font-bold">Val: {{ alert.Value }}</span>
        </div>
        <div class="mobile-card-msg font-mono" :title="alert.Message">
          {{ alert.Message }}
        </div>
        <div class="text-xs text-muted flex items-center gap-1">
          <span>{{ new Date(alert.CreatedAt).toLocaleTimeString() }}</span>
          <span v-if="alert.RuleID" class="text-muted">• {{ alert.RuleID }}</span>
          <span v-if="alert.AcknowledgedBy" class="text-amber">• Ack</span>
        </div>
      </div>

      <div class="mobile-card-actions">
        <button 
          v-if="alert.Status === 'firing'" 
          class="btn-mobile-action btn-mobile-silence" 
          title="Silence Alert (1h)"
          aria-label="Silence Alert"
          @click="emit('silence', alert)"
        >
          <span>🔕</span>
        </button>
        <button 
          v-if="alert.Status === 'firing'" 
          class="btn-mobile-action btn-mobile-ack" 
          title="Acknowledge Alert"
          aria-label="Acknowledge Alert"
          @click="emit('acknowledge', alert)"
        >
          <span>✓</span>
        </button>
        <button 
          class="btn-mobile-action btn-mobile-details" 
          title="Inspect Telemetry Details"
          aria-label="Inspect Telemetry Details"
          @click="emit('telemetry', alert)"
        >
          <span>🔍</span>
        </button>
      </div>
    </div>
  </div>
</template>
