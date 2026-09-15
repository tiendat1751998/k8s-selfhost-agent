<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  logs: LogEntry[]
  isConnected: boolean
  isPaused: boolean
  autoScroll?: boolean
  isScrollLocked?: boolean
  targetName: string
  latency?: number
  wrapLines?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  autoScroll: true,
  isScrollLocked: false,
  wrapLines: true,
  latency: 0,
})

const emit = defineEmits<{
  (e: 'scroll', event: Event): void
  (e: 'scrollToBottom'): void
  (e: 'registerTerminal', el: HTMLElement | null): void
}>()

const terminalBody = ref<HTMLElement | null>(null)

onMounted(() => emit('registerTerminal', terminalBody.value))

watch(
  [() => props.logs.length, () => props.logs[props.logs.length - 1], () => props.autoScroll],
  async () => {
    if (props.autoScroll && !props.isScrollLocked && terminalBody.value) {
      await nextTick()
      terminalBody.value.scrollTop = terminalBody.value.scrollHeight
    }
  }
)

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
    <!-- Sleek 24-28px Stream Info Strip -->
    <div class="terminal-titlebar">
      <div class="terminal-stream-status font-mono">
        <span class="pulse-dot" :class="!isConnected ? 'pulse-dot-rose' : (isPaused ? 'pulse-dot-amber' : 'pulse-dot-emerald')"></span>
        <span class="status-text font-mono" :class="isConnected ? (isPaused ? 'text-amber' : 'text-emerald') : 'text-rose'">
          {{ !isConnected ? 'RECONNECTING' : (isPaused ? 'PAUSED' : 'CONNECTED') }}
        </span>
        <span class="terminal-target-badge font-mono">{{ targetName }}</span>
        <span class="buffer-count font-mono">({{ logs.length }} entries)</span>
      </div>
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
        <span class="empty-icon" aria-hidden="true"><BaseIcon name="radio" size="lg" /></span>
        <p class="empty-title">{{ isConnected ? `Waiting for logs from [${targetName || 'cluster'}]...` : 'Disconnected from log stream. Reconnecting...' }}</p>
        <p class="empty-sub">Live stream is active. Matching log events will appear in real-time as they are emitted.</p>
      </div>
    </div>

    <!-- Jump to Bottom Floating Pill -->
    <button v-if="isScrollLocked" type="button" class="btn-scroll-bottom font-mono" aria-label="Scroll to newest logs" @click="emit('scrollToBottom')">
      <BaseIcon name="arrow-up" size="xs" style="transform: rotate(180deg);" />
      <span>Jump to Bottom</span>
    </button>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>
