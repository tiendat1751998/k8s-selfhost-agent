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
  (e: 'drain', nodeId: string): void
  (e: 'action', action: string, node: NodeHeadroom): void
}>()

const nodeActions: ActionItem[] = [
  { action: 'rebalance', label: 'Rebalance Pods', icon: 'zap' },
  { action: 'inspect', label: 'Inspect Allocations', icon: 'search' },
  { action: 'drain', label: 'Drain Node', icon: 'trash-2', isDestructive: true, variant: 'danger' },
]

function handleAction(actionId: string, node: NodeHeadroom) {
  if (actionId === 'rebalance') {
    emit('rebalance', node.id)
  } else if (actionId === 'inspect') {
    emit('inspect', node.id)
  } else if (actionId === 'drain') {
    emit('drain', node.id)
  }
  emit('action', actionId, node)
}

function getAllocationColor(pct: number): string {
  if (pct >= 85) return '#f43f5e'
  if (pct >= 70) return '#f59e0b'
  return '#06b6d4'
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

function getHeadroomTextColor(pct: number): string {
  if (pct < 15) return 'text-rose'
  if (pct < 30) return 'text-amber'
  return 'text-cyan'
}

function getNodeHealthStatus(node: NodeHeadroom): string {
  if (node.status === 'cordoned') return 'cordoned'
  if (node.status === 'critical' || node.status === 'failed') return 'critical'
  if (node.cpuUsagePercent >= 85 || node.memUsagePercent >= 85) return 'warning'
  if (node.status === 'warning' || node.cpuUsagePercent >= 70 || node.memUsagePercent >= 70) return 'warning'
  return 'ready'
}

function getNodeHealthLabel(node: NodeHeadroom): string {
  if (node.status === 'cordoned') return 'Cordoned'
  if (node.status === 'critical' || node.status === 'failed') return 'Critical'
  if (node.cpuUsagePercent >= 85 || node.memUsagePercent >= 85) return 'High Commitment'
  if (node.status === 'warning') return 'Warning'
  return 'Ready'
}

function getNodeTrajectory(node: NodeHeadroom): number[] {
  if ('trajectory' in node && Array.isArray((node as Record<string, unknown>).trajectory) && ((node as Record<string, unknown>).trajectory as number[]).length > 0) {
    return (node as Record<string, unknown>).trajectory as number[]
  }
  return [node.cpuUsagePercent, node.cpuUsagePercent]
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
          <col style="width: 12%;" />
          <col style="width: 12%;" />
          <col style="width: 11%;" />
          <col style="width: 13%;" />
          <col style="width: 40px;" />
        </colgroup>
        <thead>
          <tr>
            <th>Node Identifier</th>
            <th>Role</th>
            <th>Compute & Memory</th>
            <th>Load Trajectory</th>
            <th>Pods & Density</th>
            <th>Safe Headroom</th>
            <th>Health</th>
            <th style="width: 40px; text-align: center;" aria-label="Actions"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="nodes.length === 0">
            <td colspan="8" class="empty-state-cell font-mono text-center text-muted" style="padding: 32px 16px; text-align: center;">
              No cluster nodes reporting headroom telemetry.
            </td>
          </tr>
          <tr v-for="node in nodes" v-else :key="node.id">
            <td>
              <div class="node-name-cell">
                <span class="node-name font-mono font-medium" :title="node.name">{{ node.name }}</span>
                <span class="node-role-tag font-mono text-muted text-xs" :title="node.id">{{ node.id }}</span>
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
                    <span class="res-tag text-muted font-bold">CPU</span>
                    <span :class="getRiskTextColor(node.cpuUsagePercent)" class="font-bold">{{ node.cpuUsagePercent.toFixed(1) }}%</span>
                    <span class="text-muted text-xs">({{ node.cpuAllocatedCores }}/{{ node.cpuTotalCores }} cores)</span>
                  </div>
                  <PercentageBar :percentage="node.cpuUsagePercent" :height="5" :variant="getAllocationColor(node.cpuUsagePercent)" />
                </div>
                <div class="resource-row">
                  <div class="resource-row-info font-mono">
                    <span class="res-tag text-muted font-bold">MEM</span>
                    <span :class="getRiskTextColor(node.memUsagePercent)" class="font-bold">{{ node.memUsagePercent.toFixed(1) }}%</span>
                    <span class="text-muted text-xs">({{ node.memAllocatedGiB }}/{{ node.memTotalGiB }} GiB)</span>
                  </div>
                  <PercentageBar :percentage="node.memUsagePercent" :height="5" :variant="getAllocationColor(node.memUsagePercent)" />
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
              <div class="headroom-cell font-mono">
                <span
                  class="font-bold"
                  :class="getHeadroomTextColor(node.headroomPercent)"
                >
                  {{ node.headroomPercent.toFixed(1) }}%
                </span>
              </div>
            </td>
            <td>
              <div class="health-cell">
                <StatusBadge
                  :status="getNodeHealthStatus(node)"
                  :label="getNodeHealthLabel(node)"
                  size="sm"
                />
              </div>
            </td>
            <td style="width: 40px; text-align: center;">
              <ActionDropdown
                size="xs"
                :items="nodeActions"
                aria-label="Node actions"
                @select="handleAction($event, node)"
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
.node-name-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.node-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
.headroom-cell {
  display: flex;
  align-items: center;
}
.health-cell {
  display: flex;
  align-items: center;
}
</style>
