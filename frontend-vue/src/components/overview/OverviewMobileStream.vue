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

const viewMode = ref<'grid' | 'table'>('table')

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
</script>

<template>
  <div class="mobile-overview-stream">
    <!-- Ultra-compact single-line status ticker -->
    <div class="mobile-status-ticker glass-panel font-mono text-xs">
      <span class="ticker-item"><span class="ticker-label">Nodes</span> <span class="ticker-val">{{ overview.healthy_nodes }}/{{ overview.total_nodes }}</span></span>
      <span class="ticker-dot">•</span>
      <span class="ticker-item"><span class="ticker-label">Pods</span> <span class="ticker-val">{{ runningContainers }}/{{ totalContainers }}</span></span>
      <span class="ticker-dot">•</span>
      <span class="ticker-item"><span class="ticker-label">CPU</span> <span :class="['ticker-val', (overview.total_cpu_percent || 0) >= 80 ? 'text-rose' : 'text-emerald']">{{ Math.round(overview.total_cpu_percent || 0) }}%</span></span>
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
        <button class="toggle-btn" :class="{ active: viewMode === 'table' }" @click="viewMode = 'table'">📑 Bảng thông số</button>
        <button class="toggle-btn" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'">🗂 Thẻ</button>
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
          :class="{ 'chip-down': node.status === 'down' || node.status === 'offline', 'chip-hot': node.cpu_percent >= 75 }"
          @click="emit('inspect', node)"
        >
          <div class="node-chip-left">
            <span
              class="node-status-dot"
              :class="node.status === 'down' || node.status === 'offline' ? 'dot-red' : node.cpu_percent >= 75 ? 'dot-amber' : 'dot-green'"
            ></span>
            <div class="node-chip-info">
              <span class="node-chip-name font-bold">{{ node.node_name }}</span>
              <span class="node-chip-role font-mono">{{ node.role || 'worker' }}</span>
            </div>
          </div>
          <div class="node-chip-metrics font-mono">
            <span class="chip-m-val" :class="node.cpu_percent >= 80 ? 'text-rose' : node.cpu_percent >= 50 ? 'text-amber' : 'text-emerald'">
              CPU {{ Math.round(node.cpu_percent) }}%
            </span>
            <span class="chip-m-val text-cyan">
              RAM {{ Math.round(node.memory_percent) }}%
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
</style>
