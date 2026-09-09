<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { Incident } from '../../api/compute'

defineProps<{
  incidents: Incident[]
  selectedIncident: Incident | null
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'select', incident: Incident): void
  (e: 'triage', incident: Incident): void
  (e: 'rca', incident: Incident): void
  (e: 'resolve', incident: Incident): void
}>()

function formatTime(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <div class="mobile-cards-stream">
    <div v-if="incidents.length === 0" class="empty-list-mobile">
      <span>🛡️ Zero cluster incidents reported</span>
    </div>

    <div
      v-for="inc in incidents"
      :key="inc.id"
      class="mobile-touch-card"
      :class="{ 'mobile-card-active': selectedIncident?.id === inc.id, ['border-sev-' + inc.severity]: true }"
      @click="emit('select', inc)"
    >
      <div class="mobile-card-top">
        <div class="mobile-pod-group">
          <span class="mobile-pod-name" :title="inc.pod_name">{{ inc.pod_name }}</span>
          <StatusBadge :status="inc.severity" size="sm" />
        </div>
        <StatusBadge :status="inc.status" size="sm" />
      </div>

      <div class="mobile-card-bot font-mono">
        <div class="mobile-meta-info">
          <span class="type-pill-sm">{{ inc.type }}</span>
          <span class="mobile-loc text-muted">{{ inc.namespace }} · {{ formatTime(inc.created_at) }}</span>
        </div>

        <div class="mobile-actions-bar" @click.stop>
          <button
            type="button"
            class="mobile-btn-action"
            title="Triage"
            @click="emit('triage', inc)"
          >
            <span>🔍</span>
          </button>
          <button
            type="button"
            class="mobile-btn-action"
            title="RCATimeline"
            @click="emit('rca', inc)"
          >
            <span>🔬</span>
          </button>
          <button
            type="button"
            class="mobile-btn-action mobile-btn-resolve"
            :disabled="actionLoading === 'resolve-' + inc.id || inc.status === 'resolved'"
            title="Resolve"
            @click="emit('resolve', inc)"
          >
            <span>🕑</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
