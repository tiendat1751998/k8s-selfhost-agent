<template>
  <ModalDrawer
    :show="show"
    :title="runbook ? `Execute: ${runbook.title}` : 'Execute Runbook'"
    max-width="600px"
    @close="$emit('close')"
  >
    <div v-if="runbook" class="execute-modal-body">
      <div class="exec-info-banner glass-panel">
        <div class="exec-cat-tag font-mono">{{ runbook.category }}</div>
        <h4 class="exec-title">{{ runbook.title }}</h4>
        <p class="exec-desc text-muted">{{ runbook.steps_count || 3 }} automated DAG steps configured.</p>
      </div>

      <div class="form-group">
        <label>Execution Mode</label>
        <div class="radio-group">
          <label class="radio-label">
            <input v-model="isDryRun" type="radio" :value="false" />
            <span>⚡ Live Execution (Run diagnostic commands)</span>
          </label>
          <label class="radio-label">
            <input v-model="isDryRun" type="radio" :value="true" />
            <span>🧪 Dry Run Simulation (Validate syntax only)</span>
          </label>
        </div>
      </div>

      <div v-if="parameters.length > 0" class="parameters-section">
        <h5 class="section-label">Configured Parameters</h5>
        <div v-for="param in parameters" :key="param.key" class="form-group">
          <label>{{ param.label }}</label>
          <input 
            v-if="param.type === 'string' || param.type === 'number'"
            v-model="paramValues[param.key]"
            :type="param.type === 'number' ? 'number' : 'text'"
            class="input-glass"
          />
          <label v-else-if="param.type === 'boolean'" class="checkbox-label">
            <input v-model="paramValues[param.key]" type="checkbox" />
            <span>{{ param.description || param.label }}</span>
          </label>
        </div>
      </div>

      <div class="modal-actions">
        <button class="btn btn-secondary" @click="$emit('close')">Cancel</button>
        <button class="btn btn-primary" @click="handleRun">
          <span>{{ isDryRun ? '🧪 Run Simulation' : '⚡ Start Procedure' }}</span>
        </button>
      </div>
    </div>
  </ModalDrawer>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { Runbook } from '../../api/governance'
import type { RunbookParam } from '../../composables/useRunbooks'

const props = defineProps<{
  show: boolean
  runbook: Runbook | null
  parameters: RunbookParam[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'execute', payload: { isDryRun: boolean; params: Record<string, string | number | boolean> }): void
}>()

const isDryRun = ref(false)
const paramValues = reactive<Record<string, string | number | boolean>>({})

watch(() => props.parameters, (newParams) => {
  newParams.forEach(p => {
    if (paramValues[p.key] === undefined) {
      paramValues[p.key] = p.defaultValue ?? ''
    }
  })
}, { immediate: true })

function handleRun() {
  emit('execute', { isDryRun: isDryRun.value, params: { ...paramValues } })
}
</script>