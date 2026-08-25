<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useAlertStore } from '../../stores/alertStore'

const alertStore = useAlertStore()

// 15 seconds auto-dismiss countdown for the header-anchored dropdown toast
const DURATION_MS = 15000
const TICK_MS = 50
let timerId: ReturnType<typeof setInterval> | null = null

const isPaused = ref(false)
const progressPercent = ref(100)
const elapsedMs = ref(0)

const containerRef = ref<HTMLElement | null>(null)

function startTimer() {
  stopTimer()
  elapsedMs.value = 0
  progressPercent.value = 100

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
  alertStore.dismissToast()
}

function handleOpenDetails() {
  dismissToast()
  alertStore.openAlertCenter()
}

function handleMuteAll() {
  dismissToast()
  alertStore.muteAll('restart')
}

// Watch for toast drop trigger from alertStore
watch(
  () => alertStore.isToastDropped,
  (dropped) => {
    if (dropped && alertStore.activeAlerts.length > 0) {
      startTimer()
    } else {
      stopTimer()
    }
  },
  { immediate: true }
)

// Computeds for alert badge and preview message
const downNodeAlerts = computed(() => alertStore.activeAlerts.filter(a => a.type === 'node_down'))
const hasNodeDown = computed(() => downNodeAlerts.value.length > 0)

const toastBadgeText = computed(() => {
  if (downNodeAlerts.value.length > 0) {
    return `${downNodeAlerts.value.length} Node${downNodeAlerts.value.length > 1 ? 's' : ''} Offline`
  }
  return `${alertStore.activeAlerts.length} Alert${alertStore.activeAlerts.length > 1 ? 's' : ''}`
})

const previewMessage = computed(() => {
  if (alertStore.activeAlerts.length === 0) return ''
  const names = alertStore.activeAlerts.map(a => a.node_name || a.node_id)
  const uniqueNames = Array.from(new Set(names))

  if (hasNodeDown.value) {
    const downNames = downNodeAlerts.value.map(a => a.node_name || a.node_id)
    if (downNames.length <= 3) {
      return `${downNames.join(', ')} unreachable`
    }
    return `${downNames.slice(0, 2).join(', ')} +${downNames.length - 2} more unreachable`
  }

  if (uniqueNames.length <= 2) {
    return `${uniqueNames.join(', ')} threshold exceeded`
  }
  return `${uniqueNames.slice(0, 2).join(', ')} +${uniqueNames.length - 2} nodes reporting alerts`
})

function handleDocumentClick(e: MouseEvent) {
  if (!alertStore.isToastDropped) return
  if (containerRef.value && !containerRef.value.contains(e.target as Node)) {
    dismissToast()
  }
}

onMounted(() => {
  if (typeof document !== 'undefined') {
    document.addEventListener('click', handleDocumentClick)
  }
})

onUnmounted(() => {
  if (typeof document !== 'undefined') {
    document.removeEventListener('click', handleDocumentClick)
  }
  stopTimer()
})
</script>

<template>
  <div ref="containerRef" class="top-hud-alert-container">
    <!-- 1. TOP HUD BELL BUTTON / CAPSULE -->
    <button
      class="hud-bell-pill"
      :class="{
        'pill-critical': alertStore.activeAlerts.length > 0 && alertStore.hasCriticalAlerts,
        'pill-warning': alertStore.activeAlerts.length > 0 && !alertStore.hasCriticalAlerts,
        'pill-muted': alertStore.activeAlerts.length === 0 && alertStore.mutedAlertsCount > 0,
        'pill-clean': alertStore.activeAlerts.length === 0 && alertStore.mutedAlertsCount === 0,
      }"
      :title="
        alertStore.activeAlerts.length > 0
          ? `Cluster Alerts: ${alertStore.activeAlerts.length} active issue(s). Click to inspect Alert Center.`
          : alertStore.mutedAlertsCount > 0
            ? `Alert Center: ${alertStore.mutedAlertsCount} rule(s) currently silenced. Click to manage.`
            : 'Cluster Alerts: All green (0 active alerts). Click to open Alert Center.'
      "
      aria-label="Cluster Alert Center"
      @click="alertStore.openAlertCenter"
    >
      <!-- Active Alerts Mode -->
      <template v-if="alertStore.activeAlerts.length > 0">
        <span class="bell-icon-wrap">
          <span class="pulse-dot" :class="alertStore.hasCriticalAlerts ? 'pulse-dot-rose' : 'pulse-dot-amber'"></span>
          <span class="bell-emoji">{{ alertStore.hasCriticalAlerts ? '🚨' : '⚠️' }}</span>
        </span>
        <span class="bell-count-text">
          {{ alertStore.activeAlerts.length }} Alert{{ alertStore.activeAlerts.length > 1 ? 's' : '' }}
        </span>
      </template>

      <!-- Muted Only Mode -->
      <template v-else-if="alertStore.mutedAlertsCount > 0">
        <span class="bell-icon-wrap">
          <span class="bell-emoji">🔕</span>
        </span>
        <span class="bell-count-text">
          {{ alertStore.mutedAlertsCount }} Muted
        </span>
      </template>

      <!-- Clean Zero Alerts Mode -->
      <template v-else>
        <span class="bell-icon-wrap">
          <span class="bell-emoji">🔔</span>
        </span>
        <span class="bell-count-text">0</span>
      </template>
    </button>

    <!-- 2. HEADER-ANCHORED DROPDOWN ALERT TOAST -->
    <transition name="dropdown-toast">
      <div
        v-if="alertStore.isToastDropped && alertStore.activeAlerts.length > 0"
        class="header-alert-toast glass-panel"
        :class="alertStore.hasCriticalAlerts ? 'toast-critical' : 'toast-warning'"
        role="alert"
        aria-live="assertive"
        @mouseenter="isPaused = true"
        @mouseleave="isPaused = false"
      >
        <!-- Top Row: Icon, Title, Badge, Close -->
        <div class="toast-header">
          <div class="toast-title-group" @click="handleOpenDetails">
            <span class="toast-beacon" :class="alertStore.hasCriticalAlerts ? 'beacon-rose' : 'beacon-amber'">
              <span class="beacon-pulse"></span>
              {{ alertStore.hasCriticalAlerts ? '🚨' : '⚠️' }}
            </span>
            <div class="toast-title-text">
              <span class="toast-title">Cluster Health Warning</span>
              <span class="badge" :class="alertStore.hasCriticalAlerts ? 'badge-rose' : 'badge-amber'">
                {{ toastBadgeText }}
              </span>
            </div>
          </div>

          <button
            class="btn-toast-close"
            title="Dismiss notification (Docks into bell icon)"
            aria-label="Close notification"
            @click.stop="dismissToast"
          >
            ✕
          </button>
        </div>

        <!-- Message Preview -->
        <div class="toast-body" @click="handleOpenDetails">
          <p class="toast-preview-msg font-mono">
            {{ previewMessage }}
          </p>
          <span v-if="isPaused" class="toast-paused-badge">⏱️ Timer Paused</span>
        </div>

        <!-- Action Buttons -->
        <div class="toast-actions">
          <button
            class="btn-toast-action btn-toast-details"
            title="Open interactive Alert Center"
            @click="handleOpenDetails"
          >
            <span>🔍 View Details</span>
          </button>
          <button
            class="btn-toast-action btn-toast-mute"
            title="Silence all active node alerts until server restart"
            @click="handleMuteAll"
          >
            <span>🔕 Mute All</span>
          </button>
        </div>

        <!-- Auto-Dismiss Animated Progress Bar -->
        <div class="toast-progress-track">
          <div
            class="toast-progress-fill"
            :class="alertStore.hasCriticalAlerts ? 'progress-rose' : 'progress-amber'"
            :style="{ width: `${progressPercent}%` }"
          ></div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.top-hud-alert-container {
  position: relative;
  display: inline-flex;
  align-items: center;
}

/* ==========================================
   HUD BELL CAPSULE BUTTON
   ========================================== */
.hud-bell-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 12px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.12));
  color: var(--text-secondary, #94a3b8);
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  user-select: none;
  line-height: 1;
}

.hud-bell-pill:hover {
  transform: translateY(-1px);
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.hud-bell-pill:active {
  transform: translateY(0);
}

.bell-icon-wrap {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
}

.bell-emoji {
  display: inline-block;
  line-height: 1;
}

.bell-count-text {
  letter-spacing: 0.02em;
  font-family: var(--font-sans, inherit);
}

/* Critical Active Pill */
.pill-critical {
  background: rgba(244, 63, 94, 0.15);
  border-color: rgba(244, 63, 94, 0.55);
  color: #fb7185;
  box-shadow: 0 0 14px rgba(244, 63, 94, 0.3), inset 0 0 8px rgba(244, 63, 94, 0.15);
  animation: critical-pill-pulse 2.2s infinite;
}

.pill-critical:hover {
  background: rgba(244, 63, 94, 0.25);
  border-color: rgba(244, 63, 94, 0.75);
  color: #fff;
}

/* Warning Active Pill */
.pill-warning {
  background: rgba(245, 158, 11, 0.15);
  border-color: rgba(245, 158, 11, 0.55);
  color: #fbbf24;
  box-shadow: 0 0 14px rgba(245, 158, 11, 0.3), inset 0 0 8px rgba(245, 158, 11, 0.15);
}

.pill-warning:hover {
  background: rgba(245, 158, 11, 0.25);
  border-color: rgba(245, 158, 11, 0.75);
  color: #fff;
}

/* Muted Only Pill */
.pill-muted {
  background: rgba(15, 23, 42, 0.75);
  border: 1px dashed rgba(148, 163, 184, 0.45);
  color: #cbd5e1;
}

.pill-muted:hover {
  border-color: rgba(148, 163, 184, 0.75);
  color: #fff;
}

/* Clean Pill */
.pill-clean {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.08);
  color: #64748b;
}

.pill-clean:hover {
  border-color: rgba(6, 182, 212, 0.4);
  color: #38bdf8;
  background: rgba(6, 182, 212, 0.06);
}

@keyframes critical-pill-pulse {
  0% {
    box-shadow: 0 0 12px rgba(244, 63, 94, 0.25);
  }
  50% {
    box-shadow: 0 0 20px rgba(244, 63, 94, 0.5);
  }
  100% {
    box-shadow: 0 0 12px rgba(244, 63, 94, 0.25);
  }
}

/* ==========================================
   HEADER-ANCHORED DROPDOWN ALERT TOAST
   ========================================== */
.header-alert-toast {
  position: absolute;
  top: calc(100% + 10px);
  right: 0;
  z-index: 1000;
  width: 380px;
  max-width: calc(100vw - 32px);
  background: rgba(15, 23, 42, 0.96);
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
  border-radius: 12px;
  padding: 14px 16px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  cursor: pointer;
  overflow: hidden;
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.75), 0 0 1px rgba(255, 255, 255, 0.15);
}

.toast-critical {
  border: 1px solid rgba(244, 63, 94, 0.5);
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.75), 0 0 22px rgba(244, 63, 94, 0.25);
}

.toast-warning {
  border: 1px solid rgba(245, 158, 11, 0.5);
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.75), 0 0 22px rgba(245, 158, 11, 0.25);
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

/* Transitions */
.dropdown-toast-enter-active {
  transition: all 0.28s cubic-bezier(0.16, 1, 0.3, 1);
}

.dropdown-toast-leave-active {
  transition: all 0.22s cubic-bezier(0.4, 0, 0.2, 1);
}

.dropdown-toast-enter-from {
  opacity: 0;
  transform: translateY(-10px) scale(0.96);
}

.dropdown-toast-leave-to {
  opacity: 0;
  transform: translateY(-8px) scale(0.96);
}

@media (max-width: 640px) {
  .header-alert-toast {
    position: fixed;
    top: 65px;
    right: 12px;
    left: 12px;
    width: auto;
    max-width: none;
  }
}
</style>
