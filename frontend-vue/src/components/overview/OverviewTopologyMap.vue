<script setup lang="ts">
import type { NodeMetrics } from '../../api/overview'
import NodeCard from './nodes/NodeCard.vue'

interface Props {
  nodes: NodeMetrics[]
  filteredNodes: NodeMetrics[]
  healthyCount: number
  totalCount: number
  busiestNodeId: string | null
  selectedFilter: 'all' | 'control_plane' | 'worker' | 'hot' | 'overloaded'
  filterCounts: {
    all: number
    control: number
    worker: number
    hot: number
    overloaded: number
  }
  hasCustomOrder: boolean
  draggedNodeId: string | null
  dragOverNodeId: string | null
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:selectedFilter', filter: 'all' | 'control_plane' | 'worker' | 'hot' | 'overloaded'): void
  (e: 'inspect', node: NodeMetrics): void
  (e: 'manage', node: NodeMetrics): void
  (e: 'cardClick', node: NodeMetrics): void
  (e: 'resetOrder'): void
  (e: 'dragstart', event: DragEvent, node: NodeMetrics): void
  (e: 'dragover', event: DragEvent, node: NodeMetrics): void
  (e: 'dragenter', node: NodeMetrics): void
  (e: 'dragleave', event: DragEvent, node: NodeMetrics): void
  (e: 'drop', targetNode: NodeMetrics): void
  (e: 'dragend'): void
}>()
</script>

<template>
  <section class="topology-section">
    <!-- Header -->
    <div class="topology-header-row">
      <div class="topology-title-group">
        <div class="topology-title-with-pulse">
          <span class="pulse-beacon"></span>
          <h2 class="section-title">
            <span class="title-full">🖥️ Infrastructure Hosts &amp; Node Mesh</span>
            <span class="title-mobile">🖥️ Hosts &amp; Mesh</span>
          </h2>
        </div>
        <p class="section-subtitle">
          Interactive cluster server topology with live telemetry gauges, drag-and-drop reordering, and deep diagnostics.
        </p>
      </div>

      <div class="topology-mesh-indicator glass-panel font-mono">
        <span class="mesh-dot-active"></span>
        <span class="mesh-label">Full Mesh Connected</span>
        <span class="mesh-stats font-bold text-cyan">{{ healthyCount }}/{{ totalCount }} Online</span>
      </div>
    </div>

    <!-- Filter Bar -->
    <div class="topology-filter-bar glass-panel">
      <div class="topology-filter-pills">
        <button
          type="button"
          class="filter-pill-btn"
          :class="{ active: selectedFilter === 'all' }"
          @click="emit('update:selectedFilter', 'all')"
        >
          <span>All Servers</span>
          <span class="pill-count font-mono">{{ filterCounts.all }}</span>
        </button>

        <button
          type="button"
          class="filter-pill-btn"
          :class="{ active: selectedFilter === 'control_plane' }"
          @click="emit('update:selectedFilter', 'control_plane')"
        >
          <span>👑 Control-Plane</span>
          <span class="pill-count font-mono">{{ filterCounts.control }}</span>
        </button>

        <button
          type="button"
          class="filter-pill-btn"
          :class="{ active: selectedFilter === 'worker' }"
          @click="emit('update:selectedFilter', 'worker')"
        >
          <span>📡 Workers</span>
          <span class="pill-count font-mono">{{ filterCounts.worker }}</span>
        </button>

        <button
          type="button"
          class="filter-pill-btn"
          :class="{ active: selectedFilter === 'hot' }"
          @click="emit('update:selectedFilter', 'hot')"
        >
          <span>🔥 Hot Nodes</span>
          <span class="pill-count font-mono">{{ filterCounts.hot }}</span>
        </button>

        <button
          type="button"
          class="filter-pill-btn"
          :class="{ active: selectedFilter === 'overloaded' }"
          @click="emit('update:selectedFilter', 'overloaded')"
        >
          <span>⚠️ Overloaded</span>
          <span class="pill-count font-mono">{{ filterCounts.overloaded }}</span>
        </button>
      </div>

      <div class="topology-order-actions" v-if="hasCustomOrder">
        <button class="btn-reset-order font-mono" @click="emit('resetOrder')" title="Reset customized card order">
          <span>↺ Reset Card Order</span>
        </button>
      </div>
    </div>

    <!-- Grid -->
    <div class="node-cards-grid">
      <div v-if="filteredNodes.length === 0" class="empty-topology-state glass-panel">
        <span class="empty-topology-icon">🔍</span>
        <span class="empty-topology-text">No servers match the selected filter "{{ selectedFilter }}".</span>
        <button class="btn-reset-filters" @click="emit('update:selectedFilter', 'all')">Show All Servers</button>
      </div>

      <NodeCard
        v-for="node in filteredNodes"
        :key="node.node_id"
        :node="node"
        :busiestNodeId="busiestNodeId"
        :draggedNodeId="draggedNodeId"
        :dragOverNodeId="dragOverNodeId"
        @click="emit('cardClick', node)"
        @inspect="emit('inspect', node)"
        @manage="emit('manage', node)"
        @dragstart="(ev) => emit('dragstart', ev, node)"
        @dragover="(ev) => emit('dragover', ev, node)"
        @dragenter="emit('dragenter', node)"
        @dragleave="(ev) => emit('dragleave', ev, node)"
        @drop="emit('drop', node)"
        @dragend="emit('dragend')"
      />
    </div>
  </section>
</template>

<style scoped>
@import '../../assets/styles/views/overview.css';
</style>
