<script setup lang="ts">
interface LevelCounts {
  total: number
  error: number
  warn: number
  info: number
}

defineProps<{
  containers?: string[]
  selectedContainer: string
  tailLines: number
  showPrevious: boolean
  selectedLevel: string
  levelCounts: LevelCounts
  searchQuery: string
  isFollowing: boolean
  isConnected: boolean
  autoScroll: boolean
  wrapLines: boolean
  isLoading: boolean
  logsCopied: boolean
  pod: string
  namespace: string
  filteredCount: number
}>()

const emit = defineEmits<{
  (e: 'update:selectedContainer', val: string): void
  (e: 'update:tailLines', val: number): void
  (e: 'update:showPrevious', val: boolean): void
  (e: 'update:selectedLevel', val: string): void
  (e: 'update:searchQuery', val: string): void
  (e: 'update:autoScroll', val: boolean): void
  (e: 'update:wrapLines', val: boolean): void
  (e: 'toggleFollowing'): void
  (e: 'reloadLogs'): void
  (e: 'copyLogs'): void
  (e: 'downloadLogs'): void
  (e: 'clearLogs'): void
}>()
</script>

<template>
  <div class="log-viewer-toolbar glass-panel">
    <!-- Top Row: Targets, Search, and Level Filters -->
    <div class="toolbar-top-row">
      <!-- Container Selector -->
      <div class="toolbar-item font-mono">
        <label class="toolbar-label">CONTAINER:</label>
        <select
          v-if="containers && containers.length > 1"
          :value="selectedContainer"
          class="select-input font-mono"
          @change="emit('update:selectedContainer', ($event.target as HTMLSelectElement).value)"
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
        <select
          :value="tailLines"
          class="select-input font-mono"
          @change="emit('update:tailLines', Number(($event.target as HTMLSelectElement).value))"
        >
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
          <input
            :checked="showPrevious"
            type="checkbox"
            class="cyber-checkbox"
            @change="emit('update:showPrevious', ($event.target as HTMLInputElement).checked)"
          />
          <span>Previous (Crashed)</span>
        </label>
      </div>

      <!-- Log Level Filter Chips -->
      <div class="log-level-chips">
        <button
          type="button"
          class="chip-btn"
          :class="{ active: selectedLevel === 'all' }"
          @click="emit('update:selectedLevel', 'all')"
        >
          All ({{ levelCounts.total }})
        </button>
        <button
          type="button"
          class="chip-btn chip-rose"
          :class="{ active: selectedLevel === 'error' }"
          @click="emit('update:selectedLevel', 'error')"
        >
          🔴 Errors ({{ levelCounts.error }})
        </button>
        <button
          type="button"
          class="chip-btn chip-amber"
          :class="{ active: selectedLevel === 'warn' }"
          @click="emit('update:selectedLevel', 'warn')"
        >
          🟡 Warnings ({{ levelCounts.warn }})
        </button>
        <button
          type="button"
          class="chip-btn chip-emerald"
          :class="{ active: selectedLevel === 'info' }"
          @click="emit('update:selectedLevel', 'info')"
        >
          🟢 Info ({{ levelCounts.info }})
        </button>
      </div>

      <!-- Search Input -->
      <div class="search-box">
        <span class="search-icon">🔍</span>
        <input
          :value="searchQuery"
          type="text"
          placeholder="Filter logs / exceptions..."
          class="search-input font-mono"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
        <button v-if="searchQuery" class="clear-search-btn" @click="emit('update:searchQuery', '')">✕</button>
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
          @click="emit('toggleFollowing')"
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
          @click="emit('update:autoScroll', !autoScroll)"
          title="Auto-scroll to latest lines"
        >
          <span>⬇️</span>
          <span>Auto-Scroll: {{ autoScroll ? 'ON' : 'OFF' }}</span>
        </button>

        <button
          type="button"
          class="btn-log-action font-mono"
          :class="{ 'active-toggle': wrapLines }"
          @click="emit('update:wrapLines', !wrapLines)"
          title="Toggle word wrap for long log lines"
        >
          <span>↩️</span>
          <span>Wrap Lines: {{ wrapLines ? 'ON' : 'OFF' }}</span>
        </button>

        <button
          type="button"
          class="btn-log-action font-mono"
          :disabled="isLoading"
          @click="emit('reloadLogs')"
          title="Reload Pod Logs"
        >
          <span :class="{ 'spin-icon': isLoading }">🔄</span>
          <span>{{ isLoading ? 'Loading...' : 'Refresh' }}</span>
        </button>

        <button
          type="button"
          class="btn-log-action font-mono"
          @click="emit('copyLogs')"
          title="Copy log text to clipboard"
        >
          <span>📋</span>
          <span>{{ logsCopied ? 'Copied!' : 'Copy' }}</span>
        </button>

        <button
          type="button"
          class="btn-log-action btn-download font-mono"
          @click="emit('downloadLogs')"
          title="Download log file"
        >
          <span>💾</span>
          <span>Export .log</span>
        </button>

        <button
          type="button"
          class="btn-log-action font-mono"
          @click="emit('clearLogs')"
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
          <span class="text-emerald">{{ filteredCount }}</span>
        </span>
      </div>
    </div>
  </div>
</template>
