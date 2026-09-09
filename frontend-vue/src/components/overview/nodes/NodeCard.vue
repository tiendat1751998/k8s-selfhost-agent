<script setup lang="ts">
import type { NodeMetrics } from '../../../api/overview'
import NodeCardGauges from './NodeCardGauges.vue'
import NodeCardMetaGrid from './NodeCardMetaGrid.vue'

interface Props {
  node: NodeMetrics
  busiestNodeId?: string | null
  draggedNodeId?: string | null
  dragOverNodeId?: string | null
}

defineProps<Props>()

defineEmits<{
  (e: 'click', node: NodeMetrics): void
  (e: 'inspect', node: NodeMetrics): void
  (e: 'manage', node: NodeMetrics): void
  (e: 'dragstart', event: DragEvent, node: NodeMetrics): void
  (e: 'dragover', event: DragEvent, node: NodeMetrics): void
  (e: 'dragenter', node: NodeMetrics): void
  (e: 'dragleave', event: DragEvent, node: NodeMetrics): void
  (e: 'drop', node: NodeMetrics): void
  (e: 'dragend'): void
}>()

function isNodeOffline(node: NodeMetrics): boolean {
  return node.status === 'down' || node.status === 'offline' || node.status === 'disconnected' || node.memory_total === 0
}

function getNodeCardClass(node: NodeMetrics): string {
  if (isNodeOffline(node)) return 'node-down'
  if (node.cpu_percent >= 80 || node.memory_percent >= 80) return 'node-overloaded'
  if (node.cpu_percent >= 60 || node.memory_percent >= 60) return 'node-warning'
  return 'node-healthy'
}

function formatDistro(distro?: string, os?: string): string {
  if (distro && distro.trim().length > 0) return distro.trim()
  if (os && os.trim().length > 0) return os.trim()
  return 'Linux'
}

function formatShortDistro(distro?: string, os?: string): string {
  const d = formatDistro(distro, os)
  for (const name of ['Ubuntu', 'Debian', 'CentOS', 'Fedora', 'Alpine', 'Arch']) {
    if (d.toLowerCase().includes(name.toLowerCase())) return name
  }
  if (d.toLowerCase().includes('red hat') || d.toLowerCase().includes('rhel')) return 'RHEL'
  return d
}

function formatOsSummary(node?: NodeMetrics | null): string {
  if (!node) return 'Linux (amd64)'
  const arch = node.arch || 'amd64'
  const distro = formatShortDistro(node.os_distro, node.os)
  return `${distro} (${arch})`
}
</script>

<template>
  <div
    class="node-card glass-panel cursor-pointer smooth-opacity"
    :class="[
      getNodeCardClass(node),
      {
        'is-busiest': node.node_id === busiestNodeId,
        'is-dragging': draggedNodeId === node.node_id,
        'is-drag-over': dragOverNodeId === node.node_id && draggedNodeId !== node.node_id,
      }
    ]"
    draggable="true"
    @dragstart="$emit('dragstart', $event, node)"
    @dragover.prevent="$emit('dragover', $event, node)"
    @dragenter.prevent="$emit('dragenter', node)"
    @dragleave="$emit('dragleave', $event, node)"
    @drop.prevent="$emit('drop', node)"
    @dragend="$emit('dragend')"
    @click="$emit('click', node)"
  >
    <!-- Card Top: Name, Source Badge, Role & Status (2-Row Layout) -->
    <div class="node-card-header">
      <div class="node-header-top">
        <div class="node-identity">
          <span class="drag-handle" title="Drag to reorder server card">⋮⋮</span>
          <span class="node-status-dot status-dot-pulse" :class="!isNodeOffline(node) && (node.status === 'ready' || node.status === 'online') ? 'status-green' : 'status-red'"></span>
          <h3 class="node-name" :title="node.node_name">{{ node.node_name }}</h3>
        </div>
        <div class="node-header-badges">
          <span v-if="isNodeOffline(node)" class="badge badge-offline" title="Node Offline">🔴 OFFLINE</span>
          <span v-else-if="node.node_id === busiestNodeId" class="badge badge-amber badge-traffic-pulse" title="Highest traffic node">🔥 HOT NODE</span>
          <span v-else-if="node.role?.toLowerCase() === 'master' || node.role?.toLowerCase() === 'control-plane' || node.role?.toLowerCase() === 'manager'" class="badge badge-purple" title="Cluster Manager">👑 MANAGER</span>
          <span v-else class="badge badge-indigo" title="Telemetry Agent">📡 AGENT</span>
        </div>
      </div>
      <div class="node-header-meta">
        <span class="badge-meta font-mono" :title="formatOsSummary(node)">🐧 {{ formatOsSummary(node) }}</span>
        <span class="badge-meta font-mono" :title="`${node.running_count ?? node.container_count ?? 0} Active Containers · ${node.processes || 0} Host PIDs`">
          📦 {{ isNodeOffline(node) ? '—' : (node.running_count ?? node.container_count ?? 0) }} ctr · {{ isNodeOffline(node) ? '—' : (node.processes || 0) }} pids
        </span>
      </div>
    </div>

    <!-- Gauge Cluster: CPU / RAM / Disk -->
    <NodeCardGauges :node="node" />

    <!-- Resource Detail Stats (Symmetrical 2x2 Balanced Matrix) -->
    <NodeCardMetaGrid :node="node" />

    <!-- Footer Actions: Direct Navigation & Process Inspection -->
    <div class="node-card-footer">
      <div class="node-footer-actions">
        <button type="button" class="btn-node-action btn-inspect-node" @click.stop="$emit('inspect', node)" title="Inspect real-time telemetry, hardware saturation, and top processes">
          <span>🔍 Inspect Telemetry &amp; Apps</span>
        </button>
        <button type="button" class="btn-node-action btn-manage-host" @click.stop="$emit('manage', node)" title="Manage server in Infrastructure Registry">
          <span>⚙️ Manage</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/views/overview.css';
</style>
