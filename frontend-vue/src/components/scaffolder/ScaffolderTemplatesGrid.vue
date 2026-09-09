<script setup lang="ts">
import type { Template } from '../../api/scaffold'
import { getFrameworkIcon, getCategoryBadgeClass } from '../../composables/useScaffolder'

defineProps<{
  templates: Template[]
  loading: boolean
  deleting: boolean
}>()

const emit = defineEmits<{
  (e: 'deploy', template: Template): void
  (e: 'edit', template: Template): void
  (e: 'delete', template: Template): void
  (e: 'reset-filters'): void
}>()
</script>

<template>
  <div class="gallery-container">
    <div v-if="loading && templates.length === 0" class="loading-state glass-panel">
      <div class="spinner"></div>
      <p>Loading application templates...</p>
    </div>

    <div v-else-if="templates.length === 0" class="empty-state glass-panel">
      <div class="empty-icon">📦</div>
      <h3>No templates found</h3>
      <p>Try adjusting your category filter or search query.</p>
      <button class="btn-secondary mt-3" @click="emit('reset-filters')">
        Reset Filters
      </button>
    </div>

    <!-- Templates Grid -->
    <div v-else class="templates-grid">
      <div
        v-for="tmpl in templates"
        :key="tmpl.id"
        class="template-card glass-panel"
      >
        <div class="card-header">
          <div class="framework-avatar">
            {{ getFrameworkIcon(tmpl.framework) }}
          </div>
          <div class="card-badges">
            <span :class="['badge', getCategoryBadgeClass(tmpl.category)]">
              {{ tmpl.category.toUpperCase() }}
            </span>
            <span v-if="tmpl.built_in" class="badge badge-system">
              SYSTEM
            </span>
            <span v-else class="badge badge-custom">
              CUSTOM
            </span>
          </div>
        </div>

        <div class="card-body">
          <h3 class="template-name">{{ tmpl.name }}</h3>
          <p class="template-framework">{{ tmpl.framework }}</p>
          <p class="template-desc">{{ tmpl.description }}</p>

          <div class="template-tags">
            <span v-for="tag in tmpl.tags" :key="tag" class="tag-pill">
              #{{ tag }}
            </span>
          </div>
        </div>

        <div class="card-footer">
          <button class="btn-deploy" @click="emit('deploy', tmpl)">
            <span class="btn-icon">🚀</span> 1-Click Deploy
          </button>

          <div v-if="!tmpl.built_in" class="custom-actions">
            <button
              class="btn-icon-action"
              title="Edit Custom Template"
              @click="emit('edit', tmpl)"
            >
              ✏️
            </button>
            <button
              class="btn-icon-action btn-icon-danger"
              title="Delete Template"
              :disabled="deleting"
              @click="emit('delete', tmpl)"
            >
              🗑️
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
