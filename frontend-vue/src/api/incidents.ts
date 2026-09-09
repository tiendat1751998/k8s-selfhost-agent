import { api } from './client'

// ==========================================
// 1. Incidents, Reports, PRs Interfaces
// ==========================================

export type IncidentStatus = 'detected' | 'analyzing' | 'remediating' | 'resolved' | 'failed'
export type IncidentSeverity = 'critical' | 'high' | 'medium' | 'low'
export type IncidentType =
  | 'CrashLoopBackOff'
  | 'OOMKilled'
  | 'HighCPU'
  | 'HighMemory'
  | 'NodeNotReady'
  | 'PodPending'
  | 'ImagePullBackOff'
  | 'DeploymentReplicaMismatch'
  | 'NetworkFailure'
  | 'StorageExhaustion'
  | 'SLOBreach'
  | string

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

  async create(body: Partial<Incident>): Promise<Incident> {
    const raw = await api.post<any>('/incidents', body)
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

  async mitigate(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/incidents/${id}/mitigate`)
  },

  async resolve(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/incidents/${id}/resolve`)
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
