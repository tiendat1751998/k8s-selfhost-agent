<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import type { MetricAlert } from '../../../api/overview'
import type { MutedAlertConfig } from '../../../stores/alertStore'

interface Props {
  activeAlerts: MetricAlert[]
  mutedAlertsList: MutedAlertConfig[]
  hasCriticalAlerts: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'open-details'): void
  (e: 'mute-all', mode: 'restart' | '1h' | '24h'): void
  (e: 'dismiss-toast'): void
}>()

const DURATION_MS = 15000
const TICK_MS = 50
let timerId: ReturnType<typeof setInterval> | null = null

const isToastVisible = ref(false)
const isPaused = ref(false)
const progressPercent = ref(100)
const elapsedMs = ref(0)
const manuallyDismissedKeys = ref<string>('')

const sortedAlertKeys = computed(() => {
  return props.activeAlerts.map(a => `${a.node_id || a.node_name}-${a.type}`).sort().join(',')
})

function startTimer() {
  stopTimer()
  elapsedMs.value = 0
  progressPercent.value = 100
  isToastVisible.value = true

  timerId = setInterval(() => {
    if (!isPaused.value) {
      elapsedMs.value += TICK_MS
      progressPercent.value = Math.max(0, 100 - (elapsedMs.value / DURATION_MS) * 100)
      if (elapsedMs.value >= DURATION_MS) dismissToast()
    }
  }, TICK_MS)
}

function stopTimer() {
  if (timerId) {
    clearInterval(timerId)
    timerId = null
  }
}

function dismissToast() {
  stopTimer()
  isToastVisible.value = false
  manuallyDismissedKeys.value = sortedAlertKeys.value
  emit('dismiss-toast')
}

function onMouseEnter() { isPaused.value = true }
function onMouseLeave() { isPaused.value = false }
function handleMuteAll() { dismissToast(); emit('mute-all', 'restart') }
function handleOpenDetails() { emit('open-details') }

watch(sortedAlertKeys, (newKeys, oldKeys) => {
  if (newKeys && newKeys !== oldKeys && props.activeAlerts.length > 0) {
    if (newKeys !== manuallyDismissedKeys.value) startTimer()
  } else if (props.activeAlerts.length === 0) {
    stopTimer()
    isToastVisible.value = false
    manuallyDismissedKeys.value = ''
  }
}, { immediate: true })

onUnmounted(() => stopTimer())

const downNodeAlerts = computed(() => props.activeAlerts.filter(a => a.type === 'node_down'))
const hasNodeDown = computed(() => downNodeAlerts.value.length > 0)

const toastBadgeText = computed(() => {
  if (downNodeAlerts.value.length > 0) {
    return `${downNodeAlerts.value.length} Node${downNodeAlerts.value.length > 1 ? 's' : ''} Offline`
  }
  return `${props.activeAlerts.length} Alert${props.activeAlerts.length > 1 ? 's' : ''}`
})

const previewMessage = computed(() => {
  if (props.activeAlerts.length === 0) return ''
  const names = props.activeAlerts.map(a => a.node_name || a.node_id)
  const uniqueNames = Array.from(new Set(names))
  if (hasNodeDown.value) {
    const downNames = downNodeAlerts.value.map(a => a.node_name || a.node_id)
    return downNames.length <= 4
      ? `${downNames.join(', ')} unreachable`
      : `${downNames.slice(0, 3).join(', ')} +${downNames.length - 3} more unreachable`
  }
  return uniqueNames.length <= 3
    ? `${uniqueNames.join(', ')} threshold exceeded`
    : `${uniqueNames.slice(0, 3).join(', ')} +${uniqueNames.length - 3} nodes reporting alerts`
})
</script>

<template>
  <transition name="toast-slide">
    <div
      v-if="isToastVisible && activeAlerts.length > 0"
      class="floating-alert-toast glass-panel"
      :class="hasCriticalAlerts ? 'toast-critical' : 'toast-warning'"
      role="alert"
      aria-live="assertive"
      @mouseenter="onMouseEnter"
      @mouseleave="onMouseLeave"
    >
      <div class="toast-header">
        <div class="toast-title-group" @click="handleOpenDetails">
          <span class="toast-beacon" :class="hasCriticalAlerts ? 'beacon-rose' : 'beacon-amber'">
            <span class="beacon-pulse"></span>
            {{ hasCriticalAlerts ? '🚨' : '⚠️' }}
          </span>
          <div class="toast-title-text">
            <span class="toast-title">Cluster Health Warning</span>
            <span class="badge" :class="hasCriticalAlerts ? 'badge-rose' : 'badge-amber'">
              {{ toastBadgeText }}
            </span>
          </div>
        </div>
        <button class="btn-toast-close" @click.stop="dismissToast" title="Dismiss notification" aria-label="Close notification">✕</button>
      </div>

      <div class="toast-body" @click="handleOpenDetails">
        <p class="toast-preview-msg">{{ previewMessage }}</p>
        <span v-if="isPaused" class="toast-paused-badge">⏱️ Timer Paused</span>
      </div>

      <div class="toast-actions">
        <button class="btn-toast-action btn-toast-details" @click="handleOpenDetails" title="Open interactive Alert Center">
          <span>🔍 View Details</span>
        </button>
        <button class="btn-toast-action btn-toast-mute" @click="handleMuteAll" title="Silence all active node alerts until restart">
          <span>🔕 Mute All (Until Restart)</span>
        </button>
      </div>

      <div class="toast-progress-track">
        <div
          class="toast-progress-fill"
          :class="hasCriticalAlerts ? 'progress-rose' : 'progress-amber'"
          :style="{ width: `${progressPercent}%` }"
        ></div>
      </div>
    </div>
  </transition>

  <transition name="dock-fade">
    <div
      v-if="!isToastVisible && (activeAlerts.length > 0 || mutedAlertsList.length > 0)"
      class="docked-alert-badge glass-panel"
      :class="activeAlerts.length > 0 ? (hasCriticalAlerts ? 'docked-critical' : 'docked-warning') : 'docked-muted'"
      @click="handleOpenDetails"
      :title="activeAlerts.length > 0 ? `Click to inspect ${activeAlerts.length} active alert(s)` : `Click to manage ${mutedAlertsList.length} muted alert(s)`"
      role="button"
      tabindex="0"
      @keydown.enter="handleOpenDetails"
      @keydown.space.prevent="handleOpenDetails"
    >
      <span class="docked-icon">
        <span v-if="activeAlerts.length > 0" class="docked-pulse-dot" :class="hasCriticalAlerts ? 'pulse-rose' : 'pulse-amber'"></span>
        {{ activeAlerts.length > 0 ? (hasCriticalAlerts ? '🚨' : '⚠️') : '🔕' }}
      </span>
      <span class="docked-label">
        {{ activeAlerts.length > 0 ? `${activeAlerts.length} Alert${activeAlerts.length > 1 ? 's' : ''}` : `${mutedAlertsList.length} Muted` }}
      </span>
    </div>
  </transition>
</template>

<style scoped>
@import '../../../assets/styles/components/floating-alert-toast.css';
</style>
