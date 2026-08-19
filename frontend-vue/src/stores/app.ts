import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import type { Incident } from '../api/compute'
import type { LogEntry } from './logStore'
import type { SystemOverview } from '../api/overview'

export interface AgentInfo {
  id: string
  name: string
  status: string
  type?: string
  last_active?: string
}

export interface IntegrationItem {
  id: string
  name: string
  type: string
  status: string
  endpoint?: string
}

export const useAppStore = defineStore('app', () => {
  const connection = ref('connecting')
  const incidents = ref<Incident[]>([])
  const agents = ref<AgentInfo[]>([])
  const logs = ref<LogEntry[]>([])
  const latestMetrics = ref<SystemOverview | null>(null)
  
  const stats = reactive({
    critical: 0,
    warning: 0,
    resolved: 0,
    agentRuns: 0
  })

  function setConnection(status: string) {
    connection.value = status
  }

  function addIncident(incident: Incident) {
    incidents.value.unshift(incident)
    if (incidents.value.length > 50) incidents.value.pop()
    if (incident.severity === 'critical') stats.critical++
    else if (incident.severity === 'medium' || incident.severity === 'high') stats.warning++
  }

  function addLog(log: LogEntry) {
    logs.value.push(log)
    if (logs.value.length > 200) logs.value.shift()
  }

  function setMetrics(overview: SystemOverview) {
    latestMetrics.value = overview
  }
  
  const kubernetes = ref<IntegrationItem[]>([])
  const gitProviders = ref<IntegrationItem[]>([])
  const cicd = ref<IntegrationItem[]>([])
  const aiProviders = ref<IntegrationItem[]>([])
  const connectionHealth = ref<Record<string, unknown>>({})
  
  return {
    connection, incidents, agents, logs, latestMetrics, stats,
    kubernetes, gitProviders, cicd, aiProviders, connectionHealth,
    setConnection, addIncident, addLog, setMetrics
  }
})

