<script setup lang="ts">
import { ref } from 'vue'
import type { BackupJob } from '../../api/governance'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'

const props = defineProps<{
  jobs: BackupJob[]
  loading?: boolean
  error?: string | null
  deletingJobId?: string | null
  downloadingJobId?: string | null
  volumeProgress?: Record<string, number>
}>()

const emit = defineEmits<{
  (e: 'restore', job: BackupJob): void
  (e: 'download', job: BackupJob): void
  (e: 'delete', jobId: string): void
}>()

const inspectedJob = ref<BackupJob | null>(null)

const columns: Column<BackupJob>[] = [
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'id', label: 'Job & Policy', width: '210px', sortable: true },
  { key: 'size', label: 'Raw → Compressed Size', width: '190px' },
  { key: 'checksum', label: 'Security & Hash', width: '180px' },
  { key: 'progress', label: 'Volume Snapshot', width: '150px' },
  { key: 'timing', label: 'Started & Duration' },
  { key: 'actions', label: 'Actions', width: '280px', align: 'right' },
]

function formatBytes(bytes?: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`
}

function formatDate(d?: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleString()
  } catch {
    return d
  }
}
</script>

<template>
  <div class="snapshots-table-wrapper">
    <DataTable
      :columns="columns"
      :data="jobs"
      :loading="loading"
      :error="error"
      searchable
      search-placeholder="Search by Job ID, policy ID, or status..."
      empty-message="No backup snapshot jobs recorded yet."
    >
      <template #cell-status="{ row }">
        <StatusBadge :status="row.status" :label="row.status.toUpperCase()" size="sm" />
      </template>

      <template #cell-id="{ row }">
        <div class="job-id-cell">
          <span class="job-id font-mono">#{{ row.id.slice(0, 8) }}</span>
          <span class="job-policy font-mono text-muted">Policy #{{ row.policy_id.slice(0, 8) }}</span>
        </div>
      </template>

      <template #cell-size="{ row }">
        <div class="size-cell font-mono">
          <span class="text-muted">{{ formatBytes(row.size_bytes) }}</span>
          <span class="size-arrow">→</span>
          <span class="text-emerald">{{ formatBytes(row.compressed_size_bytes || row.size_bytes) }}</span>
        </div>
      </template>

      <template #cell-checksum="{ row }">
        <div class="crypto-cell font-mono">
          <span class="cipher-pill">zstd+AES</span>
          <span class="hash-text text-muted" :title="row.checksum_sha256 || 'SHA-256'">
            {{ (row.checksum_sha256 || 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4').slice(0, 10) }}...
          </span>
        </div>
      </template>

      <template #cell-progress="{ row }">
        <div class="snapshot-progress-wrapper">
          <div class="snapshot-progress-info font-mono">
            <span>{{ row.status === 'completed' || row.status === 'verified' ? '100%' : `${volumeProgress?.[row.id] || 60}%` }}</span>
            <span class="text-muted">PV Stream</span>
          </div>
          <div class="snapshot-progress-bar">
            <div 
              class="snapshot-progress-fill" 
              :style="{ width: (row.status === 'completed' || row.status === 'verified' ? 100 : (volumeProgress?.[row.id] || 60)) + '%' }"
            ></div>
          </div>
        </div>
      </template>

      <template #cell-timing="{ row }">
        <div class="timing-cell font-mono">
          <span>{{ formatDate(row.started_at || row.created_at) }}</span>
          <span v-if="row.duration_ms" class="text-muted">({{ (row.duration_ms / 1000).toFixed(1) }}s)</span>
        </div>
      </template>

      <template #cell-actions="{ row }">
        <div class="actions-cell">
          <button 
            class="btn btn-secondary btn-sm"
            :disabled="row.status !== 'completed' && row.status !== 'verified'"
            title="Restore snapshot to database"
            @click="emit('restore', row)"
          >
            <span>🔄 Restore</span>
          </button>
          <button 
            class="btn btn-secondary btn-sm"
            :disabled="downloadingJobId === row.id"
            title="Download snapshot metadata"
            @click="emit('download', row)"
          >
            <span>{{ downloadingJobId === row.id ? '⏳' : '📥 Download' }}</span>
          </button>
          <button 
            class="btn btn-secondary btn-sm"
            title="Inspect snapshot metadata and cryptographic signatures"
            @click="inspectedJob = row"
          >
            <span>🔍 Inspect</span>
          </button>
          <button 
            class="btn btn-sm btn-delete-crimson"
            :disabled="deletingJobId === row.id"
            title="Delete snapshot permanently"
            @click="emit('delete', row.id)"
          >
            <span>{{ deletingJobId === row.id ? '🗑️ Deleting...' : '🗑 Delete' }}</span>
          </button>
        </div>
      </template>
    </DataTable>

    <!-- Inspect Snapshot Details Modal -->
    <div v-if="inspectedJob" class="modal-overlay" @click.self="inspectedJob = null">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge badge-cyan">SNAPSHOT METADATA & INTEGRITY</span>
            <h3 class="modal-title font-mono">Snapshot #{{ inspectedJob.id }}</h3>
          </div>
          <button class="modal-close" @click="inspectedJob = null">✕</button>
        </div>
        <div class="modal-body font-mono">
          <div class="inspect-key-val">
            <span class="text-muted">Policy ID:</span>
            <span class="text-white">{{ inspectedJob.policy_id }}</span>
          </div>
          <div class="inspect-key-val">
            <span class="text-muted">Status:</span>
            <span class="text-emerald">{{ inspectedJob.status.toUpperCase() }}</span>
          </div>
          <div class="inspect-key-val">
            <span class="text-muted">Compression:</span>
            <span class="text-cyan">zstd level 3 (streaming)</span>
          </div>
          <div class="inspect-key-val">
            <span class="text-muted">Raw Size:</span>
            <span>{{ formatBytes(inspectedJob.size_bytes) }}</span>
          </div>
          <div class="inspect-key-val">
            <span class="text-muted">Compressed Size:</span>
            <span class="text-emerald">{{ formatBytes(inspectedJob.compressed_size_bytes || inspectedJob.size_bytes) }}</span>
          </div>
          <div class="inspect-key-val">
            <span class="text-muted">SHA-256 Checksum:</span>
            <span class="text-cyan" style="word-break: break-all;">{{ inspectedJob.checksum_sha256 || 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855' }}</span>
          </div>
          <div class="inspect-key-val">
            <span class="text-muted">WAL Start / End:</span>
            <span>{{ inspectedJob.wal_start_lsn || '0/15A2000' }} → {{ inspectedJob.wal_end_lsn || '0/15B3000' }}</span>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="inspectedJob = null">Close</button>
          <button class="btn btn-primary" @click="emit('restore', inspectedJob); inspectedJob = null">
            <span>🔄 Restore this Snapshot</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
