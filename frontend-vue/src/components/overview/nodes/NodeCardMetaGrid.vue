<script setup lang="ts">
import type { NodeMetrics } from '../../../api/overview'

defineProps<{ node: NodeMetrics }>()

function isNodeOffline(node: NodeMetrics): boolean {
  return node.status === 'down' || node.status === 'offline' || node.status === 'disconnected' || node.memory_total === 0
}

function formatBytes(bytes?: number, decimals = 1): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024, dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i] || 'B'}`
}

function formatIoRate(bytesPerSec?: number): string {
  if (!bytesPerSec || bytesPerSec <= 0 || isNaN(bytesPerSec)) return '0 B/s'
  return `${formatBytes(bytesPerSec)}/s`
}
</script>

<template>
  <div class="node-meta-grid">
    <!-- 1. Memory RAM -->
    <div class="meta-item">
      <div class="meta-header-row">
        <span class="meta-label">Memory RAM</span>
        <span class="meta-badge text-cyan font-mono">{{ isNodeOffline(node) ? '—' : `${Math.round(node.memory_percent)}%` }}</span>
      </div>
      <div class="meta-val font-mono" :title="isNodeOffline(node) ? 'Offline' : `${formatBytes(node.memory_used)} / ${formatBytes(node.memory_total)}`">
        <template v-if="isNodeOffline(node)">— / —</template>
        <template v-else>{{ formatBytes(node.memory_used) }}<span class="meta-sep">/</span>{{ formatBytes(node.memory_total) }}</template>
      </div>
    </div>

    <!-- 2. Disk Storage -->
    <div class="meta-item">
      <div class="meta-header-row">
        <span class="meta-label">Disk Storage</span>
        <span class="meta-badge text-emerald font-mono">{{ isNodeOffline(node) ? '—' : `${Math.round(node.disk_percent)}%` }}</span>
      </div>
      <div class="meta-val font-mono" :title="isNodeOffline(node) ? 'Offline' : `${formatBytes(node.disk_used)} / ${formatBytes(node.disk_total)}`">
        <template v-if="isNodeOffline(node)">— / —</template>
        <template v-else>{{ formatBytes(node.disk_used) }}<span class="meta-sep">/</span>{{ formatBytes(node.disk_total) }}</template>
      </div>
    </div>

    <!-- 3. Live Network I/O -->
    <div class="meta-item">
      <div class="meta-header-row"><span class="meta-label">Network I/O</span></div>
      <div v-if="isNodeOffline(node)" class="io-offline text-muted font-mono">—</div>
      <div v-else class="io-dual-stream font-mono">
        <span class="io-stream rx" title="Inbound Traffic (Download / Rx)">
          <span class="stream-arrow text-cyan">↓</span>
          <span class="stream-val">{{ formatIoRate(node.network_rx_bytes) }}</span>
        </span>
        <span class="io-stream tx" title="Outbound Traffic (Upload / Tx)">
          <span class="stream-arrow text-purple">↑</span>
          <span class="stream-val">{{ formatIoRate(node.network_tx_bytes) }}</span>
        </span>
      </div>
    </div>

    <!-- 4. Live Disk I/O Rates -->
    <div class="meta-item">
      <div class="meta-header-row"><span class="meta-label">Disk I/O Rate</span></div>
      <div v-if="isNodeOffline(node)" class="io-offline text-muted font-mono">—</div>
      <div v-else class="io-dual-stream font-mono">
        <span class="io-stream rx" title="Physical Disk Read Rate">
          <span class="stream-arrow text-sky">📖</span>
          <span class="stream-val">{{ formatIoRate(node.disk_read_bytes_per_sec || 0) }}</span>
        </span>
        <span class="io-stream tx" title="Physical Disk Write Rate">
          <span class="stream-arrow text-amber">✍️</span>
          <span class="stream-val">{{ formatIoRate(node.disk_write_bytes_per_sec || 0) }}</span>
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/views/overview.css';
</style>
