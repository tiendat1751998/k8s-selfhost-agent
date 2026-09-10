<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import type { NodeHeadroom } from '../../composables/useCapacityForecast'
import { getUsageColorBg, getUsageColorText } from '../../composables/useCapacityForecast'
import StatusBadge from '../ui/StatusBadge.vue'

const props = defineProps<{
  node: NodeHeadroom | null
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'rebalance', nodeId: string): void
}>()

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.open) {
    emit('close')
  }
}

onMounted(() => window.addEventListener('keydown', handleKeydown))
onUnmounted(() => window.removeEventListener('keydown', handleKeydown))

const cpuAvailCores = computed(() => {
  if (!props.node) return 0
  return Math.max(0, Number((props.node.cpuTotalCores - props.node.cpuAllocatedCores).toFixed(1)))
})

const memAvailGiB = computed(() => {
  if (!props.node) return 0
  return Math.max(0, Number((props.node.memTotalGiB - props.node.memAllocatedGiB).toFixed(1)))
})

const podUsagePercent = computed(() => {
  if (!props.node || props.node.podCapacity === 0) return 0
  return Math.min(100, Math.round((props.node.podCount / props.node.podCapacity) * 100))
})

const recommendation = computed(() => {
  if (!props.node) return { type: 'normal', title: 'Optimal Headroom', desc: 'Resource headroom is safe.' }
  if (props.node.headroomPercent < 20 || props.node.status === 'critical') {
    return {
      type: 'critical',
      title: 'Action Recommended: Evacuate Non-Critical Pods',
      desc: `Headroom buffer is critically compressed at ${props.node.headroomPercent.toFixed(1)}%. Rebalance or expand pool immediately.`,
    }
  }
  if (props.node.headroomPercent < 35 || props.node.status === 'warning') {
    return {
      type: 'warning',
      title: 'Elevated Saturation: Monitor Headroom',
      desc: `Headroom is narrowed to ${props.node.headroomPercent.toFixed(1)}%. Bin-packing density is high (${props.node.binPackingScore.toFixed(1)}%).`,
    }
  }
  return {
    type: 'normal',
    title: 'Optimal Headroom: Stable Capacity',
    desc: `Safe burst headroom at ${props.node.headroomPercent.toFixed(1)}% with ${cpuAvailCores.value} cores and ${memAvailGiB.value} GiB free buffer.`,
  }
})
</script>

<template>
  <div v-if="open && node" class="capacity-drawer-backdrop" @click.self="emit('close')">
    <div class="capacity-drawer-panel glass-panel" role="dialog" aria-modal="true" :aria-label="`Telemetry for ${node.name}`">
      <!-- Drawer Header -->
      <div class="capacity-drawer-header">
        <div class="capacity-drawer-title-group">
          <div style="display: flex; align-items: center; gap: 8px;">
            <span class="badge font-mono" :class="node.role === 'control-plane' ? 'badge-violet' : 'badge-cyan'">
              {{ node.role.toUpperCase() }}
            </span>
            <span class="capacity-drawer-title font-mono">{{ node.name }}</span>
          </div>
          <span class="capacity-drawer-subtitle font-mono">Node ID: {{ node.id }}</span>
        </div>
        <button type="button" class="capacity-drawer-close" aria-label="Close inspection drawer" @click="emit('close')">✕</button>
      </div>

      <!-- Drawer Body -->
      <div class="capacity-drawer-body">
        <!-- Hero Node Status Row -->
        <div class="drawer-node-hero">
          <div class="drawer-hero-badges">
            <StatusBadge :status="node.status" :label="node.status.toUpperCase()" size="sm" />
            <span class="hero-metric-pill font-mono bg-cyan text-white">
              Headroom: {{ node.headroomPercent.toFixed(1) }}%
            </span>
          </div>
          <span class="font-mono" style="font-size: 11px; color: #94a3b8;">Bin-Pack: {{ node.binPackingScore.toFixed(1) }}%</span>
        </div>

        <!-- CPU Allocation Gauge -->
        <div class="drawer-metric-card">
          <div class="drawer-metric-header font-mono">
            <span class="drawer-metric-name">⚡ Compute Allocation</span>
            <span class="drawer-metric-stat" :class="getUsageColorText(node.cpuUsagePercent)">
              {{ node.cpuUsagePercent.toFixed(1) }}%
            </span>
          </div>
          <div class="gauge-bar-bg" style="height: 6px;">
            <div
              class="gauge-bar-fill"
              :class="getUsageColorBg(node.cpuUsagePercent)"
              :style="{ width: `${Math.min(100, node.cpuUsagePercent)}%` }"
            ></div>
          </div>
          <div class="drawer-metric-subtext font-mono">
            <span>Allocated: {{ node.cpuAllocatedCores }} / {{ node.cpuTotalCores }} Cores</span>
            <span class="text-cyan">{{ cpuAvailCores }} Cores Free</span>
          </div>
        </div>

        <!-- Memory Allocation Gauge -->
        <div class="drawer-metric-card">
          <div class="drawer-metric-header font-mono">
            <span class="drawer-metric-name">🧠 Memory Allocation</span>
            <span class="drawer-metric-stat" :class="getUsageColorText(node.memUsagePercent)">
              {{ node.memUsagePercent.toFixed(1) }}%
            </span>
          </div>
          <div class="gauge-bar-bg" style="height: 6px;">
            <div
              class="gauge-bar-fill"
              :class="getUsageColorBg(node.memUsagePercent)"
              :style="{ width: `${Math.min(100, node.memUsagePercent)}%` }"
            ></div>
          </div>
          <div class="drawer-metric-subtext font-mono">
            <span>Allocated: {{ node.memAllocatedGiB }} / {{ node.memTotalGiB }} GiB</span>
            <span class="text-emerald">{{ memAvailGiB }} GiB Free</span>
          </div>
        </div>

        <!-- Pod Density & Bin-Packing -->
        <div class="drawer-metric-card">
          <div class="drawer-metric-header font-mono">
            <span class="drawer-metric-name">📦 Pod Density & Bin-Packing</span>
            <span class="drawer-metric-stat" :class="podUsagePercent >= 90 ? 'text-rose' : 'text-cyan'">
              {{ node.podCount }} / {{ node.podCapacity }} Pods ({{ podUsagePercent }}%)
            </span>
          </div>
          <div class="gauge-bar-bg" style="height: 6px;">
            <div
              class="gauge-bar-fill"
              :class="podUsagePercent >= 90 ? 'bg-rose' : 'bg-cyan'"
              :style="{ width: `${podUsagePercent}%` }"
            ></div>
          </div>
          <div class="drawer-metric-subtext font-mono">
            <span>Bin-Packing Score: {{ node.binPackingScore.toFixed(1) }}%</span>
            <span>Target: 85%</span>
          </div>
        </div>

        <!-- Safe Headroom Recommendation Card -->
        <div class="drawer-rec-card" :class="`rec-${recommendation.type}`">
          <div class="drawer-rec-title">
            <span>{{ recommendation.type === 'critical' ? '🚨' : recommendation.type === 'warning' ? '⚡' : '🛡️' }}</span>
            <span>{{ recommendation.title }}</span>
          </div>
          <p class="drawer-rec-desc">{{ recommendation.desc }}</p>
        </div>
      </div>

      <!-- Drawer Footer -->
      <div class="capacity-drawer-footer">
        <button type="button" class="btn btn-secondary" @click="emit('close')">Close</button>
        <button type="button" class="btn btn-primary" @click="emit('rebalance', node.id); emit('close')">
          <span>⚡ Rebalance Node</span>
        </button>
      </div>
    </div>
  </div>
</template>
