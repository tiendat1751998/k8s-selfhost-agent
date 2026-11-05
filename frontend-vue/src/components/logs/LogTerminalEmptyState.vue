<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  isTargetOffline?: boolean
  mode?: 'live' | 'historical'
  targetName?: string
  isConnected?: boolean
}

withDefaults(defineProps<Props>(), {
  isTargetOffline: false,
  mode: 'live',
  targetName: 'cluster',
  isConnected: true,
})

const emit = defineEmits<{
  (e: 'switchToHistory'): void
}>()
</script>

<template>
  <!-- Offline Node Empty State in Live Mode -->
  <div
    v-if="isTargetOffline && mode === 'live'"
    class="virtual-empty-state font-mono offline-node-empty"
  >
    <BaseIcon name="zap-off" size="lg" class="text-amber" />
    <p class="virtual-empty-title">
      Node [{{ targetName }}] is currently offline — live stream unavailable
    </p>
    <p class="virtual-empty-sub">
      Historical log records may still be stored in ClickHouse MergeTree archive.
    </p>
    <button
      type="button"
      class="btn-switch-history font-mono"
      @click="emit('switchToHistory')"
    >
      <BaseIcon name="clock" size="xs" />
      <span>Switch to History Logs</span>
    </button>
  </div>

  <!-- Standard Empty State -->
  <div v-else class="virtual-empty-state font-mono">
    <BaseIcon name="radio" size="lg" />
    <p class="virtual-empty-title">
      {{ isConnected ? `Waiting for logs from [${targetName || 'cluster'}]...` : 'Disconnected from log stream. Reconnecting...' }}
    </p>
    <p class="virtual-empty-sub">
      {{ isConnected ? 'Live stream connected. Matching log events will appear in real-time.' : 'Attempting to establish WebSocket stream connection...' }}
    </p>
  </div>
</template>
