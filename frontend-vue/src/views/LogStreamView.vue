<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useLogStreamer } from '../composables/useLogStreamer'
import { useHistoricalLogSearch } from '../composables/useHistoricalLogSearch'
import { useGlobalContext } from '../composables/useGlobalContext'
import LogTargetTree, { type LogTarget } from '../components/logs/LogTargetTree.vue'
import LogViewerTerminal from '../components/logs/LogViewerTerminal.vue'
import LogVolumeHistogram from '../components/logs/LogVolumeHistogram.vue'
import LogTraceDrawer from '../components/logs/LogTraceDrawer.vue'
import LogContextModal from '../components/logs/LogContextModal.vue'
import LogStreamToolbar from '../components/logs/LogStreamToolbar.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import type { LogFilterParams } from '../api/logging'
import '../assets/styles/views/logstream.css'

const { activeClusterId } = useGlobalContext()

const {
  logStore, searchKeyword, selectedLevel, autoScroll, isScrollLocked,
  latency, isConnected, isPaused,
  clearBuffer, scrollToBottom, handleScroll, setTerminalRef,
} = useLogStreamer({ autoConnect: false })

const route = useRoute()
const selectedTarget = ref<LogTarget>({ type: 'service', id: '', name: 'Select Target', icon: 'layers' })
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

let activeTargetId = ''

async function preloadRecentLogs(target: LogTarget) {
  if (!target || !target.id) return
  try {
    const kw = searchKeyword.value.trim()
    const ns = target.namespace || (route.query.namespace ? String(route.query.namespace) : undefined)
    const filter: LogFilterParams & { service?: string; container?: string; namespace?: string } = {
      limit: 50,
      query: kw ? kw : undefined,
      service: target.type === 'service' ? target.id : undefined,
      container: target.type === 'service' ? target.id : undefined,
      container_name: target.type === 'service' ? target.id : undefined,
      namespace: ns,
      node: target.type === 'node' ? target.id : undefined,
      attributes: target.type === 'node' ? { node: target.id } : undefined,
      cluster_id: activeClusterId.value || undefined,
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
    const ns = target.namespace || (route.query.namespace ? String(route.query.namespace) : undefined)
    logStore.connect({ service: target.id, container: target.id, namespace: ns })
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

function syncTargetFromRoute(): boolean {
  const node = route.query.node
  const app = route.query.app
  const pod = route.query.pod
  const ns = route.query.namespace
  const search = route.query.search

  if (search) {
    searchKeyword.value = String(search)
  }

  if (pod || app) {
    const targetId = String(pod || app)
    const displayName = pod && app && pod !== app ? `${app} (${pod})` : targetId
    selectedTarget.value = {
      type: 'service',
      id: targetId,
      name: displayName,
      icon: 'box',
      namespace: ns ? String(ns) : undefined,
    }
    return true
  } else if (node) {
    selectedTarget.value = {
      type: 'node',
      id: String(node),
      name: String(node),
      icon: 'server',
    }
    return true
  }
  return false
}

watch(
  () => [route.query.node, route.query.app, route.query.pod, route.query.namespace, route.query.search],
  () => {
    if (syncTargetFromRoute()) {
      if (selectedTarget.value.id && selectedTarget.value.id !== activeTargetId) {
        activeTargetId = selectedTarget.value.id
        if (mode.value === 'live') {
          connectTarget(selectedTarget.value)
          preloadRecentLogs(selectedTarget.value)
          fetchLiveHistogram()
        } else {
          runHistoricalQuery()
        }
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
  syncTargetFromRoute()

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
    <!-- Unified 38px Enterprise Toolbar & Mobile Command Bar -->
    <LogStreamToolbar
      v-model:search-keyword="searchKeyword"
      v-model:mode="mode"
      v-model:selected-level="selectedLevel"
      v-model:selected-time-range="selectedTimeRange"
      v-model:selected-historical-limit="selectedHistoricalLimit"
      v-model:is-sidebar-collapsed="isSidebarCollapsed"
      v-model:wrap-lines="wrapLines"
      v-model:show-histogram="showHistogram"
      v-model:show-mobile-tree="showMobileTree"
      :selected-target="selectedTarget"
      :time-ranges="timeRanges"
      :is-historical-loading="logStore.isHistoricalLoading"
      :has-more-historical="logStore.hasMoreHistorical"
      :total-historical-count="logStore.totalHistoricalCount"
      :current-logs-count="logStore.logs.length"
      :auto-scroll="autoScroll"
      :is-scroll-locked="isScrollLocked"
      @run-historical-query="runHistoricalQuery"
      @load-more-historical="loadMoreHistorical"
      @toggle-live-tail="toggleLiveTail"
      @clear-buffer="clearBuffer"
      @export-logs="handleExport"
    />

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
        <LogTargetTree v-model="selectedTarget" :logs="logStore.logs" :cluster="activeClusterId" @select="handleSelectTarget" />
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
:deep(.virtual-fps-badge) {
  white-space: nowrap !important;
  flex-shrink: 0 !important;
}
</style>