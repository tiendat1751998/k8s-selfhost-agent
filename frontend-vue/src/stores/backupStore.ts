import { defineStore } from 'pinia'
import { ref } from 'vue'
import { backupApi, type BackupPolicy, type BackupJob, type RestoreJob, type BackupStorage } from '../api/backup'

export const useBackupStore = defineStore('backup', () => {
  const policies = ref<BackupPolicy[]>([])
  const jobs = ref<BackupJob[]>([])
  const restores = ref<RestoreJob[]>([])
  const storages = ref<BackupStorage[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchAll() {
    loading.value = true
    error.value = null
    try {
      const [p, j, r, s] = await Promise.all([
        backupApi.getPolicies(),
        backupApi.getJobs(),
        backupApi.getRestores(),
        backupApi.getStorages(),
      ])
      policies.value = p
      jobs.value = j
      restores.value = r
      storages.value = s
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch backup data'
    } finally {
      loading.value = false
    }
  }

  async function triggerBackup(policyId: string, type: string = 'full') {
    loading.value = true
    try {
      const newJob = await backupApi.triggerBackup(policyId, type)
      jobs.value.unshift(newJob)
      return newJob
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to trigger backup'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function triggerRestore(jobId: string, host: string, db: string) {
    loading.value = true
    try {
      const restore = await backupApi.triggerRestore(jobId, host, db)
      restores.value.unshift(restore)
      return restore
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to trigger restore'
      throw err
    } finally {
      loading.value = false
    }
  }

  return {
    policies,
    jobs,
    restores,
    storages,
    loading,
    error,
    fetchAll,
    triggerBackup,
    triggerRestore,
  }
})
