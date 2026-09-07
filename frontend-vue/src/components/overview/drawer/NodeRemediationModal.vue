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
              <span class="safety-full">Automated &lt;30s fast-failover cordons the failed node (spec.unschedulable=true) and force-evicts stuck pods with zero grace period, immediately triggering replica controllers to reschedule workloads onto surviving nodes (worker1, worker2, k8smater).</span>
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
.remediation-backdrop { position: fixed; inset: 0; background: rgba(4, 6, 14, 0.82); backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px); z-index: 200000; display: flex; align-items: center; justify-content: center; padding: 16px; }
.remediation-modal { width: 100%; max-width: 560px; background: rgba(15, 23, 42, 0.97); border: 1px solid rgba(56, 189, 248, 0.35); border-radius: 12px; box-shadow: 0 20px 50px rgba(0, 0, 0, 0.85), 0 0 30px rgba(56, 189, 248, 0.15); display: flex; flex-direction: column; overflow: hidden; }
.modal-header { padding: 16px 20px; display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; border-bottom: 1px solid rgba(255, 255, 255, 0.08); }
.header-left { display: flex; gap: 12px; align-items: center; }
.header-icon { width: 36px; height: 36px; border-radius: 8px; font-size: 18px; display: flex; align-items: center; justify-content: center; background: rgba(245, 158, 11, 0.2); border: 1px solid rgba(245, 158, 11, 0.4); flex-shrink: 0; }
.modal-title { margin: 0; font-size: 15px; font-weight: 700; color: #f8fafc; }
.target-banner { margin-top: 4px; font-size: 12px; color: #94a3b8; }
.highlight { color: #38bdf8; font-weight: 600; }
.btn-close { background: transparent; border: none; color: #94a3b8; font-size: 16px; cursor: pointer; padding: 4px 8px; line-height: 1; border-radius: 4px; }
.btn-close:hover { color: #fff; background: rgba(255, 255, 255, 0.1); }
.safety-banner { padding: 12px 20px; background: rgba(56, 189, 248, 0.06); border-bottom: 1px solid rgba(56, 189, 248, 0.15); display: flex; gap: 10px; align-items: flex-start; }
.safety-icon { font-size: 16px; flex-shrink: 0; }
.safety-text { margin: 0; font-size: 12px; line-height: 1.5; color: #cbd5e1; }
.modal-body { padding: 20px; display: flex; flex-direction: column; gap: 16px; }
.prompt-text { margin: 0; font-size: 13px; color: #94a3b8; line-height: 1.5; }
.actions-row { display: flex; justify-content: flex-end; gap: 10px; flex-wrap: wrap; }
.btn { height: 34px; padding: 0 16px; border-radius: 8px; font-size: 12.5px; font-weight: 600; cursor: pointer; display: inline-flex; align-items: center; justify-content: center; gap: 6px; transition: all 0.2s ease; }
.btn-primary { background: linear-gradient(135deg, #0284c7 0%, #2563eb 100%); color: #fff; border: 1px solid rgba(56, 189, 248, 0.4); box-shadow: 0 2px 10px rgba(2, 132, 199, 0.3); }
.btn-primary:hover { opacity: 0.95; transform: translateY(-1px); }
.btn-action-execute { background: linear-gradient(135deg, #f59e0b 0%, #ef4444 100%); color: #fff; border: 1px solid rgba(239, 68, 68, 0.4); box-shadow: 0 0 15px rgba(239, 68, 68, 0.35); }
.btn-action-execute:hover { transform: translateY(-1px); box-shadow: 0 0 20px rgba(239, 68, 68, 0.5); }
.btn-secondary { background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.12); color: #e2e8f0; }
.btn-secondary:hover { background: rgba(255, 255, 255, 0.12); }
.btn-retry { background: linear-gradient(135deg, #e11d48 0%, #be123c 100%); color: #fff; border: 1px solid rgba(244, 63, 94, 0.4); }
.spinner-row { display: flex; align-items: center; gap: 12px; }
.cyber-spinner { width: 20px; height: 20px; border: 2px solid rgba(56, 189, 248, 0.2); border-top-color: #38bdf8; border-radius: 50%; animation: spin 0.8s linear infinite; }
.spinner-label { font-size: 13px; font-weight: 600; color: #38bdf8; }
.steps-list { display: flex; flex-direction: column; gap: 8px; }
.step-row { padding: 8px 12px; border-radius: 6px; font-size: 12px; background: rgba(255, 255, 255, 0.03); display: flex; justify-content: space-between; align-items: center; color: #64748b; border: 1px solid transparent; }
.step-row.active { color: #38bdf8; background: rgba(56, 189, 248, 0.1); border-color: rgba(56, 189, 248, 0.25); }
.step-row.done { color: #10b981; background: rgba(16, 185, 129, 0.08); }
.step-check { font-size: 10px; font-weight: 700; color: #10b981; }
.step-pulse { font-size: 10px; font-weight: 700; color: #38bdf8; animation: pulse 1s infinite; }
.success-banner { background: rgba(16, 185, 129, 0.08); border: 1px solid rgba(16, 185, 129, 0.25); border-radius: 8px; padding: 14px; display: flex; flex-direction: column; gap: 10px; }
.success-top { display: flex; align-items: center; justify-content: space-between; }
.success-heading { font-weight: 700; color: #34d399; font-size: 14px; }
.evicted-wrap { font-size: 12px; color: #cbd5e1; }
.evicted-title { font-weight: 600; margin-bottom: 4px; color: #94a3b8; }
.evicted-list { margin: 0; padding-left: 18px; color: #38bdf8; }
.evicted-empty { color: #94a3b8; font-style: italic; }
.error-banner { background: rgba(239, 68, 68, 0.1); border: 1px solid rgba(239, 68, 68, 0.3); border-radius: 8px; padding: 14px; display: flex; gap: 10px; align-items: flex-start; }
.error-icon { font-size: 16px; flex-shrink: 0; }
.error-content { display: flex; flex-direction: column; gap: 4px; }
.error-title { font-weight: 700; color: #f87171; font-size: 13px; }
.error-msg { font-size: 12px; color: #fca5a5; word-break: break-all; }
.font-mono { font-family: var(--font-mono, monospace); }
@keyframes spin { to { transform: rotate(360deg); } }
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }

.safety-full, .btn-text-full { display: inline; }
.safety-mobile, .btn-text-mobile { display: none; }

@media (max-width: 640px) {
  .safety-full, .btn-text-full { display: none !important; }
  .safety-mobile, .btn-text-mobile { display: inline !important; }
  .safety-banner { padding: 8px 10px; font-size: 11px; }
  .actions-row .btn { padding: 8px 12px; font-size: 12px; }
}
</style>
