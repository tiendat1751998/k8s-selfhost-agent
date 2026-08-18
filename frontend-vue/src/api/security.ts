import { api, type ApiResponse } from './client'

export interface ComplianceFramework {
  id: string
  name: string
  version: string
  description: string
  total_rules: number
  passing_rules: number
}

export interface SecurityViolation {
  id: string
  resource_type: string
  resource_name: string
  namespace: string
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'
  rule_id: string
  description: string
  remediation: string
  detected_at: string
}

export const securityApi = {
  async getFrameworks(): Promise<ComplianceFramework[]> {
    const res = await api.get<ApiResponse<ComplianceFramework[]>>('/compliance/frameworks')
    return res.data || []
  },

  async getViolations(severity?: string): Promise<{ items: SecurityViolation[]; total: number }> {
    const res = await api.get<ApiResponse<SecurityViolation[]>>('/compliance/violations', {
      severity: severity || '',
      limit: 100,
    })
    return {
      items: res.data || [],
      total: res.total || 0,
    }
  },
}
