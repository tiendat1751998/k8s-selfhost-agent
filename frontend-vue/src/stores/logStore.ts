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
import { parseWebSocketFrame, mapRawToLogEntry } from '../utils/logBatchParser'

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

export function matchesTarget(entry: LogEntry, opts: LogFilterOptions): boolean {
  if (opts.node) {
    const q = opts.node.toLowerCase()
    const n = (entry.node || entry.attributes?.node || entry.attributes?.node_name || entry.pod || '').toLowerCase()
    return n.includes(q)
  }
  const q = (opts.service || opts.container || opts.pod || '').toLowerCase()
  if (!q) return true
  const s = (entry.service || entry.container || '').toLowerCase()
  if (q === 'postgres_db' || q === 'postgres') {
    return s === 'postgres_db' || s === 'postgres' || s.startsWith('postgres')
  }
  if (q === 'dbus' || q === 'dbus.service') {
    return s === 'dbus' || s === 'dbus.service'
  }
  if (q === 'db' && (s === 'dbus' || s === 'dbus.service' || s.startsWith('dbus'))) {
    return false
  }
  return s.includes(q)
}

export function getLogFingerprint(entry: LogEntry): string {
  const service = entry.service || entry.pod || ''
  return `${entry.time}_${service}_${entry.msg}`
}

export const useLogStore = defineStore('log', () => {
  const logs = ref<LogEntry[]>([])
  const isConnected = ref(false)
  const isPaused = ref(false)
  const socket = ref<WebSocket | null>(null)
  const maxBufferSize = ref(10000)

  // Drawer & Modal Reactive State
  const traceDrawerOpen = ref(false)
  const selectedTraceId = ref<string | null>(null)
  const contextModalOpen = ref(false)
  const contextTargetEntry = ref<LogEntry | null>(null)

  // Big Data Telemetry
  const droppedLogsCount = ref(0)
  const streamRate = ref(0)

  function setMaxBufferSize(size: number) {
    const parsed = Number(size)
    if (isNaN(parsed) || parsed < 1000) {
      maxBufferSize.value = 1000
    } else if (parsed > 100000) {
      maxBufferSize.value = 100000
    } else {
      maxBufferSize.value = parsed
    }
    if (logs.value.length > maxBufferSize.value) {
      logs.value = logs.value.slice(-maxBufferSize.value)
    }
  }

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

  function scheduleReconnect() {
    clearReconnectTimer()
    if (reconnectAttempts.value < 10) {
      reconnectAttempts.value++
      reconnectTimer = setTimeout(() => {
        connect(activeFilter.value, undefined, true)
      }, 2500)
    } else {
      if (socket.value) {
        socket.value.onclose = null
        socket.value.onerror = null
        socket.value.close()
        socket.value = null
      }
    }
  }

  function connect(options?: LogFilterOptions | string, podArg?: string, isReconnect = false) {
    clearReconnectTimer()
    if (!isReconnect) {
      reconnectAttempts.value = 0
    }
    const opts = normalizeOptions(options, podArg)
    activeFilter.value = opts

    const hasMatchingLogs = logs.value.length > 0 && logs.value.some((l) => matchesTarget(l, opts))
    if (!hasMatchingLogs) {
      logs.value = []
    }

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
        clearReconnectTimer()
      }
      socket.value.onmessage = (event) => {
        if (isPaused.value) return
        const parsed = parseWebSocketFrame(event.data, opts.namespace || 'default', opts.service)
        if (parsed.type === 'telemetry') {
          droppedLogsCount.value = parsed.droppedCount
          streamRate.value = parsed.streamRate
        } else if (parsed.type === 'logs') {
          appendLogBatch(parsed.entries)
        }
      }
      socket.value.onclose = () => {
        isConnected.value = false
        scheduleReconnect()
      }
      socket.value.onerror = () => { isConnected.value = false }
    } catch {
      isConnected.value = false
      scheduleReconnect()
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
      socket.value.onerror = null
      socket.value.close()
      socket.value = null
    }
    isConnected.value = false
    reconnectAttempts.value = 0
  }

  function addLogEntry(entry: LogEntry) {
    if (logs.value.length > 0) {
      const last = logs.value[logs.value.length - 1]
      if (
        last.time === entry.time &&
        last.msg === entry.msg &&
        (last.service || last.pod || '') === (entry.service || entry.pod || '')
      ) {
        return
      }
    }
    logs.value.push(entry)
    if (logs.value.length > maxBufferSize.value) {
      logs.value = logs.value.slice(-maxBufferSize.value)
    }
  }

  function appendLogBatch(entries: LogEntry[]) {
    if (!entries || entries.length === 0) return
    const valid: LogEntry[] = []
    let last = logs.value.length > 0 ? logs.value[logs.value.length - 1] : null

    for (let i = 0; i < entries.length; i++) {
      const cur = entries[i]
      if (
        last &&
        last.time === cur.time &&
        last.msg === cur.msg &&
        (last.service || last.pod || '') === (cur.service || cur.pod || '')
      ) {
        continue
      }
      valid.push(cur)
      last = cur
    }

    if (valid.length === 0) return
    logs.value.push(...valid)
    if (logs.value.length > maxBufferSize.value) {
      logs.value = logs.value.slice(-maxBufferSize.value)
    }
  }

  const appendLog = addLogEntry

  function clear() {
    logs.value = []
  }

  function togglePause() {
    isPaused.value = !isPaused.value
  }

  function openTraceDrawer(traceId: string) {
    selectedTraceId.value = traceId
    traceDrawerOpen.value = true
  }

  function closeTraceDrawer() {
    traceDrawerOpen.value = false
    selectedTraceId.value = null
  }

  function openContextModal(entry: LogEntry) {
    contextTargetEntry.value = entry
    contextModalOpen.value = true
  }

  function closeContextModal() {
    contextModalOpen.value = false
    contextTargetEntry.value = null
  }

  function resetDroppedLogsCount() {
    droppedLogsCount.value = 0
  }

  async function fetchHistoricalLogs(
    filter: LogFilterParams & { service?: string; container?: string } = {},
    append: boolean = false
  ): Promise<LogSearchResult> {
    isHistoricalLoading.value = true
    try {
      const queryParams: LogFilterParams = {
        ...filter,
        container_name: filter.container_name || filter.container || (filter.service && filter.service !== 'all' ? filter.service : undefined),
      }
      const res = await searchLogs(queryParams)
      const rawEntries = res.entries || []
      const mapped: LogEntry[] = []

      for (let i = 0; i < rawEntries.length; i++) {
        const item = mapRawToLogEntry(rawEntries[i], filter.namespace || 'default', filter.service)
        if (item) mapped.push(item)
      }

      mapped.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())

      if (append) {
        const existingFingerprints = new Set(logs.value.map(getLogFingerprint))
        const uniqueMapped = mapped.filter((entry) => !existingFingerprints.has(getLogFingerprint(entry)))
        const combined = [...logs.value, ...uniqueMapped]
        combined.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
        logs.value = combined.slice(-maxBufferSize.value)
      } else {
        if (logs.value.length === 0) {
          logs.value = mapped.slice(-maxBufferSize.value)
        } else {
          const mappedFingerprints = new Set(mapped.map(getLogFingerprint))
          const existingLiveLogs = logs.value.filter((entry) => !mappedFingerprints.has(getLogFingerprint(entry)))
          const combined = [...mapped, ...existingLiveLogs]
          combined.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
          logs.value = combined.slice(-maxBufferSize.value)
        }
      }
      totalHistoricalCount.value = res.total_count || 0
      hasMoreHistorical.value = res.has_more || false
      return {
        ...res,
        entries: rawEntries.filter((r) => r.message && r.message.trim() !== '-- No entries --'),
      }
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
    reconnectAttempts,
    activeFilter,
    maxBufferSize,
    setMaxBufferSize,
    traceDrawerOpen,
    selectedTraceId,
    contextModalOpen,
    contextTargetEntry,
    droppedLogsCount,
    streamRate,
    histogram,
    isHistoricalLoading,
    isHistogramLoading,
    totalHistoricalCount,
    hasMoreHistorical,
    connect,
    setFilter,
    disconnect,
    addLogEntry,
    appendLog,
    appendLogBatch,
    clear,
    togglePause,
    openTraceDrawer,
    closeTraceDrawer,
    openContextModal,
    closeContextModal,
    resetDroppedLogsCount,
    fetchHistoricalLogs,
    fetchHistogram,
  }
})