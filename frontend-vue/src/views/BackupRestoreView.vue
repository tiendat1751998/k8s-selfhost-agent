<script setup lang="ts">
import { ref, onMounted } from 'vue'
import '../assets/styles/views/backup.css'
import '../assets/styles/components/backup-drawers.css'
import { fleetApi } from '../api/fleet'
import { useBackupRestore } from '../composables/useBackupRestore'
import BaseIcon from '../components/ui/BaseIcon.vue'
import BackupSchedulesTable from '../components/backup/BackupSchedulesTable.vue'
import BackupStoragesGrid from '../components/backup/BackupStoragesGrid.vue'
import BackupSnapshotsTable from '../components/backup/BackupSnapshotsTable.vue'
import BackupMobileCards from '../components/backup/BackupMobileCards.vue'
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
  deletingJobId,
  downloadingJobId,
  volumeProgress,
  showPolicyModal,
  showStorageModal,
  showRestoreModal,
  newStorage,
  restoreParams,
  completedJobs,
  completedJobsCount,
  fetchAllBackupData,
  handleCreatePolicy,
  handleCreateStorage,
  handleTriggerBackup,
  handleDeleteJob,
  handleDownloadSnapshot,
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
    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="backup-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="save" size="xs" /> Backup & DR ({{ policies.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Create or restore"
          aria-label="Create or restore"
          @click="activeTab === 'policies' ? showPolicyModal = true : activeTab === 'storages' ? showStorageModal = true : showRestoreModal = true"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh backup data"
          aria-label="Refresh backup data"
          @click="fetchAllBackupData"
        >
          <BaseIcon name="refresh" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="backup-micro-telemetry mobile-only font-mono" role="status" aria-label="Backup Micro Telemetry">
      <span class="tel-item tel-policies"><BaseIcon name="save" size="xs" /> {{ policies.length }} pol</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-storages"><BaseIcon name="hard-drive" size="xs" /> {{ storages.length }} stor</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-snaps"><BaseIcon name="shield" size="xs" /> {{ completedJobsCount }} snaps</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-restores"><BaseIcon name="refresh" size="xs" /> {{ restores.length }} rest</span>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- View Tabs Switcher & Actions Toolbar -->
    <div class="tabs-bar glass-panel">
      <div class="tabs-group" role="tablist" aria-label="Backup and DR Navigation">
        <button
          class="tab-btn"
          :class="{ 'tab-btn-active': activeTab === 'policies' }"
          role="tab"
          :aria-selected="activeTab === 'policies'"
          @click="activeTab = 'policies'"
        >
          <BaseIcon name="file-text" size="xs" /> <span>Backup Policies ({{ policies.length }})</span>
        </button>
        <button
          class="tab-btn"
          :class="{ 'tab-btn-active': activeTab === 'storages' }"
          role="tab"
          :aria-selected="activeTab === 'storages'"
          @click="activeTab = 'storages'"
        >
          <BaseIcon name="hard-drive" size="xs" /> <span>Storage Targets ({{ storages.length }})</span>
        </button>
        <button
          class="tab-btn"
          :class="{ 'tab-btn-active': activeTab === 'jobs' }"
          role="tab"
          :aria-selected="activeTab === 'jobs'"
          @click="activeTab = 'jobs'"
        >
          <BaseIcon name="zap" size="xs" /> <span>Backup History ({{ jobs.length }})</span>
        </button>
        <button
          class="tab-btn"
          :class="{ 'tab-btn-active': activeTab === 'restores' }"
          role="tab"
          :aria-selected="activeTab === 'restores'"
          @click="activeTab = 'restores'"
        >
          <BaseIcon name="refresh" size="xs" /> <span>Restore Actions ({{ restores.length }})</span>
        </button>
        <button
          class="tab-btn"
          :class="{ 'tab-btn-active': activeTab === 'cluster-dr' }"
          role="tab"
          :aria-selected="activeTab === 'cluster-dr'"
          @click="activeTab = 'cluster-dr'"
        >
          <BaseIcon name="anchor" size="xs" /> <span>Cluster DR & etcd</span>
        </button>
      </div>

      <div class="tabs-actions desktop-only">
        <button class="btn btn-secondary btn-sm" :disabled="loading" @click="fetchAllBackupData">
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" /> <span>{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
        <button v-if="activeTab === 'policies'" class="btn btn-primary btn-sm" @click="showPolicyModal = true">
          <span>+ Create Backup Policy</span>
        </button>
        <button v-else-if="activeTab === 'storages'" class="btn btn-primary btn-sm" @click="showStorageModal = true">
          <span>+ Add Storage Target</span>
        </button>
        <button v-else-if="activeTab === 'restores'" class="btn btn-primary btn-sm" @click="showRestoreModal = true">
          <span>+ Trigger Restore</span>
        </button>
      </div>
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
        class="desktop-only"
        :jobs="jobs"
        :loading="loading"
        :error="error"
        :deleting-job-id="deletingJobId"
        :downloading-job-id="downloadingJobId"
        :volume-progress="volumeProgress"
        @restore="openRestoreModalWithJob"
        @download="handleDownloadSnapshot"
        @delete="handleDeleteJob"
      />
      <BackupMobileCards
        class="mobile-only"
        :jobs="jobs"
        :deleting-job-id="deletingJobId"
        :downloading-job-id="downloadingJobId"
        @restore="openRestoreModalWithJob"
        @download="handleDownloadSnapshot"
        @delete="handleDeleteJob"
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
