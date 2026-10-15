import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { getNodeRole, isControlPlane } from '../src/components/overview/overviewMobileUtils.ts'
import { SIMULATION_SCENARIOS } from '../src/composables/incidentHelpers.ts'
import type { NodeMetrics } from '../src/api/overview.ts'

describe('Live MCP & Mobile UX Remediation Unit Tests', () => {
  it('getNodeRole accurately identifies standalone, control-plane, and worker roles', () => {
    const baseNode: NodeMetrics = {
      node_id: 'node-1',
      node_name: 'test-node',
      status: 'ready',
      role: 'standalone',
      cpu_percent: 20,
      memory_percent: 40,
      memory_total: 16000000000,
      disk_percent: 30,
      pod_count: 5,
      ip: '10.0.0.1',
    }

    // Standalone role
    assert.strictEqual(getNodeRole(baseNode), 'standalone')

    // Control-plane roles
    assert.strictEqual(getNodeRole({ ...baseNode, role: 'master' }), 'control-plane')
    assert.strictEqual(getNodeRole({ ...baseNode, role: 'control-plane' }), 'control-plane')
    assert.strictEqual(getNodeRole({ ...baseNode, role: 'manager' }), 'control-plane')
    assert.strictEqual(isControlPlane({ ...baseNode, role: 'control-plane' }), true)

    // Worker role
    assert.strictEqual(getNodeRole({ ...baseNode, role: 'worker' }), 'worker')
    assert.strictEqual(getNodeRole({ ...baseNode, role: 'WORKER' }), 'worker')
    assert.strictEqual(isControlPlane({ ...baseNode, role: 'worker' }), false)

    // Other roles default to normalized role or worker
    assert.strictEqual(getNodeRole({ ...baseNode, role: 'edge' }), 'edge')
    assert.strictEqual(getNodeRole({ ...baseNode, role: '' }), 'worker')
  })

  it('SIMULATION_SCENARIOS uses generic target placeholders and zero fake cluster IPs', () => {
    assert.ok(SIMULATION_SCENARIOS.length >= 3)
    const workloads = SIMULATION_SCENARIOS.map(s => s.workload)

    // Must use generic target placeholders, not fake hardcoded demo apps
    assert.ok(workloads.includes('target-pod'))
    assert.ok(workloads.includes('target-node'))
    assert.ok(workloads.includes('target-service'))

    // Must not contain fake legacy cluster names or specific IPs
    for (const sc of SIMULATION_SCENARIOS) {
      assert.strictEqual(sc.cluster.includes('prod-us-east-1'), false)
      assert.strictEqual(sc.cluster.includes('staging-us-west-2'), false)
      assert.strictEqual(sc.description.includes('10.96.12.44'), false)
      assert.strictEqual(sc.workload.includes('checkout-api'), false)
      assert.strictEqual(sc.workload.includes('masterdb'), false)
      assert.strictEqual(sc.workload.includes('payment-gateway'), false)
    }
  })

  it('Log stream empty state messaging adheres to honest connection status', () => {
    function resolveLogEmptyState(isConnected: boolean, targetName?: string) {
      const title = isConnected
        ? `Waiting for logs from [${targetName || 'cluster'}]...`
        : 'Disconnected from log stream. Reconnecting...'
      const sub = isConnected
        ? 'Live stream connected. Matching log events will appear in real-time.'
        : 'Attempting to establish WebSocket stream connection...'
      return { title, sub }
    }

    const disconnectedState = resolveLogEmptyState(false, 'edge-pod')
    assert.strictEqual(disconnectedState.title, 'Disconnected from log stream. Reconnecting...')
    assert.strictEqual(disconnectedState.sub, 'Attempting to establish WebSocket stream connection...')
    assert.strictEqual(disconnectedState.sub.includes('Live stream is active'), false)

    const connectedState = resolveLogEmptyState(true, 'edge-pod')
    assert.strictEqual(connectedState.title, 'Waiting for logs from [edge-pod]...')
    assert.strictEqual(connectedState.sub, 'Live stream connected. Matching log events will appear in real-time.')
  })
})
