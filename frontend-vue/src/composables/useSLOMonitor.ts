import { ref, reactive, computed, watch, onMounted } from 'vue'
import {
  sloApi,
  dockerApi,
  type SLODefinition,
  type SLOSnapshot,
  type CreateSLOPayload,
  type UpdateSLOPayload
} from '../api/compute'

export type TimeWindowFilter = '1h' | '6h' | '24h' | '30d'

export interface ServiceOption {
  id: string
  name: string
  desc: string
}

export interface SLOFormState {
  id?: string
  selectedService: string
  customServiceName: string
  indicator_type: 'availability' | 'latency' | 'error_rate' | 'cache_hit_rate' | string
  target: number
  window: string
  query: string
  alert_threshold: number
  target_latency_p99_ms?: number
}

export interface LatencyPercentiles {
  p50: number
  p90: number
  p99: number
  p999: number
}

export interface InspectSLOState {
  def?: SLODefinition
  snap?: SLOSnapshot
}

export interface BannerMessage {
  type: 'success' | 'warning' | 'error'
  text: string
}

export function useSLOMonitor() {
  // State
  const loading = ref(false)
  const actionInProgress = ref(false)
  const error = ref<string | null>(null)
  const bannerMessage = ref<BannerMessage | null>(null)

  const definitions = ref<SLODefinition[]>([])
  const snapshots = ref<SLOSnapshot[]>([])
  const selectedWindowFilter = ref<TimeWindowFilter>('30d')

  // Modals & Drawers
  const showCreateModal = ref(false)
  const showInspectModal = ref(false)
  const isEditing = ref(false)
  const selectedInspectSLO = ref<InspectSLOState | null>(null)

  // Real cluster services catalog fetched dynamically from Docker API
  const realServices = ref<ServiceOption[]>([
    { id: 'custom', name: 'Custom Workload...', desc: 'Enter custom service name' }
  ])
  const loadingServices = ref(false)

  // SLO Form State
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
    if (indicator === 'latency') {
      return `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{service="${service}"}[5m])) by (le)) * 1000 < 100`
    }
    if (indicator === 'cache_hit_rate') {
      return 'sum(rate(redis_keyspace_hits_total[5m])) / (sum(rate(redis_keyspace_hits_total[5m])) + sum(rate(redis_keyspace_hits_total[5m]))) * 100'
    }
    if (indicator === 'error_rate') {
      return `(1 - sum(rate(http_requests_total{service="${service}",status=~"5.."}[5m])) / sum(rate(http_requests_total{service="${service}"}[5m]))) * 100`
    }
    return `sum(rate(http_requests_total{service="${service}",status=~"2..|3.."}[5m])) / sum(rate(http_requests_total{service="${service}"}[5m])) * 100`
  }

  // Watch form changes to automatically suggest PromQL
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
          if (s.name) {
            found.set(s.name, {
              id: s.name,
              name: s.name,
              desc: s.image ? `Docker Service (${s.image})` : 'Docker Swarm Service'
            })
          }
        }
      }

      if (containersRes.status === 'fulfilled' && Array.isArray(containersRes.value)) {
        for (const c of containersRes.value) {
          const name = c.name.replace(/^\//, '')
          if (name && !found.has(name)) {
            found.set(name, {
              id: name,
              name: name,
              desc: c.image ? `Container (${c.image})` : `Container (${c.status || 'running'})`
            })
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
      realServices.value = [
        { id: 'custom', name: 'Custom Workload...', desc: 'Enter custom service name' }
      ]
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

      if (defsRes.status === 'fulfilled') {
        definitions.value = defsRes.value
      }
      if (snapRes.status === 'fulfilled') {
        snapshots.value = snapRes.value
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to retrieve SLO telemetry'
      error.value = msg
    } finally {
      loading.value = false
    }
  }

  function setWindowFilter(filter: TimeWindowFilter) {
    selectedWindowFilter.value = filter
    fetchSLOData()
  }

  // Error budget calculations & burn rate velocity
  function calculateErrorBudget(target: number, actual: number): number {
    const targetErrorRate = (100 - target) / 100
    if (targetErrorRate <= 0) return 100
    const actualErrorRate = Math.max(0, (100 - actual) / 100)
    const consumedFraction = actualErrorRate / targetErrorRate
    return Math.max(0, Math.min(100, (1 - consumedFraction) * 100))
  }

  function calculateBurnRateVelocity(burnRate: number, windowStr = '30d'): { hoursToExhaustion: number; statusText: string } {
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

  function getTargetLatencyPercentiles(targetObjective = 99.9): LatencyPercentiles {
    const baseP50 = 18
    const baseP90 = 42
    const baseP99 = 85
    const multiplier = targetObjective >= 99.99 ? 0.7 : targetObjective >= 99.9 ? 1.0 : 1.3
    return {
      p50: Math.round(baseP50 * multiplier),
      p90: Math.round(baseP90 * multiplier),
      p99: Math.round(baseP99 * multiplier),
      p999: Math.round(baseP99 * 1.8 * multiplier)
    }
  }

  // Formatters
  function formatPercent(val?: number): string {
    if (val === undefined || val === null || isNaN(val)) return '0.00%'
    const pct = val > 1 ? val : val * 100
    return `${pct.toFixed(2)}%`
  }

  function getEffectiveBurnRate(rawRate?: number): number {
    if (rawRate === undefined || rawRate === null) return 0
    return rawRate
  }

  function getBurnRateColor(rate: number): string {
    if (rate <= 1.0) return 'text-emerald'
    if (rate <= 2.5) return 'text-amber'
    return 'text-rose'
  }

  function getBudgetBarWidth(budget: number): number {
    return Math.min(Math.max(budget, 0), 100)
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
      if (matchingService && matchingService.id !== 'custom') {
        newSLO.selectedService = matchingService.id
        newSLO.customServiceName = ''
      } else {
        newSLO.selectedService = 'custom'
        newSLO.customServiceName = defToEdit.service
      }
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
    if (!def && snap) {
      def = getDefForSnapshot(snap)
    }
    if (!snap && def) {
      snap = getSnapshotForDef(def.id, def.service)
    }
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
      if (isEditing.value && newSLO.id) {
        const updatePayload: UpdateSLOPayload = {
          service: serviceName,
          target: Number(newSLO.target),
          indicator_type: newSLO.indicator_type,
          window: newSLO.window,
          query: newSLO.query,
          alert_threshold: Number(newSLO.alert_threshold) || 1.5,
        }
        await sloApi.updateDefinition(newSLO.id, updatePayload)
        bannerMessage.value = {
          type: 'success',
          text: `SLO target objective updated for service "${serviceName}".`
        }
      } else {
        const createPayload: CreateSLOPayload = {
          service: serviceName,
          target: Number(newSLO.target),
          indicator_type: newSLO.indicator_type,
          window: newSLO.window,
          query: newSLO.query,
          alert_threshold: Number(newSLO.alert_threshold) || 1.5,
        }
        await sloApi.createDefinition(createPayload)
        bannerMessage.value = {
          type: 'success',
          text: `SLO target objective successfully created for service "${serviceName}". Telemetry initialized.`
        }
      }
      showCreateModal.value = false
      await fetchSLOData()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to save SLO definition'
      bannerMessage.value = { type: 'error', text: msg }
    } finally {
      actionInProgress.value = false
    }
  }

  async function handleTriggerAlert(defId: string, serviceName: string) {
    actionInProgress.value = true
    try {
      const res = await sloApi.triggerBurnAlert(defId)
      bannerMessage.value = {
        type: 'warning',
        text: res.message || `🚨 Fast burn rate alert triggered for ${serviceName}: elevated error rate detected!`,
      }
      await fetchSLOData()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to trigger burn alert'
      bannerMessage.value = { type: 'error', text: msg }
    } finally {
      actionInProgress.value = false
    }
  }

  async function handleDeleteSLO(defId: string, serviceName: string) {
    if (!confirm(`Are you sure you want to delete the SLO objective for "${serviceName}"?`)) {
      return
    }

    actionInProgress.value = true
    try {
      await sloApi.deleteDefinition(defId)
      bannerMessage.value = {
        type: 'success',
        text: `SLO definition for "${serviceName}" deleted.`,
      }
      if (showInspectModal.value) {
        showInspectModal.value = false
      }
      await fetchSLOData()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete SLO definition'
      bannerMessage.value = { type: 'error', text: msg }
    } finally {
      actionInProgress.value = false
    }
  }

  // Computed Metrics
  const totalSLOs = computed(() => definitions.value.length)
  const healthySLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'healthy').length)
  const warningSLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'warning').length)
  const criticalSLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'critical').length)

  const avgBurnRate = computed(() => {
    if (snapshots.value.length === 0) return '—'
    const total = snapshots.value.reduce((acc, s) => acc + (s.burn_rate || 0), 0)
    return `${(total / snapshots.value.length).toFixed(2)}x`
  })

  onMounted(() => {
    fetchSLOData()
    fetchRealServices()
  })

  return {
    loading,
    actionInProgress,
    error,
    bannerMessage,
    definitions,
    snapshots,
    selectedWindowFilter,
    showCreateModal,
    showInspectModal,
    isEditing,
    selectedInspectSLO,
    realServices,
    loadingServices,
    newSLO,
    totalSLOs,
    healthySLOs,
    warningSLOs,
    criticalSLOs,
    avgBurnRate,
    fetchSLOData,
    fetchRealServices,
    setWindowFilter,
    calculateErrorBudget,
    calculateBurnRateVelocity,
    getTargetLatencyPercentiles,
    formatPercent,
    getEffectiveBurnRate,
    getBurnRateColor,
    getBudgetBarWidth,
    formatDate,
    getSnapshotForDef,
    getDefForSnapshot,
    openCreateModal,
    closeCreateModal,
    openInspect,
    closeInspect,
    handleSaveSLO,
    handleTriggerAlert,
    handleDeleteSLO,
  }
}
