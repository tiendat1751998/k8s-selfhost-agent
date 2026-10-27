<script setup lang="ts">
import { ref, computed, watch, onMounted, defineAsyncComponent } from 'vue'
import { useRoute } from 'vue-router'
import BaseIcon from '../components/ui/BaseIcon.vue'
import ExplorerToolbar from '../components/explorer/ExplorerToolbar.vue'
import EventsTimeline from '../components/k8s/EventsTimeline.vue'

import ExplorerResourceTable from '../components/explorer/ExplorerResourceTable.vue'
import ExplorerMobileCards from '../components/explorer/ExplorerMobileCards.vue'
const ExplorerDetailDrawer = defineAsyncComponent(() => import('../components/explorer/ExplorerDetailDrawer.vue'))
import ExplorerCreateModal from '../components/explorer/ExplorerCreateModal.vue'
import ExplorerScaleModal from '../components/explorer/ExplorerScaleModal.vue'
import ExplorerRestartModal from '../components/explorer/ExplorerRestartModal.vue'
import ExplorerDeleteModal from '../components/explorer/ExplorerDeleteModal.vue'
import ExplorerDrainModal from '../components/explorer/ExplorerDrainModal.vue'
const ApplyYamlModal = defineAsyncComponent(() => import('../components/explorer/ApplyYamlModal.vue'))
const ExplorerImportModal = defineAsyncComponent(() => import('../components/explorer/ExplorerImportModal.vue'))
import ExplorerCreateNsModal from '../components/explorer/ExplorerCreateNsModal.vue'
const PodLogsDrawer = defineAsyncComponent(() => import('../components/explorer/PodLogsDrawer.vue'))
const PodTerminalDrawer = defineAsyncComponent(() => import('../components/explorer/PodTerminalDrawer.vue'))
import { useK8sExplorer, getResourceStatus } from '../composables/useK8sExplorer'
import { useGlobalContext } from '../composables/useGlobalContext'
import type { ResourceKind } from '../api/k8s'

const route = useRoute()
const {
  loading, error, toastMessage, clusterOffline, offlineErrorMessage,
  k8sDegraded, degradedReason,
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
      <ExplorerToolbar
        v-model:show-mobile-search="showMobileSearch"
        v-model:search-query="searchQuery"
        :current-kind-label="currentKindLabel"
        :total-in-kind="totalInKind"
        :loading="loading"
        :selected-cluster="selectedCluster"
        :selected-namespace="selectedNamespace"
        :selected-kind="selectedKind"
        :all-kind-items="allKindItems"
        @select-kind="selectKind"
        @refresh="fetchResources"
        @apply-yaml="openApplyYamlModal"
        @create-resource="showCreateModal = true"
        @import-cluster="showImportModal = true"
        @new-namespace="showNewNsModal = true"
      />

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

      <!-- Control Plane (:6443) Offline Diagnostic Banner -->
      <div v-if="k8sDegraded && !clusterOffline" class="control-plane-banner glass-panel animate-fade-in" role="alert">
        <div class="cp-banner-header">
          <span class="cp-status-dot"></span>
          <span class="cp-title font-mono font-bold">CONTROL PLANE (:6443) UNREACHABLE</span>
          <span class="cp-badge font-mono">K8S DEGRADED</span>
        </div>
        <p class="cp-desc font-mono">
          kube-apiserver (:6443) is offline on cluster '{{ selectedCluster || 'primary-cluster' }}'. Kubernetes resource discovery is unavailable while node daemons stream live telemetry.
        </p>
        <span v-if="degradedReason" class="cp-reason font-mono text-muted">{{ degradedReason }}</span>
        <div class="cp-actions">
          <router-link to="/deployments" class="btn btn-secondary btn-xs">
            <BaseIcon name="layers" size="xs" /> <span>Inspect Live Host Daemons</span>
          </router-link>
          <router-link to="/slo" class="btn btn-secondary btn-xs">
            <BaseIcon name="activity" size="xs" /> <span>View Control Plane SLOs</span>
          </router-link>
          <button type="button" class="btn btn-secondary btn-xs" :disabled="loading" @click="fetchResources">
            <BaseIcon name="refresh" size="xs" /> <span>Retry</span>
          </button>
        </div>
      </div>

      <!-- Events View or Table / Mobile Cards -->
      <div v-if="selectedKind === 'events'" class="events-main-section animate-fade-in">
        <EventsTimeline :cluster="selectedCluster" :namespace="selectedNamespace" :auto-refresh="true" />
      </div>

      <div v-else-if="!clusterOffline" class="explorer-resources-wrapper">
        <!-- Clean Error State Panel (prevents table separator slicing) -->
        <div v-if="error && filteredResources.length === 0" class="glass-panel explorer-error-state animate-fade-in">
          <div class="empty-state-card">
            <BaseIcon name="alert-triangle" size="md" class="text-amber" />
            <h4 class="empty-title">Failed to load Kubernetes resources</h4>
            <p class="empty-desc font-mono">{{ error }}</p>
            <button type="button" class="btn btn-secondary btn-sm" @click="fetchResources">
              <BaseIcon name="refresh" size="xs" /> Retry
            </button>
          </div>
        </div>

        <template v-else>
          <div class="explorer-desktop-table explorer-table-wrap">
            <ExplorerResourceTable
              :columns="columns"
              :resources="filteredResources"
              :loading="loading"
              :error="error"
              :selected-kind="selectedKind"
              :operating-node="operatingNode"
              :k8s-degraded="k8sDegraded"
              :degraded-reason="degradedReason"
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
              :k8s-degraded="k8sDegraded"
              :degraded-reason="degradedReason"
              @select="openDetailDrawer"
              @logs="openLogsDrawer"
              @scale="openScaleModal"
              @restart="openRestartModal"
              @delete="openDeleteModal"
            />
          </div>
        </template>
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
    <ApplyYamlModal :show="showYamlModal" :cluster="selectedCluster" :namespace="(!selectedNamespace || selectedNamespace === 'all' || selectedNamespace === 'All Namespaces') ? 'default' : selectedNamespace" :initial-yaml="yamlEditorInitialContent" :title="yamlEditorTitle" :mode="yamlEditorMode" @close="showYamlModal = false" @applied="handleYamlApplied" />
    <ExplorerCreateModal :show="showCreateModal" :cluster="selectedCluster" :namespaces="namespaces" :default-namespace="selectedNamespace === 'all' ? 'default' : selectedNamespace" :default-kind="selectedKind" @close="showCreateModal = false" @applied="handleCreateApplied" />
    <ExplorerImportModal :show="showImportModal" :importing="importingCluster" @close="showImportModal = false" @import="handleImportCluster" />
    <ExplorerCreateNsModal :show="showNewNsModal" :cluster="selectedCluster" :creating="creatingNs" :error="newNsError" @close="showNewNsModal = false" @create="handleCreateNs" />
    <PodLogsDrawer :show="showLogsDrawer" :pod="logsPod" :cluster="selectedCluster" @close="showLogsDrawer = false" />
    <PodTerminalDrawer :show="showTerminalDrawer" :pod="terminalPod" :cluster="selectedCluster" @close="showTerminalDrawer = false" />
  </div>
</template>

<style scoped>
@import '../assets/styles/views/explorer.css';

.btn-apply-yaml {
  white-space: nowrap !important;
  font-size: 11px !important;
  padding: 4px 8px !important;
}

.explorer-error-state {
  padding: 36px 20px !important;
  text-align: center !important;
  display: flex !important;
  justify-content: center !important;
  align-items: center !important;
  border-radius: 12px !important;
  background: var(--color-surface-card) !important;
  border: 1px solid var(--color-border-subtle) !important;
  margin: 12px 0 !important;
}

.empty-state-card {
  display: flex !important;
  flex-direction: column !important;
  align-items: center !important;
  gap: 10px !important;
  max-width: 480px !important;
}

.empty-title {
  margin: 0 !important;
  font-size: 1rem !important;
  color: var(--color-body) !important;
}

.empty-desc {
  margin: 0 !important;
  font-size: 0.82rem !important;
  color: var(--color-muted) !important;
  word-break: break-word !important;
}
</style>
