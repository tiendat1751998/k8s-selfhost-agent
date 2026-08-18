<script setup lang="ts">
import { ref, onMounted, computed, nextTick, watch } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  agentsApi,
  type ProjectState,
  type AgentTask,
  type AgentExecution,
  type CreateTaskPayload
} from '../api/compute'

const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref(false)

const projectState = ref<ProjectState | null>(null)
const tasks = ref<AgentTask[]>([])
const executions = ref<AgentExecution[]>([])

// Create Task Modal
const showCreateModal = ref(false)
const newTask = ref<CreateTaskPayload>({
  phase: 'Phase 6: Multi-Cluster Fleet & Swarm Management',
  module: 'internal/adapter/http',
  feature: 'Autonomous Agent Orchestration',
  title: '',
  description: '',
  dependencies: []
})
const dependencyInput = ref('')

// Terminal Log Streaming Console
const terminalLogs = ref<Array<{ timestamp: string; level: 'INFO' | 'STEP' | 'SUCCESS' | 'WARN'; message: string; agent: string }>>([])
const autoScroll = ref(true)
const terminalRef = ref<HTMLDivElement | null>(null)

// Reactive DAG Stages definition based on actual task & execution state
const dagStages = computed(() => {
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
      status
    }
  })
})

async function fetchAgentData() {
  loading.value = true
  error.value = null
  try {
    const [stateRes, tasksRes, runsRes] = await Promise.allSettled([
      agentsApi.getState(),
      agentsApi.listTasks(),
      agentsApi.listRuns()
    ])

    if (stateRes.status === 'fulfilled') {
      projectState.value = stateRes.value
    }
    if (tasksRes.status === 'fulfilled') {
      tasks.value = tasksRes.value
    }
    if (runsRes.status === 'fulfilled') {
      executions.value = runsRes.value
      // Populate terminal logs from real execution records
      if (runsRes.value.length > 0) {
        terminalLogs.value = runsRes.value.slice(0, 20).map(run => ({
          timestamp: run.created_at,
          level: run.status === 'success' ? 'SUCCESS' : run.status === 'failed' ? 'WARN' : 'STEP',
          agent: run.agent_type || 'Agent',
          message: run.output || run.error_detail || `Execution step triggered for task: ${run.task_id}`
        }))
      }
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to fetch agent framework telemetry'
    error.value = msg
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchAgentData()
})

watch(terminalLogs, () => {
  if (autoScroll.value) {
    nextTick(() => {
      if (terminalRef.value) {
        terminalRef.value.scrollTop = terminalRef.value.scrollHeight
      }
    })
  }
}, { deep: true })

const completedTasksCount = computed(() => tasks.value.filter(t => t.status === 'success').length)
const activeTasksCount = computed(() => tasks.value.filter(t => t.status === 'inprogress').length)
const blockedTasksCount = computed(() => tasks.value.filter(t => t.status === 'blocked').length)

async function handleCreateTask() {
  if (!newTask.value.title.trim()) return
  actionLoading.value = true
  try {
    const deps = dependencyInput.value
      ? dependencyInput.value.split(',').map(s => s.trim()).filter(Boolean)
      : []
    
    await agentsApi.createTask({
      ...newTask.value,
      dependencies: deps
    })

    terminalLogs.value.push({
      timestamp: new Date().toISOString(),
      level: 'STEP',
      agent: 'Orchestrator',
      message: `New engineering task registered: "${newTask.value.title}". Scheduling DAG dependency solver.`
    })

    showCreateModal.value = false
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
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>SPECIALIZED MULTI-AGENT SWARM ORCHESTRATION</span>
        </div>
        <h1 class="view-title">Multi-Agent Pipeline & Task DAG</h1>
        <p class="view-desc">
          DAG task execution graph, specialized role delegators (Architect, GitOps, Kubernetes, QA), and live terminal step telemetry.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchAgentData">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Swarm' }}</span>
        </button>
        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>+ Dispatch Task</span>
        </button>
      </div>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="error-banner">
      <span class="error-icon">⚠️</span>
      <span>{{ error }}</span>
    </div>

    <!-- Metrics HUD -->
    <div class="metrics-grid">
      <MetricCard
        title="Architecture Score"
        :value="projectState?.architecture_score !== undefined && projectState.architecture_score !== null ? `${Math.round(projectState.architecture_score * 100)}%` : '—'"
        :subtitle="projectState?.architecture_score !== undefined && projectState.architecture_score !== null ? 'Zero cyclic dependencies & strict layering' : 'No architecture telemetry recorded'"
        icon="🏛️"
        :badge="projectState?.architecture_score !== undefined && projectState.architecture_score !== null ? 'SCORE' : 'NO DATA'"
        :badge-color="projectState?.architecture_score !== undefined && projectState.architecture_score !== null ? 'cyan' : 'muted'"
        :trend="projectState?.architecture_score !== undefined && projectState.architecture_score !== null ? 'High Cohesion' : 'Uncalculated'"
        :trend-type="projectState?.architecture_score !== undefined && projectState.architecture_score !== null ? 'positive' : 'neutral'"
      />
      <MetricCard
        title="Repository Health"
        :value="projectState?.repository_health !== undefined && projectState.repository_health !== null ? `${Math.round(projectState.repository_health * 100)}%` : '—'"
        :subtitle="projectState?.repository_health !== undefined && projectState.repository_health !== null ? 'Full type safety & lint compliance' : 'No health telemetry recorded'"
        icon="🛡️"
        :badge="projectState?.repository_health !== undefined && projectState.repository_health !== null ? 'HEALTH' : 'NO DATA'"
        :badge-color="projectState?.repository_health !== undefined && projectState.repository_health !== null ? 'emerald' : 'muted'"
        :trend="projectState?.repository_health !== undefined && projectState.repository_health !== null ? 'Continuous Clean' : 'Uncalculated'"
        :trend-type="projectState?.repository_health !== undefined && projectState.repository_health !== null ? 'positive' : 'neutral'"
      />
      <MetricCard
        title="Task Backlog"
        :value="tasks.length > 0 ? `${completedTasksCount}/${tasks.length}` : '0/0'"
        :subtitle="tasks.length > 0 ? `${activeTasksCount} in progress · ${blockedTasksCount} blocked` : 'No tasks in backlog'"
        icon="📋"
        badge="PIPELINE"
        :badge-color="tasks.length > 0 ? 'violet' : 'muted'"
        :trend="tasks.length > 0 ? 'DAG Scheduled' : 'Queue Empty'"
        trend-type="neutral"
      />
      <MetricCard
        title="Swarm Executions"
        :value="executions.length"
        :subtitle="executions.length > 0 ? 'Total autonomous agent runs executed' : 'No agent runs recorded'"
        icon="⚡"
        badge="RUNS"
        :badge-color="executions.length > 0 ? 'emerald' : 'muted'"
        :trend="executions.length > 0 ? 'Real-Time Step Logs' : 'Idle'"
        :trend-type="executions.length > 0 ? 'positive' : 'neutral'"
      />
    </div>

    <!-- Pipeline DAG Execution Visualizer -->
    <div class="section-box glass-panel dag-section">
      <div class="box-header">
        <div>
          <h2 class="box-title">Multi-Agent Pipeline DAG Execution Flow</h2>
          <p class="box-subtitle">Specialized role handover pipeline: Requirements ➔ Code ➔ K8s Manifests ➔ Security Verification</p>
        </div>
        <div class="dag-legend font-mono">
          <span class="legend-item"><span class="dot-emerald"></span> Verified</span>
          <span class="legend-item"><span class="dot-amber"></span> In Progress</span>
          <span class="legend-item"><span class="dot-muted"></span> Queued</span>
        </div>
      </div>

      <div class="dag-pipeline-flow">
        <div 
          v-for="(stage, idx) in dagStages" 
          :key="stage.id" 
          class="dag-node-item"
        >
          <div class="dag-node-card glass-panel" :class="`node-state-${stage.status}`">
            <div class="node-icon">{{ stage.icon }}</div>
            <div class="node-body">
              <span class="node-name">{{ stage.name }}</span>
              <span class="node-role font-mono text-muted">{{ stage.role }}</span>
            </div>
            <StatusBadge :status="stage.status" size="sm" />
          </div>

          <!-- Connector arrow between stages -->
          <div v-if="idx < dagStages.length - 1" class="dag-connector">
            <div class="connector-line" :class="{ 'connector-active': stage.status === 'completed' }"></div>
            <span class="connector-arrow">➔</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 2-Column Split: Task Dependency Backlog & Live Terminal Console -->
    <div class="agents-split-layout">
      <!-- Left: Task Dependency Backlog -->
      <div class="section-box glass-panel task-backlog-box">
        <div class="box-header">
          <div>
            <h2 class="box-title">Task Dependency Backlog</h2>
            <p class="box-subtitle">Queued architectural units with parent dependency constraints</p>
          </div>
          <button class="btn btn-primary btn-xs" @click="showCreateModal = true">+ New Task</button>
        </div>

        <div v-if="tasks.length === 0" class="empty-state">
          <span>No tasks in backlog. Dispatch an autonomous engineering task to begin.</span>
        </div>

        <div v-else class="task-items-list">
          <div 
            v-for="task in tasks" 
            :key="task.id" 
            class="task-card-row glass-panel"
            :class="`task-edge-${task.status}`"
          >
            <div class="task-row-top">
              <span class="task-title-text">{{ task.title }}</span>
              <StatusBadge :status="task.status" size="sm" />
            </div>

            <p class="task-desc-text">{{ task.description }}</p>

            <div class="task-row-meta font-mono">
              <span class="module-chip">{{ task.module }}</span>
              <span class="feature-chip">{{ task.feature }}</span>
            </div>

            <!-- Dependencies -->
            <div v-if="task.dependencies && task.dependencies.length > 0" class="deps-container">
              <span class="deps-lbl font-mono">Depends on:</span>
              <div class="deps-chips font-mono">
                <span v-for="dep in task.dependencies" :key="dep" class="dep-chip">
                  ⛓️ {{ dep.slice(0, 8) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Right: Live Dark Terminal Log Console -->
      <div class="section-box glass-panel terminal-console-box">
        <div class="terminal-header">
          <div class="terminal-title-group">
            <span class="terminal-dots">
              <span class="t-dot t-red"></span>
              <span class="t-dot t-yellow"></span>
              <span class="t-dot t-green"></span>
            </span>
            <span class="terminal-title font-mono">swarm-orchestrator.log · Live Step Console</span>
          </div>

          <div class="terminal-actions">
            <label class="autoscroll-toggle font-mono">
              <input v-model="autoScroll" type="checkbox" />
              <span>Auto-Scroll</span>
            </label>
            <button class="btn btn-secondary btn-xs font-mono" @click="clearLogs">Clear</button>
          </div>
        </div>

        <!-- Terminal Output Window -->
        <div ref="terminalRef" class="terminal-screen font-mono">
          <div class="terminal-welcome">
            --- K8S MULTI-AGENT SWARM TELEMETRY BUS CONNECTED [SESSION OK] ---
          </div>

          <div 
            v-for="(log, idx) in terminalLogs" 
            :key="idx" 
            class="terminal-log-line"
          >
            <span class="log-ts text-muted">[{{ formatTime(log.timestamp) }}]</span>
            <span class="log-level" :class="`lvl-${log.level.toLowerCase()}`">[{{ log.level }}]</span>
            <span class="log-agent text-cyan">&lt;{{ log.agent }}&gt;</span>
            <span class="log-msg">{{ log.message }}</span>
          </div>

          <div class="terminal-prompt">
            <span class="prompt-arrow">k8s-agent-swarm:~$</span>
            <span class="blinking-cursor">█</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Task Modal -->
    <ModalDrawer
      v-model:show="showCreateModal"
      mode="modal"
      title="Dispatch Swarm Engineering Task"
      subtitle="Define task specifications, module scope, and dependency prerequisites"
      max-width="580px"
    >
      <div class="modal-form">
        <div class="form-group">
          <label class="form-label">Task Title</label>
          <input v-model="newTask.title" type="text" placeholder="e.g. Implement Multi-Cluster Gateway" class="input-glass" />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Phase</label>
            <input v-model="newTask.phase" type="text" class="input-glass" />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Module</label>
            <input v-model="newTask.module" type="text" class="input-glass font-mono" />
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Feature Scope</label>
          <input v-model="newTask.feature" type="text" class="input-glass" />
        </div>

        <div class="form-group">
          <label class="form-label">Task Description</label>
          <textarea v-model="newTask.description" rows="3" class="input-glass font-mono" placeholder="Architectural acceptance criteria..."></textarea>
        </div>

        <div class="form-group">
          <label class="form-label">Parent Dependencies (comma separated IDs)</label>
          <input v-model="dependencyInput" type="text" placeholder="task-id-1, task-id-2" class="input-glass font-mono" />
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="actionLoading" @click="handleCreateTask">
          <span>{{ actionLoading ? 'Scheduling...' : 'Dispatch Task ➔' }}</span>
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.08em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 750px;
  margin-top: 4px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.error-banner {
  padding: 12px 16px;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  border-radius: 12px;
  color: #fb7185;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.section-box {
  padding: 20px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.box-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.box-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.box-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

/* DAG Pipeline Visualizer */
.dag-section {
  background: rgba(11, 15, 25, 0.7);
}

.dag-legend {
  display: flex;
  gap: 14px;
  font-size: 11px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-muted);
}

.dot-emerald { width: 6px; height: 6px; border-radius: 50%; background: #10b981; }
.dot-amber { width: 6px; height: 6px; border-radius: 50%; background: #f59e0b; }
.dot-muted { width: 6px; height: 6px; border-radius: 50%; background: #64748b; }

.dag-pipeline-flow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  overflow-x: auto;
  padding: 10px 0;
}

.dag-node-item {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 200px;
}

.dag-node-card {
  padding: 12px 14px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid var(--border-subtle);
}

.node-state-completed {
  border-color: rgba(16, 185, 129, 0.3);
}

.node-state-running {
  border-color: rgba(56, 189, 248, 0.5);
  box-shadow: 0 0 12px rgba(6, 182, 212, 0.2);
}

.node-icon {
  font-size: 20px;
}

.node-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.node-name {
  font-size: 12px;
  font-weight: 700;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-role {
  font-size: 10px;
}

.dag-connector {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--text-muted);
  font-size: 12px;
}

.connector-line {
  width: 16px;
  height: 2px;
  background: var(--border-subtle);
}

.connector-active {
  background: var(--accent-emerald);
}

/* 2-Column Layout */
.agents-split-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

@media (max-width: 1024px) {
  .agents-split-layout {
    grid-template-columns: 1fr;
  }
}

.task-backlog-box {
  background: rgba(11, 15, 25, 0.65);
}

.task-items-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 480px;
  overflow-y: auto;
}

.task-card-row {
  padding: 14px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid var(--border-subtle);
  border-left: 3px solid var(--accent-cyan);
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.task-edge-inprogress { border-left-color: var(--accent-amber); }
.task-edge-success { border-left-color: var(--accent-emerald); }
.task-edge-blocked { border-left-color: var(--accent-rose); }

.task-row-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.task-title-text {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.task-desc-text {
  font-size: 12px;
  color: var(--text-secondary);
}

.task-row-meta {
  display: flex;
  gap: 6px;
  font-size: 10px;
}

.module-chip, .feature-chip {
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-muted);
}

.deps-container {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  margin-top: 4px;
}

.deps-lbl {
  color: var(--text-muted);
}

.deps-chips {
  display: flex;
  gap: 4px;
}

.dep-chip {
  background: rgba(244, 63, 94, 0.15);
  color: #fb7185;
  padding: 2px 6px;
  border-radius: 4px;
}

/* Terminal Console Box */
.terminal-console-box {
  background: #04060a;
  border: 1px solid var(--border-medium);
  padding: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.terminal-header {
  padding: 12px 16px;
  background: #0b0f19;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.terminal-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.terminal-dots {
  display: flex;
  gap: 6px;
}

.t-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.t-red { background: #f43f5e; }
.t-yellow { background: #f59e0b; }
.t-green { background: #10b981; }

.terminal-title {
  font-size: 11px;
  color: var(--text-secondary);
}

.terminal-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.autoscroll-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-muted);
  cursor: pointer;
}

.terminal-screen {
  padding: 16px;
  height: 440px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 11px;
  line-height: 1.6;
}

.terminal-welcome {
  color: #64748b;
  margin-bottom: 6px;
}

.terminal-log-line {
  display: flex;
  align-items: baseline;
  gap: 8px;
  word-break: break-all;
}

.log-level {
  font-weight: 700;
  padding: 0 4px;
  border-radius: 2px;
}

.lvl-info { color: #38bdf8; }
.lvl-step { color: #f59e0b; }
.lvl-success { color: #34d399; }
.lvl-warn { color: #fb7185; }

.log-msg {
  color: #f1f5f9;
}

.terminal-prompt {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  color: #38bdf8;
}

.blinking-cursor {
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.flex-1 { flex: 1; }

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }
</style>
