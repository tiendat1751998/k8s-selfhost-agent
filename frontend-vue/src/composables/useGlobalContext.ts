import { ref } from 'vue'
import { api, type ApiResponse } from '../api/client'
import { fleetApi, type Cluster } from '../api/fleet'
import { k8sApi } from '../api/k8s'

export interface ClusterContextItem {
  id: string
  name: string
  status: string
  region?: string
}

function getStored(key: string, fallback: string): string {
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      return window.localStorage.getItem(key) || fallback
    }
  } catch {
    // Fallback if localStorage unavailable
  }
  return fallback
}

function setStored(key: string, value: string): void {
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.setItem(key, value)
    }
  } catch {
    // Ignore storage write errors
  }
}

// Module-level reactive singleton store
const storedCluster = getStored('k8s_active_cluster', 'local')
const activeClusterId = ref<string>(storedCluster || 'local')
const activeNamespace = ref<string>(getStored('k8s_active_namespace', 'all'))
const availableClusters = ref<ClusterContextItem[]>([])
const availableNamespaces = ref<string[]>(['all'])
const error = ref<string | null>(null)

async function loadNamespacesForCluster(clusterId: string): Promise<void> {
  if (!clusterId) {
    availableNamespaces.value = ['all']
    if (activeNamespace.value !== 'all') {
      setNamespace('all')
    }
    return
  }
  try {
    const list = await k8sApi.listNamespaces(clusterId)
    if (list && list.length > 0) {
      const names = list.map(n => n.name).filter(Boolean)
      availableNamespaces.value = Array.from(new Set(['all', ...names]))
    } else {
      availableNamespaces.value = ['all']
    }
  } catch (err: unknown) {
    availableNamespaces.value = ['all']
    error.value = err instanceof Error ? err.message : `Failed to load namespaces for ${clusterId}`
  }

  // Fix state desync on cluster switch
  if (activeNamespace.value !== 'all' && !availableNamespaces.value.includes(activeNamespace.value)) {
    setNamespace('all')
  }
}

// Cross-tab storage synchronization
function onStorage(event: StorageEvent): void {
  if (event.key === 'k8s_active_cluster' && event.newValue) {
    if (activeClusterId.value !== event.newValue) {
      activeClusterId.value = event.newValue
      loadNamespacesForCluster(event.newValue).catch(() => {})
    }
  } else if (event.key === 'k8s_active_namespace' && event.newValue) {
    if (activeNamespace.value !== event.newValue) {
      activeNamespace.value = event.newValue
    }
  }
}

if (typeof window !== 'undefined' && typeof window.addEventListener === 'function') {
  window.addEventListener('storage', onStorage)
}

async function loadClusters(): Promise<void> {
  error.value = null
  try {
    let list: Cluster[] = []
    try {
      if (typeof fleetApi.getClusters === 'function') {
        list = await fleetApi.getClusters()
      } else {
        list = await fleetApi.list()
      }
    } catch {
      try {
        const res = await api.get<ApiResponse<Cluster[]> | Cluster[]>('/clusters')
        if (Array.isArray(res)) {
          list = res
        } else if (res && Array.isArray((res as ApiResponse<Cluster[]>).data)) {
          list = (res as ApiResponse<Cluster[]>).data
        }
      } catch {
        list = []
      }
    }

    if (list && list.length > 0) {
      availableClusters.value = list.map(c => ({
        id: c.id,
        name: c.name || c.id,
        status: (c.health_status || c.status || 'healthy').toLowerCase(),
        region: c.region
      }))
      const activeCluster = availableClusters.value.find(c => ['active', 'healthy', 'ready'].includes(c.status)) || availableClusters.value[0]
      const exists = availableClusters.value.some(c => c.id === activeClusterId.value)
      if ((!exists || activeClusterId.value === 'local') && activeCluster) {
        activeClusterId.value = activeCluster.id
        setStored('k8s_active_cluster', activeClusterId.value)
      }
    } else {
      // Fallback to active cluster item so operator context is never empty
      const fallbackId = activeClusterId.value || 'local'
      availableClusters.value = [{
        id: fallbackId,
        name: fallbackId,
        status: 'healthy'
      }]
      if (!activeClusterId.value) {
        activeClusterId.value = 'local'
        setStored('k8s_active_cluster', 'local')
      }
    }
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : 'Failed to retrieve cluster list'
    const fallbackId = activeClusterId.value || 'local'
    availableClusters.value = [{
      id: fallbackId,
      name: fallbackId,
      status: 'healthy'
    }]
    if (!activeClusterId.value) {
      activeClusterId.value = 'local'
      setStored('k8s_active_cluster', 'local')
    }
  }
  await loadNamespacesForCluster(activeClusterId.value)
}

function setCluster(id: string): void {
  activeClusterId.value = id
  setStored('k8s_active_cluster', id)
  loadNamespacesForCluster(id).catch(() => {})
}

function setNamespace(ns: string): void {
  activeNamespace.value = ns
  setStored('k8s_active_namespace', ns)
}

export function useGlobalContext() {
  return {
    activeClusterId,
    activeNamespace,
    availableClusters,
    availableNamespaces,
    error,
    setCluster,
    setNamespace,
    loadClusters
  }
}
