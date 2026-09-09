import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  promotionsApi,
  dockerApi,
  type Promotion,
  type Environment,
  type CreatePromotionPayload,
  type ComputeHost
} from '../api/compute'
import { useAuthStore } from '../stores/authStore'

export interface RunningServiceOption {
  id: string
  name: string
  image: string
  type: 'swarm' | 'container' | 'k8s'
  host: string
  hostEndpoint?: string
  status?: string
  replicas?: number
  ports?: string[]
}

export function usePromotions() {
  const authStore = useAuthStore()
  const loading = ref(false)
  const loadingServices = ref(false)
  const error = ref<string | null>(null)
  const actionLoading = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

  const promotions = ref<Promotion[]>([])
  const runningServices = ref<RunningServiceOption[]>([])
  const environments: Environment[] = ['dev', 'qa', 'staging', 'production']

  const serviceSearchQuery = ref('')
  const isServiceDropdownOpen = ref(false)
  const comboboxRef = ref<HTMLElement | null>(null)
  const showCreateModal = ref(false)
  const isDiffDrawerOpen = ref(false)
  const selectedPromotionForDiff = ref<Promotion | null>(null)

  const getDefaultRequester = () => authStore.user?.email || authStore.user?.role || 'admin'
  const newPromotion = ref<CreatePromotionPayload>({
    service: '', version: '', from_env: 'dev', to_env: 'qa', requester: getDefaultRequester()
  })

  const selectedService = computed(() => runningServices.value.find(s => s.name === newPromotion.value.service))
  const filteredServices = computed(() => {
    const q = serviceSearchQuery.value.trim().toLowerCase()
    if (!q) return runningServices.value
    return runningServices.value.filter(s =>
      s.name.toLowerCase().includes(q) || s.image.toLowerCase().includes(q) ||
      s.host.toLowerCase().includes(q) || s.type.toLowerCase().includes(q) ||
      (s.hostEndpoint && s.hostEndpoint.toLowerCase().includes(q)) ||
      (s.ports && s.ports.some(p => p.toLowerCase().includes(q)))
    )
  })

  const pendingCount = computed(() => promotions.value.filter(p => p.status === 'pending').length)
  const approvedCount = computed(() => promotions.value.filter(p => p.status === 'approved' || p.status === 'promoting').length)
  const completedCount = computed(() => promotions.value.filter(p => p.status === 'completed').length)
  const rejectedCount = computed(() => promotions.value.filter(p => p.status === 'rejected' || p.status === 'failed').length)

  function selectService(svc: RunningServiceOption) {
    newPromotion.value.service = svc.name
    serviceSearchQuery.value = svc.name
    isServiceDropdownOpen.value = false
  }

  function clearServiceSearch() {
    serviceSearchQuery.value = ''
    newPromotion.value.service = ''
    isServiceDropdownOpen.value = true
  }

  function onSearchInput() {
    isServiceDropdownOpen.value = true
    const q = serviceSearchQuery.value.trim().toLowerCase()
    const matched = runningServices.value.find(s => s.name.toLowerCase() === q)
    newPromotion.value.service = matched ? matched.name : serviceSearchQuery.value.trim()
  }

  function handleClickOutside(e: MouseEvent) {
    if (comboboxRef.value && !comboboxRef.value.contains(e.target as Node)) isServiceDropdownOpen.value = false
  }

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    toastMessage.value = { text, type }
    setTimeout(() => { if (toastMessage.value?.text === text) toastMessage.value = null }, 4000)
  }

  async function fetchPromotions() {
    loading.value = true
    error.value = null
    try {
      const res = await promotionsApi.list()
      promotions.value = Array.isArray(res.data) ? res.data : []
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to retrieve promotions'
      promotions.value = []
    } finally {
      loading.value = false
    }
  }

  async function loadServices() {
    loadingServices.value = true
    try {
      const [svcRes, contRes, hostsRes] = await Promise.allSettled([
        dockerApi.getServices(), dockerApi.getContainers(), dockerApi.listHosts()
      ])
      const hostsList: ComputeHost[] = (hostsRes.status === 'fulfilled' && Array.isArray(hostsRes.value)) ? hostsRes.value : []
      const primaryHost = hostsList.find(h => h.host_type === 'docker' || h.status === 'connected') || hostsList[0]
      const cleanEndpoint = (primaryHost?.endpoint || '').replace(/^tcp:\/\/|^https?:\/\//, '').replace(/:\d+$/, '')
      const hostDisplay = primaryHost ? (primaryHost.name ? `${primaryHost.name}${cleanEndpoint ? ` (${cleanEndpoint})` : ''}` : cleanEndpoint || '--') : '--'
      const servicesList: RunningServiceOption[] = []
      const seen = new Set<string>()
      const swarmNames: string[] = []

      if (svcRes.status === 'fulfilled' && Array.isArray(svcRes.value)) {
        for (const s of svcRes.value) {
          if (s.name && !seen.has(s.name)) {
            seen.add(s.name)
            swarmNames.push(s.name)
            servicesList.push({
              id: s.id || s.name, name: s.name, image: s.image || 'unknown:latest', type: 'swarm',
              host: `${hostDisplay} [Swarm Cluster]`, hostEndpoint: cleanEndpoint,
              status: (s.replicas ?? 0) > 0 ? 'running' : 'stopped', replicas: s.replicas ?? 1, ports: Array.isArray(s.ports) ? s.ports : []
            })
          }
        }
      }

      if (contRes.status === 'fulfilled' && Array.isArray(contRes.value)) {
        for (const c of contRes.value) {
          const cleanName = (c.name || '').replace(/^\//, '')
          if (!cleanName || seen.has(cleanName) || swarmNames.some(s => cleanName.startsWith(s + '.') || cleanName.startsWith(s + '_'))) continue
          seen.add(cleanName)
          const rawPorts = (c as unknown as Record<string, unknown>).ports
          let ports: string[] = []
          if (Array.isArray(rawPorts)) {
            ports = (rawPorts as Array<Record<string, unknown> | string>).map(p =>
              typeof p === 'string' ? p : `${p.PublicPort || p.public_port || ''}:${p.PrivatePort || p.private_port || ''}/${p.Type || p.type || 'tcp'}`
            ).filter(Boolean)
          } else if (typeof rawPorts === 'string' && rawPorts) {
            ports = [rawPorts]
          }
          const isRunning = (c.state || '').toLowerCase() === 'running' || (c.status || '').toLowerCase().includes('up')
          servicesList.push({
            id: c.id || cleanName, name: cleanName, image: c.image || 'unknown:latest', type: 'container',
            host: hostDisplay, hostEndpoint: cleanEndpoint, status: c.state || c.status || 'running', replicas: isRunning ? 1 : 0, ports
          })
        }
      }

      runningServices.value = servicesList
      if (servicesList.length > 0 && !newPromotion.value.service) {
        newPromotion.value.service = servicesList[0].name
        serviceSearchQuery.value = servicesList[0].name
      }
    } catch (err: unknown) {
      console.error('Failed to load active services:', err instanceof Error ? err.message : err)
    } finally {
      loadingServices.value = false
    }
  }

  function openCreateModal() {
    if (runningServices.value.length > 0 && !newPromotion.value.service) newPromotion.value.service = runningServices.value[0].name
    if (newPromotion.value.service) serviceSearchQuery.value = newPromotion.value.service
    isServiceDropdownOpen.value = false
    newPromotion.value.requester = getDefaultRequester()
    showCreateModal.value = true
  }

  function openDiffDrawer(p: Promotion) { selectedPromotionForDiff.value = p; isDiffDrawerOpen.value = true }
  function closeDiffDrawer() { isDiffDrawerOpen.value = false; selectedPromotionForDiff.value = null }

  async function handleCreatePromotion() {
    if (!newPromotion.value.service || !newPromotion.value.version) return showToast('Service name and target version are required', 'error')
    if (newPromotion.value.from_env === newPromotion.value.to_env) return showToast('Source and destination environments must differ', 'error')
    actionLoading.value = 'create'
    try {
      await promotionsApi.create(newPromotion.value)
      showToast(`Promotion request for ${newPromotion.value.service} created!`)
      showCreateModal.value = false
      newPromotion.value.version = ''
      newPromotion.value.service = runningServices.value.length > 0 ? runningServices.value[0].name : ''
      serviceSearchQuery.value = newPromotion.value.service
      await fetchPromotions()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Promotion request failed', 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function runAction(id: string, actionFn: (id: string) => Promise<unknown>, msg: string, isErr = false) {
    actionLoading.value = id
    try {
      await actionFn(id)
      showToast(msg, isErr ? 'error' : 'success')
      await fetchPromotions()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Action failed', 'error')
    } finally {
      actionLoading.value = null
    }
  }

  function formatDate(d?: string) {
    if (!d) return '-'
    try { return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) } catch { return d }
  }

  onMounted(async () => {
    document.addEventListener('click', handleClickOutside)
    await Promise.all([fetchPromotions(), loadServices()])
  })

  onUnmounted(() => document.removeEventListener('click', handleClickOutside))

  return {
    loading, loadingServices, error, actionLoading, toastMessage,
    promotions, runningServices, environments, serviceSearchQuery,
    isServiceDropdownOpen, comboboxRef, showCreateModal, isDiffDrawerOpen,
    selectedPromotionForDiff, newPromotion, selectedService, filteredServices,
    pendingCount, approvedCount, completedCount, rejectedCount,
    selectService, clearServiceSearch, onSearchInput, onSearchFocus: () => isServiceDropdownOpen.value = true,
    handleClickOutside, showToast, fetchPromotions, loadServices, refreshAll: () => Promise.all([fetchPromotions(), loadServices()]),
    openCreateModal, openDiffDrawer, closeDiffDrawer, getPromotionsForEnv: (env: Environment) => promotions.value.filter(p => p.to_env === env),
    handleCreatePromotion,
    handleApprove: (p: Promotion) => runAction(p.id, promotionsApi.approve, `Promotion for ${p.service} approved! Ready for deployment.`),
    handleReject: (p: Promotion) => runAction(p.id, promotionsApi.reject, `Promotion for ${p.service} rejected.`),
    handleComplete: (p: Promotion) => runAction(p.id, promotionsApi.complete, `Promotion for ${p.service} completed successfully in ${p.to_env}!`),
    handleRollback: (p: Promotion) => runAction(p.id, promotionsApi.reject, `Rollback triggered for ${p.service}. Reverting to previous baseline artifact.`),
    handleAbort: (p: Promotion) => runAction(p.id, promotionsApi.reject, `Promotion ${p.service}:${p.version} aborted.`, true),
    formatDate
  }
}
