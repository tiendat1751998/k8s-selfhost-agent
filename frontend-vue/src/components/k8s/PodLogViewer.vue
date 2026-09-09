<script setup lang="ts">
import { ref } from 'vue'
import { usePodLogs, type PodLogProps } from '../../composables/usePodLogs'

const props = withDefaults(defineProps<PodLogProps>(), {
  namespace: 'default',
  container: '',
  containers: () => [],
})

const logContainerRef = ref<HTMLElement | null>(null)

const {
  rawLines,
  selectedContainer,
  tailLines,
  isFollowing,
  showPrevious,
  wrapLines,
  autoScroll,
  userScrolledUp,
  searchQuery,
  selectedLevel,
  isLoading,
  isConnected,
  streamError,
  logsCopied,
  levelCounts,
  filteredLogs,
  allParsedLogs,
  reloadLogs,
  toggleFollowing,
  handleScroll,
  scrollToBottom,
  clearLogs,
  copyLogs,
  downloadLogs,
} = usePodLogs(props, logContainerRef)
</script>

<template>
  <div class="pod-log-viewer-wrapper">
    <!-- Toolbar -->
    <div class="log-viewer-toolbar glass-panel">
      <!-- Top Row: Targets, Search, and Level Filters -->
      <div class="toolbar-top-row">
        <!-- Container Selector -->
        <div class="toolbar-item font-mono">
          <label class="toolbar-label">CONTAINER:</label>
          <select
            v-if="containers && containers.length > 1"
            v-model="selectedContainer"
            class="select-input font-mono"
          >
            <option v-for="c in containers" :key="c" :value="c">
              📦 {{ c }}
            </option>
          </select>
          <span v-else class="text-cyan font-bold font-mono">
            📦 {{ selectedContainer || 'default' }}
          </span>
        </div>

        <!-- Tail Lines -->
        <div class="toolbar-item font-mono">
          <label class="toolbar-label">TAIL:</label>
          <select v-model="tailLines" class="select-input font-mono">
            <option :value="50">50 lines</option>
            <option :value="100">100 lines</option>
            <option :value="500">500 lines</option>
            <option :value="1000">1,000 lines</option>
            <option :value="5000">5,000 lines</option>
          </select>
        </div>

        <!-- Previous Logs Toggle -->
        <div class="toolbar-item">
          <label class="checkbox-label font-mono" title="Show logs from previous terminated container instance">
            <input v-model="showPrevious" type="checkbox" class="cyber-checkbox" />
            <span>Previous (Crashed)</span>
          </label>
        </div>

        <!-- Log Level Filter Chips -->
        <div class="log-level-chips">
          <button
            type="button"
            class="chip-btn"
            :class="{ active: selectedLevel === 'all' }"
            @click="selectedLevel = 'all'"
          >
            All ({{ levelCounts.total }})
          </button>
          <button
            type="button"
            class="chip-btn chip-rose"
            :class="{ active: selectedLevel === 'error' }"
            @click="selectedLevel = 'error'"
          >
            🔴 Errors ({{ levelCounts.error }})
          </button>
          <button
            type="button"
            class="chip-btn chip-amber"
            :class="{ active: selectedLevel === 'warn' }"
            @click="selectedLevel = 'warn'"
          >
            🟡 Warnings ({{ levelCounts.warn }})
          </button>
          <button
            type="button"
            class="chip-btn chip-emerald"
            :class="{ active: selectedLevel === 'info' }"
            @click="selectedLevel = 'info'"
          >
            🟢 Info ({{ levelCounts.info }})
          </button>
        </div>

        <!-- Search Input -->
        <div class="search-box">
          <span class="search-icon">🔍</span>
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Filter logs / exceptions..."
            class="search-input font-mono"
          />
          <button v-if="searchQuery" class="clear-search-btn" @click="searchQuery = ''">✕</button>
        </div>
      </div>

      <!-- Bottom Row: Stream Actions & View Controls -->
      <div class="toolbar-bottom-row">
        <!-- Live Stream & Controls -->
        <div class="action-btn-group">
          <button
            type="button"
            class="btn-log-action btn-stream"
            :class="{ 'stream-active': isFollowing && isConnected }"
            @click="toggleFollowing"
            :title="isFollowing ? 'Pause SSE live stream' : 'Resume SSE live stream'"
          >
            <span v-if="isFollowing && isConnected" class="live-dot"></span>
            <span v-else>⏸️</span>
            <span class="font-mono">{{ isFollowing && isConnected ? 'LIVE STREAMING' : isFollowing ? 'CONNECTING...' : 'PAUSED' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            :class="{ 'active-toggle': autoScroll }"
            @click="autoScroll = !autoScroll"
            title="Auto-scroll to latest lines"
          >
            <span>⬇️</span>
            <span>Auto-Scroll: {{ autoScroll ? 'ON' : 'OFF' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            :class="{ 'active-toggle': wrapLines }"
            @click="wrapLines = !wrapLines"
            title="Toggle word wrap for long log lines"
          >
            <span>↩️</span>
            <span>Wrap Lines: {{ wrapLines ? 'ON' : 'OFF' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            :disabled="isLoading"
            @click="reloadLogs"
            title="Reload Pod Logs"
          >
            <span :class="{ 'spin-icon': isLoading }">🔄</span>
            <span>{{ isLoading ? 'Loading...' : 'Refresh' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            @click="copyLogs"
            title="Copy log text to clipboard"
          >
            <span>📋</span>
            <span>{{ logsCopied ? 'Copied!' : 'Copy' }}</span>
          </button>

          <button
            type="button"
            class="btn-log-action btn-download font-mono"
            @click="downloadLogs"
            title="Download log file"
          >
            <span>💾</span>
            <span>Export .log</span>
          </button>

          <button
            type="button"
            class="btn-log-action font-mono"
            @click="clearLogs"
            title="Clear current log view"
          >
            <span>🧹</span>
            <span>Clear</span>
          </button>
        </div>

        <!-- Meta status badge -->
        <div class="stream-status-meta font-mono">
          <span class="meta-pill">
            <span class="text-muted">Pod:</span>
            <strong class="text-cyan">{{ pod }}</strong>
          </span>
          <span class="meta-pill">
            <span class="text-muted">NS:</span>
            <span class="text-amber">{{ namespace }}</span>
          </span>
          <span class="meta-pill">
            <span class="text-muted">Lines:</span>
            <span class="text-emerald">{{ filteredLogs.length }}</span>
          </span>
        </div>
      </div>
    </div>

    <!-- Log Terminal Container -->
    <div class="log-terminal-window">
      <!-- Terminal Window Top Bar -->
      <div class="terminal-topbar">
        <div class="terminal-dots">
          <span class="tdot tdot-red"></span>
          <span class="tdot tdot-yellow"></span>
          <span class="tdot tdot-green"></span>
        </div>
        <div class="terminal-title font-mono truncate">
          {{ pod }} &gt; {{ selectedContainer || 'container' }} (namespace: {{ namespace }})
        </div>
        <div class="terminal-controls-right font-mono">
          <span v-if="isFollowing && isConnected" class="stream-badge-live">
            <span class="live-dot"></span> SSE LIVE
          </span>
          <span v-else class="stream-badge-paused">
            ⏸️ PAUSED
          </span>
        </div>
      </div>

      <!-- Terminal Body Content -->
      <div
        ref="logContainerRef"
        class="terminal-body"
        :class="{ 'wrap-text': wrapLines }"
        @scroll="handleScroll"
      >
        <!-- Loading State -->
        <div v-if="isLoading && rawLines.length === 0" class="terminal-loading">
          <div class="terminal-spinner"></div>
          <span class="font-mono">Connecting to SSE log stream for pod {{ pod }}...</span>
        </div>

        <!-- Error State -->
        <div v-else-if="streamError && rawLines.length === 0" class="terminal-error">
          <span class="text-rose font-bold font-mono">⚠️ {{ streamError }}</span>
          <button type="button" class="btn-log-action font-mono mt-2" @click="reloadLogs">
            🔄 Retry Connection
          </button>
        </div>

        <!-- Empty State -->
        <div v-else-if="filteredLogs.length === 0" class="terminal-empty">
          <span class="font-mono text-muted">🛡️ No log lines match the current filters or no logs emitted yet.</span>
        </div>

        <!-- Log Lines List -->
        <div v-else class="log-lines-list">
          <div
            v-for="line in filteredLogs"
            :key="line.id"
            class="log-line"
            :class="`log-line-${line.level}`"
          >
            <span class="log-line-num font-mono">{{ line.id }}</span>
            <span v-if="line.timestamp" class="log-line-time font-mono text-muted">{{ line.timestamp }}</span>
            <span
              class="log-line-level font-mono"
              :class="{
                'text-rose': line.level === 'error',
                'text-amber': line.level === 'warn',
                'text-emerald': line.level === 'info',
                'text-slate': line.level === 'debug',
              }"
            >
              [{{ line.level.toUpperCase() }}]
            </span>
            <span class="log-line-content font-mono">{{ line.cleanText }}</span>
          </div>
        </div>

        <!-- Jump to bottom button -->
        <button
          v-if="userScrolledUp && filteredLogs.length > 0"
          type="button"
          class="jump-bottom-btn font-mono"
          @click="scrollToBottom"
        >
          ⬇️ New logs available - Jump to bottom
        </button>
      </div>

      <!-- Terminal Footer -->
      <div class="terminal-footer font-mono">
        <span class="footer-info">
          Showing <strong>{{ filteredLogs.length }}</strong> of <strong>{{ allParsedLogs.length }}</strong> lines (Errors: {{ levelCounts.error }}, Warnings: {{ levelCounts.warn }})
        </span>
        <span class="cluster-tag">Cluster: {{ cluster }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/pod-log-viewer.css';
</style>
