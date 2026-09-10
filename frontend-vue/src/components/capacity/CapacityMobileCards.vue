<script setup lang="ts">
import type { NodeHeadroom } from '../../composables/useCapacityForecast'
import { getUsageColorBg, getUsageColorText } from '../../composables/useCapacityForecast'

defineProps<{
  nodes: NodeHeadroom[]
}>()

const emit = defineEmits<{
  (e: 'rebalance', nodeId: string): void
  (e: 'inspect', nodeId: string): void
}>()
</script>

<template>
  <div class="mobile-stream-container">
    <!-- Empty State -->
    <div v-if="nodes.length === 0" class="empty-state-box font-mono">
      <span class="empty-icon">📈</span>
      <p class="empty-desc">No cluster nodes reporting headroom telemetry.</p>
    </div>

    <!-- High-Density Node Rows -->
    <div
      v-for="node in nodes"
      :key="node.id"
      class="mobile-card-row glass-panel"
    >
      <div class="mobile-card-main">
        <div class="mobile-card-header">
          <span
            class="pulse-dot"
            :class="node.status === 'healthy' ? 'pulse-dot-emerald' : node.status === 'warning' ? 'pulse-dot-amber' : 'pulse-dot-rose'"
          ></span>
          <span class="mobile-card-title font-mono" :title="node.name">{{ node.name }}</span>
          <span class="badge font-mono" :class="node.role === 'control-plane' ? 'badge-violet' : 'badge-cyan'" style="padding: 1px 5px; font-size: 9px;">
            {{ node.role === 'control-plane' ? 'CP' : 'WRK' }}
          </span>
        </div>

        <div class="mobile-card-gauges font-mono">
          <div class="mobile-gauge-tiny">
            <div class="mobile-gauge-info">
              <span class="text-muted">CPU:</span>
              <span :class="getUsageColorText(node.cpuUsagePercent)">{{ node.cpuUsagePercent.toFixed(0) }}%</span>
            </div>
            <div class="gauge-bar-bg" style="height: 4px;">
              <div
                class="gauge-bar-fill"
                :class="getUsageColorBg(node.cpuUsagePercent)"
                :style="{ width: `${Math.min(100, node.cpuUsagePercent)}%` }"
              ></div>
            </div>
          </div>

          <div class="mobile-gauge-tiny">
            <div class="mobile-gauge-info">
              <span class="text-muted">RAM:</span>
              <span :class="getUsageColorText(node.memUsagePercent)">{{ node.memUsagePercent.toFixed(0) }}%</span>
            </div>
            <div class="gauge-bar-bg" style="height: 4px;">
              <div
                class="gauge-bar-fill"
                :class="getUsageColorBg(node.memUsagePercent)"
                :style="{ width: `${Math.min(100, node.memUsagePercent)}%` }"
              ></div>
            </div>
          </div>
        </div>
      </div>

      <div class="mobile-card-actions">
        <button
          type="button"
          class="btn-table-action"
          title="Rebalance"
          aria-label="Rebalance node"
          @click="emit('rebalance', node.id)"
        >
          ⚡
        </button>
        <button
          type="button"
          class="btn-table-action"
          title="Inspect"
          aria-label="Inspect node"
          @click="emit('inspect', node.id)"
        >
          🔍
        </button>
      </div>
    </div>
  </div>
</template>
