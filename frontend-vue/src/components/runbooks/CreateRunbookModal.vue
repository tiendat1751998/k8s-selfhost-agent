<template>
  <ModalDrawer
    :show="show"
    :title="isEdit ? 'Edit Operational Runbook' : 'Create Operational Runbook'"
    max-width="640px"
    @close="$emit('close')"
  >
    <form class="modal-form" @submit.prevent="$emit('save')">
      <div class="form-group">
        <label>Runbook Title *</label>
        <input 
          v-model="runbook.title" 
          type="text" 
          placeholder="e.g., PostgreSQL Cluster Failover Recovery" 
          class="input-glass"
          required 
        />
      </div>

      <div class="form-row" style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
        <div class="form-group">
          <label>Category *</label>
          <select v-model="runbook.category" class="input-glass" required>
            <option value="Incident Response">Incident Response</option>
            <option value="Disaster Recovery">Disaster Recovery</option>
            <option value="Security Mitigation">Security Mitigation</option>
            <option value="Database Ops">Database Ops</option>
            <option value="Network Troubleshooting">Network Troubleshooting</option>
            <option value="Maintenance">Maintenance</option>
          </select>
        </div>

        <div class="form-group">
          <label>Author / Team</label>
          <input 
            v-model="runbook.author" 
            type="text" 
            placeholder="e.g., Platform SRE Team" 
            class="input-glass" 
          />
        </div>
      </div>

      <div class="form-group">
        <label>Tags (comma separated)</label>
        <input 
          :value="tagInput" 
          type="text" 
          placeholder="database, postgres, recovery" 
          class="input-glass"
          @input="$emit('update:tagInput', ($event.target as HTMLInputElement).value)" 
        />
      </div>

      <div class="form-group">
        <label>Runbook Steps &amp; Diagnostic Commands *</label>
        <textarea 
          v-model="runbook.content" 
          rows="6" 
          placeholder="1. Verify workload status:
`kubectl get pods -A`
2. Inspect crash logs:
`kubectl logs ...`" 
          class="input-glass font-mono"
          style="resize: vertical;"
          required
        ></textarea>
        <span class="field-hint">Enclose automated diagnostic commands in backticks (`command`) to enable 1-click execution.</span>
      </div>

      <div class="modal-actions">
        <button type="button" class="btn btn-secondary" @click="$emit('close')">Cancel</button>
        <button type="submit" class="btn btn-primary">
          <span>{{ isEdit ? '💾 Save Changes' : '+ Publish Runbook' }}</span>
        </button>
      </div>
    </form>
  </ModalDrawer>
</template>

<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'

defineProps<{
  show: boolean
  isEdit?: boolean
  runbook: {
    title: string
    category: string
    content: string
    author?: string
    tags?: string[]
  }
  tagInput: string
}>()

defineEmits<{
  (e: 'close'): void
  (e: 'save'): void
  (e: 'update:tagInput', val: string): void
}>()
</script>