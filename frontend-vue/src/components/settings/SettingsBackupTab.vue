<script setup lang="ts">
import { ref, onMounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import {
  backupApi,
  type BackupStorage,
  type BackupPolicy,
  type BackupJob,
} from '../../api/backup'

defineProps<{
  saving?: boolean
}>()

const loading = ref(false)
const error = ref<string | null>(null)
const successMessage = ref<string | null>(null)
const triggeringPolicyId = ref<string | null>(null)

const storages = ref<BackupStorage[]>([])
const policies = ref<BackupPolicy[]>([])
const jobs = ref<BackupJob[]>([])

async function loadBackupData() {
  loading.value = true
  error.value = null
  try {
    const [storagesData, policiesData, jobsData] = await Promise.all([
      backupApi.getStorages(),
      backupApi.getPolicies(),
      backupApi.getJobs(),
    ])
    storages.value = storagesData
    policies.value = policiesData
    jobs.value = jobsData
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : 'Failed to retrieve backup and disaster recovery telemetry.'
  } finally {
    loading.value = false
  }
}

async function handleTriggerBackup(policyId: string) {
  if (triggeringPolicyId.value) return
  triggeringPolicyId.value = policyId
  error.value = null
  successMessage.value = null
  try {
    const newJob = await backupApi.triggerBackup(policyId, 'full')
    jobs.value.unshift(newJob)
    successMessage.value = `Immediate backup job dispatched successfully (${newJob.id}).`
    setTimeout(() => { successMessage.value = null }, 5000)
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : 'Failed to trigger immediate backup snapshot.'
  } finally {
    triggeringPolicyId.value = null
  }
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '—'
  try {
    return new Date(dateStr).toLocaleString()
  } catch {
    return dateStr
  }
}

function getJobStatusBadge(status: string): string {
  switch (status) {
    case 'completed': return 'badge-emerald'
    case 'running': return 'badge-cyan'
    case 'failed': return 'badge-rose'
    default: return 'badge-amber'
  }
}

onMounted(() => {
  loadBackupData()
})
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in backup-tab-container">
    <div class="card-header">
      <div>
        <h2 class="card-title">Disaster Recovery & Cluster Backups</h2>
        <p class="card-subtitle">
          Manage snapshot schedules, point-in-time recovery targets, and immutable storage buckets.
        </p>
      </div>
      <div class="card-header-actions">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="loading"
          @click="loadBackupData"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
          <span>{{ loading ? 'Syncing...' : 'Sync Telemetry' }}</span>
        </button>
      </div>
    </div>

    <!-- Alert / Status Banners -->
    <div v-if="error" class="banner-box banner-error" role="alert">
      <BaseIcon name="alert-triangle" size="xs" />
      <span class="banner-text">{{ error }}</span>
    </div>
    <div v-if="successMessage" class="banner-box banner-success" role="status">
      <BaseIcon name="check-circle" size="xs" />
      <span class="banner-text">{{ successMessage }}</span>
    </div>

    <!-- Loading State -->
    <div v-if="loading && policies.length === 0" class="loading-state">
      <div class="spinner"></div>
      <span>Loading backup targets and retention policies...</span>
    </div>

    <div v-else class="backup-tab-content">
      <!-- Section 1: Backup Policies -->
      <section class="backup-section" aria-labelledby="heading-policies">
        <div class="section-title-wrap">
          <h3 id="heading-policies" class="subsection-title">
            <BaseIcon name="archive" size="sm" />
            <span>Active Backup Policies ({{ policies.length }})</span>
          </h3>
        </div>

        <div v-if="policies.length === 0" class="empty-state-card">
          <p class="text-muted">No backup policies configured yet. Policies define automated database and cluster snapshots.</p>
        </div>

        <div v-else class="policies-grid">
          <div
            v-for="policy in policies"
            :key="policy.id"
            class="policy-card glass-panel"
          >
            <div class="policy-card-top">
              <div>
                <span class="policy-name font-bold">{{ policy.name }}</span>
                <span class="policy-db font-mono text-cyan">{{ policy.db_type }} / {{ policy.db_name }}</span>
              </div>
              <span class="badge" :class="policy.enabled ? 'badge-emerald' : 'badge-slate'">
                {{ policy.enabled ? 'ENABLED' : 'PAUSED' }}
              </span>
            </div>

            <div class="policy-details font-mono text-xs text-muted">
              <div><strong>Schedule:</strong> {{ policy.schedule }}</div>
              <div><strong>Retention:</strong> {{ policy.retention_count }} snapshots</div>
              <div><strong>Host:</strong> {{ policy.db_host }}:{{ policy.db_port }}</div>
            </div>

            <div class="policy-actions">
              <button
                type="button"
                class="btn btn-primary btn-xs"
                :disabled="triggeringPolicyId === policy.id"
                @click="handleTriggerBackup(policy.id)"
              >
                <BaseIcon :name="triggeringPolicyId === policy.id ? 'refresh' : 'zap'" size="xs" :class="{ 'animate-spin': triggeringPolicyId === policy.id }" />
                <span>{{ triggeringPolicyId === policy.id ? 'Snapshotting...' : 'Trigger Backup' }}</span>
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- Section 2: Storage Repositories -->
      <section class="backup-section" aria-labelledby="heading-storage">
        <div class="section-title-wrap">
          <h3 id="heading-storage" class="subsection-title">
            <BaseIcon name="hard-drive" size="sm" />
            <span>Storage Targets ({{ storages.length }})</span>
          </h3>
        </div>

        <div v-if="storages.length === 0" class="empty-state-card">
          <p class="text-muted">No remote storage targets registered. Defaulting to local node ephemeral volume.</p>
        </div>

        <div v-else class="storages-list">
          <div
            v-for="st in storages"
            :key="st.id"
            class="storage-item glass-panel"
          >
            <div class="storage-info">
              <span class="storage-name font-bold">{{ st.name }}</span>
              <span class="storage-type font-mono text-xs uppercase text-amber">[{{ st.type }}]</span>
              <span class="storage-bucket font-mono text-xs text-muted">{{ st.bucket }} ({{ st.endpoint }})</span>
            </div>
            <span class="text-xs text-muted font-mono">Added: {{ formatDate(st.created_at) }}</span>
          </div>
        </div>
      </section>

      <!-- Section 3: Recent Backup Jobs -->
      <section class="backup-section" aria-labelledby="heading-jobs">
        <div class="section-title-wrap">
          <h3 id="heading-jobs" class="subsection-title">
            <BaseIcon name="clock" size="sm" />
            <span>Recent Backup Executions ({{ jobs.length }})</span>
          </h3>
        </div>

        <div v-if="jobs.length === 0" class="empty-state-card">
          <p class="text-muted">No recent backup jobs recorded.</p>
        </div>

        <div v-else class="jobs-table-wrap">
          <table class="jobs-table font-mono text-xs">
            <thead>
              <tr>
                <th>Job ID</th>
                <th>Type</th>
                <th>Status</th>
                <th>Raw Size</th>
                <th>Compressed</th>
                <th>Started</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="job in jobs.slice(0, 8)" :key="job.id">
                <td class="text-cyan font-bold">{{ job.id.slice(0, 8) }}...</td>
                <td class="uppercase">{{ job.backup_type }}</td>
                <td>
                  <span class="badge" :class="getJobStatusBadge(job.status)">
                    {{ job.status }}
                  </span>
                </td>
                <td>{{ formatBytes(job.bytes_raw) }}</td>
                <td>{{ formatBytes(job.bytes_compressed) }}</td>
                <td class="text-muted">{{ formatDate(job.started_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/settings-backup.css';
</style>
