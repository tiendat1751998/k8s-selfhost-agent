<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  deploymentsApi,
  type DeploymentApp,
  type DeploymentTemplate
} from '../api/compute'

const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const deployments = ref<DeploymentApp[]>([])
const templates = ref<DeploymentTemplate[]>([])

// Templates Drawer
const showTemplatesDrawer = ref(false)

// Scale Modal
const showScaleModal = ref(false)
const selectedApp = ref<DeploymentApp | null>(null)
const targetReplicas = ref(1)

// Create Workload Modal
const showCreateModal = ref(false)
const newApp = ref<DeploymentApp>({
  name: '',
  team: 'platform-engineering',
  env: 'production',
  image: '',
  target: 'local-cluster',
  namespace: 'default',
  type: 'kubernetes',
  replicas: 2,
  status: 'healthy',
  cpu: '250m',
  memory: '512Mi',
  port: 80,
  netType: 'ClusterIP'
})

async function fetchDeployments() {
  loading.value = true
  error.value = null
  try {
    const [depsRes, tmplRes] = await Promise.allSettled([
      deploymentsApi.list(),
      deploymentsApi.listTemplates()
    ])

    if (depsRes.status === 'fulfilled') {
      deployments.value = depsRes.value
    }
    if (tmplRes.status === 'fulfilled') {
      templates.value = tmplRes.value
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve deployments'
    error.value = msg
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchDeployments()
})

const totalWorkloads = computed(() => deployments.value.length)
const totalReplicas = computed(() => deployments.value.reduce((acc, d) => acc + (d.replicas || 0), 0))
const healthyCount = computed(() => deployments.value.filter(d => d.status === 'healthy').length)
const k8sVsSwarm = computed(() => {
  const k8s = deployments.value.filter(d => d.type === 'kubernetes').length
  const swarm = deployments.value.filter(d => d.type === 'swarm' || d.type === 'docker').length
  return `${k8s} K8s / ${swarm} Swarm`
})

const columns: Column<DeploymentApp>[] = [
  { key: 'name', label: 'Workload Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'type', label: 'Runtime Type', width: '120px', sortable: true },
  { key: 'image', label: 'Container Image', sortable: true },
  { key: 'replicas', label: 'Replicas', width: '100px', sortable: true, align: 'center' },
  { key: 'status', label: 'Status', width: '120px', sortable: true },
  { key: 'actions', label: 'Operations', width: '240px', align: 'right' },
]

function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

function openScaleModal(app: DeploymentApp) {
  selectedApp.value = app
  targetReplicas.value = app.replicas || 1
  showScaleModal.value = true
}

async function handleScale() {
  if (!selectedApp.value) return
  actionLoading.value = 'scale'
  try {
    await deploymentsApi.scale({
      type: selectedApp.value.type,
      cluster: selectedApp.value.target,
      namespace: selectedApp.value.namespace,
      name: selectedApp.value.name,
      replicas: targetReplicas.value
    })
    selectedApp.value.replicas = targetReplicas.value
    showToast(`Scaled ${selectedApp.value.name} to ${targetReplicas.value} replicas!`)
    showScaleModal.value = false
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Scaling operation failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleRestart(app: DeploymentApp) {
  actionLoading.value = app.name
  try {
    await deploymentsApi.restart({
      type: app.type,
      cluster: app.target,
      namespace: app.namespace,
      name: app.name
    })
    showToast(`Rolling restart dispatched for ${app.name}!`)
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Restart failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleDelete(app: DeploymentApp) {
  if (!confirm(`Are you sure you want to terminate deployment ${app.name}?`)) return
  actionLoading.value = app.name
  try {
    await deploymentsApi.delete({
      type: app.type,
      cluster: app.target,
      namespace: app.namespace,
      name: app.name
    })
    showToast(`Deployment ${app.name} terminated.`)
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Deletion failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

function selectTemplate(tmpl: DeploymentTemplate) {
  newApp.value.name = tmpl.name.toLowerCase().replace(/[^a-z0-9-]/g, '-')
  newApp.value.image = `${tmpl.name.split(' ')[0].toLowerCase()}:${tmpl.version}`
  newApp.value.cpu = tmpl.cpu
  newApp.value.memory = tmpl.mem
  newApp.value.port = tmpl.ports
  showTemplatesDrawer.value = false
  showCreateModal.value = true
}

async function handleCreateApp() {
  if (!newApp.value.name || !newApp.value.image) {
    showToast('Name and image are required', 'error')
    return
  }
  actionLoading.value = 'create'
  try {
    await deploymentsApi.create(newApp.value)
    showToast(`Deployment ${newApp.value.name} successfully deployed!`)
    showCreateModal.value = false
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Deployment creation failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
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
          <span>ENTERPRISE WORKLOAD ORCHESTRATION</span>
        </div>
        <h1 class="view-title">Deployments & Workload Catalog</h1>
        <p class="view-desc">
          Unified deployment interface for Kubernetes Deployments & Docker Swarm services with rolling restarts and replica scaling.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" @click="showTemplatesDrawer = true">
          <span>📚 Blueprint Catalog</span>
        </button>
        <button class="btn btn-secondary" :disabled="loading" @click="fetchDeployments">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>+ Deploy Workload</span>
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
        title="Total Workloads"
        :value="totalWorkloads"
        subtitle="Active microservice deployments"
        icon="📦"
        badge="SERVICES"
        badge-color="cyan"
      />
      <MetricCard
        title="Running Replicas"
        :value="totalReplicas"
        subtitle="Distributed pod instances"
        icon="🚀"
        badge="SCALE"
        badge-color="emerald"
        trend="Auto-Scaled via KEDA"
        trend-type="positive"
      />
      <MetricCard
        title="Healthy Deployments"
        :value="`${healthyCount}/${totalWorkloads}`"
        subtitle="Workloads passing readiness probes"
        icon="🛡️"
        badge="HEALTH"
        badge-color="emerald"
        trend="100% Available"
        trend-type="positive"
      />
      <MetricCard
        title="Runtime Fleet"
        :value="k8sVsSwarm"
        subtitle="Target container orchestrators"
        icon="⚡"
        badge="RUNTIME"
        badge-color="violet"
      />
    </div>

    <!-- Deployments Table -->
    <div class="section-box glass-panel table-box">
      <div class="box-header" style="padding: 18px 22px; border-bottom: 1px solid var(--border-subtle);">
        <div>
          <h2 class="box-title">Active Workload Inventory</h2>
          <p class="box-subtitle">Live deployment replicas, container images, and lifecycle actions</p>
        </div>
      </div>

      <DataTable
        :columns="columns"
        :data="deployments"
        :loading="loading"
        :error="error"
        empty-message="No deployments found. Use '+ Deploy Workload' to launch a service."
        searchable
        search-placeholder="Filter workloads by name, image, namespace..."
      >
        <template #cell-name="{ row }">
          <span class="font-mono text-cyan" style="font-weight: 700;">{{ row.name }}</span>
        </template>

        <template #cell-type="{ row }">
          <span class="type-pill font-mono">{{ row.type }}</span>
        </template>

        <template #cell-image="{ row }">
          <span class="font-mono text-muted">{{ row.image }}</span>
        </template>

        <template #cell-replicas="{ row }">
          <span class="replicas-count font-mono">{{ row.replicas }}</span>
        </template>

        <template #cell-status="{ row }">
          <StatusBadge :status="row.status" size="sm" />
        </template>

        <template #cell-actions="{ row }">
          <div class="action-buttons">
            <button class="btn btn-secondary btn-xs" title="Scale Replicas" @click="openScaleModal(row)">
              <span>⚡ Scale</span>
            </button>
            <button 
              class="btn btn-secondary btn-xs" 
              :disabled="actionLoading === row.name"
              title="Rolling Restart" 
              @click="handleRestart(row)"
            >
              <span>🔄 Restart</span>
            </button>
            <button 
              class="btn btn-secondary btn-xs btn-remove" 
              :disabled="actionLoading === row.name"
              title="Delete Workload" 
              @click="handleDelete(row)"
            >
              <span>🗑️</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Scale Replicas Modal -->
    <ModalDrawer
      v-model:show="showScaleModal"
      mode="modal"
      :title="`Scale Workload: ${selectedApp?.name || ''}`"
      subtitle="Adjust target container replica count dynamically"
      max-width="480px"
    >
      <div v-if="selectedApp" class="scale-modal-content">
        <div class="scale-info-row">
          <span class="text-muted">Target:</span>
          <span class="font-mono text-cyan">{{ selectedApp.namespace }}/{{ selectedApp.name }}</span>
        </div>

        <div class="scale-slider-wrap">
          <div class="replicas-display">
            <span class="replicas-number font-mono">{{ targetReplicas }}</span>
            <span class="replicas-label">Desired Replicas</span>
          </div>

          <input 
            v-model.number="targetReplicas" 
            type="range" 
            min="0" 
            max="20" 
            class="scale-range" 
          />

          <div class="range-marks font-mono">
            <span>0 (Suspended)</span>
            <span>10</span>
            <span>20</span>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="actionLoading === 'scale'" @click="handleScale">
          <span>{{ actionLoading === 'scale' ? 'Scaling...' : 'Apply Scale ➔' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Templates Catalog Drawer -->
    <ModalDrawer
      v-model:show="showTemplatesDrawer"
      mode="drawer"
      title="Application Blueprint Catalog"
      subtitle="Pre-configured enterprise service blueprints ready to instantiate"
      max-width="560px"
    >
      <div class="templates-list">
        <div v-for="tmpl in templates" :key="tmpl.name" class="template-item glass-panel">
          <div class="tmpl-header">
            <div>
              <h4 class="tmpl-title">{{ tmpl.name }}</h4>
              <span class="tmpl-category font-mono">{{ tmpl.category }} · {{ tmpl.version }}</span>
            </div>
            <button class="btn btn-primary btn-xs" @click="selectTemplate(tmpl)">
              <span>Use Blueprint ➔</span>
            </button>
          </div>
          <p class="tmpl-desc">{{ tmpl.desc }}</p>
          <div class="tmpl-specs font-mono">
            <span>Port: {{ tmpl.ports }}</span>
            <span>CPU: {{ tmpl.cpu }}</span>
            <span>RAM: {{ tmpl.mem }}</span>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Close Catalog</button>
      </template>
    </ModalDrawer>

    <!-- Create Deployment Modal -->
    <ModalDrawer
      v-model:show="showCreateModal"
      mode="modal"
      title="Deploy New Application Workload"
      subtitle="Configure target runtime container specifications"
      max-width="580px"
    >
      <div class="modal-form">
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Deployment Name</label>
            <input v-model="newApp.name" type="text" placeholder="e.g. payment-service" class="input-glass" />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Namespace</label>
            <input v-model="newApp.namespace" type="text" class="input-glass font-mono" />
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Container Image</label>
          <input v-model="newApp.image" type="text" placeholder="e.g. nginx:alpine" class="input-glass font-mono" />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Runtime Type</label>
            <select v-model="newApp.type" class="input-glass">
              <option value="kubernetes">Kubernetes</option>
              <option value="swarm">Docker Swarm</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Initial Replicas</label>
            <input v-model.number="newApp.replicas" type="number" min="1" max="20" class="input-glass font-mono" />
          </div>
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">CPU Request</label>
            <input v-model="newApp.cpu" type="text" class="input-glass font-mono" />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Memory Request</label>
            <input v-model="newApp.memory" type="text" class="input-glass font-mono" />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Port</label>
            <input v-model.number="newApp.port" type="number" class="input-glass font-mono" />
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="actionLoading === 'create'" @click="handleCreateApp">
          <span>{{ actionLoading === 'create' ? 'Deploying...' : 'Deploy Workload ➔' }}</span>
        </button>
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

.section-box {
  border-radius: 16px;
  overflow: hidden;
}

.box-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.box-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.type-pill {
  font-size: 10px;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 4px;
  color: var(--text-secondary);
}

.replicas-count {
  font-size: 13px;
  font-weight: 700;
  color: var(--accent-emerald);
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.btn-remove:hover {
  background: rgba(244, 63, 94, 0.2);
  border-color: rgba(244, 63, 94, 0.4);
  color: #fb7185;
}

.scale-modal-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.scale-info-row {
  display: flex;
  justify-content: space-between;
  padding: 10px 14px;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 10px;
  font-size: 13px;
}

.scale-slider-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 20px;
  background: rgba(11, 15, 25, 0.6);
  border-radius: 14px;
}

.replicas-display {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.replicas-number {
  font-size: 48px;
  font-weight: 800;
  color: var(--accent-cyan);
}

.replicas-label {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
}

.scale-range {
  width: 100%;
  accent-color: var(--accent-cyan);
}

.range-marks {
  width: 100%;
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--text-muted);
}

.templates-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.template-item {
  padding: 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: rgba(11, 15, 25, 0.6);
}

.tmpl-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.tmpl-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.tmpl-category {
  font-size: 11px;
  color: var(--text-muted);
}

.tmpl-desc {
  font-size: 12px;
  color: var(--text-secondary);
}

.tmpl-specs {
  display: flex;
  gap: 16px;
  font-size: 11px;
  color: var(--accent-cyan);
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.flex-1 { flex: 1; }

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }
</style>
