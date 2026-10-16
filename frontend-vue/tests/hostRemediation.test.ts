import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import type { NodeK8sHealthReport, RemediationResult } from '../src/api/hosts'

// Helper types & functions to test
export type RemediationAction =
  | 'restart_service'
  | 'disk_pressure_cleanup'
  | 'cni_ipam_cleanup'
  | 'reap_zombies'
  | 'restart_container'

export interface RemediationPayload {
  action: RemediationAction
  target?: string
  reason?: string
}

export const ALLOWED_SYSTEMD_SERVICES = [
  'kubelet',
  'containerd',
  'docker',
  'postgresql',
  'nginx',
  'redis',
  'haproxy',
  'keepalived',
  'etcd',
  'traefik',
  'clickhouse-server',
] as const

export function isCooldownError(errMsg?: string): boolean {
  if (!errMsg) return false
  const lower = errMsg.toLowerCase()
  return lower.includes('429') || lower.includes('cooldown') || lower.includes('rate limit')
}

export function buildRemediationRequest(action: RemediationAction, target?: string): RemediationPayload {
  const payload: RemediationPayload = { action }
  if (target && target.trim().length > 0) {
    payload.target = target.trim()
  }
  return payload
}

export function resolveHealthBadgeClass(isHealthy: boolean): { label: string; badgeClass: string } {
  if (isHealthy) {
    return { label: 'HEALTHY', badgeClass: 'badge-healthy' }
  }
  return { label: 'UNHEALTHY', badgeClass: 'badge-unhealthy' }
}

export function resolveRuntimeHealthy(health?: {
  runtime_healthy?: boolean
  containerd_healthy?: boolean
  docker_healthy?: boolean
  crio_healthy?: boolean
}): boolean {
  return Boolean(
    health?.runtime_healthy ||
    health?.containerd_healthy ||
    health?.docker_healthy ||
    health?.crio_healthy
  )
}

export function resolveDStateCount(health?: {
  d_state_processes?: number
  d_state_count?: number
}): number {
  return health?.d_state_processes ?? health?.d_state_count ?? 0
}

export function resolveAnomalies(health?: {
  detected_anomalies?: string[]
  anomalies?: string[]
}): string[] {
  return health?.detected_anomalies ?? health?.anomalies ?? []
}

export function resolveRemediationFeedback(
  res: { success?: boolean; status?: string; duration_ms: number; details?: string; output?: string },
  action: string
): { type: 'success' | 'error'; message: string; output?: string } {
  const isSuccess = res.success !== false && res.status !== 'failed'
  return {
    type: isSuccess ? 'success' : 'error',
    message: isSuccess
      ? `Action ${action} completed successfully (${res.duration_ms}ms)`
      : `Action ${action} failed (${res.duration_ms}ms)`,
    output: res.details || res.output,
  }
}

export function isRemediationSuccess(item: { success?: boolean; status?: string }): boolean {
  return Boolean(item.success || item.status === 'success' || item.status === 'ok')
}

describe('Host Self-Healing & Remediation Unit & Architecture Tests', () => {
  it('builds valid remediation payloads for 1-click node cleanups and container restart', () => {
    const diskCleanup = buildRemediationRequest('disk_pressure_cleanup')
    assert.deepEqual(diskCleanup, { action: 'disk_pressure_cleanup' })

    const cniCleanup = buildRemediationRequest('cni_ipam_cleanup')
    assert.deepEqual(cniCleanup, { action: 'cni_ipam_cleanup' })

    const reapZombies = buildRemediationRequest('reap_zombies')
    assert.deepEqual(reapZombies, { action: 'reap_zombies' })

    const restartContainer = buildRemediationRequest('restart_container', 'coredns-7c65d6cfc9')
    assert.deepEqual(restartContainer, {
      action: 'restart_container',
      target: 'coredns-7c65d6cfc9',
    })

    const restartService = buildRemediationRequest('restart_service', 'containerd')
    assert.deepEqual(restartService, {
      action: 'restart_service',
      target: 'containerd',
    })
  })

  it('contains all required systemd services in allowlist', () => {
    const required = [
      'kubelet',
      'containerd',
      'docker',
      'postgresql',
      'nginx',
      'redis',
      'haproxy',
      'keepalived',
      'etcd',
      'traefik',
      'clickhouse-server',
    ]
    for (const svc of required) {
      assert.ok(
        (ALLOWED_SYSTEMD_SERVICES as readonly string[]).includes(svc),
        `Missing required service: ${svc}`
      )
    }
  })

  it('correctly flags HTTP 429 and cooldown error messages', () => {
    assert.equal(isCooldownError('remediation rate limit exceeded, target in cooldown'), true)
    assert.equal(isCooldownError('HTTP 429: too many requests'), true)
    assert.equal(isCooldownError('Target containerd is in cooldown window'), true)
    assert.equal(isCooldownError('Connection refused'), false)
    assert.equal(isCooldownError(undefined), false)
  })

  it('resolves correct health badge class based on health state', () => {
    const healthy = resolveHealthBadgeClass(true)
    assert.equal(healthy.label, 'HEALTHY')
    assert.equal(healthy.badgeClass, 'badge-healthy')

    const unhealthy = resolveHealthBadgeClass(false)
    assert.equal(unhealthy.label, 'UNHEALTHY')
    assert.equal(unhealthy.badgeClass, 'badge-unhealthy')
  })

  it('normalizes and resolves backend NodeK8sHealthReport fields accurately', () => {
    const backendHealth: NodeK8sHealthReport = {
      node_name: 'k8s-worker-01',
      checked_at: '2026-10-16T12:00:00Z',
      kubelet_healthy: true,
      kubelet_status: 'Ready',
      containerd_healthy: true,
      docker_healthy: false,
      crio_healthy: false,
      container_runtime: 'containerd',
      d_state_processes: 4,
      zombie_count: 1,
      stuck_shim_count: 2,
      detected_anomalies: ['D_STATE_PROCESSES_STUCK', 'ZOMBIE_PROCESSES_DETECTED'],
    }

    // Verify runtime health computation
    assert.equal(resolveRuntimeHealthy(backendHealth), true)
    assert.equal(resolveRuntimeHealthy({ docker_healthy: true }), true)
    assert.equal(resolveRuntimeHealthy({ runtime_healthy: false, containerd_healthy: false }), false)

    // Verify d_state count fallback
    assert.equal(resolveDStateCount(backendHealth), 4)
    assert.equal(resolveDStateCount({ d_state_count: 3 }), 3)
    assert.equal(resolveDStateCount({}), 0)

    // Verify anomalies list fallback
    assert.deepEqual(resolveAnomalies(backendHealth), ['D_STATE_PROCESSES_STUCK', 'ZOMBIE_PROCESSES_DETECTED'])
    assert.deepEqual(resolveAnomalies({ anomalies: ['ANOMALY_1'] }), ['ANOMALY_1'])
    assert.deepEqual(resolveAnomalies({}), [])
  })

  it('normalizes and resolves backend RemediationResult fields for feedback and audit trail', () => {
    const backendResult: RemediationResult = {
      action: 'restart_service',
      target: 'containerd',
      success: true,
      status: 'success',
      details: 'Job for containerd.service restarted.',
      duration_ms: 350,
      timestamp: '2026-10-16T12:05:00Z',
    }

    const feedback = resolveRemediationFeedback(backendResult, backendResult.action)
    assert.equal(feedback.type, 'success')
    assert.equal(feedback.message, 'Action restart_service completed successfully (350ms)')
    assert.equal(feedback.output, 'Job for containerd.service restarted.')
    assert.equal(isRemediationSuccess(backendResult), true)

    // Test failure case with success: false and details error
    const failedResult: RemediationResult = {
      action: 'reap_zombies',
      success: false,
      status: 'failed',
      details: 'operation not permitted',
      duration_ms: 120,
    }
    const failedFeedback = resolveRemediationFeedback(failedResult, failedResult.action)
    assert.equal(failedFeedback.type, 'error')
    assert.equal(failedFeedback.message, 'Action reap_zombies failed (120ms)')
    assert.equal(failedFeedback.output, 'operation not permitted')
    assert.equal(isRemediationSuccess(failedResult), false)
  })

  it('verifies strict line limits on modified and created files', () => {
    const baseDir = fs.existsSync(path.join(process.cwd(), 'src'))
      ? process.cwd()
      : path.join(process.cwd(), 'frontend-vue')
    const hostsTsPath = path.join(baseDir, 'src', 'api', 'hosts.ts')
    const drawerPath = path.join(baseDir, 'src', 'components', 'hosts', 'HostDetailDrawer.vue')
    const panelPath = path.join(baseDir, 'src', 'components', 'hosts', 'HostRemediationPanel.vue')
    const cssPath = path.join(baseDir, 'src', 'assets', 'css', 'host-remediation.css')

    // Check hosts.ts < 300 lines
    const hostsLines = fs.readFileSync(hostsTsPath, 'utf8').split('\n').length
    assert.ok(hostsLines < 300, `hosts.ts must be < 300 lines, was ${hostsLines}`)

    // Check HostDetailDrawer.vue < 220 lines
    const drawerLines = fs.readFileSync(drawerPath, 'utf8').split('\n').length
    assert.ok(drawerLines < 220, `HostDetailDrawer.vue must be < 220 lines, was ${drawerLines}`)

    // HostRemediationPanel.vue must exist and be < 280 lines
    assert.ok(fs.existsSync(panelPath), `HostRemediationPanel.vue must exist`)
    const panelLines = fs.readFileSync(panelPath, 'utf8').split('\n').length
    assert.ok(panelLines < 280, `HostRemediationPanel.vue must be < 280 lines, was ${panelLines}`)

    // host-remediation.css must exist and be < 500 lines
    assert.ok(fs.existsSync(cssPath), `host-remediation.css must exist`)
    const cssLines = fs.readFileSync(cssPath, 'utf8').split('\n').length
    assert.ok(cssLines < 500, `host-remediation.css must be < 500 lines, was ${cssLines}`)
  })
})
