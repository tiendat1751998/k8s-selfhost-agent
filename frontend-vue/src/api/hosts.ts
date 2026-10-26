import { api } from './client'
import { dockerApi } from './docker'

// ==========================================
// Compute Host Interfaces & Telemetry
// ==========================================

export type HostType = 'agent' | 'docker' | 'k8s' | 'prometheus' | 'git' | 'database' | 'custom' | string
export type HostStatus = 'connected' | 'disconnected' | 'pending' | 'error' | string

export interface ComputeHost {
  id: string
  name: string
  host_type: HostType
  endpoint: string
  tls_enabled: boolean
  tls_ca?: string
  tls_cert?: string
  tls_key?: string
  api_version?: string
  status: HostStatus
  last_health_check?: string
  labels: Record<string, string>
  tenant_id?: string
  runtime_environment?: string
  host_role?: string
  created_at: string
  updated_at?: string
}

export interface CreateHostRequest {
  name: string
  host_type?: HostType
  endpoint: string
  tls_enabled?: boolean
  tls_ca?: string
  tls_cert?: string
  tls_key?: string
  ca_cert?: string
  client_cert?: string
  client_key?: string
  api_version?: string
  labels?: Record<string, string>
}

export interface UpdateHostRequest {
  name?: string
  host_type?: HostType
  endpoint?: string
  tls_enabled?: boolean
  tls_ca?: string
  tls_cert?: string
  tls_key?: string
  ca_cert?: string
  client_cert?: string
  client_key?: string
  api_version?: string
  labels?: Record<string, string>
}

export interface AgentInfo {
  hostname?: string
  os?: string
  arch?: string
  os_distro?: string
  kernel_version?: string
  uptime?: number
  uptime_seconds?: number
}

export interface ProcessMetric {
  pid: number
  name: string
  command_line: string
  user: string
  cpu_percent: number
  memory_bytes: number
  memory_percent: number
  read_bytes_per_sec: number
  write_bytes_per_sec: number
  state: string
}

export interface NetworkInterface {
  name: string
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
}

export interface DiskIOStats {
  device_name: string
  read_bytes_per_sec: number
  write_bytes_per_sec: number
  read_iops: number
  write_iops: number
  avg_wait_ms: number
  avg_req_size_kb: number
  current_queue_depth: number
  io_utilization_pct: number
  is_root_device: boolean
}

export interface DiskIOMetrics {
  total_read_bytes_per_sec: number
  total_write_bytes_per_sec: number
  total_read_iops: number
  total_write_iops: number
  avg_await_ms: number
  max_io_util_pct: number
  devices: DiskIOStats[]
}

export interface NodeMetrics {
  node_id: string
  node_name: string
  role: string
  status: string
  source?: 'docker' | 'agent' | string
  cpu_percent: number
  memory_used: number
  memory_total: number
  memory_percent: number
  disk_used: number
  disk_total: number
  disk_percent: number
  network_rx_bytes: number
  network_tx_bytes: number
  network_interfaces?: NetworkInterface[]
  disk_read_bytes_per_sec?: number
  disk_write_bytes_per_sec?: number
  disk_read_iops?: number
  disk_write_iops?: number
  disk_avg_await_ms?: number
  disk_max_io_util_pct?: number
  disk_devices?: DiskIOStats[]
  container_count: number
  running_count: number
  processes?: number
  os?: string
  arch?: string
  os_distro?: string
  kernel_version?: string
  uptime?: number
  uptime_seconds?: number
  load_avg?: [number, number, number] | number[]
  load_average?: [number, number, number] | number[]
  top_processes?: ProcessMetric[]
}

export interface TestHostResponse {
  status: string
  latency_ms: number
  message?: string
  agent_info?: AgentInfo
}

// ==========================================
// Resource Explorer Interfaces
// ==========================================

export interface K8sResource {
  id: string
  kind: 'Pod' | 'Deployment' | 'Service' | 'Node' | 'Ingress' | 'PVC' | string
  name: string
  namespace?: string
  cluster: string
  status: string
  labels?: Record<string, string>
  age: string
}

// ==========================================
// K8s & Node Remediation Interfaces
// ==========================================

export interface UnhealthyContainer {
  id: string
  name: string
  image: string
  state: string
  status: string
  restart_count: number
}

export interface NodeK8sHealthReport {
  node_name: string
  timestamp?: string
  checked_at?: string
  kubelet_healthy: boolean
  kubelet_status?: string
  container_runtime?: string
  runtime_healthy?: boolean
  containerd_healthy?: boolean
  docker_healthy?: boolean
  crio_healthy?: boolean
  runtime_socket_path?: string
  cri_socket_path?: string
  d_state_count?: number
  d_state_processes?: number
  zombie_count: number
  stuck_shim_count?: number
  anomalies?: string[]
  detected_anomalies?: string[]
  unhealthy_containers?: UnhealthyContainer[]
  disk_pressure?: boolean
  inode_pressure?: boolean
  memory_pressure?: boolean
  pid_pressure?: boolean
}

export interface RemediationRequest {
  action: 'restart_service' | 'disk_pressure_cleanup' | 'cni_ipam_cleanup' | 'reap_zombies' | 'restart_container'
  target?: string
  reason?: string
}

export interface RemediationResult {
  action: string
  target?: string
  success?: boolean
  status?: string
  details?: string
  output?: string
  bytes_freed?: number
  inodes_freed?: number
  executed_at?: string
  timestamp?: string
  duration_ms: number
}

// ==========================================
// API Clients Export
// ==========================================

export const hostsApi = {
  async list(): Promise<ComputeHost[]> {
    return dockerApi.listHosts()
  },
  async create(data: CreateHostRequest): Promise<ComputeHost> {
    return dockerApi.createHost(data)
  },
  async update(id: string, data: UpdateHostRequest): Promise<ComputeHost> {
    return dockerApi.updateHost(id, data)
  },
  async delete(id: string): Promise<{ status: string }> {
    return dockerApi.deleteHost(id)
  },
  async test(id: string): Promise<TestHostResponse> {
    return dockerApi.testHost(id)
  },
  async testEndpoint(id: string): Promise<TestHostResponse> {
    return dockerApi.testHost(id)
  },
  async getK8sHealth(id: string): Promise<NodeK8sHealthReport> {
    return api.get<NodeK8sHealthReport>(`/hosts/${id}/k8s-health`)
  },
  async remediate(id: string, payload: RemediationRequest): Promise<RemediationResult> {
    return api.post<RemediationResult>(`/hosts/${id}/remediate`, payload)
  },
  async getRemediationHistory(id: string): Promise<RemediationResult[]> {
    try {
      const res = await api.get<RemediationResult[] | { data: RemediationResult[] }>(`/hosts/${id}/remediate/history`)
      if (Array.isArray(res)) return res
      if (res && Array.isArray((res as { data: RemediationResult[] }).data)) {
        return (res as { data: RemediationResult[] }).data
      }
      return []
    } catch {
      return []
    }
  },
}

export const explorerApi = {
  async search(params?: { kind?: string; cluster?: string; namespace?: string; q?: string; limit?: number; offset?: number }): Promise<{ data: K8sResource[]; total: number }> {
    const res = await api.get<{ data: K8sResource[]; total: number }>('/explorer', params)
    return {
      data: res.data || [],
      total: res.total ?? (res.data ? res.data.length : 0),
    }
  },

  async sync(resource: K8sResource): Promise<{ status: string }> {
    return api.post<{ status: string }>('/explorer/sync', resource)
  },
}
