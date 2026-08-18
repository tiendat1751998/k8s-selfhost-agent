import { api, type ApiResponse } from './client'

// ==========================================
// 1. AUDIT & VULNERABILITY MANAGEMENT
// ==========================================
export interface AuditFinding {
  id: string
  category: 'missing_integration' | 'missing_dashboard' | 'broken_route' | 'stale_provider' | 'disconnected_cluster' | 'cve_vulnerability' | 'iac_misconfiguration' | string
  severity: 'critical' | 'high' | 'medium' | 'low'
  description: string
  remediation: string
  status: 'open' | 'resolved' | 'ignored'
  detected_at: string
  resolved_at?: string
}

export interface AuditRun {
  id: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  start_time: string
  end_time?: string
  findings_count: number
}

export interface AuditTriggerResponse {
  status: string
  run_id: string
  message: string
}

export const auditApi = {
  async getFindings(status: string = 'open'): Promise<AuditFinding[]> {
    const res = await api.get<ApiResponse<AuditFinding[]>>('/audit/findings', { status })
    return res.data || []
  },
  async resolveFinding(id: string): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/audit/findings/${id}/resolve`)
  },
  async triggerRun(): Promise<AuditTriggerResponse> {
    return api.post<AuditTriggerResponse>('/audit/run')
  },
  async getLatestRun(): Promise<AuditRun | null> {
    return api.get<AuditRun>('/audit/runs/latest')
  },
}

// ==========================================
// 2. COMPLIANCE & GOVERNANCE
// ==========================================
export interface ComplianceFramework {
  id: string
  name: string
  icon?: string
  total_checks: number
  passed_checks: number
  failed_checks: number
  score: number
  last_scan_at: string
  created_at?: string
  updated_at?: string
}

export interface ComplianceViolation {
  id: string
  framework_id: string
  severity: 'critical' | 'high' | 'medium' | 'low'
  policy: string
  resource: string
  namespace: string
  cluster: string
  message: string
  resolved: boolean
  detected_at: string
}

export interface ComplianceViolationsResponse {
  data: ComplianceViolation[]
  total: number
}

export const complianceApi = {
  async getFrameworks(): Promise<ComplianceFramework[]> {
    const res = await api.get<ApiResponse<ComplianceFramework[]>>('/compliance/frameworks')
    return res.data || []
  },
  async getViolations(severity?: string, limit: number = 50, offset: number = 0): Promise<ComplianceViolationsResponse> {
    const params: Record<string, string | number> = { limit, offset }
    if (severity && severity !== 'ALL') params.severity = severity
    const res = await api.get<ComplianceViolationsResponse>('/compliance/violations', params)
    return {
      data: res.data || [],
      total: res.total ?? (res.data ? res.data.length : 0),
    }
  },
}

// ==========================================
// 3. GITOPS DRIFT DETECTION & RECONCILIATION
// ==========================================
export interface DriftRecord {
  id: string
  cluster: string
  namespace: string
  resource: string
  resource_kind: string
  expected_state: string
  actual_state: string
  diff: string
  status: 'in_sync' | 'drifted' | 'unknown'
  detected_at: string
}

export interface DriftListResponse {
  data: DriftRecord[]
  total: number
}

export const driftApi = {
  async getDrifts(params?: { cluster?: string; status?: string; limit?: number; offset?: number }): Promise<DriftListResponse> {
    const res = await api.get<DriftListResponse>('/drift', params as Record<string, string | number>)
    return {
      data: res.data || [],
      total: res.total ?? (res.data ? res.data.length : 0),
    }
  },
  async createDrift(drift: Partial<DriftRecord>): Promise<DriftRecord> {
    return api.post<DriftRecord>('/drift', drift)
  },
  async resolveDrift(id: string): Promise<{ status: string }> {
    return api.put<{ status: string }>(`/drift/${id}/resolve`)
  },
}

// ==========================================
// 4. BACKUP & DISASTER RECOVERY
// ==========================================
export interface BackupStorage {
  id: string
  tenant_id?: string
  name: string
  type: string
  endpoint: string
  bucket: string
  credentials?: Record<string, string>
  created_at?: string
  updated_at?: string
}

export interface BackupPolicy {
  id: string
  tenant_id?: string
  name: string
  db_type: string
  db_host: string
  db_port: number
  db_name: string
  storage_id: string
  secondary_storage_id?: string
  schedule: string
  retention_count: number
  backup_type: string
  compression_level?: number
  encryption_enabled?: boolean
  encryption_key_id?: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface BackupJob {
  id: string
  tenant_id?: string
  policy_id: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'verified' | string
  backup_type: string
  storage_path?: string
  local_storage_path?: string
  cloud_storage_path?: string
  size_bytes?: number
  compressed_size_bytes?: number
  duration_ms?: number
  checksum_sha256?: string
  wal_start_lsn?: string
  wal_end_lsn?: string
  verified_at?: string
  verification_status?: string
  error_message?: string
  created_at?: string
  updated_at?: string
  started_at?: string
  completed_at?: string
}

export interface RestoreJob {
  id: string
  tenant_id?: string
  backup_job_id: string
  target_db_host: string
  target_db_name: string
  pitr_timestamp?: string
  dry_run?: boolean
  source_storage_type?: string
  verification_log?: string
  status: 'pending' | 'running' | 'completed' | 'failed' | string
  error_message?: string
  created_at?: string
  updated_at?: string
  started_at?: string
  completed_at?: string
}

export const backupApi = {
  async getStorages(): Promise<BackupStorage[]> {
    const res = await api.get<ApiResponse<BackupStorage[]>>('/backup/storages')
    return res.data || []
  },
  async createStorage(storage: Partial<BackupStorage>): Promise<BackupStorage> {
    return api.post<BackupStorage>('/backup/storages', storage)
  },
  async getPolicies(): Promise<BackupPolicy[]> {
    const res = await api.get<ApiResponse<BackupPolicy[]>>('/backup/policies')
    return res.data || []
  },
  async createPolicy(policy: Partial<BackupPolicy>): Promise<BackupPolicy> {
    return api.post<BackupPolicy>('/backup/policies', policy)
  },
  async getJobs(): Promise<BackupJob[]> {
    const res = await api.get<ApiResponse<BackupJob[]>>('/backup/jobs')
    return res.data || []
  },
  async triggerBackup(policyId: string, backupType: string = 'full', storagePath: string = ''): Promise<BackupJob> {
    return api.post<BackupJob>('/backup/jobs', {
      policy_id: policyId,
      backup_type: backupType,
      storage_path: storagePath,
    })
  },
  async getRestores(): Promise<RestoreJob[]> {
    const res = await api.get<ApiResponse<RestoreJob[]>>('/backup/restores')
    return res.data || []
  },
  async triggerRestore(backupJobId: string, targetHost: string, targetDB: string): Promise<RestoreJob> {
    return api.post<RestoreJob>('/backup/restores', {
      backup_job_id: backupJobId,
      target_db_host: targetHost,
      target_db_name: targetDB,
    })
  },
}

// ==========================================
// 5. WORKFLOW AUTOMATION & SELF-HEALING
// ==========================================
export interface AutomationRule {
  id: string
  name: string
  trigger_type: 'pod_restart' | 'node_pressure' | 'deployment_failure' | 'high_cpu' | 'high_memory' | 'slo_breach' | 'error_rate' | string
  trigger_config: Record<string, string>
  action_type: 'generate_rca' | 'send_notification' | 'rollback' | 'scale_deployment' | 'create_incident' | 'cordon_node' | 'restart_pod' | string
  action_config: Record<string, string>
  enabled: boolean
  executions: number
  last_triggered?: string
  created_at?: string
  updated_at?: string
}

export interface AutomationExecution {
  id: string
  rule_id: string
  rule_name: string
  trigger_event: string
  action_taken: string
  result: 'success' | 'failure' | string
  error_detail?: string
  created_at: string
}

export interface AutomationExecutionsResponse {
  data: AutomationExecution[]
  total: number
}

export const automationApi = {
  async getRules(): Promise<AutomationRule[]> {
    const res = await api.get<ApiResponse<AutomationRule[]>>('/automation/rules')
    return res.data || []
  },
  async createRule(rule: Partial<AutomationRule>): Promise<AutomationRule> {
    return api.post<AutomationRule>('/automation/rules', rule)
  },
  async updateRule(id: string, rule: Partial<AutomationRule>): Promise<AutomationRule> {
    return api.put<AutomationRule>(`/automation/rules/${id}`, rule)
  },
  async deleteRule(id: string): Promise<{ status: string }> {
    return api.delete<{ status: string }>(`/automation/rules/${id}`)
  },
  async toggleRule(id: string, enabled: boolean): Promise<{ status: string; enabled: boolean }> {
    return api.put<{ status: string; enabled: boolean }>(`/automation/rules/${id}/toggle`, { enabled })
  },
  async getExecutions(limit: number = 50, offset: number = 0): Promise<AutomationExecutionsResponse> {
    const res = await api.get<AutomationExecutionsResponse>('/automation/executions', { limit, offset })
    return {
      data: res.data || [],
      total: res.total ?? (res.data ? res.data.length : 0),
    }
  },
}

// ==========================================
// 6. OPERATIONAL RUNBOOKS CATALOG
// ==========================================
export interface Runbook {
  id: string
  title: string
  category: string
  content: string
  tags: string[]
  author: string
  steps_count: number
  last_used_at?: string
  tenant_id?: string
  created_at?: string
  updated_at?: string
}

export interface RunbookListResponse {
  data: Runbook[]
  total: number
}

export const runbookApi = {
  async getRunbooks(category?: string, limit: number = 50, offset: number = 0): Promise<RunbookListResponse> {
    const params: Record<string, string | number> = { limit, offset }
    if (category) params.category = category
    const res = await api.get<RunbookListResponse>('/runbooks', params)
    return {
      data: res.data || [],
      total: res.total ?? (res.data ? res.data.length : 0),
    }
  },
  async getRunbookById(id: string): Promise<Runbook> {
    return api.get<Runbook>(`/runbooks/${id}`)
  },
  async createRunbook(runbook: Partial<Runbook>): Promise<Runbook> {
    return api.post<Runbook>('/runbooks', runbook)
  },
  async updateRunbook(id: string, runbook: Partial<Runbook>): Promise<Runbook> {
    return api.put<Runbook>(`/runbooks/${id}`, runbook)
  },
  async deleteRunbook(id: string): Promise<{ status: string }> {
    return api.delete<{ status: string }>(`/runbooks/${id}`)
  },
}

// ==========================================
// 7. FINOPS & COST OPTIMIZATION
// ==========================================
export interface ClusterCost {
  id: string
  name: string
  provider: string
  monthly_cost: number
  daily_cost: number
  cpu_cost: number
  memory_cost: number
  storage_cost: number
  network_cost: number
  trend: number
  updated_at: string
}

export interface NamespaceCost {
  id: string
  namespace: string
  cluster: string
  cpu_requested: string
  memory_requested: string
  monthly_cost: number
  utilization: number
  updated_at: string
}

export interface CostSummaryResponse {
  clusters: ClusterCost[]
  namespaces: NamespaceCost[]
}

export interface ResourceWaste {
  id: string
  type: string
  resource: string
  namespace: string
  cluster: string
  cpu_util?: number
  mem_util?: number
  wasted_cost: number
  severity: 'critical' | 'high' | 'medium' | 'low' | string
  updated_at: string
}

export const costApi = {
  async getSummary(): Promise<CostSummaryResponse> {
    return api.get<CostSummaryResponse>('/cost/summary')
  },
  async getWaste(): Promise<ResourceWaste[]> {
    const res = await api.get<ApiResponse<ResourceWaste[]>>('/cost/waste')
    return res.data || []
  },
}

// ==========================================
// 8. CAPACITY PLANNING & WORKLOAD FORECASTING
// ==========================================
export interface CapacityForecast {
  id: string
  cluster: string
  resource_type: 'cpu' | 'memory' | 'storage' | string
  current_usage: number // percentage 0-100
  forecast_7d: number
  forecast_30d: number
  forecast_90d: number
  exhaustion_at?: string
  status: 'healthy' | 'warning' | 'critical' | string
  recorded_at: string
}

export const capacityApi = {
  async getForecasts(cluster?: string): Promise<CapacityForecast[]> {
    const params: Record<string, string> = {}
    if (cluster) params.cluster = cluster
    const res = await api.get<ApiResponse<CapacityForecast[]>>('/capacity', params)
    return res.data || []
  },
  async recordForecast(forecast: Partial<CapacityForecast>): Promise<CapacityForecast> {
    return api.post<CapacityForecast>('/capacity', forecast)
  },
}
