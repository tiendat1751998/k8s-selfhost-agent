import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface NodeMetrics {
  nodeId: string
  nodeName: string
  cpuAllocatableCores: number
  cpuUsedCores: number
  memAllocatableGb: number
  memUsedGb: number
  isCordoned?: boolean
}

describe('Task 013: Capacity & Headroom Calculations Tests', () => {
  function calculateHeadroom(node: NodeMetrics): {
    cpuCommitmentPct: number
    memCommitmentPct: number
    cpuHeadroomCores: number
    memHeadroomGb: number
    healthStatus: 'Ready' | 'Cordoned' | 'High Commitment'
  } {
    const cpuCommitmentPct = node.cpuAllocatableCores > 0
      ? Number(((node.cpuUsedCores / node.cpuAllocatableCores) * 100).toFixed(1))
      : 0
    const memCommitmentPct = node.memAllocatableGb > 0
      ? Number(((node.memUsedGb / node.memAllocatableGb) * 100).toFixed(1))
      : 0

    const cpuHeadroomCores = Math.max(0, Number((node.cpuAllocatableCores - node.cpuUsedCores).toFixed(2)))
    const memHeadroomGb = Math.max(0, Number((node.memAllocatableGb - node.memUsedGb).toFixed(2)))

    let healthStatus: 'Ready' | 'Cordoned' | 'High Commitment' = 'Ready'
    if (node.isCordoned) {
      healthStatus = 'Cordoned'
    } else if (cpuCommitmentPct >= 90 || memCommitmentPct >= 90) {
      healthStatus = 'High Commitment'
    }

    return {
      cpuCommitmentPct,
      memCommitmentPct,
      cpuHeadroomCores,
      memHeadroomGb,
      healthStatus
    }
  }

  it('calculateHeadroom accurately derives commitment percentages and headroom values', () => {
    const node: NodeMetrics = {
      nodeId: 'node-01',
      nodeName: 'k8s-worker-01',
      cpuAllocatableCores: 16,
      cpuUsedCores: 8,
      memAllocatableGb: 64,
      memUsedGb: 32
    }
    const res = calculateHeadroom(node)
    assert.equal(res.cpuCommitmentPct, 50.0)
    assert.equal(res.memCommitmentPct, 50.0)
    assert.equal(res.cpuHeadroomCores, 8)
    assert.equal(res.memHeadroomGb, 32)
    assert.equal(res.healthStatus, 'Ready')
  })

  it('calculateHeadroom flags High Commitment status when CPU or Memory >= 90%', () => {
    const highCpu: NodeMetrics = {
      nodeId: 'node-02',
      nodeName: 'k8s-worker-02',
      cpuAllocatableCores: 16,
      cpuUsedCores: 15, // 93.75%
      memAllocatableGb: 64,
      memUsedGb: 30
    }
    const resCpu = calculateHeadroom(highCpu)
    assert.equal(resCpu.healthStatus, 'High Commitment')

    const highMem: NodeMetrics = {
      nodeId: 'node-03',
      nodeName: 'k8s-worker-03',
      cpuAllocatableCores: 16,
      cpuUsedCores: 5,
      memAllocatableGb: 64,
      memUsedGb: 58 // 90.6%
    }
    const resMem = calculateHeadroom(highMem)
    assert.equal(resMem.healthStatus, 'High Commitment')
  })

  it('calculateHeadroom prioritizes Cordoned status over high commitment', () => {
    const cordonedNode: NodeMetrics = {
      nodeId: 'node-04',
      nodeName: 'k8s-worker-04',
      cpuAllocatableCores: 16,
      cpuUsedCores: 15.5,
      memAllocatableGb: 64,
      memUsedGb: 60,
      isCordoned: true
    }
    const res = calculateHeadroom(cordonedNode)
    assert.equal(res.healthStatus, 'Cordoned')
  })

  it('calculateHeadroom clamps negative headroom to 0 when node is overcommitted', () => {
    const overcommitted: NodeMetrics = {
      nodeId: 'node-05',
      nodeName: 'k8s-worker-05',
      cpuAllocatableCores: 8,
      cpuUsedCores: 10, // overcommit
      memAllocatableGb: 32,
      memUsedGb: 36 // overcommit
    }
    const res = calculateHeadroom(overcommitted)
    assert.equal(res.cpuHeadroomCores, 0)
    assert.equal(res.memHeadroomGb, 0)
    assert.ok(res.cpuCommitmentPct > 100)
    assert.equal(res.healthStatus, 'High Commitment')
  })
})
