<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import type { NodeMetrics } from '../../../api/overview'

defineProps<{
  nodes: NodeMetrics[]
  busiestNodeId?: string | null
}>()

const emit = defineEmits<{
  (e: 'click', node: NodeMetrics): void
  (e: 'details', node: NodeMetrics): void
  (e: 'logs', node: NodeMetrics): void
  (e: 'scale', node: NodeMetrics): void
  (e: 'restart', node: NodeMetrics): void
  (e: 'yaml', node: NodeMetrics): void
  (e: 'delete', node: NodeMetrics): void
}>()

const activeMenuId = ref<string | null>(null)

function toggleMenu(nodeId: string) {
  activeMenuId.value = activeMenuId.value === nodeId ? null : nodeId
}

function handleMenuAction(event: 'scale' | 'restart' | 'yaml' | 'delete', node: NodeMetrics) {
  activeMenuId.value = null
  if (event === 'scale') emit('scale', node)
  else if (event === 'restart') emit('restart', node)
  else if (event === 'yaml') emit('yaml', node)
  else if (event === 'delete') emit('delete', node)
}

function handleDocumentClick(e: MouseEvent) {
  if (activeMenuId.value && !(e.target as HTMLElement).closest('.more-actions-wrap')) {
    activeMenuId.value = null
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && activeMenuId.value) {
    activeMenuId.value = null
  }
}

onMounted(() => {
  document.addEventListener('click', handleDocumentClick)
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('click', handleDocumentClick)
  document.removeEventListener('keydown', handleKeydown)
})

function isOffline(node: NodeMetrics): boolean {
  return node.status === 'down' || node.status === 'offline' || node.status === 'disconnected' || node.memory_total === 0
}

function getNodeStatus(node: NodeMetrics): { type: 'ready' | 'offline' | 'degraded'; label: string } {
  if (isOffline(node)) return { type: 'offline', label: 'Offline' }
  if (node.cpu_percent >= 80 || node.memory_percent >= 80 || node.disk_percent >= 90 || node.status === 'degraded' || node.status === 'warning') {
    return { type: 'degraded', label: 'Degraded' }
  }
  return { type: 'ready', label: 'Ready' }
}

function getRoleBadge(role?: string): { label: string; cls: string } {
  const r = (role || '').toLowerCase()
  if (r.includes('master') || r.includes('control') || r.includes('manager')) return { label: 'Control', cls: 'badge-control' }
  if (r.includes('worker')) return { label: 'Worker', cls: 'badge-worker' }
  return { label: 'Agent', cls: 'badge-agent' }
}

function getNodeIp(node: NodeMetrics): string {
  const anyNode = node as any
  if (anyNode.ip) return anyNode.ip
  if (anyNode.ip_address) return anyNode.ip_address
  if (anyNode.internal_ip) return anyNode.internal_ip
  if (anyNode.endpoint) return anyNode.endpoint.replace(/^https?:\/\//, '').split(':')[0]
  return '--'
}

function getOsDistro(node: NodeMetrics): string {
  const d = (node.os_distro || node.os || 'Linux').trim()
  for (const name of ['Ubuntu', 'Debian', 'CentOS', 'Alpine']) {
    if (d.toLowerCase().includes(name.toLowerCase())) return name
  }
  return d.length > 10 ? d.slice(0, 10) : d
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0B'
  const k = 1024
  const sizes = ['B', 'K', 'M', 'G', 'T']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))}${sizes[i] || 'B'}`
}

function getNodePing(node: NodeMetrics): number {
  const anyNode = node as any
  if (typeof anyNode.ping_ms === 'number') return anyNode.ping_ms
  if (typeof anyNode.latency_ms === 'number') return anyNode.latency_ms
  if (anyNode.latency && !isNaN(parseInt(anyNode.latency))) return parseInt(anyNode.latency)
  return 0
}
</script>

<template>
  <div class="node-table-container glass-panel animate-fade-in">
    <div class="table-responsive">
      <table class="node-table">
        <thead>
          <tr>
            <th class="col-status">Status</th>
            <th class="col-name">Node Name &amp; Role</th>
            <th class="col-ip">IP Address &amp; OS</th>
            <th class="col-cpu">CPU Load</th>
            <th class="col-ram">Memory RAM</th>
            <th class="col-disk">Disk Storage</th>
            <th class="col-workloads">Workloads</th>
            <th class="col-probe">Probes / Ping</th>
            <th class="col-actions text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(node, idx) in nodes"
            :key="node.node_id"
            class="node-row"
            :class="{ 'row-busiest': node.node_id === busiestNodeId }"
            @click="emit('click', node)"
          >
            <td class="col-status">
              <span class="status-wrap font-mono" :class="`status-${getNodeStatus(node).type}`">
                <span class="pulse-dot" :class="`dot-${getNodeStatus(node).type}`"></span>
                <span class="status-label">{{ getNodeStatus(node).label }}</span>
              </span>
            </td>
            <td class="col-name">
              <div class="name-role-cell">
                <span class="node-name-text font-bold" :title="node.node_name">{{ node.node_name }}</span>
                <span class="role-badge font-mono" :class="getRoleBadge(node.role).cls">[{{ getRoleBadge(node.role).label }}]</span>
                <span v-if="node.node_id === busiestNodeId" class="badge-hot" title="Highest traffic">🔥</span>
              </div>
            </td>
            <td class="col-ip font-mono">
              <span class="ip-text">{{ getNodeIp(node) }}</span>
              <span class="os-distro-pill">{{ getOsDistro(node) }}</span>
            </td>
            <td class="col-cpu font-mono">
              <span v-if="isOffline(node)" class="text-muted">—</span>
              <div v-else class="progress-cell">
                <span class="pct-num">{{ Math.round(node.cpu_percent) }}%</span>
                <div class="bar-track">
                  <div class="bar-fill" :class="node.cpu_percent >= 80 ? 'fill-rose' : node.cpu_percent >= 60 ? 'fill-amber' : 'fill-cyan'" :style="{ width: `${Math.min(100, Math.max(0, node.cpu_percent))}%` }"></div>
                </div>
              </div>
            </td>
            <td class="col-ram font-mono">
              <span v-if="isOffline(node)" class="text-muted">— / —</span>
              <span v-else class="resource-text"><strong class="text-cyan">{{ Math.round(node.memory_percent) }}%</strong><span class="sub-dim">({{ formatBytes(node.memory_used) }}/{{ formatBytes(node.memory_total) }})</span></span>
            </td>
            <td class="col-disk font-mono">
              <span v-if="isOffline(node)" class="text-muted">— / —</span>
              <span v-else class="resource-text"><strong class="text-emerald">{{ Math.round(node.disk_percent) }}%</strong><span class="sub-dim">({{ formatBytes(node.disk_used) }}/{{ formatBytes(node.disk_total) }})</span></span>
            </td>
            <td class="col-workloads font-mono">
              <span v-if="isOffline(node)" class="text-muted">—</span>
              <span v-else class="badge-workload">{{ node.running_count ?? node.container_count ?? 0 }} ctr</span>
            </td>
            <td class="col-probe font-mono">
              <span v-if="isOffline(node) || getNodePing(node) <= 0" class="text-muted">--</span>
              <span v-else class="probe-val">⚡ {{ getNodePing(node) }}ms</span>
            </td>
            <td class="col-actions text-right" @click.stop>
              <div class="sre-suite">
                <button type="button" class="sre-btn btn-logs" title="Stream Logs" @click="emit('logs', node)">📄 Logs</button>
                <button type="button" class="sre-btn btn-details" title="Diagnostics & Details" @click="emit('details', node)">🔍 Details</button>
                <div class="more-actions-wrap">
                  <button
                    type="button"
                    class="sre-btn btn-more-actions"
                    title="More Node Actions"
                    :class="{ active: activeMenuId === node.node_id }"
                    :aria-expanded="activeMenuId === node.node_id"
                    aria-haspopup="true"
                    @click.stop="toggleMenu(node.node_id)"
                  >
                    ⋯
                  </button>
                  <div
                    v-if="activeMenuId === node.node_id"
                    class="more-actions-dropdown"
                    :class="{ dropup: idx >= nodes.length - 2 && nodes.length > 2 }"
                    @click.stop
                  >
                    <button type="button" class="menu-item item-scale" @click="handleMenuAction('scale', node)">
                      <span class="menu-item-icon">⚡</span>
                      <span>Scale Workloads</span>
                    </button>
                    <button type="button" class="menu-item item-restart" @click="handleMenuAction('restart', node)">
                      <span class="menu-item-icon">🔄</span>
                      <span>Restart Agent</span>
                    </button>
                    <button type="button" class="menu-item item-yaml" @click="handleMenuAction('yaml', node)">
                      <span class="menu-item-icon">🎯</span>
                      <span>View YAML</span>
                    </button>
                    <button type="button" class="menu-item item-delete" @click="handleMenuAction('delete', node)">
                      <span class="menu-item-icon">🗑</span>
                      <span>Cordon / Evict</span>
                    </button>
                  </div>
                </div>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/views/overview.css';
</style>
