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
.topology-section { display: flex; flex-direction: column; gap: 16px; }
.topology-header-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
.topology-title-group { display: flex; flex-direction: column; gap: 4px; }
.topology-title-with-pulse { display: flex; align-items: center; gap: 10px; }
.pulse-beacon { width: 10px; height: 10px; border-radius: 50%; background: #06b6d4; box-shadow: 0 0 10px #06b6d4; animation: pulseBeacon 2s infinite ease-in-out; }
@keyframes pulseBeacon { 0%, 100% { transform: scale(1); opacity: 0.8; } 50% { transform: scale(1.3); opacity: 1; filter: drop-shadow(0 0 8px #06b6d4); } }
.section-title { font-size: 1.35rem; font-weight: 800; letter-spacing: -0.02em; color: var(--text-primary, #f8fafc); margin: 0; }
.title-full { display: inline; }
.title-mobile { display: none; }
.section-subtitle { font-size: 12px; color: var(--text-secondary, #94a3b8); margin: 0; }
.topology-mesh-indicator { display: inline-flex; align-items: center; gap: 8px; padding: 6px 14px; border-radius: 999px; background: rgba(15, 23, 42, 0.65); border: 1px solid rgba(255, 255, 255, 0.08); font-size: 11.5px; }
.mesh-dot-active { width: 7px; height: 7px; border-radius: 50%; background: #10b981; box-shadow: 0 0 8px #10b981; }
.mesh-label { color: var(--text-secondary, #94a3b8); }
.text-cyan { color: #06b6d4; }
.font-bold { font-weight: 700; }
.font-mono { font-family: var(--font-mono, monospace); }
.topology-filter-bar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 14px; border-radius: 12px; background: rgba(15, 23, 42, 0.65); border: 1px solid rgba(255, 255, 255, 0.08); flex-wrap: wrap; }
.topology-filter-pills { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.filter-pill-btn { display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; border-radius: 8px; background: rgba(255, 255, 255, 0.04); border: 1px solid rgba(255, 255, 255, 0.08); color: var(--text-secondary, #94a3b8); font-size: 11.5px; font-weight: 600; cursor: pointer; transition: all 0.2s ease; }
.filter-pill-btn:hover { background: rgba(255, 255, 255, 0.08); color: #fff; }
.filter-pill-btn.active { background: rgba(56, 189, 248, 0.15); color: #38bdf8; border-color: rgba(56, 189, 248, 0.35); box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2); }
.pill-count { font-size: 10px; padding: 1px 5px; border-radius: 4px; background: rgba(255, 255, 255, 0.08); }
.btn-reset-order { display: inline-flex; align-items: center; gap: 4px; padding: 4px 10px; font-size: 11px; color: #38bdf8; background: rgba(56, 189, 248, 0.1); border: 1px solid rgba(56, 189, 248, 0.25); border-radius: 6px; cursor: pointer; transition: all 0.2s ease; }
.btn-reset-order:hover { background: rgba(56, 189, 248, 0.2); color: #fff; }
.node-cards-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 16px; }
.empty-topology-state { padding: 36px 20px; text-align: center; border-radius: 12px; display: flex; flex-direction: column; align-items: center; gap: 12px; grid-column: 1 / -1; color: var(--text-muted, #64748b); }
.empty-topology-icon { font-size: 28px; }
.empty-topology-text { font-size: 13px; color: var(--text-secondary, #94a3b8); }
.btn-reset-filters { background: rgba(56, 189, 248, 0.12); border: 1px solid rgba(56, 189, 248, 0.3); color: #38bdf8; padding: 6px 14px; border-radius: 8px; font-size: 12px; font-weight: 600; cursor: pointer; }
.btn-reset-filters:hover { background: rgba(56, 189, 248, 0.22); }
@media (max-width: 640px) {
  .topology-header-row { flex-direction: column; align-items: flex-start; gap: 8px; }
  .topology-title-group { width: 100%; }
  .section-subtitle { display: none; }
  .title-full { display: none; }
  .title-mobile { display: inline; }
  .topology-mesh-indicator { width: 100%; justify-content: space-between; padding: 6px 10px; font-size: 10.5px; }
  .topology-filter-bar { padding: 8px 10px; gap: 8px; }
  .topology-filter-pills { width: 100%; overflow-x: auto; flex-wrap: nowrap; gap: 6px; padding: 4px 2px; }
  .filter-pill-btn { white-space: nowrap; flex-shrink: 0; padding: 6px 10px; font-size: 11.5px; }
  .node-cards-grid { grid-template-columns: 1fr; gap: 12px; }
}
</style>
