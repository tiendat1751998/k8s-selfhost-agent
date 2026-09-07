<script setup lang="ts">
import type { AgentProfile } from '../../composables/useAgentMesh'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  agents: AgentProfile[]
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'viewTranscript', agent: AgentProfile): void
}>()

function formatNumber(n: number) {
  if (n >= 1000) {
    return `${(n / 1000).toFixed(1)}k`
  }
  return String(n)
}

function calculatePercentage(used: number, total: number) {
  if (!total) return 0
  return Math.min(100, Math.round((used / total) * 100))
}
</script>

<template>
  <div class="section-box glass-panel swarm-section">
    <div class="box-header">
      <div>
        <h2 class="box-title">Active AI Agent Swarm Nodes</h2>
        <p class="box-subtitle">Distributed autonomous roles with isolated memory pools and live token quotas</p>
      </div>
      <span class="font-mono text-cyan text-xs">Active Pool: {{ agents.filter(a => a.status === 'running').length }}/{{ agents.length }} Running</span>
    </div>

    <div class="swarm-grid">
      <div 
        v-for="agent in agents" 
        :key="agent.id" 
        class="swarm-agent-card glass-panel"
      >
        <!-- Header: Avatar, State Ring, Name, Role, Status -->
        <div class="agent-card-header">
          <div class="agent-identity">
            <div class="agent-avatar-wrap">
              <span class="agent-icon-glyph">{{ agent.icon }}</span>
              <div 
                class="state-ring" 
                :class="`ring-${agent.status}`"
              ></div>
            </div>
            <div class="agent-title-col">
              <span class="agent-name">{{ agent.name }}</span>
              <span class="agent-role-tag font-mono">{{ agent.role }}</span>
            </div>
          </div>
          <StatusBadge :status="agent.status" size="sm" />
        </div>

        <!-- Token Capacity Meter -->
        <div class="agent-meter-block font-mono">
          <div class="meter-header">
            <span>Context Tokens:</span>
            <span class="meter-val">{{ formatNumber(agent.tokensUsed) }} / {{ formatNumber(agent.totalTokens) }} ({{ calculatePercentage(agent.tokensUsed, agent.totalTokens) }}%)</span>
          </div>
          <div class="token-progress-bar">
            <div 
              class="token-progress-fill" 
              :style="{ width: `${calculatePercentage(agent.tokensUsed, agent.totalTokens)}%` }"
            ></div>
          </div>
        </div>

        <!-- Footprint & Capabilities -->
        <div class="agent-meta-row font-mono">
          <span>🧠 {{ agent.memoryUsageMb }} MB Mem</span>
          <span>⚡ {{ agent.latencyMs }}ms RT</span>
          <span class="text-emerald">🛡️ {{ agent.healthScore }}% Health</span>
        </div>

        <!-- Capabilities List -->
        <div class="capabilities-list font-mono">
          <span 
            v-for="cap in agent.capabilities" 
            :key="cap" 
            class="cap-chip"
          >
            {{ cap }}
          </span>
        </div>

        <!-- Action Footer -->
        <div class="agent-actions-footer">
          <button 
            class="btn btn-secondary btn-xs font-mono flex-1"
            @click="emit('viewTranscript', agent)"
          >
            📜 View Transcript & Traces
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.swarm-section {
  background: rgba(11, 15, 25, 0.65);
}
.text-xs { font-size: 11px; }
.flex-1 { flex: 1; }

.agent-identity {
  display: flex;
  align-items: center;
  gap: 10px;
}

.agent-title-col {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.agent-name {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
  line-height: 1.2;
}

.agent-role-tag {
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  font-weight: 600;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.25);
  padding: 1px 6px;
  border-radius: 4px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.agent-meta-row {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  color: var(--text-muted);
}

.capabilities-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.cap-chip {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.08);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 10px;
  color: var(--text-secondary);
}
</style>
