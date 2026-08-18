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
  vendor: 'OpenAI' | 'Anthropic' | 'Google' | 'DeepSeek' | 'Ollama / OSS'
  providerType: 'openai' | 'ollama' | 'vllm'
  contextWindow: string
  latencyTier: 'Ultra-Fast' | 'Fast' | 'Standard' | 'Reasoning Heavy'
  capabilities: string[]
  description: string
  recommendedFor: string
  defaultEndpoint: string
}

// 20 Enterprise Industry-Grade Models across 5 major AI ecosystems
export const CATALOG_MODELS: CatalogModelCard[] = [
  // OpenAI (4)
  {
    id: 'gpt-4o',
    name: 'GPT-4o Omnichannel',
    vendor: 'OpenAI',
    providerType: 'openai',
    contextWindow: '128k',
    latencyTier: 'Fast',
    capabilities: ['Vision', 'Code', 'Function Calling', 'JSON'],
    description: 'Flagship multimodal powerhouse with high throughput and low latency for cluster RCA.',
    recommendedFor: 'Autonomous Agent Orchestration & High-Speed Diagnostic Analysis',
    defaultEndpoint: 'https://api.openai.com/v1',
  },
  {
    id: 'gpt-4o-mini',
    name: 'GPT-4o Mini',
    vendor: 'OpenAI',
    providerType: 'openai',
    contextWindow: '128k',
    latencyTier: 'Ultra-Fast',
    capabilities: ['Code', 'Fast Triage', 'JSON'],
    description: 'Ultra cost-efficient model ideal for real-time log stream categorization and noise suppression.',
    recommendedFor: 'Continuous High-Frequency Telemetry Classification',
    defaultEndpoint: 'https://api.openai.com/v1',
  },
  {
    id: 'o1-preview',
    name: 'o1 Reasoning Preview',
    vendor: 'OpenAI',
    providerType: 'openai',
    contextWindow: '128k',
    latencyTier: 'Reasoning Heavy',
    capabilities: ['Deep Logic', 'Multi-Step Math', 'Security Verification'],
    description: 'Deep reasoning LLM designed for complex distributed systems debugging and race-condition audits.',
    recommendedFor: 'Complex Kubernetes Disaster Recovery & State Discrepancy Audits',
    defaultEndpoint: 'https://api.openai.com/v1',
  },
  {
    id: 'o3-mini',
    name: 'o3-Mini High Efficiency',
    vendor: 'OpenAI',
    providerType: 'openai',
    contextWindow: '200k',
    latencyTier: 'Fast',
    capabilities: ['STEM Reasoning', 'Refactoring', 'YAML Generation'],
    description: 'Next-generation reasoning model combining fast inference with formal logic synthesis.',
    recommendedFor: 'Automated Helm/Kustomize Refactoring & Policy Synthesis',
    defaultEndpoint: 'https://api.openai.com/v1',
  },

  // Anthropic Claude (3)
  {
    id: 'claude-3-7-sonnet',
    name: 'Claude 3.7 Sonnet (Hybrid Reasoning)',
    vendor: 'Anthropic',
    providerType: 'openai',
    contextWindow: '200k',
    latencyTier: 'Fast',
    capabilities: ['Dynamic Reasoning', 'Frontier Coding', 'Vision', 'Architecture'],
    description: 'Frontier hybrid reasoning model capable of switching between rapid response and extended chain-of-thought.',
    recommendedFor: 'Autonomous Agent Orchestration & High-Complexity Architecture Refactoring',
    defaultEndpoint: 'https://api.anthropic.com/v1',
  },
  {
    id: 'claude-3-5-sonnet',
    name: 'Claude 3.5 Sonnet',
    vendor: 'Anthropic',
    providerType: 'openai',
    contextWindow: '200k',
    latencyTier: 'Fast',
    capabilities: ['Advanced Code', 'Vision', 'Architecture Design'],
    description: 'Industry benchmark for system architecture design, code generation, and complex vulnerability triage.',
    recommendedFor: 'DevSecOps Vulnerability Mitigation & Infrastructure as Code (IaC)',
    defaultEndpoint: 'https://api.anthropic.com/v1',
  },
  {
    id: 'claude-3-5-haiku',
    name: 'Claude 3.5 Haiku',
    vendor: 'Anthropic',
    providerType: 'openai',
    contextWindow: '200k',
    latencyTier: 'Ultra-Fast',
    capabilities: ['Sub-100ms Inference', 'Log Parsing', 'Fast Summaries'],
    description: 'Lightning-fast compact model for real-time incident summary cards and alerting triage.',
    recommendedFor: 'Paging Alerts & Instant Incident Digest Delivery',
    defaultEndpoint: 'https://api.anthropic.com/v1',
  },
  {
    id: 'claude-3-opus',
    name: 'Claude 3 Opus',
    vendor: 'Anthropic',
    providerType: 'openai',
    contextWindow: '200k',
    latencyTier: 'Standard',
    capabilities: ['Deep Analysis', 'Long-Horizon Planning', 'Executive Reports'],
    description: 'Highest capability Claude model for generating comprehensive quarterly compliance and SOC2 reports.',
    recommendedFor: 'Quarterly Executive Compliance & Board Reporting',
    defaultEndpoint: 'https://api.anthropic.com/v1',
  },

  // Google Gemini (3)
  {
    id: 'gemini-1.5-pro',
    name: 'Gemini 1.5 Pro',
    vendor: 'Google',
    providerType: 'openai',
    contextWindow: '2M Tokens',
    latencyTier: 'Standard',
    capabilities: ['2M Context', 'Massive Log Analysis', 'Cross-Cluster Correlator'],
    description: 'Massive 2 Million token context window capable of ingesting whole cluster log dumps in one prompt.',
    recommendedFor: 'Full Cluster 24-Hour Incident Reconstruction & Core Dumps',
    defaultEndpoint: 'https://generativelanguage.googleapis.com/v1beta/openai',
  },
  {
    id: 'gemini-1.5-flash',
    name: 'Gemini 1.5 Flash',
    vendor: 'Google',
    providerType: 'openai',
    contextWindow: '1M Tokens',
    latencyTier: 'Ultra-Fast',
    capabilities: ['Fast Multimodal', 'Long Context', 'Low Cost'],
    description: 'High-speed long-context engine built for frequent timeline syncs and multi-cluster queries.',
    recommendedFor: 'Global Fleet Health Aggregations & Metrics Summaries',
    defaultEndpoint: 'https://generativelanguage.googleapis.com/v1beta/openai',
  },
  {
    id: 'gemini-2.0-flash',
    name: 'Gemini 2.0 Flash Next-Gen',
    vendor: 'Google',
    providerType: 'openai',
    contextWindow: '1M Tokens',
    latencyTier: 'Ultra-Fast',
    capabilities: ['Sub-second Realtime', 'Tool Use', 'Structured Output'],
    description: 'Next-gen real-time reasoning model for real-time WebSocket agent co-pilots.',
    recommendedFor: 'Live Terminal Co-Pilot & Streaming K8s Command Suggestions',
    defaultEndpoint: 'https://generativelanguage.googleapis.com/v1beta/openai',
  },

  // DeepSeek (3)
  {
    id: 'deepseek-r1',
    name: 'DeepSeek R1 Open Reasoning',
    vendor: 'DeepSeek',
    providerType: 'vllm',
    contextWindow: '64k',
    latencyTier: 'Reasoning Heavy',
    capabilities: ['Open Weights', 'Chain of Thought', 'Formal Logic'],
    description: 'Open-weight reasoning titan providing self-hosted parity with proprietary reasoning models.',
    recommendedFor: 'Air-Gapped Cluster Root Cause Verification & Policy Audits',
    defaultEndpoint: 'http://vllm-deepseek-r1.ai-core.svc:8000/v1',
  },
  {
    id: 'deepseek-v3',
    name: 'DeepSeek V3 671B MoE',
    vendor: 'DeepSeek',
    providerType: 'vllm',
    contextWindow: '64k',
    latencyTier: 'Fast',
    capabilities: ['671B MoE', '37B Active', 'General Ops', 'Zero Data Leak'],
    description: 'Massive mixture-of-experts model providing high performance at low compute cost.',
    recommendedFor: 'General Enterprise Operations & Multi-Tenant Assistance',
    defaultEndpoint: 'http://vllm-deepseek-v3.ai-core.svc:8000/v1',
  },
  {
    id: 'deepseek-coder-33b',
    name: 'DeepSeek Coder 33B Spec',
    vendor: 'DeepSeek',
    providerType: 'ollama',
    contextWindow: '32k',
    latencyTier: 'Fast',
    capabilities: ['K8s CRDs', 'Go Operators', 'Terraform', 'Dockerfile'],
    description: 'Fine-tuned programming model with deep understanding of Kubernetes Go operators and Helm templates.',
    recommendedFor: 'Custom Operator Code Review & GitOps PR Generation',
    defaultEndpoint: 'http://ollama-service.ai-core.svc:11434',
  },

  // Ollama / Self-Hosted Open Source (7)
  {
    id: 'llama-3.3-70b',
    name: 'Llama 3.3 70B Instruct',
    vendor: 'Ollama / OSS',
    providerType: 'ollama',
    contextWindow: '128k',
    latencyTier: 'Fast',
    capabilities: ['Air-Gapped', 'Zero External API', 'Enterprise Tool Use'],
    description: 'Meta flagship open model running natively in air-gapped on-prem Kubernetes worker nodes.',
    recommendedFor: 'Fully Isolated Air-Gapped SRE Agent & ZeroTrust Operations',
    defaultEndpoint: 'http://ollama-service.ai-core.svc:11434',
  },
  {
    id: 'llama-3.2-3b',
    name: 'Llama 3.2 3B Edge',
    vendor: 'Ollama / OSS',
    providerType: 'ollama',
    contextWindow: '128k',
    latencyTier: 'Ultra-Fast',
    capabilities: ['Edge Compute', 'Minimal RAM (2GB)', 'Micro-Agents'],
    description: 'Ultra-lightweight edge model designed to run in worker daemonsets for localized log filtering.',
    recommendedFor: 'Edge DaemonSet In-Pod Telemetry Pre-Filtering',
    defaultEndpoint: 'http://ollama-service.ai-core.svc:11434',
  },
  {
    id: 'mistral-large',
    name: 'Mistral Large 2 123B',
    vendor: 'Ollama / OSS',
    providerType: 'vllm',
    contextWindow: '128k',
    latencyTier: 'Fast',
    capabilities: ['Multilingual', 'Advanced Reasoning', 'Precise Function Calls'],
    description: 'European sovereign AI powerhouse with strong multilingual support and native function calling.',
    recommendedFor: 'Multi-Region Fleet Topology Governance & Multilingual Teams',
    defaultEndpoint: 'http://vllm-mistral.ai-core.svc:8000/v1',
  },
  {
    id: 'qwen-2.5-72b',
    name: 'Qwen 2.5 72B Instruct',
    vendor: 'Ollama / OSS',
    providerType: 'ollama',
    contextWindow: '128k',
    latencyTier: 'Fast',
    capabilities: ['Top Coding Benchmark', 'Complex JSON Specs', 'PromQL Generator'],
    description: 'Top-tier open-source coding model with exceptional accuracy in PromQL and LogQL query generation.',
    recommendedFor: 'Automated Prometheus PromQL Rule Construction',
    defaultEndpoint: 'http://ollama-service.ai-core.svc:11434',
  },
  {
    id: 'phi-4-14b',
    name: 'Phi-4 14B High Math',
    vendor: 'Ollama / OSS',
    providerType: 'ollama',
    contextWindow: '16k',
    latencyTier: 'Fast',
    capabilities: ['Algorithmic Rigor', 'Small Footprint', 'FinOps Math'],
    description: 'Microsoft compact high-reasoning model optimized for resource forecasting and cost calculations.',
    recommendedFor: 'FinOps Node Bin-Packing & Spot Price Predictive Analytics',
    defaultEndpoint: 'http://ollama-service.ai-core.svc:11434',
  },
  {
    id: 'codellama-34b',
    name: 'CodeLlama 34B DevOps',
    vendor: 'Ollama / OSS',
    providerType: 'ollama',
    contextWindow: '16k',
    latencyTier: 'Fast',
    capabilities: ['Shell Scripts', 'Ansible', 'Security Playbooks'],
    description: 'Dedicated code generation model specialized in automated shell remediation scripts and runbooks.',
    recommendedFor: 'Automated Remediation Script Generation',
    defaultEndpoint: 'http://ollama-service.ai-core.svc:11434',
  },
  {
    id: 'gemma-2-27b',
    name: 'Gemma 2 27B Shield',
    vendor: 'Ollama / OSS',
    providerType: 'ollama',
    contextWindow: '8k',
    latencyTier: 'Fast',
    capabilities: ['Google Safety Trained', 'Guardrail Filter', 'Privacy Shield'],
    description: 'Engineered with deep safety alignment to prevent prompt injection and confidential data leakage.',
    recommendedFor: 'Inbound Prompt Guardrail & Secret Redaction Firewall',
    defaultEndpoint: 'http://ollama-service.ai-core.svc:11434',
  },
]

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
