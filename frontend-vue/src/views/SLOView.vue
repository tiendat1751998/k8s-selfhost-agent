<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import '../assets/styles/views/slo.css'
import MetricCard from '../components/ui/MetricCard.vue'
import SloCardsGrid from '../components/slo/SloCardsGrid.vue'
import SloCatalogTable from '../components/slo/SloCatalogTable.vue'
import SloMobileCards from '../components/slo/SloMobileCards.vue'
import SloCreateModal from '../components/slo/SloCreateModal.vue'
import SloInspectModal from '../components/slo/SloInspectModal.vue'
import {
  sloApi, dockerApi, type SLODefinition, type SLOSnapshot, type CreateSLOPayload
} from '../api/compute'

const loading = ref(false)
const actionInProgress = ref(false)
const error = ref<string | null>(null)
const bannerMessage = ref<{ type: 'success' | 'warning' | 'error'; text: string } | null>(null)

const definitions = ref<SLODefinition[]>([])
const snapshots = ref<SLOSnapshot[]>([])

type TimeWindowFilter = '1h' | '6h' | '24h' | '30d'
const selectedWindowFilter = ref<TimeWindowFilter>('30d')

// View Mode and Search State
const viewMode = ref<'table' | 'grid'>('table')
const searchQuery = ref('')
const showMobileSearch = ref(false)

const showCreateModal = ref(false)
const showInspectModal = ref(false)
const selectedInspectSLO = ref<{ def?: SLODefinition; snap?: SLOSnapshot } | null>(null)

interface ServiceOption { id: string; name: string; desc: string }
const realServices = ref<ServiceOption[]>([
  { id: 'custom', name: 'Custom Workload...', desc: 'Enter custom service name' }
])
const loadingServices = ref(false)

async function fetchRealServices() {
  loadingServices.value = true
  try {
    const [servicesRes, containersRes] = await Promise.allSettled([
      dockerApi.listServices(),
      dockerApi.listContainers(),
    ])
    const found = new Map<string, ServiceOption>()
    if (servicesRes.status === 'fulfilled' && Array.isArray(servicesRes.value)) {
      for (const s of servicesRes.value) {
        if (s.name) found.set(s.name, { id: s.name, name: s.name, desc: s.image ? `Docker Service (${s.image})` : 'Docker Swarm Service' })
      }
    }
    if (containersRes.status === 'fulfilled' && Array.isArray(containersRes.value)) {
      for (const c of containersRes.value) {
        const name = c.name?.replace(/^\//, '')
        if (name && !found.has(name)) found.set(name, { id: name, name, desc: c.image ? `Container (${c.image})` : `Container (${c.status || 'running'})` })
      }
    }
    realServices.value = [...Array.from(found.values()), { id: 'custom', name: 'Custom Workload...', desc: 'Enter custom service name' }]
  } catch {
    realServices.value = [{ id: 'custom', name: 'Custom Workload...', desc: 'Enter custom service name' }]
  } finally {
    loadingServices.value = false
  }
}

async function fetchSLOData() {
  loading.value = true
  error.value = null
  try {
    const [defsRes, snapRes] = await Promise.allSettled([
      sloApi.listDefinitions(),
      sloApi.listSnapshots(selectedWindowFilter.value)
    ])
    if (defsRes.status === 'fulfilled') definitions.value = defsRes.value
    if (snapRes.status === 'fulfilled') snapshots.value = snapRes.value
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : 'Failed to retrieve SLO telemetry'
  } finally {
    loading.value = false
  }
}

function setWindowFilter(filter: TimeWindowFilter) {
  selectedWindowFilter.value = filter
  fetchSLOData()
}

onMounted(() => {
  fetchSLOData()
  fetchRealServices()
})

// Computed Metrics
const totalSLOs = computed(() => definitions.value.length)
const healthySLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'healthy').length)
const warningSLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'warning').length)
const criticalSLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'critical').length)
const avgBurnRate = computed(() => {
  if (!snapshots.value.length) return '—'
  return `${(snapshots.value.reduce((acc, s) => acc + (s.burn_rate || 0), 0) / snapshots.value.length).toFixed(2)}x`
})

// Filtered data based on unified search input
const filteredSnapshots = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return q ? snapshots.value.filter(s => s.service.toLowerCase().includes(q)) : snapshots.value
})

const filteredDefinitions = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return q ? definitions.value.filter(d => d.service.toLowerCase().includes(q) || d.indicator_type.toLowerCase().includes(q)) : definitions.value
})

// Helper Functions for Mobile Card Stream and Telemetry
function formatPercent(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0.00%'
  const pct = val > 1 ? val : val * 100
  return `${pct.toFixed(2)}%`
}

function getEffectiveBurnRate(rawRate?: number): number {
  if (rawRate === undefined || rawRate === null || isNaN(rawRate)) return 0
  return rawRate
}

function getBurnRateColor(rate: number): string {
  if (rate <= 1.0) return 'text-emerald'
  if (rate <= 2.5) return 'text-amber'
  return 'text-rose'
}

function getBudgetBarWidth(budget?: number): number {
  if (budget === undefined || budget === null || isNaN(budget)) return 0
  return Math.max(0, Math.min(100, budget))
}

function getSnapshotForDef(defId: string, serviceName: string): SLOSnapshot | undefined {
  return snapshots.value.find(s => s.slo_id === defId || s.service === serviceName)
}

function handleMobileInspect(def: SLODefinition, snap?: SLOSnapshot) {
  openInspect({ def, snap })
}

function handleEditSLO(def: SLODefinition) {
  const snap = getSnapshotForDef(def.id, def.service)
  openInspect({ def, snap })
}

function showBanner(type: 'success' | 'warning' | 'error', text: string) {
  bannerMessage.value = { type, text }
  setTimeout(() => { if (bannerMessage.value?.text === text) bannerMessage.value = null }, 5000)
}

function openInspect(payload: { def?: SLODefinition; snap?: SLOSnapshot }) {
  selectedInspectSLO.value = payload
  showInspectModal.value = true
}

function openInspectFromTable(row: SLODefinition) {
  openInspect({ def: row, snap: snapshots.value.find(s => s.slo_id === row.id || s.service === row.service) })
}

async function handleCreateSLO(payload: CreateSLOPayload) {
  actionInProgress.value = true
  try {
    await sloApi.createDefinition(payload)
    showCreateModal.value = false
    showBanner('success', `SLO target objective successfully armed for service ${payload.service}!`)
    await fetchSLOData()
  } catch (err: unknown) {
    showBanner('error', err instanceof Error ? err.message : 'Failed to create SLO definition')
  } finally {
    actionInProgress.value = false
  }
}

async function handleDeleteSLO(id: string, serviceName: string) {
  if (!confirm(`Are you sure you want to delete SLO definition for '${serviceName}'?`)) return
  actionInProgress.value = true
  try {
    await sloApi.deleteDefinition(id)
    showBanner('success', `SLO definition for '${serviceName}' removed.`)
    await fetchSLOData()
  } catch (err: unknown) {
    showBanner('error', err instanceof Error ? err.message : 'Failed to delete SLO')
  } finally {
    actionInProgress.value = false
  }
}

async function handleTriggerAlert(id: string, serviceName: string) {
  actionInProgress.value = true
  try {
    await sloApi.triggerBurnAlert(id)
    showBanner('warning', `⚡ Fast burn-rate alert simulated for '${serviceName}'. Check alerts view.`)
  } catch (err: unknown) {
    showBanner('error', err instanceof Error ? err.message : 'Failed to simulate alert')
  } finally {
    actionInProgress.value = false
  }
}
</script>

<template>
  <div class="slo-view-container animate-fade-in">
    <!-- Desktop Header (>640px) -->
    <div class="view-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>ENTERPRISE RELIABILITY ENGINEERING & OBSERVABILITY</span>
        </div>
        <h1 class="view-title">Service Level Objectives & Error Budgets</h1>
        <p class="view-desc">
          Automated multi-window burn rate calculation, Google SRE error budgeting, and real-time PromQL telemetry compliance.
        </p>
      </div>

      <div class="header-actions">
        <!-- Segmented View Mode Toggle: [ 📑 Table ] [ 🗂 Cards ] -->
        <div class="segmented-control font-mono">
          <button
            type="button"
            class="segmented-btn"
            :class="{ active: viewMode === 'table' }"
            @click="viewMode = 'table'"
            title="Catalog Table View"
          >
            <span>📑 Table</span>
          </button>
          <button
            type="button"
            class="segmented-btn"
            :class="{ active: viewMode === 'grid' }"
            @click="viewMode = 'grid'"
            title="Card Grid View"
          >
            <span>🗂 Cards</span>
          </button>
        </div>

        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>➕ Create SLO Definition</span>
        </button>
        <button class="btn btn-secondary" :disabled="loading" @click="fetchSLOData">
          <span>{{ loading ? '⏳ Fetching...' : '🔄 Refresh Telemetry' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile 44px Command Bar (<=640px) -->
    <div class="slo-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🎯 SLOs ({{ totalSLOs }})</span>
      </div>
      <div class="command-bar-actions">
        <button class="btn-icon-cmd" title="Create SLO definition" aria-label="Create SLO definition" @click="showCreateModal = true">
          <span>➕</span>
        </button>
        <button class="btn-icon-cmd" :disabled="loading" title="Refresh telemetry" aria-label="Refresh telemetry" @click="fetchSLOData">
          <span>🔄</span>
        </button>
        <button class="btn-icon-cmd" :class="{ active: showMobileSearch }" title="Toggle search/filter drawer" aria-label="Toggle search/filter drawer" @click="showMobileSearch = !showMobileSearch">
          <span>🔍</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<=640px) -->
    <div class="slo-micro-telemetry mobile-only font-mono" role="status" aria-label="SLO Micro Telemetry">
      <span class="tel-item tel-total">🎯 {{ totalSLOs }} slos</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-healthy">🛡️ {{ healthySLOs }} ok</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-warn">⚠️ {{ warningSLOs + criticalSLOs }} warn</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-burn">🔥 {{ avgBurnRate }} burn</span>
    </div>

    <!-- Mobile Collapsible Search Drawer -->
    <div v-if="showMobileSearch" class="mobile-filter-drawer mobile-only animate-fade-in">
      <div class="mobile-search-inner">
        <span class="mobile-search-icon">🔍</span>
        <input
          v-model="searchQuery"
          type="search"
          class="mobile-search-input font-mono"
          placeholder="Filter SLOs by service name..."
          autofocus
        />
        <button v-if="searchQuery" type="button" class="mobile-clear-btn" title="Clear search" @click="searchQuery = ''">✕</button>
      </div>
    </div>

    <!-- Alert / Toast Banner -->
    <div v-if="bannerMessage" class="banner-box animate-fade-in" :class="`banner-${bannerMessage.type}`">
      <span>{{ bannerMessage.type === 'success' ? '✅' : bannerMessage.type === 'warning' ? '⚠️' : '🚨' }}</span>
      <span class="banner-text">{{ bannerMessage.text }}</span>
      <button class="banner-close" @click="bannerMessage = null">✕</button>
    </div>

    <!-- Time Window Filter Pill Strip / Mobile Segmented Pill Strip -->
    <div class="filter-strip glass-panel mobile-pill-strip">
      <div class="filter-pills">
        <span class="filter-label desktop-only">Multi-Window Analysis:</span>
        <button class="pill-btn" :class="{ active: selectedWindowFilter === '1h' }" @click="setWindowFilter('1h')">
          <span class="desktop-only">🔥 1h Fast Burn (14.4x)</span><span class="mobile-only">1h (14x)</span>
        </button>
        <button class="pill-btn" :class="{ active: selectedWindowFilter === '6h' }" @click="setWindowFilter('6h')">
          <span class="desktop-only">⚠️ 6h Slow Burn (6.0x)</span><span class="mobile-only">6h (6x)</span>
        </button>
        <button class="pill-btn" :class="{ active: selectedWindowFilter === '24h' }" @click="setWindowFilter('24h')">
          <span class="desktop-only">📊 24h Composite (2.0x)</span><span class="mobile-only">24h (2x)</span>
        </button>
        <button class="pill-btn" :class="{ active: selectedWindowFilter === '30d' }" @click="setWindowFilter('30d')">
          <span class="desktop-only">🗓️ 30d Baseline (1.0x)</span><span class="mobile-only">30d (1x)</span>
        </button>
      </div>

      <span class="filter-desc font-mono desktop-only">
        <span v-if="selectedWindowFilter === '1h'">Fast-burn detection: 2% budget consumed in 1h window</span>
        <span v-else-if="selectedWindowFilter === '6h'">Slow-burn detection: 5% budget consumed in 6h window</span>
        <span v-else-if="selectedWindowFilter === '24h'">Medium-window composite: 10% budget consumed in 24h</span>
        <span v-else>Rolling 30-day baseline objective compliance window</span>
      </span>

      <div class="filter-search-wrap desktop-only">
        <span class="filter-search-icon">🔍</span>
        <input
          v-model="searchQuery"
          type="search"
          class="filter-search-input font-mono"
          placeholder="Filter SLOs by service name..."
        />
        <button v-if="searchQuery" type="button" class="clear-search-btn" title="Clear search" @click="searchQuery = ''">✕</button>
      </div>
    </div>

    <!-- Metric HUD (Desktop only: 4-Column Grid, hidden on mobile <640px) -->
    <div class="metrics-grid desktop-only">
      <MetricCard
        title="Active SLOs" :value="totalSLOs" subtitle="Active services tracked against target SLIs"
        icon="🎯" badge="OBJECTIVES" badge-color="cyan"
      />
      <MetricCard
        title="Healthy Error Budgets" :value="snapshots.length > 0 ? `${healthySLOs}/${snapshots.length}` : '0/0'"
        :subtitle="snapshots.length > 0 ? 'Services with >20% remaining budget' : 'No active budget snapshots'"
        icon="shield" badge="HEALTHY" :badge-color="snapshots.length > 0 ? 'emerald' : 'muted'"
        :trend="snapshots.length > 0 ? 'Within Budget' : 'No active budgets'" :trend-type="snapshots.length > 0 ? 'positive' : 'neutral'"
      />
      <MetricCard
        title="Budget Warnings" :value="warningSLOs + criticalSLOs" subtitle="Error budget consumption > 80%"
        icon="alert" :badge="snapshots.length === 0 ? 'ZERO' : (warningSLOs + criticalSLOs) > 0 ? 'ALERT' : 'ZERO'"
        :badge-color="snapshots.length === 0 ? 'muted' : (warningSLOs + criticalSLOs) > 0 ? 'rose' : 'emerald'"
        :trend="snapshots.length === 0 ? 'No active budgets' : (warningSLOs + criticalSLOs) > 0 ? 'Elevated Failure Rate' : 'Optimal Traffic'"
        :trend-type="snapshots.length === 0 ? 'neutral' : (warningSLOs + criticalSLOs) > 0 ? 'negative' : 'positive'"
      />
      <MetricCard
        title="Average Burn Rate" :value="avgBurnRate" :subtitle="`Computed for ${selectedWindowFilter} time window`"
        icon="fire" :badge="snapshots.length === 0 ? 'NO DATA' : criticalSLOs > 0 ? 'EXHAUSTING' : warningSLOs > 0 ? 'ELEVATED' : 'NOMINAL'"
        :badge-color="snapshots.length === 0 ? 'muted' : criticalSLOs > 0 ? 'rose' : warningSLOs > 0 ? 'amber' : 'emerald'"
        :trend="snapshots.length === 0 ? 'No snapshots' : criticalSLOs > 0 ? 'Fast Burn Alert Active' : 'Normal Rate'"
        :trend-type="snapshots.length === 0 ? 'neutral' : criticalSLOs > 0 ? 'negative' : 'positive'"
      />
    </div>

    <!-- Desktop View Mode: Table OR Grid (NEVER both at the same time on desktop!) -->
    <SloCatalogTable
      v-if="viewMode === 'table'"
      class="desktop-only"
      :definitions="filteredDefinitions"
      :snapshots="filteredSnapshots"
      :loading="loading"
      :error="error"
      @create="showCreateModal = true"
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
      @create-slo="showCreateModal = true"
      @inspect="openInspect"
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
      v-model:show="showCreateModal" :real-services="realServices"
      :action-in-progress="actionInProgress" @create="handleCreateSLO"
    />
    <SloInspectModal
      v-model:show="showInspectModal" :inspect-s-l-o="selectedInspectSLO"
      :action-in-progress="actionInProgress" @trigger-alert="handleTriggerAlert"
    />
  </div>
</template>