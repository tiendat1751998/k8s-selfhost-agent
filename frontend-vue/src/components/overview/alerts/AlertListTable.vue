<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import type { MetricAlert } from '../../../api/overview'
import type { MutedAlertConfig } from '../../../stores/alertStore'
import type { AlertTabType } from './AlertFilterBar.vue'

interface Props {
  activeTab: AlertTabType
  activeAlerts: MetricAlert[]
  mutedAlertsList: MutedAlertConfig[]
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'mute-alert', alert: MetricAlert, mode: 'restart' | '1h' | '24h' | 'session' | 'forever'): void
  (e: 'unmute-alert', key: string): void
  (e: 'dismiss-alert', alert: MetricAlert): void
  (e: 'navigate-to-host', nodeNameOrId: string): void
  (e: 'remediate-node', nodeName: string): void
}>()

const openSnoozeDropdownKey = ref<string | null>(null)

function getAlertKey(alert: MetricAlert): string {
  return `${alert.node_name || alert.node_id}-${alert.type}`
}

function toggleSnoozeDropdown(key: string) {
  openSnoozeDropdownKey.value = openSnoozeDropdownKey.value === key ? null : key
}

function handleMuteAlert(alert: MetricAlert, mode: 'restart' | '1h' | '24h' | 'session' | 'forever' = 'restart') {
  openSnoozeDropdownKey.value = null
  emit('mute-alert', alert, mode)
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
    return mins < 60 ? `Snoozed (${mins}m rem)` : `Snoozed (${Math.ceil(mins / 60)}h rem)`
  }
  return config.snoozeMode
}

function handleDocumentClick(e: MouseEvent) {
  if (openSnoozeDropdownKey.value && !(e.target as HTMLElement).closest('.snooze-dropdown-wrapper')) {
    openSnoozeDropdownKey.value = null
  }
}

onMounted(() => document.addEventListener('click', handleDocumentClick))
onUnmounted(() => document.removeEventListener('click', handleDocumentClick))
</script>

<template>
  <div class="alert-modal-body">
    <!-- EMPTY STATE -->
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
        {{ activeTab === 'muted' ? 'All node alert policies are live and actively monitored.' : 'All cluster nodes are responding normally and operating within configured thresholds.' }}
      </p>
    </div>

    <!-- ACTIVE ALERTS LIST -->
    <div v-if="(activeTab === 'active' || activeTab === 'all') && activeAlerts.length > 0" class="alert-group-section">
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
            <p class="card-msg"><strong>{{ alert.message }}:</strong> {{ getAlertRecommendation(alert) }}</p>
          </div>

          <div class="card-actions-row">
            <button v-if="alert.type === 'node_down' || alert.type === 'NodeNotReady'" type="button" class="btn-card-action btn-action-failover" @click="emit('remediate-node', alert.node_name || alert.node_id)" title="Trigger 1-Click Fast Failover SRE Remediation"><span class="btn-text-full">⚡ 1-Click Failover</span><span class="btn-text-mobile">⚡ Failover</span></button>
            <button type="button" class="btn-card-action btn-card-registry" @click="emit('navigate-to-host', alert.node_name || alert.node_id)" title="Open host in Infrastructure Registry">
              <span class="btn-text-full">⚙️ Open in Registry</span><span class="btn-text-mobile">⚙️ Host</span>
            </button>

            <!-- Snooze Dropdown -->
            <div class="snooze-dropdown-wrapper">
              <button type="button" class="btn-card-mute-pill" @click="handleMuteAlert(alert, 'restart')" title="Silence this alert until server restart">
                <span class="mute-icon">🔕</span>
                <span class="btn-text-full">Mute (Until Restart)</span><span class="btn-text-mobile">Mute</span>
              </button>
              <button type="button" class="btn-card-snooze-caret" @click.stop="toggleSnoozeDropdown(getAlertKey(alert))" title="More snooze options">▾</button>

              <div v-if="openSnoozeDropdownKey === getAlertKey(alert)" class="snooze-menu glass-panel animate-scale-in" @click.stop>
                <div class="snooze-menu-header">Snooze Duration</div>
                <button type="button" class="snooze-menu-item" @click="handleMuteAlert(alert, 'restart')">
                  <span class="snooze-item-icon">🔄</span>
                  <div class="snooze-item-text"><span class="snooze-item-title">Until Server Restart</span><span class="snooze-item-desc">Muted across page refreshes</span></div>
                </button>
                <button type="button" class="snooze-menu-item" @click="handleMuteAlert(alert, '1h')">
                  <span class="snooze-item-icon">⏱️</span>
                  <div class="snooze-item-text"><span class="snooze-item-title">Snooze 1 Hour</span><span class="snooze-item-desc">Re-evaluate after 60 mins</span></div>
                </button>
                <button type="button" class="snooze-menu-item" @click="handleMuteAlert(alert, '24h')">
                  <span class="snooze-item-icon">📅</span>
                  <div class="snooze-item-text"><span class="snooze-item-title">Snooze 24 Hours</span><span class="snooze-item-desc">Re-evaluate after 1 day</span></div>
                </button>
                <button type="button" class="snooze-menu-item" @click="handleMuteAlert(alert, 'session')">
                  <span class="snooze-item-icon">🪟</span>
                  <div class="snooze-item-text"><span class="snooze-item-title">Dismiss for Session</span><span class="snooze-item-desc">Muted until browser tab closes</span></div>
                </button>
              </div>
            </div>

            <button type="button" class="btn-card-dismiss" @click="emit('dismiss-alert', alert)" title="Dismiss alert from active view">
              <span class="btn-text-full">✕ Dismiss</span><span class="btn-text-mobile">✕</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- MUTED ALERTS LIST -->
    <div v-if="(activeTab === 'muted' || activeTab === 'all') && mutedAlertsList.length > 0" class="alert-group-section">
      <div v-if="activeTab === 'all'" class="group-section-title text-muted">
        <span>🔕 SILENCED &amp; MUTED RULES ({{ mutedAlertsList.length }})</span>
      </div>

      <div class="muted-cards-grid">
        <div v-for="item in mutedAlertsList" :key="item.key" class="muted-card glass-panel">
          <div class="muted-card-left">
            <div class="muted-card-header">
              <span class="muted-card-icon">🔕</span>
              <span class="muted-card-node">{{ item.nodeName || item.nodeId || item.key }}</span>
              <span class="card-type-tag">{{ item.type.toUpperCase() }}</span>
              <span class="muted-badge-mode">{{ formatSnoozeLabel(item) }}</span>
            </div>
            <div class="muted-card-meta">
              <span>Muted at {{ new Date(item.mutedAt).toLocaleTimeString() }}</span>
              <span v-if="item.expiresAt" class="muted-expires-text"> &bull; Expires: {{ new Date(item.expiresAt).toLocaleTimeString() }}</span>
            </div>
          </div>
          <div class="muted-card-actions">
            <button type="button" class="btn-unmute-single" @click="emit('unmute-alert', item.key)" title="Unmute and restore live alerting for this node">
              <span>🔔 Unmute</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/alert-center.css';

.btn-text-full {
  display: inline;
}
.btn-text-mobile {
  display: none;
}

@media (max-width: 640px) {
  .btn-text-full {
    display: none !important;
  }
  .btn-text-mobile {
    display: inline !important;
  }
  .card-actions-row {
    display: flex;
    flex-wrap: nowrap;
    gap: 6px;
    align-items: center;
    overflow-x: auto;
  }
  .card-actions-row .btn-card-action,
  .card-actions-row .btn-card-mute-pill,
  .card-actions-row .btn-card-dismiss {
    padding: 4px 8px;
    font-size: 11px;
    white-space: nowrap;
  }
}
</style>
