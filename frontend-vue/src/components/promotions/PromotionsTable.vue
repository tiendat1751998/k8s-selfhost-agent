<script setup lang="ts">
import type { Promotion } from '../../api/compute'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  promotions: Promotion[]
  loading: boolean
  error: string | null
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'approve', promotion: Promotion): void
  (e: 'reject', promotion: Promotion): void
  (e: 'complete', promotion: Promotion): void
  (e: 'diff', promotion: Promotion): void
  (e: 'rollback', promotion: Promotion): void
  (e: 'abort', promotion: Promotion): void
}>()

const columns: Column<Promotion>[] = [
  { key: 'service', label: 'Service Name', sortable: true },
  { key: 'version', label: 'Target Version', width: '130px', sortable: true },
  { key: 'from_env', label: 'Source', width: '100px', sortable: true },
  { key: 'to_env', label: 'Destination', width: '110px', sortable: true },
  { key: 'requester', label: 'Requested By', width: '140px', sortable: true },
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'created_at', label: 'Requested At', width: '150px', sortable: true },
  { key: 'actions', label: 'Gating & Rollout Actions', width: '330px', align: 'right' }
]

function formatDate(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <div class="section-box glass-panel table-box">
    <div class="box-header" style="padding: 18px 22px; border-bottom: 1px solid var(--border-subtle);">
      <div>
        <h2 class="box-title">Promotion Request Audit Table</h2>
        <p class="box-subtitle">Full historical ledger of promotion approvals and audit trail</p>
      </div>
    </div>

    <DataTable
      :columns="columns"
      :data="promotions"
      :loading="loading"
      :error="error"
      empty-message="No promotion requests found. Click '+ Request Promotion' to submit a new release for environment gating."
      searchable
      search-placeholder="Search promotions by service, requester, or version..."
    >
      <template #cell-service="{ row }">
        <span class="font-mono text-cyan font-bold">{{ row.service }}</span>
      </template>

      <template #cell-version="{ row }">
        <span class="font-mono text-emerald">{{ row.version }}</span>
      </template>

      <template #cell-from_env="{ row }">
        <span class="env-pill-tag font-mono">{{ row.from_env }}</span>
      </template>

      <template #cell-to_env="{ row }">
        <span class="env-pill-tag font-mono text-cyan">{{ row.to_env }}</span>
      </template>

      <template #cell-status="{ row }">
        <StatusBadge :status="row.status" size="sm" />
      </template>

      <template #cell-created_at="{ row }">
        <span class="font-mono text-muted">{{ formatDate(row.created_at) }}</span>
      </template>

      <template #cell-actions="{ row }">
        <div class="table-actions-row">
          <button
            class="btn btn-secondary btn-xs btn-diff"
            title="Inspect Git Manifest Diff"
            @click="emit('diff', row)"
          >
            <span>[ 🔍 Diff ]</span>
          </button>

          <button
            v-if="row.status === 'pending'"
            class="btn btn-primary btn-xs btn-promote"
            :disabled="actionLoading === row.id"
            title="Approve & Promote Release"
            @click="emit('approve', row)"
          >
            <span>[ 🚀 Promote ]</span>
          </button>

          <button
            v-else-if="row.status === 'approved' || row.status === 'promoting'"
            class="btn btn-primary btn-xs btn-promote"
            :disabled="actionLoading === row.id"
            title="Complete Rollout"
            @click="emit('complete', row)"
          >
            <span>[ 🚀 Promote ]</span>
          </button>

          <button
            v-if="row.status === 'completed' || row.status === 'promoting' || row.status === 'failed'"
            class="btn btn-secondary btn-xs btn-rollback"
            :disabled="actionLoading === row.id"
            title="Rollback to previous revision"
            @click="emit('rollback', row)"
          >
            <span>[ ⏪ Rollback ]</span>
          </button>

          <button
            v-if="row.status === 'pending' || row.status === 'promoting' || row.status === 'approved'"
            class="btn btn-xs btn-abort"
            :disabled="actionLoading === row.id"
            title="Abort this promotion pipeline"
            @click="emit('abort', row)"
          >
            <span>[ 🗑 Abort ]</span>
          </button>
        </div>
      </template>
    </DataTable>
  </div>
</template>
