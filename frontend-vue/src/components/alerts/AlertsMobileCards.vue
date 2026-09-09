<script setup lang="ts">
import type { AlertHistory } from '../../api/management'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  alerts: AlertHistory[]
}>()

const emit = defineEmits<{
  (e: 'acknowledge', alert: AlertHistory): void
  (e: 'telemetry', alert: AlertHistory): void
}>()
</script>

<template>
  <div class="alerts-mobile-stream">
    <div 
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
          <span class="font-mono text-rose text-xs font-bold">{{ alert.Value }}</span>
        </div>
        <div class="mobile-card-msg font-mono" :title="alert.Message">
          {{ alert.Message }}
        </div>
        <div class="text-xs text-muted">
          {{ new Date(alert.CreatedAt).toLocaleTimeString() }}
          <span v-if="alert.AcknowledgedBy" class="text-amber ml-1">• Ack</span>
        </div>
      </div>

      <div class="mobile-card-actions">
        <button 
          v-if="alert.Status === 'firing'" 
          class="btn btn-primary btn-sm" 
          @click="emit('acknowledge', alert)"
        >
          <span>✓</span>
        </button>
        <button 
          class="btn btn-secondary btn-sm" 
          title="View Details"
          @click="emit('telemetry', alert)"
        >
          <span>🔍</span>
        </button>
      </div>
    </div>
  </div>
</template>
