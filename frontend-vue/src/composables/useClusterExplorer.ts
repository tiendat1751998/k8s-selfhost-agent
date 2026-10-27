export interface ParsedExplorerResult {
  items: any[]
  total: number
  isDegraded: boolean
  degradedReason: string
}

export function parseExplorerResponse(res: any): ParsedExplorerResult {
  const isDegraded = Boolean(res?.k8s_degraded)
  const defaultReason = 'kube-apiserver (:6443) connection refused — control plane offline'
  const degradedReason = res?.degraded_reason || (isDegraded ? defaultReason : '')
  const items = Array.isArray(res?.data) ? res.data : (Array.isArray(res) ? res : [])
  const total = typeof res?.total === 'number' ? res.total : items.length

  return {
    items,
    total,
    isDegraded,
    degradedReason,
  }
}

export { useK8sExplorer as useClusterExplorer } from './useK8sExplorer'
