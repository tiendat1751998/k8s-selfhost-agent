<template>
  <div v-if="show" class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal-card glass-panel">
      <div class="modal-header">
        <div class="modal-title-row">
          <span class="modal-icon">{{ isEditing ? '✏️' : '✨' }}</span>
          <h3>{{ isEditing ? 'Edit Plugin Extension' : 'Register New Plugin' }}</h3>
        </div>
        <button class="modal-close" @click="$emit('close')">✕</button>
      </div>

      <form @submit.prevent="$emit('submit')" class="modal-form">
        <div class="form-grid">
          <div class="form-group">
            <label>Plugin Name <span class="required">*</span></label>
            <input
              v-model="form.name"
              type="text"
              required
              placeholder="e.g. k8s-cost-analyzer"
              class="text-input"
            />
          </div>

          <div class="form-group">
            <label>Version</label>
            <input
              v-model="form.version"
              type="text"
              placeholder="1.0.0"
              class="text-input"
            />
          </div>

          <div class="form-group">
            <label>Category</label>
            <select v-model="form.category" class="select-input">
              <option value="monitoring">📊 Monitoring</option>
              <option value="security">🛡️ Security</option>
              <option value="devtools">🛠️ Developer Tools</option>
              <option value="integration">🔌 Integration</option>
            </select>
          </div>

          <div class="form-group">
            <label>Icon (Emoji or Icon Name)</label>
            <input
              v-model="form.icon"
              type="text"
              placeholder="e.g. 📊, ⚡, 🛡️"
              class="text-input"
            />
          </div>

          <div class="form-group full-width">
            <label>Author / Organization</label>
            <input
              v-model="form.author"
              type="text"
              placeholder="e.g. DevOps Core Team"
              class="text-input"
            />
          </div>

          <div class="form-group full-width">
            <label>JS Bundle Entry Point URL</label>
            <input
              v-model="form.entry_point"
              type="text"
              placeholder="https://cdn.example.com/plugins/bundle.js"
              class="text-input font-mono"
            />
            <span class="input-hint">Remote or relative URL to the JavaScript module bundle</span>
          </div>

          <div class="form-group full-width">
            <label>Description</label>
            <textarea
              v-model="form.description"
              rows="3"
              placeholder="Describe what this plugin does..."
              class="textarea-input"
            ></textarea>
          </div>

          <div class="form-group full-width">
            <label>Permissions (comma separated)</label>
            <input
              :value="permissionsRaw"
              @input="$emit('update:permissionsRaw', ($event.target as HTMLInputElement).value)"
              type="text"
              placeholder="k8s:read, metrics:read, exec:pods"
              class="text-input"
            />
            <span class="input-hint">e.g. k8s:read, metrics:read, logs:read, notifications:write</span>
          </div>
        </div>

        <div v-if="error" class="form-error-msg">
          ⚠️ {{ error }}
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" @click="$emit('close')">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? 'Saving...' : (isEditing ? 'Save Changes' : 'Register Plugin') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  show: boolean
  isEditing: boolean
  form: {
    name: string
    version: string
    category: string
    author: string
    icon: string
    entry_point: string
    description: string
    enabled: boolean
  }
  permissionsRaw: string
  submitting: boolean
  error: string | null
}>()

defineEmits<{
  (e: 'close'): void
  (e: 'submit'): void
  (e: 'update:permissionsRaw', value: string): void
}>()
</script>
