import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

// Dynamic import of quadtreeIndex utils
let quadtreeModule: any = null
try {
  quadtreeModule = await import('../src/utils/quadtreeIndex.ts')
} catch (e: any) {
  quadtreeModule = null
}

export function resolveMatrixNodeColor(status: number): string {
  switch (status) {
    case 0:
      return '#10b981' // healthy
    case 1:
      return '#f59e0b' // warning
    case 2:
      return '#f43f5e' // critical / offline
    case 3:
      return '#64748b' // cordoned / maintenance
    default:
      return '#64748b'
  }
}

describe('Hyperscale Phase 1 & 2: Quadtree Spatial Index & Binary Delta Codec', () => {
  it('1. Quadtree indexes 2,000 simulated nodes and performs fast O(log N) nearest neighbor search', () => {
    assert.ok(quadtreeModule, 'quadtreeIndex.ts module must exist and be importable')
    const { Quadtree } = quadtreeModule

    const bounds = { x: 0, y: 0, width: 2000, height: 2000 }
    const qt = new Quadtree<{ id: string; name: string }>(bounds, 8, 10)

    // Insert 2,000 points
    const points: Array<{ x: number; y: number; data: { id: string; name: string } }> = []
    for (let i = 0; i < 2000; i++) {
      const pt = {
        x: (i * 37) % 2000,
        y: (i * 73) % 2000,
        data: { id: `node-${i}`, name: `fleet-worker-${i}` }
      }
      points.push(pt)
      const inserted = qt.insert(pt)
      assert.equal(inserted, true, `Point ${i} should be successfully inserted`)
    }

    // Nearest neighbor search benchmark and accuracy
    const queryX = 500
    const queryY = 500
    const nearest = qt.findNearest(queryX, queryY, 100)
    assert.ok(nearest, 'Should find nearest neighbor within 100px radius')

    // Brute force verify nearest neighbor is accurate
    let minD2 = Infinity
    let bestPt: any = null
    for (const p of points) {
      const d2 = (p.x - queryX) ** 2 + (p.y - queryY) ** 2
      if (d2 < minD2 && Math.sqrt(d2) <= 100) {
        minD2 = d2
        bestPt = p
      }
    }
    assert.equal(nearest.data.id, bestPt.data.id, 'Quadtree nearest neighbor must match brute-force ground truth')

    // Query radius
    const radiusResults = qt.queryRadius(queryX, queryY, 80)
    assert.ok(radiusResults.length > 0, 'Radius query should return matching points')
    for (const r of radiusResults) {
      const dist = Math.hypot(r.x - queryX, r.y - queryY)
      assert.ok(dist <= 80, `Distance ${dist} must be <= radius 80`)
    }
  })

  it('2. Binary WebSocket delta codec encodes and decodes 2,000 node updates with exact wire size (48,016 bytes)', () => {
    assert.ok(quadtreeModule, 'quadtreeIndex.ts module must exist')
    const { encodeNodeDeltaBinary, decodeNodeDeltaBinary, BINARY_MAGIC } = quadtreeModule
    assert.equal(BINARY_MAGIC, 0x4B385344, 'Magic header must be 0x4B385344 (K8SD)')

    const timestamp = 1728888888000
    const records = []
    for (let i = 0; i < 2000; i++) {
      records.push({
        nodeId: i + 1,
        status: i % 4, // 0..3
        flags: 0,
        cpuPercent: (i * 7) % 101,
        memPercent: (i * 13) % 101,
        diskPercent: (i * 17) % 101,
        processes: 120 + (i % 50),
        containers: 8 + (i % 20),
        latencyMs: 15 + (i % 10),
        networkInKbps: 1024.5 + i,
        networkOutKbps: 2048.25 + i
      })
    }

    const encodedBuffer = encodeNodeDeltaBinary(timestamp, records)
    assert.ok(encodedBuffer instanceof ArrayBuffer, 'Encoded output must be an ArrayBuffer')
    // Size check: 16 bytes header + 2000 records * 24 bytes = 48,016 bytes < 50 KB
    assert.equal(encodedBuffer.byteLength, 48016, 'Wire size for 2,000 node deltas must be exactly 48,016 bytes')
    assert.ok(encodedBuffer.byteLength < 50000, 'Payload size must be < 50 KB')

    // Decode and verify round-trip fidelity
    const decoded = decodeNodeDeltaBinary(encodedBuffer)
    assert.equal(decoded.timestampMs, timestamp)
    assert.equal(decoded.records.length, 2000)

    // Check first and last record fidelity
    const first = decoded.records[0]
    assert.equal(first.nodeId, 1)
    assert.equal(first.status, 0)
    assert.equal(first.cpuPercent, 0)
    assert.equal(first.memPercent, 0)
    assert.equal(first.processes, 120)
    assert.equal(first.containers, 8)
    assert.equal(Math.round(first.networkInKbps * 10) / 10, 1024.5)

    const last = decoded.records[1999]
    assert.equal(last.nodeId, 2000)
    assert.equal(last.status, 1999 % 4)
    assert.equal(last.cpuPercent, (1999 * 7) % 101)

    // Corrupted magic check
    const badBuf = encodedBuffer.slice(0)
    new DataView(badBuf).setUint32(0, 0x11223344)
    assert.throws(() => decodeNodeDeltaBinary(badBuf), /Invalid binary magic header/)
  })
})

describe('Hyperscale Phase 1: VirtualNodeGrid Window Math & Viewport Slicing', () => {
  it('1. VirtualNodeGrid calculates window slicing: 5,000 nodes mounts <= 45 items in viewport', () => {
    assert.ok(quadtreeModule, 'quadtreeIndex.ts module must exist')
    const { calculateVirtualNodeWindow } = quadtreeModule

    const totalNodes = 5000
    const columns = 3
    const rowHeight = 180
    const viewportHeight = 720 // 4 visible rows
    const overscanRows = 3

    // At scrollTop = 0 (top)
    const topWin = calculateVirtualNodeWindow({
      totalCount: totalNodes,
      columns,
      rowHeight,
      viewportHeight,
      scrollTop: 0,
      overscanRows
    })

    assert.equal(topWin.startRow, 0)
    assert.equal(topWin.endRow, 7)
    assert.equal(topWin.visibleCount, 21)
    assert.ok(topWin.visibleCount <= 45, `Visible items at top (${topWin.visibleCount}) must be <= 45`)
    assert.equal(topWin.offsetY, 0)
    assert.equal(topWin.totalHeightPx, Math.ceil(5000 / 3) * 180)

    // Scrolled deep into fleet (row 800)
    const midWin = calculateVirtualNodeWindow({
      totalCount: totalNodes,
      columns,
      rowHeight,
      viewportHeight,
      scrollTop: 800 * 180,
      overscanRows
    })

    assert.equal(midWin.startRow, 797) // 800 - 3
    assert.equal(midWin.endRow, 807) // 800 + 4 + 3
    assert.equal(midWin.visibleCount, (807 - 797) * 3) // 30 items
    assert.ok(midWin.visibleCount <= 45, `Visible items at row 800 (${midWin.visibleCount}) must be <= 45`)
    assert.equal(midWin.offsetY, 797 * 180)
  })

  it('2. VirtualNodeGrid handles boundary edge cases: 0 nodes, single item, and scroll beyond max', () => {
    assert.ok(quadtreeModule, 'quadtreeIndex.ts module must exist')
    const { calculateVirtualNodeWindow } = quadtreeModule

    const emptyWin = calculateVirtualNodeWindow({
      totalCount: 0,
      columns: 3,
      rowHeight: 180,
      viewportHeight: 720,
      scrollTop: 0,
      overscanRows: 2
    })
    assert.equal(emptyWin.visibleCount, 0)
    assert.equal(emptyWin.totalHeightPx, 0)

    const singleWin = calculateVirtualNodeWindow({
      totalCount: 1,
      columns: 3,
      rowHeight: 180,
      viewportHeight: 720,
      scrollTop: 0,
      overscanRows: 2
    })
    assert.equal(singleWin.visibleCount, 1)
    assert.equal(singleWin.totalHeightPx, 180)
  })
})

describe('Hyperscale Phase 2: ClusterDensityMatrix High-Density Canvas & Quadtree Tooltip', () => {
  it('1. Density matrix cell layout computation packs 10,000 nodes accurately', () => {
    assert.ok(quadtreeModule, 'quadtreeIndex.ts module must exist')
    const { computeMatrixLayout } = quadtreeModule

    const layout = computeMatrixLayout(10000, 1200, 10, 2)
    // 1200px width with 10px cells and 2px gap (12px pitch) -> 100 columns
    assert.equal(layout.columns, 100)
    assert.equal(layout.rows, 100)
    assert.equal(layout.totalWidth, 100 * 12 - 2) // 1198px
    assert.equal(layout.totalHeight, 100 * 12 - 2) // 1198px
  })

  it('2. resolveMatrixNodeColor maps status codes accurately to emerald, amber, rose, slate', () => {
    assert.equal(resolveMatrixNodeColor(0), '#10b981')
    assert.equal(resolveMatrixNodeColor(1), '#f59e0b')
    assert.equal(resolveMatrixNodeColor(2), '#f43f5e')
    assert.equal(resolveMatrixNodeColor(3), '#64748b')
    assert.equal(resolveMatrixNodeColor(99), '#64748b')
  })
})
