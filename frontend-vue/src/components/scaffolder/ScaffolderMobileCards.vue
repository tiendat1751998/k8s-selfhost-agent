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
  <div class="mobile-cards-stream">
    <div v-if="loading && templates.length === 0" class="loading-state glass-panel">
      <div class="spinner"></div>
      <p>Loading templates...</p>
    </div>

    <div v-else-if="templates.length === 0" class="empty-state glass-panel">
      <div class="empty-icon">📦</div>
      <h3>No templates</h3>
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
          class="btn-mobile-deploy"
          title="1-Click Deploy"
          @click="emit('deploy', tmpl)"
        >
          🚀 Deploy
        </button>

        <template v-if="!tmpl.built_in">
          <button
            type="button"
            class="btn-icon-action btn-sm"
            title="Edit"
            aria-label="Edit Template"
            @click="emit('edit', tmpl)"
          >
            ✏️
          </button>
          <button
            type="button"
            class="btn-icon-action btn-icon-danger btn-sm"
            title="Delete"
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
