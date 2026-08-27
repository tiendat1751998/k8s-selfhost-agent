import { api, type ApiResponse } from './client'

export interface HelmRelease {
  name: string
  namespace: string
  chart: string
  version?: string
  app_version?: string
  appVersion?: string
  status: string
  updated?: string
  revision?: number
  description?: string
  [key: string]: unknown
}

export interface HelmReleaseDetail extends HelmRelease {
  values?: string | Record<string, unknown>
  notes?: string
  manifest?: string
  raw_values?: Record<string, unknown>
}

export interface HelmRevisionHistory {
  revision: number
  status: string
  chart: string
  app_version?: string
  appVersion?: string
  updated: string
  description?: string
}

export interface HelmRepo {
  name: string
  url: string
}

export interface HelmChart {
  name: string
  repo: string
  version: string
  app_version?: string
  appVersion?: string
  description?: string
  icon?: string
  home?: string
  keywords?: string[]
  sources?: string[]
  created?: string
  deprecated?: boolean
}

export interface InstallReleaseRequest {
  releaseName: string
  chart: string
  repo?: string
  version?: string
  namespace: string
  values?: string
  createNamespace?: boolean
}

export interface UpgradeReleaseRequest {
  chart?: string
  repo?: string
  version?: string
  values?: string
  namespace?: string
  resetValues?: boolean
  reuseValues?: boolean
}

export interface ChartValuesResponse {
  values: string
}

function unwrapList<T>(res: ApiResponse<T[]> | T[]): T[] {
  if (Array.isArray(res)) return res
  if (res && typeof res === 'object' && 'data' in res && Array.isArray(res.data)) {
    return res.data
  }
  return []
}

function unwrapItem<T>(res: ApiResponse<T> | T): T {
  if (res && typeof res === 'object') {
    if ('data' in res && res.data && typeof res.data === 'object') {
      return res.data as T
    }
  }
  return res as T
}

export const helmApi = {
  // Releases
  async listReleases(cluster: string, namespace?: string): Promise<HelmRelease[]> {
    const params: Record<string, string> = {}
    if (namespace && namespace !== 'all') {
      params.namespace = namespace
    }
    const res = await api.get<ApiResponse<HelmRelease[]> | HelmRelease[]>(
      `/helm/${encodeURIComponent(cluster)}/releases`,
      params
    )
    return unwrapList(res)
  },

  async getRelease(cluster: string, name: string, namespace?: string): Promise<HelmReleaseDetail> {
    const params: Record<string, string> = {}
    if (namespace && namespace !== 'all') {
      params.namespace = namespace
    }
    const res = await api.get<ApiResponse<HelmReleaseDetail> | HelmReleaseDetail>(
      `/helm/${encodeURIComponent(cluster)}/releases/${encodeURIComponent(name)}`,
      params
    )
    return unwrapItem(res)
  },

  async installRelease(cluster: string, req: InstallReleaseRequest): Promise<HelmRelease> {
    const res = await api.post<ApiResponse<HelmRelease> | HelmRelease>(
      `/helm/${encodeURIComponent(cluster)}/releases`,
      req
    )
    return unwrapItem(res)
  },

  async upgradeRelease(cluster: string, name: string, req: UpgradeReleaseRequest, namespace?: string): Promise<HelmRelease> {
    const query = namespace && namespace !== 'all' ? `?namespace=${encodeURIComponent(namespace)}` : ''
    const res = await api.put<ApiResponse<HelmRelease> | HelmRelease>(
      `/helm/${encodeURIComponent(cluster)}/releases/${encodeURIComponent(name)}${query}`,
      req
    )
    return unwrapItem(res)
  },

  async uninstallRelease(cluster: string, name: string, namespace?: string): Promise<{ success: boolean; message?: string }> {
    const query = namespace && namespace !== 'all' ? `?namespace=${encodeURIComponent(namespace)}` : ''
    const res = await api.delete<ApiResponse<{ success: boolean; message?: string }> | { success: boolean; message?: string }>(
      `/helm/${encodeURIComponent(cluster)}/releases/${encodeURIComponent(name)}${query}`
    )
    return unwrapItem(res)
  },

  async rollbackRelease(cluster: string, name: string, revision: number, namespace?: string): Promise<HelmRelease> {
    const query = namespace && namespace !== 'all' ? `?namespace=${encodeURIComponent(namespace)}` : ''
    const res = await api.post<ApiResponse<HelmRelease> | HelmRelease>(
      `/helm/${encodeURIComponent(cluster)}/releases/${encodeURIComponent(name)}/rollback${query}`,
      { revision, namespace }
    )
    return unwrapItem(res)
  },

  async getReleaseHistory(cluster: string, name: string, namespace?: string): Promise<HelmRevisionHistory[]> {
    const params: Record<string, string> = {}
    if (namespace && namespace !== 'all') {
      params.namespace = namespace
    }
    const res = await api.get<ApiResponse<HelmRevisionHistory[]> | HelmRevisionHistory[]>(
      `/helm/${encodeURIComponent(cluster)}/releases/${encodeURIComponent(name)}/history`,
      params
    )
    return unwrapList(res)
  },

  // Repositories
  async listRepos(): Promise<HelmRepo[]> {
    const res = await api.get<ApiResponse<HelmRepo[]> | HelmRepo[]>('/helm/repos')
    return unwrapList(res)
  },

  async addRepo(name: string, url: string): Promise<HelmRepo> {
    const res = await api.post<ApiResponse<HelmRepo> | HelmRepo>('/helm/repos', { name, url })
    return unwrapItem(res)
  },

  async removeRepo(name: string): Promise<{ success: boolean }> {
    const res = await api.delete<ApiResponse<{ success: boolean }> | { success: boolean }>(
      `/helm/repos/${encodeURIComponent(name)}`
    )
    return unwrapItem(res)
  },

  async updateRepos(): Promise<{ success: boolean; message?: string }> {
    const res = await api.put<ApiResponse<{ success: boolean; message?: string }> | { success: boolean; message?: string }>(
      '/helm/repos'
    )
    return unwrapItem(res)
  },

  // Charts
  async searchCharts(keyword?: string): Promise<HelmChart[]> {
    const params: Record<string, string> = {}
    if (keyword && keyword.trim()) {
      params.keyword = keyword.trim()
    }
    const res = await api.get<ApiResponse<HelmChart[]> | HelmChart[]>('/helm/charts', params)
    return unwrapList(res)
  },

  async getChartValues(repo: string, chart: string, version?: string): Promise<ChartValuesResponse> {
    const params: Record<string, string> = {}
    if (version && version.trim()) {
      params.version = version.trim()
    }
    const res = await api.get<ApiResponse<ChartValuesResponse> | ChartValuesResponse>(
      `/helm/charts/${encodeURIComponent(repo)}/${encodeURIComponent(chart)}/values`,
      params
    )
    if (typeof res === 'string') {
      return { values: res }
    }
    return unwrapItem(res)
  }
}