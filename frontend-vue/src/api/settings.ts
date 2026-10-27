import { api } from './client'

export interface Setting {
  id: string
  category: string
  key: string
  value: string
  tenant_id: string
  updated_by: string
  updated_at: string
}

export interface SettingUpdate {
  category: string
  key: string
  value: string
}

export interface IntegrationTestResult {
  reachable: boolean
  status_code: number
  latency_ms: number
  error?: string
}

export const settingsApi = {
  getAll: () => api.get<Setting[]>('/settings'),
  getByCategory: (category: string) => api.get<Setting[]>(`/settings?category=${category}`),
  update: (settings: SettingUpdate[]) => api.put<Setting[]>('/settings', { settings }),
  getDefaults: () => api.get<Record<string, Record<string, string>>>('/settings/defaults'),
  testIntegration: (url: string) => api.post<IntegrationTestResult>('/settings/integrations/test', { url }),
}

export interface ApiKeyItem {
  id: string
  name: string
  prefix: string
  role: string
  created_at: string
  last_used: string
  token?: string
}

export const apiKeysApi = {
  async list(): Promise<ApiKeyItem[]> {
    const res = await api.get<{ data: ApiKeyItem[] } | ApiKeyItem[]>('/settings/api-keys')
    return Array.isArray(res) ? res : (res?.data || [])
  },
  async create(data: { name: string; role: string }): Promise<ApiKeyItem> {
    return api.post<ApiKeyItem>('/settings/api-keys', data)
  },
  async revoke(id: string): Promise<{ status: string }> {
    return api.delete<{ status: string }>(`/settings/api-keys/${id}`)
  },
}
