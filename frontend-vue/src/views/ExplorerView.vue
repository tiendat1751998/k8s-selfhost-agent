<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import MetricCard from '../components/ui/MetricCard.vue'
import EventsTimeline from '../components/k8s/EventsTimeline.vue'

import ExplorerResourceTable from '../components/explorer/ExplorerResourceTable.vue'
import ExplorerMobileCards from '../components/explorer/ExplorerMobileCards.vue'
import ExplorerDetailDrawer from '../components/explorer/ExplorerDetailDrawer.vue'
import ExplorerCreateModal from '../components/explorer/ExplorerCreateModal.vue'
import ExplorerScaleModal from '../components/explorer/ExplorerScaleModal.vue'
import ExplorerRestartModal from '../components/explorer/ExplorerRestartModal.vue'
import ExplorerDeleteModal from '../components/explorer/ExplorerDeleteModal.vue'
import ExplorerDrainModal from '../components/explorer/ExplorerDrainModal.vue'
import ApplyYamlModal from '../components/explorer/ApplyYamlModal.vue'
import ExplorerImportModal from '../components/explorer/ExplorerImportModal.vue'
import ExplorerCreateNsModal from '../components/explorer/ExplorerCreateNsModal.vue'
import PodLogsDrawer from '../components/explorer/PodLogsDrawer.vue'
import PodTerminalDrawer from '../components/explorer/PodTerminalDrawer.vue'
import { useK8sExplorer } from '../composables/useK8sExplorer'
import type { ResourceKind } from '../api/k8s'
import '../assets/styles/views/explorer.css'

const route = useRoute()
const {
  loading,
  error,
  toastMessage,
  clusterOffline,
  offlineErrorMessage,
  clusters,
  selectedCluster,
  namespaces,
  selectedNamespace,
  selectedKind,
  resources,
  filteredKindCategories,
  allKindItems,
  totalInKind,
  activeNamespacesCount,
  currentKindLabel,
  columns,
  loadClusters,
  loadNamespaces,
  fetchResources,
  selectKind,
  showCreateModal,
  showYamlModal,
  yamlEditorMode,
  yamlEditorInitialContent,
  yamlEditorTitle,
  showDetailDrawer,
  selectedResource,
  showScaleModal,
  scaleTarget,
  scalingResource,
  showRestartModal,
  restartTarget,
  restartingResource,
  showDeleteModal,
  resourceToDelete,
  deletingResource,
  showDrainModal,
  drainTargetNode,
  drainingNode,
  operatingNode,
  showNewNsModal,
  creatingNs,
  newNsError,
  showImportModal,
  importingCluster,
  showLogsDrawer,
  logsPod,
  showTerminalDrawer,
  terminalPod,
  openDetailDrawer,
  openLogsDrawer,
  openTerminalDrawer,
  openScaleModal,
  openRestartModal,
  openDeleteModal,
  openDrainModal,
  openApplyYamlModal,
  openYamlEditModal,
  handleScaleConfirm,
  handleRestartConfirm,
  handleDeleteConfirm,
  handleDrainConfirm,
  handleCordonNode,
  handleUncordonNode,
  handleTriggerCronJob,
  handleToggleSuspend,
  handleImportCluster,
  handleCreateNs,
  handleYamlApplied,
  handleCreateApplied,
} = useK8sExplorer()

const showMobileSearch = ref(false)
const searchQuery = ref('')

const filteredResources = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return resources.value
  return resources.value.filter((r) => {
    const name = r.metadata?.name?.toLowerCase() || ''
    const ns = r.metadata?.namespace?.toLowerCase() || ''
    return name.includes(q) || ns.includes(q)
  })
})

watch(selectedCluster, async () => {
  await loadNamespaces()
  await fetchResources()
})
watch([selectedNamespace, selectedKind], () => {
  fetchResources()
})

onMounted(async () => {
  if (typeof route.query.cluster === 'string') selectedCluster.value = route.query.cluster
  if (typeof route.query.namespace === 'string') selectedNamespace.value = route.query.namespace
  if (typeof route.query.kind === 'string') selectedKind.value = route.query.kind as ResourceKind
  await loadClusters()
  await loadNamespaces()
  await fetchResources()
})
</script>

<template>
  <div class="explorer-layout animate-fade-in">
    <!-- Top Resource Bar (Replaces Sidebar) -->
    <div class="top-resource-bar glass-panel desktop-only">
      <div class="top-selectors">
        <div class="selector-group">
          <span class="selector-label">Cluster</span>
          <div class="selector-input-wrap">
            <select v-model="selectedCluster" class="input-glass top-select font-mono">
              <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">?? {{ c.name || c.id }}</option>
            </select>
            <button type="button" class="btn-icon" @click="showImportModal = true" title="Import Cluster">+</button>
          </div>
        </div>
        <div class="selector-divider"></div>
        <div class="selector-group">
          <span class="selector-label">Namespace</span>
          <div class="selector-input-wrap">
            <select v-model="selectedNamespace" class="input-glass top-select font-mono">
              <option value="all">?? All Namespaces</option>
              <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">?? {{ ns.name }}</option>
            </select>
            <button type="button" class="btn-icon" @click="showNewNsModal = true" title="New Namespace">+</button>
          </div>
        </div>
        <div class="selector-divider"></div>
        <div class="selector-group kind-selectors">
          <span class="selector-label">Resource</span>
          <div class="kind-dropdowns">
            <select
              v-for="category in filteredKindCategories"
              :key="category.title"
              class="input-glass top-select font-mono dropdown-selector"
              :class="{ 'active-category': category.items.some(i => i.kind === selectedKind) }"
              @change="selectKind(($event.target as HTMLSelectElement).value as any)"
            >
              <option value="" disabled :selected="!category.items.some(i => i.kind === selectedKind)">{{ category.title }} ?</option>
              <option v-for="item in category.items" :key="item.kind" :value="item.kind" :selected="selectedKind === item.kind">
                {{ item.label }}
              </option>
            </select>
          </div>
        </div>
      </div>
    </div>

    <main class="explorer-main">
      <!-- Mobile 44px Command Bar (< 640px) -->
      <div class="explorer-mobile-command-bar">
        <div class="mobile-command-brand">
          <span class="mobile-command-title font-mono">☸️ {{ currentKindLabel }}</span>
          <span class="mobile-count-badge font-mono">({{ totalInKind }})</span>
        </div>
        <div class="mobile-command-actions">
          <button type="button" class="btn-mobile-cmd" title="Toggle Search" aria-label="Search" @click="showMobileSearch = !showMobileSearch">
            🔍
          </button>
          <button type="button" class="btn-mobile-cmd" title="Refresh" aria-label="Refresh" :disabled="loading" @click="fetchResources">
            {{ loading ? '⏳' : '🔄' }}
          </button>
          <button type="button" class="btn-mobile-cmd" title="Apply YAML" aria-label="Apply YAML" @click="openApplyYamlModal">
            📄
          </button>
          <button type="button" class="btn-mobile-cmd" title="Create Resource" aria-label="Create Resource" @click="showCreateModal = true">
            ➕
          </button>
          
        </div>
      </div>

      <!-- Mobile Search Strip (toggled by 🔍) -->
      <div v-if="showMobileSearch" class="mobile-search-strip">
        <input v-model="searchQuery" type="text" placeholder="Filter resources..." class="input-glass mobile-search-input font-mono" />
      </div>

      <!-- Mobile Micro-Telemetry (20px) -->
      <div class="mobile-micro-telemetry font-mono">
        🌐 {{ selectedCluster }} · 📁 {{ selectedNamespace }} · 📦 {{ totalInKind }} {{ currentKindLabel }}
      </div>

      <!-- Mobile Horizontal Kind Scroller -->
      <div class="mobile-kind-scroller">
        <button
          v-for="item in allKindItems"
          :key="item.kind"
          type="button"
          class="mobile-kind-pill"
          :class="{ 'is-active': selectedKind === item.kind }"
          @click="selectKind(item.kind)"
        >
          <span class="pill-label font-mono">{{ item.label }}</span>
        </button>
      </div>

      <!-- Desktop Header & 3 KPI Cards wrapped in .desktop-header-wrap (Hidden on <640px) -->
      <div class="desktop-header-wrap">
        <div class="view-header glass-panel header-banner">
          <div class="header-left">
            <div class="view-tag">
              <span class="pulse-dot pulse-dot-cyan"></span>
              <span class="tag-title">KUBERNETES CONTROL PLANE</span>
              <span class="status-live-chip">LIVE</span>
            </div>
            <div class="title-with-icon">
              <h1 class="view-title font-sans">{{ currentKindLabel }}</h1>
            </div>
            <div class="breadcrumbs font-mono">
              <span class="crumb-pill crumb-cluster">🌐 {{ selectedCluster }}</span>
              <span class="crumb-sep">›</span>
              <span class="crumb-pill crumb-ns">📁 {{ selectedNamespace === 'all' ? 'All Namespaces' : selectedNamespace }}</span>
              <span class="crumb-sep">›</span>
              <span class="crumb-pill crumb-kind active-kind">{{ currentKindLabel }} ({{ totalInKind }})</span>
            </div>
          </div>

          <div class="header-actions">
            <button type="button" class="btn btn-secondary btn-header" :disabled="loading" @click="fetchResources">
              <span class="btn-emoji">{{ loading ? '⏳' : '🔄' }}</span>
              <span class="btn-label">{{ loading ? 'Syncing...' : 'Refresh' }}</span>
            </button>
            <button type="button" class="btn btn-secondary btn-header btn-yaml" @click="openApplyYamlModal">
              <span class="btn-emoji">📄</span>
              <span class="btn-label">Apply YAML</span>
            </button>
            <button type="button" class="btn btn-primary btn-header btn-create" @click="showCreateModal = true">
              <span class="btn-emoji">✨</span>
              <span class="btn-label">+ Create {{ currentKindLabel.slice(0, -1) || 'Resource' }}</span>
            </button>
          </div>
        </div>

        <div class="metrics-grid">
          <MetricCard :title="`Total ${currentKindLabel}`" :value="totalInKind" :subtitle="`Discovered in scope: ${selectedNamespace}`" icon="📦" badge="DISCOVERED" badge-color="cyan" class="hud-metric-card" />
          <MetricCard title="Active Namespaces" :value="activeNamespacesCount" subtitle="Available workload domains" icon="📁" badge="TENANCY" badge-color="emerald" class="hud-metric-card" />
          <MetricCard title="Cluster Target" :value="selectedCluster" subtitle="Kubernetes Control Plane" icon="🌐" badge="ONLINE" badge-color="violet" class="hud-metric-card" />
        </div>
      </div>

      <!-- Toast & Offline Notifications -->
      <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
        <span class="toast-icon">{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
        <span class="toast-text font-mono font-small">{{ toastMessage.text }}</span>
        <button class="toast-close" aria-label="Dismiss toast" @click="toastMessage = null">✕</button>
      </div>

      <div v-if="clusterOffline" class="offline-banner animate-fade-in">
        <span class="offline-icon">⚠️</span>
        <div class="offline-content">
          <strong class="offline-title">Kubernetes Cluster Disconnected</strong>
          <p class="offline-desc">No active Kubernetes control plane is attached to '{{ selectedCluster || 'primary-cluster' }}'. Import a valid Kubeconfig or manage Docker Swarm.</p>
          <span v-if="offlineErrorMessage" class="offline-err-detail font-mono">{{ offlineErrorMessage }}</span>
          <div class="offline-actions">
            <button type="button" class="btn btn-primary btn-xs" @click="showImportModal = true">+ Import Cluster</button>
            <router-link to="/deployments" class="btn btn-secondary btn-xs">Manage Docker Swarm</router-link>
            <button type="button" class="btn btn-secondary btn-xs" :disabled="loading" @click="fetchResources">🔄 Retry</button>
          </div>
        </div>
      </div>

      <!-- Events View or Table / Mobile Cards -->
      <div v-if="selectedKind === 'events'" class="events-main-section animate-fade-in">
        <EventsTimeline :cluster="selectedCluster" :namespace="selectedNamespace" :auto-refresh="true" />
      </div>

      <div v-else class="explorer-resources-wrapper">
        <div class="explorer-desktop-table">
          <ExplorerResourceTable
            :columns="columns"
            :resources="filteredResources"
            :loading="loading"
            :error="error"
            :selected-kind="selectedKind"
            :operating-node="operatingNode"
            @detail="openDetailDrawer"
            @logs="openLogsDrawer"
            @terminal="openTerminalDrawer"
            @yaml="openYamlEditModal"
            @scale="openScaleModal"
            @restart="openRestartModal"
            @trigger-cronjob="handleTriggerCronJob"
            @toggle-suspend="handleToggleSuspend"
            @cordon="handleCordonNode"
            @uncordon="handleUncordonNode"
            @drain="openDrainModal"
            @delete="openDeleteModal"
          />
        </div>

        <div class="explorer-mobile-cards">
          <ExplorerMobileCards
            :resources="filteredResources"
            :selected-kind="selectedKind"
            :loading="loading"
            @select="openDetailDrawer"
            @logs="openLogsDrawer"
            @scale="openScaleModal"
            @restart="openRestartModal"
            @delete="openDeleteModal"
          />
        </div>
      </div>
    </main>

    <!-- Subcomponent Modals and Drawers -->
    <ExplorerDetailDrawer
      :show="showDetailDrawer"
      :resource="selectedResource"
      :cluster="selectedCluster"
      :selected-kind="selectedKind"
      @close="showDetailDrawer = false"
      @scale="openScaleModal"
      @restart="openRestartModal"
      @edit-yaml="openYamlEditModal"
    />
    <ExplorerScaleModal :show="showScaleModal" :target="scaleTarget" :scaling="scalingResource" @close="showScaleModal = false" @confirm="handleScaleConfirm" />
    <ExplorerRestartModal :show="showRestartModal" :target="restartTarget" :restarting="restartingResource" @close="showRestartModal = false" @confirm="handleRestartConfirm" />
    <ExplorerDeleteModal :show="showDeleteModal" :resource="resourceToDelete" :deleting="deletingResource" @close="showDeleteModal = false" @confirm="handleDeleteConfirm" />
    <ExplorerDrainModal :show="showDrainModal" :target-node="drainTargetNode" :draining="drainingNode" @close="showDrainModal = false" @confirm="handleDrainConfirm" />
    <ApplyYamlModal :show="showYamlModal" :cluster="selectedCluster" :namespace="selectedNamespace" :initial-yaml="yamlEditorInitialContent" :title="yamlEditorTitle" :mode="yamlEditorMode" @close="showYamlModal = false" @applied="handleYamlApplied" />
    <ExplorerCreateModal :show="showCreateModal" :cluster="selectedCluster" :namespaces="namespaces" :default-namespace="selectedNamespace === 'all' ? 'default' : selectedNamespace" :default-kind="selectedKind" @close="showCreateModal = false" @applied="handleCreateApplied" />
    <ExplorerImportModal :show="showImportModal" :importing="importingCluster" @close="showImportModal = false" @import="handleImportCluster" />
    <ExplorerCreateNsModal :show="showNewNsModal" :cluster="selectedCluster" :creating="creatingNs" :error="newNsError" @close="showNewNsModal = false" @create="handleCreateNs" />
    <PodLogsDrawer :show="showLogsDrawer" :pod="logsPod" :cluster="selectedCluster" @close="showLogsDrawer = false" />
    <PodTerminalDrawer :show="showTerminalDrawer" :pod="terminalPod" :cluster="selectedCluster" @close="showTerminalDrawer = false" />
  </div>
</template>
