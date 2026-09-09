<template>
  <div v-if="runbooks.length > 0" class="runbooks-mobile-cards">
    <div 
      v-for="rb in runbooks" 
      :key="rb.id" 
      class="rb-mobile-card glass-panel"
    >
      <div class="rb-mobile-top">
        <div class="rb-mobile-header-left">
          <span class="rb-mobile-icon">{{ getCategoryIcon(rb.category) }}</span>
          <div class="rb-mobile-titles">
            <span class="badge badge-cyan font-mono" style="font-size: 9px;">{{ rb.category.toUpperCase() }}</span>
            <h4 class="rb-mobile-title">{{ rb.title }}</h4>
          </div>
        </div>
      </div>

      <div class="rb-mobile-meta font-mono">
        <span>Steps: <strong class="text-cyan">{{ rb.steps_count || 3 }}</strong></span>
        <span>Author: <strong class="text-secondary">{{ rb.author || 'Platform SRE' }}</strong></span>
      </div>

      <div class="rb-mobile-actions">
        <button 
          class="btn btn-primary btn-xs"
          :disabled="executingId === rb.id"
          @click="$emit('execute', rb)"
        >
          <span>{{ executingId === rb.id ? '⚡ Running...' : '⚡ Run' }}</span>
        </button>
        <button 
          class="btn btn-secondary btn-xs"
          @click="$emit('inspect', rb)"
        >
          <span>🔍 Steps</span>
        </button>
        <button 
          class="btn btn-secondary btn-xs"
          @click="$emit('edit', rb)"
        >
          <span>⚙️ Edit</span>
        </button>
        <button 
          class="btn btn-danger btn-xs btn-delete-crimson"
          @click="$emit('delete', rb.id)"
        >
          <span>🗑 Delete</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Runbook } from '../../api/governance'

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
