<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { DockerNode } from '../../api/compute'

const props = defineProps<{
  nodes: DockerNode[]
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'inspect', node: DockerNode): void
  (e: 'drain', nodeId: string): void
  (e: 'activate', nodeId: string): void
}>()

function formatMemory(mem?: number): string {
  if (!mem || mem <= 0) return '--'
  if (mem > 1024 * 1024 * 1024) {
    return `${(mem / (1024 * 1024 * 1024)).toFixed(1)} GB`
  }
  if (mem > 1024 * 1024) {
    return `${(mem / (1024 * 1024)).toFixed(0)} MB`
  }
  return `${mem} GB`
}
</script>

<template>
  <div class="nodes-rack-view animate-fade-in">
    <div v-if="props.nodes.length === 0" class="empty-state glass-panel">
      <span>No Swarm nodes discovered on the network matching your filter.</span>
    </div>

    <div v-else class="nodes-grid">
      <div
        v-for="node in props.nodes"
        :key="node.id"
        class="node-rack-card glass-panel"
        :class="{ 'node-draining': node.availability === 'drain' }"
      >
        <div class="rack-top">
          <div class="rack-header-left">
            <span class="server-icon" aria-hidden="true">🖳</span>
            <div>
              <h3 class="rack-node-name font-mono">{{ node.name }}</h3>
              <span class="rack-role font-mono" :class="node.role === 'manager' ? 'role-mgr' : 'role-wrk'">
                {{ (node.role || 'worker').toUpperCase() }} NODE
              </span>
            </div>
          </div>
          <StatusBadge :status="node.availability" size="sm" />
        </div>

        <!-- Hardware Telemetry Meters -->
        <div class="rack-meters">
          <div class="meter-item">
            <div class="meter-meta font-mono">
              <span>CPU Core Load</span>
              <span class="text-cyan">
                {{ (node as any).cpu_percent != null ? `${Math.round((node as any).cpu_percent)}%` : '--' }}
                {{ node.cpus ? `(${node.cpus} vCPU)` : '' }}
              </span>
            </div>
            <div class="meter-bar-bg" role="progressbar" :aria-valuenow="(node as any).cpu_percent ?? 0" aria-valuemin="0" aria-valuemax="100">
              <div class="meter-bar-fill fill-cyan" :style="{ width: (node as any).cpu_percent != null ? `${Math.min(100, Math.max(0, (node as any).cpu_percent))}%` : '0%' }"></div>
            </div>
          </div>

          <div class="meter-item">
            <div class="meter-meta font-mono">
              <span>RAM Allocation</span>
              <span class="text-emerald">
                {{ (node as any).memory_percent != null ? `${Math.round((node as any).memory_percent)}%` : '--' }}
                {{ formatMemory(node.memory) !== '--' ? `(${formatMemory(node.memory)})` : '' }}
              </span>
            </div>
            <div class="meter-bar-bg" role="progressbar" :aria-valuenow="(node as any).memory_percent ?? 0" aria-valuemin="0" aria-valuemax="100">
              <div class="meter-bar-fill fill-emerald" :style="{ width: (node as any).memory_percent != null ? `${Math.min(100, Math.max(0, (node as any).memory_percent))}%` : '0%' }"></div>
            </div>
          </div>
        </div>

        <div class="rack-footer">
          <div class="engine-ver font-mono text-muted">
            <span>Engine: {{ (node.engine_version || node.version) ? `Docker v${node.engine_version || node.version}` : 'Docker --' }}</span>
          </div>

          <div class="rack-actions">
            <button
              v-if="node.availability === 'active'"
              class="btn btn-secondary btn-xs"
              :disabled="props.actionLoading === `drain-${node.id}`"
              title="Drain workloads from node"
              @click="emit('drain', node.id)"
            >
              <span>{{ props.actionLoading === `drain-${node.id}` ? '⏳ Draining...' : '⏸ Drain' }}</span>
            </button>
            <button
              v-else-if="node.availability === 'drain'"
              class="btn btn-secondary btn-xs text-emerald"
              :disabled="props.actionLoading === `activate-${node.id}`"
              title="Activate node for scheduling"
              @click="emit('activate', node.id)"
            >
              <span>{{ props.actionLoading === `activate-${node.id}` ? '⏳ Activating...' : '▶ Activate' }}</span>
            </button>
            <button
              class="btn btn-secondary btn-xs"
              title="Inspect node specifications"
              @click="emit('inspect', node)"
            >
              <span>🔍 Inspect</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.node-rack-card {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: rgba(11, 15, 25, 0.65);
}

.node-draining {
  border-color: rgba(245, 158, 11, 0.4);
  background: rgba(245, 158, 11, 0.04);
}

.rack-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.rack-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.server-icon { font-size: 22px; }
.rack-node-name { font-size: 14px; font-weight: 700; color: #fff; }
.rack-role { font-size: 10px; font-weight: 700; }
.role-mgr { color: #c4b5fd; }
.role-wrk { color: var(--text-muted); }

.rack-meters {
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: rgba(0, 0, 0, 0.3);
  padding: 12px;
  border-radius: 10px;
}

.meter-item { display: flex; flex-direction: column; gap: 4px; }
.meter-meta { display: flex; justify-content: space-between; font-size: 11px; }

.meter-bar-bg {
  width: 100%;
  height: 6px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
}

.meter-bar-fill { height: 100%; border-radius: 9999px; }
.fill-cyan { background: var(--grad-cyan); }
.fill-emerald { background: var(--grad-emerald); }

.rack-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}

.rack-actions { display: flex; align-items: center; gap: 6px; }
</style>