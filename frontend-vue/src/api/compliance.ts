import { api, type ApiResponse } from './client'
import type {
  ComplianceFramework,
  ComplianceViolationsResponse,
} from './governance'

export interface ComplianceScanResult {
  success?: boolean
  message?: string
  status?: string
  run_id?: string
  frameworks?: ComplianceFramework[]
  [key: string]: unknown
}

export interface ApiPostClient {
  post<T = unknown>(endpoint: string, data?: unknown): Promise<T>
}

export interface RemediationResponse {
  success?: boolean
  violation_id?: string
  status?: 'remediated' | 'manual_action_required' | 'pending' | 'failed' | string
  automated?: boolean
  summary?: string
  commands?: string[]
  message?: string
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

  remediateViolation: (id: string, body: { confirm?: boolean } = {}) =>
    api.post<RemediationResponse>(`/compliance/violations/${id}/remediate`, body),

  resolveViolation: async (id: string, body: { confirm?: boolean } = { confirm: true }) => {
    try {
      return await api.post<RemediationResponse>(`/compliance/violations/${id}/resolve`, body)
    } catch {
      return await api.post<RemediationResponse>(`/compliance/violations/${id}/remediate`, body)
    }
  },

  async triggerScan(customClient?: ApiPostClient): Promise<ComplianceScanResult> {
    const client = customClient || api
    try {
      const res = await client.post<ComplianceScanResult>('/compliance/scan')
      return res
    } catch (err: unknown) {
      const is404 =
        (typeof err === 'object' && err !== null && ('status' in err || 'statusCode' in err) &&
          ((err as { status?: number; statusCode?: number }).status === 404 ||
            (err as { status?: number; statusCode?: number }).statusCode === 404)) ||
        (err instanceof Error && /404|not found/i.test(err.message))

      if (is404) {
        const fallbackRes = await client.post<ComplianceScanResult>('/audit/run')
        return fallbackRes
      }
      throw err
    }
  },
}
