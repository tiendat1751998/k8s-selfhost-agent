<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import type { NodeMetrics, SystemOverview, TpsSnapshot } from '../../../api/overview'
import type { NodeHistoryResponse, NodeMetricRollup } from '../../../api/compute'
import { k8sApi } from '../../../api/k8s'
import type { RemediationResult } from '../../../api/dr'
import NodeDrawerHeader from './NodeDrawerHeader.vue'
import NodeLiveDiagnostics from './NodeLiveDiagnostics.vue'
import NodeHistoricalChart from './NodeHistoricalChart.vue'
import NodeLogTerminal from './NodeLogTerminal.vue'
import NodeRemediationModal from './NodeRemediationModal.vue'

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
  (e: 'range-change', range: string, from?: string, to?: string): void
  (e: 'custom-range-apply', from?: string, to?: string): void
  (e: 'apply-preset', preset: '30m' | '2h' | '6h' | 'today'): void
  (e: 'update:initialMode', val: 'live' | 'history'): void
  (e: 'manage-host', node: NodeMetrics): void
  (e: 'open-incidents'): void
}>()

const router = useRouter()
const nodeDrawerMode = ref<'live' | 'history'>(props.initialMode)
const logTerminalRef = ref<InstanceType<typeof NodeLogTerminal> | null>(null)
const drawerBodyScrollRef = ref<HTMLElement | null>(null)

const showRemediationModal = ref(false)
const isCordonLoading = ref(false)
const isNodeUnschedulable = ref(!!props.node?.unschedulable)
const cordonFeedback = ref<{ text: string; type: 'success' | 'error' } | null>(null)

watch(() => props.node?.unschedulable, (val) => {
  isNodeUnschedulable.value = !!val
})

function setFeedback(text: string, type: 'success' | 'error') {
  cordonFeedback.value = { text, type }
  setTimeout(() => {
    if (cordonFeedback.value?.text === text) cordonFeedback.value = null
  }, 3500)
}

async function toggleCordon() {
  if (!props.node) return
  isCordonLoading.value = true
  const nodeName = props.node.node_name
  const cluster = 'default'
  try {
    if (isNodeUnschedulable.value) {
      await k8sApi.uncordonNode(cluster, nodeName)
      isNodeUnschedulable.value = false
      setFeedback(`Node ${nodeName} uncordoned`, 'success')
    } else {
      await k8sApi.cordonNode(cluster, nodeName)
      isNodeUnschedulable.value = true
      setFeedback(`Node ${nodeName} cordoned`, 'success')
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Cordon action failed'
    setFeedback(msg, 'error')
  } finally {
    isCordonLoading.value = false
  }
}

function onRemediated(res: RemediationResult) {
  isNodeUnschedulable.value = true
  setFeedback(`Node remediated: ${res.node_name}`, 'success')
}

function resetDrawerScroll() {
  nextTick(() => {
    drawerBodyScrollRef.value?.scrollTo({ top: 0, behavior: 'instant' })
  })
}

function handleClose() {
  emit('close')
  emit('update:show', false)
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    handleClose()
  }
}

function switchMode(mode: 'live' | 'history') {
  nodeDrawerMode.value = mode
  emit('update:initialMode', mode)
  resetDrawerScroll()
}

watch(() => props.initialMode, (newMode) => {
  nodeDrawerMode.value = newMode
  resetDrawerScroll()
})

watch(nodeDrawerMode, () => {
  resetDrawerScroll()
})

watch(
  () => props.show,
  (isOpen) => {
    if (typeof document !== 'undefined') {
      document.body.style.overflow = isOpen ? 'hidden' : ''
    }
    if (isOpen) {
      resetDrawerScroll()
    }
  }
)

watch(
  () => props.node?.node_id,
  () => {
    resetDrawerScroll()
  }
)

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.addEventListener('keydown', handleKeydown)
  }
})

onUnmounted(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('keydown', handleKeydown)
    document.body.style.overflow = ''
  }
})

function handleRangeChange(range: string, from?: string, to?: string) {
  emit('update:nodeHistoryRange', range)
  emit('range-change', range, range === 'custom' ? (from || props.customHistFrom) : undefined, range === 'custom' ? (to || props.customHistTo) : undefined)
}

function onSyncPointInTime(point: NodeMetricRollup, suspectApp?: string) {
  if (!point || !point.recorded_at) return
  const centerTime = new Date(point.recorded_at).getTime()
  if (isNaN(centerTime)) return

  const fromTime = new Date(centerTime - 15 * 60 * 1000)
  const toTime = new Date(centerTime + 15 * 60 * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  const formatDt = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}TX${pad(d.getHours())}:${pad(d.getMinutes())}`.replace('X', 'T')

  logTerminalRef.value?.syncToTime(formatDt(fromTime), formatDt(toTime), suspectApp)
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
  <Teleport to="body">
    <Transition name="overlay-fade">
      <div
        v-if="show"
        class="drawer-backdrop"
        @click.self="handleClose"
        role="dialog"
        aria-modal="true"
      >
        <Transition name="drawer-slide" appear>
          <aside
            v-if="show && node"
            class="node-diagnostics-drawer glass-panel"
          >
            <NodeDrawerHeader :node="node" @close="handleClose" />

            <div class="drawer-tabs-pinned">
              <div class="drawer-mode-tabs">
                <button
                  type="button"
                  class="mode-tab-btn"
                  :class="{ active: nodeDrawerMode === 'live' }"
                  @click="switchMode('live')"
                >
                  <span class="tab-icon">📩</span>
                  <span class="tab-label-full">Live Telemetry (Real-time)</span>
                  <span class="tab-label-mobile">Live</span>
                  <span class="tab-pill">5s Poll</span>
                </button>
                <button
                  type="button"
                  class="mode-tab-btn"
                  :class="{ active: nodeDrawerMode === 'history' }"
                  @click="switchMode('history')"
                >
                  <span class="tab-icon">🔅</span>
                  <span class="tab-label-full">Historical Trends (Timeseries)</span>
                  <span class="tab-label-mobile">History</span>
                  <span class="tab-pill">1h - 24h</span>
                </button>
              </div>
            </div>

            <div ref="drawerBodyScrollRef" class="drawer-body-scroll custom-scrollbar">
              <div v-if="nodeDrawerMode === 'live'" class="drawer-tab-content animate-fade-in">
                <NodeLiveDiagnostics :node="node" :overview="overview" :tpsData="tpsData" />
              </div>
              <div v-else class="drawer-tab-content animate-fade-in">
                <div class="node-history-content">
                  <NodeHistoricalChart
                     :node="node"
                    :nodeHistoryData="nodeHistoryData"
                    :nodeHistoryLoading="nodeHistoryLoading"
                    :nodeHistoryRange="nodeHistoryRange"
                    :customHistFrom="customHistFrom"
                    :customHistTo="customHistTo"
                    @update:customHistFrom="emit('update:customHistFrom', $event)"
                    @update:customHistTo="emit('update:customHistTo', $event)"
                    @range-change="handleRangeChange"
                    @custom-range-apply="emit('custom-range-apply', customHistFrom, customHistTo)"
                    @apply-preset="emit('apply-preset', $event)"
                    @sync-point-in-time="onSyncPointInTime"
                  />
                  <NodeLogTerminal
                    ref="logTerminalRef"
                    :node="node"
                    :nodeHistoryData="nodeHistoryData"
                    :tpsData="tpsData"
                  />
                </div>
              </div>
            </div>

            <!-- 4. Drawer Footer Actions -->
            <div class="node-drawer-footer">
              <span v-if="cordonFeedback" class="footer-toast font-mono" :class="cordonFeedback.type">
                {{ cordonFeedback.text }}
              </span>
              <button
                 type="button"
                class="btn btn-primary btn-failover"
                @click="showRemediationModal = true"
                title="Trigger 1-Click Fast Failover SRE Remediation"
              >
                <span>⚡ 1-Click Failover</span>
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-cordon"
                :disabled="isCordonLoading"
                @click="toggleCordon"
                :title="isNodeUnschedulable ? 'Mark node schedulable' : 'Mark node unschedulable'"
              >
                <span>{{ isNodeUnschedulable ? '🔵 Uncordon' : '🛡️ Cordon' }}</span>
              </button>
              <button type="button" class="btn btn-secondary" @click="manageHost">
                <span>⚙️ Manage Host</span>
              </button>
              <button type="button" class="btn btn-primary" @click="openAiIncidents">
                <span>🤖 AI RCA</span>
              </button>
              <button type="button" class="btn btn-secondary btn-close-footer" @click="handleClose" title="Close drawer (Esc)">
                <span>✕ Close</span>
              </button>
            </div>
          </aside>
        </Transition>
      </div>
    </Transition>

    <!-- 1-Click SRE Node Remediation Modal -->
    <NodeRemediationModal
      v-if="node"
      :show="showRemediationModal"
      :nodeName="node.node_name"
      clusterId="default"
      @close="showRemediationModal = false"
      @remediated="onRemediated"
    />
  </Teleport>
</template>

<style scoped>
@import '../../../assets/styles/components/node-diagnostics.css';
</style>
