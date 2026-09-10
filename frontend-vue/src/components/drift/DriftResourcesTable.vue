<script setup lang="ts">
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { EnrichedDriftRecord } from '../../composables/useDriftDetection'

interface Props {
  drifts: EnrichedDriftRecord[]
  loading: boolean
  error: string | null
  resolvingId: string | null
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect', drift: EnrichedDriftRecord): void
  (e: 'sync', id: string): void
  (e: 'suppress', id: string): void
}>()

const columns: Column<EnrichedDriftRecord>[] = [
  { key: 'status', label: 'Status & Severity', width: '110px', sortable: true },
  { key: 'resource', label: 'Resource & Scope', width: '180px', sortable: true },
  { key: 'driftType', label: 'Mutation Type', width: '130px', sortable: true },
  { key: 'diff', label: 'State Mutation Preview' },
  { key: 'detected_at', label: 'Detected At', width: '120px', sortable: true },
  { key: 'actions', label: 'Actions', width: '170px', align: 'right' },
]

function formatDriftStatus(s: string): string {
  if (s === 'drifted') return 'DRIFTED'
  if (s === 'in_sync') return 'IN SYNC'
  return (s || 'UNKNOWN').toUpperCase()
}

function formatDate(d: string): string {
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
    :data="drifts"
    :loading="loading"
    :error="error"
    searchable
    search-placeholder="Search resource, kind, namespace, cluster, or mutation..."
    empty-message="No configuration drift detected. All cluster workloads match Git source."
  >
    <template #cell-status="{ row }">
      <div class="drift-status-cell">
        <StatusBadge :status="row.status" :label="formatDriftStatus(row.status)" size="sm" />
        <span 
          v-if="row.status === 'drifted' && !row.isSuppressed" 
          class="severity-tag"
          :class="`severity-${row.severity}`"
        >
          {{ row.severity.toUpperCase() }}
        </span>
        <span v-if="row.isSuppressed" class="severity-tag severity-suppressed">
          SUPPRESSED
        </span>
      </div>
    </template>

    <template #cell-resource="{ row }">
      <div class="resource-cell">
        <div class="resource-title-row">
          <span class="kind-tag font-mono">{{ row.resource_kind }}</span>
          <span class="resource-name font-mono">{{ row.resource }}</span>
        </div>
        <span class="resource-sub font-mono text-muted">
          {{ row.namespace || 'default' }} @ {{ row.cluster || 'primary' }}
        </span>
      </div>
    </template>

    <template #cell-driftType="{ row }">
      <span class="drift-type-badge font-mono">
        {{ row.driftType }}
      </span>
    </template>

    <template #cell-diff="{ row }">
      <div class="diff-snippet-cell font-mono" :title="row.diff">
        {{ row.diff ? row.diff.split('\n').slice(0, 2).join(' ') : 'No mutation detected' }}
      </div>
    </template>

    <template #cell-detected_at="{ row }">
      <span class="font-mono text-muted" style="font-size: 11px;">
        {{ formatDate(row.detected_at) }}
      </span>
    </template>

    <template #cell-actions="{ row }">
      <div class="actions-cell">
        <button 
          class="btn btn-secondary btn-sm" 
          title="Inspect cryptographic state diff"
          @click="emit('inspect', row)"
        >
          <span>🔍 Diff</span>
        </button>
        <button 
          v-if="row.status === 'drifted'" 
          class="btn btn-primary btn-sm"
          :disabled="resolvingId === row.id"
          title="Reconcile live cluster etcd with Git repository"
          @click="emit('sync', row.id)"
        >
          <span>{{ resolvingId === row.id ? 'Syncing...' : '⚡ Sync' }}</span>
        </button>
        <button
          class="btn btn-icon btn-sm"
          :title="row.isSuppressed ? 'Resume drift alerts' : 'Suppress drift alert'"
          @click="emit('suppress', row.id)"
        >
          <span>{{ row.isSuppressed ? '🛡️' : '👁️' }}</span>
        </button>
      </div>
    </template>
  </DataTable>
</template>