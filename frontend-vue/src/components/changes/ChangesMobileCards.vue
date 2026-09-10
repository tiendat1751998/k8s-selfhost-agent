<script setup lang="ts">
import type { TimelineEvent } from '../../composables/useChangesTimeline'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  events: TimelineEvent[]
  loading?: boolean
}>()

defineEmits<{
  diff: [event: TimelineEvent]
  rollback: [event: TimelineEvent]
  details: [event: TimelineEvent]
  approve: [event: TimelineEvent]
  reject: [event: TimelineEvent]
  refresh: []
}>()

function formatMobileTime(isoStr: string): string {
  try {
    const d = new Date(isoStr)
    return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
  } catch {
    return ''
  }
}
</script>

<template>
  <div class="changes-mobile-stream">
    <!-- Loading State -->
    <div v-if="loading" class="mobile-loading-card">
      <div class="loading-spinner-sm"></div>
      <span>Loading change stream...</span>
    </div>

    <!-- Dedicated Empty State -->
    <div v-else-if="events.length === 0" class="mobile-empty-card">
      <span class="empty-icon">📜</span>
      <p class="empty-text">No change audit records matching filters.</p>
      <button class="btn-m-refresh" @click="$emit('refresh')" title="Refresh stream">
        <span>Tap 🔄 to refresh stream</span>
      </button>
    </div>

    <!-- Mobile High-Density Cards Stack (~68-75px/item, 0 horizontal scroll) -->
    <div v-else class="mobile-cards-stack">
      <div
        v-for="event in events"
        :key="event.id"
        class="mobile-card-item glass-panel"
        :class="`mobile-sev-${event.severity}`"
      >
        <!-- Row 1: Header, Pulse & Quick Status (~32px) -->
        <div class="mc-header-row">
          <div class="mc-title-wrap">
            <span class="mc-pulse-dot" :class="`pulse-${event.eventType}`"></span>
            <span class="mc-title" :title="event.title">{{ event.title }}</span>
          </div>
          <div class="mc-meta-wrap font-mono">
            <span class="mc-time">{{ formatMobileTime(event.timestamp) }}</span>
            <StatusBadge :status="event.status" />
          </div>
        </div>

        <!-- Row 2: Target Resource & Touch Action Buttons (>=32px) (~36px) -->
        <div class="mc-action-row">
          <div class="mc-resource-tag font-mono text-cyan" :title="event.resource">
            {{ event.resource }}
          </div>

          <div class="mc-buttons">
            <button
              class="btn-m-action btn-m-diff"
              title="Inspect Diff"
              aria-label="Inspect Diff"
              @click="$emit('diff', event)"
            >
              <span>🔍</span>
            </button>
            <button
              v-if="event.canRollback"
              class="btn-m-action btn-m-rollback"
              title="Rollback"
              aria-label="Rollback"
              @click="$emit('rollback', event)"
            >
              <span>⏪</span>
            </button>
            <button
              v-if="event.canApprove"
              class="btn-m-action btn-m-approve"
              title="Approve"
              aria-label="Approve"
              @click="$emit('approve', event)"
            >
              <span>✓</span>
            </button>
            <button
              v-if="event.canReject"
              class="btn-m-action btn-m-reject"
              title="Reject"
              aria-label="Reject"
              @click="$emit('reject', event)"
            >
              <span>✕</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
