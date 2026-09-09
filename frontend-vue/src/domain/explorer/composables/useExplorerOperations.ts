import { ref } from 'vue'
import { k8sApi } from '../../../api/k8s'
import { fleetApi, type Cluster } from '../../../api/fleet'
import type { DrainOptions, K8sNamespace, K8sResource, NodeTaint, ResourceKind, ToastMessage } from '../types'
import { useWorkloadOps } from '../domains/workloads/useWorkloadOps'
import { useNodeOps } from '../domains/cluster/useNodeOps'
import { useConfigOps } from '../domains/config/useConfigOps'

export function useExplorerOperations() {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const toastMessage = ref<ToastMessage | null>(null)
  const clusterOffline = ref(false)
  const offlineErrorMessage = ref<string | null>(null)

  const clusters = ref<Cluster[]>([])
  const selectedCluster = ref<string>('primary-cluster')
  const namespaces = ref<K8sNamespace[]>([])
  const selectedNamespace = ref<string>('all')
  const selectedKind = ref<ResourceKind>('pods')
  const resources = ref<K8sResource[]>([])

  const importingCluster = ref(false)

  const workloadOps = useWorkloadOps()
  const nodeOps = useNodeOps()
  const configOps = useConfigOps()

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    toastMessage.value = { text, type }
    setTimeout(() => {
      if (toastMessage.value?.text === text) {
        toastMessage.value = null
      }
    }, 4000)
  }

  async function loadClusters() {
    try {
      const res = await fleetApi.list()
      clusters.value = res || []
      if (clusters.value.length > 0 && (!selectedCluster.value || selectedCluster.value === 'primary-cluster')) {
        selectedCluster.value = clusters.value[0].name || clusters.value[0].id
      }
    } catch {
      // Keep default primary-cluster if fleetApi fails
    }
  }

  async function loadNamespaces() {
    if (!selectedCluster.value) return
    try {
      const res = await k8sApi.listNamespaces(selectedCluster.value)
      namespaces.value = res || []
    } catch {
      namespaces.value = []
    }
  }

  async function fetchResources() {
    if (!selectedCluster.value || !selectedKind.value) return
    loading.value = true
    error.value = null
    clusterOffline.value = false
    offlineErrorMessage.value = null

    try {
      const ns = selectedNamespace.value === 'all' ? undefined : selectedNamespace.value
      const res = await k8sApi.listResources(selectedCluster.value, selectedKind.value, ns)
      resources.value = res || []
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch resources'
      resources.value = []
      if (
        err instanceof Error &&
        (err.message.includes('offline') ||
          err.message.includes('fetch') ||
          err.message.includes('NetworkError') ||
          err.message.includes('502') ||
          err.message.includes('504'))
      ) {
        clusterOffline.value = true
        offlineErrorMessage.value = err.message
      }
    } finally {
      loading.value = false
    }
  }

  async function handleScaleConfirm(resource: K8sResource, replicas: number) {
    if (!resource.metadata?.name) return
    try {
      await workloadOps.scaleWorkload(selectedCluster.value, resource, replicas)
      showToast(`Scaled ${resource.metadata.name} to ${replicas} replicas`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to scale resource', 'error')
    }
  }

  async function handleRestartWorkload(resource: K8sResource) {
    if (!resource.metadata?.name) return
    const kind = resource.kind || 'Resource'
    const name = resource.metadata.name
    if (!confirm(`Are you sure you want to trigger a rolling restart for ${kind}/${name}?`)) return
    try {
      await workloadOps.restartWorkload(selectedCluster.value, resource)
      showToast(`Triggered rolling restart for ${kind}/${name}`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to restart workload', 'error')
    }
  }

  async function handleTriggerCronJob(resource: K8sResource) {
    if (!resource.metadata?.name) return
    const name = resource.metadata.name
    try {
      await workloadOps.triggerCronJob(selectedCluster.value, resource)
      showToast(`Triggered manual run for CronJob ${name}`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to trigger CronJob', 'error')
    }
  }

  async function handleToggleSuspendCronJob(resource: K8sResource) {
    if (!resource.metadata?.name) return
    const name = resource.metadata.name
    try {
      const newState = await workloadOps.suspendCronJob(selectedCluster.value, resource)
      showToast(`CronJob ${name} is now ${newState ? 'suspended' : 'active'}`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to update CronJob suspend state', 'error')
    }
  }

  async function handleDeleteConfirmed(resource: K8sResource) {
    if (!resource.metadata?.name) return
    const name = resource.metadata.name
    try {
      await configOps.deleteResource(selectedCluster.value, resource, selectedKind.value)
      showToast(`Deleted ${resource.kind || 'resource'} "${name}"`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to delete resource', 'error')
    }
  }

  async function handleCreateNamespace(name: string) {
    if (!name.trim()) return
    try {
      const created = await configOps.createNamespace(selectedCluster.value, name.trim())
      showToast(`Namespace "${created.name || name}" created!`)
      await loadNamespaces()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to create namespace', 'error')
    }
  }

  async function handleImportCluster(formData: FormData) {
    importingCluster.value = true
    try {
      await fleetApi.importCluster(formData)
    } finally {
      importingCluster.value = false
    }
  }

  async function handleCordonNode(node: K8sResource) {
    const name = node.metadata?.name
    if (!name) return
    try {
      await nodeOps.cordonNode(selectedCluster.value, name)
      showToast(`Node ${name} cordoned (SchedulingDisabled)`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to cordon node', 'error')
    }
  }

  async function handleUncordonNode(node: K8sResource) {
    const name = node.metadata?.name
    if (!name) return
    try {
      await nodeOps.uncordonNode(selectedCluster.value, name)
      showToast(`Node ${name} uncordoned`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to uncordon node', 'error')
    }
  }

  async function handleDrainNodeConfirm(node: K8sResource, options: DrainOptions) {
    const name = node.metadata?.name
    if (!name) return
    try {
      await nodeOps.drainNode(selectedCluster.value, name, options)
      showToast(`Node ${name} drain command issued successfully`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to drain node', 'error')
    }
  }

  async function handleAddTaint(node: K8sResource, taint: NodeTaint) {
    const name = node.metadata?.name
    if (!name) return
    try {
      await nodeOps.addTaint(selectedCluster.value, node, taint)
      showToast(`Added taint to node ${name}`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to add taint', 'error')
    }
  }

  async function handleRemoveTaint(node: K8sResource, index: number) {
    const name = node.metadata?.name
    if (!name) return
    try {
      await nodeOps.removeTaint(selectedCluster.value, node, index)
      showToast(`Removed taint from node ${name}`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to remove taint', 'error')
    }
  }

  async function handleSaveNodeLabels(node: K8sResource, labels: Record<string, string>) {
    const name = node.metadata?.name
    if (!name) return
    try {
      await nodeOps.saveNodeLabels(selectedCluster.value, node, labels)
      showToast(`Labels updated for node ${name}`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to update labels', 'error')
    }
  }

  async function handleSaveConfigMap(cm: K8sResource, data: Record<string, string>) {
    const name = cm.metadata?.name
    if (!name) return undefined
    try {
      const updated = await configOps.saveConfigMap(selectedCluster.value, cm, data)
      showToast(`ConfigMap ${name} saved successfully!`)
      await fetchResources()
      return updated
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to save ConfigMap', 'error')
      return undefined
    }
  }

  return {
    loading,
    error,
    toastMessage,
    clusterOffline,
    offlineErrorMessage,
    clusters,
    selectedCluster,
    namespaces,
    selectedNamespace,
    selectedKind,
    resources,
    scalingResource: workloadOps.scalingResource,
    savingConfigMap: configOps.savingConfigMap,
    creatingNs: configOps.creatingNs,
    deletingResource: configOps.deletingResource,
    importingCluster,
    operatingNode: nodeOps.operatingNode,
    drainingNode: nodeOps.drainingNode,
    updatingTaints: nodeOps.updatingTaints,
    updatingLabels: nodeOps.updatingLabels,
    showToast,
    loadClusters,
    loadNamespaces,
    fetchResources,
    handleScaleConfirm,
    handleRestartWorkload,
    handleTriggerCronJob,
    handleToggleSuspendCronJob,
    handleDeleteConfirmed,
    handleCreateNamespace,
    handleImportCluster,
    handleCordonNode,
    handleUncordonNode,
    handleDrainNodeConfirm,
    handleAddTaint,
    handleRemoveTaint,
    handleSaveNodeLabels,
    handleSaveConfigMap,
  }
}
