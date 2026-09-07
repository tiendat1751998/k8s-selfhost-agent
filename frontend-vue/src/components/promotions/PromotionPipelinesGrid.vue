<script setup lang="ts">
import type { Promotion, Environment } from '../../api/compute'
import StatusBadge from '../ui/StatusBadge.vue'

const props = defineProps<{
  environments: Environment[]
  promotions: Promotion[]
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'approve', promotion: Promotion): void
  (e: 'reject', promotion: Promotion): void
  (e: 'complete', promotion: Promotion): void
  (e: 'diff', promotion: Promotion): void
  (e: 'rollback', promotion: Promotion): void
  (e: 'request-promotion'): void
}>()

function getPromotionsForEnv(env: Environment): Promotion[] {
  return props.promotions.filter(p => p.to_env === env)
}
</script>

<template>
  <div class="section-box glass-panel">
    <div class="box-header">
      <div>
        <h2 class="box-title">Visual Release Pipeline Board</h2>
        <p class="box-subtitle">Progressive gate stages with active release artifacts</p>
      </div>
    </div>

    <div class="pipeline-board">
      <div v-for="env in environments" :key="env" class="pipeline-column glass-panel">
        <div class="column-header">
          <div class="stage-badge" :class="`stage-${env}`">
            {{ env.toUpperCase() }}
          </div>
          <span class="stage-count font-mono">{{ getPromotionsForEnv(env).length }} Releases</span>
        </div>

        <div class="column-body">
          <div v-if="getPromotionsForEnv(env).length === 0" class="stage-empty">
            <span>No promotion requests for {{ env }}. Click '+ Request Promotion' to submit a release.</span>
          </div>

          <div
            v-for="p in getPromotionsForEnv(env)"
            :key="p.id"
            class="promotion-card glass-panel"
            :class="`p-status-${p.status}`"
          >
            <div class="p-card-top">
              <span class="p-service">{{ p.service }}</span>
              <StatusBadge :status="p.status" size="sm" />
            </div>

            <div class="p-version font-mono">
              <span>{{ p.from_env }} ➔ </span>
              <span class="text-cyan">{{ p.version }}</span>
            </div>

            <div class="p-meta font-mono text-muted">
              <span>By: {{ p.requester }}</span>
            </div>

            <div class="p-card-actions">
              <button
                class="btn btn-secondary btn-xs btn-diff"
                title="Inspect GitOps Manifest Diff"
                @click="emit('diff', p)"
              >
                🔍 Diff
              </button>

              <template v-if="p.status === 'pending'">
                <button
                  class="btn btn-secondary btn-xs"
                  :disabled="actionLoading === p.id"
                  @click="emit('reject', p)"
                >
                  ✕ Reject
                </button>
                <button
                  class="btn btn-primary btn-xs"
                  :disabled="actionLoading === p.id"
                  @click="emit('approve', p)"
                >
                  ✓ Approve
                </button>
              </template>

              <template v-else-if="p.status === 'approved' || p.status === 'promoting'">
                <button
                  class="btn btn-primary btn-xs"
                  :disabled="actionLoading === p.id"
                  @click="emit('complete', p)"
                >
                  Complete Rollout ➔
                </button>
              </template>

              <template v-else-if="p.status === 'completed'">
                <button
                  class="btn btn-secondary btn-xs btn-rollback"
                  :disabled="actionLoading === p.id"
                  @click="emit('rollback', p)"
                >
                  ⏪ Rollback
                </button>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
