<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useLogStore } from '../../stores/logStore'
import { getTraceLogs } from '../../api/logging'
import type { LogEntry } from '../../stores/logStore'
import { mapRawToLogEntry } from '../../utils/logBatchParser'
import BaseIcon from '../ui/BaseIcon.vue'

const logStore = useLogStore()
const isLoading = ref(false)
const errorMsg = ref<string | null>(null)
const traceLogs = ref<LogEntry[]>([])
const activeErrorIdx = ref(0)
const logsBodyRef = ref<HTMLElement | null>(null)

const isOpen = computed(() => logStore.traceDrawerOpen && !!logStore.selectedTraceId)
const traceId = computed(() => logStore.selectedTraceId || '')

watch(traceId, (newId) => {
  if (newId && logStore.traceDrawerOpen) {
    loadTraceLogs(newId)
  } else {
    traceLogs.value = []
  }
})

async function loadTraceLogs(id: string) {
  isLoading.value = true
  errorMsg.value = null
  traceLogs.value = []
  activeErrorIdx.value = 0

  try {
    const res = await getTraceLogs(id)
    const rawList = res.entries || []
    const mapped: LogEntry[] = []

    for (let i = 0; i < rawList.length; i++) {
      const entry = mapRawToLogEntry(rawList[i])
      if (entry) mapped.push(entry)
    }

    // Chronological sort
    mapped.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
    traceLogs.value = mapped
  } catch (err: unknown) {
    errorMsg.value = err instanceof Error ? err.message : 'Failed to fetch trace logs'
  } finally {
    isLoading.value = false
  }
}

// Summary Metrics
const totalLines = computed(() => traceLogs.value.length)

const servicesInvolved = computed(() => {
  const set = new Set<string>()
  for (const log of traceLogs.value) {
    const s = log.service || log.container || log.pod
    if (s) set.add(s)
  }
  return Array.from(set)
})

const startTime = computed(() => {
  if (traceLogs.value.length === 0) return '-'
  const d = new Date(traceLogs.value[0].time)
  return isNaN(d.getTime()) ? traceLogs.value[0].time : d.toLocaleTimeString()
})

const endTime = computed(() => {
  if (traceLogs.value.length === 0) return '-'
  const d = new Date(traceLogs.value[traceLogs.value.length - 1].time)
  return isNaN(d.getTime()) ? traceLogs.value[traceLogs.value.length - 1].time : d.toLocaleTimeString()
})

const durationMs = computed(() => {
  if (traceLogs.value.length < 2) return 0
  const t0 = new Date(traceLogs.value[0].time).getTime()
  const t1 = new Date(traceLogs.value[traceLogs.value.length - 1].time).getTime()
  if (isNaN(t0) || isNaN(t1)) return 0
  return Math.max(0, t1 - t0)
})

const errorIndices = computed(() => {
  const indices: number[] = []
  for (let i = 0; i < traceLogs.value.length; i++) {
    const lvl = traceLogs.value[i].level.toUpperCase()
    if (lvl === 'ERROR' || lvl === 'ERR') {
      indices.push(i)
    }
  }
  return indices
})

const errorCount = computed(() => errorIndices.value.length)

function jumpToNextError() {
  if (errorIndices.value.length === 0) return
  activeErrorIdx.value = (activeErrorIdx.value + 1) % errorIndices.value.length
  scrollToErrorLine(errorIndices.value[activeErrorIdx.value])
}

function jumpToPrevError() {
  if (errorIndices.value.length === 0) return
  activeErrorIdx.value = (activeErrorIdx.value - 1 + errorIndices.value.length) % errorIndices.value.length
  scrollToErrorLine(errorIndices.value[activeErrorIdx.value])
}

function scrollToErrorLine(targetIdx: number) {
  nextTick(() => {
    if (!logsBodyRef.value) return
    const el = logsBodyRef.value.querySelector(`[data-trace-idx="${targetIdx}"]`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  })
}

function getDeltaTime(timeStr: string): string {
  if (traceLogs.value.length === 0) return '+0ms'
  const t0 = new Date(traceLogs.value[0].time).getTime()
  const tCur = new Date(timeStr).getTime()
  if (isNaN(t0) || isNaN(tCur)) return ''
  const diff = tCur - t0
  return diff >= 0 ? `+${diff}ms` : `${diff}ms`
}

function closeDrawer() {
  logStore.closeTraceDrawer()
}
</script>

<template>
  <div v-if="isOpen">
    <div class="trace-drawer-backdrop" @click="closeDrawer" />
    <div class="trace-drawer-panel glass-panel font-mono" role="dialog" aria-label="Transaction Trace Waterfall">
      <!-- Drawer Header -->
      <div class="trace-drawer-header">
        <div class="trace-title-group">
          <span class="trace-title">
            <BaseIcon name="git-commit" size="sm" />
            <span>Trace Waterfall</span>
          </span>
          <span class="trace-id-badge font-mono">{{ traceId }}</span>
        </div>
        <button type="button" class="trace-close-btn" aria-label="Close trace drawer" @click="closeDrawer">
          <BaseIcon name="x" size="sm" />
        </button>
      </div>

      <!-- Summary Metrics Grid -->
      <div class="trace-summary-grid">
        <div class="trace-summary-card">
          <span class="trace-kpi-label">Total Lines</span>
          <span class="trace-kpi-val">{{ totalLines.toLocaleString() }} lines</span>
        </div>
        <div class="trace-summary-card">
          <span class="trace-kpi-label">Services ({{ servicesInvolved.length }})</span>
          <div class="trace-services-list">
            <span v-for="svc in servicesInvolved" :key="svc" class="trace-service-tag">{{ svc }}</span>
          </div>
        </div>
        <div class="trace-summary-card">
          <span class="trace-kpi-label">Duration</span>
          <span class="trace-kpi-val">{{ durationMs }}ms ({{ startTime }} -> {{ endTime }})</span>
        </div>
        <div class="trace-summary-card">
          <span class="trace-kpi-label">Errors</span>
          <div class="trace-error-controls">
            <span class="trace-err-badge">{{ errorCount }} errs</span>
            <template v-if="errorCount > 0">
              <button type="button" class="trace-jump-btn" title="Previous error" @click="jumpToPrevError">&uarr;</button>
              <button type="button" class="trace-jump-btn" title="Next error" @click="jumpToNextError">&darr;</button>
            </template>
          </div>
        </div>
      </div>

      <!-- Loading / Error / Logs Viewport -->
      <div ref="logsBodyRef" class="trace-logs-body">
        <div v-if="isLoading" class="trace-loading-box">
          <BaseIcon name="loader" size="lg" class="animate-spin" />
          <p>Loading transaction trace logs from ClickHouse...</p>
        </div>

        <div v-else-if="errorMsg" class="trace-empty-box text-rose">
          <BaseIcon name="alert-triangle" size="lg" />
          <p>{{ errorMsg }}</p>
        </div>

        <div v-else-if="traceLogs.length === 0" class="trace-empty-box">
          <BaseIcon name="info" size="lg" />
          <p>No log records discovered for trace {{ traceId }}.</p>
        </div>

        <div v-else>
          <div
            v-for="(log, idx) in traceLogs"
            :key="idx"
            :data-trace-idx="idx"
            class="trace-log-line"
            :class="{ 'is-error-line': log.level.toUpperCase() === 'ERROR' || log.level.toUpperCase() === 'ERR' }"
          >
            <span class="trace-delta-time">{{ getDeltaTime(log.time) }}</span>
            <span class="trace-svc-badge">[{{ log.service || log.pod || 'system' }}]</span>
            <span class="trace-msg-text">{{ log.msg }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/log-trace-drawer.css';
</style>