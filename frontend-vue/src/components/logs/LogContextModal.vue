<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useLogStore, type LogEntry } from '../../stores/logStore'
import { getContextLogs } from '../../api/logging'
import { mapRawToLogEntry } from '../../utils/logBatchParser'
import BaseIcon from '../ui/BaseIcon.vue'

const logStore = useLogStore()
const isLoading = ref(false)
const errorMsg = ref<string | null>(null)
const contextLogs = ref<LogEntry[]>([])
const modalBodyRef = ref<HTMLElement | null>(null)

const isOpen = computed(() => logStore.contextModalOpen && !!logStore.contextTargetEntry)
const target = computed(() => logStore.contextTargetEntry)

watch(target, (newTarget) => {
  if (newTarget && logStore.contextModalOpen) {
    loadContext(newTarget)
  } else {
    contextLogs.value = []
  }
})

async function loadContext(entry: LogEntry) {
  isLoading.value = true
  errorMsg.value = null
  contextLogs.value = []

  try {
    const res = await getContextLogs({
      service: entry.service || entry.container || entry.pod,
      container_name: entry.container || entry.service,
      timestamp: entry.time,
      window: 50,
    })

    const rawList = res.entries || []
    const mapped: LogEntry[] = []

    for (let i = 0; i < rawList.length; i++) {
      const item = mapRawToLogEntry(rawList[i])
      if (item) mapped.push(item)
    }

    mapped.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())

    // If target entry is not in list, insert it at appropriate timestamp
    const targetTime = new Date(entry.time).getTime()
    const exists = mapped.some((m) => m.time === entry.time && m.msg === entry.msg)
    if (!exists) {
      let insertIdx = mapped.findIndex((m) => new Date(m.time).getTime() >= targetTime)
      if (insertIdx === -1) insertIdx = mapped.length
      mapped.splice(insertIdx, 0, entry)
    }

    contextLogs.value = mapped

    nextTick(() => {
      centerTargetLine()
    })
  } catch (err: unknown) {
    errorMsg.value = err instanceof Error ? err.message : 'Failed to retrieve surrounding context logs'
  } finally {
    isLoading.value = false
  }
}

function isTargetLine(log: LogEntry): boolean {
  if (!target.value) return false
  return log.time === target.value.time && log.msg === target.value.msg
}

function centerTargetLine() {
  if (!modalBodyRef.value) return
  const targetEl = modalBodyRef.value.querySelector('.context-target-line')
  if (targetEl) {
    targetEl.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }
}

function closeModal() {
  logStore.closeContextModal()
}
</script>

<template>
  <div v-if="isOpen" class="context-modal-backdrop" @click.self="closeModal">
    <div class="context-modal-container glass-panel font-mono" role="dialog" aria-label="Surrounding Log Context Window">
      <!-- Modal Header -->
      <div class="context-modal-header">
        <div class="context-header-left">
          <BaseIcon name="crosshair" size="sm" class="text-rose" />
          <span class="context-modal-title">Surrounding Log Context (&plusmn;50 logs)</span>
          <span v-if="target" class="context-service-pill">
            {{ target.service || target.pod || 'system' }}
          </span>
        </div>
        <div class="context-header-actions">
          <button type="button" class="context-btn-secondary" title="Re-center Target Line" @click="centerTargetLine">
            <BaseIcon name="target" size="xs" />
            <span>Center Target</span>
          </button>
          <button type="button" class="context-btn-close" aria-label="Close modal" @click="closeModal">
            <BaseIcon name="x" size="sm" />
          </button>
        </div>
      </div>

      <!-- Target Banner -->
      <div v-if="target" class="context-target-meta font-mono">
        <span class="meta-label">TARGET INCIDENT:</span>
        <span class="meta-timestamp">[{{ target.time }}]</span>
        <span class="meta-level font-bold text-rose">[{{ target.level }}]</span>
        <span class="meta-preview">{{ target.msg }}</span>
      </div>

      <!-- Modal Body / Log Window -->
      <div ref="modalBodyRef" class="context-modal-body font-mono">
        <div v-if="isLoading" class="context-loading-box">
          <BaseIcon name="loader" size="lg" class="animate-spin" />
          <p>Querying ClickHouse for 50 logs prior and 50 logs subsequent...</p>
        </div>

        <div v-else-if="errorMsg" class="context-empty-box text-rose">
          <BaseIcon name="alert-triangle" size="lg" />
          <p>{{ errorMsg }}</p>
        </div>

        <div v-else>
          <div
            v-for="(log, idx) in contextLogs"
            :key="idx"
            class="context-log-row"
            :class="{
              'context-target-line': isTargetLine(log),
              'context-err-line': log.level.toUpperCase() === 'ERROR' || log.level.toUpperCase() === 'ERR',
            }"
          >
            <span class="context-line-idx">{{ idx + 1 }}</span>
            <span class="context-time">[{{ log.time.split('T')[1]?.slice(0, 12) || log.time }}]</span>
            <span class="context-lvl" :class="'lvl-' + log.level.toLowerCase()">[{{ log.level.padEnd(5) }}]</span>
            <span class="context-msg">{{ log.msg }}</span>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="context-modal-footer">
        <span class="context-footer-info">Showing {{ contextLogs.length }} lines surrounding target error</span>
        <button type="button" class="context-btn-primary" @click="closeModal">Done</button>
      </div>
    </div>
  </div>
</template>

<style scoped src="@/assets/styles/components/log-context-modal.css"></style>