<script setup lang="ts">
import type { NodeMetrics } from '../../../api/overview'

defineProps<{
  nodes: NodeMetrics[]
  busiestNodeId?: string | null
}>()

defineEmits<{
  (e: 'click', node: NodeMetrics): void
  (e: 'details', node: NodeMetrics): void
  (e: 'logs', node: NodeMetrics): void
  (e: 'scale', node: NodeMetrics): void
  (e: 'restart', node: NodeMetrics): void
  (e: 'yaml', node: NodeMetrics): void
  (e: 'delete', node: NodeMetrics): void
}>()

function isOffline(node: NodeMetrics): boolean {
  return node.status === 'down' || node.status === 'offline' || node.status === 'disconnected' || node.memory_total === 0
}

function getNodeStatus(node: NodeMetrics): { type: 'ready' | 'offline' | 'degraded'; label: string } {
  if (isOffline(node)) return { type: 'offline', label: 'Offline' }
  if (node.cpu_percent >= 80 || node.memory_percent >= 80 || node.disk_percent >= 90 || node.status === 'degraded' || node.status === 'warning') {
    return { type: 'degraded', label: 'Degraded' }
  }
  return { type: 'ready', label: 'Ready' }
}

function getRoleBadge(role?: string): { label: string; cls: string } {
  const r = (role || '').toLowerCase()
  if (r.includes('master') || r.includes('control') || r.includes('manager')) return { label: 'Control', cls: 'badge-control' }
  if (r.includes('worker')) return { label: 'Worker', cls: 'badge-worker' }
  return { label: 'Agent', cls: 'badge-agent' }
}

function getNodeIp(node: NodeMetrics): string {
  const anyNode = node as any
  if (anyNode.ip) return anyNode.ip
  if (anyNode.ip_address) return anyNode.ip_address
  if (anyNode.internal_ip) return anyNode.internal_ip
  if (anyNode.endpoint) return anyNode.endpoint.replace(/^https?:\/\//, '').split(':')[0]
  return '--'
}

function getOsDistro(node: NodeMetrics): string {
  const d = (node.os_distro || node.os || 'Linux').trim()
  for (const name of ['Ubuntu', 'Debian', 'CentOS', 'Alpine']) {
    if (d.toLowerCase().includes(name.toLowerCase())) return name
  }
  return d.length > 10 ? d.slice(0, 10) : d
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0B'
  const k = 1024
  const sizes = ['B', 'K', 'M', 'G', 'T']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))}${sizes[i] || 'B'}`
}

function getNodePing(node: NodeMetrics): number {
  const anyNode = node as any
  if (typeof anyNode.ping_ms === 'number') return anyNode.ping_ms
  if (typeof anyNode.latency_ms === 'number') return anyNode.latency_ms
  if (anyNode.latency && !isNaN(parseInt(anyNode.latency))) return parseInt(anyNode.latency)
  return 0
}
</script>

<template>
  <div class="node-table-container glass-panel animate-fade-in">
    <div class="table-responsive">
      <table class="node-table">
        <thead>
          <tr>
            <th class="col-status">Status</th>
            <th class="col-name">Node Name &amp; Role</th>
            <th class="col-ip">IP Address &amp; OS</th>
            <th class="col-cpu">CPU Load</th>
            <th class="col-ram">Memory RAM</th>
            <th class="col-disk">Disk Storage</th>
            <th class="col-workloads">Workloads</th>
            <th class="col-probe">Probes / Ping</th>
            <th class="col-actions text-right">Node Operations Suite</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="node in nodes" :key="node.node_id" class="node-row" :class="{ 'row-busiest': node.node_id === busiestNodeId }" @click="$emit('click', node)">
            <td class="col-status">
              <span class="status-wrap font-mono" :class="`status-${getNodeStatus(node).type}`">
                <span class="pulse-dot" :class="`dot-${getNodeStatus(node).type}`"></span>
                <span class="status-label">{{ getNodeStatus(node).label }}</span>
              </span>
            </td>
            <td class="col-name">
              <div class="name-role-cell">
                <span class="node-name-text font-bold" :title="node.node_name">{{ node.node_name }}</span>
                <span class="role-badge font-mono" :class="getRoleBadge(node.role).cls">[{{ getRoleBadge(node.role).label }}]</span>
                <span v-if="node.node_id === busiestNodeId" class="badge-hot" title="Highest traffic">🔥</span>
              </div>
            </td>
            <td class="col-ip font-mono">
              <span class="ip-text">{{ getNodeIp(node) }}</span>
              <span class="os-distro-pill">{{ getOsDistro(node) }}</span>
            </td>
            <td class="col-cpu font-mono">
              <span v-if="isOffline(node)" class="text-muted">—</span>
              <div v-else class="progress-cell">
                <span class="pct-num">{{ Math.round(node.cpu_percent) }}%</span>
                <div class="bar-track">
                  <div class="bar-fill" :class="node.cpu_percent >= 80 ? 'fill-rose' : node.cpu_percent >= 60 ? 'fill-amber' : 'fill-cyan'" :style="{ width: `${Math.min(100, Math.max(0, node.cpu_percent))}%` }"></div>
                </div>
              </div>
            </td>
            <td class="col-ram font-mono">
              <span v-if="isOffline(node)" class="text-muted">— / —</span>
              <span v-else class="resource-text"><strong class="text-cyan">{{ Math.round(node.memory_percent) }}%</strong><span class="sub-dim">({{ formatBytes(node.memory_used) }}/{{ formatBytes(node.memory_total) }})</span></span>
            </td>
            <td class="col-disk font-mono">
              <span v-if="isOffline(node)" class="text-muted">— / —</span>
              <span v-else class="resource-text"><strong class="text-emerald">{{ Math.round(node.disk_percent) }}%</strong><span class="sub-dim">({{ formatBytes(node.disk_used) }}/{{ formatBytes(node.disk_total) }})</span></span>
            </td>
            <td class="col-workloads font-mono">
              <span v-if="isOffline(node)" class="text-muted">—</span>
              <span v-else class="badge-workload">{{ node.running_count ?? node.container_count ?? 0 }} ctr</span>
            </td>
            <td class="col-probe font-mono">
              <span v-if="isOffline(node) || getNodePing(node) <= 0" class="text-muted">--</span>
              <span v-else class="probe-val">⚡ {{ getNodePing(node) }}ms</span>
            </td>
            <td class="col-actions text-right" @click.stop>
              <div class="sre-suite">
                <button type="button" class="sre-btn btn-logs" title="Stream Logs" @click="$emit('logs', node)">📄 Logs</button>
                <button type="button" class="sre-btn btn-scale" title="Scale Workloads" @click="$emit('scale', node)">⚡ Scale</button>
                <button type="button" class="sre-btn btn-restart" title="Restart Agent" @click="$emit('restart', node)">🔄 Restart</button>
                <button type="button" class="sre-btn btn-yaml" title="View Manifest YAML" @click="$emit('yaml', node)">🎯 YAML</button>
                <button type="button" class="sre-btn btn-details" title="Diagnostics & Details" @click="$emit('details', node)">🔍 Details</button>
                <button type="button" class="sre-btn btn-delete" title="Cordon / Evict" @click="$emit('delete', node)">🗑 Delete</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.node-table-container { border-radius: 12px; overflow: hidden; border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08)); }
.table-responsive { width: 100%; overflow-x: auto; }
.node-table { width: 100%; border-collapse: collapse; text-align: left; font-size: 11.5px; }
.node-table th { padding: 6px 10px; background: rgba(15, 23, 42, 0.75); color: var(--text-muted, #94a3b8); font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.06em; border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08)); white-space: nowrap; }
.node-row { height: 38px; border-bottom: 1px solid rgba(255, 255, 255, 0.04); transition: background 0.15s ease; cursor: pointer; }
.node-row:hover { background: rgba(56, 189, 248, 0.05); }
.node-table td { padding: 2px 10px; height: 38px; vertical-align: middle; white-space: nowrap; }
.status-wrap { display: inline-flex; align-items: center; gap: 5px; font-size: 10.5px; font-weight: 700; text-transform: uppercase; }
.pulse-dot { width: 6px; height: 6px; border-radius: 50%; }
.dot-ready { background: #10b981; box-shadow: 0 0 6px rgba(16, 185, 129, 0.6); }
.dot-offline { background: #f43f5e; box-shadow: 0 0 6px rgba(244, 63, 94, 0.6); }
.dot-degraded { background: #fbbf24; box-shadow: 0 0 6px rgba(251, 191, 36, 0.6); }
.status-ready { color: #34d399; }
.status-offline { color: #fb7185; }
.status-degraded { color: #fcd34d; }
.name-role-cell { display: flex; align-items: center; gap: 6px; }
.node-name-text { color: var(--text-primary, #f8fafc); max-width: 140px; overflow: hidden; text-overflow: ellipsis; }
.role-badge { font-size: 9.5px; font-weight: 700; padding: 1px 4px; border-radius: 3px; }
.badge-control { color: #c084fc; background: rgba(168, 85, 247, 0.12); border: 1px solid rgba(168, 85, 247, 0.25); }
.badge-worker { color: #38bdf8; background: rgba(56, 189, 248, 0.12); border: 1px solid rgba(56, 189, 248, 0.25); }
.badge-agent { color: #818cf8; background: rgba(99, 102, 241, 0.12); border: 1px solid rgba(99, 102, 241, 0.25); }
.ip-text { color: var(--text-secondary, #cbd5e1); margin-right: 6px; font-size: 11px; }
.os-distro-pill { font-size: 9px; padding: 1px 4px; border-radius: 3px; background: rgba(255, 255, 255, 0.05); color: var(--text-muted, #94a3b8); }
.progress-cell { display: flex; align-items: center; gap: 6px; }
.pct-num { width: 28px; text-align: right; font-size: 11px; color: var(--text-primary, #f8fafc); }
.bar-track { width: 48px; height: 5px; background: rgba(255, 255, 255, 0.08); border-radius: 999px; overflow: hidden; }
.bar-fill { height: 100%; border-radius: 999px; }
.fill-cyan { background: #06b6d4; }
.fill-amber { background: #f59e0b; }
.fill-rose { background: #f43f5e; }
.resource-text { font-size: 11px; display: inline-flex; align-items: center; gap: 4px; }
.sub-dim { color: var(--text-muted, #64748b); font-size: 10px; }
.badge-workload { font-size: 10.5px; color: #38bdf8; background: rgba(56, 189, 248, 0.1); padding: 1px 5px; border-radius: 4px; }
.probe-val { color: #10b981; font-size: 11px; }
.sre-suite { display: inline-flex; align-items: center; gap: 3px; justify-content: flex-end; }
.sre-btn { padding: 3px 6px; font-size: 9.5px; font-weight: 600; border-radius: 4px; background: rgba(255, 255, 255, 0.04); border: 1px solid rgba(255, 255, 255, 0.08); color: var(--text-secondary, #94a3b8); cursor: pointer; white-space: nowrap; transition: all 0.15s ease; }
.sre-btn:hover { background: rgba(255, 255, 255, 0.1); color: #fff; transform: translateY(-1px); }
.btn-logs:hover { border-color: #f59e0b; color: #fbbf24; }
.btn-scale:hover { border-color: #38bdf8; color: #38bdf8; }
.btn-restart:hover { border-color: #a855f7; color: #c084fc; }
.btn-yaml:hover { border-color: #06b6d4; color: #22d3ee; }
.btn-details:hover { border-color: #10b981; color: #34d399; }
.btn-delete:hover { border-color: #f43f5e; color: #fb7185; background: rgba(244, 63, 94, 0.15); }
.font-mono { font-family: var(--font-mono, monospace); }
.font-bold { font-weight: 700; }
.text-cyan { color: #06b6d4; }
.text-emerald { color: #10b981; }
.text-muted { color: var(--text-muted, #64748b); }
.text-right { text-align: right; }
</style>

