<template>
  <div v-if="runbooks.length > 0" class="runbooks-grid">
    <div 
      v-for="rb in runbooks" 
      :key="rb.id" 
      class="runbook-card glass-panel glass-panel-glow"
    >
      <div class="rb-header">
        <div class="rb-icon-box">{{ getCategoryIcon(rb.category) }}</div>
        <div class="rb-title-group">
          <div class="rb-badge-row">
            <span class="rb-category font-mono">{{ rb.category.toUpperCase() }}</span>
            <span class="badge badge-cyan font-mono" style="font-size: 9.5px;">⚡ Auto-Remediation</span>
          </div>
          <h3 class="rb-title">{{ rb.title }}</h3>
        </div>
      </div>

      <div class="rb-tags-row">
        <span v-for="tag in (rb.tags || [])" :key="tag" class="tag-pill font-mono">#{{ tag }}</span>
      </div>

      <div class="rb-body">
        <div class="rb-meta-row font-mono">
          <span>Author: <strong class="text-secondary">{{ rb.author || 'SRE Team' }}</strong></span>
          <span>Steps: <strong class="text-cyan">{{ rb.steps_count || 3 }}</strong></span>
        </div>
        <div class="rb-meta-row font-mono text-muted" style="font-size: 10px;">
          <span>Last used: {{ rb.last_used_at ? formatDate(rb.last_used_at) : 'Not recorded' }}</span>
        </div>
      </div>

      <div class="rb-actions">
        <button 
          class="btn btn-primary" 
          :disabled="executingId === rb.id"
          title="Open parameter & target execution modal"
          @click="$emit('execute', rb)"
        >
          <span>{{ executingId === rb.id ? '⚡ Running...' : '⚡ Run' }}</span>
        </button>
        <button class="btn btn-secondary" style="flex: 1;" title="Inspect DAG Steps" @click="$emit('inspect', rb)">
          <span>🔍 Inspect</span>
        </button>
        <button class="btn btn-secondary btn-sm" title="Edit Runbook" @click="$emit('edit', rb)">
          <span>⚙️ Edit</span>
        </button>
        <button class="btn btn-danger btn-sm btn-delete-crimson" title="Delete Runbook" @click="$emit('delete', rb.id)">
          <span>🗑 Delete</span>
        </button>
      </div>
    </div>
  </div>

  <div v-else class="empty-state-box glass-panel">
    <span class="empty-icon">📖</span>
    <h3 class="empty-title">No Runbooks Discovered</h3>
    <p class="empty-desc">Create your first operational runbook with step-by-step diagnostic and remediation commands.</p>
    <button class="btn btn-primary" @click="$emit('create')">
      <span>+ Create Runbook</span>
    </button>
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
  (e: 'create'): void
}>()
</script>
