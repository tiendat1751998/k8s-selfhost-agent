import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  searchLogs,
  getLogHistogram,
  type LogFilterParams,
  type HistogramParams,
  type LogAggregationBucket,
  type LogSearchResult,
} from '../api/logging'

export interface LogEntry {
  time: string
  level: string
  namespace: string
  pod: string
  container?: string
  node?: string
  service?: string
  msg: string
  traceId?: string
  stream?: string
  attributes?: Record<string, string>
}

export interface LogFilterOptions {
  node?: string
  service?: string
  container?: string
  namespace?: string
  pod?: string
  level?: string
  keyword?: string
  query?: string
}

export const useLogStore = defineStore('log', () => {
  const logs = ref<LogEntry[]>([])
  const isConnected = ref(false)
  const isPaused = ref(false)
  const socket = ref<WebSocket | null>(null)
  const maxBufferSize = 1000
  const reconnectAttempts = ref(0)
  const activeFilter = ref<LogFilterOptions>({})
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null

  // Historical Search & Histogram State
  const histogram = ref<LogAggregationBucket[]>([])
  const isHistoricalLoading = ref(false)
  const isHistogramLoading = ref(false)
  const totalHistoricalCount = ref(0)
  const hasMoreHistorical = ref(false)

  function normalizeOptions(options?: LogFilterOptions | string, podArg?: string): LogFilterOptions {
    if (typeof options === 'string') return { namespace: options, pod: podArg }
    return options ? { ...options } : {}
  }

  function clearReconnectTimer() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  function connect(options?: LogFilterOptions | string, podArg?: string) {
    clearReconnectTimer()
    const opts = normalizeOptions(options, podArg)
    activeFilter.value = opts
    logs.value = []

    if (socket.value) {
      socket.value.onclose = null
      socket.value.close()
      socket.value = null
    }

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const params = new URLSearchParams()
    if (opts.node) params.append('node', opts.node)
    if (opts.service) params.append('service', opts.service)
    if (opts.container) params.append('container', opts.container)
    if (opts.namespace) params.append('namespace', opts.namespace)
    if (opts.pod) params.append('pod', opts.pod)
    if (opts.level) params.append('level', opts.level)
    const q = opts.query || opts.keyword
    if (q) params.append('query', q)

    const token = typeof window !== 'undefined' ? localStorage.getItem('k8s_token') : null
    if (token) params.append('token', token)

    const queryStr = params.toString()
    const wsUrl = `${proto}//${host}/api/v1/logs/stream${queryStr ? '?' + queryStr : ''}`

    try {
      socket.value = new WebSocket(wsUrl)
      socket.value.onopen = () => {
        isConnected.value = true
        reconnectAttempts.value = 0
      }
      socket.value.onmessage = (event) => {
        if (isPaused.value) return
        try {
          const raw = JSON.parse(event.data)
          appendLog({
            time: raw.timestamp || raw.time || new Date().toISOString().split('T')[1].slice(0, 12),
            level: (raw.log_level || raw.level || raw.severity || 'INFO').toUpperCase(),
            namespace: raw.namespace || raw.ns || opts.namespace || 'default',
            pod: raw.pod_name || raw.pod || raw.container_name || raw.container || raw.service || 'system',
            container: raw.container_name || raw.container || raw.pod,
            node: raw.node || raw.host || raw.attributes?.node,
            service: raw.service || raw.app || raw.container_name || raw.container,
            msg: raw.message || raw.msg || raw.log || event.data,
            traceId: raw.traceId || raw.trace_id || raw.attributes?.trace_id || raw.attributes?.traceId,
            stream: raw.stream || 'stdout',
            attributes: raw.attributes,
          })
        } catch {
          appendLog({
            time: new Date().toISOString().split('T')[1].slice(0, 12),
            level: 'INFO',
            namespace: opts.namespace || 'default',
            pod: opts.pod || 'system',
            node: opts.node,
            service: opts.service,
            msg: event.data,
            stream: 'stdout',
          })
        }
      }
      socket.value.onclose = () => {
        isConnected.value = false
        if (reconnectAttempts.value < 5) {
          reconnectAttempts.value++
          clearReconnectTimer()
          reconnectTimer = setTimeout(() => connect(activeFilter.value), 2000 * reconnectAttempts.value)
        }
      }
      socket.value.onerror = () => { isConnected.value = false }
    } catch {
      isConnected.value = false
    }
  }

  function setFilter(options: LogFilterOptions) {
    const keys: (keyof LogFilterOptions)[] = ['node', 'service', 'container', 'namespace', 'pod', 'level', 'keyword', 'query']
    const hasChanged = keys.some((k) => (options[k] || '') !== (activeFilter.value[k] || ''))
    if (!hasChanged && socket.value && socket.value.readyState === WebSocket.OPEN) return
    connect(options)
  }

  function disconnect() {
    clearReconnectTimer()
    if (socket.value) {
      socket.value.onclose = null
      socket.value.close()
      socket.value = null
    }
    isConnected.value = false
    reconnectAttempts.value = 0
  }

  function appendLog(entry: LogEntry) {
    logs.value.push(entry)
    if (logs.value.length > maxBufferSize) logs.value.splice(0, logs.value.length - maxBufferSize)
  }

  function clear() { logs.value = [] }
  function togglePause() { isPaused.value = !isPaused.value }

  async function fetchHistoricalLogs(filter: LogFilterParams = {}, append: boolean = false): Promise<LogSearchResult> {
    isHistoricalLoading.value = true
    try {
      const res = await searchLogs(filter)
      const mapped: LogEntry[] = (res.entries || []).map((raw) => ({
        time: raw.timestamp || new Date().toISOString(),
        level: (raw.log_level || 'INFO').toUpperCase(),
        namespace: raw.namespace || 'default',
        pod: raw.pod_name || 'system',
        container: raw.container_name,
        node: raw.attributes?.node,
        service: raw.attributes?.service || raw.attributes?.app || raw.container_name,
        msg: raw.message || '',
        traceId: raw.attributes?.trace_id || raw.attributes?.traceId,
        stream: raw.stream || 'stdout',
        attributes: raw.attributes,
      }))
      if (append) {
        logs.value = [...logs.value, ...mapped].slice(-maxBufferSize)
      } else {
        logs.value = mapped.slice(0, maxBufferSize)
      }
      totalHistoricalCount.value = res.total_count || 0
      hasMoreHistorical.value = res.has_more || false
      return res
    } finally {
      isHistoricalLoading.value = false
    }
  }

  async function fetchHistogram(params: HistogramParams = {}): Promise<LogAggregationBucket[]> {
    isHistogramLoading.value = true
    try {
      const buckets = await getLogHistogram(params)
      histogram.value = buckets || []
      return histogram.value
    } finally {
      isHistogramLoading.value = false
    }
  }

  return {
    logs,
    isConnected,
    isPaused,
    activeFilter,
    maxBufferSize,
    histogram,
    isHistoricalLoading,
    isHistogramLoading,
    totalHistoricalCount,
    hasMoreHistorical,
    connect,
    setFilter,
    disconnect,
    appendLog,
    clear,
    togglePause,
    fetchHistoricalLogs,
    fetchHistogram,
  }
})
