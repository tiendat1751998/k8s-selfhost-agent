<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'

export type WorkloadSegmentTab = 'All' | 'Deployments' | 'StatefulSets' | 'DaemonSets' | 'CronJobs'

const props = defineProps<{
  searchQuery: string
  selectedSegmentTab: WorkloadSegmentTab
  statusFilter: string
  activeFilterTab: string
  selectedNamespaceFilter: string
  namespaces: string[]
  totalWorkloads: number
  totalReplicas: number
  readyReplicas: number
  healthyCount: number
  degradedCount: number
  k8sCount: number
  swarmCount: number
  loading: boolean
  segmentCounts: Record<WorkloadSegmentTab, number>
}>()

const emit = defineEmits<{
  (e: 'update:searchQuery', val: string): void
  (e: 'update:selectedSegmentTab', val: WorkloadSegmentTab): void
  (e: 'update:statusFilter', val: string): void
  (e: 'update:activeFilterTab', val: string): void
  (e: 'update:selectedNamespaceFilter', val: string): void
  (e: 'deploy'): void
  (e: 'openYaml'): void
  (e: 'openBlueprints'): void
  (e: 'refresh'): void
}>()

const segmentTabs: { id: WorkloadSegmentTab; label: string; icon: string }[] = [
  { id: 'All', label: 'All', icon: 'globe' },
  { id: 'Deployments', label: 'Deployments', icon: 'play' },
  { id: 'StatefulSets', label: 'Stateful', icon: 'database' },
  { id: 'DaemonSets', label: 'Daemon', icon: 'shield' },
  { id: 'CronJobs', label: 'Cron', icon: 'clock' },
]

const showMobileFilters = ref(false)
const showMoreActions = ref(false)
const moreMenuRef = ref<HTMLElement | null>(null)

function onMoreDocClick(e: MouseEvent) {
  if (showMoreActions.value && moreMenuRef.value && !moreMenuRef.value.contains(e.target as Node)) {
    showMoreActions.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', onMoreDocClick)
})

onUnmounted(() => {
  document.removeEventListener('click', onMoreDocClick)
})
</script>

<template>
  <div class="deployments-header-bar-wrapper">
    <!-- Compact 36px Mobile Command Bar (<768px) with Inline Telemetry -->
    <div class="mobile-command-bar">
      <div class="mobile-search-compact-wrap">
        <span class="mobile-search-ico"><BaseIcon name="search" size="xs" /></span>
        <input
          :value="searchQuery"
          type="text"
          placeholder="Filter workloads..."
          class="input-glass mobile-search-compact"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
        <button v-if="searchQuery" type="button" class="mobile-search-clear" @click="emit('update:searchQuery', '')">
          <BaseIcon name="x" size="xs" />
        </button>
      </div>

      <div class="mobile-telemetry-pill font-mono" :title="`${totalWorkloads} workloads · ${readyReplicas}/${totalReplicas} pods · ${healthyCount} healthy`">
        <span class="pill-pods">{{ readyReplicas }}/{{ totalReplicas }} pods</span>
        <span class="pill-sep">·</span>
        <span class="pill-ok">{{ healthyCount }} ok</span>
      </div>

      <button
        type="button"
        class="btn-filters-toggle"
        :class="{ 'filters-active': showMobileFilters }"
        title="Toggle Filter Options"
        @click="showMobileFilters = !showMobileFilters"
      >
        <BaseIcon name="sliders" size="xs" /> <span class="filter-btn-text">Filters</span>
      </button>

      <button
        type="button"
        class="btn-mobile-deploy"
        title="Deploy Workload"
        @click="emit('deploy')"
      >+
      </button>
    </div>

    <!-- Mobile Expandable Filter Strip Accordion (<768px) -->
    <Transition name="filter-slide">
      <div v-if="showMobileFilters" class="mobile-filter-strip glass-panel">
        <div class="mobile-filter-row">
          <label class="mobile-filter-label">Namespace:</label>
          <select
            :value="selectedNamespaceFilter"
            class="input-glass select-ns-mobile font-mono"
            @change="emit('update:selectedNamespaceFilter', ($event.target as HTMLSelectElement).value)"
          >
            <option value="all">All Namespaces ({{ namespaces.length }})</option>
            <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
          </select>
        </div>

        <div class="mobile-filter-row">
          <label class="mobile-filter-label">Status:</label>
          <div class="mobile-pills-group">
            <button
              type="button"
              class="pill-btn pill-xs"
              :class="{ 'pill-active': statusFilter === 'all' }"
              @click="emit('update:statusFilter', 'all')"
            >
              All ({{ totalWorkloads }})
            </button>
            <button
              type="button"
              class="pill-btn pill-xs pill-healthy"
              :class="{ 'pill-active': statusFilter === 'healthy' }"
              @click="emit('update:statusFilter', 'healthy')"
            >
              <BaseIcon name="shield" size="xs" /> Healthy ({{ healthyCount }})
            </button>
            <button
              type="button"
              class="pill-btn pill-xs pill-degraded"
              :class="{ 'pill-active': statusFilter === 'degraded' }"
              @click="emit('update:statusFilter', 'degraded')"
            >
              <BaseIcon name="alert-triangle" size="xs" /> Degraded ({{ degradedCount }})
            </button>
          </div>
        </div>

        <div class="mobile-filter-row">
          <label class="mobile-filter-label">Type:</label>
          <div class="mobile-pills-group">
            <button
              type="button"
              class="pill-btn pill-xs"
              :class="{ 'pill-active': activeFilterTab === 'all' }"
              @click="emit('update:activeFilterTab', 'all')"
            >
              All
            </button>
            <button
              type="button"
              class="pill-btn pill-xs"
              :class="{ 'pill-active': activeFilterTab === 'k8s' }"
              @click="emit('update:activeFilterTab', 'k8s')"
            >
              <BaseIcon name="anchor" size="xs" /> K8s ({{ k8sCount }})
            </button>
            <button
              type="button"
              class="pill-btn pill-xs"
              :class="{ 'pill-active': activeFilterTab === 'swarm' }"
              @click="emit('update:activeFilterTab', 'swarm')"
            >
              <BaseIcon name="box" size="xs" /> Docker ({{ swarmCount }})
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Sleek Unified Enterprise Toolbar -->
    <div class="deployments-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="Deployments Fleet Toolbar">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          :value="searchQuery"
          type="text"
          placeholder="Filter workloads..."
          class="toolbar-search-input"
          aria-label="Filter workloads by name, image, namespace"
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

      <!-- Segmented kind capsule pills -->
      <div class="toolbar-capsule-pills" role="tablist" aria-label="Workload Kind">
        <button
          v-for="tab in segmentTabs"
          :key="tab.id"
          type="button"
          role="tab"
          :aria-selected="selectedSegmentTab === tab.id"
          class="capsule-pill font-mono"
          :class="{ active: selectedSegmentTab === tab.id }"
          @click="emit('update:selectedSegmentTab', tab.id)"
        >
          <span>{{ tab.label }}</span>
          <span class="capsule-count">({{ segmentCounts[tab.id] ?? 0 }})</span>
        </button>
      </div>

      <!-- Status filter dropdown -->
      <select
        :value="statusFilter"
        class="toolbar-select font-mono"
        aria-label="Filter by Status"
        @change="emit('update:statusFilter', ($event.target as HTMLSelectElement).value)"
      >
        <option value="all">All Status ({{ totalWorkloads }})</option>
        <option value="healthy">Healthy ({{ healthyCount }})</option>
        <option value="degraded">Degraded ({{ degradedCount }})</option>
      </select>

      <!-- Action buttons -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="toolbar-btn btn-primary toolbar-btn-deploy"
          title="Deploy Workload"
          @click="emit('deploy')"
        >
          <span>+ Deploy</span>
        </button>

        <!-- Sleek Secondary Actions Dropdown [More] -->
        <div ref="moreMenuRef" class="toolbar-more-wrap">
          <button
            type="button"
            class="toolbar-btn btn-secondary toolbar-btn-more"
            :class="{ active: showMoreActions }"
            title="More Actions"
            aria-haspopup="true"
            :aria-expanded="showMoreActions"
            @click="showMoreActions = !showMoreActions"
          >
            <span>More</span>
            <BaseIcon name="chevron-down" size="xs" class="more-chevron" />
          </button>
          <div v-if="showMoreActions" class="toolbar-more-menu glass-panel" role="menu">
            <button
              type="button"
              class="more-menu-item"
              role="menuitem"
              @click="emit('openYaml'); showMoreActions = false"
            >
              <BaseIcon name="file-text" size="xs" />
              <span>YAML Manifest</span>
            </button>
            <button
              type="button"
              class="more-menu-item"
              role="menuitem"
              @click="emit('openBlueprints'); showMoreActions = false"
            >
              <BaseIcon name="box" size="xs" />
              <span>Blueprint Catalog</span>
            </button>
          </div>
        </div>

        <button
          type="button"
          class="toolbar-btn btn-secondary toolbar-btn-refresh"
          :disabled="loading"
          title="Refresh Workloads"
          aria-label="Refresh Workloads"
          @click="emit('refresh')"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" />
        </button>
      </div>
    </div>
  </div>
</template>
