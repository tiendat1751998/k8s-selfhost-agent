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

    <!-- Terminal Body / Logs Output -->
    <div ref="terminalBody" class="terminal-body font-mono" tabindex="0" aria-label="Terminal log output" @scroll="emit('scroll', $event)">
      <div v-for="(log, idx) in logs" :key="idx" class="log-line" :class="'log-' + log.level.toLowerCase()">
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
.terminal-window { flex: 1; display: flex; flex-direction: column; background: #080c16; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; position: relative; min-height: 520px; }
.terminal-titlebar { height: 34px; background: rgba(15, 23, 42, 0.95); border-bottom: 1px solid rgba(255, 255, 255, 0.08); display: flex; align-items: center; justify-content: space-between; padding: 0 10px; gap: 10px; user-select: none; }
.terminal-stream-status { display: inline-flex; align-items: center; gap: 6px; font-size: 11px; font-weight: 700; flex-shrink: 0; }
.terminal-target-badge { color: #38bdf8; background: rgba(56, 189, 248, 0.12); padding: 1px 6px; border-radius: 4px; font-size: 10px; }
.buffer-count { color: #64748b; font-size: 10px; }
.terminal-search-group { display: flex; align-items: center; gap: 6px; flex: 1; max-width: 440px; }
.search-wrap { position: relative; display: flex; align-items: center; flex: 1; }
.search-ico { position: absolute; left: 8px; font-size: 11px; color: #64748b; pointer-events: none; }
.terminal-search-input { width: 100%; height: 24px; padding-left: 26px; padding-right: 48px; font-size: 11px; background: rgba(2, 6, 23, 0.75); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 4px; color: #f1f5f9; outline: none; transition: border-color 0.15s ease; }
.terminal-search-input:focus { border-color: rgba(56, 189, 248, 0.5); }
.regex-tag { position: absolute; right: 20px; font-size: 8.5px; font-weight: 700; color: #38bdf8; background: rgba(56, 189, 248, 0.15); padding: 0 3px; border-radius: 2px; }
.search-clear-btn { position: absolute; right: 4px; background: none; border: none; color: #64748b; font-size: 10px; cursor: pointer; padding: 0 4px; }
.search-clear-btn:hover { color: #fff; }
.terminal-level-select { height: 24px; padding: 0 6px; font-size: 10px; font-weight: 700; background: rgba(2, 6, 23, 0.75); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 4px; color: #94a3b8; outline: none; cursor: pointer; }
.terminal-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.action-group { display: inline-flex; align-items: center; gap: 6px; }
.terminal-autoscroll-toggle { display: inline-flex; align-items: center; gap: 4px; font-size: 10.5px; color: #94a3b8; cursor: pointer; }
.term-btn { height: 24px; padding: 0 8px; font-size: 10.5px; font-weight: 600; background: rgba(255, 255, 255, 0.05); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 4px; color: #cbd5e1; cursor: pointer; transition: all 0.15s ease; white-space: nowrap; }
.term-btn:hover { background: rgba(255, 255, 255, 0.1); color: #fff; }
.term-btn.btn-paused { background: rgba(245, 158, 11, 0.2); border-color: rgba(245, 158, 11, 0.4); color: #fbbf24; }
.terminal-body { flex: 1; padding: 10px 14px; overflow-y: auto; font-size: 11.5px; line-height: 1.6; background: #060911; }
.log-line { display: flex; align-items: baseline; gap: 8px; padding: 2px 4px; border-radius: 3px; transition: background 0.1s ease; white-space: pre-wrap; word-break: break-all; }
.log-line:hover { background: rgba(255, 255, 255, 0.04); }
.log-idx { color: rgba(255, 255, 255, 0.18); font-size: 9.5px; width: 32px; text-align: right; flex-shrink: 0; user-select: none; }
.badge-time { color: #64748b; font-size: 11px; flex-shrink: 0; user-select: none; }
.badge-level { font-size: 10px; font-weight: 700; flex-shrink: 0; padding: 0 4px; border-radius: 3px; user-select: none; }
.badge-info { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.25); }
.badge-warn { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.25); }
.badge-error { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.25); }
.badge-debug { background: rgba(100, 116, 139, 0.18); color: #94a3b8; border: 1px solid rgba(100, 116, 139, 0.25); }
.badge-target { color: #38bdf8; font-size: 11px; font-weight: 600; flex-shrink: 0; }
.log-msg { color: #e2e8f0; flex: 1; }
.log-warn { background: transparent; }
.log-warn .log-msg { color: #e2e8f0; }
.log-error { background: rgba(244, 63, 94, 0.04); }
.log-error .log-msg { color: #fecdd3; font-weight: 600; }
.badge-trace { font-size: 9.5px; color: #64748b; background: rgba(255, 255, 255, 0.04); padding: 0 4px; border-radius: 3px; flex-shrink: 0; }
.empty-terminal { display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 60px 20px; text-align: center; color: #64748b; }
.empty-icon { font-size: 28px; margin-bottom: 8px; opacity: 0.6; }
.empty-title { font-size: 13px; color: #94a3b8; margin-bottom: 4px; }
.empty-sub { font-size: 11px; color: #475569; }
.btn-scroll-bottom { position: absolute; bottom: 16px; right: 20px; background: rgba(6, 182, 212, 0.9); color: #fff; border: none; padding: 6px 12px; border-radius: 6px; font-size: 11px; font-weight: 700; cursor: pointer; box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5); transition: all 0.15s ease; }
.btn-scroll-bottom:hover { background: #06b6d4; transform: translateY(-1px); }

/* Responsive Visibility Helpers */
.desktop-only { display: inline-flex !important; }
.mobile-only { display: none !important; }

@media (max-width: 640px) {
  .desktop-only { display: none !important; }
  .mobile-only { display: inline-flex !important; }
  .mobile-search-bar.mobile-only { display: flex !important; }
  .terminal-window { border-radius: 0; border-left: none; border-right: none; min-height: 0; height: 100%; }
  .terminal-titlebar { height: 38px; padding: 0 8px; gap: 6px; }
  .mobile-target-btn { height: 26px; padding: 0 8px; background: rgba(56, 189, 248, 0.12); border: 1px solid rgba(56, 189, 248, 0.35); border-radius: 4px; color: #38bdf8; font-size: 11px; font-weight: 700; cursor: pointer; max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .term-icon-btn { width: 30px; height: 30px; min-width: 30px; padding: 0; align-items: center; justify-content: center; font-size: 12px; }
  .term-icon-btn.btn-active { background: rgba(56, 189, 248, 0.2); border-color: rgba(56, 189, 248, 0.5); color: #38bdf8; }
  .mobile-search-bar { height: 28px; padding: 2px 8px; background: rgba(15, 23, 42, 0.98); border-bottom: 1px solid rgba(255, 255, 255, 0.08); gap: 6px; align-items: center; }
  .mobile-search-bar .terminal-search-input { height: 24px; font-size: 11px; padding-left: 24px; padding-right: 24px; }
  .mobile-search-bar .terminal-level-select { height: 24px; font-size: 10px; padding: 0 4px; }
  .terminal-body { padding: 4px 6px; font-size: 11px; line-height: 1.35; }
  .log-line { padding: 2px 0; gap: 4px; font-size: 11px; line-height: 1.35; flex-wrap: wrap; }
  .log-idx { display: none !important; }
  .badge-time { font-size: 10px; color: #64748b; }
  .badge-level { font-size: 9.5px; padding: 0 3px; }
  .badge-target { font-size: 10.5px; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .log-msg { font-size: 11px; line-height: 1.35; width: 100%; flex: 1 1 100%; word-break: break-word; }
}
</style>
