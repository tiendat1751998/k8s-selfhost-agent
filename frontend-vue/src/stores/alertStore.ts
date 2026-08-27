import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { MetricAlert } from '../api/overview'

export interface MutedAlertConfig {
  key: string // e.g. "workerdb1-node_down"
  nodeId?: string
  nodeName?: string
  type: string
  mutedAt: number // timestamp
  snoozeMode: 'restart' | '1h' | '24h' | 'session' | 'forever'
  expiresAt?: number // timestamp if time-based
}

const MUTED_ALERTS_STORAGE_KEY = 'k8s_muted_alerts_v1'
const SESSION_MUTED_ALERTS_KEY = 'k8s_session_muted_alerts_v1'

export const useAlertStore = defineStore('alert', () => {
  // State
  const rawAlerts = ref<MetricAlert[]>([])
  const mutedAlertsMap = ref<Record<string, MutedAlertConfig>>({})
  const dismissedAlertKeys = ref<Set<string>>(new Set())
  const showAlertCenterModal = ref<boolean>(false)
  const isToastDropped = ref<boolean>(false)
  const manuallyDismissedKeys = ref<string>('')

  function getAlertKey(alert: MetricAlert): string {
    return `${alert.node_name || alert.node_id}-${alert.type}`
  }

  function isAlertMuted(alert: MetricAlert): boolean {
    const keyByName = `${alert.node_name || alert.node_id}-${alert.type}`
    const keyById = `${alert.node_id}-${alert.type}`
    const now = Date.now()

    const config = mutedAlertsMap.value[keyByName] || mutedAlertsMap.value[keyById]
    if (!config) return false
    if (config.expiresAt && now > config.expiresAt) {
      return false
    }
    return true
  }

  // Active alerts filtered against muting and dismissal
  const activeAlerts = computed<MetricAlert[]>(() => {
    return rawAlerts.value.filter(a => {
      const key = getAlertKey(a)
      const rawKey = `${a.node_id}-${a.type}`
      if (dismissedAlertKeys.value.has(key) || dismissedAlertKeys.value.has(rawKey)) {
        return false
      }
      if (isAlertMuted(a)) {
        return false
      }
      return true
    })
  })

  // List of active mute policies (filtering out expired ones)
  const mutedAlertsList = computed<MutedAlertConfig[]>(() => {
    const now = Date.now()
    return Object.values(mutedAlertsMap.value).filter(
      item => !item.expiresAt || now <= item.expiresAt
    )
  })

  const mutedAlertsCount = computed(() => mutedAlertsList.value.length)

  const hasCriticalAlerts = computed<boolean>(() => {
    return activeAlerts.value.some(a => a.value >= 90 || a.type === 'node_down')
  })

  const downNodeAlerts = computed(() => {
    return activeAlerts.value.filter(a => a.type === 'node_down')
  })

  const hasNodeDown = computed(() => downNodeAlerts.value.length > 0)

  // Load muted alerts from localStorage and sessionStorage
  function loadMutedAlerts() {
    try {
      const now = Date.now()
      const valid: Record<string, MutedAlertConfig> = {}
      let localChanged = false

      if (typeof localStorage !== 'undefined') {
        const rawLocal = localStorage.getItem(MUTED_ALERTS_STORAGE_KEY)
        if (rawLocal) {
          const parsedLocal = JSON.parse(rawLocal) as Record<string, MutedAlertConfig>
          for (const [key, item] of Object.entries(parsedLocal)) {
            if (item.expiresAt && now > item.expiresAt) {
              localChanged = true
              continue
            }
            valid[key] = item
          }
        }
      }

      if (typeof sessionStorage !== 'undefined') {
        const rawSession = sessionStorage.getItem(SESSION_MUTED_ALERTS_KEY)
        if (rawSession) {
          const parsedSession = JSON.parse(rawSession) as Record<string, MutedAlertConfig>
          for (const [key, item] of Object.entries(parsedSession)) {
            if (item.expiresAt && now > item.expiresAt) {
              continue
            }
            valid[key] = item
          }
        }
      }

      mutedAlertsMap.value = valid
      if (localChanged) {
        persistMutedAlerts()
      }
    } catch (e) {
      console.error('Failed to load muted alerts from storage:', e)
    }
  }

  // Persist muted alerts to storage
  function persistMutedAlerts() {
    try {
      const localEntries: Record<string, MutedAlertConfig> = {}
      const sessionEntries: Record<string, MutedAlertConfig> = {}

      for (const [key, item] of Object.entries(mutedAlertsMap.value)) {
        if (item.snoozeMode === 'session') {
          sessionEntries[key] = item
        } else {
          localEntries[key] = item
        }
      }

      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(MUTED_ALERTS_STORAGE_KEY, JSON.stringify(localEntries))
      }
      if (typeof sessionStorage !== 'undefined') {
        sessionStorage.setItem(SESSION_MUTED_ALERTS_KEY, JSON.stringify(sessionEntries))
      }
    } catch (e) {
      console.error('Failed to save muted alerts to storage:', e)
    }
  }

  // Sync incoming alerts from overview or WebSocket
  function syncAlerts(alerts: MetricAlert[]) {
    rawAlerts.value = alerts || []

    const currentKeys = activeAlerts.value
      .map(a => `${a.node_id || a.node_name}-${a.type}`)
      .sort()
      .join(',')

    if (currentKeys && currentKeys.length > 0) {
      if (currentKeys !== manuallyDismissedKeys.value) {
        isToastDropped.value = true
      }
    } else {
      isToastDropped.value = false
      manuallyDismissedKeys.value = ''
    }
  }

  function muteAlert(
    alert: MetricAlert,
    mode: 'restart' | '1h' | '24h' | 'session' | 'forever' = 'restart'
  ) {
    const key = getAlertKey(alert)
    const now = Date.now()
    let expiresAt: number | undefined

    if (mode === '1h') {
      expiresAt = now + 60 * 60 * 1000
    } else if (mode === '24h') {
      expiresAt = now + 24 * 60 * 60 * 1000
    }

    const config: MutedAlertConfig = {
      key,
      nodeId: alert.node_id,
      nodeName: alert.node_name,
      type: alert.type,
      mutedAt: now,
      snoozeMode: mode,
      expiresAt,
    }

    mutedAlertsMap.value = {
      ...mutedAlertsMap.value,
      [key]: config,
    }
    persistMutedAlerts()
  }

  function unmuteAlert(key: string) {
    const updated = { ...mutedAlertsMap.value }
    delete updated[key]
    mutedAlertsMap.value = updated
    persistMutedAlerts()
  }

  function muteAll(mode: 'restart' | '1h' | '24h' = 'restart') {
    const now = Date.now()
    let expiresAt: number | undefined

    if (mode === '1h') {
      expiresAt = now + 60 * 60 * 1000
    } else if (mode === '24h') {
      expiresAt = now + 24 * 60 * 60 * 1000
    }

    const newMap = { ...mutedAlertsMap.value }
    activeAlerts.value.forEach(alert => {
      const key = getAlertKey(alert)
      newMap[key] = {
        key,
        nodeId: alert.node_id,
        nodeName: alert.node_name,
        type: alert.type,
        mutedAt: now,
        snoozeMode: mode,
        expiresAt,
      }
    })

    mutedAlertsMap.value = newMap
    persistMutedAlerts()
    isToastDropped.value = false
  }

  function unmuteAll() {
    mutedAlertsMap.value = {}
    persistMutedAlerts()
  }

  function dismissAlert(alert: MetricAlert) {
    dismissedAlertKeys.value.add(`${alert.node_id}-${alert.type}`)
    dismissedAlertKeys.value.add(getAlertKey(alert))
  }

  function dismissToast() {
    isToastDropped.value = false
    const keys = activeAlerts.value
      .map(a => `${a.node_id || a.node_name}-${a.type}`)
      .sort()
      .join(',')
    manuallyDismissedKeys.value = keys
  }

  function openAlertCenter() {
    showAlertCenterModal.value = true
  }

  function closeAlertCenter() {
    showAlertCenterModal.value = false
  }

  // Initialize storage load
  loadMutedAlerts()

  return {
    rawAlerts,
    activeAlerts,
    mutedAlertsMap,
    mutedAlertsList,
    mutedAlertsCount,
    hasCriticalAlerts,
    downNodeAlerts,
    hasNodeDown,
    showAlertCenterModal,
    isToastDropped,
    manuallyDismissedKeys,
    getAlertKey,
    isAlertMuted,
    syncAlerts,
    loadMutedAlerts,
    persistMutedAlerts,
    muteAlert,
    unmuteAlert,
    muteAll,
    unmuteAll,
    dismissAlert,
    dismissToast,
    openAlertCenter,
    closeAlertCenter,
  }
})
