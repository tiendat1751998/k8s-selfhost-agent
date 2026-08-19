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
