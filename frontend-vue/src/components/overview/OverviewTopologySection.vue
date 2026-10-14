<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import NodeCard from './nodes/NodeCard.vue'
import NodeTableView from './nodes/NodeTableView.vue'
import VirtualNodeGrid from './virtual/VirtualNodeGrid.vue'
import type { SystemOverview, NodeMetrics } from '../../api/overview'

defineProps<{
  overview: SystemOverview | null
  filteredTopologyNodes: NodeMetrics[]
  topologyFilterCounts: { all: number; control: number; worker: number; hot: number; overloaded: number }
  selectedTopologyFilter: string
  nodeViewMode: 'grid' | 'table'
  busiestNodeId: string | null
  customNodeOrder: string[]
  draggedNodeId: string | null
  dragOverNodeId: string | null
}>()

const emit = defineEmits<{
  (e: 'update:selectedTopologyFilter', filter: string): void
  (e: 'update:nodeViewMode', mode: 'grid' | 'table'): void
  (e: 'resetNodeOrder'): void
  (e: 'cardClick', node: NodeMetrics): void
  (e: 'inspect', node: NodeMetrics): void
  (e: 'manage', node: NodeMetrics): void
  (e: 'logs', node: NodeMetrics): void
  (e: 'dragstart', event: DragEvent, node: NodeMetrics): void
  (e: 'dragover', event: DragEvent, node: NodeMetrics): void
  (e: 'dragenter', node: NodeMetrics): void
  (e: 'dragleave', event: DragEvent, node: NodeMetrics): void
  (e: 'drop', node: NodeMetrics): void
  (e: 'dragend'): void
}>()
</script>

<template>
  <section class="topology-section">
    <div class="hosts-header-row topology-header-row">
      <div class="hosts-title-group">
        <span class="hosts-pulse-dot"></span>
        <h2 class="hosts-section-title">Infrastructure Hosts</h2>
      </div>

      <!-- Compact Monospace Live Status Badge -->
      <div v-if="overview" class="hosts-status-badge font-mono" title="Online servers out of total cluster servers">
        <span class="hosts-dot-online"></span>
        <span class="hosts-status-text font-bold text-cyan">{{ overview.healthy_nodes }}/{{ overview.total_nodes }} Online</span>
      </div>
    </div>

    <!-- Sleek Unified 38px Host Controls Toolbar -->
    <div class="hosts-controls-toolbar topology-filter-bar glass-panel">
      <!-- Left side: Smooth Capsule Pills -->
      <div class="hosts-capsule-pills topology-filter-pills" role="tablist" aria-label="Filter servers by state">
        <button
          type="button"
          class="hosts-capsule-pill filter-pill-btn"
          :class="{ active: selectedTopologyFilter === 'all' }"
          @click="emit('update:selectedTopologyFilter', 'all')"
        >
          All ({{ topologyFilterCounts.all }})
        </button>

        <button
          type="button"
          class="hosts-capsule-pill filter-pill-btn"
          :class="{ active: selectedTopologyFilter === 'control_plane' }"
          @click="emit('update:selectedTopologyFilter', 'control_plane')"
        >
          Control-Plane ({{ topologyFilterCounts.control }})
        </button>

        <button
          type="button"
          class="hosts-capsule-pill filter-pill-btn"
          :class="{ active: selectedTopologyFilter === 'worker' }"
          @click="emit('update:selectedTopologyFilter', 'worker')"
        >
          Workers ({{ topologyFilterCounts.worker }})
        </button>

        <button
          type="button"
          class="hosts-capsule-pill filter-pill-btn"
          :class="{ active: selectedTopologyFilter === 'hot' }"
          @click="emit('update:selectedTopologyFilter', 'hot')"
        >
          Hot ({{ topologyFilterCounts.hot }})
        </button>

        <button
          type="button"
          class="hosts-capsule-pill filter-pill-btn"
          :class="{ active: selectedTopologyFilter === 'overloaded' }"
          @click="emit('update:selectedTopologyFilter', 'overloaded')"
        >
          Overloaded ({{ topologyFilterCounts.overloaded }})
        </button>
      </div>

      <!-- Right side: Segmented View Mode Toggle & Reset Order -->
      <div class="hosts-toolbar-right topology-order-actions">
        <div class="hosts-segmented-toggle view-mode-toggle" role="group" aria-label="View mode">
          <button
            type="button"
            class="hosts-seg-btn toggle-btn"
            :class="{ active: nodeViewMode === 'table' }"
            @click="emit('update:nodeViewMode', 'table')"
            title="Table view"
          >
            <BaseIcon name="file-text" size="xs" /> Table
          </button>
          <button
            type="button"
            class="hosts-seg-btn toggle-btn"
            :class="{ active: nodeViewMode === 'grid' }"
            @click="emit('update:nodeViewMode', 'grid')"
            title="Grid view"
          >
            <BaseIcon name="grid" size="xs" /> Grid
          </button>
        </div>

        <button
          v-if="customNodeOrder.length > 0"
          type="button"
          class="hosts-btn-reset-order btn-reset-order-icon font-mono"
          @click="emit('resetNodeOrder')"
          title="Reset customized card order"
          aria-label="Reset customized card order"
        >
          <BaseIcon name="refresh" size="xs" />
        </button>
      </div>
    </div>

    <template v-if="nodeViewMode === 'table'">
      <NodeTableView
        :nodes="filteredTopologyNodes"
        :busiestNodeId="busiestNodeId"
        @click="emit('cardClick', $event)"
        @details="emit('inspect', $event)"
        @logs="emit('logs', $event)"
        @scale="emit('manage', $event)"
        @restart="emit('manage', $event)"
        @yaml="emit('manage', $event)"
        @delete="emit('manage', $event)"
      />
    </template>
    <template v-else>
      <VirtualNodeGrid
        v-if="filteredTopologyNodes.length > 24"
        :nodes="filteredTopologyNodes"
        :viewport-height="640"
        @select="emit('cardClick', $event)"
      >
        <template #item="{ node }">
          <NodeCard
            :node="node"
            :busiestNodeId="busiestNodeId"
            :draggedNodeId="draggedNodeId"
            :dragOverNodeId="dragOverNodeId"
            @click="emit('cardClick', $event)"
            @inspect="emit('inspect', $event)"
            @manage="emit('manage', $event)"
            @dragstart="emit('dragstart', $event, node)"
            @dragover="emit('dragover', $event, node)"
            @dragenter="emit('dragenter', node)"
            @dragleave="emit('dragleave', $event, node)"
            @drop="emit('drop', node)"
            @dragend="emit('dragend')"
          />
        </template>
      </VirtualNodeGrid>
      <!-- Node Cards Grid -->
      <div v-else class="node-cards-grid">
        <div
          v-if="filteredTopologyNodes.length === 0"
          class="empty-topology-state glass-panel"
        >
          <span class="empty-topology-icon">
            <BaseIcon name="search" size="lg" />
          </span>
          <span class="empty-topology-text">No servers match the selected filter "{{ selectedTopologyFilter }}".</span>
          <button class="btn-reset-filters" @click="emit('update:selectedTopologyFilter', 'all')">Show All Servers</button>
        </div>

        <NodeCard
          v-for="node in filteredTopologyNodes"
          :key="node.node_id"
          :node="node"
          :busiestNodeId="busiestNodeId"
          :draggedNodeId="draggedNodeId"
          :dragOverNodeId="dragOverNodeId"
          @click="emit('cardClick', $event)"
          @inspect="emit('inspect', $event)"
          @manage="emit('manage', $event)"
          @dragstart="emit('dragstart', $event, node)"
          @dragover="emit('dragover', $event, node)"
          @dragenter="emit('dragenter', node)"
          @dragleave="emit('dragleave', $event, node)"
          @drop="emit('drop', node)"
          @dragend="emit('dragend')"
        />
      </div>
    </template>
  </section>
</template>
