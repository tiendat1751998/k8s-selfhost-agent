import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface LogEntry {
  time: string
  level: string
  namespace: string
  pod: string
  msg: string
  traceId?: string
}

export const useLogStore = defineStore('log', () => {
  const logs = ref<LogEntry[]>([])
  const isConnected = ref(false)
  const isPaused = ref(false)
  const socket = ref<WebSocket | null>(null)
  const maxBufferSize = 1000
  const reconnectAttempts = ref(0)

  function connect(namespace?: string, pod?: string) {
    if (socket.value) {
      socket.value.close()
    }

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const params = new URLSearchParams()
    if (namespace) params.append('namespace', namespace)
    if (pod) params.append('pod', pod)

    const token = typeof window !== 'undefined' ? localStorage.getItem('k8s_token') : null
    if (token) {
      params.append('token', token)
    }

    const wsUrl = `${proto}//${host}/api/v1/logs/stream?${params.toString()}`

    try {
      socket.value = new WebSocket(wsUrl)

      socket.value.onopen = () => {
        isConnected.value = true
        reconnectAttempts.value = 0
      }

      socket.value.onmessage = (event) => {
        if (isPaused.value) return
        try {
          const entry: LogEntry = JSON.parse(event.data)
          appendLog(entry)
        } catch {
          // Plain text fallback
          appendLog({
            time: new Date().toISOString().split('T')[1].slice(0, 12),
            level: 'INFO',
            namespace: namespace || 'default',
            pod: pod || 'system',
            msg: event.data,
          })
        }
      }

      socket.value.onclose = () => {
        isConnected.value = false
        // Soft reconnect
        if (reconnectAttempts.value < 5) {
          reconnectAttempts.value++
          setTimeout(() => connect(namespace, pod), 2000 * reconnectAttempts.value)
        }
      }

      socket.value.onerror = () => {
        isConnected.value = false
      }
    } catch {
      isConnected.value = false
    }
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
    connect,
    disconnect,
    appendLog,
    clear,
    togglePause,
  }
})
