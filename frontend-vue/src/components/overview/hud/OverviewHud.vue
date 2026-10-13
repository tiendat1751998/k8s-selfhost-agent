<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { SystemOverview, NodeMetrics } from '../../../api/overview'
import type { TrendPoint } from '../../../composables/useOverviewDashboard'
import BaseIcon from '../../ui/BaseIcon.vue'
import PercentageBar from '../../ui/PercentageBar.vue'
import MetricSparkline from '../../ui/MetricSparkline.vue'

interface Props {
  overview: SystemOverview
  runningContainers: number
  totalContainers: number
  peakCpuNode?: NodeMetrics | null
  clusterUsedMemBytes: number
  clusterTotalMemBytes: number
  clusterUsedDiskBytes: number
  clusterTotalDiskBytes: number
  trendHistory?: TrendPoint[]
}

const props = defineProps<Props>()

const nodesPct = computed(() => {
  if (!props.overview.total_nodes) return 0
  return (props.overview.healthy_nodes / props.overview.total_nodes) * 100
})

const containersPct = computed(() => {
  if (!props.totalContainers) return 0
  return (props.runningContainers / props.totalContainers) * 100
})

const nodesHistory = ref<number[]>([])
const containersHistory = ref<number[]>([])
const cpuHistory = ref<number[]>([])
const memHistory = ref<number[]>([])
const diskHistory = ref<number[]>([])

watch(
  () => [
    nodesPct.value,
    containersPct.value,
    props.overview?.total_cpu_percent,
    props.overview?.total_mem_percent,
    props.overview?.total_disk_percent,
  ],
  ([nPct, cPct, cpu, mem, disk]) => {
    nodesHistory.value = [...nodesHistory.value.slice(-29), nPct ?? 0]
    containersHistory.value = [...containersHistory.value.slice(-29), cPct ?? 0]
    cpuHistory.value = [...cpuHistory.value.slice(-29), cpu ?? 0]
    memHistory.value = [...memHistory.value.slice(-29), mem ?? 0]
    diskHistory.value = [...diskHistory.value.slice(-29), disk ?? 0]
  },
  { immediate: true }
)

const nodesPoints = computed<number[]>(() =>
  nodesHistory.value.length ? nodesHistory.value : [nodesPct.value]
)
const containersPoints = computed<number[]>(() =>
  containersHistory.value.length ? containersHistory.value : [containersPct.value]
)
const cpuPoints = computed<number[]>(() =>
  props.trendHistory?.length ? props.trendHistory.map((p) => p.cpu) : (cpuHistory.value.length ? cpuHistory.value : [props.overview?.total_cpu_percent || 0])
)
const memPoints = computed<number[]>(() =>
  props.trendHistory?.length ? props.trendHistory.map((p) => p.mem) : (memHistory.value.length ? memHistory.value : [props.overview?.total_mem_percent || 0])
)
const diskPoints = computed<number[]>(() =>
  props.trendHistory?.length ? props.trendHistory.map((p) => p.disk) : (diskHistory.value.length ? diskHistory.value : [props.overview?.total_disk_percent || 0])
)

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const clampedI = Math.min(sizes.length - 1, Math.max(0, i))
  return `${parseFloat((bytes / Math.pow(k, clampedI)).toFixed(1))} ${sizes[clampedI]}`
}

function getRiskColor(pct: number): 'rose' | 'amber' | 'emerald' {
  return pct >= 85 ? 'rose' : pct >= 70 ? 'amber' : 'emerald'
}

function getRiskStroke(pct: number): string {
  return pct >= 85 ? '#f43f5e' : pct >= 70 ? '#f59e0b' : '#10b981'
}
</script>

<template>
  <section class="summary-hud-row metrics-grid" aria-label="System Overview HUD">
    <!-- Card 1: Nodes Online -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <div class="hud-label-group">
          <BaseIcon name="server" size="sm" />
          <span class="hud-label">Nodes Online</span>
        </div>
        <span class="status-indicator-dot" :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'status-green' : 'status-amber'" />
      </div>
      <div class="hud-value-row">
        <span class="hud-value font-mono" :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'text-emerald' : overview.healthy_nodes > 0 ? 'text-amber' : 'text-rose'">
          {{ overview.healthy_nodes }}<span class="hud-total font-mono">/{{ overview.total_nodes }}</span>
        </span>
        <span class="hud-badge font-mono" :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'badge-emerald' : 'badge-amber'">
          {{ overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? '100% OK' : 'DEGRADED' }}
        </span>
      </div>
      <div class="hud-card-bottom">
        <PercentageBar :value="nodesPct" :height="3" variant="emerald" class="hud-bar" />
        <MetricSparkline :data="nodesPoints" color="#10b981" />
      </div>
    </div>

    <!-- Card 2: Containers -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <div class="hud-label-group">
          <BaseIcon name="box" size="sm" />
          <span class="hud-label">Containers</span>
        </div>
        <span class="status-indicator-dot status-cyan" />
      </div>
      <div class="hud-value-row">
        <span class="hud-value font-mono" :class="runningContainers === totalContainers && totalContainers > 0 ? 'text-emerald' : runningContainers > 0 ? 'text-cyan' : 'text-rose'">
          {{ runningContainers }}<span class="hud-total font-mono">/{{ totalContainers }}</span>
        </span>
        <span class="hud-badge badge-cyan font-mono">{{ runningContainers > 0 ? 'RUNNING' : 'STOPPED' }}</span>
      </div>
      <div class="hud-card-bottom">
        <PercentageBar :value="containersPct" :height="3" variant="cyan" class="hud-bar" />
        <MetricSparkline :data="containersPoints" color="#06b6d4" />
      </div>
    </div>

    <!-- Card 3: Avg CPU -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <div class="hud-label-group">
          <BaseIcon name="zap" size="sm" />
          <span class="hud-label">Avg CPU Saturation</span>
        </div>
        <span class="status-indicator-dot" :class="overview.total_cpu_percent >= 85 ? 'status-rose' : overview.total_cpu_percent >= 70 ? 'status-amber' : 'status-green'" />
      </div>
      <div class="hud-value-row">
        <span class="hud-value font-mono" :class="`text-${getRiskColor(overview.total_cpu_percent)}`">{{ Math.round(overview.total_cpu_percent) }}%</span>
        <span class="hud-badge font-mono" :class="`badge-${getRiskColor(overview.total_cpu_percent)}`">
          {{ peakCpuNode ? peakCpuNode.node_name : (overview.total_cpu_percent >= 85 ? 'CRITICAL' : overview.total_cpu_percent >= 70 ? 'ELEVATED' : 'NOMINAL') }}
        </span>
      </div>
      <div class="hud-card-bottom">
        <PercentageBar :value="overview.total_cpu_percent" :height="3" class="hud-bar" />
        <MetricSparkline :data="cpuPoints" :color="getRiskStroke(overview.total_cpu_percent)" />
      </div>
    </div>

    <!-- Card 4: Avg Memory -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <div class="hud-label-group">
          <BaseIcon name="cpu" size="sm" />
          <span class="hud-label">Avg Memory</span>
        </div>
        <span class="status-indicator-dot" :class="overview.total_mem_percent >= 85 ? 'status-rose' : overview.total_mem_percent >= 70 ? 'status-amber' : 'status-green'" />
      </div>
      <div class="hud-value-row">
        <span class="hud-value font-mono" :class="`text-${getRiskColor(overview.total_mem_percent)}`">{{ Math.round(overview.total_mem_percent) }}%</span>
        <span class="hud-badge font-mono" :class="`badge-${getRiskColor(overview.total_mem_percent)}`">
          {{ formatBytes(clusterUsedMemBytes) }} / {{ formatBytes(clusterTotalMemBytes) }}
        </span>
      </div>
      <div class="hud-card-bottom">
        <PercentageBar :value="overview.total_mem_percent" :height="3" class="hud-bar" />
        <MetricSparkline :data="memPoints" :color="getRiskStroke(overview.total_mem_percent)" />
      </div>
    </div>

    <!-- Card 5: Cluster Storage -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <div class="hud-label-group">
          <BaseIcon name="hard-drive" size="sm" />
          <span class="hud-label">Cluster Storage</span>
        </div>
        <span class="status-indicator-dot" :class="overview.total_disk_percent >= 85 ? 'status-rose' : overview.total_disk_percent >= 70 ? 'status-amber' : 'status-green'" />
      </div>
      <div class="hud-value-row">
        <span class="hud-value font-mono" :class="`text-${getRiskColor(overview.total_disk_percent)}`">{{ Math.round(overview.total_disk_percent) }}%</span>
        <span class="hud-badge font-mono" :class="`badge-${getRiskColor(overview.total_disk_percent)}`">
          {{ formatBytes(clusterUsedDiskBytes) }} / {{ formatBytes(clusterTotalDiskBytes) }}
        </span>
      </div>
      <div class="hud-card-bottom">
        <PercentageBar :value="overview.total_disk_percent" :height="3" class="hud-bar" />
        <MetricSparkline :data="diskPoints" :color="getRiskStroke(overview.total_disk_percent)" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.metrics-grid,
.summary-hud-row {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
  width: 100%;
}

.hud-card {
  height: 88px;
  max-height: 92px;
  box-sizing: border-box;
  padding: 8px 12px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
  overflow: hidden;
}

.hud-card:hover {
  border-color: rgba(56, 189, 248, 0.28);
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.25), 0 0 15px rgba(56, 189, 248, 0.1);
}

.hud-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 16px;
}

.hud-label-group {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-secondary, #94a3b8);
}

.hud-label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-indicator-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-green { background: #10b981; box-shadow: 0 0 6px #10b981; }
.status-cyan { background: #06b6d4; box-shadow: 0 0 6px #06b6d4; }
.status-amber { background: #f59e0b; box-shadow: 0 0 6px #f59e0b; }
.status-rose { background: #f43f5e; box-shadow: 0 0 6px #f43f5e; }

.hud-value-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 6px;
  height: 26px;
}

.hud-value {
  font-size: 24px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  line-height: 1;
}

.hud-total {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted, #64748b);
}

.hud-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  white-space: nowrap;
  max-width: 58%;
  overflow: hidden;
  text-overflow: ellipsis;
}

.badge-emerald { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.badge-rose { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }
.badge-cyan { background: rgba(6, 182, 212, 0.15); color: #22d3ee; border: 1px solid rgba(6, 182, 212, 0.3); }

.hud-card-bottom { display: flex; align-items: center; gap: 8px; height: 12px; }
.hud-bar { flex: 1; min-width: 0; }
.font-mono { font-family: var(--font-mono, monospace); }

.text-emerald { color: #10b981; }
.text-amber { color: #f59e0b; }
.text-rose { color: #f43f5e; }
.text-cyan { color: #06b6d4; }

/* Tablet (768px - 1023px): Symmetric 3+2 layout */
@media (min-width: 768px) and (max-width: 1023px) {
  .metrics-grid,
  .summary-hud-row {
    grid-template-columns: repeat(6, minmax(0, 1fr));
    gap: 10px;
  }
  .hud-card:nth-child(1),
  .hud-card:nth-child(2),
  .hud-card:nth-child(3) { grid-column: span 2; }
  .hud-card:nth-child(4),
  .hud-card:nth-child(5) { grid-column: span 3; }
}

/* Mobile (< 768px): Responsive auto-fit cards */
@media (max-width: 767px) {
  .metrics-grid,
  .summary-hud-row {
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 8px;
  }
  .hud-card { height: 84px; max-height: 88px; padding: 8px 10px; }
  .hud-value { font-size: 20px; }
  .hud-badge { display: none; }
}
</style>
