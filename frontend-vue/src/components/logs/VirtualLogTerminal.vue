<script setup lang="ts">
import { ref, computed } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import { useLogStore } from '../../stores/logStore'
import { calculateVirtualWindow } from '../../utils/virtualWindow'
import { detectStackTrace, type StackTraceInfo } from '../../utils/stackTraceParser'
import {
  formatTime, formatLevelBadge, getBadgeLabel, shouldShowBadge, cleanLogMessage, coalesceLogEntries,
} from './virtualLogUtils'
import { useVirtualLogScroll } from './useVirtualLogScroll'
import LogTerminalEmptyState from './LogTerminalEmptyState.vue'
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  logs: LogEntry[]
  isConnected?: boolean
  isPaused?: boolean
  autoScroll?: boolean
  isScrollLocked?: boolean
  targetName?: string
  latency?: number
  wrapLines?: boolean
  rowHeight?: number
  hoverPauseEnabled?: boolean
  hideServiceBadge?: boolean
  isTargetOffline?: boolean
  mode?: 'live' | 'historical'
  showJournalctl?: boolean
  activeService?: string
}

const props = withDefaults(defineProps<Props>(), {
  isConnected: true, isPaused: false, autoScroll: true, isScrollLocked: false,
  targetName: 'cluster', latency: 0, wrapLines: true, rowHeight: 24,
  hoverPauseEnabled: true, hideServiceBadge: false, isTargetOffline: false, mode: 'live',
  showJournalctl: false, activeService: undefined,
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
  (e: 'select-journalctl'): void
}>()

const logStore = useLogStore()
const coalescedLogs = computed(() => coalesceLogEntries(props.logs))
const effectiveRowHeight = computed(() => (isMobile.value ? 58 : props.rowHeight))

const {
  viewportRef, scrollTop, clientHeight, unpinnedNewLogsCount,
  isPinnedToBottom, isMobile, isHoverPaused, scrollToBottom,
  handleMouseEnter, handleMouseLeave,
} = useVirtualLogScroll({
  autoScroll: computed(() => props.autoScroll), isScrollLocked: computed(() => props.isScrollLocked),
  hoverPauseEnabled: computed(() => props.hoverPauseEnabled), effectiveRowHeight,
  logsCount: computed(() => coalescedLogs.value.length), emitScroll: (e) => emit('scroll', e),
  emitScrollToBottom: () => emit('scrollToBottom'), emitHoverChange: (h) => emit('hoverChange', h),
  emitRegisterTerminal: (el) => emit('registerTerminal', el),
})

const expandedStackTraces = ref<Record<number, boolean>>({})
const expandedRowIndex = ref<number | null>(null)
function toggleRowExpand(index: number) { expandedRowIndex.value = expandedRowIndex.value === index ? null : index }

const windowResult = computed(() =>
  calculateVirtualWindow({
    totalCount: coalescedLogs.value.length,
    scrollTop: scrollTop.value,
    clientHeight: clientHeight.value,
    itemHeight: effectiveRowHeight.value,
    overscan: 10,
  })
)

const visibleItems = computed(() => {
  const { startIndex, endIndex } = windowResult.value
  const slice = coalescedLogs.value.slice(startIndex, endIndex)
  return slice.map((item, offset) => {
    const actualIndex = startIndex + offset
    const stackInfo: StackTraceInfo = detectStackTrace(item.fullMsg || item.log.msg)
    const isExpanded = !!expandedStackTraces.value[actualIndex]
    return {
      ...item,
      actualIndex,
      stackInfo,
      isExpanded,
    }
  })
})

function toggleStackTrace(index: number) { expandedStackTraces.value[index] = !expandedStackTraces.value[index] }

function handleTargetClick(e: MouseEvent | KeyboardEvent, log: LogEntry) {
  const svc = getBadgeLabel(log, props.targetName, props.activeService) || log.service || log.pod || log.container
  if (!svc) return
  if ('altKey' in e && e.altKey) {
    emit('muteService', svc)
    return
  }
  emit('filterService', { node: log.node, service: svc })
}

function handleMuteClick(log: LogEntry) {
  const svc = getBadgeLabel(log, props.targetName, props.activeService) || log.service || log.pod || log.container
  if (svc) emit('muteService', svc)
}

function handleTraceClick(traceId: string) {
  emit('openTrace', traceId)
  logStore.openTraceDrawer(traceId)
}

function handleContextClick(entry: LogEntry) {
  emit('openContext', entry)
  logStore.openContextModal(entry)
}

function handleSelectJournalctl() { emit('select-journalctl') }

defineExpose({
  scrollToBottom,
  viewportRef,
  isHoverPaused,
})
</script>

<template>
  <div class="virtual-terminal-window glass-panel" role="region" aria-label="Virtual Kubernetes Terminal Log Stream">
    <div
      ref="viewportRef"
      class="virtual-viewport font-mono"
      tabindex="0"
      aria-label="Virtual log stream viewport"
      @mouseenter="handleMouseEnter"
      @mouseleave="handleMouseLeave"
    >
      <div
        class="virtual-spacer"
        :style="{ height: (windowResult.totalHeight + 12) + 'px' }"
      >
        <div
          class="virtual-content"
          :style="{ transform: `translateY(${windowResult.offsetY + 6}px)` }"
        >
          <div
            v-for="item in visibleItems"
            :key="item.actualIndex"
            class="virtual-log-row"
            :class="['log-' + item.log.level.toLowerCase()]"
          >
            <span class="virtual-badge-time">[{{ formatTime(item.log.time) }}]</span>
            <span class="virtual-badge-level" :class="'virtual-badge-' + item.log.level.toLowerCase()">
              {{ formatLevelBadge(item.log.level) }}
            </span>
            <span
              v-if="shouldShowBadge(item.log, hideServiceBadge, activeService, targetName)"
              class="virtual-badge-target"
              role="button"
              tabindex="0"
              :title="`Click to isolate ${getBadgeLabel(item.log, targetName, activeService)} • Alt+Click to mute`"
              @click.stop="handleTargetClick($event, item.log)"
              @keydown.enter="handleTargetClick($event, item.log)"
              @keydown.space.prevent="handleTargetClick($event, item.log)"
            >
              <span>[{{ getBadgeLabel(item.log, targetName, activeService) }}]</span>
              <button
                type="button"
                class="virtual-target-mute-btn"
                title="Mute this service"
                aria-label="Mute this service"
                @click.stop="handleMuteClick(item.log)"
              >
                &times;
              </button>
            </span>

            <span
              v-if="item.repeatCount > 1"
              class="virtual-repeat-badge font-mono"
              :title="`Repeated ${item.repeatCount} times consecutively`"
            >
              &times;{{ item.repeatCount }}
            </span>
            <span
              v-if="item.mergedLinesCount > 0"
              class="virtual-multiline-badge font-mono"
              :title="`Merged ${item.mergedLinesCount} continuation lines`"
              @click.stop="toggleRowExpand(item.actualIndex)"
            >
              +{{ item.mergedLinesCount }} lines
            </span>

            <div class="virtual-log-body" @click="toggleRowExpand(item.actualIndex)">
              <span class="virtual-msg-line" :class="{ 'virtual-msg-nowrap': !wrapLines }">
                {{ item.stackInfo.isStackTrace ? cleanLogMessage(item.stackInfo.firstLine) : item.cleanMsg }}
              </span>

              <pre v-if="expandedRowIndex === item.actualIndex" class="virtual-raw-expanded font-mono" @click.stop>{{ item.fullMsg || item.log.msg }}</pre>

              <div v-if="item.stackInfo.isStackTrace">
                <button
                  type="button"
                  class="stack-toggle-btn"
                  :aria-expanded="item.isExpanded"
                  @click.stop="toggleStackTrace(item.actualIndex)"
                >
                  <BaseIcon :name="item.isExpanded ? 'chevron-down' : 'chevron-right'" size="xs" />
                  <span>
                    {{ item.isExpanded ? '[- Collapse Stack Trace]' : `[+ Expand Stack Trace (${item.stackInfo.linesCount} lines)]` }}
                  </span>
                </button>
                <div v-if="item.isExpanded" class="stack-trace-expanded font-mono" @click.stop>
                  <div v-for="(sLine, sIdx) in item.stackInfo.stackLines" :key="sIdx">
                    {{ sLine }}
                  </div>
                </div>
              </div>
            </div>

            <button
              v-if="item.log.traceId"
              type="button"
              class="virtual-badge-trace font-mono"
              title="Open transaction trace waterfall"
              @click="handleTraceClick(item.log.traceId)"
            >
              trace={{ item.log.traceId.slice(0, 8) }}
            </button>
            <button
              v-if="item.log.level.toUpperCase() === 'ERROR' || item.log.level.toUpperCase() === 'ERR'"
              type="button"
              class="virtual-btn-context font-mono"
              title="Inspect surrounding ClickHouse context (+/- 50 logs)"
              @click="handleContextClick(item.log)"
            >
              <BaseIcon name="crosshair" size="xs" />
              <span>Context</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Empty or Offline State -->
      <LogTerminalEmptyState
        v-if="coalescedLogs.length === 0"
        :is-target-offline="isTargetOffline"
        :mode="mode"
        :target-name="targetName"
        :is-connected="isConnected"
        :show-journalctl="showJournalctl"
        :active-service="activeService"
        @switch-to-history="emit('switchToHistory')"
        @select-journalctl="handleSelectJournalctl"
      />
    </div>

    <!-- Floating Jump to Bottom Button -->
    <button
      v-if="!isPinnedToBottom || isScrollLocked"
      type="button"
      class="btn-jump-bottom font-mono"
      aria-label="Scroll to newest logs"
      @click="scrollToBottom"
    >
      <BaseIcon name="arrow-up" size="xs" style="transform: rotate(180deg);" />
      <span>Jump to Bottom</span>
      <span v-if="unpinnedNewLogsCount > 0" class="jump-new-count font-mono">
        +{{ unpinnedNewLogsCount > 999 ? '999+' : unpinnedNewLogsCount }}
      </span>
    </button>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/virtual-log-terminal.css';
</style>