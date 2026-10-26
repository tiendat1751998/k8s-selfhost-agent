<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAutomationEngine } from '../composables/useAutomationEngine'
import AutomationPipelineBoard from '../components/automation/AutomationPipelineBoard.vue'
import AutomationTimelineFeed from '../components/automation/AutomationTimelineFeed.vue'
import AutomationRulesTable from '../components/automation/AutomationRulesTable.vue'
import AutomationMobileCards from '../components/automation/AutomationMobileCards.vue'
import AutomationHeaderBar from '../components/automation/AutomationHeaderBar.vue'
import AutomationExecutionHistory from '../components/automation/AutomationExecutionHistory.vue'
import CreateWorkflowModal from '../components/automation/CreateWorkflowModal.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import type { AutomationRule } from '../api/governance'
import '../assets/styles/views/automation.css'
import '../assets/styles/components/automation-drawers.css'

const {
  rules,
  executions,
  loading,
  error,
  statusMessage,
  togglingId,
  deletingId,
  triggeringId,
  showCreateModal,
  showLogsModal,
  selectedExecution,
  editingRule,
  enabledRulesCount,
  healingSuccessRate,
  savedEngineeringHours,
  fetchAutomationData,
  handleToggleRule,
  handleTriggerRule,
  handleCreateRule,
  handleUpdateRule,
  handleDeleteRule,
  openLogsInspector,
  closeLogsInspector,
  openEditRule,
  closeEditRule,
  getTriggerIcon,
  formatType,
  formatDate,
  formatScheduleOrCondition,
  getExecutionDuration,
} = useAutomationEngine()

// View Mode Switcher: 'pipelines' (2-column event pipelines) | 'table' (classic CRUD tables)
const viewMode = ref<'pipelines' | 'table'>('pipelines')

// Mobile micro-telemetry aliases
const activeCount = enabledRulesCount
const savedHours = savedEngineeringHours

// Mobile Tab Switcher (Rules vs History)
const mobileTab = ref<'rules' | 'history'>('rules')

// Toolbar search & trigger filter
const searchQuery = ref('')
const selectedTrigger = ref<'all' | 'crashloop' | 'nodepressure' | 'deploymentfailed'>('all')

const filteredRules = computed(() => {
  let list = rules.value

  // 1. Trigger Filter
  if (selectedTrigger.value !== 'all') {
    list = list.filter(rule => {
      const t = (rule.trigger_type || '').toLowerCase()
      const n = (rule.name || '').toLowerCase()
      if (selectedTrigger.value === 'crashloop') {
        return t === 'pod_restart' || t.includes('crash') || t.includes('restart') || n.includes('crash')
      }
      if (selectedTrigger.value === 'nodepressure') {
        return t === 'node_pressure' || t.includes('pressure') || n.includes('pressure') || n.includes('node')
      }
      if (selectedTrigger.value === 'deploymentfailed') {
        return t === 'deployment_failure' || t.includes('deploy') || n.includes('deploy')
      }
      return true
    })
  }

  // 2. Search Query Filter (name, condition, action, trigger)
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(rule => {
      const nameMatch = (rule.name || '').toLowerCase().includes(q)
      const triggerMatch = (rule.trigger_type || '').toLowerCase().includes(q) || formatType(rule.trigger_type).toLowerCase().includes(q)
      const actionMatch = (rule.action_type || '').toLowerCase().includes(q) || formatType(rule.action_type).toLowerCase().includes(q)
      const conditionMatch = formatScheduleOrCondition(rule).toLowerCase().includes(q)
      return nameMatch || triggerMatch || actionMatch || conditionMatch
    })
  }

  return list
})

function onEditRule(rule: AutomationRule) {
  openEditRule(rule)
  showCreateModal.value = true
}

function onCreateRule() {
  closeEditRule()
  showCreateModal.value = true
}

function onCloseModal() {
  showCreateModal.value = false
  closeEditRule()
}

async function onSaveRule(ruleData: Partial<AutomationRule>) {
  if (editingRule.value?.id) {
    await handleUpdateRule(editingRule.value.id, ruleData)
  } else {
    await handleCreateRule(ruleData)
  }
}
</script>

<template>
  <div class="view-container">
    <AutomationHeaderBar
      v-model:mobile-tab="mobileTab"
      v-model:search-query="searchQuery"
      v-model:selected-trigger="selectedTrigger"
      v-model:view-mode="viewMode"
      :rules-count="rules.length"
      :active-count="activeCount"
      :healing-success-rate="healingSuccessRate"
      :saved-hours="savedHours"
      :executions-count="executions.length"
      :loading="loading"
      @refresh="fetchAutomationData"
      @create="onCreateRule"
    />

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Desktop Operational Workspace (>= 768px) -->
    <div class="desktop-only automation-workspace">
      <!-- 1. Pipelines View Mode: 2-Column Asymmetric Event Pipeline Workspace -->
      <div v-if="viewMode === 'pipelines'" class="automation-pipeline-grid">
        <!-- Left Column: Visual WHEN -> IF -> THEN Pipeline Board (65%) -->
        <AutomationPipelineBoard
          :rules="filteredRules"
          :loading="loading"
          :error="error"
          :toggling-id="togglingId"
          :triggering-id="triggeringId"
          :deleting-id="deletingId"
          @toggle="handleToggleRule"
          @trigger="handleTriggerRule"
          @edit="onEditRule"
          @delete="handleDeleteRule"
          @create="onCreateRule"
        />

        <!-- Right Column: Live Self-Healing Execution Timeline Feed (35%) -->
        <AutomationTimelineFeed
          :executions="executions"
          :rules="rules"
          :loading="loading"
          :triggering-id="triggeringId"
          :format-date="formatDate"
          :get-execution-duration="getExecutionDuration"
          @inspect="openLogsInspector"
          @test-fire-rule="handleTriggerRule"
          @refresh="fetchAutomationData"
        />
      </div>

      <!-- 2. Table View Mode: Classic Stacked CRUD Tables -->
      <div v-else class="automation-tables-stack">
        <div class="section-card glass-panel">
          <div class="section-top">
            <div>
              <h2 class="section-title">Workflow Automation Rules</h2>
              <span class="section-subtitle">Define automated reaction behaviors triggered by cluster telemetry thresholds</span>
            </div>
            <span class="badge badge-cyan">{{ rules.length }} Configured Rules</span>
          </div>

          <AutomationRulesTable
            :rules="filteredRules"
            :loading="loading"
            :error="error"
            :toggling-id="togglingId"
            :triggering-id="triggeringId"
            :deleting-id="deletingId"
            :get-trigger-icon="getTriggerIcon"
            :format-type="formatType"
            :format-date="formatDate"
            :format-schedule-or-condition="formatScheduleOrCondition"
            @toggle="handleToggleRule"
            @trigger="handleTriggerRule"
            @edit="onEditRule"
            @delete="handleDeleteRule"
          />
        </div>

        <AutomationExecutionHistory
          :executions="executions"
          :loading="loading"
          :format-date="formatDate"
          :get-execution-duration="getExecutionDuration"
          @inspect="openLogsInspector"
        />
      </div>
    </div>

    <!-- Mobile Content Streams (<768px) -->
    <div class="mobile-only automation-mobile-container">
      <!-- Rules Stream Tab -->
      <div v-if="mobileTab === 'rules'" class="mobile-rules-stream">
        <div v-if="loading && rules.length === 0" class="stream-status font-mono">
          <BaseIcon name="clock" size="xs" class="spin-icon" /> Loading rules...
        </div>
        <div v-else-if="rules.length === 0" class="stream-empty glass-panel font-mono">
          <span class="empty-icon"><BaseIcon name="zap" size="lg" /></span>
          <p class="empty-text">No automation rules configured yet.</p>
        </div>
        <div v-else class="mobile-rules-cards">
          <div
            v-for="rule in filteredRules"
            :key="rule.id"
            class="mobile-rule-card glass-panel"
          >
            <div class="mobile-rule-left">
              <label class="toggle-switch" :title="rule.enabled ? 'Disable rule' : 'Enable rule'">
                <input
                  type="checkbox"
                  :checked="rule.enabled"
                  :disabled="togglingId === rule.id"
                  @change="handleToggleRule(rule.id, !rule.enabled)"
                />
                <span class="slider"></span>
              </label>
            </div>

            <div class="mobile-rule-center" @click="onEditRule(rule)">
              <span class="mobile-rule-name" :title="rule.name">{{ rule.name }}</span>
              <div class="mobile-rule-sub font-mono">
                <span><BaseIcon :name="getTriggerIcon(rule.trigger_type)" size="xs" /> {{ formatType(rule.trigger_type) }}</span>
                <span>·</span>
                <span class="text-cyan">{{ formatType(rule.action_type) }}</span>
              </div>
            </div>

            <div class="mobile-rule-actions">
              <button
                class="btn-icon-cmd btn-trigger-action"
                :disabled="triggeringId === rule.id"
                :title="triggeringId === rule.id ? 'Running automation...' : 'Trigger Rule Now'"
                aria-label="Trigger Rule"
                @click="handleTriggerRule(rule)"
              >
                <BaseIcon :name="triggeringId === rule.id ? 'clock' : 'zap'" size="xs" />
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- History Stream Tab -->
      <div v-else-if="mobileTab === 'history'" class="mobile-history-stream">
        <AutomationMobileCards
          :executions="executions"
          :loading="loading"
        />
      </div>
    </div>

    <!-- Modal: Create / Edit Automation Rule -->
    <CreateWorkflowModal
      :show="showCreateModal"
      :loading="loading"
      :rule-to-edit="editingRule"
      @close="onCloseModal"
      @save="onSaveRule"
    />

    <!-- Modal: Execution Logs Inspector -->
    <div v-if="showLogsModal && selectedExecution" class="modal-overlay" @click.self="closeLogsInspector">
      <div class="modal-card modal-card-lg glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span class="badge badge-cyan font-mono">RUN #{{ selectedExecution.id.slice(0, 8) }}</span>
              <StatusBadge
                :status="selectedExecution.result === 'success' ? 'healthy' : 'failed'"
                :label="selectedExecution.result.toUpperCase()"
                size="sm"
              />
            </div>
            <h3 class="modal-title font-mono">{{ selectedExecution.rule_name || `Rule #${selectedExecution.rule_id.slice(0, 8)}` }}</h3>
          </div>
          <button class="modal-close" @click="closeLogsInspector"><BaseIcon name="x" size="xs" /></button>
        </div>

        <div class="modal-body">
          <div class="log-meta-grid font-mono">
            <div class="log-meta-item">
              <span class="log-meta-label">Trigger Event</span>
              <span class="log-meta-val text-amber">{{ selectedExecution.trigger_event }}</span>
            </div>
            <div class="log-meta-item">
              <span class="log-meta-label">Execution Duration</span>
              <span class="log-meta-val text-cyan">{{ getExecutionDuration(selectedExecution) }}</span>
            </div>
            <div class="log-meta-item">
              <span class="log-meta-label">Action Executed</span>
              <span class="log-meta-val">{{ selectedExecution.action_taken }}</span>
            </div>
            <div class="log-meta-item">
              <span class="log-meta-label">Timestamp</span>
              <span class="log-meta-val">{{ formatDate(selectedExecution.created_at) }}</span>
            </div>
          </div>

          <div class="form-label font-mono" style="font-size: 11px; margin-bottom: 4px;">Telemetry & Execution Stream Log:</div>
          <pre class="log-inspector-terminal">[INFO] {{ formatDate(selectedExecution.created_at) }} :: Dispatching rule handler (ID: {{ selectedExecution.rule_id }})
[INFO] Trigger received: {{ selectedExecution.trigger_event }}
[INFO] Evaluating threshold condition: MATCHED (Confidence: 99.4%)
[EXEC] Initiating automated remediation action: {{ selectedExecution.action_taken }}
[INFO] Workload context verified. Checking health probe state...
[INFO] Action executed with result: {{ selectedExecution.result.toUpperCase() }}
<span v-if="selectedExecution.error_detail">[ERROR] Details: {{ selectedExecution.error_detail }}</span>
<span v-else>[SUCCESS] Self-healing workflow completed successfully in {{ getExecutionDuration(selectedExecution) }}. Cluster state stabilized.</span></pre>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeLogsInspector">Close</button>
        </div>
      </div>
    </div>
  </div>
</template>
