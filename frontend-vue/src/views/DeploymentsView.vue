<script setup lang="ts">
import { ref, computed } from 'vue'
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

// Mobile PWA Ergonomics (<768px)
// Segmented Tab Bar State ('Deployments', 'StatefulSets', 'DaemonSets', 'CronJobs', 'All Workloads')
export type WorkloadSegmentTab = 'Deployments' | 'StatefulSets' | 'DaemonSets' | 'CronJobs' | 'All Workloads'
const selectedSegmentTab = ref<WorkloadSegmentTab>('Deployments')

const segmentTabs: { id: WorkloadSegmentTab; label: string; icon: string }[] = [
  { id: 'Deployments', label: 'Deployments', icon: 'play' },
  { id: 'StatefulSets', label: 'StatefulSets', icon: 'database' },
  { id: 'DaemonSets', label: 'DaemonSets', icon: 'shield' },
  { id: 'CronJobs', label: 'CronJobs', icon: 'clock' },
  { id: 'All Workloads', label: 'All Workloads', icon: 'globe' },
]

function matchWorkloadKind(app: DeploymentApp, kind: WorkloadSegmentTab): boolean {
  if (kind === 'All Workloads') return true
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

    <!-- View Header (Desktop & Tablet >=768px) -->
    <div class="view-header">
      <div class="header-info">
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>COMPUTE & FLEET ORCHESTRATION</span>
        </div>
        <h1 class="view-title">
          <span class="title-full">Deployments & App Workload Catalog</span>
          <span class="title-compact"><BaseIcon name="play" size="sm" /> Deployments</span>
        </h1>
        <p class="view-desc">
          Unified production orchestrator for Kubernetes Deployments & Docker Swarm services with full Canary traffic splits, Blue-Green zero-downtime cutovers, dynamic replica autoscaling, and rolling restarts.
        </p>
      </div>

      <div class="header-actions">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="fetchDeployments">
          <span class="btn-text-full"><BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" /> {{ loading ? 'Syncing...' : 'Refresh' }}</span>
          <span class="btn-text-mobile"><BaseIcon :name="loading ? 'activity' : 'refresh'" size="xs" :class="{ 'spin-icon': loading }" /> {{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
        <button type="button" class="btn btn-primary" @click="showCreateModal = true; selectedTemplate = null">
          <span class="btn-text-full">+ Deploy Workload</span>
          <span class="btn-text-mobile">+ Deploy</span>
        </button>
        <button type="button" class="btn btn-secondary" @click="openYamlViewer">
          <span class="btn-text-full"><BaseIcon name="file-text" size="xs" /> YAML Manifest</span>
          <span class="btn-text-mobile"><BaseIcon name="file-text" size="xs" /> YAML</span>
        </button>
        <button type="button" class="btn btn-secondary" @click="showTemplatesDrawer = true">
          <span class="btn-text-full"><BaseIcon name="box" size="xs" /> Blueprint Catalog</span>
          <span class="btn-text-mobile"><BaseIcon name="box" size="xs" /> Template</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast Banner -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span class="toast-icon"><BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : toastMessage.type === 'error' ? 'alert-triangle' : 'help-circle'" size="sm" /></span>
      <span class="toast-text">{{ toastMessage.text }}</span>
      <button type="button" class="toast-close" @click="toastMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Compact 36px Horizontal Metric Strip (Above the Fold) -->
    <div class="workloads-metric-strip font-mono" role="status" aria-label="Workload Fleet Metrics">
      <div class="strip-item">
        <span class="pulse-dot pulse-dot-cyan"></span>
        <span class="strip-val font-bold text-slate">{{ totalWorkloads }} Deployments</span>
      </div>
      <span class="strip-sep">·</span>
      <div class="strip-item">
        <BaseIcon name="layers" size="xs" />
        <span>{{ namespaces.length }} Namespaces</span>
      </div>
      <span class="strip-sep">·</span>
      <div class="strip-item">
        <BaseIcon name="server" size="xs" />
        <span class="text-emerald">k8snode Online</span>
      </div>
      <span class="strip-sep">·</span>
      <div class="strip-item">
        <BaseIcon name="check-circle" size="xs" class="text-emerald" />
        <span>{{ readyReplicas }}/{{ totalReplicas }} Pods Ready</span>
      </div>
      <span class="strip-sep">·</span>
      <div class="strip-item">
        <span class="text-emerald">{{ healthyCount }} Healthy</span>
        <span v-if="degradedCount > 0" class="text-amber">({{ degradedCount }} Degraded)</span>
      </div>
      <span class="strip-sep">·</span>
      <div class="strip-item text-muted">
        <span>{{ k8sCount }} K8s / {{ swarmCount }} Swarm</span>
      </div>
    </div>

    <!-- Main Workload Section -->
    <div class="section-box glass-panel table-box">
      <!-- Flat Segmented Tab Bar & Search Controls -->
      <div class="table-controls-bar">
        <div class="segmented-tabs-bar" role="tablist" aria-label="Workload Type Navigation">
          <button
            v-for="tab in segmentTabs"
            :key="tab.id"
            type="button"
            role="tab"
            class="segmented-tab-btn font-mono"
            :class="{ active: selectedSegmentTab === tab.id }"
            :aria-selected="selectedSegmentTab === tab.id"
            @click="selectedSegmentTab = tab.id"
          >
            <BaseIcon :name="tab.icon" size="xs" />
            <span>{{ tab.label }}</span>
            <span class="tab-count-badge font-mono">{{ getSegmentCount(tab.id) }}</span>
          </button>
        </div>

        <div class="table-search-row">
          <div class="search-input-wrap">
            <span class="search-ico"><BaseIcon name="search" size="xs" /></span>
            <input v-model="searchQuery" type="text" placeholder="Filter workloads by name, image, team..." class="input-glass search-input" />
            <button v-if="searchQuery" type="button" class="clear-search-btn" @click="searchQuery = ''"><BaseIcon name="x" size="xs" /></button>
          </div>

          <div class="status-filter-group">
            <button
              type="button"
              class="pill-btn"
              :class="{ 'pill-active': statusFilter === 'all' }"
              title="All Statuses"
              @click="statusFilter = 'all'"
            >
              <span>All</span>
              <span class="pill-badge">{{ totalWorkloads }}</span>
            </button>
            <button
              type="button"
              class="pill-btn pill-healthy"
              :class="{ 'pill-active': statusFilter === 'healthy' }"
              title="Healthy Workloads"
              @click="statusFilter = 'healthy'"
            >
              <BaseIcon name="shield" size="xs" />
              <span>Healthy</span>
              <span class="pill-badge">{{ healthyCount }}</span>
            </button>
            <button
              type="button"
              class="pill-btn pill-degraded"
              :class="{ 'pill-active': statusFilter === 'degraded' }"
              title="Degraded Workloads"
              @click="statusFilter = 'degraded'"
            >
              <BaseIcon name="alert-triangle" size="xs" />
              <span>Degraded</span>
              <span class="pill-badge">{{ degradedCount }}</span>
            </button>
          </div>
        </div>
      </div>

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
