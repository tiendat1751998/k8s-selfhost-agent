import { api, type ApiResponse } from './client'

export interface ComplianceFramework {
  id: string
  name: string
  version?: string
  description?: string
  icon?: string
  total_checks?: number
  passed_checks?: number
  failed_checks?: number
  score?: number
  last_scan_at?: string
  created_at?: string
  updated_at?: string
  total_rules: number
  passing_rules: number
}

export interface RawSecurityViolation {
  id?: string
  framework_id?: string
  severity?: string
  policy?: string
  resource?: string
  namespace?: string
  cluster?: string
  message?: string
  resolved?: boolean
  detected_at?: string
  rule_id?: string
  description?: string
  remediation?: string
  resource_type?: string
  resource_name?: string
}

export interface SecurityViolation {
  id: string
  framework_id?: string
  resource_type: string
  resource_name: string
  namespace: string
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'
  rule_id: string
  description: string
  remediation: string
  detected_at: string
  cluster?: string
  resolved?: boolean
  policy?: string
  resource?: string
  message?: string
}

export function parseViolationResource(resourceStr?: string, defaultType = 'workload'): { resource_type: string; resource_name: string } {
  if (!resourceStr) {
    return { resource_type: defaultType, resource_name: 'unknown' }
  }
  if (resourceStr.includes('/')) {
    const slashIdx = resourceStr.indexOf('/')
    return {
      resource_type: resourceStr.substring(0, slashIdx),
      resource_name: resourceStr.substring(slashIdx + 1),
    }
  }
  return {
    resource_type: defaultType,
    resource_name: resourceStr,
  }
}

export function normalizeViolation(raw: RawSecurityViolation): SecurityViolation {
  const parsedResource = parseViolationResource(raw.resource || (raw.resource_name ? `${raw.resource_type || 'workload'}/${raw.resource_name}` : ''))
  const resource_type = raw.resource_type || parsedResource.resource_type
  const resource_name = raw.resource_name || parsedResource.resource_name

  const rawSev = (raw.severity || 'LOW').toUpperCase()
  const severity = (['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'].includes(rawSev) ? rawSev : 'LOW') as 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'

  return {
    id: raw.id || '',
    framework_id: raw.framework_id || '',
    resource_type,
    resource_name,
    namespace: raw.namespace || 'default',
    severity,
    rule_id: raw.rule_id || raw.policy || 'RULE-UNKNOWN',
    description: raw.description || raw.message || 'No description provided',
    remediation: raw.remediation || '',
    detected_at: raw.detected_at || '',
    cluster: raw.cluster || '',
    resolved: raw.resolved || false,
    policy: raw.policy,
    resource: raw.resource,
    message: raw.message,
  }
}

export const securityApi = {
  async getFrameworks(): Promise<ComplianceFramework[]> {
    const res = await api.get<ApiResponse<ComplianceFramework[]>>('/compliance/frameworks')
    const rawList = res.data || []
    return rawList.map(f => ({
      ...f,
      total_rules: f.total_rules ?? f.total_checks ?? 0,
      passing_rules: f.passing_rules ?? f.passed_checks ?? 0,
    }))
  },

  async getViolations(severity?: string): Promise<{ items: SecurityViolation[]; total: number }> {
    const params: Record<string, string | number> = {
      limit: 100,
    }
    if (severity && severity !== 'ALL') {
      params.severity = severity.toLowerCase()
    }
    const res = await api.get<ApiResponse<RawSecurityViolation[]>>('/compliance/violations', params)
    const rawItems = res.data || []
    const items = rawItems.map(normalizeViolation)
    return {
      items,
      total: res.total ?? items.length,
    }
  },
}
