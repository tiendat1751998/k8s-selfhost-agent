<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import type { NodeMetrics, TpsSnapshot, TpsServiceMetrics } from '../../../api/overview'
import type { NodeHistoryResponse } from '../../../api/compute'
import { dockerApi } from '../../../api/docker'

interface Props {
  node: NodeMetrics
  nodeHistoryData?: NodeHistoryResponse | null
  tpsData?: TpsSnapshot | null
}

const props = withDefaults(defineProps<Props>(), {
  nodeHistoryData: null,
  tpsData: null,
})

// Log Filters & Config State
const selectedLogApp = ref<string>('all')
const selectedLogTail = ref<number | 'all'>(100)
const selectedLogSince = ref<string>('15m')
const selectedLogLevel = ref<'all' | 'error' | 'warn' | 'info' | 'debug'>('all')
const appLogSearch = ref<string>('')
const customLogFrom = ref<string>('')
const customLogTo = ref<string>('')
const showCustomDatePicker = ref<boolean>(false)
const wrapLogLines = ref<boolean>(false)

// Real-Time Live Auto-Tail & Scroll State
const isLogStreaming = ref<boolean>(true)
const logStreamInterval = ref<any>(null)
const autoScrollLogs = ref<boolean>(true)
const userScrolledUp = ref<boolean>(false)
const terminalBodyRef = ref<HTMLElement | null>(null)

const appLogsText = ref<string>('')
const appLogsLoading = ref<boolean>(false)
const appLogsError = ref<string | null>(null)
const appLogsCopied = ref<boolean>(false)

interface ParsedLogLine {
  id: number
  raw: string
  cleanText: string
  level: 'error' | 'warn' | 'info' | 'debug'
  timestamp?: string
}

// 1. Raw list of services running on this node
const rawNodeServices = computed<TpsServiceMetrics[]>(() => {
  if (!props.node || !props.tpsData?.services) return []
  const nodeId = (props.node.node_id || '').toLowerCase().trim()
  const nodeName = (props.node.node_name || '').toLowerCase().trim()

  return props.tpsData.services.filter(s => {
    const sNodeId = (s.node_id || '').toLowerCase().trim()
    const sNodeName = (s.node_name || '').toLowerCase().trim()
    if (sNodeId && (sNodeId === nodeId || sNodeId === nodeName)) return true
    if (sNodeName && (sNodeName === nodeName || sNodeName === nodeId)) return true
    return false
  })
})

const nodeAvailableLogApps = computed(() => {
  const list: { name: string; type: 'container' | 'host'; icon: string }[] = []
  const seen = new Set<string>()

  for (const s of rawNodeServices.value || []) {
    if (!seen.has(s.service_name)) {
      seen.add(s.service_name)
      list.push({
        name: s.service_name,
        type: 'container',
        icon: '📦',
      })
    }
  }

  for (const p of props.node?.top_processes || []) {
    const pName = p.name || ''
    if (pName && !seen.has(pName) && !pName.startsWith('[')) {
      seen.add(pName)
      list.push({
        name: pName,
        type: 'host',
        icon: '⚙️',
      })
    }
  }

  if (list.length === 0) {
    list.push({ name: 'k8s-agent', type: 'host', icon: '📡' })
  }
  return list
})

const nodeAvailableContainerApps = computed(() =>
  nodeAvailableLogApps.value.filter(app => app.type === 'container')
)

const nodeAvailableHostApps = computed(() =>
  nodeAvailableLogApps.value.filter(app => app.type === 'host')
)

function toIsoTime(val?: string): string | undefined {
  if (!val) return undefined
  try {
    const d = new Date(val)
    if (!isNaN(d.getTime())) return d.toISOString()
  } catch {}
  return val
}

function formatBadgeDate(dt: string): string {
  if (!dt) return ''
  try {
    const d = new Date(dt)
    if (!isNaN(d.getTime())) {
      const pad = (n: number) => String(n).padStart(2, '0')
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
    }
  } catch {
    // fallback
  }
  return dt.replace('T', ' ')
}

function formatDateTimeLocal(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  const yyyy = d.getFullYear()
  const MM = pad(d.getMonth() + 1)
  const dd = pad(d.getDate())
  const hh = pad(d.getHours())
  const mm = pad(d.getMinutes())
  return `${yyyy}-${MM}-${dd}T${hh}:${mm}`
}

function onLogSinceChange() {
  if (selectedLogSince.value === 'custom') {
    showCustomDatePicker.value = true
    if (!customLogFrom.value) {
      const now = new Date()
      customLogFrom.value = formatDateTimeLocal(new Date(now.getTime() - 2 * 60 * 60 * 1000))
      customLogTo.value = formatDateTimeLocal(now)
    }
  } else {
    showCustomDatePicker.value = false
  }
  fetchNodeAppLogs(selectedLogApp.value)
}

function applyLogPreset(preset: '30m' | '2h' | '6h' | 'today') {
  const now = new Date()
  if (preset === '30m') {
    customLogFrom.value = formatDateTimeLocal(new Date(now.getTime() - 30 * 60 * 1000))
    customLogTo.value = formatDateTimeLocal(now)
  } else if (preset === '2h') {
    customLogFrom.value = formatDateTimeLocal(new Date(now.getTime() - 2 * 60 * 60 * 1000))
    customLogTo.value = formatDateTimeLocal(now)
  } else if (preset === '6h') {
    customLogFrom.value = formatDateTimeLocal(new Date(now.getTime() - 6 * 60 * 60 * 1000))
    customLogTo.value = formatDateTimeLocal(now)
  } else if (preset === 'today') {
    const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0)
    customLogFrom.value = formatDateTimeLocal(startOfDay)
    customLogTo.value = formatDateTimeLocal(now)
  }
  selectedLogSince.value = 'custom'
  showCustomDatePicker.value = true
  fetchNodeAppLogs(selectedLogApp.value)
}

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
    cleanLower.includes('[err]')
  ) {
    return 'error'
  }
  if (
    cleanLower.includes('warn') ||
    cleanLower.includes('timeout')
  ) {
    return 'warn'
  }
  if (
    cleanLower.includes('debug') ||
    cleanLower.includes('trace')
  ) {
    return 'debug'
  }
  return 'info'
}

// 1. Memoized full parsed logs cache (split & classified once upon log text update)
const allParsedLogs = computed<ParsedLogLine[]>(() => {
  const text = appLogsText.value
  if (!text) return []
  const rawLines = text.split('\n')
  const count = rawLines.length
  const lines: ParsedLogLine[] = []

  for (let i = 0; i < count; i++) {
    const raw = rawLines[i]
    if (!raw.trim()) continue

    const clean = stripAnsi(raw)
    const lower = clean.toLowerCase()
    const level = classifyLogLevel(lower)

    lines.push({
      id: i + 1,
      raw,
      cleanText: clean,
      level,
    })
  }

  return lines
})

// 2. Count levels from memoized logs (zero re-splitting)
const appLogLevelCounts = computed(() => {
  const all = allParsedLogs.value
  let error = 0
  let warn = 0
  let info = 0
  let debug = 0

  for (let i = 0; i < all.length; i++) {
    const level = all[i].level
    if (level === 'error') error++
    else if (level === 'warn') warn++
    else if (level === 'debug') debug++
    else info++
  }

  return { total: all.length, error, warn, info, debug }
})

// 3. Filtered log lines (fast array filter)
const parsedAppLogLines = computed<ParsedLogLine[]>(() => {
  const all = allParsedLogs.value
  const q = appLogSearch.value.toLowerCase().trim()
  const filterLevel = selectedLogLevel.value

  if (filterLevel === 'all' && !q) {
    return all
  }

  const lines: ParsedLogLine[] = []
  for (let i = 0; i < all.length; i++) {
    const item = all[i]
    if (filterLevel !== 'all' && item.level !== filterLevel) {
      continue
    }
    if (q && !item.cleanText.toLowerCase().includes(q)) {
      continue
    }
    lines.push(item)
  }

  return lines
})

// Virtual Scrolling Constants & State
const LINE_HEIGHT = 22
const OVERSCAN = 15
const terminalScrollTop = ref<number>(0)
const terminalViewportHeight = ref<number>(380)

const totalLogCount = computed(() => parsedAppLogLines.value.length)
const totalVirtualHeight = computed(() => totalLogCount.value * LINE_HEIGHT)
const startIndex = computed(() => Math.max(0, Math.floor(terminalScrollTop.value / LINE_HEIGHT) - OVERSCAN))
const endIndex = computed(() => Math.min(totalLogCount.value, Math.ceil((terminalScrollTop.value + terminalViewportHeight.value) / LINE_HEIGHT) + OVERSCAN))
const visibleLogLines = computed(() => {
  return parsedAppLogLines.value.slice(startIndex.value, endIndex.value).map((line, idx) => ({
    ...line,
    virtualTop: (startIndex.value + idx) * LINE_HEIGHT,
  }))
})

async function fetchNodeAppLogs(appName?: string) {
  const targetApp = appName || selectedLogApp.value || nodeAvailableLogApps.value[0]?.name || 'all'
  selectedLogApp.value = targetApp
  appLogsLoading.value = true
  appLogsError.value = null
  try {
    const tailParam = selectedLogTail.value === 'all' ? 'all' : String(selectedLogTail.value)
    let sinceParam = ''
    let untilParam = ''

    if (selectedLogSince.value === 'custom') {
      if (customLogFrom.value) {
        sinceParam = toIsoTime(customLogFrom.value) || customLogFrom.value
      }
      if (customLogTo.value) {
        untilParam = toIsoTime(customLogTo.value) || customLogTo.value
      }
    } else if (selectedLogSince.value !== 'all') {
      sinceParam = selectedLogSince.value
    }

    const searchParam = appLogSearch.value.trim() || undefined
    const filterLevel = selectedLogLevel.value !== 'all' ? selectedLogLevel.value : undefined
    const nodeTarget = props.node?.node_id || props.node?.node_name

    let res: { logs: string } | null = null
    try {
      res = await dockerApi.getLogs(targetApp, 'service', tailParam, sinceParam, untilParam, searchParam, filterLevel, nodeTarget)
    } catch {
      res = await dockerApi.getLogs(targetApp, 'container', tailParam, sinceParam, untilParam, searchParam, filterLevel, nodeTarget)
    }

    if (res && typeof res.logs === 'string' && res.logs.trim().length > 0) {
      appLogsText.value = res.logs
    } else {
      const isHostApp = nodeAvailableLogApps.value.find(a => a.name === targetApp)?.type === 'host'
      if (isHostApp) {
        appLogsText.value = `[INFO] Host process '${targetApp}' is running as a system service. Standard stdout/stderr stream is only accessible for Docker containerized workloads (e.g. tiki_redis, my-nginx, db, nats).`
      } else {
        const windowInfo = selectedLogSince.value === 'custom'
          ? `between ${customLogFrom.value || 'start'} and ${customLogTo.value || 'now'}`
          : `in the last ${selectedLogSince.value}`
        appLogsText.value = `[INFO] No real stdout/stderr lines emitted by '${targetApp}' on node '${props.node?.node_name}' ${windowInfo}. Process is running nominally.`
      }
    }

    if (autoScrollLogs.value && !userScrolledUp.value) {
      nextTick(() => {
        scrollToBottom()
      })
    }
  } catch (err: any) {
    const isHostApp = nodeAvailableLogApps.value.find(a => a.name === targetApp)?.type === 'host'
    if (isHostApp) {
      appLogsError.value = null
      appLogsText.value = `[INFO] Host process '${targetApp}' is running as a system service. Standard stdout/stderr stream is only accessible for Docker containerized workloads (e.g. tiki_redis, my-nginx, db, nats).`
    } else {
      appLogsError.value = err.message || 'Failed to fetch logs from agent daemon'
      appLogsText.value = `[ERROR] Unable to reach agent log collector on node ${props.node?.node_name}: ${err.message}`
    }
  } finally {
    appLogsLoading.value = false
  }
}

async function fetchNodeAppLogsQuiet(appName?: string) {
  if (appLogsLoading.value) return
  const targetApp = appName || selectedLogApp.value || nodeAvailableLogApps.value[0]?.name || 'all'
  try {
    const tailParam = selectedLogTail.value === 'all' ? 'all' : String(selectedLogTail.value)
    let sinceParam = ''
    let untilParam = ''

    if (selectedLogSince.value === 'custom') {
      if (customLogFrom.value) {
        sinceParam = toIsoTime(customLogFrom.value) || customLogFrom.value
      }
      if (customLogTo.value) {
        untilParam = toIsoTime(customLogTo.value) || customLogTo.value
      }
    } else if (selectedLogSince.value !== 'all') {
      sinceParam = selectedLogSince.value
    }

    const searchParam = appLogSearch.value.trim() || undefined
    const filterLevel = selectedLogLevel.value !== 'all' ? selectedLogLevel.value : undefined
    const nodeTarget = props.node?.node_id || props.node?.node_name

    let res: { logs: string } | null = null
    try {
      res = await dockerApi.getLogs(targetApp, 'service', tailParam, sinceParam, untilParam, searchParam, filterLevel, nodeTarget)
    } catch {
      res = await dockerApi.getLogs(targetApp, 'container', tailParam, sinceParam, untilParam, searchParam, filterLevel, nodeTarget)
    }

    if (res && typeof res.logs === 'string' && res.logs.trim().length > 0) {
      appLogsText.value = res.logs
      if (autoScrollLogs.value && !userScrolledUp.value) {
        nextTick(() => {
          scrollToBottom()
        })
      }
    }
  } catch {
    // Silent fail for quiet background stream
  }
}

function startLogStreaming() {
  stopLogStreaming()
  isLogStreaming.value = true
  logStreamInterval.value = setInterval(() => {
    if (isLogStreaming.value) {
      fetchNodeAppLogsQuiet()
    }
  }, 3000)
}

function stopLogStreaming() {
  if (logStreamInterval.value) {
    clearInterval(logStreamInterval.value)
    logStreamInterval.value = null
  }
  isLogStreaming.value = false
}

function toggleLogStreaming() {
  if (isLogStreaming.value) {
    stopLogStreaming()
  } else {
    startLogStreaming()
    fetchNodeAppLogsQuiet()
  }
}

function handleTerminalScroll(event: Event) {
  const el = event.target as HTMLElement
  if (!el) return
  terminalScrollTop.value = el.scrollTop
  terminalViewportHeight.value = el.clientHeight || 380
  const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40
  userScrolledUp.value = !isNearBottom
}

function scrollToBottom() {
  if (terminalBodyRef.value) {
    terminalBodyRef.value.scrollTop = terminalBodyRef.value.scrollHeight
    terminalScrollTop.value = terminalBodyRef.value.scrollTop
    userScrolledUp.value = false
  }
}

function clearLogTerminal() {
  appLogsText.value = ''
  terminalScrollTop.value = 0
}

async function copyAppLogs() {
  if (!appLogsText.value) return
  try {
    await navigator.clipboard.writeText(appLogsText.value)
    appLogsCopied.value = true
    setTimeout(() => {
      appLogsCopied.value = false
    }, 2000)
  } catch {
    // fallback
  }
}

function downloadAppLogs() {
  const blob = new Blob([appLogsText.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.node?.node_name || 'node'}_${selectedLogApp.value}_logs.txt`
  a.click()
  URL.revokeObjectURL(url)
}

function selectIncidentLog(inc: any) {
  if (!inc) return
  if (inc.recorded_at) {
    const centerTime = new Date(inc.recorded_at).getTime()
    if (!isNaN(centerTime)) {
      const fromTime = new Date(centerTime - 15 * 60 * 1000)
      const toTime = new Date(centerTime + 15 * 60 * 1000)
      customLogFrom.value = formatDateTimeLocal(fromTime)
      customLogTo.value = formatDateTimeLocal(toTime)
      selectedLogSince.value = 'custom'
      showCustomDatePicker.value = true
    }
  }
  selectedLogLevel.value = 'error'
  fetchNodeAppLogs(selectedLogApp.value)
}

function syncToTime(from: string, to: string) {
  customLogFrom.value = from
  customLogTo.value = to
  selectedLogSince.value = 'custom'
  showCustomDatePicker.value = true
  fetchNodeAppLogs(selectedLogApp.value)
  nextTick(() => {
    if (terminalBodyRef.value) {
      terminalBodyRef.value.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  })
}

defineExpose({
  syncToTime,
  fetchNodeAppLogs,
  clearLogTerminal,
  copyAppLogs,
  downloadAppLogs,
  wrapLogLines,
})

watch(() => props.node?.node_id, (newNodeId) => {
  if (newNodeId) {
    if (nodeAvailableLogApps.value.length > 0) {
      selectedLogApp.value = nodeAvailableLogApps.value[0].name
    }
    fetchNodeAppLogs(selectedLogApp.value)
  }
})

onMounted(() => {
  if (nodeAvailableLogApps.value.length > 0) {
    selectedLogApp.value = nodeAvailableLogApps.value[0].name
  }
  fetchNodeAppLogs(selectedLogApp.value)
  startLogStreaming()
})

onUnmounted(() => {
  stopLogStreaming()
})
</script>

<template>
          <!-- Incidents & Real Workload Failure Logs on this Server -->
          <div class="node-incidents-section glass-panel">
            <div class="incidents-section-header">
              <div class="inc-title-group">
                <h4>📜 Live & Historical Failure Logs / Incident Evidence</h4>
                <span class="badge badge-rose font-mono" v-if="appLogLevelCounts.error > 0">
                  {{ appLogLevelCounts.error }} error{{ appLogLevelCounts.error === 1 ? '' : 's' }}
                </span>
                <span class="badge badge-emerald font-mono" v-else>
                  HEALTHY
                </span>
              </div>
              <p class="inc-desc">
                Real-time container stdout/stderr, automated pre-crash snapshots, and application exception logs on <strong>{{ node?.node_name }}</strong>.
              </p>
            </div>

            <!-- Log Workload Selector & Filter Toolbar -->
            <div class="log-explorer-toolbar">
              <!-- Row 1: Filter Selectors, Chips, and Search -->
              <div class="log-toolbar-row-top">
                <!-- App Selector -->
                <div class="log-toolbar-item">
                  <label class="log-toolbar-label font-mono">APP:</label>
                  <select
                    v-model="selectedLogApp"
                    class="select-log-input font-mono"
                    @change="fetchNodeAppLogs(selectedLogApp)"
                  >
                    <optgroup v-if="nodeAvailableContainerApps.length > 0" label="📦 Container Workloads">
                      <option v-for="app in nodeAvailableContainerApps" :key="app.name" :value="app.name">
                        {{ app.icon }} {{ app.name }}
                      </option>
                    </optgroup>
                    <optgroup v-if="nodeAvailableHostApps.length > 0" label="⚙️ Host Services">
                      <option v-for="app in nodeAvailableHostApps" :key="app.name" :value="app.name">
                        {{ app.icon }} {{ app.name }}
                      </option>
                    </optgroup>
                  </select>
                </div>

                <!-- Tail Depth Selector -->
                <div class="log-toolbar-item">
                  <label class="log-toolbar-label font-mono">TAIL:</label>
                  <select
                    v-model="selectedLogTail"
                    class="select-log-input font-mono"
                    @change="fetchNodeAppLogs(selectedLogApp)"
                  >
                    <option :value="100">100 lines</option>
                    <option :value="500">500 lines</option>
                    <option :value="1000">1,000 lines</option>
                    <option :value="5000">5,000 lines</option>
                    <option value="all">All lines</option>
                  </select>
                </div>

                <!-- Time Window (Since) Selector -->
                <div class="log-toolbar-item">
                  <label class="log-toolbar-label font-mono">WINDOW:</label>
                  <select
                    v-model="selectedLogSince"
                    class="select-log-input font-mono"
                    @change="onLogSinceChange"
                  >
                    <option value="all">All Time</option>
                    <option value="15m">Last 15m</option>
                    <option value="1h">Last 1h</option>
                    <option value="6h">Last 6h</option>
                    <option value="24h">Last 24h</option>
                    <option value="168h">Last 7d</option>
                    <option value="custom">📅 Custom Time Range...</option>
                  </select>
                  <button
                    v-if="selectedLogSince === 'custom' || showCustomDatePicker"
                    type="button"
                    class="btn-clear-window-pill font-mono"
                    @click="selectedLogSince = 'all'; showCustomDatePicker = false; customLogFrom = ''; customLogTo = ''; fetchNodeAppLogs(selectedLogApp)"
                    title="Reset time window to show all available logs"
                  >
                    ✕ Clear Window (Show All)
                  </button>
                </div>

                <!-- Log Level Chips -->
                <div class="log-level-chips">
                  <button
                    class="chip-btn"
                    :class="{ active: selectedLogLevel === 'all' }"
                    @click="selectedLogLevel = 'all'"
                  >
                    All ({{ appLogLevelCounts.total }})
                  </button>
                  <button
                    class="chip-btn chip-rose"
                    :class="{ active: selectedLogLevel === 'error' }"
                    @click="selectedLogLevel = 'error'"
                  >
                    🔴 Errors ({{ appLogLevelCounts.error }})
                  </button>
                  <button
                    class="chip-btn chip-amber"
                    :class="{ active: selectedLogLevel === 'warn' }"
                    @click="selectedLogLevel = 'warn'"
                  >
                    🟡 Warnings ({{ appLogLevelCounts.warn }})
                  </button>
                </div>

                <!-- Log Search Box -->
                <div class="log-search-box">
                  <span class="search-icon">🔍</span>
                  <input
                    v-model="appLogSearch"
                    type="text"
                    placeholder="Filter logs / stack traces..."
                    class="input-log-search font-mono"
                  />
                  <button v-if="appLogSearch" class="btn-clear-search" @click="appLogSearch = ''">✕</button>
                </div>
              </div>

              <!-- Row 2: Real-time Streaming & Action Controls -->
              <div class="log-toolbar-row-bottom">
                <div class="log-action-btns">
                  <button
                    class="btn-log-action btn-log-stream"
                    :class="{ 'stream-active': isLogStreaming }"
                    @click="toggleLogStreaming"
                    :title="isLogStreaming ? 'Pause Live Tail' : 'Resume Live Tail'"
                  >
                    <span v-if="isLogStreaming" class="log-live-dot"></span>
                    <span v-else>⏸️</span>
                    <span>{{ isLogStreaming ? 'LIVE TAIL (3s)' : 'PAUSED' }}</span>
                  </button>
                  <button
                    class="btn-log-action"
                    :class="{ 'active-toggle': autoScrollLogs }"
                    @click="autoScrollLogs = !autoScrollLogs"
                    title="Toggle Auto-Scroll to Bottom"
                  >
                    <span>⬇️</span>
                    <span>Auto-Scroll: {{ autoScrollLogs ? 'ON' : 'OFF' }}</span>
                  </button>
                  <button
                    class="btn-log-action"
                    :disabled="appLogsLoading"
                    @click="fetchNodeAppLogs(selectedLogApp)"
                    title="Reload Logs"
                  >
                    <span :class="{ 'spin-icon': appLogsLoading }">🔄</span>
                    {{ appLogsLoading ? 'Fetching...' : 'Refresh' }}
                  </button>
                  <button
                    class="btn-log-action"
                    @click="copyAppLogs"
                    title="Copy Log Text"
                  >
                    <span>📋</span>
                    {{ appLogsCopied ? 'Copied!' : 'Copy' }}
                  </button>
                  <button
                    class="btn-log-action btn-log-download"
                    @click="downloadAppLogs"
                    title="Download Raw Log File"
                  >
                    <span>⬇️</span>
                    Export .log
                  </button>
                </div>
              </div>
            </div>

            <!-- Custom Date-Time Range Selector Bar -->
            <div v-if="selectedLogSince === 'custom' || showCustomDatePicker" class="custom-log-range-bar glass-panel animate-fadeIn">
              <div class="custom-range-inputs">
                <div class="range-field">
                  <label class="range-label font-mono">FROM:</label>
                  <input
                    type="datetime-local"
                    v-model="customLogFrom"
                    class="input-datetime font-mono"
                    @keyup.enter="fetchNodeAppLogs(selectedLogApp)"
                  />
                </div>
                <div class="range-field">
                  <label class="range-label font-mono">TO:</label>
                  <input
                    type="datetime-local"
                    v-model="customLogTo"
                    class="input-datetime font-mono"
                    @keyup.enter="fetchNodeAppLogs(selectedLogApp)"
                  />
                </div>
              </div>

              <div class="custom-range-presets">
                <span class="preset-label font-mono">PRESETS:</span>
                <button type="button" class="btn-preset-chip font-mono" @click="applyLogPreset('30m')">Last 30m</button>
                <button type="button" class="btn-preset-chip font-mono" @click="applyLogPreset('2h')">Last 2h</button>
                <button type="button" class="btn-preset-chip font-mono" @click="applyLogPreset('6h')">Last 6h</button>
                <button type="button" class="btn-preset-chip font-mono" @click="applyLogPreset('today')">Today</button>
              </div>

              <div class="custom-range-actions">
                <button
                  type="button"
                  class="btn-apply-range font-mono"
                  :disabled="appLogsLoading"
                  @click="fetchNodeAppLogs(selectedLogApp)"
                >
                  <span class="glow-dot"></span>
                  <span>{{ appLogsLoading ? 'Fetching...' : 'Apply Range' }}</span>
                </button>
              </div>
            </div>

            <!-- Terminal Real Log Viewer -->
            <div class="log-terminal-viewer">
              <div class="terminal-topbar">
                <div class="terminal-dots">
                  <span class="tdot tdot-red"></span>
                  <span class="tdot tdot-yellow"></span>
                  <span class="tdot tdot-green"></span>
                </div>
                <div class="terminal-title font-mono">
                  stdout/stderr :: {{ selectedLogApp }} @ {{ node?.node_name }}
                </div>
                <div class="terminal-controls-right">
                  <span v-if="(selectedLogSince === 'custom' || showCustomDatePicker) && (customLogFrom || customLogTo)" class="terminal-range-badge font-mono">
                    📅 Window: {{ formatBadgeDate(customLogFrom) || 'Start' }} → {{ formatBadgeDate(customLogTo) || 'Now' }}
                  </span>
                  <span v-else-if="selectedLogSince !== 'all'" class="terminal-range-badge font-mono">
                    ⏱️ Window: Last {{ selectedLogSince }}
                  </span>
                  <span v-if="isLogStreaming" class="terminal-stream-badge font-mono">
                    <span class="log-live-dot"></span> LIVE (3s)
                  </span>
                  <span v-else class="terminal-stream-badge paused font-mono">
                    ⏸️ PAUSED
                  </span>
                  <div class="terminal-stats font-mono text-slate">
                    Showing {{ parsedAppLogLines.length }} lines (All: {{ appLogLevelCounts.total }} | Errors: {{ appLogLevelCounts.error }} | Warns: {{ appLogLevelCounts.warn }})
                  </div>
                </div>
              </div>

              <div class="terminal-body" v-if="appLogsLoading">
                <div class="terminal-loading">
                  <div class="terminal-spinner"></div>
                  <span>Streaming live stdout/stderr for {{ selectedLogApp }}...</span>
                </div>
              </div>

              <div class="terminal-body" v-else-if="appLogsError">
                <div class="terminal-error">
                  <span class="text-rose font-bold">⚠️ {{ appLogsError }}</span>
                  <button class="btn btn-xs btn-secondary mt-2" @click="fetchNodeAppLogs(selectedLogApp)">Try Again</button>
                </div>
              </div>

              <div
                ref="terminalBodyRef"
                class="terminal-body virtual-terminal"
                @scroll="handleTerminalScroll"
                v-else-if="parsedAppLogLines.length > 0"
              >
                <div class="virtual-scroll-spacer" :style="{ height: `${totalVirtualHeight}px`, position: 'relative', width: '100%' }">
                  <div
                    v-for="line in visibleLogLines"
                    :key="line.id"
                    class="log-line"
                    :class="`log-line-${line.level}`"
                    :style="{ position: 'absolute', top: `${line.virtualTop}px`, left: 0, right: 0, height: `${LINE_HEIGHT}px` }"
                  >
                    <span class="log-line-num font-mono">{{ line.id }}</span>
                    <span class="log-line-level font-mono" :class="`text-${line.level === 'error' ? 'rose' : line.level === 'warn' ? 'amber' : 'cyan'}`">
                      [{{ line.level.toUpperCase() }}]
                    </span>
                    <span class="log-line-content font-mono">{{ line.cleanText }}</span>
                  </div>
                </div>
              </div>

              <div class="terminal-body" v-else>
                <div class="terminal-empty">
                  <span>🛡️ No matching log lines found for current filters.</span>
                </div>
              </div>

              <!-- Floating Jump to Bottom Button -->
              <button
                v-if="userScrolledUp && parsedAppLogLines.length > 0"
                class="jump-bottom-btn font-mono"
                @click="scrollToBottom"
              >
                ⬇️ New logs available - Jump to bottom
              </button>
            </div>

            <!-- Incidents Table if any -->
            <div class="incidents-table-wrapper mt-4" v-if="nodeHistoryData?.incidents && nodeHistoryData.incidents.length > 0">
              <h5 class="inc-subheading">🚨 Automated Anomaly & Incident Snapshots</h5>
              <table class="incidents-mini-table">
                <thead>
                  <tr>
                    <th>Type & Severity</th>
                    <th>Message & Root Cause</th>
                    <th>Pre-Crash State</th>
                    <th>Status</th>
                    <th>Action</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="inc in nodeHistoryData.incidents" :key="inc.id" class="inc-row">
                    <td>
                      <div class="inc-type-cell">
                        <span class="badge" :class="inc.severity === 'critical' ? 'badge-rose' : inc.severity === 'high' ? 'badge-amber' : 'badge-indigo'">
                          {{ inc.severity?.toUpperCase() || 'HIGH' }}
                        </span>
                        <strong class="inc-type-name font-mono">{{ inc.type }}</strong>
                      </div>
                    </td>
                    <td class="col-inc-msg">
                      <div class="inc-msg-text">{{ inc.message }}</div>
                      <div class="inc-meta-tags font-mono" v-if="inc.raw_data">
                        <span v-if="inc.raw_data.top_processes" class="inc-tag">Has Process Dump</span>
                        <span v-if="inc.raw_data.container_count" class="inc-tag">{{ inc.raw_data.container_count }} containers</span>
                      </div>
                    </td>
                    <td class="font-mono text-slate">
                      <div v-if="inc.raw_data?.pre_incident_cpu">CPU: {{ inc.raw_data.pre_incident_cpu }}</div>
                      <div v-if="inc.raw_data?.pre_incident_memory">RAM: {{ inc.raw_data.pre_incident_memory }}</div>
                      <span v-if="!inc.raw_data?.pre_incident_cpu && !inc.raw_data?.pre_incident_memory" class="text-muted">—</span>
                    </td>
                    <td>
                      <span class="badge" :class="inc.status === 'resolved' ? 'badge-emerald' : inc.status === 'remediating' ? 'badge-cyan' : 'badge-amber'">
                        {{ (inc.status || 'open').toUpperCase() }}
                      </span>
                    </td>
                    <td>
                      <button class="btn btn-xs btn-secondary font-mono" @click="selectIncidentLog(inc)">
                        🔍 Logs
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>


</template>

<style scoped>
/* Incidents Section */
.node-incidents-section {
  padding: 16px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.incidents-section-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.inc-title-group {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.inc-title-group h4 {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.inc-desc {
  font-size: 11px;
  color: var(--text-muted);
  margin: 0;
}

.incidents-table-wrapper {
  overflow-x: auto;
}

.incidents-mini-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.incidents-mini-table th {
  text-align: left;
  padding: 8px 10px;
  color: var(--text-muted);
  font-size: 10px;
  text-transform: uppercase;
  border-bottom: 1px solid var(--border-subtle);
}

.incidents-mini-table td {
  padding: 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.inc-type-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.inc-type-name {
  font-size: 11px;
  color: var(--text-primary);
}

.inc-msg-text {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}

.inc-meta-tags {
  display: flex;
  gap: 6px;
  margin-top: 4px;
}

.inc-tag {
  font-size: 10px;
  background: rgba(255, 255, 255, 0.06);
  padding: 1px 6px;
  border-radius: 4px;
  color: var(--text-muted);
}

.inc-empty-state {
  text-align: center;
  padding: 24px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  color: var(--text-muted);
}

.inc-empty-icon {
  font-size: 24px;
}

.inc-empty-state h5 {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
  margin: 0;
}

.inc-empty-state p {
  font-size: 11px;
  margin: 0;
  max-width: 400px;
}

/* Real App & Failure Log Terminal Viewer */
.log-explorer-toolbar {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px 12px;
  background: rgba(15, 23, 42, 0.5);
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  margin-top: 4px;
}

.log-toolbar-row-top,
.log-toolbar-row-bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  width: 100%;
}

.log-toolbar-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.log-toolbar-label {
  font-size: 10.5px;
  font-weight: 700;
  color: var(--text-muted);
  white-space: nowrap;
}

.select-log-input {
  padding: 5px 8px;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  color: #38bdf8;
  font-size: 11.5px;
  font-weight: 600;
  outline: none;
  cursor: pointer;
  transition: border-color 0.2s ease;
}

.select-log-input optgroup {
  background: #0f172a;
  color: var(--text-muted, #94a3b8);
  font-weight: 700;
}

.select-log-input option {
  background: #0f172a;
  color: #f8fafc;
}

.select-log-input:hover,
.select-log-input:focus {
  border-color: #38bdf8;
}

.btn-clear-window-pill {
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.35);
  color: #fb7185;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-clear-window-pill:hover {
  background: rgba(244, 63, 94, 0.25);
  border-color: #fb7185;
  transform: translateY(-1px);
}

.btn-log-download {
  color: #38bdf8 !important;
  border-color: rgba(56, 189, 248, 0.3) !important;
}

.btn-log-download:hover {
  background: rgba(56, 189, 248, 0.15) !important;
  border-color: rgba(56, 189, 248, 0.6) !important;
}

.log-level-chips {
  display: flex;
  align-items: center;
  gap: 6px;
}

.chip-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 11px;
  border-radius: 6px;
  font-family: var(--font-mono, monospace);
  font-size: 11.5px;
  font-weight: 700;
  cursor: pointer;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: var(--text-secondary, #94a3b8);
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  user-select: none;
}

.chip-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #f8fafc;
  border-color: rgba(255, 255, 255, 0.25);
  transform: translateY(-1px);
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

.chip-btn.chip-rose:hover {
  background: rgba(244, 63, 94, 0.16);
  border-color: rgba(244, 63, 94, 0.45);
  color: #fda4af;
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

.chip-btn.chip-amber:hover {
  background: rgba(245, 158, 11, 0.16);
  border-color: rgba(245, 158, 11, 0.45);
  color: #fde68a;
}

.chip-btn.chip-amber.active {
  background: rgba(245, 158, 11, 0.22);
  border-color: #f59e0b;
  color: #fde68a;
  box-shadow: 0 0 12px rgba(245, 158, 11, 0.4);
}

.log-search-box {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 180px;
}

.input-log-search {
  width: 100%;
  padding: 6px 28px 6px 30px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  color: var(--text-primary);
  font-size: 11.5px;
}

.input-log-search:focus {
  border-color: #38bdf8;
  outline: none;
}

.log-action-btns {
  display: flex;
  align-items: center;
  gap: 6px;
}

.btn-log-action {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 6px 10px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-log-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
  color: var(--text-primary);
  border-color: rgba(56, 189, 248, 0.4);
}

.btn-log-stream {
  border-color: rgba(16, 185, 129, 0.3);
}

.btn-log-stream.stream-active {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border-color: rgba(16, 185, 129, 0.5);
}

.btn-log-stream:not(.stream-active) {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border-color: rgba(245, 158, 11, 0.4);
}

.btn-log-action.active-toggle {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border-color: rgba(56, 189, 248, 0.4);
}

.spin-icon {
  display: inline-block;
  animation: spin 1s linear infinite;
}

@keyframes logPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.85); }
}

.log-live-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.8);
  animation: logPulse 2s infinite ease-in-out;
  display: inline-block;
}

.jump-bottom-btn {
  position: absolute;
  bottom: 12px;
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

/* Custom Date-Time Range Selector Bar */
.custom-range-bar,
.custom-log-range-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 10px 14px;
  background: rgba(15, 23, 42, 0.7);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(56, 189, 248, 0.25);
  border-radius: 8px;
  margin-top: 6px;
  margin-bottom: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
}

.custom-range-inputs {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.range-field {
  display: flex;
  align-items: center;
  gap: 6px;
}

.range-label {
  font-size: 10.5px;
  font-weight: 700;
  color: #38bdf8;
  letter-spacing: 0.05em;
}

.input-datetime {
  color-scheme: dark;
  background: rgba(0, 0, 0, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  color: #38bdf8;
  font-size: 11.5px;
  padding: 5px 8px;
  outline: none;
  transition: all 0.2s ease;
}

.input-datetime:focus {
  border-color: #38bdf8;
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.2);
  background: rgba(0, 0, 0, 0.7);
}

.custom-range-presets {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.preset-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
}

.btn-preset-chip {
  padding: 3px 8px;
  font-size: 11px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-preset-chip:hover {
  background: rgba(56, 189, 248, 0.15);
  border-color: rgba(56, 189, 248, 0.4);
  color: #38bdf8;
}

.custom-range-actions {
  display: flex;
  align-items: center;
}

.btn-apply-range {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  background: linear-gradient(135deg, rgba(14, 165, 233, 0.3) 0%, rgba(56, 189, 248, 0.15) 100%);
  border: 1px solid rgba(56, 189, 248, 0.6);
  border-radius: 6px;
  color: #38bdf8;
  font-size: 11.5px;
  font-weight: 700;
  cursor: pointer;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.25);
  transition: all 0.2s ease;
}

.btn-apply-range:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(14, 165, 233, 0.5) 0%, rgba(56, 189, 248, 0.3) 100%);
  border-color: #38bdf8;
  box-shadow: 0 0 14px rgba(56, 189, 248, 0.45);
  transform: translateY(-1px);
}

.btn-apply-range:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.glow-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #38bdf8;
  box-shadow: 0 0 6px #38bdf8;
}

/* Terminal Viewer */
.log-terminal-viewer {
  position: relative;
  background: #080c14;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  overflow: hidden;
  box-shadow: inset 0 2px 10px rgba(0, 0, 0, 0.6);
  margin-top: 4px;
}

.terminal-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  background: rgba(15, 23, 42, 0.85);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.terminal-dots {
  display: flex;
  gap: 6px;
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
  color: var(--text-secondary);
  letter-spacing: 0.02em;
}

.terminal-controls-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.terminal-range-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 10.5px;
  padding: 2px 8px;
  border-radius: 9999px;
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.3);
  color: #38bdf8;
}

.terminal-stream-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 10.5px;
  padding: 2px 8px;
  border-radius: 9999px;
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.terminal-stream-badge.paused {
  background: rgba(245, 158, 11, 0.12);
  border-color: rgba(245, 158, 11, 0.3);
  color: #fbbf24;
}

.terminal-stats {
  font-size: 10.5px;
}

.terminal-body {
  max-height: 360px;
  overflow-y: auto;
  padding: 10px 14px;
  font-size: 11.5px;
  line-height: 1.6;
}

.virtual-terminal {
  overflow-y: auto;
  overflow-x: hidden;
}

.virtual-scroll-spacer {
  position: relative;
  width: 100%;
}

.terminal-loading,
.terminal-empty,
.terminal-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 30px;
  color: var(--text-muted);
  gap: 8px;
}

.terminal-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid rgba(56, 189, 248, 0.2);
  border-top-color: #38bdf8;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.log-line {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px;
  border-radius: 3px;
  transition: background 0.1s ease;
  box-sizing: border-box;
  overflow: hidden;
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
  line-height: 22px;
}

.log-line-level {
  width: 55px;
  flex-shrink: 0;
  font-weight: 700;
  font-size: 10px;
  letter-spacing: 0.02em;
  line-height: 22px;
}

.log-line-content {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: #e2e8f0;
  line-height: 22px;
}

.log-line-error .log-line-content {
  color: #fda4af;
}

.log-line-warn .log-line-content {
  color: #fde68a;
}

.text-rose { color: #f43f5e; }
.text-amber { color: #f59e0b; }
.text-cyan { color: #38bdf8; }
.text-slate { color: #94a3b8; }

.inc-subheading {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-secondary);
  margin: 0 0 8px 0;
}

</style>
