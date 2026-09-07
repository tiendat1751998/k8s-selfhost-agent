<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { BackupStorage } from '../../api/governance'

const props = defineProps<{
  modelValue: boolean
  storages: BackupStorage[]
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'create', policy: {
    name: string
    db_type: string
    db_host: string
    db_port: number
    db_name: string
    storage_id: string
    schedule: string
    retention_count: number
    backup_type: string
    enabled: boolean
  }): void
}>()

const form = reactive({
  name: '',
  db_type: 'postgres',
  db_host: 'localhost',
  db_port: 5432,
  db_name: 'app_production',
  storage_id: 'default-s3',
  schedule: '0 */6 * * *',
  retention_count: 14,
  backup_type: 'full',
  enabled: true,
})

watch(() => props.modelValue, (isOpen) => {
  if (isOpen) {
    form.name = ''
    form.db_name = 'app_production'
    form.db_host = 'localhost'
    form.db_port = 5432
  }
})

function handleSubmit() {
  emit('create', { ...form })
}
</script>

<template>
  <div v-if="modelValue" class="modal-overlay" @click.self="emit('update:modelValue', false)">
    <div class="modal-card glass-panel animate-fade-in">
      <div class="modal-header">
        <div class="modal-title-group">
          <span class="badge badge-cyan">SCHEDULED PROTECTION</span>
          <h3 class="modal-title">Create Database Backup Policy</h3>
        </div>
        <button class="modal-close" @click="emit('update:modelValue', false)">✕</button>
      </div>

      <form class="modal-body" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label class="form-label">Policy Name:</label>
          <input
            v-model="form.name"
            type="text"
            required
            class="input-glass"
            placeholder="e.g. production-postgres-main"
          />
        </div>

        <div class="form-group-row">
          <div class="form-group" style="flex: 1;">
            <label class="form-label">Database Type:</label>
            <select v-model="form.db_type" class="input-glass">
              <option value="postgres">PostgreSQL</option>
              <option value="mysql">MySQL</option>
              <option value="mariadb">MariaDB</option>
              <option value="mongodb">MongoDB</option>
              <option value="redis">Redis</option>
              <option value="nats">NATS JetStream</option>
            </select>
          </div>
          <div class="form-group" style="flex: 1;">
            <label class="form-label">Storage Target:</label>
            <select v-model="form.storage_id" class="input-glass">
              <option value="default-s3">Default S3 / MinIO</option>
              <option v-for="s in storages" :key="s.id" :value="s.id">{{ s.name }} ({{ s.type }})</option>
            </select>
          </div>
        </div>

        <div class="form-group-row">
          <div class="form-group" style="flex: 3;">
            <label class="form-label">DB Host:</label>
            <input
              v-model="form.db_host"
              type="text"
              required
              class="input-glass"
              placeholder="postgres.db.svc.cluster.local"
            />
          </div>
          <div class="form-group" style="flex: 1;">
            <label class="form-label">DB Port:</label>
            <input
              v-model.number="form.db_port"
              type="number"
              required
              class="input-glass font-mono"
              placeholder="5432"
            />
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Database Name:</label>
          <input
            v-model="form.db_name"
            type="text"
            required
            class="input-glass"
            placeholder="app_db"
          />
        </div>

        <div class="form-group-row">
          <div class="form-group" style="flex: 2;">
            <label class="form-label">Cron Schedule:</label>
            <input
              v-model="form.schedule"
              type="text"
              required
              class="input-glass font-mono"
              placeholder="0 */6 * * *"
            />
          </div>
          <div class="form-group" style="flex: 1;">
            <label class="form-label">Retention Count:</label>
            <input
              v-model.number="form.retention_count"
              type="number"
              min="1"
              max="100"
              class="input-glass font-mono"
              placeholder="14"
            />
          </div>
        </div>

        <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
          <button type="button" class="btn btn-secondary" @click="emit('update:modelValue', false)">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="loading">
            <span>{{ loading ? 'Saving...' : 'Arm & Enable Policy' }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
