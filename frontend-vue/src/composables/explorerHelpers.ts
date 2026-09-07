import type {
  K8sResource,
  ResourceKind,
  NodeTaint,
  NodeCondition,
  NodeSystemInfo,
} from '../api/k8s'

interface ContainerPort { name?: string; containerPort: number; protocol?: string; hostPort?: number }
interface ContainerSpec { name: string; image?: string; ports?: ContainerPort[]; command?: string[]; args?: string[]; env?: { name: string; value?: string }[] }
interface ContainerStateWaiting { reason?: string; message?: string }
interface ContainerStateRunning { startedAt?: string }
interface ContainerStateTerminated { exitCode?: number; reason?: string; message?: string; startedAt?: string; finishedAt?: string }
interface ContainerState { waiting?: ContainerStateWaiting; running?: ContainerStateRunning; terminated?: ContainerStateTerminated }
interface ContainerStatus { name: string; image?: string; imageID?: string; ready?: boolean; restartCount?: number; started?: boolean; state?: ContainerState; lastState?: ContainerState }
interface MergedContainerInfo { name: string; image: string; ports: string; state: string; stateType: 'running' | 'waiting' | 'terminated' | 'unknown'; ready: boolean; restarts: number }

export interface KindCategory {
  title: string
  iconKey: string
  items: { label: string; kind: ResourceKind; iconKey: string }[]
}

export const kindCategories: KindCategory[] = [
  {
    title: 'Cluster',
    iconKey: 'server',
    items: [
      { label: 'Nodes', kind: 'nodes', iconKey: 'nodes' },
      { label: 'PersistentVolumes', kind: 'persistentvolumes', iconKey: 'pv' },
      { label: 'StorageClasses', kind: 'storageclasses', iconKey: 'sc' },
    ],
  },
  {
    title: 'Workloads',
    iconKey: 'workloads',
    items: [
      { label: 'Pods', kind: 'pods', iconKey: 'pods' },
      { label: 'Deployments', kind: 'deployments', iconKey: 'deployments' },
      { label: 'StatefulSets', kind: 'statefulsets', iconKey: 'statefulsets' },
      { label: 'DaemonSets', kind: 'daemonsets', iconKey: 'daemonsets' },
      { label: 'Jobs', kind: 'jobs', iconKey: 'jobs' },
      { label: 'CronJobs', kind: 'cronjobs', iconKey: 'cronjobs' },
    ],
  },
  {
    title: 'Config & Storage',
    iconKey: 'database',
    items: [
      { label: 'ConfigMaps', kind: 'configmaps', iconKey: 'configmaps' },
      { label: 'Secrets', kind: 'secrets', iconKey: 'secrets' },
      { label: 'PersistentVolumeClaims', kind: 'persistentvolumeclaims', iconKey: 'pvc' },
      { label: 'HorizontalPodAutoscalers', kind: 'horizontalpodautoscalers', iconKey: 'hpa' },
    ],
  },
  {
    title: 'Networking & Security',
    iconKey: 'network',
    items: [
      { label: 'Services', kind: 'services', iconKey: 'services' },
      { label: 'Ingresses', kind: 'ingresses', iconKey: 'ingresses' },
      { label: 'NetworkPolicies', kind: 'networkpolicies', iconKey: 'networkpolicies' },
      { label: 'ServiceAccounts', kind: 'serviceaccounts', iconKey: 'serviceaccounts' },
    ],
  },
  {
    title: 'Observability',
    iconKey: 'activity',
    items: [
      { label: 'Events', kind: 'events', iconKey: 'events' },
    ],
  },
]

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

export function getPodPhase(resource: K8sResource): string {
  if (resource.status && typeof resource.status === 'object') {
    const status = resource.status as Record<string, unknown>
    const containerStatuses = status.containerStatuses as ContainerStatus[] | undefined
    if (Array.isArray(containerStatuses)) {
      for (const cs of containerStatuses) {
        if (cs.state?.waiting?.reason) return cs.state.waiting.reason
        if (cs.state?.terminated?.reason && cs.state.terminated.reason !== 'Completed') return cs.state.terminated.reason
      }
    }
    if (typeof status.phase === 'string' && status.phase) return status.phase
  }
  return 'Unknown'
}

export function getPodReadyCount(resource: K8sResource): string {
  const spec = resource.spec as { containers?: ContainerSpec[] } | undefined
  const status = resource.status as { containerStatuses?: ContainerStatus[] } | undefined
  const total = Array.isArray(spec?.containers) ? spec.containers.length : Array.isArray(status?.containerStatuses) ? status.containerStatuses.length : 0
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
  if (Array.isArray(status?.podIPs) && status.podIPs.length > 0 && status.podIPs[0]?.ip) return status.podIPs[0].ip
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
      portsStr = specC.ports.map(p => `${p.containerPort}${p.protocol ? '/' + p.protocol : ''}${p.name ? ' (' + p.name + ')' : ''}`).join(', ')
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
    results.push({ name, image, ports: portsStr, state: stateStr, stateType, ready: Boolean(statC?.ready), restarts: Number(statC?.restartCount) || 0 })
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
  if (labels && Object.keys(labels).length > 0) return Object.entries(labels).map(([k, v]) => `${k}=${v}`).join(', ')
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

export function getServiceType(row: K8sResource): string {
  return (row.spec as { type?: string })?.type || 'ClusterIP'
}

export function getServiceClusterIP(row: K8sResource): string {
  return (row.spec as { clusterIP?: string })?.clusterIP || 'None'
}

export function getServiceExternalIP(row: K8sResource): string {
  const spec = row.spec as { externalIPs?: string[] } | undefined
  const status = row.status as { loadBalancer?: { ingress?: { ip?: string; hostname?: string }[] } } | undefined
  const lb = status?.loadBalancer?.ingress?.[0]
  if (lb?.ip) return lb.ip
  if (lb?.hostname) return lb.hostname
  if (spec?.externalIPs && spec.externalIPs.length > 0) return spec.externalIPs.join(', ')
  return 'None'
}

export function getServicePorts(row: K8sResource): string {
  const spec = row.spec as { ports?: { port: number; targetPort?: number | string; protocol?: string }[] } | undefined
  if (Array.isArray(spec?.ports) && spec.ports.length > 0) {
    return spec.ports.map(p => `${p.port}${p.targetPort ? ':' + p.targetPort : ''}/${p.protocol || 'TCP'}`).join(', ')
  }
  return 'None'
}

export function getIngressHosts(row: K8sResource): string {
  const spec = row.spec as { rules?: { host?: string }[] } | undefined
  if (Array.isArray(spec?.rules) && spec.rules.length > 0) {
    const hosts = spec.rules.map(r => r.host).filter(Boolean)
    return hosts.length > 0 ? hosts.join(', ') : '*'
  }
  return '*'
}

export function getIngressPaths(row: K8sResource): string {
  const spec = row.spec as { rules?: { http?: { paths?: { path?: string }[] } }[] } | undefined
  if (Array.isArray(spec?.rules)) {
    const paths = spec.rules.flatMap(r => r.http?.paths?.map(p => p.path)).filter(Boolean)
    return paths.length > 0 ? paths.join(', ') : '/'
  }
  return '/'
}

export function getConfigMapKeysCount(row: K8sResource): number {
  if (row.data) return Object.keys(row.data).length
  if (row.binaryData) return Object.keys(row.binaryData).length
  return 0
}

export function getConfigMapDataPreview(row: K8sResource): string {
  if (row.data) {
    const keys = Object.keys(row.data)
    if (keys.length === 0) return 'Empty'
    return keys.slice(0, 3).join(', ') + (keys.length > 3 ? ` (+${keys.length - 3} more)` : '')
  }
  return 'None'
}

export function getSecretType(row: K8sResource): string { return row.type || 'Opaque' }
export function getSecretKeysCount(row: K8sResource): number {
  if (row.data) return Object.keys(row.data).length
  if (row.stringData) return Object.keys(row.stringData).length
  return 0
}

export function getPvcStatus(row: K8sResource): string { return (row.status as { phase?: string })?.phase || 'Pending' }
export function getPvcCapacity(row: K8sResource): string {
  const status = row.status as { capacity?: { storage?: string } } | undefined
  const spec = row.spec as { resources?: { requests?: { storage?: string } } } | undefined
  return status?.capacity?.storage || spec?.resources?.requests?.storage || 'N/A'
}
export function getPvcAccessModes(row: K8sResource): string { return (row.spec as { accessModes?: string[] })?.accessModes?.join(', ') || 'N/A' }
export function getPvcStorageClass(row: K8sResource): string { return (row.spec as { storageClassName?: string })?.storageClassName || 'N/A' }
export function getPvcVolume(row: K8sResource): string { return (row.spec as { volumeName?: string })?.volumeName || 'N/A' }

export function getPvStatus(row: K8sResource): string { return (row.status as { phase?: string })?.phase || 'Available' }
export function getPvCapacity(row: K8sResource): string { return (row.spec as { capacity?: { storage?: string } })?.capacity?.storage || 'N/A' }
export function getPvAccessModes(row: K8sResource): string { return (row.spec as { accessModes?: string[] })?.accessModes?.join(', ') || 'N/A' }
export function getPvReclaimPolicy(row: K8sResource): string { return (row.spec as { persistentVolumeReclaimPolicy?: string })?.persistentVolumeReclaimPolicy || 'Retain' }
export function getPvStorageClass(row: K8sResource): string { return (row.spec as { storageClassName?: string })?.storageClassName || 'N/A' }
export function getPvClaim(row: K8sResource): string {
  const spec = row.spec as { claimRef?: { namespace?: string; name?: string } } | undefined
  return spec?.claimRef?.name ? `${spec.claimRef.namespace || 'default'}/${spec.claimRef.name}` : 'None'
}

export function getScProvisioner(row: K8sResource): string { return (row as { provisioner?: string }).provisioner || 'N/A' }
export function getScReclaimPolicy(row: K8sResource): string { return (row as { reclaimPolicy?: string }).reclaimPolicy || 'Delete' }
export function getScVolumeBindingMode(row: K8sResource): string { return (row as { volumeBindingMode?: string }).volumeBindingMode || 'Immediate' }
export function getScDefault(row: K8sResource): string { return row.metadata?.annotations?.['storageclass.kubernetes.io/is-default-class'] === 'true' ? 'Yes' : 'No' }

export function getNetPolPodSelector(row: K8sResource): string {
  const spec = row.spec as { podSelector?: { matchLabels?: Record<string, string> } } | undefined
  const matchLabels = spec?.podSelector?.matchLabels
  if (matchLabels && Object.keys(matchLabels).length > 0) return Object.entries(matchLabels).map(([k, v]) => `${k}=${v}`).join(', ')
  return 'All Pods'
}
export function getNetPolPolicyTypes(row: K8sResource): string { return (row.spec as { policyTypes?: string[] })?.policyTypes?.join(', ') || 'Ingress' }
export function getSaSecrets(row: K8sResource): number { const s = (row as { secrets?: unknown[] }).secrets; return Array.isArray(s) ? s.length : 0 }
export function getHpaReference(row: K8sResource): string {
  const spec = row.spec as { scaleTargetRef?: { kind?: string; name?: string } } | undefined
  return (spec?.scaleTargetRef?.kind && spec.scaleTargetRef?.name) ? `${spec.scaleTargetRef.kind}/${spec.scaleTargetRef.name}` : 'N/A'
}
export function getHpaTargets(row: K8sResource): string {
  const spec = row.spec as { targetCPUUtilizationPercentage?: number } | undefined
  const status = row.status as { currentCPUUtilizationPercentage?: number } | undefined
  return spec?.targetCPUUtilizationPercentage !== undefined ? `${status?.currentCPUUtilizationPercentage ?? 0}% / ${spec.targetCPUUtilizationPercentage}%` : 'N/A'
}
export function getHpaMinMax(row: K8sResource): string {
  const spec = row.spec as { minReplicas?: number; maxReplicas?: number } | undefined
  return `${spec?.minReplicas || 1}-${spec?.maxReplicas || 1}`
}
export function getHpaReplicas(row: K8sResource): number { return (row.status as { currentReplicas?: number })?.currentReplicas || 0 }

export function getNodeTaints(resource: K8sResource): NodeTaint[] {
  return ((resource.spec as { taints?: NodeTaint[] } | undefined)?.taints) || []
}

export function getEventType(resource: K8sResource): string {
  return (resource as unknown as { type?: string }).type || 'Normal'
}

export function getResourceStatus(resource: K8sResource): string {
  if (resource.status && typeof resource.status === 'object') {
    if ('phase' in resource.status && typeof resource.status.phase === 'string') return resource.status.phase
    if ('readyReplicas' in resource.status && 'replicas' in resource.status) return `${resource.status.readyReplicas || 0}/${resource.status.replicas || 0} Ready`
  }
  return 'Active'
}

export function isNodeUnschedulable(resource: K8sResource): boolean { return Boolean((resource.spec as { unschedulable?: boolean })?.unschedulable) }
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
export function getNodeRoles(resource: K8sResource): string[] {
  const labels = resource.metadata?.labels || {}
  const roles: string[] = []
  for (const key of Object.keys(labels)) {
    if (key.startsWith('node-role.kubernetes.io/')) {
      const role = key.replace('node-role.kubernetes.io/', '')
      if (role) roles.push(role)
    }
  }
  if (labels['kubernetes.io/role'] && !roles.includes(labels['kubernetes.io/role'])) roles.push(labels['kubernetes.io/role'])
  return roles.length > 0 ? roles : ['worker']
}
export function getNodeVersion(resource: K8sResource): string { return (resource.status as { nodeInfo?: NodeSystemInfo })?.nodeInfo?.kubeletVersion || 'N/A' }
export function getNodeInternalIP(resource: K8sResource): string {
  const addresses = (resource.status as { addresses?: { type: string; address: string }[] })?.addresses
  return Array.isArray(addresses) ? (addresses.find(a => a.type === 'InternalIP')?.address || 'N/A') : 'N/A'
}
export function getNodeOSArch(resource: K8sResource): string {
  const info = (resource.status as { nodeInfo?: NodeSystemInfo })?.nodeInfo
  return (info?.operatingSystem && info?.architecture) ? info.operatingSystem + '/' + info.architecture : info?.osImage || 'Linux'
}
export function getNodePodsCount(resource: K8sResource): string {
  const allocatable = (resource.status as { allocatable?: { pods?: string } })?.allocatable
  return allocatable?.pods ? allocatable.pods + ' max' : 'N/A'
}


