<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'

defineProps<{
  show: boolean
  importing?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'import', formData: FormData, name: string): void
}>()

const importMode = ref<'file' | 'text'>('file')
const importForm = ref({ name: '', id: '', group: '', region: '', provider: 'custom', kubeconfigRaw: '' })
const kubeconfigFile = ref<File | null>(null)

function handleFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (file) kubeconfigFile.value = file
}

function submit() {
  if (!importForm.value.name.trim()) return
  const fd = new FormData()
  fd.append('name', importForm.value.name)
  if (importForm.value.id) fd.append('id', importForm.value.id)
  if (importForm.value.group) fd.append('group', importForm.value.group)
  if (importForm.value.region) fd.append('region', importForm.value.region)
  if (importForm.value.provider) fd.append('provider', importForm.value.provider)
  if (importMode.value === 'file' && kubeconfigFile.value) fd.append('kubeconfig', kubeconfigFile.value)
  else if (importForm.value.kubeconfigRaw) fd.append('kubeconfig_raw', importForm.value.kubeconfigRaw)
  emit('import', fd, importForm.value.name)
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Import Kubernetes Cluster"
    subtitle="Connect an existing cluster via kubeconfig"
    max-width="580px"
    @close="$emit('close')"
  >
    <div class="import-modal-body">
      <div class="form-group">
        <label class="form-label required">Cluster Name / Alias</label>
        <input v-model="importForm.name" type="text" placeholder="e.g. prod-gke-us-east1" class="input-glass font-mono" />
      </div>
      <div class="form-group">
        <label class="form-label">Cluster Group</label>
        <input v-model="importForm.group" type="text" placeholder="e.g. production" class="input-glass font-mono" />
      </div>
      <div class="form-group">
        <label class="form-label">Region / Location</label>
        <input v-model="importForm.region" type="text" placeholder="e.g. us-east-1" class="input-glass font-mono" />
      </div>
      <div class="form-group">
        <label class="form-label">Kubeconfig Source</label>
        <div class="source-toggle">
          <button type="button" class="btn btn-xs" :class="importMode === 'file' ? 'btn-primary' : 'btn-secondary'" @click="importMode = 'file'">File Upload</button>
          <button type="button" class="btn btn-xs" :class="importMode === 'text' ? 'btn-primary' : 'btn-secondary'" @click="importMode = 'text'">Raw YAML</button>
        </div>
      </div>
      <div v-if="importMode === 'file'" class="form-group">
        <input type="file" accept=".yaml,.yml,.config,.kubeconfig,text/*" class="input-glass file-input font-mono" @change="handleFileChange" />
      </div>
      <div v-else class="form-group">
        <textarea v-model="importForm.kubeconfigRaw" class="input-glass font-mono text-area-lg" rows="8" placeholder="apiVersion: v1&#10;clusters:&#10;  - cluster:&#10;      ..."></textarea>
      </div>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="importing" @click="$emit('close')">Cancel</button>
      <button type="button" class="btn btn-primary" :disabled="importing || !importForm.name.trim()" @click="submit">
        <span>{{ importing ? 'Importing...' : '+ Import Cluster' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
