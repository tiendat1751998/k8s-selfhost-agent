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
export type WorkloadSegmentTab = 'All' | 'Deployments' | 'StatefulSets' | 'DaemonSets' | 'CronJobs'
const selectedSegmentTab = ref<WorkloadSegmentTab>('All')

const segmentTabs: { id: WorkloadSegmentTab; label: string; icon: string }[] = [
  { id: 'All', label: 'All', icon: 'globe' },
  { id: 'Deployments', label: 'Deployments', icon: 'play' },
  { id: 'StatefulSets', label: 'StatefulSets', icon: 'database' },
  { id: 'DaemonSets', label: 'DaemonSets', icon: 'shield' },
  { id: 'CronJobs', label: 'CronJobs', icon: 'clock' },
]

function matchWorkloadKind(app: DeploymentApp, kind: WorkloadSegmentTab): boolean {
  if (kind === 'All') return true
  const n = (app.name || '').toLowerCase(), t = (app.type || '').toLowerCase()
  if (kind === 'Deployments') return !n.includes('stateful') && !n.includes('daemon') && !n.includes('cron') && !n.includes('job')
  if (kind === 'StatefulSets') return n.includes('stateful') || n.includes('db') || n.includes('postgres') || n.includes('redis') || n.includes('sql') || t === 'statefulset'
  if (kind === 'DaemonSets') return n.includes('daemon') || n.includes('agent') || n.includes('traefik') || n.includes('node') || t === 'daemonset'
  if (kind === 'CronJobs') return n.includes('cron') || n.includes('job') || n.includes('sync') || n.includes('backup') || t === 'cronjob'
  return true
}

const displayDeployments = computed(() => filteredDeployments.value.filter(app => matchWorkloadKind(app, selectedSegmentTab.value)))
const getSegmentCount = (tabId: WorkloadSegmentTab): number => filteredDeployments.value.filter(app => matchWorkloadKind(app, tabId)).length

const showMobileFilters = ref(false)

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
    <!-- Compact 44px Mobile Command Bar (<768px) -->
    <div class="mobile-command-bar">
      <div class="mobile-search-compact-wrap">
        <span class="mobile-search-ico"><BaseIcon name="search" size="xs" /></span>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter workloads..."
          class="input-glass mobile-search-compact"
        />
        <button v-if="searchQuery" type="button" class="mobile-search-clear" @click="searchQuery = ''"><BaseIcon name="x" size="xs" /></button>
      </div>

      <div class="mobile-compact-badge font-mono" title="Total workloads and ready pods">
        {{ totalWorkloads }} Workloads • {{ readyReplicas }}/{{ totalReplicas }} Ready
      </div>

      <button
        type="button"
        class="btn-filters-toggle"
        :class="{ 'filters-active': showMobileFilters }"
        @click="showMobileFilters = !showMobileFilters"
        title="Toggle Filter Options"
      >
        <BaseIcon name="sliders" size="xs" /> Filters
      </button>

      <button
        type="button"
        class="btn-mobile-deploy"
        title="Deploy Workload"
        @click="showCreateModal = true; selectedTemplate = null"
      >+
      </button>
    </div>

    <!-- Mobile Micro-Telemetry Strip (<640px) -->
    <div class="micro-telemetry-strip font-mono">
      <span><BaseIcon name="play" size="xs" /> {{ totalWorkloads }} workloads</span>
      <span class="telemetry-sep">·</span>
      <span>{{ readyReplicas }}/{{ totalReplicas }} pods</span>
      <span class="telemetry-sep">·</span>
      <span><BaseIcon name="shield" size="xs" /> {{ healthyCount }} healthy</span>
    </div>

    <!-- Mobile Expandable Filter Strip Accordion (<768px) -->
    <Transition name="filter-slide">
      <div v-if="showMobileFilters" class="mobile-filter-strip glass-panel">
        <div class="mobile-filter-row">
          <label class="mobile-filter-label">Namespace:</label>
          <select v-model="selectedNamespaceFilter" class="input-glass select-ns-mobile font-mono">
            <option value="all">All Namespaces ({{ namespaces.length }})</option>
            <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
          </select>
        </div>

        <div class="mobile-filter-row">
          <label class="mobile-filter-label">Status:</label>
          <div class="mobile-pills-group">
            <button
              type="button"
              class="pill-btn pill-xs"
              :class="{ 'pill-active': statusFilter === 'all' }"
              @click="statusFilter = 'all'"
            >
              All ({{ totalWorkloads }})
            </button>
            <button
              type="button"
              class="pill-btn pill-xs pill-healthy"
              :class="{ 'pill-active': statusFilter === 'healthy' }"
              @click="statusFilter = 'healthy'"
            >
              <BaseIcon name="shield" size="xs" /> Healthy ({{ healthyCount }})
            </button>
            <button
              type="button"
              class="pill-btn pill-xs pill-degraded"
              :class="{ 'pill-active': statusFilter === 'degraded' }"
              @click="statusFilter = 'degraded'"
            >
              <BaseIcon name="alert-triangle" size="xs" /> Degraded ({{ degradedCount }})
            </button>
          </div>
        </div>

        <div class="mobile-filter-row">
          <label class="mobile-filter-label">Type:</label>
          <div class="mobile-pills-group">
            <button
              type="button"
              class="pill-btn pill-xs"
              :class="{ 'pill-active': activeFilterTab === 'all' }"
              @click="activeFilterTab = 'all'"
            >
              All
            </button>
            <button
              type="button"
              class="pill-btn pill-xs"
              :class="{ 'pill-active': activeFilterTab === 'k8s' }"
              @click="activeFilterTab = 'k8s'"
            >
              <BaseIcon name="anchor" size="xs" /> K8s ({{ k8sCount }})
            </button>
            <button
              type="button"
              class="pill-btn pill-xs"
              :class="{ 'pill-active': activeFilterTab === 'swarm' }"
              @click="activeFilterTab = 'swarm'"
            >
              <BaseIcon name="box" size="xs" /> Docker ({{ swarmCount }})
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Notification Toast Banner -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span class="toast-icon"><BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : toastMessage.type === 'error' ? 'alert-triangle' : 'help-circle'" size="sm" /></span>
      <span class="toast-text">{{ toastMessage.text }}</span>
      <button type="button" class="toast-close" @click="toastMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Sleek Unified Enterprise Toolbar -->
    <div class="deployments-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="Deployments Fleet Toolbar">
      <!-- Search input with search icon and clear button (filters workloads by name, image, namespace) -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter workloads..."
          class="toolbar-search-input"
          aria-label="Filter workloads by name, image, namespace"
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

      <!-- Segmented kind capsule pills: All (32), Deployments (32), StatefulSets (3), DaemonSets (1), CronJobs (0) -->
      <div class="toolbar-capsule-pills" role="tablist" aria-label="Workload Kind">
        <button
          v-for="tab in segmentTabs"
          :key="tab.id"
          type="button"
          role="tab"
          :aria-selected="selectedSegmentTab === tab.id"
          class="capsule-pill font-mono"
          :class="{ active: selectedSegmentTab === tab.id }"
          @click="selectedSegmentTab = tab.id"
        >
          <span>{{ tab.label }}</span>
          <span class="capsule-count">({{ getSegmentCount(tab.id) }})</span>
        </button>
      </div>

      <!-- Status filter dropdown -->
      <select
        v-model="statusFilter"
        class="toolbar-select font-mono"
        aria-label="Filter by Status"
      >
        <option value="all">All Statuses ({{ totalWorkloads }})</option>
        <option value="healthy">Healthy ({{ healthyCount }})</option>
        <option value="degraded">Degraded ({{ degradedCount }})</option>
      </select>

      <!-- Action buttons -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="toolbar-btn btn-primary"
          title="Deploy Workload"
          @click="showCreateModal = true; selectedTemplate = null"
        >
          <span>+ Deploy Workload</span>
        </button>

        <div class="toolbar-secondary-group">
          <button
            type="button"
            class="toolbar-btn btn-secondary btn-grouped"
            title="YAML Manifest"
            @click="openYamlViewer"
          >
            <BaseIcon name="file-text" size="xs" />
            <span class="btn-label-desktop">YAML</span>
          </button>
          <button
            type="button"
            class="toolbar-btn btn-secondary btn-grouped"
            title="Blueprint Catalog"
            @click="showTemplatesDrawer = true"
          >
            <BaseIcon name="box" size="xs" />
            <span class="btn-label-desktop">Blueprints</span>
          </button>
        </div>

        <button
          type="button"
          class="toolbar-btn btn-secondary toolbar-btn-refresh"
          :disabled="loading"
          title="Refresh Workloads"
          @click="fetchDeployments"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" />
          <span class="btn-label-desktop">{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
      </div>
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
