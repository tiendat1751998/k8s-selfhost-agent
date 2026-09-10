<script setup lang="ts">
import type { PlatformReport } from '../../composables/useReports'
import type { Column } from '../ui/DataTable.vue'
import DataTable from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  reports: PlatformReport[]
  loading?: boolean
  columns: Column<PlatformReport>[]
}>()

const emit = defineEmits<{
  preview: [report: PlatformReport]
  downloadPdf: [report: PlatformReport]
  viewCsv: [report: PlatformReport]
  delete: [id: string]
}>()

function onPreview(row: Record<string, unknown>) {
  emit('preview', row as PlatformReport)
}

function onDownload(row: Record<string, unknown>) {
  const rep = row as PlatformReport
  if (rep.format === 'csv') {
    emit('viewCsv', rep)
  } else {
    emit('downloadPdf', rep)
  }
}

function onDelete(row: Record<string, unknown>) {
  emit('delete', String(row.id || ''))
}
</script>

<template>
  <DataTable
    :columns="columns"
    :data="reports"
    :loading="loading"
    searchable
    searchPlaceholder="Filter reports by title, ID, or category..."
  >
    <template #cell-id="{ value }">
      <span class="font-mono text-cyan font-bold">{{ value }}</span>
    </template>

    <template #cell-title="{ row }">
      <div class="report-title-cell">
        <span class="r-title">{{ row.title }}</span>
        <small class="r-by font-mono text-muted">Author: {{ row.created_by }}</small>
      </div>
    </template>

    <template #cell-type="{ value }">
      <span 
        class="badge font-mono font-bold"
        :class="value === 'compliance' ? 'badge-emerald' : value === 'security' ? 'badge-rose' : value === 'cost' ? 'badge-amber' : 'badge-cyan'"
      >
        {{ String(value).toUpperCase() }}
      </span>
    </template>

    <template #cell-format="{ value }">
      <span class="format-chip font-mono uppercase">{{ value }}</span>
    </template>

    <template #cell-status="{ value }">
      <StatusBadge 
        :status="value === 'completed' ? 'healthy' : value === 'generating' ? 'polling' : 'standby'" 
        :label="String(value).toUpperCase()" 
      />
    </template>

    <template #cell-created_at="{ value }">
      <span class="font-mono text-muted text-xs">{{ new Date(String(value)).toLocaleDateString() }}</span>
    </template>

    <template #cell-actions="{ row }">
      <div class="actions-group">
        <button class="btn btn-primary btn-action-32" :title="`Download ${String(row.format || 'Report').toUpperCase()}`" @click="onDownload(row)">
          <span>📥 Download</span>
        </button>
        <button class="btn btn-secondary btn-action-32" title="View / Preview Report" @click="onPreview(row)">
          <span>👁️ View</span>
        </button>
        <button class="btn btn-action-32 btn-delete-crimson" title="Delete Report" @click="onDelete(row)">
          <span>🗑️ Delete</span>
        </button>
      </div>
    </template>
  </DataTable>
</template>

<style scoped>
:deep(.table-scroll-wrapper) {
  overflow-x: hidden !important;
}

:deep(.data-table) {
  table-layout: fixed !important;
  width: 100% !important;
}

:deep(.data-table th),
:deep(.data-table td) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
