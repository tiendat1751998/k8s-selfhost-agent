<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import type { MetricAlert } from '../../../api/overview'
import type { MutedAlertConfig } from '../../../views/OverviewView.vue'

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

// 15 seconds auto-dismiss countdown
const DURATION_MS = 15000
const TICK_MS = 50
let timerId: ReturnType<typeof setInterval> | null = null

const isToastVisible = ref(false)
const isPaused = ref(false)
const progressPercent = ref(100)
const elapsedMs = ref(0)
const manuallyDismissedKeys = ref<string>('')

const sortedAlertKeys = computed(() => {
  return props.activeAlerts
    .map(a => `${a.node_id || a.node_name}-${a.type}`)
    .sort()
    .join(',')
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
      if (elapsedMs.value >= DURATION_MS) {
        dismissToast()
      }
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

function onMouseEnter() {
  isPaused.value = true
}

function onMouseLeave() {
  isPaused.value = false
}

function handleMuteAll() {
  dismissToast()
  emit('mute-all', 'restart')
}

function handleOpenDetails() {
  emit('open-details')
}

// Watch for incoming or changing active alerts
watch(
  sortedAlertKeys,
  (newKeys, oldKeys) => {
    if (newKeys && newKeys !== oldKeys && props.activeAlerts.length > 0) {
      if (newKeys !== manuallyDismissedKeys.value) {
        startTimer()
      }
    } else if (props.activeAlerts.length === 0) {
      stopTimer()
      isToastVisible.value = false
      manuallyDismissedKeys.value = ''
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  stopTimer()
})

// Computeds for alert badge and preview message
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
    if (downNames.length <= 4) {
      return `${downNames.join(', ')} unreachable`
    }
    return `${downNames.slice(0, 3).join(', ')} +${downNames.length - 3} more unreachable`
  }

  if (uniqueNames.length <= 3) {
    return `${uniqueNames.join(', ')} threshold exceeded`
  }
  return `${uniqueNames.slice(0, 3).join(', ')} +${uniqueNames.length - 3} nodes reporting alerts`
})
</script>

<template>
  <!-- TOP-RIGHT FLOATING CYBER TOAST NOTIFICATION -->
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
      <!-- Top Row: Icon, Title, Badge, Close -->
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

        <button
          class="btn-toast-close"
          @click.stop="dismissToast"
          title="Dismiss notification (Docks into bell icon)"
          aria-label="Close notification"
        >
          ✕
        </button>
      </div>

      <!-- Message Preview -->
      <div class="toast-body" @click="handleOpenDetails">
        <p class="toast-preview-msg">
          {{ previewMessage }}
        </p>
        <span v-if="isPaused" class="toast-paused-badge">⏱️ Timer Paused</span>
      </div>

      <!-- Action Buttons -->
      <div class="toast-actions">
        <button
          class="btn-toast-action btn-toast-details"
          @click="handleOpenDetails"
          title="Open interactive Alert Center"
        >
          <span>🔍 View Details</span>
        </button>
        <button
          class="btn-toast-action btn-toast-mute"
          @click="handleMuteAll"
          title="Silence all active node alerts until server restart"
        >
          <span>🔕 Mute All (Until Restart)</span>
        </button>
      </div>

      <!-- Auto-Dismiss Animated Progress Bar -->
      <div class="toast-progress-track">
        <div
          class="toast-progress-fill"
          :class="hasCriticalAlerts ? 'progress-rose' : 'progress-amber'"
          :style="{ width: `${progressPercent}%` }"
        ></div>
      </div>
    </div>
  </transition>

  <!-- TOP-RIGHT DOCKED NOTIFICATION BELL BADGE (When toast is hidden but alerts/muted exist) -->
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
/* ==========================================
   FLOATING CYBER TOAST NOTIFICATION
   ========================================== */
.floating-alert-toast {
  position: fixed;
  top: 75px;
  right: 24px;
  z-index: 99999;
  width: 420px;
  max-width: calc(100vw - 48px);
  background: rgba(15, 23, 42, 0.95);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-radius: 12px;
  padding: 14px 16px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  cursor: pointer;
  overflow: hidden;
  transition: transform 0.25s ease, box-shadow 0.25s ease;
}

.floating-alert-toast:hover {
  transform: translateY(-2px);
}

.toast-critical {
  border: 1px solid rgba(244, 63, 94, 0.45);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.65), 0 0 22px rgba(244, 63, 94, 0.25);
}

.toast-warning {
  border: 1px solid rgba(245, 158, 11, 0.45);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.65), 0 0 22px rgba(245, 158, 11, 0.25);
}

/* Header */
.toast-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.toast-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.toast-beacon {
  position: relative;
  font-size: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.beacon-pulse {
  position: absolute;
  inset: -4px;
  border-radius: 50%;
  animation: beacon-glow 1.8s infinite;
  opacity: 0.7;
}

.beacon-rose .beacon-pulse {
  background: rgba(244, 63, 94, 0.35);
}

.beacon-amber .beacon-pulse {
  background: rgba(245, 158, 11, 0.35);
}

@keyframes beacon-glow {
  0% {
    transform: scale(0.85);
    opacity: 0.8;
  }
  50% {
    transform: scale(1.3);
    opacity: 0.2;
  }
  100% {
    transform: scale(0.85);
    opacity: 0.8;
  }
}

.toast-title-text {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.toast-title {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
  letter-spacing: 0.02em;
}

.badge-rose {
  background: rgba(244, 63, 94, 0.2);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.4);
  font-size: 10.5px;
  font-weight: 700;
  padding: 1px 7px;
  border-radius: 4px;
}

.badge-amber {
  background: rgba(245, 158, 11, 0.2);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.4);
  font-size: 10.5px;
  font-weight: 700;
  padding: 1px 7px;
  border-radius: 4px;
}

.btn-toast-close {
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 14px;
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 4px;
  transition: all 0.2s ease;
  line-height: 1;
}

.btn-toast-close:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.12);
}

/* Body */
.toast-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.toast-preview-msg {
  font-size: 12px;
  color: #cbd5e1;
  margin: 0;
  line-height: 1.4;
  font-family: var(--font-mono, monospace);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.toast-paused-badge {
  font-size: 10px;
  font-weight: 600;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.15);
  padding: 1px 6px;
  border-radius: 4px;
  flex-shrink: 0;
}

/* Actions */
.toast-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-toast-action {
  font-size: 11px;
  font-weight: 600;
  padding: 5px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.btn-toast-details {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #f8fafc;
}

.btn-toast-details:hover {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.35);
  color: #fff;
}

.btn-toast-mute {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.4);
  color: #fecdd3;
}

.btn-toast-mute:hover {
  background: rgba(244, 63, 94, 0.3);
  border-color: rgba(244, 63, 94, 0.6);
  color: #fff;
}

/* Progress bar */
.toast-progress-track {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: rgba(255, 255, 255, 0.08);
}

.toast-progress-fill {
  height: 100%;
  transition: width 0.05s linear;
}

.progress-rose {
  background: linear-gradient(90deg, #f43f5e, #fb7185);
  box-shadow: 0 0 6px rgba(244, 63, 94, 0.6);
}

.progress-amber {
  background: linear-gradient(90deg, #f59e0b, #fbbf24);
  box-shadow: 0 0 6px rgba(245, 158, 11, 0.6);
}

/* Toast Transitions */
.toast-slide-enter-active {
  transition: all 0.35s cubic-bezier(0.16, 1, 0.3, 1);
}

.toast-slide-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.toast-slide-enter-from {
  opacity: 0;
  transform: translateX(100px) scale(0.95);
}

.toast-slide-leave-to {
  opacity: 0;
  transform: translateX(120px) scale(0.95);
}

/* ==========================================
   TOP-RIGHT DOCKED NOTIFICATION BELL BADGE
   ========================================== */
.docked-alert-badge {
  position: fixed;
  top: 75px;
  right: 24px;
  z-index: 99990;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 9999px;
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  cursor: pointer;
  font-size: 12px;
  font-weight: 700;
  transition: all 0.25s ease;
  user-select: none;
}

.docked-alert-badge:hover {
  transform: translateY(-2px) scale(1.03);
}

.docked-critical {
  background: rgba(15, 23, 42, 0.9);
  border: 1px solid rgba(244, 63, 94, 0.6);
  color: #fb7185;
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.5), 0 0 16px rgba(244, 63, 94, 0.35);
}

.docked-warning {
  background: rgba(15, 23, 42, 0.9);
  border: 1px solid rgba(245, 158, 11, 0.6);
  color: #fbbf24;
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.5), 0 0 16px rgba(245, 158, 11, 0.35);
}

.docked-muted {
  background: rgba(15, 23, 42, 0.85);
  border: 1px dashed rgba(148, 163, 184, 0.4);
  color: #cbd5e1;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.4);
}

.docked-muted:hover {
  border-color: rgba(148, 163, 184, 0.7);
  color: #fff;
}

.docked-icon {
  position: relative;
  font-size: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.docked-pulse-dot {
  position: absolute;
  top: -2px;
  right: -2px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  animation: pulse-dot-anim 1.5s infinite;
}

.pulse-rose {
  background: #f43f5e;
  box-shadow: 0 0 6px #f43f5e;
}

.pulse-amber {
  background: #f59e0b;
  box-shadow: 0 0 6px #f59e0b;
}

@keyframes pulse-dot-anim {
  0% {
    transform: scale(0.9);
    opacity: 1;
  }
  50% {
    transform: scale(1.6);
    opacity: 0.3;
  }
  100% {
    transform: scale(0.9);
    opacity: 1;
  }
}

.docked-label {
  letter-spacing: 0.02em;
}

.dock-fade-enter-active,
.dock-fade-leave-active {
  transition: all 0.25s ease;
}

.dock-fade-enter-from,
.dock-fade-leave-to {
  opacity: 0;
  transform: scale(0.9);
}

@media (max-width: 640px) {
  .floating-alert-toast {
    right: 12px;
    left: 12px;
    width: auto;
    max-width: none;
    top: 70px;
  }

  .docked-alert-badge {
    right: 12px;
    top: 70px;
  }
}
</style>
