import { api } from './client'

export interface DetectedTool {
  id: string
  name: string
  category: string
  status: 'detected' | 'unreachable' | 'not_configured' | string
  version: string
  endpoint: string
  source: 'settings' | 'k8s_discovery' | 'manual' | string
  health: 'healthy' | 'degraded' | 'unknown' | string
  last_checked: string
  metadata?: Record<string, string>
  tenant_id: string
}

export interface EcosystemSummary {
  total: number
  healthy: number
  degraded: number
  by_category: Record<string, number>
}

export interface CreateToolRequest {
  name: string
  category: string
  endpoint?: string
  version?: string
  status?: string
  health?: string
  metadata?: Record<string, string>
}

export const ecosystemApi = {
  getTools: (category?: string): Promise<DetectedTool[]> =>
    api.get<DetectedTool[]>('/ecosystem', category ? { category } : undefined),

  getSummary: (): Promise<EcosystemSummary> =>
    api.get<EcosystemSummary>('/ecosystem/summary'),

  triggerScan: (): Promise<DetectedTool[]> =>
    api.post<DetectedTool[]>('/ecosystem/scan', {}),

  createTool: (req: CreateToolRequest): Promise<DetectedTool> =>
    api.post<DetectedTool>('/ecosystem/tools', req),

  deleteTool: (id: string): Promise<void> =>
    api.delete<void>(`/ecosystem/tools/${encodeURIComponent(id)}`),
}
