<script setup lang="ts">
import { ref, computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import DataTable, { type Column } from '../ui/DataTable.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import type { SLODefinition, SLOSnapshot } from '../../api/compute'

interface EnrichedSLORow extends SLODefinition {
  targetNum: number
  error_budget: number
  burn_rate: number
  budget_status: string
  actual?: number
  [key: string]: unknown
}

const props = defineProps<{
  definitions: SLODefinition[]
  loading: boolean
  error: string | null
  snapshots: SLOSnapshot[]
}>()

const emit = defineEmits<{
  (e: 'inspect', row: SLODefinition): void
  (e: 'triggerAlert', id: string, service: string): void
  (e: 'deleteSlo', id: string, service: string): void
}>()

const copiedId = ref<string | null>(null)

async function copyQuery(id: string, query: string) {
  try {
    await navigator.clipboard.writeText(query)
    copiedId.value = id
    setTimeout(() => {
      if (copiedId.value === id) copiedId.value = null
    }, 2000)
  } catch (err) {
    console.error('Failed to copy PromQL query:', err)
  }
}

const enrichedRows = computed<EnrichedSLORow[]>(() => {
  return props.definitions.map(def => {
    const snap = props.snapshots.find(s => s.slo_id === def.id || s.service === def.service)
    const targetNum = def.target > 1 ? def.target : def.target * 100
    let budget = 100
    let burn = 1.0
    let status = 'healthy'
    let actual = targetNum

    if (snap) {
      if (typeof snap.error_budget === 'number') {
        budget = snap.error_budget
      } else if (typeof snap.actual === 'number') {
        const targetErr = (100 - targetNum) / 100
        const actualErr = Math.max(0, (100 - snap.actual) / 100)
        budget = targetErr <= 0 ? 100 : Math.max(0, Math.min(100, (1 - actualErr / targetErr) * 100))
      }
      if (typeof snap.burn_rate === 'number') {
        burn = snap.burn_rate
      }
      if (snap.budget_status) {
        status = snap.budget_status
      }
      if (typeof snap.actual === 'number') {
        actual = snap.actual
      }
    }

    return {
      ...def,
      targetNum,
      error_budget: budget,
      burn_rate: burn,
      budget_status: status,
      actual,
    }
  })
})

const sloColumns: Column<EnrichedSLORow>[] = [
  { key: 'service', label: 'Service / Workload', sortable: true },
  { key: 'indicator_type', label: 'SLI Type', width: '100px', sortable: true },
  { key: 'targetNum', label: 'Target', width: '85px', sortable: true },
  { key: 'error_budget', label: 'Error Budget', width: '160px', sortable: true },
  { key: 'burn_rate', label: 'Burn Velocity', width: '115px', sortable: true },
  { key: 'window', label: 'Window', width: '80px', sortable: true },
  { key: 'query', label: 'SLI Query (PromQL)' },
  { key: 'alert_threshold', label: 'Threshold', width: '90px', sortable: true },
  { key: 'actions', label: 'Actions', width: '135px', align: 'right' },
]

function formatPercent(val?: unknown): string {
  if (val === undefined || val === null) return '0.00%'
  const num = typeof val === 'number' ? val : Number(val)
  if (isNaN(num)) return '0.00%'
  const pct = num > 1 ? num : num * 100
  return `${pct.toFixed(2)}%`
}

function getBudgetGaugeClass(budget: number): string {
  if (budget >= 80) return 'gauge-emerald'
  if (budget >= 50) return 'gauge-amber'
  return 'gauge-rose'
}

function getBudgetTextClass(budget: number): string {
  if (budget >= 80) return 'text-emerald'
  if (budget >= 50) return 'text-amber'
  return 'text-rose'
}

function getBurnBadgeClass(rate: number): string {
  if (rate > 14.4) return 'burn-rose'
  if (rate > 2.0) return 'burn-amber'
  return 'burn-emerald'
}

function getRowActions(_row?: EnrichedSLORow): ActionItem[] {
  return [
    { id: 'inspect', label: 'Inspect SLI & Budget', icon: 'search' },
    { id: 'test-alert', label: 'Test Burn Alert', icon: 'zap', variant: 'warning' },
    { id: 'sep', label: '', separator: true },
    { id: 'delete', label: 'Delete SLO', icon: 'trash', variant: 'danger' },
  ]
}

function handleRowAction(actionId: string, row: EnrichedSLORow) {
  if (actionId === 'inspect') {
    emit('inspect', row)
  } else if (actionId === 'test-alert') {
    emit('triggerAlert', String(row.id || ''), String(row.service || ''))
  } else if (actionId === 'delete') {
    emit('deleteSlo', String(row.id || ''), String(row.service || ''))
  }
}
</script>

<template>
  <div class="section-box glass-panel table-box">
    <DataTable
      :columns="sloColumns"
      :data="enrichedRows"
      :loading="loading"
      :error="error"
      empty-message="No SLO definitions configured. Click '+ Add Target' to create one."
    >
      <template #cell-service="{ row }">
        <div class="service-name-cell">
          <BaseIcon name="zap" size="xs" class="service-icon" />
          <span class="service-text font-mono font-bold">{{ row.service }}</span>
        </div>
      </template>

      <template #cell-indicator_type="{ row }">
        <span class="indicator-pill font-mono">{{ row.indicator_type }}</span>
      </template>

      <template #cell-targetNum="{ row }">
        <span class="target-val font-mono text-emerald">{{ formatPercent(row.targetNum) }}</span>
      </template>

      <template #cell-error_budget="{ row }">
        <div class="budget-gauge-cell" :title="`Error budget: ${row.error_budget.toFixed(1)}% remaining`">
          <div class="budget-gauge-track">
            <div
              class="budget-gauge-fill"
              :class="getBudgetGaugeClass(row.error_budget)"
              :style="{ width: `${Math.max(0, Math.min(100, row.error_budget))}%` }"
            ></div>
          </div>
          <span class="budget-percent font-mono" :class="getBudgetTextClass(row.error_budget)">
            {{ row.error_budget.toFixed(1) }}%
          </span>
        </div>
      </template>

      <template #cell-burn_rate="{ row }">
        <div class="burn-rate-cell">
          <span class="burn-badge font-mono" :class="getBurnBadgeClass(row.burn_rate)">
            <BaseIcon name="flame" size="xs" />
            <span>{{ row.burn_rate.toFixed(1) }}x</span>
          </span>
        </div>
      </template>

      <template #cell-window="{ row }">
        <span class="window-badge font-mono">{{ row.window }}</span>
      </template>

      <template #cell-query="{ row }">
        <div class="query-cell">
          <span class="query-code font-mono" :title="String(row.query || '')">
            {{ row.query || '—' }}
          </span>
          <button
            v-if="row.query"
            type="button"
            class="btn-copy-query"
            :title="copiedId === row.id ? 'Copied PromQL query!' : 'Copy PromQL query'"
            :aria-label="copiedId === row.id ? 'Copied' : 'Copy Query'"
            @click.stop="copyQuery(String(row.id), String(row.query))"
          >
            <BaseIcon :name="copiedId === row.id ? 'check' : 'copy'" size="xs" />
          </button>
        </div>
      </template>

      <template #cell-alert_threshold="{ row }">
        <span class="threshold-badge font-mono">{{ typeof row.alert_threshold === 'number' ? `${row.alert_threshold.toFixed(1)}x` : '1.5x' }}</span>
      </template>

      <template #cell-actions="{ row }">
        <div class="table-actions-row">
          <button
            type="button"
            class="btn btn-primary btn-xs"
            title="Inspect SLO & Telemetry"
            @click="emit('inspect', row)"
          >
            <BaseIcon name="zap" size="xs" />
            <span>Inspect</span>
          </button>
          <ActionDropdown
            size="xs"
            :items="getRowActions(row)"
            trigger-title="SLO Operations"
            @select="(actionId) => handleRowAction(actionId, row)"
          />
        </div>
      </template>
    </DataTable>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/slo-table.css';
</style>
