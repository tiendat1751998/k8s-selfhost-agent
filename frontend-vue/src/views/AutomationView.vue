<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>EVENT-DRIVEN SELF-HEALING & AUTOMATION</span>
        </div>
        <h1 class="view-title">Automated Remediation & Workflow Rules</h1>
        <p class="view-desc">
          Automate incident response pipelines: <span class="highlight">Auto-Rollback</span> on deployment errors, <span class="highlight">RCA Generation</span> on crashloops, and <span class="highlight">Node Cordoning</span> on pressure.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchAutomationData">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>+ Create Automation Rule</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Metrics HUD Grid -->
    <div class="metrics-grid">
      <MetricCard
        title="Active Automation Rules"
        :value="rules.length"
        badge="ARMED"
        badge-color="cyan"
        :subtitle="`${enabledRulesCount} rules actively monitoring cluster events`"
        icon="⚡"
      />
      <MetricCard
        title="Total Actions Executed"
        :value="totalExecutionsCount"
        badge="EXECUTED"
        badge-color="emerald"
        subtitle="Self-healing & alerts dispatched"
        icon="🤖"
      />
      <MetricCard
        title="Self-Healing Success Rate"
        :value="`${healingSuccessRate}%`"
        :trend="healingSuccessRate >= 90 ? 'High Reliability' : 'Review Errors'"
        :trend-type="healingSuccessRate >= 90 ? 'positive' : 'negative'"
        badge="AUTO-RESOLVED"
        badge-color="emerald"
        subtitle="Closed without human escalation"
        icon="🛡️"
      />
      <MetricCard
        title="Recent Event Log"
        :value="executions.length"
        badge="AUDITED"
        badge-color="violet"
        subtitle="Real-time execution records"
        icon="📜"
      />
    </div>

    <!-- Section 1: Automation Rules Management Table -->
    <div class="section-card glass-panel">
      <div class="section-top">
        <div>
          <h2 class="section-title">Workflow Automation Rules</h2>
          <span class="section-subtitle">Define automated reaction behaviors triggered by cluster telemetry thresholds</span>
        </div>
        <span class="badge badge-cyan">{{ rules.length }} Configured Rules</span>
      </div>

      <DataTable
        :columns="ruleColumns"
        :data="rules"
        :loading="loading"
        :error="error"
        searchable
        search-placeholder="Search rule name, trigger, or action..."
        empty-message="No automation rules configured. Create a rule to enable automated self-healing."
      >
        <template #cell-enabled="{ row }">
          <label class="toggle-switch">
            <input 
              type="checkbox" 
              :checked="row.enabled" 
              :disabled="togglingId === row.id"
              @change="handleToggleRule(row.id, !row.enabled)"
            />
            <span class="slider"></span>
          </label>
        </template>

        <template #cell-name="{ row }">
          <div class="rule-name-cell">
            <span class="rule-name">{{ row.name }}</span>
            <span class="rule-id font-mono text-muted">ID: #{{ row.id.slice(0, 8) }}</span>
          </div>
        </template>

        <template #cell-trigger_type="{ row }">
          <div class="trigger-cell font-mono">
            <span class="trigger-icon">{{ getTriggerIcon(row.trigger_type) }}</span>
            <span class="trigger-name">{{ formatType(row.trigger_type) }}</span>
          </div>
        </template>

        <template #cell-action_type="{ row }">
          <div class="action-cell font-mono">
            <span class="action-tag">{{ formatType(row.action_type) }}</span>
          </div>
        </template>

        <template #cell-executions="{ row }">
          <span class="font-mono text-emerald font-semibold">{{ row.executions || 0 }} runs</span>
        </template>

        <template #cell-last_triggered="{ row }">
          <span class="font-mono text-muted" style="font-size: 11px;">
            {{ row.last_triggered ? formatDate(row.last_triggered) : 'Never' }}
          </span>
        </template>

        <template #cell-actions="{ row }">
          <div class="actions-cell">
            <button 
              class="btn btn-primary btn-sm"
              :disabled="triggeringId === row.id"
              @click="handleTriggerRule(row)"
            >
              <span>{{ triggeringId === row.id ? '⚡ Running...' : '⚡ Trigger' }}</span>
            </button>
            <button 
              class="btn btn-secondary btn-sm"
              :disabled="deletingId === row.id"
              @click="handleDeleteRule(row.id)"
            >
              <span>{{ deletingId === row.id ? 'Deleting...' : '🗑️' }}</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Section 2: Rule Execution History Logs -->
    <div class="section-card glass-panel">
      <div class="section-top">
        <div>
          <h2 class="section-title">Rule Execution & Self-Healing Audit Trail</h2>
          <p class="section-subtitle">Real-time audit log of automated actions taken across cluster workloads</p>
        </div>
        <span class="badge badge-emerald">Live Telemetry Log</span>
      </div>

      <DataTable
        :columns="executionColumns"
        :data="executions"
        :loading="loading"
        searchable
        search-placeholder="Search execution log by rule, event, or result..."
        empty-message="No automated executions recorded yet."
      >
        <template #cell-result="{ row }">
          <StatusBadge :status="row.result === 'success' ? 'healthy' : 'failed'" :label="row.result.toUpperCase()" size="sm" />
        </template>

        <template #cell-rule_name="{ row }">
          <span class="font-mono font-semibold">{{ row.rule_name || `Rule #${row.rule_id.slice(0, 8)}` }}</span>
        </template>

        <template #cell-trigger_event="{ row }">
          <span class="font-mono text-amber">{{ row.trigger_event }}</span>
        </template>

        <template #cell-action_taken="{ row }">
          <span class="font-mono text-cyan">{{ row.action_taken }}</span>
        </template>

        <template #cell-created_at="{ row }">
          <span class="font-mono text-muted" style="font-size: 11px;">{{ formatDate(row.created_at) }}</span>
        </template>
      </DataTable>
    </div>

    <!-- Modal: Create Automation Rule -->
    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal = false">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge badge-cyan">AUTOMATION PIPELINE</span>
            <h3 class="modal-title">Create Workflow Automation Rule</h3>
          </div>
          <button class="modal-close" @click="showCreateModal = false">✕</button>
        </div>

        <form class="modal-body" @submit.prevent="handleCreateRule">
          <div class="form-group">
            <label class="form-label">Rule Name:</label>
            <input v-model="newRule.name" type="text" required class="input-glass" placeholder="e.g. Auto-Rollback on CrashLoopBackOff" />
          </div>

          <div class="form-group">
            <label class="form-label">Trigger Condition:</label>
            <select v-model="newRule.trigger_type" class="input-glass">
              <option value="deployment_failure">Deployment Failure / CrashLoop</option>
              <option value="pod_restart">Repeated Pod Restart Count > 5</option>
              <option value="node_pressure">Node Disk / Memory Pressure</option>
              <option value="high_cpu">High CPU Utilization (> 90%)</option>
              <option value="high_memory">High Memory Utilization (> 85%)</option>
              <option value="slo_breach">SLO / Latency SLA Breach</option>
              <option value="error_rate">HTTP 5xx Error Rate Spike</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">Automated Action:</label>
            <select v-model="newRule.action_type" class="input-glass">
              <option value="rollback">Rollback to Previous Stable Git Revision</option>
              <option value="generate_rca">Trigger AI Root Cause Analysis (RCA)</option>
              <option value="scale_deployment">Auto-Scale Deployment Replicas</option>
              <option value="restart_pod">Graceful Pod Rolling Restart</option>
              <option value="cordon_node">Cordon & Drain Impacted Node</option>
              <option value="send_notification">Send High-Priority Slack / Webhook Alert</option>
              <option value="create_incident">Open P1 Incident in Health Center</option>
            </select>
          </div>

          <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
            <button type="button" class="btn btn-secondary" @click="showCreateModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="loading">
              <span>{{ loading ? 'Creating...' : 'Save & Arm Automation' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import {
  automationApi,
  type AutomationRule,
  type AutomationExecution,
} from '../api/governance'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'

const rules = ref<AutomationRule[]>([])
const executions = ref<AutomationExecution[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)
const togglingId = ref<string | null>(null)
const deletingId = ref<string | null>(null)
const showCreateModal = ref(false)

const newRule = reactive({
  name: '',
  trigger_type: 'deployment_failure',
  trigger_config: {},
  action_type: 'rollback',
  action_config: {},
  enabled: true,
})

const ruleColumns: Column<AutomationRule>[] = [
  { key: 'enabled', label: 'State', width: '90px' },
  { key: 'name', label: 'Rule Name', width: '260px', sortable: true },
  { key: 'trigger_type', label: 'Trigger Event', width: '200px', sortable: true },
  { key: 'action_type', label: 'Automated Action', width: '220px', sortable: true },
  { key: 'executions', label: 'Executions', width: '130px', sortable: true },
  { key: 'last_triggered', label: 'Last Triggered', width: '160px', sortable: true },
  { key: 'actions', label: 'Actions', width: '120px', align: 'right' },
]

const executionColumns: Column<AutomationExecution>[] = [
  { key: 'result', label: 'Result', width: '110px', sortable: true },
  { key: 'rule_name', label: 'Rule Name', width: '240px', sortable: true },
  { key: 'trigger_event', label: 'Trigger Event', width: '220px' },
  { key: 'action_taken', label: 'Action Executed' },
  { key: 'created_at', label: 'Timestamp', width: '170px', sortable: true },
]

onMounted(() => {
  fetchAutomationData()
})

async function fetchAutomationData() {
  loading.value = true
  error.value = null
  try {
    const [rulesData, execData] = await Promise.all([
      automationApi.getRules(),
      automationApi.getExecutions(50, 0),
    ])
    rules.value = rulesData
    executions.value = execData.data
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load automation workflows'
    error.value = msg
  } finally {
    loading.value = false
  }
}

async function handleToggleRule(id: string, nextState: boolean) {
  togglingId.value = id
  statusMessage.value = null
  try {
    await automationApi.toggleRule(id, nextState)
    const rule = rules.value.find(r => r.id === id)
    if (rule) rule.enabled = nextState
    statusMessage.value = {
      type: 'success',
      text: `Rule #${id.slice(0, 8)} ${nextState ? 'enabled' : 'disabled'} successfully.`,
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to toggle rule state'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    togglingId.value = null
  }
}

async function handleCreateRule() {
  loading.value = true
  statusMessage.value = null
  try {
    await automationApi.createRule(newRule)
    statusMessage.value = {
      type: 'success',
      text: `Automation rule "${newRule.name}" created and armed.`,
    }
    showCreateModal.value = false
    newRule.name = ''
    await fetchAutomationData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to create automation rule'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    loading.value = false
  }
}

async function handleDeleteRule(id: string) {
  deletingId.value = id
  statusMessage.value = null
  try {
    await automationApi.deleteRule(id)
    statusMessage.value = {
      type: 'success',
      text: `Automation rule #${id.slice(0, 8)} deleted.`,
    }
    await fetchAutomationData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to delete automation rule'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    deletingId.value = null
  }
}

const triggeringId = ref<string | null>(null)

async function handleTriggerRule(rule: AutomationRule) {
  triggeringId.value = rule.id
  statusMessage.value = null
  try {
    const res = await automationApi.triggerRule(rule.id)
    statusMessage.value = {
      type: 'success',
      text: `Automation rule "${rule.name}" triggered: ${res.action_taken}`,
    }
    await fetchAutomationData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to trigger automation rule'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    triggeringId.value = null
  }
}

const enabledRulesCount = computed(() => rules.value.filter(r => r.enabled).length)
const totalExecutionsCount = computed(() => {
  return rules.value.reduce((acc, r) => acc + (r.executions || 0), 0)
})

const healingSuccessRate = computed(() => {
  if (executions.value.length === 0) return 100
  const successes = executions.value.filter(e => e.result === 'success').length
  return Math.round((successes / executions.value.length) * 100)
})

function getTriggerIcon(t: string): string {
  const tr = (t || '').toLowerCase()
  if (tr.includes('restart') || tr.includes('crash')) return '🔄'
  if (tr.includes('pressure') || tr.includes('node')) return '🛑'
  if (tr.includes('cpu') || tr.includes('memory')) return '📈'
  if (tr.includes('slo') || tr.includes('error')) return '🚨'
  return '⚡'
}

function formatType(t: string): string {
  if (!t) return 'UNKNOWN'
  return t.replace(/_/g, ' ').toUpperCase()
}

function formatDate(d: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleString()
  } catch {
    return d
  }
}
</script>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 16px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 820px;
  margin-top: 4px;
}

.highlight {
  color: #fff;
  font-weight: 600;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.status-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 18px;
  border-radius: 12px;
  font-size: 13px;
}

.banner-success {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.banner-error {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fda4af;
}

.banner-icon {
  font-size: 16px;
}

.banner-text {
  flex: 1;
  font-weight: 500;
}

.banner-close {
  background: none;
  border: none;
  color: inherit;
  font-size: 14px;
  cursor: pointer;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.section-card {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 20px;
}

.section-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 12px;
}

.section-title {
  font-size: 16px;
  color: #fff;
}

.section-subtitle {
  font-size: 12px;
  color: var(--text-muted);
}

.toggle-switch {
  position: relative;
  display: inline-block;
  width: 36px;
  height: 20px;
}

.toggle-switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  inset: 0;
  background-color: rgba(255, 255, 255, 0.15);
  transition: .3s cubic-bezier(0.4, 0, 0.2, 1);
  border-radius: 20px;
}

.slider:before {
  position: absolute;
  content: "";
  height: 14px;
  width: 14px;
  left: 3px;
  bottom: 3px;
  background-color: #fff;
  transition: .3s cubic-bezier(0.4, 0, 0.2, 1);
  border-radius: 50%;
}

input:checked + .slider {
  background-color: var(--accent-cyan);
}

input:checked + .slider:before {
  transform: translateX(16px);
}

.rule-name-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.rule-name {
  font-weight: 700;
  color: #fff;
  font-size: 13px;
}

.rule-id {
  font-size: 10px;
}

.trigger-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #fbbf24;
}

.action-tag {
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  background: rgba(6, 182, 212, 0.12);
  border: 1px solid rgba(6, 182, 212, 0.25);
  border-radius: 6px;
  color: var(--accent-sky);
}

.actions-cell {
  display: flex;
  justify-content: flex-end;
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 20px;
}

.modal-card {
  width: 100%;
  max-width: 560px;
  background: var(--bg-sidebar);
  border: 1px solid var(--border-medium);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.7);
}

.modal-header {
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  border-bottom: 1px solid var(--border-subtle);
}

.modal-title-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.modal-title {
  font-size: 18px;
  color: #fff;
}

.modal-close {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 18px;
  cursor: pointer;
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.modal-footer {
  padding: 16px 20px;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  border-top: 1px solid var(--border-subtle);
}

@media (max-width: 768px) {
  .view-header {
    flex-direction: column;
    align-items: stretch;
    gap: 16px;
  }
  .header-actions {
    display: flex !important;
    flex-direction: row !important;
    width: 100% !important;
    gap: 8px !important;
    flex-wrap: wrap !important;
  }
  .header-actions .btn,
  .header-actions button,
  .header-actions a {
    flex: 1 1 calc(50% - 4px) !important;
    min-width: 0 !important;
    padding: 7px 10px !important;
    font-size: 11.5px !important;
    white-space: nowrap !important;
    justify-content: center !important;
  }
  .header-actions > :last-child:nth-child(odd) {
    flex: 1 1 100% !important;
  }
  .metrics-grid,
  .grid-metrics,
  .stats-grid,
  .capacity-grid,
  .kpi-grid {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 8px !important;
  }
  .metric-card,
  .stat-card,
  .hud-card,
  :deep(.metric-card) {
    padding: 10px 12px !important;
  }
  .section-top {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }
  .modal-card {
    width: 100%;
    max-width: 100%;
  }
  .modal-footer {
    flex-direction: column;
    width: 100%;
  }
  .modal-footer .btn {
    width: 100%;
    justify-content: center;
  }
  .actions-cell {
    justify-content: flex-start;
  }
}

@media (max-width: 640px) {
  .header-actions {
    display: flex !important;
    flex-direction: row !important;
    width: 100% !important;
    gap: 8px !important;
    flex-wrap: wrap !important;
  }

  .header-actions .btn,
  .header-actions button,
  .header-actions a {
    flex: 1 1 calc(50% - 4px) !important;
    min-width: 0 !important;
    padding: 7px 10px !important;
    font-size: 11.5px !important;
    white-space: nowrap !important;
    justify-content: center !important;
  }

  .header-actions > :last-child:nth-child(odd) {
    flex: 1 1 100% !important;
  }

  .metrics-grid,
  .grid-metrics,
  .stats-grid,
  .capacity-grid,
  .kpi-grid {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 8px !important;
  }

  .metric-card,
  .stat-card,
  .hud-card,
  :deep(.metric-card) {
    padding: 10px 12px !important;
  }

  .section-card {
    padding: 14px;
  }
}
</style>
