import { onMounted, onUnmounted } from 'vue'
import { useAppStore } from '../stores/app'

export function useWebSocket(url: string = 'ws://' + window.location.host + '/ws') {
  const store = useAppStore()
  let ws: WebSocket | null = null
  let watchdog: ReturnType<typeof setTimeout> | null = null

  function connect() {
    store.setConnection('connecting')
    ws = new WebSocket(url)
    ws.onopen = () => {
      store.setConnection('online')
      resetWatchdog()
    }
    ws.onmessage = (e) => {
      resetWatchdog()
      try {
        const msg = JSON.parse(e.data)
        if (msg.type === 'incident') store.addIncident(msg.data)
        if (msg.type === 'log') store.addLog(msg.data)
      } catch (err: unknown) {
        console.warn('Failed to parse WebSocket message:', err)
      }
    }
    ws.onclose = () => {
      store.setConnection('offline')
      setTimeout(connect, 2000)
    }
  }

  function resetWatchdog() {
    if (watchdog) clearTimeout(watchdog)
    watchdog = setTimeout(() => ws?.close(), 45000)
  }

  onMounted(() => {
    connect()
  })

  onUnmounted(() => {
    if (ws) {
      ws.onclose = null
      ws.close()
    }
    if (watchdog) clearTimeout(watchdog)
  })

  return { ws }
}
