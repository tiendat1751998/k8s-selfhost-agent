import { api, type ApiResponse } from './client'

export interface EtcdSnapshotResult {
  id: string;
  cluster_id?: string;
  snapshot_name?: string;
  size_bytes?: number;
  created_at: string;
  status?: 'Completed' | 'InProgress' | 'Failed' | string;
  endpoint?: string;
  message?: string;
}

export interface ClusterBackupResult {
  id: string;
  cluster_id?: string;
  name: string;
  phase: 'Completed' | 'InProgress' | 'Failed' | string;
  total_items?: number;
  items_backed_up?: number;
  backup_type?: string;
  created_at: string;
  completed_at?: string;
  size_bytes?: number;
  errors?: number;
  warnings?: number;
  message?: string;
}

export interface RemediationResult {
  node_name: string;
  action?: string;
  status?: 'Completed' | 'InProgress' | 'Failed' | 'Remediated' | string;
  message?: string;
  timestamp?: string;
  evicted_pods?: string[];
  duration_ms?: number;
}

function unwrapData<T>(res: ApiResponse<T> | T): T {
  if (res && typeof res === 'object' && 'data' in res && (res as ApiResponse<T>).data !== undefined) {
    return (res as ApiResponse<T>).data
  }
  return res as T
}

export async function triggerEtcdSnapshot(clusterId: string): Promise<EtcdSnapshotResult> {
  const cid = encodeURIComponent(clusterId)
  const res = await api.post<ApiResponse<EtcdSnapshotResult> | EtcdSnapshotResult>(`/k8s/${cid}/dr/etcd/snapshot`)
  return unwrapData(res)
}

export async function restoreEtcdSnapshot(clusterId: string, snapshotId: string): Promise<EtcdSnapshotResult> {
  const cid = encodeURIComponent(clusterId)
  const res = await api.post<ApiResponse<EtcdSnapshotResult> | EtcdSnapshotResult>(`/k8s/${cid}/dr/etcd/restore`, {
    snapshot_id: snapshotId,
    snapshotId,
  })
  return unwrapData(res)
}

export async function listClusterBackups(clusterId: string): Promise<ClusterBackupResult[]> {
  const cid = encodeURIComponent(clusterId)
  const res = await api.get<ApiResponse<ClusterBackupResult[]> | ClusterBackupResult[]>(`/k8s/${cid}/dr/backups`)
  const data = unwrapData(res)
  return Array.isArray(data) ? data : []
}

export async function createClusterBackup(clusterId: string, backupName?: string): Promise<ClusterBackupResult> {
  const cid = encodeURIComponent(clusterId)
  const payload = backupName ? { backup_name: backupName, backupName } : {}
  const res = await api.post<ApiResponse<ClusterBackupResult> | ClusterBackupResult>(`/k8s/${cid}/dr/backups`, payload)
  return unwrapData(res)
}

export async function triggerRemediation(clusterId: string, nodeName: string): Promise<RemediationResult> {
  const cid = encodeURIComponent(clusterId)
  const node = encodeURIComponent(nodeName)
  const res = await api.post<ApiResponse<RemediationResult> | RemediationResult>(`/k8s/${cid}/dr/remediation/${node}`)
  return unwrapData(res)
}

export const drApi = {
  triggerEtcdSnapshot,
  restoreEtcdSnapshot,
  listClusterBackups,
  createClusterBackup,
  triggerRemediation,
}
