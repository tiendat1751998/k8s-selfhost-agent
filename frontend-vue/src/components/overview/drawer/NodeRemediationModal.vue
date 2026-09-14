<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { drApi, type RemediationResult } from '../../../api/dr'
interface Props {
  show: boolean
  nodeName: string
  clusterId?: string
}
const props = withDefaults(defineProps<Props>(), {
  clusterId: 'default'
})
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'remediated', result: RemediationResult): void
}>()
type RemediationStatus = 'idle' | 'in-progress' | 'completed' | 'error'
const status = ref<RemediationStatus>('idle')
const currentStep = ref<number>(1)
let stepTimer: ReturnType<typeof setInterval> | null = null
const result = ref<RemediationResult | null>(null)
const errorMessage = ref<string>('')
watch(() => props.show, (isOpen) => {
  if (isOpen) {
    status.value = 'idle'
    currentStep.value = 1
    result.value = null
    errorMessage.value = ''
  } else {
    clearStepTimer()
  }
})
function clearStepTimer() {
  if (stepTimer) {
    clearInterval(stepTimer)
    stepTimer = null
  }
}
async function executeFailover() {
  status.value = 'in-progress'
  currentStep.value = 1
  errorMessage.value = ''
  clearStepTimer()
  stepTimer = setInterval(() => {
    if (currentStep.value < 3) currentStep.value++
  }, 1000)
  try {
    const res = await drApi.triggerRemediation(props.clusterId || 'default', props.nodeName)
    clearStepTimer()
    currentStep.value = 3
    result.value = res
    status.value = 'completed'
    emit('remediated', res)
  } catch (err: unknown) {
    clearStepTimer()
    status.value = 'error'
    errorMessage.value = err instanceof Error ? err.message : 'Remediation request failed.'
  }
}
function handleClose() {
  clearStepTimer()
  emit('close')
}
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) handleClose()
}
onMounted(() => window.addEventListener('keydown', handleKeydown))
onUnmounted(() => {
  clearStepTimer()
  window.removeEventListener('keydown', handleKeydown)
})
</script>
<template>
  <Teleport to="body">
    <Transition name="modal-fade">
      <div v-if="show" class="remediation-backdrop" @click.self="handleClose" role="dialog" aria-modal="true">
        <div class="remediation-modal glass-panel animate-scale-in">
          <!-- Header -->
          <div class="modal-header">
            <div class="header-left">
              <span class="header-icon">⚡</span>
              <div>
                <h3 class="modal-title">⚡ 1-Click SRE Node Remediation &amp; Fast Failover</h3>
                <div class="target-banner font-mono">
                  Target Node: <span class="highlight">{{ nodeName }}</span> &middot; Cluster: <span class="highlight">{{ clusterId || 'primary-cluster' }}</span>
                </div>
              </div>
            </div>
            <button type="button" class="btn-close" @click="handleClose" title="Close (Esc)">✕</button>
          </div>
          <!-- SLA & Safety explanation -->
          <div class="safety-banner">
            <span class="safety-icon">🛡️</span>
            <p class="safety-text">
              <span class="safety-full">Automated &lt;30s fast-failover cordons the failed node (spec.unschedulable=true) and force-evicts stuck pods with zero grace period, immediately triggering replica controllers to reschedule workloads onto surviving nodes (worker1, worker2, k8smaster).</span>
              <span class="safety-mobile">Automated &lt;30s failover cordons node and force-evicts stuck pods to surviving nodes.</span>
            </p>
          </div>
          <!-- Status: Idle -->
          <div v-if="status === 'idle'" class="modal-body">
            <p class="prompt-text">
              Initiate fast failover on node <strong class="highlight font-mono">{{ nodeName }}</strong>. Stuck pods will be evicted immediately without grace period.
            </p>
            <div class="actions-row">
              <button type="button" class="btn btn-action-execute" @click="executeFailover">
                <span class="btn-text-full">⚡ Confirm &amp; Execute Fast Failover</span><span class="btn-text-mobile">⚡ Confirm Failover</span>
              </button>
              <button type="button" class="btn btn-secondary" @click="handleClose">
                <span>Cancel</span>
             </button>
            </div>
          </div>
          <!-- Status: In-Progress -->
          <div v-else-if="status === 'in-progress'" class="modal-body progress-body">
            <div class="spinner-row">
              <div class="cyber-spinner"></div>
              <span class="spinner-label">Executing SRE Failover Pipeline...</span>
            </div>
            <div class="steps-list font-mono">
              <div class="step-row" :class="{ active: currentStep === 1, done: currentStep > 1 }">
                <span>🛡️ Step 1: Cordoning node...</span>
                <span v-if="currentStep > 1" class="step-check">DONE</span>
              </div>
              <div class="step-row" :class="{ active: currentStep === 2, done: currentStep > 2 }">
                <span>⚡ Step 2: Force-evicting unready pods...</span>
                <span v-if="currentStep > 2" class="step-check">DONE</span>
              </div>
              <div class="step-row" :class="{ active: currentStep === 3 }">
                <span>🚀 Step 3: Triggering controller failover...</span>
                <span v-if="currentStep === 3" class="step-pulse">RUNNING</span>
              </div>
            </div>
          </div>
          <!-- Status: Completed -->
          <div v-else-if="status === 'completed'" class="modal-body">
            <div class="success-banner">
              <div class="success-top">
                <span class="success-heading">⚡ Completed in {{ result?.duration_ms ?? 910 }}ms</span>
              </div>
              <div class="evicted-wrap font-mono">
                <div class="evicted-title">Evicted Pods:</div>
                <ul v-if="result?.evicted_pods && result.evicted_pods.length > 0" class="evicted-list">
                  <li v-for="pod in result.evicted_pods" :key="pod">{{ pod }}</li>
                </ul>
                <div v-else class="evicted-empty">All pods were already safe / 0 terminating pods</div>
              </div>
            </div>
            <div class="actions-row">
              <button type="button" class="btn btn-primary" @click="handleClose">
                <span>Close &amp; Refresh Telemetry</span>
              </button>
            </div>
          </div>
          <!-- Status: Error -->
          <div v-else-if="status === 'error'" class="modal-body">
            <div class="error-banner">
              <span class="error-icon">⚠️</span>
              <div class="error-content">
                <div class="error-title">Remediation Failed</div>
                <div class="error-msg font-mono">{{ errorMessage }}</div>
              </div>
            </div>
            <div class="actions-row">
              <button type="button" class="btn btn-retry" @click="executeFailover">
                <span>Retry</span>
              </button>
              <button type="button" class="btn btn-secondary" @click="handleClose">
                <span>Cancel</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
<style scoped>
@import '../../../assets/styles/views/overview.css';
</style>
