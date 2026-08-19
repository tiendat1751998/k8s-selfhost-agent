import { api } from './client'

export interface NodeMetrics {
  node_id: string
  node_name: string
  role: string // manager, worker
  status: string // ready, down
  cpu_percent: number
  memory_used: number
  memory_total: number
  memory_percent: number
  disk_used: number
  disk_total: number
  disk_percent: number
  network_rx_bytes: number
  network_tx_bytes: number
  container_count: number
  running_count: number
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
