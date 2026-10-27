<script setup lang="ts">
import { ref } from 'vue'
import { usePodLogs, type PodLogProps } from '../../composables/usePodLogs'
import PodLogToolbar from './PodLogToolbar.vue'
import BaseIcon from '../ui/BaseIcon.vue'

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
    <PodLogToolbar
      :containers="containers"
      v-model:selected-container="selectedContainer"
      v-model:tail-lines="tailLines"
      v-model:show-previous="showPrevious"
      v-model:selected-level="selectedLevel"
      v-model:search-query="searchQuery"
      v-model:auto-scroll="autoScroll"
      v-model:wrap-lines="wrapLines"
      :level-counts="levelCounts"
      :is-following="isFollowing"
      :is-connected="isConnected"
      :is-loading="isLoading"
      :logs-copied="logsCopied"
      :pod="pod"
      :namespace="namespace"
      :filtered-count="filteredLogs.length"
      @toggle-following="toggleFollowing"
      @reload-logs="reloadLogs"
      @copy-logs="copyLogs"
      @download-logs="downloadLogs"
      @clear-logs="clearLogs"
    />

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
            <BaseIcon name="pause" size="xs" /> PAUSED
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
          <span class="text-rose font-bold font-mono"><BaseIcon name="alert-triangle" size="xs" /> {{ streamError }}</span>
          <button type="button" class="btn-log-action font-mono mt-2" @click="reloadLogs">
            <BaseIcon name="refresh" size="xs" /> Retry Connection
          </button>
        </div>

        <!-- Empty State -->
        <div v-else-if="filteredLogs.length === 0" class="terminal-empty">
          <span class="font-mono text-muted"><BaseIcon name="shield" size="xs" /> No log lines match the current filters or no logs emitted yet.</span>
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
          <BaseIcon name="arrow-down" size="xs" /> New logs available - Jump to bottom
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
