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
.node-meta-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 7px; }
.meta-item { display: flex; flex-direction: column; justify-content: center; gap: 3px; min-height: 48px; background: rgba(255, 255, 255, 0.025); border: 1px solid rgba(255, 255, 255, 0.05); border-radius: 7px; padding: 5px 9px; transition: border-color 0.2s ease, background 0.2s ease; }
.meta-item:hover { background: rgba(255, 255, 255, 0.04); border-color: rgba(255, 255, 255, 0.09); }
.meta-header-row { display: flex; align-items: center; justify-content: space-between; line-height: 1; }
.meta-label { font-size: 0.65rem; font-weight: 700; letter-spacing: 0.05em; text-transform: uppercase; color: var(--text-muted, #64748b); }
.meta-badge { font-size: 0.68rem; font-weight: 700; padding: 1px 4px; border-radius: 4px; background: rgba(0, 0, 0, 0.25); border: 1px solid rgba(255, 255, 255, 0.05); line-height: 1.1; }
.meta-val { font-size: 0.76rem; font-weight: 600; color: var(--text-primary, #f8fafc); font-variant-numeric: tabular-nums; letter-spacing: -0.01em; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; line-height: 1.25; }
.meta-sep { color: var(--text-muted, #64748b); margin: 0 1px; font-weight: 400; opacity: 0.7; }
.io-offline { font-size: 0.76rem; font-weight: 600; color: var(--text-muted, #64748b); line-height: 1.25; }
.io-dual-stream { display: flex; align-items: center; justify-content: space-between; gap: 3px; font-size: 0.72rem; font-weight: 600; line-height: 1.25; font-variant-numeric: tabular-nums; }
.io-stream { display: inline-flex; align-items: center; gap: 3px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.stream-arrow { font-size: 0.76rem; font-weight: 700; flex-shrink: 0; }
.stream-val { letter-spacing: -0.01em; color: var(--text-secondary, #cbd5e1); }
.font-mono { font-family: var(--font-mono, monospace); }
.text-cyan { color: #06b6d4; }
.text-sky { color: #38bdf8; }
.text-emerald { color: #10b981; }
.text-purple { color: #c084fc; }
.text-amber { color: #fbbf24; }
.text-muted { color: var(--text-muted, #64748b); }
</style>
