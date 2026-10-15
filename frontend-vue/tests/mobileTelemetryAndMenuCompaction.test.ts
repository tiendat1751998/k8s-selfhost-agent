import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface DeploymentApp {
  id: string
  name: string
  namespace: string
  replicas: number
  available_replicas: number
  cpu?: string
  memory?: string
  restarts?: number
  age?: string
  status: 'healthy' | 'progressing' | 'degraded' | 'failed'
}

export function calculateDeploymentsTelemetry(apps: DeploymentApp[]): {
  totalApps: number
  healthyApps: number
  degradedApps: number
  totalReplicas: number
  availableReplicas: number
} {
  let totalReplicas = 0
  let availableReplicas = 0
  let healthyApps = 0
  let degradedApps = 0

  for (const app of apps) {
    totalReplicas += app.replicas || 0
    availableReplicas += app.available_replicas || 0
    if (app.status === 'healthy') {
      healthyApps++
    } else {
      degradedApps++
    }
  }

  return {
    totalApps: apps.length,
    healthyApps,
    degradedApps,
    totalReplicas,
    availableReplicas
  }
}

export function shouldHighlightRestarts(restarts?: number): boolean {
  return typeof restarts === 'number' && restarts > 0
}

export function resolveInspectorGridColumns(viewportWidth: number): number {
  return viewportWidth <= 640 ? 2 : 4
}

describe('Mobile Telemetry & Header Stack Compaction Specifications', () => {
  const sampleApps: DeploymentApp[] = [
    {
      id: 'd1',
      name: 'auth-service',
      namespace: 'production',
      replicas: 3,
      available_replicas: 3,
      cpu: '250m',
      memory: '512Mi',
      restarts: 0,
      age: '14d',
      status: 'healthy'
    },
    {
      id: 'd2',
      name: 'order-api',
      namespace: 'production',
      replicas: 5,
      available_replicas: 4,
      cpu: '800m',
      memory: '1024Mi',
      restarts: 4, // highlight in amber
      age: '2d',
      status: 'degraded'
    }
  ]

  it('calculateDeploymentsTelemetry derives total and available replicas truthfully', () => {
    const summary = calculateDeploymentsTelemetry(sampleApps)
    assert.equal(summary.totalApps, 2)
    assert.equal(summary.healthyApps, 1)
    assert.equal(summary.degradedApps, 1)
    assert.equal(summary.totalReplicas, 8)
    assert.equal(summary.availableReplicas, 7)
  })

  it('calculateDeploymentsTelemetry returns zeroes on empty app array', () => {
    const summary = calculateDeploymentsTelemetry([])
    assert.equal(summary.totalApps, 0)
    assert.equal(summary.totalReplicas, 0)
    assert.equal(summary.availableReplicas, 0)
  })

  it('shouldHighlightRestarts flags workloads with restart counts greater than zero', () => {
    assert.equal(shouldHighlightRestarts(0), false)
    assert.equal(shouldHighlightRestarts(undefined), false)
    assert.equal(shouldHighlightRestarts(-1), false)
    assert.equal(shouldHighlightRestarts(1), true)
    assert.equal(shouldHighlightRestarts(12), true)
  })

  it('resolveInspectorGridColumns sets 2 columns on mobile viewports and 4 on desktop', () => {
    assert.equal(resolveInspectorGridColumns(375), 2)
    assert.equal(resolveInspectorGridColumns(640), 2)
    assert.equal(resolveInspectorGridColumns(768), 4)
    assert.equal(resolveInspectorGridColumns(1440), 4)
  })
})
