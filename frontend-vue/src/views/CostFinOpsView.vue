<script setup lang="ts">
import { ref } from 'vue'
import { useCostFinOps } from '../composables/useCostFinOps'
import CostHudMetrics from '../components/cost/CostHudMetrics.vue'
import CostBreakdownChart from '../components/cost/CostBreakdownChart.vue'
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
    <!-- Desktop View Header (>=768px) -->
    <header class="view-header desktop-header-wrap desktop-only">
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
    </header>

    <!-- 44px Mobile Command Bar (<768px) -->
    <div class="mobile-command-bar cost-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">💰 Cost FinOps (${{ (totalMonthlyCost / 1000).toFixed(1) }}k/mo)</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh Cost Metrics"
          aria-label="Refresh"
          @click="fetchCostData"
        >
          <span>{{ loading ? '⏳' : '🔄' }}</span>
        </button>
        <button
          class="btn-icon-cmd"
          title="FinOps Cost Breakdown"
          aria-label="Cost Breakdown"
          @click="showMobileBreakdown = true"
        >
          <span>📊</span>
        </button>
      </div>
    </div>

    <!-- 20px Mobile Micro-Telemetry Strip (<768px) -->
    <div class="mobile-micro-telemetry cost-micro-telemetry mobile-only font-mono" role="status" aria-label="Cost FinOps Micro Telemetry">
      <span class="tel-item tel-monthly">💵 ${{ (totalMonthlyCost / 1000).toFixed(1) }}k/mo</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-daily">⚡ ${{ totalDailyCost }}/d</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-waste">📉 ${{ totalWastedCost }} waste</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-spot">🏷️ {{ spotRatio }}% spot</span>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
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
      :clusters="clusters"
      :loading="loading"
      @refresh="fetchCostData"
    />

    <!-- Section 1: Namespace Cost Allocations (>=768px) -->
    <NamespaceCostTable
      class="desktop-only"
      :namespaces="namespaces"
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
        <span class="badge badge-rose">{{ wasteAlerts.length }} Waste Findings</span>
      </div>

      <div class="cost-desktop-table">
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
              <span>Right-Size 🔧</span>
            </button>
          </template>
        </DataTable>
      </div>
    </div>

    <!-- Mobile High-Density Cards Stream (<768px, ~65-72px/item) -->
    <div class="mobile-only cost-mobile-container">
      <CostMobileCards
        :namespaces="namespaces"
        :waste-alerts="wasteAlerts"
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
          <h4 class="rec-title">⚡ FinOps Right-Sizing Insights</h4>
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
          <h4 class="rec-title">⚡ FinOps Right-Sizing Insights</h4>
          <p class="rec-desc">
            Historical CPU utilization sits at <strong>{{ selectedNamespace.utilization }}%</strong>.
            Downscaling requests by 20% recovers approx.
            <strong class="text-emerald">${{ Math.round(selectedNamespace.monthly_cost * 0.2).toLocaleString() }}/mo</strong>.
          </p>
        </div>
      </div>
    </ModalDrawer>
  </div>
</template>

<style scoped>
@import '../assets/styles/views/cost.css';
@import '../assets/styles/components/cost-drawers.css';

.cost-desktop-table :deep(.table-scroll-wrapper) {
  overflow-x: auto !important;
  width: 100%;
}

.cost-desktop-table :deep(.data-table) {
  table-layout: fixed !important;
  width: 100%;
  min-width: 720px;
}
</style>
