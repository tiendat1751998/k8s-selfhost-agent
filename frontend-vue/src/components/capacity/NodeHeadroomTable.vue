<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import PercentageBar from '../ui/PercentageBar.vue'
import CanvasSparkline from '../telemetry/CanvasSparkline.vue'
import type { NodeHeadroom } from '../../composables/useCapacityForecast'

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

function getRiskTextColor(pct: number): string {
  if (pct >= 85) return 'text-rose'
  if (pct >= 70) return 'text-amber'
  return 'text-cyan'
}

function getSparklineColor(pct: number): string {
  if (pct >= 85) return '#f43f5e'
  if (pct >= 70) return '#f59e0b'
  return '#06b6d4'
}

function getNodeTrajectory(node: NodeHeadroom): number[] {
  if ('trajectory' in node && Array.isArray((node as Record<string, unknown>).trajectory)) {
    return (node as Record<string, unknown>).trajectory as number[]
  }
  const base = node.cpuUsagePercent
  const mem = node.memUsagePercent
  const diff = (base - mem) / 4
  return [
    Math.max(0, Math.round((base - diff * 2) * 10) / 10),
    Math.max(0, Math.round((base - diff) * 10) / 10),
    Math.max(0, Math.round(((base + mem) / 2) * 10) / 10),
    Math.max(0, Math.round((base + diff * 0.5) * 10) / 10),
    Math.max(0, Math.round((base - diff * 0.5) * 10) / 10),
    base,
  ]
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
          <col style="width: 18%;" />
          <col style="width: 9%;" />
          <col style="width: 25%;" />
          <col style="width: 13%;" />
          <col style="width: 13%;" />
          <col style="width: 14%;" />
          <col style="width: 8%;" />
        </colgroup>
        <thead>
          <tr>
            <th>Node Identifier</th>
            <th>Role</th>
            <th>Compute & Memory</th>
            <th>Load Trajectory</th>
            <th>Pods & Density</th>
            <th>Safe Headroom</th>
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
              <div class="dual-resource-cell">
                <div class="resource-row">
                  <div class="resource-row-info font-mono">
                    <span class="res-tag text-cyan font-bold">CPU</span>
                    <span :class="getRiskTextColor(node.cpuUsagePercent)" class="font-bold">{{ node.cpuUsagePercent.toFixed(1) }}%</span>
                    <span class="text-muted text-xs">({{ node.cpuAllocatedCores }}/{{ node.cpuTotalCores }}C)</span>
                  </div>
                  <PercentageBar :percentage="node.cpuUsagePercent" :height="4" />
                </div>
                <div class="resource-row">
                  <div class="resource-row-info font-mono">
                    <span class="res-tag text-amber font-bold">MEM</span>
                    <span :class="getRiskTextColor(node.memUsagePercent)" class="font-bold">{{ node.memUsagePercent.toFixed(1) }}%</span>
                    <span class="text-muted text-xs">({{ node.memAllocatedGiB }}/{{ node.memTotalGiB }}G)</span>
                  </div>
                  <PercentageBar :percentage="node.memUsagePercent" :height="4" />
                </div>
              </div>
            </td>
            <td>
              <div class="trajectory-sparkline-cell" title="Allocation trajectory over time">
                <CanvasSparkline
                  :data="getNodeTrajectory(node)"
                  :color="getSparklineColor(node.cpuUsagePercent)"
                  :height="24"
                />
              </div>
            </td>
            <td>
              <div class="pod-density-cell font-mono">
                <div :class="node.podCount >= node.podCapacity * 0.9 ? 'text-amber font-bold' : ''">
                  {{ node.podCount }}/{{ node.podCapacity }} pods
                </div>
                <div class="text-xs text-muted">
                  Packing: <span class="text-cyan font-bold">{{ node.binPackingScore.toFixed(1) }}%</span>
                </div>
              </div>
            </td>
            <td>
              <div class="headroom-health-cell">
                <span
                  class="font-mono font-bold"
                  :class="node.headroomPercent < 20 ? 'text-rose' : node.headroomPercent < 35 ? 'text-amber' : 'text-emerald'"
                >
                  {{ node.headroomPercent.toFixed(1) }}%
                </span>
                <StatusBadge :status="node.status" :label="node.status.toUpperCase()" size="sm" />
              </div>
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
  overflow-x: auto;
  max-width: 100%;
  width: 100%;
}
.node-matrix-table {
  table-layout: fixed;
  width: 100%;
}
.dual-resource-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.resource-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.resource-row-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
}
.res-tag {
  font-size: 10px;
  letter-spacing: 0.02em;
}
.trajectory-sparkline-cell {
  width: 100%;
  min-width: 60px;
  max-width: 120px;
  padding: 2px 0;
}
.pod-density-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 11px;
}
.headroom-health-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
</style>

