import type { HostType, AgentInfo, ComputeHost, CreateHostRequest, UpdateHostRequest, NodeMetrics, ProcessMetric } from '../api/hosts'

export interface HostTestResult {
  latency_ms: number
  status: string
  timestamp: Date
  message?: string
  agent_info?: AgentInfo
}

export interface HostTestHistoryItem {
  latency_ms: number
  status: string
  timestamp: Date
  message?: string
}

export interface ModalTestResult {
  success: boolean
  latency_ms: number
  message: string
  agent_info?: AgentInfo
}

export interface HostFormData {
  name: string
  host_type: HostType
  endpoint: string
  tls_enabled: boolean
  tls_ca: string
  tls_cert: string
  tls_key: string
  api_version: string
  auth_token: string
  description: string
  labels: Array<{ key: string; value: string }>
}

export interface HostTypeDefinition {
  type: string
  label: string
  icon: string
  color: string
  badgeClass: string
  desc: string
  placeholder: string
  hint: string
}

export const hostTypeDefinitions: HostTypeDefinition[] = [
  {
    type: 'agent',
    label: 'K8s-Agent',
    icon: 'server',
    color: 'cyan',
    badgeClass: 'badge-agent',
    desc: 'System metrics telemetry (CPU/RAM/Disk/Network) via host daemon',
    placeholder: 'http://127.0.0.1:9100',
    hint: 'Run `./deploy-agent.sh user@server-ip` to install agent daemon'
  },
  {
    type: 'docker',
    label: 'Docker Engine',
    icon: 'box',
    color: 'blue',
    badgeClass: 'badge-docker',
    desc: 'Standalone Docker Engine socket / TCP container manager',
    placeholder: 'tcp://127.0.0.1:2375',
    hint: 'Use port 2376 for TLS mTLS authenticated socket endpoints'
  },
  {
    type: 'k8s',
    label: 'Kubernetes API',
    icon: 'cpu',
    color: 'purple',
    badgeClass: 'badge-k8s',
    desc: 'Direct Kubernetes Control Plane API Server endpoint',
    placeholder: 'https://k8s-master:6443',
    hint: 'Connects directly to API server with optional client certificates'
  },
  {
    type: 'prometheus',
    label: 'Prometheus Target',
    icon: 'activity',
    color: 'orange',
    badgeClass: 'badge-purple',
    desc: 'Prometheus metrics scraping and time-series query target',
    placeholder: 'http://prometheus:9090',
    hint: 'Validates `/-/healthy` and build status endpoints'
  },
  {
    type: 'git',
    label: 'Git Repository',
    icon: 'link',
    color: 'green',
    badgeClass: 'badge-emerald',
    desc: 'GitHub, GitLab, Gitea GitOps repository endpoint',
    placeholder: 'https://github.com/org/repo',
    hint: 'Git repository URL used for automated pipeline synchronization'
  },
  {
    type: 'database',
    label: 'Database Server',
    icon: 'database',
    color: 'amber',
    badgeClass: 'badge-amber',
    desc: 'PostgreSQL, MySQL, Redis, or MongoDB connection target',
    placeholder: 'postgresql://user:pass@host:5432/db',
    hint: 'Accepts standard DSN or host:port connection targets'
  },
  {
    type: 'custom',
    label: 'Custom HTTP',
    icon: 'settings',
    color: 'gray',
    badgeClass: 'badge-muted',
    desc: 'Generic HTTP microservice or health check target',
    placeholder: 'https://service:8080/health',
    hint: 'Accepts any HTTP/HTTPS health verification URI'
  }
]

export function createDefaultHostForm(): HostFormData {
  return {
    name: '',
    host_type: 'agent',
    endpoint: '',
    tls_enabled: false,
    tls_ca: '',
    tls_cert: '',
    tls_key: '',
    api_version: '',
    auth_token: '',
    description: '',
    labels: [{ key: 'env', value: 'production' }]
  }
}

export function populateHostForm(host: ComputeHost): HostFormData {
  const labelArray: Array<{ key: string; value: string }> = []
  if (host.labels) {
    for (const [k, v] of Object.entries(host.labels)) {
      if (k !== 'description' && k !== 'auth_token') {
        labelArray.push({ key: k, value: v })
      }
    }
  }
  return {
    name: host.name,
    host_type: (host.host_type as HostType) || 'agent',
    endpoint: host.endpoint,
    tls_enabled: host.tls_enabled || false,
    tls_ca: host.tls_ca || '',
    tls_cert: host.tls_cert || '',
    tls_key: '',
    api_version: host.api_version || '',
    auth_token: host.labels?.['auth_token'] || '',
    description: host.labels?.['description'] || '',
    labels: labelArray.length > 0 ? labelArray : [{ key: 'env', value: 'production' }]
  }
}

export function buildHostPayload(form: HostFormData): CreateHostRequest & UpdateHostRequest {
  const labels: Record<string, string> = {}
  for (const l of form.labels) {
    if (l.key.trim()) labels[l.key.trim()] = l.value.trim()
  }
  if (form.description.trim()) labels['description'] = form.description.trim()
  if (form.auth_token.trim()) labels['auth_token'] = form.auth_token.trim()

  return {
    name: form.name.trim(),
    host_type: form.host_type,
    endpoint: form.endpoint.trim(),
    tls_enabled: form.tls_enabled,
    tls_ca: form.tls_ca || undefined,
    tls_cert: form.tls_cert || undefined,
    tls_key: form.tls_key || undefined,
    api_version: form.api_version || undefined,
    labels
  }
}

export function matchesHostFilters(
  host: ComputeHost,
  searchQuery: string,
  typeFilter: string,
  statusFilter: string,
  labelFilter: string
): boolean {
  if (searchQuery.trim()) {
    const q = searchQuery.toLowerCase().trim()
    const matchName = host.name.toLowerCase().includes(q)
    const matchEndpoint = host.endpoint.toLowerCase().includes(q)
    const matchType = (host.host_type || '').toLowerCase().includes(q)
    const matchLabels = host.labels && Object.entries(host.labels).some(([k, v]) => 
      k.toLowerCase().includes(q) || v.toLowerCase().includes(q)
    )
    if (!matchName && !matchEndpoint && !matchType && !matchLabels) {
      return false
    }
  }

  if (typeFilter !== 'all' && (host.host_type || 'agent') !== typeFilter) {
    return false
  }

  if (statusFilter !== 'all') {
    const s = (host.status || 'connected').toLowerCase()
    if (statusFilter === 'connected' && s !== 'connected' && s !== 'ok') return false
    if (statusFilter === 'disconnected' && s !== 'disconnected' && s !== 'pending') return false
    if (statusFilter === 'error' && s !== 'error' && s !== 'unhealthy') return false
  }

  if (labelFilter !== 'all') {
    const [key, val] = labelFilter.split('=')
    if (!host.labels || host.labels[key] !== val) {
      return false
    }
  }

  return true
}

export function getHostTypeMeta(type?: string): HostTypeDefinition {
  const match = hostTypeDefinitions.find(d => d.type === (type || 'agent'))
  return match || hostTypeDefinitions[0]
}

export function getLatencyBadgeClass(latency?: number): string {
  if (latency === undefined || latency === null || latency <= 0) return 'text-muted'
  if (latency < 50) return 'latency-fast'
  if (latency < 200) return 'latency-medium'
  return 'latency-slow'
}

export function formatDate(d?: string): string {
  if (!d) return 'Never'
  try {
    const dt = new Date(d)
    const now = new Date()
    const diffSec = Math.floor((now.getTime() - dt.getTime()) / 1000)
    if (diffSec < 60) return 'Just now'
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
    return dt.toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}

export function formatUptime(seconds?: number): string {
  if (!seconds || seconds <= 0) return '0s'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${secs}s`
  return `${secs}s`
}

export function formatHostLastSeen(
  host: {
    last_health_check?: string
    status?: string
    updated_at?: string
    created_at?: string
  },
  formatDateFn: (d?: string) => string = formatDate,
  includePrefix: boolean = false
): string {
  if (host.last_health_check) {
    const formatted = formatDateFn(host.last_health_check)
    return includePrefix ? `Last check: ${formatted}` : formatted
  }
  const isConnected = (host.status || 'connected').toLowerCase() === 'connected'
  const fallbackDate = host.updated_at || host.created_at
  if (fallbackDate && isConnected) {
    return `Verified: ${formatDateFn(fallbackDate)}`
  }
  if (isConnected) {
    return 'Awaiting probe'
  }
  return includePrefix ? 'Last check: Never' : 'Never'
}

// ==========================================
// Physical Hardware Blade Fabric Telemetry
// ==========================================

export interface HostHardwareMetrics {
  cpu_percent: number
  load_average: [number, number, number] | number[]
  memory_used: number
  memory_total: number
  memory_percent: number
  disk_used: number
  disk_total: number
  disk_percent: number
  disk_avg_await_ms: number
  os_distro: string
  kernel_version: string
  arch: string
  uptime_seconds: number
  running_count: number
  container_count: number
  processes: number
  top_processes: ProcessMetric[]
  is_live: boolean
}

export interface FleetHardwareAggregates {
  fleetCpuAvgPct: number
  fleetMemUsedBytes: number
  fleetMemTotalBytes: number
  fleetMemUsedGiB: string
  fleetMemTotalGiB: string
  fleetMemPct: number
  fleetDiskUsedBytes: number
  fleetDiskTotalBytes: number
  fleetDiskUsedGiB: string
  fleetDiskTotalGiB: string
  fleetDiskPct: number
  totalRunningContainers: number
  onlineBladeCount: number
  totalBladeCount: number
}

export function matchHostWithNodeMetrics(host: ComputeHost, nodeMetricsList: NodeMetrics[]): NodeMetrics | undefined {
  if (!nodeMetricsList || nodeMetricsList.length === 0) return undefined

  // 1. Direct ID or Name match
  const directMatch = nodeMetricsList.find(n => 
    (n.node_id && (n.node_id === host.id || n.node_id === host.name)) ||
    (n.node_name && (n.node_name === host.name || n.node_name === host.id))
  )
  if (directMatch) return directMatch

  // 2. Label matches (kubernetes.io/hostname, node, hostname, instance)
  if (host.labels) {
    const labelMatch = nodeMetricsList.find(n =>
      (n.node_name && (n.node_name === host.labels['kubernetes.io/hostname'] || n.node_name === host.labels['node'] || n.node_name === host.labels['hostname'])) ||
      (n.node_id && (n.node_id === host.labels['node_id'] || n.node_id === host.labels['instance']))
    )
    if (labelMatch) return labelMatch
  }

  // 3. Endpoint IP match (e.g. host endpoint http://10.10.10.60:9100 matching node with 10.10.10.60)
  if (host.endpoint) {
    const ipMatch = host.endpoint.match(/\b(?:\d{1,3}\.){3}\d{1,3}\b/)
    if (ipMatch) {
      const ip = ipMatch[0]
      const endpointMatch = nodeMetricsList.find(n => 
        (n.node_name && n.node_name.includes(ip)) ||
        (n.node_id && n.node_id.includes(ip))
      )
      if (endpointMatch) return endpointMatch
    }
  }

  // 4. Case-insensitive name match
  const lowerName = host.name?.toLowerCase().trim()
  if (lowerName) {
    const looseMatch = nodeMetricsList.find(n => 
      n.node_name?.toLowerCase().trim() === lowerName ||
      n.node_id?.toLowerCase().trim() === lowerName
    )
    if (looseMatch) return looseMatch
  }

  return undefined
}

export function resolveHostHardwareMetrics(
  host: ComputeHost,
  node?: NodeMetrics,
  agentInfo?: AgentInfo
): HostHardwareMetrics {
  if (!node) {
    return {
      cpu_percent: 0,
      load_average: [0, 0, 0],
      memory_used: 0,
      memory_total: 0,
      memory_percent: 0,
      disk_used: 0,
      disk_total: 0,
      disk_percent: 0,
      disk_avg_await_ms: 0,
      os_distro: agentInfo?.os_distro || agentInfo?.os || '',
      kernel_version: agentInfo?.kernel_version || '',
      arch: agentInfo?.arch || '',
      uptime_seconds: agentInfo?.uptime_seconds || agentInfo?.uptime || 0,
      running_count: 0,
      container_count: 0,
      processes: 0,
      top_processes: [],
      is_live: false,
    }
  }

  const loadAvg = node.load_average || node.load_avg || [0, 0, 0]
  const memUsed = node.memory_used || 0
  const memTotal = node.memory_total || 0
  const memPct = node.memory_percent ?? (memTotal > 0 ? (memUsed / memTotal) * 100 : 0)

  const diskUsed = node.disk_used || 0
  const diskTotal = node.disk_total || 0
  const diskPct = node.disk_percent ?? (diskTotal > 0 ? (diskUsed / diskTotal) * 100 : 0)

  const isConnected = host.status === 'connected' || host.status === 'ok' ||
    node.status === 'ready' || node.status === 'ok' || node.status === 'online'

  return {
    cpu_percent: typeof node.cpu_percent === 'number' && !isNaN(node.cpu_percent) ? node.cpu_percent : 0,
    load_average: Array.isArray(loadAvg) && loadAvg.length >= 3 ? [loadAvg[0], loadAvg[1], loadAvg[2]] : [0, 0, 0],
    memory_used: memUsed,
    memory_total: memTotal,
    memory_percent: typeof memPct === 'number' && !isNaN(memPct) ? Math.min(100, Math.max(0, memPct)) : 0,
    disk_used: diskUsed,
    disk_total: diskTotal,
    disk_percent: typeof diskPct === 'number' && !isNaN(diskPct) ? Math.min(100, Math.max(0, diskPct)) : 0,
    disk_avg_await_ms: typeof node.disk_avg_await_ms === 'number' && !isNaN(node.disk_avg_await_ms) ? node.disk_avg_await_ms : 0,
    os_distro: node.os_distro || node.os || agentInfo?.os_distro || agentInfo?.os || '',
    kernel_version: node.kernel_version || agentInfo?.kernel_version || '',
    arch: node.arch || agentInfo?.arch || '',
    uptime_seconds: node.uptime_seconds || node.uptime || agentInfo?.uptime_seconds || agentInfo?.uptime || 0,
    running_count: node.running_count || 0,
    container_count: node.container_count || 0,
    processes: node.processes || 0,
    top_processes: Array.isArray(node.top_processes) ? node.top_processes : [],
    is_live: isConnected,
  }
}

export function computeFleetHardwareAggregates(
  hosts: ComputeHost[],
  hardwareMap: Record<string, HostHardwareMetrics>
): FleetHardwareAggregates {
  const onlineHosts = hosts.filter(h => h.status === 'connected' || h.status === 'ok')
  const liveMetrics = hosts
    .map(h => hardwareMap[h.id])
    .filter((m): m is HostHardwareMetrics => Boolean(m && m.is_live))

  const cpuHosts = liveMetrics.filter(m => m.cpu_percent > 0)
  const fleetCpuAvgPct = cpuHosts.length > 0
    ? parseFloat((cpuHosts.reduce((acc, m) => acc + m.cpu_percent, 0) / cpuHosts.length).toFixed(1))
    : 0

  const fleetMemUsedBytes = liveMetrics.reduce((acc, m) => acc + m.memory_used, 0)
  const fleetMemTotalBytes = liveMetrics.reduce((acc, m) => acc + m.memory_total, 0)
  const fleetDiskUsedBytes = liveMetrics.reduce((acc, m) => acc + m.disk_used, 0)
  const fleetDiskTotalBytes = liveMetrics.reduce((acc, m) => acc + m.disk_total, 0)
  const totalRunningContainers = liveMetrics.reduce((acc, m) => acc + m.running_count, 0)

  const bytesToGiB = (bytes: number): string => (bytes / (1024 * 1024 * 1024)).toFixed(1)

  return {
    fleetCpuAvgPct,
    fleetMemUsedBytes,
    fleetMemTotalBytes,
    fleetMemUsedGiB: bytesToGiB(fleetMemUsedBytes),
    fleetMemTotalGiB: bytesToGiB(fleetMemTotalBytes),
    fleetMemPct: fleetMemTotalBytes > 0 ? Math.min(100, (fleetMemUsedBytes / fleetMemTotalBytes) * 100) : 0,
    fleetDiskUsedBytes,
    fleetDiskTotalBytes,
    fleetDiskUsedGiB: bytesToGiB(fleetDiskUsedBytes),
    fleetDiskTotalGiB: bytesToGiB(fleetDiskTotalBytes),
    fleetDiskPct: fleetDiskTotalBytes > 0 ? Math.min(100, (fleetDiskUsedBytes / fleetDiskTotalBytes) * 100) : 0,
    totalRunningContainers,
    onlineBladeCount: onlineHosts.length,
    totalBladeCount: hosts.length
  }
}

