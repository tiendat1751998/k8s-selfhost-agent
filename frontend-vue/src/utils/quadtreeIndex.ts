export interface QuadBounds { x: number; y: number; width: number; height: number }
export interface QuadPoint<T> { x: number; y: number; data: T }

function sqDistToBox(px: number, py: number, b: QuadBounds): number {
  const dx = Math.max(b.x - px, 0, px - (b.x + b.width))
  const dy = Math.max(b.y - py, 0, py - (b.y + b.height))
  return dx * dx + dy * dy
}

export class Quadtree<T> {
  bounds: QuadBounds
  capacity: number
  maxDepth: number
  depth: number
  points: QuadPoint<T>[] = []
  divided = false
  nw?: Quadtree<T>; ne?: Quadtree<T>; sw?: Quadtree<T>; se?: Quadtree<T>

  constructor(bounds: QuadBounds, capacity = 8, maxDepth = 8, depth = 0) {
    this.bounds = bounds
    this.capacity = capacity
    this.maxDepth = maxDepth
    this.depth = depth
  }

  insert(pt: QuadPoint<T>): boolean {
    const { x, y, width, height } = this.bounds
    if (pt.x < x || pt.x > x + width || pt.y < y || pt.y > y + height) return false
    if (this.points.length < this.capacity || this.depth >= this.maxDepth) {
      this.points.push(pt)
      return true
    }
    if (!this.divided) this.subdivide()
    return !!(this.nw?.insert(pt) || this.ne?.insert(pt) || this.sw?.insert(pt) || this.se?.insert(pt))
  }

  private subdivide(): void {
    const { x, y, width, height } = this.bounds
    const halfW = width / 2, halfH = height / 2, d = this.depth + 1
    this.nw = new Quadtree<T>({ x, y, width: halfW, height: halfH }, this.capacity, this.maxDepth, d)
    this.ne = new Quadtree<T>({ x: x + halfW, y, width: halfW, height: halfH }, this.capacity, this.maxDepth, d)
    this.sw = new Quadtree<T>({ x, y: y + halfH, width: halfW, height: halfH }, this.capacity, this.maxDepth, d)
    this.se = new Quadtree<T>({ x: x + halfW, y: y + halfH, width: halfW, height: halfH }, this.capacity, this.maxDepth, d)
    this.divided = true
    const oldPoints = this.points
    this.points = []
    for (const p of oldPoints) this.nw.insert(p) || this.ne.insert(p) || this.sw.insert(p) || this.se.insert(p)
  }

  queryRadius(cx: number, cy: number, radius: number): QuadPoint<T>[] {
    const results: QuadPoint<T>[] = []
    if (sqDistToBox(cx, cy, this.bounds) > radius * radius) return results
    for (const p of this.points) {
      if ((p.x - cx) ** 2 + (p.y - cy) ** 2 <= radius * radius) results.push(p)
    }
    if (this.divided) {
      results.push(...this.nw!.queryRadius(cx, cy, radius), ...this.ne!.queryRadius(cx, cy, radius),
                   ...this.sw!.queryRadius(cx, cy, radius), ...this.se!.queryRadius(cx, cy, radius))
    }
    return results
  }

  findNearest(qx: number, qy: number, maxDist: number): QuadPoint<T> | null {
    let bestDistSq = maxDist * maxDist
    let bestPoint: QuadPoint<T> | null = null

    const search = (node: Quadtree<T>): void => {
      if (sqDistToBox(qx, qy, node.bounds) > bestDistSq) return
      for (const p of node.points) {
        const d2 = (p.x - qx) ** 2 + (p.y - qy) ** 2
        if (d2 <= bestDistSq) {
          bestDistSq = d2
          bestPoint = p
        }
      }
      if (node.divided) {
        const children = [node.nw!, node.ne!, node.sw!, node.se!]
        children.sort((a, b) => sqDistToBox(qx, qy, a.bounds) - sqDistToBox(qx, qy, b.bounds))
        for (const child of children) search(child)
      }
    }
    search(this)
    return bestPoint
  }
}

export const BINARY_MAGIC = 0x4b385344
export const BINARY_VERSION = 1

export interface BinaryNodeDelta {
  nodeId: number; status: number; flags?: number
  cpuPercent: number; memPercent: number; diskPercent?: number
  processes?: number; containers?: number; latencyMs?: number
  networkInKbps?: number; networkOutKbps?: number
}

export function encodeNodeDeltaBinary(timestampMs: number, records: BinaryNodeDelta[]): ArrayBuffer {
  const buf = new ArrayBuffer(16 + records.length * 24)
  const view = new DataView(buf)
  view.setUint32(0, BINARY_MAGIC, false)
  view.setUint16(4, BINARY_VERSION, true)
  view.setUint16(6, records.length, true)
  view.setBigUint64(8, BigInt(Math.floor(timestampMs)), true)

  for (let i = 0; i < records.length; i++) {
    const off = 16 + i * 24, r = records[i]
    view.setUint32(off + 0, r.nodeId, true)
    view.setUint8(off + 4, r.status)
    view.setUint8(off + 5, r.flags ?? 0)
    view.setUint8(off + 6, r.cpuPercent)
    view.setUint8(off + 7, r.memPercent)
    view.setUint8(off + 8, r.diskPercent ?? 0)
    view.setUint8(off + 9, 0)
    view.setUint16(off + 10, r.processes ?? 0, true)
    view.setUint16(off + 12, r.containers ?? 0, true)
    view.setUint16(off + 14, r.latencyMs ?? 0, true)
    view.setFloat32(off + 16, r.networkInKbps ?? 0, true)
    view.setFloat32(off + 20, r.networkOutKbps ?? 0, true)
  }
  return buf
}

export function decodeNodeDeltaBinary(buf: ArrayBuffer): { timestampMs: number; records: BinaryNodeDelta[] } {
  if (buf.byteLength < 16) throw new Error(`Binary buffer too short: ${buf.byteLength}`)
  const view = new DataView(buf)
  const magic = view.getUint32(0, false)
  if (magic !== BINARY_MAGIC) throw new Error(`Invalid binary magic header: 0x${magic.toString(16)}`)
  const count = view.getUint16(6, true)
  const expectedBytes = 16 + count * 24
  if (buf.byteLength !== expectedBytes) throw new Error(`Truncated buffer: expected ${expectedBytes}, got ${buf.byteLength}`)

  const timestampMs = Number(view.getBigUint64(8, true))
  const records: BinaryNodeDelta[] = new Array(count)
  for (let i = 0; i < count; i++) {
    const off = 16 + i * 24
    records[i] = {
      nodeId: view.getUint32(off + 0, true),
      status: view.getUint8(off + 4),
      flags: view.getUint8(off + 5),
      cpuPercent: view.getUint8(off + 6),
      memPercent: view.getUint8(off + 7),
      diskPercent: view.getUint8(off + 8),
      processes: view.getUint16(off + 10, true),
      containers: view.getUint16(off + 12, true),
      latencyMs: view.getUint16(off + 14, true),
      networkInKbps: view.getFloat32(off + 16, true),
      networkOutKbps: view.getFloat32(off + 20, true)
    }
  }
  return { timestampMs, records }
}

export interface VirtualWindowParams {
  totalCount: number; columns?: number; rowHeight?: number
  viewportHeight?: number; scrollTop?: number; overscanRows?: number
}

export function calculateVirtualNodeWindow(params: VirtualWindowParams) {
  const { totalCount, columns = 3, rowHeight = 180, viewportHeight = 720, scrollTop = 0, overscanRows = 3 } = params
  const cols = Math.max(1, columns)
  const totalRows = Math.ceil(totalCount / cols)
  const currentTopRow = Math.floor(Math.max(0, scrollTop) / rowHeight)
  const startRow = Math.max(0, currentTopRow - overscanRows)
  const visibleRowCount = Math.ceil(viewportHeight / rowHeight)
  const endRow = Math.min(totalRows, currentTopRow + visibleRowCount + overscanRows)
  const startIndex = startRow * cols
  const endIndex = Math.min(totalCount, endRow * cols)
  return {
    totalRows, totalHeightPx: totalRows * rowHeight, startRow, endRow,
    startIndex, endIndex, visibleCount: Math.max(0, endIndex - startIndex),
    offsetY: startRow * rowHeight
  }
}

export function computeMatrixLayout(nodeCount: number, containerWidth: number, cellSize = 10, gap = 2) {
  const pitch = cellSize + gap
  const columns = Math.max(1, Math.floor((containerWidth + gap) / pitch))
  const rows = Math.ceil(nodeCount / columns)
  return {
    columns, rows, pitch,
    totalWidth: Math.max(0, columns * pitch - gap),
    totalHeight: Math.max(0, rows * pitch - gap)
  }
}
