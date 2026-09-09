<script setup lang="ts">
import { useRouter } from 'vue-router'
import type { SystemOverview, NodeMetrics } from '../../api/overview'

interface Props {
  overview: SystemOverview
  nodes: NodeMetrics[]
  runningContainers: number
  totalContainers: number
  effectiveHttpRps: number
  isLiveWs: boolean
  lastUpdated: Date
  loading: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect', node: NodeMetrics): void
  (e: 'refresh'): void
  (e: 'deepDive'): void
}>()

const router = useRouter()

function formatRps(val: number): string {
  if (!val || isNaN(val)) return '0'
  if (val >= 1000) return `${(val / 1000).toFixed(1)}k`
  return Math.round(val).toString()
}
</script>

<template>
  <div class="mobile-overview-stream">
    <!-- 1. Mobile Quick Status Header -->
    <div class="mobile-stream-header glass-panel">
      <div class="mobile-sync-info">
        <span class="badge" :class="isLiveWs ? 'badge-emerald' : 'badge-cyan'">
          <span class="pulse-dot" :class="{ 'pulse-active': isLiveWs }"></span>
          <span>{{ isLiveWs ? 'LIVE WS' : 'SYNC' }}</span>
        </span>
        <span class="last-sync-text">{{ lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) }}</span>
      </div>
      <div class="mobile-quick-actions">
        <button class="btn-m-action" @click="emit('refresh')" :disabled="loading">
          <span :class="{ 'spin-icon': loading }">🔄</span>
        </button>
        <button class="btn-m-action" @click="router.push('/deployments')">🚀 Apps</button>
        <button class="btn-m-action" @click="router.push('/hosts')">🖥️ Hosts</button>
        <button class="btn-m-action" @click="router.push('/fleet')">☸️ Fleet</button>
      </div>
    </div>

    <!-- 2. Compact Touch Telemetry Grid -->
    <div class="mobile-kpi-grid">
      <div class="mobile-kpi-card glass-panel">
        <span class="m-kpi-lbl">Nodes</span>
        <span class="m-kpi-val" :class="overview.healthy_nodes === overview.total_nodes ? 'text-emerald' : 'text-amber'">
          {{ overview.healthy_nodes }}/{{ overview.total_nodes }}
        </span>
        <span class="m-kpi-sub">{{ overview.healthy_nodes === overview.total_nodes ? 'Healthy' : 'Degraded' }}</span>
      </div>

      <div class="mobile-kpi-card glass-panel">
        <span class="m-kpi-lbl">Containers</span>
        <span class="m-kpi-val text-cyan">{{ runningContainers }}/{{ totalContainers }}</span>
        <span class="m-kpi-sub">Active Pods</span>
      </div>

      <div class="mobile-kpi-card glass-panel">
        <span class="m-kpi-lbl">Total CPU</span>
        <span class="m-kpi-val" :class="(overview.total_cpu_percent || 0) >= 80 ? 'text-rose' : (overview.total_cpu_percent || 0) >= 50 ? 'text-amber' : 'text-emerald'">
          {{ Math.round(overview.total_cpu_percent || 0) }}%
        </span>
        <span class="m-kpi-sub">Saturation</span>
      </div>

      <div class="mobile-kpi-card glass-panel" @click="emit('deepDive')">
        <span class="m-kpi-lbl">Throughput</span>
        <span class="m-kpi-val text-emerald">{{ formatRps(effectiveHttpRps) }} <span class="m-unit">rps</span></span>
        <span class="m-kpi-sub text-cyan">🔍 Deep-Dive</span>
      </div>
    </div>

    <!-- 3. Touch Stream of Active Nodes -->
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
