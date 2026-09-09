<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { CustomTemplateForm } from '../../composables/useScaffolder'

defineProps<{
  show: boolean
  mode: 'create' | 'edit'
  templateForm: CustomTemplateForm
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save'): void
  (e: 'add-variable'): void
  (e: 'remove-variable', index: number): void
}>()
</script>

<template>
  <ModalDrawer
    v-if="show"
    :show="show"
    :title="mode === 'create' ? 'Create Custom Scaffold Template' : 'Edit Scaffold Template'"
    @close="emit('close')"
  >
    <div class="custom-template-modal">
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label">Template Name <span class="required-star">*</span></label>
          <input
            v-model="templateForm.name"
            type="text"
            placeholder="e.g. Rust Microservice Starter"
            class="form-input"
          />
        </div>
        <div class="form-group flex-1">
          <label class="form-label">Category</label>
          <select v-model="templateForm.category" class="form-select">
            <option value="web">Web Application</option>
            <option value="api">REST & gRPC API</option>
            <option value="database">Database & Storage</option>
            <option value="worker">Worker / Cron</option>
            <option value="fullstack">Full-Stack</option>
          </select>
        </div>
        <div class="form-group flex-1">
          <label class="form-label">Framework</label>
          <input
            v-model="templateForm.framework"
            type="text"
            placeholder="e.g. rust-actix, nextjs"
            class="form-input"
          />
        </div>
      </div>

      <div class="form-group">
        <label class="form-label">Description</label>
        <textarea
          v-model="templateForm.description"
          rows="2"
          placeholder="Brief overview of what this scaffold provides..."
          class="form-textarea"
        ></textarea>
      </div>

      <div class="form-group">
        <label class="form-label">Tags (comma-separated)</label>
        <input
          v-model="templateForm.tagsInput"
          type="text"
          placeholder="rust, actix, microservice, high-perf"
          class="form-input"
        />
      </div>

      <!-- Variable Definition Builder -->
      <div class="variable-builder-section glass-panel">
        <div class="builder-header">
          <h4>Variables (User Fillable)</h4>
          <button class="btn-secondary btn-sm" @click="emit('add-variable')">
            ➕ Add Variable
          </button>
        </div>

        <div v-if="templateForm.variables.length === 0" class="empty-vars">
          No dynamic variables defined yet. Click "Add Variable" to create placeholders like <code>&#123;&#123;.app_name&#125;&#125;</code>.
        </div>

        <div
          v-for="(varItem, idx) in templateForm.variables"
          :key="idx"
          class="var-builder-row"
        >
          <input
            v-model="varItem.name"
            type="text"
            placeholder="name (e.g. port)"
            class="form-input"
          />
          <input
            v-model="varItem.label"
            type="text"
            placeholder="Label"
            class="form-input"
          />
          <select v-model="varItem.type" class="form-select">
            <option value="string">String</option>
            <option value="number">Number</option>
            <option value="boolean">Boolean</option>
            <option value="select">Select</option>
          </select>
          <input
            v-model="varItem.default"
            type="text"
            placeholder="Default"
            class="form-input"
          />
          <label class="checkbox-label">
            <input v-model="varItem.required" type="checkbox" />
            <span>Req</span>
          </label>
          <button
            class="btn-icon-action btn-icon-danger"
            title="Remove Variable"
            @click="emit('remove-variable', idx)"
          >
            🗑️
          </button>
        </div>
      </div>

      <!-- Manifest Editors -->
      <div class="manifest-editors-section">
        <div class="form-group">
          <label class="form-label">Kubernetes YAML Template</label>
          <textarea
            v-model="templateForm.manifest_yaml"
            rows="6"
            class="form-textarea code-textarea"
            placeholder="apiVersion: apps/v1&#10;kind: Deployment..."
          ></textarea>
        </div>

        <div class="form-group">
          <label class="form-label">Helm values.yaml Template</label>
          <textarea
            v-model="templateForm.helm_values"
            rows="4"
            class="form-textarea code-textarea"
            placeholder="replicaCount: {{.replicas}}..."
          ></textarea>
        </div>

        <div class="form-group">
          <label class="form-label">Docker Compose Template</label>
          <textarea
            v-model="templateForm.docker_compose"
            rows="4"
            class="form-textarea code-textarea"
            placeholder="version: '3.8'&#10;services:..."
          ></textarea>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn-secondary" @click="emit('close')">Cancel</button>
        <button
          class="btn-primary"
          :disabled="saving"
          @click="emit('save')"
        >
          {{ saving ? 'Saving Template...' : (mode === 'create' ? 'Create Template' : 'Update Template') }}
        </button>
      </div>
    </div>
  </ModalDrawer>
</template>
