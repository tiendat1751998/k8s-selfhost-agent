import { defineStore } from 'pinia'
import { ref } from 'vue'

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
}

export interface LogFilterOptions {
  node?: string
  service?: string
  container?: string
  namespace?: string
  pod?: string
  level?: string
  keyword?: string
}

export const useLogStore = defineStore('log', () => {
  const logs = ref<LogEntry[]>([])
  const isConnected = ref(false)
  const isPaused = ref(false)
  const socket = ref<WebSocket | null>(null)
  const maxBufferSize = 1000
  const reconnectAttempts = ref(0)
  const activeFilter = ref<LogFilterOptions>({})

  function normalizeOptions(options?: LogFilterOptions | string, podArg?: string): LogFilterOptions {
    if (typeof options === 'string') {
      return { namespace: options, pod: podArg }
    }
    return options ? { ...options } : {}
  }

  function connect(options?: LogFilterOptions | string, podArg?: string) {
    const opts = normalizeOptions(options, podArg)
    activeFilter.value = opts

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
    if (opts.keyword) params.append('keyword', opts.keyword)

    const token = typeof window !== 'undefined' ? localStorage.getItem('k8s_token') : null
    if (token) {
      params.append('token', token)
    }

    const queryStr = params.toString()
    const wsUrl = `${proto}//${host}/api/v1/logs/stream${queryStr ? `?${queryStr}` : ''}`

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
          const entry: LogEntry = {
            time: raw.time || raw.timestamp || new Date().toISOString().split('T')[1].slice(0, 12),
            level: (raw.level || raw.severity || 'INFO').toUpperCase(),
            namespace: raw.namespace || raw.ns || 'default',
            pod: raw.pod || raw.container || raw.service || 'system',
            container: raw.container || raw.pod,
            node: raw.node || raw.host,
            service: raw.service || raw.app || raw.container,
            msg: raw.msg || raw.message || raw.log || event.data,
            traceId: raw.traceId || raw.trace_id,
          }
          appendLog(entry)
        } catch {
          // Plain text fallback
          appendLog({
            time: new Date().toISOString().split('T')[1].slice(0, 12),
            level: 'INFO',
            namespace: opts.namespace || 'default',
            pod: opts.pod || 'system',
            node: opts.node,
            service: opts.service,
            msg: event.data,
          })
        }
      }

      socket.value.onclose = () => {
        isConnected.value = false
        // Soft reconnect
        if (reconnectAttempts.value < 5) {
          reconnectAttempts.value++
          setTimeout(() => connect(activeFilter.value), 2000 * reconnectAttempts.value)
        }
      }

      socket.value.onerror = () => {
        isConnected.value = false
      }
    } catch {
      isConnected.value = false
    }
  }

  function setFilter(options: LogFilterOptions) {
    const keys: (keyof LogFilterOptions)[] = ['node', 'service', 'container', 'namespace', 'pod', 'level', 'keyword']
    const hasChanged = keys.some((k) => (options[k] || '') !== (activeFilter.value[k] || ''))
    if (!hasChanged && socket.value && socket.value.readyState === WebSocket.OPEN) {
      return
    }
    connect(options)
  }

  function disconnect() {
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
    if (logs.value.length > maxBufferSize) {
      logs.value.shift()
    }
  }

  function clear() {
    logs.value = []
  }

  function togglePause() {
    isPaused.value = !isPaused.value
  }

  return {
    logs,
    isConnected,
    isPaused,
    activeFilter,
    connect,
    setFilter,
    disconnect,
    appendLog,
    clear,
    togglePause,
  }
})
