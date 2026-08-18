import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'

export const useAppStore = defineStore('app', () => {
  const connection = ref('connecting')
  const incidents = ref<any[]>([])
  const agents = ref<any[]>([])
  const logs = ref<any[]>([])
  
  const stats = reactive({
    critical: 0,
    warning: 0,
    resolved: 0,
    agentRuns: 0
  })

  function setConnection(status: string) {
    connection.value = status
  }

  function addIncident(incident: any) {
    incidents.value.unshift(incident)
    if (incidents.value.length > 50) incidents.value.pop()
    if (incident.severity === 'critical') stats.critical++
    else if (incident.severity === 'warning') stats.warning++
  }

  function addLog(log: any) {
    logs.value.push(log)
    if (logs.value.length > 200) logs.value.shift()
  }
  
  const kubernetes = ref<any[]>([])
  const gitProviders = ref<any[]>([])
  const cicd = ref<any[]>([])
  const aiProviders = ref<any[]>([])
  const connectionHealth = ref<Record<string, any>>({})
  
  return {
    connection, incidents, agents, logs, stats,
    kubernetes, gitProviders, cicd, aiProviders, connectionHealth,
    setConnection, addIncident, addLog
  }
})
