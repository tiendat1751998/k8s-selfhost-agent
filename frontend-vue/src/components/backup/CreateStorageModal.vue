<script setup lang="ts">
defineProps<{
  modelValue: boolean
  form: {
    name: string
    type: string
    endpoint: string
    bucket: string
  }
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'submit'): void
}>()
</script>

<template>
  <div v-if="modelValue" class="modal-overlay" @click.self="emit('update:modelValue', false)">
    <div class="modal-card glass-panel animate-fade-in">
      <div class="modal-header">
        <div class="modal-title-group">
          <span class="badge badge-emerald">STORAGE REPOSITORY</span>
          <h3 class="modal-title">Attach Storage Target</h3>
        </div>
        <button class="modal-close" @click="emit('update:modelValue', false)">✕</button>
      </div>
      <form class="modal-body" @submit.prevent="emit('submit')">
        <div class="form-group">
          <label class="form-label">Storage Name:</label>
          <input v-model="form.name" type="text" required class="input-glass" placeholder="e.g. minio-cluster-backup" />
        </div>
        <div class="form-group">
          <label class="form-label">Storage Type:</label>
          <select v-model="form.type" class="input-glass">
            <option value="s3">AWS S3 / Compatible</option>
            <option value="minio">MinIO Object Store</option>
            <option value="local">Local NVMe Volume</option>
            <option value="nfs">Network File System (NFS)</option>
          </select>
        </div>
        <div class="form-group">
          <label class="form-label">Endpoint URL:</label>
          <input v-model="form.endpoint" type="text" class="input-glass font-mono" placeholder="https://s3.us-east-1.amazonaws.com or minio.storage.svc:9000" />
        </div>
        <div class="form-group">
          <label class="form-label">Bucket / Volume Name:</label>
          <input v-model="form.bucket" type="text" required class="input-glass font-mono" placeholder="k8s-database-backups" />
        </div>
        <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
          <button type="button" class="btn btn-secondary" @click="emit('update:modelValue', false)">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="loading">
            <span>{{ loading ? 'Attaching...' : 'Attach Storage' }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
