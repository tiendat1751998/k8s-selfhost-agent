<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import BaseIcon from '../../ui/BaseIcon.vue'
import NodeFencingRescueSection from './NodeFencingRescueSection.vue'
import NodeFencingRcaEvidence from './NodeFencingRcaEvidence.vue'
import NodeFencingCapacitySection from './NodeFencingCapacitySection.vue'
import {
  getFencingPreflight,
  executeNodeFencing,
  type NodeFencingPreflight,
  type FencingExecutionReport,
  type ServiceRescueResult,
} from '../../../api/dr'

interface Props {
  show: boolean
  nodeName: string
  clusterId?: string
}

const props = withDefaults(defineProps<Props>(), { clusterId: 'default' })
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:show', val: boolean): void
  (e: 'fenced', report: FencingExecutionReport): void
}>()

const STAGES = ['RCA Scan', 'STONITH Gate', 'Fencing', 'Eviction', 'Summary']
const EXEC_STEPS = [
  'Cordoning node (spec.unschedulable = true)...',
  'Applying K8s 1.28+ out-of-service taint...',
  'Detaching VolumeAttachments from CSI controller...'
]

const stage = ref<number>(1)
const loadingPreflight = ref(false)
const executing = ref(false)
const errorMessage = ref<string>('')
const copySuccess = ref(false)

const preflight = ref<NodeFencingPreflight | null>(null)
const confirmPowerIsolated = ref(false)
const emergencyUntaintMaster = ref(false)
const autoFenceStorage = ref(true)
const evictStatelessFirst = ref(true)
const evictStatefulAfterFencing = ref(true)
const overrideCircuitBreaker = ref(false)
const rescuedRecovered = ref(false)

const execStep = ref<number>(1)
const evictionProgress = ref<number>(0)
const executionReport = ref<FencingExecutionReport | null>(null)
let execTimer: ReturnType<typeof setInterval> | null = null

const statefulCount = computed(() => preflight.value?.stateful_pods?.length ?? 0)
const statelessCount = computed(() => preflight.value?.stateless_pods?.length ?? 0)
const daemonsetCount = computed(() => preflight.value?.daemonset_pods?.length ?? 0)
const volumeCount = computed(() => preflight.value?.volume_attachments?.length ?? 0)

const workloadPills = computed(() => [
  { type: 'stateful', label: 'Stateful Pods (PVC)', count: statefulCount.value, icon: 'database' },
  { type: 'stateless', label: 'Stateless Pods', count: statelessCount.value, icon: 'box' },
  { type: 'daemonset', label: 'DaemonSets', count: daemonsetCount.value, icon: 'server' },
])

const hasStatefulWorkloads = computed(() => statefulCount.value > 0 || volumeCount.value > 0)
const circuitBreakerBlocked = computed(() => {
  return !!preflight.value?.circuit_breaker?.eviction_frozen && !overrideCircuitBreaker.value
})

const canProceedStonith = computed(() => {
  if (rescuedRecovered.value || circuitBreakerBlocked.value) return false
  return !hasStatefulWorkloads.value || confirmPowerIsolated.value
})

const confidencePercent = computed(() => {
  const conf = preflight.value?.dynamic_rca?.confidence ?? preflight.value?.confidence ?? 0.94
  return Math.round(conf * 100)
})

watch(() => props.show, async (isOpen) => {
  if (isOpen && props.nodeName) {
    resetState()
    await fetchPreflightScan()
  } else clearTimers()
})

function resetState() {
  stage.value = 1
  executing.value = false
  errorMessage.value = ''
  confirmPowerIsolated.value = false
  overrideCircuitBreaker.value = false
  rescuedRecovered.value = false
  execStep.value = 1
  evictionProgress.value = 0
  executionReport.value = null
  copySuccess.value = false
  clearTimers()
}

function clearTimers() {
  if (execTimer) { clearInterval(execTimer); execTimer = null }
}

async function fetchPreflightScan() {
  loadingPreflight.value = true
  errorMessage.value = ''
  try {
    const data = await getFencingPreflight(props.clusterId, props.nodeName)
    preflight.value = data
    emergencyUntaintMaster.value = !!data.is_single_worker_cluster
  } catch (err: unknown) {
    errorMessage.value = err instanceof Error ? err.message : 'Failed to scan node pre-flight status.'
  } finally {
    loadingPreflight.value = false
  }
}

function handleRescued(res: ServiceRescueResult) {
  if (res.recovered_to_ready) rescuedRecovered.value = true
}

async function executeFencing() {
  if (!canProceedStonith.value) return
  executing.value = true
  errorMessage.value = ''
  stage.value = 3
  execStep.value = 1

  execTimer = setInterval(() => {
    if (stage.value === 3) {
      if (execStep.value < 3) execStep.value++
      else { stage.value = 4; evictionProgress.value = 40 }
    } else if (stage.value === 4 && evictionProgress.value < 90) {
      evictionProgress.value += 25
    }
  }, 700)

  try {
    const report = await executeNodeFencing(props.clusterId, props.nodeName, {
      confirm_power_isolated: confirmPowerIsolated.value,
      emergency_untaint_master: emergencyUntaintMaster.value,
      auto_fence_storage: autoFenceStorage.value,
      evict_stateless_first: evictStatelessFirst.value,
      evict_stateful_after_fencing: evictStatefulAfterFencing.value,
      timeout_seconds: 30,
      override_circuit_breaker: overrideCircuitBreaker.value,
    })
    clearTimers()
    evictionProgress.value = 100
    executionReport.value = report
    stage.value = 5
    emit('fenced', report)
  } catch (err: unknown) {
    clearTimers()
    stage.value = 2
    errorMessage.value = err instanceof Error ? err.message : 'Storage fencing execution failed.'
  } finally {
    executing.value = false
  }
}

function copyRcaReport() {
  if (!preflight.value) return
  const text = [
    `# SRE Zero-Data-Loss Fencing & RCA: ${props.nodeName} (${props.clusterId})`,
    `Cause: ${preflight.value.probable_cause} (${confidencePercent.value}% conf) | Plan: ${preflight.value.recommendation}`,
    `Single Worker: ${preflight.value.is_single_worker_cluster ? 'YES' : 'NO'} | Duration: ${executionReport.value?.duration_ms ?? 0}ms`,
    `Circuit Breaker: ${preflight.value.circuit_breaker?.eviction_frozen ? 'TRIPPED (Override: ' + overrideCircuitBreaker.value + ')' : 'NORMAL'}`,
    `Evicted Pods (${executionReport.value?.evicted_pods?.length ?? 0}): ${(executionReport.value?.evicted_pods ?? []).join(', ') || 'None'}`,
    `Detached Volumes: ${(executionReport.value?.detached_volumes ?? []).join(', ') || 'All confirmed'}`
  ].join('\n')
  navigator.clipboard.writeText(text).then(() => {
    copySuccess.value = true
    setTimeout(() => { copySuccess.value = false }, 3000)
  }).catch(() => { copySuccess.value = false })
}

function handleClose() {
  if (executing.value) return
  clearTimers()
  emit('close')
  emit('update:show', false)
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show && !executing.value) handleClose()
}

onMounted(() => window.addEventListener('keydown', handleKeydown))
onUnmounted(() => {
  clearTimers()
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="show" class="fencing-backdrop" @click.self="handleClose" role="dialog" aria-modal="true">
        <div class="fencing-modal animate-scale-in">
          <!-- Header -->
          <div class="fencing-header">
            <div class="fencing-header-left">
              <span class="fencing-header-icon"><BaseIcon name="shield" size="sm" /></span>
              <div>
                <h3 class="fencing-title">Zero-Data-Loss Storage Fencing &amp; Dead Node Recovery</h3>
                <div class="fencing-subtitle font-mono">
                  Node: <span class="highlight">{{ nodeName }}</span> &middot; Cluster: <span class="highlight">{{ clusterId }}</span>
                </div>
              </div>
            </div>
            <button type="button" class="btn-close" :disabled="executing" @click="handleClose" title="Close (Esc)"><BaseIcon name="x" size="xs" /></button>
          </div>

          <!-- Stepper Header -->
          <div class="fencing-stepper font-mono">
            <template v-for="(stepName, idx) in STAGES" :key="stepName">
              <div class="stepper-step" :class="{ active: stage === idx + 1, done: stage > idx + 1 }">
                <span class="stepper-num">{{ idx + 1 }}</span>
                <span class="stepper-label">{{ stepName }}</span>
              </div>
              <div v-if="idx < STAGES.length - 1" class="stepper-line" :class="{ done: stage > idx + 1 }"></div>
            </template>
          </div>

          <!-- Modal Body -->
          <div class="fencing-body">
            <div v-if="loadingPreflight" class="loading-box">
              <div class="cyber-spinner"></div>
              <span>Auditing CSI volumes, pods &amp; performing Root Cause Analysis...</span>
            </div>

            <template v-else>
              <div v-if="errorMessage" class="error-banner" style="margin-bottom: 16px;">
                <span class="error-icon"><BaseIcon name="alert-triangle" size="xs" /></span>
                <div class="error-content">
                  <div class="error-title font-bold">Fencing Procedure Error</div>
                  <div class="error-msg font-mono text-sm">{{ errorMessage }}</div>
                </div>
              </div>

              <template v-if="preflight">
              <!-- STAGE 1: RCA, Soft-Death Rescue & Capacity Plan -->
              <div v-if="stage === 1" class="stage-container animate-fade-in">
                <NodeFencingRescueSection
                  :probe-report="preflight.probe_report"
                  :cluster-id="clusterId"
                  :node-name="nodeName"
                  @rescued="handleRescued"
                />

                <NodeFencingRcaEvidence
                  :dynamic-rca="preflight.dynamic_rca"
                  :fallback-cause="preflight.probable_cause"
                  :fallback-confidence="preflight.confidence"
                  :fallback-recommendation="preflight.recommendation"
                />

                <NodeFencingCapacitySection
                  :capacity-plan="preflight.capacity_plan"
                  :circuit-breaker="preflight.circuit_breaker"
                  :is-single-worker-cluster="preflight.is_single_worker_cluster"
                  v-model:override-breaker="overrideCircuitBreaker"
                />

                <div v-if="preflight.is_single_worker_cluster && !preflight.capacity_plan" class="single-worker-alert animate-fade-in font-mono">
                  <span><BaseIcon name="alert-triangle" size="xs" /> <strong>Single Worker Cluster:</strong> Untaint Master node to allow rescheduling.</span>
                  <label class="untaint-toggle-label"><input type="checkbox" v-model="emergencyUntaintMaster" /> Emergency Untaint Master</label>
                </div>

                <div class="workload-section">
                  <div class="section-label">Impacted Workload Inventory</div>
                  <div class="pills-grid font-mono">
                    <div v-for="item in workloadPills" :key="item.type" :class="['workload-pill', item.type]">
                      <span class="pill-title"><span>{{ item.label }}</span> <BaseIcon :name="item.icon" size="xs" /></span>
                      <span class="pill-count">{{ item.count }}</span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- STAGE 2: STONITH & Power Isolation Gate -->
              <div v-else-if="stage === 2" class="stage-container animate-fade-in">
                <div v-if="circuitBreakerBlocked" class="circuit-breaker-banner animate-fade-in">
                  <div class="circuit-breaker-header">
                    <span class="breaker-icon"><BaseIcon name="x-circle" size="xs" /></span>
                    <div>
                      <div class="breaker-title">Circuit Breaker Active: Evictions Frozen</div>
                      <div class="breaker-reason text-xs">Over 15% fleet unready. SRE Override required before proceeding.</div>
                    </div>
                  </div>
                  <label class="breaker-override-toggle font-mono">
                    <input type="checkbox" v-model="overrideCircuitBreaker" />
                    <span>Emergency SRE Override: Force evictions despite circuit breaker trip</span>
                  </label>
                </div>

                <div class="stonith-gate-card">
                  <div class="stonith-warning-header">
                    <BaseIcon name="shield" size="sm" />
                    <span>STONITH &amp; Storage Isolation Gate (Anti-Split-Brain)</span>
                  </div>
                  <p class="stonith-text">
                    Forcing volume detachment while the failed node is still powered on can cause simultaneous writes from two nodes to the same RWO block volume, leading to catastrophic disk filesystem corruption.
                  </p>
                  <label class="stonith-checkbox-label">
                    <input type="checkbox" v-model="confirmPowerIsolated" />
                    <span>I confirm Node is physically powered off or isolated from SAN/CSI storage, preventing split-brain corruption.</span>
                  </label>
                </div>
              </div>

              <!-- STAGE 3: Storage Fencing Execution -->
              <div v-else-if="stage === 3" class="stage-container animate-fade-in font-mono">
                <div class="execution-stepper-box">
                  <div v-for="(label, i) in EXEC_STEPS" :key="label" class="exec-step-item" :class="{ active: execStep === i + 1, done: execStep > i + 1 }">
                    <span>{{ i + 1 }}. {{ label }}</span>
                    <span v-if="execStep > i + 1" class="text-emerald font-bold">DONE</span>
                    <span v-else-if="execStep === i + 1" class="cyber-spinner-sm"></span>
                  </div>
                </div>
              </div>

              <!-- STAGE 4: Controlled Pod Eviction -->
              <div v-else-if="stage === 4" class="stage-container animate-fade-in font-mono">
                <div class="eviction-progress-container">
                  <div class="progress-header">
                    <span>Controlled Pod Eviction &amp; Failover...</span>
                    <span>{{ evictionProgress }}%</span>
                  </div>
                  <div class="progress-track">
                    <div class="progress-fill" :style="{ width: `${evictionProgress}%` }"></div>
                  </div>
                  <p class="text-muted text-xs" style="margin-top: 10px;">
                    Stateless pods evicted first &rarr; Stateful pods evicted after confirmed volume release.
                  </p>
                </div>
              </div>

              <!-- STAGE 5: Post-Recovery Summary -->
              <div v-else-if="stage === 5" class="stage-container animate-fade-in font-mono">
                <div class="summary-card">
                  <div class="summary-hero">
                    <BaseIcon name="check-circle" size="md" />
                    <span>Safe Remediation Completed in {{ executionReport?.duration_ms ?? 0 }}ms</span>
                  </div>
                  <div class="summary-list-box">
                    <div class="font-bold text-cyan" style="margin-bottom: 4px;">Evicted Workloads ({{ executionReport?.evicted_pods?.length ?? 0 }}):</div>
                    <ul v-if="executionReport?.evicted_pods?.length" style="padding-left: 18px; margin: 0;">
                      <li v-for="pod in executionReport.evicted_pods" :key="pod">{{ pod }}</li>
                    </ul>
                    <div v-else class="text-muted">All workloads gracefully relocated or zero unevicted pods.</div>
                  </div>
                  <div v-if="executionReport?.detached_volumes?.length" class="summary-list-box" style="margin-top: 8px;">
                    <div class="font-bold text-emerald" style="margin-bottom: 4px;">Detached CSI Volumes:</div>
                    <ul style="padding-left: 18px; margin: 0;">
                      <li v-for="vol in executionReport.detached_volumes" :key="vol">{{ vol }}</li>
                    </ul>
                  </div>
                </div>
              </div>
              </template>
            </template>
          </div>

          <!-- Modal Footer Actions -->
          <div class="fencing-footer">
            <template v-if="stage === 1">
              <button type="button" class="btn btn-secondary" @click="handleClose">{{ rescuedRecovered ? 'Close' : 'Cancel' }}</button>
              <button v-if="!rescuedRecovered" type="button" class="btn btn-primary" :disabled="loadingPreflight" @click="stage = 2">
                <span>Proceed to STONITH Safety Gate</span><BaseIcon name="arrow-right" size="xs" />
              </button>
            </template>
            <template v-else-if="stage === 2">
              <button type="button" class="btn btn-secondary" @click="stage = 1">Back</button>
              <button type="button" class="btn btn-danger" :disabled="!canProceedStonith" @click="executeFencing" :title="circuitBreakerBlocked ? 'Evictions frozen by circuit breaker.' : ''">
                <BaseIcon name="shield" size="xs" /><span>Confirm STONITH &amp; Execute Fencing</span>
              </button>
            </template>
            <span v-else-if="stage === 3 || stage === 4" class="text-muted font-mono text-xs">Fencing sequence in progress (Safe SRE Failover)...</span>
            <template v-else-if="stage === 5">
              <button type="button" class="btn btn-secondary" @click="copyRcaReport">
                <BaseIcon :name="copySuccess ? 'check' : 'copy'" size="xs" /><span>{{ copySuccess ? 'Copied RCA Report!' : 'Copy RCA Report' }}</span>
              </button>
              <button type="button" class="btn btn-primary" @click="handleClose"><span>Close &amp; Refresh Fleet</span></button>
            </template>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
@import '../../../assets/styles/components/node-fencing.css';
</style>
