import { ref, computed, watch } from 'vue'
import type { Ref } from 'vue'
import type { LogTarget } from '../components/logs/LogTargetTree.vue'
import type { LogFilterParams } from '../api/logging'
import type { useLogStore, LogEntry } from '../stores/logStore'
import { isNoiseLog, isSystemdService } from '../utils/logTargetCategorizer'
import { matchesTarget } from '../stores/logStore'

export interface TimeRangeConfig {
  label: string
  ms: number
  interval: number
}

export const LOG_TIME_RANGES: TimeRangeConfig[] = [
  { label: '15m', ms: 900000, interval: 15 },
  { label: '1h', ms: 3600000, interval: 60 },
  { label: '6h', ms: 21600000, interval: 300 },
  { label: '24h', ms: 86400000, interval: 1800 },
]

export function useHistoricalLogSearch(
  logStore: ReturnType<typeof useLogStore>,
  selectedTarget: Ref<LogTarget>,
  searchKeyword: Ref<string>,
  selectedLevel: Ref<string>
) {
  const mode = ref<'live' | 'historical'>('live')
  const selectedTimeRange = ref('1h')
  const queryError = ref<string | null>(null)
  const isSearching = ref(false)
  const currentOffset = ref(0)
  const selectedHistoricalLimit = ref(1000)

  const activeServiceSubFilter = ref<string>('')
  const sourceCategory = ref<'all' | 'app' | 'systemd'>('all')
  const mutedServices = ref<string[]>([])
  const hoverPauseEnabled = ref<boolean>(true)
  const isHoverPaused = ref<boolean>(false)

  watch(
    () => selectedTarget.value,
    (t) => {
      activeServiceSubFilter.value = t?.service || ''
    },
    { immediate: true }
  )

  const effectiveNode = computed(() => {
    const t = selectedTarget.value
    return t.node || (t.type === 'node' ? t.id : undefined)
  })

  const effectiveService = computed(() => {
    const t = selectedTarget.value
    return activeServiceSubFilter.value || t.service || (t.type === 'service' ? t.id : undefined)
  })

  async function runHistoricalQuery(isLoadMore = false) {
    if (mode.value !== 'historical') return
    queryError.value = null
    if (!isLoadMore) currentOffset.value = 0
    const cfg = LOG_TIME_RANGES.find((r) => r.label === selectedTimeRange.value) || LOG_TIME_RANGES[1]
    const now = new Date()
    const kw = searchKeyword.value.trim()
    const effNode = effectiveNode.value
    const effService = effectiveService.value

    const filter: LogFilterParams = {
      start_time: new Date(now.getTime() - cfg.ms).toISOString(),
      end_time: now.toISOString(),
      query: kw ? kw : undefined,
      log_level: (selectedLevel.value && selectedLevel.value !== 'ALL') ? selectedLevel.value : undefined,
      limit: selectedHistoricalLimit.value,
      offset: currentOffset.value,
      service: effService,
      container_name: effService,
      node: effNode,
      attributes: effNode ? { node: effNode } : undefined,
    }

    try {
      isSearching.value = true
      const promises: [Promise<unknown>, Promise<unknown>?] = [
        logStore.fetchHistoricalLogs(filter, isLoadMore),
      ]
      if (!isLoadMore) {
        promises.push(logStore.fetchHistogram({
          start_time: filter.start_time,
          end_time: filter.end_time,
          query: filter.query,
          log_level: filter.log_level,
          interval_seconds: cfg.interval,
          service: effService,
          container_name: effService,
          node: effNode,
          attributes: effNode ? { node: effNode } : undefined,
        }))
      }
      await Promise.all(promises)
      if (mode.value !== 'historical') return
      if (isLoadMore) currentOffset.value += selectedHistoricalLimit.value
    } catch (err: unknown) {
      if (mode.value === 'historical') {
        queryError.value = err instanceof Error ? err.message : 'ClickHouse search failed'
      }
    } finally {
      isSearching.value = false
    }
  }

  function loadMoreHistorical() {
    currentOffset.value += selectedHistoricalLimit.value
    runHistoricalQuery(true)
  }

  async function handleHistogramFilterRange(range: { start: string; end: string }) {
    if (mode.value === 'live') mode.value = 'historical'
    const kw = searchKeyword.value.trim()
    const effNode = effectiveNode.value
    const effService = effectiveService.value
    currentOffset.value = 0
    const filter: LogFilterParams = {
      start_time: range.start,
      end_time: range.end,
      query: kw ? kw : undefined,
      log_level: (selectedLevel.value && selectedLevel.value !== 'ALL') ? selectedLevel.value : undefined,
      limit: selectedHistoricalLimit.value,
      offset: 0,
      service: effService,
      container_name: effService,
      node: effNode,
      attributes: effNode ? { node: effNode } : undefined,
    }
    try {
      isSearching.value = true
      queryError.value = null
      await logStore.fetchHistoricalLogs(filter)
    } catch (err: unknown) {
      queryError.value = err instanceof Error ? err.message : 'Historical search failed'
    } finally {
      isSearching.value = false
    }
  }

  function handleClearHistogramFilter() {
    runHistoricalQuery()
  }

  const targetFilteredLogs = computed<LogEntry[]>(() => {
    const effNode = effectiveNode.value
    const effService = effectiveService.value
    const level = selectedLevel.value
    const rawKw = searchKeyword.value.trim()
    let reg: RegExp | null = null
    if (rawKw) {
      try { reg = new RegExp(rawKw, 'i') } catch { reg = null }
    }
    const kw = rawKw.toLowerCase()
    const mutedLower = mutedServices.value.map((s) => s.toLowerCase())
    const cat = sourceCategory.value

    return logStore.logs.filter((log) => {
      // 1. Filter out empty logs, '-- No entries --'
      if (!log.msg || log.msg === '-- No entries --' || log.msg.trim() === '-- No entries --') return false

      const svc = log.service || log.container || log.attributes?.app || log.attributes?.service || log.attributes?.container_name || log.pod || ''

      // 1b. Noise filtering (init.scope, session-*.scope, etc.)
      if (svc && isNoiseLog(svc)) return false

      // 2. Muted services exclusion
      if (svc && mutedLower.includes(svc.toLowerCase())) return false

      // 3. Category filtering (Apps Only vs SystemD Only)
      if (cat === 'app' && isSystemdService(svc)) return false
      if (cat === 'systemd' && !isSystemdService(svc)) return false

      // 4. Effective Node matching
      if (effNode) {
        const nodeName = (log.node || log.attributes?.node || log.attributes?.node_name || '').toLowerCase()
        if (nodeName && !nodeName.includes(effNode.toLowerCase())) return false
      }

      // 5. Effective Service matching
      if (effService) {
        if (!matchesTarget(log, { service: effService })) return false
      }

      // 6. Level matching
      if (level && level !== 'ALL') {
        const l = log.level.toUpperCase()
        const f = level.toUpperCase()
        const match = (f === 'ERR' || f === 'ERROR') ? (l === 'ERROR' || l === 'ERR')
          : (f === 'WARN' || f === 'WARNING') ? (l === 'WARN' || l === 'WARNING')
          : l === f
        if (!match) return false
      }

      // 7. Search keyword matching
      if (rawKw) {
        if (reg) {
          if (!reg.test(log.msg) && !reg.test(log.pod) && !(log.traceId && reg.test(log.traceId))) return false
        } else {
          if (!log.msg.toLowerCase().includes(kw) && !log.pod.toLowerCase().includes(kw) && !log.traceId?.toLowerCase().includes(kw)) return false
        }
      }

      return true
    })
  })

  return {
    mode,
    selectedTimeRange,
    queryError,
    isSearching,
    currentOffset,
    selectedHistoricalLimit,
    activeServiceSubFilter,
    sourceCategory,
    mutedServices,
    hoverPauseEnabled,
    isHoverPaused,
    effectiveNode,
    effectiveService,
    timeRanges: LOG_TIME_RANGES,
    runHistoricalQuery,
    loadMoreHistorical,
    handleHistogramFilterRange,
    handleClearHistogramFilter,
    targetFilteredLogs,
  }
}