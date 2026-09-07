<script setup lang="ts">
import MetricCard from '../components/ui/MetricCard.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import SwarmNodesGrid from '../components/swarm/SwarmNodesGrid.vue'
import SwarmServicesTable from '../components/swarm/SwarmServicesTable.vue'
import SwarmContainersGrid from '../components/swarm/SwarmContainersGrid.vue'
import SwarmMobileCards from '../components/swarm/SwarmMobileCards.vue'
import DeploySwarmStackModal from '../components/swarm/DeploySwarmStackModal.vue'
import SwarmServiceDrawer from '../components/swarm/SwarmServiceDrawer.vue'
import SwarmNodeDrawer from '../components/swarm/SwarmNodeDrawer.vue'
import { useDockerSwarm } from '../composables/useDockerSwarm'
import '../assets/styles/views/swarm.css'

const {
  loading,
  error,
  actionLoading,
  toastMessage,
  activeTab,
  services,
  nodes,
  containers,
  showLogsDrawer,
  logTargetTitle,
  logContent,
  showNodeDrawer,
  selectedNode,
  selectedNodeDetails,
  nodeDetailsLoading,
  showDeployModal,
  deployingStack,
  showServiceDrawer,
  selectedService,
  selectedServiceTasks,
  serviceTasksLoading,
  totalServices,
  totalNodes,
  totalContainers,
  activeManagers,
  fetchDockerData,
  stepReplicas,
  updateService,
  removeService,
  deployStack,
  handleToggleContainer,
  drainNode,
  activateNode,
  viewLogs,
  inspectNode,
  inspectService,
} = useDockerSwarm()
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Desktop Header (>640px) -->
    <div class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>STANDALONE DOCKER & SWARM COMPUTE ENGINE</span>
        </div>
        <h1 class="view-title">Docker Swarm & Container Racks</h1>
        <p class="view-desc">
          Compute node rack telemetry, active swarm services, standalone containers, and streaming log consoles.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-primary" @click="showDeployModal = true">
          <span>🚀 Deploy Stack</span>
        </button>
        <button class="btn btn-secondary" :disabled="loading" @click="fetchDockerData">
          <span>{{ loading ? '⏳ Querying...' : '🔄 Refresh Daemon' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<=640px) -->
    <div class="swarm-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🐳 Swarm ({{ totalServices }})</span>
      </div>
      <div class="command-bar-actions">
        <button class="btn-icon-cmd" title="Deploy Stack" aria-label="Deploy Stack" @click="showDeployModal = true">
          <span>🚀</span>
        </button>
        <button class="btn-icon-cmd" :disabled="loading" title="Refresh Daemon" aria-label="Refresh Daemon" @click="fetchDockerData">
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<=640px) -->
    <div class="swarm-micro-telemetry mobile-only font-mono" role="status" aria-label="Docker Swarm Micro Telemetry">
      <span class="tel-item tel-svcs">🐳 {{ totalServices }} svcs</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-nodes">🖥️ {{ totalNodes }} nodes</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-ctrs">📦 {{ totalContainers }} ctrs</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-vip">⚡ VIP Mesh</span>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" aria-label="Close notification" @click="toastMessage = null">✕</button>
    </div>

    <!-- Error Banner -->
    <div v-if="error" class="toast-banner toast-error animate-fade-in">
      <span>⚠️</span>
      <span>{{ error }}</span>
      <button class="toast-close" aria-label="Close error" @click="error = null">✕</button>
    </div>

    <!-- Metric HUD (Desktop only) -->
    <div class="metrics-grid desktop-metrics desktop-only">
      <MetricCard
        title="Swarm Services"
        :value="totalServices"
        subtitle="Active replicated overlay services"
        icon="🐳"
        badge="SERVICES"
        badge-color="cyan"
      />
      <MetricCard
        title="Compute Nodes"
        :value="totalNodes"
        :subtitle="`${activeManagers} Active Swarm Managers`"
        icon="🖥️"
        badge="NODES"
        badge-color="emerald"
        trend="Quorum Healthy"
        trend-type="positive"
      />
      <MetricCard
        title="Running Containers"
        :value="totalContainers"
        subtitle="Standalone socket container instances"
        icon="📦"
        badge="CONTAINERS"
        badge-color="violet"
      />
      <MetricCard
        title="Engine VIP Mesh"
        value="Overlay Ready"
        subtitle="Zero-loss virtual IP round-robin routing"
        icon="⚡"
        badge="VIP MESH"
        badge-color="cyan"
        trend="Active Mesh"
        trend-type="positive"
      />
    </div>

    <!-- Tab Control Bar (3 Tabs) -->
    <div class="tab-control-bar">
      <button
        class="tab-btn"
        :class="{ 'tab-active': activeTab === 'services' }"
        @click="activeTab = 'services'"
      >
        <span>🐳 Swarm Services ({{ services.length }})</span>
      </button>
      <button
        class="tab-btn"
        :class="{ 'tab-active': activeTab === 'nodes' }"
        @click="activeTab = 'nodes'"
      >
        <span>🖥️ Node Racks & Utilization ({{ nodes.length }})</span>
      </button>
      <button
        class="tab-btn"
        :class="{ 'tab-active': activeTab === 'containers' }"
        @click="activeTab = 'containers'"
      >
        <span>⚡ Containers ({{ containers.length }})</span>
      </button>
    </div>

    <!-- Mobile Card Stream (Mobile Viewports) -->
    <div class="mobile-only-stream">
      <SwarmMobileCards
        :active-tab="activeTab"
        :services="services"
        :nodes="nodes"
        :containers="containers"
        :action-loading="actionLoading"
        @inspect-service="inspectService"
        @inspect-node="inspectNode"
        @scale-service="stepReplicas"
        @view-logs="viewLogs"
        @toggle-container="handleToggleContainer"
      />
    </div>

    <!-- Desktop Grid & Table Views -->
    <div class="desktop-only-block">
      <!-- 1. Swarm Services Tab -->
      <SwarmServicesTable
        v-if="activeTab === 'services'"
        :services="services"
        :action-loading="actionLoading"
        @inspect="inspectService"
        @scale="stepReplicas"
        @update="inspectService"
        @logs="(id, name) => viewLogs(id, name, 'service')"
        @remove="removeService"
      />

      <!-- 2. Compute Node Racks Tab -->
      <SwarmNodesGrid
        v-else-if="activeTab === 'nodes'"
        :nodes="nodes"
        :action-loading="actionLoading"
        @inspect="inspectNode"
        @drain="drainNode"
        @activate="activateNode"
      />

      <!-- 3. Container Power Switches View -->
      <SwarmContainersGrid
        v-else-if="activeTab === 'containers'"
        :containers="containers"
        :action-loading="actionLoading"
        @toggle="handleToggleContainer"
        @logs="(id, name) => viewLogs(id, name, 'container')"
      />
    </div>

    <!-- Modals & Drawers -->
    <DeploySwarmStackModal
      v-model:show="showDeployModal"
      :loading="deployingStack"
      @deploy="deployStack"
      @close="showDeployModal = false"
    />

    <SwarmServiceDrawer
      v-model:show="showServiceDrawer"
      :service="selectedService"
      :tasks="selectedServiceTasks"
      :loading="serviceTasksLoading"
      :action-loading="actionLoading"
      @scale="stepReplicas"
      @update="(svc) => updateService(svc.id, { replicas: svc.replicas })"
      @logs="(id, name) => viewLogs(id, name, 'service')"
      @close="showServiceDrawer = false"
    />

    <SwarmNodeDrawer
      v-model:show="showNodeDrawer"
      :node="selectedNode"
      :details="selectedNodeDetails"
      :loading="nodeDetailsLoading"
      @close="showNodeDrawer = false"
    />

    <!-- Logs Viewer Drawer -->
    <ModalDrawer
      v-model:show="showLogsDrawer"
      mode="drawer"
      :title="logTargetTitle"
      subtitle="Live stdout / stderr container daemon stream"
      max-width="620px"
    >
      <div class="logs-console-window">
        <pre class="logs-terminal font-mono">{{ logContent }}</pre>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Close Logs</button>
      </template>
    </ModalDrawer>
  </div>
</template>
