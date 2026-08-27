<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'

interface Props {
  cluster: string
  pod: string
  namespace?: string
  container?: string
  containers?: string[]
}

const props = withDefaults(defineProps<Props>(), {
  namespace: 'default',
  container: '',
  containers: () => [],
})

const selectedContainer = ref<string>(props.container || props.containers[0] || '')
const connectionStatus = ref<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
const errorMessage = ref<string | null>(null)
const terminalContainerRef = ref<HTMLElement | null>(null)

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
</script>

<template>
  <div class="pod-terminal-wrapper">
    <!-- Top Control Bar -->
    <div class="terminal-header glass-panel">
      <div class="header-left">
        <div class="terminal-dots">
          <span class="tdot tdot-red"></span>
          <span class="tdot tdot-yellow"></span>
          <span class="tdot tdot-green"></span>
        </div>
        <div class="terminal-meta font-mono">
          <span class="meta-item">
            <span class="text-muted">Pod:</span>
            <strong class="text-cyan">{{ pod }}</strong>
          </span>
          <span class="meta-item">
            <span class="text-muted">NS:</span>
            <span class="text-amber">{{ namespace }}</span>
          </span>
        </div>

        <!-- Container Selector (if multi-container) -->
        <div v-if="containers && containers.length > 1" class="container-select-wrap font-mono">
          <label class="select-label">CONTAINER:</label>
          <select v-model="selectedContainer" class="container-select">
            <option v-for="c in containers" :key="c" :value="c">
              📦 {{ c }}
            </option>
          </select>
        </div>
        <div v-else-if="selectedContainer" class="container-badge font-mono">
          <span class="text-muted">Container:</span>
          <span class="text-emerald">📦 {{ selectedContainer }}</span>
        </div>
      </div>

      <div class="header-right">
        <!-- Status Indicator -->
        <div class="status-indicator font-mono" :class="`status-${connectionStatus}`">
          <span class="status-dot"></span>
          <span class="status-text">
            {{
              connectionStatus === 'connected'
                ? 'CONNECTED'
                : connectionStatus === 'connecting'
                ? 'CONNECTING...'
                : connectionStatus === 'error'
                ? 'ERROR'
                : 'DISCONNECTED'
            }}
          </span>
        </div>

        <!-- Action Buttons -->
        <div class="action-btn-group">
          <button
            v-if="connectionStatus !== 'connected'"
            type="button"
            class="btn-term btn-reconnect font-mono"
            @click="reconnect"
            title="Reconnect WebSocket session"
          >
            <span>🔄</span>
            <span>Reconnect</span>
          </button>
          <button
            type="button"
            class="btn-term btn-clear font-mono"
            @click="clearTerminal"
            title="Clear terminal screen"
          >
            <span>🧹</span>
            <span>Clear</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Quick Command Shortcuts Bar -->
    <div class="quick-commands-bar font-mono">
      <span class="quick-cmd-label text-muted">Quick Exec:</span>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('ls -la')">ls -la</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('ps aux')">ps aux</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('env')">env</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('df -h')">df -h</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('top')">top</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('exit')">exit</button>
    </div>

    <!-- Terminal Screen Container -->
    <div class="terminal-body-container">
      <div ref="terminalContainerRef" class="xterm-render-target"></div>
    </div>

    <!-- Terminal Footer Info -->
    <div class="terminal-footer font-mono">
      <span class="footer-hint">💡 Shell: <code>/bin/sh</code> | Supports ANSI colors, Tab completion, and standard keystrokes.</span>
      <span class="cluster-badge">Cluster: {{ cluster }}</span>
    </div>
  </div>
</template>

<style scoped>
.pod-terminal-wrapper {
  display: flex;
  flex-direction: column;
  height: 600px;
  max-height: 75vh;
  background: #080c14;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.terminal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 16px;
  background: rgba(15, 23, 42, 0.85);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-wrap: wrap;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
}

.terminal-dots {
  display: flex;
  gap: 6px;
  align-items: center;
}

.tdot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.tdot-red { background: #ef4444; }
.tdot-yellow { background: #f59e0b; }
.tdot-green { background: #10b981; }

.terminal-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
}

.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.container-select-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.select-label {
  font-size: 10.5px;
  font-weight: 700;
  color: var(--text-muted, #94a3b8);
}

.container-select {
  padding: 4px 8px;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid rgba(56, 189, 248, 0.3);
  border-radius: 6px;
  color: #38bdf8;
  font-size: 11.5px;
  outline: none;
  cursor: pointer;
  transition: all 0.2s ease;
}

.container-select:hover,
.container-select:focus {
  border-color: #38bdf8;
  background: rgba(0, 0, 0, 0.65);
}

.container-select option {
  background: #0f172a;
  color: #f8fafc;
}

.container-badge {
  font-size: 11.5px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 9999px;
}

.status-connected {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.4);
  color: #34d399;
}

.status-connected .status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.8);
  animation: termPulse 2s infinite ease-in-out;
}

.status-connecting {
  background: rgba(245, 158, 11, 0.15);
  border: 1px solid rgba(245, 158, 11, 0.4);
  color: #fbbf24;
}

.status-connecting .status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #f59e0b;
  box-shadow: 0 0 8px rgba(245, 158, 11, 0.8);
  animation: termPulse 1s infinite ease-in-out;
}

.status-disconnected,
.status-error {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.4);
  color: #fb7185;
}

.status-disconnected .status-dot,
.status-error .status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #f43f5e;
}

@keyframes termPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.85); }
}

.action-btn-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.btn-term {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: var(--text-secondary, #94a3b8);
}

.btn-term:hover {
  background: rgba(255, 255, 255, 0.12);
  color: #f8fafc;
  border-color: rgba(56, 189, 248, 0.4);
}

.btn-reconnect {
  background: rgba(56, 189, 248, 0.12);
  border-color: rgba(56, 189, 248, 0.35);
  color: #38bdf8;
}

.btn-reconnect:hover {
  background: rgba(56, 189, 248, 0.25);
  border-color: #38bdf8;
  color: #e0f2fe;
}

/* Quick Commands Bar */
.quick-commands-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: rgba(10, 15, 26, 0.7);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  overflow-x: auto;
}

.quick-cmd-label {
  font-size: 10.5px;
  white-space: nowrap;
}

.btn-quick-cmd {
  padding: 2px 7px;
  font-size: 10.5px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 4px;
  color: var(--text-secondary, #94a3b8);
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.btn-quick-cmd:hover {
  background: rgba(56, 189, 248, 0.15);
  border-color: rgba(56, 189, 248, 0.4);
  color: #38bdf8;
}

/* Terminal Body */
.terminal-body-container {
  flex: 1;
  position: relative;
  overflow: hidden;
  background: #0a0e17;
  padding: 8px;
}

.xterm-render-target {
  width: 100%;
  height: 100%;
}

:deep(.xterm) {
  padding: 4px;
  height: 100%;
}

:deep(.xterm-viewport) {
  background-color: #0a0e17 !important;
}

/* Terminal Footer */
.terminal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 6px 14px;
  background: rgba(15, 23, 42, 0.7);
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.footer-hint code {
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.1);
  padding: 1px 4px;
  border-radius: 3px;
}

.cluster-badge {
  color: var(--text-secondary, #94a3b8);
}

.text-cyan { color: #38bdf8; }
.text-amber { color: #f59e0b; }
.text-emerald { color: #10b981; }
.text-muted { color: #64748b; }
</style>
