<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import type { LogEntry } from '../../stores/logStore'

interface Props {
  logs: LogEntry[]
  isConnected: boolean
  isPaused: boolean
  autoScroll: boolean
  isScrollLocked: boolean
  searchQuery: string
  selectedLevel: string
  targetName: string
  latency?: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:searchQuery', val: string): void
  (e: 'update:selectedLevel', val: string): void
  (e: 'update:autoScroll', val: boolean): void
  (e: 'togglePause'): void
  (e: 'clearBuffer'): void
  (e: 'exportLogs'): void
  (e: 'scroll', event: Event): void
  (e: 'scrollToBottom'): void
  (e: 'registerTerminal', el: HTMLElement | null): void
  (e: 'toggleTargetTree'): void
}>()

const terminalBody = ref<HTMLElement | null>(null)
const mobileSearchOpen = ref(false)
const wrapLines = ref(true)

onMounted(() => emit('registerTerminal', terminalBody.value))

watch(() => props.logs.length, async () => {
  if (props.autoScroll && !props.isScrollLocked && terminalBody.value) {
    await nextTick()
    terminalBody.value.scrollTop = terminalBody.value.scrollHeight
  }
})

const isRegex = computed(() => {
  if (!props.searchQuery) return false
  return /[[\]{}()*+?^$\\.|]/.test(props.searchQuery)
})

function formatTime(t: string): string {
  if (!t) return ''
  if (t.includes('T')) {
    const p = t.split('T')[1]
    return p ? p.slice(0, 8) : t
  }
  if (t.length > 8 && t[2] === ':' && t[5] === ':') {
    return t.slice(0, 8)
  }
  return t
}

function getTargetBadge(log: LogEntry): string {
  if (log.node && log.service && log.node !== log.service) return `${log.node}/${log.service}`
  if (log.node && log.pod && log.node !== log.pod) return `${log.node}/${log.pod}`
  if (log.service) return log.service
  if (log.node) return log.node
  if (log.pod) return (log.namespace && log.namespace !== 'default') ? `${log.namespace}/${log.pod}` : log.pod
  return log.namespace || 'system'
}
</script>

<template>
  <div class="terminal-window glass-panel" role="region" aria-label="Kubernetes Terminal Log Stream">
    <!-- Modern Terminal Bar: Unified Mobile 38px / Desktop 34px -->
    <div class="terminal-titlebar">
      <!-- Left: Status & Mobile Target Pill -->
      <div class="terminal-stream-status font-mono">
        <button
          type="button"
          class="mobile-target-btn font-mono mobile-only"
          aria-label="Toggle log targets drawer"
          @click="emit('toggleTargetTree')"
        >
          <span>🌲 {{ targetName || 'All' }} ▾</span>
        </button>

        <span class="pulse-dot" :class="!isConnected ? 'pulse-dot-rose' : (isPaused ? 'pulse-dot-amber' : 'pulse-dot-emerald')"></span>
        <span class="status-text desktop-only" :class="isConnected ? (isPaused ? 'text-amber' : 'text-emerald') : 'text-rose'">
          {{ !isConnected ? '○ RECONNECTING' : (isPaused ? '⏸ PAUSED' : '● CONNECTED') }}
        </span>
        <span class="status-text mobile-only font-mono" :class="isConnected ? (isPaused ? 'text-amber' : 'text-emerald') : 'text-rose'">
          {{ !isConnected ? 'DISC' : (isPaused ? 'PAUSED' : 'LIVE') }}
        </span>
        <span class="terminal-target-badge desktop-only font-mono">{{ targetName }}</span>
        <span class="buffer-count font-mono">({{ logs.length }})</span>
      </div>

      <!-- Center Search & Regex Filter (Desktop) -->
      <div class="terminal-search-group desktop-only">
        <div class="search-wrap">
          <span class="search-ico" aria-hidden="true">🔍</span>
          <input
            :value="searchQuery"
            type="text"
            placeholder="Filter logs or regex..."
            class="terminal-search-input font-mono"
            aria-label="Filter logs"
            @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
          />
          <span v-if="isRegex" class="regex-tag font-mono">REGEX</span>
          <button v-if="searchQuery" type="button" class="search-clear-btn" aria-label="Clear filter" @click="emit('update:searchQuery', '')">✕</button>
        </div>
        <select
          :value="selectedLevel"
          class="terminal-level-select font-mono"
          aria-label="Filter by level"
          @change="emit('update:selectedLevel', ($event.target as HTMLSelectElement).value)"
        >
          <option value="">ALL LEVELS</option>
          <option value="INFO">INFO</option>
          <option value="WARN">WARN</option>
          <option value="ERROR">ERROR</option>
          <option value="DEBUG">DEBUG</option>
        </select>
      </div>

      <!-- Right Actions: Desktop & Mobile Rows -->
      <div class="terminal-actions">
        <label class="terminal-autoscroll-toggle font-mono desktop-only">
          <input
            :checked="autoScroll"
            type="checkbox"
            class="toggle-cb"
            @change="emit('update:autoScroll', ($event.target as HTMLInputElement).checked)"
          />
          <span>Auto-Scroll</span>
        </label>

        <!-- Desktop Buttons -->
        <div class="desktop-only action-group">
          <button
            type="button"
            class="term-btn"
            :class="{ 'btn-active': wrapLines }"
            :title="wrapLines ? 'Switch to nowrap mode (horizontal scroll)' : 'Switch to line wrap mode'"
            @click="wrapLines = !wrapLines"
          >
            <span>[ ↵ Wrap ]</span>
          </button>
          <button type="button" class="term-btn" :class="{ 'btn-paused': isPaused }" :title="isPaused ? 'Resume live stream' : 'Pause live stream'" @click="emit('togglePause')">
            <span>{{ isPaused ? '▶ Resume' : '⏸ Pause' }}</span>
          </button>
          <button type="button" class="term-btn" title="Clear buffer" @click="emit('clearBuffer')">
            <span>🧹 Clear</span>
          </button>
          <button type="button" class="term-btn" title="Export logs" @click="emit('exportLogs')">
            <span>📥 Export</span>
          </button>
        </div>

        <!-- Mobile Buttons (30x30px Compact Icons) -->
        <div class="mobile-only action-group">
          <button
            type="button"
            class="term-btn term-icon-btn"
            :class="{ 'btn-active': wrapLines }"
            :title="wrapLines ? 'Line wrap on' : 'Line wrap off'"
            @click="wrapLines = !wrapLines"
          >
            <span>↵</span>
          </button>
          <button type="button" class="term-btn term-icon-btn" :class="{ 'btn-paused': isPaused }" :title="isPaused ? 'Resume' : 'Pause'" @click="emit('togglePause')">
            <span>{{ isPaused ? '▶' : '⏸' }}</span>
          </button>
          <button type="button" class="term-btn term-icon-btn" title="Clear buffer" @click="emit('clearBuffer')">
            <span>🧹</span>
          </button>
          <button type="button" class="term-btn term-icon-btn" :class="{ 'btn-active': mobileSearchOpen || searchQuery }" title="Toggle search" @click="mobileSearchOpen = !mobileSearchOpen">
            <span>🔍</span>
          </button>
          <button type="button" class="term-btn term-icon-btn" title="Export logs" @click="emit('exportLogs')">
            <span>📥</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Collapsible Mobile Search & Level Bar (Slim 28px) -->
    <div v-show="mobileSearchOpen" class="mobile-search-bar mobile-only font-mono">
      <div class="search-wrap">
        <span class="search-ico" aria-hidden="true">🔍</span>
        <input
          :value="searchQuery"
          type="text"
          placeholder="Filter or regex..."
          class="terminal-search-input font-mono"
          aria-label="Filter logs mobile"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
        <button v-if="searchQuery" type="button" class="search-clear-btn" aria-label="Clear filter" @click="emit('update:searchQuery', '')">✕</button>
      </div>
      <select
        :value="selectedLevel"
        class="terminal-level-select font-mono"
        aria-label="Filter by level mobile"
        @change="emit('update:selectedLevel', ($event.target as HTMLSelectElement).value)"
      >
        <option value="">ALL</option>
        <option value="INFO">INFO</option>
        <option value="WARN">WARN</option>
        <option value="ERROR">ERR</option>
        <option value="DEBUG">DBG</option>
      </select>
    </div>

    <!-- Terminal Body / Logs Output with Wrap Toggle Support -->
    <div ref="terminalBody" class="terminal-body font-mono" tabindex="0" aria-label="Terminal log output" @scroll="emit('scroll', $event)">
      <div
        v-for="(log, idx) in logs"
        :key="idx"
        class="log-line"
        :class="['log-' + log.level.toLowerCase(), wrapLines ? 'log-wrap' : 'log-nowrap']"
      >
        <span class="log-idx">{{ idx + 1 }}</span>
        <span class="badge-time font-mono">[{{ formatTime(log.time) }}]</span>
        <span class="badge-level font-mono" :class="'badge-' + log.level.toLowerCase()">[{{ log.level.padEnd(5) }}]</span>
        <span class="badge-target font-mono">[{{ getTargetBadge(log) }}]</span>
        <span class="log-msg">{{ log.msg }}</span>
        <span v-if="log.traceId" class="badge-trace font-mono">trace={{ log.traceId }}</span>
      </div>

      <!-- Clean Empty Terminal State -->
      <div v-if="logs.length === 0" class="empty-terminal font-mono">
        <span class="empty-icon" aria-hidden="true">📡</span>
        <p class="empty-title">{{ isConnected ? `Waiting for logs from [${targetName || 'cluster'}]...` : 'Disconnected from log stream. Reconnecting...' }}</p>
        <p class="empty-sub">Live stream is active. Matching log events will appear in real-time as they are emitted.</p>
      </div>
    </div>

    <!-- Jump to Bottom Floating Pill -->
    <button v-if="isScrollLocked" type="button" class="btn-scroll-bottom font-mono" aria-label="Scroll to newest logs" @click="emit('scrollToBottom')">
      <span>⬇ Jump to Bottom</span>
    </button>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>
