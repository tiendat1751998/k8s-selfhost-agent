<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>GITOPS CONTINUOUS RECONCILIATION</span>
        </div>
        <h1 class="view-title">Configuration Drift Detection & Auto-Reconcile</h1>
        <p class="view-desc">
          Continuous cryptographic state comparison between Git repository manifests (<span class="highlight">Desired State</span>) and Kubernetes live etcd runtime (<span class="highlight">Actual State</span>).
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchDriftData">
          <span>{{ loading ? '⏳ Scanning...' : '🔄 Scan Cluster Drift' }}</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Metric HUD Cards -->
    <div class="metrics-grid">
      <MetricCard
        title="Drifted Resources"
        :value="driftedCount"
        :badge="driftedCount === 0 ? 'SYNCHRONIZED' : 'DRIFT DETECTED'"
        :badge-color="driftedCount === 0 ? 'emerald' : 'rose'"
        subtitle="Uncommitted live cluster mutations"
        icon="⚡"
      />
      <MetricCard
        title="GitOps Sync Health"
        :value="`${syncRate}%`"
        :trend="syncRate >= 95 ? 'Compliant' : 'Reconciliation Pending'"
        :trend-type="syncRate >= 95 ? 'positive' : 'negative'"
        badge="ETCD AUDIT"
        badge-color="cyan"
        subtitle="Live state matches Git master"
        icon="🐙"
      />
      <MetricCard
        title="Monitored Resources"
        :value="drifts.length"
        badge="TRACKED"
        badge-color="violet"
        subtitle="Across all namespaces and CRDs"
        icon="📦"
      />
      <MetricCard
        title="Target Cluster"
        value="air-gapped-k8s"
        badge="PRIMARY"
        badge-color="emerald"
        subtitle="Flux / ArgoCD GitOps Engine"
        icon="⎈"
      />
    </div>

    <!-- Filter Bar -->
    <div class="filter-bar glass-panel">
      <div class="filter-group">
        <span class="filter-label">Filter Status:</span>
        <button 
          v-for="st in statusFilters" 
          :key="st.key"
          class="filter-pill"
          :class="[st.badgeClass, { 'filter-active': activeStatus === st.key }]"
          @click="activeStatus = st.key"
        >
          <span>{{ st.label }} ({{ st.count }})</span>
        </button>
      </div>

      <div class="filter-group">
        <span class="filter-label">Cluster:</span>
        <select v-model="clusterFilter" class="input-glass filter-select" @change="fetchDriftData">
          <option value="">All Clusters</option>
          <option value="primary">primary</option>
          <option value="edge-node-01">edge-node-01</option>
        </select>
      </div>
    </div>

    <!-- Drift DataTable -->
    <DataTable
      :columns="columns"
      :data="filteredDrifts"
      :loading="loading"
      :error="error"
      searchable
      search-placeholder="Search resource, kind, namespace, or cluster..."
      empty-message="No configuration drift detected. All cluster workloads match Git source."
    >
      <template #cell-status="{ row }">
        <StatusBadge :status="row.status" :label="formatDriftStatus(row.status)" size="sm" />
      </template>

      <template #cell-resource="{ row }">
        <div class="resource-cell">
          <div class="resource-title-row">
            <span class="kind-tag font-mono">{{ row.resource_kind }}</span>
            <span class="resource-name font-mono">{{ row.resource }}</span>
          </div>
          <span class="resource-sub font-mono text-muted">{{ row.namespace || 'default' }} @ {{ row.cluster || 'cluster-prod' }}</span>
        </div>
      </template>

      <template #cell-diff="{ row }">
        <div class="diff-snippet-cell font-mono" :title="row.diff">
          {{ row.diff ? row.diff.split('\n').slice(0, 2).join(' ') : 'No mutation detected' }}
        </div>
      </template>

      <template #cell-detected_at="{ row }">
        <span class="font-mono text-muted" style="font-size: 11px;">{{ formatDate(row.detected_at) }}</span>
      </template>

      <template #cell-actions="{ row }">
        <div class="actions-cell">
          <button class="btn btn-secondary btn-sm" @click="selectedDrift = row">
            <span>🔍 View Diff</span>
          </button>
          <button 
            v-if="row.status === 'drifted'" 
            class="btn btn-primary btn-sm"
            :disabled="resolvingId === row.id"
            @click="handleReconcile(row.id)"
          >
            <span>{{ resolvingId === row.id ? 'Reconciling...' : '⚡ Auto-Reconcile' }}</span>
          </button>
        </div>
      </template>
    </DataTable>

    <!-- Side-by-Side Diff Comparison Modal / Drawer -->
    <div v-if="selectedDrift" class="modal-overlay" @click.self="selectedDrift = null">
      <div class="modal-card modal-card-wide glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <div class="modal-badge-row">
              <StatusBadge :status="selectedDrift.status" :label="formatDriftStatus(selectedDrift.status)" />
              <span class="font-mono text-muted" style="font-size: 11px;">ID: #{{ selectedDrift.id.slice(0, 8) }}</span>
            </div>
            <h3 class="modal-title">{{ selectedDrift.resource_kind }}: {{ selectedDrift.resource }}</h3>
          </div>
          <button class="modal-close" @click="selectedDrift = null">✕</button>
        </div>

        <div class="modal-body">
          <div class="meta-strip font-mono">
            <span>Namespace: <strong class="text-cyan">{{ selectedDrift.namespace || 'default' }}</strong></span>
            <span>Cluster: <strong>{{ selectedDrift.cluster || 'primary' }}</strong></span>
            <span>Detected: <strong>{{ formatDate(selectedDrift.detected_at) }}</strong></span>
          </div>

          <!-- Unified Diff Preview if present -->
          <div v-if="selectedDrift.diff" class="diff-block">
            <div class="diff-block-header">
              <span class="font-mono">UNIFIED DIFF (GIT ↔ LIVE ETCD)</span>
            </div>
            <pre class="diff-pre font-mono">{{ selectedDrift.diff }}</pre>
          </div>

          <!-- Split Diff View -->
          <div class="diff-split-grid">
            <div class="diff-pane">
              <div class="diff-pane-header text-emerald">
                <span>📄 Expected State (Git Repository)</span>
              </div>
              <pre class="code-pre font-mono">{{ selectedDrift.expected_state || '# No Git Manifest Available' }}</pre>
            </div>

            <div class="diff-pane">
              <div class="diff-pane-header text-amber">
                <span>⚡ Actual State (Live Cluster Runtime)</span>
              </div>
              <pre class="code-pre font-mono">{{ selectedDrift.actual_state || '# No Live State Available' }}</pre>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button 
            v-if="selectedDrift.status === 'drifted'" 
            class="btn btn-primary"
            :disabled="resolvingId === selectedDrift.id"
            @click="handleReconcile(selectedDrift.id)"
          >
            <span>{{ resolvingId === selectedDrift.id ? 'Reconciling...' : '⚡ Apply Git Manifest & Auto-Reconcile' }}</span>
          </button>
          <button class="btn btn-secondary" @click="selectedDrift = null">Close</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { driftApi, type DriftRecord } from '../api/governance'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'

const drifts = ref<DriftRecord[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const activeStatus = ref<'ALL' | 'DRIFTED' | 'IN_SYNC'>('ALL')
const clusterFilter = ref('')
const resolvingId = ref<string | null>(null)
const selectedDrift = ref<DriftRecord | null>(null)
const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

const columns: Column<DriftRecord>[] = [
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'resource', label: 'Resource & Scope', width: '280px', sortable: true },
  { key: 'diff', label: 'State Mutation Preview' },
  { key: 'detected_at', label: 'Detected At', width: '170px', sortable: true },
  { key: 'actions', label: 'Actions', width: '230px', align: 'right' },
]

onMounted(() => {
  fetchDriftData()
})

async function fetchDriftData() {
  loading.value = true
  error.value = null
  try {
    const res = await driftApi.getDrifts({
      cluster: clusterFilter.value || undefined,
      status: activeStatus.value === 'ALL' ? undefined : activeStatus.value.toLowerCase(),
    })
    drifts.value = res.data
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load configuration drift records'
    error.value = msg
  } finally {
    loading.value = false
  }
}

async function handleReconcile(id: string) {
  resolvingId.value = id
  statusMessage.value = null
  try {
    await driftApi.resolveDrift(id)
    statusMessage.value = {
      type: 'success',
      text: `Reconciliation dispatched for drift #${id.slice(0, 8)}. Live etcd aligned with Git repository.`,
    }
    if (selectedDrift.value && selectedDrift.value.id === id) {
      selectedDrift.value.status = 'in_sync'
    }
    await fetchDriftData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to auto-reconcile drift'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    resolvingId.value = null
  }
}

const driftedCount = computed(() => drifts.value.filter(d => d.status === 'drifted').length)
const inSyncCount = computed(() => drifts.value.filter(d => d.status === 'in_sync').length)
const syncRate = computed(() => {
  if (drifts.value.length === 0) return 100
  return Math.round((inSyncCount.value / drifts.value.length) * 100)
})

const statusFilters = computed(() => [
  { key: 'ALL' as const, label: 'All State', count: drifts.value.length, badgeClass: 'badge-cyan' },
  { key: 'DRIFTED' as const, label: 'Drifted', count: driftedCount.value, badgeClass: 'badge-rose' },
  { key: 'IN_SYNC' as const, label: 'In Sync', count: inSyncCount.value, badgeClass: 'badge-emerald' },
])

const filteredDrifts = computed(() => {
  return drifts.value.filter(d => {
    if (activeStatus.value !== 'ALL') {
      if (d.status.toUpperCase() !== activeStatus.value) return false
    }
    return true
  })
})

function formatDriftStatus(s: string): string {
  if (s === 'drifted') return 'DRIFTED'
  if (s === 'in_sync') return 'IN SYNC'
  return (s || 'UNKNOWN').toUpperCase()
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
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 820px;
  margin-top: 4px;
}

.highlight {
  color: #fff;
  font-weight: 600;
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

.resource-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.resource-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.kind-tag {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 4px;
  color: var(--accent-cyan);
}

.resource-name {
  font-weight: 600;
  color: #fff;
  font-size: 13px;
}

.resource-sub {
  font-size: 11px;
}

.diff-snippet-cell {
  max-width: 380px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  color: #fbbf24;
  background: rgba(245, 158, 11, 0.08);
  padding: 4px 8px;
  border-radius: 6px;
  border: 1px solid rgba(245, 158, 11, 0.2);
}

.actions-cell {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
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

.modal-card-wide {
  width: 100%;
  max-width: 960px;
  background: var(--bg-sidebar);
  border: 1px solid var(--border-medium);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.7);
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

.modal-header {
  padding: 18px 24px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  border-bottom: 1px solid var(--border-subtle);
}

.modal-badge-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
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
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
}

.meta-strip {
  display: flex;
  gap: 20px;
  padding: 10px 14px;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 8px;
  font-size: 12px;
  color: var(--text-secondary);
}

.diff-block {
  display: flex;
  flex-direction: column;
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(245, 158, 11, 0.3);
  border-radius: 10px;
  overflow: hidden;
}

.diff-block-header {
  padding: 8px 14px;
  background: rgba(245, 158, 11, 0.12);
  font-size: 11px;
  font-weight: 700;
  color: #fbbf24;
  border-bottom: 1px solid rgba(245, 158, 11, 0.2);
}

.diff-pre {
  padding: 12px 16px;
  font-size: 12px;
  color: #fef08a;
  white-space: pre-wrap;
  line-height: 1.5;
  max-height: 200px;
  overflow-y: auto;
}

.diff-split-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

@media (max-width: 768px) {
  .diff-split-grid {
    grid-template-columns: 1fr;
  }
}

.diff-pane {
  display: flex;
  flex-direction: column;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  overflow: hidden;
}

.diff-pane-header {
  padding: 8px 14px;
  background: rgba(255, 255, 255, 0.04);
  font-size: 12px;
  font-weight: 700;
  border-bottom: 1px solid var(--border-subtle);
}

.code-pre {
  padding: 12px 14px;
  font-size: 11px;
  color: var(--text-primary);
  line-height: 1.5;
  max-height: 300px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.modal-footer {
  padding: 16px 24px;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  border-top: 1px solid var(--border-subtle);
}
</style>
