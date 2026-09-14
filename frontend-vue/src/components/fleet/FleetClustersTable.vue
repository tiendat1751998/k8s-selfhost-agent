<script setup lang="ts">
import { useRouter } from 'vue-router'
import type { Cluster } from '../../api/fleet'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'

defineProps<{
  clusters: Cluster[]
  loading?: boolean
  error?: string | null
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'discover', cluster: Cluster): void
  (e: 'upgrade', cluster: Cluster): void
  (e: 'remove', cluster: Cluster): void
  (e: 'details', cluster: Cluster): void
  (e: 'import'): void
}>()

const router = useRouter()

const K8S_ACTIONS: ActionItem[] = [
  { id: 'details', label: 'Cluster Details', icon: 'zap' },
  { id: 'discover', label: 'Discover Resources', icon: 'search' },
  { id: 'upgrade', label: 'Upgrade Cluster', icon: 'arrow-up' },
  { id: 'sep-1', label: '', separator: true },
  { id: 'remove', label: 'Evict Cluster', icon: 'trash', variant: 'danger' },
]

const SWARM_ACTIONS: ActionItem[] = [
  { id: 'details', label: 'Cluster Details', icon: 'zap' },
  { id: 'swarm-hosts', label: 'View Compute Hosts', icon: 'server' },
  { id: 'swarm-services', label: 'View Services', icon: 'layers' },
]

function getClusterActions(cluster: Cluster): ActionItem[] {
  return cluster.orchestrator === 'swarm' ? SWARM_ACTIONS : K8S_ACTIONS
}

const clusterColumns: Column<Cluster>[] = [
  { key: 'name', label: 'Cluster Name', sortable: true },
  { key: 'group', label: 'Fleet Tier', width: '90px', sortable: true },
  { key: 'provider', label: 'Provider / Region', width: '130px', sortable: true },
  { key: 'version', label: 'Version', width: '150px', sortable: true },
  { key: 'nodes', label: 'Nodes', width: '65px', sortable: true, align: 'center' },
  { key: 'health_status', label: 'Health Status', width: '110px', sortable: true },
  { key: 'actions', label: 'Operations', width: '130px', align: 'right' },
]

function handleClusterAction(actionId: string, cluster: Cluster) {
  if (actionId === 'details') emit('details', cluster)
  else if (actionId === 'discover') emit('discover', cluster)
  else if (actionId === 'upgrade') emit('upgrade', cluster)
  else if (actionId === 'remove') emit('remove', cluster)
  else if (actionId === 'swarm-hosts') router.push('/infra/hosts')
  else if (actionId === 'swarm-services') router.push('/compute')
}
</script>

<template>
  <div class="section-box glass-panel table-box">
    <DataTable
      :columns="clusterColumns"
      :data="clusters"
      :loading="loading"
      :error="error"
      empty-message="No clusters found matching current filters."
    >
      <template #cell-name="{ row }">
        <div class="cluster-name-cell">
          <span
            class="font-mono cluster-name-link"
            :class="row.orchestrator === 'swarm' ? 'text-blue' : 'text-cyan'"
            title="View Cluster Details"
            @click="emit('details', row)"
          >
            {{ row.name }}
          </span>
          <span
            v-if="row.orchestrator === 'swarm'"
            class="orchestrator-badge badge-swarm font-mono"
            title="Docker Swarm Cluster"
          >
            <BaseIcon name="layers" size="xs" />
            <span>Docker Swarm</span>
          </span>
          <span
            v-else
            class="orchestrator-badge badge-k8s font-mono"
            title="Kubernetes Cluster"
          >
            <BaseIcon name="anchor" size="xs" />
            <span>Kubernetes</span>
          </span>
        </div>
      </template>

      <template #cell-group="{ row }">
        <span class="tier-pill">{{ row.group || 'default' }}</span>
      </template>

      <template #cell-provider="{ row }">
        <span class="text-muted">{{ (row.provider || '').toUpperCase() }} ({{ row.region || 'local' }})</span>
      </template>

      <template #cell-version="{ row }">
        <span v-if="row.orchestrator === 'swarm'" class="font-mono text-cyan">
          SwarmKit <span v-if="row.swarm_meta" class="text-muted">({{ row.swarm_meta.manager_count }}M / {{ row.swarm_meta.worker_count }}W)</span>
        </span>
        <span v-else class="font-mono text-emerald">{{ row.version || '?' }}</span>
      </template>

      <template #cell-nodes="{ row }">
        <span class="font-mono">{{ row.nodes ?? 0 }}</span>
      </template>

      <template #cell-health_status="{ row }">
        <StatusBadge :status="row.health_status || row.status || 'unknown'" size="sm" />
      </template>

      <template #cell-actions="{ row }">
        <div class="table-actions-row">
          <button
            type="button"
            class="btn btn-primary btn-xs"
            title="Cluster Details"
            @click="emit('details', row)"
          >
            <BaseIcon name="zap" size="xs" />
            <span>Details</span>
          </button>
          <ActionDropdown
            size="xs"
            :items="getClusterActions(row)"
            :disabled="actionLoading === row.id"
            @select="(actionId) => handleClusterAction(actionId, row)"
          />
        </div>
      </template>
    </DataTable>
  </div>
</template>

<style scoped>
.tier-pill {
  font-family: var(--font-sans);
  font-weight: 600;
}

.cluster-name-cell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.cluster-name-link {
  font-weight: 700;
  cursor: pointer;
}

.cluster-name-link:hover {
  text-decoration: underline;
}

.orchestrator-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.02em;
  white-space: nowrap;
}

.badge-k8s {
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.3);
}

.badge-swarm {
  background: rgba(59, 130, 246, 0.15);
  color: #60a5fa;
  border: 1px solid rgba(59, 130, 246, 0.35);
}

.text-blue {
  color: #60a5fa;
}

.table-actions-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.table-actions-row .btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 6px;
  font-size: 11px;
}
</style>
