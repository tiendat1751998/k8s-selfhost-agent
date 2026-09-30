<script setup lang="ts">
import { ref } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import VirtualLogTerminal from './VirtualLogTerminal.vue'

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

withDefaults(defineProps<Props>(), {
  autoScroll: true,
  isScrollLocked: false,
  wrapLines: true,
  latency: 0,
})

const emit = defineEmits<{
  (e: 'scroll', event: Event): void
  (e: 'scrollToBottom'): void
  (e: 'registerTerminal', el: HTMLElement | null): void
  (e: 'openTrace', traceId: string): void
  (e: 'openContext', entry: LogEntry): void
}>()

const virtualTerminalRef = ref<InstanceType<typeof VirtualLogTerminal> | null>(null)

function scrollToBottom() {
  virtualTerminalRef.value?.scrollToBottom()
  emit('scrollToBottom')
}

defineExpose({
  scrollToBottom,
})
</script>

<template>
  <VirtualLogTerminal
    ref="virtualTerminalRef"
    :logs="logs"
    :is-connected="isConnected"
    :is-paused="isPaused"
    :auto-scroll="autoScroll"
    :is-scroll-locked="isScrollLocked"
    :target-name="targetName"
    :latency="latency"
    :wrap-lines="wrapLines"
    @scroll="emit('scroll', $event)"
    @scroll-to-bottom="emit('scrollToBottom')"
    @register-terminal="emit('registerTerminal', $event)"
    @open-trace="emit('openTrace', $event)"
    @open-context="emit('openContext', $event)"
  />
</template>