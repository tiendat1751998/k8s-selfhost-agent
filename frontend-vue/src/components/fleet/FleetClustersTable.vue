<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Cluster } from '../../api/fleet'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import FacetedFilterBar, { type FilterFacet } from '../ui/FacetedFilterBar.vue'

const props = defineProps<{
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

const searchQuery = ref('')
const activeFilters = ref<Record<string, string>>({})

const clusterFacets: FilterFacet[] = [
  {
    key: 'status',
    label: 'Status',
    options: [
      { value: 'healthy', label: 'Healthy' },
      { value: 'degraded', label: 'Degraded' },
      { value: 'offline', label: 'Offline' },
      { value: 'active', label: 'Active' },
    ]
  },
  {
    key: 'tier',
    label: 'Tier',
    options: [
      { value: 'production', label: 'Production' },
      { value: 'staging', label: 'Staging' },
      { value: 'edge', label: 'Edge' },
      { value: 'default', label: 'Default' },
    ]
  }
]

const filteredClusters = computed(() => {
  let list = props.clusters || []
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(c =>
      (c.name || '').toLowerCase().includes(q) ||
      (c.provider || '').toLowerCase().includes(q) ||
      (c.region || '').toLowerCase().includes(q) ||
      (c.group || '').toLowerCase().includes(q)
    )
  }
  const statusFilter = activeFilters.value.status
  if (statusFilter && statusFilter !== 'all') {
    list = list.filter(c => {
      const s = (c.health_status || c.status || '').toLowerCase()
      return s === statusFilter.toLowerCase()
    })
  }
  const tierFilter = activeFilters.value.tier
  if (tierFilter && tierFilter !== 'all') {
    list = list.filter(c => (c.group || 'default').toLowerCase() === tierFilter.toLowerCase())
  }
  return list
})

const CLUSTER_ACTIONS: ActionItem[] = [
  { id: 'discover', label: 'Discover Resources', icon: 'search' },
  { id: 'upgrade', label: 'Upgrade Cluster', icon: 'arrow-up' },
  { id: 'sep-1', label: '', separator: true },
  { id: 'remove', label: 'Evict Cluster', icon: 'trash', variant: 'danger' },
]

const clusterColumns: Column<Cluster>[] = [
  { key: 'name', label: 'Cluster Name', sortable: true },
  { key: 'group', label: 'Fleet Tier', width: '100px', sortable: true },
  { key: 'provider', label: 'Provider / Region', width: '125px', sortable: true },
  { key: 'version', label: 'K8s Version', width: '95px', sortable: true },
  { key: 'nodes', label: 'Nodes', width: '65px', sortable: true, align: 'center' },
  { key: 'health_status', label: 'Health Status', width: '110px', sortable: true },
  { key: 'actions', label: 'Operations', width: '120px', align: 'right' },
]

function handleClusterAction(actionId: string, cluster: Cluster) {
  if (actionId === 'discover') emit('discover', cluster)
  else if (actionId === 'upgrade') emit('upgrade', cluster)
  else if (actionId === 'remove') emit('remove', cluster)
}
</script>

<template>
  <div class="section-box glass-panel table-box">
    <div class="box-header" style="padding: 16px 20px; border-bottom: 1px solid var(--border-subtle);">
      <div>
        <h2 class="box-title">Fleet Clusters Inventory</h2>
        <p class="box-subtitle">Full tabular inventory with operational controls</p>
      </div>
      <button class="btn btn-secondary btn-xs" @click="emit('import')">
        <span>+ Add Cluster</span>
      </button>
    </div>

    <FacetedFilterBar
      v-model="searchQuery"
      v-model:active-filters="activeFilters"
      :facets="clusterFacets"
      placeholder="Filter clusters by name, tier, region, provider... (Press / to focus)"
    />

    <DataTable
      :columns="clusterColumns"
      :data="filteredClusters"
      :loading="loading"
      :error="error"
      empty-message="No clusters found matching current filters."
    >
      <template #cell-name="{ row }">
        <span
          class="font-mono text-cyan"
          style="font-weight: 700; cursor: pointer;"
          title="View Cluster Details"
          @click="emit('details', row)"
        >
          {{ row.name }}
        </span>
      </template>

      <template #cell-group="{ row }">
        <span class="tier-pill">{{ row.group || 'default' }}</span>
      </template>

      <template #cell-provider="{ row }">
        <span class="text-muted">{{ (row.provider || '').toUpperCase() }} ({{ row.region || 'local' }})</span>
      </template>

      <template #cell-version="{ row }">
        <span class="font-mono text-emerald">{{ row.version || '—' }}</span>
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
            :items="CLUSTER_ACTIONS"
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
