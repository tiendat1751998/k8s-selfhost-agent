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
.summary-hud-row {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
  width: 100%;
}

.summary-hud-row .hud-card {
  min-width: 0;
  min-height: 124px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 10px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
  transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              border-color 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              box-shadow 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.summary-hud-row .hud-card:hover {
  border-color: rgba(56, 189, 248, 0.28);
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.25), 0 0 15px rgba(56, 189, 248, 0.1);
}

.hud-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hud-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-secondary, #94a3b8);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.label-full {
  display: inline;
}

.label-mobile {
  display: none;
}

.footer-full {
  display: inline;
}

.footer-mobile {
  display: none;
}

.hud-icon {
  font-size: 14px;
  opacity: 0.8;
}

.status-indicator-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  box-shadow: 0 0 8px currentColor;
}

.status-green {
  background: #10b981;
  color: #10b981;
}

.status-amber {
  background: #f59e0b;
  color: #f59e0b;
}

.hud-value-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.hud-value {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1;
}

.hud-total {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-muted, #64748b);
}

.hud-progress-track {
  width: 100%;
  height: 4px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.hud-progress-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
}

.hud-card-footer-text {
  font-size: 11px;
  line-height: 1.3;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.font-mono {
  font-family: var(--font-mono, monospace);
}

.smooth-value {
  transition: all 0.3s ease;
}

.smooth-bar {
  transition: width 0.5s ease;
}

.text-emerald { color: #10b981; }
.text-amber { color: #f59e0b; }
.text-rose { color: #f43f5e; }
.text-cyan { color: #06b6d4; }
.text-violet { color: #8b5cf6; }

.bg-emerald { background-color: #10b981; }
.bg-amber { background-color: #f59e0b; }
.bg-rose { background-color: #f43f5e; }
.bg-cyan { background-color: #06b6d4; }
.bg-violet { background-color: #8b5cf6; }

.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.badge-emerald { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.badge-rose { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }
.badge-cyan { background: rgba(6, 182, 212, 0.15); color: #22d3ee; border: 1px solid rgba(6, 182, 212, 0.3); }
.badge-violet { background: rgba(139, 92, 246, 0.15); color: #a78bfa; border: 1px solid rgba(139, 92, 246, 0.3); }

@media (max-width: 1200px) {
  .summary-hud-row {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }
}

@media (max-width: 768px) {
  .summary-hud-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }
}

@media (max-width: 640px) {
  .label-full {
    display: none;
  }

  .label-mobile {
    display: inline;
  }

  .footer-full {
    display: none;
  }

  .footer-mobile {
    display: inline;
  }

  .hud-card:nth-child(3) .badge,
  .hud-card:nth-child(4) .badge,
  .hud-card:nth-child(5) .badge {
    display: none;
  }

  .hud-label {
    font-size: 9.5px;
    letter-spacing: 0.02em;
    font-weight: 700;
  }
  .hud-value {
    font-size: 17px;
    font-weight: 700;
  }

  .hud-total {
    font-size: 11px;
  }

  .badge {
    font-size: 8.5px;
    padding: 1px 5px;
  }

  .hud-progress-track {
    height: 3px;
  }

  .hud-card-footer-text {
    font-size: 9.5px;
  }
}
</style>
