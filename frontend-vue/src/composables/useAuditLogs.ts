import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  auditLogApi,
  auditApi,
  type AuditLogEntry,
  type AuditActionType,
  type AuditSeverity,
  type AuditResultStatus,
} from '../api/governance'

export function useAuditLogs() {
  const logs = ref<AuditLogEntry[]>([])
  const isLoading = ref(false)
  const isTriggeringScan = ref(false)
  const error = ref<string | null>(null)
  const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

  // Drawer inspection & Live tail state
  const selectedEvent = ref<AuditLogEntry | null>(null)
  const isDrawerOpen = ref(false)
  const isLiveTailing = ref(false)
  let liveTailTimer: ReturnType<typeof setInterval> | null = null

  // Filter state
  const searchQuery = ref('')
  const selectedActionType = ref<'all' | AuditActionType>('all')
  const selectedSeverity = ref<'ALL' | AuditSeverity>('ALL')
  const selectedActor = ref('all')
  const selectedStatus = ref<'all' | AuditResultStatus>('all')
  const dateRange = ref<{ start: string; end: string }>({ start: '', end: '' })

  const uniqueActors = computed(() => {
    const actorsSet = new Set<string>()
    logs.value.forEach(item => { if (item.actor) actorsSet.add(item.actor) })
    return Array.from(actorsSet).sort()
  })

  const actionTypeFilters = computed(() => [
    { key: 'all' as const, label: 'All Actions', count: logs.value.length },
    { key: 'mutation' as const, label: 'Mutations', count: logs.value.filter(l => l.action_type === 'mutation').length },
    { key: 'access' as const, label: 'Access', count: logs.value.filter(l => l.action_type === 'access').length },
    { key: 'rbac_grant' as const, label: 'RBAC Grants', count: logs.value.filter(l => l.action_type === 'rbac_grant').length },
    { key: 'deletion' as const, label: 'Deletions', count: logs.value.filter(l => l.action_type === 'deletion').length },
  ])

  // KPI Metrics
  const kpiMetrics = computed(() => {
    const totalEvents = logs.value.length
    const securityMutations = logs.value.filter(l => l.action_type === 'mutation').length
    const administrativeActions = logs.value.filter(l => l.action_type === 'rbac_grant').length
    const policyDenials = logs.value.filter(l => l.status === 'denied').length
    const signedCount = logs.value.filter(l => l.status === 'success').length
    const signedPercentage = totalEvents > 0 ? Math.round((signedCount / totalEvents) * 100) : 100
    const securityViolations = policyDenials + logs.value.filter(l => l.severity === 'critical' || l.severity === 'high').length

    return {
      totalEvents,
      securityMutations,
      administrativeActions,
      policyDenials,
      signedPercentage,
      securityViolations,
    }
  })

  // Immutable audit trail filtered query
  const filteredLogs = computed<AuditLogEntry[]>(() => {
    const q = searchQuery.value.trim().toLowerCase()
    const actionFilter = selectedActionType.value
    const sevFilter = selectedSeverity.value
    const actorFilter = selectedActor.value
    const statusFilter = selectedStatus.value
    const start = dateRange.value.start ? new Date(dateRange.value.start).getTime() : null
    const end = dateRange.value.end ? new Date(dateRange.value.end).getTime() + 86400000 : null

    return logs.value.filter(entry => {
      if (actionFilter !== 'all' && entry.action_type !== actionFilter) return false
      if (sevFilter !== 'ALL' && entry.severity.toUpperCase() !== sevFilter.toUpperCase()) return false
      if (actorFilter !== 'all' && entry.actor !== actorFilter) return false
      if (statusFilter !== 'all' && entry.status !== statusFilter) return false
      if (start || end) {
        const itemTime = new Date(entry.timestamp).getTime()
        if (start && itemTime < start) return false
        if (end && itemTime > end) return false
      }
      if (q) {
        const matchActor = entry.actor.toLowerCase().includes(q)
        const matchAction = entry.action.toLowerCase().includes(q)
        const matchTarget = entry.target_resource.toLowerCase().includes(q)
        const matchIp = entry.ip_address.toLowerCase().includes(q)
        const matchDetails = entry.details ? JSON.stringify(entry.details).toLowerCase().includes(q) : false
        if (!matchActor && !matchAction && !matchTarget && !matchIp && !matchDetails) return false
      }
      return true
    })
  })

  async function fetchLogs() {
    isLoading.value = true
    error.value = null
    try {
      const data = await auditLogApi.getLogs()
      logs.value = Object.freeze([...data]) as AuditLogEntry[]
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to query audit trail'
    } finally {
      isLoading.value = false
    }
  }

  async function triggerAuditScan() {
    isTriggeringScan.value = true
    statusMessage.value = null
    try {
      const res = await auditApi.triggerRun()
      statusMessage.value = {
        type: 'success',
        text: `Audit scan run #${res.run_id ? res.run_id.slice(0, 8) : 'new'} initiated. Log trail updating...`,
      }
      await fetchLogs()
    } catch (err: unknown) {
      statusMessage.value = {
        type: 'error',
        text: err instanceof Error ? err.message : 'Failed to trigger audit scan',
      }
    } finally {
      isTriggeringScan.value = false
    }
  }

  function toggleLiveTail() {
    if (isLiveTailing.value) {
      isLiveTailing.value = false
      if (liveTailTimer) { clearInterval(liveTailTimer); liveTailTimer = null }
    } else {
      isLiveTailing.value = true
      if (liveTailTimer) clearInterval(liveTailTimer)
      liveTailTimer = setInterval(fetchLogs, 5000)
    }
  }

  function exportToJson(filename = 'audit-trail-export.json') {
    const blob = new Blob([JSON.stringify(filteredLogs.value, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  function exportToCsv(filename = 'audit-trail-export.csv') {
    const headers = ['Timestamp', 'Actor', 'Action', 'ActionType', 'TargetResource', 'IPAddress', 'Status', 'Severity']
    const esc = (val: unknown) => {
      const str = String(val ?? '')
      return str.includes(',') || str.includes('"') || str.includes(String.fromCharCode(10)) ? `"${str.replace(/"/g, '""')}"` : str
    }
    const rows = filteredLogs.value.map(item => [
      esc(item.timestamp), esc(item.actor), esc(item.action), esc(item.action_type),
      esc(item.target_resource), esc(item.ip_address), esc(item.status), esc(item.severity),
    ])
    const csvContent = [headers.join(','), ...rows.map(r => r.join(','))].join(String.fromCharCode(10))
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  function openPayloadDrawer(event: AuditLogEntry) {
    selectedEvent.value = event
    isDrawerOpen.value = true
  }

  function closePayloadDrawer() {
    isDrawerOpen.value = false
    selectedEvent.value = null
  }

  function resetFilters() {
    searchQuery.value = ''
    selectedActionType.value = 'all'
    selectedSeverity.value = 'ALL'
    selectedActor.value = 'all'
    selectedStatus.value = 'all'
    dateRange.value = { start: '', end: '' }
  }

  function formatDate(d: string): string {
    if (!d) return '-'
    try { return new Date(d).toLocaleString() } catch { return d }
  }

  function formatRelativeTime(d: string): string {
    if (!d) return '-'
    try {
      const diffSec = Math.floor((Date.now() - new Date(d).getTime()) / 1000)
      if (diffSec < 60) return `${diffSec}s ago`
      const diffMin = Math.floor(diffSec / 60)
      if (diffMin < 60) return `${diffMin}m ago`
      const diffHr = Math.floor(diffMin / 60)
      if (diffHr < 24) return `${diffHr}h ago`
      return `${Math.floor(diffHr / 24)}d ago`
    } catch { return d }
  }

  onMounted(() => { fetchLogs() })
  onUnmounted(() => { if (liveTailTimer) clearInterval(liveTailTimer) })

  return {
    logs, filteredLogs, isLoading, isTriggeringScan, error, statusMessage,
    selectedEvent, isDrawerOpen, isLiveTailing, searchQuery,
    selectedActionType, selectedSeverity, selectedActor, selectedStatus, dateRange,
    uniqueActors, actionTypeFilters, kpiMetrics,
    fetchLogs, triggerAuditScan, toggleLiveTail, openPayloadDrawer, closePayloadDrawer,
    exportToJson, exportToCsv, resetFilters, formatDate, formatRelativeTime,
  }
}
