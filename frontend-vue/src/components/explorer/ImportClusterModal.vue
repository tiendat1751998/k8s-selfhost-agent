<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { fleetApi } from '../../api/compute'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'imported', name: string): void
  (e: 'toast', msg: string, type?: 'success' | 'error'): void
}>()

const importMode = ref<'file' | 'text'>('file')
const importForm = ref({
  id: '',
  name: '',
  group: 'production',
  region: 'us-east-1',
  provider: 'k8s',
  kubeconfigRaw: '',
})
const kubeconfigFile = ref<File | null>(null)
const importingCluster = ref(false)

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    kubeconfigFile.value = target.files[0]
  }
}

async function handleImportCluster() {
  if (!importForm.value.name.trim()) {
    emit('toast', 'Cluster name is required', 'error')
    return
  }

  let fileToUpload: File | null = kubeconfigFile.value

  if (importMode.value === 'text') {
    if (!importForm.value.kubeconfigRaw.trim()) {
      emit('toast', 'Kubeconfig content cannot be empty', 'error')
      return
    }
    fileToUpload = new File([importForm.value.kubeconfigRaw], `${importForm.value.name}-kubeconfig.yaml`, {
      type: 'text/yaml',
    })
  }

  if (!fileToUpload) {
    emit('toast', 'Kubeconfig file or content is required for cluster import', 'error')
    return
  }

  importingCluster.value = true
  try {
    const formData = new FormData()
    formData.append('id', importForm.value.id.trim() || `cluster-${Date.now().toString(36)}`)
    formData.append('name', importForm.value.name.trim())
    formData.append('group', importForm.value.group)
    formData.append('region', importForm.value.region.trim())
    formData.append('provider', importForm.value.provider)
    formData.append('kubeconfig', fileToUpload)

    await fleetApi.importCluster(formData)
    const importedName = importForm.value.name.trim()
    emit('toast', `Cluster ${importedName} successfully imported!`)
    importForm.value.name = ''
    importForm.value.id = ''
    importForm.value.kubeconfigRaw = ''
    kubeconfigFile.value = null
    emit('imported', importedName)
    emit('close')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Cluster import failed'
    emit('toast', msg, 'error')
  } finally {
    importingCluster.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Import Kubernetes Cluster"
    subtitle="Connect an external Kubernetes cluster via Kubeconfig"
    max-width="620px"
    @close="emit('close')"
  >
    <div class="import-modal-body">
      <div class="form-group">
        <label class="form-label required">Cluster Name / Identifier</label>
        <input 
          v-model="importForm.name"
          type="text" 
          class="input-glass font-mono" 
          placeholder="e.g. production-k8s, staging-eks" 
          required 
        />
      </div>

      <div class="form-row-2">
        <div class="form-group flex-1">
          <label class="form-label">Group</label>
          <input 
            v-model="importForm.group"
            type="text" 
            class="input-glass font-mono" 
            placeholder="production" 
          />
        </div>
        <div class="form-group flex-1">
          <label class="form-label">Region</label>
          <input 
            v-model="importForm.region"
            type="text" 
            class="input-glass font-mono" 
            placeholder="us-east-1" 
          />
        </div>
      </div>

      <div class="import-mode-toggle">
        <button 
          type="button" 
          class="toggle-btn"
          :class="{ 'is-active': importMode === 'file' }"
          @click="importMode = 'file'"
        >
          📁 Upload Kubeconfig File
        </button>
        <button 
          type="button" 
          class="toggle-btn"
          :class="{ 'is-active': importMode === 'text' }"
          @click="importMode = 'text'"
        >
          📝 Paste Kubeconfig Text
        </button>
      </div>

      <div v-if="importMode === 'file'" class="form-group">
        <label class="form-label required">Kubeconfig YAML File</label>
        <input 
          type="file" 
          accept=".yaml,.yml,.config,text/plain" 
          class="input-glass file-input font-mono"
          @change="handleFileChange"
        />
      </div>

      <div v-else class="form-group">
        <label class="form-label required">Kubeconfig Raw YAML</label>
        <textarea
          v-model="importForm.kubeconfigRaw"
          class="input-glass font-mono text-area-lg"
          rows="8"
          placeholder="apiVersion: v1&#10;clusters:&#10;  - cluster:&#10;      ..."
        ></textarea>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="importingCluster" @click="emit('close')">
        Cancel
      </button>
      <button 
        type="button" 
        class="btn btn-primary"
        :disabled="importingCluster || !importForm.name.trim()"
        @click="handleImportCluster"
      >
        <span>{{ importingCluster ? '⏳ Importing Cluster...' : '✨ Import Cluster' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
