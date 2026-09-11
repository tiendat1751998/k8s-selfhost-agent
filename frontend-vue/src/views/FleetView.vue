<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAppStore } from '../stores/app'
import ClusterDetailsDrawer from '../components/fleet/ClusterDetailsDrawer.vue'
import FleetClustersGrid from '../components/fleet/FleetClustersGrid.vue'
import FleetClustersTable from '../components/fleet/FleetClustersTable.vue'
import FleetMobileCards from '../components/fleet/FleetMobileCards.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import FleetImportModal from '../components/fleet/FleetImportModal.vue'
import FleetDiscoveryDrawer from '../components/fleet/FleetDiscoveryDrawer.vue'
import {
  fleetApi,
  type Cluster,
  type ClusterDiscoveryData,
  type SwarmClusterInfo,
  mapSwarmToFleetCluster
} from '../api/fleet'
import '@/assets/styles/views/fleet.css'

const appStore = useAppStore()

// Async loading & error states
const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

// Data state
const clusters = ref<Cluster[]>([])
const swarmInfo = ref<SwarmClusterInfo | null>(null)

// View mode state (Segmented toggle: Table or Cards Grid)
const viewMode = ref<'table' | 'grid'>('table')

// Filter state
const providerFilter = ref<string>('all')
const statusFilter = ref<'all' | 'healthy' | 'degraded' | 'offline'>('all')
const searchFilter = ref('')
const showMobileSearch = ref(false)

// Modals & Drawers state
const showImportModal = ref(false)
const showDiscoveryDrawer = ref(false)
const selectedCluster = ref<Cluster | null>(null)
const discoveredData = ref<ClusterDiscoveryData | null>(null)
const selectedDrawerCluster = ref<Cluster | null>(null)
const showClusterDetailsDrawer = ref(false)

async function fetchFleet() {
  loading.value = true
  error.value = null
  try {
    const [clusterList, swarm] = await Promise.all([
      fleetApi.list(),
      fleetApi.getSwarm().catch(() => null),
    ])
    clusters.value = clusterList
    swarmInfo.value = swarm
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve multi-cluster fleet'
    error.value = msg
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchFleet()
})

// Metrics computation
const hasSwarm = computed(() => swarmInfo.value !== null && (swarmInfo.value.node_count > 0 || Boolean(swarmInfo.value.id)))

// Unified clusters list: Kubernetes clusters + Swarm cluster (if active)
const unifiedClusters = computed<Cluster[]>(() => {
  const k8sClusters = clusters.value.map(c => ({
    ...c,
    orchestrator: (c.orchestrator || 'kubernetes') as 'kubernetes' | 'swarm',
  }))
  if (hasSwarm.value && swarmInfo.value) {
    return [mapSwarmToFleetCluster(swarmInfo.value), ...k8sClusters]
  }
  return k8sClusters
})

const totalClusters = computed(() => unifiedClusters.value.length)

const HEALTHY_STATUSES = new Set(['healthy', 'active', 'ready', 'connected', 'online', 'live', 'ok'])
const DEGRADED_STATUSES = new Set(['warning', 'pending', 'standby', 'degraded', 'in_progress', 'promoting'])
const OFFLINE_STATUSES = new Set(['critical', 'danger', 'failed', 'error', 'offline', 'disconnected', 'down', 'unhealthy'])

const onlineCount = computed(() =>
  unifiedClusters.value.filter(c => HEALTHY_STATUSES.has((c.health_status || c.status || '').toLowerCase())).length
)
const healthyClusters = computed(() => onlineCount.value)

const totalNodes = computed(() =>
  unifiedClusters.value.reduce((acc, c) => acc + (c.nodes || 0), 0)
)

const totalCores = computed(() => {
  return unifiedClusters.value.reduce((acc, c) => {
    const rawCores = (c as any).cores || (c as any).cpu_cores || (c.discovered_resources as any)?.total_cores || (c.discovered_resources as any)?.cpu_cores
    if (typeof rawCores === 'number') return acc + rawCores
    const nodes = c.nodes && c.nodes > 0 ? c.nodes : 1
    return acc + nodes * 4
  }, 0)
})

const totalPods = computed(() => {
  return appStore.latestMetrics?.total_containers || clusters.value.reduce((acc, c) => acc + ((c as any).pods || 0), 0) || 0
})

function matchesProvider(cluster: Cluster, provider: string): boolean {
  if (provider === 'all') return true
  const p = (cluster.provider || '').toLowerCase()
  const g = (cluster.group || '').toLowerCase()
  const orch = (cluster.orchestrator || '').toLowerCase()
  if (provider === 'kubernetes') return orch === 'kubernetes' || p !== 'swarm'
  if (provider === 'swarm') return orch === 'swarm' || p === 'swarm'
  if (provider === 'bare-metal') {
    return p === 'bare-metal' || p === 'baremetal' || p === 'onprem' || p === 'metal' || p === 'generic' || p === 'local'
  }
  if (provider === 'aws') return p === 'aws' || p === 'eks'
  if (provider === 'gcp') return p === 'gcp' || p === 'gke'
  if (provider === 'azure') return p === 'azure' || p === 'aks'
  if (provider === 'edge') return p === 'edge' || g === 'edge'
  return p === provider.toLowerCase()
}

function matchesStatus(cluster: Cluster, status: 'all' | 'healthy' | 'degraded' | 'offline'): boolean {
  if (status === 'all') return true
  const s = (cluster.health_status || cluster.status || '').toLowerCase()
  if (status === 'healthy') return HEALTHY_STATUSES.has(s)
  if (status === 'degraded') return DEGRADED_STATUSES.has(s)
  if (status === 'offline') return OFFLINE_STATUSES.has(s)
  return false
}

const filteredClusters = computed(() => {
  let result = unifiedClusters.value

  if (providerFilter.value !== 'all') {
    result = result.filter(c => matchesProvider(c, providerFilter.value))
  }
  if (statusFilter.value !== 'all') {
    result = result.filter(c => matchesStatus(c, statusFilter.value))
  }
  if (searchFilter.value.trim()) {
    const q = searchFilter.value.toLowerCase().trim()
    result = result.filter(c =>
      (c.name || '').toLowerCase().includes(q) ||
      (c.group || '').toLowerCase().includes(q) ||
      (c.provider || '').toLowerCase().includes(q) ||
      (c.region || '').toLowerCase().includes(q) ||
      (c.orchestrator || '').toLowerCase().includes(q) ||
      (c.health_status || c.status || '').toLowerCase().includes(q)
    )
  }
  return result
})

function openClusterDetails(c: Cluster) {
  selectedDrawerCluster.value = c
  showClusterDetailsDrawer.value = true
}

function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

async function handleUpgrade(cluster: Cluster) {
  actionLoading.value = cluster.id
  try {
    await fleetApi.upgrade(cluster.id)
    showToast(`Cluster upgrade sequence initiated for ${cluster.name}!`)
    await fetchFleet()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Upgrade request failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleDiscover(cluster: Cluster) {
  selectedCluster.value = cluster
  actionLoading.value = cluster.id
  try {
    const res = await fleetApi.discover(cluster.id)
    discoveredData.value = res || (cluster.discovered_resources as ClusterDiscoveryData) || null
    showDiscoveryDrawer.value = true
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Resource discovery failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleRemove(cluster: Cluster) {
  if (!confirm(`Are you sure you want to remove cluster "${cluster.name}" from the fleet?`)) return
  actionLoading.value = cluster.id
  try {
    await fleetApi.remove(cluster.id)
    showToast(`Cluster ${cluster.name} removed from fleet.`)
    await fetchFleet()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to remove cluster'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Mobile PWA Ergonomics: 44px Command Bar (<640px) -->
    <div class="mobile-fleet-command-bar">
      <div class="mobile-command-title">
        <span><BaseIcon name="anchor" size="xs" /> Clusters</span>
        <span class="mobile-badge-pill font-mono">({{ filteredClusters.length }})</span>
      </div>
      <div class="mobile-command-actions">
        <button class="btn btn-secondary btn-xs" @click="showMobileSearch = !showMobileSearch" title="Search">
          <BaseIcon name="search" size="xs" />
        </button>
        <button class="btn btn-secondary btn-xs" :disabled="loading" @click="fetchFleet" title="Refresh">
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" />
        </button>
        <button class="btn btn-primary btn-xs" @click="showImportModal = true">
          + Connect
        </button>
      </div>
    </div>

    <!-- Mobile Expandable Search Strip -->
    <div v-if="showMobileSearch" class="mobile-search-strip animate-fade-in">
      <div class="mobile-search-row">
        <input
          v-model="searchFilter"
          type="text"
          placeholder="Filter clusters..."
          class="input-glass mobile-search-input"
          autofocus
        />
        <button v-if="searchFilter" class="search-clear-btn" @click="searchFilter = ''">
          <BaseIcon name="x" size="xs" />
        </button>
      </div>
      <div class="mobile-filter-selects">
        <select v-model="providerFilter" class="toolbar-select mobile-select font-mono">
          <option value="all">All Providers</option>
          <option value="kubernetes">Kubernetes</option>
          <option value="swarm">Docker Swarm</option>
          <option value="bare-metal">Bare-Metal</option>
          <option value="aws">AWS</option>
          <option value="gcp">GCP</option>
          <option value="azure">Azure</option>
          <option value="edge">Edge</option>
        </select>
        <select v-model="statusFilter" class="toolbar-select mobile-select font-mono">
          <option value="all">All Statuses</option>
          <option value="healthy">Healthy</option>
          <option value="degraded">Degraded</option>
          <option value="offline">Offline</option>
        </select>
      </div>
    </div>

    <!-- Mobile PWA Ergonomics: 20px Micro-telemetry strip (<640px) -->
    <div class="micro-telemetry-strip font-mono">
      <span><BaseIcon name="anchor" size="xs" /> {{ totalClusters }} clusters</span>
      <span class="telemetry-sep">?</span>
      <span><BaseIcon name="shield" size="xs" /> {{ healthyClusters }} healthy</span>
      <span class="telemetry-sep">?</span>
      <span><BaseIcon name="server" size="xs" /> {{ totalNodes }} nodes</span>
      <span class="telemetry-sep">?</span>
      <span><BaseIcon name="box" size="xs" /> {{ totalPods }} pods</span>
    </div>

    <!-- Sleek Unified 38px Enterprise Toolbar (Desktop & Tablet) -->
    <div class="fleet-toolbar-sleek">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="toolbar-search-icon" />
        <input
          v-model="searchFilter"
          type="text"
          placeholder="Search clusters..."
          class="toolbar-search-input font-mono"
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

      <!-- Provider / Orchestrator filter dropdown -->
      <select v-model="providerFilter" class="toolbar-select font-mono" title="Filter by provider / orchestrator">
        <option value="all">All Providers</option>
        <option value="kubernetes">Kubernetes</option>
        <option value="swarm">Docker Swarm</option>
        <option value="bare-metal">Bare-Metal</option>
        <option value="aws">AWS</option>
        <option value="gcp">GCP</option>
        <option value="azure">Azure</option>
        <option value="edge">Edge</option>
      </select>

      <!-- Status filter dropdown -->
      <select v-model="statusFilter" class="toolbar-select font-mono" title="Filter by status">
        <option value="all">All Statuses</option>
        <option value="healthy">Healthy</option>
        <option value="degraded">Degraded</option>
        <option value="offline">Offline</option>
      </select>

      <!-- Inline compact KPI badge strip font-mono -->
      <div class="toolbar-kpi-badge font-mono">
        <span class="kpi-count">{{ totalClusters }} Clusters</span>
        <span class="kpi-meta">({{ healthyClusters }} Online ? {{ totalNodes }} Nodes ? {{ totalCores }} Cores)</span>
      </div>

      <div class="toolbar-spacer"></div>

      <!-- Segmented View Mode Toggle: [ Table ] [ Cards ] -->
      <div class="segmented-control font-mono">
        <button
          type="button"
          class="segmented-btn"
          :class="{ active: viewMode === 'table' }"
          @click="viewMode = 'table'"
          title="Table View"
        >
          <BaseIcon name="file-text" size="xs" />
          <span>Table</span>
        </button>
        <button
          type="button"
          class="segmented-btn"
          :class="{ active: viewMode === 'grid' }"
          @click="viewMode = 'grid'"
          title="Card Grid View"
        >
          <BaseIcon name="box" size="xs" />
          <span>Cards</span>
        </button>
      </div>

      <!-- Action buttons: Sync and + Connect Cluster -->
      <button
        type="button"
        class="btn btn-secondary toolbar-btn"
        :disabled="loading"
        @click="fetchFleet"
        title="Sync Fleet"
      >
        <BaseIcon :name="loading ? 'activity' : 'refresh'" size="xs" :class="{ 'spin-icon': loading }" />
        <span>Sync</span>
      </button>
      <button
        type="button"
        class="btn btn-primary toolbar-btn"
        @click="showImportModal = true"
      >
        <span>+ Connect Cluster</span>
      </button>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="sm" />
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Error Banner -->
    <div v-if="error" class="toast-banner toast-error animate-fade-in">
      <BaseIcon name="alert-triangle" size="sm" />
      <span>{{ error }}</span>
      <button class="toast-close" @click="error = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Desktop Fleet Section (Single viewMode: Table OR Grid, NEVER both) -->
    <div class="desktop-only">
      <FleetClustersTable
        v-if="viewMode === 'table'"
        :clusters="filteredClusters"
        :loading="loading"
        :error="error"
        :action-loading="actionLoading"
        @discover="handleDiscover"
        @upgrade="handleUpgrade"
        @remove="handleRemove"
        @details="openClusterDetails"
        @import="showImportModal = true"
      />

      <FleetClustersGrid
        v-else-if="viewMode === 'grid'"
        :clusters="filteredClusters"
        :total-clusters-count="unifiedClusters.length"
        :action-loading="actionLoading"
        :loading="loading"
        @discover="handleDiscover"
        @upgrade="handleUpgrade"
        @remove="handleRemove"
        @details="openClusterDetails"
        @import="showImportModal = true"
      />
    </div>

    <!-- Mobile-First Touch-Optimized Cluster Stream (<768px) -->
    <div class="mobile-only">
      <FleetMobileCards
        :clusters="filteredClusters"
        :action-loading="actionLoading"
        @discover="handleDiscover"
        @upgrade="handleUpgrade"
        @remove="handleRemove"
        @details="openClusterDetails"
      />
    </div>

    <!-- Standalone Import Cluster Modal -->
    <FleetImportModal v-model:show="showImportModal" @imported="fetchFleet" />

    <!-- Standalone Resource Discovery Drawer -->
    <FleetDiscoveryDrawer
      v-model:show="showDiscoveryDrawer"
      :cluster="selectedCluster"
      :discovered-data="discoveredData"
    />

    <!-- Cluster Details Drawer -->
    <ClusterDetailsDrawer
      v-model:show="showClusterDetailsDrawer"
      :cluster="selectedDrawerCluster"
      :discovery-data="discoveredData"
      @updated="fetchFleet"
    />
  </div>
</template>
