import type { K8sResource } from '../../types'

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

export const getConfigMapPreview = getConfigMapDataPreview

export function getSecretType(row: K8sResource): string {
  return row.type || 'Opaque'
}

export function getSecretKeysCount(row: K8sResource): number {
  if (row.data) return Object.keys(row.data).length
  if (row.stringData) return Object.keys(row.stringData).length
  return 0
}

export function getHpaReference(row: K8sResource): string {
  const spec = row.spec as { scaleTargetRef?: { kind?: string; name?: string } } | undefined
  return (spec?.scaleTargetRef?.kind && spec.scaleTargetRef?.name)
    ? `${spec.scaleTargetRef.kind}/${spec.scaleTargetRef.name}`
    : 'N/A'
}

export function getHpaTargets(row: K8sResource): string {
  const spec = row.spec as { targetCPUUtilizationPercentage?: number } | undefined
  const status = row.status as { currentCPUUtilizationPercentage?: number } | undefined
  return spec?.targetCPUUtilizationPercentage !== undefined
    ? `${status?.currentCPUUtilizationPercentage ?? 0}% / ${spec.targetCPUUtilizationPercentage}%`
    : 'N/A'
}

export function getHpaMinMax(row: K8sResource): string {
  const spec = row.spec as { minReplicas?: number; maxReplicas?: number } | undefined
  return `${spec?.minReplicas || 1}-${spec?.maxReplicas || 1}`
}

export function getHpaReplicas(row: K8sResource): number {
  return (row.status as { currentReplicas?: number })?.currentReplicas || 0
}
