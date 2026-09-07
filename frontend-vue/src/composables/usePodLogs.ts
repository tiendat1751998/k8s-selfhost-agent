import { ref, computed, onMounted, onUnmounted, watch, nextTick, type Ref } from 'vue'
import { api } from '../api/client'

export interface PodLogProps {
  cluster: string
  pod: string
  namespace?: string
  container?: string
  containers?: string[]
}

export interface ParsedLogLine {
  id: number
  raw: string
  cleanText: string
  level: 'error' | 'warn' | 'info' | 'debug'
  timestamp?: string
}

export function usePodLogs(
  props: PodLogProps,
  containerRef?: Ref<HTMLElement | null>
) {
  // Log State
  const rawLines = ref<string[]>([])
  const selectedContainer = ref<string>(props.container || props.containers?.[0] || '')
  const tailLines = ref<number>(100)
  const isFollowing = ref<boolean>(true)
  const showPrevious = ref<boolean>(false)
  const wrapLines = ref<boolean>(false)
  const autoScroll = ref<boolean>(true)
  const userScrolledUp = ref<boolean>(false)
  const searchQuery = ref<string>('')
  const selectedLevel = ref<'all' | 'error' | 'warn' | 'info' | 'debug'>('all')

  const isLoading = ref<boolean>(false)
  const isConnected = ref<boolean>(false)
  const streamError = ref<string | null>(null)
  const logsCopied = ref<boolean>(false)

  const logContainerRef = containerRef || ref<HTMLElement | null>(null)
  let eventSource: EventSource | null = null
  let fetchAbortController: AbortController | null = null

  const ANSI_REGEX = /\u001b\[[0-9;]*m/g

  function stripAnsi(str: string): string {
    if (str.indexOf('\u001b') === -1 && str.indexOf('\x1b') === -1) {
      return str
    }
    return str.replace(ANSI_REGEX, '')
  }

  function classifyLogLevel(cleanLower: string): 'error' | 'warn' | 'info' | 'debug' {
    if (
      cleanLower.includes('error') ||
      cleanLower.includes('fatal') ||
      cleanLower.includes('panic') ||
      cleanLower.includes('exception') ||
      cleanLower.includes('fail') ||
      cleanLower.includes('err ') ||
      cleanLower.includes('[err]') ||
      cleanLower.includes('level=error')
    ) {
      return 'error'
    }
    if (
      cleanLower.includes('warn') ||
      cleanLower.includes('warning') ||
      cleanLower.includes('timeout') ||
      cleanLower.includes('level=warn')
    ) {
      return 'warn'
    }
    if (
      cleanLower.includes('debug') ||
      cleanLower.includes('trace') ||
      cleanLower.includes('level=debug')
    ) {
      return 'debug'
    }
    return 'info'
  }

  // Extract timestamp if line starts with ISO-8601 (e.g. 2026-08-27T...)
  const TIMESTAMP_REGEX = /^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\s*(.*)$/

  function parseLine(raw: string, id: number): ParsedLogLine {
    const clean = stripAnsi(raw)
    let timestamp: string | undefined
    let text = clean

    const match = clean.match(TIMESTAMP_REGEX)
    if (match) {
      timestamp = match[1]
      text = match[2]
    }

    const level = classifyLogLevel(clean.toLowerCase())

    return {
      id,
      raw,
      cleanText: text || clean,
      level,
      timestamp,
    }
  }

  // Memoized parsed log lines
  const allParsedLogs = computed<ParsedLogLine[]>(() => {
    const lines = rawLines.value
    const result: ParsedLogLine[] = []
    for (let i = 0; i < lines.length; i++) {
      const raw = lines[i]
      if (!raw && raw !== '') continue
      result.push(parseLine(raw, i + 1))
    }
    return result
  })

  // Counts per level
  const levelCounts = computed(() => {
    const all = allParsedLogs.value
    let error = 0
    let warn = 0
    let info = 0
    let debug = 0

    for (let i = 0; i < all.length; i++) {
      const lvl = all[i].level
      if (lvl === 'error') error++
      else if (lvl === 'warn') warn++
      else if (lvl === 'debug') debug++
      else info++
    }

    return { total: all.length, error, warn, info, debug }
  })

  // Filtered log lines based on level and search query
  const filteredLogs = computed<ParsedLogLine[]>(() => {
    const all = allParsedLogs.value
    const q = searchQuery.value.toLowerCase().trim()
    const lvl = selectedLevel.value

    if (lvl === 'all' && !q) {
      return all
    }

    const res: ParsedLogLine[] = []
    for (let i = 0; i < all.length; i++) {
      const item = all[i]
      if (lvl !== 'all' && item.level !== lvl) {
        continue
      }
      if (q && !item.cleanText.toLowerCase().includes(q) && !(item.timestamp && item.timestamp.toLowerCase().includes(q))) {
        continue
      }
      res.push(item)
    }
    return res
  })

  function buildLogsUrl(follow: boolean): string {
    const base = import.meta.env.VITE_API_BASE_URL || '/api/v1'
    const ns = props.namespace || 'default'
    const containerParam = selectedContainer.value ? `&container=${encodeURIComponent(selectedContainer.value)}` : ''
    const prevParam = showPrevious.value ? '&previous=true' : ''
    const token = localStorage.getItem('k8s_token')
    const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''

    return `${base}/k8s/${encodeURIComponent(props.cluster)}/pods/${encodeURIComponent(props.pod)}/logs?namespace=${encodeURIComponent(ns)}${containerParam}&follow=${follow}&tailLines=${tailLines.value}${prevParam}${tokenParam}`
  }

  function startStreaming() {
    stopStreaming()
    isLoading.value = true
    streamError.value = null
    isConnected.value = false

    const url = buildLogsUrl(true)

    try {
      eventSource = new EventSource(url)

      eventSource.onopen = () => {
        isConnected.value = true
        isLoading.value = false
        streamError.value = null
      }

      eventSource.onmessage = (event: MessageEvent) => {
        isLoading.value = false
        if (typeof event.data === 'string') {
          // SSE lines may come single or multiple
          const parts = event.data.split('\n')
          for (const p of parts) {
            if (p !== undefined) {
              rawLines.value.push(p)
            }
          }
          // Limit buffer to 10000 lines
          if (rawLines.value.length > 10000) {
            rawLines.value = rawLines.value.slice(rawLines.value.length - 8000)
          }

          if (autoScroll.value && !userScrolledUp.value) {
            nextTick(() => {
              scrollToBottom()
            })
          }
        }
      }

      eventSource.onerror = () => {
        isConnected.value = false
        // If we haven't received anything yet or connection closed
        if (rawLines.value.length === 0 && isLoading.value) {
          streamError.value = 'Failed to stream logs. Switching to static fetch...'
          isLoading.value = false
          // Fallback to static fetch
          fetchStaticLogs()
        }
      }
    } catch (err: unknown) {
      isLoading.value = false
      const msg = err instanceof Error ? err.message : 'EventSource creation failed'
      streamError.value = msg
      fetchStaticLogs()
    }
  }

  function stopStreaming() {
    if (eventSource) {
      eventSource.onopen = null
      eventSource.onmessage = null
      eventSource.onerror = null
      eventSource.close()
      eventSource = null
    }
    isConnected.value = false
  }

  async function fetchStaticLogs() {
    stopStreaming()
    if (fetchAbortController) {
      fetchAbortController.abort()
    }
    fetchAbortController = new AbortController()

    isLoading.value = true
    streamError.value = null

    try {
      const url = buildLogsUrl(false)
      const res = await api.request<{ logs?: string; data?: { logs?: string } } | string>(url, {
        signal: fetchAbortController.signal,
      })

      let text = ''
      if (typeof res === 'string') {
        text = res
      } else if (res && typeof res === 'object') {
        if ('data' in res && res.data && typeof res.data.logs === 'string') {
          text = res.data.logs
        } else if ('logs' in res && typeof res.logs === 'string') {
          text = res.logs
        }
      }

      if (text) {
        rawLines.value = text.split('\n')
      } else {
        rawLines.value = []
      }

      if (autoScroll.value && !userScrolledUp.value) {
        nextTick(() => {
          scrollToBottom()
        })
      }
    } catch (err: unknown) {
      if (err instanceof Error && err.name === 'AbortError') return
      const msg = err instanceof Error ? err.message : 'Failed to fetch static logs'
      streamError.value = msg
    } finally {
      isLoading.value = false
    }
  }

  function reloadLogs() {
    rawLines.value = []
    if (isFollowing.value) {
      startStreaming()
    } else {
      fetchStaticLogs()
    }
  }

  function toggleFollowing() {
    isFollowing.value = !isFollowing.value
    if (isFollowing.value) {
      startStreaming()
    } else {
      stopStreaming()
    }
  }

  function handleScroll(e: Event) {
    const el = e.target as HTMLElement
    if (!el) return
    const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50
    userScrolledUp.value = !isNearBottom
  }

  function scrollToBottom() {
    if (logContainerRef.value) {
      logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
      userScrolledUp.value = false
    }
  }

  function clearLogs() {
    rawLines.value = []
  }

  async function copyLogs() {
    if (rawLines.value.length === 0) return
    try {
      const textToCopy = filteredLogs.value.map(l => l.cleanText).join('\n')
      await navigator.clipboard.writeText(textToCopy)
      logsCopied.value = true
      setTimeout(() => {
        logsCopied.value = false
      }, 2000)
    } catch {
      // clipboard copy fallback
    }
  }

  function downloadLogs() {
    if (rawLines.value.length === 0) return
    const textContent = filteredLogs.value.map(l => (l.timestamp ? `[${l.timestamp}] ` : '') + l.cleanText).join('\n')
    const blob = new Blob([textContent], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${props.pod}_${selectedContainer.value || 'container'}_logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  watch(
    () => [props.cluster, props.pod, props.namespace],
    () => {
      reloadLogs()
    }
  )

  watch(
    () => [selectedContainer.value, tailLines.value, showPrevious.value],
    () => {
      reloadLogs()
    }
  )

  watch(
    () => props.container,
    (newC) => {
      if (newC && newC !== selectedContainer.value) {
        selectedContainer.value = newC
      }
    }
  )

  onMounted(() => {
    if (isFollowing.value) {
      startStreaming()
    } else {
      fetchStaticLogs()
    }
  })

  onUnmounted(() => {
    stopStreaming()
    if (fetchAbortController) {
      fetchAbortController.abort()
      fetchAbortController = null
    }
  })

  return {
    rawLines,
    selectedContainer,
    tailLines,
    isFollowing,
    showPrevious,
    wrapLines,
    autoScroll,
    userScrolledUp,
    searchQuery,
    selectedLevel,
    isLoading,
    isConnected,
    streamError,
    logsCopied,
    logContainerRef,
    levelCounts,
    filteredLogs,
    allParsedLogs,
    reloadLogs,
    toggleFollowing,
    handleScroll,
    scrollToBottom,
    clearLogs,
    copyLogs,
    downloadLogs,
  }
}
