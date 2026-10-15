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
  columns: 0,
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
const containerWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1200)
let resizeObserver: ResizeObserver | null = null

const effectiveColumns = computed(() => {
  const w = containerWidth.value
  if (w < 640) return 1
  if (w < 1024) return Math.min(props.columns || 2, 2)
  return props.columns || 3
})

const windowState = computed(() => {
  return calculateVirtualNodeWindow({
    totalCount: props.nodes.length,
    columns: effectiveColumns.value,
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

function updateDimensions() {
  if (containerRef.value) {
    if (containerRef.value.clientWidth > 0) {
      containerWidth.value = containerRef.value.clientWidth
    }
    if (containerRef.value.clientHeight > 0) {
      measuredHeight.value = containerRef.value.clientHeight
    }
  } else if (typeof window !== 'undefined') {
    containerWidth.value = window.innerWidth
  }
}

onMounted(() => {
  updateDimensions()
  if (typeof window !== 'undefined') {
    window.addEventListener('resize', updateDimensions, { passive: true })
  }
  if (containerRef.value && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.height > 0) {
          measuredHeight.value = entry.contentRect.height
        }
        if (entry.contentRect.width > 0) {
          containerWidth.value = entry.contentRect.width
        }
      }
    })
    resizeObserver.observe(containerRef.value)
  }
})

onUnmounted(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('resize', updateDimensions)
  }
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
          :style="{ gridTemplateColumns: `repeat(${effectiveColumns}, minmax(0, 1fr))` }"
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

<style scoped src="../../../assets/styles/components/virtual-node-grid.css"></style>
