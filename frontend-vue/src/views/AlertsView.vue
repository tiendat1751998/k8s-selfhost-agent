<script setup lang="ts">
import { ref, computed } from 'vue'
import '../assets/styles/components/alerts-drawers.css'
import '../assets/styles/views/alerts.css'
import { useAlertManager } from '../composables/useAlertManager'
import ActiveAlertsStream from '../components/alerts/ActiveAlertsStream.vue'
import AlertRulesTable from '../components/alerts/AlertRulesTable.vue'
import AlertRulesMobileCards from '../components/alerts/AlertRulesMobileCards.vue'
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

// Active Tab navigation
const activeTab = ref<'firing' | 'rules' | 'channels' | 'history'>('firing')

// Search filtering across firing alerts, rules, channels, history
const searchQuery = ref('')

const filteredFiringAlerts = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return firingAlerts.value
  return firingAlerts.value.filter(alert =>
    (alert.Message && alert.Message.toLowerCase().includes(q)) ||
    (alert.RuleID && alert.RuleID.toLowerCase().includes(q)) ||
    (alert.ID && alert.ID.toLowerCase().includes(q))
  )
})

const filteredRules = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return rules.value
  return rules.value.filter(rule =>
    (rule.Name && rule.Name.toLowerCase().includes(q)) ||
    (rule.Description && rule.Description.toLowerCase().includes(q)) ||
    (rule.MetricName && rule.MetricName.toLowerCase().includes(q)) ||
    (rule.ID && rule.ID.toLowerCase().includes(q)) ||
    (rule.Severity && rule.Severity.toLowerCase().includes(q)) ||
    (rule.ChannelIDs && rule.ChannelIDs.some(cid => cid.toLowerCase().includes(q)))
  )
})

const filteredChannels = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return channels.value
  return channels.value.filter(ch =>
    (ch.Name && ch.Name.toLowerCase().includes(q)) ||
    (ch.Type && ch.Type.toLowerCase().includes(q)) ||
    (ch.ID && ch.ID.toLowerCase().includes(q))
  )
})

const filteredHistory = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return history.value
  return history.value.filter(item =>
    (item.Message && item.Message.toLowerCase().includes(q)) ||
    (item.RuleID && item.RuleID.toLowerCase().includes(q)) ||
    (item.ID && item.ID.toLowerCase().includes(q))
  )
})

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
    <!-- Mobile 40-44px Command Bar (<768px) -->
    <div class="alerts-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="bell" size="sm" /> Alerts ({{ firingCount }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="New Alert Rule"
          aria-label="New Alert Rule"
          @click="openCreateRule"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Sync / Refresh Telemetry"
          aria-label="Sync / Refresh Telemetry"
          @click="loadData"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="alerts-micro-telemetry mobile-only font-mono" role="status" aria-label="Alerts Micro Telemetry">
      <span class="tel-item tel-crit"><BaseIcon name="alert-triangle" size="xs" /> {{ criticalP1Count }} Critical</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-warn"><BaseIcon name="alert-triangle" size="xs" /> {{ warningCount }} Warning</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-silent"><BaseIcon name="bell-off" size="xs" /> {{ silencedRulesCount }} Silenced</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-chan"><BaseIcon name="radio" size="xs" /> {{ channels.length }} Channels</span>
    </div>

    <!-- Mobile Tab Navigation Strip (<768px) -->
    <div class="alerts-mobile-tabs mobile-only" role="tablist" aria-label="Alerts Navigation">
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'firing'"
        class="mobile-tab-btn"
        :class="{ active: activeTab === 'firing' }"
        @click="activeTab = 'firing'"
      >
        <BaseIcon name="flame" size="xs" />
        <span>Firing ({{ firingCount }})</span>
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'rules'"
        class="mobile-tab-btn"
        :class="{ active: activeTab === 'rules' }"
        @click="activeTab = 'rules'"
      >
        <BaseIcon name="sliders" size="xs" />
        <span>Rules ({{ rules.length }})</span>
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'channels'"
        class="mobile-tab-btn"
        :class="{ active: activeTab === 'channels' }"
        @click="activeTab = 'channels'"
      >
        <BaseIcon name="radio" size="xs" />
        <span>Channels ({{ channels.length }})</span>
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'history'"
        class="mobile-tab-btn"
        :class="{ active: activeTab === 'history' }"
        @click="activeTab = 'history'"
      >
        <BaseIcon name="clock" size="xs" />
        <span>History ({{ history.length }})</span>
      </button>
    </div>

    <div v-if="feedbackMessage" class="feedback-banner animate-fade-in">
      <BaseIcon name="check-circle" size="xs" class="feedback-icon" />
      <span>{{ feedbackMessage }}</span>
    </div>

    <!-- Sleek Unified 42px Enterprise Toolbar (.alerts-toolbar-sleek) -->
    <div class="alerts-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="Alerts Management Toolbar">
      <!-- Search input with search icon and clear button (filters alert message, rule ID, channel name) -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter message, rule, channel..."
          class="toolbar-search-input"
          aria-label="Filter alert message, rule ID, channel name"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="clear-input-btn"
          aria-label="Clear search"
          @click="searchQuery = ''"
        >
          <BaseIcon name="x" size="xs" />
        </button>
      </div>

      <!-- Navigation tabs / pills: Active Firing, Alert Rules, Notification Channels, Alert History -->
      <div class="toolbar-nav-pills" role="tablist" aria-label="Alerts Navigation Tabs">
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'firing'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'firing' }"
          @click="activeTab = 'firing'"
        >
          <BaseIcon name="flame" size="xs" />
          <span>Active Firing</span>
          <span class="pill-badge">{{ firingCount }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'rules'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'rules' }"
          @click="activeTab = 'rules'"
        >
          <BaseIcon name="sliders" size="xs" />
          <span>Alert Rules</span>
          <span class="pill-badge">{{ rules.length }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'channels'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'channels' }"
          @click="activeTab = 'channels'"
        >
          <BaseIcon name="radio" size="xs" />
          <span>Notification Channels</span>
          <span class="pill-badge">{{ channels.length }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'history'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'history' }"
          @click="activeTab = 'history'"
        >
          <BaseIcon name="clock" size="xs" />
          <span>Alert History</span>
          <span class="pill-badge">{{ history.length }}</span>
        </button>
      </div>

      <!-- Inline compact KPI badge strip font-mono -->
      <div class="toolbar-kpi-strip font-mono" role="status" aria-label="Alerts Telemetry KPI summary">
        <span class="kpi-badge font-mono">
          {{ firingCount }} Firing ({{ criticalP1Count }} Critical · {{ warningCount }} Warning · {{ silencedRulesCount }} Silenced)
        </span>
      </div>

      <!-- Context-Aware Action Buttons: only context CTA + Refresh across all tabs -->
      <div class="toolbar-actions-group">
        <button
          v-if="activeTab === 'rules'"
          type="button"
          class="btn btn-primary toolbar-btn"
          title="Create New Alert Rule"
          aria-label="New Alert Rule"
          @click="openCreateRule"
        >
          <BaseIcon name="plus" size="xs" />
          <span>+ New Rule</span>
        </button>
        <button
          v-else-if="activeTab === 'channels'"
          type="button"
          class="btn btn-primary toolbar-btn"
          title="Add Notification Channel"
          aria-label="Add Channel"
          @click="showChannelModal = true"
        >
          <BaseIcon name="plus" size="xs" />
          <span>+ Add Channel</span>
        </button>
        <button
          type="button"
          class="btn btn-secondary toolbar-btn"
          :disabled="loading"
          title="Refresh Alerts & Telemetry"
          aria-label="Refresh"
          @click="loadData"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" />
          <span>{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
      </div>
    </div>

    <!-- Elevated Content Stream & Tables directly below toolbar (~65-75px from Top HUD) -->
    <!-- Tab Content 1: Active Firing Stream -->
    <div v-if="activeTab === 'firing'" class="tab-content animate-fade-in">
      <ActiveAlertsStream 
        class="desktop-only"
        :alerts="filteredFiringAlerts"
        @silence="handleSilence($event)"
        @resolve="handleResolve($event)"
        @telemetry="openTelemetry($event)"
      />

      <AlertsMobileCards 
        :alerts="filteredFiringAlerts" 
        @acknowledge="handleAcknowledge"
        @silence="handleSilence($event)"
        @telemetry="openTelemetry"
      />
    </div>

    <!-- Tab Content 2: Alert Rules Table -->
    <div v-else-if="activeTab === 'rules'" class="tab-content animate-fade-in">
      <div class="desktop-table-view desktop-only">
        <AlertRulesTable 
          :rules="filteredRules" 
          :loading="loading" 
          @create="openCreateRule"
          @edit="openEditRule"
          @delete="handleDeleteRule"
          @toggle="toggleRuleState"
        />
      </div>
      <AlertRulesMobileCards
        :rules="filteredRules"
        :loading="loading"
        @edit="openEditRule"
        @delete="handleDeleteRule"
        @toggle="toggleRuleState"
      />
    </div>

    <!-- Tab Content 3: Notification Channels Grid -->
    <div v-else-if="activeTab === 'channels'" class="tab-content animate-fade-in">
      <AlertChannelsGrid 
        :channels="filteredChannels" 
        @test="triggerChannelTest" 
      />
    </div>

    <!-- Tab Content 4: Alert History Table -->
    <div v-else-if="activeTab === 'history'" class="tab-content animate-fade-in">
      <AlertsMobileCards 
        class="mobile-only"
        :alerts="filteredHistory" 
        @acknowledge="handleAcknowledge"
        @silence="handleSilence($event)"
        @telemetry="openTelemetry"
      />

      <div class="desktop-table-view">
        <DataTable
          :columns="historyColumns"
          :data="filteredHistory"
          :loading="loading"
          :searchable="false"
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
                <BaseIcon name="check" size="xs" /> <span>Ack</span>
              </button>
              <button 
                class="btn btn-secondary btn-sm" 
                title="Inspect Telemetry"
                @click="openTelemetry(row)"
              >
                <BaseIcon name="search" size="xs" />
              </button>
            </div>
          </template>
        </DataTable>
      </div>
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
