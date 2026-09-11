<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import type { HostTypeDefinition } from '../../types/hosts'

defineProps<{
  searchQuery: string
  selectedTypeFilter: string
  selectedStatusFilter: string
  selectedLabelFilter: string
  viewMode: 'grid' | 'table'
  totalHosts: number
  typeCounts: Record<string, number>
  availableLabels: string[]
  hostTypeDefinitions: HostTypeDefinition[]
  loading?: boolean
  showTelemetry?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:searchQuery', val: string): void
  (e: 'update:selectedTypeFilter', val: string): void
  (e: 'update:selectedStatusFilter', val: string): void
  (e: 'update:selectedLabelFilter', val: string): void
  (e: 'update:viewMode', val: 'grid' | 'table'): void
  (e: 'update:showTelemetry', val: boolean): void
  (e: 'refresh'): void
  (e: 'add-host'): void
}>()
</script>

<template>
  <div class="host-controls-toolbar glass-panel" role="toolbar" aria-label="Host Fleet Controls">
    <!-- Search Input (filter by hostname, IP, tag) -->
    <div class="toolbar-search-wrap">
      <span class="search-icon"><BaseIcon name="search" size="xs" /></span>
      <input
        :value="searchQuery"
        type="text"
        placeholder="Filter by hostname, IP, tag..."
        class="toolbar-search-input input-glass font-mono"
        aria-label="Search hosts"
        @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
      />
      <button
        v-if="searchQuery"
        type="button"
        class="toolbar-clear-btn"
        title="Clear search"
        aria-label="Clear search query"
        @click="emit('update:searchQuery', '')"
      >
        <BaseIcon name="x" size="xs" />
      </button>
    </div>

    <!-- Type Filter Dropdown -->
    <div class="toolbar-select-wrap">
      <label class="toolbar-lbl font-mono">TYPE:</label>
      <select
        :value="selectedTypeFilter"
        class="toolbar-select input-glass font-mono"
        aria-label="Filter by host type"
        @change="emit('update:selectedTypeFilter', ($event.target as HTMLSelectElement).value)"
      >
        <option value="all">All Types ({{ totalHosts }})</option>
        <option v-for="def in hostTypeDefinitions" :key="def.type" :value="def.type">
          {{ def.label }} ({{ typeCounts[def.type] || 0 }})
        </option>
      </select>
    </div>

    <!-- Status Filter Dropdown -->
    <div class="toolbar-select-wrap">
      <label class="toolbar-lbl font-mono">STATUS:</label>
      <select
        :value="selectedStatusFilter"
        class="toolbar-select input-glass font-mono"
        aria-label="Filter by host status"
        @change="emit('update:selectedStatusFilter', ($event.target as HTMLSelectElement).value)"
      >
        <option value="all">All Statuses</option>
        <option value="connected">Connected</option>
        <option value="disconnected">Disconnected</option>
        <option value="error">Error / Alert</option>
      </select>
    </div>

    <!-- Label/Tag Filter Dropdown -->
    <div v-if="availableLabels && availableLabels.length > 0" class="toolbar-select-wrap">
      <label class="toolbar-lbl font-mono">TAG:</label>
      <select
        :value="selectedLabelFilter"
        class="toolbar-select input-glass font-mono"
        aria-label="Filter by host tag"
        @change="emit('update:selectedLabelFilter', ($event.target as HTMLSelectElement).value)"
      >
        <option value="all">All Tags</option>
        <option v-for="lbl in availableLabels" :key="lbl" :value="lbl">{{ lbl }}</option>
      </select>
    </div>

    <!-- View Mode Switcher (Table / Grid) -->
    <div class="toolbar-view-toggle" role="group" aria-label="View mode">
      <button
        type="button"
        class="mode-btn"
        :class="{ 'mode-btn-active': viewMode === 'table' }"
        title="Table View"
        aria-label="Switch to Table View"
        @click="emit('update:viewMode', 'table')"
      >
        <BaseIcon name="table" size="xs" />
        <span>Table</span>
      </button>
      <button
        type="button"
        class="mode-btn"
        :class="{ 'mode-btn-active': viewMode === 'grid' }"
        title="Grid View"
        aria-label="Switch to Grid View"
        @click="emit('update:viewMode', 'grid')"
      >
        <BaseIcon name="grid" size="xs" />
        <span>Grid</span>
      </button>
    </div>

    <!-- Telemetry Toggle Button -->
    <button
      v-if="showTelemetry !== undefined"
      type="button"
      class="toolbar-btn toolbar-btn-telemetry font-mono"
      :class="{ 'btn-active': showTelemetry }"
      :title="showTelemetry ? 'Hide Telemetry' : 'Show Telemetry & Latency'"
      @click="emit('update:showTelemetry', !showTelemetry)"
    >
      <BaseIcon :name="showTelemetry ? 'chevron-up' : 'activity'" size="xs" />
      <span>Telemetry</span>
    </button>

    <!-- Refresh Button -->
    <button
      type="button"
      class="toolbar-btn toolbar-btn-refresh"
      :disabled="loading"
      title="Refresh hosts"
      @click="emit('refresh')"
    >
      <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
      <span>{{ loading ? 'Refreshing...' : 'Refresh' }}</span>
    </button>

    <!-- + Add Host Primary Button -->
    <button
      type="button"
      class="toolbar-btn toolbar-btn-primary"
      title="Register New Host"
      @click="emit('add-host')"
    >
      <BaseIcon name="plus" size="xs" />
      <span>Add Host</span>
    </button>
  </div>
</template>
