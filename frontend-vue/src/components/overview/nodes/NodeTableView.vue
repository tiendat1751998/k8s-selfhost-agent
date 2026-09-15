<script setup lang="ts">
import { ref, computed } from 'vue'
import type { NodeMetrics } from '../../../api/overview'
import BaseIcon from '../../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../../ui/ActionDropdown.vue'

const props = defineProps<{
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

type SortField = 'status' | 'name' | 'ip' | 'cpu' | 'ram' | 'disk' | 'workloads' | 'probe'
const sortField = ref<SortField>('status')
const sortDirection = ref<'asc' | 'desc'>('asc')

function handleSort(field: SortField) {
  if (sortField.value === field) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortField.value = field
    if (['cpu', 'ram', 'disk', 'workloads', 'probe'].includes(field)) {
      sortDirection.value = 'desc'
    } else {
      sortDirection.value = 'asc'
    }
  }
}

function getStatusRank(node: NodeMetrics): number {
  const s = getNodeStatus(node).type
  if (s === 'ready') return 0
  if (s === 'degraded') return 1
  return 2
}

const sortedNodes = computed<NodeMetrics[]>(() => {
  const list = [...props.nodes]
  const field = sortField.value
  const dir = sortDirection.value === 'asc' ? 1 : -1

  return list.sort((a, b) => {
    let diff = 0

    if (field === 'status') {
      diff = getStatusRank(a) - getStatusRank(b)
    } else if (field === 'name') {
      diff = (a.node_name || '').localeCompare(b.node_name || '')
    } else if (field === 'ip') {
      diff = getNodeIp(a).localeCompare(getNodeIp(b))
    } else if (field === 'cpu') {
      diff = (a.cpu_percent || 0) - (b.cpu_percent || 0)
    } else if (field === 'ram') {
      diff = (a.memory_percent || 0) - (b.memory_percent || 0)
    } else if (field === 'disk') {
      diff = (a.disk_percent || 0) - (b.disk_percent || 0)
    } else if (field === 'workloads') {
      const countA = a.running_count ?? a.container_count ?? 0
      const countB = b.running_count ?? b.container_count ?? 0
      diff = countA - countB
    } else if (field === 'probe') {
      diff = getNodePing(a) - getNodePing(b)
    }

    if (diff !== 0) {
      return diff * dir
    }

    // Deterministic tie-breakers: Status (Ready first) -> Name -> ID
    const statusTie = getStatusRank(a) - getStatusRank(b)
    if (statusTie !== 0) return statusTie

    const nameTie = (a.node_name || '').localeCompare(b.node_name || '')
    if (nameTie !== 0) return nameTie

    return (a.node_id || '').localeCompare(b.node_id || '')
  })
})

const nodeActions: ActionItem[] = [
  { id: 'scale', label: 'Scale Workloads', icon: 'zap' },
  { id: 'restart', label: 'Restart Agent', icon: 'refresh' },
  { id: 'yaml', label: 'View YAML', icon: 'file-text' },
  { id: 'sep-1', label: '', separator: true },
  { id: 'delete', label: 'Cordon / Evict', icon: 'trash', variant: 'danger' },
]

function handleMenuAction(event: string, node: NodeMetrics) {
  if (event === 'scale') emit('scale', node)
  else if (event === 'restart') emit('restart', node)
  else if (event === 'yaml') emit('yaml', node)
  else if (event === 'delete') emit('delete', node)
}

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
            <th class="col-status sortable-th" @click="handleSort('status')">
              <div class="th-sort-wrap">
                <span>STATUS</span>
                <span class="sort-icon" :class="{ active: sortField === 'status' }">{{ sortField === 'status' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
              </div>
            </th>
            <th class="col-name sortable-th" @click="handleSort('name')">
              <div class="th-sort-wrap">
                <span>NODE NAME &amp; ROLE</span>
                <span class="sort-icon" :class="{ active: sortField === 'name' }">{{ sortField === 'name' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
              </div>
            </th>
            <th class="col-ip sortable-th" @click="handleSort('ip')">
              <div class="th-sort-wrap">
                <span>IP ADDRESS &amp; OS</span>
                <span class="sort-icon" :class="{ active: sortField === 'ip' }">{{ sortField === 'ip' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
              </div>
            </th>
            <th class="col-cpu sortable-th" @click="handleSort('cpu')">
              <div class="th-sort-wrap">
                <span>CPU LOAD</span>
                <span class="sort-icon" :class="{ active: sortField === 'cpu' }">{{ sortField === 'cpu' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
              </div>
            </th>
            <th class="col-ram sortable-th" @click="handleSort('ram')">
              <div class="th-sort-wrap">
                <span>MEMORY RAM</span>
                <span class="sort-icon" :class="{ active: sortField === 'ram' }">{{ sortField === 'ram' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
              </div>
            </th>
            <th class="col-disk sortable-th" @click="handleSort('disk')">
              <div class="th-sort-wrap">
                <span>DISK STORAGE</span>
                <span class="sort-icon" :class="{ active: sortField === 'disk' }">{{ sortField === 'disk' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
              </div>
            </th>
            <th class="col-workloads sortable-th" @click="handleSort('workloads')">
              <div class="th-sort-wrap">
                <span>WORKLOADS</span>
                <span class="sort-icon" :class="{ active: sortField === 'workloads' }">{{ sortField === 'workloads' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
              </div>
            </th>
            <th class="col-probe sortable-th" @click="handleSort('probe')">
              <div class="th-sort-wrap">
                <span>PROBES / PING</span>
                <span class="sort-icon" :class="{ active: sortField === 'probe' }">{{ sortField === 'probe' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
              </div>
            </th>
            <th class="col-actions text-right">ACTIONS</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="node in sortedNodes"
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
                <BaseIcon v-if="node.node_id === busiestNodeId" name="flame" size="xs" class="badge-hot" title="Highest traffic" />
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
              <span v-else class="probe-val">
                <BaseIcon name="zap" size="xs" /> {{ getNodePing(node) }}ms
              </span>
            </td>
            <td class="col-actions text-right" @click.stop>
              <div class="sre-suite">
                <button type="button" class="sre-btn btn-logs" title="Stream Logs" @click="emit('logs', node)">
                  <BaseIcon name="file-text" size="xs" />
                  <span>Logs</span>
                </button>
                <button type="button" class="sre-btn btn-details" title="Diagnostics & Details" @click="emit('details', node)">
                  <BaseIcon name="search" size="xs" />
                  <span>Details</span>
                </button>
                <ActionDropdown
                  size="xs"
                  :items="nodeActions"
                  @select="(actionId) => handleMenuAction(actionId, node)"
                />
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

.sortable-th {
  cursor: pointer;
  user-select: none;
  transition: color 0.15s ease;
}

.sortable-th:hover {
  color: var(--text-primary, #fff);
}

.th-sort-wrap {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.sort-icon {
  font-size: 10px;
  opacity: 0.4;
}

.sort-icon.active {
  opacity: 1;
  color: var(--color-cyan, #06b6d4);
  font-weight: bold;
}
</style>
