<script setup lang="ts">
import { ref, computed } from 'vue'
import type { SystemOverview, NodeMetrics } from '../../api/overview'
import type { TrendPoint } from './hud/OverviewSaturationTrends.vue'
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
  trendHistory?: TrendPoint[]
  clusterAvgLatencyMs?: number
}

const props = defineProps<Props>()

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

function buildSmoothSpline(points: Array<{ x: number; y: number }>, smoothing = 0.18, minY = 8, maxY = 82): string {
  if (!points || points.length === 0) return ''
  if (points.length === 1) return `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`
  if (points.length === 2) return `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)} L ${points[1].x.toFixed(1)} ${points[1].y.toFixed(1)}`

  let d = `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[Math.max(0, i - 1)]
    const p1 = points[i]
    const p2 = points[i + 1]
    const p3 = points[Math.min(points.length - 1, i + 2)]

    const minLocalX = Math.min(p1.x, p2.x), maxLocalX = Math.max(p1.x, p2.x)
    const cp1x = Math.max(minLocalX, Math.min(maxLocalX, p1.x + ((p2.x - p0.x) / 6) * (1 - smoothing)))
    const cp2x = Math.max(minLocalX, Math.min(maxLocalX, p2.x - ((p3.x - p1.x) / 6) * (1 - smoothing)))
    const cp1y = Math.max(minY, Math.min(maxY, p1.y + ((p2.y - p0.y) / 6) * (1 - smoothing)))
    const cp2y = Math.max(minY, Math.min(maxY, p2.y - ((p3.y - p1.y) / 6) * (1 - smoothing)))

    d += ` C ${cp1x.toFixed(1)} ${cp1y.toFixed(1)}, ${cp2x.toFixed(1)} ${cp2y.toFixed(1)}, ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`
  }
  return d
}

const latestCpu = computed(() => {
  if (props.trendHistory?.length) return props.trendHistory[props.trendHistory.length - 1].cpu || 0
  return props.overview?.total_cpu_percent || 0
})

const latestMem = computed(() => {
  if (props.trendHistory?.length) return props.trendHistory[props.trendHistory.length - 1].mem || 0
  return props.overview?.total_mem_percent || 0
})

const cpuColor = computed(() => {
  const val = latestCpu.value
  if (val >= 80) return '#f43f5e'
  if (val >= 50) return '#f59e0b'
  return '#10b981'
})

const mobileCpuPath = computed(() => {
  const history = props.trendHistory || []
  if (history.length < 2) return ''
  const step = 338 / (history.length - 1)
  const points = history.map((h, i) => ({
    x: 6 + i * step,
    y: Math.max(8, Math.min(82, 82 - (Math.min(100, Math.max(0, h.cpu)) / 100) * 74)),
  }))
  return buildSmoothSpline(points, 0.18, 8, 82)
})

const mobileCpuArea = computed(() => {
  if (!mobileCpuPath.value || !props.trendHistory?.length) return ''
  const history = props.trendHistory
  const step = history.length > 1 ? 338 / (history.length - 1) : 0
  const lastX = 6 + (history.length - 1) * step
  return `${mobileCpuPath.value} L ${lastX.toFixed(1)} 82 L 6.0 82 Z`
})

const mobileMemPath = computed(() => {
  const history = props.trendHistory || []
  if (history.length < 2) return ''
  const step = 338 / (history.length - 1)
  const points = history.map((h, i) => ({
    x: 6 + i * step,
    y: Math.max(8, Math.min(82, 82 - (Math.min(100, Math.max(0, h.mem)) / 100) * 74)),
  }))
  return buildSmoothSpline(points, 0.18, 8, 82)
})

const mobileMemArea = computed(() => {
  if (!mobileMemPath.value || !props.trendHistory?.length) return ''
  const history = props.trendHistory
  const step = history.length > 1 ? 338 / (history.length - 1) : 0
  const lastX = 6 + (history.length - 1) * step
  return `${mobileMemPath.value} L ${lastX.toFixed(1)} 82 L 6.0 82 Z`
})
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

    <!-- Live Saturation Trend Line Chart Card (<768px Mobile) -->
    <div class="mobile-saturation-chart-card glass-panel" @click="emit('deepDive')" title="Tap to open Telemetry Deep-Dive Modal">
      <div class="chart-card-header">
        <div class="chart-header-left">
          <span class="chart-badge">📈 SATURATION</span>
          <span class="live-pill"><span class="live-pulse"></span>Live</span>
        </div>
        <div class="chart-header-legend font-mono text-xs">
          <span class="legend-item" :style="{ color: cpuColor }">CPU {{ Math.round(latestCpu) }}%</span>
          <span class="legend-sep">·</span>
          <span class="legend-item text-cyan">RAM {{ Math.round(latestMem) }}%</span>
          <span class="legend-sep">·</span>
          <span class="legend-item text-slate-300">{{ formatRps(effectiveHttpRps) }} rps</span>
          <span class="deepdive-arrow">›</span>
        </div>
      </div>
      <div class="chart-svg-wrap">
        <svg viewBox="0 0 350 90" preserveAspectRatio="none" class="mobile-trend-svg">
          <defs>
            <linearGradient id="mobileCpuGrad" x1="0%" y1="0%" x2="0%" y2="100%">
              <stop offset="0%" :stop-color="cpuColor" stop-opacity="0.25" />
              <stop offset="100%" :stop-color="cpuColor" stop-opacity="0.0" />
            </linearGradient>
            <linearGradient id="mobileMemGrad" x1="0%" y1="0%" x2="0%" y2="100%">
              <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.2" />
              <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
            </linearGradient>
          </defs>
          <!-- Grid lines -->
          <line x1="0" y1="8" x2="350" y2="8" stroke="rgba(255,255,255,0.05)" stroke-dasharray="3 3" />
          <line x1="0" y1="45" x2="350" y2="45" stroke="rgba(255,255,255,0.04)" stroke-dasharray="3 3" />
          <line x1="0" y1="82" x2="350" y2="82" stroke="rgba(255,255,255,0.08)" />

          <!-- Series Areas -->
          <path v-if="mobileMemArea" :d="mobileMemArea" fill="url(#mobileMemGrad)" />
          <path v-if="mobileCpuArea" :d="mobileCpuArea" fill="url(#mobileCpuGrad)" />

          <!-- Series Splines -->
          <path v-if="mobileMemPath" :d="mobileMemPath" fill="none" stroke="#06b6d4" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
          <path v-if="mobileCpuPath" :d="mobileCpuPath" fill="none" :stroke="cpuColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <!-- Time Axis Labels -->
        <div class="chart-time-labels font-mono text-xs">
          <span>-5m</span>
          <span>-3m</span>
          <span>-1m</span>
          <span>Now</span>
        </div>
      </div>
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
