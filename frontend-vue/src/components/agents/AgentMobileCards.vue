<script setup lang="ts">
import { ref } from 'vue'
import type { AgentTask } from '../../api/compute'
import type { AgentProfile } from '../../composables/useAgentMesh'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  tasks: AgentTask[]
  agents?: AgentProfile[]
}>()

const emit = defineEmits<{
  (e: 'dispatch', task?: AgentTask): void
  (e: 'logs', task: AgentTask): void
  (e: 'pause', taskId: string): void
  (e: 'terminate', taskId: string): void
}>()

const expandedTaskId = ref<string | null>(null)

function toggleExpand(taskId: string) {
  expandedTaskId.value = expandedTaskId.value === taskId ? null : taskId
}
</script>

<template>
  <div class="mobile-card-stream-container">
    <div class="mobile-stream-header">
      <span class="font-mono text-cyan text-xs font-bold">MOBILE TASK STREAM (TOUCH OPTIMIZED)</span>
      <button class="btn btn-primary btn-xs font-mono" @click="emit('dispatch')">+ New</button>
    </div>

    <div class="mobile-card-stream">
      <div 
        v-for="task in tasks" 
        :key="task.id" 
        class="mobile-card-item glass-panel"
        :class="{ expanded: expandedTaskId === task.id }"
      >
        <!-- Compact Summary Row (~65px height) -->
        <div class="mobile-compact-row" @click="toggleExpand(task.id)">
          <div class="mobile-card-left">
            <span class="mobile-card-icon">⚡</span>
            <div class="mobile-card-info">
              <div class="mobile-title-row">
                <span class="mobile-card-title">{{ task.title }}</span>
              </div>
              <div class="mobile-card-meta font-mono">
                <StatusBadge :status="task.status" size="sm" />
                <span class="module-chip-sm">{{ task.module.split('/').pop() }}</span>
              </div>
            </div>
          </div>

          <!-- Quick Action Buttons -->
          <div class="mobile-card-actions" @click.stop>
            <button 
              class="btn-icon-sm btn-dispatch-act" 
              title="Dispatch Task"
              @click="emit('dispatch', task)"
            >
              ⚡
            </button>
            <button 
              class="btn-icon-sm btn-logs-act" 
              title="View Transcript Logs"
              @click="emit('logs', task)"
            >
              📜
            </button>
            <button 
              class="btn-icon-sm btn-pause-act" 
              :title="task.status === 'blocked' ? 'Resume' : 'Pause'"
              @click="emit('pause', task.id)"
            >
              {{ task.status === 'blocked' ? '▶' : '⏸' }}
            </button>
            <button 
              class="btn-icon-sm btn-terminate" 
              title="Terminate Task"
              @click="emit('terminate', task.id)"
            >
              🗑
            </button>
          </div>
        </div>

        <!-- Expanded Details Drawer -->
        <div v-if="expandedTaskId === task.id" class="mobile-expanded-content font-mono">
          <p class="expanded-desc">{{ task.description }}</p>
          <div class="expanded-meta-grid">
            <div><span class="text-muted">Phase:</span> {{ task.phase }}</div>
            <div><span class="text-muted">Feature:</span> {{ task.feature }}</div>
            <div v-if="task.dependencies?.length">
              <span class="text-muted">Prerequisites:</span>
              <span v-for="d in task.dependencies" :key="d" class="dep-chip-xs"> {{ d }} </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/agents.css';
</style>
