<script setup lang="ts">
import type { DeploymentApp } from '../../api/compute'
import type { RolloutState } from '../../composables/useDeployments'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import { formatContainerName, formatImageName } from '../../utils/dockerFormat'

interface Props {
  deployments: DeploymentApp[]
  loading?: boolean
  error?: string | null
  actionLoading?: string | null
  getRolloutState: (app: DeploymentApp) => RolloutState
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect', app: DeploymentApp): void
  (e: 'logs', app: DeploymentApp): void
  (e: 'scale', app: DeploymentApp): void
  (e: 'strategy', app: DeploymentApp): void
  (e: 'restart', app: DeploymentApp): void
  (e: 'delete', app: DeploymentApp): void
}>()

const columns: Column<Record<string, unknown>>[] = [
  { key: 'name', label: 'Workload & Namespace', width: '220px', sortable: true },
  { key: 'strategy', label: 'Strategy', width: '130px', sortable: true },
  { key: 'image', label: 'Container Image', width: '180px', sortable: true },
  { key: 'replicas', label: 'Pods / Scale', width: '130px', sortable: true, align: 'center' },
  { key: 'status', label: 'Status', width: '110px', sortable: true },
  { key: 'actions', label: 'Actions', width: '130px', align: 'right' },
]

function asDeployment(row: unknown): DeploymentApp {
  return row as DeploymentApp
}

function getRowActions(app: DeploymentApp): ActionItem[] {
  return [
    { id: 'inspect', label: 'Inspect Details', icon: 'search' },
    { id: 'strategy', label: 'Rollout Strategy', icon: 'git-branch' },
    { id: 'restart', label: props.actionLoading === app.name ? 'Restarting...' : 'Restart Pods', icon: 'refresh', disabled: props.actionLoading === app.name },
    { id: 'sep', label: '', separator: true },
    { id: 'delete', label: 'Delete Workload', icon: 'trash', variant: 'danger', disabled: props.actionLoading === app.name },
  ]
}

function handleActionSelect(actionId: string, app: DeploymentApp) {
  if (actionId === 'inspect') emit('inspect', app)
  else if (actionId === 'strategy') emit('strategy', app)
  else if (actionId === 'restart') emit('restart', app)
  else if (actionId === 'delete') emit('delete', app)
}
</script>

<template>
  <div class="deployments-table-wrap">
    <DataTable
      :columns="columns"
      :data="(deployments as unknown as Record<string, unknown>[])"
      :loading="loading"
      :error="error"
      empty-message="0 Workloads Found. No active workloads match the selected filter criteria."
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
            <span v-if="asDeployment(row).paused" class="paused-badge font-sans"><BaseIcon name="pause" size="xs" /> Paused</span>
            <span v-if="asDeployment(row).ingressHost" class="ingress-badge"><BaseIcon name="globe" size="xs" /> {{ asDeployment(row).ingressHost }}</span>
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

      <!-- Operations Cell: Max 2 Inline Buttons + [ ⋯ ] ActionDropdown -->
      <template #cell-actions="{ row }">
        <div class="action-buttons">
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-logs"
            title="Inspect Real Container Logs"
            @click="emit('logs', asDeployment(row))"
          >
            <BaseIcon name="file-text" size="xs" />
            <span>Logs</span>
          </button>

          <button
            type="button"
            class="btn btn-secondary btn-xs"
            title="Scale Replicas"
            @click="emit('scale', asDeployment(row))"
          >
            <BaseIcon name="zap" size="xs" />
            <span>Scale</span>
          </button>

          <ActionDropdown
            :items="getRowActions(asDeployment(row))"
            size="xs"
            trigger-title="Workload Operations"
            @select="handleActionSelect($event, asDeployment(row))"
          />
        </div>
      </template>
    </DataTable>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/deployments.css';
</style>
