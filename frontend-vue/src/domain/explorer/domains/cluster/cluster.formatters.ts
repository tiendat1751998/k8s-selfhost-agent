import type { K8sResource, NodeCondition, NodeSystemInfo, NodeTaint } from '../../types'

export function formatAge(dateStr?: string): string {
  if (!dateStr) return 'Unknown'
  const diff = Date.now() - new Date(dateStr).getTime()
  if (isNaN(diff) || diff < 0) return 'Just now'
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return `${days}d`
}

export function getResourceAge(resource: K8sResource): string {
  return formatAge(resource.metadata?.creationTimestamp)
}

export function isNodeUnschedulable(resource: K8sResource): boolean {
  return Boolean((resource.spec as { unschedulable?: boolean })?.unschedulable)
}

export function getNodeStatus(resource: K8sResource): string {
  if (isNodeUnschedulable(resource)) return 'SchedulingDisabled'
  const conditions = (resource.status as { conditions?: NodeCondition[] })?.conditions
  if (Array.isArray(conditions)) {
    const readyCond = conditions.find(c => c.type === 'Ready')
    if (readyCond && readyCond.status === 'True') return 'Ready'
    if (readyCond && readyCond.status === 'False') return 'NotReady'
  }
  return 'Unknown'
}

export const getNodeStatusBadge = getNodeStatus

export function getNodeRoles(resource: K8sResource): string[] {
  const labels = resource.metadata?.labels || {}
  const roles: string[] = []
  for (const key of Object.keys(labels)) {
    if (key.startsWith('node-role.kubernetes.io/')) {
      const role = key.replace('node-role.kubernetes.io/', '')
      if (role) roles.push(role)
    }
  }
  if (labels['kubernetes.io/role'] && !roles.includes(labels['kubernetes.io/role'])) {
    roles.push(labels['kubernetes.io/role'])
  }
  return roles.length > 0 ? roles : ['worker']
}

export function getNodeVersion(resource: K8sResource): string {
  return (resource.status as { nodeInfo?: NodeSystemInfo })?.nodeInfo?.kubeletVersion || 'N/A'
}

export function getNodeInternalIP(resource: K8sResource): string {
  const addresses = (resource.status as { addresses?: { type: string; address: string }[] })?.addresses
  return Array.isArray(addresses) ? (addresses.find(a => a.type === 'InternalIP')?.address || 'N/A') : 'N/A'
}

export function getNodeOSArch(resource: K8sResource): string {
  const info = (resource.status as { nodeInfo?: NodeSystemInfo })?.nodeInfo
  return (info?.operatingSystem && info?.architecture)
    ? info.operatingSystem + '/' + info.architecture
    : info?.osImage || 'Linux'
}

export const getNodeOsArch = getNodeOSArch

export function getNodePodsCount(resource: K8sResource): string {
  const allocatable = (resource.status as { allocatable?: { pods?: string } })?.allocatable
  return allocatable?.pods ? allocatable.pods + ' max' : 'N/A'
}

export function getNodeTaints(resource: K8sResource): NodeTaint[] {
  return ((resource.spec as { taints?: NodeTaint[] } | undefined)?.taints) || []
}

export function getNodeConditions(resource: K8sResource): NodeCondition[] {
  return ((resource.status as { conditions?: NodeCondition[] } | undefined)?.conditions) || []
}

export function getNodeSystemInfo(resource: K8sResource): NodeSystemInfo | undefined {
  return (resource.status as { nodeInfo?: NodeSystemInfo } | undefined)?.nodeInfo
}

export function getNodeLabels(resource: K8sResource): Record<string, string> {
  return resource.metadata?.labels || {}
}

export function getEventType(resource: K8sResource): string {
  return (resource as unknown as { type?: string }).type || 'Normal'
}

export function getResourceStatus(resource: K8sResource): string {
  if (resource.status && typeof resource.status === 'object') {
    if ('phase' in resource.status && typeof resource.status.phase === 'string') return resource.status.phase
    if ('readyReplicas' in resource.status && 'replicas' in resource.status) {
      return `${resource.status.readyReplicas || 0}/${resource.status.replicas || 0} Ready`
    }
  }
  return 'Active'
}
