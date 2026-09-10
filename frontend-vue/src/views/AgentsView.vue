<script setup lang="ts">
import { useAgentMesh } from '../composables/useAgentMesh'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import AgentSwarmGrid from '../components/agents/AgentSwarmGrid.vue'
import AgentTasksTable from '../components/agents/AgentTasksTable.vue'
import AgentMobileCards from '../components/agents/AgentMobileCards.vue'
import DispatchAgentTaskModal from '../components/agents/DispatchAgentTaskModal.vue'
import AgentTranscriptDrawer from '../components/agents/AgentTranscriptDrawer.vue'

const {
  loading, error, actionLoading, projectState, tasks, executions,
  agentSwarm, dagStages, showDispatchModal, showTranscriptDrawer,
  selectedTask, selectedAgent, transcriptViewMode, newTask,
  dependencyInput, selectedCapabilities, terminalLogs, autoScroll,
  completedTasksCount, activeTasksCount, blockedTasksCount,
  fetchAgentData, handleCreateTask, pauseTask, terminateTask,
  openTranscript, clearLogs, formatTime
} = useAgentMesh()
</script>

<template>
  <div class="agents-view-container animate-fade-in">
    <!-- Desktop Header -->
    <div class="view-header desktop-only">
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
        <button class="btn btn-primary" @click="showDispatchModal = true">
          <span>+ Dispatch Task</span>
        </button>
      </div>
    </div>

    <!-- Mobile 44px Command Bar (<768px) -->
    <div class="agents-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🤖 Agent Swarm ({{ agentSwarm.filter(a => a.status === 'running').length }}/{{ agentSwarm.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button 
          class="btn-icon-cmd" 
          title="Dispatch Task" 
          aria-label="Dispatch Task"
          @click="showDispatchModal = true"
        >
          <span>➕</span>
        </button>
        <button 
          class="btn-icon-cmd" 
          :disabled="loading" 
          title="Refresh Swarm" 
          aria-label="Refresh Swarm" 
          @click="fetchAgentData"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="agents-micro-telemetry mobile-only font-mono" role="status" aria-label="Agent Swarm Micro Telemetry">
      <span class="tel-item tel-arch">🏛️ {{ projectState?.architecture_score !== undefined && projectState.architecture_score !== null ? `${Math.round(projectState.architecture_score * 100)}%` : '—' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-health">🛡️ {{ projectState?.repository_health !== undefined && projectState.repository_health !== null ? `${Math.round(projectState.repository_health * 100)}%` : '—' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-tasks">📋 {{ tasks.length > 0 ? `${completedTasksCount}/${tasks.length}` : '0/0' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-runs">⚡ {{ executions.length }}</span>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="error-banner">
      <span class="error-icon">⚠️</span>
      <span>{{ error }}</span>
    </div>

    <!-- Metrics HUD -->
    <div class="metrics-grid desktop-only">
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

          <div v-if="idx < dagStages.length - 1" class="dag-connector">
            <div class="connector-line" :class="{ 'connector-active': stage.status === 'completed' }"></div>
            <span class="connector-arrow">➔</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Active Agent Swarm Grid -->
    <AgentSwarmGrid 
      :agents="agentSwarm"
      :loading="loading"
      @view-transcript="openTranscript($event, null)"
    />

    <!-- Desktop Task Queue vs Mobile Touch Cards -->
    <div class="desktop-only">
      <AgentTasksTable 
        :tasks="tasks"
        :loading="loading"
        @dispatch="showDispatchModal = true"
        @logs="openTranscript(null, $event)"
        @pause="pauseTask"
        @terminate="terminateTask"
      />
    </div>

    <div class="mobile-only">
      <AgentMobileCards 
        :tasks="tasks"
        :agents="agentSwarm"
        @dispatch="showDispatchModal = true"
        @logs="openTranscript(null, $event)"
        @pause="pauseTask"
        @terminate="terminateTask"
      />
    </div>

    <!-- Live Dark Terminal Log Console -->
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

      <div ref="terminalRef" class="terminal-screen font-mono">
        <div class="terminal-welcome">
          --- K8S MULTI-AGENT SWARM TELEMETRY BUS CONNECTED [SESSION OK] ---
        </div>

        <div 
          v-for="(log, idx) in terminalLogs" 
          :key="log.id || idx" 
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

    <!-- Dispatch Task Modal -->
    <DispatchAgentTaskModal
      v-model:show="showDispatchModal"
      v-model:dependency-input="dependencyInput"
      v-model:selected-capabilities="selectedCapabilities"
      :new-task="newTask"
      :action-loading="actionLoading"
      @submit="handleCreateTask"
    />

    <!-- Agent Live Transcript Drawer -->
    <AgentTranscriptDrawer
      v-model:show="showTranscriptDrawer"
      v-model:view-mode="transcriptViewMode"
      v-model:auto-scroll="autoScroll"
      :logs="terminalLogs"
      :selected-agent="selectedAgent"
      :selected-task="selectedTask"
      @clear="clearLogs"
    />
  </div>
</template>

<style>
@import '../assets/styles/views/agents.css';
@import '../assets/styles/components/agents-drawers.css';
</style>
