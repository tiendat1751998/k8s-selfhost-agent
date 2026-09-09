<script setup lang="ts">
import { computed } from 'vue'
import type { TranscriptEntry, AgentProfile } from '../../composables/useAgentMesh'
import type { AgentTask } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'

const props = defineProps<{
  show: boolean
  logs: TranscriptEntry[]
  selectedAgent?: AgentProfile | null
  selectedTask?: AgentTask | null
  viewMode: 'steps' | 'raw'
  autoScroll: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'update:viewMode', mode: 'steps' | 'raw'): void
  (e: 'update:autoScroll', value: boolean): void
  (e: 'clear'): void
}>()

const drawerTitle = computed(() => {
  if (props.selectedAgent) {
    return `${props.selectedAgent.name} · Step Reasoning Trace`
  }
  if (props.selectedTask) {
    return `${props.selectedTask.title} · Live Transcript`
  }
  return 'Swarm Live Reasoning & Transcript Stream'
})

const drawerSubtitle = computed(() => {
  if (props.selectedAgent) {
    return `Specialized Role: ${props.selectedAgent.role} | Active Task: ${props.selectedAgent.activeTaskId || 'Autonomous Standby'}`
  }
  return 'Real-time JSONL telemetry, tool invocations, and deterministic step reasoning'
})

function formatTime(d: string) {
  try {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    placement="right"
    max-width="640px"
    :title="drawerTitle"
    :subtitle="drawerSubtitle"
    @update:show="emit('update:show', $event)"
  >
    <div class="transcript-drawer-body">
      <!-- Top View Mode & Controls -->
      <div class="drawer-view-controls font-mono">
        <div class="view-mode-tabs">
          <button 
            class="tab-btn" 
            :class="{ active: viewMode === 'steps' }"
            @click="emit('update:viewMode', 'steps')"
          >
            🧠 Reasoning Trace ({{ logs.length }})
          </button>
          <button 
            class="tab-btn" 
            :class="{ active: viewMode === 'raw' }"
            @click="emit('update:viewMode', 'raw')"
          >
            📜 Raw JSONL Stream
          </button>
        </div>

        <div class="terminal-actions">
          <label class="autoscroll-toggle">
            <input 
              :checked="autoScroll" 
              type="checkbox" 
              @change="emit('update:autoScroll', ($event.target as HTMLInputElement).checked)"
            />
            <span>Auto-Scroll</span>
          </label>
          <button class="btn btn-secondary btn-xs font-mono" @click="emit('clear')">
            Clear
          </button>
        </div>
      </div>

      <!-- Mode 1: Step Reasoning Trace -->
      <div v-if="viewMode === 'steps'" class="transcript-steps-list">
        <div v-if="logs.length === 0" class="empty-state">
          <span>No execution steps recorded for this session.</span>
        </div>

        <div 
          v-for="(step, idx) in logs" 
          :key="step.id || idx" 
          class="step-card glass-panel"
        >
          <div class="step-card-header font-mono">
            <span class="step-badge-num">STEP {{ step.stepIndex || (idx + 1) }}</span>
            <span class="text-muted">[{{ formatTime(step.timestamp) }}]</span>
            <StatusBadge :status="step.level.toLowerCase()" size="sm" :label="step.level" />
            <span class="text-cyan">&lt;{{ step.agent }}&gt;</span>
          </div>

          <div class="log-msg">{{ step.message }}</div>

          <!-- Step Reasoning Trace Box -->
          <div v-if="step.reasoning" class="step-reasoning-box font-mono">
            <div class="reasoning-lbl text-muted">🧠 Step Reasoning & Decision:</div>
            <div>{{ step.reasoning }}</div>
          </div>

          <!-- Tool Call Box -->
          <div v-if="step.toolCall" class="step-tool-call font-mono">
            <span class="text-muted">🔧 Tool Invocation: </span>
            <code>{{ step.toolCall }}</code>
          </div>
        </div>
      </div>

      <!-- Mode 2: Raw JSONL Stream Terminal -->
      <div v-else class="raw-jsonl-screen font-mono">
        <div class="terminal-welcome">
          --- K8S AGENT SWARM JSONL PROTOCOL [SOCKET STREAM CONNECTED] ---
        </div>
        <div 
          v-for="(log, idx) in logs" 
          :key="log.id || idx" 
          class="terminal-log-line"
        >
          <span class="text-muted">#{{ idx + 1 }}</span>
          <span class="jsonl-code">{{ log.rawJsonl || JSON.stringify(log) }}</span>
        </div>
        <div class="terminal-prompt">
          <span class="prompt-arrow">swarm-bus:~$</span>
          <span class="blinking-cursor">█</span>
        </div>
      </div>
    </div>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/agents.css';
</style>
