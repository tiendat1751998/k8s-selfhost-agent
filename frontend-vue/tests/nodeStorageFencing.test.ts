import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface ProbeReport {
  ping_reachable: boolean
  kubelet_responsive: boolean
  containerd_responsive: boolean
  classification: 'unreachable' | 'degraded' | 'healthy'
}

export interface CapacityPlan {
  headroom_ram_percent: number
  headroom_cpu_percent: number
  safe_to_evict: boolean
}

export interface CircuitBreaker {
  eviction_frozen: boolean
  freeze_reason: string
}

export interface NodeFencingPreflight {
  node_name: string
  is_single_worker_cluster: boolean
  probe_report: ProbeReport
  capacity_plan: CapacityPlan
  circuit_breaker: CircuitBreaker
}

export interface FencingExecutionPayload {
  node_name: string
  confirm_power_isolated: boolean
  emergency_untaint_master?: boolean
  drain_grace_period_seconds: number
}

export function validateFencingPayload(payload: FencingExecutionPayload, preflight: NodeFencingPreflight): {
  valid: boolean
  error?: string
} {
  if (!payload.confirm_power_isolated) {
    return { valid: false, error: 'STONITH power isolation must be explicitly confirmed before fencing' }
  }
  if (preflight.circuit_breaker.eviction_frozen) {
    return { valid: false, error: `Circuit breaker active: ${preflight.circuit_breaker.freeze_reason}` }
  }
  if (preflight.is_single_worker_cluster && !payload.emergency_untaint_master) {
    return { valid: false, error: 'Single worker topology requires emergency untaint master acknowledgment' }
  }
  return { valid: true }
}

export function transitionFencingStage(
  currentStage: number,
  action: 'confirm' | 'execute_success' | 'execute_error' | 'cancel' | 'retry'
): number {
  switch (action) {
    case 'confirm':
      return 2 // Move to STONITH confirmation
    case 'execute_success':
      return 5 // Move to post-recovery summary
    case 'execute_error':
      return 2 // Anti-trapped UI: reset to confirmation stage so operator is not locked out
    case 'retry':
      return 3 // Re-run stepper
    case 'cancel':
      return 1 // Return to initial analysis
    default:
      return currentStage
  }
}

export function shouldCancelFencing(rescueResult: {
  cancel_fencing?: boolean
  success?: boolean
  recovered_to_ready?: boolean
}): boolean {
  return Boolean(rescueResult.cancel_fencing || rescueResult.success || rescueResult.recovered_to_ready)
}

export function resolveFencingConfidence(preflight?: {
  dynamic_rca?: { confidence?: number }
  confidence?: number
}): number {
  return preflight?.dynamic_rca?.confidence ?? preflight?.confidence ?? 0
}

describe('Task 065: Dead Node Recovery & Zero-Data-Loss Storage Fencing Logic Tests', () => {
  const mockPreflight: NodeFencingPreflight = {
    node_name: 'worker-node-03',
    is_single_worker_cluster: false,
    probe_report: {
      ping_reachable: false,
      kubelet_responsive: false,
      containerd_responsive: false,
      classification: 'unreachable'
    },
    capacity_plan: {
      headroom_ram_percent: 35.5,
      headroom_cpu_percent: 42.0,
      safe_to_evict: true
    },
    circuit_breaker: {
      eviction_frozen: false,
      freeze_reason: ''
    }
  }

  it('validateFencingPayload rejects execution if STONITH power isolation is not confirmed', () => {
    const payload: FencingExecutionPayload = {
      node_name: 'worker-node-03',
      confirm_power_isolated: false,
      drain_grace_period_seconds: 30
    }
    const result = validateFencingPayload(payload, mockPreflight)
    assert.equal(result.valid, false)
    assert.match(result.error || '', /STONITH power isolation/)
  })

  it('validateFencingPayload rejects execution if circuit breaker has frozen evictions', () => {
    const frozenPreflight: NodeFencingPreflight = {
      ...mockPreflight,
      circuit_breaker: {
        eviction_frozen: true,
        freeze_reason: 'Remaining nodes lack sufficient memory headroom (< 10%)'
      }
    }
    const payload: FencingExecutionPayload = {
      node_name: 'worker-node-03',
      confirm_power_isolated: true,
      drain_grace_period_seconds: 30
    }
    const result = validateFencingPayload(payload, frozenPreflight)
    assert.equal(result.valid, false)
    assert.match(result.error || '', /Circuit breaker active/)
  })

  it('validateFencingPayload handles single-worker cluster topology requiring emergency untaint', () => {
    const singleWorkerPreflight: NodeFencingPreflight = {
      ...mockPreflight,
      is_single_worker_cluster: true
    }
    const unconfirmedPayload: FencingExecutionPayload = {
      node_name: 'worker-node-03',
      confirm_power_isolated: true,
      emergency_untaint_master: false,
      drain_grace_period_seconds: 30
    }
    const unconfirmedRes = validateFencingPayload(unconfirmedPayload, singleWorkerPreflight)
    assert.equal(unconfirmedRes.valid, false)
    assert.match(unconfirmedRes.error || '', /Single worker topology/)

    const confirmedPayload: FencingExecutionPayload = {
      ...unconfirmedPayload,
      emergency_untaint_master: true
    }
    const confirmedRes = validateFencingPayload(confirmedPayload, singleWorkerPreflight)
    assert.equal(confirmedRes.valid, true)
  })

  it('transitionFencingStage handles anti-trapped UI state transitions', () => {
    assert.equal(transitionFencingStage(1, 'confirm'), 2)
    assert.equal(transitionFencingStage(2, 'execute_success'), 5)
    // Error must reset to stage 2 so operator sees confirmation screen and is not trapped
    assert.equal(transitionFencingStage(3, 'execute_error'), 2)
    assert.equal(transitionFencingStage(2, 'cancel'), 1)
  })

  it('shouldCancelFencing checks cancel_fencing, success, and recovered_to_ready flags', () => {
    assert.equal(shouldCancelFencing({ cancel_fencing: true }), true)
    assert.equal(shouldCancelFencing({ success: true }), true)
    assert.equal(shouldCancelFencing({ recovered_to_ready: true }), true)
    assert.equal(shouldCancelFencing({ cancel_fencing: false, success: false }), false)
    assert.equal(shouldCancelFencing({}), false)
  })

  it('resolveFencingConfidence falls back strictly to 0 instead of hardcoded 0.94 fallback', () => {
    assert.equal(resolveFencingConfidence({ dynamic_rca: { confidence: 0.88 } }), 0.88)
    assert.equal(resolveFencingConfidence({ confidence: 0.75 }), 0.75)
    assert.equal(resolveFencingConfidence({}), 0)
    assert.equal(resolveFencingConfidence(undefined), 0)
    assert.notEqual(resolveFencingConfidence({}), 0.94)
  })
})
