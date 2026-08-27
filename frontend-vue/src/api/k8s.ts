import { api, type ApiResponse } from './client'

export interface K8sResource {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace?: string
    creationTimestamp?: string
    labels?: Record<string, string>
    annotations?: Record<string, string>
    uid?: string
    resourceVersion?: string
    generation?: number
  }
  spec?: Record<string, unknown>
  data?: Record<string, string>  // for ConfigMaps/Secrets
  stringData?: Record<string, string>
  type?: string
  status?: Record<string, unknown>
}

export interface K8sNamespace {
  name: string
  status: string
  createdAt: string
  labels?: Record<string, string>
}


export interface K8sEventObjectReference {
  kind?: string
  namespace?: string
  name?: string
  uid?: string
  apiVersion?: string
  resourceVersion?: string
  fieldPath?: string
}

export interface K8sEventSource {
  component?: string
  host?: string
}

export interface K8sEventSeries {
  count?: number
  lastObservedTime?: string
  state?: string
}

export interface K8sEvent {
  apiVersion?: string
  kind?: string
  metadata: {
    name: string
    namespace?: string
    creationTimestamp?: string
    uid?: string
    resourceVersion?: string
    labels?: Record<string, string>
    annotations?: Record<string, string>
  }
  involvedObject?: K8sEventObjectReference
  reason?: string
  message?: string
  source?: K8sEventSource
  firstTimestamp?: string
  lastTimestamp?: string
  count?: number
  type?: string
  eventTime?: string
  series?: K8sEventSeries
  action?: string
  reportingComponent?: string
  reportingInstance?: string
}

export interface NodeTaint {
  key: string
  value?: string
  effect: string
  timeAdded?: string
}

export interface NodeCondition {
  type: string
  status: string
  lastHeartbeatTime?: string
  lastTransitionTime?: string
  reason?: string
  message?: string
}

export interface NodeSystemInfo {
  machineID?: string
  systemUUID?: string
  bootID?: string
  kernelVersion?: string
  osImage?: string
  containerRuntimeVersion?: string
  kubeletVersion?: string
  kubeProxyVersion?: string
  operatingSystem?: string
  architecture?: string
}
export type ResourceKind =
  | 'pods'
  | 'configmaps'
  | 'secrets'
  | 'deployments'
  | 'services'
  | 'ingresses'
  | 'statefulsets'
  | 'daemonsets'
  | 'jobs'
  | 'cronjobs'
  | 'persistentvolumeclaims'
  | 'storageclasses'
  | 'nodes'
  | 'events'

function unwrapResource(res: ApiResponse<K8sResource> | K8sResource): K8sResource {
  if (res && typeof res === 'object') {
    if ('apiVersion' in res && 'kind' in res && 'metadata' in res) {
      return res as K8sResource
    }
    if ('data' in res && res.data && typeof res.data === 'object' && 'apiVersion' in res.data) {
      return res.data as K8sResource
    }
  }
  return res as unknown as K8sResource
}

function unwrapResourceList(res: ApiResponse<K8sResource[]> | K8sResource[]): K8sResource[] {
  if (Array.isArray(res)) return res
  if (res && typeof res === 'object' && 'data' in res && Array.isArray(res.data)) {
    return res.data
  }
  return []
}

function unwrapNamespaceList(res: ApiResponse<K8sNamespace[]> | K8sNamespace[]): K8sNamespace[] {
  if (Array.isArray(res)) return res
  if (res && typeof res === 'object' && 'data' in res && Array.isArray(res.data)) {
    return res.data
  }
  return []
}

function unwrapNamespace(res: ApiResponse<K8sNamespace> | K8sNamespace): K8sNamespace {
  if (res && typeof res === 'object') {
    if ('name' in res && typeof res.name === 'string') {
      return res as K8sNamespace
    }
    if ('data' in res && res.data && typeof res.data === 'object' && 'name' in res.data) {
      return res.data as K8sNamespace
    }
  }
  return res as unknown as K8sNamespace
}

export const k8sApi = {
  // Namespaces
  async listNamespaces(cluster: string): Promise<K8sNamespace[]> {
    const res = await api.get<ApiResponse<K8sNamespace[]> | K8sNamespace[]>(
      `/k8s/${encodeURIComponent(cluster)}/namespaces`
    )
    return unwrapNamespaceList(res)
  },

  async createNamespace(cluster: string, name: string): Promise<K8sNamespace> {
    const res = await api.post<ApiResponse<K8sNamespace> | K8sNamespace>(
      `/k8s/${encodeURIComponent(cluster)}/namespaces`,
      { name }
    )
    return unwrapNamespace(res)
  },

  async deleteNamespace(cluster: string, name: string): Promise<void> {
    await api.delete<void>(
      `/k8s/${encodeURIComponent(cluster)}/namespaces/${encodeURIComponent(name)}`
    )
  },

  // Generic resources
  async listResources(
    cluster: string,
    kind: ResourceKind,
    ns?: string
  ): Promise<K8sResource[]> {
    const query = ns ? `?ns=${encodeURIComponent(ns)}` : ''
    const path = `/k8s/${encodeURIComponent(cluster)}/resources/${encodeURIComponent(kind)}${query}`
    const res = await api.get<ApiResponse<K8sResource[]> | K8sResource[]>(path)
    return unwrapResourceList(res)
  },

  async getResource(
    cluster: string,
    kind: ResourceKind,
    name: string,
    ns?: string
  ): Promise<K8sResource> {
    const query = ns ? `?ns=${encodeURIComponent(ns)}` : ''
    const path = `/k8s/${encodeURIComponent(cluster)}/resources/${encodeURIComponent(kind)}/${encodeURIComponent(name)}${query}`
    const res = await api.get<ApiResponse<K8sResource> | K8sResource>(path)
    return unwrapResource(res)
  },

  async createResource(
    cluster: string,
    kind: ResourceKind,
    manifest: Record<string, unknown>,
    ns?: string
  ): Promise<K8sResource> {
    const query = ns ? `?ns=${encodeURIComponent(ns)}` : ''
    const path = `/k8s/${encodeURIComponent(cluster)}/resources/${encodeURIComponent(kind)}${query}`
    const res = await api.post<ApiResponse<K8sResource> | K8sResource>(path, manifest)
    return unwrapResource(res)
  },

  async updateResource(
    cluster: string,
    kind: ResourceKind,
    name: string,
    manifest: Record<string, unknown>,
    ns?: string
  ): Promise<K8sResource> {
    const query = ns ? `?ns=${encodeURIComponent(ns)}` : ''
    const path = `/k8s/${encodeURIComponent(cluster)}/resources/${encodeURIComponent(kind)}/${encodeURIComponent(name)}${query}`
    const res = await api.put<ApiResponse<K8sResource> | K8sResource>(path, manifest)
    return unwrapResource(res)
  },

  async deleteResource(
    cluster: string,
    kind: ResourceKind,
    name: string,
    ns?: string
  ): Promise<void> {
    const query = ns ? `?ns=${encodeURIComponent(ns)}` : ''
    const path = `/k8s/${encodeURIComponent(cluster)}/resources/${encodeURIComponent(kind)}/${encodeURIComponent(name)}${query}`
    await api.delete<void>(path)
  },

  // Events Listing
  async listEvents(
    cluster: string,
    params?: {
      namespace?: string
      kind?: string
      name?: string
      type?: string
      limit?: number
    }
  ): Promise<K8sEvent[]> {
    const q = new URLSearchParams()
    if (params?.namespace && params.namespace !== 'all') q.set('ns', params.namespace)
    if (params?.kind) q.set('kind', params.kind)
    if (params?.name) q.set('name', params.name)
    if (params?.type && params.type !== 'all') q.set('type', params.type)
    if (params?.limit) q.set('limit', String(params.limit))
    const query = q.toString() ? '?' + q.toString() : ''
    const path = '/k8s/' + encodeURIComponent(cluster) + '/resources/events' + query
    const res = await api.get<ApiResponse<K8sEvent[]> | K8sEvent[]>(path)
    return (unwrapResourceList(res as unknown as K8sResource[]) as unknown) as K8sEvent[]
  },

  // Node Management Operations
  async cordonNode(
    cluster: string,
    name: string
  ): Promise<{ status?: string; message?: string }> {
    const path = '/k8s/' + encodeURIComponent(cluster) + '/nodes/' + encodeURIComponent(name) + '/cordon'
    return api.post<{ status?: string; message?: string }>(path, {})
  },

  async uncordonNode(
    cluster: string,
    name: string
  ): Promise<{ status?: string; message?: string }> {
    const path = '/k8s/' + encodeURIComponent(cluster) + '/nodes/' + encodeURIComponent(name) + '/uncordon'
    return api.post<{ status?: string; message?: string }>(path, {})
  },

  async drainNode(
    cluster: string,
    name: string,
    options?: {
      gracePeriodSeconds?: number
      ignoreDaemonSets?: boolean
      deleteEmptyDirData?: boolean
      force?: boolean
    }
  ): Promise<{ status?: string; message?: string }> {
    const path = '/k8s/' + encodeURIComponent(cluster) + '/nodes/' + encodeURIComponent(name) + '/drain'
    return api.post<{ status?: string; message?: string }>(path, options || {})
  },

  async updateNodeTaints(
    cluster: string,
    name: string,
    taints: NodeTaint[]
  ): Promise<{ status?: string; message?: string }> {
    const path = '/k8s/' + encodeURIComponent(cluster) + '/nodes/' + encodeURIComponent(name) + '/taints'
    return api.put<{ status?: string; message?: string }>(path, { taints })
  },

  async updateNodeLabels(
    cluster: string,
    name: string,
    labels: Record<string, string>
  ): Promise<{ status?: string; message?: string }> {
    const path = '/k8s/' + encodeURIComponent(cluster) + '/nodes/' + encodeURIComponent(name) + '/labels'
    return api.put<{ status?: string; message?: string }>(path, { labels })
  },

  // Raw YAML
  async applyYAML(
    cluster: string,
    yaml: string,
    ns?: string
  ): Promise<{ message: string }> {
    const res = await api.post<{ message?: string } | ApiResponse<{ message: string }>>(
      `/k8s/${encodeURIComponent(cluster)}/apply`,
      { yaml, namespace: ns }
    )
    if (res && typeof res === 'object' && 'data' in res && res.data) {
      return res.data
    }
    return {
      message:
        (res as { message?: string }).message || 'YAML manifest applied successfully',
    }
  },
}
