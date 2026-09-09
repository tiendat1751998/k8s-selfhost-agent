<script setup lang="ts">
import { useCostFinOps } from '../composables/useCostFinOps'
import CostHudMetrics from '../components/cost/CostHudMetrics.vue'
import CostBreakdownChart from '../components/cost/CostBreakdownChart.vue'
import NamespaceCostTable from '../components/cost/NamespaceCostTable.vue'
import CostMobileCards from '../components/cost/CostMobileCards.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import type { ResourceWaste } from '../api/governance'

const {
  clusters,
  namespaces,
  wasteAlerts,
  loading,
  error,
  statusMessage,
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
  formatWasteType,
} = useCostFinOps()

const wasteColumns: Column<ResourceWaste>[] = [
  { key: 'severity', label: 'Severity', width: '120px', sortable: true },
  { key: 'type', label: 'Waste Category', width: '180px', sortable: true },
  { key: 'resource', label: 'Impacted Resource', width: '240px', sortable: true },
  { key: 'util', label: 'Measured Util', width: '150px' },
  { key: 'wasted_cost', label: 'Idle Cost', width: '140px', sortable: true },
  { key: 'actions', label: 'Action', width: '140px', align: 'right' },
]
</script>

<template>
  <div class="view-container">
    <!-- Desktop View Header -->
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

    <!-- 44px Mobile Command Bar (<640px) -->
    <div class="mobile-command-bar cost-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">💵 FinOps (${{ (totalMonthlyCost / 1000).toFixed(1) }}k/mo)</span>
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
      </div>
    </div>

    <!-- 20px Mobile Micro-Telemetry Strip (<640px) -->
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

    <!-- Desktop Metrics HUD Grid -->
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

    <!-- Multi-Cloud Infrastructure Cost Breakdown & Clusters -->
    <CostBreakdownChart
      class="desktop-only"
      :cloud-breakdown="cloudBreakdown"
      :clusters="clusters"
      :loading="loading"
      @refresh="fetchCostData"
    />

    <!-- Section 1: Namespace Cost Allocations -->
    <NamespaceCostTable
      class="desktop-only"
      :namespaces="namespaces"
      :loading="loading"
      :error="error"
      @save-budget="setBudgetLimit"
      @select-namespace="openNamespaceBreakdown"
    />

    <!-- Section 2: Resource Waste & Idle Allocation Alerts -->
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

    <!-- Mobile Screen 1 Cards Stream (<640px) -->
    <div class="mobile-only cost-mobile-container">
      <CostMobileCards
        :waste-alerts="wasteAlerts"
        :loading="loading"
        @right-size="handleDismissWaste"
      />
    </div>
  </div>
</template>

<style scoped>
@import '../assets/styles/views/cost.css';
</style>
