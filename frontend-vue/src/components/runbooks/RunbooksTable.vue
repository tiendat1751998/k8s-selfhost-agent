<template>
  <div class="section-card glass-panel">
    <div class="section-top">
      <div>
        <h2 class="section-title">Standard Operating Playbooks & Procedures</h2>
        <span class="section-subtitle">Catalog of automated remediation runbooks with step-by-step verification commands</span>
      </div>
      <span class="badge badge-cyan">{{ runbooks.length }} Playbooks</span>
    </div>

    <div class="runbooks-table-container">
      <table class="runbooks-table">
        <thead>
          <tr>
            <th>Runbook Title & Author</th>
            <th>Category</th>
            <th>Steps</th>
            <th>Automation Engine</th>
            <th>Last Executed</th>
            <th style="text-align: right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rb in runbooks" :key="rb.id" class="rb-table-row">
            <td>
              <div class="runbook-table-title-cell">
                <span class="table-rb-title">{{ rb.title }}</span>
                <div class="table-rb-sub font-mono">
                  <span class="text-secondary">{{ rb.author || 'Platform SRE' }}</span>
                  <span class="text-muted">• ID: #{{ rb.id.slice(0, 8) }}</span>
                </div>
              </div>
            </td>
            <td>
              <div class="table-category-cell">
                <span class="table-cat-icon">{{ getCategoryIcon(rb.category) }}</span>
                <span class="badge badge-cyan font-mono">{{ rb.category.toUpperCase() }}</span>
              </div>
            </td>
            <td>
              <span class="font-mono text-cyan font-semibold">{{ rb.steps_count || 3 }} steps</span>
            </td>
            <td>
              <span class="badge badge-emerald font-mono">⚡ 1-Click DAG</span>
            </td>
            <td>
              <span class="font-mono text-muted" style="font-size: 11.5px;">
                {{ rb.last_used_at ? formatDate(rb.last_used_at) : 'Never' }}
              </span>
            </td>
            <td>
              <div class="table-actions-cell">
                <button 
                  class="btn btn-primary btn-sm" 
                  :disabled="executingId === rb.id"
                  title="Execute Playbook"
                  @click="$emit('execute', rb)"
                >
                  <span>{{ executingId === rb.id ? '⚡ Running...' : '⚡ Execute' }}</span>
                </button>
                <button 
                  class="btn btn-secondary btn-sm"
                  title="Inspect Procedure Steps"
                  @click="$emit('inspect', rb)"
                >
                  <span>🔍 Steps</span>
                </button>
                <button 
                  class="btn btn-secondary btn-sm"
                  title="Edit Runbook"
                  @click="$emit('edit', rb)"
                >
                  <span>⚙️ Edit</span>
                </button>
                <button 
                  class="btn btn-danger btn-sm btn-delete-crimson"
                  title="Delete Runbook"
                  @click="$emit('delete', rb.id)"
                >
                  <span>🗑 Delete</span>
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="runbooks.length === 0">
            <td colspan="6" class="text-center text-muted" style="padding: 32px;">
              No operational runbooks found.
            </td>
          </tr>
        </tbody>
      </table>
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