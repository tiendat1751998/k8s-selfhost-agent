<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAutomationEngine } from '../composables/useAutomationEngine'
import AutomationRulesTable from '../components/automation/AutomationRulesTable.vue'
import AutomationMobileCards from '../components/automation/AutomationMobileCards.vue'
import AutomationExecutionHistory from '../components/automation/AutomationExecutionHistory.vue'
import CreateWorkflowModal from '../components/automation/CreateWorkflowModal.vue'
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
  editingRule,
  enabledRulesCount,
  savedEngineeringHours,
  fetchAutomationData,
  handleToggleRule,
  handleTriggerRule,
  handleCreateRule,
  handleUpdateRule,
  handleDeleteRule,
  openLogsInspector,
  openEditRule,
  closeEditRule,
  getTriggerIcon,
  formatType,
  formatDate,
  formatScheduleOrCondition,
  getExecutionDuration,
} = useAutomationEngine()

// Mobile micro-telemetry aliases
const activeCount = enabledRulesCount
const savedHours = savedEngineeringHours

// Mobile Tab Switcher (Rules vs History)
const mobileTab = ref<'rules' | 'history'>('rules')

// Toolbar search & trigger filter
const searchQuery = ref('')
const selectedTrigger = ref<'all' | 'crashloop' | 'nodepressure' | 'deploymentfailed'>('all')

const triggerFilterPills = [
  { key: 'all', label: 'All', icon: 'zap' },
  { key: 'crashloop', label: 'CrashLoop', icon: 'refresh' },
  { key: 'nodepressure', label: 'NodePressure', icon: 'shield' },
  { key: 'deploymentfailed', label: 'DeploymentFailed', icon: 'flame' },
] as const

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
    <!-- 44px Mobile Command Bar (<768px) -->
    <div class="mobile-command-bar automation-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="zap" size="xs" /> Automation ({{ rules.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh Automation Workflows"
          aria-label="Refresh"
          @click="fetchAutomationData"
        >
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          title="Create Automation Rule"
          aria-label="Create Rule"
          @click="onCreateRule"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
      </div>
    </div>

    <!-- 20px Mobile Micro-Telemetry Strip (<768px) -->
    <div class="mobile-micro-telemetry automation-micro-telemetry mobile-only font-mono" role="status" aria-label="Automation Micro Telemetry">
      <span class="tel-item tel-rules"><BaseIcon name="zap" size="xs" /> {{ rules.length }} rules</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-active"><BaseIcon name="check-circle" size="xs" /> {{ activeCount }} act</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-healed"><BaseIcon name="shield" size="xs" /> 100%</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-saved"><BaseIcon name="clock" size="xs" /> {{ savedHours }}h saved</span>
    </div>

    <!-- Mobile Segmented Tab Switcher (<768px) -->
    <div class="segmented-control mobile-segmented-control mobile-only font-mono">
      <button
        class="segmented-btn"
        :class="{ active: mobileTab === 'rules' }"
        @click="mobileTab = 'rules'"
      >
        <BaseIcon name="zap" size="xs" /> Rules ({{ rules.length }})
      </button>
      <button
        class="segmented-btn"
        :class="{ active: mobileTab === 'history' }"
        @click="mobileTab = 'history'"
      >
        <BaseIcon name="history" size="xs" /> History ({{ executions.length }})
      </button>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="automation-toolbar-sleek glass-panel desktop-only">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search rule, trigger, action..."
          class="toolbar-search-input"
          aria-label="Search rules by name, condition, action, or trigger"
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

      <!-- Trigger filter pills / tabs (All, CrashLoop, NodePressure, DeploymentFailed) -->
      <div class="toolbar-trigger-pills" role="tablist" aria-label="Trigger filters">
        <button
          v-for="pill in triggerFilterPills"
          :key="pill.key"
          type="button"
          role="tab"
          :aria-selected="selectedTrigger === pill.key"
          class="toolbar-pill-btn"
          :class="{ active: selectedTrigger === pill.key }"
          @click="selectedTrigger = pill.key"
        >
          <BaseIcon v-if="pill.icon" :name="pill.icon" size="xs" />
          <span>{{ pill.label }}</span>
        </button>
      </div>

      <!-- Inline compact execution badge strip font-mono -->
      <div class="toolbar-kpi-strip font-mono desktop-only" role="status" aria-label="Automation execution metrics">
        <span class="kpi-badge font-mono">{{ rules.length }} Rules ({{ enabledRulesCount }} Active · 100% Healed · {{ savedEngineeringHours }}h Saved)</span>
      </div>

      <!-- Action buttons: + Create Automation Rule (primary) and Refresh -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="btn btn-primary toolbar-btn"
          title="Create Automation Rule"
          aria-label="Create Automation Rule"
          @click="onCreateRule"
        >
          <BaseIcon name="plus" size="xs" />
          <span>+ Create Automation Rule</span>
        </button>
        <button
          type="button"
          class="btn btn-secondary toolbar-btn"
          title="Refresh automation workflows"
          aria-label="Refresh automation workflows"
          :disabled="loading"
          @click="fetchAutomationData"
        >
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" :class="{ 'spin-icon': loading }" />
          <span>{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
      </div>
    </div>

    <!-- Section 1: Automation Rules Management Table -->
    <div class="section-card glass-panel desktop-only">
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

    <!-- Section 2: Rule Execution History Logs -->
    <AutomationExecutionHistory
      class="desktop-only"
      :executions="executions"
      :loading="loading"
      :format-date="formatDate"
      :get-execution-duration="getExecutionDuration"
      @inspect="openLogsInspector"
    />

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
  </div>
</template>
