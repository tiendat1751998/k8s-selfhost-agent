import type { K8sResource } from '../../types'
import { formatAge } from '../cluster/cluster.formatters'
import type { ContainerSpec, ContainerStatus, MergedContainerInfo } from './workloads.types'

export function getPodPhase(resource: K8sResource): string {
  if (resource.status && typeof resource.status === 'object') {
    const status = resource.status as Record<string, unknown>
    const containerStatuses = status.containerStatuses as ContainerStatus[] | undefined
    if (Array.isArray(containerStatuses)) {
      for (const cs of containerStatuses) {
        if (cs.state?.waiting?.reason) return cs.state.waiting.reason
        if (cs.state?.terminated?.reason && cs.state.terminated.reason !== 'Completed') {
          return cs.state.terminated.reason
        }
      }
    }
    if (typeof status.phase === 'string' && status.phase) return status.phase
  }
  return 'Unknown'
}

export function getPodReadyCount(resource: K8sResource): string {
  const spec = resource.spec as { containers?: ContainerSpec[] } | undefined
  const status = resource.status as { containerStatuses?: ContainerStatus[] } | undefined
  const total = Array.isArray(spec?.containers)
    ? spec.containers.length
    : Array.isArray(status?.containerStatuses)
      ? status.containerStatuses.length
      : 0
  let ready = 0
  if (Array.isArray(status?.containerStatuses)) {
    ready = status.containerStatuses.filter(cs => Boolean(cs?.ready)).length
  }
  return `${ready}/${total}`
}

export function getPodRestarts(resource: K8sResource): number {
  const status = resource.status as { containerStatuses?: ContainerStatus[] } | undefined
  if (Array.isArray(status?.containerStatuses)) {
    return status.containerStatuses.reduce((sum, cs) => sum + (Number(cs?.restartCount) || 0), 0)
  }
  return 0
}

export function getPodNode(resource: K8sResource): string {
  const spec = resource.spec as { nodeName?: string } | undefined
  if (spec?.nodeName) return spec.nodeName
  const status = resource.status as { hostIP?: string } | undefined
  return status?.hostIP || 'Unassigned'
}

export function getPodIP(resource: K8sResource): string {
  const status = resource.status as { podIP?: string; podIPs?: { ip: string }[] } | undefined
  if (status?.podIP) return status.podIP
  if (Array.isArray(status?.podIPs) && status.podIPs.length > 0 && status.podIPs[0]?.ip) {
    return status.podIPs[0].ip
  }
  return 'None'
}

export function getHostIP(resource: K8sResource): string {
  const status = resource.status as { hostIP?: string } | undefined
  return status?.hostIP || 'None'
}

export function getPodQoS(resource: K8sResource): string {
  const status = resource.status as { qosClass?: string } | undefined
  return status?.qosClass || 'BestEffort'
}

export function getPodServiceAccount(resource: K8sResource): string {
  const spec = resource.spec as { serviceAccountName?: string; serviceAccount?: string } | undefined
  return spec?.serviceAccountName || spec?.serviceAccount || 'default'
}

export function getPodContainers(resource: K8sResource): MergedContainerInfo[] {
  const spec = resource.spec as { containers?: ContainerSpec[]; initContainers?: ContainerSpec[] } | undefined
  const status = resource.status as { containerStatuses?: ContainerStatus[]; initContainerStatuses?: ContainerStatus[] } | undefined
  const specContainers = Array.isArray(spec?.containers) ? spec.containers : []
  const statuses = Array.isArray(status?.containerStatuses) ? status.containerStatuses : []
  const statusMap = new Map<string, ContainerStatus>()
  for (const s of statuses) {
    if (s.name) statusMap.set(s.name, s)
  }
  const allNames = new Set<string>()
  specContainers.forEach(c => c.name && allNames.add(c.name))
  statuses.forEach(s => s.name && allNames.add(s.name))

  const results: MergedContainerInfo[] = []
  for (const name of allNames) {
    const specC = specContainers.find(c => c.name === name)
    const statC = statusMap.get(name)
    const image = statC?.image || specC?.image || 'unknown'
    let portsStr = 'None'
    if (Array.isArray(specC?.ports) && specC.ports.length > 0) {
      portsStr = specC.ports
        .map(p => `${p.containerPort}${p.protocol ? '/' + p.protocol : ''}${p.name ? ' (' + p.name + ')' : ''}`)
        .join(', ')
    }
    let stateStr = 'Unknown'
    let stateType: 'running' | 'waiting' | 'terminated' | 'unknown' = 'unknown'
    if (statC?.state?.running) {
      stateStr = 'Running'
      stateType = 'running'
    } else if (statC?.state?.waiting) {
      stateStr = statC.state.waiting.reason ? `Waiting: ${statC.state.waiting.reason}` : 'Waiting'
      stateType = 'waiting'
    } else if (statC?.state?.terminated) {
      const reason = statC.state.terminated.reason || `Exit ${statC.state.terminated.exitCode ?? 0}`
      stateStr = `Terminated (${reason})`
      stateType = 'terminated'
    } else if (statC?.ready) {
      stateStr = 'Ready'
      stateType = 'running'
    }
    results.push({
      name,
      image,
      ports: portsStr,
      state: stateStr,
      stateType,
      ready: Boolean(statC?.ready),
      restarts: Number(statC?.restartCount) || 0,
    })
  }
  return results
}

export function getDeploymentReplicas(row: K8sResource): string {
  const spec = row.spec as { replicas?: number } | undefined
  const status = row.status as { readyReplicas?: number; replicas?: number; updatedReplicas?: number; availableReplicas?: number } | undefined
  const ready = status?.readyReplicas || 0
  const desired = spec?.replicas ?? status?.replicas ?? 0
  return `${ready}/${desired}`
}

export function getDeploymentImage(row: K8sResource): string {
  const spec = row.spec as { template?: { spec?: { containers?: ContainerSpec[] } } } | undefined
  const containers = spec?.template?.spec?.containers
  return (Array.isArray(containers) && containers.length > 0) ? containers[0].image || 'unknown' : 'N/A'
}

export function getDeploymentSelector(row: K8sResource): string {
  const spec = row.spec as { selector?: { matchLabels?: Record<string, string> } } | undefined
  const labels = spec?.selector?.matchLabels
  if (labels && Object.keys(labels).length > 0) {
    return Object.entries(labels).map(([k, v]) => `${k}=${v}`).join(', ')
  }
  return 'N/A'
}

export function getStatefulSetReplicas(row: K8sResource): string {
  const spec = row.spec as { replicas?: number } | undefined
  const status = row.status as { readyReplicas?: number; replicas?: number } | undefined
  const ready = status?.readyReplicas || 0
  const desired = spec?.replicas ?? status?.replicas ?? 0
  return `${ready}/${desired}`
}

export function getStatefulSetImage(row: K8sResource): string {
  const spec = row.spec as { template?: { spec?: { containers?: ContainerSpec[] } } } | undefined
  const containers = spec?.template?.spec?.containers
  return containers?.[0]?.image || 'N/A'
}

export function getDaemonSetDesired(row: K8sResource): number {
  return (row.status as { desiredNumberScheduled?: number })?.desiredNumberScheduled || 0
}

export function getDaemonSetCurrent(row: K8sResource): number {
  return (row.status as { currentNumberScheduled?: number })?.currentNumberScheduled || 0
}

export function getDaemonSetReady(row: K8sResource): number {
  return (row.status as { numberReady?: number })?.numberReady || 0
}

export function getJobCompletions(row: K8sResource): string {
  const spec = row.spec as { completions?: number } | undefined
  const status = row.status as { succeeded?: number } | undefined
  return `${status?.succeeded || 0}/${spec?.completions || 1}`
}

export function getJobDuration(row: K8sResource): string {
  const status = row.status as { startTime?: string; completionTime?: string } | undefined
  if (!status?.startTime) return 'Pending'
  const start = new Date(status.startTime).getTime()
  const end = status.completionTime ? new Date(status.completionTime).getTime() : Date.now()
  const diffSec = Math.floor((end - start) / 1000)
  if (diffSec < 60) return `${diffSec}s`
  const mins = Math.floor(diffSec / 60)
  const secs = diffSec % 60
  return `${mins}m ${secs}s`
}

export function getJobStatus(row: K8sResource): string {
  const status = row.status as { succeeded?: number; failed?: number; active?: number } | undefined
  if (status?.succeeded && status.succeeded > 0) return 'Completed'
  if (status?.failed && status.failed > 0) return 'Failed'
  if (status?.active && status.active > 0) return 'Running'
  return 'Pending'
}

export function getCronJobSchedule(row: K8sResource): string {
  return (row.spec as { schedule?: string })?.schedule || 'N/A'
}

export function getCronJobSuspend(row: K8sResource): string {
  return (row.spec as { suspend?: boolean })?.suspend ? 'True' : 'False'
}

export function getCronJobActive(row: K8sResource): number {
  const status = row.status as { active?: unknown[] } | undefined
  return Array.isArray(status?.active) ? status.active.length : 0
}

export function getCronJobLastSchedule(row: K8sResource): string {
  const status = row.status as { lastScheduleTime?: string } | undefined
  return status?.lastScheduleTime ? formatAge(status.lastScheduleTime) : 'Never'
}
