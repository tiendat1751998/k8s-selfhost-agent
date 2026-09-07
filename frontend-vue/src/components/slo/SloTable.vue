<script setup lang="ts">
import DataTable, { type Column } from '../ui/DataTable.vue'
import type { SLODefinition } from '../../api/compute'

const props = defineProps<{
  definitions: SLODefinition[]
  loading: boolean
  error: string | null
  formatPercent: (val?: number) => string
  formatDate: (d?: string) => string
}>()

const emit = defineEmits<{
  (e: 'create'): void
  (e: 'inspect', def: SLODefinition): void
  (e: 'edit', def: SLODefinition): void
  (e: 'delete', defId: string, serviceName: string): void
}>()

const sloColumns: Column<SLODefinition>[] = [
  { key: 'service', label: 'Service / Workload', sortable: true },
  { key: 'indicator_type', label: 'Indicator Type (SLI)', width: '160px', sortable: true },
  { key: 'target', label: 'Target Objective', width: '140px', sortable: true },
  { key: 'window', label: 'Rolling Window', width: '130px', sortable: true },
  { key: 'query', label: 'PromQL SLI Query', width: '260px' },
  { key: 'alert_threshold', label: 'Burn Alert Threshold', width: '160px', sortable: true },
  { key: 'created_at', label: 'Defined At', width: '130px', sortable: true },
  { key: 'actions', label: 'Actions', width: '310px', align: 'right' },
]
</script>

<template>
  <div class="section-box glass-panel table-box">
    <div class="box-header" style="padding: 18px 22px; border-bottom: 1px solid var(--border-subtle);">
      <div>
        <h2 class="box-title">SLO Target Definitions Catalog</h2>
        <p class="box-subtitle">Configured Service Level Objectives with sliding compliance windows and alert thresholds</p>
      </div>
      <button class="btn btn-sm btn-primary" @click="emit('create')">
        <span>➕ Add Target</span>
      </button>
    </div>

    <DataTable
      :columns="sloColumns"
      :data="definitions"
      :loading="loading"
      :error="error"
      empty-message="No SLO definitions configured. Click '+ Create SLO Definition' to add one."
      searchable
      search-placeholder="Search SLO definitions by service..."
    >
      <template #cell-service="{ row }">
        <div class="service-name-cell">
          <span class="service-icon">⚡</span>
          <span class="service-text font-mono font-bold">{{ row.service }}</span>
        </div>
      </template>

      <template #cell-indicator_type="{ row }">
        <span class="indicator-pill font-mono">{{ row.indicator_type }}</span>
      </template>

      <template #cell-target="{ row }">
        <span class="target-val font-mono text-emerald">{{ formatPercent(row.target) }}</span>
      </template>

      <template #cell-window="{ row }">
        <span class="window-badge font-mono">{{ row.window }}</span>
      </template>

      <template #cell-query="{ row }">
        <span class="query-snippet font-mono" :title="row.query || ''">
          {{ row.query || '—' }}
        </span>
      </template>

      <template #cell-alert_threshold="{ row }">
        <span class="threshold-badge font-mono">{{ row.alert_threshold ? `${row.alert_threshold.toFixed(1)}x` : '1.5x' }}</span>
      </template>

      <template #cell-created_at="{ row }">
        <span class="font-mono text-muted">{{ formatDate(row.created_at) }}</span>
      </template>

      <template #cell-actions="{ row }">
        <div class="table-actions-cell">
          <button class="btn-tbl-action btn-tbl-burn" title="Inspect Burn Rate" @click="emit('inspect', row)">
            <span>📈 Burn Rate</span>
          </button>
          <button class="btn-tbl-action btn-tbl-edit" title="Edit SLO Definition" @click="emit('edit', row)">
            <span>✏️ Edit</span>
          </button>
          <button class="btn-tbl-action btn-tbl-del" title="Delete SLO" @click="emit('delete', row.id, row.service)">
            <span>🗑 Delete</span>
          </button>
        </div>
      </template>
    </DataTable>
  </div>
</template>
