import { api } from './client'

export interface LogFilterParams {
  query?: string
  service?: string
  namespace?: string
  pod_name?: string
  container_name?: string
  container?: string
  node?: string
  stream?: string
  log_level?: string
  start_time?: string
  end_time?: string
  limit?: number
  offset?: number
  cluster_id?: string
  attributes?: Record<string, string>
}

export interface HistogramParams {
  query?: string
  service?: string
  namespace?: string
  pod_name?: string
  container_name?: string
  node?: string
  log_level?: string
  interval_seconds?: number
  start_time?: string
  end_time?: string
  attributes?: Record<string, string>
}

export interface RawLogEntry {
  timestamp: string
  tenant_id?: string
  cluster_id?: string
  namespace?: string
  pod_name?: string
  container_name?: string
  container?: string
  node?: string
  host?: string
  service?: string
  app?: string
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
  time_bucket?: string
  timestamp?: string
  total_count?: number
  total?: number
  level_count?: Record<string, number>
  error_count?: number
  warn_count?: number
  info_count?: number
  debug_count?: number
}

export function normalizeHistogramBuckets(raw: unknown): LogAggregationBucket[] {
  const buckets = Array.isArray(raw)
    ? raw
    : Array.isArray((raw as any)?.data)
    ? (raw as any).data
    : Array.isArray((raw as any)?.buckets)
    ? (raw as any).buckets
    : []
  return buckets
}

export interface LogContextParams {
  service?: string
  container?: string
  container_name?: string
  timestamp: string
  window?: number
}

export async function searchLogs(filter: LogFilterParams = {}): Promise<LogSearchResult> {
  const params: Record<string, string | number> = {}
  if (filter.query) params.query = filter.query
  if (filter.service) {
    params.service = filter.service
    params.container_name = filter.container_name || filter.service
  } else if (filter.container_name) {
    params.container_name = filter.container_name
  } else if (filter.container) {
    params.container_name = filter.container
  }
  if (filter.namespace) params.namespace = filter.namespace
  if (filter.pod_name) params.pod_name = filter.pod_name
  if (filter.node) params.node = filter.node
  if (filter.attributes?.node) params.node = filter.attributes.node
  if (filter.stream) params.stream = filter.stream
  if (filter.log_level) params.log_level = filter.log_level
  if (filter.start_time) params.start_time = filter.start_time
  if (filter.end_time) params.end_time = filter.end_time
  if (filter.limit !== undefined) params.limit = filter.limit
  if (filter.offset !== undefined) params.offset = filter.offset
  if (filter.cluster_id) params.cluster_id = filter.cluster_id

  return api.get<LogSearchResult>('/logs/search', params)
}

export async function getLogHistogram(params: HistogramParams = {}): Promise<LogAggregationBucket[]> {
  const queryParams: Record<string, string | number> = {}
  if (params.query) queryParams.query = params.query
  if (params.service) {
    queryParams.service = params.service
    queryParams.container_name = params.container_name || params.service
  } else if (params.container_name) {
    queryParams.container_name = params.container_name
  }
  if (params.namespace) queryParams.namespace = params.namespace
  if (params.pod_name) queryParams.pod_name = params.pod_name
  if (params.node) queryParams.node = params.node
  if (params.attributes?.node) queryParams.node = params.attributes.node
  if (params.log_level) queryParams.log_level = params.log_level
  if (params.interval_seconds !== undefined) queryParams.interval_seconds = params.interval_seconds
  if (params.start_time) queryParams.start_time = params.start_time
  if (params.end_time) queryParams.end_time = params.end_time

  const raw = await api.get<unknown>('/logs/histogram', queryParams)
  return normalizeHistogramBuckets(raw)
}

export async function getTraceLogs(traceId: string): Promise<LogSearchResult> {
  return api.get<LogSearchResult>(`/logs/trace/${encodeURIComponent(traceId)}`)
}

export async function getContextLogs(params: LogContextParams): Promise<LogSearchResult> {
  const queryParams: Record<string, string | number> = {
    timestamp: params.timestamp,
    window: params.window ?? 50,
  }
  const s = params.service || params.container_name || params.container
  if (s) queryParams.service = s

  return api.get<LogSearchResult>('/logs/context', queryParams)
}

export interface ErrorRadarGroup {
  node?: string
  service: string
  level: string
  fingerprint: string
  sample_message: string
  count: number
  first_seen: string
  last_seen: string
}

export interface ErrorRadarResponse {
  groups: ErrorRadarGroup[]
  total_groups: number
  total_occurrences: number
  took_ms: number
}

export async function getErrorRadar(params: {
  node?: string
  service?: string
  level?: string
  since?: string
  until?: string
  limit?: number
} = {}): Promise<ErrorRadarResponse> {
  const query: Record<string, string | number> = {}
  if (params.node) query.node = params.node
  if (params.service) query.service = params.service
  if (params.level) query.level = params.level
  if (params.since) query.since = params.since
  if (params.until) query.until = params.until
  if (params.limit) query.limit = params.limit
  return api.get<ErrorRadarResponse>('/logs/errors', query)
}