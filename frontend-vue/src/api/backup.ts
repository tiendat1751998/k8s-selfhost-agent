import { api, type ApiResponse } from './client'

export interface BackupStorage {
  id: string
  tenant_id: string
  name: string
  type: string
  endpoint: string
  bucket: string
  created_at: string
}

export interface BackupPolicy {
  id: string
  tenant_id: string
  name: string
  db_type: string
  db_host: string
  db_port: number
  db_name: string
  storage_id: string
  schedule: string
  retention_count: number
  backup_type: string
  enabled: boolean
}

export interface BackupJob {
  id: string
  tenant_id: string
  policy_id: string
  backup_type: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  bytes_raw?: number
  bytes_compressed?: number
  checksum_sha256?: string
  storage_path?: string
  storage_path_cloud?: string
  started_at?: string
  completed_at?: string
  error_message?: string
}

export interface RestoreJob {
  id: string
  tenant_id: string
  backup_job_id: string
  target_db_host: string
  target_db_name: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  bytes_restored?: number
  error_message?: string
  started_at?: string
  completed_at?: string
}

export const backupApi = {
  async getStorages(): Promise<BackupStorage[]> {
    const res = await api.get<ApiResponse<BackupStorage[]>>('/backup/storages')
    return res.data || []
  },

  async getPolicies(): Promise<BackupPolicy[]> {
    const res = await api.get<ApiResponse<BackupPolicy[]>>('/backup/policies')
    return res.data || []
  },

  async createPolicy(policy: Partial<BackupPolicy>): Promise<BackupPolicy> {
    return api.post<BackupPolicy>('/backup/policies', policy)
  },

  async getJobs(): Promise<BackupJob[]> {
    const res = await api.get<ApiResponse<BackupJob[]>>('/backup/jobs')
    return res.data || []
  },

  async triggerBackup(policyId: string, backupType: string = 'full'): Promise<BackupJob> {
    return api.post<BackupJob>('/backup/jobs', {
      policy_id: policyId,
      backup_type: backupType,
    })
  },

  async getRestores(): Promise<RestoreJob[]> {
    const res = await api.get<ApiResponse<RestoreJob[]>>('/backup/restores')
    return res.data || []
  },

  async triggerRestore(backupJobId: string, targetHost: string, targetDB: string): Promise<RestoreJob> {
    return api.post<RestoreJob>('/backup/restores', {
      backup_job_id: backupJobId,
      target_db_host: targetHost,
      target_db_name: targetDB,
    })
  },
}
