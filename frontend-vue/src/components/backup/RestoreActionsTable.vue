<script setup lang="ts">
import type { RestoreJob } from '../../api/governance'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  restores: RestoreJob[]
  loading?: boolean
  error?: string | null
}>()

const columns: Column<RestoreJob>[] = [
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'id', label: 'Restore ID', width: '140px', sortable: true },
  { key: 'backup_job_id', label: 'Source Snapshot', width: '170px' },
  { key: 'target', label: 'Target Database & Host' },
  { key: 'created_at', label: 'Executed At', width: '160px', sortable: true },
  { key: 'log', label: 'Verification Log' },
]

function formatDate(d?: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleString()
  } catch {
    return d
  }
}
</script>

<template>
  <DataTable
    :columns="columns"
    :data="restores"
    :loading="loading"
    :error="error"
    searchable
    search-placeholder="Search restore jobs, target host or DB..."
    empty-message="No restore executions recorded yet."
  >
    <template #cell-status="{ row }">
      <StatusBadge :status="row.status" :label="row.status.toUpperCase()" size="sm" />
    </template>
    <template #cell-id="{ row }">
      <span class="font-mono text-cyan">#{{ row.id.slice(0, 8) }}</span>
    </template>
    <template #cell-backup_job_id="{ row }">
      <span class="font-mono text-muted">Snapshot #{{ row.backup_job_id.slice(0, 8) }}</span>
    </template>
    <template #cell-target="{ row }">
      <div class="target-cell font-mono">
        <span class="target-db text-emerald">{{ row.target_db_name }}</span>
        <span class="target-host text-muted">@ {{ row.target_db_host }}</span>
      </div>
    </template>
    <template #cell-created_at="{ row }">
      <span class="font-mono text-muted" style="font-size: 11px;">{{ formatDate(row.created_at) }}</span>
    </template>
    <template #cell-log="{ row }">
      <span class="log-preview font-mono" :title="row.verification_log || 'No log details'">
        {{ row.verification_log ? row.verification_log.slice(0, 40) + '...' : 'Replay logs OK' }}
      </span>
    </template>
  </DataTable>
</template>
