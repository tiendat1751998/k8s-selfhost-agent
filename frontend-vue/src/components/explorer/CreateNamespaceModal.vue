<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { k8sApi } from '../../api/k8s'

const props = defineProps<{
  show: boolean
  cluster: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'created', name: string): void
  (e: 'toast', msg: string, type?: 'success' | 'error'): void
}>()

const newNsName = ref('')
const creatingNs = ref(false)
const newNsError = ref<string | null>(null)

async function handleCreateNamespace() {
  if (!newNsName.value.trim()) {
    newNsError.value = 'Namespace name is required'
    return
  }
  creatingNs.value = true
  newNsError.value = null
  try {
    const created = await k8sApi.createNamespace(props.cluster, newNsName.value.trim())
    const ns = created.name || newNsName.value.trim()
    emit('toast', `Namespace "${ns}" created!`)
    newNsName.value = ''
    emit('created', ns)
    emit('close')
  } catch (err: unknown) {
    newNsError.value = err instanceof Error ? err.message : 'Failed to create namespace'
  } finally {
    creatingNs.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Create Kubernetes Namespace"
    :subtitle="`Cluster: ${cluster}`"
    max-width="460px"
    @close="emit('close')"
  >
    <div class="form-group">
      <label class="form-label required">Namespace Name</label>
      <input 
        v-model="newNsName"
        type="text" 
        class="input-glass font-mono" 
        placeholder="e.g. staging, payment-service" 
        required 
        @keydown.enter="handleCreateNamespace"
      />
      <span v-if="newNsError" class="text-rose text-xs">{{ newNsError }}</span>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="creatingNs" @click="emit('close')">
        Cancel
      </button>
      <button 
        type="button" 
        class="btn btn-primary"
        :disabled="creatingNs || !newNsName.trim()"
        @click="handleCreateNamespace"
      >
        <span>{{ creatingNs ? '⏳ Creating...' : '✨ Create Namespace' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
