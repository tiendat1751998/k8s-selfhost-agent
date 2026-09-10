<template>
  <div class="section-card glass-panel desktop-only-table">
    <div class="section-top">
      <div>
        <h2 class="section-title">Standard Operating Playbooks & Procedures</h2>
        <span class="section-subtitle">Catalog of automated remediation runbooks with step-by-step verification commands</span>
      </div>
      <span class="badge badge-cyan">{{ runbooks.length }} Playbooks</span>
    </div>

    <div class="runbooks-table-container">
      <table class="runbooks-table data-table">
        <colgroup>
          <col style="width: 200px;" />
          <col style="width: 130px;" />
          <col style="width: 100px;" />
          <col style="width: 130px;" />
          <col style="width: 150px;" />
          <col style="width: 120px;" />
          <col style="width: 170px;" />
        </colgroup>
        <thead>
          <tr>
            <th class="col-title">Runbook Title</th>
            <th class="col-category">Category</th>
            <th class="col-steps">Steps</th>
            <th class="col-author">Author</th>
            <th class="col-tags">Tags</th>
            <th class="col-updated">Last Executed</th>
            <th class="col-actions" style="text-align: right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rb in runbooks" :key="rb.id" class="rb-table-row">
            <td class="col-title">
              <div class="runbook-table-title-cell" :title="rb.title">
                <span class="table-rb-title">{{ rb.title }}</span>
                <span class="table-rb-sub font-mono text-muted">ID: #{{ rb.id.slice(0, 8) }}</span>
              </div>
            </td>
            <td class="col-category">
              <div class="table-category-cell" :title="rb.category">
                <span class="table-cat-icon">{{ getCategoryIcon(rb.category) }}</span>
                <span class="badge badge-cyan font-mono text-truncate">{{ rb.category.toUpperCase() }}</span>
              </div>
            </td>
            <td class="col-steps">
              <span class="font-mono text-cyan font-semibold">{{ rb.steps_count || 3 }} steps</span>
            </td>
            <td class="col-author">
              <span class="font-mono text-secondary text-truncate" :title="rb.author || 'Platform SRE'">
                {{ rb.author || 'Platform SRE' }}
              </span>
            </td>
            <td class="col-tags">
              <div class="table-tags-cell">
                <span 
                  v-for="tag in (rb.tags || []).slice(0, 2)" 
                  :key="tag" 
                  class="tag-pill font-mono" 
                  :title="'#' + tag"
                >
                  #{{ tag }}
                </span>
                <span 
                  v-if="(rb.tags || []).length > 2" 
                  class="tag-pill font-mono text-muted" 
                  :title="(rb.tags || []).slice(2).join(', ')"
                >
                  +{{ (rb.tags || []).length - 2 }}
                </span>
                <span v-if="!rb.tags || rb.tags.length === 0" class="text-muted font-mono" style="font-size: 11px;">—</span>
              </div>
            </td>
            <td class="col-updated">
              <span class="font-mono text-muted" style="font-size: 11.5px;">
                {{ rb.last_used_at ? formatDate(rb.last_used_at) : 'Never' }}
              </span>
            </td>
            <td class="col-actions">
              <div class="table-actions-cell">
                <button 
                  class="table-action-btn btn-run-action" 
                  :disabled="executingId === rb.id"
                  title="Execute Runbook"
                  aria-label="Execute Runbook"
                  @click="$emit('execute', rb)"
                >
                  <span>{{ executingId === rb.id ? '⏳' : '⚡' }}</span>
                </button>
                <button 
                  class="table-action-btn btn-steps-action"
                  title="Inspect Procedure Steps"
                  aria-label="Inspect Procedure Steps"
                  @click="$emit('inspect', rb)"
                >
                  <span>🔍</span>
                </button>
                <button 
                  class="table-action-btn btn-edit-action"
                  title="Edit Runbook"
                  aria-label="Edit Runbook"
                  @click="$emit('edit', rb)"
                >
                  <span>⚙️</span>
                </button>
                <button 
                  class="table-action-btn btn-delete-action btn-delete-crimson"
                  title="Delete Runbook"
                  aria-label="Delete Runbook"
                  @click="$emit('delete', rb.id)"
                >
                  <span>🗑</span>
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="runbooks.length === 0">
            <td colspan="7" class="text-center text-muted" style="padding: 32px;">
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
