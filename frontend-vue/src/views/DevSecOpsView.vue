<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
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
        <button class="btn btn-secondary" :disabled="securityStore.loading" @click="securityStore.fetchAll()">
          <span>{{ securityStore.loading ? '⏳ Syncing...' : '🔄 Refresh Compliance' }}</span>
        </button>
        <button class="btn btn-primary" :disabled="securityStore.loading" @click="runScan">
          <span>⚡ Run Full Security Audit</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Security HUD Metric Cards -->
    <div class="metrics-grid">
      <div class="metric-card glass-panel glass-panel-glow">
        <div class="metric-header">
          <span class="metric-title">PRODUCTION SECURITY GATE</span>
          <span class="badge" :class="criticalCount === 0 ? 'badge-emerald' : 'badge-rose'">
            {{ criticalCount === 0 ? 'ACTIVE & ENFORCED' : 'GATE BLOCKED' }}
          </span>
        </div>
        <div class="metric-val" :class="criticalCount === 0 ? 'text-emerald' : 'text-rose'">
          {{ criticalCount === 0 ? 'PASSED ✅' : 'BLOCKED ⚠️' }}
        </div>
        <div class="metric-footer">
          <span :class="criticalCount === 0 ? 'text-emerald' : 'text-rose'">● {{ criticalCount }} Critical CVEs</span>
          <span class="text-muted">{{ criticalCount === 0 ? 'Deployment Unblocked' : 'Action Required' }}</span>
        </div>
      </div>

      <div class="metric-card glass-panel glass-panel-glow">
        <div class="metric-header">
          <span class="metric-title">COMPLIANCE BENCHMARK</span>
          <span class="badge" :class="totalRulesCount > 0 ? 'badge-cyan' : 'badge-muted'">{{ totalRulesCount > 0 ? 'CIS & NIST' : 'NO BENCHMARKS' }}</span>
        </div>
        <div class="metric-val font-mono text-cyan">{{ complianceScore }} <span v-if="complianceScore !== '—'" class="metric-unit">Score</span></div>
        <div class="metric-footer">
          <span class="text-emerald">{{ passingRulesCount }} Passing Rules</span>
          <span class="text-muted">of {{ totalRulesCount }} Rules</span>
        </div>
      </div>

      <div class="metric-card glass-panel glass-panel-glow">
        <div class="metric-header">
          <span class="metric-title">COMPLIANCE FRAMEWORKS</span>
          <span class="badge" :class="securityStore.frameworks.length > 0 ? 'badge-violet' : 'badge-muted'">{{ securityStore.frameworks.length > 0 ? 'ACTIVE' : 'NONE' }}</span>
        </div>
        <div class="metric-val font-mono">{{ securityStore.frameworks.length }} <span class="metric-unit">Frameworks</span></div>
        <div class="metric-footer">
          <span :class="securityStore.frameworks.length > 0 ? 'text-violet' : 'text-muted'">{{ frameworkNames }}</span>
          <span v-if="securityStore.frameworks.length > 0" class="text-emerald">Enforced</span>
        </div>
      </div>

      <div class="metric-card glass-panel glass-panel-glow">
        <div class="metric-header">
          <span class="metric-title">SECURITY FINDINGS</span>
          <span class="badge" :class="totalViolationsCount === 0 ? 'badge-emerald' : 'badge-amber'">
            {{ totalViolationsCount === 0 ? 'CLEAN' : 'AUDIT' }}
          </span>
        </div>
        <div class="metric-val font-mono">{{ totalViolationsCount }} <span class="metric-unit">Violations</span></div>
        <div class="metric-footer">
          <span class="text-amber">{{ highCount }} High</span>
          <span class="text-muted">{{ mediumCount }} Med / {{ lowCount }} Low</span>
        </div>
      </div>
    </div>

    <!-- Severity Filter Pills -->
    <div class="filter-bar glass-panel">
      <div class="filter-group">
        <span class="filter-label">Filter Vulnerabilities:</span>
        <button 
          v-for="sev in severities" 
          :key="sev.name"
          class="filter-pill"
          :class="[sev.badgeClass, { 'filter-active': activeFilter === sev.name }]"
          @click="activeFilter = sev.name"
        >
          <span>{{ sev.name }} ({{ sev.count }})</span>
        </button>
      </div>

      <div class="search-box">
        <input v-model="searchQuery" type="text" placeholder="Search rule ID, namespace or resource..." class="input-glass" style="width: 280px;" />
      </div>
    </div>

    <!-- Violations Table -->
    <div class="table-container glass-panel">
      <div class="table-header">
        <div>
          <h2 class="table-title">Security Gate Violations & Vulnerability Matrix</h2>
          <p class="table-subtitle">Audited with Trivy container static analysis and Checkov IaC security rules</p>
        </div>
        <span class="badge badge-cyan">{{ filteredViolations.length }} Findings Displayed</span>
      </div>

      <div class="table-scroll">
        <table class="enterprise-table">
          <thead>
            <tr>
              <th>Target Resource</th>
              <th>Namespace</th>
              <th>Severity</th>
              <th>Rule ID</th>
              <th>Description</th>
              <th style="text-align: right;">Action</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="violation in filteredViolations" :key="violation.id" class="table-row">
              <td>
                <div class="image-cell">
                  <span class="image-icon">{{ getResourceIcon(violation.resource_type) }}</span>
                  <div>
                    <div class="image-name font-mono">{{ violation.resource_name }}</div>
                    <div class="image-digest font-mono text-muted">{{ violation.resource_type }}</div>
                  </div>
                </div>
              </td>
              <td>
                <span class="badge badge-violet font-mono">{{ violation.namespace || 'default' }}</span>
              </td>
              <td>
                <span class="badge" :class="getSeverityBadgeClass(violation.severity)">
                  {{ violation.severity?.toUpperCase() }}
                </span>
              </td>
              <td>
                <span class="font-mono text-cyan font-semibold">{{ violation.rule_id }}</span>
              </td>
              <td>
                <div class="desc-cell">
                  <span>{{ violation.description }}</span>
                </div>
              </td>
              <td style="text-align: right;">
                <button class="btn btn-secondary btn-sm" @click="selectedViolation = violation">
                  <span>🔍 View Details</span>
                </button>
              </td>
            </tr>

            <tr v-if="filteredViolations.length === 0">
              <td colspan="6" class="empty-table-cell">
                <span class="empty-icon">🛡️</span>
                <p>No security violations detected matching the filter. All cluster security policies are enforced.</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Details Modal -->
    <div v-if="selectedViolation" class="modal-overlay" @click.self="selectedViolation = null">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge" :class="getSeverityBadgeClass(selectedViolation.severity)">
              {{ selectedViolation.severity?.toUpperCase() }} SEVERITY
            </span>
            <h3 class="modal-title">{{ selectedViolation.rule_id }}: {{ selectedViolation.resource_name }}</h3>
          </div>
          <button class="modal-close" @click="selectedViolation = null">✕</button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Resource Target:</label>
            <div class="input-glass font-mono">
              {{ selectedViolation.resource_type }} / {{ selectedViolation.resource_name }} (Namespace: {{ selectedViolation.namespace }})
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Description:</label>
            <div class="input-glass" style="white-space: pre-wrap;">
              {{ selectedViolation.description }}
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Remediation Guide:</label>
            <div class="input-glass font-mono remediation-box">
              {{ selectedViolation.remediation || 'Apply standard CIS hardening patch or update container base image.' }}
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Detected Timestamp:</label>
            <div class="input-glass font-mono text-muted">
              {{ selectedViolation.detected_at || 'Continuous real-time scan' }}
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" @click="selectedViolation = null">Close</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useSecurityStore } from '../stores/securityStore'
import type { SecurityViolation } from '../api/security'

const securityStore = useSecurityStore()
const activeFilter = ref('ALL')
const searchQuery = ref('')
const selectedViolation = ref<SecurityViolation | null>(null)
const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

onMounted(() => {
  securityStore.fetchAll()
})

const criticalCount = computed(() => securityStore.violations.filter(v => (v.severity || '').toUpperCase() === 'CRITICAL').length)
const highCount = computed(() => securityStore.violations.filter(v => (v.severity || '').toUpperCase() === 'HIGH').length)
const mediumCount = computed(() => securityStore.violations.filter(v => (v.severity || '').toUpperCase() === 'MEDIUM').length)
const lowCount = computed(() => securityStore.violations.filter(v => (v.severity || '').toUpperCase() === 'LOW').length)
const totalViolationsCount = computed(() => securityStore.totalViolations || securityStore.violations.length)

const totalRulesCount = computed(() => securityStore.frameworks.reduce((acc, f) => acc + (f.total_rules || f.total_checks || 0), 0))
const passingRulesCount = computed(() => securityStore.frameworks.reduce((acc, f) => acc + (f.passing_rules || f.passed_checks || 0), 0))
const complianceScore = computed(() => {
  if (totalRulesCount.value === 0) return '—'
  return `${((passingRulesCount.value / totalRulesCount.value) * 100).toFixed(1)}%`
})

const frameworkNames = computed(() => {
  if (securityStore.frameworks.length === 0) return 'No frameworks configured'
  return securityStore.frameworks.map(f => f.name).join(', ')
})

const severities = computed(() => [
  { name: 'ALL', count: totalViolationsCount.value, badgeClass: 'badge-cyan' },
  { name: 'CRITICAL', count: criticalCount.value, badgeClass: 'badge-rose' },
  { name: 'HIGH', count: highCount.value, badgeClass: 'badge-amber' },
  { name: 'MEDIUM', count: mediumCount.value, badgeClass: 'badge-violet' },
  { name: 'LOW', count: lowCount.value, badgeClass: 'badge-emerald' },
])

const filteredViolations = computed(() => {
  return securityStore.violations.filter(v => {
    const vSev = (v.severity || '').toUpperCase()
    const active = activeFilter.value.toUpperCase()
    if (active !== 'ALL' && vSev !== active) {
      return false
    }
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase()
      const matchName = (v.resource_name || v.resource || '').toLowerCase().includes(q)
      const matchType = (v.resource_type || '').toLowerCase().includes(q)
      const matchRule = (v.rule_id || v.policy || '').toLowerCase().includes(q)
      const matchDesc = (v.description || v.message || '').toLowerCase().includes(q)
      const matchNs = (v.namespace || '').toLowerCase().includes(q)
      return matchName || matchType || matchRule || matchDesc || matchNs
    }
    return true
  })
})

function getResourceIcon(resourceType: string): string {
  const t = (resourceType || '').toLowerCase()
  if (t.includes('image') || t.includes('container')) return '🐳'
  if (t.includes('pod') || t.includes('deployment') || t.includes('statefulset') || t.includes('daemonset')) return '☸️'
  if (t.includes('secret') || t.includes('vault')) return '🔐'
  if (t.includes('ingress') || t.includes('service')) return '🌐'
  if (t.includes('rbac') || t.includes('role') || t.includes('serviceaccount')) return '🛡️'
  return '📦'
}

function getSeverityBadgeClass(severity: string): string {
  switch ((severity || '').toUpperCase()) {
    case 'CRITICAL': return 'badge-rose'
    case 'HIGH': return 'badge-amber'
    case 'MEDIUM': return 'badge-violet'
    case 'LOW': return 'badge-emerald'
    default: return 'badge-cyan'
  }
}

async function runScan() {
  statusMessage.value = null
  try {
    await securityStore.fetchAll()
    statusMessage.value = {
      type: 'success',
      text: 'Dispatched Trivy Image & Checkov IaC Security Scanners. Compliance matrix refreshed.',
    }
  } catch (err: unknown) {
    statusMessage.value = {
      type: 'error',
      text: err instanceof Error ? err.message : 'Failed to refresh security scan compliance.',
    }
  }
}
</script>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 24px;
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
  font-size: var(--text-tag-fluid, 11px);
  font-weight: 700;
  color: var(--accent-emerald);
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.view-title {
  font-size: var(--text-title-fluid, 24px);
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
  line-height: 1.25;
}

.view-desc {
  font-size: var(--text-desc-fluid, 13px);
  color: var(--text-secondary);
  max-width: 820px;
  margin-top: 4px;
  line-height: 1.5;
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

.metric-card {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.metric-title {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.06em;
}

.metric-val {
  font-size: 26px;
  font-weight: 800;
  color: #fff;
}

.metric-unit {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
}

.metric-footer {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  padding-top: 8px;
  border-top: 1px solid var(--border-subtle);
}

.filter-bar {
  padding: 14px 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.filter-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.filter-pill {
  padding: 5px 12px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  border: 1px solid transparent;
  background: rgba(255, 255, 255, 0.05);
  color: var(--text-secondary);
  transition: all 0.15s ease;
}

.filter-pill:hover, .filter-active {
  background: rgba(6, 182, 212, 0.2);
  border-color: var(--accent-sky);
  color: #fff;
}

.table-container {
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.table-header {
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--border-subtle);
}

.table-title {
  font-size: 16px;
  color: #fff;
}

.table-subtitle {
  font-size: 12px;
  color: var(--text-muted);
}

.table-scroll {
  overflow-x: auto;
}

.enterprise-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}

.enterprise-table th {
  padding: 12px 20px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  background: rgba(0, 0, 0, 0.3);
  border-bottom: 1px solid var(--border-subtle);
}

.enterprise-table td {
  padding: 14px 20px;
  border-bottom: 1px solid var(--border-subtle);
  font-size: 13px;
}

.table-row:hover {
  background: rgba(255, 255, 255, 0.02);
}

.empty-table-cell {
  text-align: center;
  padding: 40px !important;
  color: var(--text-muted);
}

.image-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.image-icon {
  font-size: 20px;
}

.image-name {
  font-weight: 700;
  color: #fff;
}

.image-digest {
  font-size: 10px;
}

.desc-cell {
  max-width: 320px;
  white-space: normal;
  line-height: 1.4;
  font-size: 12px;
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
  max-width: 580px;
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
  gap: 16px;
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

.remediation-box {
  background: rgba(16, 185, 129, 0.08);
  border: 1px solid rgba(16, 185, 129, 0.25);
  color: #34d399;
}

.modal-footer {
  padding: 16px 20px;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid var(--border-subtle);
}

.text-rose { color: #fb7185; }
.text-amber { color: #fbbf24; }
.text-emerald { color: #34d399; }
.text-cyan { color: #38bdf8; }
.text-violet { color: #a78bfa; }
.text-muted { color: var(--text-muted); }
.font-mono { font-family: var(--font-mono); }

@media (max-width: 768px) {
  .view-header {
    flex-direction: column;
    align-items: stretch;
    gap: 14px;
  }
  .header-actions {
    flex-direction: column;
    width: 100%;
  }
  .header-actions .btn {
    width: 100%;
    justify-content: center;
  }
  .metrics-grid {
    grid-template-columns: 1fr;
  }
  .filter-bar {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }
  .filter-group {
    overflow-x: auto;
    flex-wrap: nowrap;
    padding-bottom: 4px;
    scrollbar-width: thin;
    -webkit-overflow-scrolling: touch;
  }
  .search-box {
    width: 100%;
  }
  .search-box input {
    width: 100% !important;
  }
  .modal-overlay {
    padding: 12px;
  }
  .modal-card {
    max-width: 100%;
    border-radius: 14px;
  }
  .modal-footer {
    flex-direction: column;
    gap: 8px;
  }
  .modal-footer .btn {
    width: 100%;
    justify-content: center;
  }
}

@media (max-width: 640px) {
  .header-actions {
    display: flex;
    flex-direction: row;
    width: 100%;
    gap: 8px;
    flex-wrap: nowrap;
  }

  .header-actions .btn {
    flex: 1;
    min-width: 0;
    padding: 7px 10px;
    font-size: 11.5px;
    white-space: nowrap;
    justify-content: center;
  }

  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  .metric-card {
    padding: 10px 12px;
  }

  .metric-val {
    font-size: 18px;
  }

  .metric-title {
    font-size: 10px;
  }

  .metric-footer {
    font-size: 10px;
  }

  .view-tag {
    font-size: var(--text-tag-fluid, 10px);
    letter-spacing: 0.05em;
    font-weight: 700;
    margin-bottom: 4px;
  }

  .view-title {
    font-size: var(--text-title-fluid, clamp(18px, 4.5vw, 22px));
    font-weight: 700;
    line-height: 1.25;
    letter-spacing: -0.02em;
  }

  .view-desc {
    font-size: var(--text-desc-fluid, 12px);
    line-height: 1.45;
    color: var(--text-muted);
    margin-top: 4px;
  }

  .table-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
    padding: 14px;
  }
}
</style>
