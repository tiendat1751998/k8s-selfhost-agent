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

// Workload Action: Scale Modal State
const showScaleModal = ref(false)
const scaleTarget = ref<K8sResource | null>(null)
const scaleReplicasCount = ref(1)
const scalingResource = ref(false)

// ConfigMap KV Editor in Detail Drawer State
const configMapDataCopy = ref<Record<string, string>>({})
const newCmKey = ref('')
const newCmValue = ref('')
const savingConfigMap = ref(false)

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

// Resource Kind Tree Definition
interface KindCategory {
  title: string
  icon: string
  items: { label: string; kind: ResourceKind; icon: string }[]
}

const kindCategories: KindCategory[] = [
  {
    title: 'Cluster',
    icon: '🖥️',
    items: [
      { label: 'Nodes', kind: 'nodes', icon: '🖥️' },
      { label: 'PersistentVolumes', kind: 'persistentvolumes', icon: '💿' },
      { label: 'StorageClasses', kind: 'storageclasses', icon: '🏷️' },
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
      { label: 'HorizontalPodAutoscalers', kind: 'horizontalpodautoscalers', icon: '📈' },
    ],
  },
  {
    title: 'Networking & Security',
    icon: '🌐',
    items: [
      { label: 'Services', kind: 'services', icon: '🔌' },
      { label: 'Ingresses', kind: 'ingresses', icon: '🌐' },
      { label: 'NetworkPolicies', kind: 'networkpolicies', icon: '🛡️' },
      { label: 'ServiceAccounts', kind: 'serviceaccounts', icon: '👤' },
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

interface ContainerPort { name?: string; containerPort: number; protocol?: string; hostPort?: number }
interface ContainerSpec { name: string; image?: string; ports?: ContainerPort[]; command?: string[]; args?: string[]; env?: { name: string; value?: string }[] }
interface ContainerStateWaiting { reason?: string; message?: string }
interface ContainerStateRunning { startedAt?: string }
interface ContainerStateTerminated { exitCode?: number; reason?: string; message?: string; startedAt?: string; finishedAt?: string }
interface ContainerState { waiting?: ContainerStateWaiting; running?: ContainerStateRunning; terminated?: ContainerStateTerminated }
interface ContainerStatus { name: string; image?: string; imageID?: string; ready?: boolean; restartCount?: number; started?: boolean; state?: ContainerState; lastState?: ContainerState }
// PodCondition interface
interface MergedContainerInfo { name: string; image: string; ports: string; state: string; stateType: 'running' | 'waiting' | 'terminated' | 'unknown'; ready: boolean; restarts: number }

const podColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'ready', label: 'Ready', width: '90px', sortable: true },
  { key: 'restarts', label: 'Restarts', width: '90px', sortable: true },
  { key: 'node', label: 'Node', width: '140px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '280px', align: 'right' },
]

const deploymentColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'replicas', label: 'Replicas (Ready/Desired)', width: '190px', sortable: true },
  { key: 'image', label: 'Image', sortable: true },
  { key: 'selector', label: 'Selector', width: '180px', sortable: false },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '260px', align: 'right' },
]

const statefulSetColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'replicas', label: 'Replicas (Ready/Desired)', width: '190px', sortable: true },
  { key: 'image', label: 'Image', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '260px', align: 'right' },
]

const daemonSetColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'desired', label: 'Desired', width: '90px', sortable: true },
  { key: 'current', label: 'Current', width: '90px', sortable: true },
  { key: 'ready', label: 'Ready', width: '90px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '220px', align: 'right' },
]

const jobColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'completions', label: 'Completions', width: '130px', sortable: true },
  { key: 'duration', label: 'Duration', width: '120px', sortable: true },
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

const cronJobColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'schedule', label: 'Schedule', width: '140px', sortable: true },
  { key: 'suspend', label: 'Suspend', width: '100px', sortable: true },
  { key: 'active', label: 'Active', width: '90px', sortable: true },
  { key: 'lastSchedule', label: 'Last Schedule', width: '130px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '280px', align: 'right' },
]

const serviceColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'type', label: 'Type', width: '130px', sortable: true },
  { key: 'clusterIP', label: 'Cluster IP', width: '140px', sortable: true },
  { key: 'externalIP', label: 'External IP', width: '140px', sortable: true },
  { key: 'ports', label: 'Ports', width: '180px', sortable: false },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

const ingressColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'hosts', label: 'Hosts', width: '200px', sortable: true },
  { key: 'paths', label: 'Paths', width: '160px', sortable: false },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

const configMapColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'keysCount', label: 'Keys Count', width: '110px', sortable: true },
  { key: 'dataPreview', label: 'Data Preview', sortable: false },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '220px', align: 'right' },
]

const secretColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'type', label: 'Type', width: '160px', sortable: true },
  { key: 'keysCount', label: 'Keys Count', width: '110px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '220px', align: 'right' },
]

const pvcColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'status', label: 'Status', width: '120px', sortable: true },
  { key: 'capacity', label: 'Capacity', width: '110px', sortable: true },
  { key: 'accessModes', label: 'Access Modes', width: '150px', sortable: true },
  { key: 'storageClass', label: 'StorageClass', width: '140px', sortable: true },
  { key: 'volume', label: 'Volume', width: '160px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

const pvColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'status', label: 'Status', width: '120px', sortable: true },
  { key: 'capacity', label: 'Capacity', width: '110px', sortable: true },
  { key: 'accessModes', label: 'Access Modes', width: '150px', sortable: true },
  { key: 'reclaimPolicy', label: 'Reclaim Policy', width: '140px', sortable: true },
  { key: 'storageClass', label: 'StorageClass', width: '140px', sortable: true },
  { key: 'claim', label: 'Claim', width: '180px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

const storageClassColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'provisioner', label: 'Provisioner', sortable: true },
  { key: 'reclaimPolicy', label: 'ReclaimPolicy', width: '140px', sortable: true },
  { key: 'volumeBindingMode', label: 'VolumeBindingMode', width: '170px', sortable: true },
  { key: 'defaultClass', label: 'Default', width: '90px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

const networkPolicyColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'podSelector', label: 'Pod Selector', width: '220px', sortable: true },
  { key: 'policyTypes', label: 'Policy Types', width: '160px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

const serviceAccountColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'secretsCount', label: 'Secrets', width: '110px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

const hpaColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '140px', sortable: true },
  { key: 'reference', label: 'Reference', width: '180px', sortable: true },
  { key: 'targets', label: 'Targets', width: '150px', sortable: true },
  { key: 'minMax', label: 'Min/Max', width: '110px', sortable: true },
  { key: 'replicas', label: 'Replicas', width: '100px', sortable: true },
  { key: 'age', label: 'Age', width: '100px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
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

function formatAge(dateStr?: string): string {
  if (!dateStr) return 'Unknown'
  const diff = Date.now() - new Date(dateStr).getTime()
  if (isNaN(diff) || diff < 0) return 'Just now'
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return `${days}d`
}

function getResourceAge(resource: K8sResource): string {
  return formatAge(resource.metadata?.creationTimestamp)
}

function getPodPhase(resource: K8sResource): string {
  if (resource.status && typeof resource.status === 'object') {
    const status = resource.status as Record<string, unknown>
    const containerStatuses = status.containerStatuses as ContainerStatus[] | undefined
    if (Array.isArray(containerStatuses)) {
      for (const cs of containerStatuses) {
        if (cs.state?.waiting?.reason) return cs.state.waiting.reason
        if (cs.state?.terminated?.reason && cs.state.terminated.reason !== 'Completed') return cs.state.terminated.reason
      }
    }
    if (typeof status.phase === 'string' && status.phase) return status.phase
  }
  return 'Unknown'
}

function getPodReadyCount(resource: K8sResource): string {
  const spec = resource.spec as { containers?: ContainerSpec[] } | undefined
  const status = resource.status as { containerStatuses?: ContainerStatus[] } | undefined
  const total = Array.isArray(spec?.containers) ? spec.containers.length : Array.isArray(status?.containerStatuses) ? status.containerStatuses.length : 0
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
  if (spec?.nodeName) return spec.nodeName
  const status = resource.status as { hostIP?: string } | undefined
  return status?.hostIP || 'Unassigned'
}

function getPodIP(resource: K8sResource): string {
  const status = resource.status as { podIP?: string; podIPs?: { ip: string }[] } | undefined
  if (status?.podIP) return status.podIP
  if (Array.isArray(status?.podIPs) && status.podIPs.length > 0 && status.podIPs[0]?.ip) return status.podIPs[0].ip
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
      portsStr = specC.ports.map(p => `${p.containerPort}${p.protocol ? '/' + p.protocol : ''}${p.name ? ' (' + p.name + ')' : ''}`).join(', ')
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
    results.push({ name, image, ports: portsStr, state: stateStr, stateType, ready: Boolean(statC?.ready), restarts: Number(statC?.restartCount) || 0 })
  }
  return results
}



function getDeploymentReplicas(row: K8sResource): string {
  const spec = row.spec as { replicas?: number } | undefined
  const status = row.status as { readyReplicas?: number; replicas?: number; updatedReplicas?: number; availableReplicas?: number } | undefined
  const ready = status?.readyReplicas || 0
  const desired = spec?.replicas ?? status?.replicas ?? 0
  return `${ready}/${desired}`
}

function getDeploymentImage(row: K8sResource): string {
  const spec = row.spec as { template?: { spec?: { containers?: ContainerSpec[] } } } | undefined
  const containers = spec?.template?.spec?.containers
  return (Array.isArray(containers) && containers.length > 0) ? containers[0].image || 'unknown' : 'N/A'
}

function getDeploymentSelector(row: K8sResource): string {
  const spec = row.spec as { selector?: { matchLabels?: Record<string, string> } } | undefined
  const labels = spec?.selector?.matchLabels
  if (labels && Object.keys(labels).length > 0) return Object.entries(labels).map(([k, v]) => `${k}=${v}`).join(', ')
  return 'N/A'
}

function getStatefulSetReplicas(row: K8sResource): string {
  const spec = row.spec as { replicas?: number } | undefined
  const status = row.status as { readyReplicas?: number; replicas?: number } | undefined
  const ready = status?.readyReplicas || 0
  const desired = spec?.replicas ?? status?.replicas ?? 0
  return `${ready}/${desired}`
}

function getStatefulSetImage(row: K8sResource): string {
  const spec = row.spec as { template?: { spec?: { containers?: ContainerSpec[] } } } | undefined
  const containers = spec?.template?.spec?.containers
  return containers?.[0]?.image || 'N/A'
}

function getDaemonSetDesired(row: K8sResource): number {
  return (row.status as { desiredNumberScheduled?: number })?.desiredNumberScheduled || 0
}

function getDaemonSetCurrent(row: K8sResource): number {
  return (row.status as { currentNumberScheduled?: number })?.currentNumberScheduled || 0
}

function getDaemonSetReady(row: K8sResource): number {
  return (row.status as { numberReady?: number })?.numberReady || 0
}

function getJobCompletions(row: K8sResource): string {
  const spec = row.spec as { completions?: number } | undefined
  const status = row.status as { succeeded?: number } | undefined
  return `${status?.succeeded || 0}/${spec?.completions || 1}`
}

function getJobDuration(row: K8sResource): string {
  const status = row.status as { startTime?: string; completionTime?: string } | undefined
  if (!status?.startTime) return 'Pending'
  const start = new Date(status.startTime).getTime()
  const end = status.completionTime ? new Date(status.completionTime).getTime() : Date.now()
  const diffSec = Math.floor((end - start) / 1000)
  if (diffSec < 60) return `${diffSec}s`
  const mins = Math.floor(diffSec / 60)
  const secs = diffSec % 60
  return `${mins}m ${secs}s`
}

function getJobStatus(row: K8sResource): string {
  const status = row.status as { succeeded?: number; failed?: number; active?: number } | undefined
  if (status?.succeeded && status.succeeded > 0) return 'Completed'
  if (status?.failed && status.failed > 0) return 'Failed'
  if (status?.active && status.active > 0) return 'Running'
  return 'Pending'
}

function getCronJobSchedule(row: K8sResource): string {
  return (row.spec as { schedule?: string })?.schedule || 'N/A'
}

function getCronJobSuspend(row: K8sResource): string {
  return (row.spec as { suspend?: boolean })?.suspend ? 'True' : 'False'
}

function getCronJobActive(row: K8sResource): number {
  const status = row.status as { active?: unknown[] } | undefined
  return Array.isArray(status?.active) ? status.active.length : 0
}

function getCronJobLastSchedule(row: K8sResource): string {
  const status = row.status as { lastScheduleTime?: string } | undefined
  return status?.lastScheduleTime ? formatAge(status.lastScheduleTime) : 'Never'
}

function getServiceType(row: K8sResource): string {
  return (row.spec as { type?: string })?.type || 'ClusterIP'
}

function getServiceClusterIP(row: K8sResource): string {
  return (row.spec as { clusterIP?: string })?.clusterIP || 'None'
}

function getServiceExternalIP(row: K8sResource): string {
  const spec = row.spec as { externalIPs?: string[] } | undefined
  const status = row.status as { loadBalancer?: { ingress?: { ip?: string; hostname?: string }[] } } | undefined
  const lb = status?.loadBalancer?.ingress?.[0]
  if (lb?.ip) return lb.ip
  if (lb?.hostname) return lb.hostname
  if (spec?.externalIPs && spec.externalIPs.length > 0) return spec.externalIPs.join(', ')
  return 'None'
}

function getServicePorts(row: K8sResource): string {
  const spec = row.spec as { ports?: { port: number; targetPort?: number | string; protocol?: string }[] } | undefined
  if (Array.isArray(spec?.ports) && spec.ports.length > 0) {
    return spec.ports.map(p => `${p.port}${p.targetPort ? ':' + p.targetPort : ''}/${p.protocol || 'TCP'}`).join(', ')
  }
  return 'None'
}

function getIngressHosts(row: K8sResource): string {
  const spec = row.spec as { rules?: { host?: string }[] } | undefined
  if (Array.isArray(spec?.rules) && spec.rules.length > 0) {
    const hosts = spec.rules.map(r => r.host).filter(Boolean)
    return hosts.length > 0 ? hosts.join(', ') : '*'
  }
  return '*'
}

function getIngressPaths(row: K8sResource): string {
  const spec = row.spec as { rules?: { http?: { paths?: { path?: string }[] } }[] } | undefined
  if (Array.isArray(spec?.rules)) {
    const paths = spec.rules.flatMap(r => r.http?.paths?.map(p => p.path)).filter(Boolean)
    return paths.length > 0 ? paths.join(', ') : '/'
  }
  return '/'
}

function getConfigMapKeysCount(row: K8sResource): number {
  if (row.data) return Object.keys(row.data).length
  if (row.binaryData) return Object.keys(row.binaryData).length
  return 0
}

function getConfigMapDataPreview(row: K8sResource): string {
  if (row.data) {
    const keys = Object.keys(row.data)
    if (keys.length === 0) return 'Empty'
    return keys.slice(0, 3).join(', ') + (keys.length > 3 ? ` (+${keys.length - 3} more)` : '')
  }
  return 'None'
}

function getSecretType(row: K8sResource): string { return row.type || 'Opaque' }
function getSecretKeysCount(row: K8sResource): number {
  if (row.data) return Object.keys(row.data).length
  if (row.stringData) return Object.keys(row.stringData).length
  return 0
}

function getPvcStatus(row: K8sResource): string { return (row.status as { phase?: string })?.phase || 'Pending' }
function getPvcCapacity(row: K8sResource): string {
  const status = row.status as { capacity?: { storage?: string } } | undefined
  const spec = row.spec as { resources?: { requests?: { storage?: string } } } | undefined
  return status?.capacity?.storage || spec?.resources?.requests?.storage || 'N/A'
}
function getPvcAccessModes(row: K8sResource): string { return (row.spec as { accessModes?: string[] })?.accessModes?.join(', ') || 'N/A' }
function getPvcStorageClass(row: K8sResource): string { return (row.spec as { storageClassName?: string })?.storageClassName || 'N/A' }
function getPvcVolume(row: K8sResource): string { return (row.spec as { volumeName?: string })?.volumeName || 'N/A' }

function getPvStatus(row: K8sResource): string { return (row.status as { phase?: string })?.phase || 'Available' }
function getPvCapacity(row: K8sResource): string { return (row.spec as { capacity?: { storage?: string } })?.capacity?.storage || 'N/A' }
function getPvAccessModes(row: K8sResource): string { return (row.spec as { accessModes?: string[] })?.accessModes?.join(', ') || 'N/A' }
function getPvReclaimPolicy(row: K8sResource): string { return (row.spec as { persistentVolumeReclaimPolicy?: string })?.persistentVolumeReclaimPolicy || 'Retain' }
function getPvStorageClass(row: K8sResource): string { return (row.spec as { storageClassName?: string })?.storageClassName || 'N/A' }
function getPvClaim(row: K8sResource): string {
  const spec = row.spec as { claimRef?: { namespace?: string; name?: string } } | undefined
  return spec?.claimRef?.name ? `${spec.claimRef.namespace || 'default'}/${spec.claimRef.name}` : 'None'
}

function getScProvisioner(row: K8sResource): string { return (row as { provisioner?: string }).provisioner || 'N/A' }
function getScReclaimPolicy(row: K8sResource): string { return (row as { reclaimPolicy?: string }).reclaimPolicy || 'Delete' }
function getScVolumeBindingMode(row: K8sResource): string { return (row as { volumeBindingMode?: string }).volumeBindingMode || 'Immediate' }
function getScDefault(row: K8sResource): string { return row.metadata?.annotations?.['storageclass.kubernetes.io/is-default-class'] === 'true' ? 'Yes' : 'No' }

function getNetPolPodSelector(row: K8sResource): string {
  const spec = row.spec as { podSelector?: { matchLabels?: Record<string, string> } } | undefined
  const matchLabels = spec?.podSelector?.matchLabels
  if (matchLabels && Object.keys(matchLabels).length > 0) return Object.entries(matchLabels).map(([k, v]) => `${k}=${v}`).join(', ')
  return 'All Pods'
}
function getNetPolPolicyTypes(row: K8sResource): string { return (row.spec as { policyTypes?: string[] })?.policyTypes?.join(', ') || 'Ingress' }
function getSaSecrets(row: K8sResource): number { const s = (row as { secrets?: unknown[] }).secrets; return Array.isArray(s) ? s.length : 0 }
function getHpaReference(row: K8sResource): string {
  const spec = row.spec as { scaleTargetRef?: { kind?: string; name?: string } } | undefined
  return (spec?.scaleTargetRef?.kind && spec.scaleTargetRef?.name) ? `${spec.scaleTargetRef.kind}/${spec.scaleTargetRef.name}` : 'N/A'
}
function getHpaTargets(row: K8sResource): string {
  const spec = row.spec as { targetCPUUtilizationPercentage?: number } | undefined
  const status = row.status as { currentCPUUtilizationPercentage?: number } | undefined
  return spec?.targetCPUUtilizationPercentage !== undefined ? `${status?.currentCPUUtilizationPercentage ?? 0}% / ${spec.targetCPUUtilizationPercentage}%` : 'N/A'
}
function getHpaMinMax(row: K8sResource): string {
  const spec = row.spec as { minReplicas?: number; maxReplicas?: number } | undefined
  return `${spec?.minReplicas || 1}-${spec?.maxReplicas || 1}`
}
function getHpaReplicas(row: K8sResource): number { return (row.status as { currentReplicas?: number })?.currentReplicas || 0 }


function getNodeTaints(resource: K8sResource): NodeTaint[] {
  return ((resource.spec as { taints?: NodeTaint[] } | undefined)?.taints) || []
}

function getEventType(resource: K8sResource): string {
  return (resource as unknown as { type?: string }).type || 'Normal'
}

function getResourceStatus(resource: K8sResource): string {
  if (resource.status && typeof resource.status === 'object') {
    if ('phase' in resource.status && typeof resource.status.phase === 'string') return resource.status.phase
    if ('readyReplicas' in resource.status && 'replicas' in resource.status) return `${resource.status.readyReplicas || 0}/${resource.status.replicas || 0} Ready`
  }
  return 'Active'
}

function isNodeUnschedulable(resource: K8sResource): boolean { return Boolean((resource.spec as { unschedulable?: boolean })?.unschedulable) }
function getNodeStatus(resource: K8sResource): string {
  if (isNodeUnschedulable(resource)) return 'SchedulingDisabled'
  const conditions = (resource.status as { conditions?: NodeCondition[] })?.conditions
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
  if (labels['kubernetes.io/role'] && !roles.includes(labels['kubernetes.io/role'])) roles.push(labels['kubernetes.io/role'])
  return roles.length > 0 ? roles : ['worker']
}
function getNodeVersion(resource: K8sResource): string { return (resource.status as { nodeInfo?: NodeSystemInfo })?.nodeInfo?.kubeletVersion || 'N/A' }
function getNodeInternalIP(resource: K8sResource): string {
  const addresses = (resource.status as { addresses?: { type: string; address: string }[] })?.addresses
  return Array.isArray(addresses) ? (addresses.find(a => a.type === 'InternalIP')?.address || 'N/A') : 'N/A'
}
function getNodeOSArch(resource: K8sResource): string {
  const info = (resource.status as { nodeInfo?: NodeSystemInfo })?.nodeInfo
  return (info?.operatingSystem && info?.architecture) ? info.operatingSystem + '/' + info.architecture : info?.osImage || 'Linux'
}
function getNodePodsCount(resource: K8sResource): string {
  const allocatable = (resource.status as { allocatable?: { pods?: string } })?.allocatable
  return allocatable?.pods ? allocatable.pods + ' max' : 'N/A'
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

function initConfigMapEditor(cm: K8sResource) {
  configMapDataCopy.value = { ...(cm.data || {}) }
  newCmKey.value = ''
  newCmValue.value = ''
}

function addCmEntry() {
  if (!newCmKey.value.trim()) return
  configMapDataCopy.value[newCmKey.value.trim()] = newCmValue.value
  newCmKey.value = ''
  newCmValue.value = ''
}

function removeCmEntry(key: string) {
  delete configMapDataCopy.value[key]
}

async function handleSaveConfigMap(cm: K8sResource) {
  const name = cm.metadata?.name
  const ns = cm.metadata?.namespace || 'default'
  if (!name || !selectedCluster.value) return
  savingConfigMap.value = true
  try {
    const updated = await k8sApi.updateResource(
      selectedCluster.value,
      'configmaps',
      name,
      {
        ...cm,
        data: configMapDataCopy.value,
      },
      ns
    )
    showToast(`ConfigMap ${name} saved successfully!`)
    selectedResource.value = updated
    await fetchResources()
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to save ConfigMap', 'error')
  } finally {
    savingConfigMap.value = false
  }
}

function openDetailDrawer(resource: K8sResource) {
  selectedResource.value = resource
  activeDrawerTab.value = 'overview'
  if (resource.kind === 'Node' || selectedKind.value === 'nodes') {
    initNodeLabelsEditor(resource)
  }
  if (resource.kind === 'ConfigMap' || selectedKind.value === 'configmaps') {
    initConfigMapEditor(resource)
  }
  showDetailDrawer.value = true
}

function openScaleModal(resource: K8sResource) {
  scaleTarget.value = resource
  const spec = resource.spec as { replicas?: number } | undefined
  const status = resource.status as { replicas?: number } | undefined
  scaleReplicasCount.value = spec?.replicas ?? status?.replicas ?? 1
  showScaleModal.value = true
}

async function handleScaleConfirm() {
  if (!scaleTarget.value || !selectedCluster.value) return
  const r = scaleTarget.value
  const name = r.metadata?.name || ''
  const ns = r.metadata?.namespace || 'default'
  const kind = (r.kind || '').toLowerCase()
  scalingResource.value = true
  try {
    if (kind === 'statefulset' || selectedKind.value === 'statefulsets') {
      await k8sApi.scaleStatefulSet(selectedCluster.value, name, ns, scaleReplicasCount.value)
    } else {
      await k8sApi.scaleDeployment(selectedCluster.value, name, ns, scaleReplicasCount.value)
    }
    showToast(`Scaled ${r.kind}/${name} to ${scaleReplicasCount.value} replicas`)
    showScaleModal.value = false
    scaleTarget.value = null
    await fetchResources()
    if (selectedResource.value && selectedResource.value.metadata?.name === name) {
      selectedResource.value = await k8sApi.getResource(selectedCluster.value, selectedKind.value, name, ns)
    }
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to scale workload', 'error')
  } finally {
    scalingResource.value = false
  }
}

async function handleRestartWorkload(resource: K8sResource) {
  const name = resource.metadata?.name || ''
  const ns = resource.metadata?.namespace || 'default'
  const kind = (resource.kind || '').toLowerCase()
  if (!name || !selectedCluster.value) return
  try {
    if (kind === 'daemonset' || selectedKind.value === 'daemonsets') {
      await k8sApi.restartDaemonSet(selectedCluster.value, name, ns)
    } else {
      await k8sApi.restartDeployment(selectedCluster.value, name, ns)
    }
    showToast(`Rolling restart initiated for ${resource.kind}/${name}`)
    await fetchResources()
    if (selectedResource.value && selectedResource.value.metadata?.name === name) {
      selectedResource.value = await k8sApi.getResource(selectedCluster.value, selectedKind.value, name, ns)
    }
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to restart workload', 'error')
  }
}

async function handleTriggerCronJob(resource: K8sResource) {
  const name = resource.metadata?.name || ''
  const ns = resource.metadata?.namespace || 'default'
  if (!name || !selectedCluster.value) return
  try {
    await k8sApi.triggerCronJob(selectedCluster.value, name, ns)
    showToast(`Triggered execution for CronJob ${name}`)
    await fetchResources()
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to trigger CronJob', 'error')
  }
}

async function handleToggleSuspendCronJob(resource: K8sResource) {
  const name = resource.metadata?.name || ''
  const ns = resource.metadata?.namespace || 'default'
  const currentSuspended = Boolean((resource.spec as { suspend?: boolean })?.suspend)
  if (!name || !selectedCluster.value) return
  try {
    await k8sApi.suspendCronJob(selectedCluster.value, name, ns, !currentSuspended)
    showToast(`CronJob ${name} ${currentSuspended ? 'resumed' : 'suspended'} successfully`)
    await fetchResources()
    if (selectedResource.value && selectedResource.value.metadata?.name === name) {
      selectedResource.value = await k8sApi.getResource(selectedCluster.value, 'cronjobs', name, ns)
    }
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to update CronJob status', 'error')
  }
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
    showToast(err instanceof Error ? err.message : 'Failed to delete resource', 'error')
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
    newNsError.value = err instanceof Error ? err.message : 'Failed to create namespace'
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
    setTimeout(() => { drawerYamlCopied.value = false }, 2000)
  } catch {
    // fallback
  }
}

function copyText(text: string) {
  navigator.clipboard.writeText(text)
  showToast('Copied to clipboard!')
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
</script>


<template>
  <div class="explorer-layout animate-fade-in">
    <!-- Left Sidebar -->
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
          <option value="all">🌐 All Namespaces</option>
          <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">
            📁 {{ ns.name }}
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
            <span>KUBERNETES RESOURCE EXPLORER</span>
          </div>
          <h1 class="view-title">{{ currentKindLabel }}</h1>
          <div class="breadcrumbs font-mono">
            <span class="crumb-cluster">🌐 {{ selectedCluster }}</span>
            <span class="crumb-sep">/</span>
            <span class="crumb-ns">📁 {{ selectedNamespace === 'all' ? 'All Namespaces' : selectedNamespace }}</span>
            <span class="crumb-sep">/</span>
            <span class="crumb-kind text-cyan">{{ currentKindLabel }}</span>
          </div>
        </div>

        <div class="header-actions">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="fetchResources">
            <span>{{ loading ? '⏳ Refreshing...' : '🔄 Refresh' }}</span>
          </button>
          <button type="button" class="btn btn-secondary" @click="openApplyYamlModal">
            <span>📄 Apply YAML</span>
          </button>
          <button type="button" class="btn btn-primary" @click="openCreateModal">
            <span>✨ + Create {{ currentKindLabel.slice(0, -1) || 'Resource' }}</span>
          </button>
        </div>
      </div>

      <!-- Notification Toast -->
      <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
        <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
        <span>{{ toastMessage.text }}</span>
        <button class="toast-close" @click="toastMessage = null">✕</button>
      </div>

      <!-- Offline Banner -->
      <div v-if="clusterOffline" class="offline-banner animate-fade-in">
        <span class="offline-icon">⚠️</span>
        <div class="offline-content">
          <strong class="offline-title">Kubernetes Cluster Disconnected</strong>
          <p class="offline-desc">
            No Kubernetes control plane is currently attached to '{{ selectedCluster || 'primary-cluster' }}'. Import a Kubeconfig or manage Docker Swarm workloads on /deployments.
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
          icon="📦"
          badge="DISCOVERED"
          badge-color="cyan"
        />
        <MetricCard
          title="Active Namespaces"
          :value="activeNamespacesCount"
          subtitle="Available workload domains"
          icon="📁"
          badge="TENANCY"
          badge-color="emerald"
        />
        <MetricCard
          title="Cluster Target"
          :value="selectedCluster"
          subtitle="Kubernetes Control Plane"
          icon="🌐"
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
                {{ selectedKind === 'pods' ? '🫛' : selectedKind === 'nodes' ? '🖥️' : selectedKind === 'deployments' ? '🚀' : selectedKind === 'services' ? '🔌' : selectedKind === 'configmaps' ? '🗺️' : selectedKind === 'secrets' ? '🔒' : '📦' }}
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
              :status="selectedKind === 'pods' ? getPodPhase(row) : selectedKind === 'nodes' ? getNodeStatus(row) : selectedKind === 'persistentvolumeclaims' ? getPvcStatus(row) : selectedKind === 'persistentvolumes' ? getPvStatus(row) : selectedKind === 'jobs' ? getJobStatus(row) : getResourceStatus(row)" 
              size="sm" 
            />
          </template>

          <template #cell-ready="{ row }">
            <span class="font-mono ready-cell">
              {{ selectedKind === 'daemonsets' ? getDaemonSetReady(row) : getPodReadyCount(row) }}
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

          <!-- Deployment & StatefulSet Cells -->
          <template #cell-replicas="{ row }">
            <span class="font-mono badge-replicas">
              {{ selectedKind === 'statefulsets' ? getStatefulSetReplicas(row) : selectedKind === 'horizontalpodautoscalers' ? getHpaReplicas(row) : getDeploymentReplicas(row) }}
            </span>
          </template>

          <template #cell-image="{ row }">
            <span class="font-mono text-cyan font-small cell-image-text" :title="selectedKind === 'statefulsets' ? getStatefulSetImage(row) : getDeploymentImage(row)">
              {{ selectedKind === 'statefulsets' ? getStatefulSetImage(row) : getDeploymentImage(row) }}
            </span>
          </template>

          <template #cell-selector="{ row }">
            <span class="font-mono text-muted font-small">{{ getDeploymentSelector(row) }}</span>
          </template>

          <!-- DaemonSet Cells -->
          <template #cell-desired="{ row }">
            <span class="font-mono">{{ getDaemonSetDesired(row) }}</span>
          </template>

          <template #cell-current="{ row }">
            <span class="font-mono">{{ getDaemonSetCurrent(row) }}</span>
          </template>

          <!-- Job & CronJob Cells -->
          <template #cell-completions="{ row }">
            <span class="font-mono">{{ getJobCompletions(row) }}</span>
          </template>

          <template #cell-duration="{ row }">
            <span class="font-mono text-muted">{{ getJobDuration(row) }}</span>
          </template>

          <template #cell-schedule="{ row }">
            <span class="font-mono schedule-badge">{{ getCronJobSchedule(row) }}</span>
          </template>

          <template #cell-suspend="{ row }">
            <span class="font-mono" :class="getCronJobSuspend(row) === 'True' ? 'text-amber' : 'text-emerald'">
              {{ getCronJobSuspend(row) }}
            </span>
          </template>

          <template #cell-active="{ row }">
            <span class="font-mono">{{ getCronJobActive(row) }}</span>
          </template>

          <template #cell-lastSchedule="{ row }">
            <span class="font-mono text-muted">{{ getCronJobLastSchedule(row) }}</span>
          </template>

          <!-- Service & Ingress Cells -->
          <template #cell-type="{ row }">
            <span class="font-mono text-violet font-small font-bold">
              {{ selectedKind === 'services' ? getServiceType(row) : selectedKind === 'secrets' ? getSecretType(row) : getEventType(row) }}
            </span>
          </template>

          <template #cell-clusterIP="{ row }">
            <span class="font-mono text-muted font-small">{{ getServiceClusterIP(row) }}</span>
          </template>

          <template #cell-externalIP="{ row }">
            <span class="font-mono text-cyan font-small">{{ getServiceExternalIP(row) }}</span>
          </template>

          <template #cell-ports="{ row }">
            <span class="font-mono text-emerald font-small">{{ getServicePorts(row) }}</span>
          </template>

          <template #cell-hosts="{ row }">
            <span class="font-mono text-cyan font-small">{{ getIngressHosts(row) }}</span>
          </template>

          <template #cell-paths="{ row }">
            <span class="font-mono text-muted font-small">{{ getIngressPaths(row) }}</span>
          </template>

          <!-- ConfigMap & Secret Cells -->
          <template #cell-keysCount="{ row }">
            <span class="font-mono badge-keys">
              {{ selectedKind === 'secrets' ? getSecretKeysCount(row) : getConfigMapKeysCount(row) }} keys
            </span>
          </template>

          <template #cell-dataPreview="{ row }">
            <span class="font-mono text-muted font-small">{{ getConfigMapDataPreview(row) }}</span>
          </template>

          <!-- Storage Cells -->
          <template #cell-capacity="{ row }">
            <span class="font-mono text-cyan font-bold">{{ selectedKind === 'persistentvolumes' ? getPvCapacity(row) : getPvcCapacity(row) }}</span>
          </template>

          <template #cell-accessModes="{ row }">
            <span class="font-mono text-muted font-small">{{ selectedKind === 'persistentvolumes' ? getPvAccessModes(row) : getPvcAccessModes(row) }}</span>
          </template>

          <template #cell-storageClass="{ row }">
            <span class="font-mono text-violet font-small">{{ selectedKind === 'persistentvolumes' ? getPvStorageClass(row) : getPvcStorageClass(row) }}</span>
          </template>

          <template #cell-volume="{ row }">
            <span class="font-mono text-muted font-small">{{ getPvcVolume(row) }}</span>
          </template>

          <template #cell-reclaimPolicy="{ row }">
            <span class="font-mono font-small">{{ selectedKind === 'storageclasses' ? getScReclaimPolicy(row) : getPvReclaimPolicy(row) }}</span>
          </template>

          <template #cell-claim="{ row }">
            <span class="font-mono text-muted font-small">{{ getPvClaim(row) }}</span>
          </template>

          <template #cell-provisioner="{ row }">
            <span class="font-mono text-cyan font-small">{{ getScProvisioner(row) }}</span>
          </template>

          <template #cell-volumeBindingMode="{ row }">
            <span class="font-mono text-muted font-small">{{ getScVolumeBindingMode(row) }}</span>
          </template>

          <template #cell-defaultClass="{ row }">
            <span class="font-mono font-bold" :class="getScDefault(row) === 'Yes' ? 'text-emerald' : 'text-muted'">{{ getScDefault(row) }}</span>
          </template>

          <!-- Security & Networking Cells -->
          <template #cell-podSelector="{ row }">
            <span class="font-mono text-muted font-small">{{ getNetPolPodSelector(row) }}</span>
          </template>

          <template #cell-policyTypes="{ row }">
            <span class="font-mono text-violet font-small">{{ getNetPolPolicyTypes(row) }}</span>
          </template>

          <template #cell-secretsCount="{ row }">
            <span class="font-mono font-small">{{ getSaSecrets(row) }} secrets</span>
          </template>

          <!-- HPA Cells -->
          <template #cell-reference="{ row }">
            <span class="font-mono text-cyan font-small">{{ getHpaReference(row) }}</span>
          </template>

          <template #cell-targets="{ row }">
            <span class="font-mono text-emerald font-small">{{ getHpaTargets(row) }}</span>
          </template>

          <template #cell-minMax="{ row }">
            <span class="font-mono text-muted font-small">{{ getHpaMinMax(row) }}</span>
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

          <!-- Actions Slot for All Kinds -->
          <template #cell-actions="{ row }">
            <div class="action-buttons">
              <!-- Pod Actions -->
              <template v-if="selectedKind === 'pods'">
                <button type="button" class="btn btn-secondary btn-xs" title="Details" @click="openDetailDrawer(row)">
                  <span>📊 Details</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="YAML" @click="openResourceYaml(row)">
                  <span>📄 YAML</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Logs" @click="openPodLogs(row)">
                  <span>📜 Logs</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Terminal" @click="openPodTerminal(row)">
                  <span>🖥️</span>
                </button>
                <button type="button" class="btn btn-danger btn-xs" title="Delete Pod" @click="promptDelete(row)">
                  <span>🗑️</span>
                </button>
              </template>

              <!-- Node Actions -->
              <template v-else-if="selectedKind === 'nodes'">
                <button type="button" class="btn btn-secondary btn-xs" title="Details" @click="openDetailDrawer(row)">
                  <span>📊 Details</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="YAML" @click="openResourceYaml(row)">
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

              <!-- Workload Action: Deployments -->
              <template v-else-if="selectedKind === 'deployments'">
                <button type="button" class="btn btn-primary btn-xs" title="Scale Deployment Replicas" @click="openScaleModal(row)">
                  <span>⚡ Scale</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Rolling Restart" @click="handleRestartWorkload(row)">
                  <span>🔄 Restart</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Details" @click="openDetailDrawer(row)">
                  <span>📊</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Edit YAML" @click="openResourceYaml(row)">
                  <span>📄</span>
                </button>
                <button type="button" class="btn btn-danger btn-xs" title="Delete" @click="promptDelete(row)">
                  <span>🗑️</span>
                </button>
              </template>

              <!-- Workload Action: StatefulSets -->
              <template v-else-if="selectedKind === 'statefulsets'">
                <button type="button" class="btn btn-primary btn-xs" title="Scale StatefulSet Replicas" @click="openScaleModal(row)">
                  <span>⚡ Scale</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Details" @click="openDetailDrawer(row)">
                  <span>📊</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Edit YAML" @click="openResourceYaml(row)">
                  <span>📄</span>
                </button>
                <button type="button" class="btn btn-danger btn-xs" title="Delete" @click="promptDelete(row)">
                  <span>🗑️</span>
                </button>
              </template>

              <!-- Workload Action: DaemonSets -->
              <template v-else-if="selectedKind === 'daemonsets'">
                <button type="button" class="btn btn-secondary btn-xs" title="Rolling Restart" @click="handleRestartWorkload(row)">
                  <span>🔄 Restart</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Details" @click="openDetailDrawer(row)">
                  <span>📊</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Edit YAML" @click="openResourceYaml(row)">
                  <span>📄</span>
                </button>
                <button type="button" class="btn btn-danger btn-xs" title="Delete" @click="promptDelete(row)">
                  <span>🗑️</span>
                </button>
              </template>

              <!-- Workload Action: CronJobs -->
              <template v-else-if="selectedKind === 'cronjobs'">
                <button type="button" class="btn btn-primary btn-xs" title="Trigger Job Now" @click="handleTriggerCronJob(row)">
                  <span>⚡ Trigger</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" :title="row.spec?.suspend ? 'Resume CronJob' : 'Suspend CronJob'" @click="handleToggleSuspendCronJob(row)">
                  <span>{{ row.spec?.suspend ? '▶ Resume' : '⏸ Suspend' }}</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Details" @click="openDetailDrawer(row)">
                  <span>📊</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="YAML" @click="openResourceYaml(row)">
                  <span>📄</span>
                </button>
                <button type="button" class="btn btn-danger btn-xs" title="Delete" @click="promptDelete(row)">
                  <span>🗑️</span>
                </button>
              </template>

              <!-- Default Actions for All other resources -->
              <template v-else>
                <button type="button" class="btn btn-secondary btn-xs" title="Details" @click="openDetailDrawer(row)">
                  <span>📊 Details</span>
                </button>
                <button type="button" class="btn btn-secondary btn-xs" title="Edit / View YAML" @click="openResourceYaml(row)">
                  <span>📄 YAML</span>
                </button>
                <button type="button" class="btn btn-danger btn-xs" title="Delete Resource" @click="promptDelete(row)">
                  <span>🗑️</span>
                </button>
              </template>
            </div>
          </template>
        </DataTable>
      </div>
    </main>

    <!-- Workload Action: Scale Modal -->
    <ModalDrawer
      :show="showScaleModal"
      mode="modal"
      :title="`Scale ${scaleTarget?.kind || 'Workload'}`"
      :subtitle="`Resource: ${scaleTarget?.metadata?.name || ''} (${scaleTarget?.metadata?.namespace || 'default'})`"
      max-width="480px"
      @close="showScaleModal = false"
    >
      <div class="scale-modal-body">
        <p class="text-muted font-small">
          Adjust the desired number of pod replicas. The Kubernetes controller will automatically adjust pods.
        </p>

        <div class="scale-stepper-wrap">
          <label class="form-label">Desired Replicas</label>
          <div class="stepper-input-group">
            <button 
              type="button" 
              class="btn-stepper"
              :disabled="scaleReplicasCount <= 0"
              @click="scaleReplicasCount = Math.max(0, scaleReplicasCount - 1)"
            >
              -
            </button>
            <input 
              v-model.number="scaleReplicasCount" 
              type="number" 
              min="0" 
              max="100" 
              class="input-glass font-mono scale-num-input" 
            />
            <button 
              type="button" 
              class="btn-stepper"
              @click="scaleReplicasCount = scaleReplicasCount + 1"
            >
              +
            </button>
          </div>
          <input 
            v-model.number="scaleReplicasCount" 
            type="range" 
            min="0" 
            max="20" 
            class="scale-range-slider" 
          />
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="scalingResource" @click="showScaleModal = false">
          Cancel
        </button>
        <button 
          type="button" 
          class="btn btn-primary"
          :disabled="scalingResource"
          @click="handleScaleConfirm"
        >
          <span>{{ scalingResource ? '⏳ Scaling...' : '⚡ Apply Scale' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Node Drain Modal -->
    <ModalDrawer
      :show="showDrainModal"
      mode="modal"
      title="Drain Kubernetes Node"
      :subtitle="`Safely evict all pods from ${drainTargetNode?.metadata?.name || ''}`"
      max-width="540px"
      @close="showDrainModal = false"
    >
      <div class="drain-modal-body">
        <p class="text-muted font-small">
          Draining will cordon the node and evict all pods. Configure eviction parameters:
        </p>

        <div class="form-group">
          <label class="form-label">Grace Period (seconds)</label>
          <input 
            v-model.number="drainOptions.gracePeriodSeconds" 
            type="number" 
            class="input-glass font-mono" 
            min="0" 
            max="3600" 
          />
        </div>

        <div class="checkbox-group">
          <label class="checkbox-label">
            <input v-model="drainOptions.ignoreDaemonSets" type="checkbox" />
            <span>Ignore DaemonSet-managed pods (Recommended)</span>
          </label>

          <label class="checkbox-label">
            <input v-model="drainOptions.deleteEmptyDirData" type="checkbox" />
            <span>Delete local data in emptyDir volumes</span>
          </label>

          <label class="checkbox-label">
            <input v-model="drainOptions.force" type="checkbox" />
            <span>Force eviction (continue if pods are unmanaged)</span>
          </label>
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="drainingNode" @click="showDrainModal = false">
          Cancel
        </button>
        <button 
          type="button" 
          class="btn btn-danger"
          :disabled="drainingNode"
          @click="handleDrainNodeConfirm"
        >
          <span>{{ drainingNode ? '⏳ Draining...' : '🧹 Confirm Drain' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Create Namespace Modal -->
    <ModalDrawer
      :show="showNewNsModal"
      mode="modal"
      title="Create Kubernetes Namespace"
      :subtitle="`Cluster: ${selectedCluster}`"
      max-width="460px"
      @close="showNewNsModal = false"
    >
      <div class="form-group">
        <label class="form-label required">Namespace Name</label>
        <input 
          v-model="newNsName"
          type="text" 
          class="input-glass font-mono" 
          placeholder="e.g. staging, payment-service" 
          required 
          @keydown.enter="handleCreateNamespace"
        />
        <span v-if="newNsError" class="text-rose text-xs">{{ newNsError }}</span>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="creatingNs" @click="showNewNsModal = false">
          Cancel
        </button>
        <button 
          type="button" 
          class="btn btn-primary"
          :disabled="creatingNs || !newNsName.trim()"
          @click="handleCreateNamespace"
        >
          <span>{{ creatingNs ? '⏳ Creating...' : '✨ Create Namespace' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Delete Confirmation Modal -->
    <ModalDrawer
      :show="showDeleteModal"
      mode="modal"
      :title="`Delete ${resourceToDelete?.kind || 'Resource'}`"
      :subtitle="`Cluster: ${selectedCluster}`"
      max-width="500px"
      @close="showDeleteModal = false"
    >
      <div class="delete-modal-content">
        <div class="delete-warning-icon">⚠️</div>
        <p class="delete-msg">
          Are you sure you want to permanently delete
          <strong class="text-white font-mono">{{ resourceToDelete?.kind }}/{{ resourceToDelete?.metadata?.name }}</strong>
          <span v-if="resourceToDelete?.metadata?.namespace">
            in namespace <strong class="text-cyan font-mono">{{ resourceToDelete.metadata.namespace }}</strong>
          </span>?
        </p>
        <p class="text-muted font-small">
          This action cannot be undone. Any active workloads or bound resources may be terminated immediately.
        </p>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="deletingResource" @click="showDeleteModal = false">
          Cancel
        </button>
        <button 
          type="button" 
          class="btn btn-danger"
          :disabled="deletingResource"
          @click="handleDeleteConfirmed"
        >
          <span>{{ deletingResource ? '⏳ Deleting...' : '🗑️ Delete Permanently' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Pod Logs Modal -->
    <ModalDrawer
      :show="showLogsModal"
      mode="modal"
      :title="`Pod Logs: ${logsPod?.metadata?.name || ''}`"
      :subtitle="`Namespace: ${logsPod?.metadata?.namespace || 'default'} • Cluster: ${selectedCluster}`"
      max-width="960px"
      @close="showLogsModal = false"
    >
      <div v-if="logsPod" class="pod-logs-wrapper">
        <div v-if="getPodContainerNames(logsPod).length > 1" class="container-select-bar">
          <label class="form-label">Select Container:</label>
          <div class="container-tabs">
            <button
              v-for="cName in getPodContainerNames(logsPod)"
              :key="cName"
              type="button"
              class="container-tab-btn"
              :class="{ 'is-active': selectedPodContainer === cName }"
              @click="selectedPodContainer = cName"
            >
              {{ cName }}
            </button>
          </div>
        </div>

        <PodLogViewer
          :cluster="selectedCluster"
          :namespace="logsPod.metadata?.namespace || 'default'"
          :pod="logsPod.metadata?.name || ''"
          :container="selectedPodContainer"
        />
      </div>
    </ModalDrawer>

    <!-- Pod Terminal Modal -->
    <ModalDrawer
      :show="showTerminalModal"
      mode="modal"
      :title="`Terminal Exec: ${terminalPod?.metadata?.name || ''}`"
      :subtitle="`Namespace: ${terminalPod?.metadata?.namespace || 'default'} • Cluster: ${selectedCluster}`"
      max-width="1000px"
      @close="showTerminalModal = false"
    >
      <div v-if="terminalPod" class="pod-terminal-wrapper">
        <div v-if="getPodContainerNames(terminalPod).length > 1" class="container-select-bar">
          <label class="form-label">Select Container:</label>
          <div class="container-tabs">
            <button
              v-for="cName in getPodContainerNames(terminalPod)"
              :key="cName"
              type="button"
              class="container-tab-btn"
              :class="{ 'is-active': selectedPodContainer === cName }"
              @click="selectedPodContainer = cName"
            >
              {{ cName }}
            </button>
          </div>
        </div>

        <PodTerminal
          :cluster="selectedCluster"
          :namespace="terminalPod.metadata?.namespace || 'default'"
          :pod="terminalPod.metadata?.name || ''"
          :container="selectedPodContainer"
        />
      </div>
    </ModalDrawer>

    <!-- Import Cluster Modal -->
    <ModalDrawer
      :show="showImportModal"
      mode="modal"
      title="Import Kubernetes Cluster"
      subtitle="Connect an external Kubernetes cluster via Kubeconfig"
      max-width="620px"
      @close="showImportModal = false"
    >
      <div class="import-modal-body">
        <div class="form-group">
          <label class="form-label required">Cluster Name / Identifier</label>
          <input 
            v-model="importForm.name"
            type="text" 
            class="input-glass font-mono" 
            placeholder="e.g. production-k8s, staging-eks" 
            required 
          />
        </div>

        <div class="form-row-2">
          <div class="form-group flex-1">
            <label class="form-label">Group</label>
            <input 
              v-model="importForm.group"
              type="text" 
              class="input-glass font-mono" 
              placeholder="production" 
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Region</label>
            <input 
              v-model="importForm.region"
              type="text" 
              class="input-glass font-mono" 
              placeholder="us-east-1" 
            />
          </div>
        </div>

        <div class="import-mode-toggle">
          <button 
            type="button" 
            class="toggle-btn"
            :class="{ 'is-active': importMode === 'file' }"
            @click="importMode = 'file'"
          >
            📁 Upload Kubeconfig File
          </button>
          <button 
            type="button" 
            class="toggle-btn"
            :class="{ 'is-active': importMode === 'text' }"
            @click="importMode = 'text'"
          >
            📝 Paste Kubeconfig Text
          </button>
        </div>

        <div v-if="importMode === 'file'" class="form-group">
          <label class="form-label required">Kubeconfig YAML File</label>
          <input 
            type="file" 
            accept=".yaml,.yml,.config,text/plain" 
            class="input-glass file-input font-mono"
            @change="handleFileChange"
          />
        </div>

        <div v-else class="form-group">
          <label class="form-label required">Kubeconfig Raw YAML</label>
          <textarea
            v-model="importForm.kubeconfigRaw"
            class="input-glass font-mono text-area-lg"
            rows="8"
            placeholder="apiVersion: v1&#10;clusters:&#10;  - cluster:&#10;      ..."
          ></textarea>
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="importingCluster" @click="showImportModal = false">
          Cancel
        </button>
        <button 
          type="button" 
          class="btn btn-primary"
          :disabled="importingCluster || !importForm.name.trim()"
          @click="handleImportCluster"
        >
          <span>{{ importingCluster ? '⏳ Importing Cluster...' : '✨ Import Cluster' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Create Resource Modal -->
    <CreateResourceModal
      :show="showCreateModal"
      :cluster="selectedCluster"
      :namespaces="namespaces"
      :default-namespace="selectedNamespace !== 'all' ? selectedNamespace : 'default'"
      :default-kind="selectedKind"
      @close="showCreateModal = false"
      @created="onResourceCreated"
    />

    <!-- YAML Editor Modal -->
    <YamlEditorModal
      :show="showYamlModal"
      :cluster="selectedCluster"
      :title="yamlEditorTitle"
      :mode="yamlEditorMode"
      :initial-yaml="yamlEditorInitialContent"
      @close="showYamlModal = false"
      @applied="onYamlApplied"
    />

    <!-- Detail Drawer -->
    <ModalDrawer
      :show="showDetailDrawer"
      mode="drawer"
      :title="`${selectedResource?.kind || 'Resource'}: ${selectedResource?.metadata?.name || ''}`"
      :subtitle="`Namespace: ${selectedResource?.metadata?.namespace || 'cluster-scoped'} • Cluster: ${selectedCluster}`"
      max-width="760px"
      @close="showDetailDrawer = false"
    >
      <div v-if="selectedResource" class="detail-drawer-content">
        <!-- Tabs Navigation -->
        <div class="drawer-tabs-bar">
          <button 
            type="button" 
            class="drawer-tab-btn" 
            :class="{ 'is-active': activeDrawerTab === 'overview' }"
            @click="activeDrawerTab = 'overview'"
          >
            📊 Structured Overview
          </button>
          <button 
            v-if="selectedResource.kind === 'Pod' || selectedKind === 'pods'"
            type="button" 
            class="drawer-tab-btn" 
            :class="{ 'is-active': activeDrawerTab === 'events' }"
            @click="activeDrawerTab = 'events'"
          >
            📢 Events
          </button>
          <button 
            type="button" 
            class="drawer-tab-btn" 
            :class="{ 'is-active': activeDrawerTab === 'yaml' }"
            @click="activeDrawerTab = 'yaml'"
          >
            📄 Live YAML Manifest
          </button>
        </div>

        <!-- Tab 1: Structured Overview -->
        <div v-if="activeDrawerTab === 'overview'" class="tab-overview-content animate-fade-in">
          <!-- Metadata Summary Card -->
          <div class="info-card glass-panel">
            <h4 class="card-title">Metadata</h4>
            <div class="meta-grid font-mono">
              <div class="meta-item"><span class="meta-label">Name:</span><span class="meta-val text-white font-bold">{{ selectedResource.metadata?.name }}</span></div>
              <div class="meta-item"><span class="meta-label">Namespace:</span><span class="meta-val text-cyan">{{ selectedResource.metadata?.namespace || 'cluster-scoped' }}</span></div>
              <div class="meta-item"><span class="meta-label">Created:</span><span class="meta-val text-muted">{{ selectedResource.metadata?.creationTimestamp || 'N/A' }} ({{ getResourceAge(selectedResource) }})</span></div>
              <div class="meta-item"><span class="meta-label">UID:</span><span class="meta-val text-muted font-xs">{{ selectedResource.metadata?.uid || 'N/A' }}</span></div>
            </div>

            <!-- Labels -->
            <div v-if="selectedResource.metadata?.labels && Object.keys(selectedResource.metadata.labels).length > 0" class="labels-section">
              <span class="meta-label">Labels:</span>
              <div class="tag-chips-wrap">
                <span v-for="(val, key) in selectedResource.metadata.labels" :key="key" class="tag-chip font-mono">
                  <span class="tag-key">{{ key }}</span>: <span class="tag-val">{{ val }}</span>
                </span>
              </div>
            </div>
          </div>

          <!-- Pod Specific Details -->
          <template v-if="selectedResource.kind === 'Pod' || selectedKind === 'pods'">
            <div class="info-card glass-panel">
              <div class="card-title-row">
                <h4 class="card-title">Pod Execution Status</h4>
                <StatusBadge :status="getPodPhase(selectedResource)" size="sm" />
              </div>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Node:</span><span class="meta-val text-cyan">{{ getPodNode(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Pod IP:</span><span class="meta-val text-white">{{ getPodIP(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Host IP:</span><span class="meta-val text-muted">{{ getHostIP(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">QoS Class:</span><span class="meta-val text-violet">{{ getPodQoS(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Service Account:</span><span class="meta-val text-muted">{{ getPodServiceAccount(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Ready:</span><span class="meta-val text-emerald font-bold">{{ getPodReadyCount(selectedResource) }}</span></div>
              </div>
            </div>

            <!-- Containers List -->
            <div class="info-card glass-panel">
              <h4 class="card-title">Containers ({{ getPodContainers(selectedResource).length }})</h4>
              <div class="containers-list">
                <div v-for="c in getPodContainers(selectedResource)" :key="c.name" class="container-card">
                  <div class="c-header">
                    <div class="c-title font-mono">
                      <span class="c-dot" :class="`dot-${c.stateType}`"></span>
                      <strong>{{ c.name }}</strong>
                    </div>
                    <span class="c-state-badge font-mono" :class="`state-${c.stateType}`">{{ c.state }}</span>
                  </div>
                  <div class="c-meta font-mono font-xs">
                    <div class="c-meta-row"><span class="text-muted">Image:</span> <span class="text-cyan">{{ c.image }}</span></div>
                    <div class="c-meta-row"><span class="text-muted">Ports:</span> <span class="text-white">{{ c.ports }}</span></div>
                    <div class="c-meta-row"><span class="text-muted">Restarts:</span> <span :class="{ 'text-rose': c.restarts > 0 }">{{ c.restarts }}</span></div>
                  </div>
                </div>
              </div>
            </div>
          </template>

          <!-- Deployment Specific Details -->
          <template v-else-if="selectedResource.kind === 'Deployment' || selectedKind === 'deployments'">
            <div class="info-card glass-panel">
              <div class="card-title-row">
                <h4 class="card-title">Deployment Replicas & Status</h4>
                <div class="card-actions-row">
                  <button type="button" class="btn btn-primary btn-xs" @click="openScaleModal(selectedResource)">
                    ⚡ Scale Replicas
                  </button>
                  <button type="button" class="btn btn-secondary btn-xs" @click="handleRestartWorkload(selectedResource)">
                    🔄 Rolling Restart
                  </button>
                </div>
              </div>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Replicas:</span><span class="meta-val text-cyan font-bold">{{ getDeploymentReplicas(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Container Image:</span><span class="meta-val text-white">{{ getDeploymentImage(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Pod Selector:</span><span class="meta-val text-muted">{{ getDeploymentSelector(selectedResource) }}</span></div>
              </div>
            </div>
          </template>

          <!-- StatefulSet Specific Details -->
          <template v-else-if="selectedResource.kind === 'StatefulSet' || selectedKind === 'statefulsets'">
            <div class="info-card glass-panel">
              <div class="card-title-row">
                <h4 class="card-title">StatefulSet Status</h4>
                <div class="card-actions-row">
                  <button type="button" class="btn btn-primary btn-xs" @click="openScaleModal(selectedResource)">
                    ⚡ Scale Replicas
                  </button>
                </div>
              </div>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Replicas:</span><span class="meta-val text-cyan font-bold">{{ getStatefulSetReplicas(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Container Image:</span><span class="meta-val text-white">{{ getStatefulSetImage(selectedResource) }}</span></div>
              </div>
            </div>
          </template>

          <!-- DaemonSet Specific Details -->
          <template v-else-if="selectedResource.kind === 'DaemonSet' || selectedKind === 'daemonsets'">
            <div class="info-card glass-panel">
              <div class="card-title-row">
                <h4 class="card-title">DaemonSet Status</h4>
                <button type="button" class="btn btn-secondary btn-xs" @click="handleRestartWorkload(selectedResource)">
                  🔄 Rolling Restart
                </button>
              </div>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Desired Scheduled:</span><span class="meta-val text-white">{{ getDaemonSetDesired(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Current Scheduled:</span><span class="meta-val text-cyan">{{ getDaemonSetCurrent(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Number Ready:</span><span class="meta-val text-emerald font-bold">{{ getDaemonSetReady(selectedResource) }}</span></div>
              </div>
            </div>
          </template>

          <!-- Service Specific Details -->
          <template v-else-if="selectedResource.kind === 'Service' || selectedKind === 'services'">
            <div class="info-card glass-panel">
              <h4 class="card-title">Service Networking</h4>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Type:</span><span class="meta-val text-violet font-bold">{{ getServiceType(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Cluster IP:</span><span class="meta-val text-white">{{ getServiceClusterIP(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">External IP:</span><span class="meta-val text-cyan">{{ getServiceExternalIP(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Port Mappings:</span><span class="meta-val text-emerald">{{ getServicePorts(selectedResource) }}</span></div>
              </div>
            </div>
          </template>

          <!-- Ingress Specific Details -->
          <template v-else-if="selectedResource.kind === 'Ingress' || selectedKind === 'ingresses'">
            <div class="info-card glass-panel">
              <h4 class="card-title">Ingress Routing Rules</h4>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Host Domains:</span><span class="meta-val text-cyan">{{ getIngressHosts(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Path Prefix:</span><span class="meta-val text-white">{{ getIngressPaths(selectedResource) }}</span></div>
              </div>
            </div>
          </template>

          <!-- ConfigMap Specific Details with Live Editor -->
          <template v-else-if="selectedResource.kind === 'ConfigMap' || selectedKind === 'configmaps'">
            <div class="info-card glass-panel">
              <div class="card-title-row">
                <h4 class="card-title">ConfigMap Key-Value Entries ({{ Object.keys(configMapDataCopy).length }})</h4>
                <button 
                  type="button" 
                  class="btn btn-primary btn-xs"
                  :disabled="savingConfigMap"
                  @click="handleSaveConfigMap(selectedResource)"
                >
                  <span>{{ savingConfigMap ? '⏳ Saving...' : '💾 Save Changes' }}</span>
                </button>
              </div>

              <!-- Add Key-Value entry -->
              <div class="kv-add-row">
                <input v-model="newCmKey" type="text" class="input-glass font-mono flex-1" placeholder="New KEY_NAME" />
                <input v-model="newCmValue" type="text" class="input-glass font-mono flex-2" placeholder="Value" />
                <button type="button" class="btn btn-secondary btn-xs" @click="addCmEntry">
                  + Add
                </button>
              </div>

              <div class="kv-table-wrap">
                <div v-for="(val, k) in configMapDataCopy" :key="k" class="kv-entry-row">
                  <div class="kv-key-col font-mono">{{ k }}</div>
                  <input v-model="configMapDataCopy[k]" type="text" class="input-glass font-mono kv-val-input" />
                  <button type="button" class="btn-icon-xs" title="Copy value" @click="copyText(String(val))">
                    📋
                  </button>
                  <button type="button" class="btn-icon-xs btn-icon-danger" title="Delete key" @click="removeCmEntry(String(k))">
                    ✕
                  </button>
                </div>
              </div>
            </div>
          </template>

          <!-- Secret Specific Details -->
          <template v-else-if="selectedResource.kind === 'Secret' || selectedKind === 'secrets'">
            <SecretViewer
              :resource="selectedResource"
              :cluster="selectedCluster"
              :namespace="selectedResource.metadata?.namespace || 'default'"
            />
          </template>

          <!-- PVC Specific Details -->
          <template v-else-if="selectedResource.kind === 'PersistentVolumeClaim' || selectedKind === 'persistentvolumeclaims'">
            <div class="info-card glass-panel">
              <h4 class="card-title">Storage Claim Status</h4>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Status:</span><span class="meta-val text-emerald font-bold">{{ getPvcStatus(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Capacity:</span><span class="meta-val text-cyan font-bold">{{ getPvcCapacity(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Access Modes:</span><span class="meta-val text-white">{{ getPvcAccessModes(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">StorageClass:</span><span class="meta-val text-violet">{{ getPvcStorageClass(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Bound Volume:</span><span class="meta-val text-muted">{{ getPvcVolume(selectedResource) }}</span></div>
              </div>
            </div>
          </template>

          <!-- PV Specific Details -->
          <template v-else-if="selectedResource.kind === 'PersistentVolume' || selectedKind === 'persistentvolumes'">
            <div class="info-card glass-panel">
              <h4 class="card-title">Persistent Volume Details</h4>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Status:</span><span class="meta-val text-emerald font-bold">{{ getPvStatus(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Capacity:</span><span class="meta-val text-cyan font-bold">{{ getPvCapacity(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Access Modes:</span><span class="meta-val text-white">{{ getPvAccessModes(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Reclaim Policy:</span><span class="meta-val text-muted">{{ getPvReclaimPolicy(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Claim Reference:</span><span class="meta-val text-violet">{{ getPvClaim(selectedResource) }}</span></div>
              </div>
            </div>
          </template>

          <!-- CronJob Specific Details -->
          <template v-else-if="selectedResource.kind === 'CronJob' || selectedKind === 'cronjobs'">
            <div class="info-card glass-panel">
              <div class="card-title-row">
                <h4 class="card-title">CronJob Schedule & State</h4>
                <div class="card-actions-row">
                  <button type="button" class="btn btn-primary btn-xs" @click="handleTriggerCronJob(selectedResource)">
                    ⚡ Trigger Job Now
                  </button>
                  <button type="button" class="btn btn-secondary btn-xs" @click="handleToggleSuspendCronJob(selectedResource)">
                    {{ selectedResource.spec?.suspend ? '▶ Resume Schedule' : '⏸ Suspend Schedule' }}
                  </button>
                </div>
              </div>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Schedule Expression:</span><span class="meta-val text-cyan font-bold">{{ getCronJobSchedule(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Suspended:</span><span class="meta-val" :class="getCronJobSuspend(selectedResource) === 'True' ? 'text-amber' : 'text-emerald'">{{ getCronJobSuspend(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Active Jobs:</span><span class="meta-val text-white">{{ getCronJobActive(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Last Scheduled:</span><span class="meta-val text-muted">{{ getCronJobLastSchedule(selectedResource) }}</span></div>
              </div>
            </div>
          </template>

          <!-- Job Specific Details -->
          <template v-else-if="selectedResource.kind === 'Job' || selectedKind === 'jobs'">
            <div class="info-card glass-panel">
              <h4 class="card-title">Job Execution</h4>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Completions:</span><span class="meta-val text-emerald font-bold">{{ getJobCompletions(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Duration:</span><span class="meta-val text-cyan">{{ getJobDuration(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Execution Status:</span><span class="meta-val text-white">{{ getJobStatus(selectedResource) }}</span></div>
              </div>
            </div>
          </template>

          <!-- Node Specific Details -->
          <template v-else-if="selectedResource.kind === 'Node' || selectedKind === 'nodes'">
            <div class="info-card glass-panel">
              <div class="card-title-row">
                <h4 class="card-title">Node Capacity & System Info</h4>
                <div class="card-actions-row">
                  <button
                    v-if="isNodeUnschedulable(selectedResource)"
                    type="button"
                    class="btn btn-secondary btn-xs"
                    :disabled="operatingNode"
                    @click="handleUncordonNode(selectedResource)"
                  >
                    🔓 Uncordon Node
                  </button>
                  <button
                    v-else
                    type="button"
                    class="btn btn-secondary btn-xs"
                    :disabled="operatingNode"
                    @click="handleCordonNode(selectedResource)"
                  >
                    🔒 Cordon Node
                  </button>
                  <button
                    type="button"
                    class="btn btn-danger btn-xs"
                    :disabled="operatingNode || drainingNode"
                    @click="openDrainModal(selectedResource)"
                  >
                    🧹 Drain Node
                  </button>
                </div>
              </div>
              <div class="meta-grid font-mono">
                <div class="meta-item"><span class="meta-label">Kubelet Version:</span><span class="meta-val text-cyan">{{ getNodeVersion(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Internal IP:</span><span class="meta-val text-white">{{ getNodeInternalIP(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">OS / Architecture:</span><span class="meta-val text-muted">{{ getNodeOSArch(selectedResource) }}</span></div>
                <div class="meta-item"><span class="meta-label">Max Pods:</span><span class="meta-val text-emerald">{{ getNodePodsCount(selectedResource) }}</span></div>
              </div>
            </div>

            <!-- Taints Management -->
            <div class="info-card glass-panel">
              <h4 class="card-title">Node Taints ({{ getNodeTaints(selectedResource).length }})</h4>
              <div class="taints-add-row">
                <input v-model="newTaintKey" type="text" class="input-glass font-mono flex-1" placeholder="key (e.g. dedicated)" />
                <input v-model="newTaintValue" type="text" class="input-glass font-mono flex-1" placeholder="value (optional)" />
                <select v-model="newTaintEffect" class="input-glass font-mono">
                  <option value="NoSchedule">NoSchedule</option>
                  <option value="PreferNoSchedule">PreferNoSchedule</option>
                  <option value="NoExecute">NoExecute</option>
                </select>
                <button type="button" class="btn btn-primary btn-xs" :disabled="updatingTaints || !newTaintKey.trim()" @click="handleAddTaint(selectedResource)">
                  + Add Taint
                </button>
              </div>

              <div class="taints-list">
                <div v-for="(t, idx) in getNodeTaints(selectedResource)" :key="idx" class="taint-item font-mono">
                  <span class="text-white">{{ t.key }}</span>
                  <span v-if="t.value" class="text-muted">={{ t.value }}</span>
                  <span class="taint-effect-badge">{{ t.effect }}</span>
                  <button type="button" class="btn-icon-xs btn-icon-danger" :disabled="updatingTaints" @click="handleRemoveTaint(selectedResource, idx)">
                    ✕
                  </button>
                </div>
              </div>
            </div>

            <!-- Node Labels Editor -->
            <div class="info-card glass-panel">
              <div class="card-title-row">
                <h4 class="card-title">Node Labels</h4>
                <button type="button" class="btn btn-primary btn-xs" :disabled="updatingLabels" @click="handleSaveNodeLabels(selectedResource)">
                  <span>{{ updatingLabels ? '⏳ Saving...' : '💾 Save Labels' }}</span>
                </button>
              </div>
              <div class="taints-add-row">
                <input v-model="newLabelKey" type="text" class="input-glass font-mono flex-1" placeholder="label-key" />
                <input v-model="newLabelValue" type="text" class="input-glass font-mono flex-1" placeholder="label-value" />
                <button type="button" class="btn btn-secondary btn-xs" @click="addLabelToCopy">+ Add Label</button>
              </div>
              <div class="tag-chips-wrap">
                <span v-for="(val, k) in nodeLabelsCopy" :key="k" class="tag-chip font-mono">
                  <span class="tag-key">{{ k }}</span>: <span class="tag-val">{{ val }}</span>
                  <button type="button" class="btn-chip-remove" @click="removeLabelFromCopy(String(k))">✕</button>
                </span>
              </div>
            </div>
          </template>
        </div>

        <!-- Tab 2: Events Tab (Pods / Workloads) -->
        <div v-else-if="activeDrawerTab === 'events'" class="tab-events-content animate-fade-in">
          <EventsTimeline
            :cluster="selectedCluster"
            :namespace="selectedResource.metadata?.namespace || 'default'"
            :involved-object-name="selectedResource.metadata?.name"
            :auto-refresh="true"
          />
        </div>

        <!-- Tab 3: YAML Manifest -->
        <div v-else-if="activeDrawerTab === 'yaml'" class="tab-yaml-content animate-fade-in">
          <div class="yaml-actions-row">
            <button type="button" class="btn btn-secondary btn-xs" @click="copyDrawerYaml">
              <span>{{ drawerYamlCopied ? '✅ Copied!' : '📋 Copy YAML' }}</span>
            </button>
            <button type="button" class="btn btn-primary btn-xs" @click="openResourceYaml(selectedResource)">
              <span>✏️ Edit Manifest</span>
            </button>
          </div>
          <pre class="yaml-code-block font-mono"><code>{{ jsonToYaml(selectedResource) }}</code></pre>
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showDetailDrawer = false">
          Close
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>


<style scoped>
.explorer-layout {
  display: flex;
  min-height: calc(100vh - 64px);
  gap: 20px;
  padding: 20px;
  box-sizing: border-box;
}

.explorer-sidebar {
  width: 260px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px;
  height: fit-content;
  max-height: calc(100vh - 100px);
  overflow-y: auto;
}

.sidebar-header {
  display: flex;
  align-items: center;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.07);
}

.sidebar-brand {
  display: flex;
  align-items: center;
  gap: 8px;
}

.sidebar-title {
  font-weight: 700;
  font-size: 1rem;
  color: #fff;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.sidebar-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.section-heading-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.section-heading {
  font-size: 0.75rem;
  font-weight: 600;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.btn-icon-text {
  background: none;
  border: none;
  color: #00e5ff;
  font-size: 0.72rem;
  cursor: pointer;
  padding: 0;
}

.btn-icon-text:hover {
  text-decoration: underline;
}

.cluster-select, .ns-select {
  font-size: 0.85rem;
}

.sidebar-tree {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: 8px;
}

.tree-category {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.category-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.75rem;
  font-weight: 700;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding: 4px 6px;
}

.category-items {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tree-item-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  background: none;
  border: 1px solid transparent;
  border-radius: 6px;
  color: #94a3b8;
  cursor: pointer;
  text-align: left;
  font-size: 0.82rem;
  transition: all 0.15s ease;
}

.tree-item-btn:hover {
  background: rgba(255, 255, 255, 0.04);
  color: #f1f5f9;
}

.tree-item-btn.is-active {
  background: rgba(0, 229, 255, 0.12);
  border-color: rgba(0, 229, 255, 0.3);
  color: #00e5ff;
  font-weight: 600;
}

.item-icon {
  font-size: 0.9rem;
}

.explorer-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-width: 0;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 12px;
}

.header-left {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.7rem;
  font-weight: 700;
  color: #00e5ff;
  letter-spacing: 0.1em;
}

.view-title {
  font-size: 1.6rem;
  font-weight: 800;
  color: #fff;
  margin: 0;
}

.breadcrumbs {
  font-size: 0.78rem;
  color: #64748b;
  display: flex;
  align-items: center;
  gap: 6px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.table-box {
  padding: 16px;
}

.resource-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.res-link {
  color: #00e5ff;
  text-decoration: none;
  font-weight: 600;
}

.res-link:hover {
  text-decoration: underline;
}

.app-tag {
  font-size: 0.7rem;
  background: rgba(255, 255, 255, 0.06);
  padding: 2px 6px;
  border-radius: 4px;
  color: #94a3b8;
}

.ns-badge {
  background: rgba(139, 92, 246, 0.15);
  border: 1px solid rgba(139, 92, 246, 0.3);
  color: #c084fc;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.75rem;
}

.badge-replicas {
  color: #38bdf8;
  font-weight: 700;
}

.badge-keys {
  color: #fbbf24;
}

.schedule-badge {
  background: rgba(56, 189, 248, 0.12);
  color: #38bdf8;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.75rem;
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  flex-wrap: wrap;
}

.scale-stepper-wrap {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 12px;
}

.stepper-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-stepper {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: #fff;
  font-size: 1.2rem;
  cursor: pointer;
}

.scale-num-input {
  text-align: center;
  font-size: 1.1rem;
  font-weight: 700;
  width: 90px;
}

.scale-range-slider {
  width: 100%;
  accent-color: #00e5ff;
}

.detail-drawer-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.drawer-tabs-bar {
  display: flex;
  gap: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  padding-bottom: 8px;
}

.drawer-tab-btn {
  background: none;
  border: none;
  color: #94a3b8;
  font-size: 0.82rem;
  font-weight: 600;
  padding: 6px 12px;
  border-radius: 6px;
  cursor: pointer;
}

.drawer-tab-btn.is-active {
  background: rgba(0, 229, 255, 0.15);
  color: #00e5ff;
}

.tab-overview-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.info-card {
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.card-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-title {
  margin: 0;
  font-size: 0.88rem;
  font-weight: 700;
  color: #f1f5f9;
}

.meta-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 8px;
  font-size: 0.8rem;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.meta-label {
  font-size: 0.72rem;
  color: #64748b;
  text-transform: uppercase;
}

.tag-chips-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tag-chip {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.72rem;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.tag-key {
  color: #38bdf8;
}

.tag-val {
  color: #fff;
}

.btn-chip-remove {
  background: none;
  border: none;
  color: #fb7185;
  cursor: pointer;
  padding: 0;
  font-size: 0.7rem;
}

.containers-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.container-card {
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.06);
  padding: 10px;
  border-radius: 6px;
}

.c-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.c-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 6px;
}

.dot-running { background: #10b981; }
.dot-waiting { background: #f59e0b; }
.dot-terminated { background: #ef4444; }

.c-state-badge {
  font-size: 0.72rem;
  padding: 1px 6px;
  border-radius: 4px;
}

.state-running { background: rgba(16, 185, 129, 0.15); color: #34d399; }
.state-waiting { background: rgba(245, 158, 11, 0.15); color: #fbbf24; }
.state-terminated { background: rgba(239, 68, 68, 0.15); color: #f87171; }

.kv-add-row, .taints-add-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.kv-entry-row, .taint-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.kv-key-col {
  width: 140px;
  font-size: 0.8rem;
  color: #38bdf8;
  word-break: break-all;
}

.kv-val-input {
  flex: 1;
}

.btn-icon-xs {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 0.85rem;
  padding: 4px;
}

.yaml-actions-row {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 8px;
}

.yaml-code-block {
  background: rgba(10, 15, 30, 0.9);
  padding: 16px;
  border-radius: 8px;
  border: 1px solid rgba(0, 229, 255, 0.2);
  color: #38bdf8;
  font-size: 0.78rem;
  max-height: 480px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

.toast-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 0.85rem;
}

.toast-success { background: rgba(16, 185, 129, 0.15); border: 1px solid rgba(16, 185, 129, 0.35); color: #34d399; }
.toast-error { background: rgba(244, 63, 94, 0.15); border: 1px solid rgba(244, 63, 94, 0.35); color: #fb7185; }

.toast-close {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  margin-left: auto;
}

.offline-banner {
  display: flex;
  gap: 12px;
  padding: 14px 18px;
  border-radius: 8px;
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.35);
  color: #fbbf24;
}

.offline-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.offline-title {
  font-size: 0.95rem;
  color: #fff;
}

.offline-desc {
  margin: 0;
  font-size: 0.82rem;
  color: #cbd5e1;
}

.offline-actions {
  display: flex;
  gap: 8px;
  margin-top: 6px;
  flex-wrap: wrap;
}

@media (max-width: 960px) {
  .explorer-layout {
    flex-direction: column;
  }
  .explorer-sidebar {
    width: 100%;
    max-height: none;
  }
}
</style>