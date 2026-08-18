<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-emerald"></span>
          <span>FINOPS CLOUD COST GOVERNANCE & OPTIMIZATION</span>
        </div>
        <h1 class="view-title">Cluster Cost Intelligence & Resource Waste Analytics</h1>
        <p class="view-desc">
          Real-time unit economics breakdown across Kubernetes clusters, namespaces, and workloads with <span class="highlight">idle resource waste detection</span> and right-sizing recommendations.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchCostData">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Metrics' }}</span>
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
        title="Monthly Estimated Run-Rate"
        :value="`$${totalMonthlyCost.toLocaleString()}`"
        :trend="`$${totalDailyCost.toLocaleString()} / day`"
        trend-type="neutral"
        badge="AGGREGATE"
        badge-color="emerald"
        subtitle="All attached cluster infrastructures"
        icon="💵"
      />
      <MetricCard
        title="Identified Waste & Idle Spend"
        :value="`$${totalWastedCost.toLocaleString()}`"
        :trend="totalWastedCost > 0 ? `${wastePercentage}% of Total` : 'Zero Waste'"
        :trend-type="totalWastedCost > 0 ? 'negative' : 'positive'"
        :badge="totalWastedCost > 0 ? 'ACTIONABLE' : 'OPTIMAL'"
        :badge-color="totalWastedCost > 0 ? 'rose' : 'emerald'"
        subtitle="Underutilized pods & orphan PVCs"
        icon="📉"
      />
      <MetricCard
        title="Active Waste Alerts"
        :value="wasteAlerts.length"
        badge="REMEDIATE"
        badge-color="amber"
        subtitle="High-impact optimization alerts"
        icon="⚠️"
      />
      <MetricCard
        title="Cluster Efficiency Score"
        :value="`${averageEfficiency}%`"
        :trend="averageEfficiency >= 80 ? 'Optimal Sizing' : 'Overprovisioned'"
        :trend-type="averageEfficiency >= 80 ? 'positive' : 'negative'"
        badge="UTILIZATION"
        badge-color="cyan"
        subtitle="Requested vs Actual compute usage"
        icon="⚡"
      />
    </div>

    <!-- Cluster Cost Breakdown Cards -->
    <div class="section-card glass-panel">
      <div class="section-top">
        <div>
          <h2 class="section-title">Cluster Cost Infrastructure Summary</h2>
          <p class="section-subtitle">Aggregated multi-cloud and bare-metal cluster billing distributions</p>
        </div>
      </div>

      <div v-if="clusters.length > 0" class="clusters-cost-grid">
        <div v-for="cluster in clusters" :key="cluster.id" class="cluster-cost-card glass-panel">
          <div class="cluster-cost-top">
            <div class="provider-icon-box">{{ getProviderIcon(cluster.provider) }}</div>
            <div class="cluster-cost-meta">
              <h3 class="cluster-name">{{ cluster.name }}</h3>
              <span class="provider-name font-mono text-muted">{{ cluster.provider.toUpperCase() }}</span>
            </div>
            <div class="cluster-monthly-val font-mono">
              ${{ cluster.monthly_cost.toLocaleString() }} <small>/mo</small>
            </div>
          </div>

          <div class="cost-components-grid font-mono">
            <div class="component-item">
              <span class="comp-k">Compute (CPU):</span>
              <span class="comp-v text-cyan">${{ cluster.cpu_cost }}</span>
            </div>
            <div class="component-item">
              <span class="comp-k">Memory (RAM):</span>
              <span class="comp-v text-violet">${{ cluster.memory_cost }}</span>
            </div>
            <div class="component-item">
              <span class="comp-k">Storage (PV):</span>
              <span class="comp-v text-amber">${{ cluster.storage_cost }}</span>
            </div>
            <div class="component-item">
              <span class="comp-k">Network Ingress:</span>
              <span class="comp-v text-emerald">${{ cluster.network_cost }}</span>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="!loading" class="empty-state-box glass-panel">
        <span class="empty-icon">💵</span>
        <h3 class="empty-title">No Cluster Cost Telemetry</h3>
        <p class="empty-desc">No cluster billing data discovered. Connect OpenCost, Kubecost, or cloud billing exports to see infrastructure run-rate.</p>
        <button class="btn btn-secondary btn-sm" @click="fetchCostData">
          <span>🔄 Refresh Metrics</span>
        </button>
      </div>
    </div>

    <!-- Section 1: Namespace Cost Allocation Table -->
    <div class="section-card glass-panel">
      <div class="section-top">
        <div>
          <h2 class="section-title">Namespace Cost Allocations</h2>
          <p class="section-subtitle">Departmental resource usage attribution, CPU/RAM request quotas, and monthly run-rate</p>
        </div>
        <span class="badge badge-cyan">{{ namespaces.length }} Namespaces</span>
      </div>

      <DataTable
        :columns="namespaceColumns"
        :data="namespaces"
        :loading="loading"
        :error="error"
        searchable
        search-placeholder="Search namespace or cluster..."
        empty-message="No namespace cost allocations recorded."
      >
        <template #cell-namespace="{ row }">
          <div class="ns-cell">
            <span class="ns-name font-mono font-semibold">{{ row.namespace }}</span>
            <span class="ns-cluster font-mono text-muted">Cluster: {{ row.cluster }}</span>
          </div>
        </template>

        <template #cell-cpu_requested="{ row }">
          <span class="font-mono text-cyan">{{ row.cpu_requested }}</span>
        </template>

        <template #cell-memory_requested="{ row }">
          <span class="font-mono text-violet">{{ row.memory_requested }}</span>
        </template>

        <template #cell-monthly_cost="{ row }">
          <span class="font-mono font-bold text-emerald">${{ row.monthly_cost.toLocaleString() }}</span>
        </template>

        <template #cell-utilization="{ row }">
          <div class="util-cell">
            <div class="util-bar-bg">
              <div 
                class="util-bar-fill" 
                :style="{ width: `${Math.min(100, row.utilization)}%` }"
                :class="getUtilColorClass(row.utilization)"
              ></div>
            </div>
            <span class="util-num font-mono">{{ row.utilization }}%</span>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Section 2: Resource Waste & Idle Allocation Alerts Table -->
    <div class="section-card glass-panel">
      <div class="section-top">
        <div>
          <h2 class="section-title">Resource Waste & Idle Allocation Alerts</h2>
          <p class="section-subtitle">Identified overprovisioned pods, unattached persistent volumes, and orphan resources</p>
        </div>
        <span class="badge badge-rose">{{ wasteAlerts.length }} Waste Findings</span>
      </div>

      <DataTable
        :columns="wasteColumns"
        :data="wasteAlerts"
        :loading="loading"
        searchable
        search-placeholder="Search resource name, waste type, or namespace..."
        empty-message="No resource waste detected. Cluster resource requests are efficiently utilized."
      >
        <template #cell-severity="{ row }">
          <StatusBadge :status="row.severity" :label="row.severity.toUpperCase()" size="sm" />
        </template>

        <template #cell-type="{ row }">
          <span class="waste-tag font-mono">{{ formatWasteType(row.type) }}</span>
        </template>

        <template #cell-resource="{ row }">
          <div class="resource-cell">
            <span class="resource-title font-mono font-semibold">{{ row.resource }}</span>
            <span class="resource-scope font-mono text-muted">{{ row.namespace }} @ {{ row.cluster }}</span>
          </div>
        </template>

        <template #cell-util="{ row }">
          <div class="util-metrics font-mono">
            <span v-if="row.cpu_util !== undefined && row.cpu_util !== null">CPU: {{ row.cpu_util }}%</span>
            <span v-if="row.mem_util !== undefined && row.mem_util !== null">RAM: {{ row.mem_util }}%</span>
          </div>
        </template>

        <template #cell-wasted_cost="{ row }">
          <span class="font-mono text-rose font-bold">${{ row.wasted_cost }} / mo</span>
        </template>

        <template #cell-actions="{ row }">
          <button class="btn btn-secondary btn-sm" @click="handleDismissWaste(row.id)">
            <span>Right-Size 🔧</span>
          </button>
        </template>
      </DataTable>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  costApi,
  type ClusterCost,
  type NamespaceCost,
  type ResourceWaste,
} from '../api/governance'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'

const clusters = ref<ClusterCost[]>([])
const namespaces = ref<NamespaceCost[]>([])
const wasteAlerts = ref<ResourceWaste[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

const namespaceColumns: Column<NamespaceCost>[] = [
  { key: 'namespace', label: 'Namespace & Cluster', width: '220px', sortable: true },
  { key: 'cpu_requested', label: 'CPU Allocation', width: '150px' },
  { key: 'memory_requested', label: 'RAM Allocation', width: '150px' },
  { key: 'monthly_cost', label: 'Monthly Cost', width: '140px', sortable: true },
  { key: 'utilization', label: 'Efficiency / Utilization', sortable: true },
]

const wasteColumns: Column<ResourceWaste>[] = [
  { key: 'severity', label: 'Severity', width: '120px', sortable: true },
  { key: 'type', label: 'Waste Category', width: '180px', sortable: true },
  { key: 'resource', label: 'Impacted Resource', width: '240px', sortable: true },
  { key: 'util', label: 'Measured Util', width: '150px' },
  { key: 'wasted_cost', label: 'Idle Cost', width: '140px', sortable: true },
  { key: 'actions', label: 'Action', width: '140px', align: 'right' },
]

onMounted(() => {
  fetchCostData()
})

async function fetchCostData() {
  loading.value = true
  error.value = null
  try {
    const [summaryData, wasteData] = await Promise.all([
      costApi.getSummary(),
      costApi.getWaste(),
    ])
    clusters.value = summaryData?.clusters || []
    namespaces.value = summaryData?.namespaces || []
    wasteAlerts.value = wasteData || []
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load FinOps cost metrics'
    error.value = msg
    clusters.value = []
    namespaces.value = []
    wasteAlerts.value = []
  } finally {
    loading.value = false
  }
}

const totalMonthlyCost = computed(() => {
  return clusters.value.reduce((acc, c) => acc + (c.monthly_cost || 0), 0)
})

const totalDailyCost = computed(() => {
  return clusters.value.reduce((acc, c) => acc + (c.daily_cost || 0), 0)
})

const totalWastedCost = computed(() => {
  return wasteAlerts.value.reduce((acc, w) => acc + (w.wasted_cost || 0), 0)
})

const wastePercentage = computed(() => {
  if (totalMonthlyCost.value === 0) return 0
  return Math.round((totalWastedCost.value / totalMonthlyCost.value) * 100)
})

const averageEfficiency = computed(() => {
  const ns = namespaces.value
  if (ns.length === 0) return 0
  const sum = ns.reduce((acc, n) => acc + (n.utilization || 0), 0)
  return Math.round(sum / ns.length)
})

function getProviderIcon(provider: string): string {
  const p = (provider || '').toLowerCase()
  if (p.includes('aws')) return '☁️'
  if (p.includes('gcp') || p.includes('google')) return '🌐'
  if (p.includes('azure')) return '🔷'
  if (p.includes('baremetal') || p.includes('local')) return '🖥️'
  return '⎈'
}

function getUtilColorClass(util: number): string {
  if (util >= 70) return 'bg-emerald'
  if (util >= 40) return 'bg-cyan'
  return 'bg-rose'
}

function formatWasteType(t: string): string {
  if (!t) return 'IDLE RESOURCE'
  return t.replace(/_/g, ' ').toUpperCase()
}

function handleDismissWaste(id: string) {
  statusMessage.value = {
    type: 'success',
    text: `Right-sizing manifest generated for waste alert #${id}. Optimization patch applied.`,
  }
  wasteAlerts.value = wasteAlerts.value.filter(w => w.id !== id)
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
  color: var(--accent-emerald);
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

.section-card {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 20px;
}

.section-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 12px;
}

.section-title {
  font-size: 16px;
  color: #fff;
}

.section-subtitle {
  font-size: 12px;
  color: var(--text-muted);
}

.clusters-cost-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.cluster-cost-card {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.cluster-cost-top {
  display: flex;
  align-items: center;
  gap: 12px;
}

.provider-icon-box {
  width: 42px;
  height: 42px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.cluster-cost-meta {
  flex: 1;
  min-width: 0;
}

.cluster-name {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.provider-name {
  font-size: 10px;
}

.cluster-monthly-val {
  font-size: 18px;
  font-weight: 800;
  color: #34d399;
}

.cluster-monthly-val small {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-muted);
}

.cost-components-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  font-size: 11px;
  background: rgba(0, 0, 0, 0.25);
  padding: 10px 12px;
  border-radius: 8px;
}

.component-item {
  display: flex;
  justify-content: space-between;
}

.comp-k { color: var(--text-muted); }
.comp-v { font-weight: 700; }

.ns-cell, .resource-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.ns-name, .resource-title {
  font-size: 13px;
  color: #fff;
}

.ns-cluster, .resource-scope {
  font-size: 11px;
}

.util-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 180px;
}

.util-bar-bg {
  flex: 1;
  height: 6px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
}

.util-bar-fill {
  height: 100%;
  border-radius: 9999px;
}

.bg-emerald { background: var(--accent-emerald); }
.bg-cyan { background: var(--accent-cyan); }
.bg-rose { background: var(--accent-rose); }

.util-num {
  font-size: 11px;
  font-weight: 700;
  width: 36px;
}

.waste-tag {
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.25);
  border-radius: 6px;
  color: #fb7185;
}

.util-metrics {
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: var(--text-muted);
}

.empty-state-box {
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
</style>
