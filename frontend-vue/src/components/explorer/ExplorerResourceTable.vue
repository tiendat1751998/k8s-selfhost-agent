<script setup lang="ts">
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { K8sResource, ResourceKind } from '../../api/k8s'
import {
  getResourceAge,
  getPodPhase,
  getPodReadyCount,
  getPodRestarts,
  getPodNode,
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
  getServiceType,
  getServiceClusterIP,
  getServiceExternalIP,
  getServicePorts,
  getIngressHosts,
  getIngressPaths,
  getConfigMapKeysCount,
  getConfigMapDataPreview,
  getSecretType,
  getSecretKeysCount,
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
  getScProvisioner,
  getScReclaimPolicy,
  getScVolumeBindingMode,
  getScDefault,
  getNetPolPodSelector,
  getNetPolPolicyTypes,
  getSaSecrets,
  getHpaReference,
  getHpaTargets,
  getHpaMinMax,
  getHpaReplicas,
  getEventType,
  getResourceStatus,
  isNodeUnschedulable,
  getNodeStatus,
  getNodeRoles,
  getNodeVersion,
  getNodeInternalIP,
  getNodeOSArch,
  getNodePodsCount,
} from '../../composables/useK8sExplorer'

defineProps<{
  columns: Column<K8sResource>[]
  resources: K8sResource[]
  loading: boolean
  error: string | null
  selectedKind: ResourceKind
  operatingNode: boolean
}>()

const emit = defineEmits<{
  (e: 'detail', res: K8sResource): void
  (e: 'logs', res: K8sResource): void
  (e: 'terminal', res: K8sResource): void
  (e: 'yaml', res: K8sResource): void
  (e: 'scale', res: K8sResource): void
  (e: 'restart', res: K8sResource): void
  (e: 'trigger-cronjob', res: K8sResource): void
  (e: 'toggle-suspend', res: K8sResource): void
  (e: 'cordon', res: K8sResource): void
  (e: 'uncordon', res: K8sResource): void
  (e: 'drain', res: K8sResource): void
  (e: 'delete', res: K8sResource): void
}>()

function toResource(row: unknown): K8sResource {
  return row as K8sResource
}
</script>

<template>
  <div class="section-box glass-panel table-box">
    <DataTable
      :columns="columns"
      :data="resources"
      :loading="loading"
      :error="error"
      empty-message="No Kubernetes resources discovered matching current filters."
      searchable
      search-placeholder="Filter by name, namespace, or status..."
    >
      <template #cell-name="{ row }">
        <div class="resource-name-cell">
          <a href="javascript:void(0)" class="res-link font-mono" @click="emit('detail', toResource(row))">{{ toResource(row).metadata?.name || 'unnamed' }}</a>
        </div>
      </template>
      <template #cell-namespace="{ row }"><span class="ns-badge font-mono">{{ toResource(row).metadata?.namespace || 'cluster-scoped' }}</span></template>
      <template #cell-status="{ row }">
        <StatusBadge :status="selectedKind === 'pods' ? getPodPhase(toResource(row)) : selectedKind === 'nodes' ? getNodeStatus(toResource(row)) : selectedKind === 'persistentvolumeclaims' ? getPvcStatus(toResource(row)) : selectedKind === 'persistentvolumes' ? getPvStatus(toResource(row)) : selectedKind === 'jobs' ? getJobStatus(toResource(row)) : getResourceStatus(toResource(row))" size="sm" />
      </template>
      <template #cell-ready="{ row }"><span class="font-mono ready-cell">{{ selectedKind === 'daemonsets' ? getDaemonSetReady(toResource(row)) : getPodReadyCount(toResource(row)) }}</span></template>
      <template #cell-restarts="{ row }"><span class="font-mono restarts-cell" :class="{ 'has-restarts': getPodRestarts(toResource(row)) > 0 }">{{ getPodRestarts(toResource(row)) }}</span></template>
      <template #cell-node="{ row }"><span class="font-mono text-muted font-small">{{ getPodNode(toResource(row)) }}</span></template>
      <template #cell-age="{ row }"><span class="font-mono text-muted font-small">{{ getResourceAge(toResource(row)) }}</span></template>
      <template #cell-replicas="{ row }"><span class="font-mono badge-replicas">{{ selectedKind === 'statefulsets' ? getStatefulSetReplicas(toResource(row)) : selectedKind === 'horizontalpodautoscalers' ? getHpaReplicas(toResource(row)) : getDeploymentReplicas(toResource(row)) }}</span></template>
      <template #cell-image="{ row }"><span class="font-mono text-cyan font-small cell-image-text">{{ selectedKind === 'statefulsets' ? getStatefulSetImage(toResource(row)) : getDeploymentImage(toResource(row)) }}</span></template>
      <template #cell-selector="{ row }"><span class="font-mono text-muted font-small">{{ getDeploymentSelector(toResource(row)) }}</span></template>
      <template #cell-desired="{ row }"><span class="font-mono">{{ getDaemonSetDesired(toResource(row)) }}</span></template>
      <template #cell-current="{ row }"><span class="font-mono">{{ getDaemonSetCurrent(toResource(row)) }}</span></template>
      <template #cell-completions="{ row }"><span class="font-mono">{{ getJobCompletions(toResource(row)) }}</span></template>
      <template #cell-duration="{ row }"><span class="font-mono text-muted">{{ getJobDuration(toResource(row)) }}</span></template>
      <template #cell-schedule="{ row }"><span class="font-mono schedule-badge">{{ getCronJobSchedule(toResource(row)) }}</span></template>
      <template #cell-suspend="{ row }"><span class="font-mono" :class="getCronJobSuspend(toResource(row)) === 'True' ? 'text-amber' : 'text-emerald'">{{ getCronJobSuspend(toResource(row)) }}</span></template>
      <template #cell-active="{ row }"><span class="font-mono">{{ getCronJobActive(toResource(row)) }}</span></template>
      <template #cell-lastSchedule="{ row }"><span class="font-mono text-muted">{{ getCronJobLastSchedule(toResource(row)) }}</span></template>
      <template #cell-type="{ row }"><span class="font-mono text-violet font-small font-bold">{{ selectedKind === 'services' ? getServiceType(toResource(row)) : selectedKind === 'secrets' ? getSecretType(toResource(row)) : getEventType(toResource(row)) }}</span></template>
      <template #cell-clusterIP="{ row }"><span class="font-mono text-muted font-small">{{ getServiceClusterIP(toResource(row)) }}</span></template>
      <template #cell-externalIP="{ row }"><span class="font-mono text-cyan font-small">{{ getServiceExternalIP(toResource(row)) }}</span></template>
      <template #cell-ports="{ row }"><span class="font-mono text-emerald font-small">{{ getServicePorts(toResource(row)) }}</span></template>
      <template #cell-hosts="{ row }"><span class="font-mono text-cyan font-small">{{ getIngressHosts(toResource(row)) }}</span></template>
      <template #cell-paths="{ row }"><span class="font-mono text-muted font-small">{{ getIngressPaths(toResource(row)) }}</span></template>
      <template #cell-keysCount="{ row }"><span class="font-mono badge-keys">{{ selectedKind === 'secrets' ? getSecretKeysCount(toResource(row)) : getConfigMapKeysCount(toResource(row)) }} keys</span></template>
      <template #cell-dataPreview="{ row }"><span class="font-mono text-muted font-small">{{ getConfigMapDataPreview(toResource(row)) }}</span></template>
      <template #cell-capacity="{ row }"><span class="font-mono text-cyan font-bold">{{ selectedKind === 'persistentvolumes' ? getPvCapacity(toResource(row)) : getPvcCapacity(toResource(row)) }}</span></template>
      <template #cell-accessModes="{ row }"><span class="font-mono text-muted font-small">{{ selectedKind === 'persistentvolumes' ? getPvAccessModes(toResource(row)) : getPvcAccessModes(toResource(row)) }}</span></template>
      <template #cell-storageClass="{ row }"><span class="font-mono text-violet font-small">{{ selectedKind === 'persistentvolumes' ? getPvStorageClass(toResource(row)) : getPvcStorageClass(toResource(row)) }}</span></template>
      <template #cell-volume="{ row }"><span class="font-mono text-muted font-small">{{ getPvcVolume(toResource(row)) }}</span></template>
      <template #cell-reclaimPolicy="{ row }"><span class="font-mono font-small">{{ selectedKind === 'storageclasses' ? getScReclaimPolicy(toResource(row)) : getPvReclaimPolicy(toResource(row)) }}</span></template>
      <template #cell-claim="{ row }"><span class="font-mono text-muted font-small">{{ getPvClaim(toResource(row)) }}</span></template>
      <template #cell-provisioner="{ row }"><span class="font-mono text-cyan font-small">{{ getScProvisioner(toResource(row)) }}</span></template>
      <template #cell-volumeBindingMode="{ row }"><span class="font-mono text-muted font-small">{{ getScVolumeBindingMode(toResource(row)) }}</span></template>
      <template #cell-defaultClass="{ row }"><span class="font-mono font-bold" :class="getScDefault(toResource(row)) === 'Yes' ? 'text-emerald' : 'text-muted'">{{ getScDefault(toResource(row)) }}</span></template>
      <template #cell-podSelector="{ row }"><span class="font-mono text-muted font-small">{{ getNetPolPodSelector(toResource(row)) }}</span></template>
      <template #cell-policyTypes="{ row }"><span class="font-mono text-violet font-small">{{ getNetPolPolicyTypes(toResource(row)) }}</span></template>
      <template #cell-secretsCount="{ row }"><span class="font-mono font-small">{{ getSaSecrets(toResource(row)) }} secrets</span></template>
      <template #cell-reference="{ row }"><span class="font-mono text-cyan font-small">{{ getHpaReference(toResource(row)) }}</span></template>
      <template #cell-targets="{ row }"><span class="font-mono text-emerald font-small">{{ getHpaTargets(toResource(row)) }}</span></template>
      <template #cell-minMax="{ row }"><span class="font-mono text-muted font-small">{{ getHpaMinMax(toResource(row)) }}</span></template>
      <template #cell-roles="{ row }">
        <div class="roles-wrap"><span v-for="r in getNodeRoles(toResource(row))" :key="r" class="role-badge font-mono font-small">{{ r }}</span></div>
      </template>
      <template #cell-version="{ row }"><span class="font-mono text-cyan font-small">{{ getNodeVersion(toResource(row)) }}</span></template>
      <template #cell-internalIP="{ row }"><span class="font-mono text-muted font-small">{{ getNodeInternalIP(toResource(row)) }}</span></template>
      <template #cell-osArch="{ row }"><span class="font-mono text-muted font-small">{{ getNodeOSArch(toResource(row)) }}</span></template>
      <template #cell-podsCount="{ row }"><span class="font-mono font-small">{{ getNodePodsCount(toResource(row)) }}</span></template>

      <!-- Standardized High-Density Action Toolbar (Compact 28x28px Buttons) -->
      <template #cell-actions="{ row }">
        <div class="action-toolbar">
          <template v-if="selectedKind === 'pods'">
            <button type="button" class="action-btn action-btn-cyan" title="Diagnostics & Details" @click="emit('detail', toResource(row))"><span>🔍</span></button>
            <button type="button" class="action-btn action-btn-amber" title="Stream Logs" @click="emit('logs', toResource(row))"><span>📄</span></button>
            <button type="button" class="action-btn action-btn-emerald" title="Terminal Exec" @click="emit('terminal', toResource(row))"><span>💻</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="View YAML Manifest" @click="emit('yaml', toResource(row))"><span>🎯</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete Pod" @click="emit('delete', toResource(row))"><span>✕</span></button>
          </template>
          <template v-else-if="selectedKind === 'nodes'">
            <button type="button" class="action-btn action-btn-cyan" title="Diagnostics & Details" @click="emit('detail', toResource(row))"><span>🔍</span></button>
            <button v-if="isNodeUnschedulable(toResource(row))" type="button" class="action-btn action-btn-emerald" :disabled="operatingNode" title="Uncordon Node" @click="emit('uncordon', toResource(row))"><span>🔓</span></button>
            <button v-else type="button" class="action-btn action-btn-amber" :disabled="operatingNode" title="Cordon Node" @click="emit('cordon', toResource(row))"><span>🚫</span></button>
            <button type="button" class="action-btn action-btn-danger" :disabled="operatingNode" title="Drain Node" @click="emit('drain', toResource(row))"><span>⚠️</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="View YAML Manifest" @click="emit('yaml', toResource(row))"><span>🎯</span></button>
          </template>
          <template v-else-if="selectedKind === 'deployments'">
            <button type="button" class="action-btn action-btn-cyan" title="Diagnostics & Details" @click="emit('detail', toResource(row))"><span>🔍</span></button>
            <button type="button" class="action-btn action-btn-emerald" title="Scale Replicas" @click="emit('scale', toResource(row))"><span>⚡</span></button>
            <button type="button" class="action-btn action-btn-amber" title="Restart Rollout" @click="emit('restart', toResource(row))"><span>🔄</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="View YAML Manifest" @click="emit('yaml', toResource(row))"><span>🎯</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete Deployment" @click="emit('delete', toResource(row))"><span>✕</span></button>
          </template>
          <template v-else-if="selectedKind === 'statefulsets'">
            <button type="button" class="action-btn action-btn-cyan" title="Diagnostics & Details" @click="emit('detail', toResource(row))"><span>🔍</span></button>
            <button type="button" class="action-btn action-btn-emerald" title="Scale Replicas" @click="emit('scale', toResource(row))"><span>⚡</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="View YAML Manifest" @click="emit('yaml', toResource(row))"><span>🎯</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete StatefulSet" @click="emit('delete', toResource(row))"><span>✕</span></button>
          </template>
          <template v-else-if="selectedKind === 'cronjobs'">
            <button type="button" class="action-btn action-btn-cyan" title="Diagnostics & Details" @click="emit('detail', toResource(row))"><span>🔍</span></button>
            <button type="button" class="action-btn action-btn-emerald" title="Trigger Job" @click="emit('trigger-cronjob', toResource(row))"><span>⚡</span></button>
            <button type="button" class="action-btn" :class="(toResource(row).spec as { suspend?: boolean })?.suspend ? 'action-btn-emerald' : 'action-btn-amber'" :title="(toResource(row).spec as { suspend?: boolean })?.suspend ? 'Resume CronJob' : 'Suspend CronJob'" @click="emit('toggle-suspend', toResource(row))"><span>{{ (toResource(row).spec as { suspend?: boolean })?.suspend ? '▶' : '⏸' }}</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="View YAML Manifest" @click="emit('yaml', toResource(row))"><span>🎯</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete CronJob" @click="emit('delete', toResource(row))"><span>✕</span></button>
          </template>
          <template v-else>
            <button type="button" class="action-btn action-btn-cyan" title="Diagnostics & Details" @click="emit('detail', toResource(row))"><span>🔍</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="View YAML Manifest" @click="emit('yaml', toResource(row))"><span>🎯</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete Resource" @click="emit('delete', toResource(row))"><span>✕</span></button>
          </template>
        </div>
      </template>
    </DataTable>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
