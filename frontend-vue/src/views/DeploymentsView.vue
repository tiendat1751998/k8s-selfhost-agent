<script setup lang="ts">
import { ref } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
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
  canaryCount,
  blueGreenCount,
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
        <span class="mobile-search-ico">🔍</span>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter workloads..."
          class="input-glass mobile-search-compact"
        />
        <button v-if="searchQuery" type="button" class="mobile-search-clear" @click="searchQuery = ''">✕</button>
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
        ⚙️ Filters
      </button>

      <button
        type="button"
        class="btn-mobile-deploy"
        title="Deploy Workload"
        @click="showCreateModal = true; selectedTemplate = null"
      >
        ➕
      </button>
    </div>

    <!-- Mobile Micro-Telemetry Strip (<640px) -->
    <div class="micro-telemetry-strip font-mono">
      <span>🚀 {{ totalWorkloads }} workloads</span>
      <span class="telemetry-sep">·</span>
      <span>{{ readyReplicas }}/{{ totalReplicas }} pods</span>
      <span class="telemetry-sep">·</span>
      <span>🛡️ {{ healthyCount }} healthy</span>
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
              🛡️ Healthy ({{ healthyCount }})
            </button>
            <button
              type="button"
              class="pill-btn pill-xs pill-degraded"
              :class="{ 'pill-active': statusFilter === 'degraded' }"
              @click="statusFilter = 'degraded'"
            >
              ⚠️ Degraded ({{ degradedCount }})
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
              ☸️ K8s ({{ k8sCount }})
            </button>
            <button
              type="button"
              class="pill-btn pill-xs"
              :class="{ 'pill-active': activeFilterTab === 'swarm' }"
              @click="activeFilterTab = 'swarm'"
            >
              🐳 Docker ({{ swarmCount }})
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
          <span class="title-compact">🚀 Deployments</span>
        </h1>
        <p class="view-desc">
          Unified production orchestrator for Kubernetes Deployments & Docker Swarm services with full Canary traffic splits, Blue-Green zero-downtime cutovers, dynamic replica autoscaling, and rolling restarts.
        </p>
      </div>

      <div class="header-actions">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="fetchDeployments">
          <span class="btn-text-full"><span :class="{ 'spin-icon': loading }">🔄</span> {{ loading ? 'Syncing...' : 'Refresh' }}</span>
          <span class="btn-text-mobile">{{ loading ? '⏳ Syncing...' : '🔄 Refresh' }}</span>
        </button>
        <button type="button" class="btn btn-primary" @click="showCreateModal = true; selectedTemplate = null">
          <span class="btn-text-full">+ Deploy Workload</span>
          <span class="btn-text-mobile">+ Deploy</span>
        </button>
        <button type="button" class="btn btn-secondary" @click="openYamlViewer">
          <span class="btn-text-full">📝 YAML Manifest</span>
          <span class="btn-text-mobile">📝 YAML</span>
        </button>
        <button type="button" class="btn btn-secondary" @click="showTemplatesDrawer = true">
          <span class="btn-text-full">📦 Blueprint Catalog</span>
          <span class="btn-text-mobile">📦 Template</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast Banner -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span class="toast-icon">{{ toastMessage.type === 'success' ? '✅' : toastMessage.type === 'error' ? '⚠️' : 'ℹ️' }}</span>
      <span class="toast-text">{{ toastMessage.text }}</span>
      <button type="button" class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- Metric HUD Grid (Desktop & Tablet >=768px) -->
    <div class="metrics-grid">
      <MetricCard title="Total Workloads" :value="totalWorkloads" subtitle="Active microservice deployments" icon="📦" badge="SERVICES" badge-color="cyan" />
      <MetricCard title="Running Pods" :value="`${readyReplicas} / ${totalReplicas}`" :subtitle="`${healthyCount} of ${totalWorkloads} healthy workloads`" icon="🚀" badge="REPLICAS" badge-color="emerald" trend="Auto-Scaled via KEDA" trend-type="positive" />
      <MetricCard title="Canary & Blue-Green" :value="`${canaryCount + blueGreenCount}`" :subtitle="`${canaryCount} Canary · ${blueGreenCount} Blue-Green`" icon="🎯" badge="STRATEGY" badge-color="violet" trend="Zero Downtime" trend-type="positive" />
      <MetricCard title="Runtime Fleet" :value="`${k8sCount} K8s / ${swarmCount} Swarm`" subtitle="Kubernetes clusters & Swarm hosts" icon="⚡" badge="RUNTIME" badge-color="cyan" />
    </div>

    <!-- Main Workload Section -->
    <div class="section-box glass-panel table-box">
      <!-- Filter Controls Bar (Desktop & Tablet >=768px) -->
      <div class="table-controls-bar">
        <div class="filter-pills-row">
          <!-- Status Filter Pill Group -->
          <div class="status-filter-group">
            <button
              type="button"
              class="pill-btn"
              :class="{ 'pill-active': statusFilter === 'all' }"
              title="All Workload Statuses"
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
              <span>🛡️ Healthy</span>
              <span class="pill-badge">{{ healthyCount }}</span>
            </button>
            <button
              type="button"
              class="pill-btn pill-degraded"
              :class="{ 'pill-active': statusFilter === 'degraded' }"
              title="Degraded Workloads"
              @click="statusFilter = 'degraded'"
            >
              <span>⚠️ Degraded</span>
              <span class="pill-badge">{{ degradedCount }}</span>
            </button>
          </div>

          <div class="pills-divider" aria-hidden="true"></div>

          <!-- Type & Strategy Filter Pills -->
          <button type="button" class="pill-btn" :class="{ 'pill-active': activeFilterTab === 'all' }" @click="activeFilterTab = 'all'">
            <span class="btn-text-full">🌐 All Workloads</span>
            <span class="btn-text-mobile">All</span>
            <span class="pill-badge">{{ totalWorkloads }}</span>
          </button>
          <button type="button" class="pill-btn pill-canary" :class="{ 'pill-active': activeFilterTab === 'canary' }" @click="activeFilterTab = 'canary'">
            <span class="btn-text-full">🐥 Canary Rollouts</span>
            <span class="btn-text-mobile">Canary</span>
            <span class="pill-badge">{{ canaryCount }}</span>
          </button>
          <button type="button" class="pill-btn pill-bluegreen" :class="{ 'pill-active': activeFilterTab === 'bluegreen' }" @click="activeFilterTab = 'bluegreen'">
            <span class="btn-text-full">🔄 Blue-Green</span>
            <span class="btn-text-mobile">B/G</span>
            <span class="pill-badge">{{ blueGreenCount }}</span>
          </button>
          <button type="button" class="pill-btn" :class="{ 'pill-active': activeFilterTab === 'k8s' }" @click="activeFilterTab = 'k8s'">
            <span class="btn-text-full">☸️ Kubernetes</span>
            <span class="btn-text-mobile">K8s</span>
            <span class="pill-badge">{{ k8sCount }}</span>
          </button>
          <button type="button" class="pill-btn" :class="{ 'pill-active': activeFilterTab === 'swarm' }" @click="activeFilterTab = 'swarm'">
            <span class="btn-text-full">🐳 Docker / Swarm</span>
            <span class="btn-text-mobile">Swarm</span>
            <span class="pill-badge">{{ swarmCount }}</span>
          </button>
        </div>

        <div class="table-search-row">
          <div class="search-input-wrap">
            <span class="search-ico">🔍</span>
            <input v-model="searchQuery" type="text" placeholder="Filter workloads by name, image, team..." class="input-glass search-input" />
            <button v-if="searchQuery" type="button" class="clear-search-btn" @click="searchQuery = ''">✕</button>
          </div>
          <select v-model="selectedNamespaceFilter" class="input-glass select-ns font-mono">
            <option value="all">All Namespaces ({{ namespaces.length }})</option>
            <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
          </select>
        </div>
      </div>

      <!-- Desktop Data Table View (Completely suppressed on mobile <768px) -->
      <div class="desktop-table-container desktop-table-view">
        <DeploymentsTable
          :deployments="filteredDeployments"
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
          :deployments="filteredDeployments"
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
