import { api } from './client'

export interface LogFilterParams {
  query?: string
  search_text?: string
  namespace?: string
  pod_name?: string
  pod?: string
  container_name?: string
  container?: string
  stream?: string
  log_level?: string
  level?: string
  start_time?: string
  end_time?: string
  limit?: number
  offset?: number
  cluster_id?: string
}

export interface HistogramParams {
  query?: string
  search_text?: string
  namespace?: string
  pod_name?: string
  pod?: string
  container_name?: string
  container?: string
  log_level?: string
  interval_seconds?: number
  start_time?: string
  end_time?: string
}

export interface RawLogEntry {
  timestamp: string
  tenant_id?: string
  cluster_id?: string
  namespace?: string
  pod_name?: string
  container_name?: string
  stream?: string
  log_level?: string
  message?: string
  attributes?: Record<string, string>
}

export interface LogSearchResult {
  entries: RawLogEntry[]
  total_count: number
  has_more: boolean
}

export interface LogAggregationBucket {
  time_bucket: string
  total_count: number
  level_count?: Record<string, number>
}

export async function searchLogs(filter: LogFilterParams = {}): Promise<LogSearchResult> {
  const params: Record<string, string | number> = {}
  const q = filter.query || filter.search_text
  if (q) params.query = q
  if (filter.namespace) params.namespace = filter.namespace
  const pod = filter.pod_name || filter.pod
  if (pod) params.pod_name = pod
  const container = filter.container_name || filter.container
  if (container) params.container_name = container
  if (filter.stream) params.stream = filter.stream
  const level = filter.log_level || filter.level
  if (level) params.log_level = level
  if (filter.start_time) params.start_time = filter.start_time
  if (filter.end_time) params.end_time = filter.end_time
  if (filter.limit !== undefined) params.limit = filter.limit
  if (filter.offset !== undefined) params.offset = filter.offset
  if (filter.cluster_id) params.cluster_id = filter.cluster_id

  return api.get<LogSearchResult>('/logs/search', params)
}

export async function getLogHistogram(params: HistogramParams = {}): Promise<LogAggregationBucket[]> {
  const queryParams: Record<string, string | number> = {}
  const q = params.query || params.search_text
  if (q) queryParams.query = q
  if (params.namespace) queryParams.namespace = params.namespace
  const pod = params.pod_name || params.pod
  if (pod) queryParams.pod_name = pod
  const container = params.container_name || params.container
  if (container) queryParams.container_name = container
  if (params.log_level) queryParams.log_level = params.log_level
  if (params.interval_seconds !== undefined) queryParams.interval_seconds = params.interval_seconds
  if (params.start_time) queryParams.start_time = params.start_time
  if (params.end_time) queryParams.end_time = params.end_time

  return api.get<LogAggregationBucket[]>('/logs/histogram', queryParams)
}
