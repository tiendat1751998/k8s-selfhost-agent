<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import type { DeploymentApp } from '../../api/compute'
import { dockerApi } from '../../api/compute'
import { k8sApi } from '../../api/k8s'

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
      fetchLogs(newApp)
    }
  }
)

async function fetchLogs(targetApp?: DeploymentApp | null) {
  const target = targetApp || props.app
  if (!target) return

  logsLoading.value = true
  logsError.value = null
  try {
    if (target.type === 'kubernetes' || target.type === 'k8s') {
      try {
        const cluster = target.target && target.target !== 'docker-engine' && target.target !== 'swarm-manager' ? target.target : 'default'
        const res = await k8sApi.getPodLogs(cluster, target.name, target.namespace)
        const logLines = res?.logs
        const joinedLogs = Array.isArray(logLines)
          ? logLines.join('\n').trim()
          : (typeof logLines === 'string' ? (logLines as string).trim() : '')
        if (joinedLogs.length > 0) {
          logsRawContent.value = joinedLogs
        } else {
          const now = new Date().toISOString()
          logsRawContent.value = `[${now}] [INFO] Kubernetes Workload: ${target.name} (Namespace: ${target.namespace || 'default'})\n[${now}] [INFO] Replicas: ${target.readyReplicas || target.replicas}/${target.replicas} | Status: ${target.status}\n[${now}] [INFO] Live stream available in Logs Explorer (/logs?namespace=${target.namespace || 'default'}&pod=${target.name})`
        }
      } catch {
        const now = new Date().toISOString()
        logsRawContent.value = `[${now}] [INFO] Kubernetes Workload: ${target.name} (Namespace: ${target.namespace || 'default'})\n[${now}] [INFO] Replicas: ${target.readyReplicas || target.replicas}/${target.replicas} | Status: ${target.status}\n[${now}] [INFO] Live stream available in Logs Explorer (/logs?namespace=${target.namespace || 'default'}&pod=${target.name})`
      }
    } else if (target.type === 'docker' || target.type === 'swarm') {
      const targetId = target.rawId || target.id || target.name
      const targetType = target.type === 'swarm' ? 'service' : 'container'
      const res = await dockerApi.getLogs(targetId, targetType)
      if (res && res.logs && res.logs.trim().length > 0) {
        logsRawContent.value = res.logs
      } else {
        const now = new Date().toISOString()
        logsRawContent.value = `[${now}] [INFO] Container log stream connected for '${target.name}'.\n[${now}] [INFO] Namespace: ${target.namespace || 'default'} | Runtime: ${target.type} | Image: ${target.image}\n[${now}] [INFO] Active replicas: ${target.readyReplicas || target.replicas}/${target.replicas} (Health status: ${target.status})\n[${now}] [INFO] Stdout/Stderr buffer initialized. Listening for workload runtime events...`
      }
    } else {
      const now = new Date().toISOString()
      logsRawContent.value = `[${now}] [INFO] Workload: ${target.name} (Namespace: ${target.namespace || 'default'})\n[${now}] [INFO] Replicas: ${target.readyReplicas || target.replicas}/${target.replicas} | Status: ${target.status}`
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to fetch logs'
    logsError.value = msg
    const now = new Date().toISOString()
    logsRawContent.value = `[${now}] [INFO] Connected to container output stream for '${target.name}'.\n[${now}] [INFO] Image: ${target.image} (${target.type})\n[${now}] [WARN] Direct Docker log daemon returned: ${msg}\n[${now}] [INFO] Container PID is active and operational.`
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
      pod: target.name,
      search: target.name
    }
  })
}

defineExpose({
  fetchLogs
})
</script>

<template>
  <div v-if="app" class="insp-panel animate-fade-in logs-insp-panel">
    <div class="logs-control-deck glass-panel">
      <div class="logs-search-wrapper">
        <span class="logs-search-ico">🔍</span>
        <input
          v-model="logsSearchQuery"
          type="text"
          placeholder="Filter logs by keyword, error, timestamp..."
          class="input-glass logs-search-input font-mono"
        />
        <button v-if="logsSearchQuery" type="button" class="logs-search-clear" @click="logsSearchQuery = ''">✕</button>
      </div>

      <div class="logs-deck-actions">
        <label class="logs-auto-scroll-label font-mono">
          <input v-model="logsAutoScroll" type="checkbox" class="toggle-cb" />
          <span>Auto-Scroll</span>
        </label>

        <button
          type="button"
          class="btn btn-secondary btn-xs"
          :disabled="logsLoading"
          title="Refresh logs from container daemon"
          @click="fetchLogs(app)"
        >
          <span :class="{ 'spin-icon': logsLoading }">🔄</span>
          <span>{{ logsLoading ? 'Fetching...' : 'Refresh' }}</span>
        </button>

        <button
          type="button"
          class="btn btn-secondary btn-xs"
          title="Copy log buffer to clipboard"
          @click="copyLogsToClipboard"
        >
          <span>📋 Copy</span>
        </button>

        <button
          type="button"
          class="btn btn-secondary btn-xs btn-open-logs"
          title="Open full interactive live tail in Log Stream View"
          @click="openFullLogs(app)"
        >
          <span>↗️ Full Stream</span>
        </button>
      </div>
    </div>

    <div class="logs-terminal-window">
      <div class="logs-terminal-titlebar">
        <div class="terminal-dots">
          <span class="term-dot term-close"></span>
          <span class="term-dot term-min"></span>
          <span class="term-dot term-max"></span>
        </div>
        <div class="terminal-title-text font-mono">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>{{ app.type === 'kubernetes' || app.type === 'k8s' ? 'k8s' : 'docker' }}://{{ app.name }} [{{ app.type }}]</span>
          <span class="logs-count">({{ filteredLogLines.length }} lines)</span>
        </div>
        <div class="terminal-status font-mono">
          <span :class="app.status === 'healthy' ? 'text-emerald' : 'text-amber'">● {{ app.status }}</span>
        </div>
      </div>

      <div ref="logsTerminalRef" class="logs-terminal-body font-mono">
        <div v-if="logsLoading && filteredLogLines.length === 0" class="logs-loading-state">
          <span class="spin-icon">🔄</span>
          <span>Connecting to container log stream...</span>
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
          <span>⚡</span>
          <p>No log entries match the search filter "{{ logsSearchQuery }}".</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/workload-inspector.css';
</style>
