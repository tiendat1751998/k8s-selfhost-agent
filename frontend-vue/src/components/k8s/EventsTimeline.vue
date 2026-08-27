<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { k8sApi, type K8sEvent } from '../../api/k8s'

const props = withDefaults(
  defineProps<{
    cluster: string
    namespace?: string
    kind?: string
    name?: string
    autoRefresh?: boolean
  }>(),
  {
    autoRefresh: false,
  }
)

const events = ref<K8sEvent[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const selectedTypeFilter = ref<'all' | 'Warning' | 'Normal'>('all')
const searchQuery = ref('')
const isAutoRefreshActive = ref(props.autoRefresh)
let refreshIntervalTimer: ReturnType<typeof setInterval> | null = null

async function loadEvents(silent = false) {
  if (!props.cluster) return
  if (!silent) loading.value = true
  error.value = null

  try {
    const list = await k8sApi.listEvents(props.cluster, {
      namespace: props.namespace && props.namespace !== 'all' ? props.namespace : undefined,
      kind: props.kind,
      name: props.name,
      limit: 150,
    })

    const sorted = (Array.isArray(list) ? list : []).sort((a, b) => {
      const timeA = new Date(a.lastTimestamp || a.eventTime || a.firstTimestamp || a.metadata?.creationTimestamp || 0).getTime()
      const timeB = new Date(b.lastTimestamp || b.eventTime || b.firstTimestamp || b.metadata?.creationTimestamp || 0).getTime()
      return timeB - timeA
    })

    events.value = sorted
  } catch (err: unknown) {
    events.value = []
    error.value = err instanceof Error ? err.message : 'Failed to query Kubernetes cluster events'
  } finally {
    if (!silent) loading.value = false
  }
}

function startAutoRefresh() {
  stopAutoRefresh()
  if (isAutoRefreshActive.value) {
    refreshIntervalTimer = setInterval(() => {
      loadEvents(true)
    }, 5000)
  }
}

function stopAutoRefresh() {
  if (refreshIntervalTimer) {
    clearInterval(refreshIntervalTimer)
    refreshIntervalTimer = null
  }
}

function toggleAutoRefresh() {
  isAutoRefreshActive.value = !isAutoRefreshActive.value
  if (isAutoRefreshActive.value) {
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
}

watch(
  () => [props.cluster, props.namespace, props.kind, props.name],
  () => {
    loadEvents()
  }
)

watch(
  () => props.autoRefresh,
  (val) => {
    isAutoRefreshActive.value = val
    if (val) startAutoRefresh()
    else stopAutoRefresh()
  }
)

onMounted(() => {
  loadEvents()
  if (isAutoRefreshActive.value) {
    startAutoRefresh()
  }
})

onUnmounted(() => {
  stopAutoRefresh()
})

const filteredEvents = computed(() => {
  return events.value.filter((ev) => {
    if (selectedTypeFilter.value !== 'all') {
      const evType = ev.type || 'Normal'
      if (evType !== selectedTypeFilter.value) return false
    }

    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      const reason = (ev.reason || '').toLowerCase()
      const message = (ev.message || '').toLowerCase()
      const objKind = (ev.involvedObject?.kind || '').toLowerCase()
      const objName = (ev.involvedObject?.name || '').toLowerCase()
      const component = (ev.source?.component || '').toLowerCase()

      const match =
        reason.includes(q) ||
        message.includes(q) ||
        objKind.includes(q) ||
        objName.includes(q) ||
        component.includes(q)

      if (!match) return false
    }

    return true
  })
})

const warningCount = computed(() => events.value.filter((e) => e.type === 'Warning').length)
const normalCount = computed(() => events.value.filter((e) => e.type !== 'Warning').length)

function formatTimeAgo(dateStr?: string): string {
  if (!dateStr) return 'Unknown'
  const diff = Date.now() - new Date(dateStr).getTime()
  if (isNaN(diff) || diff < 0) return 'Just now'
  const secs = Math.floor(diff / 1000)
  if (secs < 60) return secs + 's ago'
  const mins = Math.floor(secs / 60)
  if (mins < 60) return mins + 'm ago'
  const hours = Math.floor(mins / 60)
  if (hours < 24) return hours + 'h ago'
  const days = Math.floor(hours / 24)
  return days + 'd ago'
}

function getEventTime(ev: K8sEvent): string {
  return formatTimeAgo(ev.lastTimestamp || ev.eventTime || ev.firstTimestamp || ev.metadata?.creationTimestamp)
}
</script>

<template>
  <div class="events-timeline">
    <!-- Header & Controls Bar -->
    <div class="timeline-toolbar glass-panel">
      <div class="toolbar-left">
        <!-- Type Filter Chips -->
        <div class="filter-chips">
          <button
            type="button"
            class="chip-btn"
            :class="{ 'is-active': selectedTypeFilter === 'all' }"
            @click="selectedTypeFilter = 'all'"
          >
            All ({{ events.length }})
          </button>
          <button
            type="button"
            class="chip-btn chip-warning"
            :class="{ 'is-active': selectedTypeFilter === 'Warning' }"
            @click="selectedTypeFilter = 'Warning'"
          >
            ⚠️ Warnings ({{ warningCount }})
          </button>
          <button
            type="button"
            class="chip-btn chip-normal"
            :class="{ 'is-active': selectedTypeFilter === 'Normal' }"
            @click="selectedTypeFilter = 'Normal'"
          >
            ℹ️ Normal ({{ normalCount }})
          </button>
        </div>

        <!-- Search Input -->
        <div class="search-box">
          <span class="search-icon">🔍</span>
          <input
            v-model="searchQuery"
            type="text"
            class="input-glass font-mono search-input"
            placeholder="Search reason, object, message..."
          />
          <button
            v-if="searchQuery"
            type="button"
            class="clear-btn"
            title="Clear search"
            @click="searchQuery = ''"
          >
            ✕
          </button>
        </div>
      </div>

      <div class="toolbar-right">
        <!-- Auto Refresh Toggle -->
        <button
          type="button"
          class="btn-toggle-auto font-mono"
          :class="{ 'is-active': isAutoRefreshActive }"
          @click="toggleAutoRefresh"
          title="Toggle live 5s polling"
        >
          <span class="pulse-dot" :class="isAutoRefreshActive ? 'pulse-dot-emerald' : 'pulse-dot-muted'"></span>
          <span>Live 5s</span>
        </button>

        <!-- Refresh Button -->
        <button
          type="button"
          class="btn btn-secondary btn-xs btn-refresh"
          :disabled="loading"
          @click="loadEvents()"
        >
          <span :class="{ 'spin-icon': loading }">🔄</span>
          <span>{{ loading ? 'Updating...' : 'Refresh' }}</span>
        </button>
      </div>
    </div>

    <!-- Error Banner -->
    <div v-if="error" class="events-error-banner glass-panel animate-fade-in">
      <span class="err-icon">⚠️</span>
      <div class="err-text">
        <strong>Error loading events:</strong>
        <span class="font-mono font-small">{{ error }}</span>
      </div>
      <button type="button" class="btn btn-secondary btn-xs" @click="loadEvents()">
        Retry
      </button>
    </div>

    <!-- Loading Skeleton -->
    <div v-if="loading && events.length === 0" class="timeline-loading">
      <div v-for="n in 4" :key="n" class="skeleton-event-item glass-panel animate-pulse">
        <div class="sk-line sk-line-short"></div>
        <div class="sk-line sk-line-long"></div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else-if="filteredEvents.length === 0" class="events-empty glass-panel">
      <div class="empty-icon">📢</div>
      <div class="empty-title">No Kubernetes Events Found</div>
      <div class="empty-desc text-muted">
        {{
          searchQuery
            ? 'No events match the current search filter.'
            : selectedTypeFilter !== 'all'
            ? 'No ' + selectedTypeFilter + ' events recorded in this scope.'
            : 'No Kubernetes events recorded for this scope.'
        }}
      </div>
    </div>

    <!-- Timeline Event List -->
    <div v-else class="timeline-list">
      <div 
        v-for="(ev, idx) in filteredEvents"
        :key="ev.metadata?.uid || ((ev.metadata?.name || 'ev') + '-' + idx)"
        class="event-card glass-panel"
        :class="'event-type-' + (ev.type || 'Normal').toLowerCase()"
      >
        <!-- Timeline Marker Dot -->
        <div class="event-marker">
          <span 
            class="marker-dot"
            :class="ev.type === 'Warning' ? 'marker-warning' : 'marker-normal'"
          ></span>
          <span v-if="idx !== filteredEvents.length - 1" class="marker-line"></span>
        </div>

        <!-- Event Content -->
        <div class="event-body">
          <!-- Top Row: Type Badge, Reason, Count, Age -->
          <div class="event-header-row">
            <div class="header-badges">
              <span 
                class="event-type-badge font-mono"
                :class="ev.type === 'Warning' ? 'type-warning' : 'type-normal'"
              >
                {{ ev.type === 'Warning' ? '⚠️ Warning' : 'ℹ️ Normal' }}
              </span>

              <span class="event-reason font-mono font-bold">
                {{ ev.reason || 'Event' }}
              </span>

              <span 
                v-if="ev.count && ev.count > 1"
                class="event-count-badge font-mono"
                :title="'Occurred ' + ev.count + ' times'"
              >
                x{{ ev.count }}
              </span>
            </div>

            <div class="event-meta-right">
              <span class="event-age font-mono" :title="ev.lastTimestamp || ev.eventTime || ev.firstTimestamp || ev.metadata?.creationTimestamp">
                🕒 {{ getEventTime(ev) }}
              </span>
            </div>
          </div>

          <!-- Involved Object Row -->
          <div v-if="ev.involvedObject" class="event-object-row font-mono">
            <span class="obj-label text-muted">Object:</span>
            <span class="obj-kind text-cyan">{{ ev.involvedObject.kind || 'Resource' }}</span>
            <span class="obj-sep text-muted">/</span>
            <span class="obj-name text-white">{{ ev.involvedObject.name || 'unknown' }}</span>
            <span v-if="ev.involvedObject.namespace" class="obj-ns text-muted">
              (ns: {{ ev.involvedObject.namespace }})
            </span>
          </div>

          <!-- Message Row -->
          <div class="event-message font-mono">
            {{ ev.message || 'No additional message detail.' }}
          </div>

          <!-- Footer details: Source Component / Sub-object -->
          <div v-if="ev.source?.component || ev.source?.host || ev.involvedObject?.fieldPath" class="event-footer-row font-mono text-muted font-small">
            <span v-if="ev.source?.component">
              Source: <span class="text-cyan">{{ ev.source.component }}</span>
            </span>
            <span v-if="ev.source?.host" class="footer-item">
              Host: <span class="text-muted">{{ ev.source.host }}</span>
            </span>
            <span v-if="ev.involvedObject?.fieldPath" class="footer-item">
              Field: <span class="text-amber">+{{ ev.involvedObject.fieldPath }}</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.events-timeline {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
  box-sizing: border-box;
}

.timeline-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(0, 229, 255, 0.15);
}
.toolbar-left {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  flex: 1;
  min-width: 260px;
}
.filter-chips {
  display: flex;
  align-items: center;
  gap: 6px;
}
.chip-btn {
  padding: 5px 10px;
  font-size: 0.78rem;
  font-family: inherit;
  font-weight: 600;
  border-radius: 6px;
  cursor: pointer;
  background: rgba(30, 41, 59, 0.7);
  color: #94a3c8;
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.15s ease-in-out;
  white-space: nowrap;
}
.chip-btn:hover {
  background: rgba(51, 65, 85, 0.8);
  color: #f1f5f9;
}
.chip-btn.is-active {
  background: rgba(0, 229, 255, 0.15);
  color: #00e5ff;
  border-color: rgba(0, 229, 255, 0.4);
}
.chip-warning.is-active {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
  border-color: rgba(245, 158, 11, 0.4);
}
.chip-normal.is-active {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
  border-color: rgba(16, 185, 129, 0.4);
}
.search-box {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 180px;
  max-width: 320px;
}
.search-icon {
  position: absolute;
  left: 10px;
  font-size: 0.75rem;
  opacity: 0.6;
  pointer-events: none;
}
.search-input {
  width: 100%;
  padding: 6px 28px 6px 30px;
  font-size: 0.8rem;
  border-radius: 6px;
  height: 32px;
}
.clear-btn {
  position: absolute;
  right: 8px;
  background: none;
  border: none;
  color: #94a3c8;
  cursor: pointer;
  font-size: 0.75rem;
  padding: 2px 4px;
}
.clear-btn:hover {
  color: #f1f5f9;
}
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
}
.btn-toggle-auto {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  font-size: 0.75rem;
  border-radius: 6px;
  background: rgba(30, 41, 59, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #94a3b8;
  cursor: pointer;
  transition: all 0.15s ease-in-out;
}
.btn-toggle-auto.is-active {
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.4);
  color: #10b981;
}	
.pulse-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  display: inline-block;
}
.pulse-dot-emerald {
  background: #10b981;
  box-shadow: 0 0 8px #10b981;
  animation: pulse-glow 2s infinite;
}
.pulse-dot-muted {
  background: #64748b;
}
@keyframes pulse-glow {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.85); }
}
.btn-refresh {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
}
.spin-icon {
  animation: spin 1s linear infinite;
  display: inline-block;
}
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
.events-error-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-radius: 8px;
  background: rgba(244, 63, 94, 0.1);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
}
.err-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}
.timeline-loading {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.skeleton-event-item {
  padding: 16px;
  border-radius: 8px;
  background: rgba(30, 41, 59, 0.4);
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.sk-line {
  height: 14px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 4px;
}
.sk-line-short { width: 35%; }
.sk-line-long { width: 85%; }
.events-empty {
  padding: 40px 20px;
  text-align: center;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.4);
  border: 1px dashed rgba(255, 255, 255, 0.1);
}
.empty-icon {
  font-size: 2rem;
  margin-bottom: 8px;
}
.empty-title {
  font-size: 1rem;
  font-weight: 600;
  color: #f1f5f9;
  margin-bottom: 4px;
}
.empty-desc {
  font-size: 0.85rem;
}
.timeline-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  position: relative;
}
.event-card {
  display: flex;
  gap: 14px;
  padding: 14px 16px;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.07);
  transition: all 0.15s ease-in-out;
}
.event-card:hover {
  background: rgba(15, 23, 42, 0.85);
  border-color: rgba(0, 229, 255, 0.25);
  transform: translateX(2px);
}
.event-card.event-type-warning {
  border-left: 3px solid #f59e0b;
  background: rgba(245, 158, 11, 0.04);
}
.event-card.event-type-warning:hover {
  border-left-color: #f59e0b;
  border-color: rgba(245, 158, 11, 0.3);
}
.event-card.event-type-normal {
  border-left: 3px solid #00e5ff;
}
.event-marker {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 4px;
}
.marker-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex-shrink: 0;
}
.marker-warning {
  background: #f59e0b;
  box-shadow: 0 0 6px #f59e0b;
}
.marker-normal {
  background: #00e5ff;
  border-color: rgba(0, 229, 255, 0.3);
  box-shadow: 0 0 6px #00e5ff;
}
.marker-line {
  width: 1px;
  flex: 1;
  background: rgba(255, 255, 255, 0.1);
  margin-top: 4px;
  min-height: 24px;
}
.event-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
  min-width: 0;
}
.event-header-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.header-badges {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.event-type-badge {
  font-size: 0.72rem;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: 4px;
  text-transform: uppercase;
}
.type-warning {
  background: rgba(245, 158, 11, 0.18);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.35);
}
.type-normal {
  background: rgba(0, 229, 255, 0.12);
  color: #00e5ff;
  border: 1px solid rgba(0, 229, 255, 0.3);
}
.event-reason {
  font-size: 0.88rem;
  color: #f8fafc;
}
.event-count-badge {
  font-size: 0.7rem;
  padding: 1px 6px;
  border-radius: 10px;
  background: rgba(245, 158, 11, 0.2);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.4);
  font-weight: 700;
}
.event-meta-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.event-age {
  font-size: 0.75rem;
  color: #94a3b8;
  white-space: nowrap;
}
.event-object-row {
  font-size: 0.8rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px;
}
.obj-kind {
  font-weight: 600;
}
.obj-name {
  font-weight: 600;
}
.obj-ns {
  font-size: 0.75rem;
}
.event-message {
  font-size: 0.82rem;
  line-height: 1.45;
  color: #cbd5e1;
  background: rgba(0, 0, 0, 0.25);
  padding: 6px 10px;
  border-radius: 5px;
  border-left: 2px solid rgba(255, 255, 255, 0.1);
  word-break: break-word;
}
.event-footer-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 2px;
}
.footer-item {
  position: relative;
}
.footer-item::before {
  content: "•";
  margin-right: 8px;
  color: #64748b;
}
@media (max-width: 640px) {
  .timeline-toolbar {
    flex-direction: column;
    align-items: stretch;
  }
  .toolbar-left, .toolbar-right {
    width: 100%;
    justify-content: space-between;
  }
  .search-box {
    max-width: 100%;
  }
  .event-header-row {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>