<script setup lang="ts">
import type { NodeMetrics } from '../../../api/overview'
import CircularGauge from '../../ui/CircularGauge.vue'

interface Props {
  node: NodeMetrics
  busiestNodeId?: string | null
  draggedNodeId?: string | null
  dragOverNodeId?: string | null
}

defineProps<Props>()

defineEmits<{
  (e: 'click', node: NodeMetrics): void
  (e: 'inspect', node: NodeMetrics): void
  (e: 'manage', node: NodeMetrics): void
  (e: 'dragstart', event: DragEvent, node: NodeMetrics): void
  (e: 'dragover', event: DragEvent, node: NodeMetrics): void
  (e: 'dragenter', node: NodeMetrics): void
  (e: 'dragleave', event: DragEvent, node: NodeMetrics): void
  (e: 'drop', node: NodeMetrics): void
  (e: 'dragend'): void
}>()

function getNodeCardClass(node: NodeMetrics): string {
  if (node.status === 'down' || node.status === 'offline' || node.status === 'disconnected') return 'node-down'
  if (node.cpu_percent >= 80 || node.memory_percent >= 80) return 'node-overloaded'
  if (node.cpu_percent >= 60 || node.memory_percent >= 60) return 'node-warning'
  return 'node-healthy'
}

function formatDistro(distro?: string, os?: string): string {
  if (distro && distro.trim().length > 0) return distro.trim()
  if (os && os.trim().length > 0) return os.trim()
  return 'Linux'
}

function formatShortDistro(distro?: string, os?: string): string {
  const d = formatDistro(distro, os)
  if (!d) return 'Linux'
  if (d.toLowerCase().includes('ubuntu')) return 'Ubuntu'
  if (d.toLowerCase().includes('debian')) return 'Debian'
  if (d.toLowerCase().includes('centos')) return 'CentOS'
  if (d.toLowerCase().includes('fedora')) return 'Fedora'
  if (d.toLowerCase().includes('red hat') || d.toLowerCase().includes('rhel')) return 'RHEL'
  if (d.toLowerCase().includes('alpine')) return 'Alpine'
  if (d.toLowerCase().includes('arch')) return 'Arch'
  return d
}

function formatOsSummary(node?: NodeMetrics | null): string {
  if (!node) return 'Linux (amd64)'
  const arch = node.arch || 'amd64'
  const distro = formatShortDistro(node.os_distro, node.os)
  return `${distro} (${arch})`
}

function formatBytes(bytes?: number, decimals = 1): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
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
  <div
    class="node-card glass-panel cursor-pointer smooth-opacity"
    :class="[
      getNodeCardClass(node),
      {
        'is-busiest': node.node_id === busiestNodeId,
        'is-dragging': draggedNodeId === node.node_id,
        'is-drag-over': dragOverNodeId === node.node_id && draggedNodeId !== node.node_id,
      }
    ]"
    draggable="true"
    @dragstart="$emit('dragstart', $event, node)"
    @dragover.prevent="$emit('dragover', $event, node)"
    @dragenter.prevent="$emit('dragenter', node)"
    @dragleave="$emit('dragleave', $event, node)"
    @drop.prevent="$emit('drop', node)"
    @dragend="$emit('dragend')"
    @click="$emit('click', node)"
  >
    <!-- Card Top: Name, Source Badge, Role & Status (2-Row Layout) -->
    <div class="node-card-header">
      <div class="node-header-top">
        <div class="node-identity">
          <span class="drag-handle" title="Drag to reorder server card">⋮⋮</span>
          <span
            class="node-status-dot status-dot-pulse"
            :class="node.status === 'ready' || node.status === 'online' ? 'status-green' : 'status-red'"
          ></span>
          <h3 class="node-name" :title="node.node_name">{{ node.node_name }}</h3>
        </div>
        <div class="node-header-badges">
          <span v-if="node.node_id === busiestNodeId" class="badge badge-amber badge-traffic-pulse" title="Highest traffic node">
            🔥 HOT NODE
          </span>
          <span v-else-if="node.role?.toLowerCase() === 'master' || node.role?.toLowerCase() === 'control-plane' || node.role?.toLowerCase() === 'manager'" class="badge badge-purple" title="Cluster Manager">
            👑 MANAGER
          </span>
          <span v-else class="badge badge-indigo" title="Telemetry Agent">
            📡 AGENT
          </span>
        </div>
      </div>
      <div class="node-header-meta">
        <span class="badge-meta font-mono" :title="formatOsSummary(node)">
          🐧 {{ formatOsSummary(node) }}
        </span>
        <span
          class="badge-meta font-mono"
          :title="`${node.running_count ?? node.container_count ?? 0} Active Containers · ${node.processes || 0} Host PIDs`"
        >
          📦 {{ node.running_count ?? node.container_count ?? 0 }} ctr · {{ node.processes || 0 }} pids
        </span>
      </div>
    </div>

    <!-- Gauge Cluster: CPU / RAM / Disk -->
    <div class="node-gauges-cluster">
      <div class="gauge-col">
        <CircularGauge
          :percent="node.cpu_percent"
          label="CPU"
          :size="52"
          :strokeWidth="3.5"
          :disabled="node.status === 'down' || node.status === 'offline' || node.status === 'disconnected'"
        />
      </div>
      <div class="gauge-col">
        <CircularGauge
          :percent="node.memory_percent"
          label="RAM"
          :size="52"
          :strokeWidth="3.5"
          :disabled="node.status === 'down' || node.status === 'offline' || node.status === 'disconnected'"
        />
      </div>
      <div class="gauge-col">
        <CircularGauge
          :percent="node.disk_percent"
          label="DISK"
          :size="52"
          :strokeWidth="3.5"
          :disabled="node.status === 'down' || node.status === 'offline' || node.status === 'disconnected'"
        />
      </div>
    </div>

    <!-- Node Resource Detail Stats (Symmetrical 2x2 Balanced Matrix) -->
    <div class="node-meta-grid">
      <!-- 1. Memory RAM -->
      <div class="meta-item">
        <div class="meta-header-row">
          <span class="meta-label">Memory RAM</span>
          <span class="meta-badge text-cyan font-mono">{{ Math.round(node.memory_percent) }}%</span>
        </div>
        <div class="meta-val font-mono" :title="`${formatBytes(node.memory_used)} / ${formatBytes(node.memory_total)}`">
          {{ formatBytes(node.memory_used) }}<span class="meta-sep">/</span>{{ formatBytes(node.memory_total) }}
        </div>
      </div>

      <!-- 2. Disk Storage -->
      <div class="meta-item">
        <div class="meta-header-row">
          <span class="meta-label">Disk Storage</span>
          <span class="meta-badge text-emerald font-mono">{{ Math.round(node.disk_percent) }}%</span>
        </div>
        <div class="meta-val font-mono" :title="`${formatBytes(node.disk_used)} / ${formatBytes(node.disk_total)}`">
          {{ formatBytes(node.disk_used) }}<span class="meta-sep">/</span>{{ formatBytes(node.disk_total) }}
        </div>
      </div>

      <!-- 3. Live Network I/O -->
      <div class="meta-item">
        <div class="meta-header-row">
          <span class="meta-label">Network I/O</span>
        </div>
        <div class="io-dual-stream font-mono">
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
        <div class="meta-header-row">
          <span class="meta-label">Disk I/O Rate</span>
        </div>
        <div class="io-dual-stream font-mono">
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

    <!-- Footer Actions: Direct Navigation & Process Inspection -->
    <div class="node-card-footer">
      <div class="node-footer-actions">
        <button
          type="button"
          class="btn-node-action btn-inspect-node"
          @click.stop="$emit('inspect', node)"
          title="Inspect real-time telemetry, hardware saturation, and top processes"
        >
          <span>🔍 Inspect Telemetry & Apps</span>
        </button>
        <button
          type="button"
          class="btn-node-action btn-manage-host"
          @click.stop="$emit('manage', node)"
          title="Manage server in Infrastructure Registry"
        >
          <span>⚙️ Manage</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.node-card {
  padding: 16px 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  position: relative;
  background: var(--glass-bg, rgba(26, 31, 46, 0.7));
  border: 1px solid var(--border-color, rgba(255, 255, 255, 0.08));
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
  transition: opacity 0.3s ease,
              transform 0.3s ease,
              box-shadow 0.2s cubic-bezier(0.4, 0, 0.2, 1),
              border-color 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: grab;
}

.node-card:hover {
  transform: translateY(-2px);
  border-color: rgba(56, 189, 248, 0.35);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3), 0 0 15px rgba(56, 189, 248, 0.1);
}

.node-card:active {
  cursor: grabbing;
}

.node-card.is-dragging {
  opacity: 0.35;
  transform: scale(0.97);
  border: 2px dashed rgba(56, 189, 248, 0.6) !important;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  cursor: grabbing;
}

.node-card.is-drag-over {
  border-color: #38bdf8 !important;
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.5), 0 8px 25px rgba(56, 189, 248, 0.25) !important;
  transform: translateY(-4px) scale(1.02);
  background: rgba(56, 189, 248, 0.08);
}

.node-overloaded {
  border-color: rgba(244, 63, 94, 0.4);
  box-shadow: 0 4px 20px rgba(244, 63, 94, 0.12);
}

.node-warning {
  border-color: rgba(245, 158, 11, 0.3);
}

.node-down {
  opacity: 0.6;
  filter: grayscale(0.7);
}

.is-busiest {
  box-shadow: 0 0 25px rgba(245, 158, 11, 0.18);
  border-color: rgba(245, 158, 11, 0.4);
}

.drag-handle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted, #94a3b8);
  font-size: 13px;
  letter-spacing: -2px;
  cursor: grab;
  padding: 2px 4px;
  border-radius: 4px;
  transition: color 0.15s ease, background-color 0.15s ease, opacity 0.15s ease;
  user-select: none;
  opacity: 0.5;
}

.node-card:hover .drag-handle {
  opacity: 0.95;
  color: var(--text-primary, #f8fafc);
  background: rgba(255, 255, 255, 0.06);
}

.drag-handle:hover {
  opacity: 1;
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
}

.drag-handle:active {
  cursor: grabbing;
}

.badge-traffic-pulse {
  animation: pulse-traffic 2s infinite ease-in-out;
}

@keyframes pulse-traffic {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.05); opacity: 0.85; }
}

/* 2-Row Node Card Header */
.node-card-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.node-header-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}

.node-identity {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.node-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-green { background: #10b981; box-shadow: 0 0 8px rgba(16, 185, 129, 0.6); }
.status-red { background: #f43f5e; box-shadow: 0 0 8px rgba(244, 63, 94, 0.6); }

.status-dot-pulse {
  animation: pulseDot 2s infinite ease-in-out;
}

@keyframes pulseDot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.node-name {
  font-size: 1.1rem;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-primary, #f8fafc);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin: 0;
  line-height: 1.2;
}

.node-header-badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.badge-purple { background: rgba(168, 85, 247, 0.15); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.3); }
.badge-indigo { background: rgba(99, 102, 241, 0.15); color: #818cf8; border: 1px solid rgba(99, 102, 241, 0.3); }

.node-header-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.badge-meta {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.72rem;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-secondary, #94a3b8);
  white-space: nowrap;
}

.node-gauges-cluster {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 10px 6px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(0, 0, 0, 0.18);
  border-radius: 10px;
}

.gauge-col {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.node-meta-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 7px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 3px;
  min-height: 48px;
  background: rgba(255, 255, 255, 0.025);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 7px;
  padding: 5px 9px;
  transition: border-color 0.2s ease, background 0.2s ease;
}

.meta-item:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.09);
}

.meta-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  line-height: 1;
}

.meta-label {
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted, #64748b);
}

.meta-badge {
  font-size: 0.68rem;
  font-weight: 700;
  padding: 1px 4px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.05);
  line-height: 1.1;
}

.meta-val {
  font-size: 0.76rem;
  font-weight: 600;
  color: var(--text-primary, #f8fafc);
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.01em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.25;
}

.meta-sep {
  color: var(--text-muted, #64748b);
  margin: 0 1px;
  font-weight: 400;
  opacity: 0.7;
}

.io-dual-stream {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 3px;
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1.25;
  font-variant-numeric: tabular-nums;
}

.io-stream {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.stream-arrow {
  font-size: 0.76rem;
  font-weight: 700;
  flex-shrink: 0;
}

.stream-val {
  letter-spacing: -0.01em;
  color: var(--text-secondary, #cbd5e1);
}

.node-card-footer {
  margin-top: auto;
}

.node-footer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.btn-node-action {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 7px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.09);
  color: var(--text-secondary, #94a3b8);
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  white-space: nowrap;
}

.btn-node-action:hover {
  background: rgba(255, 255, 255, 0.09);
  color: var(--text-primary, #f8fafc);
  border-color: rgba(56, 189, 248, 0.4);
  transform: translateY(-1px);
}

.btn-inspect-node {
  border-color: rgba(6, 182, 212, 0.25);
  color: #22d3ee;
  background: rgba(6, 182, 212, 0.06);
}

.btn-inspect-node:hover {
  background: rgba(6, 182, 212, 0.16);
  border-color: #06b6d4;
  color: #38bdf8;
  box-shadow: 0 0 12px rgba(6, 182, 212, 0.25);
  transform: translateY(-1px);
}

.btn-manage-host {
  border-color: rgba(129, 140, 248, 0.25);
  color: #a5b4fc;
  background: rgba(99, 102, 241, 0.06);
}

.btn-manage-host:hover {
  border-color: #818cf8;
  color: #c7d2fe;
  background: rgba(99, 102, 241, 0.16);
  box-shadow: 0 0 12px rgba(99, 102, 241, 0.25);
  transform: translateY(-1px);
}

.font-mono { font-family: var(--font-mono, monospace); }
.font-bold { font-weight: 700; }
.text-cyan { color: #06b6d4; }
.text-sky { color: #38bdf8; }
.text-emerald { color: #10b981; }
.text-purple { color: #c084fc; }
.text-amber { color: #fbbf24; }
</style>
