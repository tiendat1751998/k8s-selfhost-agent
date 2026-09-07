<script setup lang="ts">
import type { DeploymentApp } from '../../api/compute'
import type { RolloutState } from '../../composables/useDeployments'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import { formatContainerName, formatImageName } from '../../utils/dockerFormat'

interface Props {
  deployments: DeploymentApp[]
  loading?: boolean
  error?: string | null
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

const columns: Column<Record<string, unknown>>[] = [
  { key: 'name', label: 'Workload & Namespace', sortable: true },
  { key: 'strategy', label: 'Strategy / Pipeline', width: '180px', sortable: true },
  { key: 'image', label: 'Container Image & Rev', sortable: true },
  { key: 'replicas', label: 'Replicas & Scale', width: '140px', sortable: true, align: 'center' },
  { key: 'status', label: 'Health Status', width: '130px', sortable: true },
  { key: 'actions', label: 'Operations', width: '380px', align: 'right' },
]

function asDeployment(row: unknown): DeploymentApp {
  return row as DeploymentApp
}
</script>

<template>
  <div class="deployments-table-wrap">
    <DataTable
      :columns="columns"
      :data="(deployments as unknown as Record<string, unknown>[])"
      :loading="loading"
      :error="error"
      empty-message="0 Workloads Found. No active Kubernetes deployments or Docker Swarm services detected on this cluster/node."
      searchable
      search-placeholder="Search inside table rows..."
    >
      <!-- Name & Namespace Cell -->
      <template #cell-name="{ row }">
        <div class="workload-name-cell">
          <div class="name-primary-row">
            <span
              class="workload-name font-mono cursor-pointer"
              :title="asDeployment(row).name"
              @click="emit('inspect', asDeployment(row))"
            >
              {{ formatContainerName(asDeployment(row).name).serviceName }}
            </span>
            <span
              v-if="formatContainerName(asDeployment(row).name).slotBadgeText"
              class="slot-badge font-mono"
              :title="`Full Task Name: ${asDeployment(row).name}`"
            >
              {{ formatContainerName(asDeployment(row).name).slotBadgeText }}
            </span>
            <span class="runtime-pill font-mono">{{ asDeployment(row).type }}</span>
          </div>
          <div class="name-sub-row font-mono">
            <span class="ns-tag">ns:{{ asDeployment(row).namespace || 'default' }}</span>
            <span class="cluster-tag">cluster:{{ asDeployment(row).target }}</span>
            <span class="team-tag font-sans">{{ asDeployment(row).team }}</span>
          </div>
        </div>
      </template>

      <!-- Strategy / Pipeline Cell -->
      <template #cell-strategy="{ row }">
        <div class="strategy-cell">
          <div v-if="asDeployment(row).strategy === 'Canary'" class="canary-chip">
            <span class="canary-dot"></span>
            <span class="strategy-label font-mono">Canary {{ asDeployment(row).canaryWeight || 20 }}%</span>
            <div class="canary-mini-bar">
              <div class="canary-bar-fill" :style="{ width: `${asDeployment(row).canaryWeight || 20}%` }"></div>
            </div>
          </div>

          <div v-else-if="asDeployment(row).strategy === 'BlueGreen'" class="bluegreen-chip">
            <span
              class="bg-track-dot"
              :class="asDeployment(row).blueGreenActive === 'green' ? 'bg-green-active' : 'bg-blue-active'"
            ></span>
            <span class="strategy-label font-mono">
              BG: {{ (asDeployment(row).blueGreenActive || 'blue').toUpperCase() }} Live
            </span>
          </div>

          <div v-else class="rolling-chip">
            <span class="strategy-label font-mono">RollingUpdate</span>
          </div>
        </div>
      </template>

      <!-- Container Image Cell -->
      <template #cell-image="{ row }">
        <div class="image-cell">
          <span class="image-name font-mono" :title="asDeployment(row).image">
            {{ formatImageName(asDeployment(row).image).display }}
          </span>
          <div class="image-sub-row font-mono">
            <span class="rev-badge">rev #{{ asDeployment(row).revision || 1 }}</span>
            <span v-if="asDeployment(row).paused" class="paused-badge font-sans">⏸️ Paused</span>
            <span v-if="asDeployment(row).ingressHost" class="ingress-badge">🌐 {{ asDeployment(row).ingressHost }}</span>
          </div>
        </div>
      </template>

      <!-- Replicas & Scale Cell / Rollout Progress Indicator -->
      <template #cell-replicas="{ row }">
        <div class="replicas-cell" :title="getRolloutState(asDeployment(row)).statusText">
          <div class="replicas-header-row font-mono">
            <div class="replicas-nums">
              <span class="ready-count text-emerald">
                {{ asDeployment(row).readyReplicas !== undefined ? asDeployment(row).readyReplicas : (asDeployment(row).status === 'healthy' ? asDeployment(row).replicas : 0) }}
              </span>
              <span class="text-muted">/</span>
              <span class="desired-count">{{ asDeployment(row).replicas }}</span>
              <span class="replicas-word">Pods</span>
            </div>
            <span
              class="rollout-chip font-mono"
              :class="getRolloutState(asDeployment(row)).badgeClass"
            >
              <span v-if="getRolloutState(asDeployment(row)).isUpdating" class="rollout-spin-dot"></span>
              {{ getRolloutState(asDeployment(row)).label }}
            </span>
          </div>

          <div class="replicas-progress-track">
            <div
              class="replicas-progress-bar"
              :class="[
                getRolloutState(asDeployment(row)).isUpdating ? 'bg-amber progress-animated-stripes' : 'bg-emerald',
                { 'is-complete': getRolloutState(asDeployment(row)).percent === 100 }
              ]"
              :style="{ width: getRolloutState(asDeployment(row)).percent + '%' }"
            ></div>
          </div>
        </div>
      </template>

      <!-- Health Status Cell -->
      <template #cell-status="{ row }">
        <StatusBadge :status="asDeployment(row).status" size="sm" />
      </template>

      <!-- Operations Cell -->
      <template #cell-actions="{ row }">
        <div class="action-buttons">
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-logs"
            title="Inspect Real Container Logs"
            @click="emit('logs', asDeployment(row))"
          >
            <span>📜 Logs</span>
          </button>

          <button
            type="button"
            class="btn btn-secondary btn-xs"
            title="Scale Replicas"
            @click="emit('scale', asDeployment(row))"
          >
            <span>⚡ Scale</span>
          </button>

          <button
            type="button"
            class="btn btn-secondary btn-xs btn-strategy"
            title="Manage Canary / Blue-Green Rollout Strategy"
            @click="emit('strategy', asDeployment(row))"
          >
            <span>🎯 Strategy</span>
          </button>

          <button
            type="button"
            class="btn btn-secondary btn-xs"
            title="Inspect Details"
            @click="emit('inspect', asDeployment(row))"
          >
            <span>🔍 Details</span>
          </button>

          <button
            type="button"
            class="btn btn-secondary btn-xs"
            :disabled="actionLoading === asDeployment(row).name"
            title="Rolling Restart Pods"
            @click="emit('restart', asDeployment(row))"
          >
            <span :class="{ 'spin-icon': actionLoading === asDeployment(row).name }">🔄</span>
          </button>

          <button
            type="button"
            class="btn btn-secondary btn-xs btn-remove"
            :disabled="actionLoading === asDeployment(row).name"
            title="Terminate Deployment"
            @click="emit('delete', asDeployment(row))"
          >
            <span>🗑️</span>
          </button>
        </div>
      </template>
    </DataTable>
  </div>
</template>

<style scoped>
.workload-name-cell { display: flex; flex-direction: column; gap: 4px; }
.name-primary-row { display: flex; align-items: center; gap: 8px; }
.workload-name { font-weight: 700; font-size: 13px; color: #fff; }
.slot-badge { font-size: 10px; background: rgba(6, 182, 212, 0.12); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.25); padding: 1px 6px; border-radius: 4px; }
.runtime-pill { font-size: 10px; background: rgba(255, 255, 255, 0.06); color: var(--text-muted); border: 1px solid rgba(255, 255, 255, 0.1); padding: 1px 6px; border-radius: 4px; text-transform: uppercase; }
.name-sub-row { display: flex; align-items: center; gap: 8px; font-size: 11px; color: var(--text-muted); }
.ns-tag { color: var(--accent-cyan); background: rgba(6, 182, 212, 0.1); padding: 1px 6px; border-radius: 4px; }
.cluster-tag { color: var(--text-secondary); }
.team-tag { color: var(--text-muted); }

.strategy-cell { display: flex; align-items: center; gap: 6px; }
.canary-chip, .bluegreen-chip, .rolling-chip { display: inline-flex; align-items: center; gap: 6px; padding: 4px 8px; border-radius: 6px; font-size: 11px; }
.canary-chip { background: rgba(251, 191, 36, 0.12); color: #fbbf24; border: 1px solid rgba(251, 191, 36, 0.25); }
.canary-dot { width: 6px; height: 6px; border-radius: 50%; background: #fbbf24; }
.canary-mini-bar { width: 36px; height: 4px; border-radius: 2px; background: rgba(0, 0, 0, 0.3); overflow: hidden; }
.canary-bar-fill { height: 100%; background: #fbbf24; }
.bluegreen-chip { background: rgba(168, 85, 247, 0.12); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.25); }
.bg-track-dot { width: 6px; height: 6px; border-radius: 50%; }
.bg-blue-active { background: #38bdf8; }
.bg-green-active { background: #34d399; }
.rolling-chip { background: rgba(255, 255, 255, 0.05); color: var(--text-muted); border: 1px solid rgba(255, 255, 255, 0.08); }

.image-cell { display: flex; flex-direction: column; gap: 2px; }
.image-name { font-size: 12px; color: #cbd5e1; }
.image-sub-row { display: flex; align-items: center; gap: 6px; font-size: 10px; color: var(--text-muted); }
.rev-badge { background: rgba(255, 255, 255, 0.06); padding: 0 4px; border-radius: 3px; }
.paused-badge { color: #f59e0b; background: rgba(245, 158, 11, 0.15); padding: 0 4px; border-radius: 3px; }
.ingress-badge { color: #38bdf8; }

.replicas-cell { display: flex; flex-direction: column; gap: 4px; width: 100%; }
.replicas-header-row { display: flex; align-items: center; justify-content: space-between; gap: 6px; }
.replicas-nums { display: flex; align-items: center; gap: 3px; font-size: 12px; font-weight: 700; }
.replicas-word { font-size: 10px; color: var(--text-muted); font-weight: 400; margin-left: 2px; }
.rollout-chip { font-size: 10px; padding: 1px 6px; border-radius: 4px; font-weight: 600; }
.chip-ready { background: rgba(16, 185, 129, 0.15); color: #10b981; border: 1px solid rgba(16, 185, 129, 0.3); }
.chip-updating { background: rgba(245, 158, 11, 0.18); color: #f59e0b; border: 1px solid rgba(245, 158, 11, 0.35); }
.chip-paused { background: rgba(100, 116, 139, 0.2); color: #94a3b8; border: 1px solid rgba(100, 116, 139, 0.3); }
.chip-scaled-down { background: rgba(51, 65, 85, 0.3); color: #64748b; }
.rollout-spin-dot { width: 6px; height: 6px; border-radius: 50%; background: #f59e0b; display: inline-block; animation: spin 1s linear infinite; }
.replicas-progress-track { height: 4px; border-radius: 2px; background: rgba(0, 0, 0, 0.4); overflow: hidden; }
.replicas-progress-bar { height: 100%; transition: width 0.3s ease; }
.progress-animated-stripes { background-size: 1rem 1rem; animation: progress-stripes 1s linear infinite; }

.action-buttons { display: flex; align-items: center; justify-content: flex-end; gap: 6px; }
.btn-remove { color: #f87171; }
.btn-remove:hover:not(:disabled) { border-color: rgba(239, 68, 68, 0.5); background: rgba(239, 68, 68, 0.15); color: #ef4444; }
.btn-strategy:hover:not(:disabled) { border-color: rgba(168, 85, 247, 0.4); color: #c084fc; }
.btn-logs:hover:not(:disabled) { border-color: rgba(56, 189, 248, 0.4); color: #38bdf8; }
.spin-icon { display: inline-block; animation: spin 1s linear infinite; }
.cursor-pointer { cursor: pointer; }

@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
</style>