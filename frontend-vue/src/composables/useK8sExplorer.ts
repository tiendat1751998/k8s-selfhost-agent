import { ref, computed } from 'vue'
import { k8sApi, type K8sResource, type K8sNamespace, type ResourceKind } from '../api/k8s'
import { fleetApi, type Cluster } from '../api/compute'
import type { Column } from '../components/ui/DataTable.vue'
import { jsonToYaml } from '../utils/yaml'
export * from './explorerHelpers'
export * from './explorerColumns'
export type { DrainOptions } from '../domain/explorer/types'
import { kindCategories, type KindCategory } from './explorerHelpers'
import {
  podColumns, deploymentColumns, statefulSetColumns, daemonSetColumns,
  jobColumns, cronJobColumns, serviceColumns, ingressColumns,
  configMapColumns, secretColumns, pvcColumns, pvColumns,
  storageClassColumns, networkPolicyColumns, serviceAccountColumns,
  hpaColumns, nodeColumns, eventColumns, standardColumns
} from './explorerColumns'
import type { DrainOptions } from '../domain/explorer/types'

export function useK8sExplorer() {
  // Async loading & error states
  const loading = ref(false)
  const error = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
  const clusterOffline = ref(false)
  const offlineErrorMessage = ref<string | null>(null)

  // Core State
  const clusters = ref<Cluster[]>([])
  const selectedCluster = ref<string>('default')
  const namespaces = ref<K8sNamespace[]>([])
  const selectedNamespace = ref<string>('all')
  const selectedKind = ref<ResourceKind>('deployments')
  const resources = ref<K8sResource[]>([])

  // Sidebar Search & Mobile Responsive State
  const kindSearchQuery = ref('')
  const isMobileSidebarOpen = ref(false)

  // Filtered categories based on quick search
  const filteredKindCategories = computed(() => {
    const q = kindSearchQuery.value.trim().toLowerCase()
    if (!q) return kindCategories

    return kindCategories
      .map((category: KindCategory) => {
        const matchingItems = category.items.filter(
          item => item.label.toLowerCase().includes(q) || item.kind.toLowerCase().includes(q)
        )
        return {
          ...category,
          items: matchingItems,
        }
      })
      .filter((category: KindCategory) => category.items.length > 0)
  })

  // Flattened kinds list for mobile horizontal pill carousel
  const allKindItems = computed(() => {
    return kindCategories.flatMap((c: KindCategory) => c.items)
  })

  // Metrics computation
  const totalInKind = computed(() => resources.value.length)
  const activeNamespacesCount = computed(() => namespaces.value.length || 1)
  const currentKindLabel = computed(() => {
    for (const cat of kindCategories) {
      for (const item of cat.items) {
        if (item.kind === selectedKind.value) return item.label
      }
    }
    return selectedKind.value
  })

  const columns = computed<Column<K8sResource>[]>(() => {
    switch (selectedKind.value) {
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
  })

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    toastMessage.value = { text, type }
    setTimeout(() => {
      if (toastMessage.value?.text === text) {
        toastMessage.value = null
      }
    }, 4000)
  }

  function selectKind(kind: ResourceKind) {
    selectedKind.value = kind
    isMobileSidebarOpen.value = false
  }

  // Fetch Clusters
  async function loadClusters() {
    try {
      const list = await fleetApi.list()
      if (list && list.length > 0) {
        clusters.value = list
        if (!selectedCluster.value || selectedCluster.value === 'default' || !list.some(c => (c.name || c.id) === selectedCluster.value)) {
          selectedCluster.value = list[0].name || list[0].id
        }
      } else {
        clusters.value = []
        if (!selectedCluster.value || selectedCluster.value === 'default') {
          selectedCluster.value = 'primary-cluster'
        }
      }
    } catch {
      clusters.value = []
      if (!selectedCluster.value || selectedCluster.value === 'default') {
        selectedCluster.value = 'primary-cluster'
      }
    }
  }

  // Fetch Namespaces for Selected Cluster
  async function loadNamespaces() {
    if (!selectedCluster.value) {
      namespaces.value = []
      return
    }
    try {
      const nsList = await k8sApi.listNamespaces(selectedCluster.value)
      namespaces.value = Array.isArray(nsList) ? nsList : []
    } catch {
      namespaces.value = []
    }
  }

  // Fetch Resources (ZERO MOCK FALLBACK DATA)
  async function fetchResources() {
    if (!selectedCluster.value) {
      resources.value = []
      return
    }
    loading.value = true
    error.value = null
    clusterOffline.value = false
    offlineErrorMessage.value = null

    try {
      const ns = selectedNamespace.value !== 'all' ? selectedNamespace.value : undefined
      const list = await k8sApi.listResources(selectedCluster.value, selectedKind.value, ns)
      if (Array.isArray(list)) {
        resources.value = list
      } else {
        resources.value = []
      }
    } catch (err: unknown) {
      resources.value = []
      clusterOffline.value = true
      const msg = err instanceof Error ? err.message : 'Failed to query Kubernetes cluster'
      offlineErrorMessage.value = msg
    } finally {
      loading.value = false
    }
  }

  // Modals & Drawers state
  const showCreateModal = ref(false)
  const showYamlModal = ref(false)
  const yamlEditorMode = ref<'create' | 'edit'>('create')
  const yamlEditorInitialContent = ref('')
  const yamlEditorTitle = ref('Apply Kubernetes Manifest')
  const showDetailDrawer = ref(false)
  const selectedResource = ref<K8sResource | null>(null)
  const showScaleModal = ref(false)
  const scaleTarget = ref<K8sResource | null>(null)
  const scalingResource = ref(false)
  const showRestartModal = ref(false)
  const restartTarget = ref<K8sResource | null>(null)
  const restartingResource = ref(false)
  const showDeleteModal = ref(false)
  const resourceToDelete = ref<K8sResource | null>(null)
  const deletingResource = ref(false)
  const showDrainModal = ref(false)
  const drainTargetNode = ref<K8sResource | null>(null)
  const drainingNode = ref(false)
  const operatingNode = ref(false)
  const showNewNsModal = ref(false)
  const creatingNs = ref(false)
  const newNsError = ref<string | null>(null)
  const showImportModal = ref(false)
  const importingCluster = ref(false)
  const showLogsDrawer = ref(false)
  const logsPod = ref<K8sResource | null>(null)
  const showTerminalDrawer = ref(false)
  const terminalPod = ref<K8sResource | null>(null)

  function openDetailDrawer(res: K8sResource) {
    selectedResource.value = res
    showDetailDrawer.value = true
  }
  function openLogsDrawer(res: K8sResource) {
    logsPod.value = res
    showLogsDrawer.value = true
  }
  function openTerminalDrawer(res: K8sResource) {
    terminalPod.value = res
    showTerminalDrawer.value = true
  }
  function openScaleModal(res: K8sResource) {
    scaleTarget.value = res
    showScaleModal.value = true
  }
  function openRestartModal(res: K8sResource) {
    restartTarget.value = res
    showRestartModal.value = true
  }
  function openDeleteModal(res: K8sResource) {
    resourceToDelete.value = res
    showDeleteModal.value = true
  }
  function openDrainModal(res: K8sResource) {
    drainTargetNode.value = res
    showDrainModal.value = true
  }
  function openApplyYamlModal() {
    yamlEditorMode.value = 'create'
    yamlEditorInitialContent.value = ''
    yamlEditorTitle.value = 'Apply Kubernetes Manifest (YAML)'
    showYamlModal.value = true
  }
  function openYamlEditModal(res: K8sResource) {
    yamlEditorMode.value = 'edit'
    yamlEditorInitialContent.value = jsonToYaml(res)
    yamlEditorTitle.value = `Edit ${res.kind || 'Resource'}: ${res.metadata?.name || ''}`
    showYamlModal.value = true
  }

  async function handleScaleConfirm(replicas: number) {
    if (!scaleTarget.value) return
    scalingResource.value = true
    try {
      const kind = (scaleTarget.value.kind?.toLowerCase() || 'deployments') as ResourceKind
      const name = scaleTarget.value.metadata?.name || ''
      await k8sApi.scaleWorkload(selectedCluster.value, kind, name, replicas, scaleTarget.value.metadata?.namespace)
      showToast(`Scaled ${name} to ${replicas} replicas`)
      showScaleModal.value = false
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to scale resource', 'error')
    } finally {
      scalingResource.value = false
    }
  }

  async function handleRestartConfirm(target: K8sResource) {
    restartingResource.value = true
    try {
      const kind = (target.kind?.toLowerCase() || 'deployments') as ResourceKind
      const name = target.metadata?.name || ''
      await k8sApi.restartResource(selectedCluster.value, kind, name, target.metadata?.namespace)
      showToast(`Triggered rolling restart for ${name}`)
      showRestartModal.value = false
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to restart workload', 'error')
    } finally {
      restartingResource.value = false
    }
  }

  async function handleDeleteConfirm(resource: K8sResource) {
    deletingResource.value = true
    try {
      const kind = (resource.kind?.toLowerCase() || selectedKind.value) as ResourceKind
      const name = resource.metadata?.name || ''
      await k8sApi.deleteResource(selectedCluster.value, kind, name, resource.metadata?.namespace)
      showToast(`Deleted ${resource.kind || 'resource'} "${name}"`)
      showDeleteModal.value = false
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to delete resource', 'error')
    } finally {
      deletingResource.value = false
    }
  }

  async function handleDrainConfirm(options: DrainOptions) {
    if (!drainTargetNode.value?.metadata?.name) return
    drainingNode.value = true
    try {
      const name = drainTargetNode.value.metadata.name
      await k8sApi.drainNode(selectedCluster.value, name, options)
      showToast(`Drain initiated for node ${name}`)
      showDrainModal.value = false
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to drain node', 'error')
    } finally {
      drainingNode.value = false
    }
  }

  async function handleCordonNode(node: K8sResource) {
    if (!node.metadata?.name) return
    operatingNode.value = true
    try {
      await k8sApi.cordonNode(selectedCluster.value, node.metadata.name)
      showToast(`Node ${node.metadata.name} cordoned`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to cordon node', 'error')
    } finally {
      operatingNode.value = false
    }
  }

  async function handleUncordonNode(node: K8sResource) {
    if (!node.metadata?.name) return
    operatingNode.value = true
    try {
      await k8sApi.uncordonNode(selectedCluster.value, node.metadata.name)
      showToast(`Node ${node.metadata.name} uncordoned`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to uncordon node', 'error')
    } finally {
      operatingNode.value = false
    }
  }

  async function handleTriggerCronJob(res: K8sResource) {
    if (!res.metadata?.name) return
    try {
      await k8sApi.triggerCronJob(selectedCluster.value, res.metadata.name, res.metadata?.namespace)
      showToast(`Triggered CronJob ${res.metadata.name}`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to trigger CronJob', 'error')
    }
  }

  async function handleToggleSuspend(res: K8sResource) {
    if (!res.metadata?.name) return
    try {
      const isSuspended = (res.spec as { suspend?: boolean })?.suspend === true
      await k8sApi.toggleCronJobSuspend(selectedCluster.value, res.metadata.name, !isSuspended, res.metadata?.namespace)
      showToast(`CronJob ${res.metadata.name} ${!isSuspended ? 'suspended' : 'resumed'}`)
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to toggle CronJob', 'error')
    }
  }

  async function handleImportCluster(formData: FormData, name: string) {
    importingCluster.value = true
    try {
      await fleetApi.importCluster(formData)
      showToast(`Cluster ${name} successfully imported!`)
      showImportModal.value = false
      await loadClusters()
      selectedCluster.value = name
      await loadNamespaces()
      await fetchResources()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Cluster import failed', 'error')
    } finally {
      importingCluster.value = false
    }
  }

  async function handleCreateNs(name: string) {
    creatingNs.value = true
    newNsError.value = null
    try {
      await k8sApi.createNamespace(selectedCluster.value, name)
      showToast(`Namespace "${name}" created!`)
      showNewNsModal.value = false
      await loadNamespaces()
      selectedNamespace.value = name
      await fetchResources()
    } catch (err: unknown) {
      newNsError.value = err instanceof Error ? err.message : 'Failed to create namespace'
      showToast(newNsError.value, 'error')
    } finally {
      creatingNs.value = false
    }
  }

  function handleYamlApplied(result: { message: string }) {
    showYamlModal.value = false
    showToast(result.message || 'Manifest applied successfully')
    fetchResources()
  }

  function handleCreateApplied() {
    showCreateModal.value = false
    showToast('Resource created successfully!')
    fetchResources()
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
    kindSearchQuery,
    isMobileSidebarOpen,
    kindCategories,
    filteredKindCategories,
    allKindItems,
    totalInKind,
    activeNamespacesCount,
    currentKindLabel,
    columns,
    loadClusters,
    loadNamespaces,
    fetchResources,
    selectKind,
    showToast,
    // Modals state & handlers
    showCreateModal,
    showYamlModal,
    yamlEditorMode,
    yamlEditorInitialContent,
    yamlEditorTitle,
    showDetailDrawer,
    selectedResource,
    showScaleModal,
    scaleTarget,
    scalingResource,
    showRestartModal,
    restartTarget,
    restartingResource,
    showDeleteModal,
    resourceToDelete,
    deletingResource,
    showDrainModal,
    drainTargetNode,
    drainingNode,
    operatingNode,
    showNewNsModal,
    creatingNs,
    newNsError,
    showImportModal,
    importingCluster,
    showLogsDrawer,
    logsPod,
    showTerminalDrawer,
    terminalPod,
    openDetailDrawer,
    openLogsDrawer,
    openTerminalDrawer,
    openScaleModal,
    openRestartModal,
    openDeleteModal,
    openDrainModal,
    openApplyYamlModal,
    openYamlEditModal,
    handleScaleConfirm,
    handleRestartConfirm,
    handleDeleteConfirm,
    handleDrainConfirm,
    handleCordonNode,
    handleUncordonNode,
    handleTriggerCronJob,
    handleToggleSuspend,
    handleImportCluster,
    handleCreateNs,
    handleYamlApplied,
    handleCreateApplied,
  }
}
