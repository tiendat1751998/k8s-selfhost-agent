import { ref, reactive, computed, watch, onMounted } from 'vue'
import { sloApi, dockerApi } from '../api/compute'
import type {
  SLODefinition,
  SLOSnapshot,
  CreateSLOPayload,
  UpdateSLOPayload,
  TimeWindowFilter,
  ServiceOption,
  LatencyPercentiles,
  InspectSLOState,
  BannerMessage,
  SLOFormState,
  EnrichedSLOItem,
  FleetReliabilityPulse,
} from '../types/slo'

export type {
  TimeWindowFilter,
  ServiceOption,
  LatencyPercentiles,
  InspectSLOState,
  BannerMessage,
  SLOFormState,
  EnrichedSLOItem,
  FleetReliabilityPulse,
  SLODefinition,
  SLOSnapshot,
  CreateSLOPayload,
  UpdateSLOPayload,
}

// Error budget calculations & burn rate velocity
export function calculateErrorBudget(target: number, actual: number): number {
  if (actual <= 0) return 0
  const targetErrorRate = (100 - target) / 100
  if (targetErrorRate <= 0) return 100
  const actualErrorRate = Math.max(0, (100 - actual) / 100)
  const consumedFraction = actualErrorRate / targetErrorRate
  return Math.max(0, Math.min(100, (1 - consumedFraction) * 100))
}

export function calculateBurnRateVelocity(burnRate: number, windowStr = '30d'): { hoursToExhaustion: number; statusText: string } {
  const effectiveRate = Math.max(burnRate, 0.01)
  const windowHours = windowStr === '1h' ? 1 : windowStr === '6h' ? 6 : windowStr === '24h' ? 24 : windowStr === '7d' ? 168 : windowStr === '14d' ? 336 : 720
  const hoursRemaining = windowHours / effectiveRate
  let statusText = 'Normal depletion'
  if (effectiveRate >= 14.4) statusText = 'P1 Extreme Fast Burn'
  else if (effectiveRate >= 6.0) statusText = 'P2 Slow Burn Warning'
  else if (effectiveRate >= 2.0) statusText = 'P3 Elevated Depletion'
  else if (effectiveRate <= 1.0) statusText = 'Budget Positive'
  return { hoursToExhaustion: Number(hoursRemaining.toFixed(1)), statusText }
}

export function getTargetLatencyPercentiles(targetLatencyP99?: number): LatencyPercentiles {
  if (!targetLatencyP99 || targetLatencyP99 <= 0) return { p50: 0, p90: 0, p99: 0, p999: 0 }
  return { p50: 0, p90: 0, p99: Math.round(targetLatencyP99), p999: 0 }
}

export function formatLatencyMetric(val?: number): string {
  return (!val || val <= 0) ? '--' : `${val}ms`
}

export function formatPercent(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0.00%'
  if (val < 0) return '--'
  const pct = val > 1 ? val : val * 100
  return `${pct.toFixed(2)}%`
}

export function formatBurnVelocityBadge(
  burnRate: number,
  isExhausted: boolean,
  isUnmeasured: boolean
): { label: string; cls: string } {
  if (isUnmeasured) return { label: 'Unmeasured', cls: 'pill-burn-unmeasured' }
  const rateLabel = burnRate >= 100 ? `${Math.round(burnRate)}x Burn` : `${burnRate.toFixed(1)}x Burn`
  if (isExhausted) return { label: rateLabel, cls: 'pill-burn-exhausted' }
  if (burnRate >= 14.4) return { label: burnRate >= 100 ? rateLabel : `${burnRate.toFixed(1)}x Fast Burn`, cls: 'pill-burn-fast' }
  if (burnRate > 1.0) return { label: burnRate >= 100 ? rateLabel : `${burnRate.toFixed(1)}x Elevated`, cls: 'pill-burn-elevated' }
  return { label: `${burnRate.toFixed(1)}x Normal`, cls: 'pill-burn-normal' }
}

export function getEffectiveBurnRate(rawRate?: number): number {
  return (rawRate === undefined || rawRate === null || isNaN(rawRate)) ? 0 : rawRate
}

export function getBurnRateColor(rate: number): string {
  if (rate <= 1.0) return 'text-emerald'
  if (rate <= 2.5) return 'text-amber'
  return 'text-rose'
}

export function getBudgetBarWidth(budget?: number): number {
  if (budget === undefined || budget === null || isNaN(budget)) return 0
  return Math.min(Math.max(budget, 0), 100)
}

export function enrichSLOItem(def: SLODefinition, snap?: SLOSnapshot): EnrichedSLOItem {
  const target = def.target > 1 ? def.target : Number((def.target * 100).toFixed(2))
  const alertThreshold = def.alert_threshold && def.alert_threshold > 0 ? def.alert_threshold : 1.5
  const isUnmeasured = !snap || snap.budget_status === 'unmeasured' || (typeof snap.actual === 'number' && snap.actual < 0)

  if (isUnmeasured) {
    const rawBurn = snap && typeof snap.burn_rate === 'number' && !isNaN(snap.burn_rate) ? snap.burn_rate : 0
    return {
      id: def.id, service: def.service, target, actual: -1, error_budget: 0,
      burn_rate: rawBurn, alert_threshold: alertThreshold, budget_status: 'unmeasured',
      indicator_type: def.indicator_type || 'availability', window: def.window || '30d',
      query: def.query || '', isFastBurn: false, isExhausted: false, isUnmeasured: true, def, snap,
    }
  }

  let actual = target, burnRate = 1.0, budgetStatus = snap?.budget_status || 'healthy', errorBudget = 100
  if (snap) {
    if (typeof snap.actual === 'number' && !isNaN(snap.actual)) {
      actual = snap.actual > 1 ? snap.actual : (snap.actual === 0 ? 0 : Number((snap.actual * 100).toFixed(2)))
    }
    if (typeof snap.burn_rate === 'number' && !isNaN(snap.burn_rate)) burnRate = snap.burn_rate
    if (typeof snap.error_budget === 'number' && !isNaN(snap.error_budget)) {
      errorBudget = Math.max(0, Math.min(100, snap.error_budget))
    } else {
      errorBudget = calculateErrorBudget(target, actual)
    }
  }

  const isFastBurn = burnRate >= alertThreshold
  const isExhausted = actual === 0 || errorBudget <= 0 || budgetStatus === 'exhausted' || budgetStatus === 'critical'
  if (isExhausted) {
    budgetStatus = 'critical'
    errorBudget = 0
  } else if (isFastBurn && budgetStatus === 'healthy') {
    budgetStatus = 'warning'
  }

  return {
    id: def.id, service: def.service, target, actual, error_budget: errorBudget,
    burn_rate: burnRate, alert_threshold: alertThreshold, budget_status: budgetStatus,
    indicator_type: def.indicator_type || 'availability', window: def.window || '30d',
    query: def.query || '', isFastBurn, isExhausted, isUnmeasured: false, def, snap,
  }
}

export function sortSLOsByUrgency(items: EnrichedSLOItem[]): EnrichedSLOItem[] {
  return [...items].sort((a, b) => {
    const getRank = (item: EnrichedSLOItem) => {
      if (item.isExhausted || item.isFastBurn || item.budget_status === 'critical') return 0
      if (item.budget_status === 'warning') return 1
      if (item.isUnmeasured || item.budget_status === 'unmeasured') return 3
      return 2
    }
    const rankA = getRank(a)
    const rankB = getRank(b)
    if (rankA !== rankB) return rankA - rankB
    if (b.burn_rate !== a.burn_rate) return b.burn_rate - a.burn_rate
    if (a.error_budget !== b.error_budget) return a.error_budget - b.error_budget
    return a.service.localeCompare(b.service)
  })
}

export function enrichAndSortSLOs(definitions: SLODefinition[], snapshots: SLOSnapshot[]): EnrichedSLOItem[] {
  return sortSLOsByUrgency(
    definitions.map(def => enrichSLOItem(def, snapshots.find(s => s.slo_id === def.id || s.service === def.service)))
  )
}

export function deriveFleetPulse(items: EnrichedSLOItem[]): FleetReliabilityPulse {
  const monitoredCount = items.length
  if (monitoredCount === 0) {
    return {
      monitoredCount: 0,
      healthyCount: 0,
      warningCount: 0,
      criticalCount: 0,
      fastBurnCount: 0,
      exhaustedCount: 0,
      unmeasuredCount: 0,
      avgBudgetRemainingPct: 100,
    }
  }

  const measuredItems = items.filter(s => !s.isUnmeasured && s.budget_status !== 'unmeasured' && s.actual >= 0)
  const unmeasuredCount = items.length - measuredItems.length

  const healthyCount = measuredItems.filter(s => s.budget_status === 'healthy' && !s.isFastBurn && !s.isExhausted).length
  const warningCount = measuredItems.filter(s => s.budget_status === 'warning' && !s.isExhausted).length
  const criticalCount = measuredItems.filter(s => s.budget_status === 'critical' || s.isExhausted).length
  const fastBurnCount = measuredItems.filter(s => s.isFastBurn).length
  const exhaustedCount = measuredItems.filter(s => s.isExhausted || s.error_budget <= 0).length

  let avgBudgetRemainingPct = 0
  if (measuredItems.length > 0) {
    const totalBudget = measuredItems.reduce((acc, s) => acc + s.error_budget, 0)
    avgBudgetRemainingPct = Number((totalBudget / measuredItems.length).toFixed(1))
  }

  return {
    monitoredCount,
    healthyCount,
    warningCount,
    criticalCount,
    fastBurnCount,
    exhaustedCount,
    unmeasuredCount,
    avgBudgetRemainingPct,
  }
}

export function useSLOMonitor() {
  const loading = ref(false)
  const actionInProgress = ref(false)
  const error = ref<string | null>(null)
  const bannerMessage = ref<BannerMessage | null>(null)

  const definitions = ref<SLODefinition[]>([])
  const snapshots = ref<SLOSnapshot[]>([])
  const selectedWindowFilter = ref<TimeWindowFilter>('30d')
  const viewMode = ref<'board' | 'table'>('board')

  const showCreateModal = ref(false)
  const showInspectModal = ref(false)
  const isEditing = ref(false)
  const selectedInspectSLO = ref<InspectSLOState | null>(null)

  const realServices = ref<ServiceOption[]>([
    { id: 'custom', name: 'Custom Workload...', desc: 'Enter custom service name' }
  ])
  const loadingServices = ref(false)

  const newSLO = reactive<SLOFormState>({
    selectedService: 'custom',
    customServiceName: '',
    indicator_type: 'availability',
    target: 99.90,
    window: '30d',
    query: 'sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100',
    alert_threshold: 1.5,
    target_latency_p99_ms: 100,
  })

  function getPromQLTemplate(service: string, indicator: string): string {
    if (indicator === 'latency') return `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{service="${service}"}[5m])) by (le)) * 1000 < 100`
    if (indicator === 'cache_hit_rate') return 'sum(rate(redis_keyspace_hits_total[5m])) / (sum(rate(redis_keyspace_hits_total[5m])) + sum(rate(redis_keyspace_hits_total[5m]))) * 100'
    if (indicator === 'error_rate') return `(1 - sum(rate(http_requests_total{service="${service}",status=~"5.."}[5m])) / sum(rate(http_requests_total{service="${service}"}[5m]))) * 100`
    return `sum(rate(http_requests_total{service="${service}",status=~"2..|3.."}[5m])) / sum(rate(http_requests_total{service="${service}"}[5m])) * 100`
  }

  watch([() => newSLO.selectedService, () => newSLO.indicator_type], ([newSvc, newInd]) => {
    if (!isEditing.value) {
      const svc = newSvc === 'custom' ? (newSLO.customServiceName || 'my_service') : newSvc
      newSLO.query = getPromQLTemplate(svc, newInd)
    }
  })

  async function fetchRealServices() {
    loadingServices.value = true
    try {
      const [servicesRes, containersRes] = await Promise.allSettled([
        dockerApi.listServices(),
        dockerApi.listContainers(),
      ])
      const found = new Map<string, ServiceOption>()
      if (servicesRes.status === 'fulfilled' && Array.isArray(servicesRes.value)) {
        for (const s of servicesRes.value) {
          if (s.name) found.set(s.name, { id: s.name, name: s.name, desc: s.image ? `Docker Service (${s.image})` : 'Docker Swarm Service' })
        }
      }
      if (containersRes.status === 'fulfilled' && Array.isArray(containersRes.value)) {
        for (const c of containersRes.value) {
          const name = c.name.replace(/^\//, '')
          if (name && !found.has(name)) {
            found.set(name, { id: name, name, desc: c.image ? `Container (${c.image})` : `Container (${c.status || 'running'})` })
          }
        }
      }
      const list = Array.from(found.values())
      list.push({ id: 'custom', name: 'Custom Workload...', desc: 'Enter custom service name' })
      realServices.value = list
      if (list.length > 0 && (!newSLO.selectedService || !list.some(s => s.id === newSLO.selectedService))) {
        newSLO.selectedService = list[0].id
      }
    } catch (err) {
      console.error('Failed to fetch Docker services:', err)
      realServices.value = [{ id: 'custom', name: 'Custom Workload...', desc: 'Enter custom service name' }]
    } finally {
      loadingServices.value = false
    }
  }

  async function fetchSLOData() {
    loading.value = true
    error.value = null
    try {
      const [defsRes, snapRes] = await Promise.allSettled([
        sloApi.listDefinitions(),
        sloApi.listSnapshots(selectedWindowFilter.value)
      ])
      if (defsRes.status === 'fulfilled') definitions.value = defsRes.value
      if (snapRes.status === 'fulfilled') snapshots.value = snapRes.value
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to retrieve SLO telemetry'
    } finally {
      loading.value = false
    }
  }

  function setWindowFilter(filter: TimeWindowFilter) {
    selectedWindowFilter.value = filter
    fetchSLOData()
  }

  function formatDate(d?: string) {
    if (!d) return '-'
    try {
      return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' })
    } catch {
      return d
    }
  }

  function getSnapshotForDef(defId: string, serviceName: string): SLOSnapshot | undefined {
    return snapshots.value.find(s => s.slo_id === defId || s.service === serviceName)
  }

  function getDefForSnapshot(snap: SLOSnapshot): SLODefinition | undefined {
    return definitions.value.find(d => d.id === snap.slo_id || d.service === snap.service)
  }

  function openCreateModal(defToEdit?: SLODefinition) {
    if (defToEdit) {
      isEditing.value = true
      newSLO.id = defToEdit.id
      const matchingService = realServices.value.find(s => s.id === defToEdit.service)
      newSLO.selectedService = (matchingService && matchingService.id !== 'custom') ? matchingService.id : 'custom'
      newSLO.customServiceName = (matchingService && matchingService.id !== 'custom') ? '' : defToEdit.service
      newSLO.indicator_type = defToEdit.indicator_type
      newSLO.target = defToEdit.target > 1 ? defToEdit.target : Number((defToEdit.target * 100).toFixed(2))
      newSLO.window = defToEdit.window || '30d'
      newSLO.query = defToEdit.query || getPromQLTemplate(defToEdit.service, defToEdit.indicator_type)
      newSLO.alert_threshold = defToEdit.alert_threshold || 1.5
    } else {
      isEditing.value = false
      newSLO.id = undefined
      newSLO.selectedService = realServices.value[0]?.id || 'custom'
      newSLO.customServiceName = ''
      newSLO.indicator_type = 'availability'
      newSLO.target = 99.90
      newSLO.window = '30d'
      newSLO.query = getPromQLTemplate('my_service', 'availability')
      newSLO.alert_threshold = 1.5
    }
    showCreateModal.value = true
  }

  function closeCreateModal() {
    showCreateModal.value = false
    isEditing.value = false
  }

  function openInspect(def?: SLODefinition, snap?: SLOSnapshot) {
    if (!def && snap) def = getDefForSnapshot(snap)
    if (!snap && def) snap = getSnapshotForDef(def.id, def.service)
    selectedInspectSLO.value = { def, snap }
    showInspectModal.value = true
  }

  function closeInspect() {
    showInspectModal.value = false
    selectedInspectSLO.value = null
  }

  async function handleSaveSLO() {
    const serviceName = newSLO.selectedService === 'custom' ? newSLO.customServiceName.trim() : newSLO.selectedService
    if (!serviceName) {
      bannerMessage.value = { type: 'error', text: 'Please specify a service name for the SLO objective.' }
      return
    }
    actionInProgress.value = true
    bannerMessage.value = null
    try {
      const payload = {
        service: serviceName,
        target: Number(newSLO.target),
        indicator_type: newSLO.indicator_type,
        window: newSLO.window,
        query: newSLO.query,
        alert_threshold: Number(newSLO.alert_threshold) || 1.5,
      }
      if (isEditing.value && newSLO.id) {
        await sloApi.updateDefinition(newSLO.id, payload as UpdateSLOPayload)
        bannerMessage.value = { type: 'success', text: `SLO target objective updated for service "${serviceName}".` }
      } else {
        await sloApi.createDefinition(payload as CreateSLOPayload)
        bannerMessage.value = { type: 'success', text: `SLO target objective successfully created for service "${serviceName}". Telemetry initialized.` }
      }
      showCreateModal.value = false
      await fetchSLOData()
    } catch (err: unknown) {
      bannerMessage.value = { type: 'error', text: err instanceof Error ? err.message : 'Failed to save SLO definition' }
    } finally {
      actionInProgress.value = false
    }
  }

  async function handleCreateSLO(payload: CreateSLOPayload) {
    actionInProgress.value = true
    bannerMessage.value = null
    try {
      await sloApi.createDefinition(payload)
      showCreateModal.value = false
      bannerMessage.value = { type: 'success', text: `SLO target objective successfully armed for service "${payload.service}". Telemetry initialized.` }
      await fetchSLOData()
    } catch (err: unknown) {
      bannerMessage.value = { type: 'error', text: err instanceof Error ? err.message : 'Failed to create SLO definition' }
    } finally {
      actionInProgress.value = false
    }
  }

  async function handleTriggerAlert(defId: string, serviceName: string) {
    actionInProgress.value = true
    try {
      const res = await sloApi.triggerBurnAlert(defId)
      bannerMessage.value = { type: 'warning', text: res.message || `Fast burn rate alert triggered for ${serviceName}: elevated error rate detected.` }
      await fetchSLOData()
    } catch (err: unknown) {
      bannerMessage.value = { type: 'error', text: err instanceof Error ? err.message : 'Failed to trigger burn alert' }
    } finally {
      actionInProgress.value = false
    }
  }

  async function handleDeleteSLO(defId: string, serviceName: string) {
    if (!confirm(`Are you sure you want to delete the SLO objective for "${serviceName}"?`)) return
    actionInProgress.value = true
    try {
      await sloApi.deleteDefinition(defId)
      bannerMessage.value = { type: 'success', text: `SLO definition for "${serviceName}" deleted.` }
      if (showInspectModal.value) showInspectModal.value = false
      await fetchSLOData()
    } catch (err: unknown) {
      bannerMessage.value = { type: 'error', text: err instanceof Error ? err.message : 'Failed to delete SLO definition' }
    } finally {
      actionInProgress.value = false
    }
  }

  // Computed Enriched SLOs and Fleet Metrics
  const enrichedSortedSLOs = computed(() => enrichAndSortSLOs(definitions.value, snapshots.value))
  const fleetPulse = computed(() => deriveFleetPulse(enrichedSortedSLOs.value))

  const totalSLOs = computed(() => definitions.value.length)
  const healthySLOs = computed(() => fleetPulse.value.healthyCount)
  const warningSLOs = computed(() => fleetPulse.value.warningCount)
  const criticalSLOs = computed(() => fleetPulse.value.criticalCount)
  const healthySloCount = computed(() => fleetPulse.value.healthyCount)
  const fastBurnCount = computed(() => fleetPulse.value.fastBurnCount)
  const exhaustedBudgetCount = computed(() => fleetPulse.value.exhaustedCount)
  const avgErrorBudgetPct = computed(() => fleetPulse.value.avgBudgetRemainingPct)
  const activeBurnAlerts = computed(() => fleetPulse.value.fastBurnCount)

  const avgBurnRateNum = computed(() => {
    if (snapshots.value.length === 0) return 1.0
    const total = snapshots.value.reduce((acc, s) => acc + (s.burn_rate || 0), 0)
    return Number((total / snapshots.value.length).toFixed(2))
  })

  const avgBurnRate = computed(() => (snapshots.value.length === 0 ? '—' : `${avgBurnRateNum.value.toFixed(2)}x`))

  onMounted(() => {
    fetchSLOData()
    fetchRealServices()
  })

  const editingDefinition = computed<SLODefinition | null>(() => {
    if (!isEditing.value || !newSLO.id) return null
    return definitions.value.find(d => d.id === newSLO.id) || null
  })

  return {
    loading, actionInProgress, error, bannerMessage,
    definitions, snapshots, selectedWindowFilter, viewMode,
    showCreateModal, showInspectModal, isEditing, editingDefinition, selectedInspectSLO,
    realServices, loadingServices, newSLO,
    totalSLOs, healthySLOs, warningSLOs, criticalSLOs,
    healthySloCount, fastBurnCount, exhaustedBudgetCount, avgErrorBudgetPct,
    enrichedSortedSLOs, fleetPulse, activeBurnAlerts, avgBurnRate, avgBurnRateNum,
    fetchSLOData, fetchRealServices, setWindowFilter,
    calculateErrorBudget, calculateBurnRateVelocity, getTargetLatencyPercentiles,
    formatLatencyMetric, formatPercent, formatBurnVelocityBadge, getEffectiveBurnRate, getBurnRateColor,
    getBudgetBarWidth, formatDate, getSnapshotForDef, getDefForSnapshot,
    openCreateModal, closeCreateModal, openInspect, closeInspect,
    handleSaveSLO, handleCreateSLO, handleTriggerAlert, handleDeleteSLO,
  }
}
