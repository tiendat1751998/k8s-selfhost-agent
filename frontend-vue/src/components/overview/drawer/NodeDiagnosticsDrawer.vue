<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import type { NodeMetrics, SystemOverview, TpsSnapshot } from '../../../api/overview'
import type { NodeHistoryResponse, NodeMetricRollup } from '../../../api/compute'
import ModalDrawer from '../../ui/ModalDrawer.vue'
import NodeDrawerHeader from './NodeDrawerHeader.vue'
import NodeLiveDiagnostics from './NodeLiveDiagnostics.vue'
import NodeHistoricalChart from './NodeHistoricalChart.vue'
import NodeLogTerminal from './NodeLogTerminal.vue'

interface Props {
  show: boolean
  node: NodeMetrics | null
  overview?: SystemOverview | null
  tpsData?: TpsSnapshot | null
  nodeHistoryData?: NodeHistoryResponse | null
  nodeHistoryLoading?: boolean
  nodeHistoryRange?: string
  customHistFrom?: string
  customHistTo?: string
  initialMode?: 'live' | 'history'
}

const props = withDefaults(defineProps<Props>(), {
  overview: null,
  tpsData: null,
  nodeHistoryData: null,
  nodeHistoryLoading: false,
  nodeHistoryRange: '1h',
  customHistFrom: '',
  customHistTo: '',
  initialMode: 'live',
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:show', val: boolean): void
  (e: 'update:nodeHistoryRange', range: string): void
  (e: 'update:customHistFrom', val: string): void
  (e: 'update:customHistTo', val: string): void
  (e: 'range-change', range: string): void
  (e: 'custom-range-apply'): void
  (e: 'apply-preset', preset: '30m' | '2h' | '6h' | 'today'): void
  (e: 'manage-host', node: NodeMetrics): void
  (e: 'open-incidents'): void
}>()

const router = useRouter()
const nodeDrawerMode = ref<'live' | 'history'>(props.initialMode)
const logTerminalRef = ref<InstanceType<typeof NodeLogTerminal> | null>(null)

watch(() => props.initialMode, (newMode) => {
  nodeDrawerMode.value = newMode
})

function handleClose() {
  emit('close')
  emit('update:show', false)
}

function handleRangeChange(range: string) {
  emit('update:nodeHistoryRange', range)
  emit('range-change', range)
}

function onSyncPointInTime(point: NodeMetricRollup) {
  if (!point || !point.recorded_at) return
  const centerTime = new Date(point.recorded_at).getTime()
  if (isNaN(centerTime)) return

  const fromTime = new Date(centerTime - 15 * 60 * 1000)
  const toTime = new Date(centerTime + 15 * 60 * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  const formatDt = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`

  logTerminalRef.value?.syncToTime(formatDt(fromTime), formatDt(toTime))
}

function manageHost() {
  if (props.node) {
    emit('manage-host', props.node)
    router.push({ path: '/hosts', query: { search: props.node.node_name } })
  }
}

function openAiIncidents() {
  emit('open-incidents')
  router.push('/incidents')
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    maxWidth="1100px"
    @close="handleClose"
  >
    <div v-if="node" class="node-diagnostics-drawer">
      <!-- 1. Header Information Panel -->
      <NodeDrawerHeader :node="node" />

      <!-- 2. Drawer Mode Switcher -->
      <div class="drawer-mode-tabs">
        <button
          type="button"
          class="mode-tab-btn"
          :class="{ active: nodeDrawerMode === 'live' }"
          @click="nodeDrawerMode = 'live'"
        >
          <span class="tab-icon">⚡</span>
          <span>Live Diagnostics &amp; Workloads</span>
          <span class="tab-pill font-mono">Real-time</span>
        </button>

        <button
          type="button"
          class="mode-tab-btn"
          :class="{ active: nodeDrawerMode === 'history' }"
          @click="nodeDrawerMode = 'history'"
        >
          <span class="tab-icon">📈</span>
          <span>Historical Telemetry &amp; Audit</span>
          <span class="tab-pill font-mono">{{ nodeHistoryRange }}</span>
        </button>
      </div>

      <!-- TAB 1: LIVE DIAGNOSTICS -->
      <NodeLiveDiagnostics
        v-if="nodeDrawerMode === 'live'"
        :node="node"
        :overview="overview"
        :tpsData="tpsData"
      />

      <!-- TAB 2: HISTORICAL TELEMETRY & AUDIT -->
      <div v-else class="drawer-tab-content">
        <div class="node-history-content">
          <!-- Top: Multi-Series Chart & KPIs -->
          <NodeHistoricalChart
            :node="node"
            :nodeHistoryData="nodeHistoryData"
            :nodeHistoryLoading="nodeHistoryLoading"
            :nodeHistoryRange="nodeHistoryRange"
            :customHistFrom="customHistFrom"
            :customHistTo="customHistTo"
            @update:nodeHistoryRange="emit('update:nodeHistoryRange', $event)"
            @update:customHistFrom="emit('update:customHistFrom', $event)"
            @update:customHistTo="emit('update:customHistTo', $event)"
            @range-change="handleRangeChange"
            @custom-range-apply="emit('custom-range-apply')"
            @apply-preset="emit('apply-preset', $event)"
            @sync-point-in-time="onSyncPointInTime"
          />

          <!-- Bottom: Log Terminal & Incidents -->
          <NodeLogTerminal
            ref="logTerminalRef"
            :node="node"
            :nodeHistoryData="nodeHistoryData"
            :tpsData="tpsData"
          />
        </div>
      </div>

      <!-- 4. Drawer Footer Actions -->
      <div class="node-drawer-footer">
        <button
          type="button"
          class="btn btn-secondary"
          @click="manageHost"
        >
          <span>⚙️ Manage in Registry</span>
        </button>
        <button
          type="button"
          class="btn btn-primary"
          @click="openAiIncidents"
        >
          <span>🤖 AI Incident Diagnostics</span>
        </button>
      </div>
    </div>
  </ModalDrawer>
</template>

<style scoped>
.node-diagnostics-drawer {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 4px 0 16px;
}

.drawer-mode-tabs {
  display: flex;
  gap: 8px;
  background: rgba(15, 23, 42, 0.6);
  padding: 4px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  width: 100%;
}

.mode-tab-btn {
  flex: 1;
  padding: 8px 14px;
  border-radius: 8px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-muted, #94a3b8);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.mode-tab-btn:hover {
  color: var(--text-primary, #f8fafc);
  background: rgba(255, 255, 255, 0.05);
}

.mode-tab-btn.active {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border-color: rgba(56, 189, 248, 0.3);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.tab-pill {
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  background: rgba(255, 255, 255, 0.08);
  color: inherit;
}

.drawer-tab-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.node-history-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.node-drawer-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 10px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  font-weight: 600;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-primary {
  background: linear-gradient(135deg, #06b6d4 0%, #3b82f6 100%);
  color: #fff;
  border: none;
  box-shadow: 0 2px 8px rgba(6, 182, 212, 0.3);
}

.btn-primary:hover {
  opacity: 0.95;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(6, 182, 212, 0.4);
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-primary, #f8fafc);
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.2);
}

.font-mono { font-family: var(--font-mono, monospace); }
</style>\n