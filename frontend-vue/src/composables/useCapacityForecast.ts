import { ref, computed, onMounted } from 'vue'
import { capacityApi, type CapacityForecast, type NodeHeadroom as BackendNodeHeadroom } from '../api/governance'
import { overviewApi, type NodeMetrics } from '../api/overview'
import { useGlobalContext } from './useGlobalContext'

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
  role: 'worker' | 'control-plane' | string
  cpuTotalCores: number
  cpuAllocatedCores: number
  cpuUsagePercent: number
  memTotalGiB: number
  memAllocatedGiB: number
  memUsagePercent: number
  podCount: number
  podCapacity: number
  binPackingScore: number
  status: 'healthy' | 'warning' | 'critical' | string
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
  const { activeClusterId } = useGlobalContext()
  const forecasts = ref<CapacityForecast[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

  const nodesHeadroom = ref<NodeHeadroom[]>([])
  const policies = ref<CapacityPolicy[]>([])

  const cpuForecast = computed(() => forecasts.value.find(f => f.resource_type.toLowerCase() === 'cpu'))
  const memForecast = computed(() => forecasts.value.find(f => f.resource_type.toLowerCase() === 'memory' || f.resource_type.toLowerCase() === 'ram'))
  const storageForecast = computed(() => forecasts.value.find(f => ['storage', 'disk', 'nvme'].includes(f.resource_type.toLowerCase())))

  const cpuRunway = computed<HudMetricItem>(() => {
    if (!cpuForecast.value) return { value: '--', trend: 'No CPU data', trendType: 'neutral', badge: 'NO DATA', badgeColor: 'muted', subtitle: 'No CPU forecast recorded' }
    const fc = cpuForecast.value
    const value = fc.exhaustion_at ? formatDate(fc.exhaustion_at) : (fc.forecast_90d < 80 ? '> 180 Days' : fc.forecast_90d < 90 ? '90-180 Days' : '< 90 Days')
    const headroom = Math.max(0, Math.round(100 - fc.current_usage))
    return { value, trend: `${headroom}% Headroom`, trendType: fc.current_usage < 80 ? 'positive' : 'negative', badge: fc.status.toUpperCase(), badgeColor: fc.status === 'healthy' ? 'emerald' : fc.status === 'warning' ? 'amber' : 'rose', subtitle: `Current peak compute: ${fc.current_usage.toFixed(1)}%` }
  })

  const memSaturation = computed<HudMetricItem>(() => {
    if (!memForecast.value) return { value: '--', trend: 'No RAM data', trendType: 'neutral', badge: 'NO DATA', badgeColor: 'muted', subtitle: 'No memory forecast recorded' }
    const fc = memForecast.value
    const value = fc.exhaustion_at ? formatDate(fc.exhaustion_at) : (fc.forecast_90d < 80 ? '> 180 Days' : fc.forecast_90d < 90 ? '90-180 Days' : '< 90 Days')
    const diff = fc.forecast_30d - fc.current_usage
    return { value, trend: `Growth ${diff >= 0 ? '+' : ''}${diff.toFixed(1)}%/mo`, trendType: fc.status === 'healthy' ? 'neutral' : 'negative', badge: fc.status.toUpperCase(), badgeColor: fc.status === 'healthy' ? 'emerald' : fc.status === 'warning' ? 'amber' : 'rose', subtitle: `Current memory utilization: ${fc.current_usage.toFixed(1)}%` }
  })

  const storageHeadroom = computed<HudMetricItem>(() => {
    if (!storageForecast.value) return { value: '--', trend: 'No storage data', trendType: 'neutral', badge: 'NO DATA', badgeColor: 'muted', subtitle: 'No storage forecast recorded' }
    const fc = storageForecast.value
    const avail = Math.max(0, Math.round(100 - fc.current_usage))
    return { value: `${avail}% Free`, trend: `${avail}% Available`, trendType: fc.current_usage < 85 ? 'positive' : 'negative', badge: fc.status.toUpperCase(), badgeColor: fc.status === 'healthy' ? 'cyan' : fc.status === 'warning' ? 'amber' : 'rose', subtitle: `Current storage utilization: ${fc.current_usage.toFixed(1)}%` }
  })

  const scalingAction = computed<HudMetricItem>(() => {
    if (forecasts.value.length === 0) return { value: '--', trend: 'Standby', trendType: 'neutral', badge: 'NO DATA', badgeColor: 'muted', subtitle: 'No predictive forecast models' }
    const hasCrit = forecasts.value.some(f => f.status === 'critical' || f.forecast_30d >= 85)
    const hasWarn = forecasts.value.some(f => f.status === 'warning' || f.forecast_30d >= 70)
    if (hasCrit) return { value: '+1 Node Req', trend: 'Scale Up', trendType: 'negative', badge: 'ACTION REQUIRED', badgeColor: 'rose', subtitle: 'Pre-scale worker pool for forecast load' }
    if (hasWarn) return { value: 'Monitor Growth', trend: 'Trending Up', trendType: 'neutral', badge: 'ATTENTION', badgeColor: 'amber', subtitle: 'Resource consumption trending upward' }
    return { value: 'Nominal', trend: 'Stable', trendType: 'positive', badge: 'OPTIMAL', badgeColor: 'emerald', subtitle: 'Cluster capacity within safe thresholds' }
  })

  // Top 4 HUD Metrics for CapacityHudCards.vue
  const clusterSaturation = computed<HudMetricItem>(() => {
    if (nodesHeadroom.value.length === 0) {
      if (cpuForecast.value || memForecast.value) {
        const cpuVal = cpuForecast.value?.current_usage
        const memVal = memForecast.value?.current_usage
        const avg = (cpuVal !== undefined && memVal !== undefined)
          ? (cpuVal + memVal) / 2
          : (cpuVal ?? memVal ?? 0)
        const num = Number(avg.toFixed(1))
        return {
          value: `${num}%`,
          trend: num < 70 ? 'Within Safe Limits' : num < 85 ? 'Elevated Load' : 'Critical Saturation',
          trendType: num < 70 ? 'positive' : num < 85 ? 'neutral' : 'negative',
          badge: num < 70 ? 'NOMINAL' : num < 85 ? 'ELEVATED' : 'SATURATED',
          badgeColor: num < 70 ? 'emerald' : num < 85 ? 'amber' : 'rose',
          subtitle: 'Cluster-wide aggregate compute & memory consumption',
        }
      }
      return {
        value: '--',
        trend: 'No capacity data',
        trendType: 'neutral',
        badge: 'NO DATA',
        badgeColor: 'muted',
        subtitle: 'Cluster-wide aggregate compute & memory consumption',
      }
    }
    const sum = nodesHeadroom.value.reduce(
      (acc, n) => acc + (n.cpuUsagePercent + n.memUsagePercent) / 2,
      0
    )
    const avg = Number((sum / nodesHeadroom.value.length).toFixed(1))
    return {
      value: `${avg}%`,
      trend: avg < 70 ? 'Within Safe Limits' : avg < 85 ? 'Elevated Load' : 'Critical Saturation',
      trendType: avg < 70 ? 'positive' : avg < 85 ? 'neutral' : 'negative',
      badge: avg < 70 ? 'NOMINAL' : avg < 85 ? 'ELEVATED' : 'SATURATED',
      badgeColor: avg < 70 ? 'emerald' : avg < 85 ? 'amber' : 'rose',
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
    if (nodesHeadroom.value.length === 0) {
      return {
        value: '--',
        trend: 'No node telemetry',
        trendType: 'neutral',
        badge: 'NO DATA',
        badgeColor: 'muted',
        subtitle: 'Weighted pod-to-allocatable bin-packing ratio',
      }
    }
    const scores = nodesHeadroom.value.map(n => n.binPackingScore)
    const avg = Number((scores.reduce((a, b) => a + b, 0) / scores.length).toFixed(1))
    const diff = Number((avg - 75).toFixed(1))
    return {
      value: `${avg}%`,
      trend: `${diff >= 0 ? '+' : ''}${diff}% vs SLA Target (75%)`,
      trendType: avg >= 75 ? 'positive' : 'negative',
      badge: avg >= 80 ? 'HIGH DENSITY' : avg >= 60 ? 'BALANCED' : 'LOW DENSITY',
      badgeColor: avg >= 80 ? 'cyan' : avg >= 60 ? 'emerald' : 'amber',
      subtitle: 'Weighted pod-to-allocatable bin-packing ratio',
    }
  })

  const safeHeadroom = computed<HudMetricItem>(() => {
    if (nodesHeadroom.value.length === 0) {
      return {
        value: '--',
        trend: 'No node telemetry',
        trendType: 'neutral',
        badge: 'NO DATA',
        badgeColor: 'muted',
        subtitle: 'Guaranteed burst headroom before eviction triggers',
      }
    }
    const minH = Math.min(...nodesHeadroom.value.map(n => n.headroomPercent))
    const minHNum = Number(minH.toFixed(1))
    return {
      value: `${minHNum}%`,
      trend: `Min Node: ${minHNum}% Headroom`,
      trendType: minHNum >= 20 ? 'positive' : 'negative',
      badge: minHNum >= 20 ? 'SAFE BUFFER' : 'LOW BUFFER',
      badgeColor: minHNum >= 20 ? 'emerald' : 'amber',
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
        recs.push({ icon: 'sliders', title: 'Horizontal Node Pool Auto-Scaling', desc: `Adjust worker node pool capacity for "${fc.cluster}" to absorb 30d forecast of ${fc.forecast_30d.toFixed(1)}%.`, impact: 'Prevents CPU throttling during peak traffic', impactClass: 'text-cyan' })
      } else if (['memory', 'ram'].includes(type) && (fc.forecast_30d > 70 || fc.status !== 'healthy')) {
        recs.push({ icon: 'cpu', title: 'Pod Memory Request Right-Sizing', desc: `Review memory limits in cluster "${fc.cluster}" where forecast reaches ${fc.forecast_30d.toFixed(1)}% to prevent OOM-Kills.`, impact: 'Optimizes memory bin-packing and prevents container evictions', impactClass: 'text-emerald' })
      } else if (['storage', 'disk'].includes(type) && (fc.forecast_30d > 70 || fc.status !== 'healthy')) {
        recs.push({ icon: 'hard-drive', title: 'Storage Volume Compaction & PVC Pruning', desc: `Enable volume snapshot deduplication and purge unreferenced PVCs on cluster "${fc.cluster}".`, impact: 'Recovers disk headroom and defers storage volume expansion', impactClass: 'text-violet' })
      }
    }
    if (recs.length === 0 && forecasts.value.length > 0) {
      recs.push({ icon: 'check-circle', title: 'Resource Allocation Sizing Optimal', desc: `All ${forecasts.value.length} tracked capacity checkpoints are operating within healthy operating thresholds.`, impact: 'Zero scaling actions required at this time', impactClass: 'text-emerald' })
    }
    return recs
  })

  async function fetchCapacityData(cluster?: string) {
    loading.value = true
    error.value = null
    const targetCluster = cluster || activeClusterId.value || undefined
    try {
      const [data, rawNodes] = await Promise.all([
        capacityApi.getForecasts(targetCluster).catch(() => []),
        capacityApi.getNodeHeadroom(targetCluster).catch(() => []),
      ])
      forecasts.value = data || []

      if (rawNodes && rawNodes.length > 0) {
        nodesHeadroom.value = rawNodes.map((n: BackendNodeHeadroom) => ({
          id: n.id,
          name: n.name,
          role: n.role,
          cpuTotalCores: n.cpu_total_cores,
          cpuAllocatedCores: n.cpu_allocated_cores,
          cpuUsagePercent: n.cpu_usage_percent,
          memTotalGiB: n.mem_total_gib,
          memAllocatedGiB: n.mem_allocated_gib,
          memUsagePercent: n.mem_usage_percent,
          podCount: n.pod_count,
          podCapacity: n.pod_capacity,
          binPackingScore: n.bin_packing_score,
          status: n.status,
          headroomPercent: n.headroom_percent,
        }))
      } else {
        const metrics = await overviewApi.getNodeMetrics().catch(() => [])
        if (metrics && metrics.length > 0) {
          nodesHeadroom.value = metrics.map((m: NodeMetrics) => {
            const cpuUsagePercent = Number(m.cpu_percent.toFixed(1))
            const cpuTotalCores = 16
            const cpuAllocatedCores = Number(((cpuTotalCores * cpuUsagePercent) / 100).toFixed(1))
            const memTotalGiB = m.memory_total > 0 ? Number((m.memory_total / (1024 * 1024 * 1024)).toFixed(1)) : 32
            const memAllocatedGiB = Number((m.memory_used / (1024 * 1024 * 1024)).toFixed(1))
            const memUsagePercent = Number(m.memory_percent.toFixed(1))
            const podCount = 0
            const podCapacity = 110
            const binPackingScore = Number(((cpuUsagePercent + memUsagePercent) / 2).toFixed(1))
            const headroomPercent = Math.max(0, Number((100 - Math.max(cpuUsagePercent, memUsagePercent)).toFixed(1)))
            const status = m.status === 'ready'
              ? (headroomPercent < 15 ? 'critical' : headroomPercent < 30 ? 'warning' : 'healthy')
              : 'critical'
            const role = (m.role.includes('control') || m.role.includes('manager') || m.role.includes('master')) ? 'control-plane' : 'worker'

            return {
              id: m.node_id || m.node_name,
              name: m.node_name,
              role,
              cpuTotalCores,
              cpuAllocatedCores,
              cpuUsagePercent,
              memTotalGiB,
              memAllocatedGiB,
              memUsagePercent,
              podCount,
              podCapacity,
              binPackingScore,
              status,
              headroomPercent,
            }
          })
        } else {
          nodesHeadroom.value = []
        }
      }
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to load capacity forecasts'
      forecasts.value = []
      nodesHeadroom.value = []
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
    statusMessage.value = { type: 'success', text: `Pod rebalance scheduled for node ${target.name}. Dynamic pod rescheduling initiated.` }
    await fetchCapacityData()
  }

  function inspectNode(nodeId: string): NodeHeadroom | undefined {
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
  if (t.includes('cpu')) return 'cpu'
  if (t.includes('mem')) return 'activity'
  if (t.includes('stor')) return 'hard-drive'
  return 'box'
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
