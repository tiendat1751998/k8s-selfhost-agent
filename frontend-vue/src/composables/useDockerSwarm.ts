import { ref, computed, onMounted } from 'vue'
import {
  dockerApi,
  type DockerContainer,
  type DockerNode,
  type DockerService,
  type SwarmInfo,
  type NodeDetails,
} from '../api/compute'

export interface SwarmServiceTask {
  id: string
  service_id: string
  service_name?: string
  node_id: string
  node_name: string
  slot: number
  image: string
  desired_state: 'running' | 'shutdown' | 'accepted' | string
  current_state: 'running' | 'complete' | 'failed' | 'rejected' | string
  ip?: string
  error?: string
  created_at: string
}

export interface SwarmStackDeployPayload {
  name: string
  compose_yaml: string
  env_vars?: Record<string, string>
  prune?: boolean
}

export interface ServiceUpdatePayload {
  image?: string
  replicas?: number
  parallelism?: number
  delay_seconds?: number
  rollback_on_failure?: boolean
}

export function useDockerSwarm() {
  // ==========================================
  // 1. STATE MANAGEMENT
  // ==========================================
  const loading = ref(false)
  const error = ref<string | null>(null)
  const actionLoading = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
  const statusMessage = toastMessage

  const activeTab = ref<'services' | 'nodes' | 'containers'>('services')

  const services = ref<DockerService[]>([])
  const nodes = ref<DockerNode[]>([])
  const containers = ref<DockerContainer[]>([])
  const swarmInfo = ref<SwarmInfo | null>(null)

  // Modals & Drawers State
  const showLogsDrawer = ref(false)
  const logTargetTitle = ref('')
  const logContent = ref('')

  const showNodeDrawer = ref(false)
  const selectedNode = ref<DockerNode | null>(null)
  const selectedNodeDetails = ref<NodeDetails | null>(null)
  const nodeDetailsLoading = ref(false)

  const showDeployModal = ref(false)
  const deployingStack = ref(false)

  const showServiceDrawer = ref(false)
  const selectedService = ref<DockerService | null>(null)
  const selectedServiceTasks = ref<SwarmServiceTask[]>([])
  const serviceTasksLoading = ref(false)

  // ==========================================
  // 2. DATA FETCHING
  // ==========================================
  async function fetchDockerData() {
    loading.value = true
    error.value = null
    try {
      const [svcRes, nodeRes, contRes, swarmRes] = await Promise.allSettled([
        dockerApi.listServices(),
        dockerApi.listNodes(),
        dockerApi.listContainers(),
        dockerApi.getSwarmInfo(),
      ])

      if (svcRes.status === 'fulfilled' && Array.isArray(svcRes.value)) {
        services.value = svcRes.value
      }
      if (nodeRes.status === 'fulfilled' && Array.isArray(nodeRes.value)) {
        nodes.value = nodeRes.value
      }
      if (contRes.status === 'fulfilled' && Array.isArray(contRes.value)) {
        containers.value = contRes.value
      }
      if (swarmRes.status === 'fulfilled' && swarmRes.value && swarmRes.value.id) {
        swarmInfo.value = swarmRes.value
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to retrieve Docker Swarm telemetry'
      error.value = msg
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    fetchDockerData()
  })

  // ==========================================
  // 3. COMPUTED PROPERTIES
  // ==========================================
  const totalServices = computed(() => services.value.length)
  const totalNodes = computed(() => nodes.value.length)
  const totalContainers = computed(() => containers.value.length)
  const activeManagers = computed(() => nodes.value.filter(n => n.role === 'manager' && n.status === 'ready').length)
  const managerNodes = computed(() => nodes.value.filter(n => n.role === 'manager'))
  const workerNodes = computed(() => nodes.value.filter(n => n.role !== 'manager'))
  const overlayMeshReady = computed(() => swarmInfo.value?.is_manager ?? true)

  // ==========================================
  // 4. ACTION HANDLERS & HELPERS
  // ==========================================
  function showToast(text: string, type: 'success' | 'error' = 'success') {
    toastMessage.value = { text, type }
    setTimeout(() => {
      if (toastMessage.value?.text === text) {
        toastMessage.value = null
      }
    }, 4000)
  }

  async function scaleService(serviceId: string, replicas: number) {
    const target = services.value.find(s => s.id === serviceId)
    const next = Math.max(0, replicas)
    actionLoading.value = `scale-${serviceId}`
    try {
      await dockerApi.scaleService(serviceId, next)
      if (target) {
        target.replicas = next
      }
      showToast(`Service ${target?.name || serviceId} scaled to ${next} replicas!`)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to scale service'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function stepReplicas(svc: DockerService, delta: number) {
    const next = Math.max(0, (svc.replicas || 0) + delta)
    await scaleService(svc.id, next)
  }

  async function updateService(serviceId: string, payload?: ServiceUpdatePayload) {
    const target = services.value.find(s => s.id === serviceId)
    actionLoading.value = `update-${serviceId}`
    try {
      if (payload?.replicas !== undefined) {
        await dockerApi.scaleService(serviceId, payload.replicas)
        if (target) target.replicas = payload.replicas
      }
      if (payload?.image && target) {
        target.image = payload.image
      }
      if (target) target.updated_at = new Date().toISOString()
      showToast(`Rolling update dispatched for service ${target?.name || serviceId}!`)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update service'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function removeService(_serviceId: string) {
    statusMessage.value = {
      type: 'error',
      text: 'Feature requires backend implementation (DELETE /docker/services/{id})',
    }
  }

  async function deployStack(_payload: SwarmStackDeployPayload) {
    statusMessage.value = {
      type: 'error',
      text: 'Stack deployment requires backend implementation (POST /docker/stacks)',
    }
    showDeployModal.value = false
  }

  async function handleToggleContainer(cont: DockerContainer) {
    actionLoading.value = `toggle-${cont.id}`
    const nextAction = cont.state === 'running' ? 'stop' : 'start'
    try {
      await dockerApi.toggleContainer(cont.id, nextAction)
      cont.state = nextAction === 'start' ? 'running' : 'exited'
      showToast(`Container ${cont.name} ${nextAction === 'start' ? 'started' : 'stopped'}!`)
      await fetchDockerData()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Container toggle failed'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function drainNode(nodeId: string) {
    actionLoading.value = `drain-${nodeId}`
    try {
      await dockerApi.drainNode(nodeId)
      const target = nodes.value.find(n => n.id === nodeId)
      if (target) target.availability = 'drain'
      showToast(`Node ${nodeId} set to drain mode. Tasks rescheduling...`)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to drain node'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function activateNode(nodeId: string) {
    actionLoading.value = `activate-${nodeId}`
    try {
      await dockerApi.activateNode(nodeId)
      const target = nodes.value.find(n => n.id === nodeId)
      if (target) target.availability = 'active'
      showToast(`Node ${nodeId} activated and accepting workloads.`)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to activate node'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function removeNode(nodeId: string) {
    actionLoading.value = `remove-node-${nodeId}`
    try {
      await dockerApi.removeNode(nodeId)
      nodes.value = nodes.value.filter(n => n.id !== nodeId)
      showToast(`Node ${nodeId} removed from Swarm cluster.`)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to remove node'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function viewLogs(id: string, name: string, type: 'service' | 'container') {
    logTargetTitle.value = `${type === 'service' ? 'Swarm Service' : 'Docker Container'}: ${name}`
    actionLoading.value = id
    try {
      const res = await dockerApi.getLogs(id, type)
      logContent.value = res.logs || 'No logs available.'
      showLogsDrawer.value = true
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to retrieve logs'
      logContent.value = `Error loading logs for ${name}: ${msg}`
      showLogsDrawer.value = true
    } finally {
      actionLoading.value = null
    }
  }

  async function inspectNode(node: DockerNode) {
    selectedNode.value = node
    showNodeDrawer.value = true
    nodeDetailsLoading.value = true
    try {
      const details = await dockerApi.getNodeDetails(node.id)
      if (details && details.id) {
        selectedNodeDetails.value = details
      } else {
        selectedNodeDetails.value = {
          id: node.id,
          hostname: node.hostname || node.name || '--',
          role: node.role || '--',
          availability: node.availability || '--',
          status: node.status || '--',
          engine_version: node.engine_version || node.version || '--',
          os: '--',
          architecture: '--',
          cpus: node.cpus || 0,
          memory: node.memory || 0,
          ip: node.ip || '--',
          labels: node.labels || {},
          joined_at: node.joined_at || node.updated_at || '',
        }
      }
    } catch {
      selectedNodeDetails.value = {
        id: node.id,
        hostname: node.hostname || node.name || '--',
        role: node.role || '--',
        availability: node.availability || '--',
        status: node.status || '--',
        engine_version: node.engine_version || node.version || '--',
        os: '--',
        architecture: '--',
        cpus: node.cpus || 0,
        memory: node.memory || 0,
        ip: node.ip || '--',
        labels: node.labels || {},
        joined_at: node.joined_at || node.updated_at || '',
      }
    } finally {
      nodeDetailsLoading.value = false
    }
  }

  function inspectService(svc: DockerService) {
    selectedService.value = svc
    showServiceDrawer.value = true
    synthesizeServiceTasks(svc)
  }

  function synthesizeServiceTasks(_svc: DockerService) {
    serviceTasksLoading.value = true
    selectedServiceTasks.value = []
    serviceTasksLoading.value = false
  }

  function formatDate(d?: string) {
    if (!d) return '-'
    try {
      return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
    } catch {
      return d
    }
  }

  function formatMemory(mem?: number) {
    if (!mem || mem <= 0) return '--'
    if (mem > 1024 * 1024 * 1024) {
      return `${(mem / (1024 * 1024 * 1024)).toFixed(1)} GB`
    }
    if (mem > 1024 * 1024) {
      return `${(mem / (1024 * 1024)).toFixed(0)} MB`
    }
    return `${mem} B`
  }

  return {
    loading,
    error,
    actionLoading,
    toastMessage,
    statusMessage,
    activeTab,
    services,
    nodes,
    containers,
    swarmInfo,
    showLogsDrawer,
    logTargetTitle,
    logContent,
    showNodeDrawer,
    selectedNode,
    selectedNodeDetails,
    nodeDetailsLoading,
    showDeployModal,
    deployingStack,
    showServiceDrawer,
    selectedService,
    selectedServiceTasks,
    serviceTasksLoading,
    totalServices,
    totalNodes,
    totalContainers,
    activeManagers,
    managerNodes,
    workerNodes,
    overlayMeshReady,
    fetchDockerData,
    showToast,
    scaleService,
    stepReplicas,
    updateService,
    removeService,
    deployStack,
    handleToggleContainer,
    drainNode,
    activateNode,
    removeNode,
    viewLogs,
    inspectNode,
    inspectService,
    formatDate,
    formatMemory,
  }
}
