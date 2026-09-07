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
.modal-form-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 14px;
}
.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.form-group.full-width {
  grid-column: span 2;
}
.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}
.presets-chips {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.preset-chip-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all var(--transition-fast);
}
.preset-chip-btn:hover {
  background: rgba(252, 213, 53, 0.1);
  border-color: rgba(252, 213, 53, 0.3);
  color: var(--color-primary);
}
.modal-footer-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
  padding-top: 14px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}
.spin-anim {
  display: inline-block;
  animation: spin 1s linear infinite;
}
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
@media (max-width: 640px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
  .form-group.full-width {
    grid-column: span 1;
  }
}
</style>
