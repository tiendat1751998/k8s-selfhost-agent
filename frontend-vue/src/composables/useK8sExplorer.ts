import { ref, computed } from 'vue'
import { k8sApi, type K8sResource, type K8sNamespace, type ResourceKind } from '../api/k8s'
import { fleetApi, type Cluster } from '../api/compute'
import type { Column } from '../components/ui/DataTable.vue'
export * from './explorerHelpers'
export * from './explorerColumns'
export type { DrainOptions } from './useExplorerOperations'
import { kindCategories, type KindCategory } from './explorerHelpers'
import {
  podColumns, deploymentColumns, statefulSetColumns, daemonSetColumns,
  jobColumns, cronJobColumns, serviceColumns, ingressColumns,
  configMapColumns, secretColumns, pvcColumns, pvColumns,
  storageClassColumns, networkPolicyColumns, serviceAccountColumns,
  hpaColumns, nodeColumns, eventColumns, standardColumns
} from './explorerColumns'
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
  }
}

