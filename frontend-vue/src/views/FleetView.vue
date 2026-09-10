<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAppStore } from '../stores/app'
import ClusterDetailsDrawer from '../components/fleet/ClusterDetailsDrawer.vue'
import FleetMetricHud from '../components/fleet/FleetMetricHud.vue'
import FleetSwarmBanner from '../components/fleet/FleetSwarmBanner.vue'
import FleetClustersGrid from '../components/fleet/FleetClustersGrid.vue'
import FleetClustersTable from '../components/fleet/FleetClustersTable.vue'
import FleetMobileCards from '../components/fleet/FleetMobileCards.vue'
import FleetImportModal from '../components/fleet/FleetImportModal.vue'
import FleetDiscoveryDrawer from '../components/fleet/FleetDiscoveryDrawer.vue'
import {
  fleetApi,
  type Cluster,
  type ClusterDiscoveryData,
  type SwarmClusterInfo
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
const clusterTypeFilter = ref<'all' | 'k8s' | 'swarm'>('all')
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
const k8sClusterCount = computed(() => clusters.value.length)
const hasSwarm = computed(() => swarmInfo.value !== null && (swarmInfo.value.node_count > 0 || Boolean(swarmInfo.value.id)))
const totalControlPlanes = computed(() => clusters.value.length + (hasSwarm.value ? 1 : 0))
const totalClusters = computed(() => clusters.value.length)

const countK8s = computed(() => clusters.value.length)
const countSwarm = computed(() => hasSwarm.value ? 1 : 0)
const countAll = computed(() => countK8s.value + countSwarm.value)

const k8sNodes = computed(() => clusters.value.reduce((acc, c) => acc + (c.nodes || 0), 0))
const swarmNodes = computed(() => (hasSwarm.value && swarmInfo.value ? swarmInfo.value.node_count : 0))
const totalNodes = computed(() => k8sNodes.value + swarmNodes.value)

const HEALTHY_STATUSES = new Set(['healthy', 'active', 'ready', 'connected', 'online', 'live', 'ok'])
const DEGRADED_STATUSES = new Set(['warning', 'pending', 'standby', 'degraded', 'in_progress', 'promoting'])
const OFFLINE_STATUSES = new Set(['critical', 'danger', 'failed', 'error', 'offline', 'disconnected', 'down', 'unhealthy'])

const healthyK8sCount = computed(() =>
  clusters.value.filter(c => HEALTHY_STATUSES.has((c.health_status || c.status || '').toLowerCase())).length
)
const healthyCount = computed(() => healthyK8sCount.value + (hasSwarm.value ? 1 : 0))
const healthyClusters = computed(() => healthyK8sCount.value)

const totalPods = computed(() => {
  return appStore.latestMetrics?.total_containers || clusters.value.reduce((acc, c) => acc + ((c as any).pods || 0), 0) || 0
})

const cloudProviders = computed(() => {
  const providers = new Set<string>()
  clusters.value.forEach(c => {
    if (c.provider) providers.add(c.provider.toUpperCase())
  })
  if (hasSwarm.value) providers.add('SWARM')
  return Array.from(providers)
})

function matchesStatus(cluster: Cluster, status: 'all' | 'healthy' | 'degraded' | 'offline'): boolean {
  if (status === 'all') return true
  const s = (cluster.health_status || cluster.status || '').toLowerCase()
  if (status === 'healthy') return HEALTHY_STATUSES.has(s)
  if (status === 'degraded') return DEGRADED_STATUSES.has(s)
  if (status === 'offline') return OFFLINE_STATUSES.has(s)
  return false
}

const showK8sClusters = computed(() => {
  return clusterTypeFilter.value === 'all' || clusterTypeFilter.value === 'k8s'
})

const showSwarmBanner = computed(() => {
  if (!hasSwarm.value || !swarmInfo.value) return false
  if (clusterTypeFilter.value === 'k8s') return false
  if (statusFilter.value !== 'all' && statusFilter.value !== 'healthy') return false
  if (searchFilter.value.trim()) {
    const q = searchFilter.value.toLowerCase().trim()
    const match = 'docker swarm'.includes(q) || 'swarm'.includes(q) || (swarmInfo.value.id || '').toLowerCase().includes(q)
    if (!match) return false
  }
  return true
})

const filteredClusters = computed(() => {
  if (!showK8sClusters.value) return []
  let result = clusters.value

  if (statusFilter.value !== 'all') {
    result = result.filter(c => matchesStatus(c, statusFilter.value))
  }
  if (searchFilter.value.trim()) {
    const q = searchFilter.value.toLowerCase().trim()
    result = result.filter(c =>
      c.name.toLowerCase().includes(q) ||
      (c.group || '').toLowerCase().includes(q) ||
      (c.provider || '').toLowerCase().includes(q) ||
      (c.region || '').toLowerCase().includes(q)
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
        <span>☸️ Clusters</span>
        <span class="mobile-badge-pill font-mono">({{ filteredClusters.length }})</span>
      </div>
      <div class="mobile-command-actions">
        <button class="btn btn-secondary btn-xs" @click="showMobileSearch = !showMobileSearch" title="Search">
          🔍
        </button>
        <button class="btn btn-secondary btn-xs" :disabled="loading" @click="fetchFleet" title="Refresh">
          🔄
        </button>
        <button class="btn btn-primary btn-xs" @click="showImportModal = true">
          ➕ Import
        </button>
      </div>
    </div>

    <!-- Mobile Expandable Search Strip -->
    <div v-if="showMobileSearch" class="mobile-search-strip animate-fade-in">
      <input
        v-model="searchFilter"
        type="text"
        placeholder="Filter clusters..."
        class="input-glass mobile-search-input"
        autofocus
      />
      <button v-if="searchFilter" class="search-clear-btn" @click="searchFilter = ''">✕</button>
    </div>

    <!-- Mobile PWA Ergonomics: 20px Micro-telemetry strip (<640px) -->
    <div class="micro-telemetry-strip font-mono">
      <span>☸️ {{ totalClusters }} clusters</span>
      <span class="telemetry-sep">·</span>
      <span>🛡️ {{ healthyClusters }} healthy</span>
      <span class="telemetry-sep">·</span>
      <span>💻 {{ totalNodes }} nodes</span>
      <span class="telemetry-sep">·</span>
      <span>📦 {{ totalPods }} pods</span>
    </div>

    <!-- Desktop View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>HYBRID & MULTI-CLOUD FEDERATION</span>
        </div>
        <h1 class="view-title">
          <span class="title-full">Multi-Cluster Fleet Manager</span>
          <span class="title-mobile">🌐 Fleet Clusters</span>
        </h1>
        <p class="view-desc">
          Centralized topology dashboard for on-premise, edge, Docker Swarm, and cloud Kubernetes clusters with live health monitoring and zero-downtime upgrades.
        </p>
      </div>

      <div class="header-actions">
        <!-- Segmented View Mode Toggle: [ 📋 Table ] [ 🔲 Cards ] -->
        <div class="segmented-control font-mono">
          <button
            class="segmented-btn"
            :class="{ active: viewMode === 'table' }"
            @click="viewMode = 'table'"
            title="Table View"
          >
            <span>📋 Table</span>
          </button>
          <button
            class="segmented-btn"
            :class="{ active: viewMode === 'grid' }"
            @click="viewMode = 'grid'"
            title="Card Grid View"
          >
            <span>🔲 Cards</span>
          </button>
        </div>

        <button class="btn btn-secondary" :disabled="loading" @click="fetchFleet">
          <span class="btn-text-full">{{ loading ? '⏳ Syncing...' : '🔄 Refresh Fleet' }}</span>
          <span class="btn-text-mobile">{{ loading ? '⏳ Syncing...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="showImportModal = true">
          <span class="btn-text-full">+ Import Cluster</span>
          <span class="btn-text-mobile">+ Import</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- Error Banner -->
    <div v-if="error" class="toast-banner toast-error animate-fade-in">
      <span>⚠️</span>
      <span>{{ error }}</span>
      <button class="toast-close" @click="error = null">✕</button>
    </div>

    <!-- 4-Card Metric HUD (Desktop) -->
    <FleetMetricHud
      :total-control-planes="totalControlPlanes"
      :k8s-cluster-count="k8sClusterCount"
      :has-swarm="hasSwarm"
      :healthy-count="healthyCount"
      :total-nodes="totalNodes"
      :k8s-nodes="k8sNodes"
      :swarm-nodes="swarmNodes"
      :cloud-providers="cloudProviders"
    />

    <!-- Enterprise Unified Filter Bar -->
    <div class="fleet-filter-bar glass-panel">
      <!-- Search Field -->
      <div class="filter-search-wrap">
        <span class="filter-search-icon">🔍</span>
        <input
          v-model="searchFilter"
          type="text"
          placeholder="Filter fleet by name, provider, tier..."
          class="input-glass filter-search-input"
        />
        <button
          v-if="searchFilter"
          class="filter-search-clear"
          title="Clear search"
          @click="searchFilter = ''"
        >
          ✕
        </button>
      </div>

      <div class="filter-groups-wrap">
        <!-- Cluster Type Pills -->
        <div class="filter-group">
          <span class="filter-group-label">TYPE:</span>
          <div class="filter-pills">
            <button
              class="filter-pill"
              :class="{ active: clusterTypeFilter === 'all' }"
              @click="clusterTypeFilter = 'all'"
            >
              All ({{ countAll }})
            </button>
            <button
              class="filter-pill"
              :class="{ active: clusterTypeFilter === 'k8s' }"
              @click="clusterTypeFilter = 'k8s'"
            >
              ☸️ Kubernetes ({{ countK8s }})
            </button>
            <button
              class="filter-pill"
              :class="{ active: clusterTypeFilter === 'swarm' }"
              @click="clusterTypeFilter = 'swarm'"
            >
              🐳 Docker Swarm ({{ countSwarm }})
            </button>
          </div>
        </div>

        <!-- Status Filter Pills -->
        <div class="filter-group">
          <span class="filter-group-label">STATUS:</span>
          <div class="filter-pills">
            <button
              class="filter-pill"
              :class="{ active: statusFilter === 'all' }"
              @click="statusFilter = 'all'"
            >
              All
            </button>
            <button
              class="filter-pill pill-healthy"
              :class="{ active: statusFilter === 'healthy' }"
              @click="statusFilter = 'healthy'"
            >
              <span class="status-dot dot-emerald"></span> Healthy
            </button>
            <button
              class="filter-pill pill-degraded"
              :class="{ active: statusFilter === 'degraded' }"
              @click="statusFilter = 'degraded'"
            >
              <span class="status-dot dot-amber"></span> Degraded
            </button>
            <button
              class="filter-pill pill-offline"
              :class="{ active: statusFilter === 'offline' }"
              @click="statusFilter = 'offline'"
            >
              <span class="status-dot dot-rose"></span> Offline
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Docker Swarm Cluster Banner (When Active & Filtered) -->
    <FleetSwarmBanner v-if="showSwarmBanner && swarmInfo" :swarm-info="swarmInfo" />

    <!-- Desktop Kubernetes Fleet Section (Single viewMode: Table OR Grid, NEVER both) -->
    <div v-if="showK8sClusters" class="desktop-only">
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
        :total-clusters-count="clusters.length"
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
    <div v-if="showK8sClusters" class="mobile-only">
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
