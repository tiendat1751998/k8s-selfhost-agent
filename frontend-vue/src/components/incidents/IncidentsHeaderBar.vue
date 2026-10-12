<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import IncidentMicroTelemetry from './IncidentMicroTelemetry.vue'

defineProps<{
  searchQuery: string
  filterStatus: string
  filterSeverity: string
  countAll: number
  countOpen: number
  countInProgress: number
  countResolved: number
  totalIncidents: number
  criticalCount: number
  analyzingCount: number
  resolvedCount: number
  filteredCount: number
  loading: boolean
  showMobileSearch: boolean
}>()

const emit = defineEmits<{
  (e: 'update:searchQuery', val: string): void
  (e: 'update:filterStatus', val: string): void
  (e: 'update:filterSeverity', val: string): void
  (e: 'update:showMobileSearch', val: boolean): void
  (e: 'declareIncident'): void
  (e: 'simulateIncident'): void
  (e: 'refresh'): void
}>()
</script>

<template>
  <div class="incidents-header-bar-wrapper">
    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="incidents-toolbar-sleek glass-panel desktop-only">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          :value="searchQuery"
          type="text"
          placeholder="Filter pod, cluster, ns, error..."
          class="toolbar-search-input"
          aria-label="Filter incidents by pod, cluster, namespace, or error"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="clear-input-btn"
          aria-label="Clear search"
          @click="emit('update:searchQuery', '')"
        >
          <BaseIcon name="x" size="xs" />
        </button>
      </div>

      <!-- Status tabs / pills with count badges -->
      <div class="toolbar-status-pills" role="tablist" aria-label="Filter incidents by status">
        <button
          v-for="tab in [
            { key: 'all', label: 'All', count: countAll },
            { key: 'open', label: 'Open', count: countOpen },
            { key: 'in_progress', label: 'In Progress', count: countInProgress },
            { key: 'resolved', label: 'Resolved', count: countResolved }
          ]"
          :key="tab.key"
          type="button"
          role="tab"
          :aria-selected="filterStatus === tab.key"
          class="toolbar-pill-btn"
          :class="{ active: filterStatus === tab.key }"
          @click="emit('update:filterStatus', tab.key)"
        >
          {{ tab.label }} ({{ tab.count }})
        </button>
      </div>

      <!-- Severity filter dropdown -->
      <select
        :value="filterSeverity"
        class="toolbar-select severity-dropdown"
        aria-label="Filter incidents by severity"
        @change="emit('update:filterSeverity', ($event.target as HTMLSelectElement).value)"
      >
        <option value="all">All Severities</option>
        <option value="critical">Critical</option>
        <option value="high">High</option>
        <option value="medium">Medium</option>
        <option value="low">Low</option>
      </select>

      <!-- Inline compact KPI badge strip font-mono -->
      <div class="toolbar-kpi-strip font-mono" role="status" aria-label="Incident metrics summary">
        <span class="kpi-live-dot"></span>
        <span class="kpi-live-text">{{ totalIncidents }} Incidents ({{ criticalCount }} Critical · {{ resolvedCount }} Resolved)</span>
      </div>

      <!-- Action buttons -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="toolbar-btn btn-primary"
          aria-label="Declare Operational Incident"
          @click="emit('declareIncident')"
        >
          <BaseIcon name="plus" size="xs" />
          <span>Declare Incident</span>
        </button>
        <button
          type="button"
          class="toolbar-btn btn-secondary"
          :disabled="loading"
          @click="emit('refresh')"
        >
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" :class="{ 'spin-animate': loading }" />
          <span>{{ loading ? 'Querying...' : 'Refresh Feed' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile Sleek 40px Command Bar -->
    <div class="mobile-command-bar mobile-only">
      <div class="mobile-command-title">
        <span class="mobile-title-icon"><BaseIcon name="alert-triangle" size="xs" /></span>
        <span class="mobile-title-text">Incidents ({{ filteredCount }})</span>
      </div>
      <div class="mobile-command-actions">
        <button
          class="mobile-action-btn"
          title="Declare Incident"
          aria-label="Declare Incident"
          @click="emit('declareIncident')"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          class="mobile-action-btn"
          title="Simulate Test Incident (Demo Mode)"
          aria-label="Simulate Test Incident"
          @click="emit('simulateIncident')"
        >
          <BaseIcon name="zap" size="xs" />
        </button>
        <button
          class="mobile-action-btn"
          :disabled="loading"
          title="Refresh Feed"
          aria-label="Refresh Feed"
          @click="emit('refresh')"
        >
          <BaseIcon name="refresh" size="xs" />
        </button>
        <button
          class="mobile-action-btn"
          :class="{ active: showMobileSearch }"
          title="Search and Filter"
          aria-label="Toggle Search"
          @click="emit('update:showMobileSearch', !showMobileSearch)"
        >
          <BaseIcon name="search" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile Micro-Telemetry Strip (20px) -->
    <IncidentMicroTelemetry
      class="mobile-only"
      :total-incidents="totalIncidents"
      :critical-count="criticalCount"
      :analyzing-count="analyzingCount"
      :resolved-count="resolvedCount"
    />

    <!-- Mobile Collapsible Search & Filter Bar -->
    <div v-if="showMobileSearch" class="mobile-filter-bar mobile-only animate-fade-in">
      <div class="status-tab-group mobile-status-tabs">
        <button
          v-for="tab in [
            { key: 'all', label: 'All', count: countAll },
            { key: 'open', label: 'Open', count: countOpen },
            { key: 'in_progress', label: 'In Progress', count: countInProgress },
            { key: 'resolved', label: 'Resolved', count: countResolved }
          ]"
          :key="tab.key"
          class="status-tab-btn"
          :class="{ active: filterStatus === tab.key }"
          @click="emit('update:filterStatus', tab.key)"
        >
          {{ tab.label }} ({{ tab.count }})
        </button>
      </div>
      <div class="mobile-filter-inputs">
        <input
          :value="searchQuery"
          type="text"
          placeholder="Filter pod, cluster, ns..."
          class="search-mini"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
        <select
          :value="filterSeverity"
          class="select-mini"
          @change="emit('update:filterSeverity', ($event.target as HTMLSelectElement).value)"
        >
          <option value="all">All Severities</option>
          <option value="critical">Critical</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </select>
      </div>
    </div>
  </div>
</template>
