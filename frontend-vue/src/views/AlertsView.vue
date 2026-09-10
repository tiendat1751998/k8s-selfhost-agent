<script setup lang="ts">
import '../assets/styles/components/alerts-drawers.css'
import '../assets/styles/views/alerts.css'
import { useAlertManager } from '../composables/useAlertManager'
import AlertsHudMetrics from '../components/alerts/AlertsHudMetrics.vue'
import ActiveAlertsStream from '../components/alerts/ActiveAlertsStream.vue'
import AlertRulesTable from '../components/alerts/AlertRulesTable.vue'
import AlertsMobileCards from '../components/alerts/AlertsMobileCards.vue'
import CreateAlertRuleModal from '../components/alerts/CreateAlertRuleModal.vue'
import AlertDetailDrawer from '../components/alerts/AlertDetailDrawer.vue'
import AlertChannelsGrid from '../components/alerts/AlertChannelsGrid.vue'
import CreateChannelModal from '../components/alerts/CreateChannelModal.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import type { AlertHistory } from '../api/management'

const {
  loading,
  rules,
  channels,
  history,
  activeTab,
  feedbackMessage,
  showRuleModal,
  showChannelModal,
  isSubmitting,
  editingRule,
  selectedAlert,
  showDetailDrawer,
  firingAlerts,
  firingCount,
  criticalP1Count,
  warningCount,
  silencedRulesCount,
  meanTimeToAcknowledge,
  loadData,
  handleAcknowledge,
  handleSilence,
  handleResolve,
  openTelemetry,
  toggleRuleState,
  handleDeleteRule,
  handleSaveRule,
  openCreateRule,
  openEditRule,
  handleCreateChannel,
  triggerChannelTest
} = useAlertManager()

const historyColumns: Column<AlertHistory>[] = [
  { key: 'ID', label: 'Alert ID', sortable: true, width: '12%' },
  { key: 'Status', label: 'State', sortable: true, width: '10%' },
  { key: 'RuleID', label: 'Rule Identifier', sortable: true, width: '15%' },
  { key: 'Message', label: 'Alert Message & Metric Anomaly', sortable: true, width: '33%' },
  { key: 'Value', label: 'Recorded Value', sortable: true, width: '10%' },
  { key: 'CreatedAt', label: 'Triggered At', sortable: true, width: '10%' },
  { key: 'actions', label: 'Triage Action', align: 'right', width: '10%' }
]
</script>

<template>
  <div class="alerts-page">
    <div class="page-header desktop-header desktop-only">
      <div class="header-titles">
        <div class="header-badge">
          <span class="badge badge-rose">Prometheus Alertmanager</span>
          <span class="badge badge-cyan">Multi-Channel Routing</span>
        </div>
        <h1 class="page-title">Alerting Engine & Delivery Channels</h1>
        <p class="page-desc">
          Manage real-time Prometheus alert threshold rules, multi-tenant notification routing (Slack, Telegram, Email, Webhooks), and triage firing cluster anomalies.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" @click="showChannelModal = true">
          <span>+ Add Channel</span>
        </button>
        <button class="btn btn-primary" @click="openCreateRule">
          <span>+ New Alert Rule</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40-44px Command Bar (<768px) -->
    <div class="alerts-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🚨 Alerts ({{ firingCount }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="New Alert Rule"
          aria-label="New Alert Rule"
          @click="openCreateRule"
        >
          <span>➕</span>
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Sync / Refresh Telemetry"
          aria-label="Sync / Refresh Telemetry"
          @click="loadData"
        >
          <span :class="{ 'animate-spin': loading }">🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="alerts-micro-telemetry mobile-only font-mono" role="status" aria-label="Alerts Micro Telemetry">
      <span class="tel-item tel-crit">🚨 {{ criticalP1Count }} Critical</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-warn">⚠️ {{ warningCount }} Warning</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-silent">🔕 {{ silencedRulesCount }} Silenced</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-chan">📡 {{ channels.length }} Channels</span>
    </div>

    <div v-if="feedbackMessage" class="feedback-banner animate-fade-in">
      <span class="feedback-icon">✓</span>
      <span>{{ feedbackMessage }}</span>
    </div>

    <AlertsHudMetrics 
      class="desktop-only"
      :firingCount="firingCount"
      :criticalCount="criticalP1Count"
      :silencedCount="silencedRulesCount"
      :mtta="meanTimeToAcknowledge"
    />

    <div class="tab-bar glass-panel">
      <div class="tab-buttons">
        <button class="tbtn" :class="{ active: activeTab === 'history' }" @click="activeTab = 'history'">
          <span>🔥 Firing Alerts & History</span>
          <span class="tbadge">{{ history.length }}</span>
        </button>
        <button class="tbtn" :class="{ active: activeTab === 'rules' }" @click="activeTab = 'rules'">
          <span>⚙️ Alert Rules Engine</span>
          <span class="tbadge">{{ rules.length }}</span>
        </button>
        <button class="tbtn" :class="{ active: activeTab === 'channels' }" @click="activeTab = 'channels'">
          <span>📡 Delivery Channels</span>
          <span class="tbadge">{{ channels.length }}</span>
        </button>
      </div>
    </div>

    <div v-if="activeTab === 'history'" class="tab-content animate-fade-in">
      <ActiveAlertsStream 
        class="desktop-only"
        :alerts="firingAlerts"
        @silence="handleSilence($event)"
        @resolve="handleResolve($event)"
        @telemetry="openTelemetry($event)"
      />

      <AlertsMobileCards 
        :alerts="firingAlerts" 
        @acknowledge="handleAcknowledge"
        @silence="handleSilence($event)"
        @telemetry="openTelemetry"
      />

      <div class="desktop-table-view">
        <DataTable
          :columns="historyColumns"
          :data="history"
          :loading="loading"
          searchable
          searchPlaceholder="Filter alert history by ID, message, or rule..."
        >
          <template #cell-ID="{ value }">
            <span class="font-mono text-cyan font-bold truncate block">{{ value }}</span>
          </template>

          <template #cell-Status="{ value }">
            <StatusBadge 
              :status="value === 'firing' ? 'danger' : value === 'acknowledged' ? 'warning' : 'healthy'" 
              :label="String(value).toUpperCase()" 
            />
          </template>

          <template #cell-RuleID="{ value }">
            <span class="font-mono text-muted truncate block" :title="String(value)">{{ value }}</span>
          </template>

          <template #cell-Message="{ row }">
            <div class="msg-cell">
              <span class="msg-text truncate" :title="row.Message">{{ row.Message }}</span>
              <small v-if="row.AcknowledgedBy" class="msg-ack font-mono text-amber truncate">
                Ack by {{ row.AcknowledgedBy }}
              </small>
            </div>
          </template>

          <template #cell-Value="{ value }">
            <span class="font-mono text-rose font-bold">{{ value }}</span>
          </template>

          <template #cell-CreatedAt="{ value }">
            <span class="font-mono text-muted text-xs whitespace-nowrap">{{ new Date(String(value)).toLocaleString() }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-2">
              <button 
                v-if="row.Status === 'firing'" 
                class="btn btn-primary btn-sm" 
                @click="handleAcknowledge(row)"
              >
                <span>✓ Ack</span>
              </button>
              <button 
                class="btn btn-secondary btn-sm" 
                title="Inspect Telemetry"
                @click="openTelemetry(row)"
              >
                <span>🔍</span>
              </button>
            </div>
          </template>
        </DataTable>
      </div>
    </div>

    <div v-else-if="activeTab === 'rules'" class="tab-content animate-fade-in">
      <AlertRulesTable 
        :rules="rules" 
        :loading="loading" 
        @create="openCreateRule"
        @edit="openEditRule"
        @delete="handleDeleteRule"
        @toggle="toggleRuleState"
      />
    </div>

    <div v-else-if="activeTab === 'channels'" class="tab-content animate-fade-in">
      <AlertChannelsGrid 
        :channels="channels" 
        @test="triggerChannelTest" 
      />
    </div>

    <CreateAlertRuleModal 
      v-model:show="showRuleModal"
      :ruleToEdit="editingRule"
      :channels="channels"
      :isSubmitting="isSubmitting"
      @submit="handleSaveRule"
    />

    <CreateChannelModal 
      v-model:show="showChannelModal"
      :isSubmitting="isSubmitting"
      @submit="handleCreateChannel"
    />

    <AlertDetailDrawer 
      v-model:show="showDetailDrawer"
      :alert="selectedAlert"
      @acknowledge="handleAcknowledge"
      @silence="handleSilence"
      @resolve="handleResolve"
    />
  </div>
</template>
