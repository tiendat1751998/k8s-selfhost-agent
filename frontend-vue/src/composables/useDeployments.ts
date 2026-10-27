import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  deploymentsApi, dockerApi, type DeploymentApp, type DeploymentTemplate,
  type UpdateResourcesPayload, type ScaleDeploymentPayload
} from '../api/compute'
import {
  type FilterTab, type StatusFilter, type ToastMessage, type RolloutState,
  getRolloutState, buildSwarmAppItem, buildDockerContainerAppItem, filterDeploymentsList
} from './deploymentHelpers'

export type { FilterTab, StatusFilter, ToastMessage, RolloutState }
export { getRolloutState }

export interface NamespaceGroup {
  namespace: string
  cluster: string
  workloads: DeploymentApp[]
  totalReplicas: number
  healthyWorkloads: number
  degradedWorkloads: number
  overallHealth: 'healthy' | 'degraded' | 'down'
}

export interface ReplicaBlock {
  index: number
  state: 'online' | 'warning' | 'offline' | 'scaled-0'
  isZero: boolean
}

export interface ReplicaCellsInfo {
  blocks: ReplicaBlock[]
  overflowCount: number
  readyCount: number
  desiredCount: number
  label: string
}

const PRIORITY_NAMESPACES = ['kube-system', 'platform', 'ingress', 'database', 'observability']

export function groupDeploymentsByNamespace(apps: DeploymentApp[]): NamespaceGroup[] {
  const map = new Map<string, DeploymentApp[]>()
  for (const app of apps) {
    const ns = app.namespace || 'default'
    const list = map.get(ns) || []
    list.push(app)
    map.set(ns, list)
  }

  const sortedNs = Array.from(map.keys()).sort((a, b) => {
    const idxA = PRIORITY_NAMESPACES.indexOf(a.toLowerCase())
    const idxB = PRIORITY_NAMESPACES.indexOf(b.toLowerCase())
    if (idxA !== -1 && idxB !== -1) return idxA - idxB
    if (idxA !== -1) return -1
    if (idxB !== -1) return 1
    return a.localeCompare(b)
  })

  return sortedNs.map(ns => {
    const workloads = map.get(ns)!
    const totalReplicas = workloads.reduce((sum, w) => sum + (w.replicas || 0), 0)
    const healthyWorkloads = workloads.filter(w => w.status === 'healthy').length
    const degradedWorkloads = workloads.filter(w => w.status !== 'healthy').length
    const cluster = workloads[0]?.target || 'k8snode'
    let overallHealth: 'healthy' | 'degraded' | 'down' = 'healthy'
    if (workloads.length > 0 && healthyWorkloads === 0) overallHealth = 'down'
    else if (degradedWorkloads > 0) overallHealth = 'degraded'
    return { namespace: ns, cluster, workloads, totalReplicas, healthyWorkloads, degradedWorkloads, overallHealth }
  })
}

export function computeReplicaCells(app: { replicas?: number; readyReplicas?: number; status?: string }): ReplicaCellsInfo {
  const replicas = app.replicas ?? 0
  const status = app.status ?? 'healthy'
  const ready = app.readyReplicas !== undefined ? app.readyReplicas : (status === 'healthy' ? replicas : 0)

  if (replicas <= 0) {
    return {
      blocks: [{ index: 0, state: 'scaled-0', isZero: true }],
      overflowCount: 0,
      readyCount: 0,
      desiredCount: 0,
      label: '0 / 0 Pods (Scaled)'
    }
  }

  const displayLimit = 12
  const visibleCount = Math.min(replicas, displayLimit)
  const blocks: ReplicaBlock[] = []
  for (let i = 0; i < visibleCount; i++) {
    const state: 'online' | 'warning' | 'offline' = status === 'down' ? 'offline' : i < ready ? 'online' : 'warning'
    blocks.push({ index: i, state, isZero: false })
  }

  return {
    blocks,
    overflowCount: Math.max(0, replicas - displayLimit),
    readyCount: ready,
    desiredCount: replicas,
    label: `${ready} / ${replicas} Pods Ready`
  }
}

export function useDeployments() {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const actionLoading = ref<string | null>(null)
  const toastMessage = ref<ToastMessage | null>(null)

  const deployments = ref<DeploymentApp[]>([])
  const templates = ref<DeploymentTemplate[]>([])

  const activeFilterTab = ref<FilterTab>('all')
  const statusFilter = ref<StatusFilter>('all')
  const searchQuery = ref('')
  const selectedNamespaceFilter = ref<string>('all')
  const viewMode = ref<'topology' | 'table'>('topology')

  let toastTimer: ReturnType<typeof setTimeout> | null = null
  function showToast(text: string, type: 'success' | 'error' | 'info' = 'success') {
    if (toastTimer) clearTimeout(toastTimer)
    toastMessage.value = { text, type }
    toastTimer = setTimeout(() => {
      if (toastMessage.value?.text === text) {
        toastMessage.value = null
      }
    }, 4500)
  }

  async function fetchDeployments() {
    loading.value = true
    error.value = null
    try {
      const [depsRes, svcRes, contRes, tmplRes] = await Promise.allSettled([
        deploymentsApi.list(),
        dockerApi.listServices(),
        dockerApi.listContainers(),
        deploymentsApi.listTemplates()
      ])

      const allWorkloads: DeploymentApp[] = []
      const seenNames = new Set<string>()

      // 1. Live Kubernetes deployments
      if (depsRes.status === 'fulfilled' && Array.isArray(depsRes.value)) {
        for (const d of depsRes.value) {
          if (!seenNames.has(d.name)) {
            seenNames.add(d.name)
            allWorkloads.push({ ...d, rawId: d.id || d.name })
          }
        }
      }

      // 2. Docker Swarm services
      const swarmServiceNames: string[] = []
      if (svcRes.status === 'fulfilled' && Array.isArray(svcRes.value)) {
        for (const s of svcRes.value) {
          swarmServiceNames.push(s.name)
          const appItem = buildSwarmAppItem(s)
          const existingIdx = allWorkloads.findIndex(w => w.name === s.name)
          if (existingIdx >= 0) {
            allWorkloads[existingIdx] = { ...allWorkloads[existingIdx], ...appItem }
          } else {
            seenNames.add(s.name)
            allWorkloads.push(appItem)
          }
        }
      }

      // 3. Standalone Docker Containers
      if (contRes.status === 'fulfilled' && Array.isArray(contRes.value)) {
        for (const c of contRes.value) {
          const cleanName = (c.name || '').replace(/^\//, '')
          const isSwarmTask = swarmServiceNames.some(sName =>
            cleanName.startsWith(sName + '.') || cleanName.startsWith(sName + '_')
          )
          if (isSwarmTask || seenNames.has(cleanName)) continue

          seenNames.add(cleanName)
          allWorkloads.push(buildDockerContainerAppItem(c, cleanName))
        }
      }

      if (allWorkloads.length === 0) {
        deployments.value = []
        if (depsRes.status === 'rejected' && svcRes.status === 'rejected' && contRes.status === 'rejected') {
          error.value = 'Failed to connect to workload cluster. Backend API offline.'
        }
      } else {
        deployments.value = allWorkloads
      }

      if (tmplRes.status === 'fulfilled' && Array.isArray(tmplRes.value)) {
        templates.value = tmplRes.value
      }
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to retrieve deployments'
      deployments.value = []
    } finally {
      loading.value = false
    }
  }

  // Computed Metrics
  const totalWorkloads = computed(() => deployments.value.length)
  const totalReplicas = computed(() => deployments.value.reduce((acc, d) => acc + (d.replicas || 0), 0))
  const readyReplicas = computed(() => deployments.value.reduce((acc, d) => acc + (d.readyReplicas || d.replicas || 0), 0))
  const healthyCount = computed(() => deployments.value.filter(d => d.status === 'healthy').length)
  const degradedCount = computed(() => deployments.value.filter(d => d.status !== 'healthy').length)

  const canaryCount = computed(() => deployments.value.filter(d => d.strategy === 'Canary' || (d.canaryWeight !== undefined && d.canaryWeight > 0)).length)
  const blueGreenCount = computed(() => deployments.value.filter(d => d.strategy === 'BlueGreen').length)
  const k8sCount = computed(() => deployments.value.filter(d => d.type === 'kubernetes' || d.type === 'k8s').length)
  const swarmCount = computed(() => deployments.value.filter(d => d.type === 'swarm' || d.type === 'docker' || d.type === 'container').length)

  const namespaces = computed(() => {
    const set = new Set<string>()
    for (const d of deployments.value) {
      if (d.namespace) set.add(d.namespace)
    }
    return Array.from(set).sort()
  })

  const filteredDeployments = computed(() =>
    filterDeploymentsList(deployments.value, activeFilterTab.value, selectedNamespaceFilter.value, searchQuery.value, statusFilter.value)
  )

  const totalPodReplicas = computed(() => totalReplicas.value)
  const attentionCount = computed(() => degradedCount.value)
  const namespaceCount = computed(() => namespaces.value.length)
  const namespaceGroups = computed(() => groupDeploymentsByNamespace(filteredDeployments.value))

  // Operations
  async function handleApplyResources(payload: UpdateResourcesPayload) {
    actionLoading.value = 'resources'
    try {
      await deploymentsApi.updateResources(payload)
      showToast(`✅ Workload ${payload.name} updated: Memory ${payload.memory_limit}, CPU ${payload.cpu_limit}, Replicas ${payload.replicas}`, 'success')
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Resource tuning operation failed', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handleScale(payload: ScaleDeploymentPayload) {
    if (payload.type === 'daemon' || payload.type === 'host-daemon') {
      showToast('Scale not supported: Managed via Linux systemd on host', 'info')
      return false
    }
    actionLoading.value = 'scale'
    try {
      await deploymentsApi.scale(payload)
      showToast(`Scale updated for ${payload.name} to ${payload.replicas} replicas`, 'success')
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Scale operation failed', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handleRestart(app: DeploymentApp) {
    actionLoading.value = app.name
    try {
      if (app.type === 'docker') {
        await dockerApi.toggleContainer(app.rawId || app.name, 'stop')
        await dockerApi.toggleContainer(app.rawId || app.name, 'start')
        showToast(`Container ${app.name} restarted!`, 'success')
      } else {
        await deploymentsApi.restart({
          type: app.type,
          cluster: app.target,
          namespace: app.namespace,
          name: app.rawId || app.name
        })
        const isDaemon = app.type === 'daemon' || app.type === 'host-daemon'
        showToast(isDaemon ? `Service restart dispatched for ${app.name}!` : `Rolling restart dispatched for ${app.name}!`, 'success')
      }
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Restart failed', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handleApplyCanaryWeight(app: DeploymentApp, weight: number) {
    actionLoading.value = 'canary'
    try {
      await deploymentsApi.setCanaryWeight({
        type: app.type,
        cluster: app.target,
        namespace: app.namespace,
        name: app.rawId || app.name,
        weight
      })
      app.canaryWeight = weight
      showToast(`Updated Canary traffic weight to ${weight}% for ${app.name}!`, 'success')
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to adjust canary weight', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handlePromoteCanary(app: DeploymentApp) {
    actionLoading.value = 'promote'
    try {
      await deploymentsApi.setCanaryWeight({
        type: app.type,
        cluster: app.target,
        namespace: app.namespace,
        name: app.rawId || app.name,
        weight: 100
      })
      app.canaryWeight = 100
      if (app.canaryVersion) app.image = app.canaryVersion
      showToast(`Promoted Canary version to 100% stable baseline for ${app.name}!`, 'success')
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Canary promotion failed', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handleAbortCanary(app: DeploymentApp) {
    actionLoading.value = 'abort'
    try {
      await deploymentsApi.setCanaryWeight({
        type: app.type,
        cluster: app.target,
        namespace: app.namespace,
        name: app.rawId || app.name,
        weight: 0
      })
      app.canaryWeight = 0
      showToast(`Aborted Canary track. 100% traffic restored to stable baseline for ${app.name}!`, 'info')
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Canary abort failed', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handleBlueGreenCutover(app: DeploymentApp, targetColor: 'blue' | 'green') {
    actionLoading.value = 'cutover'
    try {
      await deploymentsApi.cutoverBlueGreen({
        type: app.type,
        cluster: app.target,
        namespace: app.namespace,
        name: app.rawId || app.name,
        targetColor
      })
      app.blueGreenActive = targetColor
      showToast(`Traffic cutover applied! Live traffic now routing to ${targetColor.toUpperCase()} track.`, 'success')
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Blue-Green cutover failed', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handleRollback(app: DeploymentApp) {
    actionLoading.value = 'rollback'
    try {
      const targetRev = Math.max(1, (app.revision || 2) - 1)
      await deploymentsApi.rollback({
        type: app.type,
        cluster: app.target,
        namespace: app.namespace,
        name: app.rawId || app.name,
        revision: targetRev
      })
      showToast(`Rollback to revision #${targetRev} dispatched for ${app.name}!`, 'success')
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Rollback failed', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handleTogglePause(app: DeploymentApp) {
    const action = app.paused ? 'resume' : 'pause'
    actionLoading.value = 'pause'
    try {
      await deploymentsApi.pauseResume({
        type: app.type,
        cluster: app.target,
        namespace: app.namespace,
        name: app.rawId || app.name,
        action
      })
      app.paused = !app.paused
      showToast(`Rollout ${action === 'pause' ? 'paused' : 'resumed'} for ${app.name}!`, 'info')
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : `Failed to ${action} rollout`, 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handleDelete(app: DeploymentApp) {
    if (app.type === 'daemon' || app.type === 'host-daemon') {
      showToast('Deletion not supported: Managed via Linux systemd on host', 'info')
      return false
    }
    actionLoading.value = app.name
    try {
      if (app.type === 'docker') {
        await dockerApi.toggleContainer(app.rawId || app.name, 'stop')
      } else {
        await deploymentsApi.delete({
          type: app.type,
          cluster: app.target,
          namespace: app.namespace,
          name: app.rawId || app.name
        })
      }
      showToast(`Workload ${app.name} terminated.`, 'info')
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Deletion failed', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  async function handleCreateApp(payload: DeploymentApp) {
    actionLoading.value = 'create'
    try {
      await deploymentsApi.create(payload)
      showToast(`Workload ${payload.name} successfully deployed!`, 'success')
      await fetchDeployments()
      return true
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Deployment creation failed', 'error')
      return false
    } finally {
      actionLoading.value = null
    }
  }

  onMounted(() => {
    fetchDeployments()
  })

  onUnmounted(() => {
    if (toastTimer) clearTimeout(toastTimer)
  })

  return {
    loading, error, actionLoading, toastMessage, deployments, templates,
    activeFilterTab, statusFilter, searchQuery, selectedNamespaceFilter,
    viewMode, namespaceGroups,
    totalWorkloads, totalReplicas, totalPodReplicas, readyReplicas, healthyCount, degradedCount, attentionCount, namespaceCount,
    canaryCount, blueGreenCount, k8sCount, swarmCount, namespaces,
    filteredDeployments, showToast, fetchDeployments, getRolloutState,
    handleApplyResources, handleScale, handleRestart,
    handleApplyCanaryWeight, handlePromoteCanary, handleAbortCanary,
    handleBlueGreenCutover, handleRollback, handleTogglePause, handleDelete, handleCreateApp,
  }
}