import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface ComputeHost {
  id: string
  name: string
  endpoint: string
  status: 'connected' | 'degraded' | 'disconnected' | 'maintenance'
  role: 'control-plane' | 'worker' | 'storage' | 'edge'
  tenant_id?: string
  latency_ms?: number
  last_health_check?: string
}

export function calculateHostTelemetry(hosts: ComputeHost[]): {
  onlineHostsCount: number
  offlineHostsCount: number
  totalHostsCount: number
} {
  const onlineHostsCount = hosts.filter(h => h.status === 'connected').length
  const offlineHostsCount = hosts.filter(h => h.status === 'disconnected' || h.status === 'degraded').length
  return {
    onlineHostsCount,
    offlineHostsCount,
    totalHostsCount: hosts.length
  }
}

export function resolveHostTenantId(hosts: ComputeHost[]): string {
  const target = hosts.find(h => h.status !== 'connected')
  return target?.tenant_id || 'default'
}

export function formatHostLatency(latencyMs?: number): string {
  if (typeof latencyMs !== 'number' || latencyMs <= 0 || !Number.isFinite(latencyMs)) {
    return '--'
  }
  return `${Math.round(latencyMs)}ms`
}

export function mapHostStatusToBadge(status: ComputeHost['status']): {
  color: 'emerald' | 'amber' | 'rose' | 'slate'
  label: string
} {
  switch (status) {
    case 'connected':
      return { color: 'emerald', label: 'Connected' }
    case 'degraded':
      return { color: 'amber', label: 'Degraded' }
    case 'disconnected':
      return { color: 'rose', label: 'Disconnected' }
    case 'maintenance':
    default:
      return { color: 'slate', label: 'Maintenance' }
  }
}

describe('Mobile Hosts View Defect Remediation & Card Layout Specifications', () => {
  const sampleHosts: ComputeHost[] = [
    {
      id: 'host-1',
      name: 'k8s-cp-01',
      endpoint: '192.168.1.10:6443',
      status: 'connected',
      role: 'control-plane',
      tenant_id: 'enterprise-prod',
      latency_ms: 12
    },
    {
      id: 'host-2',
      name: 'k8s-worker-01',
      endpoint: '192.168.1.11:10250',
      status: 'connected',
      role: 'worker',
      tenant_id: 'enterprise-prod',
      latency_ms: 8
    },
    {
      id: 'host-3',
      name: 'k8s-worker-02',
      endpoint: '192.168.1.12:10250',
      status: 'disconnected',
      role: 'worker',
      tenant_id: 'enterprise-prod',
      latency_ms: 0 // offline host returns 0 latency
    }
  ]

  it('calculateHostTelemetry computes online and offline host counts accurately', () => {
    const telemetry = calculateHostTelemetry(sampleHosts)
    assert.equal(telemetry.onlineHostsCount, 2)
    assert.equal(telemetry.offlineHostsCount, 1)
    assert.equal(telemetry.totalHostsCount, 3)
  })

  it('resolveHostTenantId dynamically extracts tenant_id from disconnected or degraded host', () => {
    const tenant = resolveHostTenantId(sampleHosts)
    assert.equal(tenant, 'enterprise-prod')

    const allConnected: ComputeHost[] = [
      { id: 'h1', name: 'w1', endpoint: '10.0.0.1', status: 'connected', role: 'worker' }
    ]
    assert.equal(resolveHostTenantId(allConnected), 'default')
  })

  it('formatHostLatency displays -- for offline/0ms hosts and formatted ms for live hosts', () => {
    assert.equal(formatHostLatency(0), '--')
    assert.equal(formatHostLatency(undefined), '--')
    assert.equal(formatHostLatency(-10), '--')
    assert.equal(formatHostLatency(14.8), '15ms')
    assert.equal(formatHostLatency(5), '5ms')
  })

  it('mapHostStatusToBadge maps statuses to proper semantic status badges', () => {
    assert.deepEqual(mapHostStatusToBadge('connected'), { color: 'emerald', label: 'Connected' })
    assert.deepEqual(mapHostStatusToBadge('degraded'), { color: 'amber', label: 'Degraded' })
    assert.deepEqual(mapHostStatusToBadge('disconnected'), { color: 'rose', label: 'Disconnected' })
    assert.deepEqual(mapHostStatusToBadge('maintenance'), { color: 'slate', label: 'Maintenance' })
  })
})
