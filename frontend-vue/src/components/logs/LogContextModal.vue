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

<style scoped>
.context-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(4, 7, 15, 0.8);
  backdrop-filter: blur(5px);
  z-index: 110;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.context-modal-container {
  width: 1040px;
  max-width: 96vw;
  height: 80vh;
  max-height: 800px;
  background: #080c16;
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 10px;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.9);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.context-modal-header {
  height: 44px;
  padding: 0 16px;
  background: rgba(15, 23, 42, 0.95);
  border-bottom: 1px solid rgba(148, 163, 184, 0.15);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}

.context-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.context-modal-title {
  font-size: 13px;
  font-weight: 700;
  color: #f8fafc;
}

.context-service-pill {
  font-size: 10.5px;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.25);
  padding: 1px 8px;
  border-radius: 4px;
}

.context-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.context-btn-secondary {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: rgba(30, 41, 59, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.2);
  color: #94a3b8;
  font-size: 11px;
  padding: 4px 10px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.context-btn-secondary:hover {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border-color: #38bdf8;
}

.context-btn-close {
  background: transparent;
  border: none;
  color: #94a3b8;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
}

.context-btn-close:hover {
  color: #f8fafc;
  background: rgba(255, 255, 255, 0.1);
}

.context-target-meta {
  padding: 8px 16px;
  background: rgba(244, 63, 94, 0.08);
  border-bottom: 1px solid rgba(244, 63, 94, 0.2);
  font-size: 11px;
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex-shrink: 0;
}

.meta-label {
  color: #fb7185;
  font-weight: 800;
  font-size: 10px;
}

.meta-timestamp {
  color: #94a3b8;
}

.meta-preview {
  color: #f1f5f9;
  overflow: hidden;
  text-overflow: ellipsis;
}

.context-modal-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
  background: #080c16;
}

.context-log-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 2px 6px;
  font-size: 11px;
  line-height: 1.5;
  border-radius: 3px;
  border: 1px solid transparent;
}

.context-log-row:hover {
  background: rgba(255, 255, 255, 0.04);
}

.context-target-line {
  background: rgba(245, 158, 11, 0.15) !important;
  border: 1px solid #f59e0b !important;
  box-shadow: 0 0 12px rgba(245, 158, 11, 0.25);
  font-weight: 700;
}

.context-target-line .context-msg {
  color: #fef08a !important;
}

.context-err-line:not(.context-target-line) {
  background: rgba(244, 63, 94, 0.06);
}

.context-line-idx {
  color: #475569;
  font-size: 10px;
  width: 32px;
  text-align: right;
  flex-shrink: 0;
  user-select: none;
}

.context-time {
  color: #64748b;
  font-size: 10px;
  flex-shrink: 0;
}

.context-lvl {
  font-size: 9.5px;
  font-weight: 700;
  padding: 0 4px;
  border-radius: 3px;
  flex-shrink: 0;
}

.lvl-error, .lvl-err {
  color: #fb7185;
}

.lvl-warn {
  color: #fbbf24;
}

.lvl-info {
  color: #34d399;
}

.lvl-debug {
  color: #94a3b8;
}

.context-msg {
  color: #e2e8f0;
  flex: 1;
  word-break: break-word;
  white-space: pre-wrap;
}

.context-loading-box,
.context-empty-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  color: #64748b;
}

.context-modal-footer {
  height: 40px;
  padding: 0 16px;
  background: rgba(15, 23, 42, 0.85);
  border-top: 1px solid rgba(148, 163, 184, 0.12);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}

.context-footer-info {
  font-size: 10.5px;
  color: #64748b;
}

.context-btn-primary {
  background: #38bdf8;
  color: #0b0f19;
  border: none;
  font-size: 11px;
  font-weight: 700;
  padding: 4px 14px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.context-btn-primary:hover {
  background: #7dd3fc;
}
</style>