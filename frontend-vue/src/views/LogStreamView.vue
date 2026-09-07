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
  const kw = searchKeyword.value.trim().toLowerCase()

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

    // 3. Search keyword or regex
    if (kw) {
      const matchMsg = log.msg.toLowerCase().includes(kw)
      const matchPod = log.pod.toLowerCase().includes(kw)
      const matchTrace = log.traceId?.toLowerCase().includes(kw)
      if (!matchMsg && !matchPod && !matchTrace) return false
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
.log-explorer-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: calc(100vh - 100px);
  min-height: 600px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.header-left {
  display: flex;
  flex-direction: column;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 10.5px;
  font-weight: 700;
  color: #38bdf8;
  letter-spacing: 0.05em;
  font-family: var(--font-mono, monospace);
  margin-bottom: 2px;
}

.view-title {
  font-size: 20px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
  margin: 0;
}

.mobile-tree-toggle-btn {
  display: none;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 12px;
  background: rgba(15, 23, 42, 0.85);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 6px;
  color: #38bdf8;
  font-size: 11px;
  font-weight: 700;
  font-family: var(--font-mono, monospace);
  cursor: pointer;
  transition: all 0.15s ease;
}

.mobile-tree-toggle-btn:hover {
  background: rgba(56, 189, 248, 0.15);
  border-color: rgba(56, 189, 248, 0.6);
}

/* 2-Column Explorer Grid */
.log-explorer-grid {
  flex: 1;
  display: grid;
  grid-template-columns: 260px 1fr;
  gap: 12px;
  min-height: 0;
  overflow: hidden;
}

.explorer-left-col {
  height: 100%;
  overflow-y: auto;
}

.explorer-right-col {
  height: 100%;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}

.mobile-backdrop {
  display: none;
}
.mobile-drawer-header {
  display: none;
}

@media (max-width: 900px) {
  .mobile-tree-toggle-btn {
    display: inline-flex;
  }

  .mobile-backdrop {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.75);
    backdrop-filter: blur(4px);
    z-index: 45;
  }

  .mobile-drawer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    background: rgba(15, 23, 42, 0.98);
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  }

  .drawer-title {
    font-size: 11px;
    font-weight: 700;
    color: #38bdf8;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .drawer-close-btn {
    background: none;
    border: none;
    color: #94a3b8;
    font-size: 13px;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
  }

  .drawer-close-btn:hover {
    color: #fff;
    background: rgba(255, 255, 255, 0.1);
  }

  .log-explorer-grid {
    grid-template-columns: 1fr;
    position: relative;
  }

  .explorer-left-col {
    display: none;
    position: fixed;
    top: 50px;
    left: 12px;
    right: 12px;
    bottom: 20px;
    max-width: 380px;
    z-index: 50;
    background: #0b0f19;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 8px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.9);
    flex-direction: column;
  }

  .explorer-left-col.mobile-tree-open {
    display: flex;
  }
}

@media (max-width: 640px) {
  .log-explorer-page {
    height: 100%;
    min-height: 0;
    padding: 0;
    gap: 0;
    flex: 1;
    overflow: hidden;
  }

  .view-header {
    display: none !important;
  }

  .log-explorer-grid {
    gap: 0;
    height: calc(100% - 20px);
    min-height: 0;
    flex: 1;
  }

  .explorer-left-col {
    top: 8px;
    left: 8px;
    right: 8px;
    bottom: 8px;
    max-width: none;
    border-radius: 8px;
    z-index: 50;
  }
}
</style>
