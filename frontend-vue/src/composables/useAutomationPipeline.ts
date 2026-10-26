import type { AutomationRule } from '../api/governance.ts'

export interface PipelineStageInfo {
  label: string
  subtext: string
  icon: string
  chip: string
  configKeys?: { key: string; value: string }[]
}

export interface RulePipelineStages {
  when: PipelineStageInfo
  ifGate: PipelineStageInfo
  thenAction: PipelineStageInfo
}

/**
 * Extracts a normalized 3-stage WHEN -> IF -> THEN pipeline descriptor
 * from real AutomationRule data structures.
 */
export function extractPipelineStages(rule: AutomationRule): RulePipelineStages {
  const triggerType = (rule.trigger_type || '').toLowerCase()
  const actionType = (rule.action_type || '').toLowerCase()
  const triggerCfg = rule.trigger_config || {}
  const actionCfg = rule.action_config || {}

  // 1. Stage: WHEN Trigger
  let whenIcon = 'zap'
  let whenSubtext = rule.trigger_type ? rule.trigger_type.replace(/_/g, ' ') : 'Event Trigger'

  if (triggerType === 'node_pressure') {
    whenIcon = 'shield'
    whenSubtext = 'Node Resource Pressure'
  } else if (triggerType === 'deployment_failure') {
    whenIcon = 'flame'
    whenSubtext = 'Deployment Rollout Failure'
  } else if (triggerType === 'pod_restart') {
    whenIcon = 'refresh'
    whenSubtext = 'Repeated Pod Restarts'
  } else if (triggerType === 'high_cpu') {
    whenIcon = 'activity'
    whenSubtext = 'High CPU Utilization'
  } else if (triggerType === 'high_memory') {
    whenIcon = 'cpu'
    whenSubtext = 'High Memory Saturation'
  } else if (triggerType === 'slo_breach') {
    whenIcon = 'clock'
    whenSubtext = 'SLO Latency SLA Breach'
  } else if (triggerType === 'error_rate') {
    whenIcon = 'alert-triangle'
    whenSubtext = 'HTTP 5xx Error Spike'
  } else if (triggerType === 'cron_schedule') {
    whenIcon = 'clock'
    whenSubtext = 'Cron Schedule Trigger'
  } else if (triggerType === 'webhook_trigger') {
    whenIcon = 'globe'
    whenSubtext = 'Inbound Webhook Alert'
  }

  const whenStage: PipelineStageInfo = {
    label: 'WHEN Trigger',
    subtext: whenSubtext,
    icon: whenIcon,
    chip: rule.trigger_type || 'event',
    configKeys: Object.entries(triggerCfg).map(([key, value]) => ({ key, value: String(value) })),
  }

  // 2. Stage: IF Gate / Condition
  let ifSubtext = triggerCfg.condition || ''
  if (!ifSubtext) {
    if (triggerCfg.schedule) {
      ifSubtext = `Schedule: ${triggerCfg.schedule}`
    } else if (triggerCfg.threshold) {
      ifSubtext = `Threshold: ${triggerCfg.threshold}`
    } else if (triggerType === 'high_cpu') {
      ifSubtext = 'CPU Usage > 90% (5m avg)'
    } else if (triggerType === 'high_memory') {
      ifSubtext = 'Memory > 85% (3m avg)'
    } else if (triggerType === 'pod_restart') {
      ifSubtext = 'Restart count > 5 in 10m'
    } else if (triggerType === 'node_pressure') {
      ifSubtext = 'Node Memory/Disk Pressure Active'
    } else if (triggerType === 'deployment_failure') {
      ifSubtext = 'CrashLoopBackOff or Unready > 3m'
    } else if (triggerType === 'slo_breach') {
      ifSubtext = 'p99 Latency > 200ms Target'
    } else if (triggerType === 'error_rate') {
      ifSubtext = 'HTTP 5xx Error Rate > 2.0%'
    } else {
      ifSubtext = 'Cluster telemetry threshold matched'
    }
  }

  let ifChip = 'Scope: Cluster'
  if (triggerCfg.namespace) {
    ifChip = `ns: ${triggerCfg.namespace}`
  } else if (triggerCfg.threshold) {
    ifChip = `threshold: ${triggerCfg.threshold}`
  } else if (triggerCfg.schedule) {
    ifChip = `cron: ${triggerCfg.schedule}`
  }

  const ifStage: PipelineStageInfo = {
    label: 'IF Gate Condition',
    subtext: ifSubtext,
    icon: 'sliders',
    chip: ifChip,
  }

  // 3. Stage: THEN Action
  let thenIcon = 'zap'
  let thenSubtext = rule.action_type ? rule.action_type.replace(/_/g, ' ') : 'Automated Remediation'

  if (actionType === 'rollback') {
    thenIcon = 'refresh'
    thenSubtext = 'Rollback to Previous Git Revision'
  } else if (actionType === 'generate_rca') {
    thenIcon = 'search'
    thenSubtext = 'Trigger AI Root Cause Analysis'
  } else if (actionType === 'scale_deployment') {
    thenIcon = 'trending-up'
    thenSubtext = 'Auto-Scale Deployment (+2)'
  } else if (actionType === 'restart_pod') {
    thenIcon = 'refresh'
    thenSubtext = 'Graceful Pod Rolling Restart'
  } else if (actionType === 'cordon_node') {
    thenIcon = 'shield'
    thenSubtext = 'Cordon & Drain Node'
  } else if (actionType === 'send_notification') {
    thenIcon = 'bell'
    thenSubtext = 'Send High-Priority Alert'
  } else if (actionType === 'create_incident') {
    thenIcon = 'file-text'
    thenSubtext = 'Open Incident in Command Center'
  }

  let thenChip = rule.action_type || 'action'
  if (actionCfg.target) {
    thenChip = `target: ${actionCfg.target}`
  }

  const thenStage: PipelineStageInfo = {
    label: 'THEN Automated Action',
    subtext: thenSubtext,
    icon: thenIcon,
    chip: thenChip,
    configKeys: Object.entries(actionCfg).map(([key, value]) => ({ key, value: String(value) })),
  }

  return {
    when: whenStage,
    ifGate: ifStage,
    thenAction: thenStage,
  }
}

/**
 * Format relative time (e.g. "5m ago", "2h ago", "Never")
 */
export function formatRelativeTime(dateStr?: string): string {
  if (!dateStr) return 'Never'
  const t = new Date(dateStr).getTime()
  if (isNaN(t)) return dateStr

  const diffMs = Date.now() - t
  if (diffMs < 0) return 'Just now'

  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`

  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m ago`

  const diffHours = Math.floor(diffMin / 60)
  if (diffHours < 24) return `${diffHours}h ago`

  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ago`
}
