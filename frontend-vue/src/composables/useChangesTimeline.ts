import { ref, computed, onMounted, onUnmounted } from 'vue'
import { changesApi, type ChangeRequest, type MaintenanceWindow } from '../api/management'
import { driftApi, type DriftRecord, auditApi, type AuditFinding } from '../api/governance'
import { deploymentsApi, type DeploymentApp, type RollbackDeploymentPayload } from '../api/deployments'

export type EventType = 'rfc' | 'gitops' | 'rollback' | 'audit'
export type TimeWindow = '1h' | '6h' | '24h' | '7d' | 'all'

export interface TimelineDiffPayload {
  expectedState?: string
  actualState?: string
  unifiedDiff?: string
  resourceKind?: string
  revision?: number
  filePath?: string
}

export interface TimelineEvent {
  id: string
  eventType: EventType
  title: string
  description: string
  category: string
  severity: 'low' | 'medium' | 'high' | 'critical'
  status: string
  cluster: string
  namespace: string
  resource: string
  requester?: string
  approver?: string
  timestamp: string
  raw?: ChangeRequest | DriftRecord | DeploymentApp | AuditFinding
  diffPayload?: TimelineDiffPayload
  canRollback?: boolean
  canApprove?: boolean
  canReject?: boolean
}

export function useChangesTimeline() {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const feedbackMessage = ref<string | null>(null)
  let feedbackTimer: ReturnType<typeof setTimeout> | null = null

  const changeRequests = ref<ChangeRequest[]>([])
  const driftRecords = ref<DriftRecord[]>([])
  const deployments = ref<DeploymentApp[]>([])
  const auditFindings = ref<AuditFinding[]>([])
  const maintenanceWindows = ref<MaintenanceWindow[]>([
    {
      id: 'mw-101',
      title: 'Kernel Security Patching & Node Rolling Restart',
      cluster: 'prod-us-east-1',
      start_at: new Date(Date.now() - 30 * 60 * 1000).toISOString(),
      end_at: new Date(Date.now() + 90 * 60 * 1000).toISOString(),
      active: true,
      created_at: new Date().toISOString()
    },
    {
      id: 'mw-102',
      title: 'Etcd Snapshot & TLS Certificate Rotation',
      cluster: 'prod-eu-west-1',
      start_at: new Date(Date.now() + 180 * 60 * 1000).toISOString(),
      end_at: new Date(Date.now() + 300 * 60 * 1000).toISOString(),
      active: false,
      created_at: new Date().toISOString()
    }
  ])

  // Filters & Drawer State
  const searchQuery = ref<string>('')
  const selectedCluster = ref<string>('all')
  const selectedTimeWindow = ref<TimeWindow>('24h')
  const selectedStatus = ref<string>('all')
  const selectedDiffEvent = ref<TimelineEvent | null>(null)
  const isDiffDrawerOpen = ref<boolean>(false)

  // RFC Creation Modal State
  const showCreateModal = ref(false)
  const isSubmitting = ref(false)
  const newChange = ref({
    title: '',
    description: '',
    type: 'standard' as 'standard' | 'emergency',
    cluster: 'prod-us-east-1',
    namespace: 'production-core',
    resource: 'deployment/payment-processor',
    requester: 'sre.lead@enterprise.io'
  })

  function showFeedback(msg: string) {
    if (feedbackTimer) clearTimeout(feedbackTimer)
    feedbackMessage.value = msg
    feedbackTimer = setTimeout(() => { feedbackMessage.value = null; feedbackTimer = null }, 4500)
  }

  function generateSyntheticDiff(title: string, resource: string, type: string): string {
    const ts = new Date().toISOString().substring(0, 19).replace('T', ' ')
    const reps = type === 'emergency' ? 6 : 4
    return [
      `--- a/${resource}.yaml (rev: live)`,
      `+++ b/${resource}.yaml (rev: target - ${ts})`,
      '@@ -12,6 +12,6 @@ metadata:',
      `   name: ${resource.split('/').pop() || 'resource'}`,
      ' spec:',
      `-  replicas: ${reps - 2}`,
      `+  replicas: ${reps} # Scale intent: ${title}`,
      '-  image: "registry.enterprise.io/core/service:v2.14.1"',
      '+  image: "registry.enterprise.io/core/service:v2.14.2"'
    ].join('\n')
  }

  async function loadTimelineData() {
    loading.value = true
    error.value = null
    try {
      const [changesRes, driftsRes, deploysRes, auditRes] = await Promise.allSettled([
        changesApi.getChanges(),
        driftApi.getDrifts(),
        deploymentsApi.list(),
        auditApi.getFindings('all')
      ])
      if (changesRes.status === 'fulfilled') changeRequests.value = changesRes.value?.data || []
      if (driftsRes.status === 'fulfilled') driftRecords.value = driftsRes.value?.data || []
      if (deploysRes.status === 'fulfilled') deployments.value = deploysRes.value || []
      if (auditRes.status === 'fulfilled') auditFindings.value = auditRes.value || []
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to synchronize change timeline'
    } finally {
      loading.value = false
    }
  }

  const allEvents = computed<TimelineEvent[]>(() => {
    const list: TimelineEvent[] = []

    for (const cr of changeRequests.value) {
      const isEm = cr.type === 'emergency'
      list.push({
        id: cr.id,
        eventType: 'rfc',
        title: cr.title,
        description: cr.description || 'Standard change execution under ITIL governance.',
        category: isEm ? 'Emergency Hotfix' : 'Standard RFC',
        severity: isEm ? 'high' : 'low',
        status: cr.status,
        cluster: cr.cluster || 'prod-us-east-1',
        namespace: cr.namespace || 'production-core',
        resource: cr.resource || 'cluster-config',
        requester: cr.requester,
        approver: cr.approver,
        timestamp: cr.created_at || new Date().toISOString(),
        raw: cr,
        diffPayload: {
          unifiedDiff: generateSyntheticDiff(cr.title, cr.resource || 'workload', cr.type),
          resourceKind: cr.resource?.split('/')[0] || 'Deployment',
          revision: 2
        },
        canApprove: cr.status === 'pending',
        canReject: cr.status === 'pending',
        canRollback: cr.status === 'approved' || cr.status === 'deployed'
      })
    }

    for (const drift of driftRecords.value) {
      list.push({
        id: `drift-${drift.id}`,
        eventType: 'gitops',
        title: `GitOps Sync Drift: ${drift.resource}`,
        description: `Configuration divergence detected in namespace ${drift.namespace}.`,
        category: 'Config Drift',
        severity: drift.status === 'drifted' ? 'high' : 'low',
        status: drift.status,
        cluster: drift.cluster || 'prod-us-east-1',
        namespace: drift.namespace || 'default',
        resource: `${drift.resource_kind || 'Resource'}/${drift.resource}`,
        timestamp: drift.detected_at || new Date().toISOString(),
        raw: drift,
        diffPayload: {
          expectedState: drift.expected_state,
          actualState: drift.actual_state,
          unifiedDiff: drift.diff || generateSyntheticDiff(`Drift ${drift.resource}`, drift.resource, 'standard'),
          resourceKind: drift.resource_kind || 'Resource'
        },
        canRollback: true
      })
    }

    for (const dep of deployments.value) {
      if ((dep.revision && dep.revision > 1) || dep.status !== 'healthy') {
        list.push({
          id: `dep-${dep.id || dep.name}-rev-${dep.revision || 1}`,
          eventType: 'rollback',
          title: `Deployment Checkpoint: ${dep.name} (Rev ${dep.revision || 1})`,
          description: `Active revision point. Strategy: ${dep.strategy || 'RollingUpdate'}. Image: ${dep.image}`,
          category: 'Rollback Point',
          severity: dep.status === 'healthy' ? 'low' : 'medium',
          status: dep.status,
          cluster: dep.target || 'prod-us-east-1',
          namespace: dep.namespace || 'default',
          resource: `deployment/${dep.name}`,
          timestamp: dep.created || new Date().toISOString(),
          raw: dep,
          diffPayload: {
            unifiedDiff: generateSyntheticDiff(`Rollback point for ${dep.name}`, `deployment/${dep.name}`, 'standard'),
            resourceKind: 'Deployment',
            revision: dep.revision || 1
          },
          canRollback: true
        })
      }
    }

    for (const audit of auditFindings.value) {
      if (audit.severity === 'critical' || audit.severity === 'high') {
        list.push({
          id: `audit-${audit.id}`,
          eventType: 'audit',
          title: `Audit Mutation Alert: ${audit.category}`,
          description: audit.description,
          category: 'Audit Mutation',
          severity: audit.severity,
          status: audit.status,
          cluster: 'prod-us-east-1',
          namespace: 'kube-system',
          resource: 'security-policy/rbac',
          timestamp: audit.detected_at || new Date().toISOString(),
          raw: audit,
          diffPayload: {
            unifiedDiff: generateSyntheticDiff(audit.description, 'rbac-policy', 'emergency'),
            resourceKind: 'SecurityPolicy'
          },
          canRollback: false
        })
      }
    }

    return list.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
  })

  function isWithinWindow(ts: string, w: TimeWindow): boolean {
    if (w === 'all') return true
    const hrs = (Date.now() - new Date(ts).getTime()) / 3600000
    if (isNaN(hrs)) return true
    const limits: Record<string, number> = { '1h': 1, '6h': 6, '24h': 24, '7d': 168 }
    return hrs <= (limits[w] || 24)
  }

  const filteredEvents = computed<TimelineEvent[]>(() => {
    return allEvents.value.filter(e => {
      if (selectedCluster.value !== 'all' && e.cluster !== selectedCluster.value) return false
      if (selectedStatus.value !== 'all' && e.status.toLowerCase() !== selectedStatus.value.toLowerCase()) return false
      if (!isWithinWindow(e.timestamp, selectedTimeWindow.value)) return false
      if (searchQuery.value.trim()) {
        const q = searchQuery.value.toLowerCase().trim()
        const text = `${e.id} ${e.title} ${e.description} ${e.resource} ${e.cluster} ${e.requester || ''}`.toLowerCase()
        if (!text.includes(q)) return false
      }
      return true
    })
  })

  const availableClusters = computed<string[]>(() => {
    const set = new Set<string>(['prod-us-east-1', 'prod-eu-west-1', 'staging-us-east'])
    for (const ev of allEvents.value) if (ev.cluster) set.add(ev.cluster)
    return Array.from(set)
  })

  const totalChanges24h = computed(() => allEvents.value.filter(e => isWithinWindow(e.timestamp, '24h')).length)
  const totalRollbacks = computed(() => allEvents.value.filter(e => e.eventType === 'rollback' || e.canRollback).length)
  const configDrifts = computed(() => driftRecords.value.filter(d => d.status === 'drifted').length || allEvents.value.filter(e => e.eventType === 'gitops').length)
  const highRiskMutations = computed(() => allEvents.value.filter(e => e.severity === 'high' || e.severity === 'critical').length)

  async function handleApprove(event: TimelineEvent | ChangeRequest) {
    const id = 'id' in event ? event.id : ''
    try {
      await changesApi.approveChange(id)
      const targetCr = changeRequests.value.find(c => c.id === id)
      if (targetCr) { targetCr.status = 'approved'; targetCr.approver = 'current.user@enterprise.io' }
      showFeedback(`Change request ${id} successfully approved.`)
    } catch (e: unknown) {
      showFeedback(`Failed to approve RFC: ${e instanceof Error ? e.message : 'Unknown error'}`)
    }
  }

  async function handleReject(event: TimelineEvent | ChangeRequest) {
    const id = 'id' in event ? event.id : ''
    try {
      await changesApi.rejectChange(id)
      const targetCr = changeRequests.value.find(c => c.id === id)
      if (targetCr) { targetCr.status = 'rejected'; targetCr.approver = 'current.user@enterprise.io' }
      showFeedback(`Change request ${id} rejected.`)
    } catch (e: unknown) {
      showFeedback(`Failed to reject RFC: ${e instanceof Error ? e.message : 'Unknown error'}`)
    }
  }

  async function handleRollback(event: TimelineEvent) {
    try {
      const [kind, name] = event.resource.includes('/') ? event.resource.split('/') : ['deployment', event.resource]
      const payload: RollbackDeploymentPayload = {
        cluster: event.cluster,
        namespace: event.namespace,
        name: name || event.resource,
        type: kind || 'deployment',
        revision: event.diffPayload?.revision ? Math.max(1, event.diffPayload.revision - 1) : 1
      }
      await deploymentsApi.rollback(payload)
      showFeedback(`Rollback initiated for ${event.resource} to rev ${payload.revision || 1}`)
    } catch (e: unknown) {
      showFeedback(`Rollback failed: ${e instanceof Error ? e.message : 'Unknown error'}`)
    }
  }

  async function handleCreateChange() {
    if (!newChange.value.title || !newChange.value.resource) return
    isSubmitting.value = true
    const cr: ChangeRequest = {
      id: `cr-${Date.now().toString(36)}`,
      title: newChange.value.title,
      description: newChange.value.description,
      type: newChange.value.type,
      status: 'pending',
      requester: newChange.value.requester,
      cluster: newChange.value.cluster,
      namespace: newChange.value.namespace,
      resource: newChange.value.resource,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    }
    try {
      const created = await changesApi.createChange(cr)
      changeRequests.value.unshift(created || cr)
      showCreateModal.value = false
      showFeedback(`Change Request ${cr.id} registered for review.`)
      newChange.value.title = ''
      newChange.value.description = ''
    } catch (e: unknown) {
      showFeedback(`Failed to submit RFC: ${e instanceof Error ? e.message : 'Unknown error'}`)
    } finally {
      isSubmitting.value = false
    }
  }

  function inspectDiff(event: TimelineEvent) {
    selectedDiffEvent.value = event
    isDiffDrawerOpen.value = true
  }

  function closeDiff() {
    isDiffDrawerOpen.value = false
    selectedDiffEvent.value = null
  }

  onMounted(() => { loadTimelineData() })
  onUnmounted(() => { if (feedbackTimer) { clearTimeout(feedbackTimer); feedbackTimer = null } })

  return {
    loading, error, feedbackMessage, changeRequests, maintenanceWindows,
    searchQuery, selectedCluster, selectedTimeWindow, selectedStatus,
    selectedDiffEvent, isDiffDrawerOpen, showCreateModal, isSubmitting, newChange,
    allEvents, filteredEvents, availableClusters,
    totalChanges24h, totalRollbacks, configDrifts, highRiskMutations,
    showFeedback, loadTimelineData, handleApprove, handleReject, handleRollback,
    handleCreateChange, inspectDiff, closeDiff
  }
}
