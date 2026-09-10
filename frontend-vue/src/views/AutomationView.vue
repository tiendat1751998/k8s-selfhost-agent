<script setup lang="ts">
import { ref } from 'vue'
import { useAutomationEngine } from '../composables/useAutomationEngine'
import AutomationHudCards from '../components/automation/AutomationHudCards.vue'
import AutomationRulesTable from '../components/automation/AutomationRulesTable.vue'
import AutomationMobileCards from '../components/automation/AutomationMobileCards.vue'
import AutomationExecutionHistory from '../components/automation/AutomationExecutionHistory.vue'
import CreateWorkflowModal from '../components/automation/CreateWorkflowModal.vue'
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
  executions24hCount,
  healingSuccessRate,
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
    <!-- Desktop View Header -->
    <header class="view-header desktop-header-wrap desktop-only">
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
        <button class="btn btn-primary" @click="onCreateRule">
          <span>+ Create Automation Rule</span>
        </button>
      </div>
    </header>

    <!-- 44px Mobile Command Bar (<768px) -->
    <div class="mobile-command-bar automation-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">⚡ Automation ({{ rules.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh Automation Workflows"
          aria-label="Refresh"
          @click="fetchAutomationData"
        >
          <span>{{ loading ? '⏳' : '🔄' }}</span>
        </button>
        <button
          class="btn-icon-cmd"
          title="Create Automation Rule"
          aria-label="Create Rule"
          @click="onCreateRule"
        >
          <span>➕</span>
        </button>
      </div>
    </div>

    <!-- 20px Mobile Micro-Telemetry Strip (<768px) -->
    <div class="mobile-micro-telemetry automation-micro-telemetry mobile-only font-mono" role="status" aria-label="Automation Micro Telemetry">
      <span class="tel-item tel-rules">⚡ {{ rules.length }} rules</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-active">🟢 {{ activeCount }} act</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-healed">🛡️ 100%</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-saved">⏱️ {{ savedHours }}h saved</span>
    </div>

    <!-- Mobile Segmented Tab Switcher (<768px) -->
    <div class="segmented-control mobile-segmented-control mobile-only font-mono">
      <button
        class="segmented-btn"
        :class="{ active: mobileTab === 'rules' }"
        @click="mobileTab = 'rules'"
      >
        ⚡ Rules ({{ rules.length }})
      </button>
      <button
        class="segmented-btn"
        :class="{ active: mobileTab === 'history' }"
        @click="mobileTab = 'history'"
      >
        📜 History ({{ executions.length }})
      </button>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Desktop Metrics HUD Grid -->
    <AutomationHudCards
      class="desktop-only"
      :rules-count="rules.length"
      :active-rules-count="enabledRulesCount"
      :executions24h-count="executions24hCount"
      :healing-success-rate="healingSuccessRate"
      :saved-engineering-hours="savedEngineeringHours"
    />

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
        :rules="rules"
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
          <span class="spin-icon">⏳</span> Loading rules...
        </div>
        <div v-else-if="rules.length === 0" class="stream-empty glass-panel font-mono">
          <span class="empty-icon">⚡</span>
          <p class="empty-text">No automation rules configured yet.</p>
        </div>
        <div v-else class="mobile-rules-cards">
          <div
            v-for="rule in rules"
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
                <span>{{ getTriggerIcon(rule.trigger_type) }} {{ formatType(rule.trigger_type) }}</span>
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
                <span>{{ triggeringId === rule.id ? '⏳' : '⚡' }}</span>
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
