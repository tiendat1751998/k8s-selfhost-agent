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
  <div class="promotions-mobile-section">
    <div v-if="loading" class="mobile-stream-loading font-mono glass-panel">
      <span>⏳ Loading promotions ledger...</span>
    </div>

    <div v-else-if="promotions.length === 0" class="mobile-stream-empty font-mono glass-panel">
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
        role="button"
        tabindex="0"
        title="Tap card to inspect GitOps manifest diff"
        @click="emit('diff', p)"
        @keydown.enter="emit('diff', p)"
      >
        <!-- Card Row 1: Service Name + Route + Status Badge (~24px) -->
        <div class="mobile-card-row-1">
          <div class="mobile-card-identity">
            <span class="mobile-service-name font-mono">{{ p.service }}</span>
            <span class="mobile-env-route font-mono">{{ p.from_env }} ➔ {{ p.to_env }}</span>
          </div>
          <StatusBadge :status="p.status" size="sm" />
        </div>

        <!-- Card Row 2: Target Version / Metadata + Quick Actions (~34px) -->
        <div class="mobile-card-row-2">
          <div class="mobile-card-meta font-mono">
            <span class="mobile-version text-emerald">{{ p.version }}</span>
            <span class="mobile-sep">•</span>
            <span class="mobile-date text-muted">{{ formatDate(p.created_at) }}</span>
          </div>

          <div class="mobile-card-actions" @click.stop>
            <button
              class="mobile-action-btn btn-diff"
              title="Inspect Git Diff"
              aria-label="Inspect Git Diff"
              @click.stop="emit('diff', p)"
            >
              <span>🔍 Diff</span>
            </button>

            <template v-if="p.status === 'pending'">
              <button
                class="mobile-action-btn btn-approve"
                :disabled="actionLoading === p.id"
                title="Approve Promotion"
                aria-label="Approve Promotion"
                @click.stop="emit('approve', p)"
              >
                <span>⚡ Approve</span>
              </button>
              <button
                class="mobile-action-btn btn-reject"
                :disabled="actionLoading === p.id"
                title="Reject Promotion"
                aria-label="Reject Promotion"
                @click.stop="emit('reject', p)"
              >
                <span>🛑 Reject</span>
              </button>
            </template>

            <template v-else-if="p.status === 'approved' || p.status === 'promoting'">
              <button
                class="mobile-action-btn btn-approve"
                :disabled="actionLoading === p.id"
                title="Complete Rollout"
                aria-label="Complete Rollout"
                @click.stop="emit('complete', p)"
              >
                <span>🚀 Rollout</span>
              </button>
              <button
                class="mobile-action-btn btn-abort"
                :disabled="actionLoading === p.id"
                title="Abort Promotion"
                aria-label="Abort Promotion"
                @click.stop="emit('abort', p)"
              >
                <span>🛑 Abort</span>
              </button>
            </template>

            <template v-else-if="p.status === 'completed' || p.status === 'failed'">
              <button
                class="mobile-action-btn btn-rollback"
                :disabled="actionLoading === p.id"
                title="Rollback Release"
                aria-label="Rollback Release"
                @click.stop="emit('rollback', p)"
              >
                <span>⏪ Rollback</span>
              </button>
            </template>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
