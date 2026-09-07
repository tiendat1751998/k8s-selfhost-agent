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

function onDownloadPdf(row: Record<string, unknown>) {
  emit('downloadPdf', row as PlatformReport)
}

function onViewCsv(row: Record<string, unknown>) {
  emit('viewCsv', row as PlatformReport)
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
        <button class="btn btn-secondary btn-sm" title="Preview Report" @click="onPreview(row)">
          <span>👁️ Preview</span>
        </button>
        <button class="btn btn-primary btn-sm" title="Download PDF" @click="onDownloadPdf(row)">
          <span>📥 Download PDF</span>
        </button>
        <button class="btn btn-secondary btn-sm" title="View CSV" @click="onViewCsv(row)">
          <span>📊 View CSV</span>
        </button>
        <button class="btn btn-sm btn-delete-crimson" title="Delete Report" @click="onDelete(row)">
          <span>🗑 Delete</span>
        </button>
      </div>
    </template>
  </DataTable>
</template>
