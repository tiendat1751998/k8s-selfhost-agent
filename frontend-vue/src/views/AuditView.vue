<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>CONTINUOUS SECURITY & CLUSTER AUDIT</span>
        </div>
        <h1 class="view-title">Vulnerability & Architecture Audit Engine</h1>
        <p class="view-desc">
          Automated multi-vector scanning: <span class="highlight">Trivy CVE detection</span>, <span class="highlight">Checkov IaC compliance</span>, and cluster architecture gap analysis.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchAuditData">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Audit' }}</span>
        </button>
        <button class="btn btn-primary" :disabled="triggering" @click="handleRunAudit">
          <span>{{ triggering ? '⚡ Starting Audit...' : '⚡ Trigger Audit Scan' }}</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Metrics HUD Grid -->
    <div class="metrics-grid">
      <MetricCard
        title="Open Vulnerabilities"
        :value="openFindingsCount"
        :badge="criticalCount === 0 ? 'CLEAN' : 'ACTION REQ'"
        :badge-color="criticalCount === 0 ? 'emerald' : 'rose'"
        subtitle="Unresolved security findings"
        icon="🛡️"
      />
      <MetricCard
        title="Critical Severity CVEs"
        :value="criticalCount"
        :trend="criticalCount === 0 ? '0 Blockers' : `${criticalCount} Blocker`"
        :trend-type="criticalCount === 0 ? 'positive' : 'negative'"
        badge="GATE ENFORCED"
        badge-color="rose"
        subtitle="Requires immediate remediation"
        icon="🚨"
      />
      <MetricCard
        title="High Severity Findings"
        :value="highCount"
        badge="REVIEW"
        badge-color="amber"
        subtitle="IaC & runtime security alerts"
        icon="⚠️"
      />
      <MetricCard
        title="Latest Audit Execution"
        :value="latestRun ? latestRun.status.toUpperCase() : 'STANDBY'"
        :badge="latestRun ? `#${latestRun.id.slice(0, 6)}` : 'IDLE'"
        badge-color="cyan"
        :subtitle="latestRun ? `Started: ${formatDate(latestRun.start_time)}` : 'Ready for trigger'"
        icon="⏱️"
      />
    </div>

    <!-- Filter & Toolbar -->
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

      <div class="filter-group">
        <span class="filter-label">Status:</span>
        <select v-model="statusFilter" class="input-glass filter-select" @change="fetchAuditData">
          <option value="open">Open Findings</option>
          <option value="resolved">Resolved Findings</option>
          <option value="all">All Findings</option>
        </select>
      </div>
    </div>

    <!-- Findings DataTable -->
    <DataTable
      :columns="columns"
      :data="filteredFindings"
      :loading="loading"
      :error="error"
      searchable
      search-placeholder="Search CVE, rule, category or remediation..."
      empty-message="No security audit findings matching the criteria."
    >
      <template #cell-severity="{ row }">
        <StatusBadge :status="row.severity" :label="row.severity.toUpperCase()" size="sm" />
      </template>

      <template #cell-status="{ row }">
        <StatusBadge :status="row.status" :label="row.status.toUpperCase()" size="sm" />
      </template>

      <template #cell-category="{ row }">
        <span class="category-badge font-mono">{{ formatCategory(row.category) }}</span>
      </template>

      <template #cell-description="{ row }">
        <div class="desc-cell">
          <span class="finding-desc">{{ row.description }}</span>
          <span v-if="row.remediation" class="finding-remediation font-mono">💡 {{ row.remediation }}</span>
        </div>
      </template>

      <template #cell-detected_at="{ row }">
        <span class="timestamp-cell font-mono">{{ formatDate(row.detected_at) }}</span>
      </template>

      <template #cell-actions="{ row }">
        <div class="actions-cell">
          <button 
            v-if="row.status === 'open'" 
            class="btn btn-secondary btn-sm"
            :disabled="resolvingId === row.id"
            @click="handleResolveFinding(row.id)"
          >
            <span>{{ resolvingId === row.id ? 'Resolving...' : '✓ Resolve' }}</span>
          </button>
          <button class="btn btn-secondary btn-sm" @click="selectedFinding = row">
            <span>🔍 Details</span>
          </button>
        </div>
      </template>
    </DataTable>

    <!-- Finding Details Modal -->
    <div v-if="selectedFinding" class="modal-overlay" @click.self="selectedFinding = null">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <StatusBadge :status="selectedFinding.severity" :label="`${selectedFinding.severity.toUpperCase()} SEVERITY`" />
            <h3 class="modal-title">Finding #{{ selectedFinding.id.slice(0, 8) }}</h3>
          </div>
          <button class="modal-close" @click="selectedFinding = null">✕</button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Category:</label>
            <div class="input-glass font-mono">{{ selectedFinding.category }}</div>
          </div>

          <div class="form-group">
            <label class="form-label">Description:</label>
            <div class="input-glass desc-box">{{ selectedFinding.description }}</div>
          </div>

          <div class="form-group">
            <label class="form-label">Recommended Remediation:</label>
            <div class="input-glass remediation-box font-mono">
              {{ selectedFinding.remediation || 'Run security update or apply patch manifest.' }}
            </div>
          </div>

          <div class="form-group-row">
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Detected Timestamp:</label>
              <div class="input-glass font-mono text-muted">{{ formatDate(selectedFinding.detected_at) }}</div>
            </div>
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Status:</label>
              <div class="input-glass font-mono">{{ selectedFinding.status.toUpperCase() }}</div>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button 
            v-if="selectedFinding.status === 'open'" 
            class="btn btn-primary"
            :disabled="resolvingId === selectedFinding.id"
            @click="handleResolveFinding(selectedFinding.id)"
          >
            <span>{{ resolvingId === selectedFinding.id ? 'Resolving...' : 'Mark as Resolved' }}</span>
          </button>
          <button class="btn btn-secondary" @click="selectedFinding = null">Close</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { auditApi, type AuditFinding, type AuditRun } from '../api/governance'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'

const findings = ref<AuditFinding[]>([])
const latestRun = ref<AuditRun | null>(null)
const loading = ref(false)
const triggering = ref(false)
const resolvingId = ref<string | null>(null)
const error = ref<string | null>(null)
const activeSeverity = ref<'ALL' | 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'>('ALL')
const statusFilter = ref<'open' | 'resolved' | 'all'>('open')
const selectedFinding = ref<AuditFinding | null>(null)
const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

const columns: Column<AuditFinding>[] = [
  { key: 'severity', label: 'Severity', width: '120px', sortable: true },
  { key: 'category', label: 'Category', width: '160px', sortable: true },
  { key: 'description', label: 'Finding & Remediation' },
  { key: 'status', label: 'Status', width: '110px', sortable: true },
  { key: 'detected_at', label: 'Detected At', width: '160px', sortable: true },
  { key: 'actions', label: 'Actions', width: '170px', align: 'right' },
]

onMounted(() => {
  fetchAuditData()
})

async function fetchAuditData() {
  loading.value = true
  error.value = null
  try {
    const statusParam = statusFilter.value === 'all' ? '' : statusFilter.value
    const [findingsData, runData] = await Promise.all([
      auditApi.getFindings(statusParam),
      auditApi.getLatestRun().catch(() => null),
    ])
    findings.value = findingsData
    latestRun.value = runData
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load audit findings'
    error.value = msg
  } finally {
    loading.value = false
  }
}

async function handleRunAudit() {
  triggering.value = true
  statusMessage.value = null
  try {
    const res = await auditApi.triggerRun()
    statusMessage.value = {
      type: 'success',
      text: `Audit scan triggered successfully (Run #${res.run_id ? res.run_id.slice(0, 8) : 'new'}). Telemetry updating...`,
    }
    await fetchAuditData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to trigger audit run'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    triggering.value = false
  }
}

async function handleResolveFinding(id: string) {
  resolvingId.value = id
  try {
    await auditApi.resolveFinding(id)
    statusMessage.value = {
      type: 'success',
      text: `Finding #${id.slice(0, 8)} successfully resolved.`,
    }
    if (selectedFinding.value && selectedFinding.value.id === id) {
      selectedFinding.value.status = 'resolved'
    }
    await fetchAuditData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to resolve finding'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    resolvingId.value = null
  }
}

const openFindingsCount = computed(() => findings.value.filter(f => f.status === 'open').length)
const criticalCount = computed(() => findings.value.filter(f => f.severity.toLowerCase() === 'critical' && f.status === 'open').length)
const highCount = computed(() => findings.value.filter(f => f.severity.toLowerCase() === 'high' && f.status === 'open').length)
const mediumCount = computed(() => findings.value.filter(f => f.severity.toLowerCase() === 'medium' && f.status === 'open').length)
const lowCount = computed(() => findings.value.filter(f => f.severity.toLowerCase() === 'low' && f.status === 'open').length)

const severityFilters = computed(() => [
  { key: 'ALL' as const, label: 'All Severities', count: findings.value.length, badgeClass: 'badge-cyan' },
  { key: 'CRITICAL' as const, label: 'Critical', count: criticalCount.value, badgeClass: 'badge-rose' },
  { key: 'HIGH' as const, label: 'High', count: highCount.value, badgeClass: 'badge-amber' },
  { key: 'MEDIUM' as const, label: 'Medium', count: mediumCount.value, badgeClass: 'badge-violet' },
  { key: 'LOW' as const, label: 'Low', count: lowCount.value, badgeClass: 'badge-emerald' },
])

const filteredFindings = computed(() => {
  return findings.value.filter(f => {
    if (activeSeverity.value !== 'ALL') {
      if (f.severity.toUpperCase() !== activeSeverity.value) return false
    }
    return true
  })
})

function formatCategory(cat: string): string {
  if (!cat) return 'UNKNOWN'
  return cat.replace(/_/g, ' ').toUpperCase()
}

function formatDate(d: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleString()
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
  color: var(--accent-cyan);
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

.filter-select {
  padding: 6px 12px;
  font-size: 12px;
}

.category-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 6px;
  color: var(--text-secondary);
}

.desc-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.finding-desc {
  font-size: 13px;
  color: var(--text-primary);
}

.finding-remediation {
  font-size: 11px;
  color: var(--accent-sky);
}

.timestamp-cell {
  font-size: 11px;
  color: var(--text-muted);
}

.actions-cell {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
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

.form-group-row {
  display: flex;
  gap: 12px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.desc-box {
  white-space: pre-wrap;
  line-height: 1.4;
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
  gap: 10px;
  border-top: 1px solid var(--border-subtle);
}

@media (max-width: 768px) {
  .view-header {
    flex-direction: column;
    align-items: stretch;
    gap: 14px;
  }
  .header-actions {
    display: flex !important;
    flex-direction: row !important;
    width: 100% !important;
    gap: 8px !important;
    flex-wrap: wrap !important;
  }
  .header-actions .btn,
  .header-actions button,
  .header-actions a {
    flex: 1 1 calc(50% - 4px) !important;
    min-width: 0 !important;
    padding: 7px 10px !important;
    font-size: 11.5px !important;
    white-space: nowrap !important;
    justify-content: center !important;
  }
  .metrics-grid,
  .grid-metrics,
  .stats-grid {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 8px !important;
  }
  :deep(.metric-card),
  .metric-card,
  .stat-card,
  .hud-card {
    padding: 10px 12px !important;
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
  .filter-select {
    width: 100%;
  }
  .form-group-row {
    flex-direction: column;
    gap: 12px;
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
    display: flex !important;
    flex-direction: row !important;
    width: 100% !important;
    gap: 8px !important;
    flex-wrap: wrap !important;
  }

  .header-actions .btn,
  .header-actions button,
  .header-actions a {
    flex: 1 1 calc(50% - 4px) !important;
    min-width: 0 !important;
    padding: 7px 10px !important;
    font-size: 11.5px !important;
    white-space: nowrap !important;
    justify-content: center !important;
  }

  .metrics-grid,
  .grid-metrics,
  .stats-grid {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 8px !important;
  }

  :deep(.metric-card),
  .metric-card,
  .stat-card,
  .hud-card {
    padding: 10px 12px !important;
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

  .actions-cell {
    flex-direction: column;
    gap: 4px;
  }
}
</style>
