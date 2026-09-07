import type { K8sResource } from '../../types'

export function getPvcStatus(row: K8sResource): string {
  return (row.status as { phase?: string })?.phase || 'Pending'
}

export function getPvcCapacity(row: K8sResource): string {
  const status = row.status as { capacity?: { storage?: string } } | undefined
  const spec = row.spec as { resources?: { requests?: { storage?: string } } } | undefined
  return status?.capacity?.storage || spec?.resources?.requests?.storage || 'N/A'
}

export function getPvcAccessModes(row: K8sResource): string {
  return (row.spec as { accessModes?: string[] })?.accessModes?.join(', ') || 'N/A'
}

export function getPvcStorageClass(row: K8sResource): string {
  return (row.spec as { storageClassName?: string })?.storageClassName || 'N/A'
}

export function getPvcVolume(row: K8sResource): string {
  return (row.spec as { volumeName?: string })?.volumeName || 'N/A'
}

export function getPvStatus(row: K8sResource): string {
  return (row.status as { phase?: string })?.phase || 'Available'
}

export function getPvCapacity(row: K8sResource): string {
  return (row.spec as { capacity?: { storage?: string } })?.capacity?.storage || 'N/A'
}

export function getPvAccessModes(row: K8sResource): string {
  return (row.spec as { accessModes?: string[] })?.accessModes?.join(', ') || 'N/A'
}

export function getPvReclaimPolicy(row: K8sResource): string {
  return (row.spec as { persistentVolumeReclaimPolicy?: string })?.persistentVolumeReclaimPolicy || 'Retain'
}

export function getPvStorageClass(row: K8sResource): string {
  return (row.spec as { storageClassName?: string })?.storageClassName || 'N/A'
}

export function getPvClaim(row: K8sResource): string {
  const spec = row.spec as { claimRef?: { namespace?: string; name?: string } } | undefined
  return spec?.claimRef?.name ? `${spec.claimRef.namespace || 'default'}/${spec.claimRef.name}` : 'None'
}

export function getScProvisioner(row: K8sResource): string {
  return (row as { provisioner?: string }).provisioner || 'N/A'
}

export const getStorageClassProvisioner = getScProvisioner

export function getScReclaimPolicy(row: K8sResource): string {
  return (row as { reclaimPolicy?: string }).reclaimPolicy || 'Delete'
}

export const getStorageClassReclaimPolicy = getScReclaimPolicy

export function getScVolumeBindingMode(row: K8sResource): string {
  return (row as { volumeBindingMode?: string }).volumeBindingMode || 'Immediate'
}

export const getStorageClassVolumeBindingMode = getScVolumeBindingMode

export function getScDefault(row: K8sResource): string {
  return row.metadata?.annotations?.['storageclass.kubernetes.io/is-default-class'] === 'true' ? 'Yes' : 'No'
}

export const getStorageClassDefault = getScDefault
