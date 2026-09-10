import { ref } from 'vue'
import { fleetApi } from '../api/fleet'
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
const activeClusterId = ref<string>(getStored('k8s_active_cluster', 'staging-k8s'))
const activeNamespace = ref<string>(getStored('k8s_active_namespace', 'all'))
const availableClusters = ref<ClusterContextItem[]>([])
const availableNamespaces = ref<string[]>(['all', 'default', 'kube-system', 'monitoring'])

async function loadNamespacesForCluster(clusterId: string): Promise<void> {
  if (!clusterId) return
  try {
    const list = await k8sApi.listNamespaces(clusterId)
    if (list && list.length > 0) {
      const names = list.map(n => n.name).filter(Boolean)
      availableNamespaces.value = Array.from(new Set(['all', ...names]))
      return
    }
  } catch {
    // Fallback if listNamespaces fails
  }
  if (availableNamespaces.value.length <= 1) {
    availableNamespaces.value = ['all', 'default', 'kube-system', 'monitoring', 'ingress-nginx']
  }
}

async function loadClusters(): Promise<void> {
  try {
    const list = await fleetApi.list()
    if (list && list.length > 0) {
      availableClusters.value = list.map(c => ({
        id: c.id,
        name: c.name || c.id,
        status: (c.health_status || c.status || 'healthy').toLowerCase(),
        region: c.region
      }))
      const exists = availableClusters.value.some(c => c.id === activeClusterId.value)
      if (!exists && availableClusters.value.length > 0) {
        activeClusterId.value = availableClusters.value[0].id
        setStored('k8s_active_cluster', activeClusterId.value)
      }
    } else if (availableClusters.value.length === 0) {
      availableClusters.value = [
        { id: 'staging-k8s', name: 'staging-k8s', status: 'healthy', region: 'us-east-1' },
        { id: 'prod-us-east', name: 'prod-us-east', status: 'healthy', region: 'us-east-2' },
        { id: 'prod-eu-west', name: 'prod-eu-west', status: 'healthy', region: 'eu-west-1' }
      ]
    }
  } catch {
    if (availableClusters.value.length === 0) {
      availableClusters.value = [
        { id: 'staging-k8s', name: 'staging-k8s', status: 'healthy', region: 'us-east-1' },
        { id: 'prod-us-east', name: 'prod-us-east', status: 'healthy', region: 'us-east-2' },
        { id: 'prod-eu-west', name: 'prod-eu-west', status: 'healthy', region: 'eu-west-1' }
      ]
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
    setCluster,
    setNamespace,
    loadClusters
  }
}
