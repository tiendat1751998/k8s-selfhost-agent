<script setup lang="ts">
import type { NewAppForm } from './DeployWorkloadModal.vue'

interface Props {
  form: NewAppForm
}

defineProps<Props>()
</script>

<template>
  <div class="wizard-step-body animate-fade-in">
    <div class="form-group">
      <label class="form-label">Rollout Strategy</label>
      <select v-model="form.strategy" class="input-glass">
        <option value="RollingUpdate">RollingUpdate (Zero Downtime, MaxSurge 25%)</option>
        <option value="Canary">Canary Deployment (Weighted Traffic Split)</option>
        <option value="BlueGreen">Blue-Green Deployment (Active/Standby Router)</option>
        <option value="Recreate">Recreate (Terminate all before start)</option>
      </select>
    </div>

    <div v-if="form.strategy === 'Canary'" class="glass-panel p-4">
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label">Initial Canary Traffic Weight (%)</label>
          <input v-model.number="form.canaryWeight" type="number" min="5" max="90" step="5" class="input-glass font-mono" />
        </div>
        <div class="form-group flex-1">
          <label class="form-label">Canary Track Image (Optional)</label>
          <input v-model="form.canaryVersion" type="text" placeholder="e.g. nginx:1.25-canary" class="input-glass font-mono" />
        </div>
      </div>
    </div>

    <div v-if="form.strategy === 'BlueGreen'" class="glass-panel p-4">
      <div class="form-group">
        <label class="form-label">Green (Standby) Image Tag</label>
        <input v-model="form.greenVersion" type="text" placeholder="e.g. app:v2.0.0-rc1" class="input-glass font-mono" />
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/deploy-workload-modal.css';
</style>
