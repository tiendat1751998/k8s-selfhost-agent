<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  searchQuery: string
  selectedCluster: string
  selectedTimeWindow: string
  selectedStatus: string
  clusters: string[]
}>()

defineEmits<{
  'update:searchQuery': [val: string]
  'update:selectedCluster': [val: string]
  'update:selectedTimeWindow': [val: string]
  'update:selectedStatus': [val: string]
  'create': []
}>()

const timeWindows = [
  { id: '1h', label: '1H' },
  { id: '6h', label: '6H' },
  { id: '24h', label: '24H' },
  { id: '7d', label: '7D' },
  { id: 'all', label: 'ALL' }
]

const statuses = [
  { id: 'all', label: 'ALL' },
  { id: 'pending', label: 'PENDING' },
  { id: 'approved', label: 'APPROVED' },
  { id: 'deployed', label: 'DEPLOYED' },
  { id: 'rejected', label: 'REJECTED' }
]
</script>

<template>
  <div class="changes-filter-bar glass-panel">
    <div class="filter-bar-row">
      <!-- Search Input -->
      <div class="search-input-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          :value="searchQuery"
          type="text"
          placeholder="Search RFCs, resources, clusters, or authors..."
          class="input-glass search-field"
          @input="$emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
        <button
          v-if="searchQuery"
          class="clear-search-btn"
          title="Clear search"
          @click="$emit('update:searchQuery', '')"
        ><BaseIcon name="x" size="xs" /></button>
      </div>

      <!-- Cluster Selector -->
      <div class="cluster-selector-wrap">
        <label class="filter-label">Cluster:</label>
        <select
          :value="selectedCluster"
          class="input-glass cluster-select"
          @change="$emit('update:selectedCluster', ($event.target as HTMLSelectElement).value)"
        >
          <option value="all">All Clusters</option>
          <option v-for="c in clusters" :key="c" :value="c">{{ c }}</option>
        </select>
      </div>

      <!-- Status Filters -->
      <div class="status-filters-group">
        <span class="filter-label">Status:</span>
        <div class="status-pills">
          <button
            v-for="s in statuses"
            :key="s.id"
            class="spill"
            :class="{ active: selectedStatus === s.id }"
            @click="$emit('update:selectedStatus', s.id)"
          >
            {{ s.label }}
          </button>
        </div>
      </div>

      <!-- Time Window Selector -->
      <div class="time-window-group">
        <span class="filter-label">Window:</span>
        <div class="time-pills">
          <button
            v-for="w in timeWindows"
            :key="w.id"
            class="tpill"
            :class="{ active: selectedTimeWindow === w.id }"
            @click="$emit('update:selectedTimeWindow', w.id)"
          >
            {{ w.label }}
          </button>
        </div>
      </div>

      <!-- Action: Submit RFC Button -->
      <div class="filter-actions-wrap desktop-only">
        <button type="button" class="btn btn-primary btn-sm" @click="$emit('create')">
          <span>+ Submit Change Request</span>
        </button>
      </div>
    </div>
  </div>
</template>
