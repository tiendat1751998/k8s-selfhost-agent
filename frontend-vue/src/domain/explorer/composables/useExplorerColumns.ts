import { computed, type Ref } from 'vue'
import type { Column, K8sResource, ResourceKind } from '../types'
import {
  kindCategories,
  allKindItems,
  getFilteredKindCategories,
  filteredKindCategories,
} from '../categories'

import {
  podColumns,
  deploymentColumns,
  statefulSetColumns,
  daemonSetColumns,
  jobColumns,
  cronJobColumns,
  getPodPhase,
  getPodReadyCount,
  getPodRestarts,
  getPodNode,
  getPodIP,
  getHostIP,
  getPodQoS,
  getPodServiceAccount,
  getPodContainers,
  getDeploymentReplicas,
  getDeploymentImage,
  getDeploymentSelector,
  getStatefulSetReplicas,
  getStatefulSetImage,
  getDaemonSetDesired,
  getDaemonSetCurrent,
  getDaemonSetReady,
  getJobCompletions,
  getJobDuration,
  getJobStatus,
  getCronJobSchedule,
  getCronJobSuspend,
  getCronJobActive,
  getCronJobLastSchedule,
} from '../domains/workloads'

import {
  serviceColumns,
  ingressColumns,
  networkPolicyColumns,
  serviceAccountColumns,
  getServiceType,
  getServiceClusterIP,
  getServiceExternalIP,
  getServicePorts,
  getIngressHosts,
  getIngressPaths,
  getNetworkPolicyPodSelector,
  getNetPolPodSelector,
  getNetworkPolicyTypes,
  getNetPolPolicyTypes,
  getServiceAccountSecretsCount,
  getSaSecrets,
} from '../domains/networking'

import {
  pvcColumns,
  pvColumns,
  storageClassColumns,
  getPvcStatus,
  getPvcCapacity,
  getPvcAccessModes,
  getPvcStorageClass,
  getPvcVolume,
  getPvStatus,
  getPvCapacity,
  getPvAccessModes,
  getPvReclaimPolicy,
  getPvStorageClass,
  getPvClaim,
  getStorageClassProvisioner,
  getScProvisioner,
  getStorageClassReclaimPolicy,
  getScReclaimPolicy,
  getStorageClassVolumeBindingMode,
  getScVolumeBindingMode,
  getStorageClassDefault,
  getScDefault,
} from '../domains/storage'

import {
  nodeColumns,
  eventColumns,
  standardColumns,
  formatAge,
  getResourceAge,
  getNodeStatus,
  getNodeStatusBadge,
  isNodeUnschedulable,
  getNodeRoles,
  getNodeVersion,
  getNodeInternalIP,
  getNodeOSArch,
  getNodeOsArch,
  getNodePodsCount,
  getNodeTaints,
  getNodeConditions,
  getNodeSystemInfo,
  getNodeLabels,
  getEventType,
  getResourceStatus,
} from '../domains/cluster'

import {
  configMapColumns,
  secretColumns,
  hpaColumns,
  getConfigMapKeysCount,
  getConfigMapPreview,
  getConfigMapDataPreview,
  getSecretType,
  getSecretKeysCount,
  getHpaReference,
  getHpaTargets,
  getHpaMinMax,
  getHpaReplicas,
} from '../domains/config'

export {
  kindCategories,
  allKindItems,
  getFilteredKindCategories,
  filteredKindCategories,
  podColumns,
  deploymentColumns,
  statefulSetColumns,
  daemonSetColumns,
  jobColumns,
  cronJobColumns,
  getPodPhase,
  getPodReadyCount,
  getPodRestarts,
  getPodNode,
  getPodIP,
  getHostIP,
  getPodQoS,
  getPodServiceAccount,
  getPodContainers,
  getDeploymentReplicas,
  getDeploymentImage,
  getDeploymentSelector,
  getStatefulSetReplicas,
  getStatefulSetImage,
  getDaemonSetDesired,
  getDaemonSetCurrent,
  getDaemonSetReady,
  getJobCompletions,
  getJobDuration,
  getJobStatus,
  getCronJobSchedule,
  getCronJobSuspend,
  getCronJobActive,
  getCronJobLastSchedule,
  serviceColumns,
  ingressColumns,
  networkPolicyColumns,
  serviceAccountColumns,
  getServiceType,
  getServiceClusterIP,
  getServiceExternalIP,
  getServicePorts,
  getIngressHosts,
  getIngressPaths,
  getNetworkPolicyPodSelector,
  getNetPolPodSelector,
  getNetworkPolicyTypes,
  getNetPolPolicyTypes,
  getServiceAccountSecretsCount,
  getSaSecrets,
  pvcColumns,
  pvColumns,
  storageClassColumns,
  getPvcStatus,
  getPvcCapacity,
  getPvcAccessModes,
  getPvcStorageClass,
  getPvcVolume,
  getPvStatus,
  getPvCapacity,
  getPvAccessModes,
  getPvReclaimPolicy,
  getPvStorageClass,
  getPvClaim,
  getStorageClassProvisioner,
  getScProvisioner,
  getStorageClassReclaimPolicy,
  getScReclaimPolicy,
  getStorageClassVolumeBindingMode,
  getScVolumeBindingMode,
  getStorageClassDefault,
  getScDefault,
  nodeColumns,
  eventColumns,
  standardColumns,
  formatAge,
  getResourceAge,
  getNodeStatus,
  getNodeStatusBadge,
  isNodeUnschedulable,
  getNodeRoles,
  getNodeVersion,
  getNodeInternalIP,
  getNodeOSArch,
  getNodeOsArch,
  getNodePodsCount,
  getNodeTaints,
  getNodeConditions,
  getNodeSystemInfo,
  getNodeLabels,
  getEventType,
  getResourceStatus,
  configMapColumns,
  secretColumns,
  hpaColumns,
  getConfigMapKeysCount,
  getConfigMapPreview,
  getConfigMapDataPreview,
  getSecretType,
  getSecretKeysCount,
  getHpaReference,
  getHpaTargets,
  getHpaMinMax,
  getHpaReplicas,
}

export function getColumnsForKind(kind: ResourceKind): Column<K8sResource>[] {
  switch (kind) {
    case 'pods': return podColumns
    case 'deployments': return deploymentColumns
    case 'statefulsets': return statefulSetColumns
    case 'daemonsets': return daemonSetColumns
    case 'jobs': return jobColumns
    case 'cronjobs': return cronJobColumns
    case 'services': return serviceColumns
    case 'ingresses': return ingressColumns
    case 'configmaps': return configMapColumns
    case 'secrets': return secretColumns
    case 'persistentvolumeclaims': return pvcColumns
    case 'persistentvolumes': return pvColumns
    case 'storageclasses': return storageClassColumns
    case 'networkpolicies': return networkPolicyColumns
    case 'serviceaccounts': return serviceAccountColumns
    case 'horizontalpodautoscalers': return hpaColumns
    case 'nodes': return nodeColumns
    case 'events': return eventColumns
    default: return standardColumns
  }
}

export function useExplorerColumns(selectedKind: Ref<ResourceKind>) {
  const columns = computed<Column<K8sResource>[]>(() => getColumnsForKind(selectedKind.value))
  return { columns, kindCategories, allKindItems }
}
