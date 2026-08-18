import { api } from './client'

// ==========================================
// 1. TENANCY & RBAC INTERFACES & API
// ==========================================

export interface Organization {
  id: string
  name: string
  tier: string
}

export interface Project {
  id: string
  orgId: string
  name: string
  envs: string[]
  workloads: number
}

export interface Member {
  id: string
  orgId: string
  user: string
  role: string
  scope: string
}

export type RBACMatrix = Record<string, Record<string, boolean>>

export interface TenancySummary {
  organizations: Organization[]
  projects: Project[]
  members: Member[]
  rbacMatrix: RBACMatrix
}

export const tenancyApi = {
  async getOrganizations(): Promise<Organization[]> {
    return api.get<Organization[]>('/tenancy/organizations')
  },

  async getProjects(): Promise<Project[]> {
    return api.get<Project[]>('/tenancy/projects')
  },

  async getMembers(): Promise<Member[]> {
    return api.get<Member[]>('/tenancy/members')
  },

  async getRBAC(): Promise<RBACMatrix> {
    return api.get<RBACMatrix>('/tenancy/rbac')
  },

  async getSummary(): Promise<TenancySummary> {
    return api.get<TenancySummary>('/tenancy/summary')
  },

  async createOrganization(org: Organization): Promise<Organization> {
    return api.post<Organization>('/tenancy/organizations', org)
  },

  async createProject(project: Project): Promise<Project> {
    return api.post<Project>('/tenancy/projects', project)
  },
}

// ==========================================
// 2. AI PROVIDERS & 20 MODELS CATALOG
// ==========================================

export interface AIProvider {
  name: string
  type: string
  model: string
  endpoint: string
  status: string
  latency?: string
  default: boolean
}

export interface CreateProviderPayload {
  name: string
  type: 'ollama' | 'openai' | 'vllm'
  endpoint: string
  model: string
  api_key?: string
  default?: boolean
}

export interface TestPromptPayload {
  provider?: string
  prompt: string
  system?: string
}

export interface TestPromptResult {
  content: string
  model: string
  prompt_tokens: number
  response_tokens: number
  duration_ms: number
}

export interface CatalogModelCard {
  id: string
  name: string
  vendor: 'OpenAI' | 'Anthropic' | 'Google' | 'DeepSeek' | 'Ollama / OSS' | string
  providerType: 'openai' | 'ollama' | 'vllm' | string
  contextWindow: string
  latencyTier: 'Ultra-Fast' | 'Fast' | 'Standard' | 'Reasoning Heavy' | string
  capabilities: string[]
  description: string
  recommendedFor: string
  defaultEndpoint: string
}

export const aiApi = {
  async getProviders(): Promise<AIProvider[]> {
    return api.get<AIProvider[]>('/ai/providers')
  },

  async getProvider(name: string): Promise<AIProvider> {
    return api.get<AIProvider>(`/ai/providers/${name}`)
  },

  async addProvider(payload: CreateProviderPayload): Promise<AIProvider> {
    return api.post<AIProvider>('/ai/providers', payload)
  },

  async deleteProvider(name: string): Promise<{ status: string; name: string }> {
    return api.delete<{ status: string; name: string }>(`/ai/providers/${name}`)
  },

  async healthCheckProvider(name: string): Promise<{ name: string; status: string; error?: string }> {
    return api.post<{ name: string; status: string; error?: string }>(`/ai/providers/${name}/health`)
  },

  async testPrompt(payload: TestPromptPayload): Promise<TestPromptResult> {
    return api.post<TestPromptResult>('/ai/test', payload)
  },
}

// ==========================================
// 3. CHANGE MANAGEMENT & MAINTENANCE WINDOWS
// ==========================================

export type ChangeStatus = 'pending' | 'approved' | 'rejected' | 'deployed'
export type ChangeType = 'standard' | 'emergency'

export interface ChangeRequest {
  id: string
  title: string
  description: string
  type: ChangeType
  status: ChangeStatus
  requester: string
  approver?: string
  cluster: string
  namespace: string
  resource: string
  scheduled_at?: string
  approved_at?: string
  created_at: string
  updated_at: string
}

export interface MaintenanceWindow {
  id: string
  title: string
  cluster: string
  start_at: string
  end_at: string
  active: boolean
  created_at: string
}

export const changesApi = {
  async getChanges(params?: { status?: string; limit?: number; offset?: number }): Promise<{ data: ChangeRequest[]; total: number }> {
    const res = await api.get<{ data: ChangeRequest[]; total: number }>('/changes', params)
    return res || { data: [], total: 0 }
  },

  async createChange(data: Partial<ChangeRequest>): Promise<ChangeRequest> {
    return api.post<ChangeRequest>('/changes', data)
  },

  async approveChange(id: string): Promise<{ status: string }> {
    return api.put<{ status: string }>(`/changes/${id}/approve`)
  },

  async rejectChange(id: string): Promise<{ status: string }> {
    return api.put<{ status: string }>(`/changes/${id}/reject`)
  },
}

// ==========================================
// 4. ALERTS & NOTIFICATION CHANNELS
// ==========================================

export interface ChannelConfig {
  webhook_url?: string
  channel?: string
  recipients?: string[]
  endpoint?: string
  chat_id?: string
  [key: string]: unknown
}

export interface AlertChannel {
  ID: string
  TenantID?: string
  Name: string
  Type: string
  Config: ChannelConfig
  Enabled: boolean
  CreatedAt?: string
  UpdatedAt?: string
}

export interface AlertRule {
  ID: string
  TenantID?: string
  Name: string
  Description: string
  MetricName: string
  Condition: string
  Threshold: number
  DurationSeconds: number
  Severity: 'critical' | 'high' | 'medium' | 'low' | string
  ChannelIDs: string[]
  Enabled: boolean
  CreatedAt?: string
  UpdatedAt?: string
}

export interface AlertRuleInput extends Partial<AlertRule> {
  name?: string
  description?: string
  metric_name?: string
  condition?: string
  threshold?: number
  duration_seconds?: number
  severity?: 'critical' | 'high' | 'medium' | 'low' | string
  channel_ids?: string[]
  enabled?: boolean
}

export interface AlertHistory {
  ID: string
  TenantID?: string
  RuleID: string
  Status: string
  Value: number
  Message: string
  AcknowledgedBy?: string
  CreatedAt: string
  UpdatedAt?: string
}

export const alertsApi = {
  async getChannels(): Promise<AlertChannel[]> {
    const res = await api.get<{ data: AlertChannel[] }>('/alerts/channels')
    return res?.data || []
  },

  async createChannel(data: { name: string; type: string; config: ChannelConfig; enabled: boolean }): Promise<AlertChannel> {
    return api.post<AlertChannel>('/alerts/channels', data)
  },

  async getRules(): Promise<AlertRule[]> {
    const res = await api.get<{ data: AlertRule[] }>('/alerts/rules')
    return res?.data || []
  },

  async createRule(data: AlertRuleInput): Promise<AlertRule> {
    return api.post<AlertRule>('/alerts/rules', {
      name: data.Name || data.name,
      description: data.Description || data.description,
      metric_name: data.MetricName || data.metric_name,
      condition: data.Condition || data.condition,
      threshold: data.Threshold || data.threshold,
      duration_seconds: data.DurationSeconds || data.duration_seconds,
      severity: data.Severity || data.severity,
      channel_ids: data.ChannelIDs || data.channel_ids || [],
      enabled: data.Enabled !== undefined ? data.Enabled : data.enabled !== undefined ? data.enabled : true,
    })
  },

  async updateRule(id: string, data: AlertRuleInput): Promise<AlertRule> {
    return api.put<AlertRule>(`/alerts/rules/${id}`, {
      name: data.Name || data.name,
      description: data.Description || data.description,
      metric_name: data.MetricName || data.metric_name,
      condition: data.Condition || data.condition,
      threshold: data.Threshold || data.threshold,
      duration_seconds: data.DurationSeconds || data.duration_seconds,
      severity: data.Severity || data.severity,
      channel_ids: data.ChannelIDs || data.channel_ids || [],
      enabled: data.Enabled !== undefined ? data.Enabled : data.enabled !== undefined ? data.enabled : true,
    })
  },

  async deleteRule(id: string): Promise<void> {
    return api.delete<void>(`/alerts/rules/${id}`)
  },

  async getHistory(): Promise<AlertHistory[]> {
    const res = await api.get<{ data: AlertHistory[] }>('/alerts/history')
    return res?.data || []
  },

  async acknowledgeAlert(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/alerts/history/${id}/acknowledge`)
  },
}

// ==========================================
// 5. REPORTS CENTER
// ==========================================

export interface Report {
  id: string
  title: string
  type: 'operational' | 'security' | 'compliance' | 'cost' | 'incident'
  format: 'pdf' | 'excel' | 'csv'
  status: 'pending' | 'generating' | 'completed' | 'failed'
  file_url?: string
  created_by: string
  created_at: string
  expires_at: string
}

export const reportsApi = {
  async getReports(params?: { limit?: number; offset?: number }): Promise<{ data: Report[]; total: number }> {
    const res = await api.get<{ data: Report[]; total: number }>('/reports-center', params)
    return res || { data: [], total: 0 }
  },

  async generateReport(data: { title: string; type: string; format: string }): Promise<Report> {
    return api.post<Report>('/reports-center', data)
  },

  async deleteReport(id: string): Promise<{ status: string }> {
    return api.delete<{ status: string }>(`/reports-center/${id}`)
  },
}
