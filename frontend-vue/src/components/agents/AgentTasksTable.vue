<script setup lang="ts">
import { ref, computed } from 'vue'
import type { AgentTask } from '../../api/compute'
import StatusBadge from '../ui/StatusBadge.vue'

const props = defineProps<{
  tasks: AgentTask[]
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'dispatch', task?: AgentTask): void
  (e: 'logs', task: AgentTask): void
  (e: 'pause', taskId: string): void
  (e: 'terminate', taskId: string): void
}>()

const filterStatus = ref<'all' | 'inprogress' | 'success' | 'blocked' | 'pending'>('all')
const searchQuery = ref('')

const filteredTasks = computed(() => {
  return props.tasks.filter(t => {
    const matchesStatus = filterStatus.value === 'all' || t.status === filterStatus.value
    const matchesSearch = !searchQuery.value.trim() ||
      t.title.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      t.module.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      t.feature.toLowerCase().includes(searchQuery.value.toLowerCase())
    return matchesStatus && matchesSearch
  })
})
</script>

<template>
  <div class="section-box glass-panel task-table-box">
    <div class="box-header">
      <div>
        <h2 class="box-title">Autonomous Task Queue & DAG Dependency Solver</h2>
        <p class="box-subtitle">Scheduled engineering units with parent dependency resolution and execution controls</p>
      </div>

      <div class="table-header-controls">
        <input 
          v-model="searchQuery" 
          type="text" 
          placeholder="Filter tasks by module/title..." 
          class="input-glass input-sm font-mono search-input" 
        />
        <button class="btn btn-primary btn-xs font-mono" @click="emit('dispatch')">
          + New Task
        </button>
      </div>
    </div>

    <!-- Filter Tabs -->
    <div class="task-filter-bar font-mono">
      <button 
        class="filter-tab-btn" 
        :class="{ active: filterStatus === 'all' }"
        @click="filterStatus = 'all'"
      >
        All ({{ tasks.length }})
      </button>
      <button 
        class="filter-tab-btn" 
        :class="{ active: filterStatus === 'inprogress' }"
        @click="filterStatus = 'inprogress'"
      >
        In Progress ({{ tasks.filter(t => t.status === 'inprogress').length }})
      </button>
      <button 
        class="filter-tab-btn" 
        :class="{ active: filterStatus === 'success' }"
        @click="filterStatus = 'success'"
      >
        Completed ({{ tasks.filter(t => t.status === 'success').length }})
      </button>
      <button 
        class="filter-tab-btn" 
        :class="{ active: filterStatus === 'blocked' }"
        @click="filterStatus = 'blocked'"
      >
        Paused/Blocked ({{ tasks.filter(t => t.status === 'blocked').length }})
      </button>
    </div>

    <!-- Empty State -->
    <div v-if="filteredTasks.length === 0" class="empty-state">
      <span>No tasks matching the selected filters.</span>
    </div>

    <!-- Table -->
    <div v-else class="task-table-container">
      <table class="cyber-table">
        <colgroup>
          <col style="width: 32%;" />
          <col style="width: 12%;" />
          <col style="width: 18%;" />
          <col style="width: 14%;" />
          <col style="width: 24%;" />
        </colgroup>
        <thead>
          <tr>
            <th>Task & Specifications</th>
            <th>Status</th>
            <th>Scope (Module / Feature)</th>
            <th>Prerequisites</th>
            <th class="text-right">Autonomous Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="task in filteredTasks" :key="task.id">
            <!-- Title & ID -->
            <td>
              <div class="task-cell-title">
                <span class="task-cell-name" :title="task.title">{{ task.title }}</span>
                <span class="task-cell-desc font-mono" :title="task.description">{{ task.description }}</span>
                <span class="task-id-tag font-mono">ID: {{ task.id }}</span>
              </div>
            </td>

            <!-- Status Badge -->
            <td>
              <StatusBadge :status="task.status" size="sm" />
            </td>

            <!-- Scope -->
            <td>
              <div class="scope-chips font-mono">
                <span class="module-chip" :title="task.module">{{ task.module }}</span>
                <span class="feature-chip" :title="task.feature">{{ task.feature }}</span>
              </div>
            </td>

            <!-- Prerequisites -->
            <td>
              <div v-if="task.dependencies && task.dependencies.length > 0" class="deps-chips font-mono">
                <span v-for="dep in task.dependencies" :key="dep" class="dep-chip" :title="dep">
                  ⛓️ {{ dep.slice(0, 12) }}
                </span>
              </div>
              <span v-else class="text-muted font-mono text-xs">None (Root DAG)</span>
            </td>

            <!-- Action Buttons: [ 📜 Transcript ], [ ⏸️ Pause ], [ 🛑 Terminate ] -->
            <td class="text-right">
              <div class="task-actions-group">
                <button 
                  class="btn-table-act btn-logs-act font-mono"
                  title="Inspect Live Step Transcript"
                  @click="emit('logs', task)"
                >
                  📜 Transcript
                </button>
                <button 
                  class="btn-table-act btn-pause-act font-mono"
                  :title="task.status === 'blocked' ? 'Resume Task' : 'Pause Task'"
                  @click="emit('pause', task.id)"
                >
                  {{ task.status === 'blocked' ? '▶️ Resume' : '⏸️ Pause' }}
                </button>
                <button 
                  class="btn-table-act btn-terminate font-mono"
                  title="Terminate Autonomous Worker"
                  @click="emit('terminate', task.id)"
                >
                  🛑 Terminate
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/agents.css';
</style>
