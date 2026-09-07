<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { fleetApi } from '../../api/fleet'

defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'imported'): void
}>()

const importMode = ref<'file' | 'text'>('file')
const loading = ref(false)
const errorMessage = ref<string | null>(null)

const importForm = ref({
  id: '',
  name: '',
  group: 'production',
  region: 'us-east-1',
  provider: 'aws',
  kubeconfigRaw: '',
})
const kubeconfigFile = ref<File | null>(null)

function handleFileUpload(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    kubeconfigFile.value = target.files[0]
  }
}

async function handleImportCluster() {
  errorMessage.value = null
  if (!importForm.value.name.trim()) {
    errorMessage.value = 'Cluster name is required'
    return
  }

  let fileToUpload: File | null = kubeconfigFile.value

  if (importMode.value === 'text') {
    if (!importForm.value.kubeconfigRaw.trim()) {
      errorMessage.value = 'Kubeconfig content cannot be empty'
      return
    }
    fileToUpload = new File([importForm.value.kubeconfigRaw], `${importForm.value.name}-kubeconfig.yaml`, {
      type: 'text/yaml',
    })
  }

  if (!fileToUpload) {
    errorMessage.value = 'Kubeconfig file or content is required for cluster import'
    return
  }

  loading.value = true
  try {
    const formData = new FormData()
    formData.append('id', importForm.value.id.trim() || `cluster-${Date.now().toString(36)}`)
    formData.append('name', importForm.value.name.trim())
    formData.append('group', importForm.value.group)
    formData.append('region', importForm.value.region.trim())
    formData.append('provider', importForm.value.provider)
    formData.append('kubeconfig', fileToUpload)

    await fleetApi.importCluster(formData)
    emit('imported')
    emit('update:show', false)
    importForm.value.name = ''
    importForm.value.id = ''
    importForm.value.kubeconfigRaw = ''
    kubeconfigFile.value = null
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Cluster import failed'
    errorMessage.value = msg
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Import Kubernetes Cluster into Fleet"
    subtitle="Upload cluster kubeconfig to establish secure telemetry bridge"
    max-width="600px"
    @update:show="emit('update:show', $event)"
  >
    <div class="modal-form">
      <div v-if="errorMessage" class="toast-banner toast-error animate-fade-in" style="margin-bottom: 6px;">
        <span>⚠️</span>
        <span>{{ errorMessage }}</span>
      </div>

      <div class="form-group">
        <label class="form-label">Cluster Identifier / Name *</label>
        <input v-model="importForm.name" type="text" placeholder="e.g. prod-us-east-cluster" class="input-glass" />
      </div>

      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label">Fleet Tier</label>
          <select v-model="importForm.group" class="input-glass">
            <option value="production">Production</option>
            <option value="staging">Staging</option>
            <option value="development">Development</option>
            <option value="edge">Edge Computing</option>
          </select>
        </div>

        <div class="form-group flex-1">
          <label class="form-label">Cloud Provider</label>
          <select v-model="importForm.provider" class="input-glass">
            <option value="aws">AWS (EKS)</option>
            <option value="gcp">GCP (GKE)</option>
            <option value="azure">Azure (AKS)</option>
            <option value="onprem">Bare-Metal / On-Premise</option>
            <option value="edge">Edge Device / K3s</option>
            <option value="generic">Generic Kubernetes</option>
          </select>
        </div>
      </div>

      <div class="form-group">
        <label class="form-label">Region / Datacenter</label>
        <input v-model="importForm.region" type="text" placeholder="e.g. us-east-1, eu-west-1, dc-onprem-1" class="input-glass font-mono" />
      </div>

      <div class="form-group">
        <div class="mode-tabs">
          <button
            type="button"
            class="mode-tab"
            :class="{ active: importMode === 'file' }"
            @click="importMode = 'file'"
          >
            📁 File Upload
          </button>
          <button
            type="button"
            class="mode-tab"
            :class="{ active: importMode === 'text' }"
            @click="importMode = 'text'"
          >
            📝 Paste YAML
          </button>
        </div>

        <div v-if="importMode === 'file'" class="file-input-wrap">
          <label class="form-label">Kubeconfig File (.yaml / .config / .json) *</label>
          <input type="file" accept=".yaml,.yml,.config,.json,text/*" class="input-glass" @change="handleFileUpload" />
        </div>

        <div v-else class="text-input-wrap">
          <label class="form-label">Paste Kubeconfig YAML Content *</label>
          <textarea
            v-model="importForm.kubeconfigRaw"
            rows="6"
            placeholder="apiVersion: v1&#10;clusters:&#10;  - cluster:&#10;      server: https://..."
            class="input-glass font-mono text-area-input"
          ></textarea>
        </div>

        <span class="form-hint">🔒 Kubeconfig is encrypted with AES-256 GCM in local vault before persistence. Sensitive tokens are never returned in cleartext.</span>
      </div>
    </div>

    <template #footer="{ close }">
      <button class="btn btn-secondary" @click="close">Cancel</button>
      <button class="btn btn-primary" :disabled="loading" @click="handleImportCluster">
        <span>{{ loading ? '⏳ Importing Cluster...' : 'Import Cluster ➔' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
