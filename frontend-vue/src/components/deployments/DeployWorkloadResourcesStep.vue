<script setup lang="ts">
import type { NewAppForm } from './DeployWorkloadModal.vue'

interface Props {
  form: NewAppForm
}

const props = defineProps<Props>()

function addEnvRow() {
  props.form.envList.push({ key: '', value: '' })
}

function removeEnvRow(idx: number) {
  props.form.envList.splice(idx, 1)
}
</script>

<template>
  <div class="wizard-step-body animate-fade-in">
    <div class="form-row">
      <div class="form-group flex-1">
        <label class="form-label">CPU Request</label>
        <input v-model="form.cpu" type="text" class="input-glass font-mono" />
      </div>
      <div class="form-group flex-1">
        <label class="form-label">Memory Request</label>
        <input v-model="form.memory" type="text" class="input-glass font-mono" />
      </div>
    </div>

    <div class="env-section">
      <div class="env-section-header">
        <label class="form-label">Environment Variables</label>
        <button type="button" class="btn btn-secondary btn-xs" @click="addEnvRow">
          <span>+ Add Variable</span>
        </button>
      </div>

      <div v-for="(envItem, idx) in form.envList" :key="idx" class="env-row">
        <input v-model="envItem.key" type="text" placeholder="KEY" class="input-glass font-mono flex-1" />
        <input v-model="envItem.value" type="text" placeholder="VALUE" class="input-glass font-mono flex-1" />
        <button type="button" class="btn btn-secondary btn-xs btn-remove" @click="removeEnvRow(idx)">✕</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/deploy-workload-modal.css';
</style>
