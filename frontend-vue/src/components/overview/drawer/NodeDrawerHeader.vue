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
  if (!loadAvg) return '--'
  if (typeof loadAvg === 'string') return loadAvg
  if (Array.isArray(loadAvg)) {
    return loadAvg.map(n => (typeof n === 'number' ? n.toFixed(2) : String(n))).join(', ')
  }
  return '--'
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
@import '../../../assets/styles/components/node-diagnostics.css';
</style>
