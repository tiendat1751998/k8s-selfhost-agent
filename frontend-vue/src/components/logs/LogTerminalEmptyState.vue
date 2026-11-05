<script setup lang="ts">
import { computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  isTargetOffline?: boolean
  mode?: 'live' | 'historical'
  targetName?: string
  isConnected?: boolean
  offlineNodeName?: string
  streamMode?: 'live' | 'historical'
}

const props = withDefaults(defineProps<Props>(), {
  isTargetOffline: false,
  mode: 'live',
  targetName: 'cluster',
  isConnected: true,
  offlineNodeName: undefined,
  streamMode: undefined,
})

const emit = defineEmits<{
  (e: 'switchToHistory'): void
  (e: 'switch-to-history'): void
}>()

const effectiveOffline = computed(() => props.isTargetOffline || Boolean(props.offlineNodeName))
const effectiveMode = computed(() => props.streamMode || props.mode)
const effectiveTarget = computed(() => props.offlineNodeName || props.targetName || 'cluster')

function onSwitchToHistory() {
  emit('switchToHistory')
  emit('switch-to-history')
}

defineExpose({
  onSwitchToHistory,
})
</script>

<template>
  <!-- Offline Node Empty State in Live Mode -->
  <div
    v-if="effectiveOffline && effectiveMode === 'live'"
    class="virtual-empty-state font-mono offline-node-empty"
  >
    <BaseIcon name="zap-off" size="lg" class="text-amber" />
    <p class="virtual-empty-title">
      Node [{{ effectiveTarget }}] is currently offline — live stream unavailable
    </p>
    <p class="virtual-empty-sub">
      Historical log records may still be stored in ClickHouse MergeTree archive.
    </p>
    <button
      type="button"
      class="btn-switch-history font-mono"
      @click="onSwitchToHistory"
    >
      <BaseIcon name="clock" size="xs" />
      <span>Switch to History Logs</span>
    </button>
  </div>

  <!-- Standard Empty State -->
  <div v-else class="virtual-empty-state font-mono">
    <BaseIcon name="radio" size="lg" />
    <p class="virtual-empty-title">
      {{ isConnected ? `Waiting for logs from [${effectiveTarget}]...` : 'Disconnected from log stream. Reconnecting...' }}
    </p>
    <p class="virtual-empty-sub">
      {{ isConnected ? 'Live stream connected. Matching log events will appear in real-time.' : 'Attempting to establish WebSocket stream connection...' }}
    </p>
  </div>
</template>

<style scoped>
.btn-switch-history {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 12px;
  padding: 8px 16px;
  border-radius: 8px;
  background: var(--accent-primary, #2563eb);
  color: #ffffff;
  border: 1px solid transparent;
  font-size: 0.8125rem;
  font-weight: 600;
  line-height: 1.25;
  cursor: pointer;
  transition: background-color 0.15s ease, transform 0.1s ease;
}

.btn-switch-history:hover {
  background: var(--accent-hover, #1d4ed8);
  transform: translateY(-1px);
}

.btn-switch-history:focus-visible {
  outline: 2px solid var(--accent-primary, #2563eb);
  outline-offset: 2px;
}

.btn-switch-history :deep(svg),
.btn-switch-history svg {
  flex-shrink: 0;
}
</style>
