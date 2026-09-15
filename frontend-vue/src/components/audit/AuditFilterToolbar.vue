<script setup lang="ts">
import { ref, computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import type { AuditActionType, AuditSeverity } from '../../api/governance'

const props = defineProps<{
  searchQuery: string
  selectedActionType: 'all' | AuditActionType
  selectedSeverity: 'ALL' | AuditSeverity
  selectedActor: string
  dateRange: { start: string; end: string }
  uniqueActors: string[]
  actionTypes: { key: 'all' | AuditActionType; label: string; count: number }[]
  isLiveTailing: boolean
  metrics?: {
    totalEvents: number
    securityMutations: number
    administrativeActions: number
    policyDenials: number
    signedPercentage?: number
    securityViolations?: number
  }
}>()

const emit = defineEmits<{
  (e: 'update:searchQuery', val: string): void
  (e: 'update:selectedActionType', val: 'all' | AuditActionType): void
  (e: 'update:selectedSeverity', val: 'ALL' | AuditSeverity): void
  (e: 'update:selectedActor', val: string): void
  (e: 'update:dateRange', val: { start: string; end: string }): void
  (e: 'toggle-live-tail'): void
  (e: 'export-json'): void
  (e: 'export-csv'): void
  (e: 'reset-filters'): void
}>()

const isMobileExpanded = ref(false)

const activeFilterCount = computed(() => {
  let count = 0
  if (props.selectedActionType !== 'all') count++
  if (props.selectedSeverity !== 'ALL') count++
  if (props.selectedActor !== 'all') count++
  if (props.dateRange.start || props.dateRange.end) count++
  return count
})

const totalEvents = computed(() => props.metrics?.totalEvents ?? props.actionTypes.find(a => a.key === 'all')?.count ?? 0)
const securityMutations = computed(() => props.metrics?.securityMutations ?? props.actionTypes.find(a => a.key === 'mutation')?.count ?? 0)
const accessCount = computed(() => props.actionTypes.find(a => a.key === 'access')?.count ?? 0)
const adminCount = computed(() => props.metrics?.administrativeActions ?? props.actionTypes.find(a => a.key === 'rbac_grant')?.count ?? 0)
const deletionCount = computed(() => props.actionTypes.find(a => a.key === 'deletion')?.count ?? 0)

const sleekActionPills = computed(() => [
  { key: 'all' as const, label: 'All', count: totalEvents.value },
  { key: 'mutation' as const, label: 'Mutations', count: securityMutations.value },
  { key: 'access' as const, label: 'Access', count: accessCount.value },
  { key: 'rbac_grant' as const, label: 'RBAC', count: adminCount.value },
  { key: 'deletion' as const, label: 'Deletions', count: deletionCount.value },
])

function toggleMobileFilters() {
  isMobileExpanded.value = !isMobileExpanded.value
}

function onStartDateChange(e: Event, currentEnd: string) {
  const target = e.target as HTMLInputElement
  emit('update:dateRange', { start: target.value, end: currentEnd })
}

function onEndDateChange(e: Event, currentStart: string) {
  const target = e.target as HTMLInputElement
  emit('update:dateRange', { start: currentStart, end: target.value })
}

function formatActorLabel(actor: string): string {
  if (!actor || actor === 'all') return actor
  if (actor.includes('@')) {
    const [user] = actor.split('@')
    return user.length > 16 ? user.slice(0, 14) + '...' : user
  }
  if (actor.length > 16) {
    return actor.slice(0, 14) + '...'
  }
  return actor
}
</script>

<template>
  <!-- Desktop 42px Single-Row Sleek Toolbar (>=768px) -->
  <div class="audit-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="Audit Trail Toolbar">
    <!-- Left: 30px Capsule Pill search input with prefix search icon and clear button x -->
    <div class="sleek-search-wrap">
      <BaseIcon name="search" size="xs" class="sleek-search-icon" />
      <input
        type="text"
        class="sleek-search-input font-mono"
        :value="searchQuery"
        placeholder="Search trail..."
        aria-label="Search audit trail"
        title="Search audit trail"
        @input="$emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
      />
      <button
        v-if="searchQuery"
        type="button"
        class="sleek-clear-btn"
        aria-label="Clear search"
        title="Clear search"
        @click="$emit('update:searchQuery', '')"
      >
        &times;
      </button>
    </div>

    <!-- Center: 1-Click Action Capsule Pills with dynamic counts -->
    <div class="sleek-action-pills font-mono" role="tablist" aria-label="Audit Action Filters">
      <button
        v-for="pill in sleekActionPills"
        :key="pill.key"
        type="button"
        role="tab"
        :aria-selected="selectedActionType === pill.key"
        class="sleek-pill-btn sleek-pill"
        :class="{ active: selectedActionType === pill.key }"
        :title="'Filter by ' + pill.label + ' (' + pill.count + ')'"
        @click="$emit('update:selectedActionType', pill.key)"
      >
        <span>{{ pill.label }}</span>
        <span class="sleek-pill-count">({{ pill.count }})</span>
      </button>
    </div>

    <!-- Center-Right: Compact 28px select dropdowns for Actor and Severity -->
    <div class="sleek-select-group font-mono">
      <select
        class="sleek-select sleek-select-actor"
        :value="selectedActor"
        aria-label="Filter by Actor"
        title="Filter by Actor"
        @change="$emit('update:selectedActor', ($event.target as HTMLSelectElement).value)"
      >
        <option value="all">All Actors ({{ uniqueActors.length }})</option>
        <option v-for="actor in uniqueActors" :key="actor" :value="actor" :title="actor">
          {{ formatActorLabel(actor) }}
        </option>
      </select>

      <select
        class="sleek-select sleek-select-severity"
        :value="selectedSeverity"
        aria-label="Filter by Severity"
        title="Filter by Severity"
        @change="$emit('update:selectedSeverity', ($event.target as HTMLSelectElement).value as 'ALL' | AuditSeverity)"
      >
        <option value="ALL">All Severities</option>
        <option value="critical">Critical</option>
        <option value="high">High</option>
        <option value="medium">Medium</option>
        <option value="low">Low</option>
        <option value="info">Info</option>
      </select>
    </div>

    <!-- Right: Action buttons (Live streaming toggle, CSV, JSON, and Reset icon button) -->
    <div class="sleek-actions-group">
      <button
        class="sleek-btn sleek-live-btn"
        :class="{ 'live-active': isLiveTailing }"
        type="button"
        :title="isLiveTailing ? 'Live Tail Active (Click to Pause)' : 'Start Live Stream'"
        :aria-label="isLiveTailing ? 'Live Tail Active (Click to Pause)' : 'Start Live Stream'"
        @click="$emit('toggle-live-tail')"
      >
        <span class="sleek-live-dot" :class="{ active: isLiveTailing }"></span>
        <span>Live</span>
      </button>

      <button
        class="sleek-btn sleek-btn-csv"
        type="button"
        title="Export audit events as CSV"
        aria-label="Export audit events as CSV"
        @click="$emit('export-csv')"
      >
        <BaseIcon name="file-text" size="xs" />
        <span class="sleek-btn-text">CSV</span>
      </button>

      <button
        class="sleek-btn sleek-btn-json"
        type="button"
        title="Export audit events as JSON"
        aria-label="Export audit events as JSON"
        @click="$emit('export-json')"
      >
        <BaseIcon name="box" size="xs" />
        <span class="sleek-btn-text">JSON</span>
      </button>

      <button
        class="sleek-btn sleek-btn-icon sleek-btn-reset"
        type="button"
        title="Reset all filters"
        aria-label="Reset all filters"
        @click="$emit('reset-filters')"
      >
        <BaseIcon name="refresh" size="xs" />
      </button>
    </div>
  </div>

  <!-- Mobile Collapsible Toolbar (<768px) -->
  <div class="audit-toolbar glass-panel mobile-only" :class="{ 'mobile-expanded': isMobileExpanded }">
    <div class="toolbar-primary-row">
      <div class="toolbar-search-box">
        <BaseIcon name="search" size="xs" class="search-input-icon" />
        <input
          type="text"
          class="input-glass toolbar-search-input font-mono"
          :value="searchQuery"
          placeholder="Search actor, resource, IP..."
          aria-label="Search audit trail"
          @input="$emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
      </div>

      <button
        class="toolbar-filter-toggle mobile-only-btn"
        :class="{ 'filter-active': activeFilterCount > 0 || isMobileExpanded }"
        type="button"
        :aria-expanded="isMobileExpanded"
        aria-label="Toggle detailed filters"
        @click="toggleMobileFilters"
      >
        <BaseIcon name="sliders" size="xs" /> <span>Filters ({{ activeFilterCount }})</span>
        <BaseIcon :name="isMobileExpanded ? 'chevron-up' : 'chevron-down'" size="xs" class="filter-toggle-arrow" />
      </button>
    </div>

    <div class="toolbar-expandable-drawer" :class="{ 'drawer-open': isMobileExpanded }">
      <div class="toolbar-filter-row">
        <div class="filter-group">
          <span class="filter-label">Actions:</span>
          <button
            v-for="act in actionTypes"
            :key="act.key"
            class="filter-pill font-mono"
            :class="{ 'filter-active': selectedActionType === act.key }"
            type="button"
            @click="$emit('update:selectedActionType', act.key)"
          >
            {{ act.label }} ({{ act.count }})
          </button>
        </div>

        <div class="filter-group">
          <span class="filter-label">Actor:</span>
          <select
            class="input-glass filter-select font-mono"
            :value="selectedActor"
            aria-label="Filter by actor"
            title="Filter by actor"
            @change="$emit('update:selectedActor', ($event.target as HTMLSelectElement).value)"
          >
            <option value="all">All Actors ({{ uniqueActors.length }})</option>
            <option v-for="actor in uniqueActors" :key="actor" :value="actor" :title="actor">
              {{ formatActorLabel(actor) }}
            </option>
          </select>
        </div>

        <div class="filter-group">
          <span class="filter-label">Severity:</span>
          <select
            class="input-glass filter-select font-mono"
            :value="selectedSeverity"
            aria-label="Filter by severity"
            title="Filter by severity"
            @change="$emit('update:selectedSeverity', ($event.target as HTMLSelectElement).value as 'ALL' | AuditSeverity)"
          >
            <option value="ALL">All Severities</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
            <option value="info">Info</option>
          </select>
        </div>

        <div class="filter-group date-range-box font-mono">
          <span class="filter-label">Date:</span>
          <input
            type="date"
            class="input-glass date-input"
            :value="dateRange.start"
            aria-label="Filter Start Date"
            @change="onStartDateChange($event, dateRange.end)"
          />
          <span class="date-sep">to</span>
          <input
            type="date"
            class="input-glass date-input"
            :value="dateRange.end"
            aria-label="Filter End Date"
            @change="onEndDateChange($event, dateRange.start)"
          />
        </div>
      </div>

      <div class="toolbar-mobile-actions mobile-only">
        <button
          class="btn btn-secondary btn-sm"
          :class="{ 'btn-primary': isLiveTailing }"
          type="button"
          :title="isLiveTailing ? 'Live Tail Active (Click to Pause)' : 'Start Live Stream'"
          :aria-label="isLiveTailing ? 'Live Tail Active (Click to Pause)' : 'Start Live Stream'"
          @click="$emit('toggle-live-tail')"
        >
          <BaseIcon :name="isLiveTailing ? 'pause' : 'zap'" size="xs" /> <span>{{ isLiveTailing ? 'Tail Active' : 'Live Tail' }}</span>
        </button>
        <button
          class="btn btn-secondary btn-sm"
          type="button"
          title="Export audit events as CSV"
          aria-label="Export audit events as CSV"
          @click="$emit('export-csv')"
        >
          <BaseIcon name="file-text" size="xs" /> <span>CSV</span>
        </button>
        <button
          class="btn btn-secondary btn-sm"
          type="button"
          title="Export audit events as JSON"
          aria-label="Export audit events as JSON"
          @click="$emit('export-json')"
        >
          <BaseIcon name="box" size="xs" /> <span>JSON</span>
        </button>
        <button
          class="btn btn-secondary btn-sm"
          type="button"
          title="Reset all filters"
          aria-label="Reset all filters"
          @click="$emit('reset-filters')"
        >
          <BaseIcon name="refresh" size="xs" /> <span>Reset</span>
        </button>
      </div>
    </div>
  </div>
</template>
