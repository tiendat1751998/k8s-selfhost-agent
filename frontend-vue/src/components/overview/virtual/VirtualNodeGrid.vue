<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { calculateVirtualNodeWindow } from '../../../utils/quadtreeIndex'

interface Props {
  nodes?: any[]
  rowHeight?: number
  columns?: number
  viewportHeight?: number
  overscanRows?: number
}

const props = withDefaults(defineProps<Props>(), {
  nodes: () => [],
  rowHeight: 180,
  columns: 3,
  viewportHeight: 720,
  overscanRows: 3,
})

const emit = defineEmits<{
  (e: 'select', node: any): void
  (e: 'scroll', event: UIEvent): void
}>()

const containerRef = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const measuredHeight = ref(props.viewportHeight)
let resizeObserver: ResizeObserver | null = null

const windowState = computed(() => {
  return calculateVirtualNodeWindow({
    totalCount: props.nodes.length,
    columns: props.columns,
    rowHeight: props.rowHeight,
    viewportHeight: measuredHeight.value,
    scrollTop: scrollTop.value,
    overscanRows: props.overscanRows,
  })
})

const visibleNodes = computed(() => {
  return props.nodes.slice(windowState.value.startIndex, windowState.value.endIndex)
})

function onScroll(e: Event) {
  const el = e.target as HTMLElement
  if (el) {
    scrollTop.value = el.scrollTop
    emit('scroll', e as UIEvent)
  }
}

onMounted(() => {
  if (containerRef.value && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.height > 0) {
          measuredHeight.value = entry.contentRect.height
        }
      }
    })
    resizeObserver.observe(containerRef.value)
  }
})

onUnmounted(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
})
</script>

<template>
  <div
    ref="containerRef"
    class="virtual-node-viewport"
    :style="{ height: `${viewportHeight}px` }"
    tabindex="0"
    role="region"
    aria-label="Virtualized fleet nodes list"
    @scroll.passive="onScroll"
  >
    <div
      class="virtual-phantom-spacer"
      :style="{ height: `${windowState.totalHeightPx}px` }"
    >
      <div
        class="virtual-card-track"
        :style="{ transform: `translate3d(0, ${windowState.offsetY}px, 0)` }"
      >
        <div
          class="virtual-grid"
          :style="{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }"
        >
          <div
            v-for="(node, idx) in visibleNodes"
            :key="node.id || node.node_id || (windowState.startIndex + idx)"
            class="virtual-grid-item"
          >
            <slot name="item" :node="node" :index="windowState.startIndex + idx">
              <div
                class="default-virtual-card"
                role="button"
                tabindex="0"
                @click="emit('select', node)"
                @keydown.enter="emit('select', node)"
              >
                <div class="card-top">
                  <span class="status-indicator" :class="node.status || 'healthy'"></span>
                  <span class="node-title" :title="node.name || node.node_name">{{ node.name || node.node_name || `Node #${windowState.startIndex + idx + 1}` }}</span>
                  <span class="node-badge">{{ node.role || 'WORKER' }}</span>
                </div>
                <div class="metrics-row">
                  <div class="metric">
                    <span class="metric-label">CPU</span>
                    <span class="metric-val tabular-nums">{{ node.cpuPct ?? node.cpu_percent ?? 0 }}%</span>
                  </div>
                  <div class="metric">
                    <span class="metric-label">MEM</span>
                    <span class="metric-val tabular-nums">{{ node.memPct ?? node.memory_percent ?? 0 }}%</span>
                  </div>
                </div>
              </div>
            </slot>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.virtual-node-viewport {
  position: relative;
  overflow-y: auto;
  overflow-x: hidden;
  width: 100%;
  border-radius: 8px;
  contain: strict;
  outline: none;
}
.virtual-node-viewport:focus-visible {
  box-shadow: 0 0 0 2px var(--color-primary, #38bdf8);
}
.virtual-phantom-spacer {
  position: relative;
  width: 100%;
  pointer-events: none;
}
.virtual-card-track {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  pointer-events: auto;
  will-change: transform;
}
.virtual-grid {
  display: grid;
  gap: 16px;
  padding: 8px 4px;
}
.virtual-grid-item {
  min-width: 0;
}
.default-virtual-card {
  background: rgba(15, 23, 42, 0.7);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 14px;
  cursor: pointer;
  transition: border-color 0.15s ease, transform 0.15s ease;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.default-virtual-card:hover {
  border-color: rgba(56, 189, 248, 0.4);
  transform: translateY(-1px);
}
.card-top {
  display: flex;
  align-items: center;
  gap: 8px;
}
.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 9999px;
  background: #10b981;
}
.status-indicator.warning { background: #f59e0b; }
.status-indicator.critical, .status-indicator.offline, .status-indicator.down { background: #f43f5e; }
.status-indicator.cordoned { background: #64748b; }
.node-title {
  font-weight: 600;
  font-size: 0.875rem;
  color: #f1f5f9;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}
.node-badge {
  font-size: 0.6875rem;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.06);
  color: #94a3b8;
  letter-spacing: 0.05em;
}
.metrics-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.metric {
  display: flex;
  justify-content: space-between;
  background: rgba(0, 0, 0, 0.25);
  padding: 6px 8px;
  border-radius: 4px;
}
.metric-label {
  font-size: 0.6875rem;
  color: #94a3b8;
  font-weight: 600;
}
.metric-val {
  font-size: 0.75rem;
  font-weight: 700;
  color: #e2e8f0;
}
.tabular-nums {
  font-variant-numeric: tabular-nums;
}
</style>
