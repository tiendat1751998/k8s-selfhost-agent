<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useLogStreamer } from '../composables/useLogStreamer'
import LogTargetTree, { type LogTarget } from '../components/logs/LogTargetTree.vue'
import LogTelemetryStrip from '../components/logs/LogTelemetryStrip.vue'
import LogViewerTerminal from '../components/logs/LogViewerTerminal.vue'
import type { LogFilterParams } from '../api/logging'

const {
  logStore,
  searchKeyword,
  selectedLevel,
  autoScroll,
  isScrollLocked,
  linesStreamed,
  latency,
  errorRate,
  maxBufferSize,
  isConnected,
  isPaused,
  totalBufferCount,
  clearBuffer,
  togglePause,
  setTerminalRef,
  scrollToBottom,
  handleScroll,
} = useLogStreamer()

const selectedTarget = ref<LogTarget>({
  type: 'all',
  id: 'all',
  name: 'All Cluster Logs',
  icon: '🌐',
})

const showMobileTree = ref(false)
const mode = ref<'live' | 'historical'>('live')
const selectedTimeRange = ref('1h')
const timeRanges = [
  { label: '15m', ms: 900000, interval: 15 },
  { label: '1h', ms: 3600000, interval: 60 },
  { label: '6h', ms: 21600000, interval: 300 },
  { label: '24h', ms: 86400000, interval: 1800 },
]

async function runHistoricalQuery() {
  const cfg = timeRanges.find((r) => r.label === selectedTimeRange.value) || timeRanges[1]
  const now = new Date()
  const start = new Date(now.getTime() - cfg.ms).toISOString()
  const end = now.toISOString()
  const filter: LogFilterParams = {
    start_time: start,
    end_time: end,
    query: searchKeyword.value.trim() || undefined,
    log_level: selectedLevel.value || undefined,
    limit: 100,
    pod_name: selectedTarget.value.type === 'node' ? selectedTarget.value.id : undefined,
    container_name: selectedTarget.value.type === 'service' ? selectedTarget.value.id : undefined,
  }
  await Promise.all([
    logStore.fetchHistoricalLogs(filter),
    logStore.fetchHistogram({ ...filter, interval_seconds: cfg.interval }),
  ])
}

function connectTarget(target: LogTarget) {
  if (target.type === 'node') logStore.connect({ node: target.id })
  else if (target.type === 'service') logStore.connect({ service: target.id })
  else logStore.connect()
}

watch(selectedTarget, (target) => {
  if (mode.value === 'live') {
    connectTarget(target)
    fetchLiveHistogram()
  } else {
    runHistoricalQuery()
  }
})

watch(mode, (newMode) => {
  if (newMode === 'historical') {
    logStore.disconnect()
    runHistoricalQuery()
  } else {
    connectTarget(selectedTarget.value)
    fetchLiveHistogram()
  }
})

async function fetchLiveHistogram() {
  const now = new Date()
  await logStore.fetchHistogram({
    start_time: new Date(now.getTime() - 3600000).toISOString(),
    end_time: now.toISOString(),
    interval_seconds: 60,
    pod_name: selectedTarget.value.type === 'node' ? selectedTarget.value.id : undefined,
    container_name: selectedTarget.value.type === 'service' ? selectedTarget.value.id : undefined,
  }).catch(() => {})
}

onMounted(() => {
  if (mode.value === 'live') fetchLiveHistogram()
})

function toggleLiveTail() {
  if (autoScroll.value && !isScrollLocked.value) {
    autoScroll.value = false
    isScrollLocked.value = true
  } else {
    autoScroll.value = true
    isScrollLocked.value = false
    scrollToBottom()
  }
}

interface DisplayBucket {
  key: string
  label: string
  count: number
  hasError: boolean
  hasWarn: boolean
}

const sparklineBuckets = computed<DisplayBucket[]>(() => {
  if (logStore.histogram.length > 0) {
    return logStore.histogram.map((b) => ({
      key: b.time_bucket,
      label: b.time_bucket.includes('T') ? b.time_bucket.split('T')[1].slice(0, 8) : b.time_bucket,
      count: Number(b.total_count) || 0,
      hasError: !!((b.level_count?.error || b.level_count?.ERROR) || (b.level_count?.fatal || b.level_count?.FATAL)),
      hasWarn: !!(b.level_count?.warn || b.level_count?.WARN),
    }))
  }
  const logs = targetFilteredLogs.value
  if (!logs.length) return []
  const step = Math.max(1, Math.floor(logs.length / 20))
  return Array.from({ length: Math.min(20, Math.ceil(logs.length / step)) }, (_, i) => {
    const slice = logs.slice(i * step, (i + 1) * step)
    return {
      key: 'b-' + i,
      label: slice[slice.length - 1]?.time || String(i),
      count: slice.length,
      hasError: slice.some((l) => l.level.toUpperCase() === 'ERROR'),
      hasWarn: slice.some((l) => l.level.toUpperCase() === 'WARN'),
    }
  })
})

const maxBucketVolume = computed(() => Math.max(...sparklineBuckets.value.map((b) => b.count), 1))
const hoveredBucket = ref<DisplayBucket | null>(null)

const targetFilteredLogs = computed(() => {
  const target = selectedTarget.value
  const level = selectedLevel.value
  const kw = searchKeyword.value.trim().toLowerCase()
  return logStore.logs.filter((log) => {
    if (level && log.level.toUpperCase() !== level.toUpperCase()) return false
    if (mode.value === 'live') {
      const q = target.id.toLowerCase()
      if (target.type === 'node' && !log.node?.toLowerCase().includes(q) && !log.pod?.toLowerCase().includes(q)) return false
      if (target.type === 'service' && !log.service?.toLowerCase().includes(q) && !log.container?.toLowerCase().includes(q)) return false
    }
    if (kw && !log.msg.toLowerCase().includes(kw) && !log.pod.toLowerCase().includes(kw) && !log.traceId?.toLowerCase().includes(kw)) return false
    return true
  })
})

function handleExport() {
  const logsToExport = targetFilteredLogs.value.length > 0 ? targetFilteredLogs.value : logStore.logs
  const content = logsToExport
    .map((l) => '[' + l.time + '] [' + l.level.padEnd(5) + '] [' + (l.node || l.namespace || 'node') + '/' + (l.service || l.pod || 'system') + ']: ' + l.msg + (l.traceId ? ' [trace=' + l.traceId + ']' : ''))
    .join('\n')

  if (typeof window !== 'undefined') {
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'k8s-logs-' + selectedTarget.value.id + '-' + new Date().toISOString().replace(/[:.]/g, '-') + '.log'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }
}
</script>

<template>
  <div class="view-container log-explorer-page">
    <header class="view-header">
      <div class="header-left">
        <div class="view-tag"><span class="pulse-dot pulse-dot-cyan"></span><span>CLICKHOUSE OBSERVABILITY & LOG EXPLORER</span></div>
        <h1 class="view-title">Enterprise Kubernetes Logs Explorer</h1>
      </div>
      <button type="button" class="mobile-tree-toggle-btn" aria-label="Toggle log targets drawer" @click="showMobileTree = !showMobileTree">
        <span>🌲 {{ selectedTarget.name }} ▾</span>
      </button>
    </header>

    <LogTelemetryStrip
      :lines-streamed="linesStreamed" :error-rate="errorRate" :buffer-size="totalBufferCount"
      :max-buffer-size="maxBufferSize" :latency="latency" :is-connected="mode === 'live' ? isConnected : true" :is-paused="isPaused"
    />

    <!-- Mode Switcher, Live Tail & Query Controls -->
    <div class="mode-controls-bar">
      <div class="mode-switcher-tabs font-mono" role="tablist">
        <button type="button" class="mode-tab-btn" :class="{ active: mode === 'live' }" @click="mode = 'live'"><span>⚡ Live Tail</span></button>
        <button type="button" class="mode-tab-btn" :class="{ active: mode === 'historical' }" @click="mode = 'historical'"><span>🔍 Historical Search</span></button>
      </div>

      <!-- Live Tail Button (Auto-scroll to bottom, pauses when user scrolls up) -->
      <button
        v-if="mode === 'live'"
        type="button"
        class="live-tail-toggle-btn font-mono"
        :class="{ 'tail-active': autoScroll && !isScrollLocked, 'tail-paused': !autoScroll || isScrollLocked }"
        :title="autoScroll && !isScrollLocked ? 'Live Tail active. Click to lock.' : 'Tail paused on scroll up. Click to resume.'"
        @click="toggleLiveTail"
      >
        <span class="tail-dot"></span>
        <span>{{ autoScroll && !isScrollLocked ? 'LIVE TAIL ON' : 'TAIL PAUSED (SCROLLED UP)' }}</span>
      </button>

      <!-- Historical Time Range & Query Trigger -->
      <div v-if="mode === 'historical'" class="historical-query-group font-mono">
        <div class="time-range-picker">
          <button
            v-for="r in timeRanges" :key="r.label" type="button" class="range-pill-btn"
            :class="{ active: selectedTimeRange === r.label }" @click="selectedTimeRange = r.label; runHistoricalQuery()"
          >{{ r.label }}</button>
        </div>
        <button type="button" class="historical-search-btn" :disabled="logStore.isHistoricalLoading" @click="runHistoricalQuery">
          <span>{{ logStore.isHistoricalLoading ? '⏳ Searching...' : '⚡ Query ClickHouse' }}</span>
        </button>
        <span v-if="logStore.totalHistoricalCount > 0" class="query-count-badge">{{ logStore.totalHistoricalCount }} logs found</span>
      </div>
    </div>

    <!-- Log Volume Sparkline Strip -->
    <section class="log-volume-sparkline-strip glass-panel" aria-label="Log volume histogram sparkline">
      <div class="sparkline-header font-mono">
        <div class="sparkline-title-group">
          <span>📊 LOG VOLUME SPARKLINE</span>
          <span class="sparkline-meta">{{ mode === 'historical' ? 'ClickHouse Sparse Index' : 'Live Buffer' }}</span>
        </div>
        <div class="sparkline-stats">
          <span>Peak: <strong>{{ maxBucketVolume }}</strong></span>
          <span v-if="hoveredBucket">Hover: <strong>{{ hoveredBucket.count }}</strong> @ {{ hoveredBucket.label }}</span>
        </div>
      </div>
      <div class="sparkline-bars">
        <div
          v-for="b in sparklineBuckets" :key="b.key" class="sparkline-bar-col" :title="b.label + ': ' + b.count + ' logs'"
          @mouseenter="hoveredBucket = b" @mouseleave="hoveredBucket = null"
        >
          <div class="sparkline-bar-fill" :class="{ 'bar-error': b.hasError, 'bar-warn': !b.hasError && b.hasWarn }" :style="{ height: Math.max(10, Math.round((b.count / maxBucketVolume) * 100)) + '%' }"></div>
        </div>
        <div v-if="sparklineBuckets.length === 0" class="sparkline-empty font-mono"><span>Awaiting log ingestion for histogram sparkline...</span></div>
      </div>
    </section>

    <div v-if="showMobileTree" class="mobile-backdrop" aria-hidden="true" @click="showMobileTree = false"></div>

    <div class="log-explorer-grid">
      <div class="explorer-left-col" :class="{ 'mobile-tree-open': showMobileTree }">
        <div v-if="showMobileTree" class="mobile-drawer-header">
          <span class="drawer-title font-mono">🌲 Select Target</span>
          <button type="button" class="drawer-close-btn" aria-label="Close targets drawer" @click="showMobileTree = false">✕</button>
        </div>
        <LogTargetTree v-model="selectedTarget" :logs="logStore.logs" @select="showMobileTree = false" />
      </div>

      <div class="explorer-right-col">
        <LogViewerTerminal
          :logs="targetFilteredLogs" :is-connected="mode === 'live' ? isConnected : true" :is-paused="isPaused"
          :auto-scroll="autoScroll" :is-scroll-locked="isScrollLocked" :search-query="searchKeyword"
          :selected-level="selectedLevel" :target-name="selectedTarget.name" :latency="latency"
          @update:search-query="searchKeyword = $event" @update:selected-level="selectedLevel = $event"
          @update:auto-scroll="autoScroll = $event" @toggle-pause="togglePause" @clear-buffer="clearBuffer"
          @export-logs="handleExport" @scroll="handleScroll" @scroll-to-bottom="scrollToBottom"
          @register-terminal="setTerminalRef" @toggle-target-tree="showMobileTree = !showMobileTree"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../assets/styles/views/logstream.css';

.mode-controls-bar { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 10px; padding: 8px 12px; background: rgba(15, 23, 42, 0.75); border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; }
.mode-switcher-tabs { display: inline-flex; background: rgba(2, 6, 23, 0.6); padding: 3px; border-radius: 6px; border: 1px solid rgba(255, 255, 255, 0.08); gap: 4px; }
.mode-tab-btn { background: transparent; border: none; color: #94a3b8; font-size: 11px; font-weight: 600; padding: 5px 12px; border-radius: 4px; cursor: pointer; transition: all 0.15s ease; }
.mode-tab-btn:hover { color: #f1f5f9; }
.mode-tab-btn.active { background: rgba(56, 189, 248, 0.18); color: #38bdf8; border: 1px solid rgba(56, 189, 248, 0.35); }
.live-tail-toggle-btn { display: inline-flex; align-items: center; gap: 6px; padding: 5px 12px; font-size: 11px; font-weight: 700; border-radius: 6px; cursor: pointer; border: 1px solid transparent; }
.live-tail-toggle-btn.tail-active { background: rgba(16, 185, 129, 0.15); border-color: rgba(16, 185, 129, 0.35); color: #34d399; }
.live-tail-toggle-btn.tail-paused { background: rgba(245, 158, 11, 0.15); border-color: rgba(245, 158, 11, 0.35); color: #fbbf24; }
.tail-dot { width: 7px; height: 7px; border-radius: 50%; background: currentColor; }
.tail-active .tail-dot { animation: pulse-dot 1.5s infinite; }
.historical-query-group { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.time-range-picker { display: inline-flex; background: rgba(2, 6, 23, 0.6); padding: 2px; border-radius: 4px; border: 1px solid rgba(255, 255, 255, 0.08); }
.range-pill-btn { background: transparent; border: none; color: #94a3b8; font-size: 10px; padding: 3px 8px; border-radius: 3px; cursor: pointer; }
.range-pill-btn.active { background: rgba(56, 189, 248, 0.2); color: #38bdf8; }
.historical-search-btn { background: rgba(56, 189, 248, 0.15); border: 1px solid rgba(56, 189, 248, 0.35); color: #38bdf8; font-size: 11px; font-weight: 600; padding: 5px 12px; border-radius: 6px; cursor: pointer; }
.historical-search-btn:hover:not(:disabled) { background: rgba(56, 189, 248, 0.28); }
.query-count-badge { font-size: 11px; color: #94a3b8; padding: 2px 6px; border-radius: 4px; background: rgba(255, 255, 255, 0.04); }
.log-volume-sparkline-strip { padding: 8px 12px; background: rgba(11, 15, 25, 0.7); border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; display: flex; flex-direction: column; gap: 6px; }
.sparkline-header { display: flex; align-items: center; justify-content: space-between; font-size: 10px; }
.sparkline-title-group { display: flex; align-items: center; gap: 6px; color: #e2e8f0; font-weight: 700; }
.sparkline-meta { font-size: 9px; color: #38bdf8; padding: 1px 4px; border-radius: 3px; background: rgba(56, 189, 248, 0.1); font-weight: normal; }
.sparkline-stats { display: flex; gap: 8px; color: #64748b; }
.sparkline-stats strong { color: #f1f5f9; }
.sparkline-bars { display: flex; align-items: flex-end; height: 28px; gap: 3px; overflow-x: auto; padding: 2px 0; }
.sparkline-bar-col { flex: 1; min-width: 4px; max-width: 16px; height: 100%; display: flex; align-items: flex-end; cursor: pointer; }
.sparkline-bar-fill { width: 100%; background: #38bdf8; border-radius: 2px 2px 0 0; transition: height 0.2s ease; min-height: 2px; }
.sparkline-bar-col:hover .sparkline-bar-fill { background: #7dd3fc; filter: brightness(1.2); }
.sparkline-bar-fill.bar-error { background: #f43f5e; }
.sparkline-bar-fill.bar-warn { background: #f59e0b; }
.sparkline-empty { font-size: 10px; color: #64748b; margin: auto; }
@keyframes pulse-dot { 0%, 100% { opacity: 1; transform: scale(1); } 50% { opacity: 0.4; transform: scale(0.85); } }
@media (max-width: 640px) { .mode-controls-bar { padding: 6px 8px; gap: 6px; } .sparkline-bars { height: 22px; } .log-volume-sparkline-strip { padding: 6px 8px; } }
</style>
