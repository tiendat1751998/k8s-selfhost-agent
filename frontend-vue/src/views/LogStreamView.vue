<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useLogStreamer } from '../composables/useLogStreamer'
import { useHistoricalLogSearch } from '../composables/useHistoricalLogSearch'
import LogTargetTree, { type LogTarget } from '../components/logs/LogTargetTree.vue'
import LogViewerTerminal from '../components/logs/LogViewerTerminal.vue'
import LogVolumeHistogram from '../components/logs/LogVolumeHistogram.vue'
import ClickHouseEngineBadge from '../components/logs/ClickHouseEngineBadge.vue'
import LogTraceDrawer from '../components/logs/LogTraceDrawer.vue'
import LogContextModal from '../components/logs/LogContextModal.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import type { LogFilterParams } from '../api/logging'

const {
  logStore, searchKeyword, selectedLevel, autoScroll, isScrollLocked, linesStreamed,
  latency, errorRate, isConnected, isPaused,
  clearBuffer, scrollToBottom, handleScroll, setTerminalRef,
} = useLogStreamer({ autoConnect: false })

const route = useRoute()
const selectedTarget = ref<LogTarget>({ type: 'service', id: 'postgres_db', name: 'postgres_db', icon: 'database' })
const showMobileTree = ref(false)
const isSidebarCollapsed = ref(false)
const showHistogram = ref(false)
const wrapLines = ref(true)
const showDroppedAlert = ref(true)

const {
  mode, selectedTimeRange, queryError, selectedHistoricalLimit, timeRanges,
  runHistoricalQuery, loadMoreHistorical, handleHistogramFilterRange, handleClearHistogramFilter,
  targetFilteredLogs,
} = useHistoricalLogSearch(logStore, selectedTarget, searchKeyword, selectedLevel)

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
  if (mode.value === 'historical') {
    runHistoricalQuery()
  }
}

let activeTargetId = ''

async function preloadRecentLogs(target: LogTarget) {
  if (!target || !target.id) return
  try {
    const kw = searchKeyword.value.trim()
    const filter: LogFilterParams & { service?: string; container?: string } = {
      limit: 50,
      query: kw ? kw : undefined,
      service: target.type === 'service' ? target.id : undefined,
      container: target.type === 'service' ? target.id : undefined,
      container_name: target.type === 'service' ? target.id : undefined,
      node: target.type === 'node' ? target.id : undefined,
      attributes: target.type === 'node' ? { node: target.id } : undefined,
      log_level: (selectedLevel.value && selectedLevel.value !== 'ALL') ? selectedLevel.value : undefined,
    }
    await logStore.fetchHistoricalLogs(filter, false)
    logStore.logs = logStore.logs.filter((l) => l.msg !== '-- No entries --' && l.msg.trim() !== '-- No entries --')
  } catch {
    // Gracefully ignore if offline or no historical logs
  }
}

function connectTarget(target: LogTarget) {
  if (!target || !target.id) return
  if (target.type === 'node') {
    logStore.connect({ node: target.id })
  } else {
    logStore.connect({ service: target.id, container: target.id })
  }
}

async function handleSelectTarget(target: LogTarget) {
  showMobileTree.value = false
  selectedTarget.value = target
  activeTargetId = target.id
  if (mode.value === 'live') {
    connectTarget(target)
    await preloadRecentLogs(target)
    fetchLiveHistogram()
  } else {
    runHistoricalQuery()
  }
}

watch(selectedTarget, (t) => {
  if (t.id === activeTargetId) return
  activeTargetId = t.id
  if (mode.value === 'live') {
    connectTarget(t)
    preloadRecentLogs(t)
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
    preloadRecentLogs(selectedTarget.value)
    fetchLiveHistogram()
  }
})

watch(
  () => route.query.node,
  (newNode) => {
    if (newNode) {
      selectedTarget.value = {
        type: 'node',
        id: String(newNode),
        name: String(newNode),
        icon: 'server',
      }
      if (route.query.search) {
        searchKeyword.value = String(route.query.search)
      }
    }
  }
)

async function fetchLiveHistogram() {
  const now = new Date()
  const target = selectedTarget.value
  await logStore.fetchHistogram({
    start_time: new Date(now.getTime() - 3600000).toISOString(),
    end_time: now.toISOString(),
    interval_seconds: 60,
    container_name: target.type === 'service' ? target.id : undefined,
    node: target.type === 'node' ? target.id : undefined,
    attributes: target.type === 'node' ? { node: target.id } : undefined,
  }).catch(() => {})
}

onMounted(() => {
  if (route.query.node) {
    selectedTarget.value = {
      type: 'node',
      id: String(route.query.node),
      name: String(route.query.node),
      icon: 'server',
    }
    if (route.query.search) {
      searchKeyword.value = String(route.query.search)
    }
  }

  if (mode.value === 'live' && selectedTarget.value.id) {
    activeTargetId = selectedTarget.value.id
    connectTarget(selectedTarget.value)
    preloadRecentLogs(selectedTarget.value)
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

watch(
  [() => targetFilteredLogs.value.length, () => targetFilteredLogs.value[targetFilteredLogs.value.length - 1]],
  async () => {
    if (mode.value === 'live' && autoScroll.value && !isScrollLocked.value) {
      await scrollToBottom()
    }
  }
)

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
        <span class="toolbar-target-badge font-mono" :title="selectedTarget.name"><BaseIcon :name="selectedTarget.type === 'node' ? 'server' : (selectedTarget.icon || 'box')" size="xs" /> <span>{{ selectedTarget.name }}</span></span>
        <div class="toolbar-search-wrap">
          <BaseIcon name="search" size="xs" class="search-icon" />
          <input
            v-model="searchKeyword"
            type="text"
            :placeholder="mode === 'historical' ? 'ClickHouse search (query=...)' : 'Filter logs (regex)...'"
            class="toolbar-search-input font-mono"
            aria-label="Filter logs"
            @keyup.enter="mode === 'historical' ? runHistoricalQuery() : null"
          />
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
            <option :value="50000">All (Keyset)</option>
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

    <!-- Congested Buffer Warning Alert -->
    <div
      v-if="logStore.droppedLogsCount > 0 && showDroppedAlert"
      class="dropped-alert-banner font-mono"
      role="alert"
    >
      <BaseIcon name="alert-triangle" size="xs" class="text-amber" />
      <span>
        Stream buffer congested: {{ logStore.droppedLogsCount.toLocaleString() }} logs dropped by edge network. Switch to Historical Search for full ClickHouse archive.
      </span>
      <button
        type="button"
        class="dropped-alert-action font-mono"
        @click="mode = 'historical'; logStore.resetDroppedLogsCount()"
      >
        Switch to Historical
      </button>
      <button
        type="button"
        class="query-error-dismiss"
        aria-label="Dismiss dropped alert"
        @click="showDroppedAlert = false"
      >
        <BaseIcon name="x" size="xs" />
      </button>
    </div>

    <!-- Mobile Command Bar (<768px) -->
    <div class="logs-mobile-command-bar mobile-only">
      <div class="mobile-bar-left">
        <button type="button" class="mobile-tree-toggle-btn font-mono" aria-label="Toggle log targets drawer" @click="showMobileTree = !showMobileTree"><BaseIcon :name="selectedTarget.type === 'node' ? 'server' : (selectedTarget.icon || 'box')" size="xs" /> <span>{{ selectedTarget.name }}</span><BaseIcon name="chevron-down" size="xs" /></button>
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
        <LogTargetTree v-model="selectedTarget" :logs="logStore.logs" @select="handleSelectTarget" />
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
          @open-trace="logStore.openTraceDrawer"
          @open-context="logStore.openContextModal"
        />
      </div>
    </div>

    <!-- Transaction Trace Waterfall Drawer -->
    <LogTraceDrawer />

    <!-- Surrounding Context Modal -->
    <LogContextModal />
  </div>
</template>

<style scoped>
@import '../assets/styles/views/logstream.css';
</style>