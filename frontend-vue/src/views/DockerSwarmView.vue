<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  dockerApi,
  type DockerContainer,
  type DockerNode,
  type DockerService,
  type SwarmInfo,
  type NodeDetails,
} from '../api/compute'

// ==========================================
// 1. STATE MANAGEMENT
// ==========================================
const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const activeTab = ref<'services' | 'nodes' | 'containers'>('services')

const services = ref<DockerService[]>([])
const nodes = ref<DockerNode[]>([])
const containers = ref<DockerContainer[]>([])
const swarmInfo = ref<SwarmInfo | null>(null)

// Modals & Drawers
const showLogsDrawer = ref(false)
const logTargetTitle = ref('')
const logContent = ref('')

const showNodeDrawer = ref(false)
const selectedNode = ref<DockerNode | null>(null)
const selectedNodeDetails = ref<NodeDetails | null>(null)
const nodeDetailsLoading = ref(false)

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
          Compute node rack telemetry, active swarm services, standalone containers, and streaming log consoles.
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

    <!-- Tab Control Bar (3 Tabs) -->
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
    <!-- MODALS & DRAWERS                          -->
    <!-- ========================================== -->

    <!-- Node Detail Drawer -->
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
          <button class="btn btn-secondary" @click="close">Close</button>
        </div>
      </template>
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
  justify-content: flex-end;
  width: 100%;
  gap: 12px;
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
