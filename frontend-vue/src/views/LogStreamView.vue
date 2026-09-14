<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useLogStreamer } from '../composables/useLogStreamer'
import LogTargetTree, { type LogTarget } from '../components/logs/LogTargetTree.vue'
import LogViewerTerminal from '../components/logs/LogViewerTerminal.vue'
import LogVolumeHistogram from '../components/logs/LogVolumeHistogram.vue'
import ClickHouseEngineBadge from '../components/logs/ClickHouseEngineBadge.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import type { LogFilterParams } from '../api/logging'

const {
  logStore, searchKeyword, selectedLevel, autoScroll, isScrollLocked, linesStreamed,
  latency, errorRate, isConnected, isPaused,
  clearBuffer, scrollToBottom, handleScroll, setTerminalRef,
} = useLogStreamer()

const selectedTarget = ref<LogTarget>({ type: 'all', id: 'all', name: 'All Cluster Logs' })
const showMobileTree = ref(false)
const isSidebarCollapsed = ref(false)
const showHistogram = ref(false)
const wrapLines = ref(true)

const mode = ref<'live' | 'historical'>('live')
const selectedTimeRange = ref('1h')
const queryError = ref<string | null>(null)
const isSearching = ref(false)
const currentOffset = ref(0)
const selectedHistoricalLimit = ref(1000)

const severityFilters = [
  { label: 'ALL', value: '' },
  { label: 'ERR', value: 'ERROR' },
  { label: 'WARN', value: 'WARN' },
  { label: 'INFO', value: 'INFO' },
  { label: 'DEBUG', value: 'DEBUG' },
]

function isCurrentLevel(val: string): boolean {
  if (!val) return !selectedLevel.value || selectedLevel.value === 'ALL'
  if (val === 'ERROR') return selectedLevel.value === 'ERROR' || selectedLevel.value === 'ERR'
  return selectedLevel.value.toUpperCase() === val.toUpperCase()
}

function setLevel(val: string) {
  selectedLevel.value = val
}

const timeRanges = [
  { label: '15m', ms: 900000, interval: 15 },
  { label: '1h', ms: 3600000, interval: 60 },
  { label: '6h', ms: 21600000, interval: 300 },
  { label: '24h', ms: 86400000, interval: 1800 },
]

async function runHistoricalQuery(isLoadMore = false) {
  if (mode.value !== 'historical') return
  queryError.value = null
  if (!isLoadMore) currentOffset.value = 0
  const cfg = timeRanges.find((r) => r.label === selectedTimeRange.value) || timeRanges[1]
  const now = new Date()
  const kw = searchKeyword.value.trim()
  const target = selectedTarget.value
  const queryParts = [kw, target.type === 'node' ? target.id : ''].filter(Boolean)

  const filter: LogFilterParams = {
    start_time: new Date(now.getTime() - cfg.ms).toISOString(),
    end_time: now.toISOString(),
    query: queryParts.length ? queryParts.join(' ') : undefined,
    log_level: (selectedLevel.value && selectedLevel.value !== 'ALL') ? selectedLevel.value : undefined,
    limit: selectedHistoricalLimit.value,
    offset: currentOffset.value,
    container_name: target.type === 'service' ? target.id : undefined,
  }

  try {
    isSearching.value = true
    const promises: [Promise<unknown>, Promise<unknown>?] = [
      logStore.fetchHistoricalLogs(filter, isLoadMore),
    ]
    if (!isLoadMore) {
      promises.push(logStore.fetchHistogram({
        start_time: filter.start_time,
        end_time: filter.end_time,
        query: filter.query,
        log_level: filter.log_level,
        interval_seconds: cfg.interval,
        container_name: filter.container_name,
      }))
    }
    await Promise.all(promises)
    if (mode.value !== 'historical') return
    if (isLoadMore) currentOffset.value += selectedHistoricalLimit.value
  } catch (err: unknown) {
    if (mode.value === 'historical') {
      queryError.value = err instanceof Error ? err.message : 'ClickHouse search failed'
    }
  } finally {
    isSearching.value = false
  }
}

function loadMoreHistorical() {
  currentOffset.value += selectedHistoricalLimit.value
  runHistoricalQuery(true)
}

async function preloadRecentLogs(target: LogTarget) {
  try {
    const kw = searchKeyword.value.trim()
    const queryParts = [kw, target.type === 'node' ? target.id : ''].filter(Boolean)
    await logStore.fetchHistoricalLogs({
      limit: 50,
      query: queryParts.length ? queryParts.join(' ') : undefined,
      container_name: target.type === 'service' ? target.id : undefined,
      log_level: (selectedLevel.value && selectedLevel.value !== 'ALL') ? selectedLevel.value : undefined,
    }, false)
  } catch {
    // Gracefully ignore if offline or no historical logs
  }
}

function connectTarget(target: LogTarget) {
  if (target.type === 'node') logStore.connect({ node: target.id })
  else if (target.type === 'service') logStore.connect({ service: target.id })
  else logStore.connect()
  preloadRecentLogs(target)
}

watch(selectedTarget, (t) => {
  if (mode.value === 'live') { connectTarget(t); fetchLiveHistogram() }
  else runHistoricalQuery()
})

watch(mode, (newMode) => {
  if (newMode === 'historical') { logStore.disconnect(); runHistoricalQuery() }
  else { connectTarget(selectedTarget.value); fetchLiveHistogram() }
})

async function fetchLiveHistogram() {
  const now = new Date()
  await logStore.fetchHistogram({
    start_time: new Date(now.getTime() - 3600000).toISOString(),
    end_time: now.toISOString(),
    interval_seconds: 60,
    container_name: selectedTarget.value.type === 'service' ? selectedTarget.value.id : undefined,
  }).catch(() => {})
}

onMounted(() => {
  if (mode.value === 'live') {
    connectTarget(selectedTarget.value)
    fetchLiveHistogram()
  }
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

async function handleHistogramFilterRange(range: { start: string; end: string }) {
  if (mode.value === 'live') mode.value = 'historical'
  const kw = searchKeyword.value.trim()
  const target = selectedTarget.value
  const queryParts = [kw, target.type === 'node' ? target.id : ''].filter(Boolean)
  currentOffset.value = 0
  try {
    isSearching.value = true
    queryError.value = null
    await logStore.fetchHistoricalLogs({
      start_time: range.start,
      end_time: range.end,
      query: queryParts.length ? queryParts.join(' ') : undefined,
      log_level: (selectedLevel.value && selectedLevel.value !== 'ALL') ? selectedLevel.value : undefined,
      limit: selectedHistoricalLimit.value,
      offset: 0,
      container_name: target.type === 'service' ? target.id : undefined,
    })
  } catch (err: unknown) {
    queryError.value = err instanceof Error ? err.message : 'Historical search failed'
  } finally {
    isSearching.value = false
  }
}

function handleClearHistogramFilter() {
  runHistoricalQuery()
}

const targetFilteredLogs = computed(() => {
  const target = selectedTarget.value
  const level = selectedLevel.value
  const rawKw = searchKeyword.value.trim()
  let reg: RegExp | null = null
  if (rawKw) {
    try { reg = new RegExp(rawKw, 'i') } catch { reg = null }
  }
  const kw = rawKw.toLowerCase()

  return logStore.logs.filter((log) => {
    if (level && level !== 'ALL') {
      const l = log.level.toUpperCase()
      const f = level.toUpperCase()
      const match = (f === 'ERR' || f === 'ERROR') ? (l === 'ERROR' || l === 'ERR')
        : (f === 'WARN' || f === 'WARNING') ? (l === 'WARN' || l === 'WARNING')
        : l === f
      if (!match) return false
    }
    if (mode.value === 'live') {
      const q = target.id.toLowerCase()
      if (target.type === 'node' && !log.node?.toLowerCase().includes(q) && !log.pod?.toLowerCase().includes(q)) return false
      if (target.type === 'service' && !log.service?.toLowerCase().includes(q) && !log.container?.toLowerCase().includes(q)) return false
    }
    if (rawKw) {
      if (reg) {
        if (!reg.test(log.msg) && !reg.test(log.pod) && !(log.traceId && reg.test(log.traceId))) return false
      } else {
        if (!log.msg.toLowerCase().includes(kw) && !log.pod.toLowerCase().includes(kw) && !log.traceId?.toLowerCase().includes(kw)) return false
      }
    }
    return true
  })
})

function handleExport() {
  const logsToExport = targetFilteredLogs.value.length > 0 ? targetFilteredLogs.value : logStore.logs
  const content = logsToExport
    .map((l) => `[${l.time}] [${l.level.padEnd(5)}] [${l.node || l.namespace || 'node'}/${l.service || l.pod || 'system'}]: ${l.msg}${l.traceId ? ' [trace=' + l.traceId + ']' : ''}`)
    .join('\n')

  if (typeof window !== 'undefined') {
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `k8s-logs-${selectedTarget.value.id}-${new Date().toISOString().replace(/[:.]/g, '-')}.log`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }
}
</script>

<template>
  <div class="view-container log-explorer-page">
    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="logs-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="Kubernetes Log Stream Controls">
      <!-- Zone 1 (Left - Sidebar & Stream Search) -->
      <div class="toolbar-zone-left">
        <button type="button" class="toolbar-btn btn-secondary" :class="{ active: isSidebarCollapsed }" title="Toggle Log Targets Sidebar" aria-label="Toggle Log Targets Sidebar" @click="isSidebarCollapsed = !isSidebarCollapsed"><BaseIcon name="sidebar" size="xs" /></button>
        <span class="toolbar-target-badge font-mono" :title="selectedTarget.name"><BaseIcon name="layers" size="xs" /> <span>{{ selectedTarget.name }}</span></span>
        <div class="toolbar-search-wrap">
          <BaseIcon name="search" size="xs" class="search-icon" />
          <input v-model="searchKeyword" type="text" placeholder="Filter logs (regex)..." class="toolbar-search-input font-mono" aria-label="Filter logs" />
          <button v-if="searchKeyword" type="button" class="clear-input-btn" aria-label="Clear filter" @click="searchKeyword = ''"><BaseIcon name="x" size="xs" /></button>
        </div>
      </div>

      <!-- Zone 2 (Center-Left - Mode Tabs & Severity Pills) -->
      <div class="toolbar-zone-mode">
        <div class="toolbar-nav-pills font-mono" role="tablist" aria-label="Stream Mode">
          <button type="button" role="tab" :aria-selected="mode === 'live'" class="toolbar-pill-btn" :class="{ active: mode === 'live' }" @click="mode = 'live'"><BaseIcon name="zap" size="xs" /> <span>Live Tail</span></button>
          <button type="button" role="tab" :aria-selected="mode === 'historical'" class="toolbar-pill-btn" :class="{ active: mode === 'historical' }" @click="mode = 'historical'"><BaseIcon name="search" size="xs" /> <span>Historical Search</span></button>
        </div>

        <!-- In Live Tail mode: 28px quick severity filter pills -->
        <div v-if="mode === 'live'" class="toolbar-severity-pills font-mono" role="group" aria-label="Filter by Severity">
          <button v-for="lvl in severityFilters" :key="lvl.value" type="button" class="severity-pill-btn" :class="['pill-' + lvl.label.toLowerCase(), { active: isCurrentLevel(lvl.value) }]" @click="setLevel(lvl.value)">{{ lvl.label }}</button>
        </div>

        <!-- In Historical Search mode: time range pills, limit select + query button -->
        <div v-if="mode === 'historical'" class="toolbar-historical-group font-mono">
          <div class="toolbar-time-pills">
            <button v-for="r in timeRanges" :key="r.label" type="button" class="time-pill-btn" :class="{ active: selectedTimeRange === r.label }" @click="selectedTimeRange = r.label; runHistoricalQuery()">{{ r.label }}</button>
          </div>
          <select v-model="selectedHistoricalLimit" class="historical-limit-select font-mono" title="Max Log Entries to Fetch" aria-label="Historical log limit" @change="runHistoricalQuery()">
            <option :value="100">100</option>
            <option :value="500">500</option>
            <option :value="1000">1k</option>
            <option :value="5000">5k</option>
            <option :value="10000">10k</option>
          </select>
          <button type="button" class="toolbar-btn btn-secondary" :disabled="logStore.isHistoricalLoading" title="Query ClickHouse" @click="runHistoricalQuery()"><BaseIcon name="search" size="xs" /> <span>{{ logStore.isHistoricalLoading ? 'Searching...' : 'Query ClickHouse' }}</span></button>
          <button v-if="logStore.hasMoreHistorical || logStore.totalHistoricalCount > logStore.logs.length" type="button" class="toolbar-btn btn-secondary load-more-compact" :disabled="logStore.isHistoricalLoading" title="Load more historical logs" @click="loadMoreHistorical"><span>+More ({{ logStore.logs.length }}/{{ logStore.totalHistoricalCount }})</span></button>
        </div>
      </div>

      <!-- Zone 3 (Center-Right - Telemetry & Engine Badge) -->
      <div class="toolbar-kpi-strip font-mono" role="status" aria-label="Live Stream Telemetry">
        <ClickHouseEngineBadge />
        <span class="kpi-badge font-mono"><span class="kpi-tag">[LIVE STREAM] </span><span class="kpi-metrics">{{ linesStreamed }} ev · {{ errorRate }}% err</span><span class="kpi-latency"> · {{ latency > 0 ? latency + 'ms' : '<50ms' }}</span></span>
      </div>

      <!-- Zone 4 (Right - Stream Actions Group) -->
      <div class="toolbar-actions-group">
        <button type="button" class="toolbar-btn btn-secondary" :class="{ 'btn-tail-active': autoScroll && !isScrollLocked, 'btn-tail-paused': !autoScroll || isScrollLocked }" @click="toggleLiveTail"><span class="tail-dot" /> <span>{{ autoScroll && !isScrollLocked ? 'Live Tail' : 'Paused' }}</span></button>
        <button type="button" class="toolbar-btn btn-secondary" :class="{ active: wrapLines }" title="Toggle Line Wrap" @click="wrapLines = !wrapLines"><span>Wrap</span></button>
        <button type="button" class="toolbar-btn btn-secondary" :class="{ active: showHistogram }" title="Toggle Volume Histogram" @click="showHistogram = !showHistogram"><BaseIcon name="bar-chart-2" size="xs" /> <span>Volume</span></button>
        <button type="button" class="toolbar-btn btn-secondary" title="Clear Buffer" aria-label="Clear Buffer" @click="clearBuffer"><BaseIcon name="trash" size="xs" /></button>
        <button type="button" class="toolbar-btn btn-secondary" title="Export Logs" aria-label="Export Logs" @click="handleExport"><BaseIcon name="download" size="xs" /></button>
      </div>
    </div>

    <!-- Mobile Command Bar (<768px) -->
    <div class="logs-mobile-command-bar mobile-only">
      <div class="mobile-bar-left">
        <button type="button" class="mobile-tree-toggle-btn font-mono" aria-label="Toggle log targets drawer" @click="showMobileTree = !showMobileTree"><BaseIcon name="layers" size="xs" /> <span>{{ selectedTarget.name }}</span><BaseIcon name="chevron-down" size="xs" /></button>
      </div>
      <div class="mobile-bar-actions">
        <button type="button" class="mobile-btn font-mono" :class="{ active: mode === 'live' }" @click="mode = mode === 'live' ? 'historical' : 'live'"><BaseIcon :name="mode === 'live' ? 'zap' : 'search'" size="xs" /> <span>{{ mode === 'live' ? 'Live' : 'History' }}</span></button>
        <button type="button" class="mobile-btn font-mono" :class="{ 'btn-tail-active': autoScroll && !isScrollLocked, 'btn-tail-paused': !autoScroll || isScrollLocked }" @click="toggleLiveTail"><span class="tail-dot" /> <span>{{ autoScroll && !isScrollLocked ? 'Tail' : 'Pause' }}</span></button>
        <button type="button" class="mobile-btn font-mono" :class="{ active: showHistogram }" aria-label="Toggle Volume Histogram" @click="showHistogram = !showHistogram"><BaseIcon name="bar-chart-2" size="xs" /></button>
        <button type="button" class="mobile-btn font-mono" title="Clear Buffer" aria-label="Clear Buffer" @click="clearBuffer"><BaseIcon name="trash" size="xs" /></button>
      </div>
    </div>

    <!-- Query Error Banner -->
    <div v-if="queryError" class="query-error-banner font-mono">
      <BaseIcon name="alert-triangle" size="xs" />
      <span>{{ queryError }}</span>
      <button type="button" class="query-error-dismiss" aria-label="Dismiss error" @click="queryError = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Collapsible Log Volume Histogram (70px height above grid) -->
    <div v-if="showHistogram || mode === 'historical'" class="histogram-wrapper">
      <LogVolumeHistogram :buckets="logStore.histogram" :height="70" @filter-range="handleHistogramFilterRange" @clear-filter="handleClearHistogramFilter" />
    </div>

    <div v-if="showMobileTree" class="mobile-backdrop" aria-hidden="true" @click="showMobileTree = false" />

    <!-- 2-Column Explorer Grid with Sidebar Collapse Affordance -->
    <div class="log-explorer-grid" :class="{ 'sidebar-collapsed': isSidebarCollapsed }">
      <div class="explorer-left-col" :class="{ 'mobile-tree-open': showMobileTree }">
        <div v-if="showMobileTree" class="mobile-drawer-header">
          <span class="drawer-title font-mono"><BaseIcon name="layers" size="xs" /> <span>Select Target</span></span>
          <button type="button" class="drawer-close-btn" aria-label="Close targets drawer" @click="showMobileTree = false">&times;</button>
        </div>
        <LogTargetTree v-model="selectedTarget" :logs="logStore.logs" @select="showMobileTree = false" />
      </div>

      <div class="explorer-right-col">
        <LogViewerTerminal
          :logs="targetFilteredLogs"
          :is-connected="mode === 'live' ? isConnected : false"
          :is-paused="isPaused"
          :auto-scroll="autoScroll"
          :is-scroll-locked="isScrollLocked"
          :target-name="selectedTarget.name"
          :latency="latency"
          :wrap-lines="wrapLines"
          @scroll="handleScroll"
          @scroll-to-bottom="scrollToBottom"
          @register-terminal="setTerminalRef"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../assets/styles/views/logstream.css';
</style>
