<template>
  <div class="mobile-only-stream">
    <div v-if="runbooks.length > 0" class="runbooks-mobile-cards">
      <div 
        v-for="rb in runbooks" 
        :key="rb.id" 
        class="rb-mobile-card mobile-rb-card glass-panel"
      >
        <div class="rb-mobile-icon mobile-rb-icon">
          <BaseIcon :name="getCategoryIcon(rb.category)" size="xs" />
        </div>

        <div class="rb-mobile-info mobile-rb-info">
          <div class="rb-mobile-header mobile-rb-header">
            <span class="badge badge-cyan font-mono" style="font-size: 9px;">{{ rb.category.toUpperCase() }}</span>
          </div>
          <h4 class="rb-mobile-title mobile-rb-title" :title="rb.title">{{ rb.title }}</h4>
          <div class="rb-mobile-meta mobile-rb-meta font-mono">
            <span>Steps: <strong class="text-cyan">{{ rb.steps_count || 3 }}</strong></span>
            <span class="meta-sep">·</span>
            <span class="text-secondary text-truncate">{{ rb.author || 'Platform SRE' }}</span>
          </div>
        </div>

        <div class="rb-mobile-actions mobile-rb-actions">
          <button 
            class="rb-action-btn btn-run-action"
            :disabled="executingId === rb.id"
            title="Execute Runbook"
            aria-label="Execute Runbook"
            @click="$emit('execute', rb)"
          >
            <BaseIcon :name="executingId === rb.id ? 'clock' : 'zap'" size="xs" />
          </button>
          <button 
            class="rb-action-btn btn-steps-action"
            title="Inspect Procedure Steps"
            aria-label="Inspect Procedure Steps"
            @click="$emit('inspect', rb)"
          >
            <BaseIcon name="eye" size="xs" />
          </button>
          <button 
            class="rb-action-btn btn-edit-action"
            title="Edit Runbook"
            aria-label="Edit Runbook"
            @click="$emit('edit', rb)"
          >
            <BaseIcon name="sliders" size="xs" />
          </button>
          <button 
            class="rb-action-btn btn-delete-action btn-delete-crimson"
            title="Delete Runbook"
            aria-label="Delete Runbook"
            @click="$emit('delete', rb.id)"
          >
            <BaseIcon name="trash" size="xs" />
          </button>
        </div>
      </div>
    </div>

    <div v-else class="runbooks-mobile-empty glass-panel font-mono">
      <span class="mobile-empty-icon"><BaseIcon name="book-open" size="lg" /></span>
      <p class="mobile-empty-text">No runbooks cataloged yet. Tap + to author your first operational runbook.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Runbook } from '../../api/governance'
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  runbooks: Runbook[]
  executingId: string | null
  getCategoryIcon: (cat: string) => string
  formatDate: (d?: string) => string
}>()

defineEmits<{
  (e: 'execute', rb: Runbook): void
  (e: 'inspect', rb: Runbook): void
  (e: 'edit', rb: Runbook): void
  (e: 'delete', id: string): void
}>()
</script>
