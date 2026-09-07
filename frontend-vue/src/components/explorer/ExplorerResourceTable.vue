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

      <!-- Single-Line Action Toolbar (28px height, 4px gap) -->
      <template #cell-actions="{ row }">
        <div class="action-toolbar">
          <template v-if="selectedKind === 'pods'">
            <button type="button" class="action-btn action-btn-cyan" title="Details" @click="emit('detail', toResource(row))"><span class="action-text">Details</span></button>
            <button type="button" class="action-btn action-btn-amber" title="Logs" @click="emit('logs', toResource(row))"><span class="action-text">Logs</span></button>
            <button type="button" class="action-btn action-btn-emerald" title="Exec" @click="emit('terminal', toResource(row))"><span class="action-text">Exec</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="YAML" @click="emit('yaml', toResource(row))"><span class="action-text">YAML</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete" @click="emit('delete', toResource(row))"><span class="action-text">✕</span></button>
          </template>
          <template v-else-if="selectedKind === 'nodes'">
            <button type="button" class="action-btn action-btn-cyan" title="Details" @click="emit('detail', toResource(row))"><span class="action-text">Details</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="YAML" @click="emit('yaml', toResource(row))"><span class="action-text">YAML</span></button>
            <button v-if="isNodeUnschedulable(toResource(row))" type="button" class="action-btn action-btn-emerald" :disabled="operatingNode" title="Uncordon" @click="emit('uncordon', toResource(row))"><span class="action-text">Uncordon</span></button>
            <button v-else type="button" class="action-btn action-btn-amber" :disabled="operatingNode" title="Cordon" @click="emit('cordon', toResource(row))"><span class="action-text">Cordon</span></button>
            <button type="button" class="action-btn action-btn-danger" :disabled="operatingNode" title="Drain" @click="emit('drain', toResource(row))"><span class="action-text">Drain</span></button>
          </template>
          <template v-else-if="selectedKind === 'deployments'">
            <button type="button" class="action-btn action-btn-emerald" title="Scale" @click="emit('scale', toResource(row))"><span class="action-text">Scale</span></button>
            <button type="button" class="action-btn action-btn-amber" title="Restart" @click="emit('restart', toResource(row))"><span class="action-text">Restart</span></button>
            <button type="button" class="action-btn action-btn-cyan" title="Details" @click="emit('detail', toResource(row))"><span class="action-text">Details</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="YAML" @click="emit('yaml', toResource(row))"><span class="action-text">YAML</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete" @click="emit('delete', toResource(row))"><span class="action-text">✕</span></button>
          </template>
          <template v-else-if="selectedKind === 'statefulsets'">
            <button type="button" class="action-btn action-btn-emerald" title="Scale" @click="emit('scale', toResource(row))"><span class="action-text">Scale</span></button>
            <button type="button" class="action-btn action-btn-cyan" title="Details" @click="emit('detail', toResource(row))"><span class="action-text">Details</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="YAML" @click="emit('yaml', toResource(row))"><span class="action-text">YAML</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete" @click="emit('delete', toResource(row))"><span class="action-text">✕</span></button>
          </template>
          <template v-else-if="selectedKind === 'cronjobs'">
            <button type="button" class="action-btn action-btn-emerald" title="Trigger" @click="emit('trigger-cronjob', toResource(row))"><span class="action-text">Trigger</span></button>
            <button type="button" class="action-btn" :class="(toResource(row).spec as { suspend?: boolean })?.suspend ? 'action-btn-emerald' : 'action-btn-amber'" title="Toggle Suspend" @click="emit('toggle-suspend', toResource(row))"><span class="action-text">{{ (toResource(row).spec as { suspend?: boolean })?.suspend ? 'Resume' : 'Suspend' }}</span></button>
            <button type="button" class="action-btn action-btn-cyan" title="Details" @click="emit('detail', toResource(row))"><span class="action-text">Details</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="YAML" @click="emit('yaml', toResource(row))"><span class="action-text">YAML</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete" @click="emit('delete', toResource(row))"><span class="action-text">✕</span></button>
          </template>
          <template v-else>
            <button type="button" class="action-btn action-btn-cyan" title="Details" @click="emit('detail', toResource(row))"><span class="action-text">Details</span></button>
            <button type="button" class="action-btn action-btn-secondary" title="YAML" @click="emit('yaml', toResource(row))"><span class="action-text">YAML</span></button>
            <button type="button" class="action-btn action-btn-danger" title="Delete" @click="emit('delete', toResource(row))"><span class="action-text">✕</span></button>
          </template>
        </div>
      </template>
    </DataTable>
  </div>
</template>

<style scoped>
.table-box { padding: 16px; border-radius: 16px; }
.resource-name-cell { display: flex; align-items: center; gap: 8px; }
.res-link { color: #38bdf8; text-decoration: none; font-weight: 600; }
.res-link:hover { text-decoration: underline; }
.ns-badge { background: rgba(139, 92, 246, 0.12); color: #c084fc; padding: 2px 7px; border-radius: 4px; font-size: 0.75rem; }
.badge-replicas { color: #38bdf8; font-weight: 700; }
.badge-keys { color: #fbbf24; }
.schedule-badge { background: rgba(56, 189, 248, 0.12); color: #38bdf8; padding: 2px 6px; border-radius: 4px; font-size: 0.75rem; }
.cell-image-text { max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: inline-block; }
.roles-wrap { display: flex; gap: 4px; flex-wrap: wrap; }
.role-badge { background: rgba(6, 182, 212, 0.1); border: 1px solid rgba(6, 182, 212, 0.25); color: #38bdf8; padding: 1px 6px; border-radius: 4px; font-size: 0.7rem; }
.ready-cell { font-weight: 600; color: #34d399; }
.restarts-cell.has-restarts { color: #fb7185; font-weight: 700; }

.action-toolbar { display: inline-flex; align-items: center; justify-content: flex-end; gap: 4px; flex-wrap: nowrap; }
.action-btn { height: 28px; padding: 0 8px; border-radius: 6px; font-size: 0.72rem; font-weight: 600; font-family: var(--font-mono); display: inline-flex; align-items: center; justify-content: center; cursor: pointer; border: 1px solid rgba(255, 255, 255, 0.08); background: rgba(255, 255, 255, 0.04); color: #94a3b8; }
.action-btn:hover:not(:disabled) { transform: translateY(-1px); }
.action-btn-cyan { background: rgba(6, 182, 212, 0.08); border-color: rgba(6, 182, 212, 0.25); color: #38bdf8; }
.action-btn-amber { background: rgba(245, 158, 11, 0.08); border-color: rgba(245, 158, 11, 0.25); color: #fbbf24; }
.action-btn-emerald { background: rgba(16, 185, 129, 0.08); border-color: rgba(16, 185, 129, 0.25); color: #34d399; }
.action-btn-secondary { background: rgba(99, 102, 241, 0.08); border-color: rgba(99, 102, 241, 0.25); color: #a5b4fc; }
.action-btn-danger { background: rgba(244, 63, 94, 0.08); border-color: rgba(244, 63, 94, 0.25); color: #fb7185; }
</style>
