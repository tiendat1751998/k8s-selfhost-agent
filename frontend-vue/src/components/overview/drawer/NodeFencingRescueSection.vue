<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import BaseIcon from '../../ui/BaseIcon.vue'
import { executeNodeRescue, type NodeProbeReport, type ServiceRescueResult } from '../../../api/dr'

interface Props {
  probeReport?: NodeProbeReport
  clusterId: string
  nodeName: string
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false
})

const emit = defineEmits<{
  (e: 'rescued', result: ServiceRescueResult): void
}>()

const isRescuing = ref(false)
const rescueSuccess = ref(false)
const rescueMessage = ref('')
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

function clearRescueTimer() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

async function handleRescue() {
  if (isRescuing.value || props.disabled) return
  isRescuing.value = true
  rescueMessage.value = ''
  countdown.value = 30

  clearRescueTimer()
  timer = setInterval(() => {
    if (countdown.value > 0) {
      countdown.value--
    } else {
      clearRescueTimer()
    }
  }, 1000)

  try {
    const res = await executeNodeRescue(props.clusterId, props.nodeName)
    clearRescueTimer()
    if (res.recovered_to_ready) {
      rescueSuccess.value = true
      rescueMessage.value = res.message || 'Node recovered to Ready! Fencing canceled — all workloads preserved.'
      emit('rescued', res)
    } else {
      rescueSuccess.value = false
      rescueMessage.value = res.message || 'Automated rescue did not recover node. Manual intervention or fencing required.'
    }
  } catch (err: unknown) {
    clearRescueTimer()
    rescueSuccess.value = false
    rescueMessage.value = err instanceof Error ? err.message : 'Automated service rescue failed.'
  } finally {
    isRescuing.value = false
  }
}

onUnmounted(() => {
  clearRescueTimer()
})
</script>

<template>
  <div class="rescue-section font-mono">
    <!-- Probe Health Grid -->
    <div v-if="probeReport" class="probe-telemetry-grid">
      <div class="probe-pill" :class="probeReport.ping_ok ? 'healthy' : 'failed'">
        <span class="probe-label">ICMP Host Ping</span>
        <span class="probe-val">
          <BaseIcon :name="probeReport.ping_ok ? 'check-circle' : 'x-circle'" size="xs" />
          {{ probeReport.ping_ok ? 'REACHABLE' : 'UNREACHABLE' }}
        </span>
      </div>
      <div class="probe-pill" :class="probeReport.agent_reachable ? 'healthy' : 'failed'">
        <span class="probe-label">Host Daemon Agent</span>
        <span class="probe-val">
          <BaseIcon :name="probeReport.agent_reachable ? 'check-circle' : 'x-circle'" size="xs" />
          {{ probeReport.agent_reachable ? 'CONNECTED' : 'DISCONNECTED' }}
        </span>
      </div>
      <div class="probe-pill" :class="probeReport.kubelet_healthy ? 'healthy' : 'failed'">
        <span class="probe-label">Kubelet Service</span>
        <span class="probe-val">
          <BaseIcon :name="probeReport.kubelet_healthy ? 'check-circle' : 'x-circle'" size="xs" />
          {{ probeReport.kubelet_healthy ? 'ACTIVE' : 'FAULT / DOWN' }}
        </span>
      </div>
      <div class="probe-pill" :class="probeReport.runtime_healthy ? 'healthy' : 'failed'">
        <span class="probe-label">CRI Containerd</span>
        <span class="probe-val">
          <BaseIcon :name="probeReport.runtime_healthy ? 'check-circle' : 'x-circle'" size="xs" />
          {{ probeReport.runtime_healthy ? 'RUNNING' : 'UNRESPONSIVE' }}
        </span>
      </div>
    </div>

    <!-- Soft-Death Banner & Rescue Trigger -->
    <div v-if="probeReport?.state === 'SOFT_DEATH_SERVICE_FAULT' && !rescueSuccess" class="soft-death-banner">
      <div class="soft-death-content">
        <div class="soft-death-title">
          <BaseIcon name="alert-triangle" size="sm" class="text-amber" />
          <span>Soft-Death Detected</span>
        </div>
        <p class="soft-death-desc">
          Host is alive, but Kubelet/Runtime is unresponsive. Try automated service restart first to avoid destructive storage fencing and zero pod evictions.
        </p>
      </div>
      <button
        type="button"
        class="btn-rescue"
        :disabled="isRescuing || disabled"
        @click="handleRescue"
      >
        <BaseIcon :name="isRescuing ? 'refresh' : 'zap'" size="xs" :class="{ 'spin-anim': isRescuing }" />
        <span v-if="isRescuing">Rescuing... ({{ countdown }}s)</span>
        <span v-else>Attempt Service Rescue</span>
      </button>
    </div>

    <!-- Rescue Success Banner -->
    <div v-if="rescueSuccess" class="rescue-success-banner animate-fade-in">
      <BaseIcon name="check-circle" size="sm" class="text-emerald" />
      <div>
        <strong>Node recovered to Ready!</strong> Fencing canceled — all workloads preserved.
      </div>
    </div>

    <!-- Rescue Failure Notice -->
    <div v-else-if="rescueMessage && !isRescuing" class="rescue-fail-note text-xs">
      <BaseIcon name="alert-triangle" size="xs" class="text-rose" />
      <span>{{ rescueMessage }}</span>
    </div>
  </div>
</template>
