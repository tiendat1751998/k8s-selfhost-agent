import { api, type ApiResponse } from './client'

// ==========================================
// 1. Incidents, Reports, PRs Interfaces
// ==========================================

export type IncidentStatus = 'detected' | 'analyzing' | 'remediating' | 'resolved' | 'failed'
export type IncidentSeverity = 'critical' | 'high' | 'medium' | 'low'
export type IncidentType = 'CrashLoopBackOff' | 'OOMKilled' | 'HighCPU' | 'HighMemory' | 'NodeNotReady' | 'PodPending' | 'ImagePullBackOff' | 'DeploymentReplicaMismatch' | 'NetworkFailure' | 'StorageExhaustion' | 'SLOBreach' | string

export interface Incident {
  id: string
  cluster_name: string
  namespace: string
  pod_name: string
  type: IncidentType
  status: IncidentStatus
  severity: IncidentSeverity
  message: string
  raw_data?: Record<string, string>
  created_at: string
  updated_at: string
  resolved_at?: string
}

export interface IncidentListResponse {
  data: Incident[]
  total: number
  limit: number
  offset: number
}

export interface RCAReport {
  id: string
  incident_id: string
  root_cause: string
  evidence: string[]
  confidence: number
  risk_level: 'critical' | 'high' | 'medium' | 'low'
  remediation: string
  rollback_plan: string
  llm_model?: string
  prompt_tokens?: number
  response_tokens?: number
  created_at: string
}

export interface ReportListResponse {
  data: RCAReport[]
  total: number
  limit: number
  offset: number
}

export type PRStatus = 'pending' | 'open' | 'merged' | 'closed' | 'failed'
export type FileAction = 'create' | 'modify' | 'delete'

export interface FileChange {
  path: string
  content: string
  action: FileAction
}

export interface PullRequest {
  id: string
  incident_id: string
  provider: 'github' | 'gitlab' | 'gitea' | string
  repo_url: string
  branch: string
  base_branch: string
  title: string
  description: string
  pr_url: string
  pr_number: number
  status: PRStatus
  files_changed?: FileChange[]
  created_at: string
  updated_at: string
  merged_at?: string
}

export interface PRListResponse {
  data: PullRequest[]
  total: number
  limit: number
  offset: number
}

export interface CreatePRPayload {
  incident_id?: string
  provider?: string
  repo_url?: string
  branch?: string
  base_branch?: string
  title: string
  description?: string
  files_changed?: FileChange[]
}

// ==========================================
// 2. Multi-Agent Framework Interfaces
// ==========================================

export interface ProjectState {
  id: string
  current_phase: string
  current_module: string
  current_feature: string
  current_task_id?: string
  current_subtask_id?: string
  repository_health: number
  technical_debt: number
  architecture_score: number
  quality_score: number
  updated_at: string
}

export type TaskStatus = 'pending' | 'inprogress' | 'success' | 'blocked' | 'failed'
export type SubtaskStatus = 'pending' | 'inprogress' | 'success' | 'failed'

export interface Subtask {
  id: string
  task_id: string
  title: string
  status: SubtaskStatus
  complexity: number
  exec_order: number
  completed_at?: string
}

export interface AgentTask {
  id: string
  phase: string
  module: string
  feature: string
  title: string
  description: string
  status: TaskStatus
  dependencies: string[]
  subtasks?: Subtask[]
  created_at: string
  updated_at: string
  completed_at?: string
}

export interface AgentExecution {
  id: string
  task_id: string
  agent_type: string
  status: 'running' | 'success' | 'failed'
  input?: string
  output?: string
  error_detail?: string
  created_at: string
  completed_at?: string
}

export interface CreateTaskPayload {
  phase: string
  module: string
  feature: string
  title: string
  description: string
  dependencies?: string[]
}

// ==========================================
// 3. Observability & SLO Interfaces
// ==========================================

export interface SLODefinition {
  id: string
  service: string
  target: number
  indicator_type: 'availability' | 'latency' | 'error_rate' | string
  window: string
  created_at: string
  updated_at: string
}

export interface SLOSnapshot {
  id: string
  slo_id: string
  service: string
  target: number
  actual: number
  burn_rate: number
  error_budget: number
  budget_status: 'healthy' | 'warning' | 'critical' | string
  recorded_at: string
}

// ==========================================
// 4. Multi-Cluster Fleet Interfaces
// ==========================================

export interface Cluster {
  id: string
  name: string
  group: string
  region: string
  provider: string
  status: 'active' | 'offline' | 'upgrading' | 'maintenance' | string
  version: string
  nodes: number
  encrypted_token?: string
  import_method?: string
  kubeconfig_hash?: string
  last_health_check?: string
  health_status: 'healthy' | 'degraded' | 'unhealthy' | 'unknown' | string
  discovered_resources?: Record<string, unknown>
  tenant_id?: string
  created_at: string
  updated_at: string
}

export interface NodePoolInfo {
  name: string
  instance: string
  zone: string
  count: number
}

export interface ClusterDiscoveryData {
  namespaces?: string[]
  node_pools?: NodePoolInfo[]
  [key: string]: unknown
}

export interface ClusterHealthResponse {
  health_status: string
  last_health_check?: string
}

// ==========================================
// 5. Workload Deployments Interfaces
// ==========================================

export interface DeploymentApp {
  name: string
  team: string
  env: string
  image: string
  target: string
  namespace: string
  type: 'kubernetes' | 'swarm' | string
  replicas: number
  status: 'healthy' | 'degraded' | 'down' | string
  cpu: string
  memory: string
  port: number
  netType?: string
  volume?: string
}

export interface DeploymentTemplate {
  name: string
  category: 'web' | 'data' | 'system' | string
  version: string
  desc: string
  ports: number
  cpu: string
  mem: string
}

export interface ScaleDeploymentPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  replicas: number
}

export interface RestartDeploymentPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
}

export interface DeleteDeploymentPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
}

// ==========================================
// 6. Promotions Pipeline Interfaces
// ==========================================

export type Environment = 'dev' | 'qa' | 'staging' | 'production'
export type PromotionStatus = 'pending' | 'approved' | 'promoting' | 'completed' | 'rejected' | 'failed'

export interface Promotion {
  id: string
  service: string
  version: string
  from_env: Environment
  to_env: Environment
  status: PromotionStatus
  requester: string
  approver?: string
  approved_at?: string
  completed_at?: string
  created_at: string
}

export interface CreatePromotionPayload {
  service: string
  version: string
  from_env: Environment
  to_env: Environment
  requester: string
}

// ==========================================
// 7. Docker & Swarm Interfaces
// ==========================================

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
  role: 'manager' | 'worker' | string
  availability: 'active' | 'pause' | 'drain' | string
  status: 'ready' | 'down' | string
  version: string
  updated_at: string
}

export interface DockerService {
  id: string
  name: string
  image: string
  replicas: number
  ports: string[]
  updated_at: string
}

// ==========================================
// 8. Resource Explorer Interfaces
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
// API Clients Export
// ==========================================

export const incidentsApi = {
  async list(params?: { namespace?: string; cluster?: string; status?: string; type?: string; severity?: string; limit?: number; offset?: number }): Promise<IncidentListResponse> {
    const res = await api.get<IncidentListResponse | { data: Incident[]; total: number }>('/incidents', params)
    return {
      data: res.data || [],
      total: res.total ?? (res.data ? res.data.length : 0),
      limit: (res as IncidentListResponse).limit ?? 50,
      offset: (res as IncidentListResponse).offset ?? 0,
    }
  },

  async get(id: string): Promise<Incident> {
    return api.get<Incident>(`/incidents/${id}`)
  },

  async getReport(id: string): Promise<RCAReport> {
    return api.get<RCAReport>(`/incidents/${id}/report`)
  },

  async getPR(id: string): Promise<PullRequest> {
    return api.get<PullRequest>(`/incidents/${id}/pr`)
  },

  async analyze(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/incidents/${id}/analyze`)
  },
}

export const reportsApi = {
  async list(params?: { limit?: number; offset?: number }): Promise<ReportListResponse> {
    const res = await api.get<ReportListResponse | { data: RCAReport[]; total: number }>('/reports', params)
    return {
      data: res.data || [],
      total: res.total ?? (res.data ? res.data.length : 0),
      limit: (res as ReportListResponse).limit ?? 50,
      offset: (res as ReportListResponse).offset ?? 0,
    }
  },

  async get(id: string): Promise<RCAReport> {
    return api.get<RCAReport>(`/reports/${id}`)
  },
}

export const prsApi = {
  async list(params?: { status?: string; limit?: number; offset?: number }): Promise<PRListResponse> {
    const res = await api.get<PRListResponse | { data: PullRequest[]; total: number }>('/prs', params)
    return {
      data: res.data || [],
      total: res.total ?? (res.data ? res.data.length : 0),
      limit: (res as PRListResponse).limit ?? 50,
      offset: (res as PRListResponse).offset ?? 0,
    }
  },

  async get(id: string): Promise<PullRequest> {
    return api.get<PullRequest>(`/prs/${id}`)
  },

  async create(payload: CreatePRPayload): Promise<PullRequest> {
    return api.post<PullRequest>('/prs', payload)
  },

  async merge(id: string): Promise<PullRequest> {
    return api.post<PullRequest>(`/prs/${id}/merge`)
  },

  async close(id: string): Promise<PullRequest> {
    return api.post<PullRequest>(`/prs/${id}/close`)
  },
}

export const agentsApi = {
  async getState(): Promise<ProjectState> {
    return api.get<ProjectState>('/agents/state')
  },

  async listTasks(): Promise<AgentTask[]> {
    const res = await api.get<ApiResponse<AgentTask[]>>('/agents/tasks')
    return res.data || []
  },

  async createTask(payload: CreateTaskPayload): Promise<AgentTask> {
    return api.post<AgentTask>('/agents/tasks', payload)
  },

  async listRuns(taskId?: string): Promise<AgentExecution[]> {
    const params: Record<string, string> = {}
    if (taskId) params.task_id = taskId
    const res = await api.get<ApiResponse<AgentExecution[]>>('/agents/runs', params)
    return res.data || []
  },
}

export const sloApi = {
  async listDefinitions(): Promise<SLODefinition[]> {
    const res = await api.get<ApiResponse<SLODefinition[]>>('/observability/slo/definitions')
    return res.data || []
  },

  async listSnapshots(): Promise<SLOSnapshot[]> {
    const res = await api.get<ApiResponse<SLOSnapshot[]>>('/observability/slo')
    return res.data || []
  },
}

export const fleetApi = {
  async list(): Promise<Cluster[]> {
    const res = await api.get<ApiResponse<Cluster[]>>('/fleet')
    return res.data || []
  },

  async register(cluster: Partial<Cluster>): Promise<Cluster> {
    return api.post<Cluster>('/fleet', cluster)
  },

  async remove(id: string): Promise<{ status: string }> {
    return api.delete<{ status: string }>(`/fleet/${id}`)
  },

  async upgrade(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/fleet/${id}/actions/upgrade`)
  },

  async importCluster(formData: FormData): Promise<Cluster> {
    return api.post<Cluster>('/fleet/import', formData)
  },

  async getHealth(id: string): Promise<ClusterHealthResponse> {
    return api.get<ClusterHealthResponse>(`/fleet/${id}/health`)
  },

  async discover(id: string): Promise<ClusterDiscoveryData> {
    return api.post<ClusterDiscoveryData>(`/fleet/${id}/discover`)
  },
}

export const deploymentsApi = {
  async list(): Promise<DeploymentApp[]> {
    const res = await api.get<ApiResponse<DeploymentApp[]>>('/deployments')
    return res.data || []
  },

  async listTemplates(): Promise<DeploymentTemplate[]> {
    const res = await api.get<ApiResponse<DeploymentTemplate[]>>('/deployments/templates')
    return res.data || []
  },

  async create(app: DeploymentApp): Promise<{ status: string; name: string }> {
    return api.post<{ status: string; name: string }>('/deployments', app)
  },

  async scale(payload: ScaleDeploymentPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/scale', payload)
  },

  async restart(payload: RestartDeploymentPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/restart', payload)
  },

  async delete(payload: DeleteDeploymentPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/delete', payload)
  },
}

export const promotionsApi = {
  async list(params?: { status?: string; limit?: number; offset?: number }): Promise<{ data: Promotion[]; total: number }> {
    const res = await api.get<{ data: Promotion[]; total: number }>('/promotions', params)
    return {
      data: res.data || [],
      total: res.total ?? (res.data ? res.data.length : 0),
    }
  },

  async create(payload: CreatePromotionPayload): Promise<Promotion> {
    return api.post<Promotion>('/promotions', payload)
  },

  async approve(id: string): Promise<{ status: string }> {
    return api.put<{ status: string }>(`/promotions/${id}/approve`)
  },

  async reject(id: string): Promise<{ status: string }> {
    return api.put<{ status: string }>(`/promotions/${id}/reject`)
  },

  async complete(id: string): Promise<{ status: string }> {
    return api.put<{ status: string }>(`/promotions/${id}/complete`)
  },
}

export const dockerApi = {
  async listContainers(): Promise<DockerContainer[]> {
    const res = await api.get<ApiResponse<DockerContainer[]>>('/docker/containers')
    return res.data || []
  },

  async listNodes(): Promise<DockerNode[]> {
    const res = await api.get<ApiResponse<DockerNode[]>>('/docker/nodes')
    return res.data || []
  },

  async listServices(): Promise<DockerService[]> {
    const res = await api.get<ApiResponse<DockerService[]>>('/docker/services')
    return res.data || []
  },

  async scaleService(id: string, replicas: number): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/docker/services/${id}/scale`, { replicas })
  },

  async drainNode(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/docker/nodes/${id}/drain`)
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
