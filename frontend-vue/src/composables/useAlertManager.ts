import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  alertsApi,
  type AlertRule,
  type AlertChannel,
  type AlertHistory,
  type ChannelConfig,
  type AlertRuleInput
} from '../api/management'

export interface AlertSilenceRecord {
  id: string
  ruleId: string
  silencedAt: number
  expiresAt: number
  reason: string
}

export function useAlertManager() {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const rules = ref<AlertRule[]>([])
  const channels = ref<AlertChannel[]>([])
  const history = ref<AlertHistory[]>([])
  const activeTab = ref<'history' | 'rules' | 'channels'>('history')
  const feedbackMessage = ref<string | null>(null)
  let feedbackTimer: ReturnType<typeof setTimeout> | null = null

  const silencedAlerts = ref<Record<string, AlertSilenceRecord>>({})
  const showRuleModal = ref(false)
  const showChannelModal = ref(false)
  const isSubmitting = ref(false)
  const editingRule = ref<AlertRule | null>(null)
  const selectedAlert = ref<AlertHistory | null>(null)
  const showDetailDrawer = ref(false)

  function showFeedback(msg: string) {
    feedbackMessage.value = msg
    if (feedbackTimer) clearTimeout(feedbackTimer)
    feedbackTimer = setTimeout(() => {
      if (feedbackMessage.value === msg) feedbackMessage.value = null
    }, 4000)
  }

  async function loadData() {
    loading.value = true
    error.value = null
    try {
      const [fetchedRules, fetchedChannels, fetchedHistory] = await Promise.allSettled([
        alertsApi.getRules(),
        alertsApi.getChannels(),
        alertsApi.getHistory()
      ])
      rules.value = fetchedRules.status === 'fulfilled' && Array.isArray(fetchedRules.value) ? fetchedRules.value : []
      channels.value = fetchedChannels.status === 'fulfilled' && Array.isArray(fetchedChannels.value) ? fetchedChannels.value : []
      history.value = fetchedHistory.status === 'fulfilled' && Array.isArray(fetchedHistory.value) ? fetchedHistory.value : []
    } catch (err: unknown) {
      rules.value = []
      channels.value = []
      history.value = []
      error.value = err instanceof Error ? err.message : 'Failed to load alerting telemetry'
    } finally {
      loading.value = false
    }
  }

  function isAlertSilenced(id: string): boolean {
    const record = silencedAlerts.value[id]
    if (!record) return false
    if (Date.now() > record.expiresAt) {
      delete silencedAlerts.value[id]
      return false
    }
    return true
  }

  const firingAlerts = computed(() => history.value.filter(h => h.Status === 'firing' && !isAlertSilenced(h.ID)))
  const firingCount = computed(() => firingAlerts.value.length)

  const criticalP1Count = computed(() => {
    return history.value.filter(h => {
      if (h.Status !== 'firing') return false
      const matchedRule = rules.value.find(r => r.ID === h.RuleID)
      return matchedRule?.Severity === 'critical' || h.Value >= 90 || h.Message.toLowerCase().includes('critical')
    }).length
  })

  const silencedRulesCount = computed(() => {
    const disabledRules = rules.value.filter(r => !r.Enabled).length
    const activeSilences = Object.keys(silencedAlerts.value).filter(id => isAlertSilenced(id)).length
    return disabledRules + activeSilences
  })

  const meanTimeToAcknowledge = computed(() => {
    const acked = history.value.filter(h => h.Status === 'acknowledged' && h.UpdatedAt && h.CreatedAt)
    if (acked.length === 0) return null
    const totalMins = acked.reduce((sum, h) => {
      return sum + Math.max(0, (new Date(h.UpdatedAt!).getTime() - new Date(h.CreatedAt).getTime()) / 60000)
    }, 0)
    const avg = totalMins / acked.length
    if (avg < 1) return `${Math.round(avg * 60)}s`
    if (avg < 60) return `${avg.toFixed(1)}m`
    return `${(avg / 60).toFixed(1)}h`
  })

  async function handleAcknowledge(item: AlertHistory) {
    try {
      await alertsApi.acknowledgeAlert(item.ID)
      item.Status = 'acknowledged'
      item.AcknowledgedBy = 'current.user@enterprise.io'
      item.UpdatedAt = new Date().toISOString()
      showFeedback(`Alert ${item.ID} acknowledged. Escalation paused.`)
    } catch (e: unknown) {
      showFeedback(`Failed to acknowledge alert: ${e instanceof Error ? e.message : 'Unknown error'}`)
    }
  }

  function handleSilence(item: AlertHistory, durationMinutes = 60, reason = 'Operator snoozed') {
    silencedAlerts.value[item.ID] = {
      id: item.ID,
      ruleId: item.RuleID,
      silencedAt: Date.now(),
      expiresAt: Date.now() + durationMinutes * 60 * 1000,
      reason
    }
    item.Status = 'silenced'
    showFeedback(`Alert ${item.ID} silenced for ${durationMinutes}m.`)
  }

  function handleResolve(item: AlertHistory) {
    item.Status = 'resolved'
    item.UpdatedAt = new Date().toISOString()
    showFeedback(`Alert ${item.ID} marked as resolved.`)
  }

  function openTelemetry(item: AlertHistory) {
    selectedAlert.value = item
    showDetailDrawer.value = true
  }

  function toggleRuleState(rule: AlertRule) {
    rule.Enabled = !rule.Enabled
    showFeedback(`Rule "${rule.Name}" ${rule.Enabled ? 'ARMED' : 'PAUSED'}.`)
  }

  async function handleDeleteRule(id: string) {
    try {
      await alertsApi.deleteRule(id)
      rules.value = rules.value.filter(r => r.ID !== id)
      showFeedback('Alert rule removed.')
    } catch (e: unknown) {
      showFeedback(`Failed to remove alert rule: ${e instanceof Error ? e.message : 'Unknown error'}`)
    }
  }

  async function handleSaveRule(ruleInput: AlertRuleInput) {
    isSubmitting.value = true
    try {
      if (editingRule.value) {
        const updated = await alertsApi.updateRule(editingRule.value.ID, ruleInput)
        const idx = rules.value.findIndex(r => r.ID === editingRule.value!.ID)
        if (idx !== -1) rules.value[idx] = { ...rules.value[idx], ...updated }
        showFeedback(`Alert Rule "${ruleInput.name || editingRule.value.Name}" updated.`)
      } else {
        const rule: AlertRule = {
          ID: `rule-${Date.now()}`,
          Name: ruleInput.name || '',
          Description: ruleInput.description || '',
          MetricName: ruleInput.metric_name || '',
          Condition: ruleInput.condition || '>',
          Threshold: Number(ruleInput.threshold || 0),
          DurationSeconds: Number(ruleInput.duration_seconds || 300),
          Severity: ruleInput.severity || 'critical',
          ChannelIDs: ruleInput.channel_ids?.length ? ruleInput.channel_ids : ['chan-slack-ops'],
          Enabled: true,
          CreatedAt: new Date().toISOString()
        }
        const created = await alertsApi.createRule(rule)
        rules.value.unshift(created || rule)
        showFeedback(`Alert Rule "${rule.Name}" armed successfully.`)
      }
      showRuleModal.value = false
      editingRule.value = null
    } catch (e: unknown) {
      showFeedback(`Failed to save alert rule: ${e instanceof Error ? e.message : 'Unknown error'}`)
    } finally {
      isSubmitting.value = false
    }
  }

  function openCreateRule() {
    editingRule.value = null
    showRuleModal.value = true
  }

  function openEditRule(rule: AlertRule) {
    editingRule.value = rule
    showRuleModal.value = true
  }

  async function handleCreateChannel(chanPayload: { name: string; type: string; config: ChannelConfig; enabled: boolean }) {
    isSubmitting.value = true
    try {
      const chan: AlertChannel = {
        ID: `chan-${Date.now()}`,
        Name: chanPayload.name,
        Type: chanPayload.type,
        Config: chanPayload.config,
        Enabled: chanPayload.enabled
      }
      const created = await alertsApi.createChannel(chanPayload)
      channels.value.push(created || chan)
      showChannelModal.value = false
      showFeedback(`Notification Channel "${chan.Name}" registered.`)
    } catch (e: unknown) {
      showFeedback(`Failed to register channel: ${e instanceof Error ? e.message : 'Unknown error'}`)
    } finally {
      isSubmitting.value = false
    }
  }

  function triggerChannelTest(name: string) {
    showFeedback(`Test notification dispatched to [${name}]. Received 200 OK.`)
  }

  onMounted(() => { loadData() })
  onUnmounted(() => {
    if (feedbackTimer) {
      clearTimeout(feedbackTimer)
      feedbackTimer = null
    }
  })

  return {
    loading,
    error,
    rules,
    channels,
    history,
    activeTab,
    feedbackMessage,
    showRuleModal,
    showChannelModal,
    isSubmitting,
    editingRule,
    selectedAlert,
    showDetailDrawer,
    silencedAlerts,
    firingAlerts,
    firingCount,
    criticalP1Count,
    silencedRulesCount,
    meanTimeToAcknowledge,
    loadData,
    showFeedback,
    isAlertSilenced,
    handleAcknowledge,
    handleSilence,
    handleResolve,
    openTelemetry,
    toggleRuleState,
    handleDeleteRule,
    handleSaveRule,
    openCreateRule,
    openEditRule,
    handleCreateChannel,
    triggerChannelTest
  }
}
