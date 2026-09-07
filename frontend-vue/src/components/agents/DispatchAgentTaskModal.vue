<script setup lang="ts">
import type { CreateTaskPayload } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'

const props = defineProps<{
  show: boolean
  actionLoading: boolean
  newTask: CreateTaskPayload
  dependencyInput: string
  selectedCapabilities?: string[]
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'update:dependencyInput', value: string): void
  (e: 'update:selectedCapabilities', value: string[]): void
  (e: 'submit'): void
}>()

const availableCapabilities = [
  { id: 'ast-parsing', name: 'AST Syntax Verification', desc: 'Parses Go/TS AST trees for architectural boundaries' },
  { id: 'k8s-manifests', name: 'K8s Cluster Manifests', desc: 'Synthesizes Helm templates & Traefik routing' },
  { id: 'security-audit', name: 'RBAC & DevSecOps Gate', desc: 'Audits tenant boundaries & token escalation' },
  { id: 'slo-calculation', name: 'SLO / Prometheus SLI', desc: 'Computes multi-window error budgets & burn rate' },
]

function toggleCapability(capId: string) {
  const current = [...(props.selectedCapabilities || [])]
  const idx = current.indexOf(capId)
  if (idx !== -1) {
    current.splice(idx, 1)
  } else {
    current.push(capId)
  }
  emit('update:selectedCapabilities', current)
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Dispatch Swarm Engineering Task"
    subtitle="Define task specifications, architectural prompt, and capability assignments"
    max-width="600px"
    @update:show="emit('update:show', $event)"
  >
    <div class="modal-form">
      <!-- Title -->
      <div class="form-group">
        <label class="form-label">Task Title</label>
        <input 
          v-model="newTask.title" 
          type="text" 
          placeholder="e.g. Implement Multi-Cluster Gateway" 
          class="input-glass" 
        />
      </div>

      <!-- Phase & Module -->
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label">Phase</label>
          <input v-model="newTask.phase" type="text" class="input-glass" />
        </div>
        <div class="form-group flex-1">
          <label class="form-label">Module Target</label>
          <input v-model="newTask.module" type="text" class="input-glass font-mono" />
        </div>
      </div>

      <!-- Feature Scope -->
      <div class="form-group">
        <label class="form-label">Feature Scope</label>
        <input v-model="newTask.feature" type="text" class="input-glass" />
      </div>

      <!-- Prompt / Description -->
      <div class="form-group">
        <label class="form-label">Architectural Prompt & Acceptance Criteria</label>
        <textarea 
          v-model="newTask.description" 
          rows="3" 
          class="input-glass font-mono prompt-textarea" 
          placeholder="Define prompt criteria, interface definitions, and expected unit test assertions..."
        ></textarea>
      </div>

      <!-- Parent Dependencies -->
      <div class="form-group">
        <label class="form-label">Parent DAG Dependencies (comma separated IDs)</label>
        <input 
          :value="dependencyInput" 
          type="text" 
          placeholder="task-sre-telemetry, task-sre-failover" 
          class="input-glass font-mono"
          @input="emit('update:dependencyInput', ($event.target as HTMLInputElement).value)"
        />
      </div>

      <!-- Capabilities Assignment -->
      <div class="form-group">
        <label class="form-label">Autonomous Capabilities to Delegate</label>
        <div class="capabilities-checkbox-grid font-mono">
          <label 
            v-for="cap in availableCapabilities" 
            :key="cap.id" 
            class="cap-checkbox-label"
          >
            <input 
              type="checkbox" 
              :checked="selectedCapabilities?.includes(cap.id)"
              @change="toggleCapability(cap.id)"
            />
            <div>
              <div class="cap-name">{{ cap.name }}</div>
              <div class="cap-desc font-mono text-muted">{{ cap.desc }}</div>
            </div>
          </label>
        </div>
      </div>
    </div>

    <!-- Modal Footer -->
    <template #footer="{ close }">
      <button class="btn btn-secondary" @click="close">Cancel</button>
      <button 
        class="btn btn-primary" 
        :disabled="actionLoading || !newTask.title.trim()" 
        @click="emit('submit')"
      >
        <span>{{ actionLoading ? 'Scheduling...' : 'Dispatch Task ➔' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
.prompt-textarea {
  resize: vertical;
  line-height: 1.4;
  font-size: 11px;
}

.cap-name {
  font-weight: 700;
  color: #fff;
}

.cap-desc {
  font-size: 9px;
  margin-top: 2px;
}
</style>
