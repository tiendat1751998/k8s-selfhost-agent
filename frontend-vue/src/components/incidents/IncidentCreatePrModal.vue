<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'

export interface IncidentPRForm {
  title: string
  description: string
  repoUrl: string
  branch: string
  baseBranch: string
}

defineProps<{
  show: boolean
  actionLoading: string | null
  form: IncidentPRForm
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'submit'): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Create GitOps Remediation Pull Request"
    subtitle="Synthesize patch manifest and submit pull request to repository"
    max-width="580px"
    @update:show="emit('update:show', $event)"
  >
    <div class="modal-form">
      <div class="form-group">
        <label class="form-label">Pull Request Title</label>
        <input v-model="form.title" type="text" class="input-glass" />
      </div>

      <div class="form-group">
        <label class="form-label">GitOps Repository</label>
        <input v-model="form.repoUrl" type="text" class="input-glass" />
      </div>

      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label">Branch Name</label>
          <input v-model="form.branch" type="text" class="input-glass font-mono" />
        </div>
        <div class="form-group flex-1">
          <label class="form-label">Base Branch</label>
          <input v-model="form.baseBranch" type="text" class="input-glass font-mono" />
        </div>
      </div>

      <div class="form-group">
        <label class="form-label">Description / Commit Message</label>
        <textarea v-model="form.description" rows="3" class="input-glass font-mono"></textarea>
      </div>
    </div>

    <template #footer="{ close }">
      <button class="btn-slate" @click="close">Cancel</button>
      <button class="btn-slate-primary" :disabled="actionLoading === 'create-pr'" @click="emit('submit')">
        <span>{{ actionLoading === 'create-pr' ? 'Submitting...' : 'Create Pull Request' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
