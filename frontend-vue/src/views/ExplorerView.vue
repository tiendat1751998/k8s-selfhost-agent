<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import SecretViewer from '../components/k8s/SecretViewer.vue'
import YamlEditorModal from '../components/k8s/YamlEditorModal.vue'
import CreateResourceModal from '../components/k8s/CreateResourceModal.vue'
import PodTerminal from '../components/k8s/PodTerminal.vue'
import PodLogViewer from '../components/k8s/PodLogViewer.vue'
import EventsTimeline from '../components/k8s/EventsTimeline.vue'
import {
  k8sApi,
  type K8sResource,
  type K8sNamespace,
  type ResourceKind,
  type NodeTaint,
  type NodeCondition,
  type NodeSystemInfo,
  type K8sEvent
} from '../api/k8s'
import { fleetApi, type Cluster } from '../api/compute'
import { jsonToYaml } from '../utils/yaml'

const route = useRoute()

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

// Create Modal State
const showCreateModal = ref(false)

// YAML Editor Modal State
const showYamlModal = ref(false)
const yamlEditorMode = ref<'create' | 'edit'>('create')
const yamlEditorInitialContent = ref('')
const yamlEditorTitle = ref('Apply Kubernetes Manifest')

// Detail Drawer State
const showDetailDrawer = ref(false)
const selectedResource = ref<K8sResource | null>(null)
const activeDrawerTab = ref<'overview' | 'events' | 'yaml'>('overview')
const drawerYamlCopied = ref(false)

// New Namespace Modal State
const showNewNsModal = ref(false)
const newNsName = ref('')
const creatingNs = ref(false)
const newNsError = ref<string | null>(null)

// Delete Confirmation Modal State
const showDeleteModal = ref(false)
const resourceToDelete = ref<K8sResource | null>(null)
const deletingResource = ref(false)

// Pod Terminal & Logs Modal State
const showTerminalModal = ref(false)
const showLogsModal = ref(false)
const terminalPod = ref<K8sResource | null>(null)
const logsPod = ref<K8sResource | null>(null)
const selectedPodContainer = ref<string>('')

// Resource Kind Tree Definition
interface KindCategory {
  title: string
  icon: string
  items: { label: string; kind: ResourceKind; icon: string }[]
}

const kindCategories: KindCategory[] = [
  {
    title: 'Cluster / Infrastructure',
    icon: '🖥️',
    items: [
      { label: 'Nodes', kind: 'nodes', icon: '🖥️' },
    ],
  },
  {
    title: 'Workloads',
    icon: '📦',
    items: [
      { label: 'Pods', kind: 'pods', icon: '🫛' },
      { label: 'Deployments', kind: 'deployments', icon: '🚀' },
      { label: 'StatefulSets', kind: 'statefulsets', icon: '🗄️' },
      { label: 'DaemonSets', kind: 'daemonsets', icon: '🛡️' },
      { label: 'Jobs', kind: 'jobs', icon: '⚡' },
      { label: 'CronJobs', kind: 'cronjobs', icon: '⏰' },
    ],
  },
  {
    title: 'Config & Storage',
    icon: '💾',
    items: [
      { label: 'ConfigMaps', kind: 'configmaps', icon: '🗺️' },
      { label: 'Secrets', kind: 'secrets', icon: '🔒' },
      { label: 'PersistentVolumeClaims', kind: 'persistentvolumeclaims', icon: '💾' },
      { label: 'StorageClasses', kind: 'storageclasses', icon: '🏷️' },
    ],
  },
  {
    title: 'Networking',
    icon: '🌐',
    items: [
      { label: 'Services', kind: 'services', icon: '🔌' },
      { label: 'Ingresses', kind: 'ingresses', icon: '🌐' },
    ],
  },
  {
    title: 'Observability',
    icon: '📢',
    items: [
      { label: 'Events', kind: 'events', icon: '📢' },
    ],
  },
]

// Import Cluster Modal State
const showImportModal = ref(false)
const importMode = ref<'file' | 'text'>('file')
const importForm = ref({
  id: '',
  name: '',
  group: 'production',
  region: 'us-east-1',
  provider: 'k8s',
  kubeconfigRaw: '',
})
const kubeconfigFile = ref<File | null>(null)
const importingCluster = ref(false)

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    kubeconfigFile.value = target.files[0]
  }
}

async function handleImportCluster() {
  if (!importForm.value.name.trim()) {
    showToast('Cluster name is required', 'error')
    return
  }

  let fileToUpload: File | null = kubeconfigFile.value

  if (importMode.value === 'text') {
    if (!importForm.value.kubeconfigRaw.trim()) {
      showToast('Kubeconfig content cannot be empty', 'error')
      return
    }
    fileToUpload = new File([importForm.value.kubeconfigRaw], `${importForm.value.name}-kubeconfig.yaml`, {
      type: 'text/yaml',
    })
  }

  if (!fileToUpload) {
    showToast('Kubeconfig file or content is required for cluster import', 'error')
    return
  }

  importingCluster.value = true
  try {
    const formData = new FormData()
    formData.append('id', importForm.value.id.trim() || `cluster-${Date.now().toString(36)}`)
    formData.append('name', importForm.value.name.trim())
    formData.append('group', importForm.value.group)
    formData.append('region', importForm.value.region.trim())
    formData.append('provider', importForm.value.provider)
    formData.append('kubeconfig', fileToUpload)

    await fleetApi.importCluster(formData)
    showToast(`Cluster ${importForm.value.name} successfully imported!`)
    showImportModal.value = false
    const importedName = importForm.value.name.trim()
    importForm.value.name = ''
    importForm.value.id = ''
    importForm.value.kubeconfigRaw = ''
    kubeconfigFile.value = null
    await loadClusters()
    selectedCluster.value = importedName
    await loadNamespaces()
    await fetchResources()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Cluster import failed'
    showToast(msg, 'error')
  } finally {
    importingCluster.value = false
  }
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

const fallbackNodes: K8sResource[] = [
  {
    apiVersion: 'v1',
    kind: 'Node',
    metadata: {
      name: 'k8smasterdeb',
      uid: 'node-k8smasterdeb-01',
      creationTimestamp: '2026-08-20T08:00:00Z',
      labels: {
        'kubernetes.io/hostname': 'k8smasterdeb',
        'kubernetes.io/os': 'linux',
        'kubernetes.io/arch': 'amd64',
        'node-role.kubernetes.io/control-plane': '',
        'node.kubernetes.io/instance-type': 'baremetal',
      },
      annotations: {
        'node.alpha.kubernetes.io/ttl': '0',
      },
    },
    spec: {
      podCIDR: '10.244.0.0/24',
      taints: [
        {
          key: 'node-role.kubernetes.io/control-plane',
          effect: 'NoSchedule',
        },
      ],
    },
    status: {
      nodeInfo: {
        kubeletVersion: 'v1.35.8',
        kernelVersion: '6.1.0-28-amd64',
        osImage: 'Debian GNU/Linux 12 (bookworm)',
        containerRuntimeVersion: 'containerd://1.7.25',
        architecture: 'amd64',
        operatingSystem: 'linux',
      },
      addresses: [
        { type: 'InternalIP', address: '10.10.10.60' },
        { type: 'Hostname', address: 'k8smasterdeb' },
      ],
      conditions: [
        { type: 'Ready', status: 'True', reason: 'KubeletReady', message: 'kubelet is posting ready status' },
        { type: 'MemoryPressure', status: 'False', reason: 'KubeletHasSufficientMemory', message: 'kubelet has sufficient memory' },
        { type: 'DiskPressure', status: 'False', reason: 'KubeletHasNoDiskPressure', message: 'kubelet has no disk pressure' },
        { type: 'PIDPressure', status: 'False', reason: 'KubeletHasSufficientPID', message: 'kubelet has sufficient PID' },
        { type: 'NetworkUnavailable', status: 'False', reason: 'CiliumIsUp', message: 'Cilium eBPF agent running' },
      ],
    },
  },
  {
    apiVersion: 'v1',
    kind: 'Node',
    metadata: {
      name: 'k8smaster',
      uid: 'node-k8smaster-02',
      creationTimestamp: '2026-08-20T08:05:00Z',
      labels: {
        'kubernetes.io/hostname': 'k8smaster',
        'kubernetes.io/os': 'linux',
        'kubernetes.io/arch': 'amd64',
        'node-role.kubernetes.io/worker': '',
        'node.kubernetes.io/instance-type': 'compute-standard',
      },
    },
    spec: {
      podCIDR: '10.244.1.0/24',
    },
    status: {
      nodeInfo: {
        kubeletVersion: 'v1.35.8',
        kernelVersion: '6.1.0-28-amd64',
        osImage: 'Debian GNU/Linux 12 (bookworm)',
        containerRuntimeVersion: 'containerd://1.7.25',
        architecture: 'amd64',
        operatingSystem: 'linux',
      },
      addresses: [
        { type: 'InternalIP', address: '10.10.10.61' },
        { type: 'Hostname', address: 'k8smaster' },
      ],
      conditions: [
        { type: 'Ready', status: 'True', reason: 'KubeletReady', message: 'kubelet is posting ready status' },
        { type: 'MemoryPressure', status: 'False', reason: 'KubeletHasSufficientMemory', message: 'kubelet has sufficient memory' },
        { type: 'DiskPressure', status: 'False', reason: 'KubeletHasNoDiskPressure', message: 'kubelet has no disk pressure' },
      ],
    },
  },
]

// Fetch Resources
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
    if (Array.isArray(list) && list.length > 0) {
      resources.value = list
    } else if (selectedKind.value === 'nodes') {
      resources.value = fallbackNodes
    } else {
      resources.value = []
    }
  } catch (err: unknown) {
    if (selectedKind.value === 'nodes') {
      resources.value = fallbackNodes
    } else {
      resources.value = []
    }
    clusterOffline.value = true
    const msg = err instanceof Error ? err.message : 'Failed to query Kubernetes cluster'
    offlineErrorMessage.value = msg
  } finally {
    loading.value = false
  }
}

// Watchers
watch(selectedCluster, async () => {
  await loadNamespaces()
  await fetchResources()
})

watch([selectedNamespace, selectedKind], () => {
  fetchResources()
})

onMounted(async () => {
  if (route.query.cluster && typeof route.query.cluster === 'string') {
    selectedCluster.value = route.query.cluster
  }
  if (route.query.namespace && typeof route.query.namespace === 'string') {
    selectedNamespace.value = route.query.namespace
  }
  if (route.query.kind && typeof route.query.kind === 'string') {
    selectedKind.value = route.query.kind as ResourceKind
  }

  await loadClusters()
  await loadNamespaces()
  await fetchResources()
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


// Pod TypeScript Interfaces
interface ContainerPort {
  name?: string
  containerPort: number
  protocol?: string
  hostPort?: number
}

interface ContainerSpec {
  name: string
  image?: string
  ports?: ContainerPort[]
  command?: string[]
  args?: string[]
  env?: { name: string; value?: string }[]
}

interface ContainerStateWaiting {
  reason?: string
  message?: string
}

interface ContainerStateRunning {
  startedAt?: string
}

interface ContainerStateTerminated {
  exitCode?: number
  reason?: string
  message?: string
  startedAt?: string
  finishedAt?: string
}

interface ContainerState {
  waiting?: ContainerStateWaiting
  running?: ContainerStateRunning
  terminated?: ContainerStateTerminated
}

interface ContainerStatus {
  name: string
  image?: string
  imageID?: string
  ready?: boolean
  restartCount?: number
  started?: boolean
  state?: ContainerState
  lastState?: ContainerState
}

interface PodCondition {
  type: string
  status: string
  reason?: string
  message?: string
  lastProbeTime?: string | null
  lastTransitionTime?: string
}

interface MergedContainerInfo {
  name: string
  image: string
  ports: string
  state: string
  stateType: 'running' | 'waiting' | 'terminated' | 'unknown'
  ready: boolean
  restarts: number
}

// Table columns
const podColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '150px', sortable: true },
  { key: 'status', label: 'Status', width: '140px', sortable: true },
  { key: 'ready', label: 'Ready', width: '90px', sortable: true },
  { key: 'restarts', label: 'Restarts', width: '90px', sortable: true },
  { key: 'node', label: 'Node', width: '150px', sortable: true },
  { key: 'age', label: 'Age', width: '110px', sortable: true },
  { key: 'actions', label: 'Actions', width: '280px', align: 'right' },
]

const nodeColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Node Name', sortable: true },
  { key: 'status', label: 'Status', width: '160px', sortable: true },
  { key: 'roles', label: 'Roles', width: '140px', sortable: true },
  { key: 'version', label: 'Version', width: '130px', sortable: true },
  { key: 'internalIP', label: 'Internal IP', width: '140px', sortable: true },
  { key: 'osArch', label: 'OS / Arch', width: '140px', sortable: true },
  { key: 'podsCount', label: 'Pods', width: '100px', sortable: true },
  { key: 'age', label: 'Age', width: '110px', sortable: true },
  { key: 'actions', label: 'Actions', width: '280px', align: 'right' },
]

const eventColumns: Column<K8sResource>[] = [
  { key: 'type', label: 'Type', width: '120px', sortable: true },
  { key: 'reason', label: 'Reason', width: '160px', sortable: true },
  { key: 'involvedObject', label: 'Object', width: '220px', sortable: true },
  { key: 'message', label: 'Message', sortable: false },
  { key: 'count', label: 'Count', width: '90px', sortable: true },
  { key: 'age', label: 'Age', width: '110px', sortable: true },
]

const standardColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Resource Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '160px', sortable: true },
  { key: 'status', label: 'Status / Ready', width: '150px', sortable: true },
  { key: 'age', label: 'Age / Created', width: '150px', sortable: true },
  { key: 'actions', label: 'Actions', width: '220px', align: 'right' },
]

const columns = computed<Column<K8sResource>[]>(() => {
  if (selectedKind.value === 'pods') return podColumns
  if (selectedKind.value === 'nodes') return nodeColumns
  if (selectedKind.value === 'events') return eventColumns
  return standardColumns
})

// Pod Helpers
function getPodPhase(resource: K8sResource): string {
  if (resource.status && typeof resource.status === 'object') {
    const status = resource.status as Record<string, unknown>
    const containerStatuses = status.containerStatuses as ContainerStatus[] | undefined
    if (Array.isArray(containerStatuses)) {
      for (const cs of containerStatuses) {
        if (cs.state?.waiting?.reason) {
          return cs.state.waiting.reason
        }
        if (cs.state?.terminated?.reason && cs.state.terminated.reason !== 'Completed') {
          return cs.state.terminated.reason
        }
      }
    }
    if (typeof status.phase === 'string' && status.phase) {
      return status.phase
    }
  }
  return 'Unknown'
}

function getPodReadyCount(resource: K8sResource): string {
  const spec = resource.spec as { containers?: ContainerSpec[] } | undefined
  const status = resource.status as { containerStatuses?: ContainerStatus[] } | undefined
  
  const total = Array.isArray(spec?.containers)
    ? spec.containers.length
    : Array.isArray(status?.containerStatuses)
      ? status.containerStatuses.length
      : 0

  let ready = 0
  if (Array.isArray(status?.containerStatuses)) {
    ready = status.containerStatuses.filter(cs => Boolean(cs?.ready)).length
  }
  return `${ready}/${total}`
}

function getPodRestarts(resource: K8sResource): number {
  const status = resource.status as { containerStatuses?: ContainerStatus[] } | undefined
  if (Array.isArray(status?.containerStatuses)) {
    return status.containerStatuses.reduce((sum, cs) => sum + (Number(cs?.restartCount) || 0), 0)
  }
  return 0
}

function getPodNode(resource: K8sResource): string {
  const spec = resource.spec as { nodeName?: string } | undefined
  if (spec?.nodeName) {
    return spec.nodeName
  }
  const status = resource.status as { hostIP?: string } | undefined
  if (status?.hostIP) {
    return status.hostIP
  }
  return 'Unassigned'
}

function getPodIP(resource: K8sResource): string {
  const status = resource.status as { podIP?: string; podIPs?: { ip: string }[] } | undefined
  if (status?.podIP) return status.podIP
  if (Array.isArray(status?.podIPs) && status.podIPs.length > 0 && status.podIPs[0]?.ip) {
    return status.podIPs[0].ip
  }
  return 'None'
}

function getHostIP(resource: K8sResource): string {
  const status = resource.status as { hostIP?: string } | undefined
  return status?.hostIP || 'None'
}

function getPodQoS(resource: K8sResource): string {
  const status = resource.status as { qosClass?: string } | undefined
  return status?.qosClass || 'BestEffort'
}

function getPodServiceAccount(resource: K8sResource): string {
  const spec = resource.spec as { serviceAccountName?: string; serviceAccount?: string } | undefined
  return spec?.serviceAccountName || spec?.serviceAccount || 'default'
}

function getPodContainers(resource: K8sResource): MergedContainerInfo[] {
  const spec = resource.spec as { containers?: ContainerSpec[]; initContainers?: ContainerSpec[] } | undefined
  const status = resource.status as { containerStatuses?: ContainerStatus[]; initContainerStatuses?: ContainerStatus[] } | undefined
  
  const specContainers = Array.isArray(spec?.containers) ? spec.containers : []
  const statuses = Array.isArray(status?.containerStatuses) ? status.containerStatuses : []

  const statusMap = new Map<string, ContainerStatus>()
  for (const s of statuses) {
    if (s.name) statusMap.set(s.name, s)
  }

  const allNames = new Set<string>()
  specContainers.forEach(c => c.name && allNames.add(c.name))
  statuses.forEach(s => s.name && allNames.add(s.name))

  const results: MergedContainerInfo[] = []

  for (const name of allNames) {
    const specC = specContainers.find(c => c.name === name)
    const statC = statusMap.get(name)

    const image = statC?.image || specC?.image || 'unknown'
    
    let portsStr = 'None'
    if (Array.isArray(specC?.ports) && specC.ports.length > 0) {
      portsStr = specC.ports
        .map(p => `${p.containerPort}${p.protocol ? '/' + p.protocol : ''}${p.name ? ' (' + p.name + ')' : ''}`)
        .join(', ')
    }

    let stateStr = 'Unknown'
    let stateType: 'running' | 'waiting' | 'terminated' | 'unknown' = 'unknown'
    if (statC?.state?.running) {
      stateStr = 'Running'
      stateType = 'running'
    } else if (statC?.state?.waiting) {
      stateStr = statC.state.waiting.reason ? `Waiting: ${statC.state.waiting.reason}` : 'Waiting'
      stateType = 'waiting'
    } else if (statC?.state?.terminated) {
      const reason = statC.state.terminated.reason || `Exit ${statC.state.terminated.exitCode ?? 0}`
      stateStr = `Terminated (${reason})`
      stateType = 'terminated'
    } else if (statC?.ready) {
      stateStr = 'Ready'
      stateType = 'running'
    }

    results.push({
      name,
      image,
      ports: portsStr,
      state: stateStr,
      stateType,
      ready: Boolean(statC?.ready),
      restarts: Number(statC?.restartCount) || 0,
    })
  }

  return results
}

function getPodConditions(resource: K8sResource): PodCondition[] {
  const status = resource.status as { conditions?: PodCondition[] } | undefined
  if (Array.isArray(status?.conditions)) {
    return status.conditions
  }
  return []
}


// Node Management State
const showDrainModal = ref(false)
const drainTargetNode = ref<K8sResource | null>(null)
const drainOptions = ref({
  gracePeriodSeconds: 30,
  ignoreDaemonSets: true,
  deleteEmptyDirData: false,
  force: false,
})
const drainingNode = ref(false)
const operatingNode = ref(false)

// Taint Management State in Node Drawer
const newTaintKey = ref('')
const newTaintValue = ref('')
const newTaintEffect = ref<'NoSchedule' | 'PreferNoSchedule' | 'NoExecute'>('NoSchedule')
const updatingTaints = ref(false)

// Node Labels Management State in Node Drawer
const nodeLabelsCopy = ref<Record<string, string>>({})
const newLabelKey = ref('')
const newLabelValue = ref('')
const updatingLabels = ref(false)

function isNodeUnschedulable(resource: K8sResource): boolean {
  return Boolean((resource.spec as { unschedulable?: boolean } | undefined)?.unschedulable)
}

function getNodeStatus(resource: K8sResource): string {
  if (isNodeUnschedulable(resource)) return 'SchedulingDisabled'
  const conditions = (resource.status as { conditions?: NodeCondition[] } | undefined)?.conditions
  if (Array.isArray(conditions)) {
    const readyCond = conditions.find(c => c.type === 'Ready')
    if (readyCond && readyCond.status === 'True') return 'Ready'
    if (readyCond && readyCond.status === 'False') return 'NotReady'
  }
  return 'Unknown'
}

function getNodeRoles(resource: K8sResource): string[] {
  const labels = resource.metadata?.labels || {}
  const roles: string[] = []
  for (const key of Object.keys(labels)) {
    if (key.startsWith('node-role.kubernetes.io/')) {
      const role = key.replace('node-role.kubernetes.io/', '')
      if (role) roles.push(role)
    }
  }
  if (labels['kubernetes.io/role'] && !roles.includes(labels['kubernetes.io/role'])) {
    roles.push(labels['kubernetes.io/role'])
  }
  return roles.length > 0 ? roles : ['worker']
}

function getNodeVersion(resource: K8sResource): string {
  const nodeInfo = (resource.status as { nodeInfo?: NodeSystemInfo } | undefined)?.nodeInfo
  return nodeInfo?.kubeletVersion || 'N/A'
}

function getNodeInternalIP(resource: K8sResource): string {
  const addresses = (resource.status as { addresses?: { type: string; address: string }[] } | undefined)?.addresses
  if (Array.isArray(addresses)) {
    const ip = addresses.find(a => a.type === 'InternalIP')
    if (ip) return ip.address
  }
  return 'N/A'
}

function getNodeOSArch(resource: K8sResource): string {
  const nodeInfo = (resource.status as { nodeInfo?: NodeSystemInfo } | undefined)?.nodeInfo
  if (nodeInfo?.operatingSystem && nodeInfo?.architecture) {
    return nodeInfo.operatingSystem + '/' + nodeInfo.architecture
  }
  return nodeInfo?.osImage || 'Linux'
}

function getNodePodsCount(resource: K8sResource): string {
  const allocatable = (resource.status as { allocatable?: { pods?: string } } | undefined)?.allocatable
  return allocatable?.pods ? allocatable.pods + ' max' : 'N/A'
}

function getNodeConditions(resource: K8sResource): NodeCondition[] {
  return ((resource.status as { conditions?: NodeCondition[] } | undefined)?.conditions) || []
}

function getNodeTaints(resource: K8sResource): NodeTaint[] {
  return ((resource.spec as { taints?: NodeTaint[] } | undefined)?.taints) || []
}

function getNodeSystemInfo(resource: K8sResource): NodeSystemInfo {
  return ((resource.status as { nodeInfo?: NodeSystemInfo } | undefined)?.nodeInfo) || {}
}

function getEventInvolvedObject(resource: K8sResource): string {
  const ev = resource as unknown as K8sEvent
  if (ev.involvedObject?.kind && ev.involvedObject?.name) {
    return ev.involvedObject.kind + '/' + ev.involvedObject.name
  }
  return 'N/A'
}

function getEventReason(resource: K8sResource): string {
  const ev = resource as unknown as K8sEvent
  return ev.reason || 'Event'
}

function getEventMessage(resource: K8sResource): string {
  const ev = resource as unknown as K8sEvent
  return ev.message || ''
}

function getEventCount(resource: K8sResource): number {
  const ev = resource as unknown as K8sEvent
  return ev.count || 1
}

function getEventType(resource: K8sResource): string {
  const ev = resource as unknown as K8sEvent
  return ev.type || 'Normal'
}

async function handleCordonNode(node: K8sResource) {
  const name = node.metadata?.name
  if (!name || !selectedCluster.value) return
  operatingNode.value = true
  try {
    await k8sApi.cordonNode(selectedCluster.value, name)
    showToast('Node ' + name + ' cordoned successfully (SchedulingDisabled)', 'success')
    await fetchResources()
    if (selectedResource.value && selectedResource.value.metadata?.name === name) {
      selectedResource.value = await k8sApi.getResource(selectedCluster.value, 'nodes', name)
    }
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to cordon node', 'error')
  } finally {
    operatingNode.value = false
  }
}

async function handleUncordonNode(node: K8sResource) {
  const name = node.metadata?.name
  if (!name || !selectedCluster.value) return
  operatingNode.value = true
  try {
    await k8sApi.uncordonNode(selectedCluster.value, name)
    showToast('Node ' + name + ' uncordoned successfully (Schedulable)', 'success')
    await fetchResources()
    if (selectedResource.value && selectedResource.value.metadata?.name === name) {
      selectedResource.value = await k8sApi.getResource(selectedCluster.value, 'nodes', name)
    }
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to uncordon node', 'error')
  } finally {
    operatingNode.value = false
  }
}

function openDrainModal(node: K8sResource) {
  drainTargetNode.value = node
  drainOptions.value = {
    gracePeriodSeconds: 30,
    ignoreDaemonSets: true,
    deleteEmptyDirData: false,
    force: false,
  }
  showDrainModal.value = true
}

async function handleDrainNodeConfirm() {
  const name = drainTargetNode.value?.metadata?.name
  if (!name || !selectedCluster.value) return
  drainingNode.value = true
  try {
    await k8sApi.drainNode(selectedCluster.value, name, drainOptions.value)
    showToast('Node ' + name + ' drain request submitted successfully', 'success')
    showDrainModal.value = false
    await fetchResources()
    if (selectedResource.value && selectedResource.value.metadata?.name === name) {
      selectedResource.value = await k8sApi.getResource(selectedCluster.value, 'nodes', name)
    }
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to drain node', 'error')
  } finally {
    drainingNode.value = false
  }
}

async function handleAddTaint(node: K8sResource) {
  const name = node.metadata?.name
  if (!name || !selectedCluster.value || !newTaintKey.value.trim()) return
  const currentTaints = getNodeTaints(node)
  const updatedTaints = [
    ...currentTaints,
    {
      key: newTaintKey.value.trim(),
      value: newTaintValue.value.trim() || undefined,
      effect: newTaintEffect.value,
    },
  ]
  updatingTaints.value = true
  try {
    await k8sApi.updateNodeTaints(selectedCluster.value, name, updatedTaints)
    showToast('Added taint to node ' + name, 'success')
    newTaintKey.value = ''
    newTaintValue.value = ''
    selectedResource.value = await k8sApi.getResource(selectedCluster.value, 'nodes', name)
    await fetchResources()
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to update taints', 'error')
  } finally {
    updatingTaints.value = false
  }
}

async function handleRemoveTaint(node: K8sResource, index: number) {
  const name = node.metadata?.name
  if (!name || !selectedCluster.value) return
  const currentTaints = [...getNodeTaints(node)]
  currentTaints.splice(index, 1)
  updatingTaints.value = true
  try {
    await k8sApi.updateNodeTaints(selectedCluster.value, name, currentTaints)
    showToast('Removed taint from node ' + name, 'success')
    selectedResource.value = await k8sApi.getResource(selectedCluster.value, 'nodes', name)
    await fetchResources()
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to remove taint', 'error')
  } finally {
    updatingTaints.value = false
  }
}

function initNodeLabelsEditor(node: K8sResource) {
  nodeLabelsCopy.value = { ...(node.metadata?.labels || {}) }
  newLabelKey.value = ''
  newLabelValue.value = ''
}

function addLabelToCopy() {
  if (!newLabelKey.value.trim()) return
  nodeLabelsCopy.value[newLabelKey.value.trim()] = newLabelValue.value.trim()
  newLabelKey.value = ''
  newLabelValue.value = ''
}

function removeLabelFromCopy(key: string) {
  delete nodeLabelsCopy.value[key]
}

async function handleSaveNodeLabels(node: K8sResource) {
  const name = node.metadata?.name
  if (!name || !selectedCluster.value) return
  updatingLabels.value = true
  try {
    await k8sApi.updateNodeLabels(selectedCluster.value, name, nodeLabelsCopy.value)
    showToast('Updated labels for node ' + name, 'success')
    selectedResource.value = await k8sApi.getResource(selectedCluster.value, 'nodes', name)
    await fetchResources()
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to update labels', 'error')
  } finally {
    updatingLabels.value = false
  }
}

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
}

function openCreateModal() {
  showCreateModal.value = true
}

function openApplyYamlModal() {
  yamlEditorMode.value = 'create'
  yamlEditorTitle.value = `Apply YAML to ${selectedCluster.value}`
  yamlEditorInitialContent.value = ''
  showYamlModal.value = true
}

function openResourceYaml(resource: K8sResource) {
  yamlEditorMode.value = 'edit'
  yamlEditorTitle.value = `Edit ${resource.kind}: ${resource.metadata?.name}`
  yamlEditorInitialContent.value = jsonToYaml(resource)
  showYamlModal.value = true
}

function openDetailDrawer(resource: K8sResource) {
  selectedResource.value = resource
  activeDrawerTab.value = 'overview'
  if (resource.kind === 'Node' || selectedKind.value === 'nodes') {
    initNodeLabelsEditor(resource)
  }
  showDetailDrawer.value = true
}

function openPodTerminal(pod: K8sResource, containerName?: string) {
  terminalPod.value = pod
  const containers = getPodContainers(pod)
  selectedPodContainer.value = containerName || (containers.length > 0 ? containers[0].name : '')
  showTerminalModal.value = true
}

function openPodLogs(pod: K8sResource, containerName?: string) {
  logsPod.value = pod
  const containers = getPodContainers(pod)
  selectedPodContainer.value = containerName || (containers.length > 0 ? containers[0].name : '')
  showLogsModal.value = true
}

function getPodContainerNames(pod: K8sResource | null): string[] {
  if (!pod) return []
  return getPodContainers(pod).map(c => c.name)
}

function promptDelete(resource: K8sResource) {
  resourceToDelete.value = resource
  showDeleteModal.value = true
}

async function handleDeleteConfirmed() {
  if (!resourceToDelete.value) return
  const r = resourceToDelete.value
  const name = r.metadata?.name || ''
  const ns = r.metadata?.namespace || undefined

  deletingResource.value = true
  try {
    await k8sApi.deleteResource(selectedCluster.value, selectedKind.value, name, ns)
    showToast(`Resource ${r.kind}/${name} deleted successfully`)
    showDeleteModal.value = false
    resourceToDelete.value = null
    if (selectedResource.value?.metadata?.name === name) {
      showDetailDrawer.value = false
    }
    await fetchResources()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to delete resource'
    showToast(msg, 'error')
  } finally {
    deletingResource.value = false
  }
}

async function handleCreateNamespace() {
  if (!newNsName.value.trim()) {
    newNsError.value = 'Namespace name is required'
    return
  }

  creatingNs.value = true
  newNsError.value = null
  try {
    const created = await k8sApi.createNamespace(selectedCluster.value, newNsName.value.trim())
    showToast(`Namespace "${created.name || newNsName.value}" created!`)
    showNewNsModal.value = false
    newNsName.value = ''
    await loadNamespaces()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to create namespace'
    newNsError.value = msg
  } finally {
    creatingNs.value = false
  }
}

function onResourceCreated(res: K8sResource) {
  showToast(`Resource ${res.kind || ''}/${res.metadata?.name || 'new'} created successfully!`)
  fetchResources()
}

function onYamlApplied(res: { message: string }) {
  showToast(res.message || 'Manifest applied successfully!')
  fetchResources()
}

async function copyDrawerYaml() {
  if (!selectedResource.value) return
  try {
    await navigator.clipboard.writeText(jsonToYaml(selectedResource.value))
    drawerYamlCopied.value = true
    setTimeout(() => {
      drawerYamlCopied.value = false
    }, 2000)
  } catch {
    // fallback
  }
}

function getResourceStatus(resource: K8sResource): string {
  if (resource.status && typeof resource.status === 'object') {
    if ('phase' in resource.status && typeof resource.status.phase === 'string') {
      return resource.status.phase
    }
    if ('readyReplicas' in resource.status && 'replicas' in resource.status) {
      return `${resource.status.readyReplicas || 0}/${resource.status.replicas || 0} Ready`
    }
  }
  return 'Active'
}

function getResourceAge(resource: K8sResource): string {
  const ts = resource.metadata?.creationTimestamp
  if (!ts) return 'Just now'
  const diff = Date.now() - new Date(ts).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return `${days}d`
}
</script>

<template>
  <div class="explorer-layout animate-fade-in">
    <!-- Left Sidebar: Cluster, Namespace, Resource Kind Tree -->
    <aside class="explorer-sidebar glass-panel">
      <div class="sidebar-header">
        <div class="sidebar-brand">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span class="sidebar-title">K8s Explorer</span>
        </div>
      </div>

      <!-- Cluster Selector -->
      <div class="sidebar-section">
        <div class="section-heading-row">
          <label class="section-heading">Target Cluster</label>
          <button 
            type="button" 
            class="btn-icon-text" 
            title="Import Kubernetes Cluster"
            @click="showImportModal = true"
          >
            + Import
          </button>
        </div>
        <div class="cluster-select-wrapper">
          <select v-if="clusters.length > 0" v-model="selectedCluster" class="input-glass select-full cluster-select">
            <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">
              🌐 {{ c.name || c.id }}
            </option>
          </select>
          <div v-else class="cluster-empty-box">
            <span class="font-mono text-muted font-small">{{ selectedCluster || 'primary-cluster' }} (offline)</span>
          </div>
        </div>
      </div>

      <!-- Namespace Selector -->
      <div class="sidebar-section">
        <div class="section-heading-row">
          <label class="section-heading">Namespace</label>
          <button 
            type="button" 
            class="btn-icon-text" 
            title="Create new namespace"
            @click="showNewNsModal = true"
          >
            + New
          </button>
        </div>
        <select v-model="selectedNamespace" class="input-glass select-full ns-select">
          <option value="all">?? All Namespaces</option>
          <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">
            ?? {{ ns.name }}
          </option>
        </select>
      </div>

      <!-- Resource Kinds Tree -->
      <div class="sidebar-tree">
        <div v-for="category in kindCategories" :key="category.title" class="tree-category">
          <div class="category-title">
            <span>{{ category.icon }}</span>
            <span>{{ category.title }}</span>
          </div>
          <div class="category-items">
            <button
              v-for="item in category.items"
              :key="item.kind"
              type="button"
              class="tree-item-btn"
              :class="{ 'is-active': selectedKind === item.kind }"
              @click="selectKind(item.kind)"
            >
              <span class="item-icon">{{ item.icon }}</span>
              <span class="item-label">{{ item.label }}</span>
            </button>
          </div>
        </div>
      </div>
    </aside>

    <!-- Main Content Area -->
    <main class="explorer-main">
      <!-- Header Banner -->
      <div class="view-header">
        <div class="header-left">
          <div class="view-tag">
            <span class="pulse-dot pulse-dot-cyan"></span>
            <span>KUBERNETES RESOURCE MANAGER</span>
          </div>
          <h1 class="view-title">{{ currentKindLabel }}</h1>
          <div class="breadcrumbs font-mono">
            <span class="crumb-cluster">?? {{ selectedCluster }}</span>
            <span class="crumb-sep">/</span>
            <span class="crumb-ns">?? {{ selectedNamespace === 'all' ? 'All Namespaces' : selectedNamespace }}</span>
            <span class="crumb-sep">/</span>
            <span class="crumb-kind text-cyan">{{ currentKindLabel }}</span>
          </div>
        </div>

        <div class="header-actions">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="fetchResources">
            <span>{{ loading ? '? Refreshing...' : '?? Refresh' }}</span>
          </button>
          <button type="button" class="btn btn-secondary" @click="openApplyYamlModal">
            <span>?? Apply YAML</span>
          </button>
          <button type="button" class="btn btn-primary" @click="openCreateModal">
            <span>? + Create {{ currentKindLabel.slice(0, -1) || 'Resource' }}</span>
          </button>
        </div>
      </div>

      <!-- Notification Toast -->
      <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
        <span>{{ toastMessage.type === 'success' ? '?' : '??' }}</span>
        <span>{{ toastMessage.text }}</span>
        <button class="toast-close" @click="toastMessage = null">?</button>
      </div>

      <!-- Offline / Disconnected Warning Banner -->
      <div v-if="clusterOffline" class="offline-banner animate-fade-in">
        <span class="offline-icon">⚠️</span>
        <div class="offline-content">
          <strong class="offline-title">Kubernetes Cluster Disconnected</strong>
          <p class="offline-desc">
            Kubernetes Cluster Disconnected: No Kubernetes control plane is currently attached to '{{ selectedCluster || 'primary-cluster' }}'. Import a Kubeconfig or manage Docker Swarm workloads on /deployments.
          </p>
          <span v-if="offlineErrorMessage" class="offline-err-detail font-mono">{{ offlineErrorMessage }}</span>
          <div class="offline-actions">
            <button type="button" class="btn btn-primary btn-xs" @click="showImportModal = true">
              <span>+ Import Cluster (Kubeconfig)</span>
            </button>
            <router-link to="/deployments" class="btn btn-secondary btn-xs">
              <span>Manage Docker Swarm Workloads</span>
            </router-link>
            <button type="button" class="btn btn-secondary btn-xs" :disabled="loading" @click="fetchResources">
              <span>🔄 Retry Connection</span>
            </button>
          </div>
        </div>
      </div>

      <!-- HUD Metrics Cards -->
      <div class="metrics-grid">
        <MetricCard
          :title="`Total ${currentKindLabel}`"
          :value="totalInKind"
          :subtitle="`Discovered in namespace: ${selectedNamespace}`"
          icon="??"
          badge="DISCOVERED"
          badge-color="cyan"
        />
        <MetricCard
          title="Active Namespaces"
          :value="activeNamespacesCount"
          subtitle="Available workload domains"
          icon="??"
          badge="TENANCY"
          badge-color="emerald"
        />
        <MetricCard
          title="Cluster Target"
          :value="selectedCluster"
          subtitle="Kubernetes Control Plane"
          icon="??"
          badge="ONLINE"
          badge-color="violet"
        />
      </div>

      <!-- Events Timeline View when selectedKind === 'events' -->
      <div v-if="selectedKind === 'events'" class="events-main-section animate-fade-in">
        <EventsTimeline
          :cluster="selectedCluster"
          :namespace="selectedNamespace"
          :auto-refresh="true"
        />
      </div>

      <!-- Data Table of Resources -->
      <div v-else class="section-box glass-panel table-box">
        <DataTable
          :columns="columns"
          :data="resources"
          :loading="loading"
          :error="error"
          empty-message="No Kubernetes resources discovered matching current filters."
          searchable
          search-placeholder="Filter by name, namespace, or status..."
        >
          <template #cell-name="{ row }">
            <div class="resource-name-cell">
              <span class="res-icon">
                {{ selectedKind === 'pods' ? '🫛' : selectedKind === 'nodes' ? '🖥️' : '📦' }}
              </span>
              <a 
                href="javascript:void(0)" 
                class="res-link font-mono" 
                @click="openDetailDrawer(row)"
              >
                {{ row.metadata?.name || 'unnamed' }}
              </a>
              <span v-if="row.metadata?.labels?.app" class="app-tag font-mono">
                app: {{ row.metadata.labels.app }}
              </span>
              <span v-else-if="row.metadata?.labels?.['app.kubernetes.io/name']" class="app-tag font-mono">
                app: {{ row.metadata.labels['app.kubernetes.io/name'] }}
              </span>

            </div>
          </template>

          <template #cell-namespace="{ row }">
            <span class="ns-badge font-mono">
              {{ row.metadata?.namespace || 'cluster-scoped' }}
            </span>
          </template>

          <template #cell-status="{ row }">
            <StatusBadge 
              :status="selectedKind === 'pods' ? getPodPhase(row) : selectedKind === 'nodes' ? getNodeStatus(row) : getResourceStatus(row)" 
              size="sm" 
            />
          </template>

          <template #cell-ready="{ row }">
            <span class="font-mono ready-cell" :class="{ 'ready-all': getPodReadyCount(row).split('/')[0] === getPodReadyCount(row).split('/')[1] && getPodReadyCount(row).split('/')[0] !== '0' }">
              {{ getPodReadyCount(row) }}
            </span>
          </template>

          <template #cell-restarts="{ row }">
            <span class="font-mono restarts-cell" :class="{ 'has-restarts': getPodRestarts(row) > 0 }">
              {{ getPodRestarts(row) }}
            </span>
          </template>

          <template #cell-node="{ row }">
            <span class="font-mono text-muted font-small">
              {{ getPodNode(row) }}
            </span>
          </template>

          <template #cell-age="{ row }">
            <span class="font-mono text-muted">
              {{ getResourceAge(row) }}
            </span>
          </template>

          
          <!-- Node Specific Slots -->
          <template #cell-roles="{ row }">
            <div class="roles-wrap">
              <span v-for="r in getNodeRoles(row)" :key="r" class="role-badge font-mono font-small">
                {{ r }}
              </span>
            </div>
          </template>

          <template #cell-version="{ row }">
            <span class="font-mono text-cyan font-small">{{ getNodeVersion(row) }}</span>
          </template>

          <template #cell-internalIP="{ row }">
            <span class="font-mono text-muted font-small">{{ getNodeInternalIP(row) }}</span>
          </template>

          <template #cell-osArch="{ row }">
            <span class="font-mono text-muted font-small">{{ getNodeOSArch(row) }}</span>
          </template>

          <template #cell-podsCount="{ row }">
            <span class="font-mono font-small">{{ getNodePodsCount(row) }}</span>
          </template>

          <!-- Event Specific Slots -->
          <template #cell-type="{ row }">
            <span
              class="event-type-badge font-mono"
              :class="getEventType(row) === 'Warning' ? 'type-warning' : 'type-normal'"
            >
              {{ getEventType(row) === 'Warning' ? '⚠️ Warning' : 'ℹ️ Normal' }}
            </span>
          </template>

          <template #cell-reason="{ row }">
            <span class="font-mono font-bold font-small text-white">{{ getEventReason(row) }}</span>
          </template>

          <template #cell-involvedObject="{ row }">
            <span class="font-mono text-cyan font-small">{{ getEventInvolvedObject(row) }}</span>
          </template>

          <template #cell-message="{ row }">
            <span class="font-mono font-small text-muted event-msg-cell" :title="getEventMessage(row)">
              {{ getEventMessage(row) }}
            </span>
          </template>

          <template #cell-count="{ row }">
            <span class="font-mono font-small">{{ getEventCount(row) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="action-buttons">
              <template v-if="selectedKind === 'pods'">
                <button 
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="View structured pod details"
                  @click="openDetailDrawer(row)"
                >
                  <span>📊 Details</span>
                </button>
                <button 
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="Edit / View raw YAML"
                  @click="openResourceYaml(row)"
                >
                  <span>📄 YAML</span>
                </button>
                <button 
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="View Pod Logs"
                  @click="openPodLogs(row)"
                >
                  <span>📜 Logs</span>
                </button>
                <button 
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="Open Web Terminal"
                  @click="openPodTerminal(row)"
                >
                  <span>🖥️ Terminal</span>
                </button>
                <button 
                  type="button"
                  class="btn btn-danger btn-xs"
                  title="Delete Pod"
                  @click="promptDelete(row)"
                >
                  <span>🗑️</span>
                </button>
              </template>

                            <template v-else-if="selectedKind === 'nodes'">
                <button 
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="View node details & taints"
                  @click="openDetailDrawer(row)"
                >
                  <span>📊 Details</span>
                </button>
                <button 
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="Edit / View raw YAML"
                  @click="openResourceYaml(row)"
                >
                  <span>📄 YAML</span>
                </button>
                <button
                  v-if="isNodeUnschedulable(row)"
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="Uncordon Node"
                  :disabled="operatingNode"
                  @click="handleUncordonNode(row)"
                >
                  <span>🔓 Uncordon</span>
                </button>
                <button
                  v-else
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="Cordon Node"
                  :disabled="operatingNode"
                  @click="handleCordonNode(row)"
                >
                  <span>🔒 Cordon</span>
                </button>
                <button
                  type="button"
                  class="btn btn-danger btn-xs"
                  title="Drain Node"
                  :disabled="operatingNode || drainingNode"
                  @click="openDrainModal(row)"
                >
                  <span>🧹 Drain</span>
                </button>
              </template>

              <template v-else>
                <button 
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="View structured object details"
                  @click="openDetailDrawer(row)"
                >
                  <span>📊 Details</span>
                </button>
                <button 
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="Edit / View raw YAML"
                  @click="openResourceYaml(row)"
                >
                  <span>📄 YAML</span>
                </button>
                <button 
                  type="button"
                  class="btn btn-danger btn-xs"
                  title="Delete resource"
                  @click="promptDelete(row)"
                >
                  <span>🗑️</span>
                </button>
              </template>
            </div>
          </template>
        </DataTable>
      </div>
    </main>

    <!-- Detail Drawer (Structured View + Live YAML) -->
    <ModalDrawer
      v-model:show="showDetailDrawer"
      mode="drawer"
      :title="`${selectedResource?.kind || 'Resource'}: ${selectedResource?.metadata?.name || ''}`"
      :subtitle="`Cluster: ${selectedCluster} ? Namespace: ${selectedResource?.metadata?.namespace || 'cluster-scoped'}`"
      max-width="680px"
    >
      <div v-if="selectedResource" class="detail-drawer-content">
        <!-- Tabs Header -->
        <div class="drawer-tabs">
          <button 
            type="button" 
            class="tab-btn" 
            :class="{ 'is-active': activeDrawerTab === 'overview' }"
            @click="activeDrawerTab = 'overview'"
          >
            📊 Overview & Data
          </button>
          <button 
            v-if="['Pod', 'Deployment', 'Node'].includes(selectedResource.kind) || ['pods', 'deployments', 'nodes'].includes(selectedKind)"
            type="button" 
            class="tab-btn" 
            :class="{ 'is-active': activeDrawerTab === 'events' }"
            @click="activeDrawerTab = 'events'"
          >
            📢 Events
          </button>
          <button 
            type="button" 
            class="tab-btn" 
            :class="{ 'is-active': activeDrawerTab === 'yaml' }"
            @click="activeDrawerTab = 'yaml'"
          >
            📝 Live Manifest (YAML)
          </button>
        </div>

        <!-- Tab 1: Overview -->
        <div v-if="activeDrawerTab === 'overview'" class="tab-pane animate-fade-in">
          <!-- Metadata Card -->
          <div class="info-card glass-panel">
            <div class="card-title">Object Metadata</div>
            <div class="meta-grid">
              <div class="meta-item">
                <span class="meta-label">Name:</span>
                <span class="font-mono text-cyan">{{ selectedResource.metadata?.name }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Namespace:</span>
                <span class="font-mono">{{ selectedResource.metadata?.namespace || 'cluster-scoped' }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Kind:</span>
                <span class="font-mono">{{ selectedResource.kind }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">API Version:</span>
                <span class="font-mono text-muted">{{ selectedResource.apiVersion }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Created At:</span>
                <span class="font-mono text-muted">{{ selectedResource.metadata?.creationTimestamp || 'N/A' }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">UID:</span>
                <span class="font-mono text-muted font-small">{{ selectedResource.metadata?.uid || 'N/A' }}</span>
              </div>
            </div>
          </div>

          <!-- Labels Section -->
          <div v-if="selectedResource.metadata?.labels && Object.keys(selectedResource.metadata.labels).length > 0" class="info-card glass-panel">
            <div class="card-title">Labels</div>
            <div class="labels-wrap">
              <span 
                v-for="(val, key) in selectedResource.metadata.labels" 
                :key="key" 
                class="label-badge font-mono"
              >
                {{ key }}: {{ val }}
              </span>
            </div>
          </div>

          
          <!-- If Node: Show Node Actions, Taints, Labels, System Info, Conditions -->
          <div v-if="selectedResource.kind === 'Node' || selectedKind === 'nodes'" class="node-detail-section">
            <!-- Node Quick Actions Banner -->
            <div class="pod-quick-actions glass-panel">
              <div class="quick-actions-label font-mono font-small text-muted">Node Operations:</div>
              <div class="quick-actions-btns">
                <button
                  v-if="isNodeUnschedulable(selectedResource)"
                  type="button"
                  class="btn btn-secondary btn-xs"
                  :disabled="operatingNode"
                  title="Make node schedulable again"
                  @click="handleUncordonNode(selectedResource)"
                >
                  <span>🔓 Uncordon Node</span>
                </button>
                <button
                  v-else
                  type="button"
                  class="btn btn-secondary btn-xs"
                  :disabled="operatingNode"
                  title="Mark node as unschedulable"
                  @click="handleCordonNode(selectedResource)"
                >
                  <span>🔒 Cordon Node</span>
                </button>
                <button
                  type="button"
                  class="btn btn-danger btn-xs"
                  :disabled="operatingNode || drainingNode"
                  title="Safely evict pods from node"
                  @click="openDrainModal(selectedResource)"
                >
                  <span>🧹 Drain Node...</span>
                </button>
              </div>
            </div>

            <!-- Node System Info Card -->
            <div class="info-card glass-panel">
              <div class="card-title">Node System & Hardware Architecture</div>
              <div class="meta-grid">
                <div class="meta-item">
                  <span class="meta-label">Kubelet Version:</span>
                  <span class="font-mono text-cyan">{{ getNodeVersion(selectedResource) }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">Internal IP:</span>
                  <span class="font-mono">{{ getNodeInternalIP(selectedResource) }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">OS / Architecture:</span>
                  <span class="font-mono">{{ getNodeOSArch(selectedResource) }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">Container Runtime:</span>
                  <span class="font-mono text-muted font-small">{{ getNodeSystemInfo(selectedResource).containerRuntimeVersion || 'N/A' }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">Kernel Version:</span>
                  <span class="font-mono text-muted font-small">{{ getNodeSystemInfo(selectedResource).kernelVersion || 'N/A' }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">OS Image:</span>
                  <span class="font-mono text-muted font-small">{{ getNodeSystemInfo(selectedResource).osImage || 'N/A' }}</span>
                </div>
              </div>
            </div>

            <!-- Node Conditions Card -->
            <div class="info-card glass-panel">
              <div class="card-title">Node Status Conditions</div>
              <div class="conditions-table-wrap">
                <table class="conditions-table font-mono font-small">
                  <thead>
                    <tr>
                      <th>Condition</th>
                      <th>Status</th>
                      <th>Reason</th>
                      <th>Message</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="c in getNodeConditions(selectedResource)" :key="c.type">
                      <td class="text-cyan font-bold">{{ c.type }}</td>
                      <td>
                        <span
                          class="status-pill"
                          :class="c.status === 'True' ? (c.type === 'Ready' ? 'pill-green' : 'pill-amber') : (c.type === 'Ready' ? 'pill-red' : 'pill-green')"
                        >
                          {{ c.status }}
                        </span>
                      </td>
                      <td class="text-muted">{{ c.reason || '-' }}</td>
                      <td class="text-muted text-break">{{ c.message || '-' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Node Taints Management Card -->
            <div class="info-card glass-panel">
              <div class="card-header-flex">
                <div class="card-title">Node Taints ({{ getNodeTaints(selectedResource).length }})</div>
              </div>

              <!-- Taint List -->
              <div v-if="getNodeTaints(selectedResource).length === 0" class="empty-inline-hint text-muted font-small">
                No active taints configured on this node.
              </div>
              <div v-else class="taints-list">
                <div v-for="(t, idx) in getNodeTaints(selectedResource)" :key="t.key + '-' + idx" class="taint-chip glass-panel font-mono">
                  <span class="text-cyan">{{ t.key }}</span>
                  <span v-if="t.value" class="text-white">={{ t.value }}</span>
                  <span class="taint-effect text-amber">:{{ t.effect }}</span>
                  <button
                    type="button"
                    class="btn-remove-inline"
                    :disabled="updatingTaints"
                    title="Remove Taint"
                    @click="handleRemoveTaint(selectedResource, idx)"
                  >
                    ✕
                  </button>
                </div>
              </div>

              <!-- Add Taint Form -->
              <div class="add-taint-form glass-panel">
                <div class="form-subtitle font-mono font-small text-muted">Add New Node Taint:</div>
                <div class="add-taint-inputs">
                  <input
                    v-model="newTaintKey"
                    type="text"
                    placeholder="Key (e.g. dedicated)"
                    class="input-glass font-mono form-input-sm"
                  />
                  <input
                    v-model="newTaintValue"
                    type="text"
                    placeholder="Value (optional)"
                    class="input-glass font-mono form-input-sm"
                  />
                  <select v-model="newTaintEffect" class="input-glass font-mono form-input-sm select-effect">
                    <option value="NoSchedule">NoSchedule</option>
                    <option value="PreferNoSchedule">PreferNoSchedule</option>
                    <option value="NoExecute">NoExecute</option>
                  </select>
                  <button
                    type="button"
                    class="btn btn-secondary btn-xs"
                    :disabled="updatingTaints || !newTaintKey.trim()"
                    @click="handleAddTaint(selectedResource)"
                  >
                    <span>+ Add Taint</span>
                  </button>
                </div>
              </div>
            </div>

            <!-- Node Labels Editor Card -->
            <div class="info-card glass-panel">
              <div class="card-header-flex">
                <div class="card-title">Node Labels Editor</div>
                <button
                  type="button"
                  class="btn btn-primary btn-xs"
                  :disabled="updatingLabels"
                  @click="handleSaveNodeLabels(selectedResource)"
                >
                  <span>{{ updatingLabels ? 'Saving...' : '💾 Save Labels' }}</span>
                </button>
              </div>

              <div class="labels-editor-list">
                <div v-for="(_val, key) in nodeLabelsCopy" :key="key" class="label-edit-row font-mono">
                  <span class="label-key text-cyan">{{ key }}</span>
                  <span class="label-sep text-muted">=</span>
                  <input
                    v-model="nodeLabelsCopy[key]"
                    type="text"
                    class="input-glass font-mono label-val-input"
                  />
                  <button
                    type="button"
                    class="btn-remove-inline"
                    title="Remove label"
                    @click="removeLabelFromCopy(key as string)"
                  >
                    ✕
                  </button>
                </div>
              </div>

              <!-- Add Label Row -->
              <div class="add-label-row">
                <input
                  v-model="newLabelKey"
                  type="text"
                  placeholder="New label key"
                  class="input-glass font-mono form-input-sm"
                />
                <span class="text-muted">=</span>
                <input
                  v-model="newLabelValue"
                  type="text"
                  placeholder="New label value"
                  class="input-glass font-mono form-input-sm"
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-xs"
                  :disabled="!newLabelKey.trim()"
                  @click="addLabelToCopy"
                >
                  <span>+ Add</span>
                </button>
              </div>
            </div>
          </div>

          <!-- If Pod: Show Pod Info, Container List, Conditions List -->
          <div v-if="selectedResource.kind === 'Pod' || selectedKind === 'pods'" class="pod-detail-section">
            <!-- Pod Quick Actions Banner -->
            <div class="pod-quick-actions glass-panel">
              <div class="quick-actions-label font-mono font-small text-muted">Pod Quick Actions:</div>
              <div class="quick-actions-btns">
                <button 
                  type="button" 
                  class="btn btn-secondary btn-xs" 
                  title="View Pod Logs" 
                  @click="openPodLogs(selectedResource)"
                >
                  <span>📜 View Logs</span>
                </button>
                <button 
                  type="button" 
                  class="btn btn-secondary btn-xs" 
                  title="Open Web Terminal" 
                  @click="openPodTerminal(selectedResource)"
                >
                  <span>🖥️ Terminal</span>
                </button>
                <button 
                  type="button" 
                  class="btn btn-danger btn-xs" 
                  title="Delete Pod" 
                  @click="promptDelete(selectedResource)"
                >
                  <span>🗑️ Delete Pod</span>
                </button>
              </div>
            </div>

            <!-- Pod Runtime & Networking Info Card -->
            <div class="info-card glass-panel">
              <div class="card-title">Pod Runtime & Networking</div>
              <div class="meta-grid">
                <div class="meta-item">
                  <span class="meta-label">Node Name:</span>
                  <span class="font-mono text-cyan">{{ getPodNode(selectedResource) }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">Pod IP:</span>
                  <span class="font-mono">{{ getPodIP(selectedResource) }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">Host IP:</span>
                  <span class="font-mono">{{ getHostIP(selectedResource) }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">QoS Class:</span>
                  <span class="font-mono text-emerald">{{ getPodQoS(selectedResource) }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">Service Account:</span>
                  <span class="font-mono">{{ getPodServiceAccount(selectedResource) }}</span>
                </div>
                <div class="meta-item">
                  <span class="meta-label">Status Phase:</span>
                  <div><StatusBadge :status="getPodPhase(selectedResource)" size="sm" /></div>
                </div>
              </div>
            </div>

            <!-- Container List Card -->
            <div class="info-card glass-panel">
              <div class="card-title">
                <span>Containers ({{ getPodContainers(selectedResource).length }})</span>
              </div>
              <div class="containers-list">
                <div 
                  v-for="c in getPodContainers(selectedResource)" 
                  :key="c.name" 
                  class="container-item glass-panel"
                >
                  <div class="container-header">
                    <div class="container-name-wrap">
                      <span class="container-status-dot" :class="`dot-${c.ready ? 'emerald' : c.stateType === 'waiting' ? 'amber' : c.stateType === 'terminated' ? 'rose' : 'cyan'}`"></span>
                      <strong class="container-name font-mono">{{ c.name }}</strong>
                    </div>
                    <div class="container-badges">
                      <button 
                        type="button" 
                        class="btn-container-quick-action font-mono" 
                        title="View logs for this container"
                        @click.stop="openPodLogs(selectedResource, c.name)"
                      >
                        📜 Logs
                      </button>
                      <button 
                        type="button" 
                        class="btn-container-quick-action font-mono" 
                        title="Open terminal for this container"
                        @click.stop="openPodTerminal(selectedResource, c.name)"
                      >
                        🖥️ Term
                      </button>
                      <span class="badge-sm font-mono" :class="c.ready ? 'badge-ready' : 'badge-not-ready'">
                        {{ c.ready ? 'Ready' : 'Not Ready' }}
                      </span>
                      <span class="badge-sm font-mono badge-restarts" :class="{ 'has-restarts': c.restarts > 0 }">
                        Restarts: {{ c.restarts }}
                      </span>
                    </div>
                  </div>

                  <div class="container-details-grid font-mono">
                    <div class="cd-row">
                      <span class="cd-label">Image:</span>
                      <span class="cd-val text-cyan truncate" :title="c.image">{{ c.image }}</span>
                    </div>
                    <div class="cd-row">
                      <span class="cd-label">State:</span>
                      <span class="cd-val">{{ c.state }}</span>
                    </div>
                    <div class="cd-row">
                      <span class="cd-label">Ports:</span>
                      <span class="cd-val text-muted">{{ c.ports }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Conditions List Card -->
            <div v-if="getPodConditions(selectedResource).length > 0" class="info-card glass-panel">
              <div class="card-title">Pod Conditions</div>
              <div class="conditions-table">
                <div 
                  v-for="cond in getPodConditions(selectedResource)" 
                  :key="cond.type" 
                  class="condition-row"
                >
                  <div class="condition-status-col">
                    <span 
                      class="cond-icon" 
                      :class="cond.status === 'True' ? 'cond-true' : 'cond-false'"
                    >
                      {{ cond.status === 'True' ? '✓' : '✗' }}
                    </span>
                    <span class="cond-type font-mono">{{ cond.type }}</span>
                  </div>
                  <div class="condition-details">
                    <span class="cond-status-text font-mono" :class="cond.status === 'True' ? 'text-emerald' : 'text-amber'">
                      Status: {{ cond.status }}
                    </span>
                    <span v-if="cond.reason" class="cond-reason font-mono text-muted">
                      Reason: {{ cond.reason }}
                    </span>
                    <span v-if="cond.message" class="cond-msg font-mono text-muted font-small">
                      {{ cond.message }}
                    </span>
                    <span v-if="cond.lastTransitionTime" class="cond-time font-mono text-muted font-small">
                      Transition: {{ cond.lastTransitionTime }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- If Secret: Embed SecretViewer -->
          <div v-else-if="selectedResource.kind === 'Secret' || selectedKind === 'secrets'" class="secret-embed-section">
            <div class="card-title">Secret Credentials & Keys</div>
            <SecretViewer :secret="selectedResource" />
          </div>

          <!-- If ConfigMap: Show Key-Value Pairs -->
          <div v-else-if="selectedResource.data && Object.keys(selectedResource.data).length > 0" class="info-card glass-panel">
            <div class="card-title">ConfigMap Data</div>
            <div class="kv-display-table">
              <div v-for="(val, key) in selectedResource.data" :key="key" class="kv-item">
                <div class="kv-key font-mono">{{ key }}</div>
                <div class="kv-val font-mono">{{ val }}</div>
              </div>
            </div>
          </div>

          <!-- Spec Details -->
          <div v-if="selectedResource.spec && Object.keys(selectedResource.spec).length > 0" class="info-card glass-panel">
            <div class="card-title">Spec Configuration</div>
            <pre class="spec-pre font-mono">{{ jsonToYaml(selectedResource.spec) }}</pre>
          </div>
        </div>

        
        <!-- Tab: Events Timeline -->
        <div v-else-if="activeDrawerTab === 'events'" class="tab-pane animate-fade-in">
          <EventsTimeline
            :cluster="selectedCluster"
            :namespace="selectedResource.metadata?.namespace"
            :kind="selectedResource.kind"
            :name="selectedResource.metadata?.name"
            :auto-refresh="true"
          />
        </div>

        <!-- Tab 3: YAML -->
        <div v-else class="tab-pane animate-fade-in">
          <div class="yaml-pane-toolbar">
            <span class="text-muted font-mono font-small">apiVersion: {{ selectedResource.apiVersion }}</span>
            <div class="pane-actions">
              <button 
                type="button" 
                class="btn btn-secondary btn-xs"
                :class="{ 'btn-copied': drawerYamlCopied }"
                @click="copyDrawerYaml"
              >
                <span>{{ drawerYamlCopied ? '? Copied!' : '?? Copy YAML' }}</span>
              </button>
              <button 
                type="button" 
                class="btn btn-primary btn-xs"
                @click="openResourceYaml(selectedResource)"
              >
                <span>?? Edit in Editor</span>
              </button>
            </div>
          </div>
          <div class="yaml-code-wrapper font-mono">
            <pre class="yaml-pre">{{ jsonToYaml(selectedResource) }}</pre>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" @click="close">Close</button>
        <button 
          v-if="selectedResource"
          type="button" 
          class="btn btn-danger" 
          @click="promptDelete(selectedResource)"
        >
          <span>??? Delete Resource</span>
        </button>
        <button 
          v-if="selectedResource"
          type="button" 
          class="btn btn-primary" 
          @click="openResourceYaml(selectedResource)"
        >
          <span>?? Edit Manifest</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Create Resource Modal -->
    <CreateResourceModal
      v-model:show="showCreateModal"
      :cluster="selectedCluster"
      :namespaces="namespaces"
      :default-namespace="selectedNamespace !== 'all' ? selectedNamespace : 'default'"
      :default-kind="selectedKind"
      @created="onResourceCreated"
    />

    <!-- YAML Editor Modal -->
    <YamlEditorModal
      v-model:show="showYamlModal"
      :cluster="selectedCluster"
      :namespace="selectedNamespace !== 'all' ? selectedNamespace : 'default'"
      :title="yamlEditorTitle"
      :kind="currentKindLabel"
      :initial-yaml="yamlEditorInitialContent"
      :mode="yamlEditorMode"
      @applied="onYamlApplied"
    />

    <!-- New Namespace Modal -->
    <ModalDrawer
      v-model:show="showNewNsModal"
      mode="modal"
      title="Create Namespace"
      :subtitle="`Cluster: ${selectedCluster}`"
      max-width="480px"
    >
      <div class="form-group">
        <label class="form-label required">Namespace Name</label>
        <input 
          v-model="newNsName"
          type="text" 
          class="input-glass font-mono" 
          placeholder="e.g. stage-environment" 
          @keydown.enter="handleCreateNamespace"
        />
        <p v-if="newNsError" class="field-error">{{ newNsError }}</p>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="creatingNs" @click="close">Cancel</button>
        <button 
          type="button" 
          class="btn btn-primary" 
          :disabled="creatingNs || !newNsName.trim()"
          @click="handleCreateNamespace"
        >
          <span>{{ creatingNs ? 'Creating...' : 'Create Namespace' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Delete Confirmation Modal -->
    <ModalDrawer
      v-model:show="showDeleteModal"
      mode="modal"
      title="Confirm Resource Deletion"
      subtitle="This action is permanent and cannot be undone."
      max-width="480px"
    >
      <div v-if="resourceToDelete" class="delete-dialog-content">
        <p class="delete-msg">
          Are you sure you want to delete 
          <strong>{{ resourceToDelete.kind }}/{{ resourceToDelete.metadata?.name }}</strong> 
          in namespace 
          <strong>{{ resourceToDelete.metadata?.namespace || 'default' }}</strong>?
        </p>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="deletingResource" @click="close">Cancel</button>
        <button 
          type="button" 
          class="btn btn-danger" 
          :disabled="deletingResource"
          @click="handleDeleteConfirmed"
        >
          <span>{{ deletingResource ? 'Deleting...' : 'Confirm Delete' }}</span>
        </button>
      </template>
    </ModalDrawer>

    
    <!-- Node Drain Modal -->
    <ModalDrawer
      v-model:show="showDrainModal"
      mode="modal"
      title="Drain Kubernetes Node"
      :subtitle="'Cluster: ' + selectedCluster + ' • Node: ' + (drainTargetNode?.metadata?.name || '')"
      max-width="520px"
    >
      <div v-if="drainTargetNode" class="drain-dialog-content">
        <p class="delete-msg font-mono">
          Drain evicts all pods from <strong>{{ drainTargetNode.metadata?.name }}</strong> safely before node maintenance or decommission.
        </p>

        <div class="form-group">
          <label class="form-label">Grace Period (seconds)</label>
          <input
            v-model.number="drainOptions.gracePeriodSeconds"
            type="number"
            min="0"
            class="input-glass font-mono"
            placeholder="30"
          />
        </div>

        <div class="form-group checkbox-group">
          <label class="cyber-checkbox-label">
            <input
              v-model="drainOptions.ignoreDaemonSets"
              type="checkbox"
              class="cyber-checkbox"
            />
            <span>Ignore DaemonSet pods</span>
          </label>
        </div>

        <div class="form-group checkbox-group">
          <label class="cyber-checkbox-label">
            <input
              v-model="drainOptions.deleteEmptyDirData"
              type="checkbox"
              class="cyber-checkbox"
            />
            <span>Delete pods using emptyDir volume data</span>
          </label>
        </div>

        <div class="form-group checkbox-group">
          <label class="cyber-checkbox-label">
            <input
              v-model="drainOptions.force"
              type="checkbox"
              class="cyber-checkbox"
            />
            <span>Force eviction (even if unmanaged standalone pods exist)</span>
          </label>
        </div>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="drainingNode" @click="close">Cancel</button>
        <button 
          type="button" 
          class="btn btn-danger" 
          :disabled="drainingNode"
          @click="handleDrainNodeConfirm"
        >
          <span>{{ drainingNode ? 'Draining...' : 'Confirm Drain Node' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Import Cluster Modal -->
    <ModalDrawer
      v-model:show="showImportModal"
      mode="modal"
      title="Import Kubernetes Cluster"
      subtitle="Upload cluster kubeconfig to establish secure telemetry bridge"
      max-width="600px"
    >
      <div class="modal-form">
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label required">Cluster Identifier</label>
            <input v-model="importForm.id" type="text" placeholder="e.g. k8s-prod-us" class="input-glass font-mono" />
          </div>
          <div class="form-group flex-1">
            <label class="form-label required">Cluster Display Name</label>
            <input v-model="importForm.name" type="text" placeholder="e.g. Production US-East" class="input-glass" />
          </div>
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Fleet Tier</label>
            <select v-model="importForm.group" class="input-glass">
              <option value="production">Production</option>
              <option value="staging">Staging</option>
              <option value="development">Development</option>
              <option value="edge">Edge & Remote</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Provider</label>
            <select v-model="importForm.provider" class="input-glass">
              <option value="k8s">Generic K8s / Self-Hosted</option>
              <option value="aws">AWS (EKS)</option>
              <option value="gcp">Google Cloud (GKE)</option>
              <option value="azure">Azure (AKS)</option>
              <option value="baremetal">Bare Metal</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Region / Datacenter</label>
          <input v-model="importForm.region" type="text" placeholder="e.g. us-east-1 / On-Prem DC-01" class="input-glass" />
        </div>

        <div class="form-group">
          <div class="mode-tabs">
            <button
              type="button"
              class="mode-tab"
              :class="{ active: importMode === 'file' }"
              @click="importMode = 'file'"
            >
              📁 File Upload
            </button>
            <button
              type="button"
              class="mode-tab"
              :class="{ active: importMode === 'text' }"
              @click="importMode = 'text'"
            >
              📝 Paste YAML
            </button>
          </div>

          <div v-if="importMode === 'file'" class="file-drop-area">
            <label class="form-label">Kubeconfig File (.yaml / .config / .json) *</label>
            <input type="file" accept=".yaml,.yml,.config,.json" class="file-input" @change="handleFileChange" />
          </div>

          <div v-else class="text-paste-area">
            <label class="form-label">Paste Kubeconfig YAML Content *</label>
            <textarea
              v-model="importForm.kubeconfigRaw"
              rows="6"
              class="input-glass font-mono text-area-input select-full"
              placeholder="apiVersion: v1&#10;clusters:&#10;..."
            ></textarea>
          </div>
          <span class="form-hint">🔒 Kubeconfig is encrypted with AES-256 GCM in local vault before persistence. Sensitive tokens are never returned in cleartext.</span>
        </div>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="importingCluster" @click="close">Cancel</button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="importingCluster || !importForm.name.trim()"
          @click="handleImportCluster"
        >
          <span>{{ importingCluster ? 'Importing...' : 'Import Cluster' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Pod Terminal Modal -->
    <ModalDrawer
      v-model:show="showTerminalModal"
      mode="modal"
      :title="`Pod Terminal: ${terminalPod?.metadata?.name || 'Pod'}`"
      :subtitle="`Cluster: ${selectedCluster} | Namespace: ${terminalPod?.metadata?.namespace || 'default'}`"
      max-width="1000px"
    >
      <PodTerminal
        v-if="showTerminalModal && terminalPod"
        :cluster="selectedCluster"
        :pod="terminalPod.metadata.name"
        :namespace="terminalPod.metadata.namespace || 'default'"
        :container="selectedPodContainer"
        :containers="getPodContainerNames(terminalPod)"
      />
    </ModalDrawer>

    <!-- Pod Logs Modal -->
    <ModalDrawer
      v-model:show="showLogsModal"
      mode="modal"
      :title="`Pod Logs: ${logsPod?.metadata?.name || 'Pod'}`"
      :subtitle="`Cluster: ${selectedCluster} | Namespace: ${logsPod?.metadata?.namespace || 'default'}`"
      max-width="1100px"
    >
      <PodLogViewer
        v-if="showLogsModal && logsPod"
        :cluster="selectedCluster"
        :pod="logsPod.metadata.name"
        :namespace="logsPod.metadata.namespace || 'default'"
        :container="selectedPodContainer"
        :containers="getPodContainerNames(logsPod)"
      />
    </ModalDrawer>
  </div>
</template>

<style scoped>
.explorer-layout {
  display: flex;
  gap: 24px;
  min-height: calc(100vh - 120px);
}

.explorer-sidebar {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px;
  border-radius: 16px;
}

.sidebar-header {
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-subtle);
}

.sidebar-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.sidebar-title {
  font-size: 15px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.01em;
}

.sidebar-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-heading {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.section-heading-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.btn-icon-text {
  background: none;
  border: none;
  color: var(--accent-sky);
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
}

.btn-icon-text:hover {
  text-decoration: underline;
}

.select-full {
  width: 100%;
}

.sidebar-tree {
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
  flex: 1;
}

.tree-category {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.category-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 0 4px;
}

.category-items {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tree-item-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  text-align: left;
  transition: all 0.15s ease;
}

.tree-item-btn:hover {
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-primary);
}

.tree-item-btn.is-active {
  background: rgba(6, 182, 212, 0.12);
  border-color: rgba(6, 182, 212, 0.3);
  color: #38bdf8;
  font-weight: 600;
}

.item-icon {
  font-size: 14px;
}

.explorer-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.08em;
  margin-bottom: 4px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 4px;
}

.crumb-sep {
  color: var(--text-muted);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.toast-banner {
  padding: 12px 16px;
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

.toast-close {
  margin-left: auto;
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
}

.cluster-empty-box {
  padding: 8px 10px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px dashed var(--border-subtle);
  border-radius: 8px;
}

.offline-banner {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 18px 22px;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.35);
  border-radius: 14px;
  color: #fb7185;
}

.offline-icon {
  font-size: 26px;
  line-height: 1;
}

.offline-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.offline-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.offline-desc {
  font-size: 13px;
  color: #fecdd3;
  line-height: 1.5;
  margin: 0;
}

.offline-err-detail {
  font-size: 11px;
  color: #fda4af;
  background: rgba(0, 0, 0, 0.3);
  padding: 4px 8px;
  border-radius: 6px;
  word-break: break-all;
}

.offline-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
  flex-wrap: wrap;
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
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
  letter-spacing: 0.03em;
}

.form-hint {
  font-size: 11px;
  color: var(--text-muted);
  line-height: 1.4;
}

.mode-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.mode-tab {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.mode-tab.active {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
  color: #fff;
  font-weight: 600;
}

.file-input {
  font-size: 12px;
  color: var(--text-secondary);
}

.text-area-input {
  resize: vertical;
  min-height: 120px;
  font-size: 12px;
  line-height: 1.4;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.section-box {
  border-radius: 16px;
  overflow: hidden;
}

.resource-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.res-icon {
  font-size: 14px;
}

.res-link {
  color: var(--accent-cyan);
  font-weight: 700;
  text-decoration: none;
}

.res-link:hover {
  text-decoration: underline;
}

.app-tag {
  font-size: 10px;
  background: rgba(255, 255, 255, 0.06);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-muted);
}

.ns-badge {
  font-size: 11px;
  color: var(--text-secondary);
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.detail-drawer-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.drawer-tabs {
  display: flex;
  gap: 8px;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 12px;
}

.tab-btn {
  padding: 8px 16px;
  background: transparent;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tab-btn:hover {
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-primary);
}

.tab-btn.is-active {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
  color: #38bdf8;
}

.tab-pane {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-card {
  padding: 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.meta-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}

.meta-label {
  font-size: 11px;
  color: var(--text-muted);
}

.font-small {
  font-size: 10px;
  word-break: break-all;
}

.labels-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.label-badge {
  font-size: 11px;
  padding: 3px 8px;
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.25);
  border-radius: 6px;
  color: #38bdf8;
}

.kv-display-table {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.kv-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  background: rgba(11, 15, 25, 0.6);
  border-radius: 6px;
}

.kv-key {
  font-size: 11px;
  color: var(--accent-cyan);
  font-weight: bold;
}

.kv-val {
  font-size: 12px;
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-all;
}

.spec-pre {
  background: #030712;
  padding: 12px;
  border-radius: 8px;
  font-size: 11.5px;
  color: #38bdf8;
  max-height: 240px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.yaml-pane-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.pane-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.yaml-code-wrapper {
  background: #030712;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  overflow: hidden;
}

.yaml-pre {
  padding: 16px;
  font-size: 12px;
  line-height: 1.6;
  color: #e0f2fe;
  max-height: 520px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.delete-dialog-content {
  padding: 8px 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-secondary);
}

.delete-msg strong {
  color: var(--text-primary);
}

.field-error {
  color: #fb7185;
  font-size: 12px;
  margin-top: 4px;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }

@media (max-width: 1024px) {
  .explorer-layout {
    flex-direction: column;
  }
  .explorer-sidebar {
    width: 100%;
  }
}

@media (max-width: 768px) {
  .view-header {
    flex-direction: column;
    align-items: stretch;
    gap: 14px;
  }

  .header-actions {
    display: flex !important;
    flex-direction: row !important;
    width: 100% !important;
    gap: 8px !important;
    flex-wrap: wrap !important;
  }

  .header-actions .btn,
  .header-actions button,
  .header-actions a {
    flex: 1 1 calc(50% - 4px) !important;
    min-width: 0 !important;
    padding: 7px 10px !important;
    font-size: 11.5px !important;
    white-space: nowrap !important;
    justify-content: center !important;
  }

  .header-actions > :last-child:nth-child(odd) {
    flex: 1 1 100% !important;
  }

  .metrics-grid,
  .grid-metrics,
  .stats-grid,
  .catalog-stats-grid {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 8px !important;
  }

  :deep(.metric-card),
  .metric-card,
  :deep(.stat-card),
  .stat-card,
  :deep(.catalog-stat-card),
  .catalog-stat-card {
    padding: 10px 12px !important;
  }

  .sidebar-tree {
    flex-direction: row;
    overflow-x: auto;
    scrollbar-width: thin;
    -webkit-overflow-scrolling: touch;
    gap: 12px;
    padding-bottom: 8px;
  }

  .tree-category {
    flex-direction: row;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  .category-items {
    flex-direction: row;
    gap: 4px;
    flex-shrink: 0;
  }

  .tree-item-btn {
    white-space: nowrap;
    padding: 6px 10px;
    font-size: 12px;
  }

  .yaml-pane-toolbar {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .pane-actions {
    width: 100%;
    justify-content: flex-end;
  }
}

@media (max-width: 640px) {
  .explorer-sidebar {
    padding: 14px;
    gap: 14px;
    border-radius: 12px;
  }

  .header-actions {
    display: flex !important;
    flex-direction: row !important;
    width: 100% !important;
    gap: 8px !important;
    flex-wrap: wrap !important;
  }

  .header-actions .btn,
  .header-actions button,
  .header-actions a {
    flex: 1 1 calc(50% - 4px) !important;
    min-width: 0 !important;
    padding: 7px 10px !important;
    font-size: 11.5px !important;
    white-space: nowrap !important;
    justify-content: center !important;
  }

  .header-actions > :last-child:nth-child(odd) {
    flex: 1 1 100% !important;
  }

  .metrics-grid,
  .grid-metrics,
  .stats-grid,
  .catalog-stats-grid {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 8px !important;
  }

  :deep(.metric-card),
  .metric-card,
  :deep(.stat-card),
  .stat-card,
  :deep(.catalog-stat-card),
  .catalog-stat-card {
    padding: 10px 12px !important;
  }

  :deep(.metric-card .metric-value),
  .metric-card .metric-value {
    font-size: 18px;
  }

  :deep(.metric-card .metric-title),
  .metric-card .metric-title {
    font-size: 10px;
  }

  :deep(.metric-card .metric-footer),
  .metric-card .metric-footer {
    font-size: 10px;
  }

  .breadcrumbs {
    flex-wrap: wrap;
    font-size: 11px;
    gap: 4px;
  }

  .meta-grid {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .yaml-code-wrapper {
    overflow-x: auto;
  }

  .yaml-pre,
  .spec-pre {
    height: 350px;
    max-height: 350px;
    font-size: 11px;
    padding: 10px;
    white-space: pre;
    overflow-x: auto;
    word-break: normal;
  }

  .drawer-tabs {
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: thin;
    -webkit-overflow-scrolling: touch;
  }

  .tab-btn {
    white-space: nowrap;
    flex: 1;
    text-align: center;
  }
}

/* Pod Specific Styles */
.ready-cell {
  color: var(--text-secondary);
}
.ready-all {
  color: #34d399;
  font-weight: 600;
}
.restarts-cell {
  color: var(--text-secondary);
}
.has-restarts {
  color: #fbbf24;
  font-weight: 600;
}

.pod-detail-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.pod-quick-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.02);
}

.quick-actions-btns {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-disabled,
.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed !important;
  pointer-events: auto !important;
}

.text-emerald { color: #34d399; }
.text-amber { color: #fbbf24; }

.containers-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.container-item {
  padding: 12px;
  border-radius: 10px;
  background: rgba(11, 15, 25, 0.5);
  border: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.container-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding-bottom: 6px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.container-name-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.container-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-emerald { background-color: #10b981; box-shadow: 0 0 6px #10b981; }
.dot-amber { background-color: #f59e0b; box-shadow: 0 0 6px #f59e0b; }
.dot-rose { background-color: #f43f5e; box-shadow: 0 0 6px #f43f5e; }
.dot-cyan { background-color: #06b6d4; box-shadow: 0 0 6px #06b6d4; }

.container-name {
  font-size: 13px;
  color: #fff;
}

.container-badges {
  display: flex;
  align-items: center;
  gap: 6px;
}

.badge-ready {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
}

.badge-not-ready {
  background: rgba(244, 63, 94, 0.12);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.3);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
}

.badge-restarts {
  background: rgba(255, 255, 255, 0.05);
  color: var(--text-secondary);
  border: 1px solid var(--border-subtle);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
}

.badge-restarts.has-restarts {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border-color: rgba(245, 158, 11, 0.3);
}

.container-details-grid {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 11px;
}

.cd-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.cd-label {
  color: var(--text-muted);
  width: 48px;
  flex-shrink: 0;
}

.cd-val {
  color: var(--text-primary);
  word-break: break-all;
}

.truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conditions-table {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.condition-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 8px 10px;
  background: rgba(11, 15, 25, 0.5);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
}

.condition-status-col {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 140px;
}

.cond-icon {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  font-weight: bold;
}

.cond-true {
  background: rgba(16, 185, 129, 0.2);
  color: #34d399;
}

.cond-false {
  background: rgba(244, 63, 94, 0.2);
  color: #fb7185;
}

.cond-type {
  font-size: 11px;
  font-weight: 600;
  color: #fff;
}

.condition-details {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  font-size: 11px;
}

.cond-status-text {
  font-weight: 600;
}


.btn-container-quick-action {
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.3);
  color: #38bdf8;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-container-quick-action:hover {
  background: rgba(56, 189, 248, 0.25);
  border-color: #38bdf8;
  color: #e0f2fe;
}

/* Roles Badge */
.roles-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.role-badge {
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(0, 229, 255, 0.12);
  color: #00e5ff;
  border: 1px solid rgba(0, 229, 255, 0.25);
}

/* Event Cells */
.event-type-badge {
  font-size: 0.72rem;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
  text-transform: uppercase;
}

.type-warning {
  background: rgba(245, 158, 11, 0.18);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.35);
}

.type-normal {
  background: rgba(0, 229, 255, 0.12);
  color: #00e5ff;
  border: 1px solid rgba(0, 229, 255, 0.3);
}

.event-msg-cell {
  max-width: 380px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}

/* Node Drawer Sections */
.node-detail-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.card-header-flex {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.conditions-table-wrap {
  overflow-x: auto;
}

.conditions-table {
  width: 100%;
  border-collapse: collapse;
}

.conditions-table th, .conditions-table td {
  padding: 8px 10px;
  text-align: left;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.conditions-table th {
  color: #94a3b8;
  font-weight: 600;
}

.status-pill {
  padding: 2px 7px;
  border-radius: 4px;
  font-weight: 700;
}

.pill-green {
  background: rgba(16, 185, 129, 0.2);
  color: #10b981;
}

.pill-amber {
  background: rgba(245, 158, 11, 0.2);
  color: #f59e0b;
}

.pill-red {
  background: rgba(244, 63, 94, 0.2);
  color: #fb7185;
}

.text-break {
  word-break: break-word;
  max-width: 260px;
}

.empty-inline-hint {
  padding: 8px 0;
}

.taints-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}

.taint-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(30, 41, 59, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.1);
  font-size: 0.78rem;
}

.taint-effect {
  font-weight: 600;
}

.btn-remove-inline {
  background: none;
  border: none;
  color: #fb7185;
  cursor: pointer;
  padding: 0 2px;
  font-size: 0.75rem;
}

.btn-remove-inline:hover {
  color: #f43f5e;
}

.add-taint-form {
  padding: 10px 12px;
  border-radius: 6px;
  background: rgba(15, 23, 42, 0.4);
  border: 1px dashed rgba(255, 255, 255, 0.1);
}

.add-taint-inputs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
}

.form-input-sm {
  height: 28px;
  padding: 4px 8px;
  font-size: 0.78rem;
  flex: 1;
  min-width: 110px;
}

.select-effect {
  height: 28px;
  padding: 2px 6px;
  font-size: 0.78rem;
  min-width: 130px;
}

.labels-editor-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 220px;
  overflow-y: auto;
  margin-bottom: 12px;
}

.label-edit-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.label-key {
  min-width: 140px;
  word-break: break-all;
  font-size: 0.78rem;
}

.label-val-input {
  flex: 1;
  height: 26px;
  padding: 2px 8px;
  font-size: 0.78rem;
}

.add-label-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

</style>
