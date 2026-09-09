import { api, type ApiResponse } from './client'

export type ReplicaStatus = 'synced' | 'syncing' | 'degraded' | 'faulted' | string
export type VolumeHealth = 'healthy' | 'degraded' | 'faulted' | 'syncing' | string

export interface VolumeReplica {
  node: string
  status: ReplicaStatus
  mode: string // e.g. 'RW', 'RO', 'Syncing'
  ip?: string
  disk_path?: string
  allocated_bytes?: number
}

export interface DistributedVolume {
  name: string
  namespace: string
  storage_class: string
  capacity_bytes: number
  capacity_human?: string
  health: VolumeHealth
  status?: string
  replicas: VolumeReplica[]
  pvc_name?: string
  pv_name?: string
  created_at?: string
  updated_at?: string
}

export interface VolumeExpandRequest {
  namespace?: string
  new_size_bytes: number
}

export interface VolumeSnapshotResult {
  snapshot_name: string
  volume_name: string
  namespace: string
  status: string
  size_bytes?: number
  created_at?: string
}

export function parseStorageBytes(raw: string): number {
  if (!raw || raw === 'N/A') return 10 * 1024 * 1024 * 1024
  const match = raw.trim().match(/^([0-9.]+)\s*([A-Za-z]+)?$/)
  if (!match) return 10 * 1024 * 1024 * 1024
  const val = parseFloat(match[1])
  const unit = (match[2] || 'Gi').toLowerCase()
  if (unit.startsWith('ki') || unit === 'k') return Math.round(val * 1024)
  if (unit.startsWith('mi') || unit === 'm') return Math.round(val * 1024 * 1024)
  if (unit.startsWith('gi') || unit === 'g') return Math.round(val * 1024 * 1024 * 1024)
  if (unit.startsWith('ti') || unit === 't') return Math.round(val * 1024 * 1024 * 1024 * 1024)
  return Math.round(val)
}

export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 GiB'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const val = bytes / Math.pow(1024, i)
  return `${Number.isInteger(val) ? val : val.toFixed(1)} ${units[i]}`
}

export const storageApi = {
  async listVolumes(clusterId: string): Promise<DistributedVolume[]> {
    const res = await api.get<ApiResponse<DistributedVolume[]> | DistributedVolume[]>(
      `/k8s/${encodeURIComponent(clusterId)}/storage/volumes`
    )
    if (Array.isArray(res)) return res
    if (res && Array.isArray((res as ApiResponse<DistributedVolume[]>).data)) {
      return (res as ApiResponse<DistributedVolume[]>).data
    }
    return []
  },

  async expandVolume(
    clusterId: string,
    name: string,
    namespace: string,
    newSizeBytes: number
  ): Promise<DistributedVolume> {
    const payload: VolumeExpandRequest = {
      namespace,
      new_size_bytes: newSizeBytes,
    }
    const res = await api.post<ApiResponse<DistributedVolume> | DistributedVolume>(
      `/k8s/${encodeURIComponent(clusterId)}/storage/volumes/${encodeURIComponent(name)}/expand`,
      payload
    )
    if (res && typeof res === 'object' && 'data' in res && res.data) {
      return res.data
    }
    return res as DistributedVolume
  },

  async createSnapshot(
    clusterId: string,
    name: string,
    namespace: string
  ): Promise<VolumeSnapshotResult> {
    const res = await api.post<ApiResponse<VolumeSnapshotResult> | VolumeSnapshotResult>(
      `/k8s/${encodeURIComponent(clusterId)}/storage/volumes/${encodeURIComponent(name)}/snapshot`,
      { namespace }
    )
    if (res && typeof res === 'object' && 'data' in res && res.data) {
      return res.data
    }
    return res as VolumeSnapshotResult
  },
}

export const listVolumes = storageApi.listVolumes
export const expandVolume = storageApi.expandVolume
export const createSnapshot = storageApi.createSnapshot
