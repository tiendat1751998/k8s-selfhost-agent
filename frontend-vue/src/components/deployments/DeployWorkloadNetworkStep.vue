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
        <label class="form-label">Service Network Mode</label>
        <select v-model="form.netType" class="input-glass">
          <option value="ClusterIP">ClusterIP (Internal Mesh)</option>
          <option value="NodePort">NodePort (Host Port)</option>
          <option value="LoadBalancer">LoadBalancer (Cloud LB / MetalLB)</option>
          <option value="Ingress">Ingress (HTTP Host Router)</option>
        </select>
      </div>
      <div class="form-group flex-1">
        <label class="form-label">Container Port</label>
        <input v-model.number="form.port" type="number" class="input-glass font-mono" />
      </div>
    </div>

    <div v-if="form.netType === 'Ingress'" class="form-group animate-fade-in">
      <label class="form-label">Ingress Domain Hostname</label>
      <input v-model="form.ingressHost" type="text" placeholder="e.g. api.corp.internal" class="input-glass font-mono" />
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/deploy-workload-modal.css';
</style>
