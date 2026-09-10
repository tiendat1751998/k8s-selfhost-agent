<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import type { NodeHeadroom } from '../../composables/useCapacityForecast'
import { getUsageColorBg, getUsageColorText } from '../../composables/useCapacityForecast'

defineProps<{
  nodes: NodeHeadroom[]
}>()

const emit = defineEmits<{
  (e: 'rebalance', nodeId: string): void
  (e: 'inspect', nodeId: string): void
}>()

const nodeActions: ActionItem[] = [
  { id: 'rebalance', label: 'Rebalance Pods', icon: 'zap' },
  { id: 'inspect', label: 'Inspect Telemetry', icon: 'search' },
]

function handleNodeAction(actionId: string, nodeId: string) {
  if (actionId === 'rebalance') emit('rebalance', nodeId)
  else if (actionId === 'inspect') emit('inspect', nodeId)
}
</script>

<template>
  <div class="section-card glass-panel">
    <div class="section-top">
      <div>
        <h2 class="section-title">Node Headroom & Resource Sizing Matrix</h2>
        <p class="section-subtitle">Real-time allocatable compute, memory runway, and bin-packing per cluster node</p>
      </div>
      <span class="badge badge-cyan font-mono">{{ nodes.length }} Active Nodes</span>
    </div>

    <div class="node-table-wrapper">
      <table class="node-matrix-table">
        <colgroup>
          <col style="width: 19%;" />
          <col style="width: 9%;" />
          <col style="width: 16%;" />
          <col style="width: 16%;" />
          <col style="width: 9%;" />
          <col style="width: 8%;" />
          <col style="width: 10%;" />
          <col style="width: 7%;" />
          <col style="width: 6%;" />
        </colgroup>
        <thead>
          <tr>
            <th>Node Identifier</th>
            <th>Role</th>
            <th>CPU Allocation</th>
            <th>Memory Allocation</th>
            <th>Pod Density</th>
            <th>Bin Packing</th>
            <th>Safe Headroom</th>
            <th>Health</th>
            <th style="text-align: right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="node in nodes" :key="node.id">
            <td>
              <div class="node-name-cell">
                <span class="node-name font-mono" :title="node.name">{{ node.name }}</span>
                <span class="node-role-tag font-mono text-muted" :title="node.id">{{ node.id }}</span>
              </div>
            </td>
            <td>
              <span class="badge font-mono" :class="node.role === 'control-plane' ? 'badge-violet' : 'badge-cyan'">
                {{ node.role }}
              </span>
            </td>
            <td>
              <div class="resource-bar-cell">
                <div class="resource-bar-info font-mono">
                  <span :class="getUsageColorText(node.cpuUsagePercent)">{{ node.cpuUsagePercent.toFixed(1) }}%</span>
                  <span class="text-muted">{{ node.cpuAllocatedCores }} / {{ node.cpuTotalCores }} C</span>
                </div>
                <div class="gauge-bar-bg">
                  <div
                    class="gauge-bar-fill"
                    :class="getUsageColorBg(node.cpuUsagePercent)"
                    :style="{ width: `${Math.min(100, node.cpuUsagePercent)}%` }"
                  ></div>
                </div>
              </div>
            </td>
            <td>
              <div class="resource-bar-cell">
                <div class="resource-bar-info font-mono">
                  <span :class="getUsageColorText(node.memUsagePercent)">{{ node.memUsagePercent.toFixed(1) }}%</span>
                  <span class="text-muted">{{ node.memAllocatedGiB }} / {{ node.memTotalGiB }} GiB</span>
                </div>
                <div class="gauge-bar-bg">
                  <div
                    class="gauge-bar-fill"
                    :class="getUsageColorBg(node.memUsagePercent)"
                    :style="{ width: `${Math.min(100, node.memUsagePercent)}%` }"
                  ></div>
                </div>
              </div>
            </td>
            <td class="font-mono">
              <span :class="node.podCount >= node.podCapacity * 0.9 ? 'text-amber font-bold' : ''">
                {{ node.podCount }} / {{ node.podCapacity }}
              </span>
            </td>
            <td class="font-mono text-cyan font-bold">
              {{ node.binPackingScore.toFixed(1) }}%
            </td>
            <td>
              <span
                class="font-mono font-bold"
                :class="node.headroomPercent < 20 ? 'text-rose' : node.headroomPercent < 35 ? 'text-amber' : 'text-emerald'"
              >
                {{ node.headroomPercent.toFixed(1) }}%
              </span>
            </td>
            <td>
              <StatusBadge :status="node.status" :label="node.status.toUpperCase()" size="sm" />
            </td>
            <td style="text-align: right;">
              <ActionDropdown
                size="xs"
                :items="nodeActions"
                @select="(actionId) => handleNodeAction(actionId, node.id)"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
:deep(.node-table-wrapper),
.node-table-wrapper {
  overflow-x: auto !important;
  max-width: 100%;
  width: 100%;
}
.node-matrix-table {
  table-layout: fixed;
  width: 100%;
}
</style>
