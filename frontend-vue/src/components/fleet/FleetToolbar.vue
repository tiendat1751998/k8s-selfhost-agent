<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

const searchFilter = defineModel<string>('searchFilter', { default: '' })
const providerFilter = defineModel<string>('providerFilter', { default: 'all' })
const statusFilter = defineModel<'all' | 'healthy' | 'degraded' | 'offline'>('statusFilter', { default: 'all' })
const viewMode = defineModel<'table' | 'grid'>('viewMode', { default: 'table' })

defineProps<{
  totalClusters: number
  healthyClusters: number
  totalNodes: number
  totalCores: number
  isSyncing?: boolean
}>()

const emit = defineEmits<{
  (e: 'sync'): void
  (e: 'connect'): void
}>()
</script>

<template>
  <div class="fleet-toolbar-sleek">
    <!-- Zone 1: Search input (28px height, capsule pill, 160-180px) -->
    <div class="toolbar-search-wrap">
      <BaseIcon name="search" size="xs" class="toolbar-search-icon" />
      <input
        v-model="searchFilter"
        type="text"
        placeholder="Search clusters..."
        class="toolbar-search-input"
      />
      <button
        v-if="searchFilter"
        type="button"
        class="toolbar-search-clear"
        title="Clear search"
        @click="searchFilter = ''"
      >
        <BaseIcon name="x" size="xs" />
      </button>
    </div>

    <!-- Zone 2: Provider / Orchestrator filter dropdown (28px capsule pill) -->
    <div class="pill-select-wrap">
      <select
        v-model="providerFilter"
        class="pill-select"
        title="Filter by provider / orchestrator"
      >
        <option value="all">All Providers</option>
        <option value="kubernetes">Kubernetes</option>
        <option value="swarm">Docker Swarm</option>
        <option value="bare-metal">Bare-Metal</option>
        <option value="aws">AWS</option>
        <option value="gcp">GCP</option>
        <option value="azure">Azure</option>
        <option value="edge">Edge</option>
      </select>
      <BaseIcon name="chevron-down" size="xs" class="select-chevron" />
    </div>

    <!-- Zone 3: Status Filter 4 Capsule Pills (28px height, border-radius: 9999px) -->
    <div class="toolbar-status-pills" role="tablist" aria-label="Filter by cluster health status">
      <button
        type="button"
        class="capsule-pill"
        :class="{ active: statusFilter === 'all' }"
        title="Show all clusters"
        @click="statusFilter = 'all'"
      >
        All
      </button>
      <button
        type="button"
        class="capsule-pill"
        :class="{ active: statusFilter === 'healthy' }"
        title="Filter healthy clusters"
        @click="statusFilter = 'healthy'"
      >
        <span class="status-dot dot-healthy">●</span>
        <span>Healthy</span>
      </button>
      <button
        type="button"
        class="capsule-pill"
        :class="{ active: statusFilter === 'degraded' }"
        title="Filter degraded clusters"
        @click="statusFilter = 'degraded'"
      >
        <span class="status-dot dot-degraded">▲</span>
        <span>Degraded</span>
      </button>
      <button
        type="button"
        class="capsule-pill"
        :class="{ active: statusFilter === 'offline' }"
        title="Filter offline clusters"
        @click="statusFilter = 'offline'"
      >
        <span class="status-dot dot-offline">✕</span>
        <span>Offline</span>
      </button>
    </div>

    <!-- Zone 4: Inline compact KPI badge strip font-mono -->
    <div class="toolbar-kpi-badge font-mono">
      <span class="kpi-count">{{ totalClusters }} Clusters</span>
      <span class="kpi-meta">({{ healthyClusters }} Online · {{ totalNodes }} Nodes · {{ totalCores }} Cores)</span>
    </div>

    <div class="toolbar-spacer"></div>

    <!-- Zone 5: Segmented View Mode Toggle: [ Table ] [ Cards ] (28px height) -->
    <div class="segmented-control" role="group" aria-label="View mode toggle">
      <button
        type="button"
        class="segmented-btn"
        :class="{ active: viewMode === 'table' }"
        title="Table View"
        @click="viewMode = 'table'"
      >
        <BaseIcon name="file-text" size="xs" />
        <span>Table</span>
      </button>
      <button
        type="button"
        class="segmented-btn"
        :class="{ active: viewMode === 'grid' }"
        title="Card Grid View"
        @click="viewMode = 'grid'"
      >
        <BaseIcon name="box" size="xs" />
        <span>Cards</span>
      </button>
    </div>

    <!-- Zone 6: Action buttons: Sync and + Connect Cluster (28px height) -->
    <div class="toolbar-actions-group">
      <button
        type="button"
        class="toolbar-btn btn-secondary"
        :disabled="isSyncing"
        title="Sync Fleet"
        @click="emit('sync')"
      >
        <BaseIcon :name="isSyncing ? 'activity' : 'refresh'" size="xs" :class="{ 'spin-icon': isSyncing }" />
        <span>Sync</span>
      </button>
      <button
        type="button"
        class="toolbar-btn btn-primary"
        title="Connect New Cluster"
        @click="emit('connect')"
      >
        <span>+ Connect Cluster</span>
      </button>
    </div>
  </div>
</template>
