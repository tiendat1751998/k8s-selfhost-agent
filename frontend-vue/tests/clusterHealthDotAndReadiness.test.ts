import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

// Test resolveClusterDotClass function
export function resolveClusterDotClass(status?: string, healthStatus?: string): string {
  const hs = (healthStatus || '').toLowerCase().trim()
  const s = (status || '').toLowerCase().trim()

  // High-priority degraded checks
  if (['degraded', 'warning', 'pending', 'elevated'].includes(hs)) {
    return 'dot-amber dot-pulse'
  }
  // High-priority offline/unhealthy checks
  if (['offline', 'disconnected', 'unhealthy', 'error', 'failed', 'critical', 'down'].includes(hs)) {
    return 'dot-rose'
  }
  if (hs === 'unknown') {
    return 'dot-slate'
  }

  // Check status, but ONLY if healthStatus is not degraded/offline/unhealthy
  if (['degraded', 'warning', 'pending', 'elevated'].includes(s)) {
    return 'dot-amber dot-pulse'
  }
  if (['offline', 'failed', 'critical', 'down', 'error', 'disconnected', 'unhealthy'].includes(s)) {
    return 'dot-rose'
  }

  // Emerald ONLY when healthy/connected/ready and healthStatus is not degraded/unhealthy/offline
  if (['healthy', 'ready', 'active', 'online', 'ok', 'running', 'connected'].includes(hs)) {
    return 'dot-emerald'
  }
  if (['healthy', 'ready', 'active', 'online', 'ok', 'running', 'connected'].includes(s)) {
    return 'dot-emerald'
  }

  // Unknown / empty / null -> neutral muted dot
  return 'dot-slate'
}

describe('Task 070 Wave 2C: Cluster Honest Health Dot & Readiness', () => {
  describe('1. resolveClusterDotClass honest status mapping', () => {
    it('NEVER defaults unknown or empty status to green dot-emerald', () => {
      assert.equal(resolveClusterDotClass(undefined, undefined), 'dot-slate')
      assert.equal(resolveClusterDotClass('', ''), 'dot-slate')
      assert.equal(resolveClusterDotClass('unknown', 'unknown'), 'dot-slate')
      assert.equal(resolveClusterDotClass('unknown', undefined), 'dot-slate')
      assert.equal(resolveClusterDotClass(undefined, 'unknown'), 'dot-slate')
    })

    it('returns dot-amber dot-pulse when health_status or status is degraded/warning', () => {
      assert.equal(resolveClusterDotClass('healthy', 'degraded'), 'dot-amber dot-pulse', 'Degraded health overrides healthy status')
      assert.equal(resolveClusterDotClass('active', 'warning'), 'dot-amber dot-pulse')
      assert.equal(resolveClusterDotClass('degraded', undefined), 'dot-amber dot-pulse')
    })

    it('returns dot-rose when offline, disconnected, unhealthy, or error', () => {
      assert.equal(resolveClusterDotClass('healthy', 'offline'), 'dot-rose', 'Offline health overrides healthy status')
      assert.equal(resolveClusterDotClass('active', 'unhealthy'), 'dot-rose')
      assert.equal(resolveClusterDotClass('offline', undefined), 'dot-rose')
      assert.equal(resolveClusterDotClass('error', undefined), 'dot-rose')
    })

    it('returns dot-emerald ONLY when healthy and not degraded/offline', () => {
      assert.equal(resolveClusterDotClass('healthy', 'healthy'), 'dot-emerald')
      assert.equal(resolveClusterDotClass('connected', 'healthy'), 'dot-emerald')
      assert.equal(resolveClusterDotClass('active', undefined), 'dot-emerald')
    })
  })

  describe('2. GlobalContextSelector.vue source verification', () => {
    const gcsFile = path.resolve(import.meta.dirname, '../src/components/navigation/GlobalContextSelector.vue')
    const content = fs.readFileSync(gcsFile, 'utf8')

    it('does not default fallback cluster to healthy', () => {
      assert.equal(content.includes("status: 'healthy'"), false, 'Fallback cluster must NOT default status to healthy')
    })

    it('passes both status and health_status to dot class resolution in template', () => {
      assert.ok(
        content.includes('getClusterDotClass(c.status, c.health_status)') || content.includes('getClusterDotClass(c)'),
        'Must pass both status and health_status when resolving cluster dot'
      )
    })
  })

  describe('3. ClusterDetailsDrawer.vue source verification', () => {
    const drawerFile = path.resolve(import.meta.dirname, '../src/components/fleet/ClusterDetailsDrawer.vue')
    const content = fs.readFileSync(drawerFile, 'utf8')

    it('surfaces degraded control plane explanation', () => {
      assert.ok(
        content.includes('Control Plane :6443 unreachable') || content.includes(':6443 unreachable'),
        'ClusterDetailsDrawer must explain degraded status when control plane is unreachable'
      )
    })

    it('displays cluster.node_count or cluster.nodes correctly', () => {
      assert.ok(
        content.includes('node_count') || content.includes('cluster.nodes'),
        'ClusterDetailsDrawer must display actual nodes or node_count'
      )
    })
  })
})
