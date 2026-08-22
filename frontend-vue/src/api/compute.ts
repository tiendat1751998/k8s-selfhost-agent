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
  indicator_type: 'availability' | 'latency' | 'error_rate' | 'cache_hit_rate' | string
  window: string
  query?: string
  alert_threshold?: number
  created_at: string
  updated_at?: string
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

export interface CreateSLOPayload {
  service: string
  target: number
  indicator_type: string
  window: string
  query?: string
  alert_threshold?: number
}

export interface UpdateSLOPayload {
  service?: string
  target?: number
  indicator_type?: string
  window?: string
  query?: string
  alert_threshold?: number
}

// ==========================================
// 4. Multi-Cluster Fleet Interfaces
// ==========================================

export type {
  Cluster,
  NodePoolInfo,
  ClusterDiscoveryData,
  ClusterHealthResponse,
  SwarmClusterInfo,
  ImportClusterPayload,
} from './fleet'
export { fleetApi } from './fleet'

// ==========================================
// 5. Workload Deployments Interfaces
// ==========================================

export interface DeploymentApp {
  id?: string
  rawId?: string
  name: string
  team: string
  env: string
  image: string
  target: string
  namespace: string
  type: 'kubernetes' | 'swarm' | 'docker' | 'container' | string
  replicas: number
  status: 'healthy' | 'degraded' | 'down' | string
  cpu: string
  memory: string
  port: number
  netType?: string
  volume?: string
  // Strategy & Rollout orchestration
  strategy?: 'RollingUpdate' | 'Canary' | 'BlueGreen' | 'Recreate' | string
  canaryWeight?: number
  canaryVersion?: string
  canarySteps?: number[]
  blueGreenActive?: 'blue' | 'green'
  blueVersion?: string
  greenVersion?: string
  revision?: number
  availableReplicas?: number
  updatedReplicas?: number
  readyReplicas?: number
  envVars?: Record<string, string>
  ingressHost?: string
  tlsEnabled?: boolean
  storageClass?: string
  storageSize?: string
  mountPath?: string
  livenessProbe?: string
  readinessProbe?: string
  paused?: boolean
  node?: string
  created?: string
}

export interface DeploymentTemplate {
  name: string
  category: 'web' | 'data' | 'system' | string
  version: string
  desc: string
  ports: number
  cpu: string
  mem: string
  strategy?: string
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

export interface RollbackDeploymentPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  revision?: number
}

export interface CanaryWeightPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  weight: number
}

export interface BlueGreenCutoverPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  targetColor: 'blue' | 'green'
}

export interface PauseResumePayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  action: 'pause' | 'resume'
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

export interface TestHostResponse {
  status: string
  latency_ms: number
  message?: string
  agent_info?: AgentInfo
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
// ==========================================
// API Normalization Helpers
// ==========================================

export function normalizeIncident(raw: any): Incident {
  if (!raw) return raw
  return {
    id: raw.id || raw.ID || '',
    cluster_name: raw.cluster_name || raw.ClusterName || 'default',
    namespace: raw.namespace || raw.Namespace || 'default',
    pod_name: raw.pod_name || raw.PodName || 'workload',
    type: raw.type || raw.Type || 'Incident',
    status: (raw.status || raw.Status || 'detected').toLowerCase() as IncidentStatus,
    severity: (raw.severity || raw.Severity || 'medium').toLowerCase() as IncidentSeverity,
    message: raw.message || raw.Message || '',
    raw_data: raw.raw_data || raw.RawData || {},
    created_at: raw.created_at || raw.CreatedAt || new Date().toISOString(),
    updated_at: raw.updated_at || raw.UpdatedAt || new Date().toISOString(),
    resolved_at: raw.resolved_at || raw.ResolvedAt,
  }
}

export function normalizeReport(raw: any): RCAReport {
  if (!raw) return raw
  return {
    id: raw.id || raw.ID || '',
    incident_id: raw.incident_id || raw.IncidentID || '',
    root_cause: raw.root_cause || raw.RootCause || '',
    evidence: raw.evidence || raw.Evidence || [],
    confidence: typeof raw.confidence === 'number' ? raw.confidence : (typeof raw.Confidence === 'number' ? raw.Confidence : 0.95),
    risk_level: ((raw.risk_level || raw.RiskLevel || 'medium').toLowerCase()) as RCAReport['risk_level'],
    remediation: raw.remediation || raw.Remediation || '',
    rollback_plan: raw.rollback_plan || raw.RollbackPlan || '',
    llm_model: raw.llm_model || raw.LLMModel || 'Claude 3.5 Sonnet / Multi-Agent',
    prompt_tokens: raw.prompt_tokens || raw.PromptTokens,
    response_tokens: raw.response_tokens || raw.ResponseTokens,
    created_at: raw.created_at || raw.CreatedAt || new Date().toISOString(),
  }
}

export function normalizePR(raw: any): PullRequest {
  if (!raw) return raw
  return {
    id: raw.id || raw.ID || '',
    incident_id: raw.incident_id || raw.IncidentID || '',
    provider: raw.provider || raw.Provider || 'github',
    repo_url: raw.repo_url || raw.RepoURL || '',
    branch: raw.branch || raw.Branch || '',
    base_branch: raw.base_branch || raw.BaseBranch || 'main',
    title: raw.title || raw.Title || '',
    description: raw.description || raw.Description || '',
    pr_url: raw.pr_url || raw.PRURL || '',
    pr_number: raw.pr_number || raw.PRNumber || 104,
    status: ((raw.status || raw.Status || 'open').toLowerCase()) as PRStatus,
    files_changed: raw.files_changed || raw.FilesChanged,
    created_at: raw.created_at || raw.CreatedAt || new Date().toISOString(),
    updated_at: raw.updated_at || raw.UpdatedAt || new Date().toISOString(),
    merged_at: raw.merged_at || raw.MergedAt,
  }
}

// ==========================================
// API Clients Export
// ==========================================

export const incidentsApi = {
  async list(params?: { namespace?: string; cluster?: string; status?: string; type?: string; severity?: string; limit?: number; offset?: number }): Promise<IncidentListResponse> {
    const res = await api.get<IncidentListResponse | { data: any[]; total: number }>('/incidents', params)
    const rawList = res.data || []
    const normalized = rawList.map(normalizeIncident)
    return {
      data: normalized,
      total: res.total ?? normalized.length,
      limit: (res as IncidentListResponse).limit ?? 50,
      offset: (res as IncidentListResponse).offset ?? 0,
    }
  },

  async get(id: string): Promise<Incident> {
    const raw = await api.get<any>(`/incidents/${id}`)
    return normalizeIncident(raw)
  },

  async getReport(id: string): Promise<RCAReport> {
    const raw = await api.get<any>(`/incidents/${id}/report`)
    return normalizeReport(raw)
  },

  async getPR(id: string): Promise<PullRequest> {
    const raw = await api.get<any>(`/incidents/${id}/pr`)
    return normalizePR(raw)
  },

  async analyze(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/incidents/${id}/analyze`)
  },

  async simulate(body?: { scenario?: string; pod_name?: string; namespace?: string }): Promise<Incident> {
    const raw = await api.post<any>('/incidents/simulate', body)
    return normalizeIncident(raw)
  },
}

export const reportsApi = {
  async list(params?: { limit?: number; offset?: number }): Promise<ReportListResponse> {
    const res = await api.get<ReportListResponse | { data: any[]; total: number }>('/reports', params)
    const rawList = res.data || []
    const normalized = rawList.map(normalizeReport)
    return {
      data: normalized,
      total: res.total ?? normalized.length,
      limit: (res as ReportListResponse).limit ?? 50,
      offset: (res as ReportListResponse).offset ?? 0,
    }
  },

  async get(id: string): Promise<RCAReport> {
    const raw = await api.get<any>(`/reports/${id}`)
    return normalizeReport(raw)
  },
}

export const prsApi = {
  async list(params?: { status?: string; limit?: number; offset?: number }): Promise<PRListResponse> {
    const res = await api.get<PRListResponse | { data: any[]; total: number }>('/prs', params)
    const rawList = res.data || []
    const normalized = rawList.map(normalizePR)
    return {
      data: normalized,
      total: res.total ?? normalized.length,
      limit: (res as PRListResponse).limit ?? 50,
      offset: (res as PRListResponse).offset ?? 0,
    }
  },

  async get(id: string): Promise<PullRequest> {
    const raw = await api.get<any>(`/prs/${id}`)
    return normalizePR(raw)
  },

  async create(payload: CreatePRPayload): Promise<PullRequest> {
    const raw = await api.post<any>('/prs', payload)
    return normalizePR(raw)
  },

  async merge(id: string): Promise<PullRequest> {
    const raw = await api.post<any>(`/prs/${id}/merge`)
    return normalizePR(raw)
  },

  async close(id: string): Promise<PullRequest> {
    const raw = await api.post<any>(`/prs/${id}/close`)
    return normalizePR(raw)
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
    const res = await api.get<ApiResponse<SLODefinition[]> | SLODefinition[]>('/observability/slo/definitions')
    if (Array.isArray(res)) return res
    return res.data || []
  },

  async getDefinition(id: string): Promise<SLODefinition> {
    const res = await api.get<ApiResponse<SLODefinition> | SLODefinition>(`/observability/slo/definitions/${id}`)
    if (res && 'data' in res && res.data) return res.data
    return res as SLODefinition
  },

  async listSnapshots(window?: string): Promise<SLOSnapshot[]> {
    const params: Record<string, string> = {}
    if (window) params.window = window
    const res = await api.get<ApiResponse<SLOSnapshot[]> | SLOSnapshot[]>('/observability/slo', params)
    if (Array.isArray(res)) return res
    return res.data || []
  },

  async createDefinition(payload: CreateSLOPayload): Promise<SLODefinition> {
    const res = await api.post<ApiResponse<SLODefinition> | SLODefinition>('/observability/slo/definitions', payload)
    if (res && 'data' in res && res.data) return res.data
    return res as SLODefinition
  },

  async updateDefinition(id: string, payload: UpdateSLOPayload): Promise<SLODefinition> {
    const res = await api.put<ApiResponse<SLODefinition> | SLODefinition>(`/observability/slo/definitions/${id}`, payload)
    if (res && 'data' in res && res.data) return res.data
    return res as SLODefinition
  },

  async deleteDefinition(id: string): Promise<void> {
    await api.delete(`/observability/slo/definitions/${id}`)
  },

  async triggerBurnAlert(id: string): Promise<{ success: boolean; message: string; snapshot?: SLOSnapshot }> {
    const res = await api.post<{ success: boolean; message: string; snapshot?: SLOSnapshot }>(`/observability/slo/definitions/${id}/trigger-alert`)
    return res
  },
}


export const deploymentsApi = {
  async list(): Promise<DeploymentApp[]> {
    try {
      const res = await api.get<ApiResponse<DeploymentApp[]> | DeploymentApp[] | { data: DeploymentApp[] }>('/deployments')
      let list: DeploymentApp[] = []
      if (Array.isArray(res)) {
        list = res
      } else if (res && 'data' in res && Array.isArray(res.data)) {
        list = res.data
      }
      return list.map(d => ({
        ...d,
        strategy: d.strategy || 'RollingUpdate',
        canaryWeight: d.canaryWeight !== undefined ? d.canaryWeight : 0,
        canaryVersion: d.canaryVersion,
        blueGreenActive: d.blueGreenActive,
        blueVersion: d.blueVersion,
        greenVersion: d.greenVersion,
        revision: d.revision || 1,
        availableReplicas: d.availableReplicas !== undefined ? d.availableReplicas : d.replicas,
        updatedReplicas: d.updatedReplicas !== undefined ? d.updatedReplicas : d.replicas,
        readyReplicas: d.readyReplicas !== undefined ? d.readyReplicas : (d.status === 'healthy' ? d.replicas : 0),
      }))
    } catch {
      return []
    }
  },

  async listTemplates(): Promise<DeploymentTemplate[]> {
    try {
      const res = await api.get<ApiResponse<DeploymentTemplate[]> | DeploymentTemplate[] | { data: DeploymentTemplate[] }>('/deployments/templates')
      if (Array.isArray(res)) return res
      if (res && 'data' in res && Array.isArray(res.data)) return res.data
    } catch {
      // Return empty if endpoint fails
    }
    return []
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

  async rollback(payload: RollbackDeploymentPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/rollback', payload)
  },

  async setCanaryWeight(payload: CanaryWeightPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/canary', payload)
  },

  async cutoverBlueGreen(payload: BlueGreenCutoverPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/bluegreen', payload)
  },

  async pauseResume(payload: PauseResumePayload): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/deployments/${payload.action}`, payload)
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

// ==========================================
// 12. Real-Time Throughput (TPS) Metrics
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

