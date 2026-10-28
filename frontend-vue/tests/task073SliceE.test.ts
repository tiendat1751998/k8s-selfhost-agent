import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'
import { createPinia, setActivePinia } from 'pinia'

register('data:text/javascript,' + encodeURIComponent(`
export async function resolve(specifier, context, nextResolve) {
  try {
    return await nextResolve(specifier, context);
  } catch (err) {
    if (specifier.startsWith('.')) {
      try {
        return await nextResolve(specifier + '.ts', context);
      } catch {}
    }
    throw err;
  }
}
`))

setActivePinia(createPinia())

const { calculateBlastRadius } = await import('../src/composables/incidentHelpers.ts')
const { useIncidents } = await import('../src/composables/useIncidents.ts')

describe('Task 073 Slice 73E: Final Frontend Polish, Zero-Occlusion & Telemetry Posture Tests', () => {
  describe('1. Incident Blast-Radius & SRE Health Posture Logic', () => {
    it('returns Critical Blast (92/100) and Degraded Node Capacity for active critical incidents', () => {
      const activeCriticalIncident = {
        id: 'inc-101',
        pod_name: 'payment-service-pod-0',
        namespace: 'prod-finance',
        cluster_name: 'prod-us-east',
        type: 'CrashLoopBackOff',
        severity: 'critical' as const,
        status: 'open',
        message: 'OOMKilled panic crash',
        created_at: new Date().toISOString()
      }

      const blast = calculateBlastRadius(activeCriticalIncident)
      assert.ok(blast)
      assert.equal(blast.impactScore, 92)
      assert.equal(blast.riskLevel, 'Critical Blast')
      assert.equal(blast.nodeHealth, 'Degraded Node Capacity')
      assert.equal(blast.affectedPod, 'payment-service-pod-0')
    })

    it('returns Contained (0 Active) and Nominal (Resolved) when incident status is resolved', () => {
      const resolvedIncident = {
        id: 'inc-102',
        pod_name: 'payment-service-pod-0',
        namespace: 'prod-finance',
        cluster_name: 'prod-us-east',
        type: 'CrashLoopBackOff',
        severity: 'critical' as const,
        status: 'resolved',
        message: 'Issue remediated via rollback',
        created_at: new Date().toISOString()
      }

      const blast = calculateBlastRadius(resolvedIncident)
      assert.ok(blast)
      assert.equal(blast.impactScore, 0)
      assert.equal(blast.riskLevel, 'Contained (0 Active)')
      assert.equal(blast.nodeHealth, 'Nominal (Resolved)')
    })

    it('returns Contained (0 Active) and Nominal (Resolved) when incident status is mitigated', () => {
      const mitigatedIncident = {
        id: 'inc-103',
        pod_name: 'auth-service-pod-2',
        namespace: 'auth',
        cluster_name: 'prod-eu-west',
        type: 'NodeFencing',
        severity: 'high' as const,
        status: 'mitigated',
        message: 'Node isolated and traffic rerouted',
        created_at: new Date().toISOString()
      }

      const blast = calculateBlastRadius(mitigatedIncident)
      assert.ok(blast)
      assert.equal(blast.impactScore, 0)
      assert.equal(blast.riskLevel, 'Contained (0 Active)')
      assert.equal(blast.nodeHealth, 'Nominal (Resolved)')
    })

    it('useIncidents computes truthful Contained posture when no open incidents exist', () => {
      setActivePinia(createPinia())
      const { selectedBlastRadius, selectedIncident, incidents } = useIncidents()
      incidents.value = [
        {
          id: 'inc-1',
          pod_name: 'pod-1',
          namespace: 'default',
          cluster_name: 'cluster-a',
          type: 'OOM',
          severity: 'critical' as const,
          status: 'resolved',
          created_at: new Date().toISOString()
        }
      ]
      selectedIncident.value = incidents.value[0]

      assert.ok(selectedBlastRadius.value)
      assert.equal(selectedBlastRadius.value.impactScore, 0)
      assert.equal(selectedBlastRadius.value.riskLevel, 'Contained (0 Active)')
      assert.equal(selectedBlastRadius.value.nodeHealth, 'Nominal (Resolved)')
    })
  })

  describe('2. ClickHouse Engine Telemetry & Badge Semantics', () => {
    function computeEngineMetrics(payload: { engine: string; status: string; latency_ms: number }) {
      const isOffline = payload.status === 'unavailable' || payload.status === 'offline'
      const isPrimary =
        payload.engine.includes('Distributed') ||
        payload.engine.includes('Edge LogEngine') ||
        payload.engine.includes('ClickHouse MergeTree')
      const isFallback = !isOffline && (isPrimary ? false : payload.status === 'fallback')
      const storageFormat = isOffline
        ? 'Unavailable'
        : (payload.engine.toLowerCase().includes('clickhouse')
          ? 'ClickHouse MergeTree'
          : 'Columnar Blocks (ZSTD >10x)')
      const latencyStr = payload.latency_ms > 0 ? `${payload.latency_ms.toFixed(1)}ms` : '<1ms'
      const badgeText = isOffline
        ? 'Log Engine · Offline'
        : (isFallback ? `${payload.engine} · Fallback` : `${payload.engine} · Ready · ${latencyStr}`)

      return { isOffline, isFallback, storageFormat, badgeText }
    }

    it('correctly reports ClickHouse MergeTree primary ready status with query latency', () => {
      const metrics = computeEngineMetrics({
        engine: 'ClickHouse MergeTree',
        status: 'connected',
        latency_ms: 1.4
      })

      assert.equal(metrics.isOffline, false)
      assert.equal(metrics.isFallback, false)
      assert.equal(metrics.storageFormat, 'ClickHouse MergeTree')
      assert.equal(metrics.badgeText, 'ClickHouse MergeTree · Ready · 1.4ms')
    })

    it('correctly reports Fallback status for in-memory ringbuffer degradation', () => {
      const metrics = computeEngineMetrics({
        engine: 'RingBuffer In-Memory',
        status: 'fallback',
        latency_ms: 0.2
      })

      assert.equal(metrics.isOffline, false)
      assert.equal(metrics.isFallback, true)
      assert.equal(metrics.storageFormat, 'Columnar Blocks (ZSTD >10x)')
      assert.equal(metrics.badgeText, 'RingBuffer In-Memory · Fallback')
    })

    it('correctly reports Offline status when clickhouse daemon is unreachable', () => {
      const metrics = computeEngineMetrics({
        engine: 'Unavailable',
        status: 'unavailable',
        latency_ms: 0
      })

      assert.equal(metrics.isOffline, true)
      assert.equal(metrics.isFallback, false)
      assert.equal(metrics.storageFormat, 'Unavailable')
      assert.equal(metrics.badgeText, 'Log Engine · Offline')
    })
  })

  describe('3. PWA Inline Installation Dismissal Logic', () => {
    function computePwaVisibility(opts: {
      isDismissed: boolean
      isInstalled: boolean
      isIosStandalone: boolean
      hasDeferredPrompt: boolean
    }) {
      if (opts.isDismissed) return false
      if (opts.isInstalled || opts.isIosStandalone) return false
      return opts.hasDeferredPrompt
    }

    it('shows inline card only when prompt is deferred and user has not dismissed or installed', () => {
      assert.equal(
        computePwaVisibility({
          isDismissed: false,
          isInstalled: false,
          isIosStandalone: false,
          hasDeferredPrompt: true
        }),
        true
      )
    })

    it('suppresses inline card if user already installed app', () => {
      assert.equal(
        computePwaVisibility({
          isDismissed: false,
          isInstalled: true,
          isIosStandalone: false,
          hasDeferredPrompt: true
        }),
        false
      )
    })

    it('suppresses inline card if banner was dismissed into localStorage TTL', () => {
      assert.equal(
        computePwaVisibility({
          isDismissed: true,
          isInstalled: false,
          isIosStandalone: false,
          hasDeferredPrompt: true
        }),
        false
      )
    })

    it('suppresses inline card if browser did not fire beforeinstallprompt', () => {
      assert.equal(
        computePwaVisibility({
          isDismissed: false,
          isInstalled: false,
          isIosStandalone: false,
          hasDeferredPrompt: false
        }),
        false
      )
    })
  })
})
