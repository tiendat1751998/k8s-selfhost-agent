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

const firstDownNode = computed(() => downNodeAlerts.value[0]?.node_name || downNodeAlerts.value[0]?.node_id || '')

function handleQuickFailover() {
  dismissToast()
  if (firstDownNode.value) {
    alertStore.openRemediation(firstDownNode.value)
    alertStore.openAlertCenter()
  } else {
    alertStore.openAlertCenter()
  }
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

function handleMouseEnter() {
  if (typeof window !== 'undefined' && window.matchMedia('(hover: hover)').matches) {
    isPaused.value = true
  }
}

function handleMouseLeave() {
  isPaused.value = false
}

function handleDocumentInteraction(e: MouseEvent | TouchEvent) {
  if (!alertStore.isToastDropped) return
  const target = e.target as Node | null
  if (containerRef.value && target && !containerRef.value.contains(target)) {
    dismissToast()
  }
}

onMounted(() => {
  if (typeof document !== 'undefined') {
    document.addEventListener('click', handleDocumentInteraction)
    document.addEventListener('touchend', handleDocumentInteraction, { passive: true })
  }
})

onUnmounted(() => {
  if (typeof document !== 'undefined') {
    document.removeEventListener('click', handleDocumentInteraction)
    document.removeEventListener('touchend', handleDocumentInteraction)
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
          <span class="bell-mobile-badge" :class="alertStore.hasCriticalAlerts ? 'badge-rose-bg' : 'badge-amber-bg'">
            {{ alertStore.activeAlerts.length }}
          </span>
        </span>
        <span class="bell-count-text">
          {{ alertStore.activeAlerts.length }} Alert{{ alertStore.activeAlerts.length > 1 ? 's' : '' }}
        </span>
      </template>

      <!-- Muted Only Mode -->
      <template v-else-if="alertStore.mutedAlertsCount > 0">
        <span class="bell-icon-wrap">
          <span class="bell-emoji">🔕</span>
          <span class="bell-mobile-badge badge-muted-bg">
            {{ alertStore.mutedAlertsCount }}
          </span>
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
        @mouseenter="handleMouseEnter"
        @mouseleave="handleMouseLeave"
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
            type="button"
            class="btn-toast-close"
            title="Dismiss notification (Docks into bell icon)"
            aria-label="Close notification"
            @click.stop.prevent="dismissToast"
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
            v-if="hasNodeDown"
            type="button"
            class="btn-toast-action btn-toast-failover"
            title="1-Click SRE Fast Failover for offline node"
            @click="handleQuickFailover"
          >
            <span>⚡ Quick Failover</span>
          </button>
          <button
            type="button"
            class="btn-toast-action btn-toast-details"
            title="Open interactive Alert Center"
            @click="handleOpenDetails"
          >
            <span>🔍 View Details</span>
          </button>
          <button
            type="button"
            class="btn-toast-action btn-toast-mute"
            title="Silence all active node alerts until server restart"
            @click="handleMuteAll"
          >
            <span>🔕 Mute All</span>
          </button>
          <button
            type="button"
            class="btn-toast-action btn-toast-dismiss"
            title="Dismiss toast and dock into bell icon"
            @click.stop.prevent="dismissToast"
          >
            <span>✕ Dismiss</span>
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
@import '../../assets/styles/components/floating-alert-toast.css';
</style>
