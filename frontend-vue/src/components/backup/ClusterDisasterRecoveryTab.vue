<template>
  <div class="dr-tab-container">
    <div v-if="feedback" :class="['feedback-banner', `banner-${feedback.type}`]">
      <span>{{ feedback.type === 'success' ? '✅' : '⚠️' }} {{ feedback.message }}</span>
      <button class="btn-close" @click="feedback = null">✕</button>
    </div>

    <!-- 1. etcd Snapshot Management -->
    <div class="dr-card glass-panel">
      <div class="card-header">
        <div>
          <h3 class="card-title">📸 etcd Control Plane Snapshot</h3>
          <p class="card-desc">Capture point-in-time Raft consensus state and cluster metadata.</p>
        </div>
        <button class="btn btn-secondary btn-sm" :disabled="snapshotting || !clusterId" @click="handleSnapshot">
          <span>{{ snapshotting ? '⏳ Capturing...' : '📸 Trigger etcd Snapshot' }}</span>
        </button>
      </div>

      <div v-if="latestSnapshot" class="snapshot-details">
        <div class="detail-item">
          <span class="detail-label">Latest Snapshot</span>
          <span class="detail-val font-mono">{{ latestSnapshot.snapshot_name || latestSnapshot.id }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">Status</span>
          <span :class="['phase-badge', getPhaseClass(latestSnapshot.status || 'Completed')]">{{ latestSnapshot.status || 'Completed' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">Size</span>
          <span class="detail-val font-mono">{{ formatBytes(latestSnapshot.size_bytes) }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">Created Date</span>
          <span class="detail-val font-mono">{{ formatDate(latestSnapshot.created_at) }}</span>
        </div>
        <div class="snapshot-action">
          <button class="btn btn-danger btn-sm" :disabled="restoring || !clusterId" @click="handleRestore">
            <span>{{ restoring ? '⚡ Restoring...' : '⚡ Restore etcd' }}</span>
          </button>
        </div>
      </div>
      <div v-else class="empty-hint">
        <span>No active etcd snapshot recorded for {{ clusterId || 'cluster' }}. Click "Trigger etcd Snapshot" to capture.</span>
      </div>
    </div>

    <!-- 2. Velero Full-Cluster Disaster Recovery -->
    <div class="dr-card glass-panel">
      <div class="card-header">
        <div>
          <h3 class="card-title">🛡️ Velero Full-Cluster DR Backups</h3>
          <p class="card-desc">Full cluster state recovery including custom resources, workloads, volumes, and secrets.</p>
        </div>
        <div class="header-btns">
          <button class="btn btn-secondary btn-sm" :disabled="loading" @click="loadData">
            <span>{{ loading ? '⏳' : '🔄 Refresh' }}</span>
          </button>
          <button class="btn btn-primary btn-sm" :disabled="backingUp || !clusterId" @click="handleClusterDR">
            <span>{{ backingUp ? '🚀 Dispatching...' : '🚀 1-Click Cluster DR' }}</span>
          </button>
        </div>
      </div>

      <div v-if="backups.length > 0" class="table-wrap">
        <table class="dr-table">
          <thead>
            <tr>
              <th>Backup Name</th>
              <th>Phase</th>
              <th>Total Items</th>
              <th>Size</th>
              <th>Created Date</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="b in backups" :key="b.id || b.name">
              <td class="font-mono font-bold">{{ b.name }}</td>
              <td><span :class="['phase-badge', getPhaseClass(b.phase)]">{{ b.phase }}</span></td>
              <td class="font-mono">{{ b.total_items ?? b.items_backed_up ?? '-' }}</td>
              <td class="font-mono">{{ formatBytes(b.size_bytes) }}</td>
              <td class="font-mono text-muted">{{ formatDate(b.created_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else class="empty-hint">
        <span>No Velero cluster backups found for {{ clusterId || 'cluster' }}. Click "1-Click Cluster DR" to create one.</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { drApi, type EtcdSnapshotResult, type ClusterBackupResult } from '../../api/dr'

const props = defineProps<{ clusterId: string }>()
const backups = ref<ClusterBackupResult[]>([])
const latestSnapshot = ref<EtcdSnapshotResult | null>(null)
const loading = ref(false)
const snapshotting = ref(false)
const restoring = ref(false)
const backingUp = ref(false)
const feedback = ref<{ type: 'success' | 'error'; message: string } | null>(null)

function showFeedback(type: 'success' | 'error', message: string) {
  feedback.value = { type, message }
  setTimeout(() => { if (feedback.value?.message === message) feedback.value = null }, 5000)
}

async function loadData() {
  if (!props.clusterId) return
  loading.value = true
  try {
    backups.value = await drApi.listClusterBackups(props.clusterId)
  } catch (err: unknown) {
    showFeedback('error', err instanceof Error ? err.message : 'Failed to load DR backups')
  } finally {
    loading.value = false
  }
}

async function handleSnapshot() {
  if (!props.clusterId) return
  snapshotting.value = true
  try {
    const res = await drApi.triggerEtcdSnapshot(props.clusterId)
    latestSnapshot.value = res
    showFeedback('success', `etcd snapshot "${res.snapshot_name || res.id}" captured successfully.`)
  } catch (err: unknown) {
    showFeedback('error', err instanceof Error ? err.message : 'Failed to trigger etcd snapshot')
  } finally {
    snapshotting.value = false
  }
}

async function handleRestore() {
  if (!props.clusterId || !latestSnapshot.value) return
  const sid = latestSnapshot.value.id || latestSnapshot.value.snapshot_name || 'latest'
  if (!window.confirm(`Restore etcd snapshot "${sid}" on cluster "${props.clusterId}"?`)) return
  restoring.value = true
  try {
    await drApi.restoreEtcdSnapshot(props.clusterId, sid)
    showFeedback('success', `etcd restore triggered for snapshot "${sid}" on cluster ${props.clusterId}.`)
  } catch (err: unknown) {
    showFeedback('error', err instanceof Error ? err.message : 'Failed to restore etcd snapshot')
  } finally {
    restoring.value = false
  }
}

async function handleClusterDR() {
  if (!props.clusterId) return
  backingUp.value = true
  try {
    const res = await drApi.createClusterBackup(props.clusterId)
    showFeedback('success', `Velero cluster backup "${res.name}" initiated.`)
    await loadData()
  } catch (err: unknown) {
    showFeedback('error', err instanceof Error ? err.message : 'Failed to create cluster DR backup')
  } finally {
    backingUp.value = false
  }
}

function getPhaseClass(phase?: string): string {
  const p = (phase || '').toLowerCase()
  if (p === 'completed' || p === 'verified') return 'phase-completed'
  if (p === 'inprogress' || p === 'running') return 'phase-running'
  return 'phase-failed'
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`
}

function formatDate(d?: string): string {
  if (!d) return '-'
  try { return new Date(d).toLocaleString() } catch { return d }
}

watch(() => props.clusterId, (cid) => { if (cid) loadData() })
onMounted(() => { if (props.clusterId) loadData() })
</script>

<style scoped>
.dr-tab-container { display: flex; flex-direction: column; gap: 20px; }
.dr-card { padding: 20px; border-radius: 12px; display: flex; flex-direction: column; gap: 16px; }
.card-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; flex-wrap: wrap; }
.card-title { font-size: 16px; font-weight: 700; color: #fff; margin: 0; }
.card-desc { font-size: 13px; color: var(--text-secondary); margin: 4px 0 0; }
.header-btns { display: flex; gap: 8px; }
.feedback-banner { display: flex; justify-content: space-between; align-items: center; padding: 10px 16px; border-radius: 8px; font-size: 13px; }
.banner-success { background: rgba(16, 185, 129, 0.15); border: 1px solid rgba(16, 185, 129, 0.3); color: #34d399; }
.banner-error { background: rgba(244, 63, 94, 0.15); border: 1px solid rgba(244, 63, 94, 0.3); color: #fda4af; }
.btn-close { background: none; border: none; color: inherit; cursor: pointer; }
.snapshot-details { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 12px; background: rgba(0,0,0,0.25); padding: 14px; border-radius: 8px; align-items: center; }
.detail-item { display: flex; flex-direction: column; gap: 4px; }
.detail-label { font-size: 11px; text-transform: uppercase; color: var(--text-secondary); letter-spacing: 0.05em; }
.detail-val { font-size: 13px; color: #fff; }
.phase-badge { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: 11px; font-weight: 700; text-transform: uppercase; }
.phase-completed { background: rgba(16, 185, 129, 0.2); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.phase-running { background: rgba(6, 182, 212, 0.2); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.3); }
.phase-failed { background: rgba(244, 63, 94, 0.2); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }
.btn-danger { background: rgba(239, 68, 68, 0.2); border: 1px solid rgba(239, 68, 68, 0.4); color: #fca5a5; border-radius: 6px; padding: 6px 12px; cursor: pointer; }
.btn-danger:hover:not(:disabled) { background: rgba(239, 68, 68, 0.35); }
.table-wrap { overflow-x: auto; }
.dr-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.dr-table th { text-align: left; padding: 10px; border-bottom: 1px solid rgba(255,255,255,0.1); color: var(--text-secondary); font-size: 12px; text-transform: uppercase; }
.dr-table td { padding: 10px; border-bottom: 1px solid rgba(255,255,255,0.05); color: #e2e8f0; }
.empty-hint { padding: 24px; text-align: center; color: var(--text-secondary); font-size: 13px; }
.text-muted { color: var(--text-secondary); }
</style>
