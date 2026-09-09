import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { k8sApi, type K8sEvent } from '../api/k8s'

export interface EventsTimelineProps {
  cluster: string
  namespace?: string
  kind?: string
  name?: string
  autoRefresh?: boolean
}

export function useEventsTimeline(props: EventsTimelineProps) {
  const events = ref<K8sEvent[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const selectedTypeFilter = ref<'all' | 'Warning' | 'Normal'>('all')
  const searchQuery = ref('')
  const isAutoRefreshActive = ref(props.autoRefresh ?? false)
  let refreshIntervalTimer: ReturnType<typeof setInterval> | null = null

  async function loadEvents(silent = false) {
    if (!props.cluster) return
    if (!silent) loading.value = true
    error.value = null

    try {
      const list = await k8sApi.listEvents(props.cluster, {
        namespace: props.namespace && props.namespace !== 'all' ? props.namespace : undefined,
        kind: props.kind,
        name: props.name,
        limit: 150,
      })

      const sorted = (Array.isArray(list) ? list : []).sort((a, b) => {
        const timeA = new Date(a.lastTimestamp || a.eventTime || a.firstTimestamp || a.metadata?.creationTimestamp || 0).getTime()
        const timeB = new Date(b.lastTimestamp || b.eventTime || b.firstTimestamp || b.metadata?.creationTimestamp || 0).getTime()
        return timeB - timeA
      })

      events.value = sorted
    } catch (err: unknown) {
      events.value = []
      error.value = err instanceof Error ? err.message : 'Failed to query Kubernetes cluster events'
    } finally {
      if (!silent) loading.value = false
    }
  }

  function startAutoRefresh() {
    stopAutoRefresh()
    if (isAutoRefreshActive.value) {
      refreshIntervalTimer = setInterval(() => {
        loadEvents(true)
      }, 5000)
    }
  }

  function stopAutoRefresh() {
    if (refreshIntervalTimer) {
      clearInterval(refreshIntervalTimer)
      refreshIntervalTimer = null
    }
  }

  function toggleAutoRefresh() {
    isAutoRefreshActive.value = !isAutoRefreshActive.value
    if (isAutoRefreshActive.value) {
      startAutoRefresh()
    } else {
      stopAutoRefresh()
    }
  }

  watch(
    () => [props.cluster, props.namespace, props.kind, props.name],
    () => {
      loadEvents()
    }
  )

  watch(
    () => props.autoRefresh,
    (val) => {
      isAutoRefreshActive.value = val ?? false
      if (val) startAutoRefresh()
      else stopAutoRefresh()
    }
  )

  onMounted(() => {
    loadEvents()
    if (isAutoRefreshActive.value) {
      startAutoRefresh()
    }
  })

  onUnmounted(() => {
    stopAutoRefresh()
  })

  const filteredEvents = computed(() => {
    return events.value.filter((ev) => {
      if (selectedTypeFilter.value !== 'all') {
        const evType = ev.type || 'Normal'
        if (evType !== selectedTypeFilter.value) return false
      }

      if (searchQuery.value.trim()) {
        const q = searchQuery.value.toLowerCase().trim()
        const reason = (ev.reason || '').toLowerCase()
        const message = (ev.message || '').toLowerCase()
        const objKind = (ev.involvedObject?.kind || '').toLowerCase()
        const objName = (ev.involvedObject?.name || '').toLowerCase()
        const component = (ev.source?.component || '').toLowerCase()

        const match =
          reason.includes(q) ||
          message.includes(q) ||
          objKind.includes(q) ||
          objName.includes(q) ||
          component.includes(q)

        if (!match) return false
      }

      return true
    })
  })

  const warningCount = computed(() => events.value.filter((e) => e.type === 'Warning').length)
  const normalCount = computed(() => events.value.filter((e) => e.type !== 'Warning').length)

  function formatTimeAgo(dateStr?: string): string {
    if (!dateStr) return 'Unknown'
    const diff = Date.now() - new Date(dateStr).getTime()
    if (isNaN(diff) || diff < 0) return 'Just now'
    const secs = Math.floor(diff / 1000)
    if (secs < 60) return secs + 's ago'
    const mins = Math.floor(secs / 60)
    if (mins < 60) return mins + 'm ago'
    const hours = Math.floor(mins / 60)
    if (hours < 24) return hours + 'h ago'
    const days = Math.floor(hours / 24)
    return days + 'd ago'
  }

  function getEventTime(ev: K8sEvent): string {
    return formatTimeAgo(ev.lastTimestamp || ev.eventTime || ev.firstTimestamp || ev.metadata?.creationTimestamp)
  }

  return {
    events,
    loading,
    error,
    selectedTypeFilter,
    searchQuery,
    isAutoRefreshActive,
    filteredEvents,
    warningCount,
    normalCount,
    loadEvents,
    toggleAutoRefresh,
    getEventTime,
  }
}
