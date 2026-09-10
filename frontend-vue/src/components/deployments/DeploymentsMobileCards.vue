<script setup lang="ts">
import type { DeploymentApp } from '../../api/compute'
import type { RolloutState } from '../../composables/useDeployments'
import StatusBadge from '../ui/StatusBadge.vue'
import { formatContainerName, formatImageName } from '../../utils/dockerFormat'

interface Props {
  deployments: DeploymentApp[]
  loading?: boolean
  actionLoading?: string | null
  getRolloutState: (app: DeploymentApp) => RolloutState
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect', app: DeploymentApp): void
  (e: 'logs', app: DeploymentApp): void
  (e: 'scale', app: DeploymentApp): void
  (e: 'strategy', app: DeploymentApp): void
  (e: 'restart', app: DeploymentApp): void
  (e: 'delete', app: DeploymentApp): void
}>()
</script>

<template>
  <div class="mobile-card-stream">
    <div v-if="deployments.length === 0 && !loading" class="empty-mobile-state font-mono">
      <span>📦</span>
      <p>0 Workloads matching active filters</p>
    </div>

    <div
      v-for="app in deployments"
      :key="app.rawId || app.id || app.name"
      class="mobile-workload-card glass-panel animate-fade-in"
    >
      <!-- Header Row: Name, Slot, Runtime, Status -->
      <div class="card-header-row" @click="emit('inspect', app)">
        <div class="header-left">
          <div class="workload-title-line">
            <span class="workload-name font-mono" :title="app.name">
              {{ formatContainerName(app.name).serviceName }}
            </span>
            <span
              v-if="formatContainerName(app.name).slotBadgeText"
              class="slot-badge font-mono"
            >
              {{ formatContainerName(app.name).slotBadgeText }}
            </span>
            <span class="runtime-pill font-mono">{{ app.type }}</span>
          </div>
          <div class="workload-sub-line font-mono">
            <span class="ns-tag">ns:{{ app.namespace || 'default' }}</span>
            <span class="image-tag">{{ formatImageName(app.image).display }}</span>
          </div>
        </div>
        <div class="header-right">
          <StatusBadge :status="app.status" size="sm" />
        </div>
      </div>

      <!-- Metrics Row: Replicas & Strategy -->
      <div class="card-meta-row font-mono">
        <!-- Replicas Rollout Info -->
        <div class="meta-item">
          <span class="meta-label">Pods:</span>
          <span class="ready-count text-emerald">
            {{ app.readyReplicas !== undefined ? app.readyReplicas : (app.status === 'healthy' ? app.replicas : 0) }}
          </span>
          <span class="text-muted">/</span>
          <span class="desired-count">{{ app.replicas }}</span>
          <span
            class="rollout-mini-chip"
            :class="getRolloutState(app).badgeClass"
          >
            {{ getRolloutState(app).label }}
          </span>
        </div>

        <!-- Strategy Pill -->
        <div class="meta-item">
          <span class="meta-label">Strategy:</span>
          <span v-if="app.strategy === 'Canary'" class="canary-meta-tag">
            🐥 {{ app.canaryWeight || 20 }}%
          </span>
          <span v-else-if="app.strategy === 'BlueGreen'" class="bg-meta-tag">
            🔄 {{ (app.blueGreenActive || 'blue').toUpperCase() }}
          </span>
          <span v-else class="rolling-meta-tag">
            Rolling
          </span>
        </div>
      </div>

      <!-- Quick Action Buttons Deck -->
      <div class="card-actions-grid">
        <button
          type="button"
          class="btn-card-action btn-logs"
          title="Inspect Container Logs"
          @click.stop="emit('logs', app)"
        >
          <span>📄 Logs</span>
        </button>

        <button
          type="button"
          class="btn-card-action btn-scale"
          title="Scale Replicas & Resources"
          @click.stop="emit('scale', app)"
        >
          <span>⚡ Scale</span>
        </button>

        <button
          type="button"
          class="btn-card-action btn-strategy"
          title="Canary / Blue-Green Rollout Strategy"
          @click.stop="emit('strategy', app)"
        >
          <span>🎯 Strategy</span>
        </button>

        <button
          type="button"
          class="btn-card-action btn-details"
          title="Inspect Details"
          @click="emit('inspect', app)"
        >
          <span>🔍 Details</span>
        </button>

        <button
          type="button"
          class="btn-card-action btn-restart"
          :disabled="actionLoading === app.name"
          title="Rolling Restart Pods"
          @click.stop="emit('restart', app)"
        >
          <span :class="{ 'spin-icon': actionLoading === app.name }">🔄 Restart</span>
        </button>

        <button
          type="button"
          class="btn-card-action btn-delete-crimson"
          :disabled="actionLoading === app.name"
          title="Terminate Deployment"
          @click.stop="emit('delete', app)"
        >
          <span>🗑 Delete</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/deployments.css';
</style>
