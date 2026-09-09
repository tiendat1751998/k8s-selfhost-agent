import { api } from './client'

// ==========================================
// Promotions Pipeline Interfaces
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
// API Clients Export
// ==========================================

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
