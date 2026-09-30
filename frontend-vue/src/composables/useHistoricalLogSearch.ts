import { ref, computed } from 'vue'
import type { Ref } from 'vue'
import type { LogTarget } from '../components/logs/LogTargetTree.vue'
import type { LogFilterParams } from '../api/logging'
import type { useLogStore, LogEntry } from '../stores/logStore'

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

  async function runHistoricalQuery(isLoadMore = false) {
    if (mode.value !== 'historical') return
    queryError.value = null
    if (!isLoadMore) currentOffset.value = 0
    const cfg = LOG_TIME_RANGES.find((r) => r.label === selectedTimeRange.value) || LOG_TIME_RANGES[1]
    const now = new Date()
    const kw = searchKeyword.value.trim()
    const target = selectedTarget.value

    const filter: LogFilterParams = {
      start_time: new Date(now.getTime() - cfg.ms).toISOString(),
      end_time: now.toISOString(),
      query: kw ? kw : undefined,
      log_level: (selectedLevel.value && selectedLevel.value !== 'ALL') ? selectedLevel.value : undefined,
      limit: selectedHistoricalLimit.value,
      offset: currentOffset.value,
      container_name: target.type === 'service' ? target.id : undefined,
      node: target.type === 'node' ? target.id : undefined,
      attributes: target.type === 'node' ? { node: target.id } : undefined,
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
          container_name: filter.container_name,
          node: target.type === 'node' ? target.id : undefined,
          attributes: target.type === 'node' ? { node: target.id } : undefined,
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
    const target = selectedTarget.value
    currentOffset.value = 0
    const filter: LogFilterParams = {
      start_time: range.start,
      end_time: range.end,
      query: kw ? kw : undefined,
      log_level: (selectedLevel.value && selectedLevel.value !== 'ALL') ? selectedLevel.value : undefined,
      limit: selectedHistoricalLimit.value,
      offset: 0,
      container_name: target.type === 'service' ? target.id : undefined,
      node: target.type === 'node' ? target.id : undefined,
      attributes: target.type === 'node' ? { node: target.id } : undefined,
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
    const target = selectedTarget.value
    const level = selectedLevel.value
    const rawKw = searchKeyword.value.trim()
    let reg: RegExp | null = null
    if (rawKw) {
      try { reg = new RegExp(rawKw, 'i') } catch { reg = null }
    }
    const kw = rawKw.toLowerCase()

    return logStore.logs.filter((log) => {
      if (!log.msg || log.msg === '-- No entries --' || log.msg.trim() === '-- No entries --') return false
      if (level && level !== 'ALL') {
        const l = log.level.toUpperCase()
        const f = level.toUpperCase()
        const match = (f === 'ERR' || f === 'ERROR') ? (l === 'ERROR' || l === 'ERR')
          : (f === 'WARN' || f === 'WARNING') ? (l === 'WARN' || l === 'WARNING')
          : l === f
        if (!match) return false
      }
      if (mode.value === 'live') {
        const q = target.id.toLowerCase()
        if (target.type === 'node') {
          const n = (log.node || log.attributes?.node || log.attributes?.node_name || log.pod || '').toLowerCase()
          if (!n.includes(q)) return false
        } else if (target.type === 'service') {
          const s = (log.service || log.container || log.attributes?.app || log.attributes?.service || log.attributes?.container_name || log.pod || '').toLowerCase()
          const matches = s.includes(q) ||
            (q === 'postgres_db' && (s.includes('db') || s.includes('postgres'))) ||
            (q === 'db' && s.includes('postgres'))
          if (!matches) return false
        }
      }
      if (rawKw && mode.value === 'live') {
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
    timeRanges: LOG_TIME_RANGES,
    runHistoricalQuery,
    loadMoreHistorical,
    handleHistogramFilterRange,
    handleClearHistogramFilter,
    targetFilteredLogs,
  }
}