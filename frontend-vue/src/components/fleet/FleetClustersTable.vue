<script setup lang="ts">
import type { Cluster } from '../../api/fleet'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'

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
}>()

const clusterColumns: Column<Cluster>[] = [
  { key: 'name', label: 'Cluster Name', sortable: true },
  { key: 'group', label: 'Fleet Tier', width: '130px', sortable: true },
  { key: 'provider', label: 'Provider / Region', width: '180px', sortable: true },
  { key: 'version', label: 'K8s Version', width: '130px', sortable: true },
  { key: 'nodes', label: 'Nodes', width: '100px', sortable: true, align: 'center' },
  { key: 'health_status', label: 'Health Status', width: '140px', sortable: true },
  { key: 'actions', label: 'Cluster Operations', width: '260px', align: 'right' },
]
</script>

<template>
  <div v-if="clusters.length > 0" class="section-box glass-panel table-box">
    <div class="box-header" style="padding: 18px 22px; border-bottom: 1px solid var(--border-subtle);">
      <div>
        <h2 class="box-title">Fleet Clusters Inventory</h2>
        <p class="box-subtitle">Full tabular inventory with operational controls</p>
      </div>
    </div>

    <DataTable
      :columns="clusterColumns"
      :data="clusters"
      :loading="loading"
      :error="error"
      empty-message="No clusters found."
      searchable
      search-placeholder="Filter fleet by cluster name, provider, region..."
    >
      <template #cell-name="{ row }">
        <span class="font-mono text-cyan" style="font-weight: 700;">{{ row.name }}</span>
      </template>

      <template #cell-group="{ row }">
        <span class="tier-pill font-mono">{{ row.group }}</span>
      </template>

      <template #cell-provider="{ row }">
        <span class="font-mono text-muted">{{ (row.provider || '').toUpperCase() }} ({{ row.region }})</span>
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
            class="btn btn-secondary btn-xs"
            :disabled="actionLoading === row.id"
            @click="emit('discover', row)"
          >
            <span>Discover</span>
          </button>
          <button
            class="btn btn-secondary btn-xs"
            :disabled="actionLoading === row.id"
            @click="emit('upgrade', row)"
          >
            <span>Upgrade</span>
          </button>
          <button
            class="btn btn-secondary btn-xs btn-remove"
            :disabled="actionLoading === row.id"
            @click="emit('remove', row)"
          >
            <span>Remove</span>
          </button>
        </div>
      </template>
    </DataTable>
  </div>
</template>
