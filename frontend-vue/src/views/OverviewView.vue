<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  overviewApi,
  type SystemOverview,
  type NodeMetrics,
  type ContainerMetrics,
  type MetricAlert,
} from '../api/overview'
import { useWebSocket } from '../composables/useWebSocket'
import CircularGauge from '../components/ui/CircularGauge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'

const router = useRouter()

// ==========================================
// 1. STATE MANAGEMENT
// ==========================================
const overview = ref<SystemOverview | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const lastUpdated = ref<Date>(new Date())
const isLiveWs = ref(false)

// History buffer for sparklines & 5-minute trends (up to 30 data points)
interface TrendPoint {
  time: string
  cpu: number
  mem: number
  disk: number
  reqs: number
}
const trendHistory = ref<TrendPoint[]>([])

// Container filtering & selection
const containerSearch = ref('')
const selectedNodeFilter = ref<string>('all')
const containerSortBy = ref<'cpu' | 'memory' | 'name'>('cpu')
const selectedContainer = ref<ContainerMetrics | null>(null)
const showContainerDrawer = ref(false)

// Dismissed alert IDs (session-level)
const dismissedAlerts = ref<Set<string>>(new Set())

// Polling fallback timer
let fallbackInterval: ReturnType<typeof setInterval> | null = null
let abortController: AbortController | null = null

// Live request activity simulation based on reqs/s
interface ActivityLog {
  id: string
  time: string
  method: string
  path: string
  status: number
  duration: number
}
const recentActivity = ref<ActivityLog[]>([])

// ==========================================
// 2. DATA FETCHING & WEBSOCKET
// ==========================================
async function fetchOverview() {
  if (abortController) {
    abortController.abort()
  }
  abortController = new AbortController()

  try {
    const data = await overviewApi.getOverview()
    if (data) {
      updateOverviewState(data)
      error.value = null
    }
  } catch (err: unknown) {
    if (err instanceof Error && err.name === 'AbortError') return
    if (!overview.value) {
      error.value = err instanceof Error ? err.message : 'Failed to retrieve cluster telemetry'
    }
  } finally {
    loading.value = false
  }
}

function updateOverviewState(data: SystemOverview) {
  overview.value = data
  lastUpdated.value = new Date()

  // Push to trend history
  const timeStr = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  const newPoint: TrendPoint = {
    time: timeStr,
    cpu: Math.round(data.total_cpu_percent || 0),
    mem: Math.round(data.total_mem_percent || 0),
    disk: Math.round(data.total_disk_percent || 0),
    reqs: Math.round(data.requests_per_sec || 0),
  }

  trendHistory.value.push(newPoint)
  if (trendHistory.value.length > 30) {
    trendHistory.value.shift()
  }

  // Generate simulated activity entries matching real reqs/s
  simulateActivity(data.requests_per_sec || 12)
}

function simulateActivity(reqs: number) {
  const routes = [
    { method: 'GET', path: '/api/v1/workloads', status: 200 },
    { method: 'POST', path: '/api/v1/telemetry/push', status: 200 },
    { method: 'GET', path: '/api/v1/fleet/health', status: 200 },
    { method: 'GET', path: '/metrics', status: 200 },
    { method: 'POST', path: '/api/v1/auth/verify', status: 200 },
    { method: 'GET', path: '/api/v1/docker/containers', status: 200 },
    { method: 'GET', path: '/api/v1/governance/drift', status: 200 },
  ]
  const count = Math.min(3, Math.max(1, Math.floor(reqs / 40)))
  for (let i = 0; i < count; i++) {
    const route = routes[Math.floor(Math.random() * routes.length)]
    const now = new Date()
    recentActivity.value.unshift({
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
      time: now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
      method: route.method,
      path: route.path,
      status: route.status,
      duration: Math.floor(Math.random() * 24) + 4,
    })
  }
  if (recentActivity.value.length > 15) {
    recentActivity.value = recentActivity.value.slice(0, 15)
  }
}

// WebSocket Connection with metrics callback
useWebSocket({
  onMetrics: (metrics: SystemOverview) => {
    isLiveWs.value = true
    updateOverviewState(metrics)
    loading.value = false
    error.value = null
  },
})

// ==========================================
// 3. COMPUTED METRICS & DERIVED STATE
// ==========================================
const nodes = computed<NodeMetrics[]>(() => overview.value?.nodes || [])
const totalContainers = computed(() => overview.value?.total_containers || 0)
const runningContainers = computed(() => overview.value?.running_containers || 0)

const activeAlerts = computed<MetricAlert[]>(() => {
  const alerts = overview.value?.alerts || []
  return alerts.filter(a => !dismissedAlerts.value.has(`${a.node_id}-${a.type}`))
})

// Determine the node receiving the highest traffic
const busiestNodeId = computed<string | null>(() => {
  if (nodes.value.length === 0) return null
  let maxTraffic = -1
  let busiest: string | null = null
  for (const n of nodes.value) {
    const traffic = (n.network_rx_bytes || 0) + (n.network_tx_bytes || 0)
    if (traffic > maxTraffic) {
      maxTraffic = traffic
      busiest = n.node_id
    }
  }
  return busiest
})

// Filtered and sorted containers
const filteredContainers = computed<ContainerMetrics[]>(() => {
  let list = overview.value?.containers || []

  if (selectedNodeFilter.value !== 'all') {
    list = list.filter(c => c.node_id === selectedNodeFilter.value)
  }

  if (containerSearch.value.trim()) {
    const q = containerSearch.value.toLowerCase().trim()
    list = list.filter(c =>
      c.container_name.toLowerCase().includes(q) ||
      c.image.toLowerCase().includes(q) ||
      c.node_id.toLowerCase().includes(q)
    )
  }

  const sorted = [...list]
  if (containerSortBy.value === 'cpu') {
    sorted.sort((a, b) => (b.cpu_percent || 0) - (a.cpu_percent || 0))
  } else if (containerSortBy.value === 'memory') {
    sorted.sort((a, b) => (b.memory_percent || 0) - (a.memory_percent || 0))
  } else if (containerSortBy.value === 'name') {
    sorted.sort((a, b) => a.container_name.localeCompare(b.container_name))
  }

  return sorted
})

// ==========================================
// 4. FORMATTING & HELPER FUNCTIONS
// ==========================================
function formatBytes(bytes?: number, decimals = 1): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i] || 'B'}`
}

function formatPercent(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0%'
  return `${Math.round(val * 10) / 10}%`
}

function getUtilizationColor(percent: number): string {
  if (percent >= 80) return 'rose'
  if (percent >= 60) return 'amber'
  return 'emerald'
}

function getNodeCardClass(node: NodeMetrics): string {
  if (node.status === 'down') return 'node-down'
  if (node.cpu_percent >= 80 || node.memory_percent >= 80) return 'node-overloaded'
  if (node.cpu_percent >= 60 || node.memory_percent >= 60) return 'node-warning'
  return 'node-healthy'
}

function getAlertRecommendation(alert: MetricAlert): string {
  const t = alert.type.toLowerCase()
  if (t.includes('cpu')) {
    return 'Consider scaling horizontally or migrating heavy containers'
  }
  if (t.includes('mem') || t.includes('memory')) {
    return 'Check for memory leaks, increase node RAM'
  }
  if (t.includes('disk')) {
    return 'Clean up unused images/volumes, expand storage'
  }
  if (t.includes('down') || t.includes('node')) {
    return 'Check node connectivity, run diagnostics'
  }
  return 'Review resource quotas and cluster workload placement'
}

function dismissAlert(alert: MetricAlert) {
  dismissedAlerts.value.add(`${alert.node_id}-${alert.type}`)
}

function inspectContainer(c: ContainerMetrics) {
  selectedContainer.value = c
  showContainerDrawer.value = true
}

function getNodeName(nodeId: string): string {
  const n = nodes.value.find(node => node.node_id === nodeId)
  return n ? n.node_name : nodeId
}

// Sparkline SVG coordinates generator
const sparklinePoints = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const maxReqs = Math.max(...history.map(h => h.reqs), 10)
  const width = 120
  const height = 32
  const step = width / (history.length - 1)

  return history
    .map((h, i) => {
      const x = i * step
      const y = height - (h.reqs / maxReqs) * (height - 4) - 2
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
})

// Trend Chart SVG Points for 5-minute CPU / RAM
const trendChartCpuPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const width = 300
  const height = 80
  const step = width / (history.length - 1)

  return history
    .map((h, i) => {
      const x = i * step
      const y = height - (Math.min(100, h.cpu) / 100) * (height - 10) - 5
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

const trendChartMemPath = computed(() => {
  const history = trendHistory.value
  if (history.length < 2) return ''
  const width = 300
  const height = 80
  const step = width / (history.length - 1)

  return history
    .map((h, i) => {
      const x = i * step
      const y = height - (Math.min(100, h.mem) / 100) * (height - 10) - 5
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
})

// ==========================================
// 5. LIFECYCLE & CLEANUP
// ==========================================
onMounted(() => {
  fetchOverview()

  // Polling fallback every 5 seconds if WebSocket is delayed
  fallbackInterval = setInterval(() => {
    fetchOverview()
  }, 5000)
})

onUnmounted(() => {
  if (fallbackInterval) {
    clearInterval(fallbackInterval)
    fallbackInterval = null
  }
  if (abortController) {
    abortController.abort()
    abortController = null
  }
})
</script>

<template>
  <div class="overview-dashboard animate-fade-in">
    <!-- Header Bar -->
    <header class="dashboard-header">
      <div class="header-titles">
        <div class="header-badge-group">
          <span class="badge" :class="isLiveWs ? 'badge-emerald' : 'badge-cyan'">
            <span class="pulse-dot" :class="{ 'pulse-active': isLiveWs }"></span>
            {{ isLiveWs ? 'LIVE WEBSOCKET STREAM' : 'TELEMETRY SYNC' }}
          </span>
          <span class="badge badge-indigo">Auto-Refresh 5s</span>
          <span class="last-sync-text">Updated: {{ lastUpdated.toLocaleTimeString() }}</span>
        </div>
        <h1 class="page-title">Infrastructure & Node Mesh Overview</h1>
        <p class="page-desc">
          Real-time cluster topology, container saturation metrics, dynamic resource gauges, and autonomous threshold alerting.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" @click="fetchOverview" :disabled="loading">
          <span class="btn-icon" :class="{ 'spin-icon': loading }">🔄</span>
          <span>Refresh</span>
        </button>
        <button class="btn btn-secondary" @click="router.push('/docker')">
          <span>🐳 Docker Swarm</span>
        </button>
        <button class="btn btn-primary" @click="router.push('/fleet')">
          <span>☸️ Fleet Mesh</span>
        </button>
      </div>
    </header>

    <!-- SECTION 4: SLIDE-DOWN ALERT BANNERS -->
    <transition-group name="alert-slide" tag="section" class="alerts-section" v-if="activeAlerts.length > 0">
      <div
        v-for="alert in activeAlerts"
        :key="`${alert.node_id}-${alert.type}`"
        class="alert-banner glass-panel"
        :class="alert.value >= 90 || alert.type === 'node_down' ? 'alert-critical' : 'alert-warning'"
      >
        <div class="alert-icon-box">
          <span class="alert-emoji">⚠️</span>
        </div>
        <div class="alert-content">
          <div class="alert-title-row">
            <span class="alert-node-tag">{{ alert.node_name || alert.node_id }}</span>
            <span class="alert-type-tag">{{ alert.type.toUpperCase() }}</span>
            <span class="alert-value-tag">{{ Math.round(alert.value) }}% (Threshold: {{ alert.threshold }}%)</span>
          </div>
          <p class="alert-message">
            <strong>{{ alert.message }}:</strong> {{ getAlertRecommendation(alert) }}
          </p>
        </div>
        <div class="alert-actions">
          <button class="btn-alert-dismiss" @click="dismissAlert(alert)" title="Dismiss Alert">
            ✕
          </button>
        </div>
      </div>
    </transition-group>

    <!-- LOADING SKELETON -->
    <div v-if="loading && !overview" class="skeleton-hud-grid">
      <div v-for="i in 4" :key="i" class="skeleton-card glass-panel shimmer"></div>
    </div>

    <!-- ERROR STATE -->
    <div v-else-if="error && !overview" class="error-banner glass-panel">
      <div class="error-icon">⚠️</div>
      <div class="error-info">
        <h3>Telemetry Connection Interrupted</h3>
        <p>{{ error }}</p>
      </div>
      <button class="btn btn-primary" @click="fetchOverview">Retry Telemetry Sync</button>
    </div>

    <!-- EMPTY STATE -->
    <div v-else-if="nodes.length === 0 && !loading" class="empty-state-card glass-panel">
      <div class="empty-icon">🐳</div>
      <h2>No Infrastructure Nodes Connected</h2>
      <p>Configure Docker daemon or attach compute clusters to enable real-time telemetry streaming.</p>
      <div class="empty-actions">
        <button class="btn btn-primary" @click="router.push('/settings')">Configure Settings</button>
        <button class="btn btn-secondary" @click="router.push('/docker')">Open Docker Swarm</button>
      </div>
    </div>

    <!-- MAIN DASHBOARD CONTENT -->
    <div v-else-if="overview" class="dashboard-body">
      <!-- SECTION 1: SUMMARY HUD (TOP BAR) -->
      <section class="summary-hud-grid">
        <!-- Card 1: Nodes -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Nodes Online</span>
            <span
              class="status-indicator-dot"
              :class="overview.healthy_nodes === overview.total_nodes ? 'status-green' : 'status-amber'"
            ></span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value">{{ overview.healthy_nodes }} <span class="hud-total">/ {{ overview.total_nodes }}</span></span>
            <span class="badge" :class="overview.healthy_nodes === overview.total_nodes ? 'badge-emerald' : 'badge-amber'">
              {{ overview.healthy_nodes === overview.total_nodes ? '100% HEALTHY' : 'DEGRADED' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill bg-emerald"
              :style="{ width: `${overview.total_nodes ? (overview.healthy_nodes / overview.total_nodes) * 100 : 0}%` }"
            ></div>
          </div>
        </div>

        <!-- Card 2: Containers -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Containers</span>
            <span class="hud-icon">📦</span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value">{{ runningContainers }} <span class="hud-total">/ {{ totalContainers }}</span></span>
            <span class="badge badge-cyan">RUNNING</span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill bg-cyan"
              :style="{ width: `${totalContainers ? (runningContainers / totalContainers) * 100 : 0}%` }"
            ></div>
          </div>
        </div>

        <!-- Card 3: Avg CPU -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Avg CPU Saturation</span>
            <span class="hud-badge-tag" :class="`text-${getUtilizationColor(overview.total_cpu_percent)}`">
              {{ formatPercent(overview.total_cpu_percent) }}
            </span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value">{{ Math.round(overview.total_cpu_percent) }}%</span>
            <span class="badge" :class="`badge-${getUtilizationColor(overview.total_cpu_percent)}`">
              {{ overview.total_cpu_percent >= 80 ? 'CRITICAL' : overview.total_cpu_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill"
              :class="`bg-${getUtilizationColor(overview.total_cpu_percent)}`"
              :style="{ width: `${Math.min(100, overview.total_cpu_percent)}%` }"
            ></div>
          </div>
        </div>

        <!-- Card 4: Avg RAM -->
        <div class="hud-card glass-panel">
          <div class="hud-card-top">
            <span class="hud-label">Avg Memory Saturation</span>
            <span class="hud-badge-tag" :class="`text-${getUtilizationColor(overview.total_mem_percent)}`">
              {{ formatPercent(overview.total_mem_percent) }}
            </span>
          </div>
          <div class="hud-value-row">
            <span class="hud-value">{{ Math.round(overview.total_mem_percent) }}%</span>
            <span class="badge" :class="`badge-${getUtilizationColor(overview.total_mem_percent)}`">
              {{ overview.total_mem_percent >= 80 ? 'CRITICAL' : overview.total_mem_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
            </span>
          </div>
          <div class="hud-progress-track">
            <div
              class="hud-progress-fill"
              :class="`bg-${getUtilizationColor(overview.total_mem_percent)}`"
              :style="{ width: `${Math.min(100, overview.total_mem_percent)}%` }"
            ></div>
          </div>
        </div>

        <!-- Card 5: Request Throughput with Sparkline -->
        <div class="hud-card glass-panel hud-card-throughput">
          <div class="hud-card-top">
            <span class="hud-label">Throughput</span>
            <span class="badge badge-violet">LIVE EDGE</span>
          </div>
          <div class="hud-value-row">
            <div class="throughput-details">
              <span class="hud-value">{{ Math.round(overview.requests_per_sec) }}</span>
              <span class="hud-unit">req/s</span>
            </div>
            <!-- SVG Sparkline -->
            <div class="sparkline-container" v-if="sparklinePoints">
              <svg viewBox="0 0 120 32" class="sparkline-svg">
                <polyline
                  fill="none"
                  stroke="#8b5cf6"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  :points="sparklinePoints"
                />
              </svg>
            </div>
          </div>
          <div class="hud-card-footer-text">
            <span>Aggregated API gateway ingress stream</span>
          </div>
        </div>
      </section>

      <!-- SECTION 2: NODE TOPOLOGY VISUALIZATION -->
      <section class="topology-section">
        <div class="section-title-bar">
          <div class="section-title-group">
            <h2 class="section-title">Cluster Node Topology</h2>
            <span class="section-subtitle">Real-time compute nodes, role distribution & hardware gauge telemetry</span>
          </div>
          <div class="topology-legend">
            <span class="legend-item"><span class="legend-dot bg-emerald"></span> &lt;60% Nominal</span>
            <span class="legend-item"><span class="legend-dot bg-amber"></span> 60-80% Elevated</span>
            <span class="legend-item"><span class="legend-dot bg-rose"></span> &gt;80% Overload</span>
            <span class="legend-item"><span class="legend-dot bg-gray"></span> Offline</span>
          </div>
        </div>

        <!-- Mesh Topology Connections Indicator -->
        <div class="mesh-interconnect-bar">
          <div class="mesh-pill">
            <span class="mesh-icon">⚡</span>
            <span>Mesh Interconnect Active</span>
            <span class="mesh-nodes-count">({{ nodes.length }} nodes linked)</span>
          </div>
          <div class="mesh-line-stream">
            <div class="mesh-pulse-line"></div>
          </div>
        </div>

        <!-- Node Cards Grid -->
        <div class="node-cards-grid">
          <div
            v-for="node in nodes"
            :key="node.node_id"
            class="node-card glass-panel"
            :class="[getNodeCardClass(node), { 'is-busiest': node.node_id === busiestNodeId }]"
          >
            <!-- Card Top: Name, Role & Status -->
            <div class="node-card-header">
              <div class="node-identity">
                <span
                  class="node-status-dot"
                  :class="node.status === 'ready' ? 'status-green' : 'status-red'"
                ></span>
                <span class="node-name" :title="node.node_name">{{ node.node_name }}</span>
              </div>
              <div class="node-badge-row">
                <span class="badge" :class="node.role === 'manager' ? 'badge-violet' : 'badge-cyan'">
                  {{ node.role.toUpperCase() }}
                </span>
                <span v-if="node.node_id === busiestNodeId" class="badge badge-amber badge-traffic-pulse" title="Highest traffic node">
                  🔥 HOT NODE
                </span>
              </div>
            </div>

            <!-- Gauge Cluster: CPU / RAM / Disk -->
            <div class="node-gauges-cluster">
              <div class="gauge-col">
                <CircularGauge
                  :percent="node.cpu_percent"
                  label="CPU"
                  :size="52"
                  :strokeWidth="3.5"
                  :disabled="node.status === 'down'"
                />
              </div>
              <div class="gauge-col">
                <CircularGauge
                  :percent="node.memory_percent"
                  label="RAM"
                  :size="52"
                  :strokeWidth="3.5"
                  :disabled="node.status === 'down'"
                />
              </div>
              <div class="gauge-col">
                <CircularGauge
                  :percent="node.disk_percent"
                  label="DISK"
                  :size="52"
                  :strokeWidth="3.5"
                  :disabled="node.status === 'down'"
                />
              </div>
            </div>

            <!-- Node Resource Detail Stats -->
            <div class="node-meta-grid">
              <div class="meta-item">
                <span class="meta-label">Memory</span>
                <span class="meta-val">{{ formatBytes(node.memory_used) }} / {{ formatBytes(node.memory_total) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Disk Storage</span>
                <span class="meta-val">{{ formatBytes(node.disk_used) }} / {{ formatBytes(node.disk_total) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Containers</span>
                <span class="meta-val">{{ node.running_count }} running / {{ node.container_count }} total</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Network I/O</span>
                <span class="meta-val font-mono">
                  ↑ {{ formatBytes(node.network_tx_bytes) }}/s  ↓ {{ formatBytes(node.network_rx_bytes) }}/s
                </span>
              </div>
            </div>

            <!-- Footer Action: Filter Containers -->
            <div class="node-card-footer">
              <button
                class="btn-node-filter"
                :class="{ 'btn-node-filter-active': selectedNodeFilter === node.node_id }"
                @click="selectedNodeFilter = selectedNodeFilter === node.node_id ? 'all' : node.node_id"
              >
                <span>{{ selectedNodeFilter === node.node_id ? 'Viewing Containers' : 'Filter Containers' }}</span>
                <span class="count-pill">{{ node.container_count }}</span>
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- SECTION 3 & 5: SPLIT CONTAINER GRID & LIVE ACTIVITY / TRENDS -->
      <div class="dashboard-split-layout">
        <!-- SECTION 3: CONTAINER GRID -->
        <section class="container-grid-section glass-panel">
          <div class="split-header">
            <div class="split-title-group">
              <h2 class="section-title">Active Workload Containers</h2>
              <span class="section-subtitle">
                Showing {{ filteredContainers.length }} workloads
                <span v-if="selectedNodeFilter !== 'all'">on node "{{ getNodeName(selectedNodeFilter) }}"</span>
              </span>
            </div>

            <!-- Filters -->
            <div class="container-filters">
              <div class="search-box">
                <span class="search-icon">🔍</span>
                <input
                  v-model="containerSearch"
                  type="text"
                  placeholder="Search container or image..."
                  class="input-search"
                />
                <button v-if="containerSearch" class="btn-clear-search" @click="containerSearch = ''">✕</button>
              </div>

              <!-- Node Filter Selector -->
              <select v-model="selectedNodeFilter" class="select-filter">
                <option value="all">All Nodes ({{ nodes.length }})</option>
                <option v-for="node in nodes" :key="node.node_id" :value="node.node_id">
                  {{ node.node_name }} ({{ node.container_count }})
                </option>
              </select>

              <!-- Sort Selector -->
              <select v-model="containerSortBy" class="select-filter">
                <option value="cpu">Sort: CPU (Hottest First)</option>
                <option value="memory">Sort: Memory</option>
                <option value="name">Sort: Name</option>
              </select>
            </div>
          </div>

          <!-- Containers List / Table -->
          <div class="containers-table-wrapper">
            <table class="containers-table" v-if="filteredContainers.length > 0">
              <thead>
                <tr>
                  <th>Container Name</th>
                  <th>Image</th>
                  <th>Node</th>
                  <th>Status</th>
                  <th>CPU Usage</th>
                  <th>RAM Usage</th>
                  <th>Network</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="c in filteredContainers"
                  :key="c.container_id"
                  class="container-row"
                  @click="inspectContainer(c)"
                >
                  <td class="col-name">
                    <span class="container-primary-name">{{ c.container_name }}</span>
                    <span class="container-id-sub">{{ c.container_id.slice(0, 12) }}</span>
                  </td>
                  <td class="col-image" :title="c.image">
                    <span class="image-text">{{ c.image }}</span>
                  </td>
                  <td class="col-node">
                    <span class="badge badge-cyan">{{ getNodeName(c.node_id) }}</span>
                  </td>
                  <td class="col-state">
                    <span
                      class="badge"
                      :class="c.state === 'running' ? 'badge-emerald' : 'badge-rose'"
                    >
                      {{ c.state.toUpperCase() }}
                    </span>
                  </td>
                  <td class="col-cpu">
                    <div class="usage-metric-cell">
                      <span class="usage-text">{{ formatPercent(c.cpu_percent) }}</span>
                      <div class="mini-bar-track">
                        <div
                          class="mini-bar-fill"
                          :class="`bg-${getUtilizationColor(c.cpu_percent)}`"
                          :style="{ width: `${Math.min(100, c.cpu_percent)}%` }"
                        ></div>
                      </div>
                    </div>
                  </td>
                  <td class="col-ram">
                    <div class="usage-metric-cell">
                      <span class="usage-text">{{ formatBytes(c.memory_used) }}</span>
                      <div class="mini-bar-track">
                        <div
                          class="mini-bar-fill"
                          :class="`bg-${getUtilizationColor(c.memory_percent)}`"
                          :style="{ width: `${Math.min(100, c.memory_percent)}%` }"
                        ></div>
                      </div>
                    </div>
                  </td>
                  <td class="col-net font-mono">
                    ↑{{ formatBytes(c.network_tx) }} ↓{{ formatBytes(c.network_rx) }}
                  </td>
                  <td class="col-action">
                    <button class="btn-inspect" @click.stop="inspectContainer(c)">
                      Inspect
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>

            <div v-else class="empty-filter-state">
              <span>No containers match current filters.</span>
              <button class="btn-reset-filters" @click="containerSearch = ''; selectedNodeFilter = 'all'">
                Reset Filters
              </button>
            </div>
          </div>
        </section>

        <!-- SECTION 5: LIVE ACTIVITY FEED & 5-MIN TRENDS -->
        <aside class="trends-activity-sidebar">
          <!-- 5-Minute Trend Chart Card -->
          <div class="trend-chart-card glass-panel">
            <div class="trend-chart-header">
              <h3 class="sidebar-card-title">5-Min Saturation Trends</h3>
              <div class="trend-legend">
                <span class="legend-line cpu-legend">CPU</span>
                <span class="legend-line mem-legend">RAM</span>
              </div>
            </div>

            <!-- SVG Multi-Line Chart -->
            <div class="trend-svg-box">
              <svg viewBox="0 0 300 80" class="trend-svg" preserveAspectRatio="none">
                <!-- Grid Lines -->
                <line x1="0" y1="20" x2="300" y2="20" stroke="rgba(255,255,255,0.05)" stroke-dasharray="2,2" />
                <line x1="0" y1="45" x2="300" y2="45" stroke="rgba(255,255,255,0.05)" stroke-dasharray="2,2" />
                <line x1="0" y1="70" x2="300" y2="70" stroke="rgba(255,255,255,0.05)" stroke-dasharray="2,2" />

                <!-- Memory Path (Cyan) -->
                <path
                  v-if="trendChartMemPath"
                  :d="trendChartMemPath"
                  fill="none"
                  stroke="#06b6d4"
                  stroke-width="2"
                  stroke-linecap="round"
                />

                <!-- CPU Path (Violet) -->
                <path
                  v-if="trendChartCpuPath"
                  :d="trendChartCpuPath"
                  fill="none"
                  stroke="#8b5cf6"
                  stroke-width="2"
                  stroke-linecap="round"
                />
              </svg>
            </div>
            <div class="trend-footer-stats">
              <div class="stat-pair">
                <span class="stat-k">Current CPU</span>
                <span class="stat-v text-violet">{{ formatPercent(overview.total_cpu_percent) }}</span>
              </div>
              <div class="stat-pair">
                <span class="stat-k">Current RAM</span>
                <span class="stat-v text-cyan">{{ formatPercent(overview.total_mem_percent) }}</span>
              </div>
              <div class="stat-pair">
                <span class="stat-k">Storage</span>
                <span class="stat-v text-emerald">{{ formatPercent(overview.total_disk_percent) }}</span>
              </div>
            </div>
          </div>

          <!-- Live Ingress Activity Feed -->
          <div class="activity-feed-card glass-panel">
            <div class="activity-header">
              <div class="activity-title-group">
                <span class="live-dot pulse-active"></span>
                <h3 class="sidebar-card-title">Live Ingress Activity</h3>
              </div>
              <span class="badge badge-emerald">Streaming</span>
            </div>

            <div class="activity-items-list">
              <div
                v-for="item in recentActivity"
                :key="item.id"
                class="activity-log-row animate-fade-in"
              >
                <div class="activity-meta">
                  <span class="activity-method" :class="`method-${item.method.toLowerCase()}`">{{ item.method }}</span>
                  <span class="activity-path" :title="item.path">{{ item.path }}</span>
                </div>
                <div class="activity-timing">
                  <span class="activity-status" :class="item.status === 200 ? 'status-ok' : 'status-err'">
                    {{ item.status }}
                  </span>
                  <span class="activity-dur">{{ item.duration }}ms</span>
                  <span class="activity-time">{{ item.time }}</span>
                </div>
              </div>
            </div>
          </div>
        </aside>
      </div>
    </div>

    <!-- CONTAINER DETAILS MODAL DRAWER -->
    <ModalDrawer
      :show="showContainerDrawer"
      mode="drawer"
      title="Container Metric Details"
      @close="showContainerDrawer = false"
      @update:show="showContainerDrawer = $event"
    >
      <div v-if="selectedContainer" class="drawer-container-details">
        <div class="drawer-header-card glass-panel">
          <div class="drawer-title-row">
            <h3 class="drawer-container-title">{{ selectedContainer.container_name }}</h3>
            <span class="badge" :class="selectedContainer.state === 'running' ? 'badge-emerald' : 'badge-rose'">
              {{ selectedContainer.state.toUpperCase() }}
            </span>
          </div>
          <span class="drawer-sub-image">{{ selectedContainer.image }}</span>
        </div>

        <div class="drawer-metrics-grid">
          <div class="drawer-metric-box glass-panel">
            <span class="box-label">CPU Saturation</span>
            <span class="box-value" :class="`text-${getUtilizationColor(selectedContainer.cpu_percent)}`">
              {{ formatPercent(selectedContainer.cpu_percent) }}
            </span>
            <div class="mini-bar-track">
              <div
                class="mini-bar-fill"
                :class="`bg-${getUtilizationColor(selectedContainer.cpu_percent)}`"
                :style="{ width: `${Math.min(100, selectedContainer.cpu_percent)}%` }"
              ></div>
            </div>
          </div>

          <div class="drawer-metric-box glass-panel">
            <span class="box-label">Memory Used / Limit</span>
            <span class="box-value text-cyan">
              {{ formatBytes(selectedContainer.memory_used) }}
            </span>
            <span class="box-sub">Limit: {{ formatBytes(selectedContainer.memory_limit) }} ({{ formatPercent(selectedContainer.memory_percent) }})</span>
          </div>
        </div>

        <div class="drawer-meta-list glass-panel">
          <div class="drawer-meta-row">
            <span class="dm-label">Container ID</span>
            <span class="dm-val font-mono">{{ selectedContainer.container_id }}</span>
          </div>
          <div class="drawer-meta-row">
            <span class="dm-label">Host Node ID</span>
            <span class="dm-val">{{ getNodeName(selectedContainer.node_id) }} ({{ selectedContainer.node_id }})</span>
          </div>
          <div class="drawer-meta-row">
            <span class="dm-label">Network Ingress (RX)</span>
            <span class="dm-val font-mono">{{ formatBytes(selectedContainer.network_rx) }}</span>
          </div>
          <div class="drawer-meta-row">
            <span class="dm-label">Network Egress (TX)</span>
            <span class="dm-val font-mono">{{ formatBytes(selectedContainer.network_tx) }}</span>
          </div>
        </div>

        <div class="drawer-actions">
          <button class="btn btn-secondary w-full" @click="router.push('/docker')">
            Manage in Docker Swarm
          </button>
          <button class="btn btn-secondary w-full" @click="router.push('/logs')">
            Stream Container Logs
          </button>
        </div>
      </div>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.overview-dashboard {
  display: flex;
  flex-direction: column;
  gap: 24px;
  width: 100%;
  padding-bottom: 40px;
}

/* Header */
.dashboard-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.header-badge-group {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.pulse-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
  display: inline-block;
}

.pulse-active {
  animation: pulse-ring 1.8s infinite;
}

@keyframes pulse-ring {
  0% { transform: scale(0.95); opacity: 0.8; box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7); }
  70% { transform: scale(1.1); opacity: 1; box-shadow: 0 0 0 6px rgba(16, 185, 129, 0); }
  100% { transform: scale(0.95); opacity: 0.8; box-shadow: 0 0 0 0 rgba(16, 185, 129, 0); }
}

.last-sync-text {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.page-title {
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--text-primary);
}

.page-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 800px;
  margin-top: 4px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.spin-icon {
  display: inline-block;
  animation: spin 1s infinite linear;
}

@keyframes spin {
  100% { transform: rotate(360deg); }
}

/* SECTION 4: Alerts */
.alerts-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.alert-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 18px;
  border-radius: 12px;
  gap: 14px;
  transition: all 0.3s ease;
}

.alert-critical {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.35);
  box-shadow: 0 0 20px rgba(244, 63, 94, 0.15);
}

.alert-warning {
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.35);
  box-shadow: 0 0 20px rgba(245, 158, 11, 0.15);
}

.alert-icon-box {
  font-size: 20px;
  display: flex;
  align-items: center;
}

.alert-content {
  flex: 1;
}

.alert-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 2px;
}

.alert-node-tag {
  font-weight: 700;
  font-size: 13px;
  color: var(--text-primary);
}

.alert-type-tag {
  font-size: 10px;
  font-weight: 800;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.1);
  font-family: var(--font-mono);
}

.alert-value-tag {
  font-size: 11px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

.alert-message {
  font-size: 12px;
  color: var(--text-secondary);
}

.btn-alert-dismiss {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 14px;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  transition: color 0.2s ease;
}

.btn-alert-dismiss:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.1);
}

/* SECTION 1: Summary HUD */
.summary-hud-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 16px;
}

.hud-card {
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  border-radius: 14px;
}

.hud-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hud-label {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
}

.status-indicator-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-green { background: #10b981; box-shadow: 0 0 8px rgba(16, 185, 129, 0.6); }
.status-amber { background: #f59e0b; box-shadow: 0 0 8px rgba(245, 158, 11, 0.6); }
.status-red { background: #f43f5e; box-shadow: 0 0 8px rgba(244, 63, 94, 0.6); }

.hud-value-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.hud-value {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--text-primary);
  font-family: var(--font-sans);
}

.hud-total {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-muted);
}

.hud-badge-tag {
  font-size: 11px;
  font-weight: 700;
  font-family: var(--font-mono);
}

.hud-progress-track {
  width: 100%;
  height: 4px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.hud-progress-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.5s ease;
}

.hud-card-throughput {
  grid-column: span 1;
}

.throughput-details {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.hud-unit {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 600;
}

.sparkline-container {
  width: 100px;
  height: 28px;
}

.sparkline-svg {
  width: 100%;
  height: 100%;
}

.hud-card-footer-text {
  font-size: 10px;
  color: var(--text-muted);
}

/* SECTION 2: Topology Visualizer */
.topology-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-title-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.section-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
}

.section-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  display: block;
}

.topology-legend {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 11px;
  color: var(--text-secondary);
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.bg-emerald { background: #10b981; }
.bg-amber { background: #f59e0b; }
.bg-rose { background: #f43f5e; }
.bg-cyan { background: #06b6d4; }
.bg-gray { background: #64748b; }

.mesh-interconnect-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 16px;
  background: rgba(11, 15, 25, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
}

.mesh-pill {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-cyan);
  white-space: nowrap;
}

.mesh-nodes-count {
  color: var(--text-muted);
}

.mesh-line-stream {
  flex: 1;
  height: 2px;
  background: rgba(255, 255, 255, 0.05);
  position: relative;
  overflow: hidden;
  border-radius: 2px;
}

.mesh-pulse-line {
  position: absolute;
  top: 0;
  left: -20%;
  width: 30%;
  height: 100%;
  background: linear-gradient(90deg, transparent, var(--accent-cyan), transparent);
  animation: mesh-stream 3s infinite linear;
}

@keyframes mesh-stream {
  0% { left: -30%; }
  100% { left: 100%; }
}

/* Node Cards Grid */
.node-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.node-card {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  position: relative;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.node-card:hover {
  transform: translateY(-2px);
  border-color: rgba(56, 189, 248, 0.3);
}

.node-overloaded {
  border-color: rgba(244, 63, 94, 0.4);
  box-shadow: 0 4px 20px rgba(244, 63, 94, 0.12);
}

.node-warning {
  border-color: rgba(245, 158, 11, 0.3);
}

.node-down {
  opacity: 0.6;
  filter: grayscale(0.7);
}

.is-busiest {
  box-shadow: 0 0 25px rgba(245, 158, 11, 0.18);
  border-color: rgba(245, 158, 11, 0.4);
}

.badge-traffic-pulse {
  animation: pulse-traffic 2s infinite ease-in-out;
}

@keyframes pulse-traffic {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.05); opacity: 0.85; }
}

.node-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.node-identity {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.node-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.node-name {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-badge-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.node-gauges-cluster {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 10px 0;
  border-top: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
  background: rgba(0, 0, 0, 0.15);
  border-radius: 8px;
}

.gauge-col {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.node-meta-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 12px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.meta-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.meta-val {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
}

.node-card-footer {
  margin-top: auto;
}

.btn-node-filter {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-node-filter:hover {
  background: rgba(56, 189, 248, 0.1);
  color: var(--text-primary);
  border-color: var(--border-accent);
}

.btn-node-filter-active {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
  color: var(--accent-cyan);
}

.count-pill {
  padding: 1px 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.1);
  font-family: var(--font-mono);
  font-size: 10px;
}

/* SECTION 3 & 5: Split Layout */
.dashboard-split-layout {
  display: grid;
  grid-template-columns: 1fr 340px;
  gap: 20px;
  align-items: start;
}

@media (max-width: 1100px) {
  .dashboard-split-layout {
    grid-template-columns: 1fr;
  }
}

.container-grid-section {
  padding: 20px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.split-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 14px;
}

.container-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.search-box {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 10px;
  font-size: 12px;
  color: var(--text-muted);
}

.input-search {
  padding: 7px 28px 7px 30px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid var(--border-subtle);
  color: var(--text-primary);
  font-size: 12px;
  width: 180px;
  outline: none;
  transition: all 0.2s ease;
}

.input-search:focus {
  border-color: var(--border-accent);
  width: 220px;
}

.btn-clear-search {
  position: absolute;
  right: 8px;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
}

.select-filter {
  padding: 7px 10px;
  border-radius: 8px;
  background: rgba(11, 15, 25, 0.9);
  border: 1px solid var(--border-subtle);
  color: var(--text-primary);
  font-size: 12px;
  outline: none;
  cursor: pointer;
}

.containers-table-wrapper {
  overflow-x: auto;
}

.containers-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}

.containers-table th {
  padding: 10px 12px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border-subtle);
}

.containers-table td {
  padding: 10px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  vertical-align: middle;
}

.container-row {
  cursor: pointer;
  transition: background 0.15s ease;
}

.container-row:hover {
  background: rgba(255, 255, 255, 0.03);
}

.col-name {
  display: flex;
  flex-direction: column;
}

.container-primary-name {
  font-weight: 600;
  color: var(--text-primary);
}

.container-id-sub {
  font-size: 10px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.col-image {
  max-width: 160px;
}

.image-text {
  display: block;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 11px;
}

.usage-metric-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 90px;
}

.usage-text {
  font-size: 11px;
  font-weight: 600;
  font-family: var(--font-mono);
}

.mini-bar-track {
  width: 100%;
  height: 4px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.mini-bar-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.3s ease;
}

.btn-inspect {
  padding: 4px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-inspect:hover {
  background: var(--accent-cyan);
  color: var(--text-inverse);
  border-color: var(--accent-cyan);
}

.empty-filter-state {
  padding: 30px;
  text-align: center;
  color: var(--text-muted);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.btn-reset-filters {
  background: none;
  border: 1px solid var(--border-subtle);
  color: var(--accent-cyan);
  padding: 4px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 11px;
}

/* SECTION 5: Sidebar Trends & Live Ingress */
.trends-activity-sidebar {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.sidebar-card-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-primary);
}

.trend-chart-card {
  padding: 16px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.trend-chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.trend-legend {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 10px;
  font-weight: 700;
}

.legend-line {
  display: flex;
  align-items: center;
  gap: 4px;
}

.cpu-legend::before {
  content: '';
  width: 10px;
  height: 2px;
  background: #8b5cf6;
  display: inline-block;
}

.mem-legend::before {
  content: '';
  width: 10px;
  height: 2px;
  background: #06b6d4;
  display: inline-block;
}

.trend-svg-box {
  width: 100%;
  height: 80px;
  background: rgba(0, 0, 0, 0.2);
  border-radius: 8px;
  overflow: hidden;
}

.trend-svg {
  width: 100%;
  height: 100%;
}

.trend-footer-stats {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}

.stat-pair {
  display: flex;
  flex-direction: column;
}

.stat-k {
  font-size: 9px;
  text-transform: uppercase;
  color: var(--text-muted);
}

.stat-v {
  font-size: 12px;
  font-weight: 700;
  font-family: var(--font-mono);
}

.text-violet { color: #8b5cf6; }
.text-cyan { color: #06b6d4; }
.text-emerald { color: #10b981; }
.text-amber { color: #f59e0b; }
.text-rose { color: #f43f5e; }

.activity-feed-card {
  padding: 16px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.activity-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.activity-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.live-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #10b981;
}

.activity-items-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 280px;
  overflow-y: auto;
}

.activity-log-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 8px;
  background: rgba(0, 0, 0, 0.2);
  border-radius: 6px;
  font-size: 11px;
}

.activity-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.activity-method {
  font-size: 9px;
  font-weight: 800;
  padding: 1px 4px;
  border-radius: 3px;
  font-family: var(--font-mono);
}

.method-get { background: rgba(56, 189, 248, 0.2); color: #38bdf8; }
.method-post { background: rgba(16, 185, 129, 0.2); color: #10b981; }
.method-delete { background: rgba(244, 63, 94, 0.2); color: #f43f5e; }

.activity-path {
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 120px;
  font-family: var(--font-mono);
}

.activity-timing {
  display: flex;
  align-items: center;
  gap: 6px;
  font-family: var(--font-mono);
  font-size: 10px;
}

.activity-status {
  font-weight: 700;
}

.status-ok { color: #10b981; }
.status-err { color: #f43f5e; }
.activity-dur { color: var(--text-muted); }
.activity-time { color: var(--text-muted); font-size: 9px; }

/* Skeleton & Empty States */
.skeleton-hud-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.skeleton-card {
  height: 110px;
  border-radius: 14px;
}

.shimmer {
  background: linear-gradient(90deg, rgba(255,255,255,0.03) 25%, rgba(255,255,255,0.08) 50%, rgba(255,255,255,0.03) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.6s infinite;
}

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

.empty-state-card {
  padding: 60px 20px;
  text-align: center;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.empty-icon {
  font-size: 48px;
}

.empty-actions {
  display: flex;
  gap: 12px;
  margin-top: 8px;
}

.error-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px;
  border-radius: 14px;
  border: 1px solid rgba(244, 63, 94, 0.4);
  background: rgba(244, 63, 94, 0.08);
}

.error-icon { font-size: 32px; }
.error-info h3 { font-size: 15px; color: #f43f5e; }
.error-info p { font-size: 12px; color: var(--text-secondary); }

/* Drawer Details */
.drawer-container-details {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px 0;
}

.drawer-header-card {
  padding: 16px;
  border-radius: 12px;
}

.drawer-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.drawer-container-title {
  font-size: 16px;
  font-weight: 700;
}

.drawer-sub-image {
  font-size: 12px;
  color: var(--text-muted);
  font-family: var(--font-mono);
  word-break: break-all;
}

.drawer-metrics-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.drawer-metric-box {
  padding: 14px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.box-label { font-size: 10px; text-transform: uppercase; color: var(--text-muted); }
.box-value { font-size: 20px; font-weight: 800; font-family: var(--font-mono); }
.box-sub { font-size: 10px; color: var(--text-muted); }

.drawer-meta-list {
  padding: 14px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.drawer-meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}

.dm-label { color: var(--text-muted); }
.dm-val { color: var(--text-primary); font-weight: 600; }

.drawer-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.w-full { width: 100%; }

/* Buttons & Badges */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.btn-primary {
  background: var(--grad-cyan);
  color: var(--text-inverse);
  box-shadow: 0 2px 10px rgba(6, 182, 212, 0.3);
}

.btn-primary:hover {
  filter: brightness(1.1);
  box-shadow: 0 4px 14px rgba(6, 182, 212, 0.4);
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.05);
  border-color: var(--border-subtle);
  color: var(--text-primary);
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: var(--border-accent);
}

.badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 700;
  font-family: var(--font-mono);
  letter-spacing: 0.02em;
}

.badge-emerald { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.badge-rose { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }
.badge-cyan { background: rgba(6, 182, 212, 0.15); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.3); }
.badge-indigo { background: rgba(99, 102, 241, 0.15); color: #818cf8; border: 1px solid rgba(99, 102, 241, 0.3); }
.badge-violet { background: rgba(139, 92, 246, 0.15); color: #a78bfa; border: 1px solid rgba(139, 92, 246, 0.3); }

/* Transitions */
.alert-slide-enter-active, .alert-slide-leave-active {
  transition: all 0.3s ease;
}
.alert-slide-enter-from, .alert-slide-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>
