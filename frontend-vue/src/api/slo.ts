import { api, type ApiResponse } from './client'

// ==========================================
// Observability & SLO Interfaces
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
// API Clients Export
// ==========================================

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
