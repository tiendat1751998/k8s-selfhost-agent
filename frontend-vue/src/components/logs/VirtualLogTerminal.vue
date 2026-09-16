<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import { useLogStore } from '../../stores/logStore'
import { calculateVirtualWindow } from '../../utils/virtualWindow'
import { detectStackTrace, type StackTraceInfo } from '../../utils/stackTraceParser'
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
}

const props = withDefaults(defineProps<Props>(), {
  isConnected: true,
  isPaused: false,
  autoScroll: true,
  isScrollLocked: false,
  targetName: 'cluster',
  latency: 0,
  wrapLines: true,
  rowHeight: 24,
})

const emit = defineEmits<{
  (e: 'scroll', event: Event): void
  (e: 'scrollToBottom'): void
  (e: 'registerTerminal', el: HTMLElement | null): void
  (e: 'openTrace', traceId: string): void
  (e: 'openContext', entry: LogEntry): void
}>()

const logStore = useLogStore()
const viewportRef = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const clientHeight = ref(600)
const unpinnedNewLogsCount = ref(0)
const isPinnedToBottom = ref(true)

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
}

const windowResult = computed(() => {
  return calculateVirtualWindow({
    totalCount: props.logs.length,
    scrollTop: scrollTop.value,
    clientHeight: clientHeight.value,
    itemHeight: props.rowHeight,
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
  const threshold = props.rowHeight * 2
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

watch(
  () => props.logs.length,
  (newLen, oldLen) => {
    const diff = newLen - (oldLen || 0)
    if (diff > 0 && !isPinnedToBottom.value) {
      unpinnedNewLogsCount.value += diff
    }
    if (props.autoScroll && isPinnedToBottom.value && !props.isScrollLocked) {
      nextTick(() => scrollToBottom())
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
})
</script>

<template>
  <div class="virtual-terminal-window glass-panel" role="region" aria-label="Virtual Kubernetes Terminal Log Stream">
    <!-- Stream Status Bar -->
    <div class="virtual-terminal-titlebar font-mono">
      <div class="virtual-terminal-status">
        <span class="pulse-dot" :class="!isConnected ? 'pulse-dot-rose' : (isPaused ? 'pulse-dot-amber' : 'pulse-dot-emerald')"></span>
        <span :class="isConnected ? (isPaused ? 'text-amber' : 'text-emerald') : 'text-rose'">
          {{ !isConnected ? 'RECONNECTING' : (isPaused ? 'PAUSED' : 'CONNECTED') }}
        </span>
        <span class="virtual-target-badge">{{ targetName }}</span>
        <span class="virtual-buffer-meta">({{ logs.length.toLocaleString() }} in buffer)</span>
      </div>
      <div class="virtual-fps-badge">
        <span>60 FPS Virtualized</span>
      </div>
    </div>

    <!-- Virtual Viewport Container -->
    <div
      ref="viewportRef"
      class="virtual-viewport font-mono"
      tabindex="0"
      aria-label="Virtual log stream viewport"
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
            <span class="virtual-badge-target" :title="getTargetBadge(item.log)">
              [{{ getTargetBadge(item.log) }}]
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
          Live stream is active. Matching log events will appear in real-time as they are emitted.
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