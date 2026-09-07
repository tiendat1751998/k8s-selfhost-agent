import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { useLogStore, type LogEntry } from '../stores/logStore'

export type { LogEntry }
export type LogLevel = 'ALL' | 'INFO' | 'WARN' | 'ERROR' | 'DEBUG'

export interface LogStreamerOptions {
  maxBufferSize?: number
  defaultNamespace?: string
  defaultPod?: string
}

export function useLogStreamer(options: LogStreamerOptions = {}) {
  const maxBufferSize = options.maxBufferSize ?? 1000
  const route = useRoute()
  const logStore = useLogStore()

  // Primary filtering state
  const searchKeyword = ref('')
  const selectedLevel = ref<string>('')
  const selectedNamespace = ref<string>(options.defaultNamespace || '')
  const selectedPod = ref<string>(options.defaultPod || '')

  // Auto-scroll and terminal lock state
  const autoScroll = ref(true)
  const isScrollLocked = ref(false)
  const terminalElement = ref<HTMLElement | null>(null)

  // Metrics telemetry
  const linesStreamed = ref(0)
  const latency = ref(0)
  let latencyInterval: ReturnType<typeof setInterval> | null = null

  const availableNamespaces = ['production', 'staging', 'logging', 'vault', 'kube-system', 'monitoring']

  function initFromRoute() {
    if (typeof route.query.namespace === 'string') selectedNamespace.value = route.query.namespace
    if (typeof route.query.search === 'string') {
      searchKeyword.value = route.query.search
    } else if (typeof route.query.pod === 'string') {
      selectedPod.value = route.query.pod
      searchKeyword.value = route.query.pod
    }
  }

  function connect(namespace?: string, pod?: string) {
    logStore.connect(namespace ?? selectedNamespace.value, pod ?? selectedPod.value)
    startLatencyTracker()
  }

  function disconnect() {
    logStore.disconnect()
    stopLatencyTracker()
  }

  function startLatencyTracker() {
    stopLatencyTracker()
    latencyInterval = setInterval(() => {
      latency.value = 0
    }, 3000)
  }

  function stopLatencyTracker() {
    if (latencyInterval) {
      clearInterval(latencyInterval)
      latencyInterval = null
    }
  }

  function appendLog(entry: LogEntry) {
    linesStreamed.value++
    logStore.appendLog(entry)
  }

  function clearBuffer() {
    logStore.clear()
  }

  function togglePause() {
    logStore.togglePause()
  }

  // Regex safe compiler
  const compiledRegex = computed(() => {
    const kw = searchKeyword.value.trim()
    if (!kw) return null
    try {
      return new RegExp(kw, 'i')
    } catch {
      return null
    }
  })

  // Filtered logs computation
  const filteredLogs = computed(() => {
    const level = selectedLevel.value
    const ns = selectedNamespace.value
    const pod = selectedPod.value.toLowerCase().trim()
    const kw = searchKeyword.value.trim().toLowerCase()
    const regex = compiledRegex.value

    return logStore.logs.filter((log) => {
      if (level && log.level.toUpperCase() !== level.toUpperCase()) return false
      if (ns && log.namespace !== ns) return false
      if (pod && !log.pod.toLowerCase().includes(pod)) return false
      if (kw) {
        if (regex) {
          const matchMsg = regex.test(log.msg)
          const matchPod = regex.test(log.pod)
          const matchTrace = log.traceId ? regex.test(log.traceId) : false
          if (!matchMsg && !matchPod && !matchTrace) return false
        } else {
          const matchMsg = log.msg.toLowerCase().includes(kw)
          const matchPod = log.pod.toLowerCase().includes(kw)
          const matchTrace = log.traceId?.toLowerCase().includes(kw)
          if (!matchMsg && !matchPod && !matchTrace) return false
        }
      }
      return true
    })
  })

  // HUD KPI Metrics
  const errorCount = computed(() => logStore.logs.filter((l) => l.level.toUpperCase() === 'ERROR').length)
  const errorRate = computed(() => {
    const total = logStore.logs.length
    return total === 0 ? 0 : parseFloat(((errorCount.value / total) * 100).toFixed(1))
  })
  const bufferSaturation = computed(() => Math.min(100, Math.round((logStore.logs.length / maxBufferSize) * 100)))

  function setTerminalRef(el: HTMLElement | null) {
    terminalElement.value = el
  }

  function toggleAutoScroll() {
    autoScroll.value = !autoScroll.value
    if (autoScroll.value) {
      isScrollLocked.value = false
      scrollToBottom()
    } else {
      isScrollLocked.value = true
    }
  }

  function handleScroll(e: Event) {
    const target = e.target as HTMLElement
    if (!target) return
    const isAtBottom = target.scrollHeight - target.scrollTop - target.clientHeight < 40
    isScrollLocked.value = !isAtBottom
    autoScroll.value = isAtBottom
  }

  async function scrollToBottom() {
    await nextTick()
    if (terminalElement.value) {
      terminalElement.value.scrollTop = terminalElement.value.scrollHeight
    }
  }

  watch(
    () => filteredLogs.value.length,
    async () => {
      if (autoScroll.value && !isScrollLocked.value) {
        await scrollToBottom()
      }
    }
  )

  function exportRawText(format: 'text' | 'json' = 'text'): string {
    const logsToExport = filteredLogs.value.length > 0 ? filteredLogs.value : logStore.logs
    const content = format === 'json'
      ? JSON.stringify(logsToExport, null, 2)
      : logsToExport
          .map((l) => `[${l.time}] [${l.level.padEnd(5)}] [${l.namespace}/${l.pod}]: ${l.msg}${l.traceId ? ` [trace=${l.traceId}]` : ''}`)
          .join('\n')

    if (typeof window !== 'undefined') {
      const blob = new Blob([content], { type: format === 'json' ? 'application/json' : 'text/plain' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
      a.href = url
      a.download = `k8s-logs-${selectedNamespace.value || 'all'}-${timestamp}.${format === 'json' ? 'json' : 'log'}`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    }
    return content
  }

  watch(
    () => logStore.logs.length,
    (newLen, oldLen) => {
      if (newLen > (oldLen || 0)) linesStreamed.value += newLen - (oldLen || 0)
    },
    { immediate: true }
  )

  onMounted(() => {
    initFromRoute()
    connect(selectedNamespace.value, selectedPod.value)
  })

  onUnmounted(() => disconnect())

  return {
    logStore,
    searchKeyword,
    selectedLevel,
    selectedNamespace,
    selectedPod,
    availableNamespaces,
    autoScroll,
    isScrollLocked,
    terminalElement,
    linesStreamed,
    latency,
    errorCount,
    errorRate,
    bufferSaturation,
    maxBufferSize,
    filteredLogs,
    isConnected: computed(() => logStore.isConnected),
    isPaused: computed(() => logStore.isPaused),
    totalBufferCount: computed(() => logStore.logs.length),
    connect,
    disconnect,
    appendLog,
    clearBuffer,
    togglePause,
    setTerminalRef,
    toggleAutoScroll,
    scrollToBottom,
    handleScroll,
    exportRawText,
  }
}

export type LogStreamerReturn = ReturnType<typeof useLogStreamer>
