<template>
  <div v-if="show" class="drawer-overlay" @click.self="$emit('close')">
    <div class="drawer-panel glass-panel animate-slide-left">
      <div class="drawer-header">
        <div>
          <span class="drawer-tag font-mono">PROCEDURE RUNBOOK INSPECTION</span>
          <h3 class="drawer-title">{{ runbook?.title || 'Execution Steps' }}</h3>
        </div>
        <button class="drawer-close" @click="$emit('close')">✕</button>
      </div>

      <div class="drawer-content">
        <!-- Progress Bar -->
        <div class="execution-progress">
          <div class="progress-header">
            <span>Procedure Progress</span>
            <span class="font-mono">{{ completedSteps.size }} / {{ steps.length }} Steps ({{ progressPercent }}%)</span>
          </div>
          <div class="progress-bar-bg">
            <div class="progress-bar-fill" :style="{ width: `${progressPercent}%` }"></div>
          </div>
        </div>

        <!-- Steps List -->
        <div class="steps-list">
          <div 
            v-for="(step, idx) in steps" 
            :key="step.id || idx" 
            class="step-card glass-panel"
            :class="{ 'step-done': completedSteps.has(idx) }"
          >
            <div class="step-header" @click="$emit('toggleStep', idx)">
              <div class="step-checkbox" :class="{ 'step-checkbox-active': completedSteps.has(idx) }">
                <span v-if="completedSteps.has(idx)">✓</span>
                <span v-else>{{ idx + 1 }}</span>
              </div>
              <span class="step-title" :class="{ 'text-strikethrough': completedSteps.has(idx) }">
                {{ step.title }}
              </span>
              <button 
                v-if="step.command" 
                class="btn btn-xs btn-primary font-mono"
                title="Execute single command"
                @click.stop="$emit('executeStep', step)"
              >
                <span>⚡ Run Step</span>
              </button>
            </div>

            <div class="step-body">
              <p v-if="step.content" class="step-desc">{{ step.content }}</p>
              <div v-if="step.command" class="command-box font-mono">
                <pre class="command-pre">{{ step.command }}</pre>
                <button class="copy-btn" @click.stop="$emit('copyCommand', step.command)">📋 Copy</button>
              </div>
            </div>
          </div>
        </div>

        <!-- Streaming Logs -->
        <div v-if="logs.length > 0" class="log-terminal-box">
          <div class="log-terminal-header font-mono">
            <span>LIVE EXECUTION TELEMETRY &amp; STREAMING LOGS</span>
            <span class="text-muted">{{ logs.length }} events</span>
          </div>
          <div class="log-terminal-body font-mono">
            <div v-for="(log, lIdx) in logs" :key="lIdx" class="log-terminal-line">
              <span v-if="log.includes('[ERROR]')" class="text-rose">{{ log }}</span>
              <span v-else-if="log.includes('[SUCCESS]') || log.includes('[OK]')" class="text-emerald">{{ log }}</span>
              <span v-else-if="log.includes('[EXEC]') || log.includes('[CMD]')" class="text-cyan">{{ log }}</span>
              <span v-else class="text-secondary">{{ log }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="drawer-footer">
        <button class="btn btn-secondary" @click="$emit('close')">Close</button>
        <button 
          class="btn btn-primary" 
          :disabled="completedSteps.size === steps.length"
          @click="$emit('markAllComplete')"
        >
          <span>✓ Complete All Steps</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Runbook } from '../../api/governance'
import type { RunbookStep, RunbookExecutionRecord } from '../../composables/useRunbooks'

const props = defineProps<{
  show: boolean
  runbook: Runbook | null
  execution?: RunbookExecutionRecord | null
  steps: RunbookStep[]
  completedSteps: Set<number>
  logs: string[]
}>()

defineEmits<{
  (e: 'close'): void
  (e: 'toggleStep', idx: number): void
  (e: 'markAllComplete'): void
  (e: 'executeStep', step: RunbookStep): void
  (e: 'copyCommand', cmd?: string): void
}>()

const progressPercent = computed(() => {
  if (props.steps.length === 0) return 0
  return Math.round((props.completedSteps.size / props.steps.length) * 100)
})
</script>