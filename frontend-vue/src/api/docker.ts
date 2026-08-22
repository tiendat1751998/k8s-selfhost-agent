import { api, type ApiResponse } from './client'

export interface DockerContainer {
  id: string
  name: string
  image: string
  status: string
  state: string
  created: string
}

export interface DockerNode {
  id: string
  name: string
  hostname?: string
  role: 'manager' | 'worker' | string
  availability: 'active' | 'pause' | 'drain' | string
  status: 'ready' | 'down' | 'disconnected' | string
  version?: string
  engine_version?: string
  cpus?: number
  memory?: number
  ip?: string
  labels?: Record<string, string>
  joined_at?: string
  updated_at?: string
}

export interface SwarmInfo {
  id: string
  node_count: number
  manager_count: number
  worker_count: number
  created_at: string
  is_manager: boolean
}

export interface SwarmTokens {
  worker_token: string
  manager_token: string
  manager_addr: string
  worker_token_masked?: string
  manager_token_masked?: string
  expires_in_seconds?: number
}

export interface NodeDetails {
  id: string
  hostname: string
  role: 'manager' | 'worker' | string
  availability: 'active' | 'pause' | 'drain' | string
  status: 'ready' | 'down' | 'disconnected' | string
  engine_version: string
  os: string
  architecture: string
  cpus: number
  memory: number
  ip: string
  labels: Record<string, string>
  joined_at: string
}

export interface DockerService {
  id: string
  name: string
  image: string
  replicas: number
  ports: string[]
  updated_at: string
}

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

export interface TestHostResponse {
  status: string
  latency_ms: number
  message?: string
  agent_info?: AgentInfo
}

export const dockerApi = {
  async listContainers(): Promise<DockerContainer[]> {
    const res = await api.get<ApiResponse<DockerContainer[]> | DockerContainer[]>('/docker/containers')
    return Array.isArray(res) ? res : (res.data || [])
  },

  async getContainers(): Promise<DockerContainer[]> {
    return this.listContainers()
  },

  async listNodes(): Promise<DockerNode[]> {
    const res = await api.get<ApiResponse<DockerNode[]> | DockerNode[]>('/docker/nodes')
    return Array.isArray(res) ? res : (res.data || [])
  },

  async getNodes(): Promise<DockerNode[]> {
    return this.listNodes()
  },

  async listServices(): Promise<DockerService[]> {
    const res = await api.get<ApiResponse<DockerService[]> | DockerService[]>('/docker/services')
    return Array.isArray(res) ? res : (res.data || [])
  },

  async getServices(): Promise<DockerService[]> {
    return this.listServices()
  },

  async scaleService(id: string, replicas: number): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/docker/services/${id}/scale`, { replicas })
  },

  async drainNode(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/docker/nodes/${id}/drain`)
  },

  async activateNode(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/docker/nodes/${id}/activate`)
  },

  async removeNode(id: string): Promise<{ status: string }> {
    return api.delete<{ status: string }>(`/docker/nodes/${id}`)
  },

  async getSwarmInfo(): Promise<SwarmInfo> {
    const res = await api.get<ApiResponse<SwarmInfo> | SwarmInfo>('/docker/swarm')
    return (res as ApiResponse<SwarmInfo>).data || (res as SwarmInfo)
  },

  async getSwarmTokens(password: string): Promise<SwarmTokens> {
    const res = await api.post<ApiResponse<SwarmTokens> | SwarmTokens>('/docker/swarm/tokens', { password })
    return (res as ApiResponse<SwarmTokens>).data || (res as SwarmTokens)
  },

  async getNodeDetails(id: string): Promise<NodeDetails> {
    const res = await api.get<ApiResponse<NodeDetails> | NodeDetails>(`/docker/nodes/${id}`)
    return (res as ApiResponse<NodeDetails>).data || (res as NodeDetails)
  },

  async listHosts(): Promise<ComputeHost[]> {
    const res = await api.get<ApiResponse<ComputeHost[]> | ComputeHost[]>('/docker/hosts')
    return Array.isArray(res) ? res : (res.data || [])
  },

  async registerHost(data: CreateHostRequest): Promise<ComputeHost> {
    const payload = {
      ...data,
      tls_ca: data.tls_ca || data.ca_cert || '',
      tls_cert: data.tls_cert || data.client_cert || '',
      tls_key: data.tls_key || data.client_key || '',
    }
    const res = await api.post<ApiResponse<ComputeHost> | ComputeHost>('/docker/hosts', payload)
    return (res as ApiResponse<ComputeHost>).data || (res as ComputeHost)
  },

  async createHost(data: CreateHostRequest): Promise<ComputeHost> {
    return this.registerHost(data)
  },

  async updateHost(id: string, data: UpdateHostRequest): Promise<ComputeHost> {
    const payload = {
      ...data,
      tls_ca: data.tls_ca !== undefined ? data.tls_ca : (data.ca_cert || undefined),
      tls_cert: data.tls_cert !== undefined ? data.tls_cert : (data.client_cert || undefined),
      tls_key: data.tls_key !== undefined ? data.tls_key : (data.client_key || undefined),
    }
    const res = await api.put<ApiResponse<ComputeHost> | ComputeHost>(`/docker/hosts/${id}`, payload)
    return (res as ApiResponse<ComputeHost>).data || (res as ComputeHost)
  },

  async deleteHost(id: string): Promise<{ status: string }> {
    return api.delete<{ status: string }>(`/docker/hosts/${id}`)
  },

  async removeHost(id: string): Promise<{ status: string }> {
    return this.deleteHost(id)
  },

  async testHost(id: string): Promise<TestHostResponse> {
    return api.post<TestHostResponse>(`/docker/hosts/${id}/test`)
  },

  async toggleContainer(id: string, action: 'start' | 'stop'): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/docker/containers/${id}/toggle`, { action })
  },

  async getLogs(id: string, type?: string): Promise<{ logs: string }> {
    const params: Record<string, string> = { id }
    if (type) params.type = type
    return api.get<{ logs: string }>('/docker/logs', params)
  },
}