<script setup lang="ts">
import { ref, computed } from 'vue'
import '../assets/styles/views/cost.css'
import { useCostFinOps } from '../composables/useCostFinOps'
import CostHudMetrics from '../components/cost/CostHudMetrics.vue'
import CostBreakdownChart from '../components/cost/CostBreakdownChart.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import NamespaceCostTable from '../components/cost/NamespaceCostTable.vue'
import CostMobileCards from '../components/cost/CostMobileCards.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import type { ResourceWaste } from '../api/governance'

const {
  clusters,
  namespaces,
  wasteAlerts,
  loading,
  error,
  statusMessage,
  selectedNamespace,
  totalMonthlyCost,
  totalDailyCost,
  totalWastedCost,
  wastePercentage,
  spotRatio,
  spotSavings,
  wasteSaved,
  projectedCost,
  cloudBreakdown,
  fetchCostData,
  handleDismissWaste,
  setBudgetLimit,
  openNamespaceBreakdown,
  closeNamespaceBreakdown,
  formatWasteType,
} = useCostFinOps()

const showMobileBreakdown = ref(false)
const showBudgetLimitsModal = ref(false)
const searchQuery = ref('')
const clusterFilter = ref('all')
const severityFilter = ref('all')
const namespaceFilter = ref('all')

const uniqueNamespaces = computed(() => {
  const set = new Set<string>()
  namespaces.value.forEach((n) => set.add(n.namespace))
  return Array.from(set).sort()
})

const filteredClusters = computed(() => {
  if (clusterFilter.value === 'all') return clusters.value
  return clusters.value.filter((c) => c.name === clusterFilter.value)
})

const filteredNamespaces = computed(() => {
  return namespaces.value.filter((ns) => {
    if (clusterFilter.value !== 'all' && ns.cluster !== clusterFilter.value) return false
    if (namespaceFilter.value !== 'all' && ns.namespace !== namespaceFilter.value) return false
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase().trim()
      const matchNs = ns.namespace.toLowerCase().includes(q)
      const matchTeam = (ns.team || '').toLowerCase().includes(q)
      const matchCluster = (ns.cluster || '').toLowerCase().includes(q)
      if (!matchNs && !matchTeam && !matchCluster) return false
    }
    return true
  })
})

const filteredWasteAlerts = computed(() => {
  return wasteAlerts.value.filter((w) => {
    if (clusterFilter.value !== 'all' && w.cluster !== clusterFilter.value) return false
    if (severityFilter.value !== 'all' && w.severity.toLowerCase() !== severityFilter.value.toLowerCase()) return false
    if (namespaceFilter.value !== 'all' && w.namespace !== namespaceFilter.value) return false
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase().trim()
      const matchRes = (w.resource || '').toLowerCase().includes(q)
      const matchType = (w.type || '').toLowerCase().includes(q)
      const matchNs = (w.namespace || '').toLowerCase().includes(q)
      const matchCluster = (w.cluster || '').toLowerCase().includes(q)
      if (!matchRes && !matchType && !matchNs && !matchCluster) return false
    }
    return true
  })
})

const wasteColumns: Column<ResourceWaste>[] = [
  { key: 'severity', label: 'Severity', width: '14%', sortable: true },
  { key: 'type', label: 'Waste Category', width: '18%', sortable: true },
  { key: 'resource', label: 'Impacted Resource', width: '26%', sortable: true },
  { key: 'util', label: 'Measured Util', width: '14%' },
  { key: 'wasted_cost', label: 'Idle Cost', width: '12%', sortable: true },
  { key: 'actions', label: 'Action', width: '16%', align: 'right' },
]
</script>

<template>
  <div class="view-container">
    <!-- Sleek Unified 38px Enterprise Toolbar (>=768px) -->
    <div class="cost-toolbar-sleek desktop-only" role="toolbar" aria-label="FinOps Controls Toolbar">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="toolbar-search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search costs, namespaces..."
          class="toolbar-search-input font-mono"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="toolbar-search-clear"
          title="Clear search"
          @click="searchQuery = ''"
        >
          <BaseIcon name="x" size="xs" />
        </button>
      </div>

      <!-- Cluster filter dropdown -->
      <select v-model="clusterFilter" class="toolbar-select font-mono" title="Filter by cluster">
        <option value="all">All Clusters ({{ clusters.length }})</option>
        <option v-for="c in clusters" :key="c.id" :value="c.name">
          {{ c.name }} ({{ c.provider.toUpperCase() }})
        </option>
      </select>

      <!-- Waste severity filter dropdown -->
      <select v-model="severityFilter" class="toolbar-select font-mono" title="Filter by waste severity">
        <option value="all">All Severities</option>
        <option value="critical">Critical</option>
        <option value="high">High</option>
        <option value="medium">Medium</option>
        <option value="low">Low</option>
      </select>

      <!-- Namespace filter dropdown -->
      <select v-model="namespaceFilter" class="toolbar-select font-mono" title="Filter by namespace">
        <option value="all">All Namespaces ({{ uniqueNamespaces.length }})</option>
        <option v-for="ns in uniqueNamespaces" :key="ns" :value="ns">{{ ns }}</option>
      </select>

      <!-- Inline compact KPI badge strip font-mono -->
      <div class="toolbar-kpi-badge font-mono" role="status" aria-label="FinOps Run-Rate Summary">
        <span class="kpi-runrate">${{ totalMonthlyCost.toLocaleString() }} / mo (Run-Rate)</span>
        <span class="kpi-sep">·</span>
        <span class="kpi-idle">${{ totalWastedCost.toLocaleString() }} Idle</span>
        <span class="kpi-sep">·</span>
        <span class="kpi-spot">{{ spotRatio }}% Spot</span>
      </div>

      <div class="toolbar-spacer"></div>

      <!-- Action buttons: Refresh and Budget Limits -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="btn btn-secondary toolbar-btn"
          :disabled="loading"
          title="Refresh FinOps metrics"
          @click="fetchCostData"
        >
          <BaseIcon :name="loading ? 'activity' : 'refresh'" size="xs" :class="{ 'spin-icon': loading }" />
          <span>{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
        <button
          type="button"
          class="btn btn-primary toolbar-btn"
          title="Configure FinOps budget limits"
          @click="showBudgetLimitsModal = true"
        >
          <BaseIcon name="sliders" size="xs" />
          <span>Budget Limits</span>
        </button>
      </div>
    </div>

    <!-- 44px Mobile Command Bar (<768px) -->
    <div class="mobile-command-bar cost-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="dollar-sign" size="xs" /> Cost FinOps (${{ (totalMonthlyCost / 1000).toFixed(1) }}k/mo)</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh Cost Metrics"
          aria-label="Refresh"
          @click="fetchCostData"
        >
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          title="FinOps Cost Breakdown"
          aria-label="Cost Breakdown"
          @click="showMobileBreakdown = true"
        >
          <BaseIcon name="pie-chart" size="xs" />
        </button>
      </div>
    </div>

    <!-- 20px Mobile Micro-Telemetry Strip (<768px) -->
    <div class="mobile-micro-telemetry cost-micro-telemetry mobile-only font-mono" role="status" aria-label="Cost FinOps Micro Telemetry">
      <span class="tel-item tel-monthly"><BaseIcon name="dollar-sign" size="xs" /> ${{ (totalMonthlyCost / 1000).toFixed(1) }}k/mo</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-daily"><BaseIcon name="zap" size="xs" /> ${{ totalDailyCost }}/d</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-waste"><BaseIcon name="trending-up" size="xs" /> ${{ totalWastedCost }} waste</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-spot"><BaseIcon name="tag" size="xs" /> {{ spotRatio }}% spot</span>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Desktop Metrics HUD Grid (>=768px) -->
    <CostHudMetrics
      class="desktop-only"
      :total-monthly-cost="totalMonthlyCost"
      :total-daily-cost="totalDailyCost"
      :waste-saved="wasteSaved"
      :total-wasted-cost="totalWastedCost"
      :waste-percentage="wastePercentage"
      :projected-cost="projectedCost"
      :spot-ratio="spotRatio"
      :spot-savings="spotSavings"
    />

    <!-- Multi-Cloud Infrastructure Cost Breakdown & Clusters (>=768px) -->
    <CostBreakdownChart
      class="desktop-only"
      :cloud-breakdown="cloudBreakdown"
      :clusters="filteredClusters"
      :loading="loading"
      @refresh="fetchCostData"
    />

    <!-- Section 1: Namespace Cost Allocations (>=768px) -->
    <NamespaceCostTable
      class="desktop-only"
      :namespaces="filteredNamespaces"
      :loading="loading"
      :error="error"
      @save-budget="setBudgetLimit"
      @select-namespace="openNamespaceBreakdown"
    />

    <!-- Section 2: Resource Waste & Idle Allocation Alerts (>=768px) -->
    <div class="section-card glass-panel desktop-only">
      <div class="section-top">
        <div>
          <h2 class="section-title">Resource Waste & Idle Allocation Alerts</h2>
          <p class="section-subtitle">Identified overprovisioned pods, unattached persistent volumes, and orphan resources</p>
        </div>
        <span class="badge badge-rose">{{ filteredWasteAlerts.length }} Waste Findings</span>
      </div>

      <div class="cost-desktop-table">
        <DataTable
          :columns="wasteColumns"
          :data="filteredWasteAlerts"
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
              <span class="resource-title font-mono font-semibold" :title="row.resource">{{ row.resource }}</span>
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
              <BaseIcon name="wrench" size="xs" /> <span>Right-Size</span>
            </button>
          </template>
        </DataTable>
      </div>
    </div>

    <!-- Mobile High-Density Cards Stream (<768px, ~65-72px/item) -->
    <div class="mobile-only cost-mobile-container">
      <CostMobileCards
        :namespaces="filteredNamespaces"
        :waste-alerts="filteredWasteAlerts"
        :loading="loading"
        @right-size="handleDismissWaste"
        @select-namespace="openNamespaceBreakdown"
      />
    </div>

    <!-- Mobile Multi-Cloud Breakdown Drawer (<768px) -->
    <ModalDrawer
      :show="showMobileBreakdown"
      title="FinOps Cost Breakdown"
      :subtitle="`Multi-Cloud Run-Rate: $${totalMonthlyCost.toLocaleString()}/mo (${clusters.length} clusters)`"
      mode="drawer"
      placement="right"
      max-width="420px"
      @close="showMobileBreakdown = false"
    >
      <div class="mobile-drawer-breakdown font-mono">
        <div class="breakdown-hero glass-panel">
          <div class="hero-label">MONTHLY RUN-RATE</div>
          <div class="hero-val font-mono">${{ totalMonthlyCost.toLocaleString() }}</div>
          <div class="hero-sub text-muted">Daily avg: ${{ totalDailyCost }}/day · Idle Waste: ${{ totalWastedCost }}/mo</div>
        </div>

        <div class="resource-split-section">
          <h4 class="subhead">Multi-Cloud Distribution</h4>
          <div class="breakdown-bars">
            <div v-for="item in cloudBreakdown" :key="item.provider" class="split-row">
              <div class="split-meta">
                <span :style="{ color: item.color }">{{ item.name }} ({{ item.percentage }}%)</span>
                <span>${{ item.cost.toLocaleString() }}</span>
              </div>
              <div class="util-bar-bg">
                <div class="util-bar-fill" :style="{ width: `${item.percentage}%`, backgroundColor: item.color }"></div>
              </div>
            </div>
          </div>
        </div>

        <div class="recommendations-box glass-panel">
          <h4 class="rec-title"><BaseIcon name="zap" size="xs" /> FinOps Right-Sizing Insights</h4>
          <p class="rec-desc">
            Spot usage is at <strong>{{ spotRatio }}%</strong> saving <strong>${{ spotSavings.toLocaleString() }}/mo</strong>.
            Total actionable idle waste is <strong class="text-rose">${{ totalWastedCost.toLocaleString() }}/mo</strong>.
          </p>
        </div>
      </div>
    </ModalDrawer>

    <!-- Namespace Detail Inspection Drawer -->
    <ModalDrawer
      :show="selectedNamespace !== null"
      :title="`Cost Breakdown: ${selectedNamespace?.namespace || ''}`"
      :subtitle="`Assigned Team: ${selectedNamespace?.team || ''} | Cluster: ${selectedNamespace?.cluster || ''}`"
      mode="drawer"
      placement="right"
      max-width="480px"
      @close="closeNamespaceBreakdown"
    >
      <div v-if="selectedNamespace" class="breakdown-details font-mono">
        <div class="breakdown-hero glass-panel">
          <div class="hero-label">CURRENT MONTHLY SPEND</div>
          <div class="hero-val font-mono">${{ selectedNamespace.monthly_cost.toLocaleString() }} <small>/mo</small></div>
          <div class="hero-sub">
            Budget Burn: <span :class="selectedNamespace.budget_utilization > 100 ? 'text-rose' : 'text-emerald'">
              {{ selectedNamespace.budget_utilization }}% of ${{ selectedNamespace.budget_limit.toLocaleString() }}
            </span>
          </div>
        </div>

        <div class="resource-split-section">
          <h4 class="subhead">Resource Component Breakdown</h4>
          <div class="breakdown-bars">
            <div class="split-row">
              <div class="split-meta">
                <span class="text-cyan">Compute (CPU) - {{ selectedNamespace.cpu_requested }}</span>
                <span>${{ Math.round(selectedNamespace.monthly_cost * 0.45).toLocaleString() }}</span>
              </div>
              <div class="util-bar-bg"><div class="util-bar-fill bg-cyan" style="width: 45%"></div></div>
            </div>
            <div class="split-row">
              <div class="split-meta">
                <span class="text-violet">Memory (RAM) - {{ selectedNamespace.memory_requested }}</span>
                <span>${{ Math.round(selectedNamespace.monthly_cost * 0.35).toLocaleString() }}</span>
              </div>
              <div class="util-bar-bg"><div class="util-bar-fill bg-violet" style="width: 35%"></div></div>
            </div>
            <div class="split-row">
              <div class="split-meta">
                <span class="text-amber">Storage (PV / PVCs)</span>
                <span>${{ Math.round(selectedNamespace.monthly_cost * 0.2).toLocaleString() }}</span>
              </div>
              <div class="util-bar-bg"><div class="util-bar-fill bg-amber" style="width: 20%"></div></div>
            </div>
          </div>
        </div>

        <div class="recommendations-box glass-panel">
          <h4 class="rec-title"><BaseIcon name="zap" size="xs" /> FinOps Right-Sizing Insights</h4>
          <p class="rec-desc">
            Historical CPU utilization sits at <strong>{{ selectedNamespace.utilization }}%</strong>.
            Downscaling requests by 20% recovers approx.
            <strong class="text-emerald">${{ Math.round(selectedNamespace.monthly_cost * 0.2).toLocaleString() }}/mo</strong>.
          </p>
        </div>
      </div>
    </ModalDrawer>

    <!-- Global FinOps Budget Limits Governance Modal -->
    <ModalDrawer
      :show="showBudgetLimitsModal"
      title="FinOps Budget Governance & Allocations"
      subtitle="Configure monthly spending caps and alert thresholds across team namespaces"
      mode="modal"
      max-width="520px"
      @close="showBudgetLimitsModal = false"
    >
      <div class="budget-modal-content font-mono">
        <div class="budget-modal-desc text-muted">
          Adjust namespace budget thresholds below. Allocations reaching &gt;80% trigger FinOps alerts, while &gt;100% flag cost overruns.
        </div>
        <div class="budget-list">
          <div v-for="ns in namespaces" :key="ns.namespace" class="budget-list-row glass-panel">
            <div class="budget-row-meta">
              <span class="budget-row-ns font-bold text-white">{{ ns.namespace }}</span>
              <span class="budget-row-team text-muted">{{ ns.team }} · Spend: ${{ ns.monthly_cost.toLocaleString() }}/mo</span>
            </div>
            <div class="budget-row-controls">
              <div class="budget-input-inline">
                <span class="currency-sym">$</span>
                <input
                  type="number"
                  min="100"
                  step="500"
                  :value="ns.budget_limit"
                  class="budget-num-input font-mono"
                  @change="(e) => setBudgetLimit(ns.namespace, Number((e.target as HTMLInputElement).value))"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="modal-actions">
          <button class="btn btn-primary" @click="showBudgetLimitsModal = false">Done</button>
        </div>
      </template>
    </ModalDrawer>
  </div>
</template>
