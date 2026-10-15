import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  calculateBlastRadius,
  buildRcaTimelineEvents,
  formatIncidentTime,
  SIMULATION_SCENARIOS,
  type BlastRadiusInfo,
  type RcaTimelineEvent
} from '../src/composables/incidentHelpers.ts'
import type { Incident, RCAReport, PullRequest } from '../src/api/compute'

describe('Incident Lifecycle Integration & Blast Radius Math', () => {
  it('calculateBlastRadius handles null input safely', () => {
    assert.equal(calculateBlastRadius(null), null)
  })

  it('calculateBlastRadius computes impact score and risk level for critical incidents', () => {
    const inc: Incident = {
      id: 'inc-01',
      cluster_name: 'production-k8s',
      namespace: 'payments',
      pod_name: 'payment-service-7f89b-2xk',
      severity: 'critical',
      status: 'firing',
      type: 'OOMKilled',
      message: 'Container memory limit exceeded',
      created_at: '2026-10-14T08:00:00Z',
      updated_at: '2026-10-14T08:05:00Z',
      raw_data: {
        affected_services: 'payment-gateway, checkout-api',
        dependent_workloads: 'billing-cron, ledger-worker'
      }
    }
    const res = calculateBlastRadius(inc)
    assert.ok(res)
    assert.equal(res.impactScore, 92)
    assert.equal(res.riskLevel, 'Critical Blast')
    assert.equal(res.nodeHealth, 'Degraded Node Capacity')
    assert.deepEqual(res.affectedServices, ['payment-gateway', 'checkout-api'])
    assert.deepEqual(res.dependentWorkloads, ['billing-cron', 'ledger-worker'])
    assert.equal(res.affectedNamespace, 'payments')
  })

  it('calculateBlastRadius handles missing raw_data and derives single service gracefully', () => {
    const inc: Incident = {
      id: 'inc-02',
      cluster_name: 'staging-01',
      namespace: 'frontend',
      pod_name: 'web-portal-99a',
      severity: 'medium',
      status: 'firing',
      type: 'CrashLoopBackOff',
      message: 'Container crashed with exit code 1',
      created_at: '2026-10-14T09:00:00Z',
      updated_at: '2026-10-14T09:00:00Z'
    }
    const res = calculateBlastRadius(inc)
    assert.ok(res)
    assert.equal(res.impactScore, 34)
    assert.equal(res.riskLevel, 'Localized Scope')
    assert.equal(res.nodeHealth, 'Nominal Node Status')
    assert.deepEqual(res.affectedServices, ['web-portal-99a'])
    assert.deepEqual(res.dependentWorkloads, [])
  })

  it('buildRcaTimelineEvents formats timeline without synthetic confidence or fake PR numbers', () => {
    const inc: Incident = {
      id: 'inc-03',
      cluster_name: 'fleet-eu-1',
      namespace: 'auth',
      pod_name: 'auth-server-0',
      severity: 'high',
      status: 'firing',
      type: 'ReadinessProbeFailed',
      message: 'Probe timeout after 3 retries',
      created_at: '2026-10-14T10:00:00Z',
      updated_at: '2026-10-14T10:01:00Z'
    }

    const reportWithoutConf: RCAReport = {
      id: 'rca-01',
      incident_id: 'inc-03',
      root_cause: 'Deadlock in database connection pool',
      evidence: ['db pool exhausted', '503 gateway timeout'],
      created_at: '2026-10-14T10:02:00Z',
      risk_level: 'High'
    }

    const events = buildRcaTimelineEvents(inc, reportWithoutConf, null)
    assert.equal(events.length, 3) // detection, correlation, rca
    const rcaEvt = events.find(e => e.type === 'analysis')
    assert.ok(rcaEvt)
    assert.equal(rcaEvt.meta?.confidence, 'N/A', 'Must report N/A when confidence is absent, never 95%')
  })

  it('buildRcaTimelineEvents correctly formats real numeric confidence score', () => {
    const inc: Incident = {
      id: 'inc-04',
      cluster_name: 'fleet-eu-1',
      namespace: 'auth',
      pod_name: 'auth-server-0',
      severity: 'high',
      status: 'resolved',
      type: 'ReadinessProbeFailed',
      message: 'Resolved',
      created_at: '2026-10-14T10:00:00Z',
      updated_at: '2026-10-14T10:05:00Z',
      resolved_at: '2026-10-14T10:05:00Z'
    }

    const reportWithConf: RCAReport = {
      id: 'rca-02',
      incident_id: 'inc-04',
      root_cause: 'Network partition on port 5432',
      confidence: 0.88,
      created_at: '2026-10-14T10:02:00Z',
      risk_level: 'Medium'
    }

    const pr: PullRequest = {
      id: 'pr-99',
      incident_id: 'inc-04',
      pr_number: 402,
      title: 'Fix connection timeout to 5000ms',
      branch: 'fix/conn-timeout',
      repo_url: 'https://github.com/org/repo',
      status: 'merged',
      created_at: '2026-10-14T10:03:00Z'
    }

    const events = buildRcaTimelineEvents(inc, reportWithConf, pr)
    const rcaEvt = events.find(e => e.type === 'analysis')
    assert.equal(rcaEvt?.meta?.confidence, '88%')

    const prEvt = events.find(e => e.type === 'pr')
    assert.ok(prEvt?.title.includes('#402'), 'Must use actual pr_number 402')

    const resEvt = events.find(e => e.type === 'resolution')
    assert.ok(resEvt, 'Must include resolution event when incident is resolved')
  })

  it('SIMULATION_SCENARIOS contains clean architecture workloads with zero fake IPs or clusters', () => {
    assert.ok(SIMULATION_SCENARIOS.length >= 3)
    for (const sc of SIMULATION_SCENARIOS) {
      assert.ok(sc.key)
      assert.ok(sc.workload)
      assert.ok(sc.namespace)
      assert.ok(sc.cluster)
      assert.strictEqual(sc.cluster.includes('prod-us-east-1'), false)
      assert.strictEqual(sc.description.includes('10.96.12.44'), false)
    }
  })

  it('formatIncidentTime handles valid and invalid date strings safely', () => {
    const valid = '2026-10-14T10:00:00Z'
    const formatted = formatIncidentTime(valid)
    assert.ok(typeof formatted === 'string' && formatted.length > 0)
    assert.equal(formatIncidentTime(''), '-')
  })
})
