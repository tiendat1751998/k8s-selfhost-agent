import { ref, onMounted, onUnmounted, watch, nextTick, type Ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'

export interface PodTerminalProps {
  cluster: string
  pod: string
  namespace?: string
  container?: string
  containers?: string[]
}

export function usePodTerminal(
  props: PodTerminalProps,
  terminalContainer?: Ref<HTMLElement | null>
) {
  const selectedContainer = ref<string>(props.container || props.containers?.[0] || '')
  const connectionStatus = ref<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
  const errorMessage = ref<string | null>(null)
  const terminalContainerRef = terminalContainer || ref<HTMLElement | null>(null)

  let terminal: Terminal | null = null
  let fitAddon: FitAddon | null = null
  let ws: WebSocket | null = null
  let resizeObserver: ResizeObserver | null = null

  function buildWebSocketUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const ns = props.namespace || 'default'
    const containerParam = selectedContainer.value ? `&container=${encodeURIComponent(selectedContainer.value)}` : ''
    const token = localStorage.getItem('k8s_token')
    const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''

    return `${protocol}//${host}/api/v1/k8s/${encodeURIComponent(props.cluster)}/pods/${encodeURIComponent(props.pod)}/exec?namespace=${encodeURIComponent(ns)}${containerParam}&command=/bin/sh${tokenParam}`
  }

  function sendResize(cols?: number, rows?: number) {
    if (ws && ws.readyState === WebSocket.OPEN && terminal) {
      const c = cols ?? terminal.cols
      const r = rows ?? terminal.rows
      ws.send(JSON.stringify({ type: 'resize', cols: c, rows: r }))
    }
  }

  function handleFit() {
    if (fitAddon && terminal && terminalContainerRef.value) {
      try {
        fitAddon.fit()
        sendResize(terminal.cols, terminal.rows)
      } catch {
        // Ignore fit errors if element not visible
      }
    }
  }

  function connect() {
    cleanupWebSocket()
    connectionStatus.value = 'connecting'
    errorMessage.value = null

    if (terminal) {
      terminal.reset()
      terminal.writeln(`\x1b[36mConnecting to pod \x1b[1;32m${props.pod}\x1b[0;36m in namespace \x1b[1;33m${props.namespace}\x1b[0;36m...\x1b[0m\r\n`)
    }

    const url = buildWebSocketUrl()

    try {
      ws = new WebSocket(url)
    } catch (err: unknown) {
      connectionStatus.value = 'error'
      const msg = err instanceof Error ? err.message : 'Failed to create WebSocket connection'
      errorMessage.value = msg
      if (terminal) {
        terminal.writeln(`\r\n\x1b[31m[Error] Failed to connect: ${msg}\x1b[0m\r\n`)
      }
      return
    }

    ws.onopen = () => {
      connectionStatus.value = 'connected'
      errorMessage.value = null
      if (terminal) {
        terminal.writeln(`\x1b[32m[Connected] Interactive session established with /bin/sh\x1b[0m\r\n`)
        handleFit()
      }
    }

    ws.onmessage = (event: MessageEvent) => {
      if (terminal && typeof event.data === 'string') {
        terminal.write(event.data)
      } else if (terminal && event.data instanceof Blob) {
        event.data.text().then(text => {
          terminal?.write(text)
        }).catch(() => {
          // Blob read error
        })
      }
    }

    ws.onerror = () => {
      connectionStatus.value = 'error'
      errorMessage.value = 'WebSocket connection encountered an error'
      if (terminal) {
        terminal.writeln(`\r\n\x1b[31m[Error] WebSocket connection error occurred.\x1b[0m\r\n`)
      }
    }

    ws.onclose = (event: CloseEvent) => {
      connectionStatus.value = 'disconnected'
      if (terminal) {
        terminal.writeln(`\r\n\x1b[33m[Disconnected] Session closed (code: ${event.code}${event.reason ? `, reason: ${event.reason}` : ''}).\x1b[0m\r\n`)
      }
    }
  }

  function cleanupWebSocket() {
    if (ws) {
      ws.onopen = null
      ws.onmessage = null
      ws.onerror = null
      ws.onclose = null
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close()
      }
      ws = null
    }
  }

  function clearTerminal() {
    if (terminal) {
      terminal.clear()
    }
  }

  function reconnect() {
    connect()
  }

  function sendCommand(cmd: string) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(cmd + '\n')
    }
  }

  watch(
    () => [props.cluster, props.pod, props.namespace],
    () => {
      connect()
    }
  )

  watch(
    () => selectedContainer.value,
    () => {
      connect()
    }
  )

  watch(
    () => props.container,
    (newC) => {
      if (newC && newC !== selectedContainer.value) {
        selectedContainer.value = newC
      }
    }
  )

  onMounted(() => {
    if (!terminalContainerRef.value) return

    terminal = new Terminal({
      cursorBlink: true,
      cursorStyle: 'block',
      fontSize: 13,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      lineHeight: 1.25,
      theme: {
        background: '#0a0e17',
        foreground: '#e2e8f0',
        cursor: '#38bdf8',
        cursorAccent: '#0a0e17',
        selectionBackground: 'rgba(56, 189, 248, 0.3)',
        black: '#0a0e17',
        red: '#f43f5e',
        green: '#10b981',
        yellow: '#f59e0b',
        blue: '#3b82f6',
        magenta: '#a855f7',
        cyan: '#38bdf8',
        white: '#f8fafc',
        brightBlack: '#64748b',
        brightRed: '#fb7185',
        brightGreen: '#34d399',
        brightYellow: '#fbbf24',
        brightBlue: '#60a5fa',
        brightMagenta: '#c084fc',
        brightCyan: '#67e8f9',
        brightWhite: '#ffffff',
      },
      convertEol: true,
      allowProposedApi: true,
    })

    fitAddon = new FitAddon()
    terminal.loadAddon(fitAddon)

    const webLinksAddon = new WebLinksAddon()
    terminal.loadAddon(webLinksAddon)

    terminal.open(terminalContainerRef.value)

    terminal.onData((data: string) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(data)
      }
    })

    terminal.onResize((size: { cols: number; rows: number }) => {
      sendResize(size.cols, size.rows)
    })

    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => {
        handleFit()
      })
      resizeObserver.observe(terminalContainerRef.value)
    }

    nextTick(() => {
      setTimeout(() => {
        handleFit()
        connect()
      }, 100)
    })
  })

  onUnmounted(() => {
    if (resizeObserver) {
      resizeObserver.disconnect()
      resizeObserver = null
    }

    cleanupWebSocket()

    if (terminal) {
      terminal.dispose()
      terminal = null
    }
    fitAddon = null
  })

  return {
    selectedContainer,
    connectionStatus,
    errorMessage,
    terminalContainerRef,
    reconnect,
    clearTerminal,
    sendCommand,
  }
}
