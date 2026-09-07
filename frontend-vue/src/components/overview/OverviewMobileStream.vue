<script setup lang="ts">
import { useRouter } from 'vue-router'
import type { SystemOverview, NodeMetrics } from '../../api/overview'

interface Props {
  overview: SystemOverview
  nodes: NodeMetrics[]
  runningContainers: number
  totalContainers: number
  effectiveHttpRps: number
  isLiveWs: boolean
  lastUpdated: Date
  loading: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect', node: NodeMetrics): void
  (e: 'refresh'): void
  (e: 'deepDive'): void
}>()

const router = useRouter()

function formatRps(val: number): string {
  if (!val || isNaN(val)) return '0'
  if (val >= 1000) return `${(val / 1000).toFixed(1)}k`
  return Math.round(val).toString()
}
</script>

<template>
  <div class="mobile-overview-stream">
    <!-- 1. Mobile Quick Status Header -->
    <div class="mobile-stream-header glass-panel">
      <div class="mobile-sync-info">
        <span class="badge" :class="isLiveWs ? 'badge-emerald' : 'badge-cyan'">
          <span class="pulse-dot" :class="{ 'pulse-active': isLiveWs }"></span>
          <span>{{ isLiveWs ? 'LIVE WS' : 'SYNC' }}</span>
        </span>
        <span class="last-sync-text">{{ lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) }}</span>
      </div>
      <div class="mobile-quick-actions">
        <button class="btn-m-action" @click="emit('refresh')" :disabled="loading">
          <span :class="{ 'spin-icon': loading }">🔄</span>
        </button>
        <button class="btn-m-action" @click="router.push('/deployments')">🚀 Apps</button>
        <button class="btn-m-action" @click="router.push('/hosts')">🖥️ Hosts</button>
        <button class="btn-m-action" @click="router.push('/fleet')">☸️ Fleet</button>
      </div>
    </div>

    <!-- 2. Compact Touch Telemetry Grid -->
    <div class="mobile-kpi-grid">
      <div class="mobile-kpi-card glass-panel">
        <span class="m-kpi-lbl">Nodes</span>
        <span class="m-kpi-val" :class="overview.healthy_nodes === overview.total_nodes ? 'text-emerald' : 'text-amber'">
          {{ overview.healthy_nodes }}/{{ overview.total_nodes }}
        </span>
        <span class="m-kpi-sub">{{ overview.healthy_nodes === overview.total_nodes ? 'Healthy' : 'Degraded' }}</span>
      </div>

      <div class="mobile-kpi-card glass-panel">
        <span class="m-kpi-lbl">Containers</span>
        <span class="m-kpi-val text-cyan">{{ runningContainers }}/{{ totalContainers }}</span>
        <span class="m-kpi-sub">Active Pods</span>
      </div>

      <div class="mobile-kpi-card glass-panel">
        <span class="m-kpi-lbl">Total CPU</span>
        <span class="m-kpi-val" :class="(overview.total_cpu_percent || 0) >= 80 ? 'text-rose' : (overview.total_cpu_percent || 0) >= 50 ? 'text-amber' : 'text-emerald'">
          {{ Math.round(overview.total_cpu_percent || 0) }}%
        </span>
        <span class="m-kpi-sub">Saturation</span>
      </div>

      <div class="mobile-kpi-card glass-panel" @click="emit('deepDive')">
        <span class="m-kpi-lbl">Throughput</span>
        <span class="m-kpi-val text-emerald">{{ formatRps(effectiveHttpRps) }} <span class="m-unit">rps</span></span>
        <span class="m-kpi-sub text-cyan">🔍 Deep-Dive</span>
      </div>
    </div>

    <!-- 3. Touch Stream of Active Nodes -->
    <div class="mobile-nodes-stream">
      <div class="stream-section-title">
        <span>Active Infrastructure Nodes ({{ nodes.length }})</span>
      </div>
      <div class="nodes-list">
        <div
          v-for="node in nodes"
          :key="node.node_id"
          class="mobile-node-chip glass-panel"
          :class="{ 'chip-down': node.status === 'down' || node.status === 'offline', 'chip-hot': node.cpu_percent >= 75 }"
          @click="emit('inspect', node)"
        >
          <div class="node-chip-left">
            <span
              class="node-status-dot"
              :class="node.status === 'down' || node.status === 'offline' ? 'dot-red' : node.cpu_percent >= 75 ? 'dot-amber' : 'dot-green'"
            ></span>
            <div class="node-chip-info">
              <span class="node-chip-name font-bold">{{ node.node_name }}</span>
              <span class="node-chip-role font-mono">{{ node.role || 'worker' }}</span>
            </div>
          </div>
          <div class="node-chip-metrics font-mono">
            <span class="chip-m-val" :class="node.cpu_percent >= 80 ? 'text-rose' : node.cpu_percent >= 50 ? 'text-amber' : 'text-emerald'">
              CPU {{ Math.round(node.cpu_percent) }}%
            </span>
            <span class="chip-m-val text-cyan">
              RAM {{ Math.round(node.memory_percent) }}%
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mobile-overview-stream {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
}

.mobile-stream-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 10px;
  gap: 8px;
  flex-wrap: wrap;
}

.mobile-sync-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.last-sync-text {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  font-family: var(--font-mono, monospace);
}

.mobile-quick-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.btn-m-action {
  padding: 5px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-primary, #f8fafc);
  cursor: pointer;
}

.mobile-kpi-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
}

.mobile-kpi-card {
  padding: 10px 12px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 3px;
  cursor: pointer;
}

.m-kpi-lbl {
  font-size: 10.5px;
  color: var(--text-muted, #64748b);
  text-transform: uppercase;
  font-weight: 700;
  letter-spacing: 0.03em;
}

.m-kpi-val {
  font-size: 18px;
  font-weight: 800;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.m-unit { font-size: 12px; font-weight: 600; }
.m-kpi-sub { font-size: 10.5px; color: var(--text-secondary, #94a3b8); }

.mobile-nodes-stream {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.stream-section-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-secondary, #94a3b8);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.nodes-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.mobile-node-chip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.mobile-node-chip:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(56, 189, 248, 0.3);
}

.chip-down {
  border-color: rgba(244, 63, 94, 0.4);
  background: rgba(244, 63, 94, 0.08);
}

.chip-hot {
  border-color: rgba(245, 158, 11, 0.35);
}

.node-chip-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.node-status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.dot-green { background: #10b981; box-shadow: 0 0 6px #10b981; }
.dot-amber { background: #f59e0b; box-shadow: 0 0 6px #f59e0b; }
.dot-red { background: #f43f5e; box-shadow: 0 0 6px #f43f5e; }

.node-chip-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.node-chip-name { font-size: 12.5px; color: var(--text-primary, #f8fafc); }
.node-chip-role { font-size: 10px; color: var(--text-muted, #64748b); }

.node-chip-metrics {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  font-size: 11px;
}

.font-mono { font-family: var(--font-mono, monospace); }
.font-bold { font-weight: 700; }
.text-emerald { color: #10b981; }
.text-cyan { color: #06b6d4; }
.text-amber { color: #f59e0b; }
.text-rose { color: #f43f5e; }
.spin-icon { animation: spin 1s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
</style>
