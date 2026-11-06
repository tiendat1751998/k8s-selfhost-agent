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
  hoverPauseEnabled?: boolean
  hideServiceBadge?: boolean
  isTargetOffline?: boolean
  mode?: 'live' | 'historical'
  showJournalctl?: boolean
  activeService?: string
}

withDefaults(defineProps<Props>(), {
  autoScroll: true,
  isScrollLocked: false,
  wrapLines: true,
  latency: 0,
  hoverPauseEnabled: true,
  hideServiceBadge: false,
  isTargetOffline: false,
  mode: 'live',
  showJournalctl: false,
  activeService: undefined,
})


const emit = defineEmits<{
  (e: 'scroll', event: Event): void
  (e: 'scrollToBottom'): void
  (e: 'registerTerminal', el: HTMLElement | null): void
  (e: 'openTrace', traceId: string): void
  (e: 'openContext', entry: LogEntry): void
  (e: 'filterService', payload: { node?: string; service: string }): void
  (e: 'muteService', service: string): void
  (e: 'hoverChange', hovering: boolean): void
  (e: 'switchToHistory'): void
  (e: 'selectJournalctl'): void
  (e: 'select-journalctl'): void
}>()

const virtualTerminalRef = ref<InstanceType<typeof VirtualLogTerminal> | null>(null)

function scrollToBottom() {
  virtualTerminalRef.value?.scrollToBottom()
  emit('scrollToBottom')
}

function handleSelectJournalctl() {
  emit('select-journalctl')
  emit('selectJournalctl')
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
    :hover-pause-enabled="hoverPauseEnabled"
    :hide-service-badge="hideServiceBadge"
    :is-target-offline="isTargetOffline"
    :mode="mode"
    :show-journalctl="showJournalctl"
    :active-service="activeService"
    @scroll="emit('scroll', $event)"
    @scroll-to-bottom="emit('scrollToBottom')"
    @register-terminal="emit('registerTerminal', $event)"
    @open-trace="emit('openTrace', $event)"
    @open-context="emit('openContext', $event)"
    @filter-service="emit('filterService', $event)"
    @mute-service="emit('muteService', $event)"
    @hover-change="emit('hoverChange', $event)"
    @switch-to-history="emit('switchToHistory')"
    @select-journalctl="handleSelectJournalctl"
  />
</template>