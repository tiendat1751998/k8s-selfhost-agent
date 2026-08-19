import { api } from './client'

export interface Plugin {
  id: string
  name: string
  version: string
  description: string
  author: string
  enabled: boolean
  entry_point: string
  icon: string
  category: string
  permissions: string[]
  config: Record<string, string>
  tenant_id: string
  created_at: string
  updated_at: string
}

export interface PluginStats {
  total: number
  enabled: number
  disabled: number
  by_category: Record<string, number>
}

export interface CreatePluginDTO {
  name: string
  version?: string
  description?: string
  author?: string
  enabled?: boolean
  entry_point?: string
  icon?: string
  category?: string
  permissions?: string[]
  config?: Record<string, string>
}

export interface UpdatePluginDTO {
  name: string
  version?: string
  description?: string
  author?: string
  enabled?: boolean
  entry_point?: string
  icon?: string
  category?: string
  permissions?: string[]
  config?: Record<string, string>
}

export interface TogglePluginDTO {
  enabled?: boolean
}

export const pluginsApi = {
  list: (enabledOnly?: boolean): Promise<Plugin[]> => {
    const params: Record<string, string> = {}
    if (enabledOnly !== undefined) {
      params.enabled = String(enabledOnly)
    }
    return api.get<Plugin[]>('/plugins', params)
  },

  get: (id: string): Promise<Plugin> =>
    api.get<Plugin>(`/plugins/${encodeURIComponent(id)}`),

  create: (data: CreatePluginDTO): Promise<Plugin> =>
    api.post<Plugin>('/plugins', data),

  update: (id: string, data: UpdatePluginDTO): Promise<Plugin> =>
    api.put<Plugin>(`/plugins/${encodeURIComponent(id)}`, data),

  delete: (id: string): Promise<void> =>
    api.delete<void>(`/plugins/${encodeURIComponent(id)}`),

  toggle: (id: string, enabled?: boolean): Promise<Plugin> =>
    api.post<Plugin>(`/plugins/${encodeURIComponent(id)}/toggle`, enabled !== undefined ? { enabled } : {}),

  getStats: (): Promise<PluginStats> =>
    api.get<PluginStats>('/plugins/stats'),
}
