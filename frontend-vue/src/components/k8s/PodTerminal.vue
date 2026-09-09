<script setup lang="ts">
import { ref } from 'vue'
import { usePodTerminal, type PodTerminalProps } from '../../composables/usePodTerminal'

const props = withDefaults(defineProps<PodTerminalProps>(), {
  namespace: 'default',
  container: '',
  containers: () => [],
})

const terminalContainerRef = ref<HTMLElement | null>(null)

const {
  selectedContainer,
  connectionStatus,
  reconnect,
  clearTerminal,
  sendCommand,
} = usePodTerminal(props, terminalContainerRef)
</script>

<template>
  <div class="pod-terminal-wrapper">
    <!-- Top Control Bar -->
    <div class="terminal-header glass-panel">
      <div class="header-left">
        <div class="terminal-dots">
          <span class="tdot tdot-red"></span>
          <span class="tdot tdot-yellow"></span>
          <span class="tdot tdot-green"></span>
        </div>
        <div class="terminal-meta font-mono">
          <span class="meta-item">
            <span class="text-muted">Pod:</span>
            <strong class="text-cyan">{{ pod }}</strong>
          </span>
          <span class="meta-item">
            <span class="text-muted">NS:</span>
            <span class="text-amber">{{ namespace }}</span>
          </span>
        </div>

        <!-- Container Selector (if multi-container) -->
        <div v-if="containers && containers.length > 1" class="container-select-wrap font-mono">
          <label class="select-label">CONTAINER:</label>
          <select v-model="selectedContainer" class="container-select">
            <option v-for="c in containers" :key="c" :value="c">
              📦 {{ c }}
            </option>
          </select>
        </div>
        <div v-else-if="selectedContainer" class="container-badge font-mono">
          <span class="text-muted">Container:</span>
          <span class="text-emerald">📦 {{ selectedContainer }}</span>
        </div>
      </div>

      <div class="header-right">
        <!-- Status Indicator -->
        <div class="status-indicator font-mono" :class="`status-${connectionStatus}`">
          <span class="status-dot"></span>
          <span class="status-text">
            {{
              connectionStatus === 'connected'
                ? 'CONNECTED'
                : connectionStatus === 'connecting'
                ? 'CONNECTING...'
                : connectionStatus === 'error'
                ? 'ERROR'
                : 'DISCONNECTED'
            }}
          </span>
        </div>

        <!-- Action Buttons -->
        <div class="action-btn-group">
          <button
            v-if="connectionStatus !== 'connected'"
            type="button"
            class="btn-term btn-reconnect font-mono"
            @click="reconnect"
            title="Reconnect WebSocket session"
          >
            <span>🔄</span>
            <span>Reconnect</span>
          </button>
          <button
            type="button"
            class="btn-term btn-clear font-mono"
            @click="clearTerminal"
            title="Clear terminal screen"
          >
            <span>🧹</span>
            <span>Clear</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Quick Command Shortcuts Bar -->
    <div class="quick-commands-bar font-mono">
      <span class="quick-cmd-label text-muted">Quick Exec:</span>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('ls -la')">ls -la</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('ps aux')">ps aux</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('env')">env</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('df -h')">df -h</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('top')">top</button>
      <button type="button" class="btn-quick-cmd" @click="sendCommand('exit')">exit</button>
    </div>

    <!-- Terminal Screen Container -->
    <div class="terminal-body-container">
      <div ref="terminalContainerRef" class="xterm-render-target"></div>
    </div>

    <!-- Terminal Footer Info -->
    <div class="terminal-footer font-mono">
      <span class="footer-hint">💡 Shell: <code>/bin/sh</code> | Supports ANSI colors, Tab completion, and standard keystrokes.</span>
      <span class="cluster-badge">Cluster: {{ cluster }}</span>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/pod-terminal.css';
</style>
