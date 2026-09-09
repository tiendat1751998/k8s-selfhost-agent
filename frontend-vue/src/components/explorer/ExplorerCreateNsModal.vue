<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'

defineProps<{
  show: boolean
  cluster: string
  creating?: boolean
  error?: string | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'create', name: string): void
}>()

const newNsName = ref('')

function submit() {
  if (!newNsName.value.trim()) return
  emit('create', newNsName.value.trim())
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Create Kubernetes Namespace"
    :subtitle="`Target Cluster: ${cluster}`"
    max-width="480px"
    @close="$emit('close')"
  >
    <div class="new-ns-modal-body">
      <div v-if="error" class="form-error-banner font-mono">{{ error }}</div>
      <div class="form-group">
        <label class="form-label required">Namespace Name</label>
        <input 
          v-model="newNsName" 
          type="text" 
          placeholder="e.g. production-backend" 
          class="input-glass font-mono" 
          @keyup.enter="submit" 
        />
      </div>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="creating" @click="$emit('close')">Cancel</button>
      <button type="button" class="btn btn-primary" :disabled="creating || !newNsName.trim()" @click="submit">
        <span>{{ creating ? 'Creating...' : '+ Create Namespace' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
