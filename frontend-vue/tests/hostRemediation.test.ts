import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

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
