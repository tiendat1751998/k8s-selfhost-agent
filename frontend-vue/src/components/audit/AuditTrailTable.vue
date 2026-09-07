<script setup lang="ts">
import type { AuditLogEntry } from '../../api/governance'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  logs: AuditLogEntry[]
  loading?: boolean
  error?: string | null
}>()

defineEmits<{
  (e: 'select-payload', event: AuditLogEntry): void
}>()

const columns: Column<AuditLogEntry>[] = [
  { key: 'timestamp', label: 'Timestamp', width: '140px', sortable: true },
  { key: 'actor', label: 'Actor', width: '160px', sortable: true },
  { key: 'action', label: 'Action', width: '180px', sortable: true },
  { key: 'target_resource', label: 'Target Resource', sortable: true },
  { key: 'ip_address', label: 'IP / User-Agent', width: '150px' },
  { key: 'status', label: 'Status', width: '110px', sortable: true },
  { key: 'actions', label: 'Payload', width: '160px', align: 'right' },
]

function formatActionClass(type: string): string {
  switch (type) {
    case 'mutation': return 'action-mutation'
    case 'access': return 'action-access'
    case 'rbac_grant': return 'action-rbac'
    case 'deletion': return 'action-deletion'
    default: return 'action-access'
  }
}

function formatStatus(st: string): 'active' | 'danger' | 'warning' | 'pending' {
  switch (st) {
    case 'success': return 'active'
    case 'denied': return 'danger'
    case 'error': return 'warning'
    default: return 'pending'
  }
}

function formatDate(d: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <div class="audit-table-card">
    <DataTable
      :columns="columns"
      :data="logs"
      :loading="loading"
      :error="error"
      empty-message="No audit trail events matching the query criteria."
    >
      <template #cell-timestamp="{ row }">
        <span class="timestamp-cell font-mono">{{ formatDate(row.timestamp) }}</span>
      </template>

      <template #cell-actor="{ row }">
        <span class="actor-badge">
          <span>👤</span>
          <span class="font-mono">{{ row.actor }}</span>
        </span>
      </template>

      <template #cell-action="{ row }">
        <span class="action-pill" :class="formatActionClass(row.action_type)">
          {{ row.action }}
        </span>
      </template>

      <template #cell-target_resource="{ row }">
        <code class="resource-code font-mono">{{ row.target_resource }}</code>
      </template>

      <template #cell-ip_address="{ row }">
        <div class="ip-pill font-mono" :title="row.user_agent">
          {{ row.ip_address }}
        </div>
      </template>

      <template #cell-status="{ row }">
        <StatusBadge :status="formatStatus(row.status)" :label="row.status.toUpperCase()" size="sm" />
      </template>

      <template #cell-actions="{ row }">
        <button
          class="btn btn-secondary btn-sm btn-payload"
          type="button"
          @click="$emit('select-payload', row)"
        >
          <span>[ 🔍 Event Payload ]</span>
        </button>
      </template>
    </DataTable>
  </div>
</template>
