<script setup lang="ts">
import { computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import type { HostTypeDefinition } from '../../types/hosts'
import type { ComputeHost } from '../../api/compute'

const props = defineProps<{
  searchQuery: string
  selectedTypeFilter: string
  selectedStatusFilter: string
  selectedLabelFilter: string
  viewMode: 'grid' | 'table'
  totalHosts: number
  connectedHosts?: number
  disconnectedHosts?: number
  errorHosts?: number
  hosts?: ComputeHost[]
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

const searchQuery = computed({
  get: () => props.searchQuery,
  set: (val: string) => emit('update:searchQuery', val),
})

const statusFilter = computed({
  get: () => props.selectedStatusFilter,
  set: (val: string) => emit('update:selectedStatusFilter', val),
})

const hosts = computed(() => props.hosts || { length: props.totalHosts ?? 0 })
</script>

<template>
  <div class="host-controls-toolbar glass-panel" role="toolbar" aria-label="Host Fleet Controls">
    <!-- Modern 28px Search Input -->
    <div class="search-box-wrap">
      <BaseIcon name="search" size="xs" class="search-icon" />
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Filter hosts by name, IP, OS, or tags..."
        class="search-input font-mono"
        aria-label="Filter hosts by name, IP, OS, or tags"
      />
      <button
        v-if="searchQuery"
        type="button"
        class="search-clear-btn"
        title="Clear search"
        aria-label="Clear search"
        @click="searchQuery = ''"
      >
        <BaseIcon name="x" size="xs" />
      </button>
    </div>

    <!-- 1-Click Status Segmented Pills -->
    <div class="status-pills-wrap" role="group" aria-label="Filter by host status">
      <button
        type="button"
        class="status-pill"
        :class="{ active: statusFilter === 'all' }"
        @click="statusFilter = 'all'"
      >
        All <span class="pill-count">{{ hosts.length }}</span>
      </button>
      <button
        type="button"
        class="status-pill status-pill-connected"
        :class="{ active: statusFilter === 'connected' }"
        @click="statusFilter = 'connected'"
      >
        <span class="pill-dot emerald"></span>
        Connected <span class="pill-count">{{ connectedHosts ?? 0 }}</span>
      </button>
      <button
        type="button"
        class="status-pill status-pill-disconnected"
        :class="{ active: statusFilter === 'disconnected' }"
        @click="statusFilter = 'disconnected'"
      >
        <span class="pill-dot rose"></span>
        Disconnected <span class="pill-count">{{ disconnectedHosts ?? 0 }}</span>
      </button>
      <button
        v-if="(errorHosts ?? 0) > 0"
        type="button"
        class="status-pill status-pill-error"
        :class="{ active: statusFilter === 'error' }"
        @click="statusFilter = 'error'"
      >
        <span class="pill-dot amber"></span>
        Error <span class="pill-count">{{ errorHosts ?? 0 }}</span>
      </button>
    </div>

    <!-- Refined Type Micro-Select -->
    <div class="toolbar-select-wrap">
      <select
        :value="selectedTypeFilter"
        class="toolbar-select micro-select font-mono"
        aria-label="Filter by host type"
        @change="emit('update:selectedTypeFilter', ($event.target as HTMLSelectElement).value)"
      >
        <option value="all">All Types ({{ totalHosts }})</option>
        <option v-for="def in hostTypeDefinitions" :key="def.type" :value="def.type">
          {{ def.label }} ({{ typeCounts[def.type] || 0 }})
        </option>
      </select>
    </div>

    <!-- Refined Label/Tag Micro-Select -->
    <div v-if="availableLabels && availableLabels.length > 0" class="toolbar-select-wrap">
      <select
        :value="selectedLabelFilter"
        class="toolbar-select micro-select font-mono"
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

    <!-- Primary Enterprise Cobalt Blue + Add Host Button -->
    <button
      type="button"
      class="btn btn-primary btn-sm toolbar-add-btn"
      title="Register New Host"
      @click="emit('add-host')"
    >
      <BaseIcon name="plus" size="xs" />
      <span>Add Host</span>
    </button>
  </div>
</template>
