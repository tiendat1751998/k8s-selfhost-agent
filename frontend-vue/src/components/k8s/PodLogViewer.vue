<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { api } from '../../api/client'

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

interface ParsedLogLine {
  id: number
  raw: string
  cleanText: string
  level: 'error' | 'warn' | 'info' | 'debug'
  timestamp?: string
}

// Log State
const rawLines = ref<string[]>([])
const selectedContainer = ref<string>(props.container || props.containers[0] || '')
const tailLines = ref<number>(100)
const isFollowing = ref<boolean>(true)
const showPrevious = ref<boolean>(false)
const wrapLines = ref<boolean>(false)
const autoScroll = ref<boolean>(true)
const userScrolledUp = ref<boolean>(false)
const searchQuery = ref<string>('')
const selectedLevel = ref<'all' | 'error' | 'warn' | 'info' | 'debug'>('all')

const isLoading = ref<boolean>(false)
const isConnected = ref<boolean>(false)
const streamError = ref<string | null>(null)
const logsCopied = ref<boolean>(false)

const logContainerRef = ref<HTMLElement | null>(null)
let eventSource: EventSource | null = null
let fetchAbortController: AbortController | null = null

const ANSI_REGEX = /\u001b\[[0-9;]*m/g

function stripAnsi(str: string): string {
  if (str.indexOf('\u001b') === -1 && str.indexOf('\x1b') === -1) {
    return str
  }
  return str.replace(ANSI_REGEX, '')
}

function classifyLogLevel(cleanLower: string): 'error' | 'warn' | 'info' | 'debug' {
  if (
    cleanLower.includes('error') ||
    cleanLower.includes('fatal') ||
    cleanLower.includes('panic') ||
    cleanLower.includes('exception') ||
    cleanLower.includes('fail') ||
    cleanLower.includes('err ') ||
    cleanLower.includes('[err]') ||
    cleanLower.includes('level=error')
  ) {
    return 'error'
  }
  if (
    cleanLower.includes('warn') ||
    cleanLower.includes('warning') ||
    cleanLower.includes('timeout') ||
    cleanLower.includes('level=warn')
  ) {
    return 'warn'
  }
  if (
    cleanLower.includes('debug') ||
    cleanLower.includes('trace') ||
    cleanLower.includes('level=debug')
  ) {
    return 'debug'
  }
  return 'info'
}

// Extract timestamp if line starts with ISO-8601 (e.g. 2026-08-27T...)
const TIMESTAMP_REGEX = /^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\s*(.*)$/

function parseLine(raw: string, id: number): ParsedLogLine {
  const clean = stripAnsi(raw)
  let timestamp: string | undefined
  let text = clean

  const match = clean.match(TIMESTAMP_REGEX)
  if (match) {
    timestamp = match[1]
    text = match[2]
  }

  const level = classifyLogLevel(clean.toLowerCase())

  return {
    id,
    raw,
    cleanText: text || clean,
    level,
    timestamp,
  }
}

// Memoized parsed log lines
const allParsedLogs = computed<ParsedLogLine[]>(() => {
  const lines = rawLines.value
  const result: ParsedLogLine[] = []
  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i]
    if (!raw && raw !== '') continue
    result.push(parseLine(raw, i + 1))
  }
  return result
})

// Counts per level
const levelCounts = computed(() => {
  const all = allParsedLogs.value
  let error = 0
  let warn = 0
  let info = 0
  let debug = 0

  for (let i = 0; i < all.length; i++) {
    const lvl = all[i].level
    if (lvl === 'error') error++
    else if (lvl === 'warn') warn++
    else if (lvl === 'debug') debug++
    else info++
  }

  return { total: all.length, error, warn, info, debug }
})

// Filtered log lines based on level and search query
const filteredLogs = computed<ParsedLogLine[]>(() => {
  const all = allParsedLogs.value
  const q = searchQuery.value.toLowerCase().trim()
  const lvl = selectedLevel.value

  if (lvl === 'all' && !q) {
    return all
  }

  const res: ParsedLogLine[] = []
  for (let i = 0; i < all.length; i++) {
    const item = all[i]
    if (lvl !== 'all' && item.level !== lvl) {
      continue
    }
    if (q && !item.cleanText.toLowerCase().includes(q) && !(item.timestamp && item.timestamp.toLowerCase().includes(q))) {
      continue
    }
    res.push(item)
  }
  return res
})

function buildLogsUrl(follow: boolean): string {
  const base = import.meta.env.VITE_API_BASE_URL || '/api/v1'
  const ns = props.namespace || 'default'
  const containerParam = selectedContainer.value ? `&container=${encodeURIComponent(selectedContainer.value)}` : ''
  const prevParam = showPrevious.value ? '&previous=true' : ''
  const token = localStorage.getItem('k8s_token')
  const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''

  return `${base}/k8s/${encodeURIComponent(props.cluster)}/pods/${encodeURIComponent(props.pod)}/logs?namespace=${encodeURIComponent(ns)}${containerParam}&follow=${follow}&tailLines=${tailLines.value}${prevParam}${tokenParam}`
}

function startStreaming() {
  stopStreaming()
  isLoading.value = true
  streamError.value = null
  isConnected.value = false

  const url = buildLogsUrl(true)

  try {
    eventSource = new EventSource(url)

    eventSource.onopen = () => {
      isConnected.value = true
      isLoading.value = false
      streamError.value = null
    }

    eventSource.onmessage = (event: MessageEvent) => {
      isLoading.value = false
      if (typeof event.data === 'string') {
        // SSE lines may come single or multiple
        const parts = event.data.split('\n')
        for (const p of parts) {
          if (p !== undefined) {
            rawLines.value.push(p)
          }
        }
        // Limit buffer to 10000 lines
        if (rawLines.value.length > 10000) {
          rawLines.value = rawLines.value.slice(rawLines.value.length - 8000)
        }

        if (autoScroll.value && !userScrolledUp.value) {
          nextTick(() => {
            scrollToBottom()
          })
        }
      }
    }

    eventSource.onerror = () => {
      isConnected.value = false
      // If we haven't received anything yet or connection closed
      if (rawLines.value.length === 0 && isLoading.value) {
        streamError.value = 'Failed to stream logs. Switching to static fetch...'
        isLoading.value = false
        // Fallback to static fetch
        fetchStaticLogs()
      }
    }
  } catch (err: unknown) {
    isLoading.value = false
    const msg = err instanceof Error ? err.message : 'EventSource creation failed'
    streamError.value = msg
    fetchStaticLogs()
  }
}

function stopStreaming() {
  if (eventSource) {
    eventSource.onopen = null
    eventSource.onmessage = null
    eventSource.onerror = null
    eventSource.close()
    eventSource = null
  }
  isConnected.value = false
}

async function fetchStaticLogs() {
  stopStreaming()
  if (fetchAbortController) {
    fetchAbortController.abort()
  }
  fetchAbortController = new AbortController()

  isLoading.value = true
  streamError.value = null

  try {
    const url = buildLogsUrl(false)
    const res = await api.request<{ logs?: string; data?: { logs?: string } } | string>(url, {
      signal: fetchAbortController.signal,
    })

    let text = ''
    if (typeof res === 'string') {
      text = res
    } else if (res && typeof res === 'object') {
      if ('data' in res && res.data && typeof res.data.logs === 'string') {
        text = res.data.logs
      } else if ('logs' in res && typeof res.logs === 'string') {
        text = res.logs
      }
    }

    if (text) {
      rawLines.value = text.split('\n')
    } else {
      rawLines.value = []
    }

    if (autoScroll.value && !userScrolledUp.value) {
      nextTick(() => {
        scrollToBottom()
      })
    }
  } catch (err: unknown) {
    if (err instanceof Error && err.name === 'AbortError') return
    const msg = err instanceof Error ? err.message : 'Failed to fetch static logs'
    streamError.value = msg
  } finally {
    isLoading.value = false
  }
}

function reloadLogs() {
  rawLines.value = []
  if (isFollowing.value) {
    startStreaming()
  } else {
    fetchStaticLogs()
  }
}

function toggleFollowing() {
  isFollowing.value = !isFollowing.value
  if (isFollowing.value) {
    startStreaming()
  } else {
    stopStreaming()
  }
}

function handleScroll(e: Event) {
  const el = e.target as HTMLElement
  if (!el) return
  const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50
  userScrolledUp.value = !isNearBottom
}

function scrollToBottom() {
  if (logContainerRef.value) {
    logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
    userScrolledUp.value = false
  }
}

function clearLogs() {
  rawLines.value = []
}

async function copyLogs() {
  if (rawLines.value.length === 0) return
  try {
    const textToCopy = filteredLogs.value.map(l => l.cleanText).join('\n')
    await navigator.clipboard.writeText(textToCopy)
    logsCopied.value = true
    setTimeout(() => {
      logsCopied.value = false
    }, 2000)
  } catch {
    // clipboard copy fallback
  }
}

function downloadLogs() {
  if (rawLines.value.length === 0) return
  const textContent = filteredLogs.value.map(l => (l.timestamp ? `[${l.timestamp}] ` : '') + l.cleanText).join('\n')
  const blob = new Blob([textContent], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.pod}_${selectedContainer.value || 'container'}_logs.txt`
  a.click()
  URL.revokeObjectURL(url)
}

watch(
  () => [props.cluster, props.pod, props.namespace],
  () => {
    reloadLogs()
  }
)

watch(
  () => [selectedContainer.value, tailLines.value, showPrevious.value],
  () => {
    reloadLogs()
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
  if (isFollowing.value) {
    startStreaming()
  } else {
    fetchStaticLogs()
  }
})

onUnmounted(() => {
  stopStreaming()
  if (fetchAbortController) {
    fetchAbortController.abort()
    fetchAbortController = null
  }
})
</script>

<template>
  <div class="pod-log-viewer-wrapper">
    <!-- Toolbar -->
    <div class="log-viewer-toolbar glass-panel">
      <!-- Top Row: Targets, Search, and Level Filters -->
      <div class="toolbar-top-row">
        <!-- Container Selector -->
        <div class="toolbar-item font-mono">
          <label class="toolbar-label">CONTAINER:</label>
          <select
            v-if="containers && containers.length > 1"
            v-model="selectedContainer"
            class="select-input font-mono"
          >
            <option v-for="c in containers" :key="c" :value="c">
              📦 {{ c }}
            </option>
          </select>
          <span v-else class="text-cyan font-bold font-mono">
            📦 {{ selectedContainer || 'default' }}
          </span>
        </div>

        <!-- Tail Lines -->
        <div class="toolbar-item font-mono">
          <label class="toolbar-label">TAIL:</label>
          <select v-model="tailLines" class="select-input font-mono">
            <option :value="50">50 lines</option>
            <option :value="100">100 lines</option>
            <option :value="500">500 lines</option>
            <option :value="1000">1,000 lines</option>
            <option :value="5000">5,000 lines</option>
          </select>
        </div>

        <!-- Previous Logs Toggle -->
        <div class="toolbar-item">
          <label class="checkbox-label font-mono" title="Show logs from previous terminated container instance">
            <input v-model="showPrevious" type="checkbox" class="cyber-checkbox" />
            <span>Previous (Crashed)</span>
          </label>
        </div>

        <!-- Log Level Filter Chips -->
        <div class="log-level-chips">
          <button
            type="button"
            class="chip-btn"
            :class="{ active: selectedLevel === 'all' }"
            @click="selectedLevel = 'all'"
          >
            All ({{ levelCounts.total }})
          </button>
          <button
            type="button"
            class="chip-btn chip-rose"
            :class="{ active: selectedLevel === 'error' }"
            @click="selectedLevel = 'error'"
          >
            🔴 Errors ({{ levelCounts.error }})
          </button>
          <button
            type="button"
            class="chip-btn chip-amber"
            :class="{ active: selectedLevel === 'warn' }"
            @click="selectedLevel = 'warn'"
          >
            🟡 Warnings ({{ levelCounts.warn }})
          </button>
          <button
            type="button"
            class="chip-btn chip-emerald"
            :class="{ active: selectedLevel === 'info' }"
            @click="selectedLevel = 'info'"
          >
            🟢 Info ({{ levelCounts.info }})
          </button>
        </div>

        <!-- Search Input -->
        <div class="search-box">
          <span class="search-icon">🔍</span>
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Filter logs / exceptions..."
            class="search-input font-mono"
          />
          <button v-if="searchQuery" class="clear-search-btn" @click="searchQuery = ''">✕</button>
        </div>
      </div>

      <!-- Bottom Row: Stream Actions & View Controls -->
      <div class="toolbar-bottom-row">
        <!-- Live Stream & Controls -->
        <div class="action-btn-group">
          <button
            type="button"
            class="btn-log-action btn-stream"
            :class="{ 'stream-active': isFollowing && isConnected }"
            @click="toggleFollowing"
            :title="isFollowing ? 'Pause SSE live stream' : 'Resume SSE live stream'"
          >
            <span v-if="isFollowing && isConnected" class="live-dot"></span>
            <span v-else>⏸️</span>
            <span class="font-mono">{{ isFollowing && isConnected ? 'LIVE STREAMING' : isFollowing ? 'CONNECTING...' : 'PAUSED' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            :class="{ 'active-toggle': autoScroll }"
            @click="autoScroll = !autoScroll"
            title="Auto-scroll to latest lines"
          >
            <span>⬇️</span>
            <span>Auto-Scroll: {{ autoScroll ? 'ON' : 'OFF' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            :class="{ 'active-toggle': wrapLines }"
            @click="wrapLines = !wrapLines"
            title="Toggle word wrap for long log lines"
          >
            <span>↩️</span>
            <span>Wrap Lines: {{ wrapLines ? 'ON' : 'OFF' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            :disabled="isLoading"
            @click="reloadLogs"
            title="Reload Pod Logs"
          >
            <span :class="{ 'spin-icon': isLoading }">🔄</span>
            <span>{{ isLoading ? 'Loading...' : 'Refresh' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            @click="copyLogs"
            title="Copy log text to clipboard"
          >
            <span>📋</span>
            <span>{{ logsCopied ? 'Copied!' : 'Copy' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action btn-download font-mono"
            @click="downloadLogs"
            title="Download log file"
          >
            <span>💾</span>
            <span>Export .log</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            @click="clearLogs"
            title="Clear current log view"
          >
            <span>🧹</span>
            <span>Clear</span>
          </button>
        </div>

        <!-- Meta status badge -->
        <div class="stream-status-meta font-mono">
          <span class="meta-pill">
            <span class="text-muted">Pod:</span>
            <strong class="text-cyan">{{ pod }}</strong>
          </span>
          <span class="meta-pill">
            <span class="text-muted">NS:</span>
            <span class="text-amber">{{ namespace }}</span>
          </span>
          <span class="meta-pill">
            <span class="text-muted">Lines:</span>
            <span class="text-emerald">{{ filteredLogs.length }}</span>
          </span>
        </div>
      </div>
    </div>

    <!-- Log Terminal Container -->
    <div class="log-terminal-window">
      <!-- Terminal Window Top Bar -->
      <div class="terminal-topbar">
        <div class="terminal-dots">
          <span class="tdot tdot-red"></span>
          <span class="tdot tdot-yellow"></span>
          <span class="tdot tdot-green"></span>
        </div>
        <div class="terminal-title font-mono truncate">
          {{ pod }} &gt; {{ selectedContainer || 'container' }} (namespace: {{ namespace }})
        </div>
        <div class="terminal-controls-right font-mono">
          <span v-if="isFollowing && isConnected" class="stream-badge-live">
            <span class="live-dot"></span> SSE LIVE
          </span>
          <span v-else class="stream-badge-paused">
            ⏸️ PAUSED
          </span>
        </div>
      </div>

      <!-- Terminal Body Content -->
      <div
        ref="logContainerRef"
        class="terminal-body"
        :class="{ 'wrap-text': wrapLines }"
        @scroll="handleScroll"
      >
        <!-- Loading State -->
        <div v-if="isLoading && rawLines.length === 0" class="terminal-loading">
          <div class="terminal-spinner"></div>
          <span class="font-mono">Connecting to SSE log stream for pod {{ pod }}...</span>
        </div>

        <!-- Error State -->
        <div v-else-if="streamError && rawLines.length === 0" class="terminal-error">
          <span class="text-rose font-bold font-mono">⚠️ {{ streamError }}</span>
          <button type="button" class="btn-log-action font-mono mt-2" @click="reloadLogs">
            🔄 Retry Connection
          </button>
        </div>

        <!-- Empty State -->
        <div v-else-if="filteredLogs.length === 0" class="terminal-empty">
          <span class="font-mono text-muted">🛡️ No log lines match the current filters or no logs emitted yet.</span>
        </div>

        <!-- Log Lines List -->
        <div v-else class="log-lines-list">
          <div
            v-for="line in filteredLogs"
            :key="line.id"
            class="log-line"
            :class="`log-line-${line.level}`"
          >
            <span class="log-line-num font-mono">{{ line.id }}</span>
            <span v-if="line.timestamp" class="log-line-time font-mono text-muted">{{ line.timestamp }}</span>
            <span
              class="log-line-level font-mono"
              :class="{
                'text-rose': line.level === 'error',
                'text-amber': line.level === 'warn',
                'text-emerald': line.level === 'info',
                'text-slate': line.level === 'debug',
              }"
            >
              [{{ line.level.toUpperCase() }}]
            </span>
            <span class="log-line-content font-mono">{{ line.cleanText }}</span>
          </div>
        </div>

        <!-- Jump to bottom button -->
        <button
          v-if="userScrolledUp && filteredLogs.length > 0"
          type="button"
          class="jump-bottom-btn font-mono"
          @click="scrollToBottom"
        >
          ⬇️ New logs available - Jump to bottom
        </button>
      </div>

      <!-- Terminal Footer -->
      <div class="terminal-footer font-mono">
        <span class="footer-info">
          Showing <strong>{{ filteredLogs.length }}</strong> of <strong>{{ allParsedLogs.length }}</strong> lines (Errors: {{ levelCounts.error }}, Warnings: {{ levelCounts.warn }})
        </span>
        <span class="cluster-tag">Cluster: {{ cluster }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pod-log-viewer-wrapper {
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 640px;
  max-height: 80vh;
}

/* Toolbar */
.log-viewer-toolbar {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 16px;
  background: rgba(15, 23, 42, 0.75);
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.toolbar-top-row,
.toolbar-bottom-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  width: 100%;
}

.toolbar-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
}

.toolbar-label {
  font-size: 10.5px;
  font-weight: 700;
  color: var(--text-muted, #94a3b8);
  white-space: nowrap;
}

.select-input {
  padding: 4px 8px;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid rgba(56, 189, 248, 0.3);
  border-radius: 6px;
  color: #38bdf8;
  font-size: 11.5px;
  font-weight: 600;
  outline: none;
  cursor: pointer;
  transition: border-color 0.2s ease;
}

.select-input:hover,
.select-input:focus {
  border-color: #38bdf8;
}

.select-input option {
  background: #0f172a;
  color: #f8fafc;
}

.checkbox-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
  cursor: pointer;
  user-select: none;
}

.cyber-checkbox {
  cursor: pointer;
  accent-color: #38bdf8;
}

/* Level chips */
.log-level-chips {
  display: flex;
  align-items: center;
  gap: 6px;
}

.chip-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 9px;
  border-radius: 6px;
  font-family: var(--font-mono, monospace);
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: var(--text-secondary, #94a3b8);
  transition: all 0.2s ease;
  user-select: none;
}

.chip-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #f8fafc;
  border-color: rgba(255, 255, 255, 0.25);
}

.chip-btn.active {
  background: rgba(56, 189, 248, 0.16);
  border-color: #38bdf8;
  color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.25);
}

.chip-btn.chip-rose {
  background: rgba(244, 63, 94, 0.08);
  border-color: rgba(244, 63, 94, 0.25);
  color: #fb7185;
}

.chip-btn.chip-rose.active {
  background: rgba(244, 63, 94, 0.22);
  border-color: #f43f5e;
  color: #fda4af;
  box-shadow: 0 0 12px rgba(244, 63, 94, 0.4);
}

.chip-btn.chip-amber {
  background: rgba(245, 158, 11, 0.08);
  border-color: rgba(245, 158, 11, 0.25);
  color: #fbbf24;
}

.chip-btn.chip-amber.active {
  background: rgba(245, 158, 11, 0.22);
  border-color: #f59e0b;
  color: #fde68a;
  box-shadow: 0 0 12px rgba(245, 158, 11, 0.4);
}

.chip-btn.chip-emerald {
  background: rgba(16, 185, 129, 0.08);
  border-color: rgba(16, 185, 129, 0.25);
  color: #34d399;
}

.chip-btn.chip-emerald.active {
  background: rgba(16, 185, 129, 0.22);
  border-color: #10b981;
  color: #6ee7b7;
  box-shadow: 0 0 12px rgba(16, 185, 129, 0.4);
}

/* Search Box */
.search-box {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 180px;
}

.search-icon {
  position: absolute;
  left: 8px;
  font-size: 11px;
  pointer-events: none;
}

.search-input {
  width: 100%;
  padding: 5px 26px 5px 28px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 6px;
  color: #f8fafc;
  font-size: 11.5px;
  outline: none;
  transition: border-color 0.2s ease;
}

.search-input:focus {
  border-color: #38bdf8;
}

.clear-search-btn {
  position: absolute;
  right: 6px;
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  font-size: 10px;
  padding: 2px 4px;
}

/* Action Buttons */
.action-btn-group {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.btn-log-action {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 9px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 6px;
  color: var(--text-secondary, #94a3b8);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.btn-log-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
  color: #f8fafc;
  border-color: rgba(56, 189, 248, 0.4);
}

.btn-stream {
  border-color: rgba(16, 185, 129, 0.3);
}

.btn-stream.stream-active {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border-color: rgba(16, 185, 129, 0.5);
}

.btn-stream:not(.stream-active) {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border-color: rgba(245, 158, 11, 0.4);
}

.btn-log-action.active-toggle {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border-color: rgba(56, 189, 248, 0.4);
}

.btn-download {
  color: #38bdf8;
  border-color: rgba(56, 189, 248, 0.3);
}

.live-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.8);
  animation: logPulse 2s infinite ease-in-out;
  display: inline-block;
}

@keyframes logPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.85); }
}

.spin-icon {
  display: inline-block;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.stream-status-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
}

.meta-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

/* Terminal Window */
.log-terminal-window {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #080c14;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  overflow: hidden;
  box-shadow: inset 0 2px 10px rgba(0, 0, 0, 0.6);
  min-height: 380px;
}

.terminal-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  background: rgba(15, 23, 42, 0.85);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  gap: 10px;
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

.terminal-title {
  font-size: 11.5px;
  color: var(--text-secondary, #94a3b8);
  letter-spacing: 0.02em;
  flex: 1;
}

.terminal-controls-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.stream-badge-live {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 10.5px;
  padding: 2px 8px;
  border-radius: 9999px;
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.stream-badge-paused {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 10.5px;
  padding: 2px 8px;
  border-radius: 9999px;
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.3);
  color: #fbbf24;
}

/* Terminal Body */
.terminal-body {
  flex: 1;
  position: relative;
  overflow-y: auto;
  overflow-x: auto;
  padding: 8px 12px;
  font-size: 11.5px;
  line-height: 1.5;
  background: #080c14;
}

.terminal-loading,
.terminal-empty,
.terminal-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  color: var(--text-muted, #64748b);
  gap: 10px;
}

.terminal-spinner {
  width: 26px;
  height: 26px;
  border: 2px solid rgba(56, 189, 248, 0.2);
  border-top-color: #38bdf8;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.log-lines-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.log-line {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 1px 4px;
  border-radius: 3px;
  transition: background 0.1s ease;
  box-sizing: border-box;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.04);
}

.log-line-error {
  background: rgba(244, 63, 94, 0.08);
  border-left: 2px solid #f43f5e;
}

.log-line-warn {
  background: rgba(245, 158, 11, 0.06);
  border-left: 2px solid #f59e0b;
}

.log-line-num {
  width: 32px;
  color: rgba(255, 255, 255, 0.25);
  text-align: right;
  flex-shrink: 0;
  user-select: none;
  font-size: 10px;
}

.log-line-time {
  font-size: 10px;
  flex-shrink: 0;
  color: #64748b;
  user-select: none;
}

.log-line-level {
  width: 50px;
  flex-shrink: 0;
  font-weight: 700;
  font-size: 10px;
  letter-spacing: 0.02em;
}

.log-line-content {
  flex: 1;
  white-space: nowrap;
  color: #e2e8f0;
}

.wrap-text .log-line-content {
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line-error .log-line-content {
  color: #fda4af;
}

.log-line-warn .log-line-content {
  color: #fde68a;
}

.jump-bottom-btn {
  position: absolute;
  bottom: 14px;
  right: 20px;
  z-index: 15;
  background: rgba(15, 23, 42, 0.9);
  border: 1px solid #38bdf8;
  color: #38bdf8;
  border-radius: 20px;
  padding: 4px 12px;
  font-size: 11px;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.jump-bottom-btn:hover {
  background: rgba(56, 189, 248, 0.2);
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.6);
}

/* Terminal Footer */
.terminal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 14px;
  background: rgba(15, 23, 42, 0.7);
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  font-size: 11px;
  color: var(--text-muted, #64748b);
  gap: 10px;
}

.cluster-tag {
  color: var(--text-secondary, #94a3b8);
}

.text-rose { color: #f43f5e; }
.text-amber { color: #f59e0b; }
.text-emerald { color: #10b981; }
.text-cyan { color: #38bdf8; }
.text-slate { color: #94a3b8; }
.text-muted { color: #64748b; }
.truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
