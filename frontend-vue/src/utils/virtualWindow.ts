export interface VirtualWindowOptions {
  totalCount: number
  scrollTop: number
  clientHeight: number
  itemHeight: number
  overscan?: number
}

export interface VirtualWindowResult {
  startIndex: number
  endIndex: number
  offsetY: number
  totalHeight: number
  visibleCount: number
}

export function calculateVirtualWindow(opts: VirtualWindowOptions): VirtualWindowResult {
  const { totalCount, scrollTop, clientHeight, itemHeight } = opts
  const overscan = opts.overscan ?? 10

  if (totalCount <= 0 || itemHeight <= 0) {
    return {
      startIndex: 0,
      endIndex: 0,
      offsetY: 0,
      totalHeight: 0,
      visibleCount: 0,
    }
  }

  const rawStart = Math.floor(Math.max(0, scrollTop) / itemHeight)
  const rawEnd = Math.ceil((Math.max(0, scrollTop) + Math.max(0, clientHeight)) / itemHeight)

  const startIndex = Math.max(0, rawStart - overscan)
  const endIndex = Math.min(totalCount, rawEnd + overscan)
  const offsetY = startIndex * itemHeight
  const totalHeight = totalCount * itemHeight
  const visibleCount = Math.max(0, endIndex - startIndex)

  return {
    startIndex,
    endIndex,
    offsetY,
    totalHeight,
    visibleCount,
  }
}