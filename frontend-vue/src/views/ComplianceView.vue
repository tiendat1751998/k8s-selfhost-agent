<script setup lang="ts">
import { ref } from 'vue'
import '../assets/styles/views/compliance.css'
import '../assets/styles/components/compliance-drawers.css'
import { useCompliance } from '../composables/useCompliance'
import StatusBadge from '../components/ui/StatusBadge.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import ComplianceFrameworksGrid from '../components/compliance/ComplianceFrameworksGrid.vue'
import ComplianceControlsTable from '../components/compliance/ComplianceControlsTable.vue'
import ComplianceMobileCards from '../components/compliance/ComplianceMobileCards.vue'

const mobileTab = ref<'violations' | 'frameworks'>('violations')

const {
  loading,
  isScanning,
  error,
  selectedFrameworkId,
  selectedStandard,
  activeSeverity,
  selectedViolation,
  modalMode,
  overallScore,
  passingControlsCount,
  totalControlsCount,
  criticalViolationsCount,
  severityFilters,
  filteredFrameworks,
  filteredViolations,
  fetchComplianceData,
  triggerScan,
  selectFramework,
  selectStandard,
  getSelectedFrameworkName,
  inspectControl,
  remediateControl,
  closeModal,
  exportRemediationReport,
  getProgressColorClass,
  formatFrameworkTag,
  formatDate,
} = useCompliance()
</script>

<template>
  <div class="view-container">
    <!-- Mobile 44px Command Bar (<768px) -->
    <div class="compliance-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="shield" size="xs" /> Compliance ({{ filteredFrameworks.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="loading || isScanning"
          title="Run Audit Scan"
          aria-label="Run Audit Scan"
          @click="triggerScan"
        >
          <BaseIcon name="play" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading || isScanning"
          title="Refresh Posture"
          aria-label="Refresh Posture"
          @click="fetchComplianceData"
        >
          <BaseIcon name="refresh" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile 24px Centered Micro-Telemetry Strip (<768px) -->
    <div class="compliance-micro-telemetry mobile-only font-mono" role="status" aria-label="Compliance Micro Telemetry">
      <span class="tel-item tel-score"><BaseIcon name="shield" size="xs" /> {{ Math.round(overallScore) }}% score</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-controls"><BaseIcon name="shield" size="xs" /> {{ passingControlsCount }}/{{ totalControlsCount }} controls</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-crit"><BaseIcon name="alert-triangle" size="xs" /> {{ criticalViolationsCount }} crit</span>
    </div>

    <!-- Mobile Segmented View Tabs (<768px) -->
    <div class="compliance-mobile-tabs mobile-only" role="tablist" aria-label="Compliance View Selection">
      <button
        class="mobile-tab-btn"
        :class="{ 'mobile-tab-active': mobileTab === 'violations' }"
        role="tab"
        :aria-selected="mobileTab === 'violations'"
        @click="mobileTab = 'violations'"
      >
        <BaseIcon name="alert-triangle" size="xs" /> <span>Violations</span>
        <span class="tab-badge">{{ filteredViolations.length }}</span>
      </button>
      <button
        class="mobile-tab-btn"
        :class="{ 'mobile-tab-active': mobileTab === 'frameworks' }"
        role="tab"
        :aria-selected="mobileTab === 'frameworks'"
        @click="mobileTab = 'frameworks'"
      >
        <BaseIcon name="shield" size="xs" /> <span>Frameworks</span>
        <span class="tab-badge">{{ filteredFrameworks.length }}</span>
      </button>
    </div>

    <!-- Error Banner if any -->
    <div v-if="error" class="status-banner banner-error animate-fade-in">
      <BaseIcon name="alert-triangle" size="xs" class="banner-icon" />
      <span class="banner-text">{{ error }}</span>
      <button class="banner-close" @click="error = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Frameworks Sleek Filter Strip -->
    <div class="frameworks-container" :class="{ 'mobile-hidden': mobileTab !== 'frameworks' }">
      <ComplianceFrameworksGrid
        :frameworks="filteredFrameworks"
        :selected-framework-id="selectedFrameworkId"
        :selected-standard="selectedStandard"
        :total-controls-count="totalControlsCount"
        :loading="loading || isScanning"
        :get-progress-color-class="getProgressColorClass"
        :format-date="formatDate"
        @select-framework="selectFramework"
        @select-standard="selectStandard"
        @run-scan="triggerScan"
        @refresh="fetchComplianceData"
      />
    </div>

    <!-- Desktop Controls Table (>=768px) -->
    <div class="desktop-only">
      <ComplianceControlsTable
        :violations="filteredViolations"
        :loading="loading"
        :error="error"
        :active-severity="activeSeverity"
        :severity-filters="severityFilters"
        :selected-framework-name="getSelectedFrameworkName()"
        :format-framework-tag="formatFrameworkTag"
        :format-date="formatDate"
        @filter-severity="activeSeverity = $event"
        @clear-framework="selectedFrameworkId = ''"
        @inspect="inspectControl"
        @remediate="remediateControl"
        @export="exportRemediationReport('markdown')"
      />
    </div>

    <!-- Mobile Cards Stream (~72px, 0 horizontal scroll, prominent on mobile load) -->
    <div class="mobile-only" :class="{ 'mobile-hidden': mobileTab !== 'violations' }">
      <ComplianceMobileCards
        :violations="filteredViolations"
        @inspect="inspectControl"
        @remediate="remediateControl"
      />
    </div>

    <!-- Inspection / Remediation Modal -->
    <div v-if="selectedViolation" class="modal-overlay" @click.self="closeModal">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <StatusBadge
              :status="selectedViolation.severity"
              :label="`${selectedViolation.severity.toUpperCase()} SEVERITY`"
            />
            <h3 class="modal-title">{{ selectedViolation.policy }}</h3>
          </div>
          <button class="modal-close" aria-label="Close modal" @click="closeModal"><BaseIcon name="x" size="xs" /></button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Impacted Resource:</label>
            <div class="input-glass font-mono">
              {{ selectedViolation.resource }} (Namespace: {{ selectedViolation.namespace || 'default' }} | Cluster: {{ selectedViolation.cluster || 'primary' }})
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Finding Description:</label>
            <div class="input-glass">{{ selectedViolation.message }}</div>
          </div>

          <div class="form-group">
            <label class="form-label">Regulatory Framework Control:</label>
            <div class="input-glass font-mono text-cyan">{{ formatFrameworkTag(selectedViolation.framework_id) }}</div>
          </div>

          <div class="form-group">
            <label class="form-label">Automated Remediation Guidance:</label>
            <div class="input-glass remediation-box font-mono">
              1. Inspect manifest for {{ selectedViolation.resource }} in source Git repository.<br />
              2. Apply compliant securityContext specs and network policies matching regulatory standards.<br />
              3. Trigger continuous GitOps reconciliation to satisfy audit posture.
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeModal">Close</button>
          <button
            v-if="modalMode === 'remediate'"
            class="btn btn-primary"
            @click="exportRemediationReport('markdown'); closeModal()"
          >
            <BaseIcon name="download" size="xs" /> <span>Download Playbook</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
