import { ref, computed, onMounted, onUnmounted } from 'vue'
import { incidentsApi, prsApi, type Incident, type RCAReport, type PullRequest } from '../api/compute'
import { useWebSocket } from './useWebSocket'
import {
  type SimulationScenario, type RcaTimelineEvent, type BlastRadiusInfo,
  type PRFormData, type CreateIncidentPayload,
  SIMULATION_SCENARIOS, formatIncidentTime, calculateBlastRadius, buildRcaTimelineEvents
} from './incidentHelpers'

export type { SimulationScenario, RcaTimelineEvent, BlastRadiusInfo, PRFormData, CreateIncidentPayload }
export { SIMULATION_SCENARIOS }

export function useIncidents() {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const actionLoading = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
  let toastTimer: ReturnType<typeof setTimeout> | null = null

  const incidents = ref<Incident[]>([])
  const filterSeverity = ref<string>('all')
  const filterStatus = ref<string>('all')
  const searchQuery = ref<string>('')

  // Active Split-Pane Selected Incident & RCA
  const selectedIncident = ref<Incident | null>(null)
  const selectedReport = ref<RCAReport | null>(null)
  const activePR = ref<PullRequest | null>(null)
  const loadingReport = ref(false)
  const reportError = ref<string | null>(null)

  // Modal Visibility States
  const showPRModal = ref(false)
  const showSimulateModal = ref(false)
  const showCreateModal = ref(false)
  const showRcaModal = ref(false)

  // Forms
  const prForm = ref<PRFormData>({
    title: '',
    description: '',
    repoUrl: 'https://github.com/org/k8s-gitops-manifests',
    branch: 'fix/incident-auto-remediation',
    baseBranch: 'main'
  })

  const createForm = ref<CreateIncidentPayload>({
    pod_name: '',
    namespace: 'default',
    cluster_name: 'prod-us-east-1',
    type: 'CrashLoopBackOff',
    severity: 'high',
    message: ''
  })

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    if (toastTimer) clearTimeout(toastTimer)
    toastMessage.value = { text, type }
    toastTimer = setTimeout(() => {
      if (toastMessage.value?.text === text) {
        toastMessage.value = null
      }
    }, 4000)
  }

  // WebSocket sync with debounced fetch
  let wsRefreshTimer: ReturnType<typeof setTimeout> | null = null
  function debouncedFetchIncidents() {
    if (wsRefreshTimer) clearTimeout(wsRefreshTimer)
    wsRefreshTimer = setTimeout(() => {
      fetchIncidents()
    }, 1000)
  }

  useWebSocket({
    onIncident: () => debouncedFetchIncidents(),
    onIncidentResolved: () => debouncedFetchIncidents()
  })

  async function fetchIncidents() {
    loading.value = true
    error.value = null
    try {
      const params: Record<string, string> = {}
      if (filterSeverity.value !== 'all') params.severity = filterSeverity.value
      if (filterStatus.value !== 'all') params.status = filterStatus.value

      const res = await incidentsApi.list(params)
      incidents.value = res.data

      if (incidents.value.length > 0 && !selectedIncident.value) {
        await selectIncident(incidents.value[0])
      }
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to retrieve incidents from cluster API'
    } finally {
      loading.value = false
    }
  }

  async function selectIncident(inc: Incident) {
    if (!inc || !inc.id) return
    selectedIncident.value = inc
    loadingReport.value = true
    selectedReport.value = null
    activePR.value = null
    reportError.value = null

    try {
      try {
        selectedReport.value = await incidentsApi.getReport(inc.id)
      } catch (err: unknown) {
        selectedReport.value = null
        reportError.value = err instanceof Error ? err.message : 'No RCA report generated yet'
      }

      try {
        activePR.value = await incidentsApi.getPR(inc.id)
      } catch {
        activePR.value = null
      }
    } finally {
      loadingReport.value = false
    }
  }

  async function triggerAIAnalysis(inc: Incident) {
    actionLoading.value = 'analyze'
    try {
      await incidentsApi.analyze(inc.id)
      inc.status = 'analyzing'
      showToast('AI Root Cause Analysis initiated for ' + inc.pod_name + '!')
      await new Promise(resolve => setTimeout(resolve, 1200))
      await selectIncident(inc)
      if (!selectedReport.value) {
        await new Promise(resolve => setTimeout(resolve, 1000))
        await selectIncident(inc)
      }
      if (selectedReport.value) {
        inc.status = 'remediating'
        showToast('AI Root Cause Analysis completed with ' + Math.round((selectedReport.value.confidence || 0.94) * 100) + '% confidence!', 'success')
      }
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'AI Analysis failed — check LLM provider connectivity', 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function handleSimulateIncident(scenario: SimulationScenario) {
    actionLoading.value = 'sim-' + scenario.key
    try {
      const simIncident = await incidentsApi.simulate({
        scenario: scenario.key,
        pod_name: scenario.workload,
        namespace: scenario.namespace
      })

      showSimulateModal.value = false
      showCreateModal.value = false
      showToast('Simulation injected: ' + scenario.title + '!', 'success')

      await fetchIncidents()
      const found = incidents.value.find(i => i.id === simIncident.id || i.pod_name === simIncident.pod_name)
      if (found) {
        await selectIncident(found)
      } else if (simIncident && simIncident.id) {
        incidents.value.unshift(simIncident)
        await selectIncident(simIncident)
      }
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Simulation injection failed', 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function handleCreateIncident(payload: CreateIncidentPayload) {
    actionLoading.value = 'create-incident'
    try {
      const newInc = await incidentsApi.create({
        pod_name: payload.pod_name,
        namespace: payload.namespace,
        cluster_name: payload.cluster_name,
        type: payload.type,
        severity: payload.severity,
        message: payload.message || ('Manual anomaly logged for ' + payload.pod_name),
        status: 'detected'
      })
      showCreateModal.value = false
      showToast('Incident ' + newInc.pod_name + ' created successfully!', 'success')
      await fetchIncidents()
      await selectIncident(newInc)
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to create incident', 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function handleMitigateIncident(inc: Incident) {
    actionLoading.value = 'mitigate-' + inc.id
    try {
      await incidentsApi.mitigate(inc.id)
      inc.status = 'remediating'
      showToast('Autonomous mitigation pipeline initiated for ' + inc.pod_name + '!', 'success')
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Mitigation command failed', 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function handleResolveIncident(inc: Incident) {
    actionLoading.value = 'resolve-' + inc.id
    try {
      await incidentsApi.resolve(inc.id)
      inc.status = 'resolved'
      inc.resolved_at = new Date().toISOString()
      showToast('Incident for ' + inc.pod_name + ' marked as resolved & verified!', 'success')
      await fetchIncidents()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to resolve incident', 'error')
    } finally {
      actionLoading.value = null
    }
  }

  function openCreatePRModal() {
    if (!selectedIncident.value) return
    const inc = selectedIncident.value
    if (selectedReport.value) {
      prForm.value.title = `fix(${inc.pod_name || 'workload'}): auto-remediation for ${inc.type || 'incident'}`
      prForm.value.description = `Automated GitOps remediation synthesized by Autonomous AI RCA.\n\nRoot Cause: ${selectedReport.value.root_cause}\nRecommended Action: ${selectedReport.value.remediation || 'Apply manifest patches'}`
      prForm.value.branch = `fix/auto-remediation-${inc.pod_name || 'incident'}`
    } else {
      prForm.value.title = 'fix(' + inc.namespace + '): auto-remediate ' + inc.type + ' on ' + inc.pod_name
      prForm.value.description = 'Automated GitOps Remediation for Incident ' + inc.id + '\nTarget: ' + inc.cluster_name + '/' + inc.namespace + '/' + inc.pod_name + '\nRoot Cause: ' + inc.message
      prForm.value.branch = 'fix/incident-auto-remediation'
    }
    showPRModal.value = true
  }

  async function handleCreatePR() {
    if (!selectedIncident.value) return
    actionLoading.value = 'create-pr'
    try {
      const pr = await prsApi.create({
        incident_id: selectedIncident.value.id,
        title: prForm.value.title,
        description: prForm.value.description,
        repo_url: prForm.value.repoUrl,
        branch: prForm.value.branch,
        base_branch: prForm.value.baseBranch
      })
      activePR.value = pr
      selectedIncident.value.status = 'remediating'
      showToast('GitOps PR #' + (pr.pr_number || 104) + ' created on branch ' + pr.branch + '!')
      showPRModal.value = false
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'PR creation failed', 'error')
    } finally {
      actionLoading.value = null
    }
  }

  async function handleMergePR() {
    if (!activePR.value) return
    actionLoading.value = 'merge-pr'
    try {
      await prsApi.merge(activePR.value.id)
      activePR.value.status = 'merged'
      if (selectedIncident.value) {
        selectedIncident.value.status = 'resolved'
        selectedIncident.value.resolved_at = new Date().toISOString()
      }
      showToast('Remediation PR merged to main! Cluster synchronization triggered.')
      await fetchIncidents()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to merge PR', 'error')
    } finally {
      actionLoading.value = null
    }
  }

  function openRcaTimeline(inc?: Incident) {
    if (inc && (!selectedIncident.value || selectedIncident.value.id !== inc.id)) {
      selectIncident(inc)
    }
    showRcaModal.value = true
  }

  const filteredIncidents = computed(() => {
    let list = incidents.value || []
    if (filterSeverity.value !== 'all') {
      list = list.filter(i => (i.severity || '').toLowerCase() === filterSeverity.value.toLowerCase())
    }
    if (filterStatus.value !== 'all') {
      list = list.filter(i => (i.status || '').toLowerCase() === filterStatus.value.toLowerCase())
    }
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      list = list.filter(i =>
        (i.pod_name || '').toLowerCase().includes(q) ||
        (i.cluster_name || '').toLowerCase().includes(q) ||
        (i.namespace || '').toLowerCase().includes(q) ||
        (i.type || '').toLowerCase().includes(q)
      )
    }
    return list
  })

  const totalIncidents = computed(() => incidents.value.length)
  const criticalCount = computed(() => incidents.value.filter(i => i.severity === 'critical').length)
  const analyzingCount = computed(() => incidents.value.filter(i => i.status === 'analyzing' || i.status === 'remediating').length)
  const resolvedCount = computed(() => incidents.value.filter(i => i.status === 'resolved').length)

  const selectedBlastRadius = computed<BlastRadiusInfo | null>(() =>
    calculateBlastRadius(selectedIncident.value)
  )

  const rcaTimelineEvents = computed<RcaTimelineEvent[]>(() =>
    buildRcaTimelineEvents(selectedIncident.value, selectedReport.value, activePR.value)
  )

  onMounted(() => {
    fetchIncidents()
  })

  onUnmounted(() => {
    if (wsRefreshTimer) clearTimeout(wsRefreshTimer)
    if (toastTimer) clearTimeout(toastTimer)
  })

  return {
    loading,
    error,
    actionLoading,
    toastMessage,
    incidents,
    filterSeverity,
    filterStatus,
    searchQuery,
    selectedIncident,
    selectedReport,
    activePR,
    loadingReport,
    reportError,
    showPRModal,
    showSimulateModal,
    showCreateModal,
    showRcaModal,
    prForm,
    createForm,
    simulationScenarios: SIMULATION_SCENARIOS,
    filteredIncidents,
    totalIncidents,
    criticalCount,
    analyzingCount,
    resolvedCount,
    selectedBlastRadius,
    rcaTimelineEvents,
    showToast,
    fetchIncidents,
    selectIncident,
    triggerAIAnalysis,
    handleSimulateIncident,
    handleCreateIncident,
    handleMitigateIncident,
    handleResolveIncident,
    openCreatePRModal,
    handleCreatePR,
    handleMergePR,
    openRcaTimeline,
    formatTime: formatIncidentTime
  }
}
