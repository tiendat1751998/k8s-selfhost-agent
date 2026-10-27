import type { NodeMetrics } from '../../api/overview'

export function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

export function formatRps(val: number): string {
  if (!val || isNaN(val)) return '0'
  return val >= 1000 ? `${(val / 1000).toFixed(1)}k` : Math.round(val).toString()
}

export function isNodeDown(node: NodeMetrics): boolean {
  return node.status === 'down' || node.status === 'offline' || node.status === 'disconnected' || node.memory_total === 0
}

export function getNodeStatusDotClass(node: NodeMetrics): string {
  if (isNodeDown(node)) return 'dot-red'
  if (node.cpu_percent >= 80 || node.memory_percent >= 80 || node.status === 'degraded' || node.status === 'warning') return 'dot-amber'
  return 'dot-green'
}

export function isControlPlane(node: NodeMetrics): boolean {
  return /master|control|manager/i.test(node.role || '')
}

export function getNodeRole(node: NodeMetrics): string {
  const role = (node.role || '').toLowerCase().trim()
  if (role === 'standalone') return 'standalone'
  if (isControlPlane(node)) return 'control-plane'
  if (role === 'worker') return 'worker'
  return role || 'worker'
}

export function extractNodeIp(node?: (Partial<NodeMetrics> & Record<string, unknown>) | null): string {
  if (!node) return '--'
  if (typeof node.ip === 'string' && node.ip.trim()) return node.ip.trim()
  if (typeof node.ip_address === 'string' && node.ip_address.trim()) return node.ip_address.trim()
  if (typeof node.internal_ip === 'string' && node.internal_ip.trim()) return node.internal_ip.trim()
  
  const rawAddr = (typeof node.address === 'string' ? node.address : typeof node.endpoint === 'string' ? node.endpoint : '').trim()
  if (rawAddr) {
    const stripped = rawAddr.replace(/^https?:\/\//i, '').split('/')[0]
    const host = stripped.split(':')[0]
    if (host) return host
  }
  return '--'
}

export function getNodeIdentifier(node: NodeMetrics): string {
  const ip = extractNodeIp(node as unknown as Record<string, unknown>)
  if (ip !== '--') return ip
  if (node.node_id) {
    return node.node_id.length > 8 ? node.node_id.slice(0, 8) : node.node_id
  }
  return '--'
}

export interface ContainerKpiStatus {
  primaryText: string
  badgeText: string
  badgeVariant: 'badge-cyan' | 'badge-amber' | 'badge-muted'
}

export function resolveContainerKpi(runningContainers: number, totalContainers: number, healthyNodes: number): ContainerKpiStatus {
  if (totalContainers === 0 && healthyNodes > 0) {
    return {
      primaryText: '0 PODS',
      badgeText: 'K8S :6443 OFFLINE',
      badgeVariant: 'badge-amber',
    }
  }
  if (totalContainers === 0 && healthyNodes === 0) {
    return {
      primaryText: '0/0',
      badgeText: 'STOPPED',
      badgeVariant: 'badge-muted',
    }
  }
  return {
    primaryText: `${runningContainers}/${totalContainers}`,
    badgeText: runningContainers === totalContainers ? 'RUNNING' : 'DEGRADED',
    badgeVariant: runningContainers === totalContainers ? 'badge-cyan' : 'badge-amber',
  }
}

export function getCpuColorClass(cpu?: number): string {
  const val = cpu || 0
  if (val >= 80) return 'text-rose'
  if (val >= 50) return 'text-amber'
  return 'text-emerald'
}

