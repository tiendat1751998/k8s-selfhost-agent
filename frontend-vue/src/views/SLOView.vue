<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import '../assets/styles/views/slo.css'
import BaseIcon from '../components/ui/BaseIcon.vue'
import SloBurnDownBoard from '../components/slo/SloBurnDownBoard.vue'
import SloCatalogTable from '../components/slo/SloCatalogTable.vue'
import SloMobileCards from '../components/slo/SloMobileCards.vue'
import SloCreateModal from '../components/slo/SloCreateModal.vue'
import SloInspectModal from '../components/slo/SloInspectModal.vue'
import { useSLOMonitor, type TimeWindowFilter } from '../composables/useSLOMonitor'
import type { SLODefinition, SLOSnapshot } from '../types/slo'
import { sloApi, type UpdateSLOPayload } from '../api/compute'

const {
  loading, actionInProgress, error, bannerMessage, definitions, snapshots,
  selectedWindowFilter, viewMode, showCreateModal, showInspectModal, editingDefinition, selectedInspectSLO,
  realServices, totalSLOs, healthySLOs, activeBurnAlerts, enrichedSortedSLOs,
  fleetPulse, fetchSLOData, setWindowFilter, formatPercent, getEffectiveBurnRate,
  getBurnRateColor, getBudgetBarWidth, getSnapshotForDef, openCreateModal,
  openInspect, handleCreateSLO, handleTriggerAlert, handleDeleteSLO,
} = useSLOMonitor()


// Datadog/Grafana-style Time Range Dropdown State & Options
const isTimeRangeOpen = ref(false)
const timeDropdownRef = ref<HTMLElement | null>(null)

interface TimeRangeOption {
  key: TimeWindowFilter
  label: string
  icon: string
}

const timeRangeOptions: TimeRangeOption[] = [
  { key: '1h', label: 'Last 1 Hour', icon: 'flame' },
  { key: '6h', label: 'Last 6 Hours', icon: 'clock' },
  { key: '24h', label: 'Last 24 Hours', icon: 'activity' },
  { key: '7d', label: 'Last 7 Days', icon: 'calendar' },
  { key: '30d', label: 'Last 30 Days', icon: 'calendar' },
]

const activeTimeRangeLabel = computed(() => timeRangeOptions.find(o => o.key === selectedWindowFilter.value)?.label || 'Last 30 Days')
function selectTimeRange(key: TimeWindowFilter) { setWindowFilter(key); isTimeRangeOpen.value = false }
function handleTimeDropdownClickOutside(e: MouseEvent) {
  if (timeDropdownRef.value && !timeDropdownRef.value.contains(e.target as Node)) isTimeRangeOpen.value = false
}
onMounted(() => document.addEventListener('click', handleTimeDropdownClickOutside))
onUnmounted(() => document.removeEventListener('click', handleTimeDropdownClickOutside))

const mobileWindowPills = [
  { key: '1h' as const, short: '1h' }, { key: '6h' as const, short: '6h' },
  { key: '24h' as const, short: '24h' }, { key: '7d' as const, short: '7d' }, { key: '30d' as const, short: '30d' },
]

const searchQuery = ref('')
const showMobileSearch = ref(false)

const filteredSortedSLOs = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return enrichedSortedSLOs.value
  return enrichedSortedSLOs.value.filter(s => s.service.toLowerCase().includes(q) || (s.indicator_type && s.indicator_type.toLowerCase().includes(q)))
})

const filteredDefinitions = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return definitions.value
  return definitions.value.filter(d => d.service.toLowerCase().includes(q) || (d.indicator_type && d.indicator_type.toLowerCase().includes(q)))
})

const filteredSnapshots = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return snapshots.value
  const matchingDefIds = new Set(definitions.value.filter(d => d.service.toLowerCase().includes(q) || (d.indicator_type && d.indicator_type.toLowerCase().includes(q))).map(d => d.id))
  return snapshots.value.filter(s => s.service.toLowerCase().includes(q) || matchingDefIds.has(s.slo_id))
})

const openInspectFromTable = (row: SLODefinition) => openInspect(row, snapshots.value.find(s => s.slo_id === row.id || s.service === row.service))
const handleMobileInspect = (def: SLODefinition, snap?: SLOSnapshot) => openInspect(def, snap)
const handleEditSLO = (def: SLODefinition) => openCreateModal(def)

async function handleSaveEditedSLO(data: { id: string; payload: UpdateSLOPayload }) {
  actionInProgress.value = true
  bannerMessage.value = null
  try {
    await sloApi.updateDefinition(data.id, data.payload)
    showCreateModal.value = false
    bannerMessage.value = { type: 'success', text: `SLO target objective successfully updated for service "${data.payload.service}".` }
    await fetchSLOData()
  } catch (err: unknown) {
    bannerMessage.value = { type: 'error', text: err instanceof Error ? err.message : 'Failed to update SLO definition' }
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

    <!-- Sleek Unified 48px Enterprise Toolbar -->
    <div class="slo-toolbar-sleek glass-panel desktop-only">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap slo-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter SLOs..."
          class="toolbar-search-input slo-search-input"
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

      <!-- Datadog/Grafana-style Time Range Dropdown -->
      <div ref="timeDropdownRef" class="time-range-dropdown-wrap">
        <button
          type="button"
          class="time-range-btn font-mono"
          :class="{ 'is-active': isTimeRangeOpen }"
          aria-haspopup="true"
          :aria-expanded="isTimeRangeOpen"
          aria-label="Select Time Range"
          @click.stop="isTimeRangeOpen = !isTimeRangeOpen"
        >
          <BaseIcon name="clock" size="xs" class="time-btn-icon" />
          <span>{{ activeTimeRangeLabel }}</span>
          <BaseIcon name="chevron-down" size="xs" class="time-btn-chevron" />
        </button>

        <div v-if="isTimeRangeOpen" class="time-range-menu glass-panel animate-fade-in" role="menu">
          <button
            v-for="opt in timeRangeOptions"
            :key="opt.key"
            type="button"
            role="menuitem"
            class="time-range-item font-mono"
            :class="{ active: selectedWindowFilter === opt.key }"
            @click="selectTimeRange(opt.key)"
          >
            <BaseIcon :name="opt.icon" size="xs" class="item-icon" />
            <span class="item-label">{{ opt.label }}</span>
            <BaseIcon v-if="selectedWindowFilter === opt.key" name="check" size="xs" class="check-icon" />
          </button>
        </div>
      </div>

      <!-- Board | Table View Switcher -->
      <div class="view-mode-toggle font-mono" role="radiogroup" aria-label="View Mode">
        <button
          type="button"
          class="view-mode-btn"
          :class="{ active: viewMode === 'board' }"
          role="radio"
          :aria-checked="viewMode === 'board'"
          aria-label="Board view"
          @click="viewMode = 'board'"
        >
          <BaseIcon name="layout" size="xs" />
          <span>Board</span>
        </button>
        <button
          type="button"
          class="view-mode-btn"
          :class="{ active: viewMode === 'table' }"
          role="radio"
          :aria-checked="viewMode === 'table'"
          aria-label="Table view"
          @click="viewMode = 'table'"
        >
          <BaseIcon name="list" size="xs" />
          <span>Table</span>
        </button>
      </div>

      <!-- Inline Micro-Telemetry Pill -->
      <div class="slo-micro-telemetry font-mono" role="status" aria-label="SLO micro-telemetry">
        <span class="badge-dot-emerald"></span>
        <span>{{ totalSLOs }} Targets · {{ totalSLOs > 0 ? Math.round((healthySLOs / totalSLOs) * 100) : 100 }}% · {{ activeBurnAlerts }} Alerts</span>
      </div>

      <!-- Right: Action buttons -->
      <div class="toolbar-actions-group slo-toolbar-actions">
        <!-- Action buttons: Add Target (primary) and Refresh (secondary with spinner) -->
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
        <span class="command-bar-title font-bold"><BaseIcon name="target" size="xs" /> SLOs</span>
        <span class="slo-count-pill font-mono">{{ totalSLOs }} Objectives</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :class="{ active: viewMode === 'board' }"
          :title="viewMode === 'board' ? 'Switch to Table' : 'Switch to Board'"
          :aria-label="viewMode === 'board' ? 'Switch to Table' : 'Switch to Board'"
          @click="viewMode = viewMode === 'board' ? 'table' : 'board'"
        >
          <BaseIcon :name="viewMode === 'board' ? 'list' : 'layout'" size="xs" />
        </button>
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
          v-for="w in mobileWindowPills"
          :key="w.key"
          type="button"
          class="mobile-window-btn"
          :class="{ active: selectedWindowFilter === w.key }"
          @click="setWindowFilter(w.key)"
        >
          {{ w.key }}
        </button>
      </div>
    </div>

    <!-- Primary View: Error Budget Burn-Down & Velocity Board (Default) -->
    <SloBurnDownBoard
      v-if="viewMode === 'board'"
      :items="filteredSortedSLOs"
      :pulse="fleetPulse"
      :loading="loading"
      :error="error"
      @inspect="openInspect"
      @trigger-alert="handleTriggerAlert"
      @edit="openCreateModal"
      @delete="handleDeleteSLO"
      @create="openCreateModal()"
    />

    <!-- Secondary View: Standardized High-Density Catalog Table -->
    <template v-else>
      <div class="slo-table-card desktop-only">
        <SloCatalogTable
          :definitions="filteredDefinitions"
          :snapshots="filteredSnapshots"
          :loading="loading"
          :error="error"
          @inspect="openInspectFromTable"
          @edit-slo="handleEditSLO"
          @trigger-alert="handleTriggerAlert"
          @delete-slo="handleDeleteSLO"
        />
      </div>

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
    </template>

    <!-- Modals -->
    <SloCreateModal
      v-model:show="showCreateModal"
      :real-services="realServices"
      :action-in-progress="actionInProgress"
      :editing-definition="editingDefinition"
      @save="handleSaveEditedSLO"
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
