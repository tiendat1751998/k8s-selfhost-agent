import type { K8sResource } from '../../types'

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
    return spec.ports
      .map(p => `${p.port}${p.targetPort ? ':' + p.targetPort : ''}/${p.protocol || 'TCP'}`)
      .join(', ')
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

export function getNetworkPolicyPodSelector(row: K8sResource): string {
  const spec = row.spec as { podSelector?: { matchLabels?: Record<string, string> } } | undefined
  const matchLabels = spec?.podSelector?.matchLabels
  if (matchLabels && Object.keys(matchLabels).length > 0) {
    return Object.entries(matchLabels).map(([k, v]) => `${k}=${v}`).join(', ')
  }
  return 'All Pods'
}

export const getNetPolPodSelector = getNetworkPolicyPodSelector

export function getNetworkPolicyTypes(row: K8sResource): string {
  return (row.spec as { policyTypes?: string[] })?.policyTypes?.join(', ') || 'Ingress'
}

export const getNetPolPolicyTypes = getNetworkPolicyTypes

export function getServiceAccountSecretsCount(row: K8sResource): number {
  const s = (row as { secrets?: unknown[] }).secrets
  return Array.isArray(s) ? s.length : 0
}

export const getSaSecrets = getServiceAccountSecretsCount
