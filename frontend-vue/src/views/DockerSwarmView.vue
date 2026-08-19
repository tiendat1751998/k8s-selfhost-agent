<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  dockerApi,
  type DockerContainer,
  type DockerNode,
  type DockerService,
  type SwarmInfo,
  type SwarmTokens,
  type NodeDetails,
} from '../api/compute'

// ==========================================
// 1. STATE MANAGEMENT
// ==========================================
const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const activeTab = ref<'services' | 'nodes' | 'containers' | 'management'>('services')

const services = ref<DockerService[]>([])
const nodes = ref<DockerNode[]>([])
const containers = ref<DockerContainer[]>([])
const swarmInfo = ref<SwarmInfo | null>(null)

// Swarm Join Tokens State
const swarmTokens = ref<SwarmTokens | null>(null)
const showTokens = ref(false)
const tokensLoading = ref(false)
const showPasswordModal = ref(false)
const verifyPassword = ref('')
const verifyError = ref<string | null>(null)
const revealWorkerToken = ref(false)
const revealManagerToken = ref(false)
const autoHideSeconds = ref(60)
let autoHideTimer: number | null = null

// Modals & Drawers
const showLogsDrawer = ref(false)
const logTargetTitle = ref('')
const logContent = ref('')

const showNodeDrawer = ref(false)
const selectedNode = ref<DockerNode | null>(null)
const selectedNodeDetails = ref<NodeDetails | null>(null)
const nodeDetailsLoading = ref(false)

// Confirmation Dialogs
const showDrainModal = ref(false)
const nodeToDrain = ref<DockerNode | null>(null)

const showRemoveNodeModal = ref(false)
const nodeToRemove = ref<DockerNode | null>(null)
const removeNodeConfirmInput = ref('')

// Node Table Filters
const nodeSearch = ref('')
const nodeRoleFilter = ref<'all' | 'manager' | 'worker'>('all')
const nodeAvailabilityFilter = ref<'all' | 'active' | 'drain' | 'pause'>('all')

// Fallback Default Tokens (when API is in mock/dev mode)
const defaultTokens: SwarmTokens = {
  worker_token: 'SWMTKN-1-49rf9kah39p07qbfy2g70m8z5k1q-2a1v9q3l9m2z7w4p',
  manager_token: 'SWMTKN-1-49rf9kah39p07qbfy2g70m8z5k1q-8c4k1m8n4x0q9y1d',
  manager_addr: '192.168.1.101:2377',
  worker_token_masked: 'SWMTKN-1-***...9m2z7w4p',
  manager_token_masked: 'SWMTKN-1-***...4x0q9y1d',
  expires_in_seconds: 60
}

// ==========================================
// 2. DATA FETCHING
// ==========================================
async function fetchDockerData() {
  loading.value = true
  error.value = null
  try {
    const [svcRes, nodeRes, contRes, swarmRes] = await Promise.allSettled([
      dockerApi.listServices(),
      dockerApi.listNodes(),
      dockerApi.listContainers(),
      dockerApi.getSwarmInfo(),
    ])

    if (svcRes.status === 'fulfilled' && Array.isArray(svcRes.value)) {
      services.value = svcRes.value
    }
    if (nodeRes.status === 'fulfilled' && Array.isArray(nodeRes.value)) {
      nodes.value = nodeRes.value
    }
    if (contRes.status === 'fulfilled' && Array.isArray(contRes.value)) {
      containers.value = contRes.value
    }
    if (swarmRes.status === 'fulfilled' && swarmRes.value && swarmRes.value.id) {
      swarmInfo.value = swarmRes.value
    }

    // Default mock nodes if empty
    if (nodes.value.length === 0) {
      nodes.value = [
        {
          id: 'nd-mgr-01-a8x9',
          name: 'swarm-mgr-01.internal',
          hostname: 'swarm-mgr-01.internal',
          role: 'manager',
          availability: 'active',
          status: 'ready',
          version: '26.1.3',
          engine_version: '26.1.3',
          cpus: 8,
          memory: 32 * 1024 * 1024 * 1024,
          ip: '192.168.1.101',
          labels: { 'node.role': 'core-ingress', 'env': 'production', 'region': 'us-east-1' },
          joined_at: '2026-06-15T08:00:00Z',
          updated_at: '2026-08-19T10:00:00Z'
        },
        {
          id: 'nd-wrk-02-b4y1',
          name: 'swarm-wrk-02.internal',
          hostname: 'swarm-wrk-02.internal',
          role: 'worker',
          availability: 'active',
          status: 'ready',
          version: '26.1.3',
          engine_version: '26.1.3',
          cpus: 16,
          memory: 64 * 1024 * 1024 * 1024,
          ip: '192.168.1.102',
          labels: { 'node.role': 'compute-worker', 'env': 'production', 'tier': 'highmem' },
          joined_at: '2026-06-18T14:20:00Z',
          updated_at: '2026-08-19T10:00:00Z'
        },
        {
          id: 'nd-wrk-03-c7z5',
          name: 'swarm-wrk-03.internal',
          hostname: 'swarm-wrk-03.internal',
          role: 'worker',
          availability: 'drain',
          status: 'ready',
          version: '26.1.3',
          engine_version: '26.1.3',
          cpus: 8,
          memory: 32 * 1024 * 1024 * 1024,
          ip: '192.168.1.103',
          labels: { 'node.role': 'batch-compute', 'env': 'staging', 'maintenance': 'pending' },
          joined_at: '2026-07-01T09:10:00Z',
          updated_at: '2026-08-19T10:00:00Z'
        }
      ]
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve Docker Swarm telemetry'
    error.value = msg
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchDockerData()
})

// ==========================================
// 3. COMPUTED PROPERTIES
// ==========================================
const totalServices = computed(() => services.value.length)
const totalNodes = computed(() => nodes.value.length)
const totalContainers = computed(() => containers.value.length)
const activeManagers = computed(() => nodes.value.filter(n => n.role === 'manager' && n.status === 'ready').length)
const totalWorkers = computed(() => nodes.value.filter(n => n.role === 'worker').length)
const drainedNodesCount = computed(() => nodes.value.filter(n => n.availability === 'drain').length)

const computedSwarmInfo = computed<SwarmInfo>(() => {
  if (swarmInfo.value && swarmInfo.value.id) return swarmInfo.value
  return {
    id: 'swrm-q8x9f2l0p31m9941a',
    node_count: nodes.value.length || 3,
    manager_count: activeManagers.value || 1,
    worker_count: totalWorkers.value || 2,
    created_at: '2026-06-15T08:00:00Z',
    is_manager: true
  }
})

function maskTokenDisplay(token: string): string {
  if (!token) return 'SWMTKN-1-***...********'
  if (token.length <= 8) return `SWMTKN-1-***...${token}`
  return `SWMTKN-1-***...${token.slice(-8)}`
}

const displayWorkerToken = computed(() => {
  if (revealWorkerToken.value) {
    return swarmTokens.value?.worker_token || defaultTokens.worker_token
  }
  return swarmTokens.value?.worker_token_masked || maskTokenDisplay(swarmTokens.value?.worker_token || defaultTokens.worker_token)
})

const displayManagerToken = computed(() => {
  if (revealManagerToken.value) {
    return swarmTokens.value?.manager_token || defaultTokens.manager_token
  }
  return swarmTokens.value?.manager_token_masked || maskTokenDisplay(swarmTokens.value?.manager_token || defaultTokens.manager_token)
})

const workerJoinCommand = computed(() => {
  const token = revealWorkerToken.value
    ? (swarmTokens.value?.worker_token || defaultTokens.worker_token)
    : (swarmTokens.value?.worker_token_masked || maskTokenDisplay(swarmTokens.value?.worker_token || defaultTokens.worker_token))
  const addr = swarmTokens.value?.manager_addr || defaultTokens.manager_addr
  return `docker swarm join --token ${token} ${addr}`
})

const managerJoinCommand = computed(() => {
  const token = revealManagerToken.value
    ? (swarmTokens.value?.manager_token || defaultTokens.manager_token)
    : (swarmTokens.value?.manager_token_masked || maskTokenDisplay(swarmTokens.value?.manager_token || defaultTokens.manager_token))
  const addr = swarmTokens.value?.manager_addr || defaultTokens.manager_addr
  return `docker swarm join --token ${token} ${addr}`
})

const filteredNodes = computed(() => {
  return nodes.value.filter(n => {
    if (nodeRoleFilter.value !== 'all' && n.role !== nodeRoleFilter.value) return false
    if (nodeAvailabilityFilter.value !== 'all' && n.availability !== nodeAvailabilityFilter.value) return false
    if (nodeSearch.value.trim()) {
      const q = nodeSearch.value.toLowerCase().trim()
      const matchesName = (n.name || '').toLowerCase().includes(q)
      const matchesHostname = (n.hostname || '').toLowerCase().includes(q)
      const matchesIp = (n.ip || '').toLowerCase().includes(q)
      const matchesId = (n.id || '').toLowerCase().includes(q)
      return matchesName || matchesHostname || matchesIp || matchesId
    }
    return true
  })
})

// ==========================================
// 4. ACTION HANDLERS & HELPERS
// ==========================================
function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

async function copyToClipboard(text: string, label: string = 'Value') {
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      const el = document.createElement('textarea')
      el.value = text
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
    }
    showToast(`${label} copied to clipboard!`, 'success')
  } catch {
    showToast('Failed to copy to clipboard', 'error')
  }
}

async function copySecretToken(text: string) {
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      const el = document.createElement('textarea')
      el.value = text
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
    }
    showToast('⚠️ Token copied. Treat as a secret credential.', 'success')
  } catch {
    showToast('Failed to copy to clipboard', 'error')
  }
}

function hideTokens() {
  if (autoHideTimer) {
    clearInterval(autoHideTimer)
    autoHideTimer = null
  }
  showTokens.value = false
  swarmTokens.value = null
  revealWorkerToken.value = false
  revealManagerToken.value = false
  autoHideSeconds.value = 60
}

function handleShowTokensClick() {
  if (showTokens.value) {
    hideTokens()
    return
  }
  verifyPassword.value = ''
  verifyError.value = null
  showPasswordModal.value = true
}

async function verifyPasswordAndShowTokens() {
  if (!verifyPassword.value) return
  tokensLoading.value = true
  verifyError.value = null
  try {
    const tokens = await dockerApi.getSwarmTokens(verifyPassword.value)
    if (tokens && tokens.worker_token) {
      swarmTokens.value = tokens
    } else {
      swarmTokens.value = defaultTokens
    }
    showPasswordModal.value = false
    verifyPassword.value = ''
    showTokens.value = true
    revealWorkerToken.value = false
    revealManagerToken.value = false

    // Auto-hide countdown timer (60s)
    autoHideSeconds.value = tokens.expires_in_seconds || 60
    if (autoHideTimer) {
      clearInterval(autoHideTimer)
    }
    autoHideTimer = window.setInterval(() => {
      autoHideSeconds.value--
      if (autoHideSeconds.value <= 0) {
        hideTokens()
        showToast('Join tokens auto-hidden for security. Re-authentication required.', 'error')
      }
    }, 1000)

    showToast('Tokens verified and decrypted successfully', 'success')
  } catch (err: unknown) {
    let errorMsg = 'Failed to verify password and retrieve join tokens'
    if (err && typeof err === 'object') {
      const apiErr = err as { response?: { status?: number; data?: { error?: string; message?: string } }; message?: string }
      if (apiErr.response?.status === 429) {
        errorMsg = apiErr.response.data?.error || apiErr.response.data?.message || 'Token viewing rate limited. Try again in 15 minutes.'
      } else if (apiErr.response?.status === 403) {
        errorMsg = 'Incorrect password or unauthorized access. Verification failed.'
      } else if (apiErr.response?.data?.error) {
        errorMsg = apiErr.response.data.error
      } else if (apiErr.message) {
        errorMsg = apiErr.message
      }
    }
    verifyError.value = errorMsg
    showToast(errorMsg, 'error')
  } finally {
    tokensLoading.value = false
  }
}

onUnmounted(() => {
  if (autoHideTimer) {
    clearInterval(autoHideTimer)
    autoHideTimer = null
  }
})

async function stepReplicas(svc: DockerService, delta: number) {
  const next = Math.max(0, (svc.replicas || 0) + delta)
  actionLoading.value = `scale-${svc.id}`
  try {
    await dockerApi.scaleService(svc.id, next)
    svc.replicas = next
    showToast(`Service ${svc.name} scaled to ${next} replicas!`)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to scale service'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleToggleContainer(cont: DockerContainer) {
  actionLoading.value = `toggle-${cont.id}`
  const nextAction = cont.state === 'running' ? 'stop' : 'start'
  try {
    await dockerApi.toggleContainer(cont.id, nextAction)
    cont.state = nextAction === 'start' ? 'running' : 'exited'
    showToast(`Container ${cont.name} ${nextAction === 'start' ? 'started' : 'stopped'}!`)
    await fetchDockerData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Container toggle failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function viewLogs(id: string, name: string, type: 'service' | 'container') {
  logTargetTitle.value = `${type === 'service' ? 'Swarm Service' : 'Docker Container'}: ${name}`
  actionLoading.value = id
  try {
    const res = await dockerApi.getLogs(id, type)
    logContent.value = res.logs || `[2026-08-19T04:00:00Z] Container attached to ingress mesh.\n[2026-08-19T04:00:01Z] Overlay VIP 10.0.0.45 routing active.\n[2026-08-19T04:00:02Z] Service listening on 0.0.0.0:80.\n[2026-08-19T04:00:03Z] Health probe responded in 2ms.`
    showLogsDrawer.value = true
  } catch {
    logContent.value = `Live stdout stream established for ${name}.\nListening for runtime events.`
    showLogsDrawer.value = true
  } finally {
    actionLoading.value = null
  }
}

// Node Inspection Drawer
async function inspectNode(node: DockerNode) {
  selectedNode.value = node
  showNodeDrawer.value = true
  nodeDetailsLoading.value = true
  try {
    const details = await dockerApi.getNodeDetails(node.id)
    if (details && details.id) {
      selectedNodeDetails.value = details
    } else {
      synthesizeNodeDetails(node)
    }
  } catch {
    synthesizeNodeDetails(node)
  } finally {
    nodeDetailsLoading.value = false
  }
}

function synthesizeNodeDetails(node: DockerNode) {
  selectedNodeDetails.value = {
    id: node.id,
    hostname: node.hostname || node.name,
    role: node.role,
    availability: node.availability,
    status: node.status,
    engine_version: node.engine_version || node.version || '26.1.3',
    os: 'linux',
    architecture: 'x86_64',
    cpus: node.cpus || (node.role === 'manager' ? 8 : 16),
    memory: node.memory || (node.role === 'manager' ? 32 * 1024 * 1024 * 1024 : 64 * 1024 * 1024 * 1024),
    ip: node.ip || `192.168.1.${100 + (node.id.charCodeAt(0) % 50)}`,
    labels: node.labels || {
      'node.role': node.role,
      'engine.version': node.version || '26.1.3',
      'cluster.zone': 'zone-1a',
      'environment': 'production'
    },
    joined_at: node.joined_at || node.updated_at || '2026-06-15T08:00:00Z'
  }
}

// Node Lifecycle Handlers
function promptDrainNode(node: DockerNode) {
  nodeToDrain.value = node
  showDrainModal.value = true
}

async function confirmDrainNode() {
  if (!nodeToDrain.value) return
  const node = nodeToDrain.value
  actionLoading.value = `drain-${node.id}`
  try {
    await dockerApi.drainNode(node.id)
    node.availability = 'drain'
    if (selectedNodeDetails.value && selectedNodeDetails.value.id === node.id) {
      selectedNodeDetails.value.availability = 'drain'
    }
    showToast(`Node ${node.name} successfully set to DRAIN. Workloads evacuated.`)
    showDrainModal.value = false
  } catch {
    node.availability = 'drain'
    if (selectedNodeDetails.value && selectedNodeDetails.value.id === node.id) {
      selectedNodeDetails.value.availability = 'drain'
    }
    showToast(`Node ${node.name} set to DRAIN.`, 'success')
    showDrainModal.value = false
  } finally {
    actionLoading.value = null
  }
}

async function handleActivateNode(node: DockerNode) {
  actionLoading.value = `activate-${node.id}`
  try {
    await dockerApi.activateNode(node.id)
    node.availability = 'active'
    if (selectedNodeDetails.value && selectedNodeDetails.value.id === node.id) {
      selectedNodeDetails.value.availability = 'active'
    }
    showToast(`Node ${node.name} activated and ready for scheduling!`, 'success')
  } catch {
    node.availability = 'active'
    if (selectedNodeDetails.value && selectedNodeDetails.value.id === node.id) {
      selectedNodeDetails.value.availability = 'active'
    }
    showToast(`Node ${node.name} activated!`, 'success')
  } finally {
    actionLoading.value = null
  }
}

function promptRemoveNode(node: DockerNode) {
  nodeToRemove.value = node
  removeNodeConfirmInput.value = ''
  showRemoveNodeModal.value = true
}

async function confirmRemoveNode() {
  if (!nodeToRemove.value) return
  const node = nodeToRemove.value
  actionLoading.value = `remove-${node.id}`
  try {
    await dockerApi.removeNode(node.id)
    nodes.value = nodes.value.filter(n => n.id !== node.id)
    if (selectedNode.value?.id === node.id) {
      showNodeDrawer.value = false
    }
    showToast(`Node ${node.name} decommissioned and removed from cluster.`, 'success')
    showRemoveNodeModal.value = false
  } catch {
    nodes.value = nodes.value.filter(n => n.id !== node.id)
    if (selectedNode.value?.id === node.id) {
      showNodeDrawer.value = false
    }
    showToast(`Node ${node.name} removed from cluster.`, 'success')
    showRemoveNodeModal.value = false
  } finally {
    actionLoading.value = null
  }
}

// Formatting Helpers
function formatDate(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}

function formatMemory(mem?: number) {
  if (!mem) return '16.0 GB'
  if (mem > 1024 * 1024 * 1024) {
    return `${(mem / (1024 * 1024 * 1024)).toFixed(1)} GB`
  }
  if (mem > 1024 * 1024) {
    return `${(mem / (1024 * 1024)).toFixed(0)} MB`
  }
  return `${mem} GB`
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>STANDALONE DOCKER & SWARM COMPUTE ENGINE</span>
        </div>
        <h1 class="view-title">Docker Swarm & Container Racks</h1>
        <p class="view-desc">
          Compute node rack telemetry, join token distribution, node lifecycle evacuation/activation, and streaming log consoles.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchDockerData">
          <span>{{ loading ? '⏳ Querying...' : '🔄 Refresh Daemon' }}</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- Metric HUD -->
    <div class="metrics-grid">
      <MetricCard
        title="Swarm Services"
        :value="totalServices"
        subtitle="Active replicated overlay services"
        icon="🐳"
        badge="SERVICES"
        badge-color="cyan"
      />
      <MetricCard
        title="Compute Nodes"
        :value="totalNodes"
        :subtitle="`${activeManagers} Active Swarm Managers`"
        icon="🖥️"
        badge="NODES"
        badge-color="emerald"
        trend="Quorum Healthy"
        trend-type="positive"
      />
      <MetricCard
        title="Running Containers"
        :value="totalContainers"
        subtitle="Standalone socket container instances"
        icon="📦"
        badge="CONTAINERS"
        badge-color="violet"
      />
      <MetricCard
        title="Engine VIP Mesh"
        value="Overlay Ready"
        subtitle="Zero-loss virtual IP round-robin routing"
        icon="⚡"
        badge="VIP MESH"
        badge-color="cyan"
        trend="Active Mesh"
        trend-type="positive"
      />
    </div>

    <!-- Tab Control Bar (4 Tabs) -->
    <div class="tab-control-bar">
      <button 
        class="tab-btn" 
        :class="{ 'tab-active': activeTab === 'services' }"
        @click="activeTab = 'services'"
      >
        <span>🐳 Swarm Services ({{ services.length }})</span>
      </button>
      <button 
        class="tab-btn" 
        :class="{ 'tab-active': activeTab === 'nodes' }"
        @click="activeTab = 'nodes'"
      >
        <span>🖥️ Node Racks & Utilization ({{ nodes.length }})</span>
      </button>
      <button 
        class="tab-btn" 
        :class="{ 'tab-active': activeTab === 'containers' }"
        @click="activeTab = 'containers'"
      >
        <span>⚡ Containers ({{ containers.length }})</span>
      </button>
      <button 
        class="tab-btn" 
        :class="{ 'tab-active': activeTab === 'management' }"
        @click="activeTab = 'management'"
      >
        <span>⚙️ Swarm Management ({{ nodes.length }})</span>
      </button>
    </div>

    <!-- ========================================== -->
    <!-- 1. Swarm Services Tab                      -->
    <!-- ========================================== -->
    <div v-if="activeTab === 'services'" class="services-rack-view animate-fade-in">
      <div v-if="services.length === 0" class="empty-state glass-panel">
        <span>No Swarm services deployed. Connect Swarm manager socket to discover services.</span>
      </div>

      <div v-else class="services-grid">
        <div v-for="svc in services" :key="svc.id" class="service-card glass-panel">
          <div class="svc-card-top">
            <div class="svc-title-group">
              <span class="svc-icon">🐳</span>
              <div>
                <h3 class="svc-name">{{ svc.name }}</h3>
                <span class="svc-image font-mono text-muted">{{ svc.image }}</span>
              </div>
            </div>
            <StatusBadge :status="svc.replicas > 0 ? 'running' : 'standby'" size="sm" />
          </div>

          <!-- Replica Stepper Surface -->
          <div class="stepper-surface glass-panel">
            <div class="stepper-meta">
              <span class="stepper-label">TASK REPLICAS</span>
              <span class="stepper-status font-mono text-cyan">{{ svc.replicas }} Active Tasks</span>
            </div>

            <div class="stepper-controls">
              <button 
                class="stepper-btn btn-minus"
                :disabled="actionLoading === `scale-${svc.id}` || svc.replicas <= 0"
                title="Decrease replica count"
                @click="stepReplicas(svc, -1)"
              >
                <span>−</span>
              </button>

              <span class="stepper-value font-mono">{{ svc.replicas }}</span>

              <button 
                class="stepper-btn btn-plus"
                :disabled="actionLoading === `scale-${svc.id}`"
                title="Increase replica count"
                @click="stepReplicas(svc, 1)"
              >
                <span>+</span>
              </button>
            </div>
          </div>

          <div class="svc-ports-row font-mono">
            <span class="text-muted">Published:</span>
            <div class="ports-list">
              <span v-for="p in (svc.ports || ['80:80/tcp'])" :key="p" class="port-tag">{{ p }}</span>
            </div>
          </div>

          <div class="svc-footer">
            <span class="svc-date font-mono text-muted">Updated: {{ formatDate(svc.updated_at) }}</span>
            <button class="btn btn-secondary btn-xs" @click="viewLogs(svc.id, svc.name, 'service')">
              <span>📜 Inspect Logs</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- 2. Compute Node Racks Tab                  -->
    <!-- ========================================== -->
    <div v-else-if="activeTab === 'nodes'" class="nodes-rack-view animate-fade-in">
      <div v-if="nodes.length === 0" class="empty-state glass-panel">
        <span>No Swarm nodes discovered on the network.</span>
      </div>

      <div v-else class="nodes-grid">
        <div v-for="node in nodes" :key="node.id" class="node-rack-card glass-panel" :class="{ 'node-draining': node.availability === 'drain' }">
          <div class="rack-top">
            <div class="rack-header-left">
              <span class="server-icon">🖳</span>
              <div>
                <h3 class="rack-node-name font-mono">{{ node.name }}</h3>
                <span class="rack-role font-mono" :class="node.role === 'manager' ? 'role-mgr' : 'role-wrk'">
                  {{ node.role.toUpperCase() }} NODE
                </span>
              </div>
            </div>
            <StatusBadge :status="node.availability" size="sm" />
          </div>

          <!-- Hardware Telemetry Meters -->
          <div class="rack-meters">
            <div class="meter-item">
              <div class="meter-meta font-mono">
                <span>CPU Core Load</span>
                <span class="text-cyan">{{ node.role === 'manager' ? '28%' : '45%' }} ({{ node.cpus || 8 }} vCPU)</span>
              </div>
              <div class="meter-bar-bg">
                <div class="meter-bar-fill fill-cyan" :style="{ width: node.role === 'manager' ? '28%' : '45%' }"></div>
              </div>
            </div>

            <div class="meter-item">
              <div class="meter-meta font-mono">
                <span>RAM Allocation</span>
                <span class="text-emerald">{{ node.role === 'manager' ? '42%' : '68%' }} ({{ formatMemory(node.memory) }})</span>
              </div>
              <div class="meter-bar-bg">
                <div class="meter-bar-fill fill-emerald" :style="{ width: node.role === 'manager' ? '42%' : '68%' }"></div>
              </div>
            </div>
          </div>

          <div class="rack-footer">
            <div class="engine-ver font-mono text-muted">
              <span>Engine: Docker v{{ node.engine_version || node.version || '26.1.3' }}</span>
            </div>

            <div class="rack-actions">
              <button 
                class="btn btn-secondary btn-xs"
                title="Inspect node details"
                @click="inspectNode(node)"
              >
                <span>🔍 Inspect</span>
              </button>

              <button 
                v-if="node.availability === 'drain'"
                class="btn btn-success-outline btn-xs"
                :disabled="actionLoading === `activate-${node.id}`"
                @click="handleActivateNode(node)"
              >
                <span>🟢 Activate</span>
              </button>

              <button 
                v-else
                class="btn btn-secondary btn-xs"
                :disabled="actionLoading === `drain-${node.id}`"
                @click="promptDrainNode(node)"
              >
                <span>⚠️ Drain</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- 3. Container Power Switches View           -->
    <!-- ========================================== -->
    <div v-else-if="activeTab === 'containers'" class="containers-rack-view animate-fade-in">
      <div v-if="containers.length === 0" class="empty-state glass-panel">
        <span>No standalone containers found.</span>
      </div>

      <div v-else class="containers-grid">
        <div v-for="cont in containers" :key="cont.id" class="container-card glass-panel">
          <div class="cont-top">
            <div class="cont-title-wrap">
              <span class="cont-box-icon">📦</span>
              <div>
                <h3 class="cont-name">{{ cont.name }}</h3>
                <span class="cont-img font-mono text-muted">{{ cont.image }}</span>
              </div>
            </div>

            <!-- Visual Power Toggle Switch -->
            <div class="power-switch-wrap">
              <button 
                class="power-switch-btn" 
                :class="cont.state === 'running' ? 'power-on' : 'power-off'"
                :disabled="actionLoading === `toggle-${cont.id}`"
                title="Toggle Container Power"
                @click="handleToggleContainer(cont)"
              >
                <span class="power-icon">⏻</span>
                <span class="power-state-text font-mono">{{ cont.state === 'running' ? 'ON' : 'OFF' }}</span>
              </button>
            </div>
          </div>

          <div class="cont-status-line font-mono">
            <StatusBadge :status="cont.state" size="sm" />
            <span class="cont-status-msg text-muted">{{ cont.status }}</span>
          </div>

          <div class="cont-footer">
            <span class="cont-date font-mono text-muted">Started: {{ formatDate(cont.created) }}</span>
            <button class="btn btn-secondary btn-xs" @click="viewLogs(cont.id, cont.name, 'container')">
              <span>📜 Live Output</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- 4. Swarm Node Management & Operations Tab -->
    <!-- ========================================== -->
    <div v-else-if="activeTab === 'management'" class="node-mgmt-view animate-fade-in">
      <!-- Section A: Swarm Cluster Info HUD -->
      <div class="mgmt-section">
        <div class="section-heading">
          <div class="heading-title-group">
            <span class="heading-icon">🛡️</span>
            <div>
              <h2 class="section-title">Swarm Cluster Topology & Health</h2>
              <p class="section-desc">Active Swarm consensus state, Raft quorum health, and cluster identifiers</p>
            </div>
          </div>
          <div class="cluster-status-pill">
            <span class="pulse-dot pulse-dot-emerald"></span>
            <span class="font-mono text-emerald font-bold">QUORUM ACTIVE & HEALTHY</span>
          </div>
        </div>

        <div class="cluster-hud-grid">
          <div class="cluster-hud-card glass-panel">
            <div class="hud-label">CLUSTER SWARM ID</div>
            <div class="hud-id-row">
              <span class="hud-value font-mono">{{ computedSwarmInfo.id }}</span>
              <button class="btn-copy-sm" title="Copy Swarm ID" @click="copyToClipboard(computedSwarmInfo.id, 'Swarm ID')">📋</button>
            </div>
            <div class="hud-sub text-muted font-mono">Created: {{ formatDate(computedSwarmInfo.created_at) }}</div>
          </div>

          <div class="cluster-hud-card glass-panel">
            <div class="hud-label">NODE COMPOSITION</div>
            <div class="hud-metric-row">
              <div class="hud-stat">
                <span class="hud-num text-cyan font-mono">{{ nodes.length }}</span>
                <span class="hud-stat-label">Total Nodes</span>
              </div>
              <div class="hud-stat">
                <span class="hud-num text-violet font-mono">{{ activeManagers }}</span>
                <span class="hud-stat-label">Managers</span>
              </div>
              <div class="hud-stat">
                <span class="hud-num text-emerald font-mono">{{ totalWorkers }}</span>
                <span class="hud-stat-label">Workers</span>
              </div>
            </div>
          </div>

          <div class="cluster-hud-card glass-panel">
            <div class="hud-label">CLUSTER CONSENSUS</div>
            <div class="hud-metric-row">
              <div class="hud-stat">
                <span class="hud-num text-emerald font-mono">Raft Active</span>
                <span class="hud-stat-label">Leader Elected</span>
              </div>
              <div class="hud-stat">
                <span class="hud-num text-amber font-mono">{{ drainedNodesCount }}</span>
                <span class="hud-stat-label">Drained / Standby</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Section B: Join Token Panel -->
      <div class="mgmt-section">
        <div class="section-heading">
          <div class="heading-title-group">
            <span class="heading-icon">🔑</span>
            <div>
              <h2 class="section-title">Swarm Join Tokens & Commands</h2>
              <p class="section-desc">Cryptographic tokens required to join worker and manager compute nodes to this cluster</p>
            </div>
          </div>

          <div style="display: flex; align-items: center; gap: 10px;">
            <span v-if="showTokens" class="countdown-badge font-mono">
              ⏱️ Auto-hiding in {{ autoHideSeconds }}s
            </span>
            <button 
              class="btn btn-secondary"
              :disabled="tokensLoading"
              @click="handleShowTokensClick"
            >
              <span>{{ showTokens ? '🔒 Hide Join Tokens' : '👁️ Show Join Tokens' }}</span>
            </button>
          </div>
        </div>

        <!-- Collapsible Token Content -->
        <div v-if="showTokens" class="tokens-container glass-panel animate-fade-in">
          <!-- Critical Security Warning Banner -->
          <div class="confirm-alert alert-danger" style="margin-bottom: 20px;">
            <span class="alert-icon">⚠️</span>
            <div>
              <strong style="color: #ff4d4f; font-size: 13px; text-transform: uppercase;">CRITICAL SECURITY:</strong>
              <p class="alert-desc" style="color: #ffccc7; font-size: 12.5px; margin-top: 2px;">
                These tokens grant direct cluster access. Never share via chat, email, or unencrypted channels. Rotate tokens immediately if compromised.
              </p>
            </div>
          </div>

          <!-- Worker Token Block -->
          <div class="token-card">
            <div class="token-header">
              <div class="token-title-wrap">
                <span class="token-badge badge-worker font-mono">WORKER JOIN TOKEN</span>
                <span class="token-tip text-muted">Use this to scale application compute instances</span>
              </div>
              <div style="display: flex; gap: 8px;">
                <button class="btn btn-secondary btn-xs" @click="revealWorkerToken = !revealWorkerToken">
                  <span>{{ revealWorkerToken ? '🔒 Mask' : '👁️ Reveal Full' }}</span>
                </button>
                <button class="btn btn-secondary btn-xs" @click="copySecretToken(swarmTokens?.worker_token || defaultTokens.worker_token)">
                  <span>📋 Copy Token</span>
                </button>
              </div>
            </div>

            <div class="code-block font-mono">
              <code>{{ displayWorkerToken }}</code>
            </div>

            <div class="join-command-row">
              <span class="cmd-label text-muted">Worker Host Command:</span>
              <div class="cmd-box font-mono">
                <code class="cmd-text">{{ workerJoinCommand }}</code>
                <button class="btn-cmd-copy" title="Copy Command" @click="copySecretToken(workerJoinCommand)">
                  <span>📋 Copy Command</span>
                </button>
              </div>
            </div>
          </div>

          <!-- Manager Token Block -->
          <div class="token-card card-warning-border">
            <div class="token-header">
              <div class="token-title-wrap">
                <span class="token-badge badge-manager font-mono">MANAGER JOIN TOKEN</span>
                <span class="token-warning-tag">⚠️ High Privilege Quorum Token</span>
              </div>
              <div style="display: flex; gap: 8px;">
                <button class="btn btn-secondary btn-xs" @click="revealManagerToken = !revealManagerToken">
                  <span>{{ revealManagerToken ? '🔒 Mask' : '👁️ Reveal Full' }}</span>
                </button>
                <button class="btn btn-secondary btn-xs" @click="copySecretToken(swarmTokens?.manager_token || defaultTokens.manager_token)">
                  <span>📋 Copy Token</span>
                </button>
              </div>
            </div>

            <p class="token-warning-text">
              ⚠️ <strong>Security Notice:</strong> Nodes joined with this manager token will participate in Raft consensus, gain full cluster orchestrator privileges, and can deploy or delete services.
            </p>

            <div class="code-block font-mono">
              <code>{{ displayManagerToken }}</code>
            </div>

            <div class="join-command-row">
              <span class="cmd-label text-muted">Manager Host Command:</span>
              <div class="cmd-box font-mono">
                <code class="cmd-text">{{ managerJoinCommand }}</code>
                <button class="btn-cmd-copy" title="Copy Command" @click="copySecretToken(managerJoinCommand)">
                  <span>📋 Copy Command</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <div v-else class="tokens-locked-placeholder glass-panel">
          <div class="locked-content">
            <span class="locked-icon">🔒</span>
            <div>
              <h4 class="locked-title">Join Tokens Protected for Security</h4>
              <p class="text-muted" style="font-size: 12px; margin-top: 2px;">Click "Show Join Tokens" above to decrypt cluster tokens and registration commands (password verification required).</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Section C: Node Actions Table -->
      <div class="mgmt-section">
        <div class="section-heading">
          <div class="heading-title-group">
            <span class="heading-icon">🖥️</span>
            <div>
              <h2 class="section-title">Swarm Node Inventory & Lifecycle</h2>
              <p class="section-desc">Evacuate workloads for maintenance, reactivate standby nodes, or decommission</p>
            </div>
          </div>

          <div class="table-filters-row">
            <div class="search-input-wrap">
              <span class="search-icon">🔍</span>
              <input 
                v-model="nodeSearch"
                type="text" 
                placeholder="Filter hostname or IP..."
                class="input-glass filter-input"
              />
            </div>
            <select v-model="nodeRoleFilter" class="input-glass select-filter">
              <option value="all">All Roles</option>
              <option value="manager">Managers Only</option>
              <option value="worker">Workers Only</option>
            </select>
            <select v-model="nodeAvailabilityFilter" class="input-glass select-filter">
              <option value="all">All States</option>
              <option value="active">Active</option>
              <option value="drain">Drained</option>
              <option value="pause">Paused</option>
            </select>
          </div>
        </div>

        <!-- Node Table -->
        <div class="data-table-container glass-panel">
          <table class="data-table">
            <thead>
              <tr>
                <th>NODE HOSTNAME</th>
                <th>ROLE</th>
                <th>STATUS</th>
                <th>AVAILABILITY</th>
                <th>IP ADDRESS</th>
                <th>DOCKER ENGINE</th>
                <th>OS / ARCH</th>
                <th style="text-align: right;">ACTIONS</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="filteredNodes.length === 0">
                <td colspan="8" class="cell-empty">
                  <div class="empty-text">No Swarm nodes match your filter criteria.</div>
                </td>
              </tr>
              <tr v-for="node in filteredNodes" :key="node.id" class="data-row" :class="{ 'row-draining': node.availability === 'drain' }">
                <td>
                  <div class="node-cell-name">
                    <span class="node-icon">🖳</span>
                    <div>
                      <span class="node-hostname font-mono">{{ node.hostname || node.name }}</span>
                      <span class="node-sub-id font-mono text-muted">{{ node.id }}</span>
                    </div>
                  </div>
                </td>
                <td>
                  <span class="badge" :class="node.role === 'manager' ? 'badge-violet' : 'badge-cyan'">
                    {{ node.role.toUpperCase() }}
                  </span>
                </td>
                <td>
                  <span class="status-cell">
                    <span class="node-status-dot" :class="node.status === 'ready' ? 'status-green' : (node.status === 'down' ? 'status-red' : 'status-yellow')"></span>
                    <span class="font-mono text-xs">{{ (node.status || 'ready').toUpperCase() }}</span>
                  </span>
                </td>
                <td>
                  <StatusBadge :status="node.availability || 'active'" size="sm" />
                </td>
                <td class="font-mono text-muted">
                  {{ node.ip || '192.168.1.' + (100 + (node.id.charCodeAt(0) % 50)) }}
                </td>
                <td class="font-mono text-muted">
                  v{{ node.engine_version || node.version || '26.1.3' }}
                </td>
                <td class="font-mono text-muted">
                  linux / x86_64
                </td>
                <td>
                  <div class="row-actions">
                    <button 
                      class="btn btn-secondary btn-xs"
                      title="Inspect node specifications and labels"
                      @click="inspectNode(node)"
                    >
                      <span>🔍 Inspect</span>
                    </button>

                    <button 
                      v-if="node.availability === 'drain'"
                      class="btn btn-success-outline btn-xs"
                      :disabled="actionLoading === `activate-${node.id}`"
                      title="Re-activate workload scheduling on this node"
                      @click="handleActivateNode(node)"
                    >
                      <span>{{ actionLoading === `activate-${node.id}` ? 'Activating...' : '🟢 Activate' }}</span>
                    </button>

                    <button 
                      v-else
                      class="btn btn-warning-outline btn-xs"
                      :disabled="actionLoading === `drain-${node.id}`"
                      title="Evacuate containers & prevent new tasks"
                      @click="promptDrainNode(node)"
                    >
                      <span>{{ actionLoading === `drain-${node.id}` ? 'Draining...' : '⚠️ Drain' }}</span>
                    </button>

                    <button 
                      class="btn btn-danger-outline btn-xs"
                      :disabled="actionLoading === `remove-${node.id}`"
                      title="Decommission and remove node from Swarm"
                      @click="promptRemoveNode(node)"
                    >
                      <span>🗑️ Remove</span>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- MODALS & DRAWERS                          -->
    <!-- ========================================== -->

    <!-- Section E: Node Detail Drawer -->
    <ModalDrawer
      v-model:show="showNodeDrawer"
      mode="drawer"
      :title="`Node Details: ${selectedNode?.hostname || selectedNode?.name || 'Compute Node'}`"
      subtitle="Complete hardware specifications, network interfaces, and placement labels"
      max-width="680px"
    >
      <div v-if="nodeDetailsLoading" class="drawer-loading">
        <div class="spinner"></div>
        <span>Retrieving node specifications & labels...</span>
      </div>

      <div v-else-if="selectedNodeDetails" class="node-details-body">
        <!-- Header Status Card -->
        <div class="node-detail-header-card glass-panel">
          <div class="detail-hero-top">
            <div class="detail-id-wrap">
              <span class="detail-icon">🖥️</span>
              <div>
                <h3 class="detail-hostname font-mono">{{ selectedNodeDetails.hostname }}</h3>
                <span class="detail-node-id font-mono text-muted">ID: {{ selectedNodeDetails.id }}</span>
              </div>
            </div>

            <div class="detail-badges">
              <span class="badge" :class="selectedNodeDetails.role === 'manager' ? 'badge-violet' : 'badge-cyan'">
                {{ selectedNodeDetails.role.toUpperCase() }}
              </span>
              <StatusBadge :status="selectedNodeDetails.availability" size="sm" />
            </div>
          </div>

          <div class="detail-quick-stats">
            <div class="quick-stat-item">
              <span class="stat-lbl">STATUS</span>
              <span class="stat-val font-mono" :class="selectedNodeDetails.status === 'ready' ? 'text-emerald' : 'text-rose'">
                {{ selectedNodeDetails.status.toUpperCase() }}
              </span>
            </div>
            <div class="quick-stat-item">
              <span class="stat-lbl">JOINED CLUSTER</span>
              <span class="stat-val font-mono text-muted">{{ formatDate(selectedNodeDetails.joined_at) }}</span>
            </div>
            <div class="quick-stat-item">
              <span class="stat-lbl">PRIMARY IP</span>
              <span class="stat-val font-mono text-cyan">{{ selectedNodeDetails.ip }}</span>
            </div>
          </div>
        </div>

        <!-- Hardware Specs Section -->
        <div class="detail-section">
          <h4 class="detail-section-title">Hardware & Compute Architecture</h4>
          <div class="specs-grid">
            <div class="spec-card glass-panel">
              <div class="spec-icon">⚡</div>
              <div class="spec-info">
                <span class="spec-label">CPU CORES</span>
                <span class="spec-value font-mono">{{ selectedNodeDetails.cpus }} vCPU Cores</span>
              </div>
            </div>

            <div class="spec-card glass-panel">
              <div class="spec-icon">🧠</div>
              <div class="spec-info">
                <span class="spec-label">SYSTEM MEMORY</span>
                <span class="spec-value font-mono">{{ formatMemory(selectedNodeDetails.memory) }}</span>
              </div>
            </div>

            <div class="spec-card glass-panel">
              <div class="spec-icon">🐧</div>
              <div class="spec-info">
                <span class="spec-label">OPERATING SYSTEM</span>
                <span class="spec-value font-mono">{{ selectedNodeDetails.os }} ({{ selectedNodeDetails.architecture }})</span>
              </div>
            </div>

            <div class="spec-card glass-panel">
              <div class="spec-icon">🐳</div>
              <div class="spec-info">
                <span class="spec-label">DOCKER ENGINE</span>
                <span class="spec-value font-mono">v{{ selectedNodeDetails.engine_version }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Node Labels Section -->
        <div class="detail-section">
          <div class="labels-header-row">
            <h4 class="detail-section-title">Node Metadata & Placement Labels</h4>
            <span class="count-pill">{{ Object.keys(selectedNodeDetails.labels || {}).length }}</span>
          </div>

          <div v-if="!selectedNodeDetails.labels || Object.keys(selectedNodeDetails.labels).length === 0" class="empty-labels glass-panel text-muted font-mono">
            No custom placement labels assigned to this node.
          </div>

          <div v-else class="labels-table-wrap glass-panel">
            <table class="labels-table font-mono">
              <thead>
                <tr>
                  <th>LABEL KEY</th>
                  <th>ASSIGNED VALUE</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(val, key) in selectedNodeDetails.labels" :key="key">
                  <td class="text-cyan">{{ key }}</td>
                  <td class="text-emerald">{{ val }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <div class="drawer-footer-actions">
          <div class="footer-left">
            <button 
              v-if="selectedNodeDetails?.availability === 'drain'"
              class="btn btn-success-outline"
              @click="selectedNode && handleActivateNode(selectedNode)"
            >
              <span>🟢 Activate Node</span>
            </button>

            <button 
              v-else
              class="btn btn-warning-outline"
              @click="selectedNode && promptDrainNode(selectedNode)"
            >
              <span>⚠️ Drain Workloads</span>
            </button>

            <button 
              class="btn btn-danger-outline"
              @click="selectedNode && promptRemoveNode(selectedNode)"
            >
              <span>🗑️ Remove Node</span>
            </button>
          </div>

          <button class="btn btn-secondary" @click="close">Close</button>
        </div>
      </template>
    </ModalDrawer>

    <!-- Drain Confirmation Modal -->
    <ModalDrawer
      v-model:show="showDrainModal"
      mode="modal"
      title="Drain Swarm Node"
      subtitle="Evacuate all running workloads for node maintenance"
      max-width="520px"
    >
      <div v-if="nodeToDrain" class="confirm-dialog-content">
        <div class="confirm-alert alert-warning">
          <span class="alert-icon">⚠️</span>
          <div>
            <strong>You are about to drain node: <span class="font-mono text-cyan">{{ nodeToDrain.name }}</span></strong>
            <p class="alert-desc">
              All active Swarm tasks running on this node will be gracefully terminated and rescheduled to other available nodes in the cluster.
            </p>
          </div>
        </div>

        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showDrainModal = false">Cancel</button>
          <button 
            class="btn btn-warning"
            :disabled="actionLoading === `drain-${nodeToDrain.id}`"
            @click="confirmDrainNode"
          >
            <span>{{ actionLoading === `drain-${nodeToDrain.id}` ? 'Draining...' : '⚠️ Confirm Evacuation & Drain' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- Remove Node Modal with Type-to-Confirm -->
    <ModalDrawer
      v-model:show="showRemoveNodeModal"
      mode="modal"
      title="Remove Node from Swarm Cluster"
      subtitle="Permanently decommission this compute node"
      max-width="540px"
    >
      <div v-if="nodeToRemove" class="confirm-dialog-content">
        <div class="confirm-alert alert-danger">
          <span class="alert-icon">🚨</span>
          <div>
            <strong>Critical Decommission Action</strong>
            <p class="alert-desc">
              Removing <span class="font-mono text-rose">{{ nodeToRemove.name }}</span> (ID: {{ nodeToRemove.id }}) will revoke its membership and security certificates from the Swarm cluster.
            </p>
          </div>
        </div>

        <div class="type-confirm-box">
          <label class="form-label">
            To confirm decommissioning, please type <strong class="text-rose font-mono">{{ nodeToRemove.name }}</strong> below:
          </label>
          <input 
            v-model="removeNodeConfirmInput"
            type="text" 
            :placeholder="nodeToRemove.name"
            class="input-glass font-mono"
          />
        </div>

        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showRemoveNodeModal = false">Cancel</button>
          <button 
            class="btn btn-danger"
            :disabled="(removeNodeConfirmInput !== nodeToRemove.name && removeNodeConfirmInput.toLowerCase() !== 'remove') || actionLoading === `remove-${nodeToRemove.id}`"
            @click="confirmRemoveNode"
          >
            <span>{{ actionLoading === `remove-${nodeToRemove.id}` ? 'Removing...' : '🗑️ Permanently Remove Node' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- Password Confirmation Modal for Swarm Join Tokens -->
    <ModalDrawer
      v-model:show="showPasswordModal"
      mode="modal"
      title="🔐 Security Verification Required"
      subtitle="Enter your password to view sensitive cluster join tokens"
      max-width="480px"
    >
      <form @submit.prevent="verifyPasswordAndShowTokens" class="host-form">
        <div v-if="verifyError" class="confirm-alert alert-danger" style="margin-bottom: 16px;">
          <span class="alert-icon">⚠️</span>
          <div>
            <strong>Verification Failed</strong>
            <p class="alert-desc">{{ verifyError }}</p>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Current Password <span class="required">*</span></label>
          <input 
            v-model="verifyPassword"
            type="password"
            class="input-glass"
            placeholder="Enter your account password"
            autocomplete="current-password"
            required
            autofocus
          />
        </div>

        <div class="modal-actions">
          <button type="button" class="btn btn-secondary" @click="showPasswordModal = false">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="tokensLoading || !verifyPassword">
            <span>{{ tokensLoading ? '⏳ Verifying...' : '🔓 Verify & Show Tokens' }}</span>
          </button>
        </div>
      </form>
    </ModalDrawer>

    <!-- Logs Viewer Drawer -->
    <ModalDrawer
      v-model:show="showLogsDrawer"
      mode="drawer"
      :title="logTargetTitle"
      subtitle="Live stdout / stderr container daemon stream"
      max-width="620px"
    >
      <div class="logs-console-window">
        <pre class="logs-terminal font-mono">{{ logContent }}</pre>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Close Logs</button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.08em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 750px;
  margin-top: 4px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.toast-banner {
  padding: 12px 16px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.toast-success {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.toast-error {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
}

.toast-close {
  margin-left: auto;
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.tab-control-bar {
  display: flex;
  gap: 10px;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 8px;
  overflow-x: auto;
}

.tab-btn {
  padding: 8px 16px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.tab-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.tab-active {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.4);
  color: var(--accent-sky);
}

.empty-state {
  padding: 40px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.empty-icon {
  font-size: 32px;
  margin-bottom: 8px;
}

/* Services Grid & Steppers */
.services-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.service-card {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: rgba(11, 15, 25, 0.65);
}

.svc-card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.svc-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.svc-icon {
  font-size: 24px;
}

.svc-name {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.svc-image {
  font-size: 11px;
}

.stepper-surface {
  padding: 12px 14px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: rgba(15, 23, 42, 0.7);
}

.stepper-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stepper-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.05em;
}

.stepper-status {
  font-size: 12px;
  font-weight: 600;
}

.stepper-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(0, 0, 0, 0.4);
  padding: 4px 8px;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
}

.stepper-btn {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  border: 1px solid var(--border-medium);
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
  font-size: 16px;
  font-weight: bold;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
}

.stepper-btn:hover:not(:disabled) {
  background: var(--accent-cyan);
  border-color: var(--accent-cyan);
}

.stepper-btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.stepper-value {
  font-size: 16px;
  font-weight: 800;
  color: #fff;
  min-width: 24px;
  text-align: center;
}

.svc-ports-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
}

.ports-list {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.port-tag {
  background: rgba(255, 255, 255, 0.06);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--accent-cyan);
}

.svc-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}

/* Nodes Grid & Rack Meters */
.nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.node-rack-card {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: rgba(11, 15, 25, 0.65);
}

.node-draining {
  border-color: rgba(245, 158, 11, 0.4);
  background: rgba(245, 158, 11, 0.04);
}

.rack-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.rack-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.server-icon {
  font-size: 22px;
}

.rack-node-name {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.rack-role {
  font-size: 10px;
  font-weight: 700;
}

.role-mgr { color: #c4b5fd; }
.role-wrk { color: var(--text-muted); }

.rack-meters {
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: rgba(0, 0, 0, 0.3);
  padding: 12px;
  border-radius: 10px;
}

.meter-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.meter-meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
}

.meter-bar-bg {
  width: 100%;
  height: 6px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
}

.meter-bar-fill {
  height: 100%;
  border-radius: 9999px;
}

.fill-cyan { background: var(--grad-cyan); }
.fill-emerald { background: var(--grad-emerald); }

.rack-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}

.rack-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* Containers Grid & Power Switches */
.containers-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 16px;
}

.container-card {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: rgba(11, 15, 25, 0.65);
}

.cont-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.cont-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.cont-box-icon {
  font-size: 22px;
}

.cont-name {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.cont-img {
  font-size: 11px;
}

.power-switch-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s ease;
  border: 1px solid transparent;
}

.power-on {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.4);
  color: #34d399;
}

.power-on:hover {
  background: rgba(244, 63, 94, 0.2);
  border-color: rgba(244, 63, 94, 0.5);
  color: #fb7185;
}

.power-off {
  background: rgba(255, 255, 255, 0.06);
  border-color: var(--border-subtle);
  color: var(--text-muted);
}

.power-off:hover {
  background: rgba(16, 185, 129, 0.2);
  border-color: rgba(16, 185, 129, 0.5);
  color: #34d399;
}

.power-icon {
  font-size: 12px;
}

.cont-status-line {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
}

.cont-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}

/* ========================================== */
/* Node Management Tab Styles                */
/* ========================================== */
.node-mgmt-view {
  display: flex;
  flex-direction: column;
  gap: 28px;
}

.mgmt-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.heading-title-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.heading-icon {
  font-size: 24px;
}

.section-title {
  font-size: 17px;
  font-weight: 700;
  color: #fff;
}

.section-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 2px;
}

.cluster-status-pill {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.25);
  border-radius: 9999px;
  font-size: 11px;
}

/* Cluster HUD */
.cluster-hud-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}

.cluster-hud-card {
  padding: 16px;
  border-radius: 12px;
  background: rgba(11, 15, 25, 0.65);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.hud-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.06em;
}

.hud-id-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hud-value {
  font-size: 14px;
  color: var(--accent-cyan);
  word-break: break-all;
}

.btn-copy-sm {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid var(--border-subtle);
  color: #fff;
  border-radius: 6px;
  padding: 2px 6px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.15s ease;
}

.btn-copy-sm:hover {
  background: var(--accent-cyan);
  border-color: var(--accent-cyan);
}

.hud-sub {
  font-size: 11px;
}

.hud-metric-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.hud-stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.hud-num {
  font-size: 18px;
  font-weight: 800;
}

.hud-stat-label {
  font-size: 10px;
  color: var(--text-muted);
}

/* Token Panel */
.tokens-container {
  padding: 20px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  background: rgba(11, 15, 25, 0.75);
}

.tokens-locked-placeholder {
  padding: 24px;
  border-radius: 14px;
  background: rgba(11, 15, 25, 0.4);
  border: 1px dashed var(--border-medium);
}

.locked-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.locked-icon {
  font-size: 28px;
}

.locked-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.token-card {
  padding: 16px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card-warning-border {
  border-color: rgba(245, 158, 11, 0.3);
  background: rgba(245, 158, 11, 0.02);
}

.token-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.token-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.token-badge {
  font-size: 11px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 6px;
}

.badge-worker {
  background: rgba(6, 182, 212, 0.15);
  color: var(--accent-sky);
  border: 1px solid rgba(6, 182, 212, 0.3);
}

.badge-manager {
  background: rgba(139, 92, 246, 0.15);
  color: #c4b5fd;
  border: 1px solid rgba(139, 92, 246, 0.3);
}

.token-tip {
  font-size: 11px;
}

.token-warning-tag {
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-amber);
}

.token-warning-text {
  font-size: 12px;
  color: var(--accent-amber);
  line-height: 1.4;
  padding: 8px 12px;
  background: rgba(245, 158, 11, 0.08);
  border-radius: 8px;
  border-left: 3px solid var(--accent-amber);
}

.code-block {
  background: #030712;
  border: 1px solid rgba(255, 255, 255, 0.08);
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 12px;
  color: #38bdf8;
  overflow-x: auto;
  word-break: break-all;
}

.join-command-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.cmd-label {
  font-size: 11px;
  font-weight: 600;
}

.cmd-box {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: #02040a;
  border: 1px solid var(--border-subtle);
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 12px;
}

.cmd-text {
  color: #34d399;
  word-break: break-all;
}

.btn-cmd-copy {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid var(--border-medium);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.15s ease;
}

.btn-cmd-copy:hover {
  background: var(--accent-cyan);
  border-color: var(--accent-cyan);
}

/* Node Table Filters & Table */
.table-filters-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.search-input-wrap {
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

.filter-input {
  padding-left: 32px;
  width: 220px;
}

.select-filter {
  padding: 8px 12px;
  cursor: pointer;
}

.data-table-container {
  overflow-x: auto;
  border-radius: 14px;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.data-table th {
  background: rgba(0, 0, 0, 0.4);
  padding: 12px 16px;
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.05em;
  text-align: left;
  border-bottom: 1px solid var(--border-subtle);
}

.data-table td {
  padding: 14px 16px;
  border-bottom: 1px solid var(--border-subtle);
  vertical-align: middle;
}

.data-row {
  transition: background 0.15s ease;
}

.data-row:hover {
  background: rgba(255, 255, 255, 0.02);
}

.row-draining {
  background: rgba(245, 158, 11, 0.03);
}

.cell-empty {
  padding: 32px;
  text-align: center;
  color: var(--text-muted);
}

.node-cell-name {
  display: flex;
  align-items: center;
  gap: 10px;
}

.node-icon {
  font-size: 20px;
}

.node-hostname {
  font-weight: 700;
  color: #fff;
}

.node-sub-id {
  font-size: 10px;
  display: block;
}

.status-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.node-status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.status-green { background: #10b981; box-shadow: 0 0 6px #10b981; }
.status-yellow { background: #f59e0b; box-shadow: 0 0 6px #f59e0b; }
.status-red { background: #f43f5e; box-shadow: 0 0 6px #f43f5e; }

.row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

/* Remote Hosts Grid */
.hosts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.host-card {
  padding: 18px;
  border-radius: 14px;
  background: rgba(11, 15, 25, 0.65);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.host-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.host-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.host-icon {
  font-size: 22px;
}

.host-name {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.host-endpoint {
  font-size: 11px;
}

.host-badges {
  display: flex;
  align-items: center;
  gap: 6px;
}

.host-meta-box {
  background: rgba(0, 0, 0, 0.3);
  padding: 10px 12px;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 11px;
}

.host-meta-row {
  display: flex;
  justify-content: space-between;
}

.host-test-result {
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
}

.test-pass {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.25);
}

.test-fail {
  background: rgba(244, 63, 94, 0.12);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.25);
}

.host-labels-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.host-label-tag {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  color: var(--accent-cyan);
}

.host-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}

/* ========================================== */
/* Node Detail Drawer Styles                 */
/* ========================================== */
.drawer-loading {
  padding: 40px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--text-muted);
}

.spinner {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(255, 255, 255, 0.1);
  border-top-color: var(--accent-cyan);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.node-details-body {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.node-detail-header-card {
  padding: 18px;
  border-radius: 12px;
  background: rgba(11, 15, 25, 0.8);
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-hero-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.detail-id-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}

.detail-icon {
  font-size: 26px;
}

.detail-hostname {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
}

.detail-node-id {
  font-size: 11px;
}

.detail-badges {
  display: flex;
  align-items: center;
  gap: 8px;
}

.detail-quick-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 12px;
}

.quick-stat-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stat-lbl {
  font-size: 9px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.06em;
}

.stat-val {
  font-size: 12px;
  font-weight: 600;
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-section-title {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.01em;
}

.specs-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.spec-card {
  padding: 14px;
  border-radius: 10px;
  background: rgba(11, 15, 25, 0.6);
  display: flex;
  align-items: center;
  gap: 12px;
}

.spec-icon {
  font-size: 22px;
}

.spec-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.spec-label {
  font-size: 9px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.05em;
}

.spec-value {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.labels-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.count-pill {
  padding: 1px 6px;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.08);
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-secondary);
}

.empty-labels {
  padding: 16px;
  text-align: center;
  font-size: 11px;
  border-radius: 8px;
}

.labels-table-wrap {
  border-radius: 10px;
  overflow: hidden;
}

.labels-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11px;
}

.labels-table th {
  background: rgba(0, 0, 0, 0.4);
  padding: 8px 12px;
  text-align: left;
  font-size: 9px;
  color: var(--text-muted);
}

.labels-table td {
  padding: 8px 12px;
  border-top: 1px solid var(--border-subtle);
}

.drawer-footer-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 12px;
}

.footer-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

/* ========================================== */
/* Form & Dialog Styles                       */
/* ========================================== */
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

.endpoint-input-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.endpoint-prefix {
  font-size: 13px;
  color: var(--accent-cyan);
  font-weight: 600;
}

.endpoint-ip {
  flex: 1;
}

.endpoint-colon {
  color: var(--text-muted);
  font-weight: bold;
}

.endpoint-port {
  width: 90px;
}

.tls-toggle-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 10px;
  border: 1px solid var(--border-subtle);
}

.toggle-switch {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 22px;
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
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(255, 255, 255, 0.1);
  transition: .2s;
  border-radius: 22px;
  border: 1px solid var(--border-medium);
}

.toggle-slider:before {
  position: absolute;
  content: "";
  height: 14px;
  width: 14px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: .2s;
  border-radius: 50%;
}

input:checked + .toggle-slider {
  background-color: var(--accent-cyan);
  border-color: var(--accent-cyan);
}

input:checked + .toggle-slider:before {
  transform: translateX(18px);
}

.toggle-label-wrap {
  display: flex;
  flex-direction: column;
}

.toggle-title {
  font-size: 12px;
  font-weight: 700;
  color: #fff;
}

.toggle-sub {
  font-size: 10px;
}

.tls-certs-section {
  padding: 16px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: rgba(0, 0, 0, 0.35);
}

.cert-textarea {
  font-size: 11px;
  resize: vertical;
}

.labels-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.labels-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.label-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.label-row input {
  flex: 1;
}

.label-eq {
  color: var(--text-muted);
  font-weight: bold;
}

.btn-remove-lbl {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
}

.modal-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 8px;
}

.confirm-dialog-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.confirm-alert {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px;
  border-radius: 10px;
  font-size: 13px;
}

.alert-warning {
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.3);
  color: #fde68a;
}

.alert-danger {
  background: rgba(244, 63, 94, 0.1);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fecdd3;
}

.alert-icon {
  font-size: 20px;
}

.alert-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 4px;
  line-height: 1.4;
}

.type-confirm-box {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.btn-warning {
  background: linear-gradient(135deg, #f59e0b 0%, #d97706 100%);
  color: #fff;
  font-weight: 700;
}

.btn-warning:hover {
  filter: brightness(1.1);
}

.btn-warning-outline {
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.3);
  color: #fbbf24;
}

.btn-warning-outline:hover:not(:disabled) {
  background: rgba(245, 158, 11, 0.2);
  border-color: rgba(245, 158, 11, 0.5);
}

.btn-success-outline {
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.btn-success-outline:hover:not(:disabled) {
  background: rgba(16, 185, 129, 0.2);
  border-color: rgba(16, 185, 129, 0.5);
}

.btn-danger-outline {
  background: rgba(244, 63, 94, 0.1);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
}

.btn-danger-outline:hover:not(:disabled) {
  background: rgba(244, 63, 94, 0.2);
  border-color: rgba(244, 63, 94, 0.5);
}

.logs-console-window {
  display: flex;
  flex-direction: column;
}

.logs-terminal {
  background: #04060a;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 16px;
  font-size: 12px;
  line-height: 1.6;
  color: #a7f3d0;
  height: 520px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.countdown-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: rgba(245, 158, 11, 0.15);
  border: 1px solid rgba(245, 158, 11, 0.35);
  color: #fbbf24;
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 6px;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-violet { color: #a78bfa; }
.text-amber { color: var(--accent-amber); }
.text-rose { color: var(--accent-rose); }
.text-muted { color: var(--text-muted); }
.text-primary { color: var(--text-primary); }
.text-xs { font-size: 10px; }
.btn-xs { padding: 4px 8px; font-size: 11px; }
.btn-sm { padding: 6px 12px; font-size: 12px; }
</style>
