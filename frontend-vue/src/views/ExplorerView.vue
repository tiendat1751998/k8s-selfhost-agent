<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import BaseIcon from '../components/ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../components/ui/ActionDropdown.vue'
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
import { useK8sExplorer, getResourceStatus } from '../composables/useK8sExplorer'
import { useGlobalContext } from '../composables/useGlobalContext'
import type { ResourceKind } from '../api/k8s'

const route = useRoute()
const {
  loading, error, toastMessage, clusterOffline, offlineErrorMessage,
  selectedCluster, namespaces, selectedNamespace, selectedKind, resources,
  allKindItems, totalInKind, currentKindLabel, columns,
  loadClusters, loadNamespaces, fetchResources, selectKind,
  showCreateModal, showYamlModal, yamlEditorMode, yamlEditorInitialContent, yamlEditorTitle,
  showDetailDrawer, selectedResource, showScaleModal, scaleTarget, scalingResource,
  showRestartModal, restartTarget, restartingResource, showDeleteModal, resourceToDelete, deletingResource,
  showDrainModal, drainTargetNode, drainingNode, operatingNode, showNewNsModal, creatingNs, newNsError,
  showImportModal, importingCluster, showLogsDrawer, logsPod, showTerminalDrawer, terminalPod,
  openDetailDrawer, openLogsDrawer, openTerminalDrawer, openScaleModal, openRestartModal, openDeleteModal,
  openDrainModal, openApplyYamlModal, openYamlEditModal,
  handleScaleConfirm, handleRestartConfirm, handleDeleteConfirm, handleDrainConfirm,
  handleCordonNode, handleUncordonNode, handleTriggerCronJob, handleToggleSuspend,
  handleImportCluster, handleCreateNs, handleYamlApplied, handleCreateApplied,
} = useK8sExplorer()

const { activeClusterId, activeNamespace } = useGlobalContext()

// Sync with Global Context Top HUD
watch(activeClusterId, (newCluster) => {
  if (newCluster && selectedCluster.value !== newCluster) {
    selectedCluster.value = newCluster
  }
})

watch(activeNamespace, (newNs) => {
  if (newNs && selectedNamespace.value !== newNs) {
    selectedNamespace.value = newNs
  }
})

// Primary Kinds for Linear Segmented Control
const primaryKinds: { label: string; kind: ResourceKind }[] = [
  { label: 'Pods', kind: 'pods' },
  { label: 'Deployments', kind: 'deployments' },
  { label: 'StatefulSets', kind: 'statefulsets' },
  { label: 'DaemonSets', kind: 'daemonsets' },
  { label: 'ConfigMaps', kind: 'configmaps' },
  { label: 'Services', kind: 'services' },
]

// Secondary Kinds in Dropdown
const secondaryKinds: { label: string; kind: ResourceKind; icon: string }[] = [
  { label: 'Jobs', kind: 'jobs', icon: 'clock' },
  { label: 'CronJobs', kind: 'cronjobs', icon: 'clock' },
  { label: 'Secrets', kind: 'secrets', icon: 'shield' },
  { label: 'Ingresses', kind: 'ingresses', icon: 'globe' },
  { label: 'Nodes', kind: 'nodes', icon: 'server' },
  { label: 'Events', kind: 'events', icon: 'activity' },
  { label: 'PersistentVolumeClaims', kind: 'persistentvolumeclaims', icon: 'database' },
  { label: 'HorizontalPodAutoscalers', kind: 'horizontalpodautoscalers', icon: 'layers' },
  { label: 'NetworkPolicies', kind: 'networkpolicies', icon: 'shield' },
]

const moreActions = computed<ActionItem[]>(() => [
  ...secondaryKinds.map((k) => ({
    id: `kind:${k.kind}`,
    label: k.label,
    icon: k.icon,
  })),
  { id: 'sep-actions', label: '', separator: true },
  { id: 'refresh', label: 'Refresh', icon: 'refresh' },
  { id: 'import-cluster', label: 'Import Cluster', icon: 'cloud' },
  { id: 'new-namespace', label: 'New Namespace', icon: 'plus' },
])

function handleMoreActionSelect(actionId: string) {
  if (actionId.startsWith('kind:')) {
    const kind = actionId.replace('kind:', '') as ResourceKind
    selectKind(kind)
  } else if (actionId === 'refresh') {
    fetchResources()
  } else if (actionId === 'import-cluster') {
    showImportModal.value = true
  } else if (actionId === 'new-namespace') {
    showNewNsModal.value = true
  }
}

const showMobileSearch = ref(false)
const searchQuery = ref('')

const filteredResources = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return resources.value
  return resources.value.filter((r) => {
    const name = r.metadata?.name?.toLowerCase() || ''
    const ns = r.metadata?.namespace?.toLowerCase() || ''
    const status = getResourceStatus(r).toLowerCase()
    return name.includes(q) || ns.includes(q) || status.includes(q)
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
  if (typeof route.query.cluster === 'string') {
    selectedCluster.value = route.query.cluster
  } else if (activeClusterId.value) {
    selectedCluster.value = activeClusterId.value
  }
  if (typeof route.query.namespace === 'string') {
    selectedNamespace.value = route.query.namespace
  } else if (activeNamespace.value) {
    selectedNamespace.value = activeNamespace.value
  }
  if (typeof route.query.kind === 'string') selectedKind.value = route.query.kind as ResourceKind
  await loadClusters()
  await loadNamespaces()
  await fetchResources()
})
</script>

<template>
  <div class="explorer-layout animate-fade-in">
    <main class="explorer-main">
      <!-- Mobile 44px Command Bar (< 640px) -->
      <div class="explorer-mobile-command-bar">
        <div class="mobile-command-brand">
          <span class="mobile-command-title font-mono"><BaseIcon name="anchor" size="xs" /> {{ currentKindLabel }}</span>
          <span class="mobile-count-badge font-mono">({{ totalInKind }})</span>
        </div>
        <div class="mobile-command-actions">
          <button type="button" class="btn-mobile-cmd" title="Toggle Search" aria-label="Search" @click="showMobileSearch = !showMobileSearch"><BaseIcon name="search" size="xs" /></button>
          <button type="button" class="btn-mobile-cmd" title="Refresh" aria-label="Refresh" :disabled="loading" @click="fetchResources"><BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" /></button>
          <button type="button" class="btn-mobile-cmd" title="Apply YAML" aria-label="Apply YAML" @click="openApplyYamlModal"><BaseIcon name="file-text" size="xs" /></button>
          <button type="button" class="btn-mobile-cmd" title="Create Resource" aria-label="Create Resource" @click="showCreateModal = true"><BaseIcon name="plus" size="xs" /></button>
        </div>
      </div>

      <!-- Mobile Search Strip (toggled by search) -->
      <div v-if="showMobileSearch" class="mobile-search-strip">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input v-model="searchQuery" type="text" placeholder="Filter by name, namespace, status..." class="input-glass mobile-search-input font-mono" />
      </div>

      <!-- Mobile Micro-Telemetry (20px) -->
      <div class="mobile-micro-telemetry font-mono">
        <BaseIcon name="globe" size="xs" /> {{ selectedCluster }} · <BaseIcon name="folder" size="xs" /> {{ selectedNamespace }} · <BaseIcon name="box" size="xs" /> {{ totalInKind }} {{ currentKindLabel }}
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

      <!-- Sleek 1-line Enterprise Command Bar (~42px) -->
      <div class="explorer-command-bar glass-panel desktop-only" role="toolbar" aria-label="Kubernetes Explorer Command Bar">
        <!-- Left: Search input -->
        <div class="toolbar-search-wrap">
          <BaseIcon name="search" size="xs" class="search-icon" />
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search resources in namespace..."
            class="toolbar-search-input font-mono"
            aria-label="Search resources in namespace"
          />
          <button
            v-if="searchQuery"
            type="button"
            class="clear-input-btn"
            aria-label="Clear search"
            @click="searchQuery = ''"
          >
            <BaseIcon name="x" size="xs" />
          </button>
        </div>

        <!-- Center: Linear-style segmented control -->
        <div class="linear-segmented-group" role="tablist" aria-label="Resource Types">
          <button
            v-for="item in primaryKinds"
            :key="item.kind"
            type="button"
            role="tab"
            :aria-selected="selectedKind === item.kind"
            class="linear-segment-btn font-mono"
            :class="{ active: selectedKind === item.kind }"
            @click="selectKind(item.kind)"
          >
            <span>{{ item.label }}</span>
          </button>
          <!-- Secondary Active Kind Indicator -->
          <div
            v-if="!primaryKinds.some(k => k.kind === selectedKind)"
            class="linear-segment-btn active font-mono secondary-active-segment"
          >
            <span>{{ currentKindLabel }}</span>
          </div>
        </div>

        <!-- Right: Actions group -->
        <div class="toolbar-actions-group">
          <button
            type="button"
            class="toolbar-btn btn-secondary"
            title="Apply YAML Manifest"
            @click="openApplyYamlModal"
          >
            <BaseIcon name="file-text" size="xs" />
            <span>Apply YAML</span>
          </button>

          <button
            type="button"
            class="toolbar-btn btn-primary"
            title="Create Resource"
            @click="showCreateModal = true"
          >
            <BaseIcon name="plus" size="xs" />
            <span>+ Create Resource</span>
          </button>

          <ActionDropdown
            :items="moreActions"
            size="sm"
            trigger-title="More actions"
            @select="handleMoreActionSelect"
          />
        </div>
      </div>

      <!-- Toast & Offline Notifications -->
      <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
        <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="toast-icon" />
        <span class="toast-text font-mono font-small">{{ toastMessage.text }}</span>
        <button class="toast-close" aria-label="Dismiss toast" @click="toastMessage = null"><BaseIcon name="x" size="xs" /></button>
      </div>

      <div v-if="clusterOffline" class="offline-banner animate-fade-in">
        <BaseIcon name="alert-triangle" size="xs" class="offline-icon" />
        <div class="offline-content">
          <strong class="offline-title">Kubernetes Cluster Disconnected</strong>
          <p class="offline-desc">No active Kubernetes control plane is attached to '{{ selectedCluster || 'primary-cluster' }}'. Import a valid Kubeconfig or manage Docker Swarm.</p>
          <span v-if="offlineErrorMessage" class="offline-err-detail font-mono">{{ offlineErrorMessage }}</span>
          <div class="offline-actions">
            <button type="button" class="btn btn-primary btn-xs" @click="showImportModal = true">+ Import Cluster</button>
            <router-link to="/deployments" class="btn btn-secondary btn-xs">Manage Docker Swarm</router-link>
            <button type="button" class="btn btn-secondary btn-xs" :disabled="loading" @click="fetchResources"><BaseIcon name="refresh" size="xs" /> Retry</button>
          </div>
        </div>
      </div>

      <!-- Events View or Table / Mobile Cards -->
      <div v-if="selectedKind === 'events'" class="events-main-section animate-fade-in">
        <EventsTimeline :cluster="selectedCluster" :namespace="selectedNamespace" :auto-refresh="true" />
      </div>

      <div v-else class="explorer-resources-wrapper">
        <div class="explorer-desktop-table explorer-table-wrap">
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

<style scoped>
@import '../assets/styles/views/explorer.css';
</style>
