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
  (e: 'preview', template: Template): void
  (e: 'edit', template: Template): void
  (e: 'delete', template: Template): void
  (e: 'reset-filters'): void
}>()
</script>

<template>
  <div class="mobile-cards-stream">
    <div v-if="loading && templates.length === 0" class="loading-state glass-panel">
      <div class="spinner"></div>
      <p>Loading templates...</p>
    </div>

    <div v-else-if="templates.length === 0" class="mobile-empty-state glass-panel">
      <div class="empty-icon">🏗️</div>
      <p class="empty-title">No scaffolder templates found matching filters.</p>
      <p class="empty-hint">Tap ➕ to register a template.</p>
      <button class="btn btn-secondary btn-sm mt-2" @click="emit('reset-filters')">
        Reset Filters
      </button>
    </div>

    <div
      v-for="tmpl in templates"
      :key="tmpl.id"
      class="mobile-card-item glass-panel"
      @click="emit('deploy', tmpl)"
    >
      <div class="mobile-card-left">
        <div class="mobile-avatar">
          {{ getFrameworkIcon(tmpl.framework) }}
        </div>
        <div class="mobile-details">
          <span class="mobile-title font-mono">{{ tmpl.name }}</span>
          <div class="mobile-meta font-mono">
            <span :class="['badge', getCategoryBadgeClass(tmpl.category)]">
              {{ tmpl.category }}
            </span>
            <span class="mobile-framework-tag">{{ tmpl.framework }}</span>
          </div>
        </div>
      </div>

      <div class="mobile-card-right" @click.stop>
        <button
          type="button"
          class="btn-icon-action"
          title="Use Template"
          aria-label="Use Template"
          @click="emit('deploy', tmpl)"
        >
          🚀
        </button>

        <button
          type="button"
          class="btn-icon-action"
          title="Preview Template"
          aria-label="Preview Template"
          @click="emit('preview', tmpl)"
        >
          👁️
        </button>

        <template v-if="!tmpl.built_in">
          <button
            type="button"
            class="btn-icon-action"
            title="Edit Template"
            aria-label="Edit Template"
            @click="emit('edit', tmpl)"
          >
            ✏️
          </button>
          <button
            type="button"
            class="btn-icon-action btn-icon-danger"
            title="Delete Template"
            aria-label="Delete Template"
            :disabled="deleting"
            @click="emit('delete', tmpl)"
          >
            🗑️
          </button>
        </template>
      </div>
    </div>
  </div>
</template>
