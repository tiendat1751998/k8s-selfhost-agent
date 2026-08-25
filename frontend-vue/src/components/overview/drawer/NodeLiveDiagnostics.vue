<script setup lang="ts">
import { ref, computed } from 'vue'
import type { NodeMetrics, SystemOverview, TpsSnapshot, TpsServiceMetrics } from '../../../api/overview'

export type ProcessCategoryType = 'container' | 'host_app' | 'system'

export interface UnifiedProcessItem {
  pid: number | string
  name: string
  command_line?: string
  user?: string
  cpu_percent: number
  memory_bytes: number
  memory_percent?: number
  requests_per_sec: number
  error_rate: number
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  state?: string
  priority?: number | string
  is_container?: boolean
  is_app?: boolean
  app_type?: ProcessCategoryType
  container_name?: string
}

interface Props {
  node: NodeMetrics
  overview?: SystemOverview | null
  tpsData?: TpsSnapshot | null
}

const props = withDefaults(defineProps<Props>(), {
  overview: null,
  tpsData: null,
})

// Process Category & Sorting State
const processCategoryFilter = ref<'all' | 'container' | 'host_daemon' | 'kernel'>('all')
const processSearch = ref('')
const processSortBy = ref<'cpu' | 'mem' | 'rps' | 'bandwidth' | 'pid' | 'name'>('cpu')
const processSortOrder = ref<'asc' | 'desc'>('desc')
const processPageSize = ref<number>(25)
const processCurrentPage = ref<number>(1)

const filteredNodeInterfaces = computed(() => {
  return props.node.network_interfaces || []
})

const filteredNodeDiskDevices = computed(() => {
  return props.node.disk_devices || []
})

// Classify process helper
function classifyProcessType(p: { name?: string; command_line?: string; user?: string; is_container?: boolean }): ProcessCategoryType {
  if (p.is_container) return 'container'

  const name = (p.name || '').toLowerCase()
  const cmd = (p.command_line || '').toLowerCase()
  const user = (p.user || '').toLowerCase()

  // 1. Definite OS Kernel threads: [rcu_preempt], [ksoftirqd], etc.
  if (name.startsWith('[') || name.endsWith(']') || cmd.startsWith('[') || cmd.endsWith(']')) {
    return 'system'
  }

  // 2. Definite Core OS system init & maintenance daemons
  const coreOsSystemDaemons = [
    'systemd', 'journald', 'udevd', 'resolved', 'timesyncd', 'logind', 'dbus-daemon',
    'polkitd', 'accounts-daemon', 'cron', 'crond', 'rsyslogd', 'agetty', 'login',
    'irqbalance', 'multipathd', 'thermald', 'snapd', 'unattended-upgr', 'packagekitd',
    'auditd', 'acpid', 'atd', 'smartd'
  ]
  if (coreOsSystemDaemons.some(d => name === d || name.startsWith(d) || name.endsWith(d))) {
    return 'system'
  }

  // 3. User-space non-root application
  if (user && user !== 'root' && user !== 'daemon' && user !== 'sys') {
    return 'host_app'
  }

  // 4. Known App & Platform service binaries
  const knownAppKeywords = [
    'k8s-agent', 'k8s-master', 'k8s-controller', 'hermes', 'proxysql', 'dockerd', 'containerd',
    'traefik', 'redis', 'nats', 'postgres', 'pgsql', 'mysql', 'mariadb', 'nginx', 'caddy',
    'envoy', 'prometheus', 'grafana', 'vector', 'fluent', 'loki', 'alloy', 'vault', 'consul',
    'etcd', 'kubelet', 'kube-proxy', 'node', 'python', 'java', 'golang', 'ruby', 'php',
    'uvicorn', 'gunicorn', 'vite', 'cargo', 'dotnet', 'celery'
  ]
  if (knownAppKeywords.some(k => name.includes(k) || cmd.includes(k))) {
    return 'host_app'
  }

  // 5. Command path in user home or custom app directories
  if (cmd.includes('/home/') || cmd.includes('/opt/') || cmd.includes('/srv/') || cmd.includes('/app/') || cmd.includes('/usr/local/')) {
    return 'host_app'
  }

  return 'system'
}

// 1. Raw list of services running on this node
const rawNodeServices = computed<TpsServiceMetrics[]>(() => {
  if (!props.node || !props.tpsData?.services) return []
  const nodeId = (props.node.node_id || '').toLowerCase().trim()
  const nodeName = (props.node.node_name || '').toLowerCase().trim()
  const isSingleNode = (props.overview?.nodes?.length || 0) === 1

  return props.tpsData.services.filter(s => {
    const sNodeId = (s.node_id || '').toLowerCase().trim()
    const sNodeName = (s.node_name || '').toLowerCase().trim()

    if (sNodeId && (sNodeId === nodeId || sNodeId === nodeName)) return true
    if (sNodeName && (sNodeName === nodeName || sNodeName === nodeId)) return true
    if (sNodeName && nodeName) {
      if (nodeName === sNodeName || nodeName.includes(sNodeName) || sNodeName.includes(nodeName)) return true
      const normS = sNodeName.replace(/^(k8s|node-)/, '')
      const normN = nodeName.replace(/^(k8s|node-)/, '')
      if (normS && normN && (normS === normN || normS.includes(normN) || normN.includes(normS))) return true
    }
    if (isSingleNode && (!s.node_id || sNodeId === nodeId || sNodeId === nodeName)) return true
    return false
  })
})

// Unified processes + container workloads for this node
const unifiedNodeProcesses = computed<UnifiedProcessItem[]>(() => {
  const procs = props.node?.top_processes || []
  const services = rawNodeServices.value || []
  const matchedServices = new Set<string>()

  const list: UnifiedProcessItem[] = procs.map(p => {
    const pName = (p.name || '').toLowerCase()
    const pCmd = (p.command_line || '').toLowerCase()

    // Find matching container service
    const matchedSvc = services.find(s => {
      const sName = (s.service_name || '').toLowerCase()
      const sClean = sName.replace(/^(tiki_|k8s_|docker_)/, '')
      return (
        sName === pName ||
        pName.includes(sClean) ||
        pCmd.includes(sName) ||
        pCmd.includes(sClean) ||
        (sClean === 'redis' && (pName.includes('redis') || pCmd.includes('redis'))) ||
        (sClean === 'traefik' && (pName.includes('traefik') || pCmd.includes('traefik'))) ||
        (sClean === 'nats' && (pName.includes('nats') || pCmd.includes('nats'))) ||
        ((sClean === 'postgres' || sClean === 'db') && (pName.includes('postgres') || pCmd.includes('postgres')))
      )
    })

    if (matchedSvc) {
      matchedServices.add(matchedSvc.service_name)
      return {
        pid: p.pid,
        name: matchedSvc.service_name,
        command_line: p.command_line || p.name,
        user: p.user || 'root',
        cpu_percent: Math.max(p.cpu_percent || 0, matchedSvc.cpu_percent || 0),
        memory_bytes: Math.max(p.memory_bytes || 0, (matchedSvc.memory_used_mb || 0) * 1024 * 1024),
        memory_percent: p.memory_percent,
        requests_per_sec: matchedSvc.requests_per_sec || 0,
        error_rate: matchedSvc.error_rate || 0,
        rx_bytes_per_sec: matchedSvc.rx_bytes_per_sec || p.read_bytes_per_sec || 0,
        tx_bytes_per_sec: matchedSvc.tx_bytes_per_sec || p.write_bytes_per_sec || 0,
        state: matchedSvc.status === 'healthy' || matchedSvc.status === 'running' ? 'healthy' : (p.state || 'running'),
        is_container: true,
        is_app: true,
        app_type: 'container',
        container_name: matchedSvc.service_name,
      }
    }

    const appType = classifyProcessType({ name: p.name, command_line: p.command_line, user: p.user, is_container: false })
    return {
      pid: p.pid,
      name: p.name,
      command_line: p.command_line,
      user: p.user || 'root',
      cpu_percent: p.cpu_percent || 0,
      memory_bytes: p.memory_bytes || 0,
      memory_percent: p.memory_percent,
      requests_per_sec: 0,
      error_rate: 0,
      rx_bytes_per_sec: p.read_bytes_per_sec || 0,
      tx_bytes_per_sec: p.write_bytes_per_sec || 0,
      state: p.state || 'running',
      is_container: false,
      is_app: appType !== 'system',
      app_type: appType,
    }
  })

  // Append any container services that didn't match an OS PID
  for (const s of services) {
    if (!matchedServices.has(s.service_name)) {
      list.unshift({
        pid: 'CTR',
        name: s.service_name,
        command_line: `Container Service: ${s.service_name}`,
        user: 'docker',
        cpu_percent: s.cpu_percent || 0,
        memory_bytes: (s.memory_used_mb || 0) * 1024 * 1024,
        requests_per_sec: s.requests_per_sec || 0,
        error_rate: s.error_rate || 0,
        rx_bytes_per_sec: s.rx_bytes_per_sec || 0,
        tx_bytes_per_sec: s.tx_bytes_per_sec || 0,
        state: s.status || 'healthy',
        is_container: true,
        is_app: true,
        app_type: 'container',
        container_name: s.service_name,
      })
    }
  }

  return list
})

const processCategoryCounts = computed(() => {
  const all = unifiedNodeProcesses.value
  let container = 0
  let hostDaemon = 0
  let kernel = 0
  for (const p of all) {
    if (p.app_type === 'container') container++
    else if (p.app_type === 'host_app') hostDaemon++
    else kernel++
  }
  return { total: all.length, container, hostDaemon, kernel }
})

// Filtered and sorted processes for this node
const filteredNodeProcesses = computed<UnifiedProcessItem[]>(() => {
  let list = [...unifiedNodeProcesses.value]

  // Category filter
  if (processCategoryFilter.value === 'container') {
    list = list.filter(p => p.app_type === 'container')
  } else if (processCategoryFilter.value === 'host_daemon') {
    list = list.filter(p => p.app_type === 'host_app')
  } else if (processCategoryFilter.value === 'kernel') {
    list = list.filter(p => p.app_type === 'system')
  }

  // Search text filter
  if (processSearch.value.trim()) {
    const q = processSearch.value.toLowerCase().trim()
    list = list.filter(p =>
      p.name.toLowerCase().includes(q) ||
      (p.command_line && p.command_line.toLowerCase().includes(q)) ||
      (p.user && p.user.toLowerCase().includes(q)) ||
      String(p.pid).toLowerCase().includes(q)
    )
  }

  const isAsc = processSortOrder.value === 'asc'
  const multiplier = isAsc ? 1 : -1

  if (processSortBy.value === 'cpu') {
    list.sort((a, b) => ((a.cpu_percent || 0) - (b.cpu_percent || 0)) * multiplier)
  } else if (processSortBy.value === 'mem') {
    list.sort((a, b) => ((a.memory_bytes || 0) - (b.memory_bytes || 0)) * multiplier)
  } else if (processSortBy.value === 'rps') {
    list.sort((a, b) => ((a.requests_per_sec || 0) - (b.requests_per_sec || 0)) * multiplier)
  } else if (processSortBy.value === 'bandwidth') {
    list.sort((a, b) => {
      const aBw = (a.rx_bytes_per_sec || 0) + (a.tx_bytes_per_sec || 0)
      const bBw = (b.rx_bytes_per_sec || 0) + (b.tx_bytes_per_sec || 0)
      return (aBw - bBw) * multiplier
    })
  } else if (processSortBy.value === 'pid') {
    list.sort((a, b) => {
      const pA = typeof a.pid === 'number' ? a.pid : 999999
      const pB = typeof b.pid === 'number' ? b.pid : 999999
      return (pA - pB) * multiplier
    })
  } else if (processSortBy.value === 'name') {
    list.sort((a, b) => a.name.localeCompare(b.name) * multiplier)
  }

  return list
})

const totalProcessPages = computed(() => {
  return Math.ceil(filteredNodeProcesses.value.length / processPageSize.value) || 1
})

const paginatedNodeProcesses = computed(() => {
  const start = (processCurrentPage.value - 1) * processPageSize.value
  return filteredNodeProcesses.value.slice(start, start + processPageSize.value)
})

function getProcessCpuColor(cpu?: number): string {
  if (!cpu) return 'slate'
  if (cpu >= 70) return 'rose'
  if (cpu >= 40) return 'amber'
  if (cpu >= 15) return 'cyan'
  return 'slate'
}

function getUnifiedProcessState(proc: UnifiedProcessItem): { label: string; class: string } {
  const s = (proc.state || '').toLowerCase()
  if (s === 'healthy' || s === 'running' || s === 'r') {
    return { label: 'RUNNING', class: 'badge-emerald' }
  }
  if (s === 'sleeping' || s === 's') {
    return { label: 'SLEEP', class: 'badge-indigo' }
  }
  if (s === 'zombie' || s === 'z') {
    return { label: 'ZOMBIE', class: 'badge-rose' }
  }
  if (s === 'disk_sleep' || s === 'd') {
    return { label: 'D-SLEEP', class: 'badge-amber' }
  }
  if (s === 'stopped' || s === 't') {
    return { label: 'STOPPED', class: 'badge-slate' }
  }
  return { label: s.toUpperCase() || 'ACTIVE', class: 'badge-cyan' }
}

function getProcessOriginBadge(proc: UnifiedProcessItem): { label: string; class: string; icon: string } {
  if (proc.app_type === 'container') {
    return { label: 'CONTAINER', class: 'tag-container', icon: '📦' }
  }
  if (proc.app_type === 'host_app') {
    return { label: 'HOST APP', class: 'tag-host-daemon', icon: '⚙️' }
  }
  return { label: 'KERNEL', class: 'tag-kernel', icon: '🐧' }
}

function getUtilizationColor(pct: number): string {
  if (pct >= 85) return 'rose'
  if (pct >= 65) return 'amber'
  return 'emerald'
}

function formatPercent(val: number): string {
  return `${Math.round(val)}%`
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
        <div class="drawer-tab-content">
          <!-- Hardware Saturation Telemetry -->
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
                    {{ formatPercent(node.cpu_percent) }}
                  </span>
                </div>
                <div class="hw-progress-track">
                  <div
                    class="hw-progress-fill smooth-bar"
                    :class="`bg-${getUtilizationColor(node.cpu_percent || 0)}`"
                    :style="{ width: `${Math.min(100, node.cpu_percent || 0)}%` }"
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
                    {{ formatPercent(node.memory_percent) }}
                  </span>
                </div>
                <div class="hw-progress-track">
                  <div
                    class="hw-progress-fill smooth-bar"
                    :class="`bg-${getUtilizationColor(node.memory_percent || 0)}`"
                    :style="{ width: `${Math.min(100, node.memory_percent || 0)}%` }"
                  ></div>
                </div>
                <div class="hw-gauge-sub">
                  <span>{{ formatBytes(node.memory_used || 0) }} / {{ formatBytes(node.memory_total || 0) }}</span>
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
                    {{ formatPercent(node.disk_percent) }}
                  </span>
                </div>
                <div class="hw-progress-track">
                  <div
                    class="hw-progress-fill smooth-bar"
                    :class="`bg-${getUtilizationColor(node.disk_percent || 0)}`"
                    :style="{ width: `${Math.min(100, node.disk_percent || 0)}%` }"
                  ></div>
                </div>
                <div class="hw-gauge-sub">
                  <span>{{ formatBytes(node.disk_used || 0) }} / {{ formatBytes(node.disk_total || 0) }}</span>
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
                  <span class="text-cyan font-mono font-bold" :title="'Real-time Download Rate (Rx)'">↓ {{ formatIoRate(node.network_rx_bytes) }}</span>
                  <span class="text-purple font-mono font-bold" :title="'Real-time Upload Rate (Tx)'">↑ {{ formatIoRate(node.network_tx_bytes) }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Network Interfaces Section (Compact 4-Card Row) -->
          <div class="network-interfaces-section" v-if="filteredNodeInterfaces.length > 0">
            <div class="hud-section-header">
              <h4 class="hud-section-heading">
                <span class="heading-icon">📡</span> Network Interface Throughput
              </h4>
              <span class="badge badge-cyan font-mono">{{ filteredNodeInterfaces.length }} {{ filteredNodeInterfaces.length === 1 ? 'interface' : 'interfaces' }}</span>
            </div>
            <div class="interfaces-grid">
              <div
                v-for="iface in filteredNodeInterfaces"
                :key="iface.name"
                class="iface-card glass-panel"
              >
                <div class="iface-top">
                  <div class="iface-name-group">
                    <span class="iface-icon">🌐</span>
                    <span class="iface-name font-mono" :title="iface.name">{{ iface.name }}</span>
                  </div>
                  <span class="iface-nic-tag font-mono">NIC</span>
                </div>
                <div class="iface-rates-row font-mono">
                  <span class="iface-rate-item text-cyan" :title="'Download Rate (Rx): ' + formatIoRate(iface.rx_bytes_per_sec)">
                    <span class="rate-arrow">↓</span> {{ formatIoRate(iface.rx_bytes_per_sec) }}
                  </span>
                  <span class="iface-rate-sep">·</span>
                  <span class="iface-rate-item text-purple" :title="'Upload Rate (Tx): ' + formatIoRate(iface.tx_bytes_per_sec)">
                    <span class="rate-arrow">↑</span> {{ formatIoRate(iface.tx_bytes_per_sec) }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- Disk I/O & Block Device Telemetry Section -->
          <div class="disk-io-section">
            <div class="hud-section-header">
              <h4 class="hud-section-heading">
                <span class="heading-icon">💽</span> Disk I/O & Block Device Telemetry
              </h4>
              <span class="badge badge-indigo font-mono" v-if="filteredNodeDiskDevices.length > 0">
                {{ filteredNodeDiskDevices.length }} {{ filteredNodeDiskDevices.length === 1 ? 'device' : 'devices' }}
              </span>
              <span class="badge badge-slate font-mono" v-else>
                HOST TELEMETRY
              </span>
            </div>

            <!-- Summary Strip -->
            <div class="disk-summary-strip glass-panel">
              <div class="disk-summary-item" title="Physical Disk Read Throughput">
                <span class="disk-summary-label">📖 READ THROUGHPUT</span>
                <span class="disk-summary-val font-mono text-cyan font-bold">
                  {{ formatBytes(node.disk_read_bytes_per_sec || 0) }}/s
                </span>
              </div>
              <div class="disk-summary-item" title="Physical Disk Write Throughput">
                <span class="disk-summary-label">✍️ WRITE THROUGHPUT</span>
                <span class="disk-summary-val font-mono text-purple font-bold">
                  {{ formatBytes(node.disk_write_bytes_per_sec || 0) }}/s
                </span>
              </div>
              <div class="disk-summary-item" title="Total Physical Read & Write IOPS">
                <span class="disk-summary-label">⚡ TOTAL IOPS</span>
                <span class="disk-summary-val font-mono text-emerald font-bold">
                  {{ (node.disk_read_iops || 0).toFixed(0) }} R · {{ (node.disk_write_iops || 0).toFixed(0) }} W
                </span>
              </div>
              <div class="disk-summary-item" title="Average Total Latency Per I/O Operation (await)">
                <span class="disk-summary-label">⏱️ AVG LATENCY (await)</span>
                <span
                  class="disk-summary-val font-mono font-bold"
                  :class="(node.disk_avg_await_ms || 0) > 30 ? 'text-rose' : (node.disk_avg_await_ms || 0) > 10 ? 'text-amber' : 'text-emerald'"
                >
                  {{ (node.disk_avg_await_ms || 0).toFixed(1) }} ms
                </span>
              </div>
              <div class="disk-summary-item" title="Maximum Block Device I/O Saturation Percentage">
                <span class="disk-summary-label">📊 MAX %UTIL</span>
                <span
                  class="disk-summary-val font-mono font-bold"
                  :class="`text-${getUtilizationColor(node.disk_max_io_util_pct || 0)}`"
                >
                  {{ (node.disk_max_io_util_pct || 0).toFixed(1) }}%
                </span>
              </div>
            </div>

            <!-- Block Devices Table / Micro-Cards -->
            <div class="disk-devices-wrapper" v-if="filteredNodeDiskDevices.length > 0">
              <div class="disk-devices-grid">
                <div
                  v-for="dev in filteredNodeDiskDevices"
                  :key="dev.device_name"
                  class="disk-device-card glass-panel"
                >
                  <div class="dev-card-header">
                    <div class="dev-name-group">
                      <span class="dev-icon">💾</span>
                      <span class="dev-name font-mono font-bold" :title="dev.device_name">{{ dev.device_name }}</span>
                    </div>
                    <span
                      class="dev-badge font-mono font-bold"
                      :class="dev.is_root_device ? 'badge-root' : 'badge-part'"
                      :title="dev.is_root_device ? 'Root Partition / Primary Device' : 'Storage Device / Partition'"
                    >
                      {{ dev.is_root_device ? 'ROOT' : 'PART' }}
                    </span>
                  </div>

                  <div class="dev-metrics-grid font-mono">
                    <div class="dev-metric-col" title="Read Throughput Rate">
                      <span class="dev-metric-label">READ RATE</span>
                      <span class="dev-metric-val text-cyan font-bold">📖 {{ formatIoRate(dev.read_bytes_per_sec) }}</span>
                    </div>
                    <div class="dev-metric-col" title="Write Throughput Rate">
                      <span class="dev-metric-label">WRITE RATE</span>
                      <span class="dev-metric-val text-purple font-bold">✍️ {{ formatIoRate(dev.write_bytes_per_sec) }}</span>
                    </div>
                    <div class="dev-metric-col" title="IOPS (Read / Write)">
                      <span class="dev-metric-label">IOPS (R/W)</span>
                      <span class="dev-metric-val text-emerald font-bold">{{ dev.read_iops.toFixed(0) }} R / {{ dev.write_iops.toFixed(0) }} W</span>
                    </div>
                    <div class="dev-metric-col" title="Average Wait Latency (await)">
                      <span class="dev-metric-label">AWAIT LATENCY</span>
                      <span
                        class="dev-metric-val font-bold"
                        :class="dev.avg_wait_ms > 30 ? 'text-rose' : dev.avg_wait_ms > 10 ? 'text-amber' : 'text-emerald'"
                      >
                        {{ dev.avg_wait_ms.toFixed(1) }} ms
                      </span>
                    </div>
                    <div class="dev-metric-col" title="Average Request Size">
                      <span class="dev-metric-label">AVG REQ SZ</span>
                      <span class="dev-metric-val text-slate font-bold">{{ dev.avg_req_size_kb.toFixed(1) }} KB</span>
                    </div>
                    <div class="dev-metric-col" title="Current Queue Depth">
                      <span class="dev-metric-label">QUEUE DEPTH</span>
                      <span
                        class="dev-metric-val font-bold"
                        :class="dev.current_queue_depth > 10 ? 'text-rose' : dev.current_queue_depth > 4 ? 'text-amber' : 'text-slate'"
                      >
                        {{ dev.current_queue_depth }}
                      </span>
                    </div>
                  </div>

                  <!-- %util Progress Track -->
                  <div class="dev-util-row">
                    <div class="dev-util-label-group">
                      <span class="dev-util-label">DEVICE I/O UTILIZATION (%UTIL)</span>
                      <span
                        class="dev-util-pct font-mono font-bold"
                        :class="`text-${getUtilizationColor(dev.io_utilization_pct || 0)}`"
                      >
                        {{ (dev.io_utilization_pct || 0).toFixed(1) }}%
                      </span>
                    </div>
                    <div class="hw-progress-track">
                      <div
                        class="hw-progress-fill smooth-bar"
                        :class="`bg-${getUtilizationColor(dev.io_utilization_pct || 0)}`"
                        :style="{ width: `${Math.min(100, Math.max(0, dev.io_utilization_pct || 0))}%` }"
                      ></div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Fallback when no devices reported -->
            <div class="disk-fallback-panel glass-panel" v-else>
              <div class="fallback-content">
                <span class="fallback-icon">💾</span>
                <div class="fallback-info">
                  <span class="fallback-title font-mono">Block Device Breakdown Unavailable</span>
                  <span class="fallback-subtitle text-muted">
                    Displaying host-level aggregate storage metrics. Individual block device breakdown is available when reported by node agent.
                  </span>
                </div>
                <div class="fallback-rates font-mono">
                  <span class="rate-pill text-cyan">📖 {{ formatIoRate(node.disk_read_bytes_per_sec || 0) }} Read</span>
                  <span class="rate-pill text-purple">✍️ {{ formatIoRate(node.disk_write_bytes_per_sec || 0) }} Write</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Unified Top Resource & Workload Processes -->
          <div class="node-processes-section">
            <div class="proc-header-row">
              <div class="proc-title-group">
                <h4 class="proc-section-heading">🔥 Top Resource-Consuming Apps & Processes</h4>
                <span class="badge badge-indigo font-mono" v-if="filteredNodeProcesses.length > 0">
                  {{ filteredNodeProcesses.length }} active
                </span>
              </div>

              <!-- Top Category Filter Chips -->
              <div class="service-filter-chips proc-category-chips">
                <button
                  class="chip-btn"
                  :class="{ active: processCategoryFilter === 'all' }"
                  @click="processCategoryFilter = 'all'; processCurrentPage = 1"
                >
                  All ({{ processCategoryCounts.total }})
                </button>
                <button
                  class="chip-btn chip-healthy"
                  :class="{ active: processCategoryFilter === 'container' }"
                  @click="processCategoryFilter = 'container'; processCurrentPage = 1"
                >
                  📦 Workloads ({{ processCategoryCounts.container }})
                </button>
                <button
                  class="chip-btn chip-traffic"
                  :class="{ active: processCategoryFilter === 'host_daemon' }"
                  @click="processCategoryFilter = 'host_daemon'; processCurrentPage = 1"
                >
                  ⚡ Host Daemons ({{ processCategoryCounts.hostDaemon }})
                </button>
                <button
                  class="chip-btn chip-degraded"
                  :class="{ active: processCategoryFilter === 'kernel' }"
                  @click="processCategoryFilter = 'kernel'; processCurrentPage = 1"
                >
                  ⚙️ OS Kernel ({{ processCategoryCounts.kernel }})
                </button>
              </div>
            </div>
            <div class="proc-controls">
              <div class="proc-search-box wide-search">
                <span class="search-icon">🔍</span>
                <input
                  v-model="processSearch"
                  type="text"
                  placeholder="Search processes by name, PID, user, command line..."
                  class="input-proc-search"
                  @input="processCurrentPage = 1"
                />
                <button v-if="processSearch" class="btn-clear-search" @click="processSearch = ''; processCurrentPage = 1">✕</button>
              </div>

              <div class="proc-sort-group">
                <select v-model="processSortBy" class="select-proc-sort" @change="processCurrentPage = 1">
                  <option value="cpu">Sort: CPU % (High to Low)</option>
                  <option value="mem">Sort: Memory</option>
                  <option value="rps">Sort: Ingress Req/s</option>
                  <option value="bandwidth">Sort: Bandwidth (Rx/Tx)</option>
                  <option value="name">Sort: App Name</option>
                  <option value="pid">Sort: PID</option>
                </select>
              </div>
            </div>

            <div class="processes-table-wrapper" v-if="filteredNodeProcesses.length > 0">
              <table class="processes-table">
                <thead>
                  <tr>
                    <th class="th-pid">PID</th>
                    <th class="th-app">App / Command</th>
                    <th class="th-user">User</th>
                    <th class="th-cpu">CPU %</th>
                    <th class="th-mem">Memory</th>
                    <th class="th-rps">Req/s</th>
                    <th class="th-err">Err %</th>
                    <th class="th-bw">Bandwidth</th>
                    <th class="th-state">State</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="proc in paginatedNodeProcesses"
                    :key="proc.pid + '-' + proc.name"
                    class="proc-row"
                    :class="{ 'proc-row-hot': proc.cpu_percent >= 70, 'proc-row-container': proc.is_container }"
                  >
                    <td class="col-proc-pid">
                      <span class="pid-tag font-mono" :class="{ 'pid-tag-ctr': proc.pid === 'CTR' }">
                        {{ typeof proc.pid === 'number' ? '#' + proc.pid : proc.pid }}
                      </span>
                    </td>
                    <td class="col-proc-app">
                      <div class="proc-app-info" :title="proc.command_line ? `${proc.name}\nType: ${getProcessOriginBadge(proc).label}\nCommand: ${proc.command_line}` : proc.name">
                        <div class="proc-name-row">
                          <span class="proc-name">{{ proc.name }}</span>
                          <span class="proc-origin-tag font-mono" :class="getProcessOriginBadge(proc).class">
                            {{ getProcessOriginBadge(proc).icon }} {{ getProcessOriginBadge(proc).label }}
                          </span>
                        </div>
                        <span class="proc-cmd-line font-mono" v-if="proc.command_line">
                          {{ proc.command_line }}
                        </span>
                      </div>
                    </td>
                    <td class="col-proc-user">
                      <span class="user-badge font-mono">{{ proc.user || 'root' }}</span>
                    </td>
                    <td class="col-proc-cpu">
                      <div class="proc-cpu-box">
                        <div class="proc-cpu-val-row">
                          <span class="proc-cpu-val smooth-value" :class="`text-${getProcessCpuColor(proc.cpu_percent)}`">
                            {{ formatPercent(proc.cpu_percent) }}
                          </span>
                          <span v-if="proc.cpu_percent >= 70" class="badge badge-rose badge-hot-pulse">
                            HOT
                          </span>
                        </div>
                        <div class="proc-mini-track">
                          <div
                            class="proc-mini-fill smooth-bar"
                            :class="`bg-${getProcessCpuColor(proc.cpu_percent)}`"
                            :style="{ width: `${Math.min(100, proc.cpu_percent)}%` }"
                          ></div>
                        </div>
                      </div>
                    </td>
                    <td class="col-proc-mem">
                      <div class="proc-mem-box">
                        <span class="proc-mem-val smooth-value">{{ formatBytes(proc.memory_bytes) }}</span>
                        <span class="proc-mem-pct font-mono" v-if="proc.memory_percent">
                          ({{ formatPercent(proc.memory_percent) }})
                        </span>
                      </div>
                    </td>
                    <td class="col-proc-rps font-mono text-emerald">
                      <span v-if="proc.requests_per_sec > 0">
                        ⚡ {{ proc.requests_per_sec.toLocaleString() }}
                      </span>
                      <span v-else class="text-slate opacity-40">0</span>
                    </td>
                    <td class="col-proc-err font-mono">
                      <span v-if="proc.error_rate > 0" class="text-rose font-bold">
                        {{ proc.error_rate.toFixed(1) }}%
                      </span>
                      <span v-else class="text-slate opacity-40">0.0%</span>
                    </td>
                    <td class="col-proc-bw font-mono text-slate">
                      <span class="bw-split">
                        <span class="bw-rx text-cyan" :title="'Real-time Download / Read: ' + formatIoRate(proc.rx_bytes_per_sec)">↓ {{ formatIoRate(proc.rx_bytes_per_sec) }}</span>
                        <span class="bw-tx text-purple" :title="'Real-time Upload / Write: ' + formatIoRate(proc.tx_bytes_per_sec)">↑ {{ formatIoRate(proc.tx_bytes_per_sec) }}</span>
                      </span>
                    </td>
                    <td class="col-proc-state">
                      <span class="badge font-mono" :class="getUnifiedProcessState(proc).class">
                        {{ getUnifiedProcessState(proc).label }}
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Unified Page Size & Pagination Footer -->
            <div class="proc-pagination" v-if="filteredNodeProcesses.length > 0">
              <div class="pagination-info">
                Showing
                <span class="text-cyan font-mono font-bold">{{ filteredNodeProcesses.length === 0 ? 0 : (processCurrentPage - 1) * processPageSize + 1 }}–{{ Math.min(processCurrentPage * processPageSize, filteredNodeProcesses.length) }}</span>
                of
                <span class="text-slate font-mono font-bold">{{ filteredNodeProcesses.length }}</span>
                processes & workloads
              </div>

              <!-- Page Size Selector -->
              <div class="page-size-group">
                <span class="page-size-label font-mono">PAGE SIZE:</span>
                <div class="page-size-pill">
                  <button
                    v-for="size in [10, 25, 50, 100]"
                    :key="size"
                    class="btn-page-size font-mono"
                    :class="{ active: processPageSize === size }"
                    @click="processPageSize = size; processCurrentPage = 1"
                  >
                    {{ size }}
                  </button>
                </div>
              </div>

              <div class="pagination-actions" v-if="totalProcessPages > 1">
                <button
                  class="btn-page"
                  :disabled="processCurrentPage <= 1"
                  @click="processCurrentPage = 1"
                  title="First Page"
                >
                  «
                </button>
                <button
                  class="btn-page"
                  :disabled="processCurrentPage <= 1"
                  @click="processCurrentPage--"
                >
                  ‹ Prev
                </button>
                <span class="page-indicator font-mono">
                  Page {{ processCurrentPage }} / {{ totalProcessPages }}
                </span>
                <button
                  class="btn-page"
                  :disabled="processCurrentPage >= totalProcessPages"
                  @click="processCurrentPage++"
                >
                  Next ›
                </button>
                <button
                  class="btn-page"
                  :disabled="processCurrentPage >= totalProcessPages"
                  @click="processCurrentPage = totalProcessPages"
                  title="Last Page"
                >
                  »
                </button>
              </div>
            </div>

            <div class="proc-empty-state glass-panel" v-else>
              <div class="radar-glow-container">
                <div class="radar-beacon"></div>
                <span class="proc-empty-icon">📡</span>
              </div>
              <h5 class="proc-empty-title">
                {{ processSearch ? 'No Processes Matching Filter' : 'No Process Telemetry Streamed' }}
              </h5>
              <p class="proc-empty-desc" v-if="processSearch">
                No active processes matched "<span class="text-cyan font-mono">{{ processSearch }}</span>". Try searching for a different process name or PID.
              </p>
              <div class="proc-empty-telemetry-hint" v-else>
                <p class="proc-empty-desc">
                  Host metrics are active, but high-resolution process inspection stream is pending agent collector broadcast.
                </p>
                <div class="troubleshooting-hint-box">
                  <div class="hint-header">
                    <span class="hint-icon">💡</span>
                    <strong class="hint-title">Troubleshooting & Activation</strong>
                  </div>
                  <ul class="hint-list">
                    <li>Verify <code>k8s-agent</code> daemon is running on this node.</li>
                    <li>Ensure agent has process collection permissions (e.g. host <code>/proc</code> mount).</li>
                    <li>Processes will populate automatically upon receiving telemetry broadcast.</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- ========================================== -->

</template>

<style scoped>
/* Hardware Telemetry HUD */
.hardware-hud-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.hud-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hud-section-heading {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  gap: 6px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin: 0;
}

.heading-icon {
  font-size: 14px;
}

.hardware-gauges-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 12px;
}

.hw-gauge-card {
  padding: 14px 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  transition: all 0.2s ease;
}

.hw-gauge-card:hover {
  border-color: rgba(56, 189, 248, 0.25);
  transform: translateY(-1px);
}

.hw-gauge-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.hw-gauge-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  white-space: nowrap;
}

.hw-gauge-value {
  font-size: 16px;
  font-weight: 800;
  font-family: var(--font-mono);
  transition: color 0.5s ease;
  white-space: nowrap;
}

.hw-net-inline-rates {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  white-space: nowrap;
}

.hw-net-inline-rates .net-sep {
  color: rgba(255, 255, 255, 0.25);
  font-weight: 700;
}

.hw-progress-track {
  width: 100%;
  height: 5px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.hw-progress-track.hw-net-track {
  background: rgba(168, 85, 247, 0.25);
}

.hw-progress-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
}

.hw-gauge-sub {
  font-size: 10.5px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.bw-split {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}

.bw-rx {
  color: #38bdf8;
  font-weight: 600;
}

.bw-tx {
  color: #c084fc;
  font-weight: 600;
}

/* Network Interface Section in Node Drawer */
.network-interfaces-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.interfaces-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}

@media (max-width: 900px) {
  .interfaces-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 600px) {
  .interfaces-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.bg-emerald { background-color: #10b981 !important; }
.bg-amber { background-color: #f59e0b !important; }
.bg-rose { background-color: #f43f5e !important; }
.bg-cyan { background-color: #06b6d4 !important; }
.bg-purple { background-color: #a855f7 !important; }
.bg-slate { background-color: #64748b !important; }

.text-emerald { color: #34d399 !important; }
.text-amber { color: #fbbf24 !important; }
.text-rose { color: #fb7185 !important; }
.text-cyan { color: #38bdf8 !important; }
.text-purple { color: #c084fc !important; }
.text-slate { color: #94a3b8 !important; }

.iface-card {
  padding: 6px 8px;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 4px;
  min-height: 46px;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.iface-card:hover {
  border-color: rgba(56, 189, 248, 0.25);
  transform: translateY(-1px);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
}

.iface-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.iface-name-group {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.iface-icon {
  font-size: 10px;
  flex-shrink: 0;
}

.iface-name {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-primary);
  max-width: 90px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.iface-nic-tag {
  font-size: 8px;
  font-weight: 700;
  letter-spacing: 0.05em;
  padding: 1px 3px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-muted);
  border: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.iface-rates-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 2px;
  font-size: 9.5px;
  font-weight: 600;
  background: rgba(0, 0, 0, 0.25);
  padding: 2px 5px;
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.04);
}

.iface-rate-item {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  white-space: nowrap;
}

.rate-arrow {
  font-weight: 800;
  font-size: 9px;
}

.iface-rate-sep {
  color: rgba(255, 255, 255, 0.2);
  font-size: 9px;
}

/* Disk I/O & Block Device Telemetry Section in Node Drawer */
.disk-io-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.disk-summary-strip {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 12px;
  padding: 10px 14px;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

@media (max-width: 900px) {
  .disk-summary-strip {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 600px) {
  .disk-summary-strip {
    grid-template-columns: repeat(2, 1fr);
  }
}

.disk-summary-item {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.disk-summary-label {
  font-size: 9.5px;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  text-transform: uppercase;
  white-space: nowrap;
}

.disk-summary-val {
  font-size: 13.5px;
  font-weight: 700;
  white-space: nowrap;
}

.disk-devices-wrapper {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.disk-devices-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 8px;
}

.disk-device-card {
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.disk-device-card:hover {
  border-color: rgba(56, 189, 248, 0.3);
  transform: translateY(-1px);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
}

.dev-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding-bottom: 6px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.dev-name-group {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.dev-icon {
  font-size: 13px;
  flex-shrink: 0;
}

.dev-name {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dev-badge {
  font-size: 8.5px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 4px;
  letter-spacing: 0.04em;
  flex-shrink: 0;
}

.badge-root {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.35);
}

.badge-part {
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.25);
}

.dev-metrics-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px 8px;
  background: rgba(0, 0, 0, 0.22);
  padding: 6px 8px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.04);
}

.dev-metric-col {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.dev-metric-label {
  font-size: 8px;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  text-transform: uppercase;
}

.dev-metric-val {
  font-size: 10.5px;
  font-weight: 600;
  white-space: nowrap;
}

.dev-util-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dev-util-label-group {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.dev-util-label {
  font-size: 8px;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  text-transform: uppercase;
}

.dev-util-pct {
  font-size: 10.5px;
}

.disk-fallback-panel {
  padding: 12px 16px;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.45);
  border: 1px dashed rgba(255, 255, 255, 0.1);
}

.fallback-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.fallback-icon {
  font-size: 18px;
  flex-shrink: 0;
}

.fallback-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 200px;
}

.fallback-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-secondary);
}

.fallback-subtitle {
  font-size: 11px;
  color: var(--text-muted);
}

.fallback-rates {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.rate-pill {
  font-size: 11px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.06);
}

/* Service filter chips and interactive table controls */
.service-filter-chips {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.chip-btn {
  font-size: 11px;
  font-weight: 600;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.2s ease;
}

.chip-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text-primary);
}

.chip-btn.active {
  background: rgba(56, 189, 248, 0.15);
  border-color: rgba(56, 189, 248, 0.4);
  color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.15);
}

.chip-healthy.active {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.4);
  color: #34d399;
}

.chip-degraded.active {
  background: rgba(245, 158, 11, 0.15);
  border-color: rgba(245, 158, 11, 0.4);
  color: #fbbf24;
}

.chip-traffic.active {
  background: rgba(139, 92, 246, 0.15);
  border-color: rgba(139, 92, 246, 0.4);
  color: #c084fc;
}

.sort-indicator {
  font-size: 9px;
  color: #38bdf8;
  margin-left: 3px;
}

.empty-services-row {
  padding: 24px;
  text-align: center;
}

.empty-services-msg {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  font-size: 12px;
}

.btn-clear-inline {
  background: none;
  border: 1px solid rgba(255, 255, 255, 0.12);
  padding: 3px 10px;
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-clear-inline:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
  border-color: var(--border-accent);
}

.node-services-section {
  padding: 18px 20px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.node-processes-section {
  padding: 18px 20px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.proc-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.node-section-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.services-table-wrapper {
  overflow-x: auto;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.2);
}

.services-mini-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}

.services-mini-table thead tr {
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid var(--border-subtle);
}

.services-mini-table th {
  padding: 10px 12px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  white-space: nowrap;
}

.services-mini-table tbody tr {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: background 0.15s ease;
}

.services-mini-table tbody tr:last-child {
  border-bottom: none;
}

.services-mini-table tbody tr:hover {
  background: rgba(56, 189, 248, 0.05);
}

.services-mini-table td {
  padding: 10px 12px;
  vertical-align: middle;
}

.col-svc-name {
  min-width: 160px;
}

.svc-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.svc-title {
  font-weight: 700;
  color: var(--text-primary);
  font-size: 12px;
}

.col-svc-cpu,
.col-svc-mem,
.col-svc-rx,
.col-svc-tx,
.col-svc-rps,
.col-svc-err {
  white-space: nowrap;
}

.col-svc-status {
  white-space: nowrap;
  width: 110px;
}

.node-section-empty {
  padding: 18px;
  text-align: center;
  background: rgba(0, 0, 0, 0.15);
  border-radius: 8px;
  border: 1px dashed var(--border-subtle);
}

/* Processes Section */
.processes-section {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.processes-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.proc-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.proc-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.proc-section-heading {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.proc-section-desc {
  font-size: 12px;
  color: var(--text-muted);
  margin: 0;
}

.proc-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.proc-category-chips {
  display: flex;
  align-items: center;
  gap: 4px;
}

.proc-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.proc-search-box {
  position: relative;
  display: flex;
  align-items: center;
}

.proc-search-box.wide-search {
  flex: 1;
  min-width: 240px;
}

.input-proc-search {
  width: 100%;
  padding: 8px 32px 8px 34px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 12px;
  transition: all 0.2s ease;
}

.input-proc-search:focus {
  outline: none;
  border-color: var(--accent-cyan);
  background: rgba(0, 0, 0, 0.5);
  box-shadow: 0 0 12px rgba(6, 182, 212, 0.25);
}

.page-size-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.page-size-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.page-size-pill {
  display: inline-flex;
  align-items: center;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  padding: 2px;
  gap: 2px;
}

.btn-page-size {
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  padding: 3px 9px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  line-height: 1.2;
}

.btn-page-size:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.08);
}

.btn-page-size.active {
  background: rgba(56, 189, 248, 0.18);
  color: #38bdf8;
  border-color: rgba(56, 189, 248, 0.4);
  font-weight: 700;
  box-shadow: 0 0 8px rgba(56, 189, 248, 0.2);
}

.proc-sort-group {
  display: flex;
  align-items: center;
}

.select-proc-sort {
  padding: 8px 14px;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  outline: none;
  transition: all 0.2s ease;
}

.select-proc-sort:focus,
.select-proc-sort:hover {
  border-color: var(--accent-cyan);
  color: var(--text-primary);
  box-shadow: 0 0 10px rgba(6, 182, 212, 0.2);
}

/* Processes Table (Zero Horizontal Scroll Layout) */
.processes-table-wrapper {
  overflow-x: auto;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.2);
}

.processes-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}

.processes-table thead tr {
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid var(--border-subtle);
}

.processes-table th {
  padding: 8px 10px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  white-space: nowrap;
}

.processes-table tbody tr {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: background 0.15s ease;
}

.processes-table tbody tr:last-child {
  border-bottom: none;
}

.processes-table tbody tr:hover {
  background: rgba(56, 189, 248, 0.05);
}

.proc-row-hot {
  background: rgba(244, 63, 94, 0.04);
}

.proc-row-hot:hover {
  background: rgba(244, 63, 94, 0.08) !important;
}

.processes-table td {
  padding: 8px 10px;
  vertical-align: middle;
}

.col-proc-pid {
  white-space: nowrap;
  width: 55px;
}

.pid-tag {
  font-size: 11px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 5px;
  border-radius: 4px;
  border: 1px solid var(--border-subtle);
}

.pid-tag-ctr {
  background: rgba(56, 189, 248, 0.15) !important;
  color: #38bdf8 !important;
  border-color: rgba(56, 189, 248, 0.3) !important;
}

.col-proc-app {
  min-width: 110px;
  max-width: 160px;
}

.ctr-icon {
  font-size: 11px;
  margin-right: 3px;
}

.proc-row-container {
  background: rgba(56, 189, 248, 0.02);
}

.proc-app-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  cursor: help;
}

.proc-name-row {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex-wrap: nowrap;
}

.proc-name {
  font-weight: 700;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
}

.proc-origin-tag {
  font-size: 8px;
  font-weight: 700;
  letter-spacing: 0.04em;
  padding: 1px 4px;
  border-radius: 3px;
  display: inline-flex;
  align-items: center;
  gap: 2px;
  white-space: nowrap;
  line-height: 1.2;
}

.origin-badge-container {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.origin-badge-systemd {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.3);
}

.origin-badge-kernel {
  background: rgba(148, 163, 184, 0.12);
  color: #94a3b8;
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.proc-cmd-line {
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 150px;
}

.col-proc-user {
  white-space: nowrap;
  width: 55px;
}

.user-badge {
  font-size: 10px;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.05);
  padding: 1px 5px;
  border-radius: 4px;
}

.col-proc-cpu {
  width: 80px;
  min-width: 75px;
  white-space: nowrap;
}

.proc-cpu-box {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.proc-cpu-val-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.proc-cpu-val {
  font-weight: 700;
  font-family: var(--font-mono);
  font-size: 11px;
  transition: color 0.5s ease;
}

.badge-hot-pulse {
  font-size: 8px;
  padding: 1px 4px;
  animation: pulse-hot 1.2s infinite ease-in-out;
}

@keyframes pulse-hot {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.08); opacity: 0.8; }
}

.proc-mini-track {
  width: 100%;
  height: 3px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.proc-mini-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.8s ease-in-out, background-color 0.5s ease;
}

.col-proc-mem {
  white-space: nowrap;
  width: 85px;
  min-width: 75px;
}

.proc-mem-box {
  display: flex;
  align-items: baseline;
  gap: 3px;
}

.proc-mem-val {
  font-weight: 600;
  color: var(--text-primary);
  font-family: var(--font-mono);
  transition: color 0.5s ease;
  font-size: 11.5px;
}

.proc-mem-pct {
  font-size: 9.5px;
  color: var(--text-muted);
}

.col-proc-rps {
  white-space: nowrap;
  width: 65px;
  font-size: 11.5px;
}

.col-proc-err {
  white-space: nowrap;
  width: 55px;
  font-size: 11.5px;
}

.col-proc-bw {
  white-space: nowrap;
  width: 140px;
}

.col-proc-state {
  white-space: nowrap;
  width: 85px;
  min-width: 80px;
  text-align: right;
}

/* Pagination & Page Size Footer */
.proc-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 10px;
  flex-wrap: wrap;
}

.pagination-info {
  font-size: 11.5px;
  color: var(--text-muted);
}

.pagination-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.btn-page {
  padding: 3px 8px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-page:hover:not(:disabled) {
  background: rgba(56, 189, 248, 0.15);
  border-color: rgba(56, 189, 248, 0.35);
  color: #38bdf8;
}

.btn-page:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.page-indicator {
  font-size: 11px;
  color: var(--text-muted);
  padding: 0 4px;
}

.th-status-right,
.td-status-right {
  text-align: right;
}

/* Empty State */
.proc-empty-state {
  padding: 36px 24px;
  text-align: center;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.4);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px dashed rgba(56, 189, 248, 0.25);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  position: relative;
  overflow: hidden;
}

.radar-glow-container {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  margin-bottom: 4px;
}

.radar-beacon {
  position: absolute;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(6, 182, 212, 0.25) 0%, rgba(6, 182, 212, 0) 70%);
  animation: radar-beacon-pulse 2.4s infinite cubic-bezier(0.4, 0, 0.6, 1);
}

@keyframes radar-beacon-pulse {
  0% {
    transform: scale(0.6);
    opacity: 0.8;
  }
  50% {
    transform: scale(1.4);
    opacity: 0.2;
  }
  100% {
    transform: scale(1.8);
    opacity: 0;
  }
}

.proc-empty-icon {
  font-size: 32px;
  position: relative;
  z-index: 1;
  filter: drop-shadow(0 0 12px rgba(6, 182, 212, 0.6));
  animation: icon-float 3s ease-in-out infinite alternate;
}

@keyframes icon-float {
  0% { transform: translateY(0px); }
  100% { transform: translateY(-4px); }
}

.proc-empty-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
  margin: 0;
}

.proc-empty-desc {
  font-size: 12.5px;
  color: var(--text-secondary);
  max-width: 480px;
  margin: 0;
  line-height: 1.55;
}

.proc-empty-telemetry-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: 100%;
  max-width: 520px;
}

.troubleshooting-hint-box {
  width: 100%;
  text-align: left;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.hint-header {
  display: flex;
  align-items: center;
  gap: 6px;
}

.hint-icon {
  font-size: 14px;
}

.hint-title {
  font-size: 11.5px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.hint-list {
  margin: 0;
  padding-left: 18px;
  font-size: 11.5px;
  color: var(--text-muted);
  line-height: 1.6;
}

.hint-list code {
  font-family: var(--font-mono);
  background: rgba(255, 255, 255, 0.08);
  padding: 1px 5px;
  border-radius: 4px;
  color: var(--accent-cyan);
  font-size: 11px;
}

.proc-empty-actions {
  margin-top: 4px;
}

.btn-sm {
  padding: 5px 12px;
  font-size: 11px;
}

/* Drawer Footer */
.node-drawer-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--border-subtle);
}


</style>
