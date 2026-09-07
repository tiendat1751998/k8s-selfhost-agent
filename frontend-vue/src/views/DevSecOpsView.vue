<template>
  <div class="view-container">
    <!-- Desktop View Header -->
    <div class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-emerald"></span>
          <span>DEVSECOPS SHIFT-LEFT & SECRETS GOVERNANCE</span>
        </div>
        <h1 class="view-title">Automated Security Gates, CVE Scanner & Vault Sync</h1>
        <p class="view-desc">
          Continuous Container Image Vulnerability Analysis (<span class="highlight">Trivy</span>), IaC Security & CIS Benchmarks (<span class="highlight">Checkov</span>), and Dynamic Secrets (<span class="highlight">HashiCorp Vault + ESO</span>).
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="securityStore.loading || isScanning" @click="securityStore.fetchAll()">
          <span>{{ securityStore.loading ? '⏳ Syncing...' : '🔄 Refresh Compliance' }}</span>
        </button>
        <button class="btn btn-primary" :disabled="securityStore.loading || isScanning" @click="runSecurityScan">
          <span>{{ isScanning ? '⚡ Auditing Cluster...' : '⚡ Run Full Security Audit' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="secops-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🛡️ DevSecOps ({{ filteredCveFindings.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="securityStore.loading || isScanning"
          title="Run Full Security Audit"
          aria-label="Run Full Security Audit"
          @click="runSecurityScan"
        >
          <span>⚡</span>
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="securityStore.loading || isScanning"
          title="Refresh Compliance"
          aria-label="Refresh Compliance"
          @click="securityStore.fetchAll()"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="secops-micro-telemetry mobile-only font-mono" role="status" aria-label="DevSecOps Micro Telemetry">
      <span class="tel-item tel-score">🛡️ {{ securityPostureScore }}% score</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-crit">🔥 {{ criticalCveCount }} crit</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-high">⚠️ {{ highCveCount }} high</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-secrets">🔑 {{ exposedSecretsCount }} secr</span>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Security HUD Metric Cards -->
    <SecurityScoreCards
      class="desktop-only"
      :posture-score="securityPostureScore"
      :passing-rules="passingRulesCount"
      :total-rules="totalRulesCount"
      :critical-cves="criticalCveCount"
      :gate-passed="gatePassed"
      :exposed-secrets="exposedSecretsCount"
      :total-violations="totalViolationsCount"
      :high-violations="highCveCount"
      :med-violations="mediumCveCount"
      :low-violations="lowCveCount"
      :frameworks-count="securityStore.frameworks.length"
      :framework-names="frameworkNames"
    />

    <!-- Desktop View: High-density Vulnerability Matrix & Secrets Grid -->
    <div class="desktop-only-table" style="display: flex; flex-direction: column; gap: 24px;">
      <VulnerabilityScanTable
        :findings="filteredCveFindings"
        :severities="severities"
        :active-filter="activeFilter"
        :search-query="searchQuery"
        :loading="securityStore.loading"
        :get-resource-icon="getResourceIcon"
        :get-severity-badge-class="getSeverityBadgeClass"
        :get-cvss-badge-class="getCvssBadgeClass"
        @update:active-filter="activeFilter = $event"
        @update:search-query="searchQuery = $event"
        @view-details="selectedFinding = $event"
        @patch-vulnerability="patchVulnerability"
      />

      <SecretAuditGrid
        :secrets="filteredSecretAudits"
        :loading="securityStore.loading"
        @rotate-secret="rotateSecret"
      />
    </div>

    <!-- Mobile View: Ultra-compact Zero-Horizontal-Scroll Stream (~65px/item) -->
    <div class="mobile-stream-container">
      <SecOpsMobileCards
        :findings="filteredCveFindings"
        :secrets="filteredSecretAudits"
        :get-resource-icon="getResourceIcon"
        :get-severity-badge-class="getSeverityBadgeClass"
        :get-cvss-badge-class="getCvssBadgeClass"
        @view-finding="selectedFinding = $event"
        @patch-finding="patchVulnerability"
        @rotate-secret="rotateSecret"
      />
    </div>

    <!-- Details & Remediation Modal -->
    <div v-if="selectedFinding" class="modal-overlay" @click.self="selectedFinding = null">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span class="badge" :class="getSeverityBadgeClass(selectedFinding.severity)">
                {{ selectedFinding.severity?.toUpperCase() }} SEVERITY
              </span>
              <span class="cvss-badge" :class="getCvssBadgeClass(selectedFinding.cvss_score)">
                CVSS {{ selectedFinding.cvss_score.toFixed(1) }}
              </span>
              <span class="scanner-tag">{{ selectedFinding.scanner }}</span>
            </div>
            <h3 class="modal-title">{{ selectedFinding.cve_id }}: {{ selectedFinding.resource_name }}</h3>
          </div>
          <button class="modal-close" @click="selectedFinding = null">✕</button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Resource Target & Namespace:</label>
            <div class="input-glass font-mono">
              {{ selectedFinding.resource_type }} / {{ selectedFinding.resource_name }} (Namespace: {{ selectedFinding.namespace }})
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Affected Package & Remediation Version:</label>
            <div class="input-glass font-mono">
              Package: <span class="text-cyan">{{ selectedFinding.package_name }}</span> | Installed: <span class="text-rose">{{ selectedFinding.installed_version }}</span> → Fixed: <span class="text-emerald">{{ selectedFinding.fixed_version }}</span>
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Vulnerability Analysis:</label>
            <div class="input-glass" style="white-space: pre-wrap;">
              {{ selectedFinding.description }}
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">CIS / NIST Remediation Guide:</label>
            <div class="input-glass font-mono remediation-box">
              {{ selectedFinding.remediation || 'Apply standard CIS hardening patch or update container base image.' }}
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Detected Timestamp:</label>
            <div class="input-glass font-mono text-muted">
              {{ selectedFinding.detected_at }}
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" @click="selectedFinding = null">Close</button>
          <button class="btn btn-primary btn-patch" :disabled="isPatching" @click="patchVulnerability(selectedFinding)">
            <span>{{ isPatching ? 'Applying...' : '🛡️ Dispatch Automated Patch PR' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import '../assets/styles/views/secops.css'
import { useDevSecOps } from '../composables/useDevSecOps'
import SecurityScoreCards from '../components/secops/SecurityScoreCards.vue'
import VulnerabilityScanTable from '../components/secops/VulnerabilityScanTable.vue'
import SecretAuditGrid from '../components/secops/SecretAuditGrid.vue'
import SecOpsMobileCards from '../components/secops/SecOpsMobileCards.vue'

const {
  securityStore,
  activeFilter,
  searchQuery,
  selectedFinding,
  statusMessage,
  isScanning,
  isPatching,
  filteredCveFindings,
  filteredSecretAudits,
  criticalCveCount,
  highCveCount,
  mediumCveCount,
  lowCveCount,
  totalViolationsCount,
  exposedSecretsCount,
  gatePassed,
  totalRulesCount,
  passingRulesCount,
  securityPostureScore,
  frameworkNames,
  severities,
  getResourceIcon,
  getSeverityBadgeClass,
  getCvssBadgeClass,
  runSecurityScan,
  patchVulnerability,
  rotateSecret,
} = useDevSecOps()
</script>
