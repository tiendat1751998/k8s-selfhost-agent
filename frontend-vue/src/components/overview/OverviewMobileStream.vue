<script setup lang="ts">
import { ref } from 'vue'
import type { SystemOverview, NodeMetrics } from '../../api/overview'
import NodeTableView from './nodes/NodeTableView.vue'

interface Props {
  overview: SystemOverview
  nodes: NodeMetrics[]
  runningContainers: number
  totalContainers: number
  effectiveHttpRps: number
  tpsData?: any
  isLiveWs?: boolean
  lastUpdated?: Date
  loading?: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect', node: NodeMetrics): void
  (e: 'manage', node: NodeMetrics): void
  (e: 'refresh'): void
  (e: 'deepDive'): void
}>()

const viewMode = ref<'grid' | 'table'>('grid')

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

function formatRps(val: number): string {
  if (!val || isNaN(val)) return '0'
  if (val >= 1000) return `${(val / 1000).toFixed(1)}k`
  return Math.round(val).toString()
}

function isNodeDown(node: NodeMetrics): boolean {
  return node.status === 'down' || node.status === 'offline' || node.status === 'disconnected' || node.memory_total === 0
}

function getNodeStatusDotClass(node: NodeMetrics): string {
  if (isNodeDown(node)) return 'dot-red'
  if (node.cpu_percent >= 80 || node.memory_percent >= 80 || node.status === 'degraded' || node.status === 'warning') return 'dot-amber'
  return 'dot-green'
}

function isControlPlane(node: NodeMetrics): boolean {
  const r = (node.role || '').toLowerCase()
  return r.includes('master') || r.includes('control') || r.includes('manager')
}

function getNodeRole(node: NodeMetrics): string {
  return isControlPlane(node) ? 'control-plane' : 'worker'
}

function getNodeIdentifier(node: NodeMetrics): string {
  const anyNode = node as any
  if (anyNode.ip) return anyNode.ip
  if (anyNode.ip_address) return anyNode.ip_address
  if (anyNode.internal_ip) return anyNode.internal_ip
  if (anyNode.endpoint) return anyNode.endpoint.replace(/^https?:\/\//, '').split(':')[0]
  return node.node_id || '--'
}

function getCpuColorClass(cpu?: number): string {
  const val = cpu || 0
  if (val >= 80) return 'text-rose'
  if (val >= 50) return 'text-amber'
  return 'text-emerald'
}
</script>

<template>
  <div class="mobile-overview-stream">
    <!-- Ultra-compact single-line status ticker -->
    <div class="mobile-status-ticker glass-panel font-mono text-xs">
      <span class="ticker-item"><span class="ticker-label">Nodes</span> <span class="ticker-val">{{ overview.healthy_nodes }}/{{ overview.total_nodes }}</span></span>
      <span class="ticker-dot">•</span>
      <span class="ticker-item"><span class="ticker-label">Pods</span> <span class="ticker-val">{{ runningContainers }}/{{ totalContainers }}</span></span>
      <span class="ticker-dot">•</span>
      <span class="ticker-item"><span class="ticker-label">CPU</span> <span :class="['ticker-val', (overview.total_cpu_percent || 0) >= 80 ? 'text-rose' : (overview.total_cpu_percent || 0) >= 50 ? 'text-amber' : 'text-emerald']">{{ Math.round(overview.total_cpu_percent || 0) }}%</span></span>
      <span class="ticker-dot">•</span>
      <span class="ticker-item"><span class="ticker-label">RAM</span> <span :class="['ticker-val', (overview.total_mem_percent || 0) >= 80 ? 'text-rose' : 'text-cyan']">{{ Math.round(overview.total_mem_percent || 0) }}%</span></span>
      <span class="ticker-dot">•</span>
      <span class="ticker-item"><span class="ticker-label">NET</span> <span class="ticker-val text-violet">↓{{ formatBytes(tpsData?.network?.total_rx_bytes_per_sec) }}/s ↑{{ formatBytes(tpsData?.network?.total_tx_bytes_per_sec) }}/s</span></span>
      <span class="ticker-dot">•</span>
      <span class="ticker-item"><span class="ticker-val text-cyan">{{ formatRps(effectiveHttpRps) }} rps</span></span>
    </div>

    <!-- Touch Stream of Active Nodes -->
    <div class="mobile-nodes-stream">
      <div class="stream-section-title mobile-view-toggle">
        <button type="button" class="toggle-btn" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'">🗂 Thẻ gọn</button>
        <button type="button" class="toggle-btn" :class="{ active: viewMode === 'table' }" @click="viewMode = 'table'">📑 Bảng</button>
      </div>

      <template v-if="viewMode === 'table'">
        <div class="table-responsive mobile-table-wrapper">
          <NodeTableView
            :nodes="nodes"
            @click="node => emit('inspect', node)"
            @details="node => emit('inspect', node)"
            @logs="node => emit('manage', node)"
            @scale="node => emit('manage', node)"
            @restart="node => emit('manage', node)"
            @yaml="node => emit('manage', node)"
            @delete="node => emit('manage', node)"
          />
        </div>
      </template>

      <template v-else>
        <div class="nodes-list">
          <div
            v-for="node in nodes"
            :key="node.node_id"
            class="mobile-node-chip glass-panel"
            :class="{ 'chip-down': isNodeDown(node), 'chip-hot': node.cpu_percent >= 75 }"
            @click="emit('inspect', node)"
          >
            <!-- Row 1: Status dot + Node Name + Role badge + IP address / node ID -->
            <div class="chip-row-top">
              <div class="chip-identity">
                <span class="node-status-dot" :class="getNodeStatusDotClass(node)"></span>
                <span class="node-chip-name font-semibold text-slate-100" :title="node.node_name">
                  {{ node.node_name }}
                </span>
                <span class="node-chip-role-badge" :class="isControlPlane(node) ? 'badge-cp' : 'badge-worker'">
                  {{ getNodeRole(node) }}
                </span>
              </div>
              <span class="node-chip-ip font-mono text-xs" :title="getNodeIdentifier(node)">
                {{ getNodeIdentifier(node) }}
              </span>
            </div>

            <!-- Row 2: CPU % (with color text: emerald < 50%, amber 50-80%, rose >= 80%), RAM % (cyan), Disk / Pod count -->
            <div class="chip-row-bottom font-mono text-xs">
              <span class="chip-metric" :class="getCpuColorClass(node.cpu_percent)">
                CPU {{ Math.round(node.cpu_percent || 0) }}%
              </span>
              <span class="chip-sep">·</span>
              <span class="chip-metric text-cyan">
                RAM {{ Math.round(node.memory_percent || 0) }}%
              </span>
              <span class="chip-sep">·</span>
              <span class="chip-metric text-slate-300">
                Disk {{ Math.round(node.disk_percent || 0) }}%
              </span>
              <span class="chip-sep">·</span>
              <span class="chip-metric text-slate-300">
                {{ isNodeDown(node) ? '0' : (node.running_count ?? node.container_count ?? 0) }} pods
              </span>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/overview.css';
@import '../../assets/styles/views/overview-mobile.css';
</style>
