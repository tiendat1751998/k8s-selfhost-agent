<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  searchQuery: string
  selectedType: string
  categoryCounts: Record<string, number>
  loading?: boolean
}>()

const emit = defineEmits<{
  'update:searchQuery': [value: string]
  'update:selectedType': [category: string]
  schedule: []
  refresh: []
  generate: []
}>()

interface CategoryOption {
  key: string
  label: string
}

const categories: CategoryOption[] = [
  { key: 'all', label: 'All' },
  { key: 'compliance', label: 'Compliance' },
  { key: 'security', label: 'Security' },
  { key: 'cost', label: 'Cost' },
  { key: 'operational', label: 'Operational' },
  { key: 'incident', label: 'Incident' }
]

function onSearchInput(event: Event) {
  const target = event.target as HTMLInputElement
  emit('update:searchQuery', target.value)
}

function clearSearch() {
  emit('update:searchQuery', '')
}
</script>

<template>
  <div class="reports-toolbar-sleek glass-panel" role="toolbar" aria-label="Reports Management Toolbar">
    <!-- Left: 28px search input with search icon and clear button -->
    <div class="toolbar-left">
      <div class="search-input-wrapper">
        <BaseIcon name="search" size="xs" class="search-icon text-muted" />
        <input
          :value="searchQuery"
          type="text"
          placeholder="Search reports..."
          class="toolbar-search-input font-mono"
          aria-label="Search reports by title, ID, or author"
          @input="onSearchInput"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="search-clear-btn"
          aria-label="Clear search query"
          title="Clear search"
          @click="clearSearch"
        >
          &times;
        </button>
      </div>
    </div>

    <!-- Center: Category filter pills with dynamic count badges -->
    <div class="toolbar-center">
      <div class="category-pills">
        <button
          v-for="cat in categories"
          :key="cat.key"
          type="button"
          class="category-pill"
          :class="{ active: selectedType === cat.key }"
          @click="emit('update:selectedType', cat.key)"
        >
          <span class="pill-label">{{ cat.label }}</span>
          <span class="pill-badge font-mono">({{ categoryCounts[cat.key] ?? 0 }})</span>
        </button>
      </div>
    </div>

    <!-- Right: Schedule Cadence button, Refresh icon button, + Generate Report primary button -->
    <div class="toolbar-right">
      <button
        type="button"
        class="btn-cadence"
        title="Schedule Automated Cadence"
        @click="emit('schedule')"
      >
        <BaseIcon name="clock" size="xs" />
        <span class="btn-cadence-text">Schedule Cadence</span>
      </button>

      <button
        type="button"
        class="btn-refresh"
        :disabled="loading"
        title="Refresh Reports Telemetry"
        aria-label="Refresh reports telemetry"
        @click="emit('refresh')"
      >
        <BaseIcon name="refresh" size="xs" :class="{ 'spin-active': loading }" />
      </button>

      <button
        type="button"
        class="btn-generate-primary"
        title="Compile and Generate New Report"
        @click="emit('generate')"
      >
        <BaseIcon name="plus" size="xs" />
        <span>+ Generate Report</span>
      </button>
    </div>
  </div>
</template>
