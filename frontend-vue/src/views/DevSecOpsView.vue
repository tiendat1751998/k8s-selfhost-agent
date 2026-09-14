<template>
  <div class="view-container">
    <!-- Sleek Unified 38px Enterprise Toolbar (.secops-toolbar-sleek) -->
    <div class="secops-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="DevSecOps Management Toolbar">
      <!-- Zone 1 (Left - Search) -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter CVE, package, secret..."
          class="toolbar-search-input"
          aria-label="Filter CVE, package, secret"
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

      <!-- Zone 2 (Center-Left - Capsule Tabs) -->
      <div class="toolbar-nav-pills" role="tablist" aria-label="DevSecOps Navigation Tabs">
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'vulnerabilities'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'vulnerabilities' }"
          @click="activeTab = 'vulnerabilities'"
        >
          <BaseIcon name="shield" size="xs" />
          <span>Vulnerabilities</span>
          <span class="pill-badge">{{ filteredCveFindings.length }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'secrets'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'secrets' }"
          @click="activeTab = 'secrets'"
        >
          <BaseIcon name="lock" size="xs" />
          <span>Exposed Secrets</span>
          <span class="pill-badge">{{ filteredSecretAudits.length }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'all'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'all' }"
          @click="activeTab = 'all'"
        >
          <BaseIcon name="layers" size="xs" />
          <span>All</span>
          <span class="pill-badge">{{ filteredCveFindings.length + filteredSecretAudits.length }}</span>
        </button>
      </div>

      <!-- Zone 3 (Center-Right - Inline KPI Strip) -->
      <div class="toolbar-kpi-strip font-mono" role="status" aria-label="DevSecOps Telemetry KPI summary">
        <span
          class="kpi-badge font-mono"
          :class="gatePassed ? 'kpi-badge-passed' : 'kpi-badge-restricted'"
        >
          [GATE {{ gatePassed ? 'PASSED' : 'RESTRICTED' }}] {{ criticalCveCount }} Crit · {{ exposedSecretsCount }} Secrets · {{ securityPostureScore }} Posture
        </span>
      </div>

      <!-- Zone 4 (Right - Actions) -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="toolbar-btn btn-secondary"
          :disabled="securityStore.loading || isScanning"
          title="Refresh Compliance"
          aria-label="Sync Compliance"
          @click="securityStore.fetchAll()"
        >
          <BaseIcon :name="securityStore.loading ? 'clock' : 'refresh'" size="xs" :class="{ 'spin-icon': securityStore.loading }" />
          <span>Sync</span>
        </button>
        <button
          type="button"
          class="toolbar-btn btn-primary"
          :disabled="securityStore.loading || isScanning"
          title="Run Full Security Audit"
          aria-label="Run Audit"
          @click="runSecurityScan"
        >
          <BaseIcon name="zap" size="xs" />
          <span>Run Audit</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="secops-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="shield" size="xs" /> DevSecOps ({{ filteredCveFindings.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="securityStore.loading || isScanning"
          title="Run Full Security Audit"
          aria-label="Run Full Security Audit"
          @click="runSecurityScan"
        >
          <BaseIcon name="zap" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="securityStore.loading || isScanning"
          title="Refresh Compliance"
          aria-label="Refresh Compliance"
          @click="securityStore.fetchAll()"
        >
          <BaseIcon name="refresh" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="secops-micro-telemetry mobile-only font-mono" role="status" aria-label="DevSecOps Micro Telemetry">
      <span class="tel-item tel-score"><BaseIcon name="shield" size="xs" /> {{ securityPostureScore }} score</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-crit"><BaseIcon name="flame" size="xs" /> {{ criticalCveCount }} crit</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-high"><BaseIcon name="alert-triangle" size="xs" /> {{ highCveCount }} high</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-secrets"><BaseIcon name="lock" size="xs" /> {{ exposedSecretsCount }} secr</span>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Desktop View: High-density Vulnerability Matrix & Secrets Grid -->
    <div class="desktop-only-table" style="min-width: 0; width: 100%; max-width: 100%; display: flex; flex-direction: column; gap: 24px; box-sizing: border-box;">
      <VulnerabilityScanTable
        v-if="activeTab === 'all' || activeTab === 'vulnerabilities'"
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
        v-if="activeTab === 'all' || activeTab === 'secrets'"
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
          <button class="modal-close" @click="selectedFinding = null"><BaseIcon name="x" size="xs" /></button>
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
            <BaseIcon name="shield" size="xs" /> <span>{{ isPatching ? 'Applying...' : 'Dispatch Automated Patch PR' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import '../assets/styles/views/secops.css'
import { useDevSecOps } from '../composables/useDevSecOps'
import BaseIcon from '../components/ui/BaseIcon.vue'
import VulnerabilityScanTable from '../components/secops/VulnerabilityScanTable.vue'
import SecretAuditGrid from '../components/secops/SecretAuditGrid.vue'
import SecOpsMobileCards from '../components/secops/SecOpsMobileCards.vue'

const activeTab = ref<'all' | 'vulnerabilities' | 'secrets'>('all')

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
  exposedSecretsCount,
  gatePassed,
  securityPostureScore,
  severities,
  getResourceIcon,
  getSeverityBadgeClass,
  getCvssBadgeClass,
  runSecurityScan,
  patchVulnerability,
  rotateSecret,
} = useDevSecOps()
</script>
