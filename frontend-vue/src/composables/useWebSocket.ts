import { onMounted, onUnmounted } from 'vue'
import { useAppStore } from '../stores/app'
import type { SystemOverview } from '../api/overview'

export interface UseWebSocketOptions {
  url?: string
  onMetrics?: (metrics: SystemOverview) => void
  onIncident?: (incident: unknown) => void
  onLog?: (log: unknown) => void
}

export function useWebSocket(options?: UseWebSocketOptions | string) {
  const store = useAppStore()
  let ws: WebSocket | null = null
  let watchdog: ReturnType<typeof setTimeout> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let isUnmounted = false

  const resolvedOptions: UseWebSocketOptions = typeof options === 'string'
    ? { url: options }
    : (options || {})

  function getWsUrl(): string {
    if (resolvedOptions.url) return resolvedOptions.url
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const token = typeof window !== 'undefined' ? localStorage.getItem('k8s_token') : null
    const query = token ? `?token=${encodeURIComponent(token)}` : ''
    return `${proto}//${host}/ws${query}`
  }

  function connect() {
    if (isUnmounted) return
    store.setConnection('connecting')
    try {
      const url = getWsUrl()
      ws = new WebSocket(url)
      
      ws.onopen = () => {
        if (isUnmounted) {
          ws?.close()
          return
        }
        store.setConnection('online')
        resetWatchdog()
      }

      ws.onmessage = (e) => {
        resetWatchdog()
        try {
          const msg = JSON.parse(e.data)
          if (msg.type === 'incident') {
            store.addIncident(msg.data)
            resolvedOptions.onIncident?.(msg.data)
          } else if (msg.type === 'log') {
            store.addLog(msg.data)
            resolvedOptions.onLog?.(msg.data)
          } else if (msg.type === 'metrics') {
            store.setMetrics(msg.data)
            resolvedOptions.onMetrics?.(msg.data)
          }
        } catch (err: unknown) {
          console.warn('Failed to parse WebSocket message:', err)
        }
      }

      ws.onerror = () => {
        store.setConnection('error')
      }

      ws.onclose = () => {
        if (isUnmounted) return
        store.setConnection('offline')
        if (reconnectTimer) clearTimeout(reconnectTimer)
        reconnectTimer = setTimeout(() => {
          if (!isUnmounted) connect()
        }, 3000)
      }
    } catch (err: unknown) {
      console.warn('WebSocket connection error:', err)
      store.setConnection('offline')
      if (reconnectTimer) clearTimeout(reconnectTimer)
      reconnectTimer = setTimeout(() => {
        if (!isUnmounted) connect()
      }, 3000)
    }
  }

  function resetWatchdog() {
    if (watchdog) clearTimeout(watchdog)
    watchdog = setTimeout(() => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.close()
      }
    }, 45000)
  }

  function disconnect() {
    isUnmounted = true
    if (watchdog) {
      clearTimeout(watchdog)
      watchdog = null
    }
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (ws) {
      ws.onopen = null
      ws.onmessage = null
      ws.onerror = null
      ws.onclose = null
      ws.close()
      ws = null
    }
  }

  onMounted(() => {
    isUnmounted = false
    connect()
  })

  onUnmounted(() => {
    disconnect()
  })

  return { ws, connect, disconnect }
}

