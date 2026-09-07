<script setup lang="ts">
import type { BackupJob } from '../../api/governance'
import type { RestoreParams } from '../../composables/useBackupRestore'

defineProps<{
  modelValue: boolean
  form: RestoreParams
  completedJobs: BackupJob[]
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'submit'): void
}>()

function formatDate(d?: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleString()
  } catch {
    return d
  }
}
</script>

<template>
  <div v-if="modelValue" class="drawer-overlay" @click.self="emit('update:modelValue', false)">
    <div class="drawer-panel animate-fade-in">
      <div class="drawer-header">
        <div class="modal-title-group">
          <span class="badge badge-rose">POINT-IN-TIME RECOVERY & REMAP WIZARD</span>
          <h3 class="modal-title font-mono">Disaster Recovery Execution</h3>
        </div>
        <button class="modal-close" @click="emit('update:modelValue', false)">✕</button>
      </div>

      <form class="drawer-body" @submit.prevent="emit('submit')">
        <!-- Safety Alert -->
        <div class="alert-box alert-warning">
          <strong>⚠️ High Impact Action:</strong> Restore will replay transaction WAL records and overwrite contents of the target database instance.
        </div>

        <!-- Section 1: Source Snapshot Selection -->
        <div class="drawer-section">
          <span class="drawer-section-title">📦 Source Snapshot Selection</span>
          <div class="form-group">
            <label class="form-label">Verified Backup Snapshot:</label>
            <select v-model="form.backup_job_id" required class="input-glass font-mono">
              <option value="" disabled>-- Select a completed backup job --</option>
              <option v-for="j in completedJobs" :key="j.id" :value="j.id">
                Job #{{ j.id.slice(0, 8) }} (Policy #{{ j.policy_id.slice(0, 8) }} - {{ formatDate(j.started_at || j.created_at) }})
              </option>
            </select>
          </div>
        </div>

        <!-- Section 2: Point-in-Time Recovery Timestamp -->
        <div class="drawer-section">
          <span class="drawer-section-title">⏱️ Point-in-Time (PITR) Recovery Boundary</span>
          <div class="form-group">
            <label class="form-label">Recovery Timestamp (ISO 8601):</label>
            <input 
              v-model="form.pitr_timestamp" 
              type="text" 
              class="input-glass font-mono" 
              placeholder="2026-08-28T12:00:00Z (Leave blank for latest LSN)" 
            />
          </div>
        </div>

        <!-- Section 3: Target Database & Host Remap -->
        <div class="drawer-section">
          <span class="drawer-section-title">🎯 Target Destination & Namespace Remapping</span>
          
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

          <div class="form-group">
            <label class="form-label">Target Kubernetes Namespace:</label>
            <input 
              v-model="form.target_namespace" 
              type="text" 
              class="input-glass font-mono" 
              placeholder="default" 
            />
          </div>

          <div class="form-group" style="flex-direction: row; align-items: center; gap: 10px; margin-top: 4px;">
            <input 
              id="dry-run-check" 
              v-model="form.dry_run" 
              type="checkbox" 
              style="accent-color: var(--accent-cyan, #06b6d4);" 
            />
            <label for="dry-run-check" class="form-label" style="margin-bottom: 0; cursor: pointer;">
              Dry Run Simulation (Validate WAL integrity without writing to disk)
            </label>
          </div>
        </div>

        <div class="drawer-footer" style="background: transparent; border-top: none; padding: 0;">
          <button type="button" class="btn btn-secondary" @click="emit('update:modelValue', false)">
            Cancel
          </button>
          <button 
            type="submit" 
            class="btn btn-danger" 
            :disabled="loading || !form.backup_job_id"
          >
            <span>{{ loading ? 'Executing PITR Restore...' : '⚡ Execute Instant Restore' }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
