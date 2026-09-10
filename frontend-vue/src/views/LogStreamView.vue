<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useLogStreamer } from '../composables/useLogStreamer'
import LogTargetTree, { type LogTarget } from '../components/logs/LogTargetTree.vue'
import LogTelemetryStrip from '../components/logs/LogTelemetryStrip.vue'
import LogViewerTerminal from '../components/logs/LogViewerTerminal.vue'

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

watch(selectedTarget, (target) => {
  if (target.type === 'node') {
    logStore.connect({ node: target.id })
  } else if (target.type === 'service') {
    logStore.connect({ service: target.id })
  } else {
    logStore.connect()
  }
})

const targetFilteredLogs = computed(() => {
  const target = selectedTarget.value
  const level = selectedLevel.value
  const rawKw = searchKeyword.value.trim()

  // Real safe regex compilation with graceful fallback
  let reg: RegExp | null = null
  if (rawKw) {
    try {
      reg = new RegExp(rawKw, 'i')
    } catch {
      reg = null
    }
  }
  const kw = rawKw.toLowerCase()

  return logStore.logs.filter((log) => {
    // 1. Level filter
    if (level && log.level.toUpperCase() !== level.toUpperCase()) return false

    // 2. Target drill-down filter (Node / Service / All)
    if (target.type === 'node') {
      const q = target.id.toLowerCase()
      const matchNode = log.node && log.node.toLowerCase().includes(q)
      const matchPod = log.pod && log.pod.toLowerCase().includes(q)
      const matchNs = log.namespace && log.namespace.toLowerCase().includes(q)
      if (!matchNode && !matchPod && !matchNs) return false
    } else if (target.type === 'service') {
      const q = target.id.toLowerCase()
      const matchSvc = log.service && log.service.toLowerCase().includes(q)
      const matchCtr = log.container && log.container.toLowerCase().includes(q)
      const matchPod = log.pod && log.pod.toLowerCase().includes(q)
      const matchNs = log.namespace && log.namespace.toLowerCase().includes(q)
      if (!matchSvc && !matchCtr && !matchPod && !matchNs) return false
    }

    // 3. Search keyword or real regex
    if (rawKw) {
      if (reg) {
        const matchMsg = reg.test(log.msg)
        const matchPod = reg.test(log.pod)
        const matchTrace = log.traceId ? reg.test(log.traceId) : false
        const matchSvc = log.service ? reg.test(log.service) : false
        if (!matchMsg && !matchPod && !matchTrace && !matchSvc) return false
      } else {
        const matchMsg = log.msg.toLowerCase().includes(kw)
        const matchPod = log.pod.toLowerCase().includes(kw)
        const matchTrace = log.traceId?.toLowerCase().includes(kw)
        const matchSvc = log.service?.toLowerCase().includes(kw)
        if (!matchMsg && !matchPod && !matchTrace && !matchSvc) return false
      }
    }

    return true
  })
})

function handleExport() {
  const logsToExport = targetFilteredLogs.value.length > 0 ? targetFilteredLogs.value : logStore.logs
  const content = logsToExport
    .map((l) => {
      const node = l.node || (l.namespace !== 'default' ? l.namespace : 'node')
      const service = l.service || l.container || l.pod || 'system'
      return `[${l.time}] [${l.level.padEnd(5)}] [${node}/${service}]: ${l.msg}${l.traceId ? ` [trace=${l.traceId}]` : ''}`
    })
    .join('\n')

  if (typeof window !== 'undefined') {
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
    a.href = url
    a.download = `k8s-logs-${selectedTarget.value.id}-${timestamp}.log`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }
}
</script>

<template>
  <div class="view-container log-explorer-page">
    <!-- Compact View Header -->
    <header class="view-header">
      <div class="header-left">
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>REAL-TIME OBSERVABILITY & LOG EXPLORER</span>
        </div>
        <h1 class="view-title">Enterprise Kubernetes Logs Explorer</h1>
      </div>

      <!-- Mobile Target Drawer Toggle -->
      <button
        type="button"
        class="mobile-tree-toggle-btn"
        aria-label="Toggle log targets drawer"
        @click="showMobileTree = !showMobileTree"
      >
        <span>🌲 {{ selectedTarget.name }} ▾</span>
      </button>
    </header>

    <!-- Compact 32px Telemetry Strip -->
    <LogTelemetryStrip
      :lines-streamed="linesStreamed"
      :error-rate="errorRate"
      :buffer-size="totalBufferCount"
      :max-buffer-size="maxBufferSize"
      :latency="latency"
      :is-connected="isConnected"
      :is-paused="isPaused"
    />

    <!-- Mobile Backdrop Overlay -->
    <div
      v-if="showMobileTree"
      class="mobile-backdrop"
      aria-hidden="true"
      @click="showMobileTree = false"
    ></div>

    <!-- 2-Column Explorer Grid (Datadog/Loki Style) -->
    <div class="log-explorer-grid">
      <!-- Left Column: Log Target Drill-Down Tree -->
      <div class="explorer-left-col" :class="{ 'mobile-tree-open': showMobileTree }">
        <div v-if="showMobileTree" class="mobile-drawer-header">
          <span class="drawer-title font-mono">🌲 Select Target</span>
          <button type="button" class="drawer-close-btn" aria-label="Close targets drawer" @click="showMobileTree = false">✕</button>
        </div>
        <LogTargetTree
          v-model="selectedTarget"
          :logs="logStore.logs"
          @select="showMobileTree = false"
        />
      </div>

      <!-- Right Column: Clean Modern Terminal Window -->
      <div class="explorer-right-col">
        <LogViewerTerminal
          :logs="targetFilteredLogs"
          :is-connected="isConnected"
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
</style>
