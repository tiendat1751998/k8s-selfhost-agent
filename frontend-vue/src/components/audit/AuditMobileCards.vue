<script setup lang="ts">
import type { AuditLogEntry } from '../../api/governance'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  logs: AuditLogEntry[]
  loading?: boolean
}>()

defineEmits<{
  (e: 'select-payload', event: AuditLogEntry): void
}>()

function formatStatusDot(st: string): string {
  switch (st) {
    case 'success': return 'dot-success'
    case 'denied': return 'dot-denied'
    case 'error': return 'dot-error'
    default: return 'dot-pending'
  }
}

function formatRelativeTime(d: string): string {
  if (!d) return '-'
  try {
    const diffSec = Math.floor((Date.now() - new Date(d).getTime()) / 1000)
    if (diffSec < 60) return `${diffSec}s ago`
    const diffMin = Math.floor(diffSec / 60)
    if (diffMin < 60) return `${diffMin}m ago`
    const diffHr = Math.floor(diffMin / 60)
    if (diffHr < 24) return `${diffHr}h ago`
    return `${Math.floor(diffHr / 24)}d ago`
  } catch {
    return d
  }
}
</script>

<template>
  <div class="mobile-cards-stream">
    <div v-if="loading" class="text-muted font-mono" style="padding: 16px; text-align: center;">
      ⏳ Loading audit events...
    </div>

    <div v-else-if="logs.length === 0" class="text-muted font-mono" style="padding: 24px; text-align: center;">
      No audit records found matching criteria.
    </div>

    <div
      v-for="entry in logs"
      :key="entry.id"
      class="mobile-audit-card glass-panel"
      role="button"
      tabindex="0"
      aria-label="View audit event payload"
      @click="$emit('select-payload', entry)"
      @keydown.enter="$emit('select-payload', entry)"
    >
      <!-- Row 1: Status Dot + Action Name + Severity Badge + Relative Time -->
      <div class="mobile-card-row-top">
        <div class="mobile-card-top-left">
          <span class="mobile-status-dot" :class="formatStatusDot(entry.status)"></span>
          <span class="mobile-action-name font-mono">{{ entry.action }}</span>
          <StatusBadge :status="entry.severity" :label="entry.severity.toUpperCase()" size="sm" />
        </div>
        <span class="mobile-card-time font-mono">{{ formatRelativeTime(entry.timestamp) }}</span>
      </div>

      <!-- Row 2: Actor + Target Resource + Payload Button (>=32px) -->
      <div class="mobile-card-row-bottom">
        <div class="mobile-card-bottom-left">
          <span class="mobile-actor font-mono">👤 {{ entry.actor }}</span>
          <span class="text-muted">•</span>
          <span class="mobile-target font-mono" :title="entry.target_resource">
            {{ entry.target_resource }}
          </span>
        </div>
        <button
          class="mobile-payload-btn font-mono"
          type="button"
          aria-label="Inspect Event Payload"
          @click.stop="$emit('select-payload', entry)"
        >
          🔍 Payload
        </button>
      </div>
    </div>
  </div>
</template>
