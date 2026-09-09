import { ref, computed, onMounted } from 'vue'
import { driftApi, type DriftRecord } from '../api/governance'

export type DriftSeverity = 'critical' | 'high' | 'medium' | 'low'
export type DriftFilterStatus = 'ALL' | 'DRIFTED' | 'CRITICAL' | 'IN_SYNC'

export interface EnrichedDriftRecord extends Record<string, unknown> {
  id: string
  cluster: string
  namespace: string
  resource: string
  resource_kind: string
  expected_state: string
  actual_state: string
  diff: string
  status: 'in_sync' | 'drifted' | 'unknown'
  detected_at: string
  severity: DriftSeverity
  driftType: string
  isSuppressed: boolean
  suppressionReason?: string
}

export interface DriftStatusMessage {
  type: 'success' | 'error' | 'info'
  text: string
}

const STORAGE_SUPPRESSED_KEY = 'k8s_drift_suppressions_v1'
const STORAGE_REMEDIATED_COUNT_KEY = 'k8s_drift_remediated_today_v1'

export function useDriftDetection() {
  const drifts = ref<DriftRecord[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const activeStatus = ref<DriftFilterStatus>('ALL')
  const clusterFilter = ref('')
  const searchQuery = ref('')
  const resolvingId = ref<string | null>(null)
  const selectedDrift = ref<EnrichedDriftRecord | null>(null)
  const diffViewMode = ref<'split' | 'unified'>('split')
  const statusMessage = ref<DriftStatusMessage | null>(null)
  const suppressedIds = ref<Record<string, string>>({})
  const remediatedTodayCount = ref(0)
  const autoRemediateCritical = ref(false)

  // Initialize stored suppressions and remediation count
  try {
    const rawSuppressed = localStorage.getItem(STORAGE_SUPPRESSED_KEY)
    if (rawSuppressed) {
      suppressedIds.value = JSON.parse(rawSuppressed)
    }
    const rawRemediated = localStorage.getItem(STORAGE_REMEDIATED_COUNT_KEY)
    if (rawRemediated) {
      const parsed = JSON.parse(rawRemediated)
      const today = new Date().toISOString().slice(0, 10)
      if (parsed.date === today) {
        remediatedTodayCount.value = parsed.count || 0
      }
    }
  } catch {
    suppressedIds.value = {}
  }

  function saveSuppressedState() {
    try {
      localStorage.setItem(STORAGE_SUPPRESSED_KEY, JSON.stringify(suppressedIds.value))
    } catch {
      // Ignore storage write errors
    }
  }

  function recordRemediationSuccess() {
    remediatedTodayCount.value += 1
    try {
      const today = new Date().toISOString().slice(0, 10)
      localStorage.setItem(STORAGE_REMEDIATED_COUNT_KEY, JSON.stringify({
        date: today,
        count: remediatedTodayCount.value,
      }))
    } catch {
      // Ignore storage write errors
    }
  }

  // Drift Severity Analysis Engine
  function analyzeDriftSeverity(record: DriftRecord): DriftSeverity {
    if (record.status !== 'drifted') return 'low'
    const kind = (record.resource_kind || '').toLowerCase()
    const diff = (record.diff || '').toLowerCase()

    if (kind === 'deployment' || kind === 'daemonset' || kind === 'statefulset') {
      if (diff.includes('replicas: 0') || diff.includes('securitycontext') || diff.includes('privileged')) {
        return 'critical'
      }
      if (diff.includes('image:') || diff.includes('containerport') || diff.includes('env:')) {
        return 'high'
      }
      return 'medium'
    }

    if (kind === 'secret' || kind === 'rbac' || kind === 'clusterrole' || kind === 'networkpolicy') {
      return 'critical'
    }

    if (kind === 'configmap' || kind === 'service') {
      return 'medium'
    }

    return 'low'
  }

  // Drift Classification Engine
  function classifyDriftType(record: DriftRecord): string {
    const diff = (record.diff || '').toLowerCase()
    if (diff.includes('replicas:')) return 'REPLICA_MISMATCH'
    if (diff.includes('image:') || diff.includes('tag:')) return 'CONTAINER_IMAGE'
    if (diff.includes('securitycontext') || diff.includes('runasuser')) return 'SECURITY_MUTATION'
    if (diff.includes('env:') || diff.includes('envfrom:')) return 'ENV_VARIABLE_DRIFT'
    if (diff.includes('ports:') || diff.includes('targetport:')) return 'NETWORK_PORT'
    if (diff.includes('labels:') || diff.includes('annotations:')) return 'METADATA_DRIFT'
    return 'SPEC_MUTATION'
  }

  // Enriched Drifts with severity, type and suppression state
  const enrichedDrifts = computed<EnrichedDriftRecord[]>(() => {
    return drifts.value.map(record => {
      const isSuppressed = Boolean(suppressedIds.value[record.id])
      return {
        ...record,
        severity: analyzeDriftSeverity(record),
        driftType: classifyDriftType(record),
        isSuppressed,
        suppressionReason: suppressedIds.value[record.id],
      }
    })
  })

  // Filtered List
  const filteredDrifts = computed<EnrichedDriftRecord[]>(() => {
    const query = searchQuery.value.trim().toLowerCase()

    return enrichedDrifts.value.filter(d => {
      // Cluster filter
      if (clusterFilter.value && d.cluster !== clusterFilter.value) {
        return false
      }

      // Status filter
      if (activeStatus.value === 'DRIFTED' && (d.status !== 'drifted' || d.isSuppressed)) {
        return false
      }
      if (activeStatus.value === 'IN_SYNC' && d.status !== 'in_sync') {
        return false
      }
      if (activeStatus.value === 'CRITICAL' && (d.severity !== 'critical' || d.status !== 'drifted' || d.isSuppressed)) {
        return false
      }

      // Search query
      if (query) {
        const matchKind = (d.resource_kind || '').toLowerCase().includes(query)
        const matchRes = (d.resource || '').toLowerCase().includes(query)
        const matchNs = (d.namespace || '').toLowerCase().includes(query)
        const matchCluster = (d.cluster || '').toLowerCase().includes(query)
        const matchType = (d.driftType || '').toLowerCase().includes(query)
        if (!matchKind && !matchRes && !matchNs && !matchCluster && !matchType) {
          return false
        }
      }

      return true
    })
  })

  // Metric Computations
  const driftedCount = computed(() => enrichedDrifts.value.filter(d => d.status === 'drifted' && !d.isSuppressed).length)
  const criticalCount = computed(() => enrichedDrifts.value.filter(d => d.status === 'drifted' && d.severity === 'critical' && !d.isSuppressed).length)
  const inSyncCount = computed(() => enrichedDrifts.value.filter(d => d.status === 'in_sync').length)
  const syncRate = computed(() => {
    if (enrichedDrifts.value.length === 0) return 100
    return Math.round((inSyncCount.value / enrichedDrifts.value.length) * 100)
  })
  const gitReposTracked = computed(() => {
    const clusters = new Set(enrichedDrifts.value.map(d => d.cluster || 'primary'))
    return Math.max(clusters.size, 1)
  })

  const statusFilters = computed(() => [
    { key: 'ALL' as const, label: 'All Workloads', count: enrichedDrifts.value.length, badgeClass: 'badge-cyan' },
    { key: 'DRIFTED' as const, label: 'Drifted', count: driftedCount.value, badgeClass: 'badge-rose' },
    { key: 'CRITICAL' as const, label: 'Critical Out-of-Sync', count: criticalCount.value, badgeClass: 'badge-amber' },
    { key: 'IN_SYNC' as const, label: 'In Sync', count: inSyncCount.value, badgeClass: 'badge-emerald' },
  ])

  // Fetch API
  async function fetchDriftData() {
    loading.value = true
    error.value = null
    try {
      const res = await driftApi.getDrifts({
        cluster: clusterFilter.value || undefined,
        status: activeStatus.value === 'ALL' || activeStatus.value === 'CRITICAL' ? undefined : activeStatus.value.toLowerCase(),
      })
      drifts.value = res.data
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load configuration drift records'
      error.value = msg
    } finally {
      loading.value = false
    }
  }

  // Reconciliation API Call
  async function handleReconcile(id: string) {
    resolvingId.value = id
    statusMessage.value = null
    try {
      await driftApi.resolveDrift(id)
      recordRemediationSuccess()
      statusMessage.value = {
        type: 'success',
        text: `Reconciliation dispatched for workload #${id.slice(0, 8)}. Live cluster etcd aligned with Git repository.`,
      }

      // Update in-memory state
      const target = drifts.value.find(d => d.id === id)
      if (target) {
        target.status = 'in_sync'
      }
      if (selectedDrift.value && selectedDrift.value.id === id) {
        selectedDrift.value.status = 'in_sync'
      }

      await fetchDriftData()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to auto-reconcile drift'
      statusMessage.value = { type: 'error', text: msg }
    } finally {
      resolvingId.value = null
    }
  }

  // Batch Auto-Reconcile for Critical or All Drifted Workloads
  async function handleBatchReconcile(onlyCritical = false) {
    const targets = enrichedDrifts.value.filter(d => 
      d.status === 'drifted' && !d.isSuppressed && (!onlyCritical || d.severity === 'critical')
    )
    if (targets.length === 0) return

    loading.value = true
    let successCount = 0
    for (const target of targets) {
      try {
        await driftApi.resolveDrift(target.id)
        recordRemediationSuccess()
        target.status = 'in_sync'
        successCount++
      } catch (err) {
        console.warn(`Failed to reconcile ${target.id}:`, err)
      }
    }
    statusMessage.value = {
      type: 'success',
      text: `Auto-reconciled ${successCount} of ${targets.length} drifted workload manifests with Git repository.`,
    }
    await fetchDriftData()
  }

  // Drift Suppression Management
  function toggleDriftSuppression(id: string, reason = 'Operator manual suppression') {
    if (suppressedIds.value[id]) {
      const copy = { ...suppressedIds.value }
      delete copy[id]
      suppressedIds.value = copy
      statusMessage.value = { type: 'info', text: `Suppression removed for workload #${id.slice(0, 8)}.` }
    } else {
      suppressedIds.value = { ...suppressedIds.value, [id]: reason }
      statusMessage.value = { type: 'info', text: `Workload #${id.slice(0, 8)} suppressed from active drift alerts.` }
    }
    saveSuppressedState()
  }

  // Diff Drawer Helpers
  function openDiff(drift: EnrichedDriftRecord) {
    selectedDrift.value = drift
  }

  function closeDiff() {
    selectedDrift.value = null
  }

  function setDiffViewMode(mode: 'split' | 'unified') {
    diffViewMode.value = mode
  }

  function formatDriftStatus(s: string): string {
    if (s === 'drifted') return 'DRIFTED'
    if (s === 'in_sync') return 'IN SYNC'
    return (s || 'UNKNOWN').toUpperCase()
  }

  function formatDate(d: string): string {
    if (!d) return '-'
    try {
      return new Date(d).toLocaleString()
    } catch {
      return d
    }
  }

  onMounted(() => {
    fetchDriftData()
  })

  return {
    drifts,
    enrichedDrifts,
    filteredDrifts,
    loading,
    error,
    activeStatus,
    clusterFilter,
    searchQuery,
    resolvingId,
    selectedDrift,
    diffViewMode,
    statusMessage,
    remediatedTodayCount,
    autoRemediateCritical,
    driftedCount,
    criticalCount,
    inSyncCount,
    syncRate,
    gitReposTracked,
    statusFilters,
    fetchDriftData,
    handleReconcile,
    handleBatchReconcile,
    toggleDriftSuppression,
    openDiff,
    closeDiff,
    setDiffViewMode,
    formatDriftStatus,
    formatDate,
  }
}