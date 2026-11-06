<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useLogStreamer } from '../composables/useLogStreamer'
import { useHistoricalLogSearch } from '../composables/useHistoricalLogSearch'
import { useGlobalContext } from '../composables/useGlobalContext'
import { useInfraHosts } from '../composables/useInfraHosts'
import { useLogTargets, type LogTarget } from '../composables/useLogTargets'
import LogViewerTerminal from '../components/logs/LogViewerTerminal.vue'
import LogVolumeHistogram from '../components/logs/LogVolumeHistogram.vue'
import LogTraceDrawer from '../components/logs/LogTraceDrawer.vue'
import LogContextModal from '../components/logs/LogContextModal.vue'
import LogStreamToolbar from '../components/logs/LogStreamToolbar.vue'
import LogErrorRadarPanel from '../components/logs/LogErrorRadarPanel.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import type { LogFilterParams, ErrorRadarGroup } from '../api/logging'
import { exportLogsToTextFile } from '../components/logs/virtualLogUtils'
import '../assets/styles/views/logstream.css'

const { activeClusterId } = useGlobalContext()
const { hosts } = useInfraHosts()

const {
  logStore, searchKeyword, selectedLevel, autoScroll, isScrollLocked,
  latency, isConnected, isPaused,
  clearBuffer, scrollToBottom, handleScroll, setTerminalRef,
} = useLogStreamer({ autoConnect: false })

const route = useRoute()
const selectedTarget = ref<LogTarget>({ type: 'all', id: 'all', name: 'All Nodes', icon: 'globe' })
const showHistogram = ref(false)
const wrapLines = ref(false)
const showDroppedAlert = ref(true)
const showErrorRadar = ref(false)

const {
  dynamicHostNodes,
  getDiscoveredServicesForNode,
  canViewJournalctl,
} = useLogTargets({ modelValue: selectedTarget.value, logs: logStore.logs, cluster: activeClusterId.value }, (_event, target) => {
  selectedTarget.value = target
})

const isSelectedNodeOffline = computed<boolean>(() => isTargetOffline.value)

const {
  mode, selectedTimeRange, queryError, selectedHistoricalLimit, timeRanges,
  activeServiceSubFilter, mutedServices, hoverPauseEnabled, isHoverPaused,
  effectiveNode, effectiveService,
  runHistoricalQuery, loadMoreHistorical, handleHistogramFilterRange, handleClearHistogramFilter,
  targetFilteredLogs,
} = useHistoricalLogSearch(logStore, selectedTarget, searchKeyword, selectedLevel)

let activeTargetId = ''

const selectedNode = computed<string>({
  get: () => {
    if (selectedTarget.value.type === 'node') {
      return selectedTarget.value.id || selectedTarget.value.node || ''
    }
    return ''
  },
  set: (nodeId: string) => {
    handleSelectNode(nodeId)
  },
})

function handleSelectNode(nodeId: string) {
  if (!nodeId || nodeId === 'all') {
    selectedTarget.value = { type: 'all', id: 'all', name: 'All Nodes', icon: 'globe' }
  } else {
    const match = dynamicHostNodes.value.find((n) => n.id.toLowerCase() === nodeId.toLowerCase())
    selectedTarget.value = {
      type: 'node',
      id: nodeId,
      name: match ? match.name : nodeId,
      icon: 'server',
      node: nodeId,
      online: match?.online !== false,
    }
  }
}

const availableServiceNames = computed(() => getDiscoveredServicesForNode(selectedNode.value))

const isTargetOffline = computed<boolean>(() => {
  if (selectedTarget.value.type !== 'node') return false
  if (selectedTarget.value.online === false || selectedTarget.value.status === 'offline') return true
  const nodeName = (selectedTarget.value.node || selectedTarget.value.id || selectedTarget.value.name).toLowerCase()
  const match = hosts.value.find((h) => {
    const name = (h.name || h.id || h.endpoint).toLowerCase()
    return name === nodeName || name.includes(nodeName) || nodeName.includes(name)
  })
  if (match) {
    const isOnline = match.status === 'connected' || match.status === 'online' || match.status === 'healthy'
    return !isOnline
  }
  return false
})

function handleSwitchToHistoryFromOffline() {
  mode.value = 'historical'
  selectedTimeRange.value = '24h'
  runHistoricalQuery()
}

async function preloadRecentLogs(target: LogTarget) {
  if (!target || !target.id) return
  try {
    const kw = searchKeyword.value.trim()
    const ns = target.namespace || (route.query.namespace ? String(route.query.namespace) : undefined)
    const effNode = effectiveNode.value
    const effService = effectiveService.value
    const filter: LogFilterParams = {
      limit: 50, query: kw || undefined, service: effService, container: effService, container_name: effService,
      namespace: ns, node: effNode, attributes: effNode ? { node: effNode } : undefined,
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
  const ns = target.namespace || (route.query.namespace ? String(route.query.namespace) : undefined)
  logStore.connect({ node: effectiveNode.value, service: effectiveService.value, container: effectiveService.value, namespace: ns })
}

function refreshTargetStream(target = selectedTarget.value, disconnectFirst = false) {
  if (mode.value === 'live') {
    connectTarget(target)
    preloadRecentLogs(target)
    fetchLiveHistogram()
  } else {
    if (disconnectFirst) logStore.disconnect()
    runHistoricalQuery()
  }
}

function handleToggleMuteService(serviceId: string) {
  const lower = serviceId.toLowerCase()
  const idx = mutedServices.value.indexOf(lower)
  if (idx >= 0) {
    mutedServices.value.splice(idx, 1)
  } else {
    mutedServices.value.push(lower)
    if (activeServiceSubFilter.value.toLowerCase() === lower) activeServiceSubFilter.value = ''
  }
}

function handleFilterServiceFromRow(payload: { node?: string; service: string }) {
  if (payload.node && payload.node !== selectedTarget.value.node && payload.node !== selectedTarget.value.id) {
    handleSelectNode(payload.node)
  }
  activeServiceSubFilter.value = payload.service
}

watch(activeServiceSubFilter, () => refreshTargetStream())

watch(selectedTarget, (t) => {
  if (t.id === activeTargetId) return
  activeTargetId = t.id
  refreshTargetStream(t)
})

watch(mode, (newMode) => refreshTargetStream(selectedTarget.value, newMode === 'historical'))

function syncTargetFromRoute(): boolean {
  const node = route.query.node ? String(route.query.node) : ''
  const app = route.query.app ? String(route.query.app) : ''
  const pod = route.query.pod ? String(route.query.pod) : ''
  const ns = route.query.namespace ? String(route.query.namespace) : undefined
  if (route.query.search) searchKeyword.value = String(route.query.search)
  const targetId = pod || app
  if (node && targetId) {
    selectedTarget.value = { type: 'node', id: node, name: `${node} / ${targetId}`, icon: 'server', node, service: targetId, namespace: ns }
    activeServiceSubFilter.value = targetId
    return true
  }
  if (targetId) {
    selectedTarget.value = { type: 'service', id: targetId, name: pod && app && pod !== app ? `${app} (${pod})` : targetId, icon: 'box', namespace: ns }
    activeServiceSubFilter.value = targetId
    return true
  }
  if (node) {
    handleSelectNode(node)
    return true
  }
  return false
}

watch(
  () => [route.query.node, route.query.app, route.query.pod, route.query.namespace, route.query.search],
  () => {
    if (syncTargetFromRoute() && selectedTarget.value.id && selectedTarget.value.id !== activeTargetId) {
      activeTargetId = selectedTarget.value.id
      refreshTargetStream()
    }
  }
)

async function fetchLiveHistogram() {
  const now = new Date()
  const effNode = effectiveNode.value
  const effService = effectiveService.value
  await logStore.fetchHistogram({
    start_time: new Date(now.getTime() - 3600000).toISOString(),
    end_time: now.toISOString(),
    interval_seconds: 60,
    service: effService,
    container_name: effService,
    node: effNode,
    attributes: effNode ? { node: effNode } : undefined,
  }).catch(() => {})
}

onMounted(() => {
  syncTargetFromRoute()
  if (mode.value === 'live' && selectedTarget.value.id) {
    activeTargetId = selectedTarget.value.id
    refreshTargetStream()
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

function handleInspectErrorGroup(group: ErrorRadarGroup, radarRange?: string) {
  showErrorRadar.value = false
  if (group.node) {
    handleSelectNode(group.node)
  }
  if (group.service) activeServiceSubFilter.value = group.service
  const r = (radarRange || '').toLowerCase()
  selectedTimeRange.value = (r === 'all' || !radarRange) ? '7d' : radarRange
  selectedLevel.value = 'ERROR'
  searchKeyword.value = ''
  if (mode.value === 'historical') {
    runHistoricalQuery()
  } else {
    mode.value = 'historical'
  }
}
</script>

<template>
  <div class="view-container log-explorer-page">
    <LogStreamToolbar
      v-model:search-keyword="searchKeyword" v-model:mode="mode" v-model:selected-level="selectedLevel"
      v-model:selected-time-range="selectedTimeRange" v-model:selected-historical-limit="selectedHistoricalLimit"
      v-model:wrap-lines="wrapLines" v-model:show-histogram="showHistogram" v-model:show-error-radar="showErrorRadar"
      v-model:selected-node="selectedNode" :nodes="dynamicHostNodes" :services="availableServiceNames"
      :is-node-offline="isTargetOffline" :show-journalctl="canViewJournalctl && !isSelectedNodeOffline"
      :selected-target="selectedTarget" :time-ranges="timeRanges"
      :is-historical-loading="logStore.isHistoricalLoading" :has-more-historical="logStore.hasMoreHistorical"
      :total-historical-count="logStore.totalHistoricalCount" :current-logs-count="logStore.logs.length"
      :auto-scroll="autoScroll" :is-scroll-locked="isScrollLocked" :active-service="activeServiceSubFilter"
      :hover-pause-enabled="hoverPauseEnabled" :is-hover-paused="isHoverPaused"
      @clear-active-service="activeServiceSubFilter = ''" @update:active-service="activeServiceSubFilter = $event"
      @update:hover-pause-enabled="hoverPauseEnabled = $event" @run-historical-query="runHistoricalQuery"
      @load-more-historical="loadMoreHistorical" @toggle-live-tail="toggleLiveTail" @clear-buffer="clearBuffer"
      @export-logs="exportLogsToTextFile(targetFilteredLogs.length > 0 ? targetFilteredLogs : logStore.logs, selectedTarget.name)"
    />

    <div v-if="logStore.droppedLogsCount > 0 && showDroppedAlert" class="dropped-alert-banner font-mono" role="alert">
      <BaseIcon name="alert-triangle" size="xs" class="text-amber" />
      <span>Stream buffer congested: {{ logStore.droppedLogsCount.toLocaleString() }} logs dropped. Switch to History.</span>
      <button type="button" class="dropped-alert-action font-mono" @click="mode = 'historical'; logStore.resetDroppedLogsCount()">Switch to Historical</button>
      <button type="button" class="query-error-dismiss" aria-label="Dismiss dropped alert" @click="showDroppedAlert = false"><BaseIcon name="x" size="xs" /></button>
    </div>

    <div v-if="queryError" class="query-error-banner font-mono">
      <BaseIcon name="alert-triangle" size="xs" />
      <span>{{ queryError }}</span>
      <button type="button" class="query-error-dismiss" aria-label="Dismiss error" @click="queryError = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <div v-if="showHistogram" class="histogram-wrapper">
      <LogVolumeHistogram :buckets="logStore.histogram" :height="70" @filter-range="handleHistogramFilterRange" @clear-filter="handleClearHistogramFilter" />
    </div>

    <div class="log-explorer-grid">
      <div class="explorer-right-col">
        <LogErrorRadarPanel
          v-if="showErrorRadar" :node="effectiveNode" :service="effectiveService"
          :selected-time-range="selectedTimeRange" @close="showErrorRadar = false"
          @inspect-group="handleInspectErrorGroup"
        />

        <LogViewerTerminal
          :logs="targetFilteredLogs" :is-connected="mode === 'live' ? isConnected : true" :is-paused="isPaused"
          :auto-scroll="autoScroll" :is-scroll-locked="isScrollLocked" :target-name="selectedTarget.name"
          :latency="latency" :wrap-lines="wrapLines" :hover-pause-enabled="hoverPauseEnabled"
          :hide-service-badge="Boolean(effectiveService)" :is-target-offline="isTargetOffline" :mode="mode"
          :show-journalctl="canViewJournalctl && !isSelectedNodeOffline" :active-service="activeServiceSubFilter"
          @switch-to-history="handleSwitchToHistoryFromOffline" @scroll="handleScroll" @scroll-to-bottom="scrollToBottom"
          @register-terminal="setTerminalRef" @open-trace="logStore.openTraceDrawer" @open-context="logStore.openContextModal"
          @filter-service="handleFilterServiceFromRow" @mute-service="handleToggleMuteService"
          @hover-change="isHoverPaused = $event && hoverPauseEnabled"
          @select-journalctl="activeServiceSubFilter = 'journalctl'"
        />
      </div>
    </div>

    <LogTraceDrawer />
    <LogContextModal />
  </div>
</template>

<style scoped>
:deep(.virtual-fps-badge) {
  white-space: nowrap !important;
  flex-shrink: 0 !important;
}
</style>