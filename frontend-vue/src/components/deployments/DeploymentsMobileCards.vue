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
          class="btn-card-action btn-restart"
          :disabled="actionLoading === app.name"
          title="Rolling Restart Pods"
          @click.stop="emit('restart', app)"
        >
          <span :class="{ 'spin-icon': actionLoading === app.name }">🔄 Restart</span>
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
.mobile-card-stream {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.empty-mobile-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
  background: rgba(15, 23, 42, 0.4);
  border: 1px dashed rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  color: var(--text-muted);
  font-size: 12px;
  gap: 6px;
}

.mobile-workload-card {
  min-height: 70px;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: border-color 0.15s ease, transform 0.15s ease;
}

.mobile-workload-card:hover {
  border-color: rgba(6, 182, 212, 0.3);
}

.card-header-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  cursor: pointer;
}

.header-left {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
  flex: 1;
}

.workload-title-line {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.workload-name {
  font-weight: 700;
  font-size: 13px;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 170px;
}

.slot-badge {
  font-size: 9px;
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.25);
  padding: 1px 5px;
  border-radius: 4px;
}

.runtime-pill {
  font-size: 9px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-muted);
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 1px 4px;
  border-radius: 3px;
  text-transform: uppercase;
}

.workload-sub-line {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ns-tag {
  color: var(--accent-cyan);
  background: rgba(6, 182, 212, 0.08);
  padding: 0 4px;
  border-radius: 3px;
}

.image-tag {
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 140px;
}

.card-meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 10px;
  background: rgba(0, 0, 0, 0.25);
  padding: 4px 8px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.04);
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.meta-label {
  color: var(--text-muted);
}

.rollout-mini-chip {
  font-size: 9px;
  padding: 0 4px;
  border-radius: 3px;
  font-weight: 600;
}

.canary-meta-tag {
  color: #fbbf24;
  font-weight: 600;
}

.bg-meta-tag {
  color: #a855f7;
  font-weight: 600;
}

.rolling-meta-tag {
  color: #94a3b8;
}

.card-actions-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 4px;
}

.btn-card-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 5px 2px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-secondary);
  transition: all 0.15s ease;
  white-space: nowrap;
}

.btn-card-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
}

.btn-card-action:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-logs:hover:not(:disabled) {
  border-color: rgba(56, 189, 248, 0.4);
  color: #38bdf8;
}

.btn-scale:hover:not(:disabled) {
  border-color: rgba(251, 191, 36, 0.4);
  color: #fbbf24;
}

.btn-restart:hover:not(:disabled) {
  border-color: rgba(16, 185, 129, 0.4);
  color: #34d399;
}

.btn-strategy:hover:not(:disabled) {
  border-color: rgba(168, 85, 247, 0.4);
  color: #c084fc;
}

.btn-delete-crimson {
  background: rgba(220, 38, 38, 0.18);
  border: 1px solid rgba(220, 38, 38, 0.45);
  color: #f87171;
}

.btn-delete-crimson:hover:not(:disabled) {
  background: rgba(220, 38, 38, 0.35);
  border-color: #ef4444;
  color: #fff;
  box-shadow: 0 0 8px rgba(239, 68, 68, 0.35);
}

.chip-ready {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}

.chip-updating {
  background: rgba(245, 158, 11, 0.18);
  color: #f59e0b;
}

.chip-paused {
  background: rgba(100, 116, 139, 0.2);
  color: #94a3b8;
}

.chip-scaled-down {
  background: rgba(51, 65, 85, 0.3);
  color: #64748b;
}

.spin-icon {
  display: inline-block;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.animate-fade-in {
  animation: fadeIn 0.15s ease-in-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(2px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>