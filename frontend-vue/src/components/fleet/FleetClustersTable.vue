<script setup lang="ts">
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
  (e: 'inspect', cluster: Cluster): void
  (e: 'probe', cluster: Cluster): void
  (e: 'rebalance', cluster: Cluster): void
  (e: 'cordon', cluster: Cluster): void
  (e: 'evict', cluster: Cluster): void
  (e: 'import'): void
}>()

const clusterActions: ActionItem[] = [
  { action: 'inspect', label: 'Inspect', icon: 'search' },
  { action: 'probe', label: 'Probe', icon: 'activity' },
  { action: 'rebalance', label: 'Rebalance', icon: 'zap' },
  { action: 'cordon', label: 'Cordon / Drain', icon: 'pause' },
  { id: 'sep-1', label: '', separator: true },
  { action: 'evict', label: 'Evict', icon: 'trash', isDestructive: true, variant: 'danger' },
]

const clusterColumns: Column<Cluster>[] = [
  { key: 'name', label: 'Cluster Name', sortable: true },
  { key: 'group', label: 'Fleet Tier', width: '90px', sortable: true },
  { key: 'provider', label: 'Provider / Region', width: '130px', sortable: true },
  { key: 'version', label: 'Version', width: '150px', sortable: true },
  { key: 'nodes', label: 'Nodes', width: '65px', sortable: true, align: 'center' },
  { key: 'health_status', label: 'Health Status', width: '110px', sortable: true },
  { key: 'actions', label: '', width: '40px', align: 'center' },
]

function handleAction(action: string, cluster: Cluster) {
  if (action === 'inspect' || action === 'details') {
    emit('details', cluster)
    emit('inspect', cluster)
  } else if (action === 'probe' || action === 'discover') {
    emit('discover', cluster)
    emit('probe', cluster)
  } else if (action === 'rebalance') {
    emit('rebalance', cluster)
  } else if (action === 'cordon') {
    emit('cordon', cluster)
  } else if (action === 'evict' || action === 'remove') {
    emit('remove', cluster)
    emit('evict', cluster)
  } else if (action === 'upgrade') {
    emit('upgrade', cluster)
  }
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
            class="cluster-name-link"
            title="View Cluster Details"
            @click="emit('details', row)"
          >
            {{ row.name }}
          </span>
          <span
            v-if="row.orchestrator === 'swarm'"
            class="orchestrator-badge badge-swarm"
            title="Docker Swarm Cluster"
          >
            <BaseIcon name="layers" size="xs" />
            <span>Docker Swarm</span>
          </span>
          <span
            v-else
            class="orchestrator-badge badge-k8s"
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
        <span v-if="row.orchestrator === 'swarm'" class="version-text">
          SwarmKit <span v-if="row.swarm_meta" class="text-muted">({{ row.swarm_meta.manager_count }}M / {{ row.swarm_meta.worker_count }}W)</span>
        </span>
        <span v-else class="version-text">{{ row.version || '?' }}</span>
      </template>

      <template #cell-nodes="{ row }">
        <span class="font-mono">{{ row.nodes ?? 0 }}</span>
      </template>

      <template #cell-health_status="{ row }">
        <StatusBadge :status="row.health_status || row.status || 'unknown'" size="sm" />
      </template>

      <template #cell-actions="{ row }">
        <div class="kebab-cell">
          <ActionDropdown
            size="xs"
            :items="clusterActions"
            :disabled="actionLoading === row.id"
            @select="handleAction($event, row)"
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
  font-family: var(--font-sans);
  font-weight: 600;
  color: var(--text-primary);
  cursor: pointer;
}

.cluster-name-link:hover {
  text-decoration: underline;
}

.version-text {
  font-family: var(--font-mono);
  color: var(--text-secondary);
  font-size: 11.5px;
}

.orchestrator-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  font-family: var(--font-sans);
  letter-spacing: 0.02em;
  white-space: nowrap;
}

.badge-k8s {
  background: var(--color-badge-k8s-bg);
  color: var(--color-badge-k8s-text);
  border: 1px solid var(--color-badge-k8s-border);
}

.badge-swarm {
  background: var(--color-badge-docker-bg);
  color: var(--color-badge-docker-text);
  border: 1px solid var(--color-badge-docker-border);
}

.kebab-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
}
</style>
