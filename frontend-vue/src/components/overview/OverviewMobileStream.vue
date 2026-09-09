<script setup lang="ts">
import type { SystemOverview, NodeMetrics } from '../../api/overview'

interface Props {
  overview: SystemOverview
  nodes: NodeMetrics[]
  runningContainers: number
  totalContainers: number
  effectiveHttpRps: number
  isLiveWs?: boolean
  lastUpdated?: Date
  loading?: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect', node: NodeMetrics): void
  (e: 'refresh'): void
  (e: 'deepDive'): void
}>()

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
      <span class="ticker-item"><span class="ticker-val text-cyan">{{ formatRps(effectiveHttpRps) }} rps</span></span>
    </div>

    <!-- Touch Stream of Active Nodes -->
    <div class="mobile-nodes-stream">
      <div class="stream-section-title">
        <span>Active Infrastructure Nodes ({{ nodes.length }})</span>
      </div>
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
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/overview.css';
</style>
