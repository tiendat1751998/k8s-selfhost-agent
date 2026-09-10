<script setup lang="ts">
import DataTable, { type Column } from '../ui/DataTable.vue'
import type { SLODefinition, SLOSnapshot } from '../../api/compute'

type SLORow = SLODefinition & Record<string, unknown>

defineProps<{
  definitions: SLODefinition[]
  loading: boolean
  error: string | null
  snapshots: SLOSnapshot[]
}>()

const emit = defineEmits<{
  (e: 'create'): void
  (e: 'inspect', row: SLODefinition): void
  (e: 'triggerAlert', id: string, service: string): void
  (e: 'deleteSlo', id: string, service: string): void
}>()

const sloColumns: Column<SLORow>[] = [
  { key: 'service', label: 'Service / Workload', sortable: true },
  { key: 'indicator_type', label: 'Indicator Type (SLI)', width: '160px', sortable: true },
  { key: 'target', label: 'Target Objective', width: '140px', sortable: true },
  { key: 'window', label: 'Rolling Window', width: '130px', sortable: true },
  { key: 'query', label: 'PromQL SLI Query', width: '280px' },
  { key: 'alert_threshold', label: 'Burn Alert Threshold', width: '160px', sortable: true },
  { key: 'created_at', label: 'Defined At', width: '130px', sortable: true },
  { key: 'actions', label: 'Actions', width: '160px', align: 'right' },
]

function formatPercent(val?: unknown): string {
  if (val === undefined || val === null) return '0.00%'
  const num = typeof val === 'number' ? val : Number(val)
  if (isNaN(num)) return '0.00%'
  const pct = num > 1 ? num : num * 100
  return `${pct.toFixed(2)}%`
}

function formatDate(d?: unknown): string {
  if (!d) return '—'
  try {
    return new Date(String(d)).toLocaleDateString([], { month: 'short', day: 'numeric' })
  } catch {
    return String(d)
  }
}
</script>

<template>
  <div class="section-box glass-panel table-box">
    <div class="box-header">
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
      :data="(definitions as unknown as SLORow[])"
      :loading="loading"
      :error="error"
      empty-message="No SLO definitions configured. Click '+ Create SLO Definition' to add one."
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
        <span class="query-snippet font-mono" :title="String(row.query || '')">
          {{ row.query || '—' }}
        </span>
      </template>

      <template #cell-alert_threshold="{ row }">
        <span class="threshold-badge font-mono">{{ typeof row.alert_threshold === 'number' ? `${row.alert_threshold.toFixed(1)}x` : '1.5x' }}</span>
      </template>

      <template #cell-created_at="{ row }">
        <span class="font-mono text-muted">{{ formatDate(row.created_at) }}</span>
      </template>

      <template #cell-actions="{ row }">
        <div class="table-actions-cell">
          <button class="btn-icon-action" title="Inspect SLI" @click="emit('inspect', (row as unknown as SLODefinition))">
            <span>🔍</span>
          </button>
          <button class="btn-icon-action btn-icon-warn" title="Test Burn Alert" @click="emit('triggerAlert', String(row.id || ''), String(row.service || ''))">
            <span>⚡</span>
          </button>
          <button class="btn-icon-action btn-icon-del" title="Delete SLO" @click="emit('deleteSlo', String(row.id || ''), String(row.service || ''))">
            <span>🗑️</span>
          </button>
        </div>
      </template>
    </DataTable>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/slo.css';
</style>
