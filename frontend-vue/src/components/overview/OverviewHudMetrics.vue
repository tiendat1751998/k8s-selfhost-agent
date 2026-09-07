<script setup lang="ts">
import type { SystemOverview, NodeMetrics } from '../../api/overview'

interface Props {
  overview: SystemOverview
  runningContainers: number
  totalContainers: number
  peakCpuNode?: NodeMetrics | null
  clusterUsedMemBytes: number
  clusterTotalMemBytes: number
  clusterUsedDiskBytes: number
  clusterTotalDiskBytes: number
}

defineProps<Props>()

function getUtilizationColor(pct: number): string {
  if (pct >= 80) return 'rose'
  if (pct >= 60) return 'amber'
  return 'emerald'
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}
</script>

<template>
  <section class="summary-hud-row">
    <!-- Card 1: Nodes -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <span class="hud-label">Nodes Online</span>
        <span
          class="status-indicator-dot"
          :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'status-green' : 'status-amber'"
        ></span>
      </div>
      <div class="hud-value-row">
        <span
          class="hud-value"
          :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'text-emerald' : overview.healthy_nodes > 0 ? 'text-amber' : 'text-rose'"
        >
          {{ overview.healthy_nodes }}<span class="hud-total">/{{ overview.total_nodes }}</span>
        </span>
        <span
          class="badge"
          :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'badge-emerald' : 'badge-amber'"
        >
          {{ overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? '100% HEALTHY' : 'DEGRADED' }}
        </span>
      </div>
      <div class="hud-progress-track">
        <div
          class="hud-progress-fill"
          :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'bg-emerald' : overview.healthy_nodes > 0 ? 'bg-amber' : 'bg-rose'"
          :style="{ width: `${overview.total_nodes ? (overview.healthy_nodes / overview.total_nodes) * 100 : 0}%` }"
        ></div>
      </div>
      <div class="hud-card-footer font-mono">
        <span :class="overview.healthy_nodes === overview.total_nodes ? 'text-emerald' : 'text-amber'">
          🖥️ {{ overview.healthy_nodes }} Online · {{ (overview.total_nodes || 0) - (overview.healthy_nodes || 0) }} Offline
        </span>
      </div>
    </div>

    <!-- Card 2: Containers -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <span class="hud-label">Containers</span>
        <span class="hud-icon">📦</span>
      </div>
      <div class="hud-value-row">
        <span
          class="hud-value"
          :class="runningContainers === totalContainers && totalContainers > 0 ? 'text-emerald' : runningContainers > 0 ? 'text-cyan' : 'text-rose'"
        >
          {{ runningContainers }}<span class="hud-total">/{{ totalContainers }}</span>
        </span>
        <span class="badge badge-cyan">
          {{ runningContainers > 0 ? `${runningContainers} RUNNING` : 'STOPPED' }}
        </span>
      </div>
      <div class="hud-progress-track">
        <div
          class="hud-progress-fill bg-cyan"
          :style="{ width: `${totalContainers ? (runningContainers / totalContainers) * 100 : 0}%` }"
        ></div>
      </div>
      <div class="hud-card-footer font-mono">
        <span class="text-cyan">🚀 Active App Workloads</span>
      </div>
    </div>

    <!-- Card 3: Total CPU -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <span class="hud-label">CPU Saturation</span>
        <span class="hud-icon">⚡</span>
      </div>
      <div class="hud-value-row">
        <span class="hud-value" :class="`text-${getUtilizationColor(overview.total_cpu_percent || 0)}`">
          {{ Math.round(overview.total_cpu_percent || 0) }}<span class="hud-unit">%</span>
        </span>
        <span class="badge" :class="`badge-${getUtilizationColor(overview.total_cpu_percent || 0)}`">
          {{ (overview.total_cpu_percent || 0) >= 80 ? 'CRITICAL' : (overview.total_cpu_percent || 0) >= 60 ? 'ELEVATED' : 'NOMINAL' }}
        </span>
      </div>
      <div class="hud-progress-track">
        <div
          class="hud-progress-fill"
          :class="`bg-${getUtilizationColor(overview.total_cpu_percent || 0)}`"
          :style="{ width: `${Math.min(100, overview.total_cpu_percent || 0)}%` }"
        ></div>
      </div>
      <div class="hud-card-footer font-mono">
        <span v-if="peakCpuNode" class="text-muted">
          Peak: <span class="text-white">{{ peakCpuNode.node_name }}</span> ({{ Math.round(peakCpuNode.cpu_percent) }}%)
        </span>
        <span v-else class="text-muted">Total cluster cores</span>
      </div>
    </div>

    <!-- Card 4: RAM / Storage -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <span class="hud-label">Memory &amp; Storage</span>
        <span class="hud-icon">🧠</span>
      </div>
      <div class="hud-value-row">
        <span class="hud-value" :class="`text-${getUtilizationColor(overview.total_mem_percent || 0)}`">
          {{ Math.round(overview.total_mem_percent || 0) }}<span class="hud-unit">%</span>
        </span>
        <span class="badge" :class="`badge-${getUtilizationColor(overview.total_mem_percent || 0)}`">
          RAM {{ formatBytes(clusterUsedMemBytes) }}
        </span>
      </div>
      <div class="hud-progress-track">
        <div
          class="hud-progress-fill"
          :class="`bg-${getUtilizationColor(overview.total_mem_percent || 0)}`"
          :style="{ width: `${Math.min(100, overview.total_mem_percent || 0)}%` }"
        ></div>
      </div>
      <div class="hud-card-footer font-mono">
        <span class="text-muted">
          Disk: <span :class="`text-${getUtilizationColor(overview.total_disk_percent || 0)}`">{{ Math.round(overview.total_disk_percent || 0) }}%</span> ({{ formatBytes(clusterUsedDiskBytes) }} / {{ formatBytes(clusterTotalDiskBytes) }})
        </span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.summary-hud-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.hud-card {
  padding: 18px 20px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: transform 0.2s ease, border-color 0.2s ease;
}

.hud-card:hover {
  transform: translateY(-2px);
  border-color: rgba(56, 189, 248, 0.3);
}

.hud-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hud-label {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary, #94a3b8);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.hud-value-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.hud-value {
  font-size: 28px;
  font-weight: 800;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.hud-total {
  font-size: 18px;
  color: var(--text-muted, #64748b);
  font-weight: 600;
}

.hud-unit {
  font-size: 18px;
  font-weight: 600;
}

.hud-progress-track {
  width: 100%;
  height: 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.06);
  overflow: hidden;
}

.hud-progress-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.4s ease;
}

.hud-card-footer {
  font-size: 11.5px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.status-indicator-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-green {
  background: #10b981;
  box-shadow: 0 0 8px #10b981;
}

.status-amber {
  background: #f59e0b;
  box-shadow: 0 0 8px #f59e0b;
}

.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 7px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  font-family: var(--font-mono, monospace);
}

.badge-emerald { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.badge-cyan { background: rgba(6, 182, 212, 0.15); color: #22d3ee; border: 1px solid rgba(6, 182, 212, 0.3); }
.badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.badge-rose { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }

.bg-emerald { background-color: #10b981; }
.bg-cyan { background-color: #06b6d4; }
.bg-amber { background-color: #f59e0b; }
.bg-rose { background-color: #f43f5e; }

.text-emerald { color: #10b981; }
.text-cyan { color: #06b6d4; }
.text-amber { color: #f59e0b; }
.text-rose { color: #f43f5e; }
.text-white { color: #f8fafc; }
.text-muted { color: #64748b; }

@media (max-width: 640px) {
  .summary-hud-row { grid-template-columns: repeat(2, 1fr); gap: 10px; }
  .hud-card { padding: 12px; gap: 6px; }
  .hud-value { font-size: 20px; }
  .hud-label { font-size: 11px; }
  .hud-card-footer { font-size: 10px; }
}
</style>
