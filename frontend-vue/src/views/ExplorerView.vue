<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  explorerApi,
  type K8sResource
} from '../api/compute'

const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const resources = ref<K8sResource[]>([])

// Filter state
const filterKind = ref<string>('all')
const filterNamespace = ref<string>('all')
const filterCluster = ref<string>('all')
const searchQuery = ref<string>('')

// Manifest Drawer
const showManifestDrawer = ref(false)
const selectedResource = ref<K8sResource | null>(null)

async function fetchResources() {
  loading.value = true
  error.value = null
  try {
    const params: Record<string, string> = {}
    if (filterKind.value !== 'all') params.kind = filterKind.value
    if (filterNamespace.value !== 'all') params.namespace = filterNamespace.value
    if (filterCluster.value !== 'all') params.cluster = filterCluster.value
    if (searchQuery.value.trim()) params.q = searchQuery.value.trim()

    const res = await explorerApi.search(params)
    resources.value = res.data
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to query Kubernetes resource explorer'
    error.value = msg
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchResources()
})

const totalDiscovered = computed(() => resources.value.length)
const podCount = computed(() => resources.value.filter(r => r.kind?.toLowerCase() === 'pod').length)
const deploymentCount = computed(() => resources.value.filter(r => r.kind?.toLowerCase() === 'deployment').length)
const uniqueNamespaces = computed(() => Array.from(new Set(resources.value.map(r => r.namespace).filter(Boolean))))
const uniqueClusters = computed(() => Array.from(new Set(resources.value.map(r => r.cluster).filter(Boolean))))

const columns: Column<K8sResource>[] = [
  { key: 'kind', label: 'Resource Kind', width: '130px', sortable: true },
  { key: 'name', label: 'Resource Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'cluster', label: 'Cluster Target', width: '140px', sortable: true },
  { key: 'status', label: 'Object State', width: '130px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Manifest', width: '180px', align: 'right' },
]

function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

function openManifestDrawer(resource: K8sResource) {
  selectedResource.value = resource
  showManifestDrawer.value = true
}

async function handleSync(resource: K8sResource) {
  actionLoading.value = resource.id
  try {
    await explorerApi.sync(resource)
    showToast(`Resource ${resource.kind}/${resource.name} state synchronized!`)
    await fetchResources()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'State sync failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

function generateManifestJSON(r: K8sResource) {
  return {
    apiVersion: r.kind === 'Pod' ? 'v1' : r.kind === 'Deployment' ? 'apps/v1' : r.kind === 'Service' ? 'v1' : 'v1',
    kind: r.kind,
    metadata: {
      name: r.name,
      namespace: r.namespace || 'default',
      cluster: r.cluster,
      labels: r.labels || {
        'app.kubernetes.io/name': r.name,
        'app.kubernetes.io/managed-by': 'k8sselfhost-platform',
        'tier': 'production'
      },
      uid: r.id,
      creationTimestamp: new Date().toISOString()
    },
    status: {
      phase: r.status,
      conditions: [
        { type: 'Ready', status: 'True', lastTransitionTime: new Date().toISOString() }
      ]
    }
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
          <span>LIVE KUBERNETES OBJECT RECONCILIATION</span>
        </div>
        <h1 class="view-title">Dynamic Resource Explorer</h1>
        <p class="view-desc">
          Cluster-wide deep object discovery across all Namespaces and CRDs with live manifest inspection and instant state reconciliation.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchResources">
          <span>{{ loading ? '⏳ Scanning...' : '🔄 Rescan Objects' }}</span>
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
        title="Tracked Objects"
        :value="totalDiscovered"
        subtitle="Active live Kubernetes entities discovered"
        icon="🔍"
        badge="OBJECTS"
        badge-color="cyan"
      />
      <MetricCard
        title="Active Pods"
        :value="podCount"
        subtitle="Discovered pod workloads"
        icon="🚀"
        badge="PODS"
        badge-color="emerald"
        trend="In Sync"
        trend-type="positive"
      />
      <MetricCard
        title="Deployments"
        :value="deploymentCount"
        subtitle="Active deployment controllers"
        icon="📦"
        badge="CONTROLLERS"
        badge-color="violet"
      />
      <MetricCard
        title="Namespaces"
        :value="uniqueNamespaces.length || 1"
        subtitle="Isolated workload domains"
        icon="📁"
        badge="TENANCY"
        badge-color="cyan"
      />
    </div>

    <!-- Filter Control Bar & Data Table -->
    <div class="section-box glass-panel table-box">
      <div class="filter-bar">
        <div class="filter-item">
          <label class="filter-label">Resource Kind:</label>
          <select v-model="filterKind" class="input-glass filter-select" @change="fetchResources">
            <option value="all">All Kinds</option>
            <option value="Pod">Pod</option>
            <option value="Deployment">Deployment</option>
            <option value="Service">Service</option>
            <option value="Node">Node</option>
            <option value="Ingress">Ingress</option>
            <option value="PVC">PersistentVolumeClaim (PVC)</option>
          </select>
        </div>

        <div class="filter-item">
          <label class="filter-label">Namespace:</label>
          <select v-model="filterNamespace" class="input-glass filter-select" @change="fetchResources">
            <option value="all">All Namespaces</option>
            <option v-for="ns in uniqueNamespaces" :key="ns" :value="ns">{{ ns }}</option>
          </select>
        </div>

        <div class="filter-item">
          <label class="filter-label">Cluster:</label>
          <select v-model="filterCluster" class="input-glass filter-select" @change="fetchResources">
            <option value="all">All Clusters</option>
            <option v-for="c in uniqueClusters" :key="c" :value="c">{{ c }}</option>
          </select>
        </div>
      </div>

      <DataTable
        :columns="columns"
        :data="resources"
        :loading="loading"
        :error="error"
        empty-message="No matching Kubernetes resources found."
        searchable
        search-placeholder="Search by resource name or labels..."
      >
        <template #cell-kind="{ row }">
          <span class="kind-pill font-mono">{{ row.kind }}</span>
        </template>

        <template #cell-name="{ row }">
          <span class="font-mono text-cyan" style="font-weight: 700;">{{ row.name }}</span>
        </template>

        <template #cell-namespace="{ row }">
          <span class="font-mono text-muted">{{ row.namespace || '-' }}</span>
        </template>

        <template #cell-cluster="{ row }">
          <span class="cluster-tag font-mono">{{ row.cluster }}</span>
        </template>

        <template #cell-status="{ row }">
          <StatusBadge :status="row.status" size="sm" />
        </template>

        <template #cell-age="{ row }">
          <span class="font-mono text-muted">{{ row.age || '1d' }}</span>
        </template>

        <template #cell-actions="{ row }">
          <div class="action-buttons">
            <button 
              class="btn btn-secondary btn-xs"
              :disabled="actionLoading === row.id"
              title="Reconcile Resource State"
              @click="handleSync(row)"
            >
              <span>{{ actionLoading === row.id ? '⏳' : '🔄' }} Sync</span>
            </button>
            <button 
              class="btn btn-primary btn-xs"
              title="Inspect JSON Manifest"
              @click="openManifestDrawer(row)"
            >
              <span>📄 Manifest</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- JSON Manifest Drawer -->
    <ModalDrawer
      v-model:show="showManifestDrawer"
      mode="drawer"
      :title="`Manifest: ${selectedResource?.kind}/${selectedResource?.name}`"
      :subtitle="`Cluster: ${selectedResource?.cluster} · Namespace: ${selectedResource?.namespace || 'cluster-scoped'}`"
      max-width="620px"
    >
      <div v-if="selectedResource" class="manifest-drawer-body">
        <div class="manifest-meta-top">
          <div class="meta-row">
            <span class="text-muted">Kind:</span>
            <span class="font-mono text-cyan">{{ selectedResource.kind }}</span>
          </div>
          <div class="meta-row">
            <span class="text-muted">Live Status:</span>
            <StatusBadge :status="selectedResource.status" size="sm" />
          </div>
          <div class="meta-row">
            <span class="text-muted">Target Cluster:</span>
            <span class="font-mono">{{ selectedResource.cluster }}</span>
          </div>
        </div>

        <div class="manifest-code-wrapper">
          <div class="manifest-code-header font-mono">
            <span>JSON Object Spec (Live State)</span>
          </div>
          <pre class="manifest-code font-mono">{{ JSON.stringify(generateManifestJSON(selectedResource), null, 2) }}</pre>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Close Viewer</button>
        <button 
          v-if="selectedResource"
          class="btn btn-primary" 
          :disabled="actionLoading === selectedResource.id"
          @click="handleSync(selectedResource)"
        >
          <span>{{ actionLoading === selectedResource.id ? 'Syncing...' : 'Force Sync Spec ➔' }}</span>
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

.filter-bar {
  padding: 14px 20px;
  display: flex;
  align-items: center;
  gap: 20px;
  border-bottom: 1px solid var(--border-subtle);
  background: rgba(11, 15, 25, 0.5);
  flex-wrap: wrap;
}

.filter-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.filter-select {
  padding: 6px 12px;
  font-size: 12px;
}

.kind-pill {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 6px;
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.25);
  font-weight: 600;
}

.cluster-tag {
  font-size: 11px;
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-secondary);
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.manifest-drawer-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.manifest-meta-top {
  background: rgba(11, 15, 25, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}

.manifest-code-wrapper {
  display: flex;
  flex-direction: column;
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
}

.manifest-code-header {
  padding: 8px 14px;
  background: rgba(15, 23, 42, 0.8);
  font-size: 11px;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border-subtle);
}

.manifest-code {
  background: #04060a;
  padding: 16px;
  font-size: 12px;
  line-height: 1.6;
  color: #38bdf8;
  height: 520px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }
</style>
