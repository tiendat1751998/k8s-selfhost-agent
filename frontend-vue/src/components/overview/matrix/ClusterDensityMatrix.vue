<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { Quadtree, computeMatrixLayout } from '../../../utils/quadtreeIndex'

export interface MatrixNode {
  id: string
  name: string
  status: string
  cpuPct: number
  memPct: number
  region?: string
}

interface Props {
  nodes?: MatrixNode[]
  cellSize?: number
  gap?: number
}

const props = withDefaults(defineProps<Props>(), {
  nodes: () => [],
  cellSize: 10,
  gap: 2,
})

const emit = defineEmits<{
  (e: 'select-node', node: MatrixNode): void
  (e: 'hover-node', node: MatrixNode | null): void
}>()

const containerRef = ref<HTMLDivElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)
const hoveredNode = ref<MatrixNode | null>(null)
const selectedNodeId = ref<string | null>(null)
const tooltipX = ref(0)
const tooltipY = ref(0)

let quadtree: Quadtree<MatrixNode> | null = null
let resizeObserver: ResizeObserver | null = null
let rafHandle: number | null = null

function getNodeColor(node: MatrixNode): string {
  const s = (node.status || '').toLowerCase()
  if (s === 'offline' || s === 'down' || s === 'critical' || s === 'unhealthy') return '#f43f5e'
  if (s === 'cordoned' || s === 'maintenance') return '#64748b'
  const maxSat = Math.max(node.cpuPct || 0, node.memPct || 0)
  if (maxSat >= 85) return '#f43f5e'
  if (maxSat >= 70) return '#f59e0b'
  return '#10b981'
}

function renderMatrix() {
  if (rafHandle) cancelAnimationFrame(rafHandle)
  rafHandle = requestAnimationFrame(() => {
    const canvas = canvasRef.value
    const container = containerRef.value
    if (!canvas || !container) return

    const containerWidth = container.clientWidth || 800
    const layout = computeMatrixLayout(props.nodes.length, containerWidth, props.cellSize, props.gap)
    const dpr = typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1

    canvas.width = Math.max(1, Math.round(layout.totalWidth * dpr))
    canvas.height = Math.max(1, Math.round(layout.totalHeight * dpr))
    canvas.style.width = `${layout.totalWidth}px`
    canvas.style.height = `${layout.totalHeight}px`

    const ctx = canvas.getContext('2d')
    if (!ctx) return
    ctx.scale(dpr, dpr)
    ctx.clearRect(0, 0, layout.totalWidth, layout.totalHeight)

    const qt = new Quadtree<MatrixNode>({ x: 0, y: 0, width: layout.totalWidth || 1, height: layout.totalHeight || 1 }, 8, 8)

    for (let i = 0; i < props.nodes.length; i++) {
      const node = props.nodes[i]
      const col = i % layout.columns
      const row = Math.floor(i / layout.columns)
      const x = col * layout.pitch
      const y = row * layout.pitch

      ctx.fillStyle = getNodeColor(node)
      ctx.beginPath()
      ctx.roundRect ? ctx.roundRect(x, y, props.cellSize, props.cellSize, 2) : ctx.rect(x, y, props.cellSize, props.cellSize)
      ctx.fill()

      if (selectedNodeId.value === node.id || hoveredNode.value?.id === node.id) {
        ctx.strokeStyle = '#38bdf8'
        ctx.lineWidth = 1.5
        ctx.stroke()
      }
      qt.insert({ x: x + props.cellSize / 2, y: y + props.cellSize / 2, data: node })
    }
    quadtree = qt
  })
}

let mouseMoveRafId: number | null = null
let pendingMouseMoveEvent: MouseEvent | null = null

function processMouseMove() {
  mouseMoveRafId = null
  if (!pendingMouseMoveEvent) return
  const event = pendingMouseMoveEvent
  pendingMouseMoveEvent = null

  const canvas = canvasRef.value
  if (!canvas || !quadtree) return
  const rect = canvas.getBoundingClientRect()
  const mouseX = event.clientX - rect.left
  const mouseY = event.clientY - rect.top
  const nearest = quadtree.findNearest(mouseX, mouseY, props.cellSize + props.gap)
  if (nearest) {
    hoveredNode.value = nearest.data
    tooltipX.value = event.clientX + 14
    tooltipY.value = event.clientY + 14
    emit('hover-node', nearest.data)
  } else {
    hoveredNode.value = null
    emit('hover-node', null)
  }
}

function onMouseMove(event: MouseEvent) {
  pendingMouseMoveEvent = event
  if (mouseMoveRafId === null) {
    mouseMoveRafId = requestAnimationFrame(processMouseMove)
  }
}

function onClick(event: MouseEvent) {
  const canvas = canvasRef.value
  if (!canvas || !quadtree) return
  const rect = canvas.getBoundingClientRect()
  const nearest = quadtree.findNearest(event.clientX - rect.left, event.clientY - rect.top, props.cellSize + props.gap)
  if (nearest) {
    selectedNodeId.value = nearest.data.id
    emit('select-node', nearest.data)
    renderMatrix()
  }
}

function onMouseLeave() {
  if (mouseMoveRafId !== null) {
    cancelAnimationFrame(mouseMoveRafId)
    mouseMoveRafId = null
  }
  pendingMouseMoveEvent = null
  hoveredNode.value = null
  emit('hover-node', null)
}

watch(() => props.nodes, () => nextTick(renderMatrix), { deep: false })
watch(() => [props.cellSize, props.gap], () => nextTick(renderMatrix))

onMounted(() => {
  renderMatrix()
  if (containerRef.value && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => renderMatrix())
    resizeObserver.observe(containerRef.value)
  }
})

onUnmounted(() => {
  if (rafHandle) cancelAnimationFrame(rafHandle)
  if (mouseMoveRafId !== null) cancelAnimationFrame(mouseMoveRafId)
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
})
</script>

<template>
  <div ref="containerRef" class="density-matrix-wrapper" role="region" aria-label="Cluster node density matrix map">
    <div class="matrix-canvas-container">
      <canvas ref="canvasRef" class="matrix-canvas" @mousemove="onMouseMove" @mouseleave="onMouseLeave" @click="onClick"></canvas>
    </div>
    <!-- Tooltip -->
    <div v-if="hoveredNode" class="matrix-tooltip glass-panel" :style="{ left: `${tooltipX}px`, top: `${tooltipY}px` }">
      <div class="tooltip-header">
        <span class="status-dot" :class="hoveredNode.status"></span>
        <span class="tooltip-title">{{ hoveredNode.name }}</span>
      </div>
      <div class="tooltip-grid">
        <div class="tooltip-item"><span class="label">CPU</span><span class="val tabular-nums">{{ hoveredNode.cpuPct }}%</span></div>
        <div class="tooltip-item"><span class="label">MEM</span><span class="val tabular-nums">{{ hoveredNode.memPct }}%</span></div>
        <div v-if="hoveredNode.region" class="tooltip-item full"><span class="label">REGION</span><span class="val tabular-nums font-mono">{{ hoveredNode.region }}</span></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.density-matrix-wrapper {
  position: relative;
  width: 100%;
  padding: 12px;
  background: var(--color-surface-inset, rgba(15, 23, 42, 0.4));
  border: 1px solid var(--color-border-subtle, rgba(255, 255, 255, 0.06));
  border-radius: 8px;
  box-sizing: border-box;
}
.matrix-canvas-container {
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
}
.matrix-canvas { display: block; cursor: crosshair; }
.matrix-tooltip {
  position: fixed;
  z-index: 9999;
  pointer-events: none;
  background: var(--color-surface-card, rgba(15, 23, 42, 0.95));
  border: 1px solid rgba(56, 189, 248, 0.3);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
  border-radius: 6px;
  padding: 8px 12px;
  min-width: 160px;
  font-size: 0.75rem;
}
.tooltip-header { display: flex; align-items: center; gap: 6px; margin-bottom: 6px; font-weight: 600; color: var(--color-heading, #f8fafc); }
.status-dot { width: 6px; height: 6px; border-radius: 9999px; background: #10b981; }
.status-dot.offline, .status-dot.down, .status-dot.critical { background: #f43f5e; }
.status-dot.warning { background: #f59e0b; }
.status-dot.cordoned { background: #64748b; }
.tooltip-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 4px 8px; }
.tooltip-item { display: flex; justify-content: space-between; }
.tooltip-item.full { grid-column: span 2; }
.label { color: var(--color-muted, #94a3b8); font-size: 0.6875rem; font-weight: 600; }
.val { color: var(--color-body, #e2e8f0); font-weight: 700; }
.tabular-nums { font-variant-numeric: tabular-nums; }
</style>
