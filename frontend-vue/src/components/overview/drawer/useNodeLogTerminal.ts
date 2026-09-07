import { ref, computed, onMounted, onUnmounted, nextTick, watch, type Ref } from 'vue'
import type { NodeMetrics, TpsSnapshot, TpsServiceMetrics } from '../../../api/overview'
import type { NodeHistoryResponse } from '../../../api/compute'
import { requestNodeLogs } from './fetchNodeLogs'
import {
  type ParsedLogLine,
  LINE_HEIGHT,
  OVERSCAN,
  stripAnsi,
  classifyLogLevel,
  formatBadgeDate,
  formatDateTimeLocal,
} from './nodeLogUtils'

export interface LogTerminalProps {
  node: NodeMetrics
  nodeHistoryData?: NodeHistoryResponse | null
  tpsData?: TpsSnapshot | null
}

export function useNodeLogTerminal(props: LogTerminalProps, externalTerminalRef?: Ref<HTMLElement | null>) {
  // Log Filters & Config State
  const selectedLogApp = ref<string>('all')
  const selectedLogTail = ref<number | 'all'>(100)
  const selectedLogSince = ref<string>('15m')
  const selectedLogLevel = ref<'all' | 'error' | 'warn' | 'info' | 'debug'>('all')
  const appLogSearch = ref<string>('')
  const customLogFrom = ref<string>('')
  const customLogTo = ref<string>('')
  const showCustomDatePicker = ref<boolean>(false)
  const wrapLogLines = ref<boolean>(false)

  // Real-Time Live Auto-Tail & Scroll State
  const isLogStreaming = ref<boolean>(true)
  const logStreamInterval = ref<any>(null)
  const autoScrollLogs = ref<boolean>(true)
  const userScrolledUp = ref<boolean>(false)
  const terminalBodyRef = externalTerminalRef || ref<HTMLElement | null>(null)

  const appLogsText = ref<string>('')
  const appLogsLoading = ref<boolean>(false)
  const appLogsError = ref<string | null>(null)
  const appLogsCopied = ref<boolean>(false)

  // Virtual Scrolling State
  const terminalScrollTop = ref<number>(0)
  const terminalViewportHeight = ref<number>(380)

  // Services running on this node
  const rawNodeServices = computed<TpsServiceMetrics[]>(() => {
    if (!props.node || !props.tpsData?.services) return []
    const nodeId = (props.node.node_id || '').toLowerCase().trim()
    const nodeName = (props.node.node_name || '').toLowerCase().trim()

    return props.tpsData.services.filter(s => {
      const sNodeId = (s.node_id || '').toLowerCase().trim()
      const sNodeName = (s.node_name || '').toLowerCase().trim()
      if (sNodeId && (sNodeId === nodeId || sNodeId === nodeName)) return true
      if (sNodeName && (sNodeName === nodeName || sNodeName === nodeId)) return true
      return false
    })
  })

  const nodeAvailableLogApps = computed(() => {
    const list: { name: string; type: 'container' | 'host'; icon: string }[] = []
    const seen = new Set<string>()

    for (const s of rawNodeServices.value || []) {
      if (!seen.has(s.service_name)) {
        seen.add(s.service_name)
        list.push({ name: s.service_name, type: 'container', icon: '📦' })
      }
    }

    for (const p of props.node?.top_processes || []) {
      const pName = p.name || ''
      if (pName && !seen.has(pName) && !pName.startsWith('[')) {
        seen.add(pName)
        list.push({ name: pName, type: 'host', icon: '⚙️' })
      }
    }

    if (list.length === 0) {
      list.push({ name: 'k8s-agent', type: 'host', icon: '📡' })
    }
    return list
  })

  const nodeAvailableContainerApps = computed(() =>
    nodeAvailableLogApps.value.filter(app => app.type === 'container')
  )

  const nodeAvailableHostApps = computed(() =>
    nodeAvailableLogApps.value.filter(app => app.type === 'host')
  )

  function onLogSinceChange() {
    if (selectedLogSince.value === 'custom') {
      showCustomDatePicker.value = true
      if (!customLogFrom.value) {
        const now = new Date()
        customLogFrom.value = formatDateTimeLocal(new Date(now.getTime() - 2 * 60 * 60 * 1000))
        customLogTo.value = formatDateTimeLocal(now)
      }
    } else {
      showCustomDatePicker.value = false
    }
    fetchNodeAppLogs(selectedLogApp.value)
  }

  function applyLogPreset(preset: '30m' | '2h' | '6h' | 'today') {
    const now = new Date()
    if (preset === '30m') {
      customLogFrom.value = formatDateTimeLocal(new Date(now.getTime() - 30 * 60 * 1000))
      customLogTo.value = formatDateTimeLocal(now)
    } else if (preset === '2h') {
      customLogFrom.value = formatDateTimeLocal(new Date(now.getTime() - 2 * 60 * 60 * 1000))
      customLogTo.value = formatDateTimeLocal(now)
    } else if (preset === '6h') {
      customLogFrom.value = formatDateTimeLocal(new Date(now.getTime() - 6 * 60 * 60 * 1000))
      customLogTo.value = formatDateTimeLocal(now)
    } else if (preset === 'today') {
      const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0)
      customLogFrom.value = formatDateTimeLocal(startOfDay)
      customLogTo.value = formatDateTimeLocal(now)
    }
    selectedLogSince.value = 'custom'
    showCustomDatePicker.value = true
    fetchNodeAppLogs(selectedLogApp.value)
  }

  // Parsed log lines
  const allParsedLogs = computed<ParsedLogLine[]>(() => {
    const text = appLogsText.value
    if (!text) return []
    const rawLines = text.split('\n')
    const count = rawLines.length
    const lines: ParsedLogLine[] = []

    for (let i = 0; i < count; i++) {
      const raw = rawLines[i]
      if (!raw.trim()) continue

      const clean = stripAnsi(raw)
      const lower = clean.toLowerCase()
      const level = classifyLogLevel(lower)

      lines.push({ id: i + 1, raw, cleanText: clean, level })
    }
    return lines
  })

  const appLogLevelCounts = computed(() => {
    const all = allParsedLogs.value
    let error = 0
    let warn = 0
    let info = 0
    let debug = 0

    for (let i = 0; i < all.length; i++) {
      const level = all[i].level
      if (level === 'error') error++
      else if (level === 'warn') warn++
      else if (level === 'debug') debug++
      else info++
    }
    return { total: all.length, error, warn, info, debug }
  })

  const parsedAppLogLines = computed<ParsedLogLine[]>(() => {
    const all = allParsedLogs.value
    const q = appLogSearch.value.toLowerCase().trim()
    const filterLevel = selectedLogLevel.value

    if (filterLevel === 'all' && !q) return all

    const lines: ParsedLogLine[] = []
    for (let i = 0; i < all.length; i++) {
      const item = all[i]
      if (filterLevel !== 'all' && item.level !== filterLevel) continue
      if (q && !item.cleanText.toLowerCase().includes(q)) continue
      lines.push(item)
    }
    return lines
  })

  const totalLogCount = computed(() => parsedAppLogLines.value.length)
  const totalVirtualHeight = computed(() => totalLogCount.value * LINE_HEIGHT)
  const startIndex = computed(() => Math.max(0, Math.floor(terminalScrollTop.value / LINE_HEIGHT) - OVERSCAN))
  const endIndex = computed(() => Math.min(totalLogCount.value, Math.ceil((terminalScrollTop.value + terminalViewportHeight.value) / LINE_HEIGHT) + OVERSCAN))
  const visibleLogLines = computed(() => {
    return parsedAppLogLines.value.slice(startIndex.value, endIndex.value).map((line, idx) => ({
      ...line,
      virtualTop: (startIndex.value + idx) * LINE_HEIGHT,
    }))
  })

  async function fetchNodeAppLogs(appName?: string) {
    const targetApp = appName || selectedLogApp.value || nodeAvailableLogApps.value[0]?.name || 'all'
    selectedLogApp.value = targetApp
    appLogsLoading.value = true
    appLogsError.value = null
    const isHostApp = nodeAvailableLogApps.value.find(a => a.name === targetApp)?.type === 'host'
    try {
      const logs = await requestNodeLogs({
        targetApp,
        tail: selectedLogTail.value,
        since: selectedLogSince.value,
        customFrom: customLogFrom.value,
        customTo: customLogTo.value,
        search: appLogSearch.value,
        filterLevel: selectedLogLevel.value,
        nodeTarget: props.node?.node_id || props.node?.node_name,
        nodeName: props.node?.node_name,
        isHostApp,
      })
      appLogsText.value = logs

      if (autoScrollLogs.value && !userScrolledUp.value) {
        nextTick(() => scrollToBottom())
      }
    } catch (err: any) {
      if (isHostApp) {
        appLogsError.value = null
        appLogsText.value = `[INFO] Host process '${targetApp}' is running as a system service. Standard stdout/stderr stream is only accessible for Docker containerized workloads (e.g. tiki_redis, my-nginx, db, nats).`
      } else {
        appLogsError.value = err.message || 'Failed to fetch logs from agent daemon'
        appLogsText.value = `[ERROR] Unable to reach agent log collector on node ${props.node?.node_name}: ${err.message}`
      }
    } finally {
      appLogsLoading.value = false
    }
  }

  async function fetchNodeAppLogsQuiet(appName?: string) {
    if (appLogsLoading.value) return
    const targetApp = appName || selectedLogApp.value || nodeAvailableLogApps.value[0]?.name || 'all'
    const isHostApp = nodeAvailableLogApps.value.find(a => a.name === targetApp)?.type === 'host'
    try {
      const logs = await requestNodeLogs({
        targetApp,
        tail: selectedLogTail.value,
        since: selectedLogSince.value,
        customFrom: customLogFrom.value,
        customTo: customLogTo.value,
        search: appLogSearch.value,
        filterLevel: selectedLogLevel.value,
        nodeTarget: props.node?.node_id || props.node?.node_name,
        nodeName: props.node?.node_name,
        isHostApp,
      })

      if (logs && logs.trim().length > 0) {
        appLogsText.value = logs
        if (autoScrollLogs.value && !userScrolledUp.value) {
          nextTick(() => scrollToBottom())
        }
      }
    } catch {
      // Silent fail for quiet background stream
    }
  }

  function startLogStreaming() {
    stopLogStreaming()
    isLogStreaming.value = true
    logStreamInterval.value = setInterval(() => {
      if (isLogStreaming.value) fetchNodeAppLogsQuiet()
    }, 3000)
  }

  function stopLogStreaming() {
    if (logStreamInterval.value) {
      clearInterval(logStreamInterval.value)
      logStreamInterval.value = null
    }
    isLogStreaming.value = false
  }

  function toggleLogStreaming() {
    if (isLogStreaming.value) {
      stopLogStreaming()
    } else {
      startLogStreaming()
      fetchNodeAppLogsQuiet()
    }
  }

  function handleTerminalScroll(event: Event) {
    const el = event.target as HTMLElement
    if (!el) return
    terminalScrollTop.value = el.scrollTop
    terminalViewportHeight.value = el.clientHeight || 380
    const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40
    userScrolledUp.value = !isNearBottom
  }

  function scrollToBottom() {
    if (terminalBodyRef.value) {
      terminalBodyRef.value.scrollTop = terminalBodyRef.value.scrollHeight
      terminalScrollTop.value = terminalBodyRef.value.scrollTop
      userScrolledUp.value = false
    }
  }

  function clearLogTerminal() {
    appLogsText.value = ''
    terminalScrollTop.value = 0
  }

  async function copyAppLogs() {
    if (!appLogsText.value) return
    try {
      await navigator.clipboard.writeText(appLogsText.value)
      appLogsCopied.value = true
      setTimeout(() => { appLogsCopied.value = false }, 2000)
    } catch {}
  }

  function downloadAppLogs() {
    const blob = new Blob([appLogsText.value], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${props.node?.node_name || 'node'}_${selectedLogApp.value}_logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  function selectIncidentLog(inc: any) {
    if (!inc) return
    if (inc.recorded_at) {
      const centerTime = new Date(inc.recorded_at).getTime()
      if (!isNaN(centerTime)) {
        const fromTime = new Date(centerTime - 15 * 60 * 1000)
        const toTime = new Date(centerTime + 15 * 60 * 1000)
        customLogFrom.value = formatDateTimeLocal(fromTime)
        customLogTo.value = formatDateTimeLocal(toTime)
        selectedLogSince.value = 'custom'
        showCustomDatePicker.value = true
      }
    }
    selectedLogLevel.value = 'error'
    fetchNodeAppLogs(selectedLogApp.value)
  }

  function syncToTime(from: string, to: string, targetApp?: string) {
    if (targetApp) {
      const matched = nodeAvailableLogApps.value.find(
        app => app.name.toLowerCase() === targetApp.toLowerCase()
      )
      selectedLogApp.value = matched ? matched.name : targetApp
    }
    customLogFrom.value = from
    customLogTo.value = to
    selectedLogSince.value = 'custom'
    showCustomDatePicker.value = true
    fetchNodeAppLogs(selectedLogApp.value)
    nextTick(() => {
      terminalBodyRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
  }

  watch(() => props.node?.node_id, (newNodeId) => {
    if (newNodeId) {
      if (nodeAvailableLogApps.value.length > 0) {
        selectedLogApp.value = nodeAvailableLogApps.value[0].name
      }
      fetchNodeAppLogs(selectedLogApp.value)
    }
  })

  onMounted(() => {
    if (nodeAvailableLogApps.value.length > 0) {
      selectedLogApp.value = nodeAvailableLogApps.value[0].name
    }
    fetchNodeAppLogs(selectedLogApp.value)
    startLogStreaming()
  })

  onUnmounted(() => {
    stopLogStreaming()
  })

  return {
    selectedLogApp,
    selectedLogTail,
    selectedLogSince,
    selectedLogLevel,
    appLogSearch,
    customLogFrom,
    customLogTo,
    showCustomDatePicker,
    wrapLogLines,
    isLogStreaming,
    autoScrollLogs,
    userScrolledUp,
    terminalBodyRef,
    appLogsLoading,
    appLogsError,
    appLogsCopied,
    nodeAvailableContainerApps,
    nodeAvailableHostApps,
    appLogLevelCounts,
    parsedAppLogLines,
    totalVirtualHeight,
    visibleLogLines,
    onLogSinceChange,
    applyLogPreset,
    fetchNodeAppLogs,
    toggleLogStreaming,
    handleTerminalScroll,
    scrollToBottom,
    clearLogTerminal,
    copyAppLogs,
    downloadAppLogs,
    selectIncidentLog,
    syncToTime,
    formatBadgeDate,
  }
}
