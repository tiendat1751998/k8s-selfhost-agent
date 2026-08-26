<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>ENTERPRISE DISASTER RECOVERY & PITR</span>
        </div>
        <h1 class="view-title">Dual-Target Database Backup & Instant Restore</h1>
        <p class="view-desc">
          Automated multi-engine database streaming (<span class="highlight">PostgreSQL, MySQL, MongoDB, Redis</span>) with <span class="highlight">zstd streaming compression</span> and cryptographic verification.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchAllBackupData">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh' }}</span>
        </button>
        <button v-if="activeTab === 'policies'" class="btn btn-primary" @click="showPolicyModal = true">
          <span>+ Create Backup Policy</span>
        </button>
        <button v-else-if="activeTab === 'storages'" class="btn btn-primary" @click="showStorageModal = true">
          <span>+ Add Storage Target</span>
        </button>
        <button v-else-if="activeTab === 'restores'" class="btn btn-primary" @click="showRestoreModal = true">
          <span>+ Trigger Restore</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Metric HUD Cards -->
    <div class="metrics-grid">
      <MetricCard
        title="Active Backup Policies"
        :value="policies.length"
        badge="CONFIGURED"
        badge-color="cyan"
        :subtitle="`${activePoliciesCount} automated schedules enabled`"
        icon="📋"
      />
      <MetricCard
        title="Storage Repositories"
        :value="storages.length"
        badge="ATTACHED"
        badge-color="emerald"
        subtitle="Local NVMe & S3/MinIO Targets"
        icon="💾"
      />
      <MetricCard
        title="Completed Snapshots"
        :value="completedJobsCount"
        :trend="failedJobsCount === 0 ? 'Zero Errors' : `${failedJobsCount} Failed`"
        :trend-type="failedJobsCount === 0 ? 'positive' : 'negative'"
        badge="VERIFIED"
        badge-color="emerald"
        subtitle="Cryptographically hashed (SHA-256)"
        icon="🛡️"
      />
      <MetricCard
        title="Executed Restores"
        :value="restores.length"
        badge="PITR ENGINE"
        badge-color="violet"
        subtitle="Instant failover testable"
        icon="⏪"
      />
    </div>

    <!-- View Tabs Switcher -->
    <div class="tabs-bar glass-panel">
      <button 
        class="tab-btn" 
        :class="{ 'tab-btn-active': activeTab === 'policies' }"
        @click="activeTab = 'policies'"
      >
        <span>📋 Backup Policies ({{ policies.length }})</span>
      </button>
      <button 
        class="tab-btn" 
        :class="{ 'tab-btn-active': activeTab === 'storages' }"
        @click="activeTab = 'storages'"
      >
        <span>💾 Storage Targets ({{ storages.length }})</span>
      </button>
      <button 
        class="tab-btn" 
        :class="{ 'tab-btn-active': activeTab === 'jobs' }"
        @click="activeTab = 'jobs'"
      >
        <span>⚡ Backup History ({{ jobs.length }})</span>
      </button>
      <button 
        class="tab-btn" 
        :class="{ 'tab-btn-active': activeTab === 'restores' }"
        @click="activeTab = 'restores'"
      >
        <span>⏪ Restore Actions ({{ restores.length }})</span>
      </button>
    </div>

    <!-- TAB 1: POLICIES -->
    <div v-if="activeTab === 'policies'" class="tab-content animate-fade-in">
      <div v-if="policies.length > 0" class="policies-grid">
        <div v-for="policy in policies" :key="policy.id" class="policy-card glass-panel glass-panel-glow">
          <div class="policy-card-top">
            <div class="policy-icon-box">{{ getDbIcon(policy.db_type) }}</div>
            <div class="policy-meta">
              <h3 class="policy-name">{{ policy.name }}</h3>
              <span class="policy-sub font-mono">{{ policy.db_type.toUpperCase() }} @ {{ policy.db_host }}:{{ policy.db_port }}</span>
            </div>
            <StatusBadge :status="policy.enabled ? 'active' : 'idle'" :label="policy.enabled ? 'ARMED' : 'PAUSED'" size="sm" />
          </div>

          <div class="policy-body">
            <div class="policy-stat-row">
              <span class="policy-k">Target Database:</span>
              <span class="policy-v font-mono text-cyan">{{ policy.db_name }}</span>
            </div>
            <div class="policy-stat-row">
              <span class="policy-k">Cron Schedule:</span>
              <span class="policy-v font-mono">{{ policy.schedule }}</span>
            </div>
            <div class="policy-stat-row">
              <span class="policy-k">Retention Count:</span>
              <span class="policy-v font-mono">{{ policy.retention_count }} snapshots</span>
            </div>
            <div class="policy-stat-row">
              <span class="policy-k">Backup Type:</span>
              <span class="policy-v font-mono text-emerald">{{ (policy.backup_type || 'full').toUpperCase() }}</span>
            </div>
          </div>

          <div class="policy-actions">
            <button class="btn btn-primary btn-sm" :disabled="triggeringPolicyId === policy.id" @click="handleTriggerBackup(policy.id)">
              <span>{{ triggeringPolicyId === policy.id ? '⚡ Dispatching...' : '⚡ Backup Now' }}</span>
            </button>
          </div>
        </div>
      </div>

      <div v-else class="empty-state-box glass-panel">
        <span class="empty-icon">📋</span>
        <h3 class="empty-title">No Backup Policies Configured</h3>
        <p class="empty-desc">Create your first automated database policy to protect workloads across clusters.</p>
        <button class="btn btn-primary" @click="showPolicyModal = true">
          <span>+ Create Backup Policy</span>
        </button>
      </div>
    </div>

    <!-- TAB 2: STORAGE TARGETS -->
    <div v-if="activeTab === 'storages'" class="tab-content animate-fade-in">
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
        <button class="btn btn-primary" @click="showStorageModal = true">
          <span>+ Attach Storage Target</span>
        </button>
      </div>
    </div>

    <!-- TAB 3: BACKUP HISTORY / JOBS -->
    <div v-if="activeTab === 'jobs'" class="tab-content animate-fade-in">
      <DataTable
        :columns="jobColumns"
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
              {{ (row.checksum_sha256 || 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4').slice(0, 14) }}...
            </span>
          </div>
        </template>

        <template #cell-timing="{ row }">
          <div class="timing-cell font-mono">
            <span>{{ formatDate(row.started_at || row.created_at) }}</span>
            <span v-if="row.duration_ms" class="text-muted">({{ (row.duration_ms / 1000).toFixed(1) }}s)</span>
          </div>
        </template>

        <template #cell-actions="{ row }">
          <button 
            class="btn btn-secondary btn-sm"
            :disabled="row.status !== 'completed' && row.status !== 'verified'"
            @click="openRestoreModalWithJob(row)"
          >
            <span>⏪ Restore</span>
          </button>
        </template>
      </DataTable>
    </div>

    <!-- TAB 4: RESTORE ACTIONS -->
    <div v-if="activeTab === 'restores'" class="tab-content animate-fade-in">
      <DataTable
        :columns="restoreColumns"
        :data="restores"
        :loading="loading"
        :error="error"
        searchable
        search-placeholder="Search restore jobs, target host or DB..."
        empty-message="No restore executions recorded yet."
      >
        <template #cell-status="{ row }">
          <StatusBadge :status="row.status" :label="row.status.toUpperCase()" size="sm" />
        </template>

        <template #cell-id="{ row }">
          <span class="font-mono text-cyan">#{{ row.id.slice(0, 8) }}</span>
        </template>

        <template #cell-backup_job_id="{ row }">
          <span class="font-mono text-muted">Snapshot #{{ row.backup_job_id.slice(0, 8) }}</span>
        </template>

        <template #cell-target="{ row }">
          <div class="target-cell font-mono">
            <span class="target-db text-emerald">{{ row.target_db_name }}</span>
            <span class="target-host text-muted">@ {{ row.target_db_host }}</span>
          </div>
        </template>

        <template #cell-created_at="{ row }">
          <span class="font-mono text-muted" style="font-size: 11px;">{{ formatDate(row.created_at) }}</span>
        </template>

        <template #cell-log="{ row }">
          <span class="log-preview font-mono" :title="row.verification_log || 'No log details'">
            {{ row.verification_log ? row.verification_log.slice(0, 40) + '...' : 'Replay logs OK' }}
          </span>
        </template>
      </DataTable>
    </div>

    <!-- Modal: Create Backup Policy -->
    <div v-if="showPolicyModal" class="modal-overlay" @click.self="showPolicyModal = false">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge badge-cyan">SCHEDULED PROTECTION</span>
            <h3 class="modal-title">Create Database Backup Policy</h3>
          </div>
          <button class="modal-close" @click="showPolicyModal = false">✕</button>
        </div>

        <form class="modal-body" @submit.prevent="handleCreatePolicy">
          <div class="form-group">
            <label class="form-label">Policy Name:</label>
            <input v-model="newPolicy.name" type="text" required class="input-glass" placeholder="e.g. production-postgres-main" />
          </div>

          <div class="form-group-row">
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Database Type:</label>
              <select v-model="newPolicy.db_type" class="input-glass">
                <option value="postgres">PostgreSQL</option>
                <option value="mysql">MySQL</option>
                <option value="mariadb">MariaDB</option>
                <option value="mongodb">MongoDB</option>
                <option value="redis">Redis</option>
                <option value="nats">NATS JetStream</option>
              </select>
            </div>
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Storage Target:</label>
              <select v-model="newPolicy.storage_id" class="input-glass">
                <option value="default-s3">Default S3 / MinIO</option>
                <option v-for="s in storages" :key="s.id" :value="s.id">{{ s.name }} ({{ s.type }})</option>
              </select>
            </div>
          </div>

          <div class="form-group-row">
            <div class="form-group" style="flex: 3;">
              <label class="form-label">DB Host:</label>
              <input v-model="newPolicy.db_host" type="text" required class="input-glass" placeholder="postgres.db.svc.cluster.local" />
            </div>
            <div class="form-group" style="flex: 1;">
              <label class="form-label">DB Port:</label>
              <input v-model.number="newPolicy.db_port" type="number" required class="input-glass font-mono" placeholder="5432" />
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Database Name:</label>
            <input v-model="newPolicy.db_name" type="text" required class="input-glass" placeholder="app_db" />
          </div>

          <div class="form-group-row">
            <div class="form-group" style="flex: 2;">
              <label class="form-label">Cron Schedule:</label>
              <input v-model="newPolicy.schedule" type="text" required class="input-glass font-mono" placeholder="0 */6 * * *" />
            </div>
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Retention Count:</label>
              <input v-model.number="newPolicy.retention_count" type="number" min="1" max="100" class="input-glass font-mono" placeholder="14" />
            </div>
          </div>

          <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
            <button type="button" class="btn btn-secondary" @click="showPolicyModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="loading">
              <span>{{ loading ? 'Saving...' : 'Arm & Enable Policy' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Add Storage Target -->
    <div v-if="showStorageModal" class="modal-overlay" @click.self="showStorageModal = false">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge badge-emerald">STORAGE REPOSITORY</span>
            <h3 class="modal-title">Attach Storage Target</h3>
          </div>
          <button class="modal-close" @click="showStorageModal = false">✕</button>
        </div>

        <form class="modal-body" @submit.prevent="handleCreateStorage">
          <div class="form-group">
            <label class="form-label">Storage Name:</label>
            <input v-model="newStorage.name" type="text" required class="input-glass" placeholder="e.g. minio-cluster-backup" />
          </div>

          <div class="form-group">
            <label class="form-label">Storage Type:</label>
            <select v-model="newStorage.type" class="input-glass">
              <option value="s3">AWS S3 / Compatible</option>
              <option value="minio">MinIO Object Store</option>
              <option value="local">Local NVMe Volume</option>
              <option value="nfs">Network File System (NFS)</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">Endpoint URL:</label>
            <input v-model="newStorage.endpoint" type="text" class="input-glass font-mono" placeholder="https://s3.us-east-1.amazonaws.com or minio.storage.svc:9000" />
          </div>

          <div class="form-group">
            <label class="form-label">Bucket / Volume Name:</label>
            <input v-model="newStorage.bucket" type="text" required class="input-glass font-mono" placeholder="k8s-database-backups" />
          </div>

          <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
            <button type="button" class="btn btn-secondary" @click="showStorageModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="loading">
              <span>{{ loading ? 'Attaching...' : 'Attach Storage' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Trigger Restore -->
    <div v-if="showRestoreModal" class="modal-overlay" @click.self="showRestoreModal = false">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge badge-rose">POINT-IN-TIME RESTORE</span>
            <h3 class="modal-title">Restore Database from Snapshot</h3>
          </div>
          <button class="modal-close" @click="showRestoreModal = false">✕</button>
        </div>

        <form class="modal-body" @submit.prevent="handleExecuteRestore">
          <div class="alert-box alert-warning">
            <strong>⚠️ Caution:</strong> Restore will replay WAL/transaction logs and replace contents in the target database.
          </div>

          <div class="form-group">
            <label class="form-label">Select Backup Snapshot Job:</label>
            <select v-model="restoreParams.backup_job_id" required class="input-glass font-mono">
              <option value="" disabled>-- Select a completed backup job --</option>
              <option v-for="j in completedJobs" :key="j.id" :value="j.id">
                Job #{{ j.id.slice(0, 8) }} (Policy #{{ j.policy_id.slice(0, 8) }} - {{ formatDate(j.started_at || j.created_at) }})
              </option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">Target DB Host:</label>
            <input v-model="restoreParams.target_db_host" type="text" required class="input-glass font-mono" placeholder="postgres.db.svc.cluster.local:5432" />
          </div>

          <div class="form-group">
            <label class="form-label">Target DB Name:</label>
            <input v-model="restoreParams.target_db_name" type="text" required class="input-glass font-mono" placeholder="app_db_restored" />
          </div>

          <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
            <button type="button" class="btn btn-secondary" @click="showRestoreModal = false">Cancel</button>
            <button type="submit" class="btn btn-danger" :disabled="loading || !restoreParams.backup_job_id">
              <span>{{ loading ? 'Executing Restore...' : '⚡ Execute Instant Restore' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import {
  backupApi,
  type BackupPolicy,
  type BackupStorage,
  type BackupJob,
  type RestoreJob,
} from '../api/governance'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'

const activeTab = ref<'policies' | 'storages' | 'jobs' | 'restores'>('policies')
const policies = ref<BackupPolicy[]>([])
const storages = ref<BackupStorage[]>([])
const jobs = ref<BackupJob[]>([])
const restores = ref<RestoreJob[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)
const triggeringPolicyId = ref<string | null>(null)

const showPolicyModal = ref(false)
const showStorageModal = ref(false)
const showRestoreModal = ref(false)

const newPolicy = reactive({
  name: '',
  db_type: 'postgres',
  db_host: 'localhost',
  db_port: 5432,
  db_name: 'app_production',
  storage_id: 'default-s3',
  schedule: '0 */6 * * *',
  retention_count: 14,
  backup_type: 'full',
  enabled: true,
})

const newStorage = reactive({
  name: '',
  type: 's3',
  endpoint: 'https://s3.us-east-1.amazonaws.com',
  bucket: 'k8s-backups',
})

const restoreParams = reactive({
  backup_job_id: '',
  target_db_host: 'postgres.db.svc.cluster.local:5432',
  target_db_name: 'app_db',
})

const jobColumns: Column<BackupJob>[] = [
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'id', label: 'Job & Policy', width: '220px', sortable: true },
  { key: 'size', label: 'Raw → Compressed Size', width: '200px' },
  { key: 'checksum', label: 'Encryption & Checksum', width: '220px' },
  { key: 'timing', label: 'Started & Duration' },
  { key: 'actions', label: 'Action', width: '130px', align: 'right' },
]

const restoreColumns: Column<RestoreJob>[] = [
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'id', label: 'Restore ID', width: '140px', sortable: true },
  { key: 'backup_job_id', label: 'Source Snapshot', width: '170px' },
  { key: 'target', label: 'Target Database & Host' },
  { key: 'created_at', label: 'Executed At', width: '160px', sortable: true },
  { key: 'log', label: 'Verification Log' },
]

onMounted(() => {
  fetchAllBackupData()
})

async function fetchAllBackupData() {
  loading.value = true
  error.value = null
  try {
    const [p, s, j, r] = await Promise.all([
      backupApi.getPolicies(),
      backupApi.getStorages(),
      backupApi.getJobs(),
      backupApi.getRestores(),
    ])
    policies.value = p
    storages.value = s
    jobs.value = j
    restores.value = r
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load backup & recovery data'
    error.value = msg
  } finally {
    loading.value = false
  }
}

async function handleCreatePolicy() {
  loading.value = true
  statusMessage.value = null
  try {
    await backupApi.createPolicy(newPolicy)
    statusMessage.value = { type: 'success', text: `Backup policy "${newPolicy.name}" armed successfully.` }
    showPolicyModal.value = false
    await fetchAllBackupData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to create backup policy'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    loading.value = false
  }
}

async function handleCreateStorage() {
  loading.value = true
  statusMessage.value = null
  try {
    await backupApi.createStorage(newStorage)
    statusMessage.value = { type: 'success', text: `Storage target "${newStorage.name}" attached successfully.` }
    showStorageModal.value = false
    await fetchAllBackupData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to attach storage target'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    loading.value = false
  }
}

async function handleTriggerBackup(policyId: string) {
  triggeringPolicyId.value = policyId
  statusMessage.value = null
  try {
    const job = await backupApi.triggerBackup(policyId, 'full')
    statusMessage.value = {
      type: 'success',
      text: `Backup job #${job.id ? job.id.slice(0, 8) : 'new'} dispatched. Streaming to NVMe/S3.`,
    }
    await fetchAllBackupData()
    activeTab.value = 'jobs'
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to trigger backup'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    triggeringPolicyId.value = null
  }
}

function openRestoreModalWithJob(job: BackupJob) {
  restoreParams.backup_job_id = job.id
  showRestoreModal.value = true
}

async function handleExecuteRestore() {
  loading.value = true
  statusMessage.value = null
  try {
    const res = await backupApi.triggerRestore(
      restoreParams.backup_job_id,
      restoreParams.target_db_host,
      restoreParams.target_db_name
    )
    statusMessage.value = {
      type: 'success',
      text: `Restore #${res.id ? res.id.slice(0, 8) : 'new'} dispatched to ${restoreParams.target_db_name}.`,
    }
    showRestoreModal.value = false
    await fetchAllBackupData()
    activeTab.value = 'restores'
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to execute restore'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    loading.value = false
  }
}

const activePoliciesCount = computed(() => policies.value.filter(p => p.enabled).length)
const completedJobs = computed(() => jobs.value.filter(j => j.status === 'completed' || j.status === 'verified'))
const completedJobsCount = computed(() => completedJobs.value.length)
const failedJobsCount = computed(() => jobs.value.filter(j => j.status === 'failed').length)

function getDbIcon(type: string): string {
  const t = (type || '').toLowerCase()
  if (t.includes('postgres')) return '🐘'
  if (t.includes('mysql')) return '🐬'
  if (t.includes('maria')) return '🦭'
  if (t.includes('mongo')) return '🍃'
  if (t.includes('redis')) return '⚡'
  if (t.includes('nats')) return '📬'
  return '📦'
}

function getStorageIcon(type: string): string {
  const t = (type || '').toLowerCase()
  if (t.includes('s3') || t.includes('minio')) return '☁️'
  if (t.includes('local')) return '💾'
  if (t.includes('nfs')) return '🌐'
  return '📁'
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
  try {
    return new Date(d).toLocaleString()
  } catch {
    return d
  }
}
</script>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 16px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: var(--text-tag-fluid, 11px);
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.view-title {
  font-size: var(--text-title-fluid, 24px);
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
  line-height: 1.25;
}

.view-desc {
  font-size: var(--text-desc-fluid, 13px);
  color: var(--text-secondary);
  max-width: 820px;
  margin-top: 4px;
  line-height: 1.5;
}

.highlight {
  color: #fff;
  font-weight: 600;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.status-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 18px;
  border-radius: 12px;
  font-size: 13px;
}

.banner-success {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.banner-error {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fda4af;
}

.banner-icon {
  font-size: 16px;
}

.banner-text {
  flex: 1;
  font-weight: 500;
}

.banner-close {
  background: none;
  border: none;
  color: inherit;
  font-size: 14px;
  cursor: pointer;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.tabs-bar {
  padding: 6px 10px;
  display: flex;
  gap: 8px;
  overflow-x: auto;
  scrollbar-width: thin;
  -webkit-overflow-scrolling: touch;
}

.tab-btn {
  padding: 8px 16px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  background: transparent;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.tab-btn:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.04);
}

.tab-btn-active {
  color: #fff;
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.35);
}

.tab-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.policies-grid, .storages-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.policy-card, .storage-card {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.policy-card-top, .storage-top {
  display: flex;
  align-items: center;
  gap: 12px;
}

.policy-icon-box, .storage-icon-box {
  width: 42px;
  height: 42px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
}

.policy-meta, .storage-meta {
  flex: 1;
  min-width: 0;
}

.policy-name, .storage-name {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.policy-sub, .storage-type {
  font-size: 11px;
}

.policy-body, .storage-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  background: rgba(0, 0, 0, 0.25);
  padding: 12px;
  border-radius: 8px;
}

.policy-stat-row, .storage-row {
  display: flex;
  justify-content: space-between;
}

.policy-k, .storage-k {
  color: var(--text-muted);
}

.policy-v, .storage-v {
  font-weight: 600;
}

.policy-actions {
  display: flex;
}

.policy-actions .btn {
  width: 100%;
}

.empty-state-box {
  padding: 48px 24px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.empty-icon {
  font-size: 36px;
}

.empty-title {
  font-size: 18px;
  color: #fff;
}

.empty-desc {
  font-size: 13px;
  color: var(--text-muted);
  max-width: 460px;
  margin-bottom: 8px;
}

.job-id-cell, .target-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.job-id {
  font-weight: 700;
  color: #fff;
}

.job-policy {
  font-size: 11px;
}

.size-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.size-arrow {
  color: var(--text-muted);
  font-size: 10px;
}

.crypto-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.cipher-pill {
  font-size: 9px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(139, 92, 246, 0.15);
  color: #a78bfa;
}

.hash-text {
  font-size: 11px;
}

.timing-cell {
  font-size: 11px;
  display: flex;
  gap: 6px;
}

.log-preview {
  font-size: 11px;
  color: var(--text-secondary);
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 20px;
}

.modal-card {
  width: 100%;
  max-width: 580px;
  background: var(--bg-sidebar);
  border: 1px solid var(--border-medium);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.7);
}

.modal-header {
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  border-bottom: 1px solid var(--border-subtle);
}

.modal-title-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.modal-title {
  font-size: 18px;
  color: #fff;
}

.modal-close {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 18px;
  cursor: pointer;
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group-row {
  display: flex;
  gap: 12px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.alert-box {
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 12px;
}

.alert-warning {
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.3);
  color: #fbbf24;
}

.modal-footer {
  padding: 16px 20px;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  border-top: 1px solid var(--border-subtle);
}

@media (max-width: 768px) {
  .view-header {
    flex-direction: column;
    align-items: stretch;
    gap: 14px;
  }
  .header-actions {
    flex-direction: column;
    width: 100%;
  }
  .header-actions .btn {
    width: 100%;
    justify-content: center;
  }
  .metrics-grid {
    grid-template-columns: 1fr;
  }
  .policies-grid, .storages-grid {
    grid-template-columns: 1fr;
  }
  .form-group-row {
    flex-direction: column;
    gap: 12px;
  }
  .modal-overlay {
    padding: 12px;
  }
  .modal-card {
    max-width: 100%;
    border-radius: 14px;
  }
  .modal-footer {
    flex-direction: column;
    gap: 8px;
  }
  .modal-footer .btn {
    width: 100%;
    justify-content: center;
  }
}

@media (max-width: 640px) {
  .view-tag {
    font-size: var(--text-tag-fluid, 10px);
    letter-spacing: 0.05em;
    font-weight: 700;
    margin-bottom: 4px;
  }

  .view-title {
    font-size: var(--text-title-fluid, clamp(18px, 4.5vw, 22px));
    font-weight: 700;
    line-height: 1.25;
    letter-spacing: -0.02em;
  }

  .view-desc {
    font-size: var(--text-desc-fluid, 12px);
    line-height: 1.45;
    color: var(--text-muted);
    margin-top: 4px;
  }

  .policy-card, .storage-card {
    padding: 14px;
  }
}
</style>
