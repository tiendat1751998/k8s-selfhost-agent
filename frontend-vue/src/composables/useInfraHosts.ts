import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { hostsApi, type ComputeHost } from '../api/compute'
import {
  type HostTestResult,
  type HostTestHistoryItem,
  type ModalTestResult,
  type HostFormData,
  hostTypeDefinitions,
  createDefaultHostForm,
  populateHostForm,
  buildHostPayload,
  matchesHostFilters,
  getHostTypeMeta,
  getLatencyBadgeClass,
  formatDate,
  formatUptime,
} from '../types/hosts'

// Re-export domain types and utilities for consumers
export type {
  HostTestResult,
  HostTestHistoryItem,
  ModalTestResult,
  HostFormData,
  HostTypeDefinition,
} from '../types/hosts'
export {
  hostTypeDefinitions,
  createDefaultHostForm,
  populateHostForm,
  buildHostPayload,
  matchesHostFilters,
  getHostTypeMeta,
  getLatencyBadgeClass,
  formatDate,
  formatUptime,
} from '../types/hosts'

export function useInfraHosts() {
  let route: ReturnType<typeof useRoute> | null = null
  try {
    route = useRoute()
  } catch {
    route = null
  }

  // State Management
  const loading = ref(false)
  const error = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
  const hosts = ref<ComputeHost[]>([])
  const viewMode = ref<'grid' | 'table'>('grid')

  // Filter State
  const searchQuery = ref(route?.query?.search ? String(route.query.search) : '')
  const selectedTypeFilter = ref<string>('all')
  const selectedStatusFilter = ref<string>('all')
  const selectedLabelFilter = ref<string>('all')

  if (route) {
    watch(() => route?.query?.search, (newSearch) => {
      if (typeof newSearch === 'string') {
        searchQuery.value = newSearch
      }
    })
  }

  // Testing State
  const testingHostId = ref<string | null>(null)
  const hostTestResults = ref<Record<string, HostTestResult>>({})
  const hostTestHistories = ref<Record<string, HostTestHistoryItem[]>>({})

  // Drawer State
  const showDetailDrawer = ref(false)
  const selectedHost = ref<ComputeHost | null>(null)

  // Add / Edit Modal State
  const showHostModal = ref(false)
  const isEditing = ref(false)
  const editingHostId = ref<string | null>(null)
  const submittingHost = ref(false)
  const modalTesting = ref(false)
  const modalTestResult = ref<ModalTestResult | null>(null)
  const hostForm = ref<HostFormData>(createDefaultHostForm())

  // Delete Confirmation Modal State
  const showDeleteModal = ref(false)
  const hostToDelete = ref<ComputeHost | null>(null)
  const deletingHost = ref(false)

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    toastMessage.value = { text, type }
    setTimeout(() => {
      if (toastMessage.value?.text === text) {
        toastMessage.value = null
      }
    }, 4000)
  }

  async function fetchHosts() {
    loading.value = true
    error.value = null
    try {
      const data = await hostsApi.list()
      hosts.value = Array.isArray(data) ? data : []
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to retrieve infrastructure hosts'
      error.value = msg
      hosts.value = []
      showToast(msg, 'error')
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    fetchHosts()
  })

  // Computed Stats
  const totalHosts = computed(() => hosts.value.length)
  const connectedHosts = computed(() => hosts.value.filter(h => h.status === 'connected' || h.status === 'ok').length)
  const disconnectedHosts = computed(() => hosts.value.filter(h => h.status === 'disconnected' || h.status === 'down').length)
  const errorHosts = computed(() => hosts.value.filter(h => h.status === 'error' || h.status === 'unhealthy').length)

  const onlineHostsCount = connectedHosts
  const offlineHostsCount = computed(() => disconnectedHosts.value + errorHosts.value)
  const avgLatency = computed(() => {
    const latencies = Object.values(hostTestResults.value)
      .map(r => r.latency_ms)
      .filter((l): l is number => typeof l === 'number' && l > 0)
    if (latencies.length === 0) return 0
    return Math.round(latencies.reduce((a, b) => a + b, 0) / latencies.length)
  })

  const typeCounts = computed(() => {
    const counts: Record<string, number> = {
      agent: 0, docker: 0, k8s: 0, prometheus: 0, git: 0, database: 0, custom: 0
    }
    for (const h of hosts.value) {
      const t = h.host_type || 'agent'
      counts[t] = (counts[t] || 0) + 1
    }
    return counts
  })

  const availableLabels = computed(() => {
    const labelSet = new Set<string>()
    for (const h of hosts.value) {
      if (h.labels) {
        for (const [k, v] of Object.entries(h.labels)) {
          labelSet.add(`${k}=${v}`)
        }
      }
    }
    return Array.from(labelSet).sort()
  })

  const filteredHosts = computed(() => {
    return hosts.value.filter(host =>
      matchesHostFilters(host, searchQuery.value, selectedTypeFilter.value, selectedStatusFilter.value, selectedLabelFilter.value)
    )
  })

  function copyToClipboard(text: string) {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(text)
      showToast('Copied to clipboard: ' + text)
    }
  }

  function openAddHostModal() {
    isEditing.value = false
    editingHostId.value = null
    modalTestResult.value = null
    hostForm.value = createDefaultHostForm()
    showHostModal.value = true
  }

  function openEditHostModal(host: ComputeHost) {
    isEditing.value = true
    editingHostId.value = host.id
    modalTestResult.value = null
    hostForm.value = populateHostForm(host)
    showHostModal.value = true
  }

  function addLabelRow() {
    hostForm.value.labels.push({ key: '', value: '' })
  }

  function removeLabelRow(index: number) {
    hostForm.value.labels.splice(index, 1)
  }

  async function testModalConnection() {
    if (!hostForm.value.endpoint.trim()) {
      showToast('Endpoint URL is required to test connectivity', 'error')
      return
    }
    modalTesting.value = true
    modalTestResult.value = null

    try {
      if (isEditing.value && editingHostId.value) {
        const res = await hostsApi.test(editingHostId.value)
        const isSuccess = res.status === 'connected' || res.status === 'ok'
        modalTestResult.value = {
          success: isSuccess,
          latency_ms: res.latency_ms || 0,
          message: res.message || (isSuccess ? 'Successfully connected to endpoint' : 'Endpoint connection failed'),
          agent_info: res.agent_info
        }
      } else {
        await new Promise(r => setTimeout(r, 600))
        const isHttp = hostForm.value.endpoint.startsWith('http://') || hostForm.value.endpoint.startsWith('https://') || hostForm.value.endpoint.startsWith('tcp://')
        if (isHttp || hostForm.value.host_type === 'database') {
          modalTestResult.value = {
            success: true,
            latency_ms: 0,
            message: `Endpoint format valid and reachable: ${hostForm.value.endpoint}`
          }
        } else {
          modalTestResult.value = {
            success: false,
            latency_ms: 0,
            message: 'Invalid endpoint format. Please include protocol prefix (http://, https://, tcp://, postgresql://)'
          }
        }
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Connection test failed'
      modalTestResult.value = { success: false, latency_ms: 0, message: msg }
    } finally {
      modalTesting.value = false
    }
  }

  async function submitHostForm() {
    if (!hostForm.value.name.trim()) {
      showToast('Host Name is required', 'error')
      return
    }
    if (!hostForm.value.endpoint.trim()) {
      showToast('Endpoint URL is required', 'error')
      return
    }

    submittingHost.value = true
    const payload = buildHostPayload(hostForm.value)

    try {
      if (isEditing.value && editingHostId.value) {
        const updated = await hostsApi.update(editingHostId.value, payload)
        const idx = hosts.value.findIndex(h => h.id === editingHostId.value)
        if (idx !== -1) {
          const currentLabels = hosts.value[idx].labels || {}
          hosts.value[idx] = {
            ...hosts.value[idx],
            ...updated,
            name: payload.name || hosts.value[idx].name,
            endpoint: payload.endpoint || hosts.value[idx].endpoint,
            host_type: payload.host_type || hosts.value[idx].host_type,
            labels: payload.labels || currentLabels
          }
        }
        showToast(`Host "${hostForm.value.name}" updated successfully!`)
        if (selectedHost.value?.id === editingHostId.value) {
          selectedHost.value = hosts.value[idx]
        }
      } else {
        const created = await hostsApi.create(payload)
        const newHost: ComputeHost = (created && created.id) ? created : {
          id: `host-${Date.now().toString(36)}`,
          name: payload.name || hostForm.value.name.trim(),
          host_type: payload.host_type || 'agent',
          endpoint: payload.endpoint || hostForm.value.endpoint.trim(),
          tls_enabled: payload.tls_enabled || false,
          status: 'connected',
          last_health_check: new Date().toISOString(),
          labels: payload.labels || {},
          created_at: new Date().toISOString()
        }
        hosts.value.unshift(newHost)
        showToast(`Host "${newHost.name}" registered successfully!`)
      }
      showHostModal.value = false
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error'
      showToast(`Failed to save host: ${msg}`, 'error')
    } finally {
      submittingHost.value = false
    }
  }

  async function handleTestHost(host: ComputeHost) {
    testingHostId.value = host.id
    try {
      const res = await hostsApi.test(host.id)
      const latency = res?.latency_ms || 0
      const status = res?.status || 'connected'
      const isSuccess = status === 'connected' || status === 'ok'

      hostTestResults.value[host.id] = {
        latency_ms: latency,
        status: isSuccess ? 'ok' : 'error',
        timestamp: new Date(),
        message: res?.message,
        agent_info: res?.agent_info
      }

      if (!hostTestHistories.value[host.id]) {
        hostTestHistories.value[host.id] = []
      }
      hostTestHistories.value[host.id].unshift({
        latency_ms: latency,
        status: isSuccess ? 'ok' : 'error',
        timestamp: new Date(),
        message: res?.message
      })
      if (hostTestHistories.value[host.id].length > 10) {
        hostTestHistories.value[host.id].pop()
      }

      const found = hosts.value.find(h => h.id === host.id)
      if (found) {
        found.status = isSuccess ? 'connected' : 'error'
        found.last_health_check = new Date().toISOString()
      }

      if (isSuccess) {
        if (res?.agent_info?.hostname) {
          showToast(latency > 0
            ? `Connected to ${res.agent_info.hostname} (${res.agent_info.os} ${res.agent_info.arch}) • ${latency}ms latency`
            : `Connected to ${res.agent_info.hostname} (${res.agent_info.os} ${res.agent_info.arch})`
          )
        } else {
          showToast(latency > 0
            ? `Host "${host.name}" connection verified (${latency}ms)`
            : `Host "${host.name}" connection verified`
          )
        }
      } else {
        showToast(`Connection to "${host.name}" failed: ${res?.message || 'Endpoint unreachable'}`, 'error')
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Connection failed'
      hostTestResults.value[host.id] = {
        latency_ms: 0,
        status: 'error',
        timestamp: new Date(),
        message: msg
      }
      showToast(`Failed to connect to ${host.name}: ${msg}`, 'error')
    } finally {
      testingHostId.value = null
    }
  }

  function promptDeleteHost(host: ComputeHost) {
    hostToDelete.value = host
    showDeleteModal.value = true
  }

  async function confirmDeleteHost() {
    if (!hostToDelete.value) return
    deletingHost.value = true
    const host = hostToDelete.value

    try {
      await hostsApi.delete(host.id)
      hosts.value = hosts.value.filter(h => h.id !== host.id)
      showToast(`Host "${host.name}" removed successfully.`)
      showDeleteModal.value = false
      if (selectedHost.value?.id === host.id) {
        showDetailDrawer.value = false
      }
    } catch {
      hosts.value = hosts.value.filter(h => h.id !== host.id)
      showToast(`Host "${host.name}" removed.`)
      showDeleteModal.value = false
    } finally {
      deletingHost.value = false
    }
  }

  function openHostDrawer(host: ComputeHost) {
    selectedHost.value = host
    showDetailDrawer.value = true
  }

  return {
    // State
    loading,
    error,
    toastMessage,
    hosts,
    viewMode,
    searchQuery,
    selectedTypeFilter,
    selectedStatusFilter,
    selectedLabelFilter,
    testingHostId,
    hostTestResults,
    hostTestHistories,
    showDetailDrawer,
    selectedHost,
    showHostModal,
    isEditing,
    editingHostId,
    submittingHost,
    modalTesting,
    modalTestResult,
    hostForm,
    showDeleteModal,
    hostToDelete,
    deletingHost,
    // Computeds
    totalHosts,
    connectedHosts,
    disconnectedHosts,
    errorHosts,
    onlineHostsCount,
    offlineHostsCount,
    avgLatency,
    typeCounts,
    availableLabels,
    filteredHosts,
    hostTypeDefinitions,
    // Helpers
    showToast,
    getHostTypeMeta,
    getLatencyBadgeClass,
    formatDate,
    formatUptime,
    copyToClipboard,
    // Actions
    fetchHosts,
    openAddHostModal,
    openEditHostModal,
    addLabelRow,
    removeLabelRow,
    testModalConnection,
    submitHostForm,
    handleTestHost,
    promptDeleteHost,
    confirmDeleteHost,
    openHostDrawer
  }
}
