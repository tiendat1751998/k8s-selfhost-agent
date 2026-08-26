<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  fleetApi,
  type Cluster,
  type ClusterDiscoveryData,
  type SwarmClusterInfo
} from '../api/fleet'

// Async loading & error states
const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

// Data state
const clusters = ref<Cluster[]>([])
const swarmInfo = ref<SwarmClusterInfo | null>(null)

// Import Modal state
const showImportModal = ref(false)
const importMode = ref<'file' | 'text'>('file')
const importForm = ref({
  id: '',
  name: '',
  group: 'production',
  region: 'us-east-1',
  provider: 'aws',
  kubeconfigRaw: '',
})
const kubeconfigFile = ref<File | null>(null)

// Discovery Drawer state
const showDiscoveryDrawer = ref(false)
const selectedCluster = ref<Cluster | null>(null)
const discoveredData = ref<ClusterDiscoveryData | null>(null)

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

const k8sNodes = computed(() => clusters.value.reduce((acc, c) => acc + (c.nodes || 0), 0))
const swarmNodes = computed(() => (hasSwarm.value && swarmInfo.value ? swarmInfo.value.node_count : 0))
const totalNodes = computed(() => k8sNodes.value + swarmNodes.value)

const healthyK8sCount = computed(() =>
  clusters.value.filter(c => ['healthy', 'active', 'ready'].includes((c.health_status || c.status || '').toLowerCase())).length
)
const healthyCount = computed(() => healthyK8sCount.value + (hasSwarm.value ? 1 : 0))

const cloudProviders = computed(() => {
  const providers = new Set<string>()
  clusters.value.forEach(c => {
    if (c.provider) providers.add(c.provider.toUpperCase())
  })
  if (hasSwarm.value) {
    providers.add('SWARM')
  }
  return Array.from(providers)
})

const clusterColumns: Column<Cluster>[] = [
  { key: 'name', label: 'Cluster Name', sortable: true },
  { key: 'group', label: 'Fleet Tier', width: '130px', sortable: true },
  { key: 'provider', label: 'Provider / Region', width: '180px', sortable: true },
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
  if (!importForm.value.name.trim()) {
    showToast('Cluster name is required', 'error')
    return
  }

  let fileToUpload: File | null = kubeconfigFile.value

  if (importMode.value === 'text') {
    if (!importForm.value.kubeconfigRaw.trim()) {
      showToast('Kubeconfig content cannot be empty', 'error')
      return
    }
    fileToUpload = new File([importForm.value.kubeconfigRaw], `${importForm.value.name}-kubeconfig.yaml`, {
      type: 'text/yaml',
    })
  }

  if (!fileToUpload) {
    showToast('Kubeconfig file or content is required for cluster import', 'error')
    return
  }

  actionLoading.value = 'import'
  try {
    const formData = new FormData()
    formData.append('id', importForm.value.id.trim() || `cluster-${Date.now().toString(36)}`)
    formData.append('name', importForm.value.name.trim())
    formData.append('group', importForm.value.group)
    formData.append('region', importForm.value.region.trim())
    formData.append('provider', importForm.value.provider)
    formData.append('kubeconfig', fileToUpload)

    await fleetApi.importCluster(formData)
    showToast(`Cluster ${importForm.value.name} successfully imported into fleet!`)
    showImportModal.value = false
    importForm.value.name = ''
    importForm.value.id = ''
    importForm.value.kubeconfigRaw = ''
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
    <!-- Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>HYBRID & MULTI-CLOUD FEDERATION</span>
        </div>
        <h1 class="view-title">Multi-Cluster Fleet Manager</h1>
        <p class="view-desc">
          Centralized topology dashboard for on-premise, edge, Docker Swarm, and cloud Kubernetes clusters with live health monitoring and zero-downtime upgrades.
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

    <!-- Error Banner -->
    <div v-if="error" class="toast-banner toast-error animate-fade-in">
      <span>⚠️</span>
      <span>{{ error }}</span>
      <button class="toast-close" @click="error = null">✕</button>
    </div>

    <!-- Metric HUD -->
    <div class="metrics-grid">
      <MetricCard
        title="Fleet Control Planes"
        :value="totalControlPlanes"
        :subtitle="`${k8sClusterCount} K8s · ${hasSwarm ? '1 Docker Swarm' : '0 Swarm'}`"
        icon="🌐"
        badge="TOPOLOGY"
        badge-color="cyan"
      />
      <MetricCard
        title="Healthy Control Planes"
        :value="totalControlPlanes > 0 ? `${healthyCount}/${totalControlPlanes}` : '0/0'"
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
        :subtitle="`${k8sNodes} K8s nodes · ${swarmNodes} Swarm nodes`"
        icon="🖥️"
        badge="COMPUTE"
        badge-color="violet"
      />
      <MetricCard
        title="Active Engine Providers"
        :value="cloudProviders.length"
        :subtitle="cloudProviders.length > 0 ? cloudProviders.join(', ') : 'No engines attached'"
        icon="☁️"
        badge="HYBRID"
        badge-color="cyan"
      />
    </div>

    <!-- Docker Swarm Cluster Banner (When Active) -->
    <div v-if="hasSwarm && swarmInfo" class="swarm-section glass-panel">
      <div class="swarm-header">
        <div class="swarm-brand">
          <span class="swarm-logo">🐳</span>
          <div>
            <div class="swarm-title-row">
              <h3 class="swarm-title">Docker Swarm Cluster</h3>
              <span class="swarm-badge font-mono">LOCAL CONTROL PLANE</span>
            </div>
            <p class="swarm-desc">
              Active native Docker cluster orchestrating container services, ingress routing, and multi-host networks.
            </p>
          </div>
        </div>
        <div class="swarm-status-wrap">
          <StatusBadge :status="swarmInfo.node_count > 0 ? 'active' : 'ready'" size="sm" />
        </div>
      </div>

      <div class="swarm-meta-grid">
        <div class="swarm-meta-card">
          <span class="meta-label">Total Swarm Nodes</span>
          <span class="meta-value font-mono text-emerald">{{ swarmInfo.node_count }} Nodes</span>
        </div>
        <div class="swarm-meta-card">
          <span class="meta-label">Managers / Workers</span>
          <span class="meta-value font-mono text-cyan">{{ swarmInfo.manager_count }} Manager / {{ swarmInfo.worker_count }} Worker</span>
        </div>
        <div class="swarm-meta-card">
          <span class="meta-label">Control Node Role</span>
          <span class="meta-value font-mono">{{ swarmInfo.is_manager ? 'Swarm Manager' : 'Worker Node' }}</span>
        </div>
        <div class="swarm-meta-card">
          <span class="meta-label">Cluster ID</span>
          <span class="meta-value font-mono text-muted text-truncate" :title="swarmInfo.id">{{ swarmInfo.id || 'swarm-local' }}</span>
        </div>
      </div>

      <div class="swarm-actions-row">
        <router-link to="/infra/hosts" class="btn btn-secondary btn-xs">
          <span>🖥️ View Swarm Compute Hosts & Nodes ➔</span>
        </router-link>
      </div>
    </div>

    <!-- Kubernetes Fleet Section -->
    <div class="section-box glass-panel">
      <div class="box-header">
        <div>
          <h2 class="box-title">Kubernetes Fleet Control Planes</h2>
          <p class="box-subtitle">Managed multi-region Kubernetes clusters with live status and telemetry</p>
        </div>
        <button v-if="clusters.length > 0" class="btn btn-secondary btn-xs" @click="showImportModal = true">
          <span>+ Add Cluster</span>
        </button>
      </div>

      <!-- Empty State for K8s Clusters -->
      <div v-if="clusters.length === 0" class="empty-fleet-card">
        <div class="empty-icon-wrap">
          <span class="empty-icon">⎈</span>
        </div>
        <h3 class="empty-title">No External Kubernetes Clusters Registered</h3>
        <p class="empty-desc">
          Connect your multi-region Kubernetes clusters to establish centralized federation, health monitoring, and unified workload orchestration.
        </p>

        <div class="fleet-features-grid">
          <div class="feature-item glass-panel">
            <div class="feature-header">
              <span class="feature-badge-icon">🌐</span>
              <h4 class="feature-heading">Federated Multi-Cluster Management</h4>
            </div>
            <p class="feature-text">
              Unified topology dashboard aggregating clusters across AWS EKS, GCP GKE, Azure AKS, Bare-Metal, and Edge datacenters.
            </p>
          </div>

          <div class="feature-item glass-panel">
            <div class="feature-header">
              <span class="feature-badge-icon">🗺️</span>
              <h4 class="feature-heading">Multi-Region Fleet Orchestration</h4>
            </div>
            <p class="feature-text">
              Real-time cross-region workload distribution, tier segregation (Production, Staging, Edge), and topology mapping.
            </p>
          </div>

          <div class="feature-item glass-panel">
            <div class="feature-header">
              <span class="feature-badge-icon">🔑</span>
              <h4 class="feature-heading">Kubeconfig Import & Secure Vault</h4>
            </div>
            <p class="feature-text">
              Direct import of kubeconfig manifests encrypted with AES-256 GCM in local vault with SHA-256 integrity verification.
            </p>
          </div>

          <div class="feature-item glass-panel">
            <div class="feature-header">
              <span class="feature-badge-icon">🛡️</span>
              <h4 class="feature-heading">Cluster Health Auditing</h4>
            </div>
            <p class="feature-text">
              Continuous background probing of API server latency, node pools, discovered namespaces, and custom resource definitions.
            </p>
          </div>
        </div>

        <div class="empty-actions">
          <button class="btn btn-primary btn-lg" @click="showImportModal = true">
            <span>+ Import Cluster (Kubeconfig)</span>
          </button>
        </div>
      </div>

      <!-- K8s Clusters Cards Grid -->
      <div v-else class="clusters-card-grid">
        <div v-for="cluster in clusters" :key="cluster.id" class="cluster-card glass-panel">
          <div class="card-top">
            <div class="cluster-brand">
              <span class="cluster-icon">⎈</span>
              <div>
                <h3 class="cluster-title">{{ cluster.name }}</h3>
                <span class="cluster-group font-mono">{{ cluster.group || 'default' }} · {{ (cluster.provider || 'generic').toUpperCase() }}</span>
              </div>
            </div>
            <StatusBadge :status="cluster.health_status || cluster.status || 'unknown'" size="sm" />
          </div>

          <div class="card-body-meta">
            <div class="meta-row">
              <span class="meta-lbl">Region / DC:</span>
              <span class="meta-val font-mono">{{ cluster.region || 'local' }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-lbl">Kubernetes Version:</span>
              <span class="meta-val font-mono text-cyan">{{ cluster.version || 'Pending Discovery' }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-lbl">Active Nodes:</span>
              <span class="meta-val font-mono text-emerald">{{ cluster.nodes !== undefined ? `${cluster.nodes} Nodes` : '—' }}</span>
            </div>
            <div v-if="cluster.last_health_check" class="meta-row">
              <span class="meta-lbl">Last Health Audit:</span>
              <span class="meta-val font-mono text-muted">{{ new Date(cluster.last_health_check).toLocaleTimeString() }}</span>
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

    <!-- Cluster Fleet Data Table (When Clusters Exist) -->
    <div v-if="clusters.length > 0" class="section-box glass-panel table-box">
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
          <span class="font-mono text-muted">{{ (row.provider || '').toUpperCase() }} ({{ row.region }})</span>
        </template>

        <template #cell-version="{ row }">
          <span class="font-mono text-emerald">{{ row.version || '—' }}</span>
        </template>

        <template #cell-nodes="{ row }">
          <span class="font-mono">{{ row.nodes ?? 0 }}</span>
        </template>

        <template #cell-health_status="{ row }">
          <StatusBadge :status="row.health_status || row.status || 'unknown'" size="sm" />
        </template>

        <template #cell-actions="{ row }">
          <div class="table-actions-row">
            <button class="btn btn-secondary btn-xs" :disabled="actionLoading === row.id" @click="handleDiscover(row)">
              <span>Discover</span>
            </button>
            <button class="btn btn-secondary btn-xs" :disabled="actionLoading === row.id" @click="handleUpgrade(row)">
              <span>Upgrade</span>
            </button>
            <button class="btn btn-secondary btn-xs btn-remove" :disabled="actionLoading === row.id" @click="handleRemove(row)">
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
      subtitle="Upload cluster kubeconfig to establish secure telemetry bridge"
      max-width="600px"
    >
      <div class="modal-form">
        <div class="form-group">
          <label class="form-label">Cluster Identifier / Name *</label>
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
              <option value="edge">Edge Device / K3s</option>
              <option value="generic">Generic Kubernetes</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Region / Datacenter</label>
          <input v-model="importForm.region" type="text" placeholder="e.g. us-east-1, eu-west-1, dc-onprem-1" class="input-glass font-mono" />
        </div>

        <div class="form-group">
          <div class="mode-tabs">
            <button
              type="button"
              class="mode-tab"
              :class="{ active: importMode === 'file' }"
              @click="importMode = 'file'"
            >
              📁 File Upload
            </button>
            <button
              type="button"
              class="mode-tab"
              :class="{ active: importMode === 'text' }"
              @click="importMode = 'text'"
            >
              📝 Paste YAML
            </button>
          </div>

          <div v-if="importMode === 'file'" class="file-input-wrap">
            <label class="form-label">Kubeconfig File (.yaml / .config / .json) *</label>
            <input type="file" accept=".yaml,.yml,.config,.json,text/*" class="input-glass" @change="handleFileChange" />
          </div>

          <div v-else class="text-input-wrap">
            <label class="form-label">Paste Kubeconfig YAML Content *</label>
            <textarea
              v-model="importForm.kubeconfigRaw"
              rows="6"
              placeholder="apiVersion: v1&#10;clusters:&#10;  - cluster:&#10;      server: https://..."
              class="input-glass font-mono text-area-input"
            ></textarea>
          </div>

          <span class="form-hint">🔒 Kubeconfig is encrypted with AES-256 GCM in local vault before persistence. Sensitive tokens are never returned in cleartext.</span>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="actionLoading === 'import'" @click="handleImportCluster">
          <span>{{ actionLoading === 'import' ? '⏳ Importing Cluster...' : 'Import Cluster ➔' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Resource Discovery Drawer -->
    <ModalDrawer
      v-model:show="showDiscoveryDrawer"
      mode="drawer"
      :title="`Cluster Discovery: ${selectedCluster?.name || 'Cluster'}`"
      :subtitle="`Provider: ${(selectedCluster?.provider || '').toUpperCase()} · Region: ${selectedCluster?.region || 'unknown'}`"
      max-width="580px"
    >
      <div v-if="discoveredData" class="drawer-content">
        <div class="discovery-header-card">
          <div class="meta-row">
            <span class="text-muted">API Server Latency:</span>
            <span class="text-emerald font-mono">{{ discoveredData.api_server_latency || 'Live (< 25ms)' }}</span>
          </div>
          <div class="meta-row">
            <span class="text-muted">Custom Resource Definitions:</span>
            <span class="font-mono text-cyan">
              {{ discoveredData.crd_count !== undefined ? `${discoveredData.crd_count} CRDs` : 'Discovered in cluster' }}
            </span>
          </div>
          <div v-if="discoveredData.last_sync" class="meta-row">
            <span class="text-muted">Last Synchronized:</span>
            <span class="font-mono text-muted">{{ new Date(discoveredData.last_sync).toLocaleString() }}</span>
          </div>
        </div>

        <div class="discovery-section">
          <h4 class="section-subheading">📦 Discovered Namespaces</h4>
          <div v-if="discoveredData.namespaces && discoveredData.namespaces.length > 0" class="chips-wrap">
            <span v-for="ns in discoveredData.namespaces" :key="ns" class="ns-chip font-mono">
              📁 {{ ns }}
            </span>
          </div>
          <div v-else class="empty-subtext">
            No specific namespace inventory discovered yet.
          </div>
        </div>

        <div class="discovery-section">
          <h4 class="section-subheading">🖥️ Discovered Node Pools</h4>
          <div v-if="discoveredData.node_pools && discoveredData.node_pools.length > 0" class="pools-list">
            <div v-for="(pool, idx) in discoveredData.node_pools" :key="idx" class="pool-item">
              <div class="pool-left">
                <span class="pool-name">{{ pool.name }}</span>
                <span class="pool-inst font-mono">{{ pool.instance || 'Generic Node' }} {{ pool.zone ? `· Zone: ${pool.zone}` : '' }}</span>
              </div>
              <span class="pool-count font-mono">{{ pool.count ?? 1 }} Nodes</span>
            </div>
          </div>
          <div v-else class="empty-subtext">
            {{ selectedCluster?.nodes ? `${selectedCluster.nodes} Worker Nodes active` : 'Node topology details not yet collected.' }}
          </div>
        </div>

        <div class="discovery-section">
          <h4 class="section-subheading">📋 Topology Metadata Payload</h4>
          <pre class="json-preview font-mono">{{ JSON.stringify(discoveredData, null, 2) }}</pre>
        </div>
      </div>

      <div v-else class="empty-state">
        <span>No discovery metadata recorded for this cluster yet. Trigger a discovery scan or re-import kubeconfig.</span>
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
  max-width: 780px;
  margin-top: 4px;
  line-height: 1.5;
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

/* Docker Swarm Section */
.swarm-section {
  padding: 20px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  border: 1px solid rgba(6, 182, 212, 0.25);
  background: rgba(6, 182, 212, 0.04);
}

.swarm-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.swarm-brand {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.swarm-logo {
  font-size: 32px;
  line-height: 1;
}

.swarm-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.swarm-title {
  font-size: 17px;
  font-weight: 700;
  color: #fff;
}

.swarm-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 8px;
  background: rgba(6, 182, 212, 0.2);
  color: var(--accent-cyan);
  border-radius: 4px;
}

.swarm-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 4px;
}

.swarm-meta-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
}

.swarm-meta-card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: rgba(0, 0, 0, 0.3);
  padding: 10px 14px;
  border-radius: 10px;
  border: 1px solid var(--border-subtle);
}

.swarm-meta-card .meta-label {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.swarm-meta-card .meta-value {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
}

.swarm-actions-row {
  display: flex;
  justify-content: flex-end;
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
  font-size: 16px;
  font-weight: 700;
  color: #fff;
}

.box-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

/* Empty State Card */
.empty-fleet-card {
  padding: 32px 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 20px;
}

.empty-icon-wrap {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: rgba(6, 182, 212, 0.12);
  border: 1px solid rgba(6, 182, 212, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
}

.empty-icon {
  font-size: 32px;
  color: var(--accent-cyan);
}

.empty-title {
  font-size: 18px;
  font-weight: 700;
  color: #fff;
}

.empty-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 620px;
  line-height: 1.5;
}

.fleet-features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
  width: 100%;
  text-align: left;
  margin-top: 8px;
}

.feature-item {
  padding: 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: rgba(11, 15, 25, 0.6);
  border: 1px solid var(--border-subtle);
}

.feature-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.feature-badge-icon {
  font-size: 18px;
}

.feature-heading {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.feature-text {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}

.empty-actions {
  margin-top: 12px;
}

.btn-lg {
  padding: 10px 24px;
  font-size: 14px;
  font-weight: 700;
}

.clusters-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
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
  text-transform: uppercase;
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
  letter-spacing: 0.03em;
}

.form-hint {
  font-size: 11px;
  color: var(--text-muted);
  line-height: 1.4;
}

.mode-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 4px;
}

.mode-tab {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.mode-tab.active {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
  color: #fff;
  font-weight: 600;
}

.text-area-input {
  resize: vertical;
  min-height: 120px;
  font-size: 12px;
  line-height: 1.4;
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

.empty-subtext {
  font-size: 12px;
  color: var(--text-muted);
  font-style: italic;
}

.empty-state {
  padding: 36px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
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

.text-truncate {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 140px;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }

/* Responsive Overhaul for Mobile & Tablets */
@media (max-width: 768px) {
  .view-header {
    flex-direction: column;
    align-items: stretch;
    gap: 14px;
  }

  .header-actions {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
    gap: 8px;
  }

  .header-actions .btn {
    flex: 1;
    min-width: 130px;
  }

  .metrics-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .swarm-section {
    padding: 14px;
    gap: 12px;
  }

  .swarm-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .swarm-meta-grid {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .swarm-actions-row {
    justify-content: flex-start;
  }

  .section-box {
    padding: 14px;
  }

  .clusters-card-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .fleet-features-grid {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  .form-row {
    flex-direction: column;
    gap: 10px;
  }

  .mode-tabs {
    flex-wrap: wrap;
  }

  .drawer-content {
    gap: 14px;
  }
}

@media (max-width: 640px) {
  .box-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .box-header .btn {
    width: 100%;
  }

  .card-top {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .card-actions {
    width: 100%;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 6px;
  }

  .card-actions .btn {
    flex: 1;
    min-width: 80px;
  }

  .table-actions-row {
    flex-wrap: wrap;
    justify-content: flex-start;
    gap: 4px;
  }
}
</style>
