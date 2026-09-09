<template>
  <div class="view-container">
    <!-- Desktop View Header -->
    <div class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-emerald"></span>
          <span>REGULATORY & POSTURE GOVERNANCE</span>
        </div>
        <h1 class="view-title">Compliance Frameworks & Policy Violations</h1>
        <p class="view-desc">
          Continuous validation against industry standards (<span class="highlight">CIS Benchmark</span>, <span class="highlight">NIST SP 800-53</span>, <span class="highlight">PCI-DSS</span>, <span class="highlight">SOC 2</span>, <span class="highlight">HIPAA</span>) with automated remediation guidance.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading || isScanning" @click="fetchComplianceData">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Posture' }}</span>
        </button>
        <button class="btn btn-primary" :disabled="loading || isScanning" @click="triggerScan">
          <span>{{ isScanning ? '⚡ Scanning...' : '🚀 Run Audit Scan' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="compliance-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">⚖️ Compliance ({{ filteredFrameworks.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="loading || isScanning"
          title="Run Audit Scan"
          aria-label="Run Audit Scan"
          @click="triggerScan"
        >
          <span>🚀</span>
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading || isScanning"
          title="Refresh Posture"
          aria-label="Refresh Posture"
          @click="fetchComplianceData"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="compliance-micro-telemetry mobile-only font-mono" role="status" aria-label="Compliance Micro Telemetry">
      <span class="tel-item tel-score">⚖️ {{ overallScore }}% score</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-controls">🛡️ {{ passingControlsCount }}/{{ totalControlsCount }} controls</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-crit">🛑 {{ criticalViolationsCount }} crit</span>
    </div>

    <!-- Error Banner if any -->
    <div v-if="error" class="status-banner banner-error animate-fade-in">
      <span class="banner-icon">⚠️</span>
      <span class="banner-text">{{ error }}</span>
      <button class="banner-close" @click="error = null">✕</button>
    </div>

    <!-- Top HUD Cards (Overall Score, Passing Controls, Critical Failures, Automated Audit Status) -->
    <ComplianceScoreCards
      class="desktop-only"
      :overall-score="overallScore"
      :passing-controls="passingControlsCount"
      :total-controls="totalControlsCount"
      :critical-failures="criticalViolationsCount"
      :audit-status="auditStatus"
      :last-scan-time="latestRun?.start_time ? formatDate(latestRun.start_time) : undefined"
    />

    <!-- Frameworks Grid with Gauges and Quick Filters -->
    <ComplianceFrameworksGrid
      :frameworks="filteredFrameworks"
      :selected-framework-id="selectedFrameworkId"
      :selected-standard="selectedStandard"
      :loading="loading"
      :get-progress-color-class="getProgressColorClass"
      :format-date="formatDate"
      @select-framework="selectFramework"
      @select-standard="selectStandard"
      @run-scan="triggerScan"
    />

    <!-- Desktop Controls Table -->
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

    <!-- Mobile Cards Stream (~65px, 0 horizontal scroll) -->
    <div class="mobile-only">
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
          <button class="modal-close" @click="closeModal">✕</button>
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
            <div class="input-glass font-mono text-cyan">{{ selectedViolation.framework_id }}</div>
          </div>

          <div class="form-group">
            <label class="form-label">Automated Remediation Guidance:</label>
            <div class="input-glass remediation-box font-mono">
              1. Inspect manifest for {{ selectedViolation.resource }} in source Git repository.<br />
              2. Apply compliant securityContext specs and network policies matching CIS/NIST standards.<br />
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
            📥 Download Playbook
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import '../assets/styles/views/compliance.css'
import { useCompliance } from '../composables/useCompliance'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ComplianceScoreCards from '../components/compliance/ComplianceScoreCards.vue'
import ComplianceFrameworksGrid from '../components/compliance/ComplianceFrameworksGrid.vue'
import ComplianceControlsTable from '../components/compliance/ComplianceControlsTable.vue'
import ComplianceMobileCards from '../components/compliance/ComplianceMobileCards.vue'

const {
  loading,
  isScanning,
  error,
  selectedFrameworkId,
  selectedStandard,
  activeSeverity,
  auditStatus,
  latestRun,
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
