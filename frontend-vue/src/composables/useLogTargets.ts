import { ref, computed, watch, onMounted } from 'vue'
import { getActivePinia } from 'pinia'
import type { LogEntry } from '../stores/logStore'
import { useAuthStore } from '../stores/authStore'
import { useInfraHosts } from './useInfraHosts'
import { api } from '../api/client'
import { getLogServices } from '../api/logging'
import { dockerApi } from '../api/docker'
import { k8sApi } from '../api/k8s'
import { isNoiseLog, classifyLogTarget, isHostOrSystemUnit } from '../utils/logTargetCategorizer'
import { cleanLogMessage } from '../components/logs/virtualLogUtils'

export { isHostOrSystemUnit } from '../utils/logTargetCategorizer'

export interface LogTarget {
  type: 'service' | 'node' | 'all'
  id: string
  name: string
  icon?: string
  namespace?: string
  node?: string
  service?: string
  status?: string
  online?: boolean
}

export interface TargetNodeItem {
  id: string
  name: string
  icon: string
  role: string
  systemd?: boolean
  online?: boolean
}

export interface TargetServiceItem {
  id: string
  name: string
  icon: string
  type: string
  category: 'app' | 'systemd'
}

export interface TargetWorkloadItem {
  id: string
  name: string
  icon: string
  kind: string
  namespace?: string
}

export interface UseLogTargetsProps {
  modelValue: LogTarget
  logs: LogEntry[]
  cluster?: string
}

// Module-level persistent catalogs so filtered streams never wipe out discovered services!
const discoveredServicesByNode = ref<Record<string, Set<string>>>({})
const globalDiscoveredServices = ref<Set<string>>(new Set())

export function enrichDiscoveredServices(logs?: LogEntry[]) {
  if (!logs || !Array.isArray(logs)) return
  for (const log of logs) {
    const node = (log.node || log.attributes?.node || log.attributes?.node_name || '').toLowerCase()
    const svc = log.service || log.container || log.attributes?.app || log.attributes?.service || log.attributes?.unit || log.attributes?.container_name || log.pod || ''
    if (!svc || isNoiseLog(svc)) continue
    globalDiscoveredServices.value.add(svc)
    if (node) {
      if (!discoveredServicesByNode.value[node]) {
        discoveredServicesByNode.value[node] = new Set()
      }
      discoveredServicesByNode.value[node].add(svc)
    }
  }
}

export function useLogTargets(
  props: UseLogTargetsProps,
  emit: {
    (e: 'update:modelValue', target: LogTarget): void
    (e: 'select', target: LogTarget): void
  }
) {
  const { hosts } = useInfraHosts()
  const apiServices = ref<TargetServiceItem[]>([])
  const apiWorkloads = ref<TargetWorkloadItem[]>([])
  const targetSearch = ref('')

  // Immediately enrich from initial logs and react to updates
  enrichDiscoveredServices(props.logs)
  watch(() => props.logs, (newLogs) => {
    enrichDiscoveredServices(newLogs)
  }, { deep: false })

  async function fetchDynamicServices() {
    const foundNames = new Set<string>()
    const items: TargetServiceItem[] = []
    const workloads: TargetWorkloadItem[] = []

    function addService(name?: string | null, defaultType?: string) {
      if (!name || isNoiseLog(name) || isHostOrSystemUnit(name) || foundNames.has(name.toLowerCase())) return
      foundNames.add(name.toLowerCase())
      const c = classifyLogTarget(name, defaultType)
      items.push({ id: name, name, icon: c.icon, type: c.type, category: c.category })
    }

    function addWorkload(name?: string | null, kind = 'Deployment', namespace?: string) {
      if (!name || isNoiseLog(name) || isHostOrSystemUnit(name)) return
      const key = `wl:${name.toLowerCase()}`
      if (foundNames.has(key)) return
      foundNames.add(key)
      workloads.push({ id: name, name, icon: 'layers', kind, namespace })
    }

    // 1. Try GET /logs/services (supports by_node and services)
    try {
      const res = await getLogServices()
      if (res.by_node) {
        for (const [node, svcs] of Object.entries(res.by_node)) {
          const nLower = node.toLowerCase()
          if (!discoveredServicesByNode.value[nLower]) {
            discoveredServicesByNode.value[nLower] = new Set()
          }
          for (const s of svcs) {
            if (!isNoiseLog(s) && !isHostOrSystemUnit(s)) {
              discoveredServicesByNode.value[nLower].add(s)
              globalDiscoveredServices.value.add(s)
              addService(s)
            }
          }
        }
      }
      if (res.services && Array.isArray(res.services)) {
        for (const s of res.services) {
          if (!isNoiseLog(s) && !isHostOrSystemUnit(s)) {
            globalDiscoveredServices.value.add(s)
            addService(s)
          }
        }
      }
    } catch {
      try {
        const res = await api.get<{ targets?: string[]; services?: string[] } | string[]>('/logs/targets')
        const rawList = Array.isArray(res) ? res : (res?.targets || res?.services || [])
        for (const item of rawList) {
          const s = typeof item === 'string' ? item : (item as { name?: string })?.name || String(item)
          if (!isNoiseLog(s) && !isHostOrSystemUnit(s)) {
            globalDiscoveredServices.value.add(s)
            addService(s)
          }
        }
      } catch {}
    }

    // 2. Try GET /docker/services and /docker/containers
    try {
      const [dockerServices, dockerContainers] = await Promise.allSettled([
        dockerApi.listServices(),
        dockerApi.listContainers(),
      ])
      if (dockerServices.status === 'fulfilled' && Array.isArray(dockerServices.value)) {
        for (const s of dockerServices.value) addService(s.name || s.id, 'Swarm Service')
      }
      if (dockerContainers.status === 'fulfilled' && Array.isArray(dockerContainers.value)) {
        for (const c of dockerContainers.value) addService((c.name || c.id).replace(/^\//, ''), c.state || 'Container')
      }
    } catch {}

    // 3. Try GET /deployments for active k8s workloads
    try {
      const deployRes = await api.get<{ data?: any[] } | any[]>('/deployments')
      const deploys = Array.isArray(deployRes) ? deployRes : (deployRes?.data || [])
      for (const d of deploys) {
        if (d && d.name) addWorkload(d.name, d.kind || 'Deployment', d.namespace)
      }
    } catch {}

    // 4. Fallback cluster resources if cluster specified
    if (props.cluster) {
      try {
        const [svcRes, podRes, deployRes] = await Promise.allSettled([
          k8sApi.listResources(props.cluster, 'services'),
          k8sApi.listResources(props.cluster, 'pods'),
          k8sApi.listResources(props.cluster, 'deployments'),
        ])
        if (deployRes.status === 'fulfilled' && Array.isArray(deployRes.value)) {
          for (const d of deployRes.value) addWorkload(d.metadata?.name || (d as { name?: string }).name, 'Deployment', d.metadata?.namespace)
        }
        if (svcRes.status === 'fulfilled' && Array.isArray(svcRes.value)) {
          for (const s of svcRes.value) addService(s.metadata?.name || (s as { name?: string }).name, 'Service')
        }
        if (podRes.status === 'fulfilled' && Array.isArray(podRes.value)) {
          for (const p of podRes.value) addService(p.metadata?.name || (p as { name?: string }).name, 'Pod')
        }
      } catch {}
    }

    if (items.length > 0) apiServices.value = items
    if (workloads.length > 0) apiWorkloads.value = workloads
  }

  const dynamicWorkloads = computed<TargetWorkloadItem[]>(() => {
    const map = new Map<string, TargetWorkloadItem>()
    for (const w of apiWorkloads.value) map.set(w.id.toLowerCase(), w)
    return Array.from(map.values())
  })

  function getNodeServices(nodeId: string): TargetServiceItem[] {
    const nLower = (nodeId || '').toLowerCase()
    if (!nLower || nLower === 'all') {
      const allNames = Array.from(globalDiscoveredServices.value).filter(s => !isNoiseLog(s))
      // Also include apiServices
      for (const s of apiServices.value) {
        if (!isNoiseLog(s.name) && !allNames.includes(s.name)) {
          allNames.push(s.name)
        }
      }
      return allNames.map(svc => {
        const classified = classifyLogTarget(svc)
        return {
          id: svc,
          name: svc,
          icon: classified.icon,
          type: classified.type,
          category: classified.category,
        }
      }).sort((a, b) => {
        if (a.category !== b.category) return a.category === 'app' ? -1 : 1
        return a.name.localeCompare(b.name)
      })
    }

    const nodeSet = discoveredServicesByNode.value[nLower]
    if (!nodeSet || nodeSet.size === 0) {
      return []
    }
    const items: TargetServiceItem[] = []
    for (const svc of nodeSet) {
      if (!isNoiseLog(svc)) {
        const classified = classifyLogTarget(svc)
        items.push({
          id: svc,
          name: svc,
          icon: classified.icon,
          type: classified.type,
          category: classified.category,
        })
      }
    }
    items.sort((a, b) => {
      if (a.category !== b.category) {
        return a.category === 'app' ? -1 : 1
      }
      return a.name.localeCompare(b.name)
    })
    return items
  }

  function getDiscoveredServicesForNode(nodeId?: string): string[] {
    return getNodeServices(nodeId || '')
      .map(s => s.name)
      .filter(name => !isHostOrSystemUnit(name))
  }

  const dynamicServices = computed<TargetServiceItem[]>(() => getNodeServices('all'))
  const dynamicAppServices = computed(() => dynamicServices.value.filter(s => s.category === 'app'))
  const dynamicSystemdServices = computed(() => dynamicServices.value.filter(s => s.category === 'systemd'))

  const dynamicHostNodes = computed<TargetNodeItem[]>(() => {
    const map = new Map<string, TargetNodeItem>()
    if (hosts.value && hosts.value.length > 0) {
      for (const h of hosts.value) {
        const name = h.name || h.id || h.endpoint
        const isMaster = name.toLowerCase().includes('mater') || name.toLowerCase().includes('master')
        const isOnline = h.status === 'connected' || h.status === 'online' || h.status === 'healthy'
        map.set(name.toLowerCase(), {
          id: name,
          name,
          icon: 'server',
          role: isMaster ? 'Control Plane · SystemD' : 'Worker Node · SystemD',
          systemd: true,
          online: isOnline,
        })
      }
    }
    for (const log of props.logs) {
      const node = log.node || log.attributes?.node
      if (node && !map.has(node.toLowerCase())) {
        map.set(node.toLowerCase(), { id: node, name: node, icon: 'server', role: 'Host Node · SystemD', systemd: true, online: false })
      }
    }
    return Array.from(map.values())
  })

  const filteredK8sWorkloads = computed(() => {
    const q = targetSearch.value.trim().toLowerCase()
    return q ? dynamicWorkloads.value.filter(w => w.name.toLowerCase().includes(q) || w.kind.toLowerCase().includes(q)) : dynamicWorkloads.value
  })

  const filteredAppServices = computed(() => {
    const q = targetSearch.value.trim().toLowerCase()
    return q ? dynamicAppServices.value.filter(s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q)) : dynamicAppServices.value
  })

  const filteredSystemdServices = computed(() => {
    const q = targetSearch.value.trim().toLowerCase()
    return q ? dynamicSystemdServices.value.filter(s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q)) : dynamicSystemdServices.value
  })

  const filteredHostNodes = computed(() => {
    const q = targetSearch.value.trim().toLowerCase()
    return q ? dynamicHostNodes.value.filter(n => n.name.toLowerCase().includes(q) || n.role.toLowerCase().includes(q)) : dynamicHostNodes.value
  })

  function selectTarget(target: LogTarget) {
    emit('update:modelValue', target)
    emit('select', target)
  }

  const targetLogCounts = computed(() => {
    const serviceCounts = new Map<string, number>()
    const nodeCounts = new Map<string, number>()
    for (const l of props.logs) {
      if (!l.msg || !cleanLogMessage(l.msg)) continue
      const svc = l.service || l.container || l.attributes?.app || l.attributes?.service || l.pod
      if (svc && isNoiseLog(svc)) continue
      const sTargets = [l.service, l.container, l.attributes?.app, l.attributes?.service, l.pod, l.namespace].filter(Boolean) as string[]
      for (const key of new Set(sTargets.map(s => s.toLowerCase()))) {
        serviceCounts.set(key, (serviceCounts.get(key) || 0) + 1)
      }
      const nTargets = [l.node, l.attributes?.node, l.pod, l.namespace].filter(Boolean) as string[]
      for (const key of new Set(nTargets.map(n => n.toLowerCase()))) {
        nodeCounts.set(key, (nodeCounts.get(key) || 0) + 1)
      }
    }
    return { serviceCounts, nodeCounts }
  })

  function getNodeCount(nodeId: string): number {
    return targetLogCounts.value.nodeCounts.get(nodeId.toLowerCase()) || 0
  }

  function getServiceCount(serviceId: string): number {
    return targetLogCounts.value.serviceCounts.get(serviceId.toLowerCase()) || 0
  }

  const nodeServiceCounts = computed<Map<string, number>>(() => {
    const map = new Map<string, number>()
    for (const l of props.logs) {
      if (!l.msg || !cleanLogMessage(l.msg)) continue
      const node = (l.node || l.attributes?.node || l.attributes?.node_name || '').toLowerCase()
      if (!node) continue
      const sTargets = [l.service, l.container, l.attributes?.app, l.attributes?.service, l.pod].filter(Boolean) as string[]
      for (const s of new Set(sTargets.map(st => st.toLowerCase()))) {
        if (isNoiseLog(s)) continue
        const key = `${node}::${s}`
        map.set(key, (map.get(key) || 0) + 1)
      }
    }
    return map
  })

  function getNodeServiceCount(nodeId: string, serviceId: string): number {
    return nodeServiceCounts.value.get(`${nodeId.toLowerCase()}::${serviceId.toLowerCase()}`) || 0
  }

  const canViewJournalctl = computed(() => {
    try {
      if (!getActivePinia()) return false
      return useAuthStore().user?.role === 'platform_admin'
    } catch {
      return false
    }
  })
  const availableAppServices = computed<string[]>(() => {
    return getDiscoveredServicesForNode(props.modelValue?.node || props.modelValue?.id || 'all')
  })

  function autoSelectDefaultTarget() {
    if (props.modelValue && props.modelValue.type !== 'all' && props.modelValue.id && props.modelValue.id !== 'all') {
      return
    }
    if (dynamicWorkloads.value.length > 0) {
      const first = dynamicWorkloads.value[0]
      selectTarget({ type: 'service', id: first.id, name: first.name, icon: first.icon, namespace: first.namespace })
    } else if (dynamicAppServices.value.length > 0) {
      const first = dynamicAppServices.value[0]
      selectTarget({ type: 'service', id: first.id, name: first.name, icon: first.icon })
    } else if (dynamicSystemdServices.value.length > 0) {
      const first = dynamicSystemdServices.value[0]
      selectTarget({ type: 'service', id: first.id, name: first.name, icon: first.icon })
    } else if (dynamicHostNodes.value.length > 0) {
      const first = dynamicHostNodes.value[0]
      selectTarget({ type: 'node', id: first.id, name: first.name, icon: 'server' })
    }
  }

  watch(() => props.cluster, () => { fetchDynamicServices() })
  watch([dynamicWorkloads, dynamicServices, dynamicHostNodes], () => { autoSelectDefaultTarget() }, { immediate: true })

  onMounted(async () => {
    await fetchDynamicServices()
    autoSelectDefaultTarget()
  })

  return {
    targetSearch,
    filteredK8sWorkloads,
    filteredAppServices,
    filteredSystemdServices,
    filteredHostNodes,
    getNodeCount,
    getServiceCount,
    getNodeServices,
    getDiscoveredServicesForNode,
    getNodeServiceCount,
    selectTarget,
    dynamicHostNodes,
    discoveredServicesByNode,
    globalDiscoveredServices,
    canViewJournalctl,
    availableAppServices,
    isHostOrSystemUnit,
  }
}
