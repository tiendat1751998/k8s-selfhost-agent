<script setup lang="ts">
import { ref, computed } from 'vue'
import type { SystemOverview, NodeMetrics } from '../../api/overview'
import type { TrendPoint } from './hud/OverviewSaturationTrends.vue'
import NodeTableView from './nodes/NodeTableView.vue'
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  overview: SystemOverview
  nodes: NodeMetrics[]
  runningContainers: number
  totalContainers: number
  effectiveHttpRps: number
  tpsData?: any; isLiveWs?: boolean; lastUpdated?: Date
  loading?: boolean; trendHistory?: TrendPoint[]; clusterAvgLatencyMs?: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect' | 'manage' | 'logs', node: NodeMetrics): void
  (e: 'refresh' | 'deepDive'): void
}>()

const viewMode = ref<'grid' | 'table'>('grid')

import {
  formatRps,
  isNodeDown,
  isControlPlane,
  getNodeStatusDotClass,
  getNodeRole,
  getNodeIdentifier,
  getCpuColorClass,
} from './overviewMobileUtils'
import { buildSmoothSpline } from './hud/trendMath'


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
    <!-- Live Saturation Trend Line Chart Card (<768px Mobile) -->
    <div class="mobile-saturation-chart-card glass-panel" @click="emit('deepDive')" title="Tap to open Telemetry Deep-Dive Modal">
      <div class="chart-card-header">
        <div class="chart-header-left">
          <span class="chart-badge">
            <BaseIcon name="trending-up" size="xs" />
            <span>SATURATION</span>
          </span>
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
        <button type="button" class="toggle-btn" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'">
          <BaseIcon name="layers" size="xs" />
          <span>Compact</span>
        </button>
        <button type="button" class="toggle-btn" :class="{ active: viewMode === 'table' }" @click="viewMode = 'table'">
          <BaseIcon name="file-text" size="xs" />
          <span>Table</span>
        </button>
      </div>

      <template v-if="viewMode === 'table'">
        <div class="table-responsive mobile-table-wrapper">
          <NodeTableView
            :nodes="nodes"
            @click="node => emit('inspect', node)"
            @details="node => emit('inspect', node)"
            @logs="node => emit('logs', node)"
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
.chart-badge, .mobile-view-toggle .toggle-btn { display: inline-flex; align-items: center; gap: 4px; }
</style>
