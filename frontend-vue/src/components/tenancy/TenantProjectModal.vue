<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { Organization } from '../../api/management'

export interface ProjectForm {
  id?: string
  orgId?: string
  name?: string
  envs?: string
  workloads?: number
}

defineProps<{
  show: boolean
  organizations: Organization[]
  newProj: ProjectForm
  isSubmitting: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'create'): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    title="Create Project Namespace"
    subtitle="Allocate cluster namespaces and resource quotas to an organization."
    @update:show="emit('update:show', $event)"
  >
    <form class="form-layout" @submit.prevent="emit('create')">
      <div class="form-group">
        <label>Parent Organization</label>
        <select v-model="newProj.orgId" class="input-glass">
          <option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.name }} ({{ org.id }})</option>
        </select>
      </div>
      <div class="form-group">
        <label>Project Name</label>
        <input v-model="newProj.name" type="text" placeholder="e.g. Payment Gateway Ingress" class="input-glass" required />
      </div>
      <div class="form-group">
        <label>Project Identifier</label>
        <input v-model="newProj.id" type="text" placeholder="e.g. proj-payment-gw" class="input-glass font-mono" required />
      </div>
      <div class="form-group">
        <label>Environments (comma separated)</label>
        <input v-model="newProj.envs" type="text" placeholder="dev, staging, prod" class="input-glass" />
      </div>
      <div class="form-group">
        <label>Initial Workload Pod Capacity</label>
        <input v-model="newProj.workloads" type="number" min="1" max="500" class="input-glass" />
      </div>
    </form>
    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
      <button class="btn btn-primary" :disabled="isSubmitting" @click="emit('create')">
        {{ isSubmitting ? 'Creating...' : 'Create Project' }}
      </button>
    </template>
  </ModalDrawer>
</template>
