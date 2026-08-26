<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>HIGH-THROUGHPUT REALTIME LOG AGGREGATOR</span>
        </div>
        <h1 class="view-title">Enterprise Kubernetes Log Stream Explorer</h1>
        <p class="view-desc">
          Ultra-low latency (<span class="highlight">&lt;50ms</span>) WebSocket pub/sub stream harvested by <span class="highlight">Vector DaemonSet</span> with in-memory non-blocking <span class="highlight">RingBuffer</span>.
        </p>
      </div>

      <div class="header-actions">
        <button 
          class="btn"
          :class="logStore.isPaused ? 'btn-primary' : 'btn-secondary'"
          @click="logStore.togglePause()"
        >
          <span>{{ logStore.isPaused ? '▶ Resume Live Tail' : '⏸ Pause Live Stream' }}</span>
        </button>
        <button class="btn btn-secondary" @click="logStore.clear()">
          <span>🧹 Clear Buffer</span>
        </button>
      </div>
    </div>

    <!-- Filter & Control Deck -->
    <div class="control-deck glass-panel">
      <div class="deck-row">
        <!-- Search Filter -->
        <div class="search-input-wrapper">
          <span class="search-icon">🔍</span>
          <input 
            v-model="searchKeyword" 
            type="text" 
            placeholder="Search regex, error codes, trace IDs, panic, OOM..." 
            class="input-glass search-input font-mono" 
          />
          <button v-if="searchKeyword" class="search-clear" @click="searchKeyword = ''">✕</button>
        </div>

        <!-- Level Selector -->
        <div class="deck-select-group">
          <span class="deck-label">Level:</span>
          <select v-model="selectedLevel" class="input-glass deck-select">
            <option value="">ALL LEVELS</option>
            <option value="ERROR">ERROR</option>
            <option value="WARN">WARN</option>
            <option value="INFO">INFO</option>
            <option value="DEBUG">DEBUG</option>
          </select>
        </div>

        <!-- Namespace Selector -->
        <div class="deck-select-group">
          <span class="deck-label">Namespace:</span>
          <select v-model="selectedNamespace" class="input-glass deck-select" @change="onNamespaceChange">
            <option value="">ALL NAMESPACES</option>
            <option value="production">production</option>
            <option value="staging">staging</option>
            <option value="logging">logging</option>
            <option value="vault">vault</option>
          </select>
        </div>

        <!-- Auto-scroll toggle -->
        <label class="auto-scroll-label">
          <input v-model="autoScroll" type="checkbox" class="toggle-cb" />
          <span>Auto-Scroll</span>
        </label>
      </div>
    </div>

    <!-- Terminal Window Container -->
    <div class="terminal-window glass-panel">
      <!-- Terminal Header / Titlebar -->
      <div class="terminal-titlebar">
        <div class="window-buttons">
          <span class="win-btn win-close"></span>
          <span class="win-btn win-min"></span>
          <span class="win-btn win-max"></span>
        </div>

        <div class="terminal-title font-mono">
          <span class="pulse-dot" :class="logStore.isPaused ? 'pulse-dot-amber' : (logStore.isConnected ? 'pulse-dot-emerald' : 'pulse-dot-rose')"></span>
          <span>live-tail://k8s-cluster.internal/logs/stream</span>
          <span class="buffer-count">({{ filteredLogs.length }} events in buffer)</span>
        </div>

        <div class="terminal-actions">
          <span class="font-mono" :class="logStore.isConnected ? 'text-emerald' : 'text-rose'" style="font-size: 11px;">
            WebSocket: {{ logStore.isConnected ? 'CONNECTED (<50ms)' : 'DISCONNECTED' }}
          </span>
        </div>
      </div>

      <!-- Terminal Body / Logs -->
      <div ref="terminalBody" class="terminal-body font-mono">
        <div v-for="(log, idx) in filteredLogs" :key="idx" class="log-line" :class="'log-' + log.level.toLowerCase()">
          <span class="log-idx">{{ idx + 1 }}</span>
          <span class="log-time">{{ log.time }}</span>
          <span class="log-level-badge" :class="'badge-' + log.level.toLowerCase()">{{ log.level }}</span>
          <span class="log-ns">[{{ log.namespace }}]</span>
          <span class="log-pod">{{ log.pod }}:</span>
          <span class="log-msg">{{ log.msg }}</span>
          <span v-if="log.traceId" class="log-trace font-mono">trace_id={{ log.traceId }}</span>
        </div>

        <div v-if="filteredLogs.length === 0" class="empty-terminal">
          <span class="empty-icon">⚡</span>
          <p>{{ logStore.isConnected ? 'Waiting for log events matching the filter...' : 'Disconnected from log stream. Reconnecting...' }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { useLogStore } from '../stores/logStore'

const route = useRoute()
const logStore = useLogStore()
const autoScroll = ref(true)
const searchKeyword = ref('')
const selectedLevel = ref('')
const selectedNamespace = ref('')
const terminalBody = ref<HTMLElement | null>(null)

onMounted(() => {
  if (route.query.namespace && typeof route.query.namespace === 'string') {
    selectedNamespace.value = route.query.namespace
  }
  if (route.query.search && typeof route.query.search === 'string') {
    searchKeyword.value = route.query.search
  } else if (route.query.pod && typeof route.query.pod === 'string') {
    searchKeyword.value = route.query.pod
  }
  logStore.connect(selectedNamespace.value)
})

onUnmounted(() => {
  logStore.disconnect()
})

function onNamespaceChange() {
  logStore.connect(selectedNamespace.value)
}

const filteredLogs = computed(() => {
  return logStore.logs.filter(l => {
    if (selectedLevel.value && l.level !== selectedLevel.value) return false
    if (selectedNamespace.value && l.namespace !== selectedNamespace.value) return false
    if (searchKeyword.value) {
      const kw = searchKeyword.value.toLowerCase()
      const matchMsg = l.msg?.toLowerCase().includes(kw)
      const matchPod = l.pod?.toLowerCase().includes(kw)
      const matchTrace = l.traceId?.toLowerCase().includes(kw)
      if (!matchMsg && !matchPod && !matchTrace) return false
    }
    return true
  })
})

watch(
  () => filteredLogs.value.length,
  async () => {
    if (autoScroll.value && terminalBody.value) {
      await nextTick()
      terminalBody.value.scrollTop = terminalBody.value.scrollHeight
    }
  }
)
</script>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: calc(100vh - 120px);
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 16px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 820px;
  margin-top: 4px;
}

.highlight {
  color: #fff;
  font-weight: 600;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.control-deck {
  padding: 12px 18px;
}

.deck-row {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
}

.search-input-wrapper {
  position: relative;
  flex: 1;
  min-width: 260px;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 12px;
  font-size: 13px;
  color: var(--text-muted);
}

.search-input {
  width: 100%;
  padding-left: 36px;
  padding-right: 32px;
}

.search-clear {
  position: absolute;
  right: 12px;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
}

.deck-select-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.deck-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.deck-select {
  padding: 6px 12px;
}

.auto-scroll-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
  cursor: pointer;
}

.toggle-cb {
  accent-color: var(--accent-cyan);
}

.terminal-window {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: var(--bg-terminal);
  border: 1px solid rgba(255, 255, 255, 0.1);
  overflow: hidden;
  box-shadow: 0 20px 40px -15px rgba(0, 0, 0, 0.9);
}

.terminal-titlebar {
  height: 38px;
  background: rgba(16, 24, 40, 0.9);
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
}

.window-buttons {
  display: flex;
  gap: 7px;
}

.win-btn {
  width: 11px;
  height: 11px;
  border-radius: 50%;
}

.win-close { background: #ff5f56; }
.win-min { background: #ffbd2e; }
.win-max { background: #27c93f; }

.terminal-title {
  font-size: 11px;
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  gap: 8px;
}

.buffer-count {
  color: var(--text-muted);
}

.terminal-body {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
  font-size: 12px;
  line-height: 1.7;
}

.log-line {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  transition: background 0.1s ease;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.04);
}

.log-idx {
  color: rgba(255, 255, 255, 0.2);
  width: 24px;
  user-select: none;
  font-size: 10px;
}

.log-time {
  color: var(--text-muted);
  user-select: none;
  font-size: 11px;
}

.log-level-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
  user-select: none;
}

.badge-info { background: rgba(16, 185, 129, 0.15); color: #34d399; }
.badge-warn { background: rgba(245, 158, 11, 0.15); color: #fbbf24; }
.badge-error { background: rgba(244, 63, 94, 0.15); color: #fb7185; }

.log-ns {
  color: #c084fc;
}

.log-pod {
  color: var(--accent-sky);
}

.log-msg {
  color: var(--text-primary);
  flex: 1;
  word-break: break-all;
}

.log-trace {
  font-size: 10px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.05);
  padding: 1px 6px;
  border-radius: 4px;
}

.log-error .log-msg {
  color: #fda4af;
  font-weight: 600;
}

.empty-terminal {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-muted);
  gap: 10px;
}

.empty-icon {
  font-size: 32px;
}

.pulse-dot-rose {
  background-color: #f43f5e;
  box-shadow: 0 0 10px #f43f5e;
}

.text-emerald { color: #34d399; }
.text-rose { color: #fb7185; }
.text-cyan { color: #38bdf8; }
.font-mono { font-family: var(--font-mono); }

/* Responsive Overhaul for Mobile & Tablets */
@media (max-width: 768px) {
  .view-container {
    height: auto;
    min-height: calc(100vh - 100px);
    gap: 14px;
  }

  .view-header {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .header-actions {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
    gap: 8px;
  }

  .header-actions .btn {
    flex: 1;
    min-width: 120px;
  }

  .control-deck {
    position: sticky;
    top: 0;
    z-index: 20;
    backdrop-filter: blur(12px);
    padding: 10px 14px;
    border-radius: 12px;
  }

  .deck-row {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }

  .search-input-wrapper {
    min-width: 100%;
  }

  .deck-select-group {
    width: 100%;
    justify-content: space-between;
  }

  .terminal-window {
    height: 380px;
    min-height: 380px;
    border-radius: 12px;
  }

  .terminal-body {
    padding: 10px 12px;
  }

  .log-line {
    flex-wrap: wrap;
    gap: 4px 8px;
    font-size: 11px;
  }

  .log-idx {
    display: none;
  }
}

@media (max-width: 640px) {
  .header-actions {
    display: flex;
    flex-direction: row;
    width: 100%;
    gap: 8px;
    flex-wrap: nowrap;
  }

  .header-actions .btn {
    flex: 1;
    min-width: 0;
    padding: 7px 10px;
    font-size: 11.5px;
    white-space: nowrap;
    justify-content: center;
  }

  .terminal-titlebar {
    padding: 0 10px;
  }

  .window-buttons {
    display: none;
  }
}
</style>
