<script setup lang="ts">
import type { Promotion } from '../../api/compute'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  promotions: Promotion[]
  loading: boolean
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'approve', promotion: Promotion): void
  (e: 'reject', promotion: Promotion): void
  (e: 'complete', promotion: Promotion): void
  (e: 'diff', promotion: Promotion): void
  (e: 'rollback', promotion: Promotion): void
  (e: 'abort', promotion: Promotion): void
  (e: 'request-promotion'): void
}>()

function formatDate(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <div class="promotions-mobile-section glass-panel">
    <div class="mobile-section-header">
      <div class="mobile-header-title font-mono">
        <span>📱 RELEASE PROMOTION STREAM</span>
        <span class="mobile-badge">{{ promotions.length }} ITEMS</span>
      </div>
    </div>

    <div v-if="loading" class="mobile-stream-loading font-mono">
      <span>⏳ Loading promotions ledger...</span>
    </div>

    <div v-else-if="promotions.length === 0" class="mobile-stream-empty font-mono">
      <span>No promotion requests recorded yet.</span>
      <button class="btn btn-primary btn-xs" @click="emit('request-promotion')">
        + Request Promotion
      </button>
    </div>

    <div v-else class="promotions-mobile-stream">
      <div
        v-for="p in promotions"
        :key="p.id"
        class="mobile-card-item glass-panel"
        :class="`mobile-status-${p.status}`"
      >
        <div class="mobile-card-main">
          <div class="mobile-card-row-1">
            <span class="mobile-service-name font-mono">{{ p.service }}</span>
            <StatusBadge :status="p.status" size="sm" />
          </div>

          <div class="mobile-card-row-2 font-mono">
            <span class="mobile-env-route">{{ p.from_env }} ➔ {{ p.to_env }}</span>
            <span class="mobile-sep">•</span>
            <span class="mobile-version text-emerald">{{ p.version }}</span>
            <span class="mobile-sep">•</span>
            <span class="mobile-date text-muted">{{ formatDate(p.created_at) }}</span>
          </div>
        </div>

        <div class="mobile-card-actions">
          <button
            class="mobile-action-btn btn-diff"
            title="Inspect Git Diff"
            @click="emit('diff', p)"
          >
            🔍 Diff
          </button>

          <button
            v-if="p.status === 'pending'"
            class="mobile-action-btn btn-promote"
            :disabled="actionLoading === p.id"
            @click="emit('approve', p)"
          >
            🚀 Promote
          </button>

          <button
            v-else-if="p.status === 'approved' || p.status === 'promoting'"
            class="mobile-action-btn btn-promote"
            :disabled="actionLoading === p.id"
            @click="emit('complete', p)"
          >
            🚀 Rollout
          </button>

          <button
            v-if="p.status === 'completed'"
            class="mobile-action-btn btn-rollback"
            :disabled="actionLoading === p.id"
            @click="emit('rollback', p)"
          >
            ⏪ Rollback
          </button>

          <button
            v-if="p.status === 'pending' || p.status === 'promoting'"
            class="mobile-action-btn btn-abort"
            :disabled="actionLoading === p.id"
            @click="emit('abort', p)"
          >
            🗑 Abort
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
