<script setup lang="ts">
import { computed } from 'vue'
import type { AlertRule } from '../../api/management'
import DataTable, { type Column } from '../ui/DataTable.vue'

export type AlertRuleRow = AlertRule & Record<string, unknown>

const props = defineProps<{
  rules: AlertRule[]
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'create'): void
  (e: 'edit', rule: AlertRule): void
  (e: 'delete', id: string): void
  (e: 'toggle', rule: AlertRule): void
}>()

const tableRows = computed<AlertRuleRow[]>(() => props.rules as unknown as AlertRuleRow[])

const ruleColumns: Column<AlertRuleRow>[] = [
  { key: 'Name', label: 'Rule Name & Description', sortable: true },
  { key: 'MetricName', label: 'Prometheus Metric / PromQL', sortable: true },
  { key: 'Condition', label: 'Trigger Condition', sortable: true, width: '160px' },
  { key: 'Severity', label: 'Severity', sortable: true, width: '120px' },
  { key: 'Enabled', label: 'Status', sortable: true, width: '110px' },
  { key: 'actions', label: 'Management', align: 'right', width: '160px' }
]
</script>

<template>
  <DataTable
    :columns="ruleColumns"
    :data="tableRows"
    :loading="loading"
    searchable
    searchPlaceholder="Search PromQL rules by name, description, or metric..."
  >
    <template #toolbar>
      <button class="btn btn-primary btn-sm" @click="emit('create')">
        <span>+ Create Rule</span>
      </button>
    </template>

    <template #cell-Name="{ row }">
      <div class="rule-name-cell">
        <span class="rule-title">{{ row.Name }}</span>
        <small class="rule-desc">{{ row.Description }}</small>
      </div>
    </template>

    <template #cell-MetricName="{ value }">
      <span class="font-mono text-cyan">{{ value }}</span>
    </template>

    <template #cell-Condition="{ row }">
      <span class="font-mono text-amber font-bold">
        {{ row.Condition }} {{ row.Threshold }} ({{ row.DurationSeconds }}s)
      </span>
    </template>

    <template #cell-Severity="{ value }">
      <span 
        class="badge font-mono font-bold" 
        :class="value === 'critical' ? 'badge-rose' : value === 'high' ? 'badge-amber' : 'badge-cyan'"
      >
        {{ String(value).toUpperCase() }}
      </span>
    </template>

    <template #cell-Enabled="{ row }">
      <button 
        class="btn-state" 
        :class="row.Enabled ? 'state-active' : 'state-disabled'"
        @click="emit('toggle', row as AlertRule)"
      >
        {{ row.Enabled ? 'ARMED' : 'PAUSED' }}
      </button>
    </template>

    <template #cell-actions="{ row }">
      <div class="flex items-center justify-end gap-2">
        <button 
          class="btn btn-secondary btn-sm" 
          title="Edit Rule Configuration" 
          @click="emit('edit', row as AlertRule)"
        >
          <span>⚙️ Edit</span>
        </button>
        <button 
          class="btn btn-sm btn-delete-crimson" 
          title="Delete Rule" 
          @click="emit('delete', String(row.ID))"
        >
          <span>🗑 Delete</span>
        </button>
      </div>
    </template>
  </DataTable>
</template>
