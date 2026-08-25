<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import type { MetricAlert } from '../../../api/overview'
import type { MutedAlertConfig } from '../../../stores/alertStore'

interface Props {
  show: boolean
  activeAlerts: MetricAlert[]
  mutedAlertsList: MutedAlertConfig[]
  hasCriticalAlerts: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'mute-alert', alert: MetricAlert, mode: 'restart' | '1h' | '24h' | 'session' | 'forever'): void
  (e: 'unmute-alert', key: string): void
  (e: 'mute-all', mode: 'restart' | '1h' | '24h'): void
  (e: 'unmute-all'): void
  (e: 'dismiss-alert', alert: MetricAlert): void
  (e: 'navigate-to-host', nodeNameOrId: string): void
}>()

// Active tab filter: 'active' | 'muted' | 'all'
const activeTab = ref<'active' | 'muted' | 'all'>('active')

// When modal opens or alert list changes, select appropriate default tab
watch(
  () => props.show,
  (isOpen) => {
    if (isOpen) {
      if (props.activeAlerts.length > 0) {
        activeTab.value = 'active'
      } else if (props.mutedAlertsList.length > 0) {
        activeTab.value = 'muted'
      } else {
        activeTab.value = 'all'
      }
    }
  },
  { immediate: true }
)

const openSnoozeDropdownKey = ref<string | null>(null)

function getAlertKey(alert: MetricAlert): string {
  return `${alert.node_name || alert.node_id}-${alert.type}`
}

function toggleSnoozeDropdown(key: string) {
  if (openSnoozeDropdownKey.value === key) {
    openSnoozeDropdownKey.value = null
  } else {
    openSnoozeDropdownKey.value = key
  }
}

function handleMuteAlert(
  alert: MetricAlert,
  mode: 'restart' | '1h' | '24h' | 'session' | 'forever' = 'restart'
) {
  openSnoozeDropdownKey.value = null
  emit('mute-alert', alert, mode)
}

function handleMuteAll(mode: 'restart' | '1h' | '24h' = 'restart') {
  emit('mute-all', mode)
}

function handleUnmuteAlert(key: string) {
  emit('unmute-alert', key)
}

function handleUnmuteAll() {
  emit('unmute-all')
}

function handleDismiss(alert: MetricAlert) {
  emit('dismiss-alert', alert)
}

function handleNavigateToHost(nodeNameOrId: string) {
  emit('navigate-to-host', nodeNameOrId)
}

function getAlertRecommendation(alert: MetricAlert): string {
  const t = alert.type.toLowerCase()
  if (t.includes('cpu')) return 'Check top consumer processes or scale horizontal replicas.'
  if (t.includes('mem') || t.includes('memory')) return 'Investigate potential memory leaks or optimize heap limits.'
  if (t.includes('disk')) return 'Prune orphaned container images or rotate system journal logs.'
  if (t.includes('node') || t.includes('down')) return 'Host unreachable. Check agent daemon status on port 9100.'
  return 'Review host telemetry and system diagnostics.'
}

function formatSnoozeLabel(config: MutedAlertConfig): string {
  if (config.snoozeMode === 'restart') return 'Until Server Restart'
  if (config.snoozeMode === 'forever') return 'Muted Forever'
  if (config.snoozeMode === 'session') return 'Session Only'
  if (config.expiresAt) {
    const diffMs = config.expiresAt - Date.now()
    if (diffMs <= 0) return 'Expired'
    const mins = Math.ceil(diffMs / (60 * 1000))
    if (mins < 60) return `Snoozed (${mins}m rem)`
    const hours = Math.ceil(mins / 60)
    return `Snoozed (${hours}h rem)`
  }
  return config.snoozeMode
}

function handleDocumentClick(e: MouseEvent) {
  if (openSnoozeDropdownKey.value) {
    const target = e.target as HTMLElement
    if (!target.closest('.snooze-dropdown-wrapper')) {
      openSnoozeDropdownKey.value = null
    }
  }
}

function handleKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    emit('close')
  }
}

onMounted(() => {
  document.addEventListener('click', handleDocumentClick)
  document.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  document.removeEventListener('click', handleDocumentClick)
  document.removeEventListener('keydown', handleKeyDown)
})
</script>

<template>
  <transition name="modal-fade">
    <div
      v-if="show"
      class="alert-modal-backdrop animate-fade-in"
      role="dialog"
      aria-modal="true"
      aria-labelledby="alert-modal-title"
      @click.self="emit('close')"
    >
      <div class="alert-center-modal glass-panel animate-scale-in">
        <!-- HEADER -->
        <div class="alert-modal-header">
          <div class="alert-modal-title-group">
            <div
              class="alert-modal-icon-badge"
              :class="activeAlerts.length > 0 ? (hasCriticalAlerts ? 'badge-icon-rose' : 'badge-icon-amber') : 'badge-icon-cyan'"
            >
              {{ activeAlerts.length > 0 ? (hasCriticalAlerts ? '🚨' : '⚠️') : '🛡️' }}
            </div>
            <div>
              <div class="title-with-badge">
                <h3 id="alert-modal-title" class="alert-modal-title">
                  Cluster Alert Center &amp; Diagnostics
                </h3>
                <span
                  v-if="activeAlerts.length > 0"
                  class="badge"
                  :class="hasCriticalAlerts ? 'badge-rose' : 'badge-amber'"
                >
                  {{ activeAlerts.length }} Active
                </span>
                <span v-if="mutedAlertsList.length > 0" class="badge badge-muted-tag">
                  {{ mutedAlertsList.length }} Silenced
                </span>
              </div>
              <p class="alert-modal-subtitle">
                Real-time node health violations, autonomous threshold warnings, and persistent silencing policies.
              </p>
            </div>
          </div>

          <div class="alert-header-right-actions">
            <button
              v-if="activeAlerts.length > 0"
              class="btn-mute-all-header"
              @click="handleMuteAll('restart')"
              title="Silence all active node alerts until server restart"
            >
              <span>🔕 Mute All</span>
            </button>
            <button
              v-if="mutedAlertsList.length > 0"
              class="btn-unmute-all-header"
              @click="handleUnmuteAll"
              title="Restore all silenced alert rules"
            >
              <span>🔔 Unmute All</span>
            </button>
            <button
              class="btn-modal-close"
              @click="emit('close')"
              title="Close Alert Center"
              aria-label="Close"
            >
              ✕
            </button>
          </div>
        </div>

        <!-- TAB NAVIGATION -->
        <div class="alert-modal-tabs">
          <button
            class="tab-btn"
            :class="{ 'tab-btn-active': activeTab === 'active' }"
            @click="activeTab = 'active'"
          >
            <span class="tab-icon">🚨</span>
            <span>Active Alerts</span>
            <span class="tab-count" :class="activeAlerts.length > 0 ? 'count-rose' : 'count-muted'">
              {{ activeAlerts.length }}
            </span>
          </button>

          <button
            class="tab-btn"
            :class="{ 'tab-btn-active': activeTab === 'muted' }"
            @click="activeTab = 'muted'"
          >
            <span class="tab-icon">🔕</span>
            <span>Silenced / Muted</span>
            <span class="tab-count" :class="mutedAlertsList.length > 0 ? 'count-amber' : 'count-muted'">
              {{ mutedAlertsList.length }}
            </span>
          </button>

          <button
            class="tab-btn"
            :class="{ 'tab-btn-active': activeTab === 'all' }"
            @click="activeTab = 'all'"
          >
            <span class="tab-icon">📋</span>
            <span>All Alerts</span>
            <span class="tab-count count-cyan">
              {{ activeAlerts.length + mutedAlertsList.length }}
            </span>
          </button>
        </div>

        <!-- BODY LIST -->
        <div class="alert-modal-body">
          <!-- EMPTY STATE: ALL GREEN -->
          <div
            v-if="(activeTab === 'active' && activeAlerts.length === 0) || (activeTab === 'muted' && mutedAlertsList.length === 0) || (activeTab === 'all' && activeAlerts.length === 0 && mutedAlertsList.length === 0)"
            class="alert-empty-state"
          >
            <div class="empty-icon-shield">
              <span class="empty-emoji">{{ activeTab === 'muted' ? '🔔' : '🟢' }}</span>
            </div>
            <h4 class="empty-title">
              {{ activeTab === 'muted' ? 'No Silenced Alert Rules' : 'Cluster Telemetry All Green' }}
            </h4>
            <p class="empty-desc">
              {{
                activeTab === 'muted'
                  ? 'All node alert policies are live and actively monitored.'
                  : 'All cluster nodes are responding normally and operating within configured thresholds.'
              }}
            </p>
          </div>

          <!-- ACTIVE ALERTS LIST -->
          <div
            v-if="(activeTab === 'active' || activeTab === 'all') && activeAlerts.length > 0"
            class="alert-group-section"
          >
            <div v-if="activeTab === 'all'" class="group-section-title text-rose">
              <span>🚨 ACTIVE TELEMETRY ALERTS ({{ activeAlerts.length }})</span>
            </div>

            <div class="alert-cards-grid">
              <div
                v-for="alert in activeAlerts"
                :key="`${alert.node_id}-${alert.type}`"
                class="alert-card glass-panel"
                :class="alert.value >= 90 || alert.type === 'node_down' ? 'card-critical' : 'card-warning'"
              >
                <div class="card-header-row">
                  <div class="card-node-info">
                    <span class="card-icon">{{ alert.value >= 90 || alert.type === 'node_down' ? '🚨' : '⚠️' }}</span>
                    <span class="card-node-name">{{ alert.node_name || alert.node_id }}</span>
                    <span class="card-type-tag">{{ alert.type.toUpperCase() }}</span>
                  </div>
                  <span class="card-threshold-tag">
                    {{ Math.round(alert.value) }}% <span class="threshold-label">(Threshold: {{ alert.threshold }}%)</span>
                  </span>
                </div>

                <div class="card-body-content">
                  <p class="card-msg">
                    <strong>{{ alert.message }}:</strong> {{ getAlertRecommendation(alert) }}
                  </p>
                </div>

                <div class="card-actions-row">
                  <button
                    class="btn-card-action btn-card-registry"
                    @click="handleNavigateToHost(alert.node_name || alert.node_id)"
                    title="Open host in Infrastructure Registry"
                  >
                    <span>⚙️ Open in Registry</span>
                  </button>

                  <!-- Snooze / Mute Dropdown -->
                  <div class="snooze-dropdown-wrapper">
                    <button
                      class="btn-card-mute-pill"
                      @click="handleMuteAlert(alert, 'restart')"
                      title="Silence this alert until server restart"
                    >
                      <span class="mute-icon">🔕</span>
                      <span>Mute (Until Restart)</span>
                    </button>
                    <button
                      class="btn-card-snooze-caret"
                      @click.stop="toggleSnoozeDropdown(getAlertKey(alert))"
                      title="More snooze options"
                    >
                      ▾
                    </button>

                    <!-- Dropdown Menu -->
                    <div
                      v-if="openSnoozeDropdownKey === getAlertKey(alert)"
                      class="snooze-menu glass-panel animate-scale-in"
                      @click.stop
                    >
                      <div class="snooze-menu-header">Snooze Duration</div>
                      <button class="snooze-menu-item" @click="handleMuteAlert(alert, 'restart')">
                        <span class="snooze-item-icon">🔄</span>
                        <div class="snooze-item-text">
                          <span class="snooze-item-title">Until Server Restart</span>
                          <span class="snooze-item-desc">Muted across page refreshes</span>
                        </div>
                      </button>
                      <button class="snooze-menu-item" @click="handleMuteAlert(alert, '1h')">
                        <span class="snooze-item-icon">⏱️</span>
                        <div class="snooze-item-text">
                          <span class="snooze-item-title">Snooze 1 Hour</span>
                          <span class="snooze-item-desc">Re-evaluate after 60 mins</span>
                        </div>
                      </button>
                      <button class="snooze-menu-item" @click="handleMuteAlert(alert, '24h')">
                        <span class="snooze-item-icon">📅</span>
                        <div class="snooze-item-text">
                          <span class="snooze-item-title">Snooze 24 Hours</span>
                          <span class="snooze-item-desc">Re-evaluate after 1 day</span>
                        </div>
                      </button>
                      <button class="snooze-menu-item" @click="handleMuteAlert(alert, 'session')">
                        <span class="snooze-item-icon">🪟</span>
                        <div class="snooze-item-text">
                          <span class="snooze-item-title">Dismiss for Session</span>
                          <span class="snooze-item-desc">Muted until browser tab closes</span>
                        </div>
                      </button>
                    </div>
                  </div>

                  <button
                    class="btn-card-dismiss"
                    @click="handleDismiss(alert)"
                    title="Dismiss alert from active view"
                  >
                    ✕ Dismiss
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- MUTED ALERTS LIST -->
          <div
            v-if="(activeTab === 'muted' || activeTab === 'all') && mutedAlertsList.length > 0"
            class="alert-group-section"
          >
            <div v-if="activeTab === 'all'" class="group-section-title text-muted">
              <span>🔕 SILENCED &amp; MUTED RULES ({{ mutedAlertsList.length }})</span>
            </div>

            <div class="muted-cards-grid">
              <div
                v-for="item in mutedAlertsList"
                :key="item.key"
                class="muted-card glass-panel"
              >
                <div class="muted-card-left">
                  <div class="muted-card-header">
                    <span class="muted-card-icon">🔕</span>
                    <span class="muted-card-node">{{ item.nodeName || item.nodeId || item.key }}</span>
                    <span class="card-type-tag">{{ item.type.toUpperCase() }}</span>
                    <span class="muted-badge-mode">{{ formatSnoozeLabel(item) }}</span>
                  </div>
                  <div class="muted-card-meta">
                    <span>Muted at {{ new Date(item.mutedAt).toLocaleTimeString() }}</span>
                    <span v-if="item.expiresAt" class="muted-expires-text">
                      &bull; Expires: {{ new Date(item.expiresAt).toLocaleTimeString() }}
                    </span>
                  </div>
                </div>

                <div class="muted-card-actions">
                  <button
                    class="btn-unmute-single"
                    @click="handleUnmuteAlert(item.key)"
                    title="Unmute and restore live alerting for this node"
                  >
                    <span>🔔 Unmute</span>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- FOOTER -->
        <div class="alert-modal-footer">
          <div class="footer-summary-text">
            <span>
              {{ activeAlerts.length }} active &bull; {{ mutedAlertsList.length }} silenced
            </span>
          </div>

          <div class="footer-actions">
            <button
              v-if="activeAlerts.length > 0"
              class="btn btn-secondary btn-footer-mute"
              @click="handleMuteAll('restart')"
            >
              <span>🔕 Mute All (Until Restart)</span>
            </button>
            <button
              v-if="mutedAlertsList.length > 0"
              class="btn btn-secondary btn-footer-unmute"
              @click="handleUnmuteAll"
            >
              <span>🔔 Unmute All</span>
            </button>
            <button class="btn btn-primary btn-footer-close" @click="emit('close')">
              <span>Done</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
/* ==========================================
   ALERT CENTER MODAL STYLES
   ========================================== */
.alert-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.8);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100000;
  padding: 20px;
}

.alert-center-modal {
  width: 95%;
  max-width: 860px;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
  background: rgba(15, 23, 42, 0.96);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 16px;
  box-shadow: 0 25px 60px rgba(0, 0, 0, 0.8), 0 0 30px rgba(244, 63, 94, 0.15);
  overflow: hidden;
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
}

/* Header */
.alert-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.02);
  flex-wrap: wrap;
}

.alert-modal-title-group {
  display: flex;
  align-items: center;
  gap: 14px;
  flex: 1;
  min-width: 240px;
}

.alert-modal-icon-badge {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  flex-shrink: 0;
}

.badge-icon-rose {
  background: rgba(244, 63, 94, 0.18);
  border: 1px solid rgba(244, 63, 94, 0.4);
}

.badge-icon-amber {
  background: rgba(245, 158, 11, 0.18);
  border: 1px solid rgba(245, 158, 11, 0.4);
}

.badge-icon-cyan {
  background: rgba(6, 182, 212, 0.18);
  border: 1px solid rgba(6, 182, 212, 0.4);
}

.title-with-badge {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 2px;
}

.alert-modal-title {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
  margin: 0;
  letter-spacing: 0.02em;
}

.badge-rose {
  background: rgba(244, 63, 94, 0.2);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.4);
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 4px;
}

.badge-amber {
  background: rgba(245, 158, 11, 0.2);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.4);
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 4px;
}

.badge-muted-tag {
  background: rgba(148, 163, 184, 0.15);
  border: 1px solid rgba(148, 163, 184, 0.3);
  color: #cbd5e1;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
}

.alert-modal-subtitle {
  font-size: 12px;
  color: #94a3b8;
  margin: 0;
  line-height: 1.4;
}

.alert-header-right-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-mute-all-header {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.4);
  color: #fecdd3;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-mute-all-header:hover {
  background: rgba(244, 63, 94, 0.3);
  border-color: rgba(244, 63, 94, 0.6);
  color: #fff;
}

.btn-unmute-all-header {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.4);
  color: #34d399;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-unmute-all-header:hover {
  background: rgba(16, 185, 129, 0.3);
  border-color: rgba(16, 185, 129, 0.6);
  color: #fff;
}

.btn-modal-close {
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 18px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.btn-modal-close:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.12);
}

/* Tabs */
.alert-modal-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 24px;
  background: rgba(0, 0, 0, 0.25);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.tab-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  border-radius: 8px;
  background: transparent;
  border: 1px solid transparent;
  color: #94a3b8;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.tab-btn:hover {
  background: rgba(255, 255, 255, 0.05);
  color: #fff;
}

.tab-btn-active {
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.2);
  color: #fff;
}

.tab-icon {
  font-size: 13px;
}

.tab-count {
  font-size: 10.5px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 9999px;
}

.count-rose {
  background: rgba(244, 63, 94, 0.25);
  color: #fb7185;
}

.count-amber {
  background: rgba(245, 158, 11, 0.25);
  color: #fbbf24;
}

.count-cyan {
  background: rgba(6, 182, 212, 0.25);
  color: #38bdf8;
}

.count-muted {
  background: rgba(255, 255, 255, 0.08);
  color: #94a3b8;
}

/* Modal Body */
.alert-modal-body {
  padding: 20px 24px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-height: 56vh;
}

.alert-group-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.group-section-title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  padding-bottom: 2px;
}

.text-rose {
  color: #fb7185;
}

.text-muted {
  color: #94a3b8;
}

/* Empty State */
.alert-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 36px 16px;
  text-align: center;
}

.empty-icon-shield {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 4px;
}

.empty-emoji {
  font-size: 28px;
}

.empty-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
  margin: 0;
}

.empty-desc {
  font-size: 12.5px;
  color: #94a3b8;
  margin: 0;
  max-width: 420px;
  line-height: 1.5;
}

/* Cards Grid */
.alert-cards-grid,
.muted-cards-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* Active Alert Card */
.alert-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 18px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.85);
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.25);
  transition: all 0.2s ease;
}

.card-critical {
  border-color: rgba(244, 63, 94, 0.45);
  background: linear-gradient(90deg, rgba(244, 63, 94, 0.14) 0%, rgba(15, 23, 42, 0.9) 100%);
}

.card-warning {
  border-color: rgba(245, 158, 11, 0.45);
  background: linear-gradient(90deg, rgba(245, 158, 11, 0.14) 0%, rgba(15, 23, 42, 0.9) 100%);
}

.card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.card-node-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.card-icon {
  font-size: 16px;
}

.card-node-name {
  font-weight: 700;
  font-size: 13.5px;
  color: #fff;
  font-family: var(--font-mono, monospace);
}

.card-type-tag {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.1);
  color: #fbbf24;
}

.card-threshold-tag {
  font-size: 11px;
  font-family: var(--font-mono, monospace);
  color: #94a3b8;
}

.threshold-label {
  font-size: 10px;
  opacity: 0.8;
}

.card-body-content {
  margin: 0;
}

.card-msg {
  font-size: 12px;
  color: #cbd5e1;
  margin: 0;
  line-height: 1.45;
}

.card-actions-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.btn-card-action {
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

.btn-card-registry {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.18);
  color: #f8fafc;
}

.btn-card-registry:hover {
  background: rgba(255, 255, 255, 0.18);
  color: #fff;
}

/* Snooze dropdown in card */
.snooze-dropdown-wrapper {
  position: relative;
  display: inline-flex;
  align-items: stretch;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.16);
}

.btn-card-mute-pill {
  background: transparent;
  border: none;
  color: #fbbf24;
  font-size: 11px;
  font-weight: 600;
  padding: 5px 9px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  transition: all 0.2s ease;
}

.btn-card-mute-pill:hover {
  background: rgba(251, 191, 36, 0.15);
  color: #fde68a;
}

.mute-icon {
  font-size: 12px;
}

.btn-card-snooze-caret {
  background: transparent;
  border: none;
  border-left: 1px solid rgba(255, 255, 255, 0.12);
  color: #fbbf24;
  padding: 0 7px;
  font-size: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-card-snooze-caret:hover {
  background: rgba(251, 191, 36, 0.2);
  color: #fff;
}

.snooze-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 230px;
  background: rgba(15, 23, 42, 0.96);
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 8px;
  padding: 6px;
  z-index: 120;
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.snooze-menu-header {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: #94a3b8;
  padding: 4px 8px;
  font-weight: 700;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  margin-bottom: 2px;
}

.snooze-menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 8px;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: #f8fafc;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease;
  width: 100%;
}

.snooze-menu-item:hover {
  background: rgba(255, 255, 255, 0.12);
}

.snooze-item-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.snooze-item-text {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.snooze-item-title {
  font-size: 11px;
  font-weight: 600;
  color: #f8fafc;
}

.snooze-item-desc {
  font-size: 9.5px;
  color: #94a3b8;
  line-height: 1.2;
}

.btn-card-dismiss {
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #94a3b8;
  font-size: 11px;
  cursor: pointer;
  padding: 5px 9px;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.btn-card-dismiss:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.25);
}

/* Muted Card */
.muted-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px dashed rgba(148, 163, 184, 0.25);
  transition: all 0.2s ease;
  flex-wrap: wrap;
}

.muted-card:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(148, 163, 184, 0.4);
}

.muted-card-left {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.muted-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.muted-card-icon {
  font-size: 14px;
}

.muted-card-node {
  font-weight: 700;
  font-size: 13px;
  color: #f8fafc;
  font-family: var(--font-mono, monospace);
}

.muted-badge-mode {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(56, 189, 248, 0.15);
  border: 1px solid rgba(56, 189, 248, 0.3);
  color: #38bdf8;
}

.muted-card-meta {
  font-size: 11px;
  color: #94a3b8;
}

.muted-expires-text {
  color: #fbbf24;
}

.muted-card-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-unmute-single {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.35);
  color: #34d399;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.btn-unmute-single:hover {
  background: rgba(16, 185, 129, 0.3);
  border-color: rgba(16, 185, 129, 0.6);
  color: #fff;
}

/* Footer */
.alert-modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 24px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.25);
  flex-wrap: wrap;
}

.footer-summary-text {
  font-size: 12px;
  color: #94a3b8;
}

.footer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-footer-mute {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.4);
  color: #fecdd3;
  padding: 7px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-footer-mute:hover {
  background: rgba(244, 63, 94, 0.3);
  color: #fff;
}

.btn-footer-unmute {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.4);
  color: #34d399;
  padding: 7px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-footer-unmute:hover {
  background: rgba(16, 185, 129, 0.3);
  color: #fff;
}

.btn-footer-close {
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #fff;
  padding: 7px 18px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-footer-close:hover {
  background: linear-gradient(135deg, #60a5fa, #3b82f6);
  transform: translateY(-1px);
}

/* Modal Animations */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

@media (max-width: 640px) {
  .alert-center-modal {
    width: 100%;
    max-height: 94vh;
    border-radius: 12px;
  }

  .alert-modal-header {
    padding: 14px 16px;
  }

  .alert-modal-body {
    padding: 14px 16px;
  }

  .alert-modal-footer {
    padding: 12px 16px;
    flex-direction: column;
    align-items: stretch;
  }

  .footer-actions {
    justify-content: flex-end;
  }
}
</style>
