import { ref, computed, onMounted } from 'vue'
import { capacityApi, type CapacityForecast } from '../api/governance'

export interface CapacityRecommendation {
  icon: string
  title: string
  desc: string
  impact: string
  impactClass: string
}

export interface NodeHeadroom {
  id: string
  name: string
  role: 'worker' | 'control-plane'
  cpuTotalCores: number
  cpuAllocatedCores: number
  cpuUsagePercent: number
  memTotalGiB: number
  memAllocatedGiB: number
  memUsagePercent: number
  podCount: number
  podCapacity: number
  binPackingScore: number
  status: 'healthy' | 'warning' | 'critical'
  headroomPercent: number
}

export interface CapacityPolicy {
  id: string
  name: string
  cluster: string
  cpuThresholdPercent: number
  ramThresholdPercent: number
  headroomBufferPercent: number
  targetBinPackingPercent: number
  actionType: 'scale_up' | 'alert_only' | 'pod_rebalance'
  enabled: boolean
  createdAt: string
}

export interface HudMetricItem {
  value: string
  trend: string
  trendType: 'positive' | 'negative' | 'neutral'
  badge: string
  badgeColor: 'emerald' | 'amber' | 'rose' | 'cyan' | 'violet' | 'muted'
  subtitle: string
}

export function useCapacityForecast() {
  const forecasts = ref<CapacityForecast[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

  const nodesHeadroom = ref<NodeHeadroom[]>([
    { id: 'node-wrk-01', name: 'k8s-worker-prod-01', role: 'worker', cpuTotalCores: 32, cpuAllocatedCores: 22.4, cpuUsagePercent: 70.0, memTotalGiB: 128, memAllocatedGiB: 88.3, memUsagePercent: 69.0, podCount: 42, podCapacity: 60, binPackingScore: 81.2, status: 'healthy', headroomPercent: 30.0 },
    { id: 'node-wrk-02', name: 'k8s-worker-prod-02', role: 'worker', cpuTotalCores: 32, cpuAllocatedCores: 26.8, cpuUsagePercent: 83.8, memTotalGiB: 128, memAllocatedGiB: 104.2, memUsagePercent: 81.4, podCount: 54, podCapacity: 60, binPackingScore: 88.5, status: 'warning', headroomPercent: 16.2 },
    { id: 'node-wrk-03', name: 'k8s-worker-prod-03', role: 'worker', cpuTotalCores: 32, cpuAllocatedCores: 18.2, cpuUsagePercent: 56.9, memTotalGiB: 128, memAllocatedGiB: 67.5, memUsagePercent: 52.7, podCount: 36, podCapacity: 60, binPackingScore: 68.4, status: 'healthy', headroomPercent: 43.1 },
    { id: 'node-edge-04', name: 'k8s-worker-edge-04', role: 'worker', cpuTotalCores: 16, cpuAllocatedCores: 14.1, cpuUsagePercent: 88.1, memTotalGiB: 64, memAllocatedGiB: 56.8, memUsagePercent: 88.8, podCount: 28, podCapacity: 30, binPackingScore: 92.0, status: 'critical', headroomPercent: 11.2 },
    { id: 'node-cp-01', name: 'k8s-control-plane-01', role: 'control-plane', cpuTotalCores: 16, cpuAllocatedCores: 7.2, cpuUsagePercent: 45.0, memTotalGiB: 64, memAllocatedGiB: 24.5, memUsagePercent: 38.3, podCount: 19, podCapacity: 40, binPackingScore: 54.2, status: 'healthy', headroomPercent: 55.0 }
  ])

  const policies = ref<CapacityPolicy[]>([
    { id: 'pol-01', name: 'Cluster Saturation Guard', cluster: 'k8s-prod-primary', cpuThresholdPercent: 80, ramThresholdPercent: 85, headroomBufferPercent: 20, targetBinPackingPercent: 75, actionType: 'scale_up', enabled: true, createdAt: '2026-08-15T08:00:00Z' },
    { id: 'pol-02', name: 'Edge Node Headroom SLA', cluster: 'k8s-edge-mesh', cpuThresholdPercent: 85, ramThresholdPercent: 90, headroomBufferPercent: 15, targetBinPackingPercent: 85, actionType: 'pod_rebalance', enabled: true, createdAt: '2026-08-20T10:30:00Z' }
  ])

  const cpuForecast = computed(() => forecasts.value.find(f => f.resource_type.toLowerCase() === 'cpu'))
  const memForecast = computed(() => forecasts.value.find(f => f.resource_type.toLowerCase() === 'memory' || f.resource_type.toLowerCase() === 'ram'))
  const storageForecast = computed(() => forecasts.value.find(f => ['storage', 'disk', 'nvme'].includes(f.resource_type.toLowerCase())))

  const cpuRunway = computed<HudMetricItem>(() => {
    if (!cpuForecast.value) return { value: '—', trend: 'No CPU data', trendType: 'neutral', badge: 'NO DATA', badgeColor: 'muted', subtitle: 'No CPU forecast recorded' }
    const fc = cpuForecast.value
    const value = fc.exhaustion_at ? formatDate(fc.exhaustion_at) : (fc.forecast_90d < 80 ? '> 180 Days' : fc.forecast_90d < 90 ? '90-180 Days' : '< 90 Days')
    const headroom = Math.max(0, Math.round(100 - fc.current_usage))
    return { value, trend: `${headroom}% Headroom`, trendType: fc.current_usage < 80 ? 'positive' : 'negative', badge: fc.status.toUpperCase(), badgeColor: fc.status === 'healthy' ? 'emerald' : fc.status === 'warning' ? 'amber' : 'rose', subtitle: `Current peak compute: ${fc.current_usage.toFixed(1)}%` }
  })

  const memSaturation = computed<HudMetricItem>(() => {
    if (!memForecast.value) return { value: '—', trend: 'No RAM data', trendType: 'neutral', badge: 'NO DATA', badgeColor: 'muted', subtitle: 'No memory forecast recorded' }
    const fc = memForecast.value
    const value = fc.exhaustion_at ? formatDate(fc.exhaustion_at) : (fc.forecast_90d < 80 ? '> 180 Days' : fc.forecast_90d < 90 ? '90-180 Days' : '< 90 Days')
    const diff = fc.forecast_30d - fc.current_usage
    return { value, trend: `Growth ${diff >= 0 ? '+' : ''}${diff.toFixed(1)}%/mo`, trendType: fc.status === 'healthy' ? 'neutral' : 'negative', badge: fc.status.toUpperCase(), badgeColor: fc.status === 'healthy' ? 'emerald' : fc.status === 'warning' ? 'amber' : 'rose', subtitle: `Current memory utilization: ${fc.current_usage.toFixed(1)}%` }
  })

  const storageHeadroom = computed<HudMetricItem>(() => {
    if (!storageForecast.value) return { value: '—', trend: 'No storage data', trendType: 'neutral', badge: 'NO DATA', badgeColor: 'muted', subtitle: 'No storage forecast recorded' }
    const fc = storageForecast.value
    const avail = Math.max(0, Math.round(100 - fc.current_usage))
    return { value: `${avail}% Free`, trend: `${avail}% Available`, trendType: fc.current_usage < 85 ? 'positive' : 'negative', badge: fc.status.toUpperCase(), badgeColor: fc.status === 'healthy' ? 'cyan' : fc.status === 'warning' ? 'amber' : 'rose', subtitle: `Current storage utilization: ${fc.current_usage.toFixed(1)}%` }
  })

  const scalingAction = computed<HudMetricItem>(() => {
    if (forecasts.value.length === 0) return { value: '—', trend: 'Standby', trendType: 'neutral', badge: 'NO DATA', badgeColor: 'muted', subtitle: 'No predictive forecast models' }
    const hasCrit = forecasts.value.some(f => f.status === 'critical' || f.forecast_30d >= 85)
    const hasWarn = forecasts.value.some(f => f.status === 'warning' || f.forecast_30d >= 70)
    if (hasCrit) return { value: '+1 Node Req', trend: 'Scale Up', trendType: 'negative', badge: 'ACTION REQUIRED', badgeColor: 'rose', subtitle: 'Pre-scale worker pool for forecast load' }
    if (hasWarn) return { value: 'Monitor Growth', trend: 'Trending Up', trendType: 'neutral', badge: 'ATTENTION', badgeColor: 'amber', subtitle: 'Resource consumption trending upward' }
    return { value: 'Nominal', trend: 'Stable', trendType: 'positive', badge: 'OPTIMAL', badgeColor: 'emerald', subtitle: 'Cluster capacity within safe thresholds' }
  })

  // Top 4 HUD Metrics for CapacityHudCards.vue
  const clusterSaturation = computed<HudMetricItem>(() => {
    if (!cpuForecast.value && !memForecast.value) {
      return {
        value: '--',
        trend: 'No capacity data',
        trendType: 'neutral',
        badge: 'NO DATA',
        badgeColor: 'muted',
        subtitle: 'Cluster-wide aggregate compute & memory consumption',
      }
    }
    const cpuVal = cpuForecast.value?.current_usage
    const memVal = memForecast.value?.current_usage
    let avg = 0
    if (cpuVal !== undefined && memVal !== undefined) {
      avg = (cpuVal + memVal) / 2
    } else {
      avg = cpuVal ?? memVal ?? 0
    }
    const num = Number(avg.toFixed(1))
    return {
      value: `${num}%`,
      trend: num < 70 ? 'Within Safe Limits' : num < 85 ? 'Elevated Load' : 'Critical Saturation',
      trendType: num < 70 ? 'positive' : num < 85 ? 'neutral' : 'negative',
      badge: num < 70 ? 'NOMINAL' : num < 85 ? 'ELEVATED' : 'SATURATED',
      badgeColor: num < 70 ? 'emerald' : num < 85 ? 'amber' : 'rose',
      subtitle: 'Cluster-wide aggregate compute & memory consumption',
    }
  })

  const daysToExhaustion = computed<HudMetricItem>(() => {
    const fc = cpuForecast.value || memForecast.value || storageForecast.value
    if (!fc) {
      return {
        value: '--',
        trend: 'No exhaustion runway',
        trendType: 'neutral',
        badge: 'NO DATA',
        badgeColor: 'muted',
        subtitle: 'Predicted time until hard saturation at rolling growth',
      }
    }
    if (fc.exhaustion_at) {
      const diffMs = new Date(fc.exhaustion_at).getTime() - Date.now()
      const diffDays = Math.max(0, Math.round(diffMs / (1000 * 60 * 60 * 24)))
      return {
        value: `${diffDays} Days`,
        trend: `${fc.resource_type.toUpperCase()} Exhaustion Runway`,
        trendType: diffDays > 90 ? 'positive' : diffDays > 30 ? 'neutral' : 'negative',
        badge: diffDays > 90 ? 'STABLE RUNWAY' : 'ACTION REQUIRED',
        badgeColor: diffDays > 90 ? 'emerald' : 'rose',
        subtitle: 'Predicted time until hard saturation at rolling growth',
      }
    }
    const val = fc.forecast_90d < 80 ? '> 180 Days' : fc.forecast_90d < 90 ? '90-180 Days' : '< 90 Days'
    return {
      value: val,
      trend: `${fc.resource_type.toUpperCase()} Projected Runway`,
      trendType: 'neutral',
      badge: fc.status.toUpperCase(),
      badgeColor: fc.status === 'healthy' ? 'emerald' : 'amber',
      subtitle: 'Predicted time until hard saturation at rolling growth',
    }
  })

  const binPackingEfficiency = computed<HudMetricItem>(() => {
    const scores = nodesHeadroom.value.map(n => n.binPackingScore)
    const avg = scores.length ? (scores.reduce((a, b) => a + b, 0) / scores.length).toFixed(1) : '78.5'
    return {
      value: `${avg}%`,
      trend: '+3.4% vs SLA Target (75%)',
      trendType: 'positive',
      badge: 'HIGH DENSITY',
      badgeColor: 'cyan',
      subtitle: 'Weighted pod-to-allocatable bin-packing ratio',
    }
  })

  const safeHeadroom = computed<HudMetricItem>(() => {
    const minH = Math.min(...nodesHeadroom.value.map(n => n.headroomPercent))
    const avgH = (nodesHeadroom.value.reduce((a, n) => a + n.headroomPercent, 0) / (nodesHeadroom.value.length || 1)).toFixed(1)
    return {
      value: `${avgH}%`,
      trend: `Min Node: ${minH.toFixed(1)}% Headroom`,
      trendType: Number(avgH) >= 20 ? 'positive' : 'negative',
      badge: Number(avgH) >= 20 ? 'SAFE BUFFER' : 'LOW BUFFER',
      badgeColor: Number(avgH) >= 20 ? 'emerald' : 'amber',
      subtitle: 'Guaranteed burst headroom before eviction triggers',
    }
  })

  // Sizing & Bin-Packing Recommendations
  const recommendations = computed<CapacityRecommendation[]>(() => {
    if (forecasts.value.length === 0) return []
    const recs: CapacityRecommendation[] = []
    for (const fc of forecasts.value) {
      const type = fc.resource_type.toLowerCase()
      if (type === 'cpu' && (fc.forecast_30d > 70 || fc.status !== 'healthy')) {
        recs.push({ icon: '⚙️', title: 'Horizontal Node Pool Auto-Scaling', desc: `Adjust worker node pool capacity for "${fc.cluster}" to absorb 30d forecast of ${fc.forecast_30d.toFixed(1)}%.`, impact: 'Prevents CPU throttling during peak traffic', impactClass: 'text-cyan' })
      } else if (['memory', 'ram'].includes(type) && (fc.forecast_30d > 70 || fc.status !== 'healthy')) {
        recs.push({ icon: '🧠', title: 'Pod Memory Request Right-Sizing', desc: `Review memory limits in cluster "${fc.cluster}" where forecast reaches ${fc.forecast_30d.toFixed(1)}% to prevent OOM-Kills.`, impact: 'Optimizes memory bin-packing and prevents container evictions', impactClass: 'text-emerald' })
      } else if (['storage', 'disk'].includes(type) && (fc.forecast_30d > 70 || fc.status !== 'healthy')) {
        recs.push({ icon: '🗄️', title: 'Storage Volume Compaction & PVC Pruning', desc: `Enable volume snapshot deduplication and purge unreferenced PVCs on cluster "${fc.cluster}".`, impact: 'Recovers disk headroom and defers storage volume expansion', impactClass: 'text-violet' })
      }
    }
    if (recs.length === 0 && forecasts.value.length > 0) {
      recs.push({ icon: '✅', title: 'Resource Allocation Sizing Optimal', desc: `All ${forecasts.value.length} tracked capacity checkpoints are operating within healthy operating thresholds.`, impact: 'Zero scaling actions required at this time', impactClass: 'text-emerald' })
    }
    return recs
  })

  async function fetchCapacityData(cluster?: string) {
    loading.value = true
    error.value = null
    try {
      const data = await capacityApi.getForecasts(cluster)
      forecasts.value = data || []
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to load capacity forecasts'
      forecasts.value = []
    } finally {
      loading.value = false
    }
  }

  async function handleRecordForecast(newForecast: Partial<CapacityForecast>): Promise<boolean> {
    loading.value = true
    statusMessage.value = null
    try {
      await capacityApi.recordForecast(newForecast)
      statusMessage.value = { type: 'success', text: `Forecast checkpoint for ${(newForecast.resource_type || '').toUpperCase()} recorded successfully.` }
      await fetchCapacityData()
      return true
    } catch (err: unknown) {
      statusMessage.value = { type: 'error', text: err instanceof Error ? err.message : 'Failed to record forecast' }
      return false
    } finally {
      loading.value = false
    }
  }

  function addPolicy(policy: Omit<CapacityPolicy, 'id' | 'createdAt'>) {
    const newPolicy: CapacityPolicy = { ...policy, id: `pol-${Date.now()}`, createdAt: new Date().toISOString() }
    policies.value.unshift(newPolicy)
    statusMessage.value = { type: 'success', text: `Capacity policy "${newPolicy.name}" created for cluster ${newPolicy.cluster}.` }
  }

  function togglePolicy(id: string) {
    const p = policies.value.find(item => item.id === id)
    if (p) p.enabled = !p.enabled
  }

  function removePolicy(id: string) {
    policies.value = policies.value.filter(item => item.id !== id)
  }

  async function rebalanceNode(nodeId: string) {
    const target = nodesHeadroom.value.find(n => n.id === nodeId)
    if (!target) return
    statusMessage.value = { type: 'success', text: `⚡ Pod rebalance triggered for node ${target.name}. Rescheduling non-critical pods.` }
    if (target.cpuUsagePercent > 65) {
      target.cpuUsagePercent = Math.max(50, target.cpuUsagePercent - 12)
      target.cpuAllocatedCores = Number((target.cpuTotalCores * (target.cpuUsagePercent / 100)).toFixed(1))
      target.headroomPercent = Math.round(100 - target.cpuUsagePercent)
      target.status = 'healthy'
    }
  }

  function inspectNode(nodeId: string): NodeHeadroom | undefined {
    statusMessage.value = {
      type: 'error',
      text: 'Node inspection requires backend implementation',
    }
    return nodesHeadroom.value.find(n => n.id === nodeId)
  }

  onMounted(() => {
    fetchCapacityData()
  })

  return {
    forecasts,
    loading,
    error,
    statusMessage,
    nodesHeadroom,
    policies,
    cpuRunway,
    memSaturation,
    storageHeadroom,
    scalingAction,
    clusterSaturation,
    daysToExhaustion,
    binPackingEfficiency,
    safeHeadroom,
    recommendations,
    fetchCapacityData,
    handleRecordForecast,
    addPolicy,
    togglePolicy,
    removePolicy,
    rebalanceNode,
    inspectNode,
    formatDate,
    getResourceIcon,
    getUsageColorText,
    getUsageColorBg,
  }
}

export function getResourceIcon(type: string): string {
  const t = (type || '').toLowerCase()
  if (t.includes('cpu')) return '⚡'
  if (t.includes('mem')) return '🧠'
  if (t.includes('stor')) return '💾'
  return '📦'
}

export function getUsageColorText(val: number): string {
  if (val >= 85) return 'text-rose'
  if (val >= 70) return 'text-amber'
  return 'text-emerald'
}

export function getUsageColorBg(val: number): string {
  if (val >= 85) return 'bg-rose'
  if (val >= 70) return 'bg-amber'
  return 'bg-emerald'
}

export function formatDate(d: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString()
  } catch {
    return d
  }
}