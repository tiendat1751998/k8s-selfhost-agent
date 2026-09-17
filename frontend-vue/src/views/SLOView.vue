<script setup lang="ts">
import { ref, computed } from 'vue'
import '../assets/styles/views/slo.css'
import BaseIcon from '../components/ui/BaseIcon.vue'
import SloCardsGrid from '../components/slo/SloCardsGrid.vue'
import SloCatalogTable from '../components/slo/SloCatalogTable.vue'
import SloMobileCards from '../components/slo/SloMobileCards.vue'
import SloCreateModal from '../components/slo/SloCreateModal.vue'
import SloInspectModal from '../components/slo/SloInspectModal.vue'
import { useSLOMonitor } from '../composables/useSLOMonitor'
import type { SLODefinition, SLOSnapshot } from '../api/compute'

const {
  loading,
  actionInProgress,
  error,
  bannerMessage,
  definitions,
  snapshots,
  selectedWindowFilter,
  showCreateModal,
  showInspectModal,
  selectedInspectSLO,
  realServices,
  totalSLOs,
  healthySLOs,
  warningSLOs,
  criticalSLOs,
  activeBurnAlerts,
  avgBurnRate,
  avgBurnRateNum,
  fetchSLOData,
  setWindowFilter,
  formatPercent,
  getEffectiveBurnRate,
  getBurnRateColor,
  getBudgetBarWidth,
  getSnapshotForDef,
  openCreateModal,
  openInspect,
  handleCreateSLO,
  handleTriggerAlert,
  handleDeleteSLO,
} = useSLOMonitor()

const windowPills = [
  { key: '1h' as const, label: '1h (Fast)', title: '1h Fast Burn (14.4x rate)', icon: 'flame', short: 'Fast' },
  { key: '6h' as const, label: '6h (Slow)', title: '6h Slow Burn (6.0x rate)', icon: 'alert-triangle', short: 'Slow' },
  { key: '24h' as const, label: '24h (Composite)', title: '24h Composite (2.0x rate)', icon: 'activity', short: 'Comp' },
  { key: '30d' as const, label: '30d (Baseline)', title: '30d Baseline (1.0x rate)', icon: 'calendar', short: 'Base' },
]

// View Mode and Search State
const viewMode = ref<'table' | 'grid'>('table')
const searchQuery = ref('')
const showMobileSearch = ref(false)

// Filtered definitions based on search query
const filteredDefinitions = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return definitions.value
  return definitions.value.filter(d =>
    d.service.toLowerCase().includes(q) ||
    (d.indicator_type && d.indicator_type.toLowerCase().includes(q))
  )
})

// Filtered snapshots based on search query
const filteredSnapshots = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return snapshots.value
  const matchingDefIds = new Set(
    definitions.value
      .filter(d => d.service.toLowerCase().includes(q) || (d.indicator_type && d.indicator_type.toLowerCase().includes(q)))
      .map(d => d.id)
  )
  return snapshots.value.filter(s =>
    s.service.toLowerCase().includes(q) ||
    matchingDefIds.has(s.slo_id)
  )
})

function openInspectFromTable(row: SLODefinition) {
  openInspect(row, snapshots.value.find(s => s.slo_id === row.id || s.service === row.service))
}

function handleMobileInspect(def: SLODefinition, snap?: SLOSnapshot) {
  openInspect(def, snap)
}

function handleEditSLO(def: SLODefinition) {
  openCreateModal(def)
}
</script>

<template>
  <div class="slo-view-container animate-fade-in">
    <!-- Alert / Toast Banner -->
    <div v-if="bannerMessage" class="banner-box animate-fade-in" :class="`banner-${bannerMessage.type}`">
      <BaseIcon :name="bannerMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" />
      <span class="banner-text">{{ bannerMessage.text }}</span>
      <button class="banner-close" @click="bannerMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Standard 4-Card KPI Strip (Desktop & Tablet) -->
    <div class="slo-kpi-grid desktop-only" role="region" aria-label="SLO Summary Metrics">
      <!-- Card 1: Total Objectives -->
      <div class="slo-kpi-card glass-panel" title="Total active SLO target definitions">
        <div class="kpi-card-header">
          <div class="kpi-card-title-group">
            <BaseIcon name="target" size="xs" class="kpi-card-icon text-cyan" />
            <span class="kpi-card-title">Total Objectives</span>
          </div>
          <span class="badge kpi-badge badge-cyan">{{ totalSLOs }} TARGETS</span>
        </div>
        <div class="kpi-card-body">
          <span class="kpi-card-value font-mono">{{ totalSLOs }}</span>
          <span class="kpi-card-trend font-mono">Configured Objectives</span>
        </div>
        <div class="kpi-card-gauge">
          <div class="kpi-gauge-track">
            <div class="kpi-gauge-fill gauge-cyan" :style="{ width: totalSLOs > 0 ? '100%' : '0%' }"></div>
          </div>
        </div>
      </div>

      <!-- Card 2: Healthy Objectives -->
      <div class="slo-kpi-card glass-panel" title="SLO targets meeting compliance within error budget">
        <div class="kpi-card-header">
          <div class="kpi-card-title-group">
            <BaseIcon name="shield" size="xs" class="kpi-card-icon text-emerald" />
            <span class="kpi-card-title">Healthy Objectives</span>
          </div>
          <span class="badge kpi-badge badge-emerald">
            {{ totalSLOs > 0 ? Math.round((healthySLOs / totalSLOs) * 100) + '%' : '--' }}
          </span>
        </div>
        <div class="kpi-card-body">
          <span class="kpi-card-value font-mono">{{ healthySLOs }} / {{ totalSLOs }}</span>
          <span class="kpi-card-trend font-mono">
            {{ totalSLOs > 0 ? (healthySLOs === totalSLOs ? '100% Compliant' : `${healthySLOs} within budget`) : 'No SLOs' }}
          </span>
        </div>
        <div class="kpi-card-gauge">
          <div class="kpi-gauge-track">
            <div
              class="kpi-gauge-fill gauge-emerald"
              :style="{ width: totalSLOs > 0 ? `${(healthySLOs / totalSLOs) * 100}%` : '0%' }"
            ></div>
          </div>
        </div>
      </div>

      <!-- Card 3: Active Burn Alerts -->
      <div class="slo-kpi-card glass-panel" title="SLO targets actively depleting error budgets at elevated rates">
        <div class="kpi-card-header">
          <div class="kpi-card-title-group">
            <BaseIcon name="flame" size="xs" class="kpi-card-icon" :class="activeBurnAlerts === 0 ? 'text-emerald' : 'text-rose'" />
            <span class="kpi-card-title">Active Burn Alerts</span>
          </div>
          <span class="badge kpi-badge" :class="activeBurnAlerts === 0 ? 'badge-emerald' : 'badge-rose'">
            {{ activeBurnAlerts === 0 ? 'NOMINAL' : 'ALERT' }}
          </span>
        </div>
        <div class="kpi-card-body">
          <span class="kpi-card-value font-mono" :class="activeBurnAlerts === 0 ? 'text-emerald' : 'text-rose'">
            {{ activeBurnAlerts }} Active
          </span>
          <span class="kpi-card-trend font-mono">
            {{ activeBurnAlerts === 0 ? 'Zero fast depletions' : `${activeBurnAlerts} targets alerting` }}
          </span>
        </div>
        <div class="kpi-card-gauge">
          <div class="kpi-gauge-track">
            <div
              class="kpi-gauge-fill"
              :class="activeBurnAlerts === 0 ? 'gauge-emerald' : 'gauge-rose'"
              :style="{ width: activeBurnAlerts === 0 ? '100%' : `${Math.min(100, activeBurnAlerts * 33)}%` }"
            ></div>
          </div>
        </div>
      </div>

      <!-- Card 4: Avg Burn Velocity -->
      <div class="slo-kpi-card glass-panel" title="Average error budget consumption rate across all active services">
        <div class="kpi-card-header">
          <div class="kpi-card-title-group">
            <BaseIcon name="activity" size="xs" class="kpi-card-icon" :class="avgBurnRateNum <= 1.0 ? 'text-emerald' : avgBurnRateNum <= 2.0 ? 'text-amber' : 'text-rose'" />
            <span class="kpi-card-title">Avg Burn Velocity</span>
          </div>
          <span class="badge kpi-badge" :class="avgBurnRateNum <= 1.0 ? 'badge-emerald' : avgBurnRateNum <= 2.0 ? 'badge-amber' : 'badge-rose'">
            {{ avgBurnRateNum <= 1.0 ? 'NOMINAL' : avgBurnRateNum <= 2.0 ? 'ELEVATED' : 'FAST BURN' }}
          </span>
        </div>
        <div class="kpi-card-body">
          <span class="kpi-card-value font-mono" :class="avgBurnRateNum <= 1.0 ? 'text-emerald' : avgBurnRateNum <= 2.0 ? 'text-amber' : 'text-rose'">
            {{ avgBurnRate }}
          </span>
          <span class="kpi-card-trend font-mono">
            {{ avgBurnRateNum <= 1.0 ? 'Budget Positive' : avgBurnRateNum <= 2.0 ? 'Slow Depletion' : 'Fast Exhaustion' }}
          </span>
        </div>
        <div class="kpi-card-gauge">
          <div class="kpi-gauge-track">
            <div
              class="kpi-gauge-fill"
              :class="avgBurnRateNum <= 1.0 ? 'gauge-emerald' : avgBurnRateNum <= 2.0 ? 'gauge-amber' : 'gauge-rose'"
              :style="{ width: `${Math.min(100, avgBurnRateNum * 50)}%` }"
            ></div>
          </div>
        </div>
      </div>
    </div>

    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="slo-toolbar-sleek glass-panel desktop-only">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter SLOs..."
          class="toolbar-search-input"
          aria-label="Filter SLOs by service name or indicator"
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

      <!-- Time Window multi-window analysis pills -->
      <div class="toolbar-window-pills font-mono" role="tablist" aria-label="Multi-window analysis">
        <button
          v-for="w in windowPills"
          :key="w.key"
          type="button"
          role="tab"
          :aria-selected="selectedWindowFilter === w.key"
          class="toolbar-pill-btn"
          :class="{ active: selectedWindowFilter === w.key }"
          :title="w.title"
          @click="setWindowFilter(w.key)"
        >
          <BaseIcon :name="w.icon" size="xs" />
          <span>{{ w.label }}</span>
        </button>
      </div>

      <!-- Subtle monospace status in muted slate -->
      <div class="toolbar-kpi-status font-mono" role="status" aria-label="SLO metrics summary">
        <span class="kpi-live-dot" aria-hidden="true"></span>
        <span>{{ totalSLOs }} SLOs ({{ healthySLOs }} Healthy)</span>
      </div>

      <!-- Right: Segmented viewMode toggle & Action buttons -->
      <div class="toolbar-actions-group">
        <!-- Segmented viewMode toggle: [ Table ] and [ Cards ] -->
        <div class="view-mode-toggle font-mono" role="group" aria-label="View mode">
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'table' }"
            title="Catalog Table View"
            aria-label="Table View"
            @click="viewMode = 'table'"
          >
            <BaseIcon name="table" size="xs" />
            <span>Table</span>
          </button>
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'grid' }"
            title="Card Grid View"
            aria-label="Cards View"
            @click="viewMode = 'grid'"
          >
            <BaseIcon name="grid" size="xs" />
            <span>Cards</span>
          </button>
        </div>

        <!-- Action buttons: + Add Target (primary) and Refresh (secondary with spinner) -->
        <button
          type="button"
          class="toolbar-btn btn-primary"
          title="Add Target"
          aria-label="Add Target"
          @click="openCreateModal()"
        >
          <BaseIcon name="plus" size="xs" />
          <span>Add Target</span>
        </button>

        <button
          type="button"
          class="toolbar-btn btn-secondary"
          :disabled="loading"
          title="Refresh SLO telemetry"
          aria-label="Refresh SLO telemetry"
          @click="fetchSLOData"
        >
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" :class="{ 'spin-icon': loading }" />
          <span>{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile 44px Command Bar (<=767px) -->
    <div class="slo-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="target" size="xs" /> SLOs ({{ totalSLOs }})</span>
      </div>
      <div class="command-bar-actions">
        <button class="btn-icon-cmd" title="Create SLO definition" aria-label="Create SLO definition" @click="openCreateModal()">
          <BaseIcon name="plus" size="xs" />
        </button>
        <button class="btn-icon-cmd" :disabled="loading" title="Refresh telemetry" aria-label="Refresh telemetry" @click="fetchSLOData">
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" :class="{ 'spin-icon': loading }" />
        </button>
        <button class="btn-icon-cmd" :class="{ active: showMobileSearch }" title="Toggle search/filter drawer" aria-label="Toggle search/filter drawer" @click="showMobileSearch = !showMobileSearch">
          <BaseIcon name="search" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<=767px) -->
    <div class="slo-micro-telemetry mobile-only font-mono" role="status" aria-label="SLO Micro Telemetry">
      <span class="tel-item tel-total"><BaseIcon name="target" size="xs" /> {{ totalSLOs }} slos</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-healthy"><BaseIcon name="shield" size="xs" /> {{ healthySLOs }} ok</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-warn"><BaseIcon name="alert-triangle" size="xs" /> {{ warningSLOs + criticalSLOs }} warn</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-burn"><BaseIcon name="flame" size="xs" /> {{ avgBurnRate }} burn</span>
    </div>

    <!-- Mobile Collapsible Search Drawer -->
    <div v-if="showMobileSearch" class="mobile-filter-drawer mobile-only animate-fade-in">
      <div class="mobile-search-inner">
        <BaseIcon name="search" size="xs" class="mobile-search-icon" />
        <input
          v-model="searchQuery"
          type="search"
          class="mobile-search-input"
          placeholder="Filter SLOs by service name or indicator..."
          autofocus
        />
        <button v-if="searchQuery" type="button" class="mobile-clear-btn" title="Clear search" @click="searchQuery = ''"><BaseIcon name="x" size="xs" /></button>
      </div>
    </div>

    <!-- Mobile Time Window Pills (<768px) -->
    <div class="slo-mobile-window-strip mobile-only">
      <div class="mobile-window-pills font-mono">
        <button
          v-for="w in windowPills"
          :key="w.key"
          type="button"
          class="mobile-window-btn"
          :class="{ active: selectedWindowFilter === w.key }"
          @click="setWindowFilter(w.key)"
        >
          {{ w.key }} ({{ w.short }})
        </button>
      </div>
    </div>

    <!-- Desktop View Mode: Table OR Grid (NEVER both at the same time on desktop!) -->
    <SloCatalogTable
      v-if="viewMode === 'table'"
      class="desktop-only"
      :definitions="filteredDefinitions"
      :snapshots="filteredSnapshots"
      :loading="loading"
      :error="error"
      @inspect="openInspectFromTable"
      @trigger-alert="handleTriggerAlert"
      @delete-slo="handleDeleteSLO"
    />

    <SloCardsGrid
      v-else-if="viewMode === 'grid'"
      class="desktop-only"
      :definitions="filteredDefinitions"
      :snapshots="filteredSnapshots"
      :selected-window-filter="selectedWindowFilter"
      :action-in-progress="actionInProgress"
      @create-slo="openCreateModal()"
      @inspect="(p) => openInspect(p.def, p.snap)"
      @trigger-alert="handleTriggerAlert"
      @delete-slo="handleDeleteSLO"
    />

    <!-- First-Class Mobile Card Stream (<768px) -->
    <SloMobileCards
      class="mobile-only"
      :definitions="filteredDefinitions"
      :snapshots="filteredSnapshots"
      :format-percent="formatPercent"
      :get-effective-burn-rate="getEffectiveBurnRate"
      :get-burn-rate-color="getBurnRateColor"
      :get-budget-bar-width="getBudgetBarWidth"
      :get-snapshot-for-def="getSnapshotForDef"
      @inspect="handleMobileInspect"
      @edit="handleEditSLO"
      @delete="handleDeleteSLO"
    />

    <!-- Modals -->
    <SloCreateModal
      v-model:show="showCreateModal"
      :real-services="realServices"
      :action-in-progress="actionInProgress"
      @create="handleCreateSLO"
    />
    <SloInspectModal
      v-model:show="showInspectModal"
      :inspect-s-l-o="selectedInspectSLO"
      :action-in-progress="actionInProgress"
      @trigger-alert="handleTriggerAlert"
    />
  </div>
</template>
