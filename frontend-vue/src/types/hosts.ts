import type { HostType, AgentInfo, ComputeHost, CreateHostRequest, UpdateHostRequest } from '../api/compute'

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
    icon: '📡',
    color: 'cyan',
    badgeClass: 'badge-slate text-muted',
    desc: 'System metrics telemetry (CPU/RAM/Disk/Network) via host daemon',
    placeholder: 'http://10.10.10.200:9100',
    hint: '💡 Run `./deploy-agent.sh user@server-ip` to install agent daemon'
  },
  {
    type: 'docker',
    label: 'Docker Engine',
    icon: '🐳',
    color: 'blue',
    badgeClass: 'badge-slate text-muted',
    desc: 'Standalone Docker Engine socket / TCP container manager',
    placeholder: 'tcp://10.10.10.133:2375',
    hint: '💡 Use port 2376 for TLS mTLS authenticated socket endpoints'
  },
  {
    type: 'k8s',
    label: 'Kubernetes API',
    icon: '☸️',
    color: 'purple',
    badgeClass: 'badge-slate text-muted',
    desc: 'Direct Kubernetes Control Plane API Server endpoint',
    placeholder: 'https://k8s-master:6443',
    hint: '💡 Connects directly to API server with optional client certificates'
  },
  {
    type: 'prometheus',
    label: 'Prometheus Target',
    icon: '📊',
    color: 'orange',
    badgeClass: 'badge-slate text-muted',
    desc: 'Prometheus metrics scraping and time-series query target',
    placeholder: 'http://prometheus:9090',
    hint: '💡 Validates `/-/healthy` and build status endpoints'
  },
  {
    type: 'git',
    label: 'Git Repository',
    icon: '🔗',
    color: 'green',
    badgeClass: 'badge-slate text-muted',
    desc: 'GitHub, GitLab, Gitea GitOps repository endpoint',
    placeholder: 'https://github.com/org/repo',
    hint: '💡 Git repository URL used for automated pipeline synchronization'
  },
  {
    type: 'database',
    label: 'Database Server',
    icon: '🗄️',
    color: 'amber',
    badgeClass: 'badge-slate text-muted',
    desc: 'PostgreSQL, MySQL, Redis, or MongoDB connection target',
    placeholder: 'postgresql://user:pass@host:5432/db',
    hint: '💡 Accepts standard DSN or host:port connection targets'
  },
  {
    type: 'custom',
    label: 'Custom HTTP',
    icon: '⚙️',
    color: 'gray',
    badgeClass: 'badge-slate text-muted',
    desc: 'Generic HTTP microservice or health check target',
    placeholder: 'https://service:8080/health',
    hint: '💡 Accepts any HTTP/HTTPS health verification URI'
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
