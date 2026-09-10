<script setup lang="ts">
import type { BackupJob } from '../../api/governance'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  jobs: BackupJob[]
  deletingJobId?: string | null
  downloadingJobId?: string | null
}>()

const emit = defineEmits<{
  (e: 'restore', job: BackupJob): void
  (e: 'download', job: BackupJob): void
  (e: 'delete', jobId: string): void
}>()

function formatBytes(bytes?: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

function formatDateShort(d?: string): string {
  if (!d) return '-'
  try {
    const date = new Date(d)
    return `${date.getMonth() + 1}/${date.getDate()} ${date.getHours()}:${String(date.getMinutes()).padStart(2, '0')}`
  } catch {
    return d
  }
}
</script>

<template>
  <div class="backup-mobile-stream">
    <div 
      v-for="job in jobs" 
      :key="job.id" 
      class="mobile-stream-card"
    >
      <div class="mobile-stream-main">
        <div class="mobile-stream-title-row">
          <span class="font-mono text-cyan" style="font-weight: 700; font-size: 13px;">#{{ job.id.slice(0, 8) }}</span>
          <StatusBadge :status="job.status" :label="job.status.toUpperCase()" size="sm" />
        </div>
        <div class="mobile-stream-sub font-mono">
          <span class="text-emerald">{{ formatBytes(job.compressed_size_bytes || job.size_bytes) }}</span>
          <span>&bull;</span>
          <span class="text-muted">Pol #{{ job.policy_id.slice(0, 6) }}</span>
          <span>&bull;</span>
          <span class="text-muted">{{ formatDateShort(job.started_at || job.created_at) }}</span>
        </div>
      </div>

      <div class="mobile-stream-actions">
        <button 
          class="btn-action-icon"
          :disabled="job.status !== 'completed' && job.status !== 'verified'"
          title="Restore Snapshot"
          aria-label="Restore Snapshot"
          @click="emit('restore', job)"
        >
          <span>🔄</span>
        </button>
        <button 
          class="btn-action-icon"
          :disabled="downloadingJobId === job.id"
          title="Download Snapshot"
          aria-label="Download Snapshot"
          @click="emit('download', job)"
        >
          <span>{{ downloadingJobId === job.id ? '⏳' : '📥' }}</span>
        </button>
        <button 
          class="btn-action-icon btn-action-delete"
          :disabled="deletingJobId === job.id"
          title="Delete Snapshot"
          aria-label="Delete Snapshot"
          @click="emit('delete', job.id)"
        >
          <span>{{ deletingJobId === job.id ? '⏳' : '🗑' }}</span>
        </button>
      </div>
    </div>

    <div v-if="jobs.length === 0" class="empty-state-box glass-panel">
      <span class="empty-icon">📦</span>
      <h3 class="empty-title">No Backup Snapshots</h3>
      <p class="empty-desc">No backup snapshots found in repository.</p>
    </div>
  </div>
</template>
