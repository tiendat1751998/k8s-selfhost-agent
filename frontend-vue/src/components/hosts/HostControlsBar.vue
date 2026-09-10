<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import type { HostTypeDefinition } from '../../types/hosts'

defineProps<{
  searchQuery: string; selectedTypeFilter: string; selectedStatusFilter: string
  selectedLabelFilter: string; viewMode: 'grid' | 'table'; totalHosts: number
  typeCounts: Record<string, number>; availableLabels: string[]; hostTypeDefinitions: HostTypeDefinition[]
}>()

const emit = defineEmits<{
  (e: 'update:searchQuery', val: string): void
  (e: 'update:selectedTypeFilter', val: string): void
  (e: 'update:selectedStatusFilter', val: string): void
  (e: 'update:selectedLabelFilter', val: string): void
  (e: 'update:viewMode', val: 'grid' | 'table'): void
}>()
</script>

<template>
  <div class="host-controls-wrapper">
    <!-- Type Breakdown Pill Bar -->
    <div class="type-breakdown-bar glass-panel">
      <span class="breakdown-label font-mono">REGISTRY COMPOSITION:</span>
      <div class="breakdown-pills">
        <span class="type-pill font-mono" :class="{ 'pill-active': selectedTypeFilter === 'all' }" @click="emit('update:selectedTypeFilter', 'all')">
          <BaseIcon name="globe" size="xs" /><span>All</span><span class="pill-count">{{ totalHosts }}</span>
        </span>
        <span v-for="def in hostTypeDefinitions" :key="def.type" class="type-pill font-mono" :class="{ 'pill-active': selectedTypeFilter === def.type }" @click="emit('update:selectedTypeFilter', selectedTypeFilter === def.type ? 'all' : def.type)">
          <BaseIcon :name="def.icon" size="xs" /><span>{{ def.label }}</span><span class="pill-count">{{ typeCounts[def.type] || 0 }}</span>
        </span>
      </div>
    </div>

    <!-- Filter & Search Control Bar -->
    <div class="filter-control-bar glass-panel">
      <!-- Search Input -->
      <div class="search-input-wrap">
        <span class="search-icon"><BaseIcon name="search" size="sm" /></span>
        <input :value="searchQuery" type="text" placeholder="Search hosts by name, IP, endpoint, label, or type..." class="search-input input-glass" @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)" />
        <button v-if="searchQuery" class="clear-search" @click="emit('update:searchQuery', '')"><BaseIcon name="x" size="xs" /></button>
      </div>

      <!-- Filters Group -->
      <div class="filters-group">
        <div class="filter-select-wrap">
          <label class="filter-lbl font-mono">TYPE:</label>
          <select :value="selectedTypeFilter" class="filter-select input-glass" @change="emit('update:selectedTypeFilter', ($event.target as HTMLSelectElement).value)">
            <option value="all">All Types ({{ totalHosts }})</option>
            <option v-for="def in hostTypeDefinitions" :key="def.type" :value="def.type">{{ def.label }} ({{ typeCounts[def.type] || 0 }})</option>
          </select>
        </div>

        <div class="filter-select-wrap">
          <label class="filter-lbl font-mono">STATUS:</label>
          <select :value="selectedStatusFilter" class="filter-select input-glass" @change="emit('update:selectedStatusFilter', ($event.target as HTMLSelectElement).value)">
            <option value="all">All Statuses</option>
            <option value="connected">Connected</option>
            <option value="disconnected">Disconnected</option>
            <option value="error">Error / Alert</option>
          </select>
        </div>

        <div v-if="availableLabels.length > 0" class="filter-select-wrap">
          <label class="filter-lbl font-mono">LABEL:</label>
          <select :value="selectedLabelFilter" class="filter-select input-glass" @change="emit('update:selectedLabelFilter', ($event.target as HTMLSelectElement).value)">
            <option value="all">All Labels</option>
            <option v-for="lbl in availableLabels" :key="lbl" :value="lbl">{{ lbl }}</option>
          </select>
        </div>

        <!-- View Mode Switcher -->
        <div class="view-mode-toggle">
          <button class="mode-btn" :class="{ 'mode-btn-active': viewMode === 'grid' }" title="Grid View" @click="emit('update:viewMode', 'grid')">
            <BaseIcon name="grid" size="sm" /><span>Grid</span>
          </button>
          <button class="mode-btn" :class="{ 'mode-btn-active': viewMode === 'table' }" title="Table View" @click="emit('update:viewMode', 'table')">
            <BaseIcon name="table" size="sm" /><span>Table</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
