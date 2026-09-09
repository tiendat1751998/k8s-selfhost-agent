<script setup lang="ts">
import { ref, onMounted } from 'vue'
import '../assets/styles/views/backup.css'
import { fleetApi } from '../api/fleet'
import { useBackupRestore } from '../composables/useBackupRestore'
import MetricCard from '../components/ui/MetricCard.vue'
import BackupSchedulesTable from '../components/backup/BackupSchedulesTable.vue'
import BackupStoragesGrid from '../components/backup/BackupStoragesGrid.vue'
import BackupSnapshotsTable from '../components/backup/BackupSnapshotsTable.vue'
import RestoreActionsTable from '../components/backup/RestoreActionsTable.vue'
import ClusterDisasterRecoveryTab from '../components/backup/ClusterDisasterRecoveryTab.vue'
import BackupCreateModal from '../components/backup/BackupCreateModal.vue'
import CreateStorageModal from '../components/backup/CreateStorageModal.vue'
import BackupRestoreModal from '../components/backup/BackupRestoreModal.vue'

const {
  activeTab,
  policies,
  storages,
  jobs,
  restores,
  loading,
  error,
  statusMessage,
  triggeringPolicyId,
  showPolicyModal,
  showStorageModal,
  showRestoreModal,
  newStorage,
  restoreParams,
  activePoliciesCount,
  completedJobs,
  completedJobsCount,
  failedJobsCount,
  fetchAllBackupData,
  handleCreatePolicy,
  handleCreateStorage,
  handleTriggerBackup,
  openRestoreModalWithJob,
  handleExecuteRestore,
} = useBackupRestore()

const selectedCluster = ref<string>('primary-cluster')

onMounted(async () => {
  try {
    const list = await fleetApi.list()
    if (list && list.length > 0) {
      selectedCluster.value = list[0].name || list[0].id || 'primary-cluster'
    }
  } catch {
    // fallback primary-cluster
  }
})
</script>

<template>
  <div class="view-container">
    <!-- Desktop View Header -->
    <div class="view-header desktop-header desktop-only">
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

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="backup-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">💾 Backup & DR ({{ policies.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Create or restore"
          aria-label="Create or restore"
          @click="activeTab === 'policies' ? showPolicyModal = true : activeTab === 'storages' ? showStorageModal = true : showRestoreModal = true"
        >
          <span>➕</span>
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh backup data"
          aria-label="Refresh backup data"
          @click="fetchAllBackupData"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="backup-micro-telemetry mobile-only font-mono" role="status" aria-label="Backup Micro Telemetry">
      <span class="tel-item tel-policies">💾 {{ policies.length }} pol</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-storages">🗄️ {{ storages.length }} stor</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-snaps">🛡️ {{ completedJobsCount }} snaps</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-restores">⏪ {{ restores.length }} rest</span>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Metric HUD Cards -->
    <div class="metrics-grid desktop-metrics desktop-only">
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
      <button
        class="tab-btn"
        :class="{ 'tab-btn-active': activeTab === 'cluster-dr' }"
        @click="activeTab = 'cluster-dr'"
      >
        <span>🌐 Cluster DR & etcd</span>
      </button>
    </div>

    <!-- TAB 1: POLICIES -->
    <div v-if="activeTab === 'policies'" class="tab-content animate-fade-in">
      <BackupSchedulesTable
        :policies="policies"
        :triggering-policy-id="triggeringPolicyId"
        @create="showPolicyModal = true"
        @trigger="handleTriggerBackup"
      />
    </div>

    <!-- TAB 2: STORAGE TARGETS -->
    <div v-if="activeTab === 'storages'" class="tab-content animate-fade-in">
      <BackupStoragesGrid
        :storages="storages"
        @create="showStorageModal = true"
      />
    </div>

    <!-- TAB 3: BACKUP HISTORY / JOBS -->
    <div v-if="activeTab === 'jobs'" class="tab-content animate-fade-in">
      <BackupSnapshotsTable
        :jobs="jobs"
        :loading="loading"
        :error="error"
        @restore="openRestoreModalWithJob"
      />
    </div>

    <!-- TAB 4: RESTORE ACTIONS -->
    <div v-if="activeTab === 'restores'" class="tab-content animate-fade-in">
      <RestoreActionsTable
        :restores="restores"
        :loading="loading"
        :error="error"
      />
    </div>

    <!-- TAB 5: CLUSTER DR & ETCD -->
    <div v-if="activeTab === 'cluster-dr'" class="tab-content animate-fade-in">
      <ClusterDisasterRecoveryTab :cluster-id="selectedCluster" />
    </div>

    <!-- Modals -->
    <BackupCreateModal
      v-model="showPolicyModal"
      :storages="storages"
      :loading="loading"
      @create="handleCreatePolicy"
    />

    <CreateStorageModal
      v-model="showStorageModal"
      :form="newStorage"
      :loading="loading"
      @submit="handleCreateStorage"
    />

    <BackupRestoreModal
      v-model="showRestoreModal"
      :completed-jobs="completedJobs"
      :initial-job-id="restoreParams.backup_job_id"
      :loading="loading"
      @restore="handleExecuteRestore"
    />
  </div>
</template>
