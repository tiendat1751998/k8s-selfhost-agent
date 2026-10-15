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

export function getNodeIdentifier(node: NodeMetrics): string {
  const n = node as unknown as Record<string, unknown>
  const ip = (n.ip as string) || (n.ip_address as string) || (n.internal_ip as string) || (typeof n.endpoint === 'string' ? n.endpoint.replace(/^https?:\/\//, '').split(':')[0] : '')
  if (ip) return ip
  if (node.node_id) {
    return node.node_id.length > 8 ? node.node_id.slice(0, 8) : node.node_id
  }
  return '--'
}

export function getCpuColorClass(cpu?: number): string {
  const val = cpu || 0
  if (val >= 80) return 'text-rose'
  if (val >= 50) return 'text-amber'
  return 'text-emerald'
}
