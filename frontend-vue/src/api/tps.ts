import { api } from './client'
import type { Incident } from './incidents'

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
  total_rx_bytes?: number
  total_tx_bytes?: number
  requests_per_sec?: number
  error_rate?: number
  avg_latency_ms?: number
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

// ==========================================
// Node Historical Telemetry & Rollups
// ==========================================

export interface NodeMetricRollup {
  id: string
  tenant_id: string
  node_id: string
  node_name: string
  cpu_percent: number
  cpu_peak: number
  mem_used_bytes: number
  mem_total_bytes: number
  mem_percent: number
  disk_used_bytes: number
  disk_total_bytes: number
  disk_percent: number
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  process_count: number
  container_count: number
  status: string
  resolution: string
  recorded_at: string
}

export interface NodeHistoricalSummary {
  node_id: string
  node_name: string
  avg_cpu_percent: number
  peak_cpu_percent: number
  avg_mem_percent: number
  peak_mem_percent: number
  peak_rx_bytes_sec: number
  peak_tx_bytes_sec: number
  uptime_percent: number
  offline_count: number
  total_samples: number
  window_start: string
  window_end: string
}

export interface NodeHistoryResponse {
  node_id: string
  range: string
  resolution: string
  summary: NodeHistoricalSummary | null
  history: NodeMetricRollup[]
  incidents: Incident[]
}

function toIsoTime(val?: string): string | undefined {
  if (!val) return undefined
  try {
    const d = new Date(val)
    if (!isNaN(d.getTime())) return d.toISOString()
  } catch {}
  return val
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

export const nodeHistoryApi = {
  async getNodeHistory(nodeId: string, range = '24h', from?: string, to?: string): Promise<NodeHistoryResponse> {
    const isoFrom = toIsoTime(from) || from
    const isoTo = toIsoTime(to) || to
    let url = `/overview/nodes/${encodeURIComponent(nodeId)}/history?range=${encodeURIComponent(range)}`
    if (isoFrom) {
      url += `&from=${encodeURIComponent(isoFrom)}`
    }
    if (isoTo) {
      url += `&to=${encodeURIComponent(isoTo)}`
    }
    const res = await api.get<NodeHistoryResponse | { data: NodeHistoryResponse }>(url)
    if (res && typeof res === 'object' && 'data' in res && (res as any).data && Array.isArray((res as any).data.history)) {
      return (res as any).data as NodeHistoryResponse
    }
    return res as NodeHistoryResponse
  },
}
