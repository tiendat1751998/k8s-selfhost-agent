<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useLogStreamer } from '../composables/useLogStreamer'
import LogTargetTree, { type LogTarget } from '../components/logs/LogTargetTree.vue'
import LogTelemetryStrip from '../components/logs/LogTelemetryStrip.vue'
import LogViewerTerminal from '../components/logs/LogViewerTerminal.vue'
import ClickHouseEngineBadge from '../components/logs/ClickHouseEngineBadge.vue'
import LogVolumeHistogram from '../components/logs/LogVolumeHistogram.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import type { LogFilterParams } from '../api/logging'

const {
  logStore, searchKeyword, selectedLevel, autoScroll, isScrollLocked, linesStreamed,
  latency, errorRate, maxBufferSize, isConnected, isPaused, totalBufferCount,
  clearBuffer, togglePause, setTerminalRef, scrollToBottom, handleScroll,
} = useLogStreamer()

const selectedTarget = ref<LogTarget>({ type: 'all', id: 'all', name: 'All Cluster Logs' })
const showMobileTree = ref(false)
const mode = ref<'live' | 'historical'>('live')
const selectedTimeRange = ref('1h')
const queryError = ref<string | null>(null)
const isSearching = ref(false)
const currentOffset = ref(0)
const PAGE_SIZE = 100

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
    log_level: selectedLevel.value || undefined,
    limit: PAGE_SIZE,
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
    if (isLoadMore) currentOffset.value += PAGE_SIZE
  } catch (err: unknown) {
    if (mode.value === 'historical') {
      queryError.value = err instanceof Error ? err.message : 'ClickHouse search failed'
    }
  } finally {
    isSearching.value = false
  }
}

function loadMoreHistorical() {
  currentOffset.value += PAGE_SIZE
  runHistoricalQuery(true)
}

function connectTarget(target: LogTarget) {
  if (target.type === 'node') logStore.connect({ node: target.id })
  else if (target.type === 'service') logStore.connect({ service: target.id })
  else logStore.connect()
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

onMounted(() => { if (mode.value === 'live') fetchLiveHistogram() })

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
  if (mode.value === 'live') {
    mode.value = 'historical'
  }
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
      log_level: selectedLevel.value || undefined,
      limit: PAGE_SIZE,
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
    if (level && log.level.toUpperCase() !== level.toUpperCase()) return false
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
    <header class="view-header">
      <div class="header-left">
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan" />
          <span>CLICKHOUSE OBSERVABILITY & LOG EXPLORER</span>
        </div>
        <h1 class="view-title">Enterprise Kubernetes Logs Explorer</h1>
      </div>
      <div class="header-actions">
        <ClickHouseEngineBadge />
        <button
          type="button"
          class="mobile-tree-toggle-btn font-mono"
          aria-label="Toggle log targets drawer"
          @click="showMobileTree = !showMobileTree"
        >
          <BaseIcon name="layers" size="xs" />
          <span>{{ selectedTarget.name }}</span>
          <BaseIcon name="chevron-down" size="xs" />
        </button>
      </div>
    </header>

    <LogTelemetryStrip
      :lines-streamed="linesStreamed"
      :error-rate="errorRate"
      :buffer-size="totalBufferCount"
      :max-buffer-size="maxBufferSize"
      :latency="latency"
      :is-connected="mode === 'live' ? isConnected : false"
      :is-paused="mode === 'live' ? isPaused : false"
    />

    <div class="mode-controls-bar">
      <div class="mode-switcher-tabs font-mono" role="tablist">
        <button type="button" class="mode-tab-btn" :class="{ active: mode === 'live' }" @click="mode = 'live'">
          <BaseIcon name="zap" size="xs" />
          <span>Live Tail</span>
        </button>
        <button type="button" class="mode-tab-btn" :class="{ active: mode === 'historical' }" @click="mode = 'historical'">
          <BaseIcon name="search" size="xs" />
          <span>Historical Search</span>
        </button>
      </div>

      <button
        v-if="mode === 'live'"
        type="button"
        class="live-tail-toggle-btn font-mono"
        :class="{ 'tail-active': autoScroll && !isScrollLocked, 'tail-paused': !autoScroll || isScrollLocked }"
        :title="autoScroll && !isScrollLocked ? 'Live Tail active. Click to lock.' : 'Tail paused on scroll up. Click to resume.'"
        @click="toggleLiveTail"
      >
        <span class="tail-dot" />
        <span>{{ autoScroll && !isScrollLocked ? 'LIVE TAIL ON' : 'TAIL PAUSED (SCROLLED UP)' }}</span>
      </button>

      <div v-if="mode === 'historical'" class="historical-query-group font-mono">
        <div class="time-range-picker">
          <button
            v-for="r in timeRanges"
            :key="r.label"
            type="button"
            class="range-pill-btn"
            :class="{ active: selectedTimeRange === r.label }"
            @click="selectedTimeRange = r.label; runHistoricalQuery()"
          >
            {{ r.label }}
          </button>
        </div>
        <button type="button" class="historical-search-btn" :disabled="logStore.isHistoricalLoading" @click="runHistoricalQuery()">
          <BaseIcon name="search" size="xs" />
          <span>{{ logStore.isHistoricalLoading ? 'Searching...' : 'Query ClickHouse' }}</span>
        </button>
        <button
          v-if="logStore.hasMoreHistorical || logStore.totalHistoricalCount > logStore.logs.length"
          type="button"
          class="load-more-btn"
          :disabled="logStore.isHistoricalLoading"
          @click="loadMoreHistorical"
        >
          <span>Load More ({{ logStore.logs.length }}/{{ logStore.totalHistoricalCount }})</span>
        </button>
        <span v-if="logStore.totalHistoricalCount > 0" class="query-count-badge">
          {{ logStore.totalHistoricalCount }} logs found
        </span>
      </div>
    </div>

    <div v-if="queryError" class="query-error-banner font-mono">
      <BaseIcon name="alert-triangle" size="xs" />
      <span>{{ queryError }}</span>
    </div>

    <!-- 80px Interactive Stacked SVG Bar Chart Mounted Above Terminal Stream -->
    <LogVolumeHistogram
      :buckets="logStore.histogram"
      :height="80"
      @filter-range="handleHistogramFilterRange"
      @clear-filter="handleClearHistogramFilter"
    />

    <div v-if="showMobileTree" class="mobile-backdrop" aria-hidden="true" @click="showMobileTree = false" />

    <div class="log-explorer-grid">
      <div class="explorer-left-col" :class="{ 'mobile-tree-open': showMobileTree }">
        <div v-if="showMobileTree" class="mobile-drawer-header">
          <span class="drawer-title font-mono">
            <BaseIcon name="layers" size="xs" />
            <span>Select Target</span>
          </span>
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
          :search-query="searchKeyword"
          :selected-level="selectedLevel"
          :target-name="selectedTarget.name"
          :latency="latency"
          @update:search-query="searchKeyword = $event"
          @update:selected-level="selectedLevel = $event"
          @update:auto-scroll="autoScroll = $event"
          @toggle-pause="togglePause"
          @clear-buffer="clearBuffer"
          @export-logs="handleExport"
          @scroll="handleScroll"
          @scroll-to-bottom="scrollToBottom"
          @register-terminal="setTerminalRef"
          @toggle-target-tree="showMobileTree = !showMobileTree"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../assets/styles/views/logstream.css';

.mode-controls-bar { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 10px; padding: 8px 12px; background: rgba(15, 23, 42, 0.75); border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; }
.mode-switcher-tabs { display: inline-flex; background: rgba(2, 6, 23, 0.6); padding: 3px; border-radius: 6px; border: 1px solid rgba(255, 255, 255, 0.08); gap: 4px; }
.mode-tab-btn { display: inline-flex; align-items: center; gap: 6px; background: transparent; border: none; color: #94a3b8; font-size: 11px; font-weight: 600; padding: 5px 12px; border-radius: 4px; cursor: pointer; transition: all 0.15s ease; }
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
.historical-search-btn, .load-more-btn { display: inline-flex; align-items: center; gap: 5px; background: rgba(56, 189, 248, 0.15); border: 1px solid rgba(56, 189, 248, 0.35); color: #38bdf8; font-size: 11px; font-weight: 600; padding: 5px 12px; border-radius: 6px; cursor: pointer; }
.historical-search-btn:hover:not(:disabled), .load-more-btn:hover:not(:disabled) { background: rgba(56, 189, 248, 0.28); }
.query-count-badge { font-size: 11px; color: #94a3b8; padding: 2px 6px; border-radius: 4px; background: rgba(255, 255, 255, 0.04); }
.query-error-banner { display: flex; align-items: center; gap: 8px; padding: 6px 12px; background: rgba(244, 63, 94, 0.15); border: 1px solid rgba(244, 63, 94, 0.35); color: #f43f5e; border-radius: 6px; font-size: 11px; }
@keyframes pulse-dot { 0%, 100% { opacity: 1; transform: scale(1); } 50% { opacity: 0.4; transform: scale(0.85); } }
@media (max-width: 640px) { .mode-controls-bar { padding: 6px 8px; gap: 6px; } }
</style>
