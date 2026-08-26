<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-emerald"></span>
          <span>REGULATORY & POSTURE GOVERNANCE</span>
        </div>
        <h1 class="view-title">Compliance Frameworks & Policy Violations</h1>
        <p class="view-desc">
          Continuous validation against industry standards (<span class="highlight">CIS Kubernetes</span>, <span class="highlight">SOC 2 Type II</span>, <span class="highlight">HIPAA Security</span>, <span class="highlight">PCI-DSS</span>) with automated remediation guidance.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchComplianceData">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Posture' }}</span>
        </button>
      </div>
    </div>

    <!-- Error Banner if any -->
    <div v-if="error" class="status-banner banner-error animate-fade-in">
      <span class="banner-icon">⚠️</span>
      <span class="banner-text">{{ error }}</span>
      <button class="banner-close" @click="error = null">✕</button>
    </div>

    <!-- Framework Score Cards Grid -->
    <div v-if="frameworks.length > 0" class="frameworks-grid">
      <div 
        v-for="fw in frameworks" 
        :key="fw.id" 
        class="framework-card glass-panel glass-panel-glow"
        :class="{ 'framework-card-selected': selectedFrameworkId === fw.id }"
        @click="selectFramework(fw.id)"
      >
        <div class="fw-header">
          <div class="fw-icon-box">{{ fw.icon || '🛡️' }}</div>
          <div class="fw-title-group">
            <h3 class="fw-name">{{ fw.name }}</h3>
            <span class="fw-last-scan text-muted font-mono">Scanned: {{ formatDate(fw.last_scan_at) }}</span>
          </div>
          <span class="badge" :class="getScoreBadgeClass(fw.score)">
            {{ fw.score.toFixed(1) }}%
          </span>
        </div>

        <div class="fw-progress-wrapper">
          <div class="progress-bar-bg">
            <div 
              class="progress-bar-fill" 
              :style="{ width: `${Math.min(100, Math.max(0, fw.score))}%` }"
              :class="getProgressColorClass(fw.score)"
            ></div>
          </div>
        </div>

        <div class="fw-stats-row font-mono">
          <div class="fw-stat">
            <span class="stat-k">Passed:</span>
            <span class="stat-v text-emerald">{{ fw.passed_checks }}</span>
          </div>
          <div class="fw-stat">
            <span class="stat-k">Failed:</span>
            <span class="stat-v text-rose">{{ fw.failed_checks }}</span>
          </div>
          <div class="fw-stat">
            <span class="stat-k">Total Checks:</span>
            <span class="stat-v">{{ fw.total_checks }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty Frameworks State -->
    <div v-else-if="!loading" class="empty-frameworks-box glass-panel">
      <span class="empty-icon">🛡️</span>
      <h3 class="empty-title">No Compliance Frameworks Configured</h3>
      <p class="empty-desc">
        No active regulatory benchmarks (CIS, SOC2, HIPAA, PCI-DSS) detected. Connect compliance scanning engines to begin posture assessment.
      </p>
      <button class="btn btn-secondary btn-sm" @click="fetchComplianceData">
        <span>🔄 Run Compliance Scan</span>
      </button>
    </div>

    <!-- Telemetry Overview Metrics -->
    <div class="metrics-grid">
      <MetricCard
        title="Aggregate Compliance"
        :value="`${averageScore.toFixed(1)}%`"
        :trend="averageScore >= 80 ? 'Passing Audit' : 'Action Required'"
        :trend-type="averageScore >= 80 ? 'positive' : 'negative'"
        badge="AGGREGATE"
        badge-color="emerald"
        subtitle="Across all active frameworks"
        icon="📊"
      />
      <MetricCard
        title="Active Violations"
        :value="totalViolations"
        :badge="totalViolations === 0 ? 'CLEAN' : `${criticalViolationsCount} CRITICAL`"
        :badge-color="criticalViolationsCount === 0 ? 'emerald' : 'rose'"
        subtitle="Policy rules failing validation"
        icon="⚠️"
      />
      <MetricCard
        title="Critical Policy Failures"
        :value="criticalViolationsCount"
        badge="GATE IMPACT"
        badge-color="rose"
        subtitle="Blocks production promotions"
        icon="🚨"
      />
      <MetricCard
        title="Passing Rules"
        :value="totalPassedChecks"
        badge="VALIDATED"
        badge-color="cyan"
        subtitle="Automated checks passed"
        icon="✅"
      />
    </div>

    <!-- Active Violations Table Section -->
    <div class="violations-section">
      <div class="filter-bar glass-panel">
        <div class="filter-group">
          <span class="filter-label">Severity:</span>
          <button 
            v-for="sev in severityFilters" 
            :key="sev.key"
            class="filter-pill"
            :class="[sev.badgeClass, { 'filter-active': activeSeverity === sev.key }]"
            @click="activeSeverity = sev.key"
          >
            <span>{{ sev.label }} ({{ sev.count }})</span>
          </button>
        </div>

        <div v-if="selectedFrameworkId" class="filter-group">
          <span class="active-filter-badge">
            Framework: {{ getSelectedFrameworkName() }}
            <button class="clear-btn" @click="selectedFrameworkId = ''">✕</button>
          </span>
        </div>
      </div>

      <DataTable
        :columns="columns"
        :data="filteredViolations"
        :loading="loading"
        :error="error"
        searchable
        search-placeholder="Filter by policy, namespace, cluster, or resource..."
        empty-message="No compliance violations found. All security posture controls are satisfied."
      >
        <template #cell-severity="{ row }">
          <StatusBadge :status="row.severity" :label="row.severity.toUpperCase()" size="sm" />
        </template>

        <template #cell-resource="{ row }">
          <div class="resource-cell">
            <span class="resource-name font-mono">{{ row.resource }}</span>
            <span class="resource-ns font-mono text-muted">{{ row.namespace || 'default' }} ({{ row.cluster || 'primary' }})</span>
          </div>
        </template>

        <template #cell-policy="{ row }">
          <div class="policy-cell">
            <span class="policy-name">{{ row.policy }}</span>
            <span class="policy-msg text-muted">{{ row.message }}</span>
          </div>
        </template>

        <template #cell-framework_id="{ row }">
          <span class="fw-tag font-mono">{{ formatFrameworkTag(row.framework_id) }}</span>
        </template>

        <template #cell-detected_at="{ row }">
          <span class="font-mono text-muted" style="font-size: 11px;">{{ formatDate(row.detected_at) }}</span>
        </template>

        <template #cell-actions="{ row }">
          <button class="btn btn-secondary btn-sm" @click="selectedViolation = row">
            <span>📖 Remediation</span>
          </button>
        </template>
      </DataTable>
    </div>

    <!-- Remediation Guidance Modal -->
    <div v-if="selectedViolation" class="modal-overlay" @click.self="selectedViolation = null">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <StatusBadge :status="selectedViolation.severity" :label="`${selectedViolation.severity.toUpperCase()} SEVERITY`" />
            <h3 class="modal-title">{{ selectedViolation.policy }}</h3>
          </div>
          <button class="modal-close" @click="selectedViolation = null">✕</button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Impacted Resource:</label>
            <div class="input-glass font-mono">
              {{ selectedViolation.resource }} (Namespace: {{ selectedViolation.namespace || 'default' }} | Cluster: {{ selectedViolation.cluster || 'primary' }})
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Violation Message:</label>
            <div class="input-glass">{{ selectedViolation.message }}</div>
          </div>

          <div class="form-group">
            <label class="form-label">Framework Control:</label>
            <div class="input-glass font-mono text-cyan">{{ selectedViolation.framework_id }}</div>
          </div>

          <div class="form-group">
            <label class="form-label">Automated Remediation Guidance:</label>
            <div class="input-glass remediation-box font-mono">
              1. Inspect manifest for {{ selectedViolation.resource }} in Git repository.<br />
              2. Ensure securityContext and compliance spec parameters match CIS/SOC2 benchmark.<br />
              3. Commit changes to trigger continuous GitOps reconciliation.
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
import { complianceApi, type ComplianceFramework, type ComplianceViolation } from '../api/governance'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'

const frameworks = ref<ComplianceFramework[]>([])
const violations = ref<ComplianceViolation[]>([])
const totalViolations = ref(0)
const loading = ref(false)
const error = ref<string | null>(null)
const selectedFrameworkId = ref<string>('')
const activeSeverity = ref<'ALL' | 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'>('ALL')
const selectedViolation = ref<ComplianceViolation | null>(null)

const columns: Column<ComplianceViolation>[] = [
  { key: 'severity', label: 'Severity', width: '120px', sortable: true },
  { key: 'resource', label: 'Resource & Namespace', width: '220px', sortable: true },
  { key: 'policy', label: 'Policy & Finding' },
  { key: 'framework_id', label: 'Framework', width: '150px', sortable: true },
  { key: 'detected_at', label: 'Detected At', width: '160px', sortable: true },
  { key: 'actions', label: 'Action', width: '140px', align: 'right' },
]

onMounted(() => {
  fetchComplianceData()
})

async function fetchComplianceData() {
  loading.value = true
  error.value = null
  try {
    const [fwData, viData] = await Promise.all([
      complianceApi.getFrameworks(),
      complianceApi.getViolations(activeSeverity.value === 'ALL' ? undefined : activeSeverity.value.toLowerCase()),
    ])
    frameworks.value = fwData || []
    violations.value = viData?.data || []
    totalViolations.value = viData?.total || 0
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load compliance data'
    error.value = msg
    frameworks.value = []
    violations.value = []
    totalViolations.value = 0
  } finally {
    loading.value = false
  }
}

function selectFramework(id: string) {
  if (selectedFrameworkId.value === id) {
    selectedFrameworkId.value = ''
  } else {
    selectedFrameworkId.value = id
  }
}

function getSelectedFrameworkName(): string {
  const fw = frameworks.value.find(f => f.id === selectedFrameworkId.value)
  return fw ? fw.name : selectedFrameworkId.value
}

const averageScore = computed(() => {
  const list = frameworks.value
  if (list.length === 0) return 0
  const sum = list.reduce((acc, f) => acc + f.score, 0)
  return sum / list.length
})

const totalPassedChecks = computed(() => {
  return frameworks.value.reduce((acc, f) => acc + f.passed_checks, 0)
})

const criticalViolationsCount = computed(() => {
  return violations.value.filter(v => v.severity.toLowerCase() === 'critical').length
})
const highViolationsCount = computed(() => {
  return violations.value.filter(v => v.severity.toLowerCase() === 'high').length
})
const mediumViolationsCount = computed(() => {
  return violations.value.filter(v => v.severity.toLowerCase() === 'medium').length
})
const lowViolationsCount = computed(() => {
  return violations.value.filter(v => v.severity.toLowerCase() === 'low').length
})

const severityFilters = computed(() => [
  { key: 'ALL' as const, label: 'All Severities', count: violations.value.length, badgeClass: 'badge-cyan' },
  { key: 'CRITICAL' as const, label: 'Critical', count: criticalViolationsCount.value, badgeClass: 'badge-rose' },
  { key: 'HIGH' as const, label: 'High', count: highViolationsCount.value, badgeClass: 'badge-amber' },
  { key: 'MEDIUM' as const, label: 'Medium', count: mediumViolationsCount.value, badgeClass: 'badge-violet' },
  { key: 'LOW' as const, label: 'Low', count: lowViolationsCount.value, badgeClass: 'badge-emerald' },
])

const filteredViolations = computed(() => {
  return violations.value.filter(v => {
    if (selectedFrameworkId.value && v.framework_id !== selectedFrameworkId.value) {
      return false
    }
    if (activeSeverity.value !== 'ALL' && v.severity.toUpperCase() !== activeSeverity.value) {
      return false
    }
    return true
  })
})

function getScoreBadgeClass(score: number): string {
  if (score >= 90) return 'badge-emerald'
  if (score >= 75) return 'badge-amber'
  return 'badge-rose'
}

function getProgressColorClass(score: number): string {
  if (score >= 90) return 'bg-emerald'
  if (score >= 75) return 'bg-amber'
  return 'bg-rose'
}

function formatFrameworkTag(tag: string): string {
  if (!tag) return 'POLICY'
  const fw = frameworks.value.find(f => f.id === tag)
  if (fw) {
    if (fw.name.includes('CIS')) return 'CIS LEVEL 2'
    if (fw.name.includes('SOC 2') || fw.name.includes('SOC2')) return 'SOC 2'
    if (fw.name.includes('HIPAA')) return 'HIPAA'
    if (fw.name.includes('PCI')) return 'PCI-DSS'
    return fw.name.toUpperCase()
  }
  return tag.replace(/-/g, ' ').toUpperCase()
}

function formatDate(d: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString()
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

.frameworks-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}

.framework-card {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  cursor: pointer;
}

.framework-card-selected {
  border-color: var(--accent-cyan);
  box-shadow: 0 0 15px rgba(6, 182, 212, 0.25);
}

.fw-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.fw-icon-box {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.fw-title-group {
  flex: 1;
  min-width: 0;
}

.fw-name {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.fw-last-scan {
  font-size: 10px;
}

.fw-progress-wrapper {
  width: 100%;
}

.progress-bar-bg {
  width: 100%;
  height: 6px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
}

.progress-bar-fill {
  height: 100%;
  border-radius: 9999px;
  transition: width 0.3s ease;
}

.bg-emerald { background: var(--accent-emerald); }
.bg-amber { background: var(--accent-amber); }
.bg-rose { background: var(--accent-rose); }

.fw-stats-row {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  padding-top: 10px;
  border-top: 1px solid var(--border-subtle);
}

.fw-stat {
  display: flex;
  gap: 4px;
}

.stat-k { color: var(--text-muted); }
.stat-v { font-weight: 700; }

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.violations-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.filter-bar {
  padding: 12px 18px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 14px;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.filter-pill {
  padding: 4px 10px;
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

.active-filter-badge {
  padding: 4px 10px;
  border-radius: 8px;
  font-size: 11px;
  background: rgba(6, 182, 212, 0.15);
  border: 1px solid rgba(6, 182, 212, 0.3);
  color: var(--accent-sky);
  display: flex;
  align-items: center;
  gap: 6px;
}

.clear-btn {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
}

.resource-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.resource-name {
  font-weight: 600;
  color: #fff;
  font-size: 12px;
}

.resource-ns {
  font-size: 11px;
}

.policy-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.policy-name {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 13px;
}

.policy-msg {
  font-size: 11px;
}

.fw-tag {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-secondary);
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
  max-width: 600px;
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

.remediation-box {
  background: rgba(16, 185, 129, 0.08);
  border: 1px solid rgba(16, 185, 129, 0.25);
  color: #34d399;
  line-height: 1.6;
}

.modal-footer {
  padding: 16px 20px;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid var(--border-subtle);
}

.empty-frameworks-box {
  padding: 36px 20px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.empty-icon {
  font-size: 32px;
}

.empty-title {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
}

.empty-desc {
  font-size: 12px;
  color: var(--text-muted);
  max-width: 480px;
  margin-bottom: 6px;
}

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
  .frameworks-grid {
    grid-template-columns: 1fr;
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

  :deep(.metric-card),
  .metric-card {
    padding: 10px 12px;
  }

  :deep(.metric-card .metric-value),
  .metric-card .metric-value {
    font-size: 18px;
  }

  :deep(.metric-card .metric-title),
  .metric-card .metric-title {
    font-size: 10px;
  }

  :deep(.metric-card .metric-footer),
  .metric-card .metric-footer {
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

  .framework-card {
    padding: 14px;
  }
  .fw-stats-row {
    flex-direction: column;
    gap: 6px;
  }
}
</style>
