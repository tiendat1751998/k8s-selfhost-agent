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
  (e: 'details', cluster: Cluster): void
  (e: 'import'): void
}>()

const clusterColumns: Column<Cluster>[] = [
  { key: 'name', label: 'Cluster Name', sortable: true },
  { key: 'group', label: 'Fleet Tier', width: '120px', sortable: true },
  { key: 'provider', label: 'Provider / Region', width: '170px', sortable: true },
  { key: 'version', label: 'K8s Version', width: '130px', sortable: true },
  { key: 'nodes', label: 'Nodes', width: '90px', sortable: true, align: 'center' },
  { key: 'health_status', label: 'Health Status', width: '130px', sortable: true },
  { key: 'actions', label: 'Cluster Operations', width: '330px', align: 'right' },
]
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

    <DataTable
      :columns="clusterColumns"
      :data="clusters"
      :loading="loading"
      :error="error"
      empty-message="No clusters found matching current filters."
    >
      <template #cell-name="{ row }">
        <span
          class="font-mono text-cyan"
          style="font-weight: 700; cursor: pointer;"
          title="View Cluster Essentials"
          @click="emit('details', row)"
        >
          {{ row.name }}
        </span>
      </template>

      <template #cell-group="{ row }">
        <span class="tier-pill font-mono">{{ row.group || 'default' }}</span>
      </template>

      <template #cell-provider="{ row }">
        <span class="font-mono text-muted">{{ (row.provider || '').toUpperCase() }} ({{ row.region || 'local' }})</span>
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
            class="btn btn-primary btn-xs"
            title="Cluster Essentials"
            @click="emit('details', row)"
          >
            <span>⚡ Essentials</span>
          </button>
          <button
            class="btn btn-secondary btn-xs"
            :disabled="actionLoading === row.id"
            title="Discover Resources"
            @click="emit('discover', row)"
          >
            <span>🔍 Discover</span>
          </button>
          <button
            class="btn btn-secondary btn-xs"
            :disabled="actionLoading === row.id"
            title="Upgrade Cluster"
            @click="emit('upgrade', row)"
          >
            <span>⬆️ Upgrade</span>
          </button>
          <button
            class="btn btn-secondary btn-xs btn-remove btn-evict"
            :disabled="actionLoading === row.id"
            title="Evict Cluster"
            @click="emit('remove', row)"
          >
            <span>🗑️ Evict</span>
          </button>
        </div>
      </template>
    </DataTable>
  </div>
</template>
