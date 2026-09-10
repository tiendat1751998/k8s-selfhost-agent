<script setup lang="ts">
import { ref, computed } from 'vue'
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
</script>

<template>
  <div class="audit-toolbar glass-panel" :class="{ 'mobile-expanded': isMobileExpanded }">
    <!-- Command Bar / Primary Row: compact 40px on mobile, flex row on desktop -->
    <div class="toolbar-primary-row">
      <div class="toolbar-search-box">
        <span class="search-input-icon">🔍</span>
        <input
          type="text"
          class="input-glass toolbar-search-input"
          :value="searchQuery"
          placeholder="Search actor, resource, action, IP, or payload..."
          @input="$emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
      </div>

      <!-- Mobile Filter Toggle Button (<768px): [ ⚙️ Filters (${activeFilterCount}) ] -->
      <button
        class="toolbar-filter-toggle mobile-only-btn"
        :class="{ 'filter-active': activeFilterCount > 0 || isMobileExpanded }"
        type="button"
        :aria-expanded="isMobileExpanded"
        aria-label="Toggle detailed filters"
        @click="toggleMobileFilters"
      >
        <span>⚙️ Filters ({{ activeFilterCount }})</span>
        <span class="filter-toggle-arrow">{{ isMobileExpanded ? '▲' : '▼' }}</span>
      </button>

      <!-- Desktop Action Buttons (>=768px) -->
      <div class="toolbar-actions desktop-only-actions">
        <button
          class="btn btn-secondary btn-sm"
          :class="{ 'btn-primary': isLiveTailing }"
          type="button"
          @click="$emit('toggle-live-tail')"
        >
          <span>{{ isLiveTailing ? '🔴 Tail Active (Pause)' : '⚡ Live Audit Tail' }}</span>
        </button>
        <button class="btn btn-secondary btn-sm" type="button" @click="$emit('export-csv')">
          <span>📄 CSV</span>
        </button>
        <button class="btn btn-secondary btn-sm" type="button" @click="$emit('export-json')">
          <span>📦 JSON</span>
        </button>
        <button class="btn btn-secondary btn-sm" type="button" title="Reset all filters" @click="$emit('reset-filters')">
          <span>↺ Reset</span>
        </button>
      </div>
    </div>

    <!-- Expandable Detailed Filter Drawer:
         Desktop: always visible.
         Mobile (<768px): smoothly toggled when isMobileExpanded is true. -->
    <div class="toolbar-expandable-drawer" :class="{ 'drawer-open': isMobileExpanded }">
      <div class="toolbar-filter-row">
        <!-- Action Type Filter Pills -->
        <div class="filter-group">
          <span class="filter-label">Actions:</span>
          <button
            v-for="act in actionTypes"
            :key="act.key"
            class="filter-pill"
            :class="{ 'filter-active': selectedActionType === act.key }"
            type="button"
            @click="$emit('update:selectedActionType', act.key)"
          >
            {{ act.label }} ({{ act.count }})
          </button>
        </div>

        <!-- Actor Dropdown -->
        <div class="filter-group">
          <span class="filter-label">Actor:</span>
          <select
            class="input-glass filter-select"
            :value="selectedActor"
            @change="$emit('update:selectedActor', ($event.target as HTMLSelectElement).value)"
          >
            <option value="all">All Actors ({{ uniqueActors.length }})</option>
            <option v-for="actor in uniqueActors" :key="actor" :value="actor">
              {{ actor }}
            </option>
          </select>
        </div>

        <!-- Severity Dropdown -->
        <div class="filter-group">
          <span class="filter-label">Severity:</span>
          <select
            class="input-glass filter-select"
            :value="selectedSeverity"
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

        <!-- Date Range -->
        <div class="filter-group date-range-box">
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

      <!-- Mobile-only quick actions inside drawer (Export CSV, Export JSON, Live Tail, Reset) -->
      <div class="toolbar-mobile-actions mobile-only">
        <button
          class="btn btn-secondary btn-sm"
          :class="{ 'btn-primary': isLiveTailing }"
          type="button"
          @click="$emit('toggle-live-tail')"
        >
          <span>{{ isLiveTailing ? '🔴 Tail Active' : '⚡ Live Tail' }}</span>
        </button>
        <button class="btn btn-secondary btn-sm" type="button" @click="$emit('export-csv')">
          <span>📄 CSV</span>
        </button>
        <button class="btn btn-secondary btn-sm" type="button" @click="$emit('export-json')">
          <span>📦 JSON</span>
        </button>
        <button
          class="btn btn-secondary btn-sm"
          type="button"
          title="Reset all filters"
          @click="$emit('reset-filters')"
        >
          <span>↺ Reset</span>
        </button>
      </div>
    </div>
  </div>
</template>
