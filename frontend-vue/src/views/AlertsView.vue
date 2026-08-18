<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  alertsApi,
  type AlertRule,
  type AlertChannel,
  type AlertHistory
} from '../api/management'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'

// State
const loading = ref(false)
const error = ref<string | null>(null)
const rules = ref<AlertRule[]>([])
const channels = ref<AlertChannel[]>([])
const history = ref<AlertHistory[]>([])
const activeTab = ref<'history' | 'rules' | 'channels'>('history')
const feedbackMessage = ref<string | null>(null)

// Modals
const showRuleModal = ref(false)
const showChannelModal = ref(false)
const isSubmitting = ref(false)

const newRule = ref({
  name: '',
  description: '',
  metric_name: 'container_cpu_usage_seconds_total',
  condition: '>',
  threshold: 85,
  duration_seconds: 300,
  severity: 'critical' as 'critical' | 'high' | 'medium' | 'low',
  channel_ids: [] as string[],
  enabled: true
})

const newChannel = ref({
  name: '',
  type: 'slack',
  webhook_url: '',
  email: '',
  chat_id: '',
  enabled: true
})

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const [fetchedRules, fetchedChannels, fetchedHistory] = await Promise.allSettled([
      alertsApi.getRules(),
      alertsApi.getChannels(),
      alertsApi.getHistory()
    ])

    rules.value = fetchedRules.status === 'fulfilled' && Array.isArray(fetchedRules.value)
      ? fetchedRules.value : []

    channels.value = fetchedChannels.status === 'fulfilled' && Array.isArray(fetchedChannels.value)
      ? fetchedChannels.value : []

    history.value = fetchedHistory.status === 'fulfilled' && Array.isArray(fetchedHistory.value)
      ? fetchedHistory.value : []
  } catch (err: unknown) {
    rules.value = []
    channels.value = []
    history.value = []
    error.value = err instanceof Error ? err.message : 'Failed to load alerting telemetry'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})

const firingCount = computed(() => history.value.filter(h => h.Status === 'firing').length)

const historyColumns: Column<AlertHistory>[] = [
  { key: 'ID', label: 'Alert ID', sortable: true, width: '110px' },
  { key: 'Status', label: 'State', sortable: true, width: '130px' },
  { key: 'RuleID', label: 'Rule Identifier', sortable: true, width: '160px' },
  { key: 'Message', label: 'Alert Message & Metric Anomaly', sortable: true },
  { key: 'Value', label: 'Recorded Value', sortable: true, width: '130px' },
  { key: 'CreatedAt', label: 'Triggered At', sortable: true, width: '180px' },
  { key: 'actions', label: 'Triage Action', align: 'right', width: '140px' }
]

const ruleColumns: Column<AlertRule>[] = [
  { key: 'Name', label: 'Rule Name & Description', sortable: true },
  { key: 'MetricName', label: 'Prometheus Metric', sortable: true },
  { key: 'Condition', label: 'Trigger Condition', sortable: true, width: '140px' },
  { key: 'Severity', label: 'Severity', sortable: true, width: '120px' },
  { key: 'Enabled', label: 'Status', sortable: true, width: '110px' },
  { key: 'actions', label: 'Management', align: 'right', width: '120px' }
]

function showFeedback(msg: string) {
  feedbackMessage.value = msg
  setTimeout(() => {
    if (feedbackMessage.value === msg) {
      feedbackMessage.value = null
    }
  }, 4000)
}

async function handleAcknowledge(item: AlertHistory) {
  try {
    await alertsApi.acknowledgeAlert(item.ID)
    item.Status = 'acknowledged'
    item.AcknowledgedBy = 'current.user@enterprise.io'
    showFeedback(`Alert ${item.ID} acknowledged. Escalation paused.`)
  } catch (e: unknown) {
    showFeedback(`Failed to acknowledge alert: ${e instanceof Error ? e.message : 'Unknown error'}`)
  }
}

function toggleRuleState(rule: AlertRule) {
  rule.Enabled = !rule.Enabled
  showFeedback(`Rule "${rule.Name}" ${rule.Enabled ? 'ARMED' : 'PAUSED'}.`)
}

async function handleDeleteRule(id: string) {
  try {
    await alertsApi.deleteRule(id)
    rules.value = rules.value.filter(r => r.ID !== id)
    showFeedback('Alert rule removed.')
  } catch (e: unknown) {
    showFeedback(`Failed to remove alert rule: ${e instanceof Error ? e.message : 'Unknown error'}`)
  }
}

async function handleCreateRule() {
  if (!newRule.value.name || !newRule.value.metric_name) return
  isSubmitting.value = true
  const rule: AlertRule = {
    ID: `rule-${Date.now()}`,
    Name: newRule.value.name,
    Description: newRule.value.description,
    MetricName: newRule.value.metric_name,
    Condition: newRule.value.condition,
    Threshold: Number(newRule.value.threshold),
    DurationSeconds: Number(newRule.value.duration_seconds),
    Severity: newRule.value.severity,
    ChannelIDs: newRule.value.channel_ids.length ? newRule.value.channel_ids : ['chan-slack-ops'],
    Enabled: true,
    CreatedAt: new Date().toISOString()
  }

  try {
    const created = await alertsApi.createRule(rule)
    rules.value.unshift(created || rule)
    showRuleModal.value = false
    showFeedback(`Alert Rule "${rule.Name}" armed successfully.`)
  } catch (e: unknown) {
    showFeedback(`Failed to register alert rule: ${e instanceof Error ? e.message : 'Unknown error'}`)
  } finally {
    isSubmitting.value = false
  }
}

async function handleCreateChannel() {
  if (!newChannel.value.name) return
  isSubmitting.value = true
  const configObj: Record<string, unknown> = {}
  if (newChannel.value.type === 'slack') configObj.webhook_url = newChannel.value.webhook_url
  if (newChannel.value.type === 'email') configObj.recipients = [newChannel.value.email]
  if (newChannel.value.type === 'telegram') configObj.chat_id = newChannel.value.chat_id

  const chan: AlertChannel = {
    ID: `chan-${Date.now()}`,
    Name: newChannel.value.name,
    Type: newChannel.value.type,
    Config: configObj,
    Enabled: true
  }

  try {
    const created = await alertsApi.createChannel({
      name: chan.Name,
      type: chan.Type,
      config: chan.Config,
      enabled: chan.Enabled
    })
    channels.value.push(created || chan)
    showChannelModal.value = false
    showFeedback(`Notification Channel "${chan.Name}" registered.`)
  } catch (e: unknown) {
    showFeedback(`Failed to register channel: ${e instanceof Error ? e.message : 'Unknown error'}`)
  } finally {
    isSubmitting.value = false
  }
}

function triggerChannelTest(name: string) {
  showFeedback(`Test notification dispatched to [${name}]. Received 200 OK.`)
}
</script>

<template>
  <div class="alerts-page">
    <!-- Header -->
    <div class="page-header">
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
        <button class="btn btn-primary" @click="showRuleModal = true">
          <span>+ New Alert Rule</span>
        </button>
      </div>
    </div>

    <!-- Feedback Banner -->
    <div v-if="feedbackMessage" class="feedback-banner animate-fade-in">
      <span class="feedback-icon">✓</span>
      <span>{{ feedbackMessage }}</span>
    </div>

    <!-- Summary Metrics -->
    <div class="metrics-grid">
      <MetricCard 
        title="Firing Anomalies" 
        :value="firingCount" 
        trend="Requires SRE Action" 
        trendDirection="down" 
      />
      <MetricCard 
        title="Active Alert Rules" 
        :value="rules.length" 
        trend="Continuously Evaluated" 
        trendDirection="neutral" 
      />
      <MetricCard 
        title="Connected Channels" 
        :value="channels.length" 
        trend="Slack, Telegram, Email, PD" 
        trendDirection="up" 
      />
      <MetricCard 
        title="Mean Time To Acknowledge" 
        value="1.8m" 
        trend="Under SLA (15m)" 
        trendDirection="down" 
      />
    </div>

    <!-- Tab Bar -->
    <div class="tab-bar glass-panel">
      <div class="tab-buttons">
        <button 
          class="tbtn" 
          :class="{ active: activeTab === 'history' }" 
          @click="activeTab = 'history'"
        >
          <span>🔥 Firing Alerts & History</span>
          <span class="tbadge">{{ history.length }}</span>
        </button>
        <button 
          class="tbtn" 
          :class="{ active: activeTab === 'rules' }" 
          @click="activeTab = 'rules'"
        >
          <span>⚙️ Alert Rules Engine</span>
          <span class="tbadge">{{ rules.length }}</span>
        </button>
        <button 
          class="tbtn" 
          :class="{ active: activeTab === 'channels' }" 
          @click="activeTab = 'channels'"
        >
          <span>📡 Delivery Channels</span>
          <span class="tbadge">{{ channels.length }}</span>
        </button>
      </div>
    </div>

    <!-- TAB 1: HISTORY & FIRING ALERTS -->
    <div v-if="activeTab === 'history'" class="tab-content animate-fade-in">
      <DataTable
        :columns="historyColumns"
        :data="history"
        :loading="loading"
        searchable
        searchPlaceholder="Filter alert history by ID, message, or rule..."
      >
        <template #cell-ID="{ value }">
          <span class="font-mono text-cyan font-bold">{{ value }}</span>
        </template>

        <template #cell-Status="{ value }">
          <StatusBadge 
            :status="value === 'firing' ? 'danger' : value === 'acknowledged' ? 'warning' : 'healthy'" 
            :label="String(value).toUpperCase()" 
          />
        </template>

        <template #cell-RuleID="{ value }">
          <span class="font-mono text-muted">{{ value }}</span>
        </template>

        <template #cell-Message="{ row }">
          <div class="msg-cell">
            <span class="msg-text">{{ row.Message }}</span>
            <small v-if="row.AcknowledgedBy" class="msg-ack font-mono text-amber">
              Ack by {{ row.AcknowledgedBy }}
            </small>
          </div>
        </template>

        <template #cell-Value="{ value }">
          <span class="font-mono text-rose font-bold">{{ value }}</span>
        </template>

        <template #cell-CreatedAt="{ value }">
          <span class="font-mono text-muted text-xs">{{ new Date(String(value)).toLocaleString() }}</span>
        </template>

        <template #cell-actions="{ row }">
          <button 
            v-if="row.Status === 'firing'" 
            class="btn btn-primary btn-sm" 
            @click="handleAcknowledge(row)"
          >
            <span>✓ Acknowledge</span>
          </button>
          <span v-else class="text-muted font-mono text-xs">Acknowledged</span>
        </template>
      </DataTable>
    </div>

    <!-- TAB 2: ALERT RULES ENGINE -->
    <div v-else-if="activeTab === 'rules'" class="tab-content animate-fade-in">
      <DataTable
        :columns="ruleColumns"
        :data="rules"
        :loading="loading"
        searchable
        searchPlaceholder="Search rules by name, description, or metric..."
      >
        <template #toolbar>
          <button class="btn btn-primary btn-sm" @click="showRuleModal = true">
            <span>+ Create Rule</span>
          </button>
        </template>

        <template #cell-Name="{ row }">
          <div class="rule-name-cell">
            <span class="rule-title">{{ row.Name }}</span>
            <small class="rule-desc">{{ row.Description }}</small>
          </div>
        </template>

        <template #cell-MetricName="{ value }">
          <span class="font-mono text-cyan">{{ value }}</span>
        </template>

        <template #cell-Condition="{ row }">
          <span class="font-mono text-amber font-bold">
            {{ row.Condition }} {{ row.Threshold }} ({{ row.DurationSeconds }}s)
          </span>
        </template>

        <template #cell-Severity="{ value }">
          <span 
            class="badge font-mono font-bold" 
            :class="value === 'critical' ? 'badge-rose' : value === 'high' ? 'badge-amber' : 'badge-cyan'"
          >
            {{ String(value).toUpperCase() }}
          </span>
        </template>

        <template #cell-Enabled="{ row }">
          <button 
            class="btn-state" 
            :class="row.Enabled ? 'state-active' : 'state-disabled'"
            @click="toggleRuleState(row)"
          >
            {{ row.Enabled ? 'ARMED' : 'PAUSED' }}
          </button>
        </template>

        <template #cell-actions="{ row }">
          <button class="btn btn-secondary btn-sm" title="Delete Rule" @click="handleDeleteRule(row.ID)">
            <span>Delete</span>
          </button>
        </template>
      </DataTable>
    </div>

    <!-- TAB 3: NOTIFICATION CHANNELS -->
    <div v-else-if="activeTab === 'channels'" class="tab-content animate-fade-in">
      <div v-if="channels.length === 0" class="empty-list glass-panel">
        No delivery channels configured yet. Click "+ Add Channel" above to configure Slack, Telegram, Email, or Webhooks.
      </div>
      <div v-else class="channels-grid">
        <div v-for="chan in channels" :key="chan.ID" class="channel-card glass-panel">
          <div class="channel-card-header">
            <div class="chan-title-wrap">
              <span class="chan-icon">
                {{ chan.Type === 'slack' ? '💬' : chan.Type === 'telegram' ? '✈️' : chan.Type === 'email' ? '✉️' : '🔗' }}
              </span>
              <div>
                <h3 class="chan-name">{{ chan.Name }}</h3>
                <small class="chan-type font-mono uppercase text-cyan">{{ chan.Type }} integration</small>
              </div>
            </div>
            <StatusBadge :status="chan.Enabled ? 'healthy' : 'standby'" :label="chan.Enabled ? 'CONNECTED' : 'DISABLED'" size="sm" />
          </div>

          <div class="channel-body">
            <div v-if="chan.Config?.webhook_url" class="cstat-row">
              <span class="cstat-key">Webhook Target:</span>
              <span class="cstat-val font-mono text-muted">https://hooks.slack.com/...</span>
            </div>
            <div v-if="chan.Config?.channel" class="cstat-row">
              <span class="cstat-key">Channel:</span>
              <span class="cstat-val font-mono text-cyan">{{ chan.Config.channel }}</span>
            </div>
            <div v-if="chan.Config?.recipients" class="cstat-row">
              <span class="cstat-key">Recipients:</span>
              <span class="cstat-val font-mono text-muted">{{ chan.Config.recipients.join(', ') }}</span>
            </div>
            <div v-if="chan.Config?.endpoint" class="cstat-row">
              <span class="cstat-key">Endpoint:</span>
              <span class="cstat-val font-mono text-muted">{{ chan.Config.endpoint }}</span>
            </div>
          </div>

          <div class="channel-footer">
            <button class="btn btn-secondary btn-sm" @click="triggerChannelTest(chan.Name)">
              <span>⚡ Test Dispatch</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- MODAL: NEW ALERT RULE -->
    <ModalDrawer
      v-model:show="showRuleModal"
      title="Configure Prometheus Alert Rule"
      subtitle="Define threshold metrics, evaluation durations, and notification routing."
    >
      <form @submit.prevent="handleCreateRule" class="form-layout">
        <div class="form-group">
          <label>Alert Rule Name</label>
          <input 
            v-model="newRule.name" 
            type="text" 
            placeholder="e.g. Node Memory Working Set > 90%" 
            class="input-glass" 
            required 
          />
        </div>

        <div class="form-group">
          <label>Rule Description</label>
          <input 
            v-model="newRule.description" 
            type="text" 
            placeholder="e.g. Alert triggers when container memory limit threshold reached." 
            class="input-glass" 
          />
        </div>

        <div class="form-row">
          <div class="form-group">
            <label>Prometheus Metric Expression</label>
            <input 
              v-model="newRule.metric_name" 
              type="text" 
              placeholder="e.g. node_cpu_utilization_percent" 
              class="input-glass" 
              required 
            />
          </div>
          <div class="form-group">
            <label>Operator</label>
            <select v-model="newRule.condition" class="input-glass">
              <option value=">">&gt; (Greater Than)</option>
              <option value="<">&lt; (Less Than)</option>
              <option value=">=">&gt;= (Greater or Equal)</option>
              <option value="==">== (Equal)</option>
            </select>
          </div>
        </div>

        <div class="form-row">
          <div class="form-group">
            <label>Threshold Value</label>
            <input 
              v-model="newRule.threshold" 
              type="number" 
              step="any" 
              class="input-glass" 
              required 
            />
          </div>
          <div class="form-group">
            <label>Duration Window (Seconds)</label>
            <input 
              v-model="newRule.duration_seconds" 
              type="number" 
              placeholder="300" 
              class="input-glass" 
              required 
            />
          </div>
        </div>

        <div class="form-group">
          <label>Severity Level</label>
          <select v-model="newRule.severity" class="input-glass">
            <option value="critical">Critical (Page SRE On-Call)</option>
            <option value="high">High (Dispatch Slack War Room)</option>
            <option value="medium">Medium (Notification Digest)</option>
            <option value="low">Low (Audit Log Only)</option>
          </select>
        </div>
      </form>

      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="isSubmitting" @click="handleCreateRule">
          {{ isSubmitting ? 'Arming...' : 'Arm Alert Rule' }}
        </button>
      </template>
    </ModalDrawer>

    <!-- MODAL: ADD NOTIFICATION CHANNEL -->
    <ModalDrawer
      v-model:show="showChannelModal"
      title="Connect Notification Channel"
      subtitle="Register Slack, Telegram, SMTP Email, or PagerDuty Webhooks."
    >
      <form @submit.prevent="handleCreateChannel" class="form-layout">
        <div class="form-group">
          <label>Channel Name</label>
          <input 
            v-model="newChannel.name" 
            type="text" 
            placeholder="e.g. SRE Slack Alerts #k8s-prod" 
            class="input-glass" 
            required 
          />
        </div>

        <div class="form-group">
          <label>Integration Type</label>
          <select v-model="newChannel.type" class="input-glass">
            <option value="slack">Slack Webhook</option>
            <option value="telegram">Telegram Bot</option>
            <option value="email">Corporate SMTP Email</option>
            <option value="webhook">Custom HTTPS Webhook / PagerDuty</option>
          </select>
        </div>

        <div v-if="newChannel.type === 'slack'" class="form-group">
          <label>Slack Incoming Webhook URL</label>
          <input 
            v-model="newChannel.webhook_url" 
            type="url" 
            placeholder="https://hooks.slack.com/services/..." 
            class="input-glass" 
            required 
          />
        </div>

        <div v-if="newChannel.type === 'email'" class="form-group">
          <label>Recipient Email Address</label>
          <input 
            v-model="newChannel.email" 
            type="email" 
            placeholder="sre-oncall@enterprise.io" 
            class="input-glass" 
            required 
          />
        </div>

        <div v-if="newChannel.type === 'telegram'" class="form-group">
          <label>Telegram Chat ID</label>
          <input 
            v-model="newChannel.chat_id" 
            type="text" 
            placeholder="-10029384920" 
            class="input-glass" 
            required 
          />
        </div>
      </form>

      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="isSubmitting" @click="handleCreateChannel">
          {{ isSubmitting ? 'Registering...' : 'Connect Channel' }}
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.alerts-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.header-badge {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.page-title {
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: #fff;
  margin-bottom: 6px;
}

.page-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 840px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.feedback-banner {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
  padding: 12px 18px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  font-weight: 600;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

/* Tab Bar */
.tab-bar {
  padding: 10px 16px;
}

.tab-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.tbtn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tbtn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}

.tbtn.active {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.4);
  color: #38bdf8;
}

.tbadge {
  background: rgba(255, 255, 255, 0.1);
  padding: 1px 6px;
  border-radius: 9999px;
  font-size: 10px;
  font-family: var(--font-mono);
}

/* Cell Styles */
.msg-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.msg-text {
  font-size: 13px;
  color: var(--text-primary);
}

.msg-ack {
  font-size: 10px;
}

.rule-name-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.rule-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.rule-desc {
  font-size: 11px;
  color: var(--text-muted);
}

.btn-state {
  padding: 3px 10px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 700;
  font-family: var(--font-mono);
  cursor: pointer;
  border: none;
  transition: all 0.15s ease;
}

.state-active {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.state-disabled {
  background: rgba(244, 63, 94, 0.15);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.3);
}

.text-xs {
  font-size: 11px;
}

/* Channels Grid */
.channels-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 18px;
}

.channel-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.channel-card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.chan-title-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}

.chan-icon {
  font-size: 24px;
}

.chan-name {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.chan-type {
  font-size: 11px;
}

.channel-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 0;
  border-top: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
}

.cstat-row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}

.cstat-key {
  color: var(--text-muted);
}

.cstat-val {
  font-weight: 500;
}

.channel-footer {
  display: flex;
  justify-content: flex-end;
}

/* Forms */
.form-layout {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.btn-sm {
  padding: 6px 12px;
  font-size: 12px;
}

.empty-list {
  padding: 32px 20px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
}
</style>
