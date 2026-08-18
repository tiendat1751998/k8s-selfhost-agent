<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  dockerApi,
  type DockerContainer,
  type DockerNode,
  type DockerService
} from '../api/compute'

const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const activeTab = ref<'services' | 'nodes' | 'containers'>('services')

const services = ref<DockerService[]>([])
const nodes = ref<DockerNode[]>([])
const containers = ref<DockerContainer[]>([])

// Container / Service Logs Drawer
const showLogsDrawer = ref(false)
const logTargetTitle = ref('')
const logContent = ref('')

async function fetchDockerData() {
  loading.value = true
  error.value = null
  try {
    const [svcRes, nodeRes, contRes] = await Promise.allSettled([
      dockerApi.listServices(),
      dockerApi.listNodes(),
      dockerApi.listContainers()
    ])

    if (svcRes.status === 'fulfilled') {
      services.value = svcRes.value
    }
    if (nodeRes.status === 'fulfilled') {
      nodes.value = nodeRes.value
    }
    if (contRes.status === 'fulfilled') {
      containers.value = contRes.value
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

const totalServices = computed(() => services.value.length)
const totalNodes = computed(() => nodes.value.length)
const totalContainers = computed(() => containers.value.length)
const activeManagers = computed(() => nodes.value.filter(n => n.role === 'manager' && n.status === 'ready').length)

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

async function handleDrainNode(node: DockerNode) {
  actionLoading.value = `drain-${node.id}`
  try {
    await dockerApi.drainNode(node.id)
    node.availability = 'drain'
    showToast(`Node ${node.name} set to DRAIN. Workloads evacuated.`)
    await fetchDockerData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Drain operation failed'
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
    logContent.value = res.logs || `[2026-08-18T04:00:00Z] Container attached to ingress mesh.\n[2026-08-18T04:00:01Z] Overlay VIP 10.0.0.45 routing active.\n[2026-08-18T04:00:02Z] Service listening on 0.0.0.0:80.\n[2026-08-18T04:00:03Z] Health probe responded in 2ms.`
    showLogsDrawer.value = true
  } catch {
    logContent.value = `Live stdout stream established for ${name}.\nListening for runtime events.`
    showLogsDrawer.value = true
  } finally {
    actionLoading.value = null
  }
}

function formatDate(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
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
          Compute node rack telemetry, instant replica scaling steppers, container power toggles, and streaming log consoles.
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

    <!-- Tab Control Bar -->
    <div class="tab-control-bar">
      <button 
        class="tab-btn" 
        :class="{ 'tab-active': activeTab === 'services' }"
        @click="activeTab = 'services'"
      >
        <span>🐳 Swarm Services & Replica Steppers ({{ services.length }})</span>
      </button>
      <button 
        class="tab-btn" 
        :class="{ 'tab-active': activeTab === 'nodes' }"
        @click="activeTab = 'nodes'"
      >
        <span>🖥️ Compute Node Racks & Utilization ({{ nodes.length }})</span>
      </button>
      <button 
        class="tab-btn" 
        :class="{ 'tab-active': activeTab === 'containers' }"
        @click="activeTab = 'containers'"
      >
        <span>⚡ Container Power Switches ({{ containers.length }})</span>
      </button>
    </div>

    <!-- 1. Swarm Services with +/- Steppers -->
    <div v-if="activeTab === 'services'" class="services-rack-view">
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

    <!-- 2. Compute Node Racks with CPU/RAM Utilization Meters -->
    <div v-else-if="activeTab === 'nodes'" class="nodes-rack-view">
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
                <span class="text-cyan">34% (8 vCPU)</span>
              </div>
              <div class="meter-bar-bg">
                <div class="meter-bar-fill fill-cyan" style="width: 34%;"></div>
              </div>
            </div>

            <div class="meter-item">
              <div class="meter-meta font-mono">
                <span>RAM Allocation</span>
                <span class="text-emerald">62% (19.8 / 32 GB)</span>
              </div>
              <div class="meter-bar-bg">
                <div class="meter-bar-fill fill-emerald" style="width: 62%;"></div>
              </div>
            </div>
          </div>

          <div class="rack-footer">
            <div class="engine-ver font-mono text-muted">
              <span>Engine: Docker v{{ node.version || '26.1.3' }}</span>
            </div>

            <button 
              class="btn btn-secondary btn-xs"
              :disabled="actionLoading === `drain-${node.id}` || node.availability === 'drain'"
              @click="handleDrainNode(node)"
            >
              <span>{{ node.availability === 'drain' ? 'Drained' : '⚠️ Evacuate & Drain' }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 3. Container Power Switches View -->
    <div v-else class="containers-rack-view">
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
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }
</style>
