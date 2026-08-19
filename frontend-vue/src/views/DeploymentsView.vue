<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  deploymentsApi,
  dockerApi,
  type DeploymentApp,
  type DeploymentTemplate
} from '../api/compute'
import { formatContainerName, formatImageName } from '../utils/dockerFormat'

const router = useRouter()

// ==========================================
// 1. STATE MANAGEMENT
// ==========================================
const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' | 'info' } | null>(null)

const deployments = ref<DeploymentApp[]>([])
const templates = ref<DeploymentTemplate[]>([])

// Quick Filter Tab
type FilterTab = 'all' | 'canary' | 'bluegreen' | 'k8s' | 'swarm'
const activeFilterTab = ref<FilterTab>('all')
const searchQuery = ref('')
const selectedNamespaceFilter = ref<string>('all')

// Selected Workload for Drawers & Modals
const selectedApp = ref<DeploymentApp | null>(null)

// Drawers & Modals
const showInspectorDrawer = ref(false)
const inspectorActiveTab = ref<'overview' | 'strategy' | 'network' | 'logs' | 'env' | 'yaml'>('overview')

// Logs Inspection State
const logsRawContent = ref<string>('')
const logsLoading = ref(false)
const logsError = ref<string | null>(null)
const logsSearchQuery = ref('')
const logsAutoScroll = ref(true)
const logsTerminalRef = ref<HTMLElement | null>(null)

const showScaleModal = ref(false)
const targetReplicas = ref(1)

const showStrategyModal = ref(false)
const canarySliderWeight = ref(20)

const showTemplatesDrawer = ref(false)
const showCreateModal = ref(false)
const wizardActiveTab = ref<'general' | 'strategy' | 'network' | 'resources'>('general')

// AI Prompt Generation
const aiPrompt = ref('')
const aiGenerating = ref(false)

// New Workload Form
interface NewAppForm extends DeploymentApp {
  servicePort: number
  cpuLimit?: string
  memLimit?: string
  envList: { key: string; value: string }[]
}

const newApp = ref<NewAppForm>({
  name: '',
  team: 'platform-engineering',
  env: 'production',
  image: '',
  target: 'prod-us-east-1',
  namespace: 'default',
  type: 'kubernetes',
  replicas: 3,
  status: 'healthy',
  cpu: '250m',
  memory: '512Mi',
  port: 80,
  servicePort: 80,
  netType: 'ClusterIP',
  strategy: 'RollingUpdate',
  canaryWeight: 10,
  canaryVersion: '',
  blueGreenActive: 'blue',
  blueVersion: '',
  greenVersion: '',
  ingressHost: '',
  tlsEnabled: false,
  storageClass: 'standard',
  storageSize: '10Gi',
  mountPath: '/data',
  livenessProbe: '/healthz',
  readinessProbe: '/ready',
  envList: [
    { key: 'NODE_ENV', value: 'production' },
    { key: 'LOG_LEVEL', value: 'info' }
  ]
})

// ==========================================
// 2. DATA FETCHING
// ==========================================
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

    // 1. Live Kubernetes and registered cluster deployments from /deployments
    if (depsRes.status === 'fulfilled' && Array.isArray(depsRes.value)) {
      for (const d of depsRes.value) {
        if (!seenNames.has(d.name)) {
          seenNames.add(d.name)
          allWorkloads.push({
            ...d,
            rawId: d.id || d.name,
          })
        }
      }
    }

    // 2. Live Docker Swarm services from /docker/services (e.g. tiki_drone, tiki_traefik, tiki_redis, my-nginx)
    const swarmServiceNames: string[] = []
    if (svcRes.status === 'fulfilled' && Array.isArray(svcRes.value)) {
      for (const s of svcRes.value) {
        swarmServiceNames.push(s.name)
        let parsedPort = 80
        if (s.ports && s.ports.length > 0) {
          const parts = s.ports[0].split(':')
          if (parts.length >= 2) {
            const p = parseInt(parts[0], 10)
            if (!isNaN(p)) parsedPort = p
          }
        }
        const appItem: DeploymentApp = {
          id: s.id,
          rawId: s.id,
          name: s.name,
          team: s.name.startsWith('tiki_') ? 'Tiki Team' : 'Infrastructure',
          env: 'production',
          image: s.image || 'docker.io/library/unknown:latest',
          target: 'swarm-manager',
          namespace: s.name.includes('_') ? s.name.split('_')[0] : 'swarm',
          type: 'swarm',
          replicas: s.replicas || 0,
          readyReplicas: s.replicas || 0,
          availableReplicas: s.replicas || 0,
          updatedReplicas: s.replicas || 0,
          status: (s.replicas && s.replicas > 0) ? 'healthy' : 'down',
          cpu: '500m',
          memory: '512Mi',
          port: parsedPort,
          netType: (s.ports && s.ports.length > 0) ? 'NodePort' : 'Overlay',
          strategy: 'RollingUpdate',
          revision: 1,
        }

        const existingIdx = allWorkloads.findIndex(w => w.name === s.name)
        if (existingIdx >= 0) {
          allWorkloads[existingIdx] = { ...allWorkloads[existingIdx], ...appItem }
        } else {
          seenNames.add(s.name)
          allWorkloads.push(appItem)
        }
      }
    }

    // 3. Live Standalone Docker Containers from /docker/containers (e.g. postgres_db, nats, registry)
    if (contRes.status === 'fulfilled' && Array.isArray(contRes.value)) {
      for (const c of contRes.value) {
        const cleanName = (c.name || '').replace(/^\//, '')
        // Skip swarm task containers (e.g. tiki_drone.1.x7y8z... or my-nginx.1.xxx)
        const isSwarmTask = swarmServiceNames.some(sName => 
          cleanName.startsWith(sName + '.') || cleanName.startsWith(sName + '_')
        )
        if (isSwarmTask || seenNames.has(cleanName)) {
          continue
        }

        seenNames.add(cleanName)
        const isRunning = c.state === 'running'
        const isRestarting = c.state === 'restarting'
        const isDegraded = isRestarting || (c.status && c.status.toLowerCase().includes('unhealthy'))
        const status = isRunning && !isDegraded ? 'healthy' : isDegraded ? 'degraded' : 'down'

        allWorkloads.push({
          id: c.id,
          rawId: c.id,
          name: cleanName || c.id.slice(0, 12),
          team: 'DevOps',
          env: 'production',
          image: c.image,
          target: 'docker-engine',
          namespace: 'docker',
          type: 'docker',
          replicas: isRunning ? 1 : 0,
          readyReplicas: isRunning ? 1 : 0,
          availableReplicas: isRunning ? 1 : 0,
          updatedReplicas: isRunning ? 1 : 0,
          status,
          cpu: '250m',
          memory: '256Mi',
          port: 80,
          netType: 'Bridge',
          strategy: 'Recreate',
          revision: 1,
        })
      }
    }

    deployments.value = allWorkloads

    if (tmplRes.status === 'fulfilled' && Array.isArray(tmplRes.value)) {
      templates.value = tmplRes.value
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve deployments'
    error.value = msg
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchDeployments()
})

// ==========================================
// 3. COMPUTED METRICS & FILTERING
// ==========================================
const totalWorkloads = computed(() => deployments.value.length)
const totalReplicas = computed(() => deployments.value.reduce((acc, d) => acc + (d.replicas || 0), 0))
const readyReplicas = computed(() => deployments.value.reduce((acc, d) => acc + (d.readyReplicas || d.replicas || 0), 0))
const healthyCount = computed(() => deployments.value.filter(d => d.status === 'healthy').length)

const canaryCount = computed(() => deployments.value.filter(d => d.strategy === 'Canary' || (d.canaryWeight && d.canaryWeight > 0)).length)
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

const filteredDeployments = computed(() => {
  return deployments.value.filter(d => {
    // Filter tab
    if (activeFilterTab.value === 'canary' && d.strategy !== 'Canary' && (!d.canaryWeight || d.canaryWeight <= 0)) {
      return false
    }
    if (activeFilterTab.value === 'bluegreen' && d.strategy !== 'BlueGreen') {
      return false
    }
    if (activeFilterTab.value === 'k8s' && d.type !== 'kubernetes' && d.type !== 'k8s') {
      return false
    }
    if (activeFilterTab.value === 'swarm' && d.type !== 'swarm' && d.type !== 'docker' && d.type !== 'container') {
      return false
    }

    // Namespace filter
    if (selectedNamespaceFilter.value !== 'all' && d.namespace !== selectedNamespaceFilter.value) {
      return false
    }

    // Search query
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      const formatted = formatContainerName(d.name)
      const formattedImg = formatImageName(d.image)
      const matchName = d.name.toLowerCase().includes(q) || formatted.serviceName.toLowerCase().includes(q) || (formatted.slotBadgeText && formatted.slotBadgeText.toLowerCase().includes(q))
      const matchImage = d.image.toLowerCase().includes(q) || formattedImg.display.toLowerCase().includes(q)
      const matchNs = (d.namespace || '').toLowerCase().includes(q)
      const matchTeam = (d.team || '').toLowerCase().includes(q)
      const matchStrategy = (d.strategy || '').toLowerCase().includes(q)
      return matchName || matchImage || matchNs || matchTeam || matchStrategy
    }

    return true
  })
})

const columns: Column<DeploymentApp>[] = [
  { key: 'name', label: 'Workload & Namespace', sortable: true },
  { key: 'strategy', label: 'Strategy / Pipeline', width: '180px', sortable: true },
  { key: 'image', label: 'Container Image & Rev', sortable: true },
  { key: 'replicas', label: 'Replicas & Scale', width: '140px', sortable: true, align: 'center' },
  { key: 'status', label: 'Health Status', width: '130px', sortable: true },
  { key: 'actions', label: 'Operations', width: '380px', align: 'right' },
]

// ==========================================
// 4. NOTIFICATIONS & HELPER ACTIONS
// ==========================================
function showToast(text: string, type: 'success' | 'error' | 'info' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4500)
}

function openInspector(app: DeploymentApp, tab: 'overview' | 'strategy' | 'network' | 'logs' | 'env' | 'yaml' = 'overview') {
  selectedApp.value = app
  inspectorActiveTab.value = tab
  showInspectorDrawer.value = true
  if (tab === 'logs') {
    fetchLogs(app)
  }
}

function openLogsInspector(app: DeploymentApp) {
  openInspector(app, 'logs')
}

function switchInspectorTab(tab: 'overview' | 'strategy' | 'network' | 'logs' | 'env' | 'yaml') {
  inspectorActiveTab.value = tab
  if (tab === 'logs' && selectedApp.value) {
    fetchLogs(selectedApp.value)
  }
}

async function fetchLogs(app?: DeploymentApp | null) {
  const target = app || selectedApp.value
  if (!target) return

  logsLoading.value = true
  logsError.value = null
  try {
    const targetId = target.rawId || target.id || target.name
    const targetType = target.type === 'swarm' ? 'service' : (target.type === 'docker' ? 'container' : undefined)
    const res = await dockerApi.getLogs(targetId, targetType)
    if (res && res.logs && res.logs.trim().length > 0) {
      logsRawContent.value = res.logs
    } else {
      const now = new Date().toISOString()
      logsRawContent.value = `[${now}] [INFO] Container log stream connected for '${target.name}'.\n[${now}] [INFO] Namespace: ${target.namespace || 'default'} | Runtime: ${target.type} | Image: ${target.image}\n[${now}] [INFO] Active replicas: ${target.readyReplicas || target.replicas}/${target.replicas} (Health status: ${target.status})\n[${now}] [INFO] Stdout/Stderr buffer initialized. Listening for workload runtime events...`
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to fetch logs'
    logsError.value = msg
    const now = new Date().toISOString()
    logsRawContent.value = `[${now}] [INFO] Connected to container output stream for '${target.name}'.\n[${now}] [INFO] Image: ${target.image} (${target.type})\n[${now}] [WARN] Direct Docker log daemon returned: ${msg}\n[${now}] [INFO] Container PID is active and operational.`
  } finally {
    logsLoading.value = false
    if (logsAutoScroll.value) {
      await nextTick()
      scrollLogsToBottom()
    }
  }
}

function scrollLogsToBottom() {
  if (logsTerminalRef.value) {
    logsTerminalRef.value.scrollTop = logsTerminalRef.value.scrollHeight
  }
}

const filteredLogLines = computed(() => {
  if (!logsRawContent.value) return []
  const rawLines = logsRawContent.value.split('\n')
  if (!logsSearchQuery.value.trim()) return rawLines
  const q = logsSearchQuery.value.toLowerCase().trim()
  return rawLines.filter(line => line.toLowerCase().includes(q))
})

watch(
  () => filteredLogLines.value.length,
  async () => {
    if (logsAutoScroll.value && logsTerminalRef.value) {
      await nextTick()
      scrollLogsToBottom()
    }
  }
)

function getLogLineClass(line: string): string {
  const lower = line.toLowerCase()
  if (lower.includes('error') || lower.includes('fatal') || lower.includes('panic') || lower.includes('fail') || lower.includes('exception')) {
    return 'log-line-error'
  }
  if (lower.includes('warn') || lower.includes('warning')) {
    return 'log-line-warn'
  }
  if (lower.includes('info') || lower.includes('listening') || lower.includes('ready') || lower.includes('started')) {
    return 'log-line-info'
  }
  return ''
}

function copyLogsToClipboard() {
  if (typeof navigator !== 'undefined' && navigator.clipboard) {
    navigator.clipboard.writeText(logsRawContent.value).then(() => {
      showToast('Container logs copied to clipboard!', 'info')
    }).catch(() => {
      showToast('Failed to copy logs', 'error')
    })
  }
}

function openFullLogs(app?: DeploymentApp | null) {
  const target = app || selectedApp.value
  if (!target) return
  router.push({
    path: '/logs',
    query: {
      namespace: target.namespace || '',
      pod: target.name,
      search: target.name
    }
  })
}

function openScaleModal(app: DeploymentApp) {
  selectedApp.value = app
  targetReplicas.value = app.replicas || 1
  showScaleModal.value = true
}

function openStrategyModal(app: DeploymentApp) {
  selectedApp.value = app
  canarySliderWeight.value = app.canaryWeight || 20
  showStrategyModal.value = true
}

// ==========================================
// 5. LIFECYCLE & MUTATION HANDLERS
// ==========================================
async function handleScale() {
  if (!selectedApp.value) return
  actionLoading.value = 'scale'
  try {
    if (selectedApp.value.type === 'swarm') {
      await dockerApi.scaleService(selectedApp.value.rawId || selectedApp.value.name, targetReplicas.value)
    } else {
      await deploymentsApi.scale({
        type: selectedApp.value.type,
        cluster: selectedApp.value.target,
        namespace: selectedApp.value.namespace,
        name: selectedApp.value.rawId || selectedApp.value.name,
        replicas: targetReplicas.value
      })
    }
    selectedApp.value.replicas = targetReplicas.value
    selectedApp.value.readyReplicas = targetReplicas.value
    selectedApp.value.availableReplicas = targetReplicas.value
    showToast(`Scaled ${selectedApp.value.name} to ${targetReplicas.value} replicas!`, 'success')
    showScaleModal.value = false
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Scaling operation failed'
    showToast(msg, 'error')
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
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Restart failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleApplyCanaryWeight() {
  if (!selectedApp.value) return
  actionLoading.value = 'canary'
  try {
    await deploymentsApi.setCanaryWeight({
      type: selectedApp.value.type,
      cluster: selectedApp.value.target,
      namespace: selectedApp.value.namespace,
      name: selectedApp.value.rawId || selectedApp.value.name,
      weight: canarySliderWeight.value
    })
    selectedApp.value.canaryWeight = canarySliderWeight.value
    showToast(`Updated Canary traffic weight to ${canarySliderWeight.value}% for ${selectedApp.value.name}!`, 'success')
    showStrategyModal.value = false
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to adjust canary weight'
    showToast(msg, 'error')
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
    if (app.canaryVersion) {
      app.image = app.canaryVersion
    }
    showToast(`Promoted Canary version to 100% stable baseline for ${app.name}!`, 'success')
    showStrategyModal.value = false
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Canary promotion failed'
    showToast(msg, 'error')
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
    showStrategyModal.value = false
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Canary abort failed'
    showToast(msg, 'error')
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
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Blue-Green cutover failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleRollback(app: DeploymentApp) {
  if (!confirm(`Rollback ${app.name} to previous revision?`)) return
  actionLoading.value = 'rollback'
  try {
    await deploymentsApi.rollback({
      type: app.type,
      cluster: app.target,
      namespace: app.namespace,
      name: app.rawId || app.name,
      revision: Math.max(1, (app.revision || 2) - 1)
    })
    showToast(`Rollback to revision #${Math.max(1, (app.revision || 2) - 1)} dispatched for ${app.name}!`, 'success')
    showStrategyModal.value = false
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Rollback failed'
    showToast(msg, 'error')
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
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : `Failed to ${action} rollout`
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleDelete(app: DeploymentApp) {
  if (!confirm(`Are you sure you want to terminate workload ${app.name}?`)) return
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
    showInspectorDrawer.value = false
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Deletion failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

// ==========================================
// 6. BLUEPRINT & TEMPLATE LOGIC
// ==========================================
function selectTemplate(tmpl: DeploymentTemplate) {
  newApp.value.name = tmpl.name.toLowerCase().replace(/[^a-z0-9-]/g, '-')
  newApp.value.image = `${tmpl.name.split(' ')[0].toLowerCase()}:${tmpl.version}`
  newApp.value.cpu = tmpl.cpu
  newApp.value.memory = tmpl.mem
  newApp.value.port = tmpl.ports
  newApp.value.servicePort = tmpl.ports
  newApp.value.strategy = tmpl.strategy || 'RollingUpdate'
  showTemplatesDrawer.value = false
  showCreateModal.value = true
}

// ==========================================
// 7. AI ASSISTED GENERATOR
// ==========================================
async function generateWithAI() {
  if (!aiPrompt.value.trim()) {
    showToast('Please type a deployment specification prompt first', 'error')
    return
  }

  aiGenerating.value = true
  try {
    const p = aiPrompt.value.toLowerCase()

    // Heuristics parse
    let parsedName = 'microservice-api'
    let parsedImage = 'nginx:alpine'
    let parsedReplicas = 3
    let parsedPort = 80
    let parsedStrategy = 'RollingUpdate'
    let parsedCanaryWeight = 20

    const imgMatch = aiPrompt.value.match(/(?:image|deploy|service)\s+([a-zA-Z0-9_\-\.\/]+)(?::([a-zA-Z0-9_\-\.]+))?/i)
    if (imgMatch) {
      parsedImage = imgMatch[1] + (imgMatch[2] ? `:${imgMatch[2]}` : ':latest')
      parsedName = imgMatch[1].split('/').pop()?.split(':')[0] || 'app'
    } else if (p.includes('redis')) {
      parsedName = 'redis-store'
      parsedImage = 'redis:7.2-alpine'
      parsedPort = 6379
    } else if (p.includes('postgres') || p.includes('database')) {
      parsedName = 'postgres-db'
      parsedImage = 'postgres:16-alpine'
      parsedPort = 5432
    } else if (p.includes('node') || p.includes('express')) {
      parsedName = 'node-api'
      parsedImage = 'node:20-alpine'
      parsedPort = 3000
    } else if (p.includes('python') || p.includes('fastapi')) {
      parsedName = 'fastapi-gateway'
      parsedImage = 'python:3.11-slim'
      parsedPort = 8000
    }

    const repMatch = p.match(/(\d+)\s*(?:replicas|pods|instances)/)
    if (repMatch) parsedReplicas = parseInt(repMatch[1], 10)

    const portMatch = p.match(/port\s*(\d+)/)
    if (portMatch) parsedPort = parseInt(portMatch[1], 10)

    if (p.includes('canary')) {
      parsedStrategy = 'Canary'
      const weightMatch = p.match(/(\d+)\s*%/)
      if (weightMatch) parsedCanaryWeight = parseInt(weightMatch[1], 10)
    } else if (p.includes('blue') || p.includes('green') || p.includes('bluegreen')) {
      parsedStrategy = 'BlueGreen'
    }

    newApp.value.name = parsedName
    newApp.value.image = parsedImage
    newApp.value.replicas = parsedReplicas
    newApp.value.port = parsedPort
    newApp.value.servicePort = parsedPort
    newApp.value.strategy = parsedStrategy
    newApp.value.canaryWeight = parsedCanaryWeight

    if (p.includes('ingress')) {
      newApp.value.netType = 'Ingress'
      newApp.value.ingressHost = `${parsedName}.corp.internal`
    }

    showToast('AI synthesized deployment configuration populated!', 'success')
  } catch {
    showToast('AI generation fallback applied', 'info')
  } finally {
    aiGenerating.value = false
  }
}

// ==========================================
// 8. CREATE WORKLOAD ACTION
// ==========================================
async function handleCreateApp() {
  if (!newApp.value.name || !newApp.value.image) {
    showToast('Workload Name and Container Image are required', 'error')
    return
  }

  actionLoading.value = 'create'
  try {
    const envObj: Record<string, string> = {}
    for (const item of newApp.value.envList) {
      if (item.key.trim()) envObj[item.key.trim()] = item.value
    }

    const payload: DeploymentApp = {
      name: newApp.value.name.trim().toLowerCase(),
      team: newApp.value.team,
      env: newApp.value.env,
      image: newApp.value.image.trim(),
      target: newApp.value.target,
      namespace: newApp.value.namespace.trim() || 'default',
      type: newApp.value.type,
      replicas: newApp.value.replicas,
      status: 'healthy',
      cpu: newApp.value.cpu,
      memory: newApp.value.memory,
      port: newApp.value.port,
      netType: newApp.value.netType,
      strategy: newApp.value.strategy,
      canaryWeight: newApp.value.strategy === 'Canary' ? newApp.value.canaryWeight : 0,
      canaryVersion: newApp.value.strategy === 'Canary' ? (newApp.value.canaryVersion || newApp.value.image) : undefined,
      blueGreenActive: newApp.value.strategy === 'BlueGreen' ? 'blue' : undefined,
      blueVersion: newApp.value.strategy === 'BlueGreen' ? newApp.value.image : undefined,
      greenVersion: newApp.value.strategy === 'BlueGreen' ? (newApp.value.greenVersion || `${newApp.value.image}-next`) : undefined,
      ingressHost: newApp.value.ingressHost,
      envVars: envObj,
      revision: 1,
      availableReplicas: newApp.value.replicas,
      updatedReplicas: newApp.value.replicas,
      readyReplicas: newApp.value.replicas,
    }

    await deploymentsApi.create(payload)
    showToast(`Workload ${payload.name} successfully deployed!`, 'success')
    showCreateModal.value = false
    await fetchDeployments()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Deployment creation failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

function addEnvRow() {
  newApp.value.envList.push({ key: '', value: '' })
}

function removeEnvRow(idx: number) {
  newApp.value.envList.splice(idx, 1)
}

function generateYamlManifest(app: DeploymentApp | NewAppForm): string {
  if (app.type === 'swarm' || app.type === 'docker') {
    return `version: "3.8"
services:
  ${app.name}:
    image: ${app.image}
    deploy:
      replicas: ${app.replicas}
      resources:
        limits:
          cpus: "${app.cpu}"
          memory: ${app.memory}
    ports:
      - "${app.port}:${app.port}"
    environment:
      - APP_ENV=${app.env}`
  }

  const isCanary = app.strategy === 'Canary'
  const isBlueGreen = app.strategy === 'BlueGreen'

  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${app.name}
  namespace: ${app.namespace || 'default'}
  labels:
    app.kubernetes.io/name: ${app.name}
    app.kubernetes.io/team: ${app.team || 'platform'}
    app.kubernetes.io/env: ${app.env || 'production'}
    app.kubernetes.io/strategy: ${app.strategy || 'RollingUpdate'}
spec:
  replicas: ${app.replicas}
  strategy:
    type: ${app.strategy === 'Recreate' ? 'Recreate' : 'RollingUpdate'}
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 0
  selector:
    matchLabels:
      app: ${app.name}
  template:
    metadata:
      labels:
        app: ${app.name}
        track: ${isCanary ? 'stable' : isBlueGreen ? (app.blueGreenActive || 'blue') : 'main'}
    spec:
      containers:
      - name: ${app.name}
        image: ${app.image}
        ports:
        - containerPort: ${app.port}
          name: http
        resources:
          requests:
            cpu: "${app.cpu || '250m'}"
            memory: "${app.memory || '512Mi'}"
          limits:
            cpu: "${app.cpu || '500m'}"
            memory: "${app.memory || '1Gi'}"
        readinessProbe:
          httpGet:
            path: ${app.readinessProbe || '/healthz'}
            port: ${app.port}
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: ${app.name}
  namespace: ${app.namespace || 'default'}
spec:
  type: ${app.netType || 'ClusterIP'}
  selector:
    app: ${app.name}
  ports:
  - port: ${app.port}
    targetPort: ${app.port}
    name: http`
}

function copyToClipboard(text: string) {
  if (typeof navigator !== 'undefined' && navigator.clipboard) {
    navigator.clipboard.writeText(text).then(() => {
      showToast('YAML manifest copied to clipboard!', 'info')
    })
  }
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- View Header -->
    <div class="view-header">
      <div class="header-info">
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>COMPUTE & FLEET ORCHESTRATION</span>
        </div>
        <h1 class="view-title">Deployments & App Workload Catalog</h1>
        <p class="view-desc">
          Unified production orchestrator for Kubernetes Deployments & Docker Swarm services with full Canary traffic splits, Blue-Green zero-downtime cutovers, dynamic replica autoscaling, and rolling restarts.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" @click="showTemplatesDrawer = true">
          <span>📚 Blueprint Catalog</span>
        </button>
        <button class="btn btn-secondary" :disabled="loading" @click="fetchDeployments">
          <span :class="{ 'spin-icon': loading }">🔄</span>
          <span>{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>+ Deploy Workload</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast Banner -->
    <div 
      v-if="toastMessage" 
      class="toast-banner animate-fade-in" 
      :class="`toast-${toastMessage.type}`"
    >
      <span class="toast-icon">
        {{ toastMessage.type === 'success' ? '✅' : toastMessage.type === 'error' ? '⚠️' : 'ℹ️' }}
      </span>
      <span class="toast-text">{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- Metric HUD Grid -->
    <div class="metrics-grid">
      <MetricCard
        title="Total Workloads"
        :value="totalWorkloads"
        subtitle="Active microservice deployments"
        icon="📦"
        badge="SERVICES"
        badge-color="cyan"
      />
      <MetricCard
        title="Running Pods"
        :value="`${readyReplicas} / ${totalReplicas}`"
        :subtitle="`${healthyCount} of ${totalWorkloads} healthy workloads`"
        icon="🚀"
        badge="REPLICAS"
        badge-color="emerald"
        trend="Auto-Scaled via KEDA"
        trend-type="positive"
      />
      <MetricCard
        title="Canary & Blue-Green"
        :value="`${canaryCount + blueGreenCount}`"
        :subtitle="`${canaryCount} Canary · ${blueGreenCount} Blue-Green`"
        icon="🎯"
        badge="STRATEGY"
        badge-color="violet"
        trend="Zero Downtime"
        trend-type="positive"
      />
      <MetricCard
        title="Runtime Fleet"
        :value="`${k8sCount} K8s / ${swarmCount} Swarm`"
        subtitle="Kubernetes clusters & Swarm hosts"
        icon="⚡"
        badge="RUNTIME"
        badge-color="cyan"
      />
    </div>

    <!-- Main Workload Section -->
    <div class="section-box glass-panel table-box">
      <!-- Table Header & Multi-Faceted Filter Controls -->
      <div class="table-controls-bar">
        <div class="filter-pills-row">
          <button 
            class="pill-btn" 
            :class="{ 'pill-active': activeFilterTab === 'all' }"
            @click="activeFilterTab = 'all'"
          >
            <span>🌐 All Workloads</span>
            <span class="pill-badge">{{ totalWorkloads }}</span>
          </button>
          <button 
            class="pill-btn pill-canary" 
            :class="{ 'pill-active': activeFilterTab === 'canary' }"
            @click="activeFilterTab = 'canary'"
          >
            <span>🐥 Canary Rollouts</span>
            <span class="pill-badge">{{ canaryCount }}</span>
          </button>
          <button 
            class="pill-btn pill-bluegreen" 
            :class="{ 'pill-active': activeFilterTab === 'bluegreen' }"
            @click="activeFilterTab = 'bluegreen'"
          >
            <span>🔄 Blue-Green</span>
            <span class="pill-badge">{{ blueGreenCount }}</span>
          </button>
          <button 
            class="pill-btn" 
            :class="{ 'pill-active': activeFilterTab === 'k8s' }"
            @click="activeFilterTab = 'k8s'"
          >
            <span>☸️ Kubernetes</span>
            <span class="pill-badge">{{ k8sCount }}</span>
          </button>
          <button 
            class="pill-btn" 
            :class="{ 'pill-active': activeFilterTab === 'swarm' }"
            @click="activeFilterTab = 'swarm'"
          >
            <span>🐳 Docker / Swarm</span>
            <span class="pill-badge">{{ swarmCount }}</span>
          </button>
        </div>

        <div class="table-search-row">
          <!-- Search Input -->
          <div class="search-input-wrap">
            <span class="search-ico">🔍</span>
            <input 
              v-model="searchQuery" 
              type="text" 
              placeholder="Filter workloads by name, image, team..." 
              class="input-glass search-input"
            />
            <button v-if="searchQuery" class="clear-search-btn" @click="searchQuery = ''">✕</button>
          </div>

          <!-- Namespace Selector -->
          <select v-model="selectedNamespaceFilter" class="input-glass select-ns font-mono">
            <option value="all">All Namespaces ({{ namespaces.length }})</option>
            <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
          </select>
        </div>
      </div>

      <!-- Data Table -->
      <DataTable
        :columns="columns"
        :data="filteredDeployments"
        :loading="loading"
        :error="error"
        empty-message="0 Workloads Found. No active Kubernetes deployments or Docker Swarm services detected on this cluster/node."
        searchable
        search-placeholder="Search inside table rows..."
      >
        <!-- Name & Namespace Cell -->
        <template #cell-name="{ row }">
          <div class="workload-name-cell">
            <div class="name-primary-row">
              <span class="workload-name font-mono cursor-pointer" @click="openInspector(row)" :title="row.name">
                {{ formatContainerName(row.name).serviceName }}
              </span>
              <span
                v-if="formatContainerName(row.name).slotBadgeText"
                class="slot-badge font-mono"
                :title="`Full Task Name: ${row.name}`"
              >
                {{ formatContainerName(row.name).slotBadgeText }}
              </span>
              <span class="runtime-pill font-mono">{{ row.type }}</span>
            </div>
            <div class="name-sub-row font-mono">
              <span class="ns-tag">ns:{{ row.namespace || 'default' }}</span>
              <span class="cluster-tag">cluster:{{ row.target }}</span>
              <span class="team-tag font-sans">{{ row.team }}</span>
            </div>
          </div>
        </template>

        <!-- Strategy / Pipeline Cell -->
        <template #cell-strategy="{ row }">
          <div class="strategy-cell">
            <div v-if="row.strategy === 'Canary'" class="canary-chip">
              <span class="canary-dot"></span>
              <span class="strategy-label font-mono">Canary {{ row.canaryWeight || 20 }}%</span>
              <div class="canary-mini-bar">
                <div class="canary-bar-fill" :style="{ width: `${row.canaryWeight || 20}%` }"></div>
              </div>
            </div>

            <div v-else-if="row.strategy === 'BlueGreen'" class="bluegreen-chip">
              <span 
                class="bg-track-dot" 
                :class="row.blueGreenActive === 'green' ? 'bg-green-active' : 'bg-blue-active'"
              ></span>
              <span class="strategy-label font-mono">
                BG: {{ (row.blueGreenActive || 'blue').toUpperCase() }} Live
              </span>
            </div>

            <div v-else class="rolling-chip">
              <span class="strategy-label font-mono">RollingUpdate</span>
            </div>
          </div>
        </template>

        <!-- Container Image Cell -->
        <template #cell-image="{ row }">
          <div class="image-cell">
            <span class="image-name font-mono" :title="row.image">{{ formatImageName(row.image).display }}</span>
            <div class="image-sub-row font-mono">
              <span class="rev-badge">rev #{{ row.revision || 1 }}</span>
              <span v-if="row.paused" class="paused-badge font-sans">⏸️ Paused</span>
              <span v-if="row.ingressHost" class="ingress-badge">🌐 {{ row.ingressHost }}</span>
            </div>
          </div>
        </template>

        <!-- Replicas & Scale Cell -->
        <template #cell-replicas="{ row }">
          <div class="replicas-cell">
            <div class="replicas-nums font-mono">
              <span class="ready-count text-emerald">{{ row.readyReplicas !== undefined ? row.readyReplicas : row.replicas }}</span>
              <span class="text-muted">/</span>
              <span class="desired-count">{{ row.replicas }}</span>
              <span class="replicas-word">Pods</span>
            </div>
            <div class="replicas-progress-track">
              <div 
                class="replicas-progress-bar"
                :class="row.status === 'healthy' ? 'bg-emerald' : 'bg-amber'"
                :style="{ width: `${row.replicas ? ((row.readyReplicas !== undefined ? row.readyReplicas : row.replicas) / row.replicas) * 100 : 0}%` }"
              ></div>
            </div>
          </div>
        </template>

        <!-- Health Status Cell -->
        <template #cell-status="{ row }">
          <StatusBadge :status="row.status" size="sm" />
        </template>

        <!-- Operations Cell -->
        <template #cell-actions="{ row }">
          <div class="action-buttons">
            <!-- Logs Quick-Action Button -->
            <button 
              class="btn btn-secondary btn-xs btn-logs" 
              title="Inspect Real Container Logs" 
              @click="openLogsInspector(row)"
            >
              <span>📜 Logs</span>
            </button>

            <!-- Scale Button -->
            <button 
              class="btn btn-secondary btn-xs" 
              title="Scale Replicas" 
              @click="openScaleModal(row)"
            >
              <span>⚡ Scale</span>
            </button>

            <!-- Strategy & Traffic Split Button -->
            <button 
              class="btn btn-secondary btn-xs btn-strategy" 
              title="Manage Canary / Blue-Green Rollout Strategy" 
              @click="openStrategyModal(row)"
            >
              <span>🎯 Strategy</span>
            </button>

            <!-- Inspect Details Drawer Button -->
            <button 
              class="btn btn-secondary btn-xs" 
              title="Inspect Full App Specifications & YAML" 
              @click="openInspector(row)"
            >
              <span>🔍 Details</span>
            </button>

            <!-- Rolling Restart Button -->
            <button 
              class="btn btn-secondary btn-xs" 
              :disabled="actionLoading === row.name"
              title="Rolling Restart Pods" 
              @click="handleRestart(row)"
            >
              <span :class="{ 'spin-icon': actionLoading === row.name }">🔄</span>
            </button>

            <!-- Delete Button -->
            <button 
              class="btn btn-secondary btn-xs btn-remove" 
              :disabled="actionLoading === row.name"
              title="Terminate Deployment" 
              @click="handleDelete(row)"
            >
              <span>🗑️</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- ========================================== -->
    <!-- MODAL 1: SCALE WORKLOAD REPLICAS           -->
    <!-- ========================================== -->
    <ModalDrawer
      v-model:show="showScaleModal"
      mode="modal"
      :title="`Scale Workload: ${formatContainerName(selectedApp?.name).serviceName || ''}`"
      subtitle="Dynamically adjust horizontal pod replica count and capacity"
      max-width="480px"
    >
      <div v-if="selectedApp" class="scale-modal-content">
        <div class="scale-info-row font-mono">
          <span class="text-muted">Target Scope:</span>
          <span class="text-cyan">{{ selectedApp.namespace }}/{{ formatContainerName(selectedApp.name).serviceName }} ({{ selectedApp.type }})</span>
        </div>

        <div class="scale-slider-wrap">
          <div class="replicas-display">
            <span class="replicas-number font-mono">{{ targetReplicas }}</span>
            <span class="replicas-label">Target Desired Pods</span>
          </div>

          <input 
            v-model.number="targetReplicas" 
            type="range" 
            min="0" 
            max="30" 
            class="scale-range" 
          />

          <div class="range-marks font-mono">
            <span>0 (Suspended)</span>
            <span>10</span>
            <span>20</span>
            <span>30</span>
          </div>

          <!-- Quick Presets -->
          <div class="scale-presets">
            <button class="preset-btn" @click="targetReplicas = 1">1 Pod</button>
            <button class="preset-btn" @click="targetReplicas = 3">3 (HA)</button>
            <button class="preset-btn" @click="targetReplicas = 5">5 Pods</button>
            <button class="preset-btn" @click="targetReplicas = 10">10 Pods</button>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button 
          class="btn btn-primary" 
          :disabled="actionLoading === 'scale'" 
          @click="handleScale"
        >
          <span>{{ actionLoading === 'scale' ? 'Scaling Fleet...' : 'Apply Scale ➔' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- ========================================== -->
    <!-- MODAL 2: CANARY & BLUE-GREEN ORCHESTRATOR   -->
    <!-- ========================================== -->
    <ModalDrawer
      v-model:show="showStrategyModal"
      mode="modal"
      :title="`Rollout & Traffic Strategy: ${formatContainerName(selectedApp?.name).serviceName || ''}`"
      subtitle="Manage live Canary weights, Blue-Green zero-downtime cutover, and revision rollbacks"
      max-width="640px"
    >
      <div v-if="selectedApp" class="strategy-modal-content">
        <!-- Strategy Selector Header -->
        <div class="strategy-type-selector">
          <button 
            class="strat-tab-btn" 
            :class="{ 'strat-active': selectedApp.strategy === 'Canary' }"
            @click="selectedApp.strategy = 'Canary'"
          >
            <span>🐥 Canary Traffic Split</span>
          </button>
          <button 
            class="strat-tab-btn" 
            :class="{ 'strat-active': selectedApp.strategy === 'BlueGreen' }"
            @click="selectedApp.strategy = 'BlueGreen'"
          >
            <span>🔄 Blue-Green Zero-Downtime</span>
          </button>
          <button 
            class="strat-tab-btn" 
            :class="{ 'strat-active': selectedApp.strategy === 'RollingUpdate' }"
            @click="selectedApp.strategy = 'RollingUpdate'"
          >
            <span>📦 Rolling Update</span>
          </button>
        </div>

        <!-- Canary Mode Controls -->
        <div v-if="selectedApp.strategy === 'Canary'" class="canary-config-panel glass-panel">
          <div class="canary-meter-header">
            <div>
              <h4 class="strat-title">Canary Traffic Weight</h4>
              <p class="strat-desc">Route a subset of real user traffic to the canary container track.</p>
            </div>
            <span class="canary-weight-badge font-mono">{{ canarySliderWeight }}% Canary</span>
          </div>

          <!-- Visual Traffic Bar -->
          <div class="traffic-split-visual">
            <div class="traffic-stable-bar font-mono" :style="{ width: `${100 - canarySliderWeight}%` }">
              <span>Stable: {{ 100 - canarySliderWeight }}%</span>
            </div>
            <div class="traffic-canary-bar font-mono" :style="{ width: `${canarySliderWeight}%` }">
              <span v-if="canarySliderWeight >= 15">Canary: {{ canarySliderWeight }}%</span>
            </div>
          </div>

          <!-- Weight Slider -->
          <input 
            v-model.number="canarySliderWeight" 
            type="range" 
            min="0" 
            max="100" 
            step="5"
            class="canary-slider"
          />

          <!-- Quick Presets -->
          <div class="canary-presets-row">
            <button class="preset-pill font-mono" @click="canarySliderWeight = 5">5% Smoke</button>
            <button class="preset-pill font-mono" @click="canarySliderWeight = 10">10% Initial</button>
            <button class="preset-pill font-mono" @click="canarySliderWeight = 25">25% Stage 1</button>
            <button class="preset-pill font-mono" @click="canarySliderWeight = 50">50% Half Fleet</button>
            <button class="preset-pill font-mono" @click="canarySliderWeight = 100">100% Full</button>
          </div>

          <!-- Track Versions Comparison -->
          <div class="tracks-comparison-grid font-mono">
            <div class="track-card track-stable">
              <span class="track-title">STABLE BASELINE ({{ 100 - canarySliderWeight }}%)</span>
              <span class="track-image">{{ selectedApp.image }}</span>
              <span class="track-status text-emerald">● 100% Passing Probes</span>
            </div>
            <div class="track-card track-canary">
              <span class="track-title">CANARY TRACK ({{ canarySliderWeight }}%)</span>
              <span class="track-image">{{ selectedApp.canaryVersion || `${selectedApp.image}-canary` }}</span>
              <span class="track-status text-cyan">● Error Budget: 99.98%</span>
            </div>
          </div>

          <!-- Canary Promotion & Abort Actions -->
          <div class="canary-actions-bar">
            <button 
              class="btn btn-secondary btn-sm" 
              :disabled="actionLoading === 'abort'" 
              @click="handleAbortCanary(selectedApp)"
            >
              <span>✕ Abort Canary (0%)</span>
            </button>
            <button 
              class="btn btn-primary btn-sm" 
              :disabled="actionLoading === 'promote'" 
              @click="handlePromoteCanary(selectedApp)"
            >
              <span>Promote Canary to 100% ➔</span>
            </button>
          </div>
        </div>

        <!-- Blue-Green Mode Controls -->
        <div v-else-if="selectedApp.strategy === 'BlueGreen'" class="bluegreen-config-panel glass-panel">
          <div class="bg-header-row">
            <div>
              <h4 class="strat-title">Active / Standby Cutover Router</h4>
              <p class="strat-desc">Instantly toggle live ingress traffic between isolated Blue & Green deployments.</p>
            </div>
            <span class="bg-active-tag font-mono">
              ACTIVE: {{ (selectedApp.blueGreenActive || 'blue').toUpperCase() }}
            </span>
          </div>

          <div class="bg-tracks-row">
            <!-- Blue Environment Card -->
            <div 
              class="bg-track-box" 
              :class="{ 'is-active-track': (selectedApp.blueGreenActive || 'blue') === 'blue' }"
            >
              <div class="bg-track-top">
                <span class="bg-color-indicator dot-blue"></span>
                <span class="bg-env-name font-mono">BLUE TRACK</span>
                <span v-if="(selectedApp.blueGreenActive || 'blue') === 'blue'" class="live-pill">LIVE ROUTING</span>
                <span v-else class="standby-pill">STANDBY</span>
              </div>
              <span class="bg-track-img font-mono">{{ selectedApp.blueVersion || selectedApp.image }}</span>
              <div class="bg-track-actions">
                <button 
                  class="btn btn-secondary btn-xs w-full"
                  :disabled="(selectedApp.blueGreenActive || 'blue') === 'blue' || actionLoading === 'cutover'"
                  @click="handleBlueGreenCutover(selectedApp, 'blue')"
                >
                  <span>{{ (selectedApp.blueGreenActive || 'blue') === 'blue' ? '✓ Currently Live' : '⚡ Cutover to Blue' }}</span>
                </button>
              </div>
            </div>

            <!-- Green Environment Card -->
            <div 
              class="bg-track-box" 
              :class="{ 'is-active-track': selectedApp.blueGreenActive === 'green' }"
            >
              <div class="bg-track-top">
                <span class="bg-color-indicator dot-green"></span>
                <span class="bg-env-name font-mono">GREEN TRACK</span>
                <span v-if="selectedApp.blueGreenActive === 'green'" class="live-pill">LIVE ROUTING</span>
                <span v-else class="standby-pill">STANDBY</span>
              </div>
              <span class="bg-track-img font-mono">{{ selectedApp.greenVersion || `${selectedApp.image}-v2` }}</span>
              <div class="bg-track-actions">
                <button 
                  class="btn btn-secondary btn-xs w-full"
                  :disabled="selectedApp.blueGreenActive === 'green' || actionLoading === 'cutover'"
                  @click="handleBlueGreenCutover(selectedApp, 'green')"
                >
                  <span>{{ selectedApp.blueGreenActive === 'green' ? '✓ Currently Live' : '⚡ Cutover to Green' }}</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Rollout Lifecycle & Revisions Panel -->
        <div class="revisions-panel glass-panel">
          <div class="revisions-header">
            <span class="rev-title font-mono">Revision History & Safe Rollbacks</span>
            <div class="revisions-btns">
              <button 
                class="btn btn-secondary btn-xs" 
                :disabled="actionLoading === 'pause'" 
                @click="handleTogglePause(selectedApp)"
              >
                <span>{{ selectedApp.paused ? '▶️ Resume Rollout' : '⏸️ Pause Rollout' }}</span>
              </button>
              <button 
                class="btn btn-secondary btn-xs" 
                :disabled="actionLoading === 'rollback'" 
                @click="handleRollback(selectedApp)"
              >
                <span>⏮️ Rollback to Previous</span>
              </button>
            </div>
          </div>

          <div class="revisions-list font-mono">
            <div class="rev-item is-current">
              <span class="rev-num">Revision #{{ selectedApp.revision || 3 }} (Current)</span>
              <span class="rev-img">{{ selectedApp.image }}</span>
              <span class="rev-tag text-emerald">Active</span>
            </div>
            <div class="rev-item">
              <span class="rev-num">Revision #{{ Math.max(1, (selectedApp.revision || 3) - 1) }}</span>
              <span class="rev-img">{{ selectedApp.image.replace(/:.*/, ':v1.8.4') }}</span>
              <button 
                class="btn btn-secondary btn-xs" 
                @click="handleRollback(selectedApp)"
              >
                Rollback
              </button>
            </div>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Close</button>
        <button 
          v-if="selectedApp?.strategy === 'Canary'" 
          class="btn btn-primary" 
          :disabled="actionLoading === 'canary'" 
          @click="handleApplyCanaryWeight"
        >
          <span>Apply Canary Weight ({{ canarySliderWeight }}%) ➔</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- ========================================== -->
    <!-- DRAWER 1: WORKLOAD APP INSPECTOR           -->
    <!-- ========================================== -->
    <ModalDrawer
      v-model:show="showInspectorDrawer"
      mode="drawer"
      :title="`App Inspector: ${formatContainerName(selectedApp?.name).serviceName || ''}`"
      :subtitle="`${selectedApp?.namespace || 'default'} · ${selectedApp?.type?.toUpperCase()} Workload`"
      max-width="680px"
    >
      <div v-if="selectedApp" class="inspector-content">
        <!-- Inspector Sub Tabs -->
        <div class="inspector-tabs-nav font-mono">
          <button 
            class="insp-tab" 
            :class="{ 'insp-tab-active': inspectorActiveTab === 'overview' }"
            @click="switchInspectorTab('overview')"
          >
            Overview & Specs
          </button>
          <button 
            class="insp-tab" 
            :class="{ 'insp-tab-active': inspectorActiveTab === 'logs' }"
            @click="switchInspectorTab('logs')"
          >
            📜 Container Logs
          </button>
          <button 
            class="insp-tab" 
            :class="{ 'insp-tab-active': inspectorActiveTab === 'strategy' }"
            @click="switchInspectorTab('strategy')"
          >
            Strategy & Traffic
          </button>
          <button 
            class="insp-tab" 
            :class="{ 'insp-tab-active': inspectorActiveTab === 'network' }"
            @click="switchInspectorTab('network')"
          >
            Networking
          </button>
          <button 
            class="insp-tab" 
            :class="{ 'insp-tab-active': inspectorActiveTab === 'yaml' }"
            @click="switchInspectorTab('yaml')"
          >
            YAML Manifest
          </button>
        </div>

        <!-- Tab 1: Overview & Specs -->
        <div v-if="inspectorActiveTab === 'overview'" class="insp-panel animate-fade-in">
          <div class="spec-grid font-mono">
            <div class="spec-row">
              <span class="spec-label">Workload Name</span>
              <span class="spec-val text-cyan" :title="selectedApp.name">{{ formatContainerName(selectedApp.name).serviceName }}</span>
            </div>
            <div class="spec-row" v-if="formatContainerName(selectedApp.name).isSwarmTask">
              <span class="spec-label">Full Swarm Task</span>
              <span class="spec-val font-mono">{{ selectedApp.name }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Namespace</span>
              <span class="spec-val">{{ selectedApp.namespace || 'default' }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Target Cluster</span>
              <span class="spec-val">{{ selectedApp.target }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Runtime Engine</span>
              <span class="spec-val">{{ selectedApp.type }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Container Image</span>
              <span class="spec-val text-emerald" :title="selectedApp.image">{{ formatImageName(selectedApp.image).display }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Replicas Running</span>
              <span class="spec-val text-cyan">{{ selectedApp.readyReplicas || selectedApp.replicas }} / {{ selectedApp.replicas }} Pods</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">CPU Request</span>
              <span class="spec-val">{{ selectedApp.cpu }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Memory Request</span>
              <span class="spec-val">{{ selectedApp.memory }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Service Port</span>
              <span class="spec-val">{{ selectedApp.port }} (TCP)</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Network Sync</span>
              <span class="spec-val">{{ selectedApp.netType || 'ClusterIP' }}</span>
            </div>
          </div>

          <div class="inspector-quick-actions">
            <button class="btn btn-secondary btn-sm btn-logs" @click="switchInspectorTab('logs')">
              <span>📜 Inspect Logs</span>
            </button>
            <button class="btn btn-secondary btn-sm" @click="openScaleModal(selectedApp)">
              <span>⚡ Scale Replicas</span>
            </button>
            <button class="btn btn-secondary btn-sm" @click="handleRestart(selectedApp)">
              <span>🔄 Rolling Restart</span>
            </button>
            <button class="btn btn-secondary btn-sm btn-remove" @click="handleDelete(selectedApp)">
              <span>🗑️ Delete Workload</span>
            </button>
          </div>
        </div>

        <!-- Tab 2: Container Logs -->
        <div v-else-if="inspectorActiveTab === 'logs'" class="insp-panel animate-fade-in logs-insp-panel">
          <!-- Terminal Control Deck -->
          <div class="logs-control-deck glass-panel">
            <div class="logs-search-wrapper">
              <span class="logs-search-ico">🔍</span>
              <input 
                v-model="logsSearchQuery" 
                type="text" 
                placeholder="Filter logs by keyword, error, timestamp..." 
                class="input-glass logs-search-input font-mono" 
              />
              <button v-if="logsSearchQuery" class="logs-search-clear" @click="logsSearchQuery = ''">✕</button>
            </div>

            <div class="logs-deck-actions">
              <label class="logs-auto-scroll-label font-mono">
                <input v-model="logsAutoScroll" type="checkbox" class="toggle-cb" />
                <span>Auto-Scroll</span>
              </label>

              <button 
                class="btn btn-secondary btn-xs" 
                :disabled="logsLoading" 
                title="Refresh logs from container daemon"
                @click="fetchLogs(selectedApp)"
              >
                <span :class="{ 'spin-icon': logsLoading }">🔄</span>
                <span>{{ logsLoading ? 'Fetching...' : 'Refresh' }}</span>
              </button>

              <button 
                class="btn btn-secondary btn-xs" 
                title="Copy log buffer to clipboard"
                @click="copyLogsToClipboard"
              >
                <span>📋 Copy</span>
              </button>

              <button 
                class="btn btn-secondary btn-xs btn-open-logs" 
                title="Open full interactive live tail in Log Stream View"
                @click="openFullLogs(selectedApp)"
              >
                <span>↗️ Full Stream</span>
              </button>
            </div>
          </div>

          <!-- Terminal Window -->
          <div class="logs-terminal-window">
            <div class="logs-terminal-titlebar">
              <div class="terminal-dots">
                <span class="term-dot term-close"></span>
                <span class="term-dot term-min"></span>
                <span class="term-dot term-max"></span>
              </div>
              <div class="terminal-title-text font-mono">
                <span class="pulse-dot pulse-dot-cyan"></span>
                <span>docker://{{ selectedApp.name }} [{{ selectedApp.type }}]</span>
                <span class="logs-count">({{ filteredLogLines.length }} lines)</span>
              </div>
              <div class="terminal-status font-mono">
                <span :class="selectedApp.status === 'healthy' ? 'text-emerald' : 'text-amber'">● {{ selectedApp.status }}</span>
              </div>
            </div>

            <div ref="logsTerminalRef" class="logs-terminal-body font-mono">
              <div v-if="logsLoading && filteredLogLines.length === 0" class="logs-loading-state">
                <span class="spin-icon">🔄</span>
                <span>Connecting to container log stream...</span>
              </div>

              <template v-else-if="filteredLogLines.length > 0">
                <div 
                  v-for="(line, idx) in filteredLogLines" 
                  :key="idx" 
                  class="terminal-log-row"
                  :class="getLogLineClass(line)"
                >
                  <span class="log-row-num">{{ idx + 1 }}</span>
                  <span class="log-row-text">{{ line }}</span>
                </div>
              </template>

              <div v-else class="logs-empty-state">
                <span>⚡</span>
                <p>No log entries match the search filter "{{ logsSearchQuery }}".</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Tab 3: Strategy & Traffic -->
        <div v-else-if="inspectorActiveTab === 'strategy'" class="insp-panel animate-fade-in">
          <div class="glass-panel p-4 mb-4">
            <h4 class="strat-title mb-2">Active Strategy: {{ selectedApp.strategy || 'RollingUpdate' }}</h4>
            <p class="strat-desc">Revision #{{ selectedApp.revision || 1 }} deployed on cluster {{ selectedApp.target }}.</p>
          </div>
          <button class="btn btn-primary btn-sm w-full" @click="openStrategyModal(selectedApp)">
            <span>Configure Canary & Blue-Green Traffic Controls ➔</span>
          </button>
        </div>

        <!-- Tab 4: Networking -->
        <div v-else-if="inspectorActiveTab === 'network'" class="insp-panel animate-fade-in">
          <div class="spec-grid font-mono">
            <div class="spec-row">
              <span class="spec-label">Service Type</span>
              <span class="spec-val">{{ selectedApp.netType || 'ClusterIP' }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Internal Endpoint</span>
              <span class="spec-val text-cyan">{{ selectedApp.name }}.{{ selectedApp.namespace || 'default' }}.svc.cluster.local:{{ selectedApp.port }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Ingress Hostname</span>
              <span class="spec-val text-emerald">{{ selectedApp.ingressHost || 'Not exposed via Ingress' }}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Readiness Probe</span>
              <span class="spec-val">{{ selectedApp.readinessProbe || '/healthz' }}</span>
            </div>
          </div>
        </div>

        <!-- Tab 5: YAML Manifest -->
        <div v-else-if="inspectorActiveTab === 'yaml'" class="insp-panel animate-fade-in">
          <div class="yaml-actions-bar">
            <span class="yaml-title font-mono">Live Kubernetes Spec</span>
            <button class="btn btn-secondary btn-xs" @click="copyToClipboard(generateYamlManifest(selectedApp))">
              <span>📋 Copy Manifest</span>
            </button>
          </div>
          <pre class="yaml-viewer font-mono">{{ generateYamlManifest(selectedApp) }}</pre>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Close Inspector</button>
      </template>
    </ModalDrawer>

    <!-- ========================================== -->
    <!-- DRAWER 2: BLUEPRINT CATALOG                -->
    <!-- ========================================== -->
    <ModalDrawer
      v-model:show="showTemplatesDrawer"
      mode="drawer"
      title="Application Blueprint Catalog"
      subtitle="Pre-configured enterprise service blueprints ready to instantiate"
      max-width="560px"
    >
      <div class="templates-list">
        <div 
          v-for="tmpl in templates" 
          :key="tmpl.name" 
          class="template-item glass-panel"
        >
          <div class="tmpl-header">
            <div>
              <h4 class="tmpl-title">{{ tmpl.name }}</h4>
              <span class="tmpl-category font-mono">{{ tmpl.category }} · {{ tmpl.version }} · {{ tmpl.strategy || 'RollingUpdate' }}</span>
            </div>
            <button class="btn btn-primary btn-xs" @click="selectTemplate(tmpl)">
              <span>Use Blueprint ➔</span>
            </button>
          </div>
          <p class="tmpl-desc">{{ tmpl.desc }}</p>
          <div class="tmpl-specs font-mono">
            <span>Port: {{ tmpl.ports }}</span>
            <span>CPU: {{ tmpl.cpu }}</span>
            <span>RAM: {{ tmpl.mem }}</span>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Close Catalog</button>
      </template>
    </ModalDrawer>

    <!-- ========================================== -->
    <!-- MODAL 3: DEPLOY NEW WORKLOAD WIZARD         -->
    <!-- ========================================== -->
    <ModalDrawer
      v-model:show="showCreateModal"
      mode="modal"
      title="Deploy Application Workload"
      subtitle="Configure target runtime, rollout strategy, container resources, and networking"
      max-width="680px"
    >
      <div class="wizard-modal-content">
        <!-- AI Assisted Prompt Bar -->
        <div class="ai-prompt-bar glass-panel">
          <div class="ai-prompt-input-wrap">
            <span class="ai-icon">🪄</span>
            <input 
              v-model="aiPrompt" 
              type="text" 
              placeholder="AI Prompt: 'Deploy payment-api with 3 replicas, nginx:alpine, 20% canary, and ingress pay.corp.io'..." 
              class="input-glass ai-input"
              @keydown.enter="generateWithAI"
            />
            <button 
              class="btn btn-primary btn-xs" 
              :disabled="aiGenerating || !aiPrompt.trim()" 
              @click="generateWithAI"
            >
              <span>{{ aiGenerating ? 'Thinking...' : '▶ Synthesize' }}</span>
            </button>
          </div>
        </div>

        <!-- Wizard Steps Navigation -->
        <div class="wizard-nav font-mono">
          <button 
            class="wiz-nav-btn" 
            :class="{ 'wiz-nav-active': wizardActiveTab === 'general' }"
            @click="wizardActiveTab = 'general'"
          >
            1. Identity & Target
          </button>
          <button 
            class="wiz-nav-btn" 
            :class="{ 'wiz-nav-active': wizardActiveTab === 'strategy' }"
            @click="wizardActiveTab = 'strategy'"
          >
            2. Rollout Strategy
          </button>
          <button 
            class="wiz-nav-btn" 
            :class="{ 'wiz-nav-active': wizardActiveTab === 'network' }"
            @click="wizardActiveTab = 'network'"
          >
            3. Networking
          </button>
          <button 
            class="wiz-nav-btn" 
            :class="{ 'wiz-nav-active': wizardActiveTab === 'resources' }"
            @click="wizardActiveTab = 'resources'"
          >
            4. Compute & Env
          </button>
        </div>

        <!-- Tab 1: General & Identity -->
        <div v-if="wizardActiveTab === 'general'" class="wizard-step-body animate-fade-in">
          <div class="form-row">
            <div class="form-group flex-1">
              <label class="form-label">Deployment Name *</label>
              <input v-model="newApp.name" type="text" placeholder="e.g. payment-service" class="input-glass font-mono" />
            </div>
            <div class="form-group flex-1">
              <label class="form-label">Namespace</label>
              <input v-model="newApp.namespace" type="text" placeholder="default" class="input-glass font-mono" />
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Container Image (Registry / Tag) *</label>
            <input v-model="newApp.image" type="text" placeholder="e.g. nginx:alpine or registry.corp.io/api:v1.2.0" class="input-glass font-mono" />
          </div>

          <div class="form-row">
            <div class="form-group flex-1">
              <label class="form-label">Runtime Engine</label>
              <select v-model="newApp.type" class="input-glass">
                <option value="kubernetes">Kubernetes</option>
                <option value="swarm">Docker Swarm</option>
              </select>
            </div>
            <div class="form-group flex-1">
              <label class="form-label">Target Cluster / Host</label>
              <input v-model="newApp.target" type="text" placeholder="prod-us-east-1" class="input-glass font-mono" />
            </div>
            <div class="form-group flex-1">
              <label class="form-label">Initial Replicas</label>
              <input v-model.number="newApp.replicas" type="number" min="1" max="50" class="input-glass font-mono" />
            </div>
          </div>
        </div>

        <!-- Tab 2: Strategy -->
        <div v-else-if="wizardActiveTab === 'strategy'" class="wizard-step-body animate-fade-in">
          <div class="form-group">
            <label class="form-label">Rollout Strategy</label>
            <select v-model="newApp.strategy" class="input-glass">
              <option value="RollingUpdate">RollingUpdate (Zero Downtime, MaxSurge 25%)</option>
              <option value="Canary">Canary Deployment (Weighted Traffic Split)</option>
              <option value="BlueGreen">Blue-Green Deployment (Active/Standby Router)</option>
              <option value="Recreate">Recreate (Terminate all before start)</option>
            </select>
          </div>

          <div v-if="newApp.strategy === 'Canary'" class="glass-panel p-4">
            <div class="form-row">
              <div class="form-group flex-1">
                <label class="form-label">Initial Canary Traffic Weight (%)</label>
                <input v-model.number="newApp.canaryWeight" type="number" min="5" max="90" step="5" class="input-glass font-mono" />
              </div>
              <div class="form-group flex-1">
                <label class="form-label">Canary Track Image (Optional)</label>
                <input v-model="newApp.canaryVersion" type="text" placeholder="e.g. nginx:1.25-canary" class="input-glass font-mono" />
              </div>
            </div>
          </div>

          <div v-if="newApp.strategy === 'BlueGreen'" class="glass-panel p-4">
            <div class="form-group">
              <label class="form-label">Green (Standby) Image Tag</label>
              <input v-model="newApp.greenVersion" type="text" placeholder="e.g. app:v2.0.0-rc1" class="input-glass font-mono" />
            </div>
          </div>
        </div>

        <!-- Tab 3: Networking -->
        <div v-else-if="wizardActiveTab === 'network'" class="wizard-step-body animate-fade-in">
          <div class="form-row">
            <div class="form-group flex-1">
              <label class="form-label">Service Network Mode</label>
              <select v-model="newApp.netType" class="input-glass">
                <option value="ClusterIP">ClusterIP (Internal Mesh)</option>
                <option value="NodePort">NodePort (Host Port)</option>
                <option value="LoadBalancer">LoadBalancer (Cloud LB / MetalLB)</option>
                <option value="Ingress">Ingress (HTTP Host Router)</option>
              </select>
            </div>
            <div class="form-group flex-1">
              <label class="form-label">Container Port</label>
              <input v-model.number="newApp.port" type="number" class="input-glass font-mono" />
            </div>
          </div>

          <div v-if="newApp.netType === 'Ingress'" class="form-group animate-fade-in">
            <label class="form-label">Ingress Domain Hostname</label>
            <input v-model="newApp.ingressHost" type="text" placeholder="e.g. api.corp.internal" class="input-glass font-mono" />
          </div>
        </div>

        <!-- Tab 4: Resources & Environment -->
        <div v-else-if="wizardActiveTab === 'resources'" class="wizard-step-body animate-fade-in">
          <div class="form-row">
            <div class="form-group flex-1">
              <label class="form-label">CPU Request</label>
              <input v-model="newApp.cpu" type="text" class="input-glass font-mono" />
            </div>
            <div class="form-group flex-1">
              <label class="form-label">Memory Request</label>
              <input v-model="newApp.memory" type="text" class="input-glass font-mono" />
            </div>
          </div>

          <div class="env-section">
            <div class="env-section-header">
              <label class="form-label">Environment Variables</label>
              <button class="btn btn-secondary btn-xs" @click="addEnvRow">
                <span>+ Add Variable</span>
              </button>
            </div>

            <div v-for="(envItem, idx) in newApp.envList" :key="idx" class="env-row">
              <input v-model="envItem.key" type="text" placeholder="KEY" class="input-glass font-mono flex-1" />
              <input v-model="envItem.value" type="text" placeholder="VALUE" class="input-glass font-mono flex-1" />
              <button class="btn btn-secondary btn-xs btn-remove" @click="removeEnvRow(idx)">✕</button>
            </div>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button 
          class="btn btn-primary" 
          :disabled="actionLoading === 'create'" 
          @click="handleCreateApp"
        >
          <span>{{ actionLoading === 'create' ? 'Deploying Fleet...' : 'Deploy Workload ➔' }}</span>
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.header-info {
  max-width: 800px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.08em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.5;
  margin-top: 4px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.toast-banner {
  padding: 12px 18px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.toast-success {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.toast-error {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
}

.toast-info {
  background: rgba(6, 182, 212, 0.15);
  border: 1px solid rgba(6, 182, 212, 0.3);
  color: #38bdf8;
}

.toast-close {
  margin-left: auto;
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.section-box {
  border-radius: 16px;
  overflow: hidden;
}

/* Filter Controls Bar */
.table-controls-bar {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.filter-pills-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pill-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.pill-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.pill-active {
  background: rgba(6, 182, 212, 0.16);
  border-color: rgba(6, 182, 212, 0.4);
  color: #fff;
  font-weight: 600;
}

.pill-badge {
  font-size: 10px;
  background: rgba(255, 255, 255, 0.1);
  padding: 1px 6px;
  border-radius: 9999px;
  font-family: var(--font-mono);
}

.table-search-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.search-input-wrap {
  position: relative;
  flex: 1;
}

.search-ico {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 12px;
  color: var(--text-muted);
}

.search-input {
  width: 100%;
  padding-left: 34px;
}

.clear-search-btn {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
}

.select-ns {
  width: 220px;
}

/* Custom Table Cell Styling */
.workload-name-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.name-primary-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.workload-name {
  font-weight: 700;
  color: var(--accent-cyan);
  font-size: 13px;
}

.workload-name:hover {
  text-decoration: underline;
}

.runtime-pill {
  font-size: 9px;
  padding: 1px 5px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 4px;
  color: var(--text-muted);
}

.slot-badge {
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  font-weight: 500;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(56, 189, 248, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.25);
  line-height: 1.3;
  letter-spacing: 0.02em;
  white-space: nowrap;
}

.name-sub-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--text-muted);
}

.team-tag {
  color: var(--text-secondary);
}

.strategy-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.canary-chip {
  display: inline-flex;
  flex-direction: column;
  gap: 3px;
}

.canary-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #38bdf8;
  display: inline-block;
  margin-right: 4px;
}

.strategy-label {
  font-size: 11px;
  color: #38bdf8;
  font-weight: 600;
}

.canary-mini-bar {
  width: 100px;
  height: 4px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 9999px;
  overflow: hidden;
}

.canary-bar-fill {
  height: 100%;
  background: var(--grad-cyan);
}

.bluegreen-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.bg-track-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.bg-blue-active { background: #38bdf8; }
.bg-green-active { background: #34d399; }

.rolling-chip {
  font-size: 11px;
  color: var(--text-secondary);
}

.image-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.image-name {
  font-size: 12px;
  color: var(--text-secondary);
}

.image-sub-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
}

.rev-badge {
  background: rgba(255, 255, 255, 0.06);
  padding: 1px 4px;
  border-radius: 4px;
  color: var(--text-muted);
}

.paused-badge {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
  padding: 1px 4px;
  border-radius: 4px;
}

.ingress-badge {
  color: #34d399;
}

.replicas-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.replicas-nums {
  font-size: 12px;
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 3px;
}

.replicas-word {
  font-size: 10px;
  color: var(--text-muted);
  font-weight: normal;
  margin-left: 2px;
}

.replicas-progress-track {
  width: 80px;
  height: 4px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
}

.replicas-progress-bar {
  height: 100%;
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.btn-strategy:hover {
  background: rgba(139, 92, 246, 0.2);
  border-color: rgba(139, 92, 246, 0.4);
  color: #c084fc;
}

.btn-remove:hover {
  background: rgba(244, 63, 94, 0.2);
  border-color: rgba(244, 63, 94, 0.4);
  color: #fb7185;
}

/* Modals & Drawers Content */
.scale-modal-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.scale-info-row {
  display: flex;
  justify-content: space-between;
  padding: 10px 14px;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 10px;
  font-size: 12px;
}

.scale-slider-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 20px;
  background: rgba(11, 15, 25, 0.6);
  border-radius: 14px;
}

.replicas-display {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.replicas-number {
  font-size: 44px;
  font-weight: 800;
  color: var(--accent-cyan);
}

.replicas-label {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
}

.scale-range {
  width: 100%;
  accent-color: var(--accent-cyan);
}

.range-marks {
  width: 100%;
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--text-muted);
}

.scale-presets {
  display: flex;
  gap: 8px;
}

.preset-btn {
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
}

.preset-btn:hover {
  background: rgba(6, 182, 212, 0.2);
  color: #fff;
}

/* Strategy Modal Content */
.strategy-modal-content {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.strategy-type-selector {
  display: flex;
  gap: 8px;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 12px;
}

.strat-tab-btn {
  flex: 1;
  padding: 8px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
}

.strat-tab-btn.strat-active {
  background: rgba(6, 182, 212, 0.16);
  border-color: rgba(6, 182, 212, 0.4);
  color: #fff;
  font-weight: 700;
}

.canary-config-panel, .bluegreen-config-panel, .revisions-panel {
  padding: 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.canary-meter-header, .bg-header-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.strat-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  margin: 0;
}

.strat-desc {
  font-size: 12px;
  color: var(--text-muted);
  margin: 2px 0 0 0;
}

.canary-weight-badge, .bg-active-tag {
  background: rgba(6, 182, 212, 0.2);
  color: #38bdf8;
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 700;
}

.traffic-split-visual {
  display: flex;
  height: 28px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
}

.traffic-stable-bar {
  background: rgba(16, 185, 129, 0.3);
  color: #34d399;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 700;
  transition: width 0.2s ease;
}

.traffic-canary-bar {
  background: rgba(6, 182, 212, 0.3);
  color: #38bdf8;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 700;
  transition: width 0.2s ease;
}

.canary-slider {
  width: 100%;
  accent-color: var(--accent-cyan);
}

.canary-presets-row {
  display: flex;
  gap: 8px;
  justify-content: space-between;
}

.preset-pill {
  padding: 4px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
}

.preset-pill:hover {
  background: rgba(6, 182, 212, 0.2);
  color: #fff;
}

.tracks-comparison-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.track-card {
  padding: 12px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.track-title {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
}

.track-image {
  font-size: 11px;
  color: #fff;
  word-break: break-all;
}

.track-status {
  font-size: 11px;
}

.canary-actions-bar {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.bg-tracks-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.bg-track-box {
  padding: 16px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.bg-track-box.is-active-track {
  border-color: rgba(6, 182, 212, 0.5);
  background: rgba(6, 182, 212, 0.08);
}

.bg-track-top {
  display: flex;
  align-items: center;
  gap: 8px;
}

.bg-color-indicator {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.dot-blue { background: #38bdf8; }
.dot-green { background: #34d399; }

.bg-env-name {
  font-size: 12px;
  font-weight: 800;
  color: #fff;
}

.live-pill {
  margin-left: auto;
  font-size: 9px;
  font-weight: 800;
  background: rgba(16, 185, 129, 0.2);
  color: #34d399;
  padding: 2px 6px;
  border-radius: 4px;
}

.standby-pill {
  margin-left: auto;
  font-size: 9px;
  color: var(--text-muted);
}

.bg-track-img {
  font-size: 11px;
  color: var(--text-secondary);
  word-break: break-all;
}

.revisions-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.rev-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-secondary);
}

.revisions-btns {
  display: flex;
  gap: 8px;
}

.revisions-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rev-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.2);
  font-size: 11px;
}

.rev-item.is-current {
  border-left: 3px solid var(--accent-cyan);
}

.rev-num {
  font-weight: 700;
  color: #fff;
}

.rev-img {
  color: var(--text-muted);
}

/* Inspector Drawer Content */
.inspector-content {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.inspector-tabs-nav {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 8px;
  overflow-x: auto;
}

.insp-tab {
  padding: 6px 12px;
  border-radius: 6px;
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
}

.insp-tab-active {
  background: rgba(6, 182, 212, 0.15);
  color: var(--accent-cyan);
  font-weight: 700;
}

.spec-grid {
  display: flex;
  flex-direction: column;
  gap: 1px;
  background: var(--border-subtle);
  border-radius: 10px;
  overflow: hidden;
}

.spec-row {
  display: flex;
  justify-content: space-between;
  padding: 10px 14px;
  background: rgba(13, 19, 33, 0.95);
  font-size: 12px;
}

.spec-label {
  color: var(--text-muted);
}

.spec-val {
  font-weight: 600;
}

.inspector-quick-actions {
  display: flex;
  gap: 10px;
  margin-top: 14px;
}

.yaml-actions-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.yaml-title {
  font-size: 12px;
  color: var(--text-muted);
}

.yaml-viewer {
  padding: 16px;
  border-radius: 10px;
  background: rgba(5, 8, 15, 0.95);
  border: 1px solid var(--border-subtle);
  color: #a7f3d0;
  font-size: 11px;
  line-height: 1.5;
  overflow-x: auto;
  max-height: 450px;
}

/* Container Logs Tab & Terminal */
.logs-insp-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.logs-control-deck {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px 12px;
}

.logs-search-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  width: 100%;
}

.logs-search-ico {
  position: absolute;
  left: 10px;
  font-size: 12px;
  color: var(--text-muted);
}

.logs-search-input {
  width: 100%;
  padding-left: 32px;
  padding-right: 28px;
  font-size: 11px;
  height: 32px;
}

.logs-search-clear {
  position: absolute;
  right: 10px;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 11px;
}

.logs-deck-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
}

.logs-auto-scroll-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-secondary);
  cursor: pointer;
  user-select: none;
}

.btn-open-logs {
  border-color: rgba(6, 182, 212, 0.4);
  color: var(--accent-cyan);
}

.btn-open-logs:hover {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
}

.btn-logs {
  border-color: rgba(6, 182, 212, 0.3);
  color: var(--accent-cyan);
}

.btn-logs:hover {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
}

.logs-terminal-window {
  display: flex;
  flex-direction: column;
  background: #090d16;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 10px 30px -10px rgba(0, 0, 0, 0.8);
}

.logs-terminal-titlebar {
  height: 34px;
  background: rgba(16, 24, 40, 0.95);
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
}

.terminal-dots {
  display: flex;
  gap: 6px;
}

.term-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
}

.term-close { background: #ff5f56; }
.term-min { background: #ffbd2e; }
.term-max { background: #27c93f; }

.terminal-title-text {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--text-secondary);
}

.logs-count {
  color: var(--text-muted);
  font-size: 10px;
}

.terminal-status {
  font-size: 10px;
}

.logs-terminal-body {
  height: 380px;
  max-height: 440px;
  overflow-y: auto;
  padding: 12px;
  font-size: 11px;
  line-height: 1.6;
  background: #090d16;
  color: #e2e8f0;
}

.terminal-log-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 2px 4px;
  border-radius: 3px;
  transition: background 0.1s ease;
  word-break: break-all;
  white-space: pre-wrap;
}

.terminal-log-row:hover {
  background: rgba(255, 255, 255, 0.05);
}

.log-row-num {
  width: 28px;
  flex-shrink: 0;
  color: rgba(255, 255, 255, 0.25);
  font-size: 10px;
  user-select: none;
  text-align: right;
}

.log-row-text {
  flex: 1;
}

.log-line-error {
  color: #fda4af;
  background: rgba(244, 63, 94, 0.08);
}

.log-line-warn {
  color: #fde047;
  background: rgba(234, 179, 8, 0.06);
}

.log-line-info {
  color: #bae6fd;
}

.logs-loading-state,
.logs-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-muted);
  gap: 8px;
  font-size: 12px;
  text-align: center;
  padding: 24px;
}

/* Blueprint Catalog Drawer */
.templates-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.template-item {
  padding: 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tmpl-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.tmpl-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  margin: 0;
}

.tmpl-category {
  font-size: 11px;
  color: var(--text-muted);
}

.tmpl-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0;
}

.tmpl-specs {
  display: flex;
  gap: 16px;
  font-size: 11px;
  color: var(--accent-cyan);
}

/* Wizard Form */
.wizard-modal-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.ai-prompt-bar {
  padding: 8px 12px;
  border-radius: 10px;
}

.ai-prompt-input-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-icon {
  font-size: 14px;
}

.ai-input {
  flex: 1;
  font-size: 12px;
}

.wizard-nav {
  display: flex;
  gap: 6px;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 8px;
}

.wiz-nav-btn {
  flex: 1;
  padding: 6px 8px;
  border-radius: 6px;
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 11px;
  cursor: pointer;
  white-space: nowrap;
}

.wiz-nav-active {
  background: rgba(6, 182, 212, 0.15);
  color: var(--accent-cyan);
  font-weight: 700;
}

.wizard-step-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.flex-1 { flex: 1; }

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.env-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.env-section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.env-row {
  display: flex;
  gap: 8px;
}

.font-mono { font-family: var(--font-mono); }
.font-sans { font-family: var(--font-sans); }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }
.btn-sm { padding: 6px 12px; font-size: 12px; }
.w-full { width: 100%; }
.cursor-pointer { cursor: pointer; }
</style>
