<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useOverviewDashboard } from '../composables/useOverviewDashboard'

import OverviewHud from '../components/overview/hud/OverviewHud.vue'
import RequestFlowBar from '../components/overview/hud/RequestFlowBar.vue'
import OverviewSaturationTrends from '../components/overview/hud/OverviewSaturationTrends.vue'
import OverviewMobileStream from '../components/overview/OverviewMobileStream.vue'
import NodeCard from '../components/overview/nodes/NodeCard.vue'
import NodeTableView from '../components/overview/nodes/NodeTableView.vue'
import NodeDiagnosticsDrawer from '../components/overview/drawer/NodeDiagnosticsDrawer.vue'
import DeepDiveTrafficModal from '../components/overview/modals/DeepDiveTrafficModal.vue'
import type { MutedAlertConfig } from '../stores/alertStore'

export type { MutedAlertConfig }

const router = useRouter()
const nodeViewMode = ref<'grid' | 'table'>('table')

const {
  overview,
  loading,
  error,
  lastUpdated,
  isLiveWs,
  tpsData,
  tpsLoading,
  trendHistory,
  showDeepDiveModal,
  customNodeOrder,
  draggedNodeId,
  dragOverNodeId,
  selectedTopologyFilter,
  showNodeDrawer,
  nodeDrawerMode,
  nodeHistoryData,
  nodeHistoryLoading,
  nodeHistoryRange,
  customHistFrom,
  customHistTo,
  busiestNodeId,
  nodes,
  selectedNode,
  totalContainers,
  runningContainers,
  effectiveHttpRps,
  httpActiveConns,
  httpQueuedReqs,
  httpErrorRate,
  clusterAvgLatencyMs,
  clusterTotalMemBytes,
  clusterUsedMemBytes,
  clusterTotalDiskBytes,
  clusterUsedDiskBytes,
  peakCpuNode,
  filteredTopologyNodes,
  topologyFilterCounts,
  openDeepDiveModal,
  closeDeepDiveModal,
  resetNodeOrder,
  onDragStart,
  onDragOver,
  onDragEnter,
  onDragLeave,
  onDrop,
  onDragEnd,
  handleNodeCardClick,
  inspectNode,
  manageNode,
  pollClusterMetrics,
  loadNodeHistory,
  applyCustomHistoryPreset,
} = useOverviewDashboard()
</script>

<template>
  <div class="overview-dashboard animate-fade-in">
    <!-- Sleek 44px Mobile Command Bar (<768px) -->
    <div v-if="overview" class="mobile-command-bar">
      <div class="command-bar-left">
        <span class="command-bar-title font-semibold text-slate-100">
          🖥️ Overview • {{ overview.healthy_nodes }}/{{ overview.total_nodes }} Online
        </span>
      </div>
      <div class="command-bar-actions">
        <button
          type="button"
          class="mobile-cmd-btn"
          :disabled="loading"
          title="Refresh Telemetry"
          aria-label="Refresh Telemetry"
          @click="pollClusterMetrics"
        >
          <span class="cmd-icon" :class="{ 'spin-icon': loading || tpsLoading }">🔄</span>
          <span class="cmd-label">Refresh</span>
        </button>
        <button
          type="button"
          class="mobile-cmd-btn"
          title="Deep-Dive Telemetry"
          aria-label="Deep-Dive Telemetry"
          @click="openDeepDiveModal"
        >
          <span class="cmd-icon">📊</span>
          <span class="cmd-label">Deep-Dive</span>
        </button>
      </div>
    </div>

    <!-- Desktop Header Bar (>=768px) -->
    <header class="dashboard-header">
      <div class="header-titles">
        <div class="header-badge-group">
          <span class="badge" :class="isLiveWs ? 'badge-emerald' : 'badge-cyan'">
            <span class="pulse-dot" :class="{ 'pulse-active': isLiveWs }"></span>
            <span>{{ isLiveWs ? 'Live Stream' : 'Telemetry Synced' }}</span>
          </span>
          <span class="badge badge-indigo badge-auto-refresh">Auto-Refresh 5s</span>
          <span class="last-sync-text">Updated: {{ lastUpdated.toLocaleTimeString() }}</span>
        </div>
        <h1 class="page-title">Cluster Overview</h1>
        <p class="page-desc">
          Real-time cluster topology, container saturation metrics, dynamic resource gauges, and autonomous threshold alerting.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" @click="pollClusterMetrics" :disabled="loading" title="Refresh Telemetry">
          <span class="btn-icon" :class="{ 'spin-icon': loading || tpsLoading }">🔄</span>
          <span>Refresh</span>
        </button>
        <button class="btn btn-secondary" @click="openDeepDiveModal" title="Deep-Dive Telemetry">
          <span class="btn-icon">📊</span>
          <span>Deep-Dive Telemetry</span>
        </button>
      </div>
    </header>

    

    <!-- LOADING SKELETON -->
    <div v-if="loading && !overview" class="skeleton-hud-grid">
      <div v-for="i in 4" :key="i" class="skeleton-card glass-panel shimmer"></div>
    </div>

    <!-- ERROR STATE -->
    <div v-else-if="error && !overview" class="error-banner glass-panel">
      <div class="error-icon">⚠️</div>
      <div class="error-info">
        <h3>Telemetry Connection Interrupted</h3>
        <p>{{ error }}</p>
      </div>
      <button class="btn btn-primary" @click="pollClusterMetrics">Retry Telemetry Sync</button>
    </div>

    <!-- EMPTY STATE -->
    <div v-else-if="nodes.length === 0 && !loading" class="empty-state-card glass-panel">
      <div class="empty-icon">🖥️</div>
      <h2>No Infrastructure Servers Connected</h2>
      <p>Deploy k8s-agent (port 9100) on your hosts or attach compute clusters to enable real-time telemetry streaming.</p>
      <div class="empty-actions">
        <button class="btn btn-primary" @click="router.push('/hosts')">Manage Infrastructure Hosts</button>
        <button class="btn btn-secondary" @click="router.push('/settings')">Configure Settings</button>
      </div>
    </div>

    <!-- MAIN DASHBOARD CONTENT -->
    <div v-else-if="overview" class="dashboard-body">
      <!-- Mobile Overview Stream (<640px) -->
      <div class="mobile-stream-section">
        <OverviewMobileStream
          :overview="overview"
          :nodes="nodes"
          :runningContainers="runningContainers"
          :totalContainers="totalContainers"
          :effectiveHttpRps="effectiveHttpRps"
          :tpsData="tpsData"
          :isLiveWs="isLiveWs"
          :lastUpdated="lastUpdated"
          :loading="loading"
          @inspect="inspectNode"
          @refresh="pollClusterMetrics"
          @deepDive="openDeepDiveModal"
        />
      </div>

      <!-- Desktop Overview Sections (>= 640px) -->
      <div class="desktop-overview-sections">

      <!-- 1. TOP KPI HUD -->
      <OverviewHud
        :overview="overview"
        :runningContainers="runningContainers"
        :totalContainers="totalContainers"
        :peakCpuNode="peakCpuNode"
        :clusterUsedMemBytes="clusterUsedMemBytes"
        :clusterTotalMemBytes="clusterTotalMemBytes"
        :clusterUsedDiskBytes="clusterUsedDiskBytes"
        :clusterTotalDiskBytes="clusterTotalDiskBytes"
      />

      <!-- 2. ANIMATED REQUEST FLOW BAR -->
      <RequestFlowBar
        :isLiveWs="isLiveWs"
        :effectiveHttpRps="effectiveHttpRps"
        :httpQueuedReqs="httpQueuedReqs"
        :httpErrorRate="httpErrorRate"
      />

      <!-- 3. 5-MIN SATURATION TRENDS CHART -->
      <OverviewSaturationTrends
        :trendHistory="trendHistory"
        :effectiveHttpRps="effectiveHttpRps"
        :httpActiveConns="httpActiveConns"
        :httpQueuedReqs="httpQueuedReqs"
        :clusterAvgLatencyMs="clusterAvgLatencyMs"
        :httpErrorRate="httpErrorRate"
        @openDeepDive="openDeepDiveModal"
      />

      <!-- 4. INFRASTRUCTURE SERVER TOPOLOGY & NODE CARDS -->
      <section class="topology-section">
        <div class="topology-header-row">
          <div class="topology-title-group">
            <div class="topology-title-with-pulse">
              <span class="pulse-beacon"></span>
              <h2 class="section-title">Infrastructure Hosts &amp; Node Mesh</h2>
            </div>
            <p class="section-subtitle">
              Interactive cluster server topology with live telemetry gauges, drag-and-drop reordering, and deep diagnostics.
            </p>
          </div>

          <!-- Mesh Topology Connections Indicator -->
          <div class="topology-mesh-indicator glass-panel font-mono">
            <span class="mesh-dot-active"></span>
            <span class="mesh-label">Full Mesh Connected</span>
            <span class="mesh-stats font-bold text-cyan">{{ overview.healthy_nodes }}/{{ overview.total_nodes }} Online</span>
          </div>
        </div>

        <!-- Topology Filter Pills Bar -->
        <div class="topology-filter-bar glass-panel">
          <div class="topology-filter-pills">
            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'all' }"
              @click="selectedTopologyFilter = 'all'"
            >
              <span>All Servers</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.all }}</span>
            </button>

            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'control_plane' }"
              @click="selectedTopologyFilter = 'control_plane'"
            >
              <span>👑 Control-Plane</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.control }}</span>
            </button>

            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'worker' }"
              @click="selectedTopologyFilter = 'worker'"
            >
              <span>📡 Workers / Agents</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.worker }}</span>
            </button>

            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'hot' }"
              @click="selectedTopologyFilter = 'hot'"
            >
              <span>🔥 Hot Nodes</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.hot }}</span>
            </button>

            <button
              type="button"
              class="filter-pill-btn"
              :class="{ active: selectedTopologyFilter === 'overloaded' }"
              @click="selectedTopologyFilter = 'overloaded'"
            >
              <span>⚠️ Overloaded / Down</span>
              <span class="pill-count font-mono">{{ topologyFilterCounts.overloaded }}</span>
            </button>
          </div>

          <div class="topology-order-actions">
            <div class="view-mode-toggle glass-panel">
              <button class="toggle-btn" :class="{ active: nodeViewMode === 'table' }" @click="nodeViewMode = 'table'">📑 Bảng</button>
              <button class="toggle-btn" :class="{ active: nodeViewMode === 'grid' }" @click="nodeViewMode = 'grid'">🗂 Thẻ</button>
            </div>
            
            <button v-if="customNodeOrder.length > 0" class="btn-reset-order font-mono" @click="resetNodeOrder" title="Reset customized card order">
              <span>↺ Reset Card Order</span>
            </button>
          </div>
        </div>

        <template v-if="nodeViewMode === 'table'">
          <NodeTableView
            :nodes="filteredTopologyNodes"
            :busiestNodeId="busiestNodeId"
            @click="handleNodeCardClick"
            @details="inspectNode"
            @logs="manageNode"
            @scale="manageNode"
            @restart="manageNode"
            @yaml="manageNode"
            @delete="manageNode"
          />
        </template>
        <template v-else>
          <!-- Node Cards Grid -->
          <div class="node-cards-grid">
          <div
            v-if="filteredTopologyNodes.length === 0"
            class="empty-topology-state glass-panel"
          >
            <span class="empty-topology-icon">🔍</span>
            <span class="empty-topology-text">No servers match the selected filter "{{ selectedTopologyFilter }}".</span>
            <button class="btn-reset-filters" @click="selectedTopologyFilter = 'all'">Show All Servers</button>
          </div>

          <NodeCard
            v-for="node in filteredTopologyNodes"
            :key="node.node_id"
            :node="node"
            :busiestNodeId="busiestNodeId" :draggedNodeId="draggedNodeId" :dragOverNodeId="dragOverNodeId"
            @click="handleNodeCardClick"
            @inspect="inspectNode"
            @manage="manageNode"
            @dragstart="onDragStart" @dragover="onDragOver" @dragenter="onDragEnter" @dragleave="onDragLeave" @drop="onDrop" @dragend="onDragEnd"
          />
        </div>
        </template>
      </section>
      </div>
    </div>

    <!-- NODE DIAGNOSTICS & TOP PROCESSES INSPECTOR DRAWER -->
    <NodeDiagnosticsDrawer
      v-model:show="showNodeDrawer"
      :node="selectedNode"
      :overview="overview"
      :tpsData="tpsData"
      :nodeHistoryData="nodeHistoryData"
      :nodeHistoryLoading="nodeHistoryLoading"
      v-model:nodeHistoryRange="nodeHistoryRange"
      v-model:customHistFrom="customHistFrom"
      v-model:customHistTo="customHistTo"
      v-model:initialMode="nodeDrawerMode"
      @range-change="loadNodeHistory"
      @custom-range-apply="loadNodeHistory(undefined, 'custom', customHistFrom, customHistTo)"
      @apply-preset="applyCustomHistoryPreset"
      @manage-host="manageNode"
      @close="showNodeDrawer = false"
    />

    <!-- CLUSTER TELEMETRY & THROUGHPUT DEEP-DIVE MODAL -->
    <DeepDiveTrafficModal
      v-model:show="showDeepDiveModal"
      :trendHistory="trendHistory"
      :nodes="nodes"
      :tpsData="tpsData"
      :dockerContainers="overview?.containers || []"
      @close="closeDeepDiveModal"
    />
  </div>
</template>

<style scoped>
@import '../assets/styles/views/overview.css';
@import '../assets/styles/views/overview-mobile.css';
</style>