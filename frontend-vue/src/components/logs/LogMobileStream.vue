<script setup lang="ts">
import { ref } from 'vue'
import type { LogEntry } from '../../stores/logStore'

interface Props {
  logs: LogEntry[]
  isConnected: boolean
  isPaused: boolean
  autoScroll: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'togglePause'): void
  (e: 'clearBuffer'): void
  (e: 'scrollToBottom'): void
}>()

const expandedIndex = ref<number | null>(null)

function toggleExpand(idx: number) {
  expandedIndex.value = expandedIndex.value === idx ? null : idx
}
</script>

<template>
  <div class="mobile-stream-container" role="feed" aria-label="Mobile Log Feed">
    <div
      v-for="(log, idx) in logs"
      :key="idx"
      class="mobile-log-card"
      :class="'card-' + log.level.toLowerCase()"
      @click="toggleExpand(idx)"
    >
      <div class="mobile-log-header">
        <div class="mobile-meta-tags">
          <span class="log-level-badge" :class="'badge-' + log.level.toLowerCase()">{{ log.level }}</span>
          <span class="log-pod font-mono">{{ log.pod }}</span>
          <span class="log-ns font-mono">[{{ log.namespace }}]</span>
        </div>
        <span class="log-time font-mono">{{ log.time }}</span>
      </div>

      <div
        class="mobile-log-msg font-mono"
        :style="{
          display: expandedIndex === idx ? 'block' : '-webkit-box',
          WebkitLineClamp: expandedIndex === idx ? 'unset' : '3',
          WebkitBoxOrient: 'vertical',
          overflow: 'hidden'
        }"
      >
        {{ log.msg }}
      </div>

      <div v-if="log.traceId && expandedIndex === idx" class="log-trace font-mono" style="margin-top: 4px;">
        trace_id={{ log.traceId }}
      </div>
    </div>

    <!-- Empty mobile state -->
    <div v-if="logs.length === 0" class="empty-terminal" style="padding: 40px 16px;">
      <span class="empty-icon" aria-hidden="true">⚡</span>
      <p>
        {{ isConnected ? 'No logs matching current filter' : 'Disconnected from stream...' }}
      </p>
    </div>

    <!-- Sticky Mobile Floating Bottom Control Bar -->
    <div class="mobile-floating-bar font-mono">
      <button
        type="button"
        class="btn btn-secondary"
        style="padding: 6px 12px; font-size: 11px;"
        @click="emit('togglePause')"
      >
        <span>{{ isPaused ? '▶ Resume' : '⏸ Pause' }}</span>
      </button>

      <span class="buffer-count" style="font-size: 11px;">
        {{ logs.length }} logs
      </span>

      <div style="display: flex; gap: 8px;">
        <button
          type="button"
          class="btn btn-secondary"
          style="padding: 6px 10px; font-size: 11px;"
          title="Scroll to bottom"
          @click="emit('scrollToBottom')"
        >
          <span>⬇ Bottom</span>
        </button>
        <button
          type="button"
          class="btn btn-secondary"
          style="padding: 6px 10px; font-size: 11px;"
          title="Clear buffer"
          @click="emit('clearBuffer')"
        >
          <span>🧹</span>
        </button>
      </div>
    </div>
  </div>
</template>
