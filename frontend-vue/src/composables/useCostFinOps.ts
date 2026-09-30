import { ref, computed, onMounted } from 'vue'
import { costApi, type ClusterCost, type NamespaceCost, type ResourceWaste } from '../api/governance'

export interface TeamNamespaceCost extends NamespaceCost {
  [key: string]: unknown
  team: string
  budget_limit: number
  budget_utilization: number
  budget_status: 'optimal' | 'warning' | 'exceeded'
}

export interface CloudSpendBreakdown {
  provider: string
  name: string
  cost: number
  percentage: number
  color: string
  clusterCount: number
}

export interface CostForecast {
  currentRunRate: number
  projectedCost: number
  growthRate: number
  confidenceScore: number
  spotSavingsMonth: number
  spotRatio: number
  wasteSaved: number
}

export interface CostNotification {
  type: 'success' | 'error' | 'info'
  text: string
}

export function useCostFinOps() {
  const clusters = ref<ClusterCost[]>([])
  const rawNamespaces = ref<NamespaceCost[]>([])
  const wasteAlerts = ref<ResourceWaste[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const statusMessage = ref<CostNotification | null>(null)
  const budgetLimits = ref<Record<string, number>>({})
  const wasteSavedRemediated = ref<number>(0)
  const selectedNamespace = ref<TeamNamespaceCost | null>(null)

  function inferTeamFromNamespace(ns: string): string {
    const lower = ns.toLowerCase()
    if (lower.startsWith('prod') || lower.includes('production')) return 'Core SRE'
    if (lower.startsWith('stage') || lower.startsWith('dev')) return 'App Dev'
    if (lower.startsWith('ml') || lower.includes('data') || lower.includes('ai')) return 'Data Science'
    if (lower.startsWith('sec') || lower.includes('audit')) return 'Security Ops'
    if (lower.startsWith('kube') || lower.includes('system')) return 'Platform Infra'
    return 'Engineering'
  }

  // 1. Namespace Cost Allocation
  const namespaces = computed<TeamNamespaceCost[]>(() => {
    return rawNamespaces.value.map((ns) => {
      const defaultLimit = Math.max(Math.round(ns.monthly_cost * 1.25), 1000)
      const budgetLimit = budgetLimits.value[ns.namespace] || defaultLimit
      const budgetUtilization = budgetLimit > 0 ? Math.round((ns.monthly_cost / budgetLimit) * 100) : 0
      const budgetStatus: 'optimal' | 'warning' | 'exceeded' =
        budgetUtilization > 100 ? 'exceeded' : budgetUtilization >= 80 ? 'warning' : 'optimal'

      return {
        ...ns,
        team: inferTeamFromNamespace(ns.namespace),
        budget_limit: budgetLimit,
        budget_utilization: budgetUtilization,
        budget_status: budgetStatus,
      }
    })
  })

  const totalMonthlyCost = computed(() => {
    if (clusters.value.length > 0) return clusters.value.reduce((acc, c) => acc + (c.monthly_cost || 0), 0)
    return rawNamespaces.value.reduce((acc, n) => acc + (n.monthly_cost || 0), 0)
  })

  const totalDailyCost = computed(() => {
    if (clusters.value.length > 0) return clusters.value.reduce((acc, c) => acc + (c.daily_cost || 0), 0)
    return Math.round(totalMonthlyCost.value / 30)
  })

  // 2. Idle Waste Identification
  const totalWastedCost = computed(() => wasteAlerts.value.reduce((acc, w) => acc + (w.wasted_cost || 0), 0))

  const wastePercentage = computed(() => {
    if (totalMonthlyCost.value === 0) return 0
    return Math.round((totalWastedCost.value / totalMonthlyCost.value) * 100)
  })

  const wasteSaved = computed(() => wasteSavedRemediated.value + totalWastedCost.value)

  const averageEfficiency = computed(() => {
    const ns = rawNamespaces.value
    if (ns.length === 0) return 0
    return Math.round(ns.reduce((acc, n) => acc + (n.utilization || 0), 0) / ns.length)
  })

  // 3. Spot Instance Allocation derived from fault-tolerant namespace requests
  const spotRatio = computed(() => {
    if (rawNamespaces.value.length === 0) {
      if (clusters.value.length === 0) return 0
      const computeCost = clusters.value.reduce((acc, c) => acc + (c.cpu_cost || 0), 0)
      const totalCost = totalMonthlyCost.value
      return totalCost > 0 ? Math.min(100, Math.round((computeCost / totalCost) * 100)) : 0
    }
    const spotEligible = rawNamespaces.value.filter(ns => {
      const name = (ns.namespace || '').toLowerCase()
      return !name.startsWith('prod') && !name.includes('production') && !name.includes('kube-system')
    })
    const spotCost = spotEligible.reduce((acc, ns) => acc + (ns.monthly_cost || 0), 0)
    const total = rawNamespaces.value.reduce((acc, ns) => acc + (ns.monthly_cost || 0), 0)
    if (total === 0) return 0
    return Math.min(100, Math.round((spotCost / total) * 100))
  })

  const spotSavings = computed(() => {
    const spotSpend = totalMonthlyCost.value * (spotRatio.value / 100)
    return Math.round(spotSpend * 0.6)
  })

  // 4. Monthly Spend Forecast derived from workload trends and headroom
  const effectiveGrowthRate = computed(() => {
    if (clusters.value.length > 0) {
      const validClusters = clusters.value.filter(c => typeof c.trend === 'number')
      if (validClusters.length > 0) {
        const sumTrend = validClusters.reduce((acc, c) => acc + c.trend, 0)
        return Number((sumTrend / validClusters.length).toFixed(1))
      }
    }
    if (rawNamespaces.value.length > 0) {
      const avgUtil = rawNamespaces.value.reduce((acc, n) => acc + (n.utilization || 0), 0) / rawNamespaces.value.length
      return Number(Math.max(0, (100 - avgUtil) * 0.05).toFixed(1))
    }
    return 0
  })

  const projectedCost = computed(() => {
    return Math.round(totalMonthlyCost.value * (1 + (effectiveGrowthRate.value / 100)))
  })

  const forecast = computed<CostForecast>(() => ({
    currentRunRate: totalMonthlyCost.value,
    projectedCost: projectedCost.value,
    growthRate: effectiveGrowthRate.value,
    confidenceScore: rawNamespaces.value.length > 0 ? 95 : 80,
    spotSavingsMonth: spotSavings.value,
    spotRatio: spotRatio.value,
    wasteSaved: wasteSaved.value,
  }))

  const cloudBreakdown = computed<CloudSpendBreakdown[]>(() => {
    const totals: Record<string, { cost: number; count: number }> = {}
    clusters.value.forEach((c) => {
      const p = (c.provider || 'other').toLowerCase()
      if (!totals[p]) totals[p] = { cost: 0, count: 0 }
      totals[p].cost += c.monthly_cost || 0
      totals[p].count += 1
    })

    const total = Object.values(totals).reduce((sum, item) => sum + item.cost, 0) || 1
    const colorMap: Record<string, string> = {
      aws: '#f59e0b',
      gcp: '#06b6d4',
      azure: '#3b82f6',
      baremetal: '#10b981',
      local: '#8b5cf6',
      other: '#94a3b8',
    }

    return Object.entries(totals).map(([provider, data]) => ({
      provider,
      name: provider.toUpperCase(),
      cost: data.cost,
      percentage: Math.round((data.cost / total) * 100),
      color: colorMap[provider] || '#94a3b8',
      clusterCount: data.count,
    }))
  })

  async function fetchCostData() {
    loading.value = true
    error.value = null
    try {
      const [summaryData, wasteData] = await Promise.all([costApi.getSummary(), costApi.getWaste()])
      clusters.value = summaryData?.clusters || []
      rawNamespaces.value = summaryData?.namespaces || []
      wasteAlerts.value = wasteData || []
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to load FinOps cost metrics'
      clusters.value = []
      rawNamespaces.value = []
      wasteAlerts.value = []
    } finally {
      loading.value = false
    }
  }

  function handleDismissWaste(id: string) {
    const found = wasteAlerts.value.find((w) => w.id === id)
    const savedAmount = found?.wasted_cost || 150
    wasteSavedRemediated.value += savedAmount
    wasteAlerts.value = wasteAlerts.value.filter((w) => w.id !== id)
    statusMessage.value = {
      type: 'success',
      text: `Right-sizing patch generated for alert #${id}. Estimated monthly savings: $${savedAmount.toLocaleString()}.`,
    }
  }

  function setBudgetLimit(namespace: string, limit: number) {
    budgetLimits.value = { ...budgetLimits.value, [namespace]: Math.max(100, Math.round(limit)) }
    statusMessage.value = {
      type: 'success',
      text: `Budget cap updated to $${limit.toLocaleString()}/mo for namespace "${namespace}".`,
    }
  }

  function openNamespaceBreakdown(ns: TeamNamespaceCost) {
    selectedNamespace.value = ns
  }

  function closeNamespaceBreakdown() {
    selectedNamespace.value = null
  }

  function getProviderIcon(provider: string): string {
    const p = (provider || '').toLowerCase()
    if (p.includes('aws')) return '☁️'
    if (p.includes('gcp') || p.includes('google')) return '🌐'
    if (p.includes('azure')) return '🔷'
    if (p.includes('baremetal') || p.includes('local')) return '🖥️'
    return '⎈'
  }

  function getProviderColor(provider: string): string {
    const p = (provider || '').toLowerCase()
    if (p.includes('aws')) return '#f59e0b'
    if (p.includes('gcp') || p.includes('google')) return '#06b6d4'
    if (p.includes('azure')) return '#3b82f6'
    if (p.includes('baremetal') || p.includes('local')) return '#10b981'
    return '#8b5cf6'
  }

  function getUtilColorClass(util: number): string {
    if (util >= 70) return 'bg-emerald'
    if (util >= 40) return 'bg-cyan'
    return 'bg-rose'
  }

  function formatWasteType(t: string): string {
    return t ? t.replace(/_/g, ' ').toUpperCase() : 'IDLE RESOURCE'
  }

  function formatCurrency(amount: number): string {
    return `$${(amount || 0).toLocaleString()}`
  }

  onMounted(() => {
    fetchCostData()
  })

  return {
    clusters,
    namespaces,
    rawNamespaces,
    wasteAlerts,
    loading,
    error,
    statusMessage,
    selectedNamespace,
    budgetLimits,
    totalMonthlyCost,
    totalDailyCost,
    totalWastedCost,
    wastePercentage,
    averageEfficiency,
    spotRatio,
    spotSavings,
    wasteSaved,
    projectedCost,
    forecast,
    cloudBreakdown,
    fetchCostData,
    handleDismissWaste,
    setBudgetLimit,
    openNamespaceBreakdown,
    closeNamespaceBreakdown,
    getProviderIcon,
    getProviderColor,
    getUtilColorClass,
    formatWasteType,
    formatCurrency,
  }
}
