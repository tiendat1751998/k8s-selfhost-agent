<script setup lang="ts">
import { ref } from 'vue'
import { useNodeLogTerminal, type LogTerminalProps } from './useNodeLogTerminal'
import { LINE_HEIGHT } from './nodeLogUtils'
import NodeLogIncidentsTable from './NodeLogIncidentsTable.vue'
import NodeLogCustomRangeBar from './NodeLogCustomRangeBar.vue'

const props = withDefaults(defineProps<LogTerminalProps>(), {
  nodeHistoryData: null,
  tpsData: null,
})

const terminalBodyRef = ref<HTMLElement | null>(null)

const {
  selectedLogApp,
  selectedLogTail,
  selectedLogSince,
  selectedLogLevel,
  appLogSearch,
  customLogFrom,
  customLogTo,
  showCustomDatePicker,
  wrapLogLines,
  isLogStreaming,
  autoScrollLogs,
  userScrolledUp,
  appLogsLoading,
  appLogsError,
  appLogsCopied,
  nodeAvailableContainerApps,
  nodeAvailableHostApps,
  appLogLevelCounts,
  parsedAppLogLines,
  totalVirtualHeight,
  visibleLogLines,
  onLogSinceChange,
  applyLogPreset,
  fetchNodeAppLogs,
  toggleLogStreaming,
  handleTerminalScroll,
  scrollToBottom,
  clearLogTerminal,
  copyAppLogs,
  downloadAppLogs,
  selectIncidentLog,
  syncToTime,
} = useNodeLogTerminal(props, terminalBodyRef)

defineExpose({
  syncToTime,
  fetchNodeAppLogs,
  clearLogTerminal,
  copyAppLogs,
  downloadAppLogs,
  wrapLogLines,
})
</script>

<template>
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
    <NodeLogCustomRangeBar
      v-if="selectedLogSince === 'custom' || showCustomDatePicker"
      v-model:from="customLogFrom"
      v-model:to="customLogTo"
      :loading="appLogsLoading"
      @apply-preset="applyLogPreset"
      @apply="fetchNodeAppLogs(selectedLogApp)"
    />

    <!-- Terminal Real Log Viewer -->
    <div class="log-terminal-viewer">
      <div class="terminal-topbar">
        <span class="terminal-title font-mono">
          stdout/stderr :: {{ selectedLogApp }} @ {{ node?.node_name }}
        </span>
        <div class="terminal-stats font-mono text-slate">
          {{ parsedAppLogLines.length }} lines · {{ appLogLevelCounts.error }} err · {{ appLogLevelCounts.warn }} warn
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
    <NodeLogIncidentsTable
      v-if="nodeHistoryData?.incidents && nodeHistoryData.incidents.length > 0"
      :incidents="nodeHistoryData.incidents"
      @select-incident="selectIncidentLog"
    />
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/node-log-terminal.css';
</style>
