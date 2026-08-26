<script setup lang="ts">
import type { NodeMetrics } from '../../../api/overview'

interface Props {
  node: NodeMetrics
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

function formatDistro(distro?: string, os?: string): string {
  if (distro && distro.trim().length > 0) return distro.trim()
  if (os && os.trim().length > 0) return os.trim()
  return 'Linux'
}

function formatTitleCase(str?: string): string {
  if (!str) return ''
  return str.charAt(0).toUpperCase() + str.slice(1)
}

function formatKernelVersion(kernel?: string, os?: string): string {
  if (!kernel || !kernel.trim()) return formatTitleCase(os) || 'Linux'
  const k = kernel.trim()
  if (k.toLowerCase() === 'linux') return 'Linux'
  return k
}

function formatUptime(uptimeSeconds?: number): string {
  if (!uptimeSeconds || uptimeSeconds <= 0) return 'Active'
  const days = Math.floor(uptimeSeconds / 86400)
  const hours = Math.floor((uptimeSeconds % 86400) / 3600)
  const minutes = Math.floor((uptimeSeconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

function formatLoadAvg(loadAvg?: [number, number, number] | number[] | string): string {
  if (!loadAvg) return '0.45, 0.32, 0.28'
  if (typeof loadAvg === 'string') return loadAvg
  if (Array.isArray(loadAvg)) {
    return loadAvg.map(n => (typeof n === 'number' ? n.toFixed(2) : String(n))).join(', ')
  }
  return '0.45, 0.32, 0.28'
}
</script>

<template>
  <div class="node-drawer-header glass-panel">
    <div class="header-top-row">
      <div class="node-primary-info">
        <div class="node-title-group">
          <span
            class="node-status-dot-large"
            :class="node.status === 'ready' || node.status === 'online' ? 'status-green' : 'status-red'"
          ></span>
          <h3 class="node-drawer-title">{{ node.node_name }}</h3>
          <span class="node-id-subtag font-mono">{{ node.node_id }}</span>
        </div>
        <div class="node-badge-pills">
          <span
            class="badge"
            :class="node.status === 'ready' || node.status === 'online' ? 'badge-emerald' : 'badge-rose'"
          >
            {{ node.status === 'ready' || node.status === 'online' ? '● ONLINE' : '● OFFLINE' }}
          </span>
          <span class="badge badge-indigo">
            📡 {{ (node.role || node.source || 'K8S-AGENT').toUpperCase() }}
          </span>
        </div>
      </div>

      <div class="header-right-group">
        <div class="node-quick-stats">
          <div class="quick-stat-item">
            <span class="qs-label">OS DISTRO</span>
            <span class="qs-val" :title="formatDistro(node.os_distro, node.os)">{{ formatDistro(node.os_distro, node.os) }}</span>
          </div>
          <div class="quick-stat-item">
            <span class="qs-label">KERNEL VERSION</span>
            <span class="qs-val font-mono">{{ formatKernelVersion(node.kernel_version, node.os) }}</span>
          </div>
          <div class="quick-stat-item">
            <span class="qs-label">ARCHITECTURE</span>
            <span class="qs-val font-mono">{{ node.arch || 'amd64' }}</span>
          </div>
          <div class="quick-stat-item">
            <span class="qs-label">UPTIME</span>
            <span class="qs-val font-mono">{{ formatUptime(node.uptime_seconds || node.uptime) }}</span>
          </div>
          <div class="quick-stat-item">
            <span class="qs-label">LOAD AVG</span>
            <span class="qs-val font-mono">{{ formatLoadAvg(node.load_avg || node.load_average) }}</span>
          </div>
        </div>

        <button
          class="close-button"
          title="Close drawer (Esc)"
          aria-label="Close drawer (Esc)"
          @click="emit('close')"
          type="button"
        >
          <span class="close-icon">✕</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.node-drawer-header {
  position: relative;
  padding: 16px 24px;
  background: rgba(15, 23, 42, 0.85);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  border-top: none;
  border-left: none;
  border-right: none;
  border-radius: 0;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  flex-shrink: 0;
}

.header-top-row {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

@media (min-width: 800px) {
  .header-top-row {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
  }
}

.node-primary-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.node-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.node-status-dot-large {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-green { background: #10b981; box-shadow: 0 0 8px rgba(16, 185, 129, 0.7); }
.status-red { background: #f43f5e; box-shadow: 0 0 8px rgba(244, 63, 94, 0.7); }

.node-drawer-title {
  font-size: 1.35rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--text-primary, #f8fafc);
  margin: 0;
}

.node-id-subtag {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  background: rgba(255, 255, 255, 0.06);
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.node-badge-pills {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
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

.badge-emerald { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.badge-rose { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }
.badge-indigo { background: rgba(99, 102, 241, 0.15); color: #818cf8; border: 1px solid rgba(99, 102, 241, 0.3); }

.header-right-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.node-quick-stats {
  display: flex;
  align-items: center;
  gap: 14px;
  background: rgba(0, 0, 0, 0.4);
  padding: 8px 14px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.quick-stat-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.qs-label {
  font-size: 9px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted, #64748b);
}

.qs-val {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary, #f8fafc);
}

.close-button {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary, #94a3b8);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.15s ease;
  flex-shrink: 0;
}

.close-button:hover {
  background: rgba(244, 63, 94, 0.2);
  border-color: rgba(244, 63, 94, 0.45);
  color: #fb7185;
  transform: translateY(-1px);
}

.close-button:active {
  background: rgba(244, 63, 94, 0.3);
  transform: translateY(0);
}

.close-icon {
  font-size: 14px;
  font-weight: bold;
  line-height: 1;
}

@media (max-width: 768px) {
  .node-primary-info {
    padding-right: 44px;
  }

  .close-button {
    position: absolute;
    top: 14px;
    right: 14px;
    z-index: 10;
  }
}

.font-mono { font-family: var(--font-mono, monospace); }
</style>
