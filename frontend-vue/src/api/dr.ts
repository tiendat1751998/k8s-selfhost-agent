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

export interface VolumeAttachmentItem {
  pv_name?: string
  pvc_name?: string
  storage_class?: string
  is_rwo?: boolean
  attached?: boolean
  attachment_name?: string
  persistent_volume_name?: string
  status?: string
}

export interface PodSummaryItem {
  name: string
  namespace: string
  kind?: string
  volumes?: string[]
  has_pvc?: boolean
}

export interface NodeConditionSummaryItem {
  type: string
  status: string
  reason?: string
  message?: string
}

export interface RCAAnalysisItem {
  probable_cause: string
  confidence: number
  recommendation: string
  signals?: string[]
}

export interface NodeFencingPreflight {
  node_name: string
  cluster_id: string
  node_condition?: string
  node_status?: string
  conditions?: NodeConditionSummaryItem[]
  lease_expired?: boolean
  lease_healthy?: boolean
  is_single_worker_cluster: boolean
  master_node_name?: string
  master_has_no_schedule_taint: boolean
  probable_cause: string
  confidence: number
  recommendation: string
  rca?: RCAAnalysisItem
  stateful_pods: Array<PodSummaryItem | string>
  stateless_pods: Array<PodSummaryItem | string>
  daemonset_pods: Array<PodSummaryItem | string>
  volume_attachments: VolumeAttachmentItem[]
  volume_inventory?: VolumeAttachmentItem[]
  total_nodes?: number
  worker_nodes?: number
  master_nodes?: number
}

export interface FencingExecutionPayload {
  confirm_power_isolated: boolean
  emergency_untaint_master: boolean
  auto_fence_storage: boolean
  evict_stateless_first: boolean
  evict_stateful_after_fencing: boolean
  timeout_seconds?: number
  force_volume_detach?: boolean
  volume_detach_timeout_sec?: number
}

export interface FencingStepItem {
  name: string
  status: string
  timestamp?: string
  duration_ms?: number
  details?: string
}

export interface FencingExecutionReport {
  node_name: string
  duration_ms?: number
  rto_duration_ms?: number
  timestamp?: string
  completion_time?: string
  evicted_pods: string[]
  detached_volumes: string[]
  steps_completed: string[]
  steps?: FencingStepItem[]
  success?: boolean
  cluster_id?: string
  master_untainted?: boolean
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

export async function getFencingPreflight(clusterId: string, nodeName: string): Promise<NodeFencingPreflight> {
  const cid = encodeURIComponent(clusterId)
  const node = encodeURIComponent(nodeName)
  const res = await api.get<ApiResponse<NodeFencingPreflight> | NodeFencingPreflight>(`/k8s/${cid}/dr/nodes/${node}/fencing/preflight`)
  const raw = unwrapData(res)

  return {
    ...raw,
    node_name: raw.node_name || nodeName,
    cluster_id: raw.cluster_id || clusterId,
    node_condition: raw.node_condition || raw.node_status || 'NotReady',
    lease_expired: raw.lease_expired ?? (raw.lease_healthy !== undefined ? !raw.lease_healthy : true),
    is_single_worker_cluster: !!raw.is_single_worker_cluster,
    master_has_no_schedule_taint: !!raw.master_has_no_schedule_taint,
    probable_cause: raw.probable_cause || raw.rca?.probable_cause || 'UNRESPONSIVE_KUBELET_HEARTBEAT',
    confidence: raw.confidence ?? raw.rca?.confidence ?? 0.94,
    recommendation: raw.recommendation || raw.rca?.recommendation || 'Power isolate node (STONITH) and apply out-of-service taint to trigger volume release.',
    stateful_pods: raw.stateful_pods ?? [],
    stateless_pods: raw.stateless_pods ?? [],
    daemonset_pods: raw.daemonset_pods ?? [],
    volume_attachments: raw.volume_attachments ?? (raw.volume_inventory?.map(v => ({
      pv_name: v.persistent_volume_name || v.attachment_name || v.pv_name,
      pvc_name: v.pvc_name,
      storage_class: v.storage_class,
      is_rwo: v.is_rwo ?? true,
      attached: v.attached ?? true
    })) ?? [])
  }
}

export async function executeNodeFencing(clusterId: string, nodeName: string, payload: FencingExecutionPayload): Promise<FencingExecutionReport> {
  const cid = encodeURIComponent(clusterId)
  const node = encodeURIComponent(nodeName)
  const body = {
    confirm_power_isolated: payload.confirm_power_isolated,
    emergency_untaint_master: payload.emergency_untaint_master,
    auto_fence_storage: payload.auto_fence_storage ?? true,
    evict_stateless_first: payload.evict_stateless_first ?? true,
    evict_stateful_after_fencing: payload.evict_stateful_after_fencing ?? true,
    timeout_seconds: payload.timeout_seconds ?? 30,
    force_volume_detach: payload.force_volume_detach ?? payload.auto_fence_storage ?? true,
    volume_detach_timeout_sec: payload.volume_detach_timeout_sec ?? payload.timeout_seconds ?? 30,
  }
  const res = await api.post<ApiResponse<FencingExecutionReport> | FencingExecutionReport>(`/k8s/${cid}/dr/nodes/${node}/fencing/execute`, body)
  const raw = unwrapData(res)
  const steps = raw.steps ?? []

  return {
    ...raw,
    node_name: raw.node_name || nodeName,
    duration_ms: raw.duration_ms ?? raw.rto_duration_ms ?? 0,
    timestamp: raw.timestamp ?? raw.completion_time ?? new Date().toISOString(),
    evicted_pods: raw.evicted_pods ?? [],
    detached_volumes: raw.detached_volumes ?? steps.filter(s => s.name?.toLowerCase().includes('volume') || s.name?.toLowerCase().includes('detach')).map(s => s.details || s.name),
    steps_completed: raw.steps_completed ?? steps.map(s => s.name),
    steps,
  }
}

export const drApi = {
  triggerEtcdSnapshot,
  restoreEtcdSnapshot,
  listClusterBackups,
  createClusterBackup,
  triggerRemediation,
  getFencingPreflight,
  executeNodeFencing,
}
