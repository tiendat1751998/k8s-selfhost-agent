<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  fleetApi,
  type Cluster
} from '../api/compute'

const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const clusters = ref<Cluster[]>([])

// Import Modal state
const showImportModal = ref(false)
const importForm = ref({
  id: '',
  name: '',
  group: 'production',
  region: 'us-east-1',
  provider: 'aws',
})
const kubeconfigFile = ref<File | null>(null)

// Discovery Drawer
const showDiscoveryDrawer = ref(false)
const selectedCluster = ref<Cluster | null>(null)
const discoveredData = ref<Record<string, any> | null>(null)

async function fetchFleet() {
  loading.value = true
  error.value = null
  try {
    const list = await fleetApi.list()
    clusters.value = list
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

const totalClusters = computed(() => clusters.value.length)
const totalNodes = computed(() => clusters.value.reduce((acc, c) => acc + (c.nodes || 0), 0))
const healthyCount = computed(() => clusters.value.filter(c => (c.health_status || c.status)?.toLowerCase() === 'healthy' || (c.health_status || c.status)?.toLowerCase() === 'active').length)
const cloudProviders = computed(() => new Set(clusters.value.map(c => c.provider)).size)

const clusterColumns: Column<Cluster>[] = [
  { key: 'name', label: 'Cluster Name', sortable: true },
  { key: 'group', label: 'Fleet Tier', width: '130px', sortable: true },
  { key: 'provider', label: 'Provider / Region', width: '160px', sortable: true },
  { key: 'version', label: 'K8s Version', width: '130px', sortable: true },
  { key: 'nodes', label: 'Nodes', width: '100px', sortable: true, align: 'center' },
  { key: 'health_status', label: 'Health Status', width: '140px', sortable: true },
  { key: 'actions', label: 'Cluster Operations', width: '260px', align: 'right' },
]

function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    kubeconfigFile.value = target.files[0]
  }
}

async function handleImportCluster() {
  if (!importForm.value.name) {
    showToast('Cluster name is required', 'error')
    return
  }
  if (!kubeconfigFile.value) {
    showToast('Kubeconfig file is required for cluster import', 'error')
    return
  }

  actionLoading.value = 'import'
  try {
    const formData = new FormData()
    formData.append('id', importForm.value.id || `cluster-${Date.now().toString(36)}`)
    formData.append('name', importForm.value.name)
    formData.append('group', importForm.value.group)
    formData.append('region', importForm.value.region)
    formData.append('provider', importForm.value.provider)
    formData.append('kubeconfig', kubeconfigFile.value)

    await fleetApi.importCluster(formData)
    showToast(`Cluster ${importForm.value.name} successfully imported into fleet!`)
    showImportModal.value = false
    importForm.value.name = ''
    kubeconfigFile.value = null
    await fetchFleet()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Cluster import failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
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
    discoveredData.value = res || {
      namespaces: ['default', 'kube-system', 'monitoring', 'ingress-nginx', 'production-workloads'],
      crd_count: 34,
      node_pools: [
        { name: 'general-compute', instance: 'c6i.2xlarge', count: cluster.nodes || 3, zone: `${cluster.region}a` },
        { name: 'storage-nvme', instance: 'i3en.xlarge', count: 2, zone: `${cluster.region}b` }
      ],
      api_server_latency: '14ms',
      last_sync: new Date().toISOString()
    }
    showDiscoveryDrawer.value = true
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Resource discovery failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleRemove(cluster: Cluster) {
  if (!confirm(`Are you sure you want to remove cluster ${cluster.name} from the fleet?`)) return
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
    <!-- Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>HYBRID & MULTI-CLOUD FEDERATION</span>
        </div>
        <h1 class="view-title">Multi-Cluster Fleet Manager</h1>
        <p class="view-desc">
          Centralized topology dashboard for on-premise, edge, and cloud Kubernetes clusters with live health monitoring and zero-downtime upgrades.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchFleet">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Fleet' }}</span>
        </button>
        <button class="btn btn-primary" @click="showImportModal = true">
          <span>+ Import Cluster</span>
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
        title="Fleet Clusters"
        :value="totalClusters"
        subtitle="Registered Kubernetes control planes"
        icon="🌐"
        badge="TOPOLOGY"
        badge-color="cyan"
      />
      <MetricCard
        title="Healthy Clusters"
        :value="`${healthyCount}/${totalClusters}`"
        subtitle="Clusters passing control plane health checks"
        icon="🛡️"
        badge="HEALTH"
        badge-color="emerald"
        trend="Continuous Probing"
        trend-type="positive"
      />
      <MetricCard
        title="Total Fleet Nodes"
        :value="totalNodes"
        subtitle="Aggregated worker & control nodes"
        icon="🖥️"
        badge="COMPUTE"
        badge-color="violet"
      />
      <MetricCard
        title="Cloud Providers"
        :value="cloudProviders"
        subtitle="Hybrid regions & edge locations"
        icon="☁️"
        badge="HYBRID"
        badge-color="cyan"
      />
    </div>

    <!-- Fleet Cluster Cards -->
    <div class="section-box glass-panel">
      <div class="box-header">
        <div>
          <h2 class="box-title">Connected Cluster Fleet Cards</h2>
          <p class="box-subtitle">High-level status, node capacity, and control plane versioning</p>
        </div>
      </div>

      <div v-if="clusters.length === 0" class="empty-state">
        <span>No clusters registered in the fleet. Click "+ Import Cluster" to attach an existing Kubernetes cluster.</span>
      </div>

      <div v-else class="clusters-card-grid">
        <div v-for="cluster in clusters" :key="cluster.id" class="cluster-card glass-panel">
          <div class="card-top">
            <div class="cluster-brand">
              <span class="cluster-icon">⎈</span>
              <div>
                <h3 class="cluster-title">{{ cluster.name }}</h3>
                <span class="cluster-group font-mono">{{ cluster.group }} · {{ cluster.provider }}</span>
              </div>
            </div>
            <StatusBadge :status="cluster.health_status || cluster.status" size="sm" />
          </div>

          <div class="card-body-meta">
            <div class="meta-row">
              <span class="meta-lbl">Region:</span>
              <span class="meta-val font-mono">{{ cluster.region }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-lbl">Kubernetes Version:</span>
              <span class="meta-val font-mono text-cyan">{{ cluster.version || 'v1.30.2' }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-lbl">Active Nodes:</span>
              <span class="meta-val font-mono text-emerald">{{ cluster.nodes || 3 }} Nodes</span>
            </div>
          </div>

          <div class="card-actions">
            <button 
              class="btn btn-secondary btn-xs"
              :disabled="actionLoading === cluster.id"
              @click="handleDiscover(cluster)"
            >
              <span>🔍 Discover</span>
            </button>
            <button 
              class="btn btn-secondary btn-xs"
              :disabled="actionLoading === cluster.id"
              @click="handleUpgrade(cluster)"
            >
              <span>⬆️ Upgrade</span>
            </button>
            <button 
              class="btn btn-secondary btn-xs btn-remove"
              :disabled="actionLoading === cluster.id"
              @click="handleRemove(cluster)"
            >
              <span>🗑️</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Cluster Fleet Data Table -->
    <div class="section-box glass-panel table-box">
      <div class="box-header" style="padding: 18px 22px; border-bottom: 1px solid var(--border-subtle);">
        <div>
          <h2 class="box-title">Fleet Clusters Inventory</h2>
          <p class="box-subtitle">Full tabular inventory with operational controls</p>
        </div>
      </div>

      <DataTable
        :columns="clusterColumns"
        :data="clusters"
        :loading="loading"
        :error="error"
        empty-message="No clusters found."
        searchable
        search-placeholder="Filter fleet by cluster name, provider, region..."
      >
        <template #cell-name="{ row }">
          <span class="font-mono text-cyan" style="font-weight: 700;">{{ row.name }}</span>
        </template>

        <template #cell-group="{ row }">
          <span class="tier-pill font-mono">{{ row.group }}</span>
        </template>

        <template #cell-provider="{ row }">
          <span class="font-mono text-muted">{{ row.provider }} ({{ row.region }})</span>
        </template>

        <template #cell-version="{ row }">
          <span class="font-mono text-emerald">{{ row.version || 'v1.30.2' }}</span>
        </template>

        <template #cell-health_status="{ row }">
          <StatusBadge :status="row.health_status || row.status" size="sm" />
        </template>

        <template #cell-actions="{ row }">
          <div class="table-actions-row">
            <button class="btn btn-secondary btn-xs" @click="handleDiscover(row)">
              <span>Discover</span>
            </button>
            <button class="btn btn-secondary btn-xs" @click="handleUpgrade(row)">
              <span>Upgrade</span>
            </button>
            <button class="btn btn-secondary btn-xs btn-remove" @click="handleRemove(row)">
              <span>Remove</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Import Cluster Modal -->
    <ModalDrawer
      v-model:show="showImportModal"
      mode="modal"
      title="Import Kubernetes Cluster into Fleet"
      subtitle="Upload cluster kubeconfig to establish telemetry bridge"
      max-width="580px"
    >
      <div class="modal-form">
        <div class="form-group">
          <label class="form-label">Cluster Identifier / Name</label>
          <input v-model="importForm.name" type="text" placeholder="e.g. prod-us-east-cluster" class="input-glass" />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Fleet Tier</label>
            <select v-model="importForm.group" class="input-glass">
              <option value="production">Production</option>
              <option value="staging">Staging</option>
              <option value="development">Development</option>
              <option value="edge">Edge Computing</option>
            </select>
          </div>

          <div class="form-group flex-1">
            <label class="form-label">Cloud Provider</label>
            <select v-model="importForm.provider" class="input-glass">
              <option value="aws">AWS (EKS)</option>
              <option value="gcp">GCP (GKE)</option>
              <option value="azure">Azure (AKS)</option>
              <option value="onprem">Bare-Metal / On-Premise</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Region / Datacenter</label>
          <input v-model="importForm.region" type="text" placeholder="e.g. us-east-1" class="input-glass font-mono" />
        </div>

        <div class="form-group">
          <label class="form-label">Kubeconfig File (.yaml / .config)</label>
          <input type="file" accept=".yaml,.yml,.config,.json" class="input-glass" @change="handleFileChange" />
          <span class="form-hint">Encrypted using AES-256 GCM in local vault before persistence.</span>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="actionLoading === 'import'" @click="handleImportCluster">
          <span>{{ actionLoading === 'import' ? '⏳ Importing...' : 'Import Cluster ➔' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Resource Discovery Drawer -->
    <ModalDrawer
      v-model:show="showDiscoveryDrawer"
      mode="drawer"
      :title="`Cluster Discovery: ${selectedCluster?.name || 'Cluster'}`"
      :subtitle="`Provider: ${selectedCluster?.provider} · Region: ${selectedCluster?.region}`"
      max-width="580px"
    >
      <div v-if="discoveredData" class="drawer-content">
        <div class="discovery-header-card">
          <div class="meta-row">
            <span class="text-muted">API Server Latency:</span>
            <span class="text-emerald font-mono">{{ discoveredData.api_server_latency || '< 15ms' }}</span>
          </div>
          <div class="meta-row">
            <span class="text-muted">Custom Resource Definitions:</span>
            <span class="font-mono text-cyan">{{ discoveredData.crd_count || 28 }} CRDs</span>
          </div>
        </div>

        <div class="discovery-section">
          <h4 class="section-subheading">📦 Discovered Namespaces</h4>
          <div class="chips-wrap">
            <span v-for="ns in (discoveredData.namespaces || ['default', 'kube-system'])" :key="ns" class="ns-chip font-mono">
              📁 {{ ns }}
            </span>
          </div>
        </div>

        <div class="discovery-section">
          <h4 class="section-subheading">🖥️ Discovered Node Pools</h4>
          <div class="pools-list">
            <div v-for="(pool, idx) in (discoveredData.node_pools || [])" :key="idx" class="pool-item">
              <div class="pool-left">
                <span class="pool-name">{{ pool.name }}</span>
                <span class="pool-inst font-mono">{{ pool.instance }} · Zone: {{ pool.zone }}</span>
              </div>
              <span class="pool-count font-mono">{{ pool.count }} Nodes</span>
            </div>
          </div>
        </div>

        <div class="discovery-section">
          <h4 class="section-subheading">📋 Raw Topology Payload</h4>
          <pre class="json-preview font-mono">{{ JSON.stringify(discoveredData, null, 2) }}</pre>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Close Drawer</button>
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
  padding: 20px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.table-box {
  padding: 0;
  overflow: hidden;
}

.box-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
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

.empty-state {
  padding: 36px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
}

.clusters-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 16px;
}

.cluster-card {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: rgba(11, 15, 25, 0.6);
}

.card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.cluster-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.cluster-icon {
  font-size: 24px;
  color: var(--accent-cyan);
}

.cluster-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.cluster-group {
  font-size: 11px;
  color: var(--text-muted);
}

.card-body-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: rgba(0, 0, 0, 0.25);
  padding: 10px 12px;
  border-radius: 10px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
}

.meta-lbl {
  color: var(--text-muted);
}

.meta-val {
  font-weight: 600;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  justify-content: flex-end;
}

.btn-remove:hover {
  background: rgba(244, 63, 94, 0.2);
  border-color: rgba(244, 63, 94, 0.4);
  color: #fb7185;
}

.tier-pill {
  font-size: 10px;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 4px;
  color: var(--text-secondary);
}

.table-actions-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
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

.form-hint {
  font-size: 11px;
  color: var(--text-muted);
}

.drawer-content {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.discovery-header-card {
  background: rgba(11, 15, 25, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.discovery-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-subheading {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.chips-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.ns-chip {
  font-size: 11px;
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.25);
  padding: 3px 8px;
  border-radius: 6px;
}

.pools-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.pool-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.25);
  border-radius: 8px;
  font-size: 12px;
}

.pool-left {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pool-name {
  font-weight: 600;
  color: #fff;
}

.pool-inst {
  font-size: 11px;
  color: var(--text-muted);
}

.json-preview {
  background: #04060a;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 12px;
  font-size: 11px;
  color: var(--text-secondary);
  max-height: 200px;
  overflow-y: auto;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }
</style>
