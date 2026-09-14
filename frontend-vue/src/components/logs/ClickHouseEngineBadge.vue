<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import BaseIcon from '../ui/BaseIcon.vue'

interface EngineStatusPayload {
  engine: string
  status: 'connected' | 'fallback' | string
  latency_ms: number
  retention_days: number
  total_records: number
}

const showPopover = ref(false)

const status = ref<EngineStatusPayload>({
  engine: 'ClickHouse MergeTree',
  status: 'connected',
  latency_ms: 0,
  retention_days: 30,
  total_records: 0,
})

const isFallback = computed(() => {
  const isPrimary =
    status.value.engine.includes('Distributed') ||
    status.value.engine.includes('Edge LogEngine') ||
    status.value.engine.includes('ClickHouse MergeTree')
  return isPrimary ? false : status.value.status === 'fallback'
})

const storageFormat = computed(() =>
  status.value.engine.toLowerCase().includes('clickhouse')
    ? 'ClickHouse MergeTree'
    : 'Columnar Blocks (ZSTD >10x)'
)

const badgeText = computed(() => {
  if (isFallback.value) return `${status.value.engine} · Fallback`
  const latency = status.value.latency_ms > 0 ? `${status.value.latency_ms.toFixed(1)}ms` : '<1ms'
  return `${status.value.engine} · Ready · ${latency}`
})

const shortEngineName = computed(() => {
  const eng = status.value.engine
  if (eng.includes('ClickHouse')) return 'ClickHouse'
  if (eng.includes('Distributed') || eng.includes('Edge')) return 'Edge LogEngine'
  if (eng.includes('RingBuffer')) return 'RingBuffer'
  return eng.split(' ')[0] || eng
})

const shortBadgeText = computed(() => {
  if (isFallback.value) return `${shortEngineName.value} · Fallback`
  return `${shortEngineName.value} · Ready`
})

async function fetchStatus() {
  try {
    const res = await api.get<EngineStatusPayload>('/logs/status')
    if (res?.engine) {
      status.value = {
        engine: res.engine,
        status: res.status || 'connected',
        latency_ms: res.latency_ms ?? 0,
        retention_days: res.retention_days ?? 30,
        total_records: res.total_records ?? 0,
      }
    }
  } catch {
    status.value = {
      engine: 'In-Memory RingBuffer (Fallback)',
      status: 'fallback',
      latency_ms: 0.1,
      retention_days: 7,
      total_records: 0,
    }
  }
}

onMounted(fetchStatus)
</script>

<template>
  <div class="engine-badge-container log-engine-badge-wrapper" @mouseenter="showPopover = true" @mouseleave="showPopover = false">
    <button
      type="button"
      class="engine-badge engine-badge-trigger font-mono"
      :class="isFallback ? 'badge-fallback' : 'badge-connected'"
      role="status"
      aria-haspopup="dialog"
      :aria-expanded="showPopover"
      @click="showPopover = !showPopover"
    >
      <span class="pulse-dot" :class="isFallback ? 'pulse-dot-cyan' : 'pulse-dot-emerald'" />
      <span class="engine-badge-text">
        <span class="badge-text-full">{{ badgeText }}</span>
        <span class="badge-text-compact">{{ shortBadgeText }}</span>
      </span>
      <BaseIcon name="info" size="xs" style="opacity: 0.6;" />
    </button>

    <transition name="fade">
      <div v-if="showPopover" class="engine-popover glass-panel font-mono" role="tooltip">
        <div class="popover-header">
          <div class="popover-title-group">
            <BaseIcon name="layers" size="xs" />
            <span class="popover-title">ENGINE ARCHITECTURE</span>
          </div>
          <span class="popover-tag" :class="isFallback ? 'tag-amber' : 'tag-emerald'">
            {{ isFallback ? 'FALLBACK' : 'PRIMARY' }}
          </span>
        </div>
        <div class="popover-row">
          <span class="row-label"><BaseIcon name="server" size="xs" /><span>Engine Type</span></span>
          <strong class="text-slate">{{ status.engine }}</strong>
        </div>
        <div class="popover-row">
          <span class="row-label"><BaseIcon name="activity" size="xs" /><span>Status</span></span>
          <span class="status-pill" :class="isFallback ? 'pill-amber' : 'pill-emerald'">
            {{ isFallback ? status.status : 'Active' }}
          </span>
        </div>
        <div class="popover-row">
          <span class="row-label"><BaseIcon name="database" size="xs" /><span>Storage Format</span></span>
          <strong class="text-emerald">{{ storageFormat }}</strong>
        </div>
        <div class="popover-row">
          <span class="row-label"><BaseIcon name="filter" size="xs" /><span>Index</span></span>
          <strong class="text-cyan">Token Bloom Filter + Sparse Index</strong>
        </div>
        <div class="popover-row">
          <span class="row-label"><BaseIcon name="zap" size="xs" /><span>Query Latency</span></span>
          <strong class="text-emerald tabular-nums">{{ status.latency_ms.toFixed(1) }} ms</strong>
        </div>
        <div class="popover-row">
          <span class="row-label"><BaseIcon name="calendar" size="xs" /><span>Retention Window</span></span>
          <strong class="text-amber tabular-nums">{{ status.retention_days }} Days</strong>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.engine-badge-container { position: relative; display: inline-flex; align-items: center; }
.engine-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  min-height: 28px;
  line-height: 28px;
  padding: 0 10px;
  border-radius: 9999px;
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
  font-family: inherit;
  border: 1px solid transparent;
  background: transparent;
  box-sizing: border-box;
}
.badge-text-compact { display: none; }
.badge-text-full { display: inline; }
.badge-connected { background: rgba(16, 185, 129, 0.1); border: 1px solid rgba(16, 185, 129, 0.3); color: #34d399; }
.badge-connected:hover { background: rgba(16, 185, 129, 0.18); border-color: rgba(16, 185, 129, 0.5); }
.badge-fallback { background: rgba(6, 182, 212, 0.1); border: 1px solid rgba(6, 182, 212, 0.3); color: #22d3ee; }
.badge-fallback:hover { background: rgba(6, 182, 212, 0.18); border-color: rgba(6, 182, 212, 0.5); }
.engine-badge-text { letter-spacing: -0.01em; white-space: nowrap; }
.pulse-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.pulse-dot-emerald { background: #10b981; box-shadow: 0 0 6px #10b981; }
.pulse-dot-cyan { background: #06b6d4; box-shadow: 0 0 6px #06b6d4; }
.engine-popover {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  width: 310px;
  max-width: calc(100vw - 24px);
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.85);
  border: 1px solid rgba(255, 255, 255, 0.12);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  z-index: 100;
  display: flex;
  flex-direction: column;
  gap: 6px;
  box-sizing: border-box;
}
@media (max-width: 1300px) {
  .badge-text-full { display: none !important; }
  .badge-text-compact { display: inline !important; }
}
@media (max-width: 1200px) {
  .engine-popover { left: auto; right: 0; }
}
.popover-header { display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid rgba(255, 255, 255, 0.08); padding-bottom: 6px; margin-bottom: 2px; }
.popover-title-group { display: inline-flex; align-items: center; gap: 6px; color: #94a3b8; }
.popover-title { font-size: 10px; font-weight: 700; letter-spacing: 0.05em; color: #94a3b8; }
.popover-tag { font-size: 9px; font-weight: 700; padding: 1px 5px; border-radius: 4px; }
.tag-emerald { background: rgba(16, 185, 129, 0.2); color: #34d399; }
.tag-amber { background: rgba(245, 158, 11, 0.2); color: #fbbf24; }
.popover-row { display: flex; align-items: center; justify-content: space-between; font-size: 11px; gap: 8px; color: #94a3b8; }
.row-label { display: inline-flex; align-items: center; gap: 5px; color: #94a3b8; white-space: nowrap; }
.status-pill { display: inline-flex; align-items: center; padding: 1px 6px; border-radius: 9999px; font-size: 10px; font-weight: 600; }
.pill-emerald { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.pill-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.tabular-nums { font-variant-numeric: tabular-nums; }
.popover-row strong { font-weight: 600; }
.text-slate { color: #f1f5f9; }
.text-cyan { color: #22d3ee; }
.text-emerald { color: #34d399; }
.text-amber { color: #fbbf24; }
.fade-enter-active, .fade-leave-active { transition: opacity 0.15s ease, transform 0.15s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; transform: translateY(-4px); }
</style>
