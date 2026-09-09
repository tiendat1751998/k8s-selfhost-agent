<script setup lang="ts">
import { ref, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { clusterApi, type BootstrapRequest } from '../../api/cluster'

const props = withDefaults(
  defineProps<{
    show: boolean
    clusterId: string
    clusterName?: string
    initialItem?: string
  }>(),
  { show: false }
)

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'bootstrapped'): void
}>()

interface OptionItem {
  id: string
  name: string
  desc: string
  icon: string
}

const availableOptions: OptionItem[] = [
  { id: 'metrics-server', name: 'Metrics Server', desc: 'Resource metrics pipeline for kubectl top and HPA.', icon: '📊' },
  { id: 'local-storage', name: 'Local StorageClass', desc: 'Dynamic local-path-provisioner storageclass.', icon: '💾' },
  { id: 'agent-daemonset', name: 'Agent DaemonSet', desc: 'Cluster host-agent metrics and health probe.', icon: '🤖' },
  { id: 'master-taints', name: 'Master Taints Removal', desc: 'Allow workload pods to run on control plane nodes.', icon: '⚡' },
]

const selected = ref<string[]>([])
const executing = ref(false)
const finished = ref(false)
const logs = ref<string[]>([])
const currentStep = ref<string>('')
const error = ref<string | null>(null)

watch(
  () => props.show,
  (val) => {
    if (val) {
      logs.value = []
      finished.value = false
      error.value = null
      currentStep.value = ''
      if (props.initialItem && availableOptions.some(o => o.id === props.initialItem)) {
        selected.value = [props.initialItem]
      } else {
        selected.value = availableOptions.map(o => o.id)
      }
    }
  }
)

function toggle(id: string) {
  if (executing.value) return
  const idx = selected.value.indexOf(id)
  if (idx === -1) selected.value.push(id)
  else selected.value.splice(idx, 1)
}

function selectAll() {
  if (executing.value) return
  selected.value = availableOptions.map(o => o.id)
}

function clearAll() {
  if (executing.value) return
  selected.value = []
}

async function startBootstrap() {
  if (selected.value.length === 0 || executing.value) return
  executing.value = true
  finished.value = false
  error.value = null
  logs.value = []

  const addLog = (msg: string) => {
    const time = new Date().toLocaleTimeString()
    logs.value.push(`[${time}] ${msg}`)
  }

  addLog(`Starting bootstrap process for cluster: ${props.clusterName || props.clusterId}...`)
  addLog(`Selected primitives: ${selected.value.join(', ')}`)

  try {
    const req: BootstrapRequest = {
      items: selected.value,
    }

    addLog('Executing cluster essentials deployment API call...')
    const result = await clusterApi.executeBootstrap(props.clusterId, req)

    if (result.logs && Array.isArray(result.logs)) {
      for (const log of result.logs) {
        addLog(log)
      }
    }

    if (result.steps && Array.isArray(result.steps)) {
      for (const step of result.steps) {
        currentStep.value = step.item
        addLog(`Step [${step.item}]: ${step.status.toUpperCase()}`)
        if (step.output) addLog(`   -> ${step.output}`)
        if (step.error) addLog(`   -> ERROR: ${step.error}`)
      }
    } else {
      addLog(`Status: ${result.success ? 'SUCCESS' : 'FAILED'}`)
      if (result.message) addLog(`Message: ${result.message}`)
    }

    if (!result.success) {
      throw new Error(result.message || 'Bootstrap step reported failure')
    }

    addLog('Cluster essentials bootstrapped successfully!')
    finished.value = true
    emit('bootstrapped')
  } catch (err: unknown) {
    const errMsg = err instanceof Error ? err.message : 'Bootstrap encountered an unexpected failure'
    error.value = errMsg
    addLog(`ERROR: ${errMsg}`)
  } finally {
    executing.value = false
  }
}

function handleClose() {
  if (executing.value) return
  emit('update:show', false)
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    width="680px"
    title="⚡ 1-Click Cluster Essentials Bootstrap"
    subtitle="Provision telemetry, local storage class, agent daemon, and configure node taints."
    @update:show="emit('update:show', $event)"
  >
    <div class="bootstrap-content">
      <div v-if="!executing && !finished" class="selection-pane">
        <div class="pane-top">
          <span class="step-lbl font-mono text-muted">SELECT COMPONENTS TO INSTALL</span>
          <div class="btn-group font-mono font-xs">
            <button class="link-btn" @click="selectAll">Select All</button>
            <span class="sep">|</span>
            <button class="link-btn" @click="clearAll">Clear</button>
          </div>
        </div>

        <div class="options-grid">
          <div
            v-for="opt in availableOptions"
            :key="opt.id"
            class="opt-card"
            :class="{ 'is-selected': selected.includes(opt.id) }"
            @click="toggle(opt.id)"
          >
            <div class="opt-check">
              <input type="checkbox" :checked="selected.includes(opt.id)" @click.stop="toggle(opt.id)" />
            </div>
            <span class="opt-icon">{{ opt.icon }}</span>
            <div class="opt-text">
              <strong class="opt-title font-mono">{{ opt.name }}</strong>
              <p class="opt-desc">{{ opt.desc }}</p>
            </div>
          </div>
        </div>
      </div>

      <div v-if="executing || finished || logs.length > 0" class="terminal-pane glass-panel">
        <div class="term-header">
          <div class="term-dots">
            <span class="dot red"></span>
            <span class="dot yellow"></span>
            <span class="dot green"></span>
          </div>
          <span class="term-title font-mono font-xs">
            {{ executing ? 'EXECUTION IN PROGRESS: ' + currentStep : finished ? 'INSTALLATION COMPLETED' : 'BOOTSTRAP TERMINAL' }}
          </span>
          <span v-if="executing" class="spinner font-mono">⏳</span>
        </div>
        <div class="term-body font-mono">
          <div v-for="(line, idx) in logs" :key="idx" class="term-line">{{ line }}</div>
        </div>
      </div>

      <div v-if="error" class="alert-error font-mono font-xs">
        ⚠️ {{ error }}
      </div>
    </div>

    <template #footer>
      <div class="footer-actions">
        <button class="btn btn-secondary btn-sm" :disabled="executing" @click="handleClose">
          <span>{{ finished ? 'Close' : 'Cancel' }}</span>
        </button>
        <button
          v-if="!finished"
          class="btn btn-primary btn-sm"
          :disabled="selected.length === 0 || executing"
          @click="startBootstrap"
        >
          <span>{{ executing ? '⏳ Applying...' : '⚡ Apply Essentials (' + selected.length + ')' }}</span>
        </button>
      </div>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/fleet.css';
</style>
