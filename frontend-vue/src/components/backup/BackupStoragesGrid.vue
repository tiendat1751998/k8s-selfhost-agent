<script setup lang="ts">
import type { BackupStorage } from '../../api/governance'

defineProps<{
  storages: BackupStorage[]
}>()

const emit = defineEmits<{
  (e: 'create'): void
}>()

function getStorageIcon(type: string): string {
  const t = (type || '').toLowerCase()
  if (t.includes('s3') || t.includes('minio')) return '☁️'
  if (t.includes('local')) return '💾'
  if (t.includes('nfs')) return '🌐'
  return '📁'
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
  <div class="storages-grid-wrapper">
    <div v-if="storages.length > 0" class="storages-grid">
      <div v-for="storage in storages" :key="storage.id" class="storage-card glass-panel">
        <div class="storage-top">
          <div class="storage-icon-box">{{ getStorageIcon(storage.type) }}</div>
          <div class="storage-meta">
            <h3 class="storage-name">{{ storage.name }}</h3>
            <span class="storage-type font-mono text-cyan">{{ storage.type.toUpperCase() }} STORAGE</span>
          </div>
          <span class="badge badge-emerald">ATTACHED</span>
        </div>

        <div class="storage-body font-mono">
          <div class="storage-row">
            <span class="storage-k">Endpoint:</span>
            <span class="storage-v">{{ storage.endpoint || 'Local NVMe Path' }}</span>
          </div>
          <div class="storage-row">
            <span class="storage-k">Bucket / Dir:</span>
            <span class="storage-v text-emerald">{{ storage.bucket || 'k8s-backups' }}</span>
          </div>
          <div class="storage-row">
            <span class="storage-k">Created:</span>
            <span class="storage-v text-muted">{{ formatDate(storage.created_at || '') }}</span>
          </div>
        </div>
      </div>
    </div>

    <div v-else class="empty-state-box glass-panel">
      <span class="empty-icon">💾</span>
      <h3 class="empty-title">No Storage Targets Attached</h3>
      <p class="empty-desc">Configure S3, MinIO, or local NVMe storage targets to store compressed snapshots.</p>
      <button class="btn btn-primary" @click="emit('create')">
        <span>+ Attach Storage Target</span>
      </button>
    </div>
  </div>
</template>
