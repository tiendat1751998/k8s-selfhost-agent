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
@import '../../assets/styles/views/overview.css';
</style>
