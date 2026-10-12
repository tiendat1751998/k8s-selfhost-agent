<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useOverviewDashboard } from '../composables/useOverviewDashboard'

import OverviewHud from '../components/overview/hud/OverviewHud.vue'
import RequestFlowBar from '../components/overview/hud/RequestFlowBar.vue'
import OverviewSaturationTrends from '../components/overview/hud/OverviewSaturationTrends.vue'
import OverviewMobileStream from '../components/overview/OverviewMobileStream.vue'
import OverviewTopologySection from '../components/overview/OverviewTopologySection.vue'
import NodeDiagnosticsDrawer from '../components/overview/drawer/NodeDiagnosticsDrawer.vue'
import DeepDiveTrafficModal from '../components/overview/modals/DeepDiveTrafficModal.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
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
  viewNodeLogs,
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
          <BaseIcon name="server" size="xs" /> Overview • {{ overview.healthy_nodes }}/{{ overview.total_nodes }} Online
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
          <span class="cmd-icon" :class="{ 'spin-icon': loading || tpsLoading }">
            <BaseIcon name="refresh" size="sm" />
          </span>
          <span class="cmd-label">Refresh</span>
        </button>
        <button
          type="button"
          class="mobile-cmd-btn"
          title="Deep-Dive Telemetry"
          aria-label="Deep-Dive Telemetry"
          @click="openDeepDiveModal"
        >
          <span class="cmd-icon">
            <BaseIcon name="activity" size="sm" />
          </span>
          <span class="cmd-label">Deep-Dive</span>
        </button>
      </div>
    </div>

    <!-- Sleek 38px Enterprise Desktop Toolbar (>=768px) -->
    <div v-if="overview" class="overview-toolbar-sleek">
      <div class="toolbar-left-group">
        <!-- Live stream status pill -->
        <span class="badge" :class="isLiveWs ? 'badge-emerald' : 'badge-cyan'">
          <span class="pulse-dot" :class="{ 'pulse-active': isLiveWs }"></span>
          <span class="font-mono text-xs">Live · {{ lastUpdated.toLocaleTimeString() }}</span>
        </span>
      </div>

      <div class="toolbar-actions-group">
        <!-- Compact Deep-Dive and Refresh Buttons -->
        <button
          type="button"
          class="btn-telemetry-action"
          @click="openDeepDiveModal"
          title="Deep-Dive Telemetry"
        >
          <BaseIcon name="activity" size="xs" />
          <span>Deep-Dive Telemetry</span>
        </button>

        <button
          type="button"
          class="btn-telemetry-refresh"
          @click="pollClusterMetrics"
          :disabled="loading"
          title="Refresh Telemetry"
          aria-label="Refresh Telemetry"
        >
          <span class="btn-icon" :class="{ 'spin-icon': loading || tpsLoading }">
            <BaseIcon name="refresh" size="xs" />
          </span>
        </button>
      </div>
    </div>

    <!-- LOADING SKELETON -->
    <div v-if="loading && !overview" class="skeleton-hud-grid">
      <div v-for="i in 4" :key="i" class="skeleton-card glass-panel shimmer"></div>
    </div>

    <!-- ERROR STATE -->
    <div v-else-if="error && !overview" class="error-banner glass-panel">
      <div class="error-icon">
        <BaseIcon name="alert-triangle" size="xl" />
      </div>
      <div class="error-info">
        <h3>Telemetry Connection Interrupted</h3>
        <p>{{ error }}</p>
      </div>
      <button class="btn btn-primary" @click="pollClusterMetrics">Retry Telemetry Sync</button>
    </div>

    <!-- EMPTY STATE -->
    <div v-else-if="nodes.length === 0 && !loading" class="empty-state-card glass-panel">
      <div class="empty-icon">
        <BaseIcon name="server" size="xl" />
      </div>
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
          :trendHistory="trendHistory"
          :clusterAvgLatencyMs="clusterAvgLatencyMs"
          @inspect="inspectNode"
          @logs="viewNodeLogs"
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
      <OverviewTopologySection
        v-model:selected-topology-filter="selectedTopologyFilter"
        v-model:node-view-mode="nodeViewMode"
        :overview="overview"
        :filtered-topology-nodes="filteredTopologyNodes"
        :topology-filter-counts="topologyFilterCounts"
        :busiest-node-id="busiestNodeId"
        :custom-node-order="customNodeOrder"
        :dragged-node-id="draggedNodeId"
        :drag-over-node-id="dragOverNodeId"
        @reset-node-order="resetNodeOrder"
        @card-click="handleNodeCardClick"
        @inspect="inspectNode"
        @manage="manageNode"
        @logs="viewNodeLogs"
        @dragstart="onDragStart"
        @dragover="onDragOver"
        @dragenter="onDragEnter"
        @dragleave="onDragLeave"
        @drop="onDrop"
        @dragend="onDragEnd"
      />
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
