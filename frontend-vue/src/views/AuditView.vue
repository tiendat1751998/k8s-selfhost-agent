<template>
  <div class="view-container audit-view">
    <!-- Top Security Posture Cockpit (Always-Visible Radial Scorecards + Severity Bar) -->
    <CompliancePostureHeader
      :frameworks="frameworks"
      :violations="violations"
      :findings="findings"
      :selected-framework-id="selectedFrameworkId"
      :active-severity="activeSeverity"
      :loading="complianceLoading || findingsLoading"
      :is-scanning="isScanning"
      @select-framework="selectFramework"
      @filter-severity="handleFilterSeverity"
      @trigger-scan="handleTriggerScan"
      @refresh="refreshAll"
    />

    <!-- Notification Banner -->
    <div
      v-if="statusMessage"
      class="audit-banner animate-fade-in"
      :class="statusMessage.type === 'success' ? 'banner-success' : 'banner-error'"
      role="alert"
    >
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" aria-label="Dismiss banner" @click="statusMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- 3 Segmented Sub-Views Switcher -->
    <div class="audit-tab-switcher" role="tablist" aria-label="Security Posture Sub-Views">
      <button
        type="button"
        class="audit-tab-btn"
        :class="{ active: activeTab === 'compliance' }"
        role="tab"
        :aria-selected="activeTab === 'compliance'"
        @click="switchTab('compliance')"
      >
        <BaseIcon name="shield" size="xs" />
        <span>Compliance & Controls ({{ filteredViolations.length }})</span>
      </button>
      <button
        type="button"
        class="audit-tab-btn"
        :class="{ active: activeTab === 'findings' }"
        role="tab"
        :aria-selected="activeTab === 'findings'"
        @click="switchTab('findings')"
      >
        <BaseIcon name="alert-triangle" size="xs" />
        <span>Container CVEs ({{ openFindingsCount }})</span>
      </button>
      <button
        type="button"
        class="audit-tab-btn"
        :class="{ active: activeTab === 'logs' }"
        role="tab"
        :aria-selected="activeTab === 'logs'"
        @click="switchTab('logs')"
      >
        <BaseIcon name="clock" size="xs" />
        <span>Forensic Audit Timeline ({{ filteredLogs.length }})</span>
      </button>
    </div>

    <!-- SUB-VIEW 1: COMPLIANCE & VIOLATIONS -->
    <section v-if="activeTab === 'compliance'" class="audit-subview-section" aria-label="Compliance Policy Controls">
      <div class="audit-desktop-view">
        <ComplianceControlsTable
          :violations="filteredViolations"
          :loading="complianceLoading"
          :error="complianceError"
          :selected-framework-name="selectedFrameworkName"
          :format-framework-tag="formatFrameworkTag"
          :format-date="formatDate"
          @clear-framework="selectFramework('')"
          @inspect="inspectControl"
          @remediate="handleRemediateViolation"
          @export="exportRemediationReport('markdown')"
        />
      </div>
      <div class="audit-mobile-view">
        <ComplianceMobileCards
          :violations="filteredViolations"
          @inspect="inspectControl"
          @remediate="handleRemediateViolation"
        />
      </div>
    </section>

    <!-- SUB-VIEW 2: CONTAINER CVES (TRIVY VULNERABILITIES) -->
    <section v-else-if="activeTab === 'findings'" class="audit-subview-section" aria-label="Container Vulnerabilities">
      <VulnerabilityFindingsTable
        :findings="displayedFindings"
        :loading="findingsLoading"
        :error="findingsError"
        :resolving-id="resolvingFindingId"
        @resolve="handleResolveFinding"
        @refresh="loadFindings"
      />
    </section>

    <!-- SUB-VIEW 3: FORENSIC AUDIT TIMELINE -->
    <section v-else-if="activeTab === 'logs'" class="audit-subview-section" aria-label="Forensic Audit Trail">
      <!-- Mobile 40px Command Bar (<640px) -->
      <div class="audit-mobile-command-bar mobile-only">
        <div class="command-bar-left">
          <span class="command-bar-title font-bold"><BaseIcon name="shield" size="xs" /> Audit Trail ({{ filteredLogs.length }})</span>
        </div>
        <div class="command-bar-actions">
          <button class="btn-icon-cmd" :disabled="isTriggeringScan" title="Trigger Audit Scan" aria-label="Trigger Audit Scan" @click="triggerAuditScan">
            <BaseIcon name="zap" size="xs" />
          </button>
          <button class="btn-icon-cmd" :disabled="isLoading" title="Refresh Trail" aria-label="Refresh Trail" @click="fetchLogs">
            <BaseIcon name="refresh" size="xs" />
          </button>
          <button class="btn-icon-cmd" :class="{ active: isLiveTailing }" title="Toggle Live Tail" aria-label="Toggle Live Tail" @click="toggleLiveTail">
            <BaseIcon name="radio" size="xs" />
          </button>
        </div>
      </div>

      <!-- Filter Toolbar -->
      <AuditFilterToolbar
        v-model:search-query="searchQuery"
        v-model:selected-action-type="selectedActionType"
        v-model:selected-severity="selectedSeverity"
        v-model:selected-actor="selectedActor"
        v-model:date-range="dateRange"
        :metrics="kpiMetrics"
        :unique-actors="uniqueActors"
        :action-types="actionTypeFilters"
        :is-live-tailing="isLiveTailing"
        :loading="isLoading"
        :triggering-scan="isTriggeringScan"
        @toggle-live-tail="toggleLiveTail"
        @export-json="exportToJson"
        @export-csv="exportToCsv"
        @reset-filters="resetFilters"
        @refresh="fetchLogs"
        @trigger-scan="triggerAuditScan"
      />

      <!-- Desktop Audit Trail Table -->
      <div class="audit-desktop-view">
        <AuditTrailTable :logs="filteredLogs" :loading="isLoading" :error="error" @select-payload="openPayloadDrawer" />
      </div>

      <!-- Mobile Card Stream -->
      <div class="audit-mobile-view">
        <AuditMobileCards :logs="filteredLogs" :loading="isLoading" @select-payload="openPayloadDrawer" />
      </div>

      <!-- Event Payload Drawer -->
      <AuditPayloadDrawer :event="selectedEvent" :open="isDrawerOpen" @close="closePayloadDrawer" />
    </section>

    <!-- Violation Inspection / Remediation Drawer -->
    <!-- Violation Inspection / Remediation Drawer -->
    <ComplianceViolationDrawer
      :violation="selectedViolation"
      :mode="modalMode"
      :format-framework-tag="formatFrameworkTag"
      @close="closeModal"
      @remediate="handleRemediateViolation"
    />

    <!-- Manual Remediation Action Drawer -->
    <RemediationActionDrawer
      v-model:show="showRemediationDrawer"
      :violation="remediationTargetViolation"
      :summary="remediationSummary"
      :commands="remediationCommands"
      @remediated="onManualRemediated"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import '../assets/styles/views/audit.css'
import '../assets/styles/views/compliance.css'
import { useAuditLogs } from '../composables/useAuditLogs'
import { useCompliance, type SeverityFilter, filterAuditFindings } from '../composables/useCompliance'
import BaseIcon from '../components/ui/BaseIcon.vue'
import AuditTrailTable from '../components/audit/AuditTrailTable.vue'
import AuditFilterToolbar from '../components/audit/AuditFilterToolbar.vue'
import AuditMobileCards from '../components/audit/AuditMobileCards.vue'
import AuditPayloadDrawer from '../components/audit/AuditPayloadDrawer.vue'
import VulnerabilityFindingsTable from '../components/audit/VulnerabilityFindingsTable.vue'
import CompliancePostureHeader from '../components/compliance/CompliancePostureHeader.vue'
import ComplianceControlsTable from '../components/compliance/ComplianceControlsTable.vue'
import ComplianceMobileCards from '../components/compliance/ComplianceMobileCards.vue'
import ComplianceViolationDrawer from '../components/compliance/ComplianceViolationDrawer.vue'
import RemediationActionDrawer from '../components/compliance/RemediationActionDrawer.vue'
import { auditApi, complianceApi, type AuditFinding, type ComplianceViolation } from '../api/governance'

const activeTab = ref<'compliance' | 'findings' | 'logs'>('compliance')
const findings = ref<AuditFinding[]>([])
const findingsLoading = ref(false)
const findingsError = ref<string | null>(null)
const resolvingFindingId = ref<string | null>(null)

// Remediation Drawer State
const showRemediationDrawer = ref(false)
const remediationTargetViolation = ref<ComplianceViolation | null>(null)
const remediationSummary = ref('')
const remediationCommands = ref<string[]>([])

function onManualRemediated() {
  fetchComplianceData()
  statusMessage.value = { type: 'success', text: 'Violation marked as remediated.' }
}

// Compliance Composable
const {
  frameworks, violations, loading: complianceLoading, isScanning, error: complianceError,
  selectedFrameworkId, activeSeverity, selectedViolation, modalMode, filteredViolations,
  fetchComplianceData, triggerComplianceScan, selectFramework, getSelectedFrameworkName,
  inspectControl, closeModal, exportRemediationReport, formatFrameworkTag, formatDate,
} = useCompliance()

// Audit Trail Composable
const {
  filteredLogs, isLoading, isTriggeringScan, error, statusMessage, selectedEvent,
  isDrawerOpen, isLiveTailing, searchQuery, selectedActionType, selectedSeverity,
  selectedActor, dateRange, uniqueActors, actionTypeFilters, kpiMetrics,
  fetchLogs, triggerAuditScan, toggleLiveTail, openPayloadDrawer, closePayloadDrawer,
  exportToJson, exportToCsv, resetFilters,
} = useAuditLogs()

const selectedFrameworkName = computed(() => getSelectedFrameworkName())
const openFindingsCount = computed(() => findings.value.filter(f => f.status !== 'resolved').length)

const displayedFindings = computed(() => {
  return filterAuditFindings(findings.value, activeSeverity.value, '')
})

async function loadFindings() {
  findingsLoading.value = true
  findingsError.value = null
  try {
    findings.value = await auditApi.getFindings('open')
  } catch (err: unknown) {
    findingsError.value = err instanceof Error ? err.message : 'Failed to load vulnerability findings'
  } finally {
    findingsLoading.value = false
  }
}

function switchTab(tab: 'compliance' | 'findings' | 'logs') {
  activeTab.value = tab
  if (tab === 'findings' && findings.value.length === 0) {
    loadFindings()
  }
}

function handleFilterSeverity(sev: SeverityFilter) {
  activeSeverity.value = sev
}

async function handleResolveFinding(id: string) {
  resolvingFindingId.value = id
  findingsError.value = null
  try {
    await auditApi.resolveFinding(id)
    findings.value = findings.value.map(f => f.id === id ? { ...f, status: 'resolved' } : f)
    statusMessage.value = { type: 'success', text: `Vulnerability finding [${id}] marked as resolved.` }
  } catch (err: unknown) {
    const errorMsg = err instanceof Error ? err.message : 'Failed to resolve finding'
    findingsError.value = errorMsg
    statusMessage.value = { type: 'error', text: `Failed to resolve finding: ${errorMsg}` }
  } finally {
    resolvingFindingId.value = null
  }
}

async function handleRemediateViolation(violation: ComplianceViolation) {
  try {
    const res = await complianceApi.remediateViolation(violation.id)
    if (res?.status === 'manual_action_required') {
      remediationTargetViolation.value = violation
      remediationSummary.value = res.summary || `Manual action required for ${violation.resource}.`
      remediationCommands.value = res.commands || []
      showRemediationDrawer.value = true
    } else {
      closeModal()
      await fetchComplianceData()
      statusMessage.value = { type: 'success', text: res?.message || `Policy remediation triggered for ${violation.resource}.` }
    }
  } catch (err: unknown) {
    const errorMsg = err instanceof Error ? err.message : 'Failed to remediate'
    statusMessage.value = { type: 'error', text: `Remediation failed: ${errorMsg}` }
  }
}

async function handleTriggerScan() {
  try {
    await triggerComplianceScan()
    await Promise.all([
      fetchComplianceData(),
      loadFindings(),
      fetchLogs(),
    ])
    const avgScore = frameworks.value.length > 0
      ? (frameworks.value.reduce((acc, f) => acc + f.score, 0) / frameworks.value.length).toFixed(1)
      : '100'
    const openFindings = openFindingsCount.value
    const openViolations = violations.value.filter(v => !v.resolved).length
    statusMessage.value = {
      type: 'success',
      text: `Live compliance scan evaluated: Frameworks at ${avgScore}%, ${openViolations} active violation(s), ${openFindings} open container CVE(s).`,
    }
  } catch (err: unknown) {
    const errorMsg = err instanceof Error ? err.message : 'Scan execution failed'
    statusMessage.value = { type: 'error', text: `Compliance scan failed: ${errorMsg}` }
  }
}

async function refreshAll() {
  await Promise.allSettled([fetchComplianceData(), loadFindings(), fetchLogs()])
}

onMounted(async () => {
  await Promise.allSettled([fetchComplianceData(), loadFindings(), fetchLogs()])
  if (frameworks.value.length > 0 || violations.value.length > 0) activeTab.value = 'compliance'
})
</script>
