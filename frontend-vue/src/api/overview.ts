import { api } from './client'

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

export interface NodeMetrics {
  node_id: string
  node_name: string
  role: string // manager, worker
  status: string // ready, down
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
  container_count: number
  running_count: number
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

export interface ContainerMetrics {
  container_id: string
  container_name: string
  node_id: string
  image: string
  state: string
  cpu_percent: number
  memory_used: number
  memory_limit: number
  memory_percent: number
  network_rx: number
  network_tx: number
}

export interface MetricAlert {
  node_id: string
  node_name: string
  type: string // cpu_high, memory_high, disk_high, node_down
  message: string
  value: number
  threshold: number
}

export interface SystemOverview {
  nodes: NodeMetrics[]
  containers: ContainerMetrics[]
  total_nodes: number
  healthy_nodes: number
  total_containers: number
  running_containers: number
  total_cpu_percent: number
  total_mem_percent: number
  total_disk_percent: number
  requests_per_sec: number
  alerts: MetricAlert[]
  collected_at: string
}

export const overviewApi = {
  async getOverview(): Promise<SystemOverview> {
    const res = await api.get<SystemOverview | { data: SystemOverview }>('/overview')
    if (res && typeof res === 'object' && 'data' in res && res.data) {
      return res.data
    }
    return res as SystemOverview
  },
  async getNodeMetrics(): Promise<NodeMetrics[]> {
    const res = await api.get<NodeMetrics[] | { data: NodeMetrics[] }>('/overview/nodes')
    if (res && typeof res === 'object' && 'data' in res && Array.isArray(res.data)) {
      return res.data
    }
    return (res as NodeMetrics[]) || []
  },
  async getAlerts(): Promise<MetricAlert[]> {
    const res = await api.get<MetricAlert[] | { data: MetricAlert[] }>('/overview/alerts')
    if (res && typeof res === 'object' && 'data' in res && Array.isArray(res.data)) {
      return res.data
    }
    return (res as MetricAlert[]) || []
  },
}

// ==========================================
// Real-Time Throughput (TPS) Metrics
// ==========================================

export interface TpsNetworkMetrics {
  total_rx_bytes_per_sec: number
  total_tx_bytes_per_sec: number
}

export interface TpsHttpMetrics {
  requests_per_sec: number
  active_connections: number
  total_requests: number
  error_rate: number
  avg_latency_ms: number
}

export interface TpsDatabaseMetrics {
  transactions_per_sec: number
  reads_per_sec: number
  writes_per_sec: number
  active_connections: number
  cache_hit_ratio: number
}

export interface TpsMessagingMetrics {
  in_msgs_per_sec: number
  out_msgs_per_sec: number
  in_bytes_per_sec: number
  out_bytes_per_sec: number
  connections: number
}

export interface TpsNodeMetrics {
  node_name: string
  node_id: string
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  processes: number
}

export interface TpsServiceMetrics {
  service_name: string
  node_id?: string
  node_name?: string
  container_count: number
  cpu_percent: number
  memory_used_mb: number
  memory_percent: number
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  status: string
}

export interface TpsSnapshot {
  timestamp: string
  network: TpsNetworkMetrics
  http: TpsHttpMetrics
  database: TpsDatabaseMetrics
  messaging: TpsMessagingMetrics
  per_node: TpsNodeMetrics[]
  services?: TpsServiceMetrics[]
}

export const tpsApi = {
  async getSnapshot(): Promise<TpsSnapshot> {
    const res = await api.get<TpsSnapshot | { data: TpsSnapshot }>('/overview/tps')
    if (res && typeof res === 'object' && 'data' in res && res.data) {
      return res.data
    }
    return res as TpsSnapshot
  },
}

