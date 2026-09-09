<script setup lang="ts">
import type { SystemOverview, NodeMetrics } from '../../../api/overview'

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
        <span class="hud-label">
          <span class="label-full">Nodes Online</span>
          <span class="label-mobile">Nodes</span>
        </span>
        <span
          class="status-indicator-dot"
          :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'status-green' : 'status-amber'"
        ></span>
      </div>
      <div class="hud-value-row">
        <span
          class="hud-value smooth-value"
          :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'text-emerald' : overview.healthy_nodes > 0 ? 'text-amber' : 'text-rose'"
        >
          {{ overview.healthy_nodes }}<span class="hud-total">/{{ overview.total_nodes }}</span>
        </span>
        <span
          class="badge smooth-value"
          :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'badge-emerald' : 'badge-amber'"
        >
          {{ overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? '100% HEALTHY' : 'DEGRADED' }}
        </span>
      </div>
      <div class="hud-progress-track">
        <div
          class="hud-progress-fill smooth-bar"
          :class="overview.healthy_nodes === overview.total_nodes && overview.total_nodes > 0 ? 'bg-emerald' : overview.healthy_nodes > 0 ? 'bg-amber' : 'bg-rose'"
          :style="{ width: `${overview.total_nodes ? (overview.healthy_nodes / overview.total_nodes) * 100 : 0}%` }"
        ></div>
      </div>
      <div class="hud-card-footer-text font-mono">
        <span :class="overview.healthy_nodes === overview.total_nodes ? 'text-emerald' : 'text-amber'">
          <span class="footer-full">🖥️ {{ overview.healthy_nodes }} Online · {{ (overview.total_nodes || 0) - (overview.healthy_nodes || 0) }} Offline</span>
          <span class="footer-mobile">{{ overview.healthy_nodes }} up · {{ (overview.total_nodes || 0) - (overview.healthy_nodes || 0) }} down</span>
        </span>
      </div>
    </div>

    <!-- Card 2: Containers -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <span class="hud-label">
          <span class="label-full">Containers</span>
          <span class="label-mobile">Containers</span>
        </span>
        <span class="hud-icon">📦</span>
      </div>
      <div class="hud-value-row">
        <span
          class="hud-value smooth-value"
          :class="runningContainers === totalContainers && totalContainers > 0 ? 'text-emerald' : runningContainers > 0 ? 'text-cyan' : 'text-rose'"
        >
          {{ runningContainers }}<span class="hud-total">/{{ totalContainers }}</span>
        </span>
        <span class="badge badge-cyan smooth-value">
          {{ runningContainers > 0 ? `${runningContainers} RUNNING` : 'STOPPED' }}
        </span>
      </div>
      <div class="hud-progress-track">
        <div
          class="hud-progress-fill bg-cyan smooth-bar"
          :style="{ width: `${totalContainers ? (runningContainers / totalContainers) * 100 : 0}%` }"
        ></div>
      </div>
      <div class="hud-card-footer-text font-mono">
        <span class="text-cyan">
          <span class="footer-full">🚀 Across {{ overview.healthy_nodes || 0 }} Active Nodes</span>
          <span class="footer-mobile">{{ overview.healthy_nodes || 0 }} active node{{ (overview.healthy_nodes || 0) === 1 ? '' : 's' }}</span>
        </span>
      </div>
    </div>

    <!-- Card 3: Avg CPU -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <span class="hud-label">
          <span class="label-full">Avg CPU Saturation</span>
          <span class="label-mobile">CPU</span>
        </span>
        <span class="hud-icon">⚡</span>
      </div>
      <div class="hud-value-row">
        <span class="hud-value smooth-value" :class="`text-${getUtilizationColor(overview.total_cpu_percent)}`">
          {{ Math.round(overview.total_cpu_percent) }}%
        </span>
        <span class="badge smooth-value" :class="`badge-${getUtilizationColor(overview.total_cpu_percent)}`">
          {{ overview.total_cpu_percent >= 80 ? 'CRITICAL' : overview.total_cpu_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
        </span>
      </div>
      <div class="hud-progress-track">
        <div
          class="hud-progress-fill smooth-bar"
          :class="`bg-${getUtilizationColor(overview.total_cpu_percent)}`"
          :style="{ width: `${Math.min(100, overview.total_cpu_percent)}%` }"
        ></div>
      </div>
      <div class="hud-card-footer-text font-mono">
        <span class="text-violet">
          <span class="footer-full">🔥 Peak: {{ peakCpuNode?.node_name || 'k8smater' }} ({{ Math.round(peakCpuNode?.cpu_percent || overview.total_cpu_percent) }}%)</span>
          <span class="footer-mobile">🔥 {{ peakCpuNode?.node_name || 'k8smater' }} {{ Math.round(peakCpuNode?.cpu_percent || overview.total_cpu_percent) }}%</span>
        </span>
      </div>
    </div>

    <!-- Card 4: Avg RAM -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <span class="hud-label">
          <span class="label-full">Avg Memory Saturation</span>
          <span class="label-mobile">Memory</span>
        </span>
        <span class="hud-icon">🧠</span>
      </div>
      <div class="hud-value-row">
        <span class="hud-value smooth-value" :class="`text-${getUtilizationColor(overview.total_mem_percent)}`">
          {{ Math.round(overview.total_mem_percent) }}%
        </span>
        <span class="badge smooth-value" :class="`badge-${getUtilizationColor(overview.total_mem_percent)}`">
          {{ overview.total_mem_percent >= 80 ? 'CRITICAL' : overview.total_mem_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
        </span>
      </div>
      <div class="hud-progress-track">
        <div
          class="hud-progress-fill smooth-bar"
          :class="`bg-${getUtilizationColor(overview.total_mem_percent)}`"
          :style="{ width: `${Math.min(100, overview.total_mem_percent)}%` }"
        ></div>
      </div>
      <div class="hud-card-footer-text font-mono">
        <span class="text-cyan">
          <span class="footer-full">📊 {{ formatBytes(clusterUsedMemBytes) }} / {{ formatBytes(clusterTotalMemBytes) }}</span>
          <span class="footer-mobile">{{ formatBytes(clusterUsedMemBytes) }} / {{ formatBytes(clusterTotalMemBytes) }}</span>
        </span>
      </div>
    </div>

    <!-- Card 5: Cluster Storage -->
    <div class="hud-card glass-panel">
      <div class="hud-card-top">
        <span class="hud-label">
          <span class="label-full">Cluster Storage</span>
          <span class="label-mobile">Storage</span>
        </span>
        <span class="hud-icon">💾</span>
      </div>
      <div class="hud-value-row">
        <span class="hud-value smooth-value" :class="`text-${getUtilizationColor(overview.total_disk_percent)}`">
          {{ Math.round(overview.total_disk_percent) }}%
        </span>
        <span class="badge smooth-value" :class="`badge-${getUtilizationColor(overview.total_disk_percent)}`">
          {{ overview.total_disk_percent >= 80 ? 'CRITICAL' : overview.total_disk_percent >= 60 ? 'ELEVATED' : 'NOMINAL' }}
        </span>
      </div>
      <div class="hud-progress-track">
        <div
          class="hud-progress-fill smooth-bar"
          :class="`bg-${getUtilizationColor(overview.total_disk_percent)}`"
          :style="{ width: `${Math.min(100, overview.total_disk_percent)}%` }"
        ></div>
      </div>
      <div class="hud-card-footer-text font-mono">
        <span class="text-emerald">
          <span class="footer-full">💽 {{ formatBytes(clusterUsedDiskBytes) }} / {{ formatBytes(clusterTotalDiskBytes) }}</span>
          <span class="footer-mobile">{{ formatBytes(clusterUsedDiskBytes) }} / {{ formatBytes(clusterTotalDiskBytes) }}</span>
        </span>
      </div>
    </div>
  </section>
</template>

<style scoped>
@import '../../../assets/styles/views/overview.css';
</style>
