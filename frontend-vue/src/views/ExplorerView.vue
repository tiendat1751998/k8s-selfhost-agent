<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import SecretViewer from '../components/k8s/SecretViewer.vue'
import YamlEditorModal from '../components/k8s/YamlEditorModal.vue'
import CreateResourceModal from '../components/k8s/CreateResourceModal.vue'
import {
  k8sApi,
  type K8sResource,
  type K8sNamespace,
  type ResourceKind
} from '../api/k8s'
import { fleetApi, type Cluster } from '../api/compute'
import { jsonToYaml } from '../utils/yaml'

const route = useRoute()

// Async loading & error states
const loading = ref(false)
const error = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
const clusterOffline = ref(false)
const offlineErrorMessage = ref<string | null>(null)

// Core State
const clusters = ref<Cluster[]>([])
const selectedCluster = ref<string>('default')
const namespaces = ref<K8sNamespace[]>([])
const selectedNamespace = ref<string>('all')
const selectedKind = ref<ResourceKind>('deployments')
const resources = ref<K8sResource[]>([])

// Create Modal State
const showCreateModal = ref(false)

// YAML Editor Modal State
const showYamlModal = ref(false)
const yamlEditorMode = ref<'create' | 'edit'>('create')
const yamlEditorInitialContent = ref('')
const yamlEditorTitle = ref('Apply Kubernetes Manifest')

// Detail Drawer State
const showDetailDrawer = ref(false)
const selectedResource = ref<K8sResource | null>(null)
const activeDrawerTab = ref<'overview' | 'yaml'>('overview')
const drawerYamlCopied = ref(false)

// New Namespace Modal State
const showNewNsModal = ref(false)
const newNsName = ref('')
const creatingNs = ref(false)
const newNsError = ref<string | null>(null)

// Delete Confirmation Modal State
const showDeleteModal = ref(false)
const resourceToDelete = ref<K8sResource | null>(null)
const deletingResource = ref(false)

// Resource Kind Tree Definition
interface KindCategory {
  title: string
  icon: string
  items: { label: string; kind: ResourceKind; icon: string }[]
}

const kindCategories: KindCategory[] = [
  {
    title: 'Workloads',
    icon: '??',
    items: [
      { label: 'Deployments', kind: 'deployments', icon: '??' },
      { label: 'StatefulSets', kind: 'statefulsets', icon: '??' },
      { label: 'DaemonSets', kind: 'daemonsets', icon: '???' },
      { label: 'Jobs', kind: 'jobs', icon: '??' },
      { label: 'CronJobs', kind: 'cronjobs', icon: '??' },
    ],
  },
  {
    title: 'Config & Storage',
    icon: '??',
    items: [
      { label: 'ConfigMaps', kind: 'configmaps', icon: '??' },
      { label: 'Secrets', kind: 'secrets', icon: '??' },
      { label: 'PersistentVolumeClaims', kind: 'persistentvolumeclaims', icon: '??' },
      { label: 'StorageClasses', kind: 'storageclasses', icon: '??' },
    ],
  },
  {
    title: 'Networking',
    icon: '??',
    items: [
      { label: 'Services', kind: 'services', icon: '??' },
      { label: 'Ingresses', kind: 'ingresses', icon: '??' },
    ],
  },
]

// Import Cluster Modal State
const showImportModal = ref(false)
const importMode = ref<'file' | 'text'>('file')
const importForm = ref({
  id: '',
  name: '',
  group: 'production',
  region: 'us-east-1',
  provider: 'k8s',
  kubeconfigRaw: '',
})
const kubeconfigFile = ref<File | null>(null)
const importingCluster = ref(false)

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

  importingCluster.value = true
  try {
    const formData = new FormData()
    formData.append('id', importForm.value.id.trim() || `cluster-${Date.now().toString(36)}`)
    formData.append('name', importForm.value.name.trim())
    formData.append('group', importForm.value.group)
    formData.append('region', importForm.value.region.trim())
    formData.append('provider', importForm.value.provider)
    formData.append('kubeconfig', fileToUpload)

    await fleetApi.importCluster(formData)
    showToast(`Cluster ${importForm.value.name} successfully imported!`)
    showImportModal.value = false
    const importedName = importForm.value.name.trim()
    importForm.value.name = ''
    importForm.value.id = ''
    importForm.value.kubeconfigRaw = ''
    kubeconfigFile.value = null
    await loadClusters()
    selectedCluster.value = importedName
    await loadNamespaces()
    await fetchResources()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Cluster import failed'
    showToast(msg, 'error')
  } finally {
    importingCluster.value = false
  }
}

// Fetch Clusters
async function loadClusters() {
  try {
    const list = await fleetApi.list()
    if (list && list.length > 0) {
      clusters.value = list
      if (!selectedCluster.value || selectedCluster.value === 'default' || !list.some(c => (c.name || c.id) === selectedCluster.value)) {
        selectedCluster.value = list[0].name || list[0].id
      }
    } else {
      clusters.value = []
      if (!selectedCluster.value || selectedCluster.value === 'default') {
        selectedCluster.value = 'primary-cluster'
      }
    }
  } catch {
    clusters.value = []
    if (!selectedCluster.value || selectedCluster.value === 'default') {
      selectedCluster.value = 'primary-cluster'
    }
  }
}

// Fetch Namespaces for Selected Cluster
async function loadNamespaces() {
  if (!selectedCluster.value) {
    namespaces.value = []
    return
  }
  try {
    const nsList = await k8sApi.listNamespaces(selectedCluster.value)
    namespaces.value = Array.isArray(nsList) ? nsList : []
  } catch {
    namespaces.value = []
  }
}

// Fetch Resources
async function fetchResources() {
  if (!selectedCluster.value) {
    resources.value = []
    return
  }
  loading.value = true
  error.value = null
  clusterOffline.value = false
  offlineErrorMessage.value = null

  try {
    const ns = selectedNamespace.value !== 'all' ? selectedNamespace.value : undefined
    const list = await k8sApi.listResources(selectedCluster.value, selectedKind.value, ns)
    resources.value = Array.isArray(list) ? list : []
  } catch (err: unknown) {
    resources.value = []
    clusterOffline.value = true
    const msg = err instanceof Error ? err.message : 'Failed to query Kubernetes cluster'
    offlineErrorMessage.value = msg
  } finally {
    loading.value = false
  }
}

// Watchers
watch(selectedCluster, async () => {
  await loadNamespaces()
  await fetchResources()
})

watch([selectedNamespace, selectedKind], () => {
  fetchResources()
})

onMounted(async () => {
  if (route.query.cluster && typeof route.query.cluster === 'string') {
    selectedCluster.value = route.query.cluster
  }
  if (route.query.namespace && typeof route.query.namespace === 'string') {
    selectedNamespace.value = route.query.namespace
  }
  if (route.query.kind && typeof route.query.kind === 'string') {
    selectedKind.value = route.query.kind as ResourceKind
  }

  await loadClusters()
  await loadNamespaces()
  await fetchResources()
})

// Metrics computation
const totalInKind = computed(() => resources.value.length)
const activeNamespacesCount = computed(() => namespaces.value.length || 1)
const currentKindLabel = computed(() => {
  for (const cat of kindCategories) {
    for (const item of cat.items) {
      if (item.kind === selectedKind.value) return item.label
    }
  }
  return selectedKind.value
})

// Table columns
const columns: Column<K8sResource>[] = [
  { key: 'name', label: 'Resource Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '160px', sortable: true },
  { key: 'status', label: 'Status / Ready', width: '150px', sortable: true },
  { key: 'age', label: 'Age / Created', width: '150px', sortable: true },
  { key: 'actions', label: 'Actions', width: '220px', align: 'right' },
]

function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

function selectKind(kind: ResourceKind) {
  selectedKind.value = kind
}

function openCreateModal() {
  showCreateModal.value = true
}

function openApplyYamlModal() {
  yamlEditorMode.value = 'create'
  yamlEditorTitle.value = `Apply YAML to ${selectedCluster.value}`
  yamlEditorInitialContent.value = ''
  showYamlModal.value = true
}

function openResourceYaml(resource: K8sResource) {
  yamlEditorMode.value = 'edit'
  yamlEditorTitle.value = `Edit ${resource.kind}: ${resource.metadata?.name}`
  yamlEditorInitialContent.value = jsonToYaml(resource)
  showYamlModal.value = true
}

function openDetailDrawer(resource: K8sResource) {
  selectedResource.value = resource
  activeDrawerTab.value = 'overview'
  showDetailDrawer.value = true
}

function promptDelete(resource: K8sResource) {
  resourceToDelete.value = resource
  showDeleteModal.value = true
}

async function handleDeleteConfirmed() {
  if (!resourceToDelete.value) return
  const r = resourceToDelete.value
  const name = r.metadata?.name || ''
  const ns = r.metadata?.namespace || undefined

  deletingResource.value = true
  try {
    await k8sApi.deleteResource(selectedCluster.value, selectedKind.value, name, ns)
    showToast(`Resource ${r.kind}/${name} deleted successfully`)
    showDeleteModal.value = false
    resourceToDelete.value = null
    if (selectedResource.value?.metadata?.name === name) {
      showDetailDrawer.value = false
    }
    await fetchResources()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to delete resource'
    showToast(msg, 'error')
  } finally {
    deletingResource.value = false
  }
}

async function handleCreateNamespace() {
  if (!newNsName.value.trim()) {
    newNsError.value = 'Namespace name is required'
    return
  }

  creatingNs.value = true
  newNsError.value = null
  try {
    const created = await k8sApi.createNamespace(selectedCluster.value, newNsName.value.trim())
    showToast(`Namespace "${created.name || newNsName.value}" created!`)
    showNewNsModal.value = false
    newNsName.value = ''
    await loadNamespaces()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to create namespace'
    newNsError.value = msg
  } finally {
    creatingNs.value = false
  }
}

function onResourceCreated(res: K8sResource) {
  showToast(`Resource ${res.kind || ''}/${res.metadata?.name || 'new'} created successfully!`)
  fetchResources()
}

function onYamlApplied(res: { message: string }) {
  showToast(res.message || 'Manifest applied successfully!')
  fetchResources()
}

async function copyDrawerYaml() {
  if (!selectedResource.value) return
  try {
    await navigator.clipboard.writeText(jsonToYaml(selectedResource.value))
    drawerYamlCopied.value = true
    setTimeout(() => {
      drawerYamlCopied.value = false
    }, 2000)
  } catch {
    // fallback
  }
}

function getResourceStatus(resource: K8sResource): string {
  if (resource.status && typeof resource.status === 'object') {
    if ('phase' in resource.status && typeof resource.status.phase === 'string') {
      return resource.status.phase
    }
    if ('readyReplicas' in resource.status && 'replicas' in resource.status) {
      return `${resource.status.readyReplicas || 0}/${resource.status.replicas || 0} Ready`
    }
  }
  return 'Active'
}

function getResourceAge(resource: K8sResource): string {
  const ts = resource.metadata?.creationTimestamp
  if (!ts) return 'Just now'
  const diff = Date.now() - new Date(ts).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return `${days}d`
}
</script>

<template>
  <div class="explorer-layout animate-fade-in">
    <!-- Left Sidebar: Cluster, Namespace, Resource Kind Tree -->
    <aside class="explorer-sidebar glass-panel">
      <div class="sidebar-header">
        <div class="sidebar-brand">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span class="sidebar-title">K8s Explorer</span>
        </div>
      </div>

      <!-- Cluster Selector -->
      <div class="sidebar-section">
        <div class="section-heading-row">
          <label class="section-heading">Target Cluster</label>
          <button 
            type="button" 
            class="btn-icon-text" 
            title="Import Kubernetes Cluster"
            @click="showImportModal = true"
          >
            + Import
          </button>
        </div>
        <div class="cluster-select-wrapper">
          <select v-if="clusters.length > 0" v-model="selectedCluster" class="input-glass select-full cluster-select">
            <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">
              🌐 {{ c.name || c.id }}
            </option>
          </select>
          <div v-else class="cluster-empty-box">
            <span class="font-mono text-muted font-small">{{ selectedCluster || 'primary-cluster' }} (offline)</span>
          </div>
        </div>
      </div>

      <!-- Namespace Selector -->
      <div class="sidebar-section">
        <div class="section-heading-row">
          <label class="section-heading">Namespace</label>
          <button 
            type="button" 
            class="btn-icon-text" 
            title="Create new namespace"
            @click="showNewNsModal = true"
          >
            + New
          </button>
        </div>
        <select v-model="selectedNamespace" class="input-glass select-full ns-select">
          <option value="all">?? All Namespaces</option>
          <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">
            ?? {{ ns.name }}
          </option>
        </select>
      </div>

      <!-- Resource Kinds Tree -->
      <div class="sidebar-tree">
        <div v-for="category in kindCategories" :key="category.title" class="tree-category">
          <div class="category-title">
            <span>{{ category.icon }}</span>
            <span>{{ category.title }}</span>
          </div>
          <div class="category-items">
            <button
              v-for="item in category.items"
              :key="item.kind"
              type="button"
              class="tree-item-btn"
              :class="{ 'is-active': selectedKind === item.kind }"
              @click="selectKind(item.kind)"
            >
              <span class="item-icon">{{ item.icon }}</span>
              <span class="item-label">{{ item.label }}</span>
            </button>
          </div>
        </div>
      </div>
    </aside>

    <!-- Main Content Area -->
    <main class="explorer-main">
      <!-- Header Banner -->
      <div class="view-header">
        <div class="header-left">
          <div class="view-tag">
            <span class="pulse-dot pulse-dot-cyan"></span>
            <span>KUBERNETES RESOURCE MANAGER</span>
          </div>
          <h1 class="view-title">{{ currentKindLabel }}</h1>
          <div class="breadcrumbs font-mono">
            <span class="crumb-cluster">?? {{ selectedCluster }}</span>
            <span class="crumb-sep">/</span>
            <span class="crumb-ns">?? {{ selectedNamespace === 'all' ? 'All Namespaces' : selectedNamespace }}</span>
            <span class="crumb-sep">/</span>
            <span class="crumb-kind text-cyan">{{ currentKindLabel }}</span>
          </div>
        </div>

        <div class="header-actions">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="fetchResources">
            <span>{{ loading ? '? Refreshing...' : '?? Refresh' }}</span>
          </button>
          <button type="button" class="btn btn-secondary" @click="openApplyYamlModal">
            <span>?? Apply YAML</span>
          </button>
          <button type="button" class="btn btn-primary" @click="openCreateModal">
            <span>? + Create {{ currentKindLabel.slice(0, -1) || 'Resource' }}</span>
          </button>
        </div>
      </div>

      <!-- Notification Toast -->
      <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
        <span>{{ toastMessage.type === 'success' ? '?' : '??' }}</span>
        <span>{{ toastMessage.text }}</span>
        <button class="toast-close" @click="toastMessage = null">?</button>
      </div>

      <!-- Offline / Disconnected Warning Banner -->
      <div v-if="clusterOffline" class="offline-banner animate-fade-in">
        <span class="offline-icon">⚠️</span>
        <div class="offline-content">
          <strong class="offline-title">Kubernetes Cluster Disconnected</strong>
          <p class="offline-desc">
            Kubernetes Cluster Disconnected: No Kubernetes control plane is currently attached to '{{ selectedCluster || 'primary-cluster' }}'. Import a Kubeconfig or manage Docker Swarm workloads on /deployments.
          </p>
          <span v-if="offlineErrorMessage" class="offline-err-detail font-mono">{{ offlineErrorMessage }}</span>
          <div class="offline-actions">
            <button type="button" class="btn btn-primary btn-xs" @click="showImportModal = true">
              <span>+ Import Cluster (Kubeconfig)</span>
            </button>
            <router-link to="/deployments" class="btn btn-secondary btn-xs">
              <span>Manage Docker Swarm Workloads</span>
            </router-link>
            <button type="button" class="btn btn-secondary btn-xs" :disabled="loading" @click="fetchResources">
              <span>🔄 Retry Connection</span>
            </button>
          </div>
        </div>
      </div>

      <!-- HUD Metrics Cards -->
      <div class="metrics-grid">
        <MetricCard
          :title="`Total ${currentKindLabel}`"
          :value="totalInKind"
          :subtitle="`Discovered in namespace: ${selectedNamespace}`"
          icon="??"
          badge="DISCOVERED"
          badge-color="cyan"
        />
        <MetricCard
          title="Active Namespaces"
          :value="activeNamespacesCount"
          subtitle="Available workload domains"
          icon="??"
          badge="TENANCY"
          badge-color="emerald"
        />
        <MetricCard
          title="Cluster Target"
          :value="selectedCluster"
          subtitle="Kubernetes Control Plane"
          icon="??"
          badge="ONLINE"
          badge-color="violet"
        />
      </div>

      <!-- Data Table of Resources -->
      <div class="section-box glass-panel table-box">
        <DataTable
          :columns="columns"
          :data="resources"
          :loading="loading"
          :error="error"
          empty-message="No Kubernetes resources discovered matching current filters."
          searchable
          search-placeholder="Filter by name, namespace, or status..."
        >
          <template #cell-name="{ row }">
            <div class="resource-name-cell">
              <span class="res-icon">??</span>
              <a 
                href="javascript:void(0)" 
                class="res-link font-mono" 
                @click="openDetailDrawer(row)"
              >
                {{ row.metadata?.name || 'unnamed' }}
              </a>
              <span v-if="row.metadata?.labels?.app" class="app-tag font-mono">
                app: {{ row.metadata.labels.app }}
              </span>
            </div>
          </template>

          <template #cell-namespace="{ row }">
            <span class="ns-badge font-mono">
              {{ row.metadata?.namespace || 'cluster-scoped' }}
            </span>
          </template>

          <template #cell-status="{ row }">
            <StatusBadge :status="getResourceStatus(row)" size="sm" />
          </template>

          <template #cell-age="{ row }">
            <span class="font-mono text-muted">
              {{ getResourceAge(row) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="action-buttons">
              <button 
                type="button"
                class="btn btn-secondary btn-xs"
                title="View structured object details"
                @click="openDetailDrawer(row)"
              >
                <span>?? Details</span>
              </button>
              <button 
                type="button"
                class="btn btn-secondary btn-xs"
                title="Edit / View raw YAML"
                @click="openResourceYaml(row)"
              >
                <span>?? YAML</span>
              </button>
              <button 
                type="button"
                class="btn btn-danger btn-xs"
                title="Delete resource"
                @click="promptDelete(row)"
              >
                <span>???</span>
              </button>
            </div>
          </template>
        </DataTable>
      </div>
    </main>

    <!-- Detail Drawer (Structured View + Live YAML) -->
    <ModalDrawer
      v-model:show="showDetailDrawer"
      mode="drawer"
      :title="`${selectedResource?.kind || 'Resource'}: ${selectedResource?.metadata?.name || ''}`"
      :subtitle="`Cluster: ${selectedCluster} ? Namespace: ${selectedResource?.metadata?.namespace || 'cluster-scoped'}`"
      max-width="680px"
    >
      <div v-if="selectedResource" class="detail-drawer-content">
        <!-- Tabs Header -->
        <div class="drawer-tabs">
          <button 
            type="button" 
            class="tab-btn" 
            :class="{ 'is-active': activeDrawerTab === 'overview' }"
            @click="activeDrawerTab = 'overview'"
          >
            ?? Overview & Data
          </button>
          <button 
            type="button" 
            class="tab-btn" 
            :class="{ 'is-active': activeDrawerTab === 'yaml' }"
            @click="activeDrawerTab = 'yaml'"
          >
            ?? Live Manifest (YAML)
          </button>
        </div>

        <!-- Tab 1: Overview -->
        <div v-if="activeDrawerTab === 'overview'" class="tab-pane animate-fade-in">
          <!-- Metadata Card -->
          <div class="info-card glass-panel">
            <div class="card-title">Object Metadata</div>
            <div class="meta-grid">
              <div class="meta-item">
                <span class="meta-label">Name:</span>
                <span class="font-mono text-cyan">{{ selectedResource.metadata?.name }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Namespace:</span>
                <span class="font-mono">{{ selectedResource.metadata?.namespace || 'cluster-scoped' }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Kind:</span>
                <span class="font-mono">{{ selectedResource.kind }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">API Version:</span>
                <span class="font-mono text-muted">{{ selectedResource.apiVersion }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Created At:</span>
                <span class="font-mono text-muted">{{ selectedResource.metadata?.creationTimestamp || 'N/A' }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">UID:</span>
                <span class="font-mono text-muted font-small">{{ selectedResource.metadata?.uid || 'N/A' }}</span>
              </div>
            </div>
          </div>

          <!-- Labels Section -->
          <div v-if="selectedResource.metadata?.labels && Object.keys(selectedResource.metadata.labels).length > 0" class="info-card glass-panel">
            <div class="card-title">Labels</div>
            <div class="labels-wrap">
              <span 
                v-for="(val, key) in selectedResource.metadata.labels" 
                :key="key" 
                class="label-badge font-mono"
              >
                {{ key }}: {{ val }}
              </span>
            </div>
          </div>

          <!-- If Secret: Embed SecretViewer -->
          <div v-if="selectedResource.kind === 'Secret' || selectedKind === 'secrets'" class="secret-embed-section">
            <div class="card-title">Secret Credentials & Keys</div>
            <SecretViewer :secret="selectedResource" />
          </div>

          <!-- If ConfigMap: Show Key-Value Pairs -->
          <div v-else-if="selectedResource.data && Object.keys(selectedResource.data).length > 0" class="info-card glass-panel">
            <div class="card-title">ConfigMap Data</div>
            <div class="kv-display-table">
              <div v-for="(val, key) in selectedResource.data" :key="key" class="kv-item">
                <div class="kv-key font-mono">{{ key }}</div>
                <div class="kv-val font-mono">{{ val }}</div>
              </div>
            </div>
          </div>

          <!-- Spec Details -->
          <div v-if="selectedResource.spec && Object.keys(selectedResource.spec).length > 0" class="info-card glass-panel">
            <div class="card-title">Spec Configuration</div>
            <pre class="spec-pre font-mono">{{ jsonToYaml(selectedResource.spec) }}</pre>
          </div>
        </div>

        <!-- Tab 2: YAML -->
        <div v-else class="tab-pane animate-fade-in">
          <div class="yaml-pane-toolbar">
            <span class="text-muted font-mono font-small">apiVersion: {{ selectedResource.apiVersion }}</span>
            <div class="pane-actions">
              <button 
                type="button" 
                class="btn btn-secondary btn-xs"
                :class="{ 'btn-copied': drawerYamlCopied }"
                @click="copyDrawerYaml"
              >
                <span>{{ drawerYamlCopied ? '? Copied!' : '?? Copy YAML' }}</span>
              </button>
              <button 
                type="button" 
                class="btn btn-primary btn-xs"
                @click="openResourceYaml(selectedResource)"
              >
                <span>?? Edit in Editor</span>
              </button>
            </div>
          </div>
          <div class="yaml-code-wrapper font-mono">
            <pre class="yaml-pre">{{ jsonToYaml(selectedResource) }}</pre>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" @click="close">Close</button>
        <button 
          v-if="selectedResource"
          type="button" 
          class="btn btn-danger" 
          @click="promptDelete(selectedResource)"
        >
          <span>??? Delete Resource</span>
        </button>
        <button 
          v-if="selectedResource"
          type="button" 
          class="btn btn-primary" 
          @click="openResourceYaml(selectedResource)"
        >
          <span>?? Edit Manifest</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Create Resource Modal -->
    <CreateResourceModal
      v-model:show="showCreateModal"
      :cluster="selectedCluster"
      :namespaces="namespaces"
      :default-namespace="selectedNamespace !== 'all' ? selectedNamespace : 'default'"
      :default-kind="selectedKind"
      @created="onResourceCreated"
    />

    <!-- YAML Editor Modal -->
    <YamlEditorModal
      v-model:show="showYamlModal"
      :cluster="selectedCluster"
      :namespace="selectedNamespace !== 'all' ? selectedNamespace : 'default'"
      :title="yamlEditorTitle"
      :kind="currentKindLabel"
      :initial-yaml="yamlEditorInitialContent"
      :mode="yamlEditorMode"
      @applied="onYamlApplied"
    />

    <!-- New Namespace Modal -->
    <ModalDrawer
      v-model:show="showNewNsModal"
      mode="modal"
      title="Create Namespace"
      :subtitle="`Cluster: ${selectedCluster}`"
      max-width="480px"
    >
      <div class="form-group">
        <label class="form-label required">Namespace Name</label>
        <input 
          v-model="newNsName"
          type="text" 
          class="input-glass font-mono" 
          placeholder="e.g. stage-environment" 
          @keydown.enter="handleCreateNamespace"
        />
        <p v-if="newNsError" class="field-error">{{ newNsError }}</p>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="creatingNs" @click="close">Cancel</button>
        <button 
          type="button" 
          class="btn btn-primary" 
          :disabled="creatingNs || !newNsName.trim()"
          @click="handleCreateNamespace"
        >
          <span>{{ creatingNs ? 'Creating...' : 'Create Namespace' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Delete Confirmation Modal -->
    <ModalDrawer
      v-model:show="showDeleteModal"
      mode="modal"
      title="Confirm Resource Deletion"
      subtitle="This action is permanent and cannot be undone."
      max-width="480px"
    >
      <div v-if="resourceToDelete" class="delete-dialog-content">
        <p class="delete-msg">
          Are you sure you want to delete 
          <strong>{{ resourceToDelete.kind }}/{{ resourceToDelete.metadata?.name }}</strong> 
          in namespace 
          <strong>{{ resourceToDelete.metadata?.namespace || 'default' }}</strong>?
        </p>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="deletingResource" @click="close">Cancel</button>
        <button 
          type="button" 
          class="btn btn-danger" 
          :disabled="deletingResource"
          @click="handleDeleteConfirmed"
        >
          <span>{{ deletingResource ? 'Deleting...' : 'Confirm Delete' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Import Cluster Modal -->
    <ModalDrawer
      v-model:show="showImportModal"
      mode="modal"
      title="Import Kubernetes Cluster"
      subtitle="Upload cluster kubeconfig to establish secure telemetry bridge"
      max-width="600px"
    >
      <div class="modal-form">
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label required">Cluster Identifier</label>
            <input v-model="importForm.id" type="text" placeholder="e.g. k8s-prod-us" class="input-glass font-mono" />
          </div>
          <div class="form-group flex-1">
            <label class="form-label required">Cluster Display Name</label>
            <input v-model="importForm.name" type="text" placeholder="e.g. Production US-East" class="input-glass" />
          </div>
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Fleet Tier</label>
            <select v-model="importForm.group" class="input-glass">
              <option value="production">Production</option>
              <option value="staging">Staging</option>
              <option value="development">Development</option>
              <option value="edge">Edge & Remote</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Provider</label>
            <select v-model="importForm.provider" class="input-glass">
              <option value="k8s">Generic K8s / Self-Hosted</option>
              <option value="aws">AWS (EKS)</option>
              <option value="gcp">Google Cloud (GKE)</option>
              <option value="azure">Azure (AKS)</option>
              <option value="baremetal">Bare Metal</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Region / Datacenter</label>
          <input v-model="importForm.region" type="text" placeholder="e.g. us-east-1 / On-Prem DC-01" class="input-glass" />
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

          <div v-if="importMode === 'file'" class="file-drop-area">
            <label class="form-label">Kubeconfig File (.yaml / .config / .json) *</label>
            <input type="file" accept=".yaml,.yml,.config,.json" class="file-input" @change="handleFileChange" />
          </div>

          <div v-else class="text-paste-area">
            <label class="form-label">Paste Kubeconfig YAML Content *</label>
            <textarea
              v-model="importForm.kubeconfigRaw"
              rows="6"
              class="input-glass font-mono text-area-input select-full"
              placeholder="apiVersion: v1&#10;clusters:&#10;..."
            ></textarea>
          </div>
          <span class="form-hint">🔒 Kubeconfig is encrypted with AES-256 GCM in local vault before persistence. Sensitive tokens are never returned in cleartext.</span>
        </div>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="importingCluster" @click="close">Cancel</button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="importingCluster || !importForm.name.trim()"
          @click="handleImportCluster"
        >
          <span>{{ importingCluster ? 'Importing...' : 'Import Cluster' }}</span>
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.explorer-layout {
  display: flex;
  gap: 24px;
  min-height: calc(100vh - 120px);
}

.explorer-sidebar {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px;
  border-radius: 16px;
}

.sidebar-header {
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-subtle);
}

.sidebar-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.sidebar-title {
  font-size: 15px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.01em;
}

.sidebar-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-heading {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.section-heading-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.btn-icon-text {
  background: none;
  border: none;
  color: var(--accent-sky);
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
}

.btn-icon-text:hover {
  text-decoration: underline;
}

.select-full {
  width: 100%;
}

.sidebar-tree {
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
  flex: 1;
}

.tree-category {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.category-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 0 4px;
}

.category-items {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tree-item-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  text-align: left;
  transition: all 0.15s ease;
}

.tree-item-btn:hover {
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-primary);
}

.tree-item-btn.is-active {
  background: rgba(6, 182, 212, 0.12);
  border-color: rgba(6, 182, 212, 0.3);
  color: #38bdf8;
  font-weight: 600;
}

.item-icon {
  font-size: 14px;
}

.explorer-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
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
  margin-bottom: 4px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 4px;
}

.crumb-sep {
  color: var(--text-muted);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
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

.cluster-empty-box {
  padding: 8px 10px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px dashed var(--border-subtle);
  border-radius: 8px;
}

.offline-banner {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 18px 22px;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.35);
  border-radius: 14px;
  color: #fb7185;
}

.offline-icon {
  font-size: 26px;
  line-height: 1;
}

.offline-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.offline-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.offline-desc {
  font-size: 13px;
  color: #fecdd3;
  line-height: 1.5;
  margin: 0;
}

.offline-err-detail {
  font-size: 11px;
  color: #fda4af;
  background: rgba(0, 0, 0, 0.3);
  padding: 4px 8px;
  border-radius: 6px;
  word-break: break-all;
}

.offline-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
  flex-wrap: wrap;
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
  margin-bottom: 8px;
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

.file-input {
  font-size: 12px;
  color: var(--text-secondary);
}

.text-area-input {
  resize: vertical;
  min-height: 120px;
  font-size: 12px;
  line-height: 1.4;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.section-box {
  border-radius: 16px;
  overflow: hidden;
}

.resource-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.res-icon {
  font-size: 14px;
}

.res-link {
  color: var(--accent-cyan);
  font-weight: 700;
  text-decoration: none;
}

.res-link:hover {
  text-decoration: underline;
}

.app-tag {
  font-size: 10px;
  background: rgba(255, 255, 255, 0.06);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-muted);
}

.ns-badge {
  font-size: 11px;
  color: var(--text-secondary);
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.detail-drawer-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.drawer-tabs {
  display: flex;
  gap: 8px;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 12px;
}

.tab-btn {
  padding: 8px 16px;
  background: transparent;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tab-btn:hover {
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-primary);
}

.tab-btn.is-active {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
  color: #38bdf8;
}

.tab-pane {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-card {
  padding: 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.meta-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}

.meta-label {
  font-size: 11px;
  color: var(--text-muted);
}

.font-small {
  font-size: 10px;
  word-break: break-all;
}

.labels-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.label-badge {
  font-size: 11px;
  padding: 3px 8px;
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.25);
  border-radius: 6px;
  color: #38bdf8;
}

.kv-display-table {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.kv-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  background: rgba(11, 15, 25, 0.6);
  border-radius: 6px;
}

.kv-key {
  font-size: 11px;
  color: var(--accent-cyan);
  font-weight: bold;
}

.kv-val {
  font-size: 12px;
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-all;
}

.spec-pre {
  background: #030712;
  padding: 12px;
  border-radius: 8px;
  font-size: 11.5px;
  color: #38bdf8;
  max-height: 240px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.yaml-pane-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.pane-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.yaml-code-wrapper {
  background: #030712;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  overflow: hidden;
}

.yaml-pre {
  padding: 16px;
  font-size: 12px;
  line-height: 1.6;
  color: #e0f2fe;
  max-height: 520px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.delete-dialog-content {
  padding: 8px 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-secondary);
}

.delete-msg strong {
  color: var(--text-primary);
}

.field-error {
  color: #fb7185;
  font-size: 12px;
  margin-top: 4px;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }

@media (max-width: 1024px) {
  .explorer-layout {
    flex-direction: column;
  }
  .explorer-sidebar {
    width: 100%;
  }
}

@media (max-width: 768px) {
  .view-header {
    flex-direction: column;
    align-items: stretch;
    gap: 14px;
  }

  .header-actions {
    flex-wrap: wrap;
    width: 100%;
    gap: 8px;
  }

  .header-actions .btn {
    flex: 1 1 auto;
    justify-content: center;
  }

  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
  }

  .sidebar-tree {
    flex-direction: row;
    overflow-x: auto;
    scrollbar-width: thin;
    -webkit-overflow-scrolling: touch;
    gap: 12px;
    padding-bottom: 8px;
  }

  .tree-category {
    flex-direction: row;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  .category-items {
    flex-direction: row;
    gap: 4px;
    flex-shrink: 0;
  }

  .tree-item-btn {
    white-space: nowrap;
    padding: 6px 10px;
    font-size: 12px;
  }

  .yaml-pane-toolbar {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .pane-actions {
    width: 100%;
    justify-content: flex-end;
  }
}

@media (max-width: 640px) {
  .explorer-sidebar {
    padding: 14px;
    gap: 14px;
    border-radius: 12px;
  }

  .metrics-grid {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  .breadcrumbs {
    flex-wrap: wrap;
    font-size: 11px;
    gap: 4px;
  }

  .header-actions .btn {
    flex: 1 1 100%;
    width: 100%;
  }

  .meta-grid {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .yaml-code-wrapper {
    overflow-x: auto;
  }

  .yaml-pre,
  .spec-pre {
    height: 350px;
    max-height: 350px;
    font-size: 11px;
    padding: 10px;
    white-space: pre;
    overflow-x: auto;
    word-break: normal;
  }

  .drawer-tabs {
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: thin;
    -webkit-overflow-scrolling: touch;
  }

  .tab-btn {
    white-space: nowrap;
    flex: 1;
    text-align: center;
  }
}
</style>
