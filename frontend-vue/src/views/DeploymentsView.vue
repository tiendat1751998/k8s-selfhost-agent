<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useGlobalContext } from '../composables/useGlobalContext'
import BaseIcon from '../components/ui/BaseIcon.vue'
import DeploymentsTable from '../components/deployments/DeploymentsTable.vue'
import DeploymentsMobileCards from '../components/deployments/DeploymentsMobileCards.vue'
import CanaryStrategyModal from '../components/deployments/CanaryStrategyModal.vue'
import DeployWorkloadModal from '../components/deployments/DeployWorkloadModal.vue'
import ScaleResourceModal from '../components/deployments/ScaleResourceModal.vue'
import WorkloadInspectorDrawer from '../components/deployments/WorkloadInspectorDrawer.vue'
import BlueprintCatalogDrawer from '../components/deployments/BlueprintCatalogDrawer.vue'
import DeploymentsHeaderBar, { type WorkloadSegmentTab } from '../components/deployments/DeploymentsHeaderBar.vue'
import { useDeployments } from '../composables/useDeployments'
import type { DeploymentApp, DeploymentTemplate, UpdateResourcesPayload } from '../api/compute'
import '../assets/styles/views/deployments.css'

const {
  loading,
  error,
  actionLoading,
  toastMessage,
  templates,
  activeFilterTab,
  statusFilter,
  searchQuery,
  selectedNamespaceFilter,
  totalWorkloads,
  totalReplicas,
  readyReplicas,
  healthyCount,
  degradedCount,
  k8sCount,
  swarmCount,
  namespaces,
  filteredDeployments,
  showToast,
  fetchDeployments,
  getRolloutState,
  handleApplyResources,
  handleRestart,
  handleApplyCanaryWeight,
  handlePromoteCanary,
  handleAbortCanary,
  handleBlueGreenCutover,
  handleRollback,
  handleTogglePause,
  handleDelete,
  handleCreateApp,
} = useDeployments()

const { activeNamespace } = useGlobalContext()
watch(activeNamespace, (newNs) => {
  selectedNamespaceFilter.value = newNs || 'all'
}, { immediate: true })

// Segmented Tab Bar State: 'All' (first), 'Deployments', 'StatefulSets', 'DaemonSets', 'CronJobs'
const selectedSegmentTab = ref<WorkloadSegmentTab>('All')

function matchWorkloadKind(app: DeploymentApp, kind: WorkloadSegmentTab): boolean {
  if (kind === 'All') return true
  const rawKind = ((app as { kind?: string }).kind || app.type || '').toLowerCase()
  if (kind === 'Deployments') {
    return rawKind.includes('deployment') || rawKind === 'kubernetes' || rawKind === 'swarm' || rawKind === 'docker' || rawKind === 'container' || (!rawKind.includes('stateful') && !rawKind.includes('daemon') && !rawKind.includes('cron') && !rawKind.includes('job'))
  }
  if (kind === 'StatefulSets') return rawKind.includes('stateful')
  if (kind === 'DaemonSets') return rawKind.includes('daemon')
  if (kind === 'CronJobs') return rawKind.includes('cron') || rawKind.includes('job')
  return true
}

const displayDeployments = computed(() => filteredDeployments.value.filter(app => matchWorkloadKind(app, selectedSegmentTab.value)))
const getSegmentCount = (tabId: WorkloadSegmentTab): number => filteredDeployments.value.filter(app => matchWorkloadKind(app, tabId)).length

const segmentCounts = computed<Record<WorkloadSegmentTab, number>>(() => ({
  All: getSegmentCount('All'),
  Deployments: getSegmentCount('Deployments'),
  StatefulSets: getSegmentCount('StatefulSets'),
  DaemonSets: getSegmentCount('DaemonSets'),
  CronJobs: getSegmentCount('CronJobs'),
}))

// Drawers & Modals State
const selectedApp = ref<DeploymentApp | null>(null)
const selectedTemplate = ref<DeploymentTemplate | null>(null)
const showInspectorDrawer = ref(false)
const inspectorActiveTab = ref<'overview' | 'strategy' | 'network' | 'logs' | 'env' | 'yaml'>('overview')
const showScaleModal = ref(false)
const showStrategyModal = ref(false)
const showTemplatesDrawer = ref(false)
const showCreateModal = ref(false)

function openInspector(app: DeploymentApp, tab: 'overview' | 'strategy' | 'network' | 'logs' | 'env' | 'yaml' = 'overview') {
  selectedApp.value = app
  inspectorActiveTab.value = tab
  showInspectorDrawer.value = true
}

function openLogsInspector(app: DeploymentApp) {
  openInspector(app, 'logs')
}

function openScaleModal(app: DeploymentApp) {
  selectedApp.value = app
  showScaleModal.value = true
}

function openStrategyModal(app: DeploymentApp) {
  selectedApp.value = app
  showStrategyModal.value = true
}

function openYamlViewer() {
  if (selectedApp.value) {
    openInspector(selectedApp.value, 'yaml')
  } else if (filteredDeployments.value.length > 0) {
    openInspector(filteredDeployments.value[0], 'yaml')
  } else {
    showCreateModal.value = true
  }
}

function handleSelectTemplate(tmpl: DeploymentTemplate) {
  selectedTemplate.value = tmpl
  showTemplatesDrawer.value = false
  showCreateModal.value = true
}

async function onApplyResources(payload: UpdateResourcesPayload) {
  const success = await handleApplyResources(payload)
  if (success) {
    showScaleModal.value = false
  }
}

async function onCreateApp(payload: DeploymentApp) {
  const success = await handleCreateApp(payload)
  if (success) {
    showCreateModal.value = false
  }
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- 36px mobile-command-bar with inline mobile-telemetry-pill (pods readiness) in DeploymentsHeaderBar -->
    <DeploymentsHeaderBar
      v-model:search-query="searchQuery"
      v-model:selected-segment-tab="selectedSegmentTab"
      v-model:status-filter="statusFilter"
      v-model:active-filter-tab="activeFilterTab"
      v-model:selected-namespace-filter="selectedNamespaceFilter"
      :namespaces="namespaces"
      :total-workloads="totalWorkloads"
      :total-replicas="totalReplicas"
      :ready-replicas="readyReplicas"
      :healthy-count="healthyCount"
      :degraded-count="degradedCount"
      :k8s-count="k8sCount"
      :swarm-count="swarmCount"
      :loading="loading"
      :segment-counts="segmentCounts"
      @deploy="showCreateModal = true; selectedTemplate = null"
      @open-yaml="openYamlViewer"
      @open-blueprints="showTemplatesDrawer = true"
      @refresh="fetchDeployments"
    />

    <!-- Notification Toast Banner -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span class="toast-icon"><BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : toastMessage.type === 'error' ? 'alert-triangle' : 'help-circle'" size="sm" /></span>
      <span class="toast-text">{{ toastMessage.text }}</span>
      <button type="button" class="toast-close" @click="toastMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Main Workload Section -->
    <div class="section-box glass-panel table-box">
      <!-- Desktop Data Table View (Completely suppressed on mobile <768px) -->
      <div class="desktop-table-container desktop-table-view">
        <DeploymentsTable
          :deployments="displayDeployments"
          :loading="loading"
          :error="error"
          :action-loading="actionLoading"
          :get-rollout-state="getRolloutState"
          @inspect="openInspector($event)"
          @logs="openLogsInspector($event)"
          @scale="openScaleModal($event)"
          @strategy="openStrategyModal($event)"
          @restart="handleRestart($event)"
          @delete="handleDelete($event)"
        />
      </div>

      <!-- Mobile Touch-Friendly Card View (Visible only on mobile <768px) -->
      <div class="mobile-cards-view">
        <DeploymentsMobileCards
          :deployments="displayDeployments"
          :loading="loading"
          @inspect="openInspector($event)"
          @logs="openLogsInspector($event)"
        />
      </div>
    </div>

    <!-- Modals & Drawers -->
    <ScaleResourceModal
      v-model:show="showScaleModal"
      :app="selectedApp"
      :action-loading="actionLoading"
      @apply="onApplyResources"
    />

    <CanaryStrategyModal
      v-model:show="showStrategyModal"
      :app="selectedApp"
      :action-loading="actionLoading"
      @update:strategy="(val: any) => { if (selectedApp) selectedApp.strategy = val }"
      @apply-canary-weight="selectedApp && handleApplyCanaryWeight(selectedApp, $event)"
      @promote-canary="handlePromoteCanary"
      @abort-canary="handleAbortCanary"
      @cutover-blue-green="handleBlueGreenCutover"
      @rollback="handleRollback"
      @toggle-pause="handleTogglePause"
    />

    <DeployWorkloadModal
      v-model:show="showCreateModal"
      :action-loading="actionLoading"
      :template="selectedTemplate"
      @create="onCreateApp"
      @toast="(msg, type) => showToast(msg, type)"
    />

    <WorkloadInspectorDrawer
      v-model:show="showInspectorDrawer"
      :app="selectedApp"
      :initial-tab="inspectorActiveTab"
      @open-scale="openScaleModal"
      @open-strategy="openStrategyModal"
      @restart="handleRestart"
      @delete="handleDelete"
      @toast="(msg, type) => showToast(msg, type)"
    />

    <BlueprintCatalogDrawer
      v-model:show="showTemplatesDrawer"
      :templates="templates"
      @select="handleSelectTemplate"
    />
  </div>
</template>
