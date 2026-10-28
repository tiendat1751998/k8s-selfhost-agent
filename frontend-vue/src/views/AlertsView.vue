<script setup lang="ts">
import { ref, computed } from 'vue'
import '../assets/styles/components/alerts-drawers.css'
import '../assets/styles/views/alerts.css'
import { useAlerts } from '../composables/useAlerts'
import ActiveAlertsStream from '../components/alerts/ActiveAlertsStream.vue'
import AlertRulesTable from '../components/alerts/AlertRulesTable.vue'
import AlertRulesMobileCards from '../components/alerts/AlertRulesMobileCards.vue'
import AlertsMobileCards from '../components/alerts/AlertsMobileCards.vue'
import AlertsHeaderBar from '../components/alerts/AlertsHeaderBar.vue'
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
  triggerChannelTest,
  deleteChannel
} = useAlerts()

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
    <AlertsHeaderBar
      v-model:active-tab="activeTab"
      v-model:search-query="searchQuery"
      :firing-count="firingCount"
      :rules-count="rules.length"
      :channels-count="channels.length"
      :history-count="history.length"
      :critical-p1-count="criticalP1Count"
      :warning-count="warningCount"
      :silenced-rules-count="silencedRulesCount"
      :loading="loading"
      @open-create-rule="openCreateRule"
      @open-create-channel="showChannelModal = true"
      @refresh="loadData"
    />

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
      <div class="desktop-table-view rules-table-desktop">
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
        @delete="deleteChannel"
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
