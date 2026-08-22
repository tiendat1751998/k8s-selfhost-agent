import { api, type ApiResponse } from './client'

export interface UpdateResourcesPayload {
  type?: string
  cluster?: string
  namespace?: string
  name: string
  replicas?: number
  memory_limit?: string // e.g. "2GiB", "512MiB"
  memory_reservation?: string // e.g. "512MiB"
  cpu_limit?: string // e.g. "2", "4"
  cpu_reservation?: string // e.g. "500m"
}

export interface DeploymentApp {
  id?: string
  rawId?: string
  name: string
  team: string
  env: string
  image: string
  target: string
  namespace: string
  type: 'kubernetes' | 'swarm' | 'docker' | 'container' | string
  replicas: number
  status: 'healthy' | 'degraded' | 'down' | string
  cpu: string
  memory: string
  port: number
  netType?: string
  volume?: string
  // Strategy & Rollout orchestration
  strategy?: 'RollingUpdate' | 'Canary' | 'BlueGreen' | 'Recreate' | string
  canaryWeight?: number
  canaryVersion?: string
  canarySteps?: number[]
  blueGreenActive?: 'blue' | 'green'
  blueVersion?: string
  greenVersion?: string
  revision?: number
  availableReplicas?: number
  updatedReplicas?: number
  readyReplicas?: number
  envVars?: Record<string, string>
  ingressHost?: string
  tlsEnabled?: boolean
  storageClass?: string
  storageSize?: string
  mountPath?: string
  livenessProbe?: string
  readinessProbe?: string
  paused?: boolean
  node?: string
  created?: string
  // Resource allocations
  cpuLimit?: string
  cpuReservation?: string
  memoryLimit?: string
  memoryReservation?: string
}

export interface DeploymentTemplate {
  name: string
  category: 'web' | 'data' | 'system' | string
  version: string
  desc: string
  ports: number
  cpu: string
  mem: string
  strategy?: string
}

export interface ScaleDeploymentPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  replicas: number
}

export interface RestartDeploymentPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
}

export interface DeleteDeploymentPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
}

export interface RollbackDeploymentPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  revision?: number
}

export interface CanaryWeightPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  weight: number
}

export interface BlueGreenCutoverPayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  targetColor: 'blue' | 'green'
}

export interface PauseResumePayload {
  type?: string
  cluster?: string
  namespace: string
  name: string
  action: 'pause' | 'resume'
}

export const deploymentsApi = {
  async list(): Promise<DeploymentApp[]> {
    try {
      const res = await api.get<ApiResponse<DeploymentApp[]> | DeploymentApp[] | { data: DeploymentApp[] }>('/deployments')
      let list: DeploymentApp[] = []
      if (Array.isArray(res)) {
        list = res
      } else if (res && 'data' in res && Array.isArray(res.data)) {
        list = res.data
      }
      return list.map(d => ({
        ...d,
        strategy: d.strategy || 'RollingUpdate',
        canaryWeight: d.canaryWeight !== undefined ? d.canaryWeight : 0,
        canaryVersion: d.canaryVersion,
        blueGreenActive: d.blueGreenActive,
        blueVersion: d.blueVersion,
        greenVersion: d.greenVersion,
        revision: d.revision || 1,
        availableReplicas: d.availableReplicas !== undefined ? d.availableReplicas : d.replicas,
        updatedReplicas: d.updatedReplicas !== undefined ? d.updatedReplicas : d.replicas,
        readyReplicas: d.readyReplicas !== undefined ? d.readyReplicas : (d.status === 'healthy' ? d.replicas : 0),
      }))
    } catch {
      return []
    }
  },

  async listTemplates(): Promise<DeploymentTemplate[]> {
    try {
      const res = await api.get<ApiResponse<DeploymentTemplate[]> | DeploymentTemplate[] | { data: DeploymentTemplate[] }>('/deployments/templates')
      if (Array.isArray(res)) return res
      if (res && 'data' in res && Array.isArray(res.data)) return res.data
    } catch {
      // Return empty if endpoint fails
    }
    return []
  },

  async create(app: DeploymentApp): Promise<{ status: string; name: string }> {
    return api.post<{ status: string; name: string }>('/deployments', app)
  },

  async scale(payload: ScaleDeploymentPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/scale', payload)
  },

  async updateResources(payload: UpdateResourcesPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/resources', payload)
  },

  async restart(payload: RestartDeploymentPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/restart', payload)
  },

  async delete(payload: DeleteDeploymentPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/delete', payload)
  },

  async rollback(payload: RollbackDeploymentPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/rollback', payload)
  },

  async setCanaryWeight(payload: CanaryWeightPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/canary', payload)
  },

  async cutoverBlueGreen(payload: BlueGreenCutoverPayload): Promise<{ status: string }> {
    return api.post<{ status: string }>('/deployments/bluegreen', payload)
  },

  async pauseResume(payload: PauseResumePayload): Promise<{ status: string }> {
    return api.post<{ status: string }>(`/deployments/${payload.action}`, payload)
  },
}