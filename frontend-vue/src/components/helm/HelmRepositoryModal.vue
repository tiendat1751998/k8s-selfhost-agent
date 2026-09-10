<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'

defineProps<{
  show: boolean
  addRepoForm: {
    name: string
    url: string
  }
  addingRepo: boolean
  repoPresets: Array<{ name: string; url: string; icon: string }>
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'close'): void
  (e: 'addRepo'): void
  (e: 'applyPreset', preset: { name: string; url: string }): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    max-width="600px"
    title="Add Helm Repository"
    subtitle="Register an HTTP or OCI chart repository"
    @close="emit('close'); emit('update:show', false)"
  >
    <div class="modal-form-body">
      <div class="form-grid">
        <div class="form-group">
          <label class="form-label">Repository Name <span class="text-rose">*</span></label>
          <input
            v-model="addRepoForm.name"
            type="text"
            class="input-glass"
            placeholder="e.g. bitnami"
          />
        </div>

        <div class="form-group">
          <label class="form-label">Repository URL <span class="text-rose">*</span></label>
          <input
            v-model="addRepoForm.url"
            type="url"
            class="input-glass"
            placeholder="https://charts.bitnami.com/bitnami"
          />
        </div>

        <!-- Popular Presets -->
        <div class="form-group full-width">
          <label class="form-label text-muted">Quick Presets:</label>
          <div class="presets-chips">
            <button
              v-for="p in repoPresets"
              :key="p.name"
              type="button"
              class="preset-chip-btn"
              @click="emit('applyPreset', p)"
            >
              <span>{{ p.icon }}</span>
              <span>{{ p.name }}</span>
            </button>
          </div>
        </div>
      </div>

      <div class="modal-footer-actions">
        <button type="button" class="btn-cyber btn-secondary" @click="emit('close'); emit('update:show', false)">
          Cancel
        </button>
        <button
          type="button"
          class="btn-cyber btn-primary"
          :disabled="addingRepo || !addRepoForm.name.trim() || !addRepoForm.url.trim()"
          @click="emit('addRepo')"
        >
          <span :class="{ 'spin-anim': addingRepo }">➕</span>
          <span>{{ addingRepo ? 'Adding & Indexing...' : 'Add Repository' }}</span>
        </button>
      </div>
    </div>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/components/helm-drawers.css';
</style>
