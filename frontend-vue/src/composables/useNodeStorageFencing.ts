import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import {
  getFencingPreflight,
  executeNodeFencing,
  type NodeFencingPreflight,
  type FencingExecutionReport,
  type ServiceRescueResult,
} from '../api/dr'

export interface UseNodeStorageFencingProps {
  show: boolean
  nodeName: string
  clusterId?: string
}

export function useNodeStorageFencing(
  props: UseNodeStorageFencingProps,
  emit: {
    (e: 'close'): void
    (e: 'update:show', val: boolean): void
    (e: 'fenced', report: FencingExecutionReport): void
  }
) {
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
    const conf = preflight.value?.dynamic_rca?.confidence ?? preflight.value?.confidence ?? 0
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
      const data = await getFencingPreflight(props.clusterId || 'default', props.nodeName)
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
      const report = await executeNodeFencing(props.clusterId || 'default', props.nodeName, {
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
      `# SRE Zero-Data-Loss Fencing & RCA: ${props.nodeName} (${props.clusterId || 'default'})`,
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

  return {
    stage,
    loadingPreflight,
    executing,
    errorMessage,
    copySuccess,
    preflight,
    confirmPowerIsolated,
    emergencyUntaintMaster,
    overrideCircuitBreaker,
    rescuedRecovered,
    execStep,
    evictionProgress,
    executionReport,
    workloadPills,
    circuitBreakerBlocked,
    canProceedStonith,
    handleRescued,
    executeFencing,
    copyRcaReport,
    handleClose,
  }
}
