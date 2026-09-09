import { ref, computed, onMounted } from 'vue'
import {
  deploymentsApi, dockerApi, type DeploymentApp, type DeploymentTemplate,
  type UpdateResourcesPayload, type ScaleDeploymentPayload
} from '../api/compute'
import {
  type FilterTab, type ToastMessage, type RolloutState,
  getRolloutState, buildSwarmAppItem, buildDockerContainerAppItem, filterDeploymentsList
} from './deploymentHelpers'

export type { FilterTab, ToastMessage, RolloutState }
export { getRolloutState }

export function useDeployments() {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const actionLoading = ref<string | null>(null)
  const toastMessage = ref<ToastMessage | null>(null)

  const deployments = ref<DeploymentApp[]>([])
  const templates = ref<DeploymentTemplate[]>([])

  const activeFilterTab = ref<FilterTab>('all')
  const searchQuery = ref('')
  const selectedNamespaceFilter = ref<string>('all')

  function showToast(text: string, type: 'success' | 'error' | 'info' = 'success') {
    toastMessage.value = { text, type }
    setTimeout(() => {
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

      deployments.value = allWorkloads

      if (tmplRes.status === 'fulfilled' && Array.isArray(tmplRes.value)) {
        templates.value = tmplRes.value
      }
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to retrieve deployments'
    } finally {
      loading.value = false
    }
  }

  // Computed Metrics
  const totalWorkloads = computed(() => deployments.value.length)
  const totalReplicas = computed(() => deployments.value.reduce((acc, d) => acc + (d.replicas || 0), 0))
  const readyReplicas = computed(() => deployments.value.reduce((acc, d) => acc + (d.readyReplicas || d.replicas || 0), 0))
  const healthyCount = computed(() => deployments.value.filter(d => d.status === 'healthy').length)

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
    filterDeploymentsList(deployments.value, activeFilterTab.value, selectedNamespaceFilter.value, searchQuery.value)
  )

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
        showToast(`Rolling restart dispatched for ${app.name}!`, 'success')
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

  return {
    loading, error, actionLoading, toastMessage, deployments, templates,
    activeFilterTab, searchQuery, selectedNamespaceFilter,
    totalWorkloads, totalReplicas, readyReplicas, healthyCount,
    canaryCount, blueGreenCount, k8sCount, swarmCount, namespaces,
    filteredDeployments, showToast, fetchDeployments, getRolloutState,
    handleApplyResources, handleScale, handleRestart,
    handleApplyCanaryWeight, handlePromoteCanary, handleAbortCanary,
    handleBlueGreenCutover, handleRollback, handleTogglePause, handleDelete, handleCreateApp,
  }
}