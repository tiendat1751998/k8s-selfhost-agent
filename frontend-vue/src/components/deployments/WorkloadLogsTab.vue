<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import type { DeploymentApp } from '../../api/compute'
import { dockerApi } from '../../api/compute'
import { k8sApi } from '../../api/k8s'
import BaseIcon from '../ui/BaseIcon.vue'
import WorkloadLogToolbar from './WorkloadLogToolbar.vue'

interface Props {
  app: DeploymentApp | null
  active?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  active: false
})

const emit = defineEmits<{
  (e: 'toast', msg: string, type?: 'success' | 'error' | 'info'): void
}>()

const router = useRouter()
const logsRawContent = ref<string>('')
const logsLoading = ref(false)
const logsError = ref<string | null>(null)
const logsSearchQuery = ref('')
const logsAutoScroll = ref(true)
const logsTerminalRef = ref<HTMLElement | null>(null)

const availablePods = ref<string[]>([])
const selectedPod = ref<string>('')
const availableContainers = ref<string[]>([])
const selectedContainer = ref<string>('')

const isK8s = computed(() => {
  return props.app?.type === 'kubernetes' || props.app?.type === 'k8s'
})

watch(
  () => props.active,
  (isActive) => {
    if (isActive && props.app) {
      fetchLogs(props.app)
    }
  },
  { immediate: true }
)

watch(
  () => props.app,
  (newApp) => {
    if (props.active && newApp) {
      selectedPod.value = ''
      selectedContainer.value = ''
      availablePods.value = []
      availableContainers.value = []
      fetchLogs(newApp)
    }
  }
)

watch([selectedPod, selectedContainer], () => {
  if (props.active && props.app) {
    fetchLogs(props.app)
  }
})

async function fetchLogs(targetApp?: DeploymentApp | null) {
  const target = targetApp || props.app
  if (!target) return

  logsLoading.value = true
  logsError.value = null

  try {
    if (isK8s.value) {
      const cluster = target.target && target.target !== 'docker-engine' && target.target !== 'swarm-manager'
        ? target.target
        : 'default'
      const ns = target.namespace && target.namespace !== 'all' ? target.namespace : 'default'

      const res = await k8sApi.getWorkloadLogs(cluster, target.name, ns, {
        pod: selectedPod.value || undefined,
        container: selectedContainer.value || undefined,
      })

      if (Array.isArray(res.pods) && res.pods.length > 0) {
        availablePods.value = res.pods
      }

      if (res.logs && res.logs.trim().length > 0) {
        logsRawContent.value = res.logs
      } else {
        logsRawContent.value = ''
      }
    } else if (target.type === 'docker' || target.type === 'swarm') {
      const targetId = target.rawId || target.id || target.name
      const targetType = target.type === 'swarm' ? 'service' : 'container'
      const res = await dockerApi.getLogs(targetId, targetType)
      logsRawContent.value = res?.logs?.trim() || ''
    } else {
      logsRawContent.value = ''
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to fetch logs'
    logsError.value = msg
    logsRawContent.value = ''
  } finally {
    logsLoading.value = false
    if (logsAutoScroll.value) {
      await nextTick()
      scrollLogsToBottom()
    }
  }
}

function scrollLogsToBottom() {
  if (logsTerminalRef.value) {
    logsTerminalRef.value.scrollTop = logsTerminalRef.value.scrollHeight
  }
}

const filteredLogLines = computed(() => {
  if (!logsRawContent.value) return []
  const rawLines = logsRawContent.value.split('\n')
  if (!logsSearchQuery.value.trim()) return rawLines
  const q = logsSearchQuery.value.toLowerCase().trim()
  return rawLines.filter(line => line.toLowerCase().includes(q))
})

watch(
  () => filteredLogLines.value.length,
  async () => {
    if (logsAutoScroll.value && logsTerminalRef.value) {
      await nextTick()
      scrollLogsToBottom()
    }
  }
)

function getLogLineClass(line: string): string {
  const lower = line.toLowerCase()
  if (lower.includes('error') || lower.includes('fatal') || lower.includes('panic') || lower.includes('fail') || lower.includes('exception')) {
    return 'log-line-error'
  }
  if (lower.includes('warn') || lower.includes('warning')) {
    return 'log-line-warn'
  }
  if (lower.includes('info') || lower.includes('listening') || lower.includes('ready') || lower.includes('started')) {
    return 'log-line-info'
  }
  return ''
}

function copyLogsToClipboard() {
  if (typeof navigator !== 'undefined' && navigator.clipboard) {
    navigator.clipboard.writeText(logsRawContent.value).then(() => {
      emit('toast', 'Container logs copied to clipboard!', 'info')
    }).catch(() => {
      emit('toast', 'Failed to copy logs', 'error')
    })
  }
}

function openFullLogs(targetApp?: DeploymentApp | null) {
  const target = targetApp || props.app
  if (!target) return
  router.push({
    path: '/logs',
    query: {
      namespace: target.namespace || '',
      pod: selectedPod.value || undefined,
      app: target.name,
      search: target.name,
    }
  })
}

defineExpose({
  fetchLogs
})
</script>

<template>
  <div v-if="app" class="insp-panel animate-fade-in logs-insp-panel">
    <WorkloadLogToolbar
      v-model:search-query="logsSearchQuery"
      v-model:auto-scroll="logsAutoScroll"
      v-model:selected-pod="selectedPod"
      v-model:selected-container="selectedContainer"
      :loading="logsLoading"
      :is-k8s="isK8s"
      :available-pods="availablePods"
      :available-containers="availableContainers"
      @refresh="fetchLogs(app)"
      @copy="copyLogsToClipboard"
      @full-stream="openFullLogs(app)"
    />

    <div class="logs-terminal-window">
      <div class="logs-terminal-titlebar">
        <div class="terminal-dots">
          <span class="term-dot term-close"></span>
          <span class="term-dot term-min"></span>
          <span class="term-dot term-max"></span>
        </div>
        <div class="terminal-title-text font-mono">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>{{ isK8s ? 'k8s' : 'docker' }}://{{ selectedPod || app.name }} [{{ app.type }}]</span>
          <span class="logs-count">({{ filteredLogLines.length }} lines)</span>
        </div>
        <div class="terminal-status font-mono">
          <span :class="app.status === 'healthy' ? 'text-emerald' : 'text-amber'">● {{ app.status }}</span>
        </div>
      </div>

      <div ref="logsTerminalRef" class="logs-terminal-body font-mono">
        <div v-if="logsLoading && filteredLogLines.length === 0" class="logs-loading-state">
          <BaseIcon name="refresh" size="xs" class="spin-icon" />
          <span>Connecting to container log stream...</span>
        </div>

        <div v-else-if="logsError" class="logs-empty-state">
          <BaseIcon name="alert-triangle" size="md" class="text-amber" />
          <p class="font-mono">Log stream unavailable: {{ logsError }}</p>
        </div>

        <template v-else-if="filteredLogLines.length > 0">
          <div
            v-for="(line, idx) in filteredLogLines"
            :key="idx"
            class="terminal-log-row"
            :class="getLogLineClass(line)"
          >
            <span class="log-row-num">{{ idx + 1 }}</span>
            <span class="log-row-text">{{ line }}</span>
          </div>
        </template>

        <div v-else class="logs-empty-state">
          <BaseIcon name="file-text" size="md" />
          <p v-if="logsSearchQuery">No log entries match the search filter "{{ logsSearchQuery }}".</p>
          <p v-else>No log entries available for {{ app.name }} (0 active pods or empty output).</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/workload-inspector.css';
</style>
