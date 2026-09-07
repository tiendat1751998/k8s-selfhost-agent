import { api, type ApiResponse } from './client'

export interface EssentialItem {
  id: 'metrics-server' | 'local-storage' | 'agent-daemonset' | 'master-taints' | string
  name: string
  status: 'installed' | 'running' | 'not_found' | 'pending' | 'failed' | string
  description: string
  installed?: boolean
  message?: string
  lastChecked?: string
}

export interface ClusterEssentialsStatus {
  clusterId: string
  essentials: EssentialItem[]
  allReady?: boolean
  lastChecked?: string
}

export interface BootstrapRequest {
  items: string[]
  force?: boolean
}

export interface BootstrapStepResult {
  item: string
  status: 'success' | 'failed' | 'in_progress' | 'skipped'
  output?: string
  error?: string
}

export interface BootstrapResult {
  clusterId: string
  success: boolean
  message?: string
  steps?: BootstrapStepResult[]
  logs?: string[]
}

export interface PodMetricItem {
  namespace: string
  podName: string
  cpuMillicores: number
  memoryBytes: number
  cpuLimit?: number
  memoryLimit?: number
  timestamp?: string
}

export interface PodMetricsResponse {
  clusterId: string
  metrics: PodMetricItem[]
}

export const clusterApi = {
  async getEssentialsStatus(clusterId: string): Promise<ClusterEssentialsStatus> {
    const res = await api.get<ApiResponse<ClusterEssentialsStatus> | ClusterEssentialsStatus>(
      '/k8s/' + encodeURIComponent(clusterId) + '/essentials'
    )
    if (res && typeof res === 'object' && 'data' in res && res.data) {
      return res.data
    }
    return res as ClusterEssentialsStatus
  },

  async executeBootstrap(clusterId: string, req: BootstrapRequest): Promise<BootstrapResult> {
    const res = await api.post<ApiResponse<BootstrapResult> | BootstrapResult>(
      '/k8s/' + encodeURIComponent(clusterId) + '/bootstrap',
      req
    )
    if (res && typeof res === 'object' && 'data' in res && res.data) {
      return res.data
    }
    return res as BootstrapResult
  },

  async getPodMetrics(clusterId: string): Promise<any> {
    const res = await api.get<any>(
      '/k8s/' + encodeURIComponent(clusterId) + '/metrics/pods'
    )
    if (res && typeof res === 'object' && 'data' in res && res.data) {
      return res.data
    }
    return res
  },
}
