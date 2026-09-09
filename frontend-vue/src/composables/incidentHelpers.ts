import type { Incident, RCAReport, PullRequest, IncidentSeverity } from '../api/compute'

export interface SimulationScenario {
  key: string
  icon: string
  title: string
  subtitle: string
  workload: string
  namespace: string
  cluster: string
  type: string
  severity: 'critical' | 'high' | 'medium'
  description: string
  badgeText: string
}

export interface RcaTimelineEvent {
  id: string
  title: string
  timestamp: string
  type: 'detection' | 'correlation' | 'analysis' | 'pr' | 'resolution'
  severity?: string
  description: string
  meta?: Record<string, string | number | boolean>
  evidence?: string[]
}

export interface BlastRadiusInfo {
  affectedNamespace: string
  affectedPod: string
  cluster: string
  impactScore: number
  riskLevel: string
  affectedServices: string[]
  dependentWorkloads: string[]
  nodeHealth: string
}

export interface PRFormData {
  title: string
  description: string
  repoUrl: string
  branch: string
  baseBranch: string
}

export interface CreateIncidentPayload {
  pod_name: string
  namespace: string
  cluster_name: string
  type: string
  severity: IncidentSeverity
  message: string
}

export const SIMULATION_SCENARIOS: SimulationScenario[] = [
  {
    key: 'oom',
    icon: '🔥',
    title: 'Pod OOMKilled (Exit Code 137)',
    subtitle: 'JVM Heap Memory Exhaustion on checkout-api',
    workload: 'checkout-api-7b9c6f8d-4x2kl',
    namespace: 'ecommerce',
    cluster: 'prod-us-east-1',
    type: 'OOMKilled',
    severity: 'critical',
    description: 'cgroup memory limit reached (512Mi). Kubernetes Linux kernel OOM killer terminated container with exit code 137.',
    badgeText: 'JVM Heap Exhaustion'
  },
  {
    key: 'node_down',
    icon: '🚨',
    title: 'Server Node Down (NodeNotReady)',
    subtitle: 'Infrastructure Host masterdb Unreachable',
    workload: 'masterdb',
    namespace: 'kube-system',
    cluster: 'prod-eu-west-1',
    type: 'NodeNotReady',
    severity: 'critical',
    description: 'Host node heartbeat lease failed. Kubelet stopped posting status (NodeStatusUnknown), causing node eviction.',
    badgeText: 'Host Node Down'
  },
  {
    key: 'crashloop',
    icon: '⚠️',
    title: 'CrashLoopBackOff',
    subtitle: 'PostgreSQL Connection Refused on payment-gateway',
    workload: 'payment-gateway-5f8d9b-w9z7x',
    namespace: 'payments',
    cluster: 'prod-us-east-1',
    type: 'CrashLoopBackOff',
    severity: 'high',
    description: 'PostgreSQL Connection Refused on payment-gateway (dial tcp 10.96.12.44:5432). Repeated container exits triggered CrashLoopBackOff.',
    badgeText: 'DB Connection Refused'
  }
]

export function formatIncidentTime(d?: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return d
  }
}

export function calculateBlastRadius(inc: Incident | null): BlastRadiusInfo | null {
  if (!inc) return null
  const isCritical = inc.severity === 'critical'
  const isHigh = inc.severity === 'high'
  const score = isCritical ? 92 : (isHigh ? 68 : 34)
  const appBase = (inc.pod_name || 'workload').split('-')[0]
  return {
    affectedNamespace: inc.namespace || 'default',
    affectedPod: inc.pod_name,
    cluster: inc.cluster_name || 'prod-us-east-1',
    impactScore: score,
    riskLevel: isCritical ? 'Critical Blast' : (isHigh ? 'Elevated Impact' : 'Localized Scope'),
    affectedServices: [appBase, `${appBase}-ingress`, `${appBase}-svc`],
    dependentWorkloads: [`${appBase}-worker`, `${appBase}-metrics`],
    nodeHealth: isCritical ? 'Degraded Node Capacity' : 'Nominal Node Status'
  }
}

export function buildRcaTimelineEvents(
  inc: Incident | null,
  report: RCAReport | null,
  activePR: PullRequest | null
): RcaTimelineEvent[] {
  if (!inc) return []
  const events: RcaTimelineEvent[] = [
    {
      id: 'evt-detection',
      title: `Telemetry Anomaly Detected (${inc.type})`,
      timestamp: inc.created_at,
      type: 'detection',
      severity: inc.severity,
      description: inc.message || `Workload ${inc.pod_name} reported anomalous health condition in namespace ${inc.namespace}.`,
      meta: { pod: inc.pod_name, namespace: inc.namespace, cluster: inc.cluster_name, severity: inc.severity }
    }
  ]

  const evidenceList = report?.evidence || [
    'cgroup memory threshold breached (> 98%)',
    'Readiness probe failed 3 consecutive times',
    'Container termination signal received from Linux OOM killer'
  ]
  events.push({
    id: 'evt-correlation',
    title: 'Alert Correlation & Multi-Cluster Telemetry Ingestion',
    timestamp: inc.created_at,
    type: 'correlation',
    description: `Correlated ${evidenceList.length} telemetry streams and Kubernetes cluster events across ${inc.cluster_name}.`,
    evidence: evidenceList
  })

  if (report) {
    events.push({
      id: 'evt-rca',
      title: 'Multi-Agent AI Diagnostic Synthesis',
      timestamp: report.created_at,
      type: 'analysis',
      description: report.root_cause || inc.message,
      meta: {
        model: report.llm_model || 'Claude 3.5 Sonnet / Multi-Agent',
        confidence: `${Math.round((report.confidence || 0.94) * 100)}%`,
        riskLevel: report.risk_level
      }
    })
  }

  if (activePR) {
    events.push({
      id: 'evt-pr',
      title: `GitOps Remediation PR #${activePR.pr_number || 104} Generated`,
      timestamp: activePR.created_at,
      type: 'pr',
      description: activePR.title || 'Remediation manifest synthesized with resource limit patches.',
      meta: { branch: activePR.branch, status: activePR.status, repoUrl: activePR.repo_url }
    })
  }

  if (inc.status === 'resolved' || activePR?.status === 'merged') {
    events.push({
      id: 'evt-resolution',
      title: 'Autonomous Patch Applied & Cluster Synchronized',
      timestamp: inc.resolved_at || inc.updated_at,
      type: 'resolution',
      description: 'Remediation patch merged and applied to target cluster. Telemetry returned to nominal baseline.',
      meta: { status: 'Resolved & Verified' }
    })
  }

  return events
}
