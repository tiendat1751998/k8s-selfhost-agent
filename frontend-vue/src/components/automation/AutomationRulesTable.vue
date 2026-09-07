<script setup lang="ts">
import { computed } from 'vue'
import type { AutomationRule } from '../../api/governance'
import DataTable, { type Column } from '../ui/DataTable.vue'

export interface AutomationRuleRow extends AutomationRule {
  [key: string]: unknown
}

const props = defineProps<{
  rules: AutomationRule[]
  loading: boolean
  error: string | null
  togglingId: string | null
  triggeringId: string | null
  deletingId: string | null
  getTriggerIcon: (type: string) => string
  formatType: (type: string) => string
  formatDate: (date: string) => string
  formatScheduleOrCondition: (rule: AutomationRule) => string
}>()

const emit = defineEmits<{
  (e: 'toggle', id: string, nextState: boolean): void
  (e: 'trigger', rule: AutomationRule): void
  (e: 'edit', rule: AutomationRule): void
  (e: 'delete', id: string): void
}>()

const tableRules = computed(() => props.rules as unknown as AutomationRuleRow[])

const ruleColumns: Column<AutomationRuleRow>[] = [
  { key: 'enabled', label: 'State', width: '80px' },
  { key: 'name', label: 'Rule Name', width: '250px', sortable: true },
  { key: 'trigger_type', label: 'Trigger & Schedule / Webhook', width: '260px', sortable: true },
  { key: 'action_type', label: 'Automated Action', width: '200px', sortable: true },
  { key: 'executions', label: 'Executions', width: '120px', sortable: true },
  { key: 'last_triggered', label: 'Last Triggered', width: '160px', sortable: true },
  { key: 'actions', label: 'Actions', width: '220px', align: 'right' },
]
</script>

<template>
  <div class="desktop-table-container">
    <DataTable
      :columns="ruleColumns"
      :data="tableRules"
      :loading="loading"
      :error="error"
      searchable
      search-placeholder="Search rule name, trigger, schedule, or action..."
      empty-message="No automation rules configured. Create a rule to enable automated self-healing."
    >
      <template #cell-enabled="{ row }">
        <label class="toggle-switch">
          <input
            type="checkbox"
            :checked="row.enabled"
            :disabled="togglingId === row.id"
            @change="emit('toggle', row.id, !row.enabled)"
          />
          <span class="slider"></span>
        </label>
      </template>

      <template #cell-name="{ row }">
        <div class="rule-name-cell">
          <span class="rule-name">{{ row.name }}</span>
          <span class="rule-id font-mono">ID: #{{ row.id.slice(0, 8) }}</span>
        </div>
      </template>

      <template #cell-trigger_type="{ row }">
        <div class="trigger-cell font-mono">
          <div class="trigger-main">
            <span>{{ getTriggerIcon(row.trigger_type) }}</span>
            <span class="font-semibold">{{ formatType(row.trigger_type) }}</span>
          </div>
          <span class="trigger-condition-badge font-mono">
            {{ formatScheduleOrCondition(row) }}
          </span>
        </div>
      </template>

      <template #cell-action_type="{ row }">
        <div class="action-cell font-mono">
          <span class="action-tag">{{ formatType(row.action_type) }}</span>
        </div>
      </template>

      <template #cell-executions="{ row }">
        <span class="font-mono text-emerald font-semibold">{{ row.executions || 0 }} runs</span>
      </template>

      <template #cell-last_triggered="{ row }">
        <span class="font-mono text-muted" style="font-size: 11px;">
          {{ row.last_triggered ? formatDate(row.last_triggered) : 'Never' }}
        </span>
      </template>

      <template #cell-actions="{ row }">
        <div class="actions-cell">
          <button
            class="btn btn-primary btn-sm"
            :disabled="triggeringId === row.id"
            @click="emit('trigger', row)"
          >
            <span>{{ triggeringId === row.id ? '⚡ Running...' : '⚡ Trigger Now' }}</span>
          </button>
          <button
            class="btn btn-secondary btn-sm"
            @click="emit('edit', row)"
          >
            <span>⚙️ Edit</span>
          </button>
          <button
            class="btn btn-crimson-delete btn-sm"
            :disabled="deletingId === row.id"
            @click="emit('delete', row.id)"
          >
            <span>{{ deletingId === row.id ? 'Deleting...' : '🗑 Delete' }}</span>
          </button>
        </div>
      </template>
    </DataTable>
  </div>
</template>
