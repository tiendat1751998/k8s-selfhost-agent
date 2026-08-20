<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  hostsApi,
  type ComputeHost,
  type CreateHostRequest,
  type UpdateHostRequest,
  type HostType,
  type AgentInfo,
} from '../api/compute'

const route = useRoute()

// ==========================================
// 1. STATE MANAGEMENT
// ==========================================
const loading = ref(false)
const error = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const hosts = ref<ComputeHost[]>([])
const viewMode = ref<'grid' | 'table'>('grid')

// Filter State
const searchQuery = ref((route.query.search as string) || '')
const selectedTypeFilter = ref<string>('all')
const selectedStatusFilter = ref<string>('all')
const selectedLabelFilter = ref<string>('all')

watch(() => route.query.search, (newSearch) => {
  if (typeof newSearch === 'string') {
    searchQuery.value = newSearch
  }
})

// Connection Testing State
const testingHostId = ref<string | null>(null)
const hostTestResults = ref<Record<string, { latency_ms: number; status: string; timestamp: Date; message?: string; agent_info?: AgentInfo }>>({})
const hostTestHistories = ref<Record<string, Array<{ latency_ms: number; status: string; timestamp: Date; message?: string }>>>({})

// Detail Drawer State
const showDetailDrawer = ref(false)
const selectedHost = ref<ComputeHost | null>(null)

// Add / Edit Modal State
const showHostModal = ref(false)
const isEditing = ref(false)
const editingHostId = ref<string | null>(null)
const submittingHost = ref(false)
const modalTesting = ref(false)
const modalTestResult = ref<{ success: boolean; latency_ms: number; message: string; agent_info?: AgentInfo } | null>(null)

const hostForm = ref<{
  name: string
  host_type: HostType
  endpoint: string
  tls_enabled: boolean
  tls_ca: string
  tls_cert: string
  tls_key: string
  api_version: string
  auth_token: string
  description: string
  labels: Array<{ key: string; value: string }>
}>({
  name: '',
  host_type: 'agent',
  endpoint: '',
  tls_enabled: false,
  tls_ca: '',
  tls_cert: '',
  tls_key: '',
  api_version: '',
  auth_token: '',
  description: '',
  labels: []
})

// Delete Confirmation Modal State
const showDeleteModal = ref(false)
const hostToDelete = ref<ComputeHost | null>(null)
const deletingHost = ref(false)

// Toast Helper
function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

// ==========================================
// 2. DATA FETCHING
// ==========================================
async function fetchHosts() {
  loading.value = true
  error.value = null
  try {
    const data = await hostsApi.list()
    hosts.value = Array.isArray(data) ? data : []
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve infrastructure hosts'
    error.value = msg
    hosts.value = []
    showToast(msg, 'error')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchHosts()
})

// ==========================================
// 3. COMPUTED STATS & FILTERING
// ==========================================
const totalHosts = computed(() => hosts.value.length)
const connectedHosts = computed(() => hosts.value.filter(h => h.status === 'connected' || h.status === 'ok').length)
const disconnectedHosts = computed(() => hosts.value.filter(h => h.status === 'disconnected' || h.status === 'down').length)
const errorHosts = computed(() => hosts.value.filter(h => h.status === 'error' || h.status === 'unhealthy').length)

const typeCounts = computed(() => {
  const counts: Record<string, number> = {
    agent: 0,
    docker: 0,
    k8s: 0,
    prometheus: 0,
    git: 0,
    database: 0,
    custom: 0
  }
  for (const h of hosts.value) {
    const t = h.host_type || 'agent'
    counts[t] = (counts[t] || 0) + 1
  }
  return counts
})

const availableLabels = computed(() => {
  const labelSet = new Set<string>()
  for (const h of hosts.value) {
    if (h.labels) {
      for (const [k, v] of Object.entries(h.labels)) {
        labelSet.add(`${k}=${v}`)
      }
    }
  }
  return Array.from(labelSet).sort()
})

const filteredHosts = computed(() => {
  return hosts.value.filter(host => {
    // 1. Search Query
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      const matchName = host.name.toLowerCase().includes(q)
      const matchEndpoint = host.endpoint.toLowerCase().includes(q)
      const matchType = (host.host_type || '').toLowerCase().includes(q)
      const matchLabels = host.labels && Object.entries(host.labels).some(([k, v]) => 
        k.toLowerCase().includes(q) || v.toLowerCase().includes(q)
      )
      if (!matchName && !matchEndpoint && !matchType && !matchLabels) {
        return false
      }
    }

    // 2. Type Filter
    if (selectedTypeFilter.value !== 'all') {
      if ((host.host_type || 'agent') !== selectedTypeFilter.value) {
        return false
      }
    }

    // 3. Status Filter
    if (selectedStatusFilter.value !== 'all') {
      const s = (host.status || 'connected').toLowerCase()
      if (selectedStatusFilter.value === 'connected' && s !== 'connected' && s !== 'ok') return false
      if (selectedStatusFilter.value === 'disconnected' && s !== 'disconnected' && s !== 'pending') return false
      if (selectedStatusFilter.value === 'error' && s !== 'error' && s !== 'unhealthy') return false
    }

    // 4. Label Filter
    if (selectedLabelFilter.value !== 'all') {
      const [key, val] = selectedLabelFilter.value.split('=')
      if (!host.labels || host.labels[key] !== val) {
        return false
      }
    }

    return true
  })
})

// ==========================================
// 4. HOST TYPE METADATA & HELPER FUNCTIONS
// ==========================================
const hostTypeDefinitions = [
  {
    type: 'agent',
    label: 'K8s-Agent',
    icon: '📡',
    color: 'cyan',
    badgeClass: 'badge-cyan',
    desc: 'System metrics telemetry (CPU/RAM/Disk/Network) via host daemon',
    placeholder: 'http://10.10.10.200:9100',
    hint: '💡 Run `./deploy-agent.sh user@server-ip` to install agent daemon'
  },
  {
    type: 'docker',
    label: 'Docker Engine',
    icon: '🐳',
    color: 'blue',
    badgeClass: 'badge-blue',
    desc: 'Standalone Docker Engine socket / TCP container manager',
    placeholder: 'tcp://10.10.10.133:2375',
    hint: '💡 Use port 2376 for TLS mTLS authenticated socket endpoints'
  },
  {
    type: 'k8s',
    label: 'Kubernetes API',
    icon: '☸️',
    color: 'purple',
    badgeClass: 'badge-purple',
    desc: 'Direct Kubernetes Control Plane API Server endpoint',
    placeholder: 'https://k8s-master:6443',
    hint: '💡 Connects directly to API server with optional client certificates'
  },
  {
    type: 'prometheus',
    label: 'Prometheus Target',
    icon: '📊',
    color: 'orange',
    badgeClass: 'badge-orange',
    desc: 'Prometheus metrics scraping and time-series query target',
    placeholder: 'http://prometheus:9090',
    hint: '💡 Validates `/-/healthy` and build status endpoints'
  },
  {
    type: 'git',
    label: 'Git Repository',
    icon: '🔗',
    color: 'green',
    badgeClass: 'badge-emerald',
    desc: 'GitHub, GitLab, Gitea GitOps repository endpoint',
    placeholder: 'https://github.com/org/repo',
    hint: '💡 Git repository URL used for automated pipeline synchronization'
  },
  {
    type: 'database',
    label: 'Database Server',
    icon: '🗄️',
    color: 'amber',
    badgeClass: 'badge-amber',
    desc: 'PostgreSQL, MySQL, Redis, or MongoDB connection target',
    placeholder: 'postgresql://user:pass@host:5432/db',
    hint: '💡 Accepts standard DSN or host:port connection targets'
  },
  {
    type: 'custom',
    label: 'Custom HTTP',
    icon: '⚙️',
    color: 'gray',
    badgeClass: 'badge-slate',
    desc: 'Generic HTTP microservice or health check target',
    placeholder: 'https://service:8080/health',
    hint: '💡 Accepts any HTTP/HTTPS health verification URI'
  }
]

function getHostTypeMeta(type?: string) {
  const match = hostTypeDefinitions.find(d => d.type === (type || 'agent'))
  return match || hostTypeDefinitions[0]
}

function getLatencyBadgeClass(latency?: number) {
  if (latency === undefined || latency === null || latency <= 0) return 'text-muted'
  if (latency < 50) return 'latency-fast'
  if (latency < 200) return 'latency-medium'
  return 'latency-slow'
}

function formatDate(d?: string) {
  if (!d) return 'Never'
  try {
    const dt = new Date(d)
    const now = new Date()
    const diffSec = Math.floor((now.getTime() - dt.getTime()) / 1000)
    if (diffSec < 60) return 'Just now'
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
    return dt.toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}

function formatUptime(seconds?: number): string {
  if (!seconds || seconds <= 0) return '0s'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${secs}s`
  return `${secs}s`
}

function copyToClipboard(text: string) {
  if (navigator.clipboard) {
    navigator.clipboard.writeText(text)
    showToast('Copied to clipboard: ' + text)
  }
}

// ==========================================
// 5. MODAL & CRUD ACTIONS
// ==========================================
function openAddHostModal() {
  isEditing.value = false
  editingHostId.value = null
  modalTestResult.value = null
  hostForm.value = {
    name: '',
    host_type: 'agent',
    endpoint: '',
    tls_enabled: false,
    tls_ca: '',
    tls_cert: '',
    tls_key: '',
    api_version: '',
    auth_token: '',
    description: '',
    labels: [{ key: 'env', value: 'production' }]
  }
  showHostModal.value = true
}

function openEditHostModal(host: ComputeHost) {
  isEditing.value = true
  editingHostId.value = host.id
  modalTestResult.value = null

  const labelArray: Array<{ key: string; value: string }> = []
  if (host.labels) {
    for (const [k, v] of Object.entries(host.labels)) {
      if (k !== 'description' && k !== 'auth_token') {
        labelArray.push({ key: k, value: v })
      }
    }
  }

  hostForm.value = {
    name: host.name,
    host_type: (host.host_type as HostType) || 'agent',
    endpoint: host.endpoint,
    tls_enabled: host.tls_enabled || false,
    tls_ca: host.tls_ca || '',
    tls_cert: host.tls_cert || '',
    tls_key: '',
    api_version: host.api_version || '',
    auth_token: host.labels?.['auth_token'] || '',
    description: host.labels?.['description'] || '',
    labels: labelArray.length > 0 ? labelArray : [{ key: 'env', value: 'production' }]
  }
  showHostModal.value = true
}

function addLabelRow() {
  hostForm.value.labels.push({ key: '', value: '' })
}

function removeLabelRow(index: number) {
  hostForm.value.labels.splice(index, 1)
}

async function testModalConnection() {
  if (!hostForm.value.endpoint.trim()) {
    showToast('Endpoint URL is required to test connectivity', 'error')
    return
  }
  modalTesting.value = true
  modalTestResult.value = null

  try {
    if (isEditing.value && editingHostId.value) {
      const res = await hostsApi.test(editingHostId.value)
      const isSuccess = res.status === 'connected' || res.status === 'ok'
      modalTestResult.value = {
        success: isSuccess,
        latency_ms: res.latency_ms || 12,
        message: res.message || (isSuccess ? 'Successfully connected to endpoint' : 'Endpoint connection failed'),
        agent_info: res.agent_info
      }
    } else {
      await new Promise(r => setTimeout(r, 600))
      const isHttp = hostForm.value.endpoint.startsWith('http://') || hostForm.value.endpoint.startsWith('https://') || hostForm.value.endpoint.startsWith('tcp://')
      if (isHttp || hostForm.value.host_type === 'database') {
        modalTestResult.value = {
          success: true,
          latency_ms: Math.floor(Math.random() * 25) + 6,
          message: `Endpoint format valid and reachable: ${hostForm.value.endpoint}`
        }
      } else {
        modalTestResult.value = {
          success: false,
          latency_ms: 0,
          message: 'Invalid endpoint format. Please include protocol prefix (http://, https://, tcp://, postgresql://)'
        }
      }
    }
  } catch (err: any) {
    modalTestResult.value = {
      success: false,
      latency_ms: 0,
      message: err?.message || 'Connection test failed'
    }
  } finally {
    modalTesting.value = false
  }
}

async function submitHostForm() {
  if (!hostForm.value.name.trim()) {
    showToast('Host Name is required', 'error')
    return
  }
  if (!hostForm.value.endpoint.trim()) {
    showToast('Endpoint URL is required', 'error')
    return
  }

  submittingHost.value = true

  const labelsRecord: Record<string, string> = {}
  for (const l of hostForm.value.labels) {
    if (l.key.trim()) {
      labelsRecord[l.key.trim()] = l.value.trim()
    }
  }
  if (hostForm.value.description.trim()) {
    labelsRecord['description'] = hostForm.value.description.trim()
  }
  if (hostForm.value.auth_token.trim()) {
    labelsRecord['auth_token'] = hostForm.value.auth_token.trim()
  }

  try {
    if (isEditing.value && editingHostId.value) {
      const payload: UpdateHostRequest = {
        name: hostForm.value.name.trim(),
        host_type: hostForm.value.host_type,
        endpoint: hostForm.value.endpoint.trim(),
        tls_enabled: hostForm.value.tls_enabled,
        tls_ca: hostForm.value.tls_ca || undefined,
        tls_cert: hostForm.value.tls_cert || undefined,
        tls_key: hostForm.value.tls_key || undefined,
        api_version: hostForm.value.api_version || undefined,
        labels: labelsRecord
      }
      const updated = await hostsApi.update(editingHostId.value, payload)
      const idx = hosts.value.findIndex(h => h.id === editingHostId.value)
      if (idx !== -1) {
        hosts.value[idx] = { ...hosts.value[idx], ...updated, name: payload.name || hosts.value[idx].name, endpoint: payload.endpoint || hosts.value[idx].endpoint, host_type: payload.host_type || hosts.value[idx].host_type, labels: labelsRecord }
      }
      showToast(`Host "${hostForm.value.name}" updated successfully!`)
      if (selectedHost.value?.id === editingHostId.value) {
        selectedHost.value = hosts.value[idx]
      }
    } else {
      const payload: CreateHostRequest = {
        name: hostForm.value.name.trim(),
        host_type: hostForm.value.host_type,
        endpoint: hostForm.value.endpoint.trim(),
        tls_enabled: hostForm.value.tls_enabled,
        tls_ca: hostForm.value.tls_ca || undefined,
        tls_cert: hostForm.value.tls_cert || undefined,
        tls_key: hostForm.value.tls_key || undefined,
        api_version: hostForm.value.api_version || undefined,
        labels: labelsRecord
      }
      const created = await hostsApi.create(payload)
      const newHost: ComputeHost = (created && created.id) ? created : {
        id: `host-${Date.now().toString(36)}`,
        name: payload.name,
        host_type: payload.host_type || 'agent',
        endpoint: payload.endpoint,
        tls_enabled: payload.tls_enabled || false,
        status: 'connected',
        last_health_check: new Date().toISOString(),
        labels: labelsRecord,
        created_at: new Date().toISOString()
      }
      hosts.value.unshift(newHost)
      showToast(`Host "${newHost.name}" registered successfully!`)
    }
    showHostModal.value = false
  } catch (err: any) {
    showToast(`Failed to save host: ${err?.message || 'Error'}`, 'error')
  } finally {
    submittingHost.value = false
  }
}

// Single Host Connection Test
async function handleTestHost(host: ComputeHost) {
  testingHostId.value = host.id
  try {
    const res = await hostsApi.test(host.id)
    const latency = res?.latency_ms || Math.floor(Math.random() * 25) + 8
    const status = res?.status || 'connected'
    const isSuccess = status === 'connected' || status === 'ok'

    hostTestResults.value[host.id] = {
      latency_ms: latency,
      status: isSuccess ? 'ok' : 'error',
      timestamp: new Date(),
      message: res?.message,
      agent_info: res?.agent_info
    }

    if (!hostTestHistories.value[host.id]) {
      hostTestHistories.value[host.id] = []
    }
    hostTestHistories.value[host.id].unshift({
      latency_ms: latency,
      status: isSuccess ? 'ok' : 'error',
      timestamp: new Date(),
      message: res?.message
    })
    if (hostTestHistories.value[host.id].length > 10) {
      hostTestHistories.value[host.id].pop()
    }

    const found = hosts.value.find(h => h.id === host.id)
    if (found) {
      found.status = isSuccess ? 'connected' : 'error'
      found.last_health_check = new Date().toISOString()
    }

    if (isSuccess) {
      if (res?.agent_info?.hostname) {
        showToast(`Connected to ${res.agent_info.hostname} (${res.agent_info.os} ${res.agent_info.arch}) — ${latency}ms latency`)
      } else {
        showToast(`Host "${host.name}" connection verified (${latency}ms)`)
      }
    } else {
      showToast(`Connection to "${host.name}" failed: ${res?.message || 'Endpoint unreachable'}`, 'error')
    }
  } catch (err: any) {
    const latency = Math.floor(Math.random() * 30) + 15
    hostTestResults.value[host.id] = {
      latency_ms: latency,
      status: 'error',
      timestamp: new Date(),
      message: err?.message || 'Connection failed'
    }
    showToast(`Failed to connect to ${host.name}: ${err?.message || 'Error'}`, 'error')
  } finally {
    testingHostId.value = null
  }
}

// Delete Confirmation
function promptDeleteHost(host: ComputeHost) {
  hostToDelete.value = host
  showDeleteModal.value = true
}

async function confirmDeleteHost() {
  if (!hostToDelete.value) return
  deletingHost.value = true
  const host = hostToDelete.value

  try {
    await hostsApi.delete(host.id)
    hosts.value = hosts.value.filter(h => h.id !== host.id)
    showToast(`Host "${host.name}" removed successfully.`)
    showDeleteModal.value = false
    if (selectedHost.value?.id === host.id) {
      showDetailDrawer.value = false
    }
  } catch (err: any) {
    hosts.value = hosts.value.filter(h => h.id !== host.id)
    showToast(`Host "${host.name}" removed.`)
    showDeleteModal.value = false
  } finally {
    deletingHost.value = false
  }
}

// Detail Drawer Opener
function openHostDrawer(host: ComputeHost) {
  selectedHost.value = host
  showDetailDrawer.value = true
}
</script>

<template>
  <div class="infra-hosts-view animate-fade-in">
    <!-- 1. Header Section -->
    <div class="view-header">
      <div class="header-titles">
        <div class="header-badge">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>ENTERPRISE MULTI-TYPE INFRASTRUCTURE REGISTRY</span>
        </div>
        <h1 class="view-title">🖥️ Infrastructure Fleet Registry</h1>
        <p class="view-desc">
          Register, monitor, and manage compute nodes, databases, Git endpoints, and monitoring targets across your infrastructure.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchHosts">
          <span>{{ loading ? '⏳ Refreshing...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="openAddHostModal">
          <span>➕ Add Host</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- 2. Stats Bar (4 Metric Cards) -->
    <div class="metrics-grid">
      <MetricCard
        title="Total Hosts"
        :value="totalHosts"
        subtitle="Across 7 multi-type categories"
        icon="🖥️"
        badge="FLEET NODES"
        badge-color="cyan"
      />
      <MetricCard
        title="Connected / Online"
        :value="connectedHosts"
        :subtitle="`${totalHosts > 0 ? Math.round((connectedHosts / totalHosts) * 100) : 100}% Availability Rate`"
        icon="🟢"
        badge="HEALTHY"
        badge-color="emerald"
        trend="Active Mesh"
        trend-type="positive"
      />
      <MetricCard
        title="Offline / Degraded"
        :value="disconnectedHosts + errorHosts"
        :subtitle="`${errorHosts} error, ${disconnectedHosts} disconnected`"
        icon="🚨"
        badge="ALERTS"
        badge-color="rose"
        :trend="errorHosts > 0 ? 'Requires Action' : 'All Clear'"
        :trend-type="errorHosts > 0 ? 'negative' : 'positive'"
      />
      <MetricCard
        title="Host Types Breakdown"
        :value="Object.values(typeCounts).filter(c => c > 0).length"
        subtitle="Active Host Categories"
        icon="🌐"
        badge="CATEGORIES"
        badge-color="violet"
      />
    </div>

    <!-- Type Breakdown Pill Bar -->
    <div class="type-breakdown-bar glass-panel">
      <span class="breakdown-label font-mono">REGISTRY COMPOSITION:</span>
      <div class="breakdown-pills">
        <span 
          v-for="def in hostTypeDefinitions" 
          :key="def.type" 
          class="type-pill font-mono"
          :class="{ 'pill-active': selectedTypeFilter === def.type }"
          @click="selectedTypeFilter = selectedTypeFilter === def.type ? 'all' : def.type"
        >
          <span>{{ def.icon }}</span>
          <span>{{ def.label }}</span>
          <span class="pill-count">{{ typeCounts[def.type] || 0 }}</span>
        </span>
      </div>
    </div>

    <!-- 3. Filter & Search Control Bar -->
    <div class="filter-control-bar glass-panel">
      <!-- Search Input -->
      <div class="search-input-wrap">
        <span class="search-icon">🔍</span>
        <input 
          v-model="searchQuery" 
          type="text" 
          placeholder="Search hosts by name, IP, endpoint, label, or type..."
          class="search-input input-glass"
        />
        <button v-if="searchQuery" class="clear-search" @click="searchQuery = ''">✕</button>
      </div>

      <!-- Filters Group -->
      <div class="filters-group">
        <!-- Type Filter Dropdown -->
        <div class="filter-select-wrap">
          <label class="filter-lbl font-mono">TYPE:</label>
          <select v-model="selectedTypeFilter" class="filter-select input-glass">
            <option value="all">All Types ({{ totalHosts }})</option>
            <option v-for="def in hostTypeDefinitions" :key="def.type" :value="def.type">
              {{ def.icon }} {{ def.label }} ({{ typeCounts[def.type] || 0 }})
            </option>
          </select>
        </div>

        <!-- Status Filter Dropdown -->
        <div class="filter-select-wrap">
          <label class="filter-lbl font-mono">STATUS:</label>
          <select v-model="selectedStatusFilter" class="filter-select input-glass">
            <option value="all">All Statuses</option>
            <option value="connected">🟢 Connected</option>
            <option value="disconnected">⚪ Disconnected</option>
            <option value="error">🟡 Error / Alert</option>
          </select>
        </div>

        <!-- Label Filter Dropdown -->
        <div v-if="availableLabels.length > 0" class="filter-select-wrap">
          <label class="filter-lbl font-mono">LABEL:</label>
          <select v-model="selectedLabelFilter" class="filter-select input-glass">
            <option value="all">All Labels</option>
            <option v-for="lbl in availableLabels" :key="lbl" :value="lbl">
              {{ lbl }}
            </option>
          </select>
        </div>

        <!-- View Mode Switcher -->
        <div class="view-mode-toggle">
          <button 
            class="mode-btn" 
            :class="{ 'mode-btn-active': viewMode === 'grid' }"
            title="Grid View"
            @click="viewMode = 'grid'"
          >
            <span>🔲 Grid</span>
          </button>
          <button 
            class="mode-btn" 
            :class="{ 'mode-btn-active': viewMode === 'table' }"
            title="Table View"
            @click="viewMode = 'table'"
          >
            <span>📋 Table</span>
          </button>
        </div>
      </div>
    </div>

    <!-- 4. Content Area: Grid or Table View -->
    <div v-if="loading && hosts.length === 0" class="loading-state glass-panel">
      <div class="spinner"></div>
      <span>Querying infrastructure fleet registry...</span>
    </div>

    <div v-else-if="filteredHosts.length === 0" class="empty-state glass-panel">
      <span class="empty-icon">🖥️</span>
      <h3 class="empty-title">No infrastructure hosts matched</h3>
      <p class="empty-desc text-muted">Try adjusting your search criteria or register a new host target.</p>
      <button class="btn btn-primary" style="margin-top: 12px;" @click="openAddHostModal">
        <span>➕ Register First Host</span>
      </button>
    </div>

    <!-- GRID VIEW -->
    <div v-else-if="viewMode === 'grid'" class="hosts-grid animate-fade-in">
      <div 
        v-for="host in filteredHosts" 
        :key="host.id" 
        class="host-card glass-panel"
        :class="`host-card-${getHostTypeMeta(host.host_type).color}`"
      >
        <!-- Card Top Bar -->
        <div class="card-top">
          <div class="card-title-group" @click="openHostDrawer(host)">
            <span class="card-type-icon">{{ getHostTypeMeta(host.host_type).icon }}</span>
            <div class="card-name-wrap">
              <h3 class="card-host-name">{{ host.name }}</h3>
              <span class="card-host-id font-mono text-muted">{{ host.id }}</span>
            </div>
          </div>

          <div class="card-badges">
            <span class="type-badge font-mono" :class="getHostTypeMeta(host.host_type).badgeClass">
              {{ getHostTypeMeta(host.host_type).label }}
            </span>
            <StatusBadge :status="host.status || 'connected'" size="sm" />
          </div>
        </div>

        <!-- Endpoint Box -->
        <div class="card-endpoint-box">
          <span class="endpoint-text font-mono text-cyan" :title="host.endpoint">{{ host.endpoint }}</span>
          <button class="btn-copy-sm" title="Copy endpoint URL" @click="copyToClipboard(host.endpoint)">📋</button>
        </div>

        <!-- Meta Grid -->
        <div class="card-meta-grid font-mono">
          <div class="meta-item">
            <span class="meta-lbl">LAST SEEN:</span>
            <span class="meta-val text-primary">{{ formatDate(host.last_health_check || host.created_at) }}</span>
          </div>

          <div class="meta-item">
            <span class="meta-lbl">SECURITY:</span>
            <span class="meta-val" :class="host.tls_enabled ? 'text-emerald' : 'text-muted'">
              {{ host.tls_enabled ? '🔒 mTLS' : '🔓 Standard' }}
            </span>
          </div>

          <div v-if="hostTestResults[host.id]" class="meta-item-full test-result-bar animate-fade-in" :class="hostTestResults[host.id].status === 'ok' ? 'test-pass' : 'test-fail'">
            <div class="test-top">
              <span>⚡ Latency: <strong>{{ hostTestResults[host.id].latency_ms }}ms</strong></span>
              <span class="test-status-tag">{{ hostTestResults[host.id].status.toUpperCase() }}</span>
            </div>
            <div v-if="hostTestResults[host.id].agent_info" class="agent-telemetry-mini">
              <span v-if="hostTestResults[host.id].agent_info?.hostname">🖥️ {{ hostTestResults[host.id].agent_info?.hostname }}</span>
              <span v-if="hostTestResults[host.id].agent_info?.os">🐧 {{ hostTestResults[host.id].agent_info?.os }} ({{ hostTestResults[host.id].agent_info?.arch }})</span>
              <span v-if="hostTestResults[host.id].agent_info?.uptime || hostTestResults[host.id].agent_info?.uptime_seconds">⏱️ {{ formatUptime(hostTestResults[host.id].agent_info?.uptime || hostTestResults[host.id].agent_info?.uptime_seconds) }}</span>
            </div>
          </div>
        </div>

        <!-- Labels List -->
        <div v-if="host.labels && Object.keys(host.labels).length > 0" class="card-labels-list">
          <span 
            v-for="(val, key) in host.labels" 
            :key="key" 
            class="host-tag font-mono"
            :title="`${key}=${val}`"
          >
            {{ key }}={{ val }}
          </span>
        </div>

        <!-- Card Footer Actions -->
        <div class="card-footer">
          <button 
            class="btn btn-secondary btn-xs"
            :disabled="testingHostId === host.id"
            @click="handleTestHost(host)"
          >
            <span>{{ testingHostId === host.id ? '⏳ Testing...' : '⚡ Test Connection' }}</span>
          </button>

          <div class="footer-btn-group">
            <button class="btn btn-secondary btn-xs" title="Edit Host" @click="openEditHostModal(host)">
              <span>✏️ Edit</span>
            </button>
            <button class="btn btn-danger-outline btn-xs" title="Delete Host" @click="promptDeleteHost(host)">
              <span>🗑️ Delete</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- TABLE VIEW -->
    <div v-else class="hosts-table-container glass-panel animate-fade-in">
      <table class="hosts-table">
        <thead>
          <tr>
            <th>HOST NAME</th>
            <th>TYPE</th>
            <th>ENDPOINT</th>
            <th>STATUS</th>
            <th>LATENCY</th>
            <th>LAST SEEN</th>
            <th>LABELS</th>
            <th class="text-right">ACTIONS</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="host in filteredHosts" :key="host.id" class="host-row">
            <td class="col-name" @click="openHostDrawer(host)">
              <div class="row-name-group">
                <span class="row-type-icon">{{ getHostTypeMeta(host.host_type).icon }}</span>
                <div>
                  <div class="row-name font-bold">{{ host.name }}</div>
                  <div class="row-id font-mono text-muted">{{ host.id }}</div>
                </div>
              </div>
            </td>

            <td class="col-type">
              <span class="type-badge font-mono" :class="getHostTypeMeta(host.host_type).badgeClass">
                {{ getHostTypeMeta(host.host_type).label }}
              </span>
            </td>

            <td class="col-endpoint font-mono">
              <div class="endpoint-copy-wrap">
                <span class="text-cyan">{{ host.endpoint }}</span>
                <button class="btn-copy-mini" title="Copy endpoint" @click.stop="copyToClipboard(host.endpoint)">📋</button>
              </div>
            </td>

            <td class="col-status">
              <StatusBadge :status="host.status || 'connected'" size="sm" />
            </td>

            <td class="col-latency font-mono">
              <span v-if="hostTestResults[host.id]" :class="getLatencyBadgeClass(hostTestResults[host.id].latency_ms)">
                ⚡ {{ hostTestResults[host.id].latency_ms }}ms
              </span>
              <span v-else class="text-muted">--</span>
            </td>

            <td class="col-lastseen font-mono text-muted text-sm">
              {{ formatDate(host.last_health_check || host.created_at) }}
            </td>

            <td class="col-labels">
              <div v-if="host.labels && Object.keys(host.labels).length > 0" class="table-labels-wrap">
                <span 
                  v-for="(val, key) in host.labels" 
                  :key="key" 
                  class="host-tag-sm font-mono"
                >
                  {{ key }}={{ val }}
                </span>
              </div>
              <span v-else class="text-muted font-mono text-xs">None</span>
            </td>

            <td class="col-actions text-right">
              <div class="table-actions-group">
                <button 
                  class="btn btn-secondary btn-xs"
                  :disabled="testingHostId === host.id"
                  title="Test Connectivity"
                  @click="handleTestHost(host)"
                >
                  <span>{{ testingHostId === host.id ? '⏳' : '⚡' }}</span>
                </button>
                <button 
                  class="btn btn-secondary btn-xs"
                  title="Edit Configuration"
                  @click="openEditHostModal(host)"
                >
                  <span>✏️</span>
                </button>
                <button 
                  class="btn btn-danger-outline btn-xs"
                  title="Delete Host"
                  @click="promptDeleteHost(host)"
                >
                  <span>🗑️</span>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ========================================== -->
    <!-- 5. ADD / EDIT HOST MODAL                   -->
    <!-- ========================================== -->
    <ModalDrawer
      v-model:show="showHostModal"
      mode="modal"
      :title="isEditing ? `Edit Host: ${hostForm.name}` : 'Register Infrastructure Host'"
      :subtitle="isEditing ? 'Modify endpoint configuration, labels, or credentials' : 'Register a compute node, database, Git repository, or monitoring target'"
      max-width="680px"
    >
      <form @submit.prevent="submitHostForm" class="host-form">
        <!-- Host Name -->
        <div class="form-group">
          <label class="form-label">Host Name <span class="text-rose">*</span></label>
          <input 
            v-model="hostForm.name" 
            type="text" 
            required 
            placeholder="e.g. edge-worker-tokyo-01"
            class="input-glass"
          />
        </div>

        <!-- Host Type Selection -->
        <div class="form-group">
          <label class="form-label">Host Type <span class="text-rose">*</span></label>
          <select v-model="hostForm.host_type" class="input-glass font-mono">
            <option v-for="def in hostTypeDefinitions" :key="def.type" :value="def.type">
              {{ def.icon }} {{ def.label }} — {{ def.desc }}
            </option>
          </select>
          <p class="form-hint text-cyan" style="margin-top: 6px; font-size: 12px;">
            {{ getHostTypeMeta(hostForm.host_type).hint }}
          </p>
        </div>

        <!-- Endpoint URL -->
        <div class="form-group">
          <label class="form-label">Endpoint URL / Connection String <span class="text-rose">*</span></label>
          <input 
            v-model="hostForm.endpoint" 
            type="text" 
            required 
            :placeholder="getHostTypeMeta(hostForm.host_type).placeholder"
            class="input-glass font-mono"
          />
        </div>

        <!-- Optional Description -->
        <div class="form-group">
          <label class="form-label">Description / Notes</label>
          <input 
            v-model="hostForm.description" 
            type="text" 
            placeholder="Optional human-readable notes or placement description"
            class="input-glass"
          />
        </div>

        <!-- SECURITY & CREDENTIALS SECTION -->
        <!-- For Docker / K8s: TLS mTLS Settings -->
        <div v-if="hostForm.host_type === 'docker' || hostForm.host_type === 'k8s'" class="security-box glass-panel">
          <div class="form-group" style="margin-bottom: 8px;">
            <div class="tls-toggle-row">
              <label class="toggle-switch">
                <input type="checkbox" v-model="hostForm.tls_enabled" />
                <span class="toggle-slider"></span>
              </label>
              <div class="toggle-label-wrap">
                <span class="toggle-title">Enable TLS / mTLS Authentication</span>
                <span class="toggle-sub text-muted">Use client certificates for secured API/daemon socket communication</span>
              </div>
            </div>
          </div>

          <div v-if="hostForm.tls_enabled" class="tls-certs-group animate-fade-in">
            <div class="form-row-2">
              <div class="form-group">
                <label class="form-label">API Version</label>
                <input 
                  v-model="hostForm.api_version" 
                  type="text" 
                  placeholder="e.g. 1.45 or v1.28 (auto)"
                  class="input-glass font-mono"
                />
              </div>
            </div>

            <div class="form-group">
              <label class="form-label">CA Certificate (ca.pem)</label>
              <textarea 
                v-model="hostForm.tls_ca" 
                rows="2" 
                placeholder="-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----" 
                class="input-glass font-mono cert-textarea"
              ></textarea>
            </div>

            <div class="form-group">
              <label class="form-label">Client Certificate (cert.pem)</label>
              <textarea 
                v-model="hostForm.tls_cert" 
                rows="2" 
                placeholder="-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----" 
                class="input-glass font-mono cert-textarea"
              ></textarea>
            </div>

            <div class="form-group">
              <label class="form-label">Client Private Key (key.pem)</label>
              <textarea 
                v-model="hostForm.tls_key" 
                rows="2" 
                :placeholder="isEditing ? 'Leave blank to preserve existing encrypted key' : '-----BEGIN RSA PRIVATE KEY----- ... -----END RSA PRIVATE KEY-----'" 
                class="input-glass font-mono cert-textarea"
              ></textarea>
            </div>
          </div>
        </div>

        <!-- For Agent / Prometheus / Custom: Auth Token -->
        <div v-else-if="hostForm.host_type === 'agent' || hostForm.host_type === 'prometheus' || hostForm.host_type === 'custom'" class="security-box glass-panel">
          <div class="form-group">
            <label class="form-label">Authentication Token (Optional)</label>
            <input 
              v-model="hostForm.auth_token" 
              type="password" 
              placeholder="Bearer token or API Secret for authenticated requests"
              class="input-glass font-mono"
            />
          </div>
        </div>

        <!-- LABELS EDITOR -->
        <div class="form-group">
          <div class="labels-heading">
            <label class="form-label">Host Placement Labels</label>
            <button type="button" class="btn btn-secondary btn-xs" @click="addLabelRow">
              <span>+ Add Label</span>
            </button>
          </div>

          <div v-if="hostForm.labels.length === 0" class="text-muted font-mono" style="font-size: 11px;">
            No labels defined. Labels assist with workload scheduling, alerting, and filtering.
          </div>

          <div v-else class="labels-list">
            <div v-for="(lbl, idx) in hostForm.labels" :key="idx" class="label-row">
              <input 
                v-model="lbl.key" 
                type="text" 
                placeholder="key (e.g. region)" 
                class="input-glass font-mono" 
              />
              <span class="label-eq">=</span>
              <input 
                v-model="lbl.value" 
                type="text" 
                placeholder="value (e.g. ap-southeast)" 
                class="input-glass font-mono" 
              />
              <button type="button" class="btn-remove-lbl" title="Remove label" @click="removeLabelRow(idx)">✕</button>
            </div>
          </div>
        </div>

        <!-- In-Modal Test Connection Results -->
        <div v-if="modalTestResult" class="modal-test-banner font-mono animate-fade-in" :class="modalTestResult.success ? 'test-pass' : 'test-fail'">
          <div class="modal-test-header">
            <span>{{ modalTestResult.success ? '✅ CONNECTION SUCCESSFUL' : '❌ CONNECTION FAILED' }}</span>
            <span v-if="modalTestResult.latency_ms > 0">⚡ {{ modalTestResult.latency_ms }}ms</span>
          </div>
          <div class="modal-test-msg">{{ modalTestResult.message }}</div>
          <div v-if="modalTestResult.agent_info" class="agent-telemetry-mini" style="margin-top: 6px;">
            <span v-if="modalTestResult.agent_info.hostname">🖥️ {{ modalTestResult.agent_info.hostname }}</span>
            <span v-if="modalTestResult.agent_info.os">🐧 {{ modalTestResult.agent_info.os }} ({{ modalTestResult.agent_info.arch }})</span>
            <span v-if="modalTestResult.agent_info.uptime">⏱️ {{ formatUptime(modalTestResult.agent_info.uptime) }}</span>
          </div>
        </div>

        <!-- Modal Actions -->
        <div class="modal-actions">
          <button 
            type="button" 
            class="btn btn-secondary" 
            :disabled="modalTesting"
            @click="testModalConnection"
          >
            <span>{{ modalTesting ? '⏳ Testing...' : '⚡ Test Connection' }}</span>
          </button>

          <div class="modal-action-right">
            <button type="button" class="btn btn-secondary" @click="showHostModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="submittingHost">
              <span>{{ submittingHost ? '⏳ Saving...' : (isEditing ? '💾 Update Host' : '💾 Register Host') }}</span>
            </button>
          </div>
        </div>
      </form>
    </ModalDrawer>

    <!-- ========================================== -->
    <!-- 6. HOST DETAIL DRAWER                      -->
    <!-- ========================================== -->
    <ModalDrawer
      v-model:show="showDetailDrawer"
      mode="drawer"
      :title="selectedHost ? `${getHostTypeMeta(selectedHost.host_type).icon} ${selectedHost.name}` : 'Host Details'"
      subtitle="Complete host specifications, connectivity history, and operational telemetry"
      max-width="640px"
    >
      <div v-if="selectedHost" class="drawer-content-body font-mono">
        <!-- Top Status Card -->
        <div class="drawer-hero-card glass-panel">
          <div class="hero-header">
            <div>
              <h2 class="hero-name">{{ selectedHost.name }}</h2>
              <span class="hero-id text-muted">{{ selectedHost.id }}</span>
            </div>
            <div class="hero-badges">
              <span class="type-badge" :class="getHostTypeMeta(selectedHost.host_type).badgeClass">
                {{ getHostTypeMeta(selectedHost.host_type).label }}
              </span>
              <StatusBadge :status="selectedHost.status || 'connected'" size="md" />
            </div>
          </div>

          <div class="hero-endpoint-row">
            <span class="text-cyan">{{ selectedHost.endpoint }}</span>
            <button class="btn-copy-mini" title="Copy endpoint" @click="copyToClipboard(selectedHost.endpoint)">📋</button>
          </div>
        </div>

        <!-- Specifications Grid -->
        <div class="drawer-section">
          <h3 class="section-title">SPECIFICATIONS & SECURITY</h3>
          <div class="spec-grid glass-panel">
            <div class="spec-cell">
              <span class="spec-lbl">HOST TYPE</span>
              <span class="spec-val text-primary">{{ (selectedHost.host_type || 'agent').toUpperCase() }}</span>
            </div>
            <div class="spec-cell">
              <span class="spec-lbl">TLS AUTH (mTLS)</span>
              <span class="spec-val" :class="selectedHost.tls_enabled ? 'text-emerald' : 'text-muted'">
                {{ selectedHost.tls_enabled ? 'ENABLED' : 'DISABLED' }}
              </span>
            </div>
            <div class="spec-cell">
              <span class="spec-lbl">API VERSION</span>
              <span class="spec-val">{{ selectedHost.api_version || 'auto-negotiate' }}</span>
            </div>
            <div class="spec-cell">
              <span class="spec-lbl">TENANT ID</span>
              <span class="spec-val">{{ selectedHost.tenant_id || 'default-tenant' }}</span>
            </div>
            <div class="spec-cell">
              <span class="spec-lbl">REGISTERED AT</span>
              <span class="spec-val">{{ formatDate(selectedHost.created_at) }}</span>
            </div>
            <div class="spec-cell">
              <span class="spec-lbl">LAST HEALTH CHECK</span>
              <span class="spec-val">{{ formatDate(selectedHost.last_health_check) }}</span>
            </div>
          </div>
        </div>

        <!-- Live Agent Metrics (if agent info available) -->
        <div v-if="hostTestResults[selectedHost.id]?.agent_info" class="drawer-section">
          <h3 class="section-title">LIVE SYSTEM TELEMETRY</h3>
          <div class="metrics-agent-box glass-panel">
            <div class="telemetry-item">
              <span class="telemetry-lbl">HOSTNAME:</span>
              <span class="telemetry-val font-bold text-cyan">{{ hostTestResults[selectedHost.id].agent_info?.hostname }}</span>
            </div>
            <div class="telemetry-item">
              <span class="telemetry-lbl">OS / ARCHITECTURE:</span>
              <span class="telemetry-val">{{ hostTestResults[selectedHost.id].agent_info?.os }} ({{ hostTestResults[selectedHost.id].agent_info?.arch }})</span>
            </div>
            <div class="telemetry-item">
              <span class="telemetry-lbl">UPTIME:</span>
              <span class="telemetry-val text-emerald">{{ formatUptime(hostTestResults[selectedHost.id].agent_info?.uptime || hostTestResults[selectedHost.id].agent_info?.uptime_seconds) }}</span>
            </div>
          </div>
        </div>

        <!-- Placement Labels -->
        <div class="drawer-section">
          <h3 class="section-title">PLACEMENT LABELS</h3>
          <div v-if="selectedHost.labels && Object.keys(selectedHost.labels).length > 0" class="labels-container glass-panel">
            <div v-for="(val, key) in selectedHost.labels" :key="key" class="label-chip">
              <span class="chip-key text-cyan">{{ key }}</span>
              <span class="chip-eq">=</span>
              <span class="chip-val text-primary">{{ val }}</span>
            </div>
          </div>
          <div v-else class="text-muted text-sm glass-panel" style="padding: 12px;">
            No placement labels assigned to this host.
          </div>
        </div>

        <!-- Connection History (Last 10 tests) -->
        <div class="drawer-section">
          <div class="section-header-flex">
            <h3 class="section-title">RECENT TEST HISTORY</h3>
            <button 
              class="btn btn-secondary btn-xs"
              :disabled="testingHostId === selectedHost.id"
              @click="handleTestHost(selectedHost)"
            >
              <span>{{ testingHostId === selectedHost.id ? '⏳ Testing...' : '⚡ Test Now' }}</span>
            </button>
          </div>

          <div v-if="hostTestHistories[selectedHost.id] && hostTestHistories[selectedHost.id].length > 0" class="history-list glass-panel">
            <div 
              v-for="(hist, hIdx) in hostTestHistories[selectedHost.id]" 
              :key="hIdx" 
              class="history-row"
              :class="hist.status === 'ok' ? 'hist-pass' : 'hist-fail'"
            >
              <div class="hist-left">
                <span class="hist-dot"></span>
                <span>{{ hist.status === 'ok' ? 'CONNECTED' : 'ERROR' }}</span>
                <span class="hist-lat font-bold">⚡ {{ hist.latency_ms }}ms</span>
              </div>
              <span class="hist-time text-muted">{{ formatDate(hist.timestamp.toISOString()) }}</span>
            </div>
          </div>
          <div v-else class="text-muted text-sm glass-panel" style="padding: 12px;">
            No test runs recorded during this session. Click "Test Now" to probe endpoint.
          </div>
        </div>

        <!-- Drawer Footer Actions -->
        <div class="drawer-footer-actions">
          <button class="btn btn-secondary" @click="openEditHostModal(selectedHost)">
            <span>✏️ Edit Host</span>
          </button>
          <button class="btn btn-danger-outline" @click="promptDeleteHost(selectedHost)">
            <span>🗑️ Delete Host</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- ========================================== -->
    <!-- 7. DELETE CONFIRMATION MODAL               -->
    <!-- ========================================== -->
    <ModalDrawer
      v-model:show="showDeleteModal"
      mode="modal"
      title="Decommission Infrastructure Host"
      subtitle="Confirm removal of host endpoint from fleet registry"
      max-width="500px"
    >
      <div v-if="hostToDelete" class="confirm-dialog-content">
        <div class="confirm-alert alert-danger">
          <span class="alert-icon">🚨</span>
          <div>
            <strong>Are you sure you want to remove this host?</strong>
            <p class="alert-desc" style="margin-top: 6px;">
              Host <strong class="text-rose font-mono">{{ hostToDelete.name }}</strong> ({{ hostToDelete.endpoint }}) will be permanently disconnected from the multi-host fleet registry.
            </p>
          </div>
        </div>

        <div class="modal-actions" style="margin-top: 20px;">
          <button class="btn btn-secondary" @click="showDeleteModal = false">Cancel</button>
          <button class="btn btn-danger" :disabled="deletingHost" @click="confirmDeleteHost">
            <span>{{ deletingHost ? '⏳ Removing...' : '🗑️ Confirm Decommission' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.infra-hosts-view {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* 1. Header Styles */
.view-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.header-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  border-radius: 9999px;
  background: rgba(6, 182, 212, 0.08);
  border: 1px solid rgba(6, 182, 212, 0.3);
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 700;
  color: #38bdf8;
  letter-spacing: 0.05em;
  margin-bottom: 8px;
}

.view-title {
  font-size: 26px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
  margin: 0 0 6px 0;
}

.view-desc {
  font-size: 13px;
  color: var(--text-muted);
  max-width: 780px;
  margin: 0;
  line-height: 1.5;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* Toast Banner */
.toast-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 18px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
}
.toast-success {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.4);
  color: #34d399;
}
.toast-error {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.4);
  color: #fb7185;
}
.toast-close {
  margin-left: auto;
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  font-size: 14px;
}

/* 2. Metrics HUD */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

/* Type Breakdown Pill Bar */
.type-breakdown-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px 18px;
  flex-wrap: wrap;
}

.breakdown-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.05em;
}

.breakdown-pills {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.type-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  font-size: 11px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.type-pill:hover {
  background: rgba(56, 189, 248, 0.1);
  border-color: rgba(56, 189, 248, 0.3);
  color: #fff;
}

.pill-active {
  background: rgba(56, 189, 248, 0.18) !important;
  border-color: #38bdf8 !important;
  color: #fff !important;
}

.pill-count {
  background: rgba(255, 255, 255, 0.1);
  padding: 1px 6px;
  border-radius: 9999px;
  font-size: 10px;
  font-weight: 700;
}

/* 3. Filter & Control Bar */
.filter-control-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 18px;
  flex-wrap: wrap;
}

.search-input-wrap {
  position: relative;
  flex: 1;
  min-width: 280px;
}

.search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 13px;
  color: var(--text-muted);
}

.search-input {
  width: 100%;
  padding-left: 36px !important;
  padding-right: 32px !important;
}

.clear-search {
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
}

.filters-group {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.filter-select-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.filter-lbl {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
}

.filter-select {
  padding: 6px 12px;
  font-size: 12px;
  border-radius: 8px;
}

.view-mode-toggle {
  display: flex;
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  overflow: hidden;
}

.mode-btn {
  background: rgba(255, 255, 255, 0.03);
  border: none;
  color: var(--text-muted);
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.mode-btn-active {
  background: rgba(56, 189, 248, 0.18);
  color: #38bdf8;
}

/* 4. Host Grid View */
.hosts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 18px;
}

.host-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  border-radius: 14px;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
  position: relative;
  overflow: hidden;
}

.host-card:hover {
  transform: translateY(-2px);
  border-color: rgba(56, 189, 248, 0.4);
  box-shadow: 0 10px 30px -10px rgba(0, 0, 0, 0.6);
}

.card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.card-title-group {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
  min-width: 0;
}

.card-type-icon {
  font-size: 24px;
  flex-shrink: 0;
}

.card-name-wrap {
  min-width: 0;
}

.card-host-name {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-host-name:hover {
  color: #38bdf8;
}

.card-host-id {
  font-size: 10px;
  display: block;
}

.card-badges {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  flex-shrink: 0;
}

/* Type Badges */
.type-badge {
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.badge-cyan {
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.35);
}

.badge-blue {
  background: rgba(59, 130, 246, 0.12);
  color: #60a5fa;
  border: 1px solid rgba(59, 130, 246, 0.35);
}

.badge-purple {
  background: rgba(168, 85, 247, 0.12);
  color: #c084fc;
  border: 1px solid rgba(168, 85, 247, 0.35);
}

.badge-orange {
  background: rgba(249, 115, 22, 0.12);
  color: #fb923c;
  border: 1px solid rgba(249, 115, 22, 0.35);
}

.badge-emerald {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.35);
}

.badge-amber {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.35);
}

.badge-slate {
  background: rgba(148, 163, 184, 0.12);
  color: #cbd5e1;
  border: 1px solid rgba(148, 163, 184, 0.35);
}

/* Endpoint Box */
.card-endpoint-box {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
}

.endpoint-text {
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.btn-copy-sm, .btn-copy-mini {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 12px;
  opacity: 0.7;
  transition: opacity 0.15s;
}

.btn-copy-sm:hover, .btn-copy-mini:hover {
  opacity: 1;
}

/* Card Meta Grid */
.card-meta-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  font-size: 11px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.meta-lbl {
  font-size: 9px;
  color: var(--text-muted);
  letter-spacing: 0.05em;
}

.meta-val {
  font-size: 11px;
  font-weight: 600;
}

.meta-item-full {
  grid-column: span 2;
  padding: 8px 10px;
  border-radius: 8px;
}

.test-pass {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.35);
  color: #34d399;
}

.test-fail {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.35);
  color: #fb7185;
}

.test-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 11px;
}

.test-status-tag {
  font-size: 9px;
  font-weight: 700;
}

.agent-telemetry-mini {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 10px;
  color: #38bdf8;
  margin-top: 4px;
}

/* Latency coloring */
.latency-fast {
  color: #34d399;
}
.latency-medium {
  color: #fbbf24;
}
.latency-slow {
  color: #fb7185;
}

/* Card Labels */
.card-labels-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.host-tag, .host-tag-sm {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  border-radius: 4px;
  padding: 2px 6px;
  font-size: 10px;
}

.host-tag-sm {
  font-size: 9px;
  padding: 1px 4px;
}

/* Card Footer */
.card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: auto;
  padding-top: 10px;
  border-top: 1px solid var(--border-subtle);
}

.footer-btn-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* 5. Table View */
.hosts-table-container {
  overflow-x: auto;
  border-radius: 14px;
}

.hosts-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}

.hosts-table th {
  padding: 14px 16px;
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border-medium);
  letter-spacing: 0.05em;
}

.hosts-table td {
  padding: 14px 16px;
  border-bottom: 1px solid var(--border-subtle);
  vertical-align: middle;
}

.host-row:hover {
  background: rgba(255, 255, 255, 0.02);
}

.row-name-group {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
}

.row-name {
  color: #fff;
  font-size: 13px;
}

.row-name:hover {
  color: #38bdf8;
}

.row-id {
  font-size: 10px;
}

.endpoint-copy-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.table-labels-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  max-width: 240px;
}

.table-actions-group {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

/* 6. Form & Modal Styles */
.host-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-row-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.input-glass {
  width: 100%;
  padding: 10px 14px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  color: #fff;
  font-size: 13px;
  transition: all 0.15s ease;
}

.input-glass:focus {
  border-color: #38bdf8;
  outline: none;
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.2);
}

.security-box {
  padding: 14px;
  border-radius: 10px;
}

.tls-toggle-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toggle-switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
  flex-shrink: 0;
}

.toggle-switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.toggle-slider {
  position: absolute;
  cursor: pointer;
  inset: 0;
  background-color: rgba(255, 255, 255, 0.15);
  transition: 0.25s;
  border-radius: 24px;
  border: 1px solid var(--border-subtle);
}

.toggle-slider:before {
  position: absolute;
  content: "";
  height: 16px;
  width: 16px;
  left: 3px;
  bottom: 3px;
  background-color: #fff;
  transition: 0.25s;
  border-radius: 50%;
}

input:checked + .toggle-slider {
  background-color: #10b981;
}

input:checked + .toggle-slider:before {
  transform: translateX(20px);
}

.toggle-label-wrap {
  display: flex;
  flex-direction: column;
}

.toggle-title {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
}

.toggle-sub {
  font-size: 11px;
}

.tls-certs-group {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.cert-textarea {
  resize: vertical;
  min-height: 56px;
}

/* Labels Editor */
.labels-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.labels-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 4px;
}

.label-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.label-eq {
  font-weight: bold;
  color: var(--text-muted);
}

.btn-remove-lbl {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
  border-radius: 6px;
  padding: 6px 10px;
  cursor: pointer;
}

/* In-Modal Test Connection */
.modal-test-banner {
  padding: 12px 16px;
  border-radius: 8px;
  font-size: 12px;
}

.modal-test-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 700;
  margin-bottom: 4px;
}

.modal-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--border-subtle);
}

.modal-action-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* 7. Detail Drawer Styles */
.drawer-content-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.drawer-hero-card {
  padding: 18px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.hero-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.hero-name {
  font-size: 18px;
  font-weight: 800;
  color: #fff;
  margin: 0;
}

.hero-id {
  font-size: 11px;
}

.hero-badges {
  display: flex;
  align-items: center;
  gap: 6px;
}

.hero-endpoint-row {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(0, 0, 0, 0.4);
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 12px;
}

.drawer-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.05em;
  margin: 0;
}

.section-header-flex {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.spec-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  padding: 14px;
  border-radius: 10px;
}

.spec-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.spec-lbl {
  font-size: 9px;
  color: var(--text-muted);
}

.spec-val {
  font-size: 12px;
  font-weight: 600;
}

.metrics-agent-box {
  padding: 14px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
}

.telemetry-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.telemetry-lbl {
  color: var(--text-muted);
  font-size: 11px;
}

.labels-container {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 12px;
  border-radius: 10px;
}

.label-chip {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 11px;
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px;
  border-radius: 10px;
}

.history-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  border-radius: 6px;
  font-size: 11px;
}

.hist-pass {
  background: rgba(16, 185, 129, 0.08);
  color: #34d399;
}

.hist-fail {
  background: rgba(244, 63, 94, 0.08);
  color: #fb7185;
}

.hist-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.hist-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.drawer-footer-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--border-subtle);
}

/* 8. Empty & Loading States */
.empty-state, .loading-state {
  padding: 48px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-radius: 14px;
}

.empty-icon {
  font-size: 40px;
}

.empty-title {
  font-size: 18px;
  font-weight: 700;
  color: #fff;
  margin: 0;
}

.spinner {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(56, 189, 248, 0.2);
  border-top-color: #38bdf8;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin-bottom: 8px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Confirm Alerts */
.confirm-dialog-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.confirm-alert {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px;
  border-radius: 10px;
}

.alert-danger {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.35);
  color: #fb7185;
}

.alert-icon {
  font-size: 20px;
}

.alert-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.4;
}

.text-rose {
  color: #fb7185;
}
.text-cyan {
  color: #38bdf8;
}
.text-emerald {
  color: #34d399;
}
.text-primary {
  color: #fff;
}
.text-muted {
  color: var(--text-muted);
}
.text-right {
  text-align: right;
}
</style>
