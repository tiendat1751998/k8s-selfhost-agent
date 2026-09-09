<script setup lang="ts">
import type { NodeMetrics } from '../../../../api/overview'
import { getUtilizationColor, formatPercent, formatBytes, formatIoRate } from './nodeDiagnosticsUtils'

interface Props {
  node: NodeMetrics
  isNodeOffline?: boolean
}

defineProps<Props>()
</script>

<template>
  <div class="hardware-hud-section">
    <div class="hud-section-header">
      <h4 class="hud-section-heading">
        <span class="heading-icon">📊</span> Hardware Saturation Telemetry
      </h4>
      <span class="badge badge-indigo font-mono">LIVE METRICS</span>
    </div>
    <div class="hardware-gauges-grid">
      <!-- CPU Load -->
      <div class="hw-gauge-card glass-panel">
        <div class="hw-gauge-top">
          <span class="hw-gauge-label">CPU LOAD</span>
          <span
            class="hw-gauge-val font-mono font-bold smooth-value"
            :class="`text-${getUtilizationColor(node.cpu_percent || 0)}`"
          >
            {{ isNodeOffline ? '—' : formatPercent(node.cpu_percent) }}
          </span>
        </div>
        <div class="hw-progress-track">
          <div
            class="hw-progress-fill smooth-bar"
            :class="`bg-${getUtilizationColor(node.cpu_percent || 0)}`"
            :style="{ width: isNodeOffline ? '0%' : `${Math.min(100, node.cpu_percent || 0)}%` }"
          ></div>
        </div>
        <div class="hw-gauge-sub">
          <span>Saturation:</span>
          <strong :class="`text-${getUtilizationColor(node.cpu_percent || 0)}`">
            {{ (node.cpu_percent || 0) > 85 ? 'HIGH' : (node.cpu_percent || 0) > 60 ? 'ELEVATED' : 'NOMINAL' }}
          </strong>
        </div>
      </div>

      <!-- Memory Usage -->
      <div class="hw-gauge-card glass-panel">
        <div class="hw-gauge-top">
          <span class="hw-gauge-label">MEMORY USAGE</span>
          <span
            class="hw-gauge-val font-mono font-bold smooth-value"
            :class="`text-${getUtilizationColor(node.memory_percent || 0)}`"
          >
            {{ isNodeOffline ? '—' : formatPercent(node.memory_percent) }}
          </span>
        </div>
        <div class="hw-progress-track">
          <div
            class="hw-progress-fill smooth-bar"
            :class="`bg-${getUtilizationColor(node.memory_percent || 0)}`"
            :style="{ width: isNodeOffline ? '0%' : `${Math.min(100, node.memory_percent || 0)}%` }"
          ></div>
        </div>
        <div class="hw-gauge-sub">
          <span :class="{ 'text-muted': isNodeOffline }">{{ isNodeOffline ? '— / —' : `${formatBytes(node.memory_used || 0)} / ${formatBytes(node.memory_total || 0)}` }}</span>
        </div>
      </div>

      <!-- Disk Storage -->
      <div class="hw-gauge-card glass-panel">
        <div class="hw-gauge-top">
          <span class="hw-gauge-label">DISK STORAGE</span>
          <span
            class="hw-gauge-val font-mono font-bold smooth-value"
            :class="`text-${getUtilizationColor(node.disk_percent || 0)}`"
          >
            {{ isNodeOffline ? '—' : formatPercent(node.disk_percent) }}
          </span>
        </div>
        <div class="hw-progress-track">
          <div
            class="hw-progress-fill smooth-bar"
            :class="`bg-${getUtilizationColor(node.disk_percent || 0)}`"
            :style="{ width: isNodeOffline ? '0%' : `${Math.min(100, node.disk_percent || 0)}%` }"
          ></div>
        </div>
        <div class="hw-gauge-sub">
          <span :class="{ 'text-muted': isNodeOffline }">{{ isNodeOffline ? '— / —' : `${formatBytes(node.disk_used || 0)} / ${formatBytes(node.disk_total || 0)}` }}</span>
        </div>
      </div>

      <!-- NETWORK I/O HUD -->
      <div class="hw-gauge-card glass-panel hw-gauge-card-net">
        <div class="hw-gauge-top">
          <span class="hw-gauge-label">NETWORK I/O</span>
          <span class="badge badge-slate font-mono font-bold" style="padding: 1px 6px; font-size: 9px;">
            {{ node.running_count ?? node.container_count ?? 0 }} SVC
          </span>
        </div>
        <div class="hw-progress-track hw-net-track">
          <div
            class="hw-progress-fill bg-cyan smooth-bar"
            :style="{ width: `${Math.min(100, Math.max(15, (node.network_rx_bytes || 0) / ((node.network_rx_bytes || 0) + (node.network_tx_bytes || 0) || 1) * 100))}%` }"
            title="Download (Rx) vs Upload (Tx) distribution"
          ></div>
        </div>
        <div class="hw-gauge-sub hw-net-dual-sub">
          <span class="text-cyan font-mono font-bold" :title="'Real-time Download Rate (Rx)'">↓ {{ isNodeOffline ? '—' : formatIoRate(node.network_rx_bytes) }}</span>
          <span class="text-purple font-mono font-bold" :title="'Real-time Upload Rate (Tx)'">↑ {{ isNodeOffline ? '—' : formatIoRate(node.network_tx_bytes) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../../../assets/styles/components/node-diagnostics.css';
</style>
