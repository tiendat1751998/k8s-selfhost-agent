<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { BackupJob } from '../../api/governance'

const props = defineProps<{
  modelValue: boolean
  completedJobs: BackupJob[]
  initialJobId?: string
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'restore', params: {
    backup_job_id: string
    target_db_host: string
    target_db_name: string
  }): void
}>()

const form = reactive({
  backup_job_id: '',
  target_db_host: 'postgres.db.svc.cluster.local:5432',
  target_db_name: 'app_db',
})

watch(() => props.modelValue, (isOpen) => {
  if (isOpen) {
    if (props.initialJobId) {
      form.backup_job_id = props.initialJobId
    } else if (props.completedJobs.length > 0 && !form.backup_job_id) {
      form.backup_job_id = props.completedJobs[0].id
    }
  }
})

watch(() => props.initialJobId, (newId) => {
  if (newId) {
    form.backup_job_id = newId
  }
})

function formatDate(d?: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleString()
  } catch {
    return d
  }
}

function handleSubmit() {
  if (!form.backup_job_id) return
  emit('restore', { ...form })
}
</script>

<template>
  <div v-if="modelValue" class="modal-overlay" @click.self="emit('update:modelValue', false)">
    <div class="modal-card glass-panel animate-fade-in">
      <div class="modal-header">
        <div class="modal-title-group">
          <span class="badge badge-rose">POINT-IN-TIME RESTORE</span>
          <h3 class="modal-title">Restore Database from Snapshot</h3>
        </div>
        <button class="modal-close" @click="emit('update:modelValue', false)">✕</button>
      </div>

      <form class="modal-body" @submit.prevent="handleSubmit">
        <div class="alert-box alert-warning">
          <strong>⚠️ Caution:</strong> Restore will replay WAL/transaction logs and replace contents in the target database.
        </div>

        <div class="form-group">
          <label class="form-label">Select Backup Snapshot Job:</label>
          <select v-model="form.backup_job_id" required class="input-glass font-mono">
            <option value="" disabled>-- Select a completed backup job --</option>
            <option v-for="j in completedJobs" :key="j.id" :value="j.id">
              Job #{{ j.id.slice(0, 8) }} (Policy #{{ j.policy_id.slice(0, 8) }} - {{ formatDate(j.started_at || j.created_at) }})
            </option>
          </select>
        </div>

        <div class="form-group">
          <label class="form-label">Target DB Host:</label>
          <input
            v-model="form.target_db_host"
            type="text"
            required
            class="input-glass font-mono"
            placeholder="postgres.db.svc.cluster.local:5432"
          />
        </div>

        <div class="form-group">
          <label class="form-label">Target DB Name:</label>
          <input
            v-model="form.target_db_name"
            type="text"
            required
            class="input-glass font-mono"
            placeholder="app_db_restored"
          />
        </div>

        <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
          <button type="button" class="btn btn-secondary" @click="emit('update:modelValue', false)">Cancel</button>
          <button type="submit" class="btn btn-danger" :disabled="loading || !form.backup_job_id">
            <span>{{ loading ? 'Executing Restore...' : '⚡ Execute Instant Restore' }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
