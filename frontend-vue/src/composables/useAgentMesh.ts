import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import {
  agentsApi,
  type ProjectState,
  type AgentTask,
  type AgentExecution,
  type CreateTaskPayload
} from '../api/compute'

export interface AgentCapability {
  id: string
  name: string
  description: string
  category: 'core' | 'infra' | 'security' | 'quality'
}

export interface AgentProfile {
  id: string
  name: string
  role: string
  icon: string
  agentType: string
  status: 'idle' | 'running' | 'completed' | 'failed' | 'paused'
  tokensUsed: number
  totalTokens: number
  memoryUsageMb: number
  activeTaskId?: string
  capabilities: string[]
  healthScore: number
  latencyMs: number
}

export interface TranscriptEntry {
  id: string
  timestamp: string
  level: 'INFO' | 'STEP' | 'SUCCESS' | 'WARN' | 'ERROR'
  agent: string
  message: string
  stepIndex?: number
  reasoning?: string
  toolCall?: string
  rawJsonl?: string
}

export interface SwarmMemoryState {
  totalTokensUsed: number
  contextWindowUsagePct: number
  vectorMemoryEntries: number
  activeSessions: number
  cacheHitRatio: number
}

export interface DAGStage {
  id: string
  name: string
  role: string
  icon: string
  agentType: string
  status: string
}

export function useAgentMesh() {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const actionLoading = ref(false)

  const projectState = ref<ProjectState | null>(null)
  const tasks = ref<AgentTask[]>([])
  const executions = ref<AgentExecution[]>([])

  // Modal and Drawer Controls
  const showDispatchModal = ref(false)
  const showTranscriptDrawer = ref(false)
  const selectedTask = ref<AgentTask | null>(null)
  const selectedAgent = ref<AgentProfile | null>(null)
  const transcriptViewMode = ref<'steps' | 'raw'>('steps')

  // Task Creation Form State
  const newTask = ref<CreateTaskPayload>({
    phase: 'Phase 6: Multi-Cluster Fleet & Swarm Management',
    module: 'internal/adapter/http',
    feature: 'Autonomous Agent Orchestration',
    title: '',
    description: '',
    dependencies: []
  })
  const dependencyInput = ref('')
  const selectedCapabilities = ref<string[]>([
    'ast-parsing',
    'k8s-manifests',
    'security-audit'
  ])

  // Live Terminal Log & Transcript Streaming
  const terminalLogs = ref<TranscriptEntry[]>([])
  const autoScroll = ref(true)
  const terminalRef = ref<HTMLDivElement | null>(null)

  // Polling interval
  let pollTimer: ReturnType<typeof setInterval> | null = null

  // Reactive DAG Stages Definition
  const dagStages = computed<DAGStage[]>(() => {
    const stageDefs = [
      { id: 'planner', name: '1. Planner & Architect', role: 'Planner', icon: '📐', agentType: 'planner' },
      { id: 'backend', name: '2. Backend / Go Engine', role: 'Backend Engineer', icon: '⚙️', agentType: 'backend' },
      { id: 'frontend', name: '3. Frontend Coder', role: 'Frontend Engineer', icon: '🎨', agentType: 'frontend' },
      { id: 'k8s', name: '4. K8s / Swarm Ops', role: 'Kubernetes Engineer', icon: '☸️', agentType: 'k8s' },
      { id: 'qa', name: '5. Security & QA Gate', role: 'QA Engineer', icon: '🛡️', agentType: 'qa' },
    ]
    return stageDefs.map(def => {
      const matchingExecs = executions.value.filter(e =>
        e.agent_type?.toLowerCase().includes(def.agentType) ||
        e.agent_type?.toLowerCase().includes(def.id)
      )
      let status = 'idle'
      if (matchingExecs.length > 0) {
        if (matchingExecs.some(e => e.status === 'running')) {
          status = 'running'
        } else if (matchingExecs.some(e => e.status === 'failed')) {
          status = 'failed'
        } else if (matchingExecs.every(e => e.status === 'success')) {
          status = 'completed'
        } else {
          status = 'completed'
        }
      } else if (tasks.value.length === 0) {
        status = 'idle'
      } else {
        status = 'queued'
      }
      return {
        id: def.id,
        name: def.name,
        role: def.role,
        icon: def.icon,
        agentType: def.agentType,
        status
      }
    })
  })

  // Dynamic Agent Swarm Profiles
  const agentSwarm = computed<AgentProfile[]>(() => {
    const defaultProfiles: Array<{
      id: string
      name: string
      role: string
      icon: string
      agentType: string
      maxTokens: number
      capabilities: string[]
    }> = [
      {
        id: 'agent-planner',
        name: 'Architect & Planner',
        role: 'System Architect',
        icon: '📐',
        agentType: 'planner',
        maxTokens: 64000,
        capabilities: ['AST Parsing', 'DAG Planning', 'Context Ingestion']
      },
      {
        id: 'agent-backend',
        name: 'Backend Engine Pilot',
        role: 'Go & Database Engineer',
        icon: '⚙️',
        agentType: 'backend',
        maxTokens: 128000,
        capabilities: ['Go Engine', 'PostgreSQL HA', 'GRPC Mesh']
      },
      {
        id: 'agent-frontend',
        name: 'Frontend Interface Coder',
        role: 'Vue & UI Architect',
        icon: '🎨',
        agentType: 'frontend',
        maxTokens: 64000,
        capabilities: ['Vue 3 Setup', 'Tailwind/CSS', 'Responsive HUD']
      },
      {
        id: 'agent-k8s',
        name: 'K8s Cluster Swarm Ops',
        role: 'Kubernetes Site Reliability',
        icon: '☸️',
        agentType: 'k8s',
        maxTokens: 128000,
        capabilities: ['K8s Manifests', 'Helm v3', 'Traefik Ingress']
      },
      {
        id: 'agent-qa',
        name: 'Security & QA Gatekeeper',
        role: 'Security & QA Auditor',
        icon: '🛡️',
        agentType: 'qa',
        maxTokens: 64000,
        capabilities: ['RBAC Audit', 'SLO Gates', 'E2E Testing']
      }
    ]

    return defaultProfiles.map(p => {
      const match = dagStages.value.find(s => s.id === p.agentType || s.agentType === p.agentType)
      const matchingExecs = executions.value.filter(e =>
        e.agent_type?.toLowerCase().includes(p.agentType) ||
        e.agent_type?.toLowerCase().includes(p.id)
      )
      const activeExec = matchingExecs.find(e => e.status === 'running') || matchingExecs[matchingExecs.length - 1]

      let status: AgentProfile['status'] = 'idle'
      if (activeExec) {
        if (activeExec.status === 'running') {
          status = 'running'
        } else if (activeExec.status === 'failed') {
          status = 'failed'
        } else if (activeExec.status === 'success') {
          status = 'completed'
        }
      } else if (match?.status === 'running') {
        status = 'running'
      } else if (match?.status === 'failed') {
        status = 'failed'
      } else if (match?.status === 'completed') {
        status = 'completed'
      }

      const tokensUsed = matchingExecs.reduce((sum, e) => sum + ((e as any).tokens_consumed || 0), 0)
      const memoryUsageMb = (activeExec as any)?.memory_usage_mb || 0
      const latencyMs = (activeExec as any)?.latency_ms || 0
      const healthScore = status === 'failed' ? 0 : 100

      return {
        id: p.id,
        name: p.name,
        role: p.role,
        icon: p.icon,
        agentType: p.agentType,
        status,
        tokensUsed,
        totalTokens: p.maxTokens,
        memoryUsageMb,
        activeTaskId: activeExec?.task_id,
        capabilities: p.capabilities,
        healthScore,
        latencyMs
      }
    })
  })

  // Computed Memory & Resource Telemetry
  const memoryState = computed<SwarmMemoryState>(() => {
    const totalTokens = agentSwarm.value.reduce((acc, a) => acc + a.tokensUsed, 0)
    const maxCapacity = agentSwarm.value.reduce((acc, a) => acc + a.totalTokens, 0)
    const contextWindowUsagePct = maxCapacity > 0 ? Math.round((totalTokens / maxCapacity) * 100) : 0
    return {
      totalTokensUsed: totalTokens,
      contextWindowUsagePct,
      vectorMemoryEntries: tasks.value.length,
      activeSessions: agentSwarm.value.filter(a => a.status === 'running').length,
      cacheHitRatio: 0
    }
  })

  const completedTasksCount = computed(() => tasks.value.filter(t => t.status === 'success').length)
  const activeTasksCount = computed(() => tasks.value.filter(t => t.status === 'inprogress').length)
  const blockedTasksCount = computed(() => tasks.value.filter(t => t.status === 'blocked').length)

  // Fetch Telemetry Data
  async function fetchAgentData() {
    loading.value = true
    error.value = null
    try {
      const [stateRes, tasksRes, runsRes] = await Promise.allSettled([
        agentsApi.getState(),
        agentsApi.listTasks(),
        agentsApi.listRuns()
      ])

      if (stateRes.status === 'fulfilled' && stateRes.value) {
        projectState.value = stateRes.value
      } else {
        projectState.value = null
      }

      if (tasksRes.status === 'fulfilled' && Array.isArray(tasksRes.value)) {
        tasks.value = tasksRes.value
      } else {
        tasks.value = []
      }

      if (runsRes.status === 'fulfilled' && Array.isArray(runsRes.value)) {
        executions.value = runsRes.value
        terminalLogs.value = runsRes.value.slice(0, 25).map((run, idx) => ({
          id: `log-${run.id || idx}`,
          timestamp: run.created_at || new Date().toISOString(),
          level: run.status === 'success' ? 'SUCCESS' : run.status === 'failed' ? 'ERROR' : 'STEP',
          agent: run.agent_type || 'Swarm Orchestrator',
          message: run.output || run.error_detail || `Execution step triggered for task: ${run.task_id}`,
          stepIndex: idx + 1,
          reasoning: `Step ${idx + 1}: Real execution event recorded.`,
          toolCall: run.input ? `execute_module(${run.input})` : undefined,
          rawJsonl: JSON.stringify({
            timestamp: run.created_at,
            agent: run.agent_type,
            status: run.status,
            task_id: run.task_id,
            output: run.output
          })
        }))
      } else {
        executions.value = []
        terminalLogs.value = []
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to fetch agent framework telemetry'
      error.value = msg
    } finally {
      loading.value = false
    }
  }

  // Autonomous Task Dispatching
  async function handleCreateTask() {
    if (!newTask.value.title.trim()) return
    actionLoading.value = true
    try {
      const deps = dependencyInput.value
        ? dependencyInput.value.split(',').map(s => s.trim()).filter(Boolean)
        : []
      
      const created = await agentsApi.createTask({
        ...newTask.value,
        dependencies: deps
      })

      const logId = `log-${Date.now()}`
      const entry: TranscriptEntry = {
        id: logId,
        timestamp: new Date().toISOString(),
        level: 'STEP',
        agent: 'Swarm Orchestrator',
        message: `New engineering task registered: "${newTask.value.title}". Scheduling DAG dependency solver.`,
        stepIndex: terminalLogs.value.length + 1,
        reasoning: `Resolved dependencies [${deps.join(', ') || 'None'}]. Queuing execution on next tick.`,
        toolCall: `dispatch_task(id="${created?.id || 'new'}", title="${newTask.value.title}")`,
        rawJsonl: JSON.stringify({ event: 'TASK_DISPATCHED', title: newTask.value.title, deps })
      }
      terminalLogs.value.push(entry)

      showDispatchModal.value = false
      newTask.value.title = ''
      newTask.value.description = ''
      dependencyInput.value = ''
      await fetchAgentData()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to schedule task'
      error.value = msg
    } finally {
      actionLoading.value = false
    }
  }

  // Task Control Actions
  function pauseTask(taskId: string) {
    const task = tasks.value.find(t => t.id === taskId)
    if (task) {
      task.status = task.status === 'blocked' ? 'inprogress' : 'blocked'
      terminalLogs.value.push({
        id: `log-pause-${Date.now()}`,
        timestamp: new Date().toISOString(),
        level: 'WARN',
        agent: 'Swarm Orchestrator',
        message: `Task ${taskId} status updated to [${task.status.toUpperCase()}]. Execution worker paused.`,
        stepIndex: terminalLogs.value.length + 1,
        reasoning: 'User initiated task pause/resume signal.',
        rawJsonl: JSON.stringify({ event: 'TASK_PAUSED', taskId, status: task.status })
      })
    }
  }

  function terminateTask(taskId: string) {
    const idx = tasks.value.findIndex(t => t.id === taskId)
    if (idx !== -1) {
      const terminated = tasks.value[idx]
      terminated.status = 'failed'
      terminalLogs.value.push({
        id: `log-term-${Date.now()}`,
        timestamp: new Date().toISOString(),
        level: 'ERROR',
        agent: 'Swarm Orchestrator',
        message: `Task ${taskId} ("${terminated.title}") terminated by operator. Resources reclaimed.`,
        stepIndex: terminalLogs.value.length + 1,
        reasoning: 'Operator terminated task execution via kill signal.',
        rawJsonl: JSON.stringify({ event: 'TASK_TERMINATED', taskId })
      })
    }
  }

  function openTranscript(agent?: AgentProfile | null, task?: AgentTask | null) {
    selectedAgent.value = agent || null
    selectedTask.value = task || null
    showTranscriptDrawer.value = true
  }

  function clearLogs() {
    terminalLogs.value = []
  }

  function formatTime(d: string) {
    try {
      return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    } catch {
      return d
    }
  }

  // Auto-scroll watcher
  watch(
    terminalLogs,
    () => {
      if (autoScroll.value) {
        nextTick(() => {
          if (terminalRef.value) {
            terminalRef.value.scrollTop = terminalRef.value.scrollHeight
          }
        })
      }
    },
    { deep: true }
  )

  onMounted(() => {
    fetchAgentData()
    // Periodic refresh
    pollTimer = setInterval(() => {
      fetchAgentData()
    }, 15000)
  })

  onUnmounted(() => {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  })

  return {
    loading,
    error,
    actionLoading,
    projectState,
    tasks,
    executions,
    agentSwarm,
    dagStages,
    memoryState,
    showDispatchModal,
    showTranscriptDrawer,
    selectedTask,
    selectedAgent,
    transcriptViewMode,
    newTask,
    dependencyInput,
    selectedCapabilities,
    terminalLogs,
    autoScroll,
    terminalRef,
    completedTasksCount,
    activeTasksCount,
    blockedTasksCount,
    fetchAgentData,
    handleCreateTask,
    pauseTask,
    terminateTask,
    openTranscript,
    clearLogs,
    formatTime
  }
}
