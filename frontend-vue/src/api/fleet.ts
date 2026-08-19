import { api, type ApiResponse } from './client'

export interface NodePoolInfo {
  name: string
  instance?: string
  zone?: string
  count?: number
}

export interface ClusterDiscoveryData {
  namespaces?: string[]
  crd_count?: number
  node_pools?: NodePoolInfo[]
  api_server_latency?: string
  last_sync?: string
  raw?: Record<string, unknown>
  [key: string]: unknown
}

export interface Cluster {
  id: string
  name: string
  group: string
  region: string
  provider: string
  status: 'active' | 'offline' | 'upgrading' | 'maintenance' | string
  version?: string
  nodes?: number
  encrypted_token?: string
  import_method?: string
  kubeconfig_hash?: string
  last_health_check?: string
  health_status: 'healthy' | 'degraded' | 'unhealthy' | 'unknown' | string
  discovered_resources?: ClusterDiscoveryData | Record<string, unknown>
  tenant_id?: string
  created_at?: string
  updated_at?: string
}

export interface ClusterHealthResponse {
  health_status: string
  last_health_check?: string
}

export interface SwarmClusterInfo {
  id: string
  node_count: number
  manager_count: number
  worker_count: number
  created_at: string
  is_manager: boolean
}

export interface ImportClusterPayload {
  id?: string
  name: string
  group: string
  region: string
  provider: string
  kubeconfig: File
}

export const fleetApi = {
  /**
   * Fetch all registered Kubernetes clusters in the fleet.
   */
  async list(): Promise<Cluster[]> {
    try {
      const res = await api.get<ApiResponse<Cluster[]> | Cluster[]>('/fleet')
      if (Array.isArray(res)) return res
      if (res && Array.isArray((res as ApiResponse<Cluster[]>).data)) {
        return (res as ApiResponse<Cluster[]>).data
      }
      return []
    } catch {
      const res = await api.get<ApiResponse<Cluster[]> | Cluster[]>('/fleet/clusters')
      if (Array.isArray(res)) return res
      if (res && Array.isArray((res as ApiResponse<Cluster[]>).data)) {
        return (res as ApiResponse<Cluster[]>).data
      }
      return []
    }
  },

  /**
   * Fetch active Docker Swarm cluster status.
   * Returns null if Swarm is inactive or unavailable.
   */
  async getSwarm(): Promise<SwarmClusterInfo | null> {
    try {
      const res = await api.get<ApiResponse<SwarmClusterInfo> | SwarmClusterInfo>('/docker/swarm')
      const data = (res && typeof res === 'object' && 'data' in res && res.data)
        ? res.data
        : (res as SwarmClusterInfo)
      if (data && (data.id || data.node_count > 0)) {
        return data
      }
      return null
    } catch {
      return null
    }
  },

  /**
   * Register a new cluster without kubeconfig file.
   */
  async register(cluster: Partial<Cluster>): Promise<Cluster> {
    return api.post<Cluster>('/fleet', cluster)
  },

  /**
   * Remove a cluster from the fleet by ID.
   */
  async remove(id: string): Promise<{ status: string }> {
    return api.delete<{ status: string }>(`/fleet/${id}`)
  },

  /**
   * Request control-plane upgrade for a cluster.
   */
  async upgrade(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/fleet/${id}/actions/upgrade`)
  },

  /**
   * Import an external Kubernetes cluster using a kubeconfig file.
   */
  async importCluster(formData: FormData): Promise<Cluster> {
    return api.post<Cluster>('/fleet/import', formData)
  },

  /**
   * Fetch real-time health status for a cluster.
   */
  async getHealth(id: string): Promise<ClusterHealthResponse> {
    return api.get<ClusterHealthResponse>(`/fleet/${id}/health`)
  },

  /**
   * Trigger discovery of namespaces, CRDs, and node pools on a cluster.
   */
  async discover(id: string): Promise<ClusterDiscoveryData | null> {
    const res = await api.post<ClusterDiscoveryData>(`/fleet/${id}/discover`)
    return res || null
  },
}
