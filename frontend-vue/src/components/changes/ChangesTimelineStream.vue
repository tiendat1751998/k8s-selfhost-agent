<script setup lang="ts">
import { ref } from 'vue'
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
}>()

const expandedEvents = ref<Record<string, boolean>>({})

function toggleDetails(id: string) {
  expandedEvents.value[id] = !expandedEvents.value[id]
}

function formatTime(isoStr: string): string {
  try {
    const d = new Date(isoStr)
    return d.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
  } catch {
    return isoStr
  }
}
</script>

<template>
  <div class="timeline-stream-container glass-panel">
    <!-- Stream Header -->
    <div class="stream-header">
      <div class="stream-header-title">
        <span class="pulse-dot pulse-dot-cyan"></span>
        <span class="heading-text">Unified Change & Governance Timeline</span>
      </div>
      <div class="stream-header-meta">
        <span class="badge badge-cyan font-mono">{{ events.length }} Events</span>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="stream-loading">
      <div class="loading-spinner"></div>
      <span>Synchronizing cluster events & audit log...</span>
    </div>

    <!-- Empty State -->
    <div v-else-if="events.length === 0" class="stream-empty">
      <span class="empty-icon">📭</span>
      <p class="empty-text">No change events match the specified filter criteria.</p>
      <small class="empty-subtext">Adjust the cluster picker, time window, or clear search queries.</small>
    </div>

    <!-- Timeline List (Zero Horizontal Overflow) -->
    <div v-else class="timeline-list">
      <div
        v-for="event in events"
        :key="event.id"
        class="timeline-item"
        :class="`event-type-${event.eventType} severity-${event.severity}`"
      >
        <!-- Timeline Marker & Rail -->
        <div class="timeline-rail">
          <div class="timeline-node" :class="`node-${event.eventType}`">
            <span v-if="event.eventType === 'rfc'">📋</span>
            <span v-else-if="event.eventType === 'gitops'">🔄</span>
            <span v-else-if="event.eventType === 'rollback'">↺</span>
            <span v-else>🛡️</span>
          </div>
          <div class="timeline-line"></div>
        </div>

        <!-- Event Card Content -->
        <div class="timeline-card glass-panel">
          <div class="card-header">
            <div class="card-badges">
              <span class="badge font-mono" :class="event.eventType === 'rfc' ? 'badge-cyan' : event.eventType === 'gitops' ? 'badge-amber' : event.eventType === 'rollback' ? 'badge-emerald' : 'badge-rose'">
                {{ event.category.toUpperCase() }}
              </span>
              <span v-if="event.severity === 'critical' || event.severity === 'high'" class="badge badge-rose">
                HIGH RISK
              </span>
              <StatusBadge :status="event.status" />
            </div>
            <div class="card-time font-mono">{{ formatTime(event.timestamp) }}</div>
          </div>

          <div class="card-body">
            <h3 class="card-title" :title="event.title">{{ event.title }}</h3>
            <p class="card-description" :title="event.description">{{ event.description }}</p>

            <div class="card-resource-meta font-mono">
              <span class="tc-resource text-cyan" :title="event.resource">📦 {{ event.resource }}</span>
              <span class="tc-cluster text-muted" :title="`${event.cluster} / ${event.namespace}`">🖥️ {{ event.cluster }} / {{ event.namespace }}</span>
              <span v-if="event.requester" class="tc-user text-muted" :title="`Requester: ${event.requester}`">👤 {{ event.requester }}</span>
              <span v-if="event.approver" class="tc-approver text-emerald" :title="`Approver: ${event.approver}`">✓ Approved by {{ event.approver }}</span>
            </div>

            <!-- Expandable Details Drawer/Block -->
            <div v-if="expandedEvents[event.id]" class="card-expanded-details animate-fade-in">
              <div class="details-row">
                <span class="d-label">Event ID:</span>
                <span class="d-val font-mono">{{ event.id }}</span>
              </div>
              <div class="details-row">
                <span class="d-label">Resource Kind:</span>
                <span class="d-val font-mono">{{ event.diffPayload?.resourceKind || 'Workload' }}</span>
              </div>
              <div v-if="event.diffPayload?.revision" class="details-row">
                <span class="d-label">Revision Point:</span>
                <span class="d-val font-mono">Rev #{{ event.diffPayload.revision }}</span>
              </div>
            </div>
          </div>

          <!-- Action Buttons Bar (Compact 32px Buttons) -->
          <div class="card-actions-bar">
            <div class="primary-actions">
              <button
                class="btn-stream-action btn-diff"
                title="Inspect Visual Unified Diff"
                @click="$emit('diff', event)"
              >
                <span>🔍 Inspect Diff</span>
              </button>

              <button
                class="btn-stream-action btn-rollback"
                :disabled="!event.canRollback"
                :title="event.canRollback ? 'Rollback to this state' : 'Rollback unavailable for this record'"
                @click="$emit('rollback', event)"
              >
                <span>⏪ Rollback</span>
              </button>

              <button
                class="btn-stream-action btn-details"
                @click="toggleDetails(event.id)"
              >
                <span>👁️ {{ expandedEvents[event.id] ? 'Hide Info' : 'Details' }}</span>
              </button>
            </div>

            <!-- Governance Approval Flow if Pending -->
            <div v-if="event.canApprove || event.canReject" class="governance-actions">
              <button
                v-if="event.canApprove"
                class="btn-stream-action btn-approve"
                @click="$emit('approve', event)"
              >
                <span>✓ Approve</span>
              </button>
              <button
                v-if="event.canReject"
                class="btn-stream-action btn-reject"
                @click="$emit('reject', event)"
              >
                <span>✕ Reject</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
