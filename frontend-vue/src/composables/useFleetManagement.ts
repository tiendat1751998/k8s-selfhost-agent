import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  fleetApi,
  type Cluster,
  type ClusterDiscoveryData,
  type SwarmClusterInfo
} from '../api/fleet'

export interface GeoLocationInfo {
  region: string
  label: string
  flag: string
  zone: 'Americas' | 'Europe' | 'Asia-Pacific' | 'Global' | 'On-Prem'
  lat: number
  lng: number
}

export interface ImportClusterForm {
  id: string
  name: string
  group: string
  region: string
  provider: string
  kubeconfigRaw: string
}

export function useFleetManagement() {
  // 1. Async and notification state
  const loading = ref(false)
  const error = ref<string | null>(null)
  const actionLoading = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

  // 2. Multi-cluster federation state
  const clusters = ref<Cluster[]>([])
  const swarmInfo = ref<SwarmClusterInfo | null>(null)
  const lastSyncTime = ref<Date | null>(null)

  // 3. Cluster Join Wizard state
  const showImportModal = ref(false)
  const importMode = ref<'file' | 'text' | 'token'>('file')
  const importForm = ref<ImportClusterForm>({
    id: '',
    name: '',
    group: 'production',
    region: 'us-east-1',
    provider: 'aws',
    kubeconfigRaw: '',
  })
  const kubeconfigFile = ref<File | null>(null)
  const joinToken = ref('')

  // 4. Discovery & Policy Drawers state
  const showDiscoveryDrawer = ref(false)
  const selectedCluster = ref<Cluster | null>(null)
  const discoveredData = ref<ClusterDiscoveryData | null>(null)
  const showPolicyDrawer = ref(false)
  const selectedPolicyCluster = ref<Cluster | null>(null)

  // 5. Health heartbeat timer
  let heartbeatTimer: ReturnType<typeof setInterval> | null = null

  // 6. Metrics & Derived state
  const k8sClusterCount = computed(() => clusters.value.length)
  const hasSwarm = computed(() => swarmInfo.value !== null && (swarmInfo.value.node_count > 0 || Boolean(swarmInfo.value.id)))
  const totalControlPlanes = computed(() => clusters.value.length + (hasSwarm.value ? 1 : 0))

  const k8sNodes = computed(() => clusters.value.reduce((acc, c) => acc + (c.nodes || 0), 0))
  const swarmNodes = computed(() => (hasSwarm.value && swarmInfo.value ? swarmInfo.value.node_count : 0))
  const totalNodes = computed(() => k8sNodes.value + swarmNodes.value)

  const healthyK8sCount = computed(() =>
    clusters.value.filter(c => ['healthy', 'active', 'ready'].includes((c.health_status || c.status || '').toLowerCase())).length
  )
  const healthyCount = computed(() => healthyK8sCount.value + (hasSwarm.value ? 1 : 0))

  const cloudProviders = computed(() => {
    const providers = new Set<string>()
    clusters.value.forEach(c => {
      if (c.provider) providers.add(c.provider.toUpperCase())
    })
    if (hasSwarm.value) {
      providers.add('SWARM')
    }
    return Array.from(providers)
  })

  // 7. Geo-location mapping
  const geoMap: Record<string, GeoLocationInfo> = {
    'us-east-1': { region: 'us-east-1', label: 'N. Virginia (US-East)', flag: '🇺🇸', zone: 'Americas', lat: 38.13, lng: -78.45 },
    'us-west-2': { region: 'us-west-2', label: 'Oregon (US-West)', flag: '🇺🇸', zone: 'Americas', lat: 45.83, lng: -119.70 },
    'eu-west-1': { region: 'eu-west-1', label: 'Ireland (EU-West)', flag: '🇮🇪', zone: 'Europe', lat: 53.33, lng: -6.25 },
    'eu-central-1': { region: 'eu-central-1', label: 'Frankfurt (EU-Central)', flag: '🇩🇪', zone: 'Europe', lat: 50.11, lng: 8.68 },
    'ap-southeast-1': { region: 'ap-southeast-1', label: 'Singapore (AP-SE)', flag: '🇸🇬', zone: 'Asia-Pacific', lat: 1.35, lng: 103.82 },
    'ap-northeast-1': { region: 'ap-northeast-1', label: 'Tokyo (AP-NE)', flag: '🇯🇵', zone: 'Asia-Pacific', lat: 35.68, lng: 139.69 },
    'local': { region: 'local', label: 'On-Premise / Edge', flag: '🏢', zone: 'On-Prem', lat: 0, lng: 0 },
  }

  function getRegionGeo(regionStr: string): GeoLocationInfo {
    const normalized = (regionStr || '').toLowerCase().trim()
    return geoMap[normalized] || {
      region: regionStr || 'unknown',
      label: regionStr ? regionStr.toUpperCase() : 'Global Multi-Region',
      flag: '🌐',
      zone: 'Global',
      lat: 0,
      lng: 0,
    }
  }

  // 8. Toast Helper
  function showToast(text: string, type: 'success' | 'error' = 'success') {
    toastMessage.value = { text, type }
    setTimeout(() => {
      if (toastMessage.value?.text === text) {
        toastMessage.value = null
      }
    }, 4000)
  }

  // 9. API Actions
  async function fetchFleet(silent = false) {
    if (!silent) loading.value = true
    error.value = null
    try {
      const [clusterList, swarm] = await Promise.all([
        fleetApi.list(),
        fleetApi.getSwarm().catch(() => null),
      ])
      clusters.value = clusterList
      swarmInfo.value = swarm
      lastSyncTime.value = new Date()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to retrieve multi-cluster fleet'
      error.value = msg
    } finally {
      if (!silent) loading.value = false
    }
  }

  function handleFileChange(event: Event) {
    const target = event.target as HTMLInputElement
    if (target.files && target.files.length > 0) {
      kubeconfigFile.value = target.files[0]
    }
  }

  function handleGenerateJoinToken() {
    const randomBytes = new Uint8Array(16)
    crypto.getRandomValues(randomBytes)
    const randomHex = Array.from(randomBytes, b => b.toString(16).padStart(2, '0')).join('')
    joinToken.value = `k8s-agent-tk-${Date.now().toString(36)}-${randomHex}`
    showToast('New cluster join token generated successfully!')
  }

  async function handleImportCluster() {
    if (!importForm.value.name.trim()) {
      showToast('Cluster name is required', 'error')
      return
    }

    if (importMode.value === 'token') {
      actionLoading.value = 'import'
      try {
        await fleetApi.register({
          name: importForm.value.name.trim(),
          group: importForm.value.group,
          region: importForm.value.region.trim(),
          provider: importForm.value.provider,
          status: 'active',
          health_status: 'healthy',
        })
        showToast(`Cluster ${importForm.value.name} registered via join token!`)
        showImportModal.value = false
        resetImportForm()
        await fetchFleet()
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : 'Cluster registration failed'
        showToast(msg, 'error')
      } finally {
        actionLoading.value = null
      }
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

    actionLoading.value = 'import'
    try {
      const formData = new FormData()
      formData.append('id', importForm.value.id.trim() || `cluster-${Date.now().toString(36)}`)
      formData.append('name', importForm.value.name.trim())
      formData.append('group', importForm.value.group)
      formData.append('region', importForm.value.region.trim())
      formData.append('provider', importForm.value.provider)
      formData.append('kubeconfig', fileToUpload)

      await fleetApi.importCluster(formData)
      showToast(`Cluster ${importForm.value.name} successfully imported into fleet!`)
      showImportModal.value = false
      resetImportForm()
      await fetchFleet()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Cluster import failed'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  function resetImportForm() {
    importForm.value.name = ''
    importForm.value.id = ''
    importForm.value.kubeconfigRaw = ''
    kubeconfigFile.value = null
    joinToken.value = ''
  }

  async function handleUpgrade(cluster: Cluster) {
    actionLoading.value = cluster.id
    try {
      await fleetApi.upgrade(cluster.id)
      showToast(`Cluster upgrade sequence initiated for ${cluster.name}!`)
      await fetchFleet(true)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Upgrade request failed'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function handleSync(cluster: Cluster) {
    actionLoading.value = cluster.id
    try {
      await fleetApi.getHealth(cluster.id).catch(() => null)
      showToast(`Cluster federation state synchronized for ${cluster.name}!`)
      await fetchFleet(true)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Sync request failed'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function handleDiscover(cluster: Cluster) {
    selectedCluster.value = cluster
    actionLoading.value = cluster.id
    try {
      const res = await fleetApi.discover(cluster.id)
      discoveredData.value = res || (cluster.discovered_resources as ClusterDiscoveryData) || null
      showDiscoveryDrawer.value = true
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Resource discovery failed'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  function handlePolicy(cluster: Cluster) {
    selectedPolicyCluster.value = cluster
    showPolicyDrawer.value = true
  }

  async function handleRemove(cluster: Cluster) {
    if (!confirm(`Are you sure you want to evict cluster "${cluster.name}" from the fleet?`)) return
    actionLoading.value = cluster.id
    try {
      await fleetApi.remove(cluster.id)
      showToast(`Cluster ${cluster.name} evicted from fleet.`)
      await fetchFleet()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to evict cluster'
      showToast(msg, 'error')
    } finally {
      actionLoading.value = null
    }
  }

  // 10. Health heartbeat scheduler
  function startHeartbeat() {
    stopHeartbeat()
    heartbeatTimer = setInterval(() => {
      fetchFleet(true)
    }, 30000)
  }

  function stopHeartbeat() {
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer)
      heartbeatTimer = null
    }
  }

  onMounted(() => {
    fetchFleet()
    startHeartbeat()
  })

  onUnmounted(() => {
    stopHeartbeat()
  })

  return {
    // State
    loading,
    error,
    actionLoading,
    toastMessage,
    clusters,
    swarmInfo,
    lastSyncTime,
    showImportModal,
    importMode,
    importForm,
    kubeconfigFile,
    joinToken,
    showDiscoveryDrawer,
    selectedCluster,
    discoveredData,
    showPolicyDrawer,
    selectedPolicyCluster,

    // Metrics
    k8sClusterCount,
    hasSwarm,
    totalControlPlanes,
    k8sNodes,
    swarmNodes,
    totalNodes,
    healthyK8sCount,
    healthyCount,
    cloudProviders,

    // Methods
    getRegionGeo,
    showToast,
    fetchFleet,
    handleFileChange,
    handleGenerateJoinToken,
    handleImportCluster,
    handleUpgrade,
    handleSync,
    handleDiscover,
    handlePolicy,
    handleRemove,
  }
}
