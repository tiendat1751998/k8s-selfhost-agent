<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import { useLogStore } from '../../stores/logStore'
import { calculateVirtualWindow } from '../../utils/virtualWindow'
import { detectStackTrace, type StackTraceInfo } from '../../utils/stackTraceParser'
import { formatTime, getTargetBadge } from './virtualLogUtils'
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
}

const props = withDefaults(defineProps<Props>(), {
  isConnected: true, isPaused: false, autoScroll: true, isScrollLocked: false,
  targetName: 'cluster', latency: 0, wrapLines: true, rowHeight: 24,
  hoverPauseEnabled: true,
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
}>()

const logStore = useLogStore()
const viewportRef = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const clientHeight = ref(600)
const unpinnedNewLogsCount = ref(0)
const isPinnedToBottom = ref(true)
const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth < 640 : false)
const effectiveRowHeight = computed(() => isMobile.value ? 58 : props.rowHeight)

// Expanded stack traces map: index -> boolean
const expandedStackTraces = ref<Record<number, boolean>>({})

onMounted(() => {
  emit('registerTerminal', viewportRef.value)
  if (viewportRef.value) {
    clientHeight.value = viewportRef.value.clientHeight || 600
    viewportRef.value.addEventListener('scroll', handleViewportScroll, { passive: true })
  }
  window.addEventListener('resize', handleResize)
  if (props.autoScroll) {
    scrollToBottom()
  }
})

onBeforeUnmount(() => {
  if (viewportRef.value) {
    viewportRef.value.removeEventListener('scroll', handleViewportScroll)
  }
  window.removeEventListener('resize', handleResize)
})

function handleResize() {
  if (viewportRef.value) {
    clientHeight.value = viewportRef.value.clientHeight || 600
  }
  if (typeof window !== 'undefined') {
    isMobile.value = window.innerWidth < 640
  }
}

const windowResult = computed(() => {
  return calculateVirtualWindow({
    totalCount: props.logs.length,
    scrollTop: scrollTop.value,
    clientHeight: clientHeight.value,
    itemHeight: effectiveRowHeight.value,
    overscan: 10,
  })
})

const visibleItems = computed(() => {
  const { startIndex, endIndex } = windowResult.value
  const slice = props.logs.slice(startIndex, endIndex)
  return slice.map((log, offset) => {
    const actualIndex = startIndex + offset
    const stackInfo: StackTraceInfo = detectStackTrace(log.msg)
    const isExpanded = !!expandedStackTraces.value[actualIndex]
    return {
      log,
      actualIndex,
      stackInfo,
      isExpanded,
    }
  })
})

function toggleStackTrace(index: number) {
  expandedStackTraces.value[index] = !expandedStackTraces.value[index]
}

function handleViewportScroll(e: Event) {
  emit('scroll', e)
  const el = viewportRef.value
  if (!el) return

  scrollTop.value = el.scrollTop
  const threshold = effectiveRowHeight.value * 2
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight <= threshold

  if (atBottom) {
    isPinnedToBottom.value = true
    unpinnedNewLogsCount.value = 0
  } else {
    isPinnedToBottom.value = false
  }
}

function scrollToBottom() {
  const el = viewportRef.value
  if (!el) return
  requestAnimationFrame(() => {
    el.scrollTop = el.scrollHeight
    scrollTop.value = el.scrollTop
    isPinnedToBottom.value = true
    unpinnedNewLogsCount.value = 0
    emit('scrollToBottom')
  })
}

const isHovered = ref(false)
const isHoverPaused = computed(() => !!props.hoverPauseEnabled && isHovered.value)

function handleMouseEnter() {
  isHovered.value = true
  emit('hoverChange', true)
}

function handleMouseLeave() {
  isHovered.value = false
  emit('hoverChange', false)
  if (props.autoScroll && isPinnedToBottom.value && !props.isScrollLocked) {
    scrollToBottom()
  }
}

function handleTargetClick(e: MouseEvent | KeyboardEvent, log: LogEntry) {
  const svc = log.service || log.pod || log.container
  if (!svc) return
  if ('altKey' in e && e.altKey) {
    emit('muteService', svc)
    return
  }
  emit('filterService', { node: log.node, service: svc })
}

function handleMuteClick(log: LogEntry) {
  const svc = log.service || log.pod || log.container
  if (svc) emit('muteService', svc)
}

watch(
  () => props.logs.length,
  (newLen, oldLen) => {
    const diff = newLen - (oldLen || 0)
    const isPausedReading = isHoverPaused.value || props.isScrollLocked || !isPinnedToBottom.value
    if (diff > 0 && isPausedReading) {
      unpinnedNewLogsCount.value += diff
    }
    if (props.autoScroll && isPinnedToBottom.value && !props.isScrollLocked && !isHoverPaused.value) {
      nextTick(() => scrollToBottom())
    }
  }
)

function handleTraceClick(traceId: string) {
  emit('openTrace', traceId)
  logStore.openTraceDrawer(traceId)
}

function handleContextClick(entry: LogEntry) {
  emit('openContext', entry)
  logStore.openContextModal(entry)
}

defineExpose({
  scrollToBottom,
  viewportRef,
  isHoverPaused,
})
</script>

<template>
  <div class="virtual-terminal-window glass-panel" role="region" aria-label="Virtual Kubernetes Terminal Log Stream">
    <!-- Virtual Viewport Container -->
    <div
      ref="viewportRef"
      class="virtual-viewport font-mono"
      tabindex="0"
      aria-label="Virtual log stream viewport"
      @mouseenter="handleMouseEnter"
      @mouseleave="handleMouseLeave"
    >
      <!-- Spacer providing the total virtual scroll height -->
      <div
        class="virtual-spacer"
        :style="{ height: windowResult.totalHeight + 'px' }"
      >
        <!-- Transformed slice rendering visible DOM items -->
        <div
          class="virtual-content"
          :style="{ transform: `translateY(${windowResult.offsetY}px)` }"
        >
          <div
            v-for="item in visibleItems"
            :key="item.actualIndex"
            class="virtual-log-row"
            :class="['log-' + item.log.level.toLowerCase()]"
          >
            <span class="virtual-log-idx">{{ item.actualIndex + 1 }}</span>
            <span class="virtual-badge-time">[{{ formatTime(item.log.time) }}]</span>
            <span class="virtual-badge-level" :class="'virtual-badge-' + item.log.level.toLowerCase()">
              [{{ item.log.level.padEnd(5) }}]
            </span>
            <span
              class="virtual-badge-target"
              role="button"
              tabindex="0"
              :title="`Click to isolate ${item.log.service || item.log.pod || 'service'} • Alt+Click to mute`"
              @click.stop="handleTargetClick($event, item.log)"
              @keydown.enter="handleTargetClick($event, item.log)"
              @keydown.space.prevent="handleTargetClick($event, item.log)"
            >
              <span>[{{ getTargetBadge(item.log, targetName) }}]</span>
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

            <!-- Log Message & Stack Trace Section -->
            <div class="virtual-log-body">
              <span class="virtual-msg-line" :class="{ 'virtual-msg-nowrap': !wrapLines }">
                {{ item.stackInfo.isStackTrace ? item.stackInfo.firstLine : item.log.msg }}
              </span>

              <!-- Stack Trace Interactive Toggle -->
              <div v-if="item.stackInfo.isStackTrace">
                <button
                  type="button"
                  class="stack-toggle-btn"
                  :aria-expanded="item.isExpanded"
                  @click="toggleStackTrace(item.actualIndex)"
                >
                  <BaseIcon :name="item.isExpanded ? 'chevron-down' : 'chevron-right'" size="xs" />
                  <span>
                    {{ item.isExpanded ? '[- Collapse Stack Trace]' : `[+ Expand Stack Trace (${item.stackInfo.linesCount} lines)]` }}
                  </span>
                </button>
                <div v-if="item.isExpanded" class="stack-trace-expanded font-mono">
                  <div v-for="(sLine, sIdx) in item.stackInfo.stackLines" :key="sIdx">
                    {{ sLine }}
                  </div>
                </div>
              </div>
            </div>

            <!-- Action Badges (Trace & Context) -->
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

      <!-- Empty State -->
      <div v-if="logs.length === 0" class="virtual-empty-state font-mono">
        <BaseIcon name="radio" size="lg" />
        <p class="virtual-empty-title">
          {{ isConnected ? `Waiting for logs from [${targetName || 'cluster'}]...` : 'Disconnected from log stream. Reconnecting...' }}
        </p>
        <p class="virtual-empty-sub">
          {{ isConnected ? 'Live stream connected. Matching log events will appear in real-time.' : 'Attempting to establish WebSocket stream connection...' }}
        </p>
      </div>
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