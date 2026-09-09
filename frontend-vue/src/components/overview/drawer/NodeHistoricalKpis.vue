<script setup lang="ts">
import type { NodeMetrics } from '../../../api/overview'
import type { NodeHistoryResponse } from '../../../api/compute'
import { formatPercent, formatBytes, formatIoRate } from './nodeChartMath'

interface Props {
  node?: NodeMetrics | null
  nodeHistoryData?: NodeHistoryResponse | null
}

defineProps<Props>()
</script>

<template>
  <div class="hist-kpi-grid">
    <!-- Card 1: Realtime CPU & Peak -->
    <div class="hist-kpi-card glass-panel">
      <span class="kpi-label">REALTIME CPU / PEAK</span>
      <span class="kpi-val text-violet">{{ formatPercent(node?.cpu_percent) }}</span>
      <span class="kpi-sub font-mono">
        🔥 Peak: {{ formatPercent(nodeHistoryData?.summary?.peak_cpu_percent || node?.cpu_percent) }} <span class="text-slate">| Avg: {{ formatPercent(nodeHistoryData?.summary?.avg_cpu_percent || node?.cpu_percent) }}</span>
      </span>
    </div>

    <!-- Card 2: Realtime RAM & Peak -->
    <div class="hist-kpi-card glass-panel">
      <span class="kpi-label">REALTIME RAM / PEAK</span>
      <span class="kpi-val text-cyan">{{ formatPercent(node?.memory_percent) }}</span>
      <span class="kpi-sub font-mono">
        🧠 {{ formatBytes(node?.memory_used) }} / {{ formatBytes(node?.memory_total) }} <span class="text-slate">(Peak: {{ formatPercent(nodeHistoryData?.summary?.peak_mem_percent || node?.memory_percent) }})</span>
      </span>
    </div>

    <!-- Card 3: Live Network I/O -->
    <div class="hist-kpi-card glass-panel">
      <span class="kpi-label">LIVE NETWORK I/O</span>
      <div class="hist-kpi-net-row font-mono">
        <div class="net-pill net-pill-rx">
          <span class="net-pill-arrow">↓</span>
          <span class="net-pill-val">{{ formatIoRate(node?.network_rx_bytes || 0) }}</span>
        </div>
        <div class="net-pill net-pill-tx">
          <span class="net-pill-arrow">↑</span>
          <span class="net-pill-val">{{ formatIoRate(node?.network_tx_bytes || 0) }}</span>
        </div>
      </div>
      <span class="kpi-sub font-mono">
        ⚡ Peak: <span class="text-emerald">↓ {{ formatIoRate(nodeHistoryData?.summary?.peak_rx_bytes_sec || node?.network_rx_bytes) }}</span> <span class="text-slate">·</span> <span class="text-cyan">↑ {{ formatIoRate(nodeHistoryData?.summary?.peak_tx_bytes_sec || node?.network_tx_bytes) }}</span>
      </span>
    </div>

    <!-- Card 4: Uptime & Disk Usage -->
    <div class="hist-kpi-card glass-panel">
      <span class="kpi-label">UPTIME & DISK USAGE</span>
      <span class="kpi-val text-emerald">
        {{ formatPercent(nodeHistoryData?.summary?.uptime_percent || 99) }}
      </span>
      <span class="kpi-sub font-mono">
        💾 Disk: {{ formatPercent(node?.disk_percent) }} <span class="text-slate">({{ formatBytes(node?.disk_used) }} / {{ formatBytes(node?.disk_total) }})</span>
      </span>
    </div>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/node-historical-chart.css';
</style>
