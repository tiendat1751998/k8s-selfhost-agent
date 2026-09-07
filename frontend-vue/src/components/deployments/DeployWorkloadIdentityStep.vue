<script setup lang="ts">
import type { NewAppForm } from './DeployWorkloadModal.vue'

interface Props {
  form: NewAppForm
}

defineProps<Props>()
</script>

<template>
  <div class="wizard-step-body animate-fade-in">
    <div class="form-row">
      <div class="form-group flex-1">
        <label class="form-label">Deployment Name *</label>
        <input v-model="form.name" type="text" placeholder="e.g. payment-service" class="input-glass font-mono" />
      </div>
      <div class="form-group flex-1">
        <label class="form-label">Namespace</label>
        <input v-model="form.namespace" type="text" placeholder="default" class="input-glass font-mono" />
      </div>
    </div>

    <div class="form-group">
      <label class="form-label">Container Image (Registry / Tag) *</label>
      <input v-model="form.image" type="text" placeholder="e.g. nginx:alpine or registry.corp.io/api:v1.2.0" class="input-glass font-mono" />
    </div>

    <div class="form-row">
      <div class="form-group flex-1">
        <label class="form-label">Runtime Engine</label>
        <select v-model="form.type" class="input-glass">
          <option value="kubernetes">Kubernetes</option>
          <option value="swarm">Docker Swarm</option>
        </select>
      </div>
      <div class="form-group flex-1">
        <label class="form-label">Target Cluster / Host</label>
        <input v-model="form.target" type="text" placeholder="prod-us-east-1" class="input-glass font-mono" />
      </div>
      <div class="form-group flex-1">
        <label class="form-label">Initial Replicas</label>
        <input v-model.number="form.replicas" type="number" min="1" max="50" class="input-glass font-mono" />
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/deploy-workload-modal.css';
</style>
