<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import '../assets/styles/views/slo.css'
import BaseIcon from '../components/ui/BaseIcon.vue'
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
const windowPills = [
  { key: '1h' as const, label: '1h (Fast)', title: '1h Fast Burn (14.4x rate)', icon: 'flame', short: 'Fast' },
  { key: '6h' as const, label: '6h (Slow)', title: '6h Slow Burn (6.0x rate)', icon: 'alert-triangle', short: 'Slow' },
  { key: '24h' as const, label: '24h (Composite)', title: '24h Composite (2.0x rate)', icon: 'activity', short: 'Comp' },
  { key: '30d' as const, label: '30d (Baseline)', title: '30d Baseline (1.0x rate)', icon: 'calendar', short: 'Base' }
]

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

// Filtered data based on unified search input (filters SLOs by service name or indicator)
const filteredDefinitions = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return definitions.value
  return definitions.value.filter(d =>
    d.service.toLowerCase().includes(q) ||
    (d.indicator_type && d.indicator_type.toLowerCase().includes(q))
  )
})

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
    showBanner('warning', `Fast burn-rate alert simulated for '${serviceName}'. Check alerts view.`)
  } catch (err: unknown) {
    showBanner('error', err instanceof Error ? err.message : 'Failed to simulate alert')
  } finally {
    actionInProgress.value = false
  }
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
          @click="showCreateModal = true"
        >
          <BaseIcon name="plus" size="xs" />
          <span>+ Add Target</span>
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
        <button class="btn-icon-cmd" title="Create SLO definition" aria-label="Create SLO definition" @click="showCreateModal = true">
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
