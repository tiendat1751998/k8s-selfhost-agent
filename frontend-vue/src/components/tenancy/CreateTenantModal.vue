<script setup lang="ts">
import { ref } from 'vue'
import type { Organization } from '../../api/management'
import ModalDrawer from '../ui/ModalDrawer.vue'

interface Props {
  show: boolean
  isSubmitting?: boolean
}

withDefaults(defineProps<Props>(), {
  isSubmitting: false
})

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'create', org: Partial<Organization>): void
}>()

const formModel = ref({
  id: '',
  name: '',
  tier: 'Enterprise Tier-1 (Dedicated Nodes)',
  quotaPreset: 'standard'
})

function onNameInput() {
  if (!formModel.value.id || formModel.value.id.startsWith('org-')) {
    formModel.value.id = 'org-' + formModel.value.name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '')
  }
}

function handleFormSubmit() {
  if (!formModel.value.id || !formModel.value.name) return
  emit('create', {
    id: formModel.value.id,
    name: formModel.value.name,
    tier: formModel.value.tier,
    quotaPreset: formModel.value.quotaPreset
  })
  formModel.value = {
    id: '',
    name: '',
    tier: 'Enterprise Tier-1 (Dedicated Nodes)',
    quotaPreset: 'standard'
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    title="Provision Organization Boundary"
    subtitle="Create a new isolated multi-tenant organization container with cluster namespace quotas."
    @update:show="emit('update:show', $event)"
  >
    <form class="form-layout" @submit.prevent="handleFormSubmit">
      <div class="form-group">
        <label>Organization Display Name</label>
        <input
          v-model="formModel.name"
          type="text"
          placeholder="e.g. APAC Fintech Operations"
          class="input-glass"
          required
          @input="onNameInput"
        />
      </div>

      <div class="form-group">
        <label>Organization Identifier (Slug)</label>
        <input
          v-model="formModel.id"
          type="text"
          placeholder="e.g. org-fintech-apac"
          class="input-glass font-mono"
          required
        />
        <small class="form-hint">Used for Kubernetes namespace isolation and RBAC policy binding.</small>
      </div>

      <div class="form-group">
        <label>Service Tier & Isolation Level</label>
        <select v-model="formModel.tier" class="input-glass">
          <option value="Enterprise Tier-1 (Dedicated Nodes)">Enterprise Tier-1 (Dedicated Nodes)</option>
          <option value="GovCloud High-Sec (FIPS-140-3)">GovCloud High-Sec (FIPS-140-3)</option>
          <option value="High-Performance (GPU Accelerated)">High-Performance (GPU Accelerated)</option>
          <option value="Standard Multi-Tenant">Standard Multi-Tenant</option>
        </select>
      </div>

      <div class="form-group">
        <label>Resource Quota Preset</label>
        <select v-model="formModel.quotaPreset" class="input-glass">
          <option value="standard">Standard (50 Pods, 16 CPU, 64GB RAM)</option>
          <option value="large">Large Scale (200 Pods, 64 CPU, 256GB RAM)</option>
          <option value="maximum">Maximum Capacity (500 Pods, 128 CPU, 512GB RAM)</option>
        </select>
      </div>
    </form>

    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
      <button
        class="btn btn-primary"
        :disabled="isSubmitting"
        @click="handleFormSubmit"
      >
        {{ isSubmitting ? 'Provisioning...' : 'Confirm Organization' }}
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/components/tenancy-drawers.css';
</style>
