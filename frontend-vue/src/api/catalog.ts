import { api } from './client'

export interface ServiceEntry {
  id: string
  name: string
  description: string
  type: string // service, api, library, database, frontend, worker
  lifecycle: string // development, staging, production, deprecated
  owner_team: string
  owner_email: string
  repo_url: string
  docs_url: string
  tags: string[]
  annotations: Record<string, string>
  tenant_id: string
  created_at: string
  updated_at: string
}

export interface CatalogStats {
  total: number
  by_type: Record<string, number>
  by_lifecycle: Record<string, number>
}

export interface ServiceFilter {
  type?: string
  lifecycle?: string
  owner_team?: string
  search?: string
}

export type ServiceType =
  | 'service'
  | 'api'
  | 'library'
  | 'database'
  | 'frontend'
  | 'worker'

export type ServiceLifecycle =
  | 'development'
  | 'staging'
  | 'production'
  | 'deprecated'

export const catalogApi = {
  list: (filter?: ServiceFilter): Promise<ServiceEntry[]> =>
    api.get<ServiceEntry[]>('/catalog/services', filter as Record<string, string | number> | undefined),

  get: (id: string): Promise<ServiceEntry> =>
    api.get<ServiceEntry>(`/catalog/services/${encodeURIComponent(id)}`),

  create: (entry: Partial<ServiceEntry>): Promise<ServiceEntry> =>
    api.post<ServiceEntry>('/catalog/services', entry),

  update: (id: string, entry: Partial<ServiceEntry>): Promise<ServiceEntry> =>
    api.put<ServiceEntry>(`/catalog/services/${encodeURIComponent(id)}`, entry),

  delete: (id: string): Promise<void> =>
    api.delete<void>(`/catalog/services/${encodeURIComponent(id)}`),

  stats: (): Promise<CatalogStats> =>
    api.get<CatalogStats>('/catalog/stats'),
}
