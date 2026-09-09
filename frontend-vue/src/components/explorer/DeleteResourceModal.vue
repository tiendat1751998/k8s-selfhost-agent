<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { k8sApi, type K8sResource, type ResourceKind } from '../../api/k8s'

const props = defineProps<{
  show: boolean
  resource: K8sResource | null
  cluster: string
  selectedKind: ResourceKind
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'deleted', res: K8sResource): void
  (e: 'toast', msg: string, type?: 'success' | 'error'): void
}>()

const deletingResource = ref(false)

async function handleDeleteConfirmed() {
  if (!props.resource || !props.cluster) return
  const r = props.resource
  const name = r.metadata?.name || ''
  const ns = r.metadata?.namespace || undefined
  deletingResource.value = true
  try {
    await k8sApi.deleteResource(props.cluster, props.selectedKind, name, ns)
    emit('toast', `Resource ${r.kind}/${name} deleted successfully`)
    emit('deleted', r)
    emit('close')
  } catch (err: unknown) {
    emit('toast', err instanceof Error ? err.message : 'Failed to delete resource', 'error')
  } finally {
    deletingResource.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="`Delete ${resource?.kind || 'Resource'}`"
    :subtitle="`Cluster: ${cluster}`"
    max-width="500px"
    @close="emit('close')"
  >
    <div class="delete-modal-content">
      <div class="delete-warning-icon">⚠️</div>
      <p class="delete-msg">
        Are you sure you want to permanently delete
        <strong class="text-white font-mono">{{ resource?.kind }}/{{ resource?.metadata?.name }}</strong>
        <span v-if="resource?.metadata?.namespace">
          in namespace <strong class="text-cyan font-mono">{{ resource.metadata.namespace }}</strong>
        </span>?
      </p>
      <p class="text-muted font-small">
        This action cannot be undone. Any active workloads or bound resources may be terminated immediately.
      </p>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="deletingResource" @click="emit('close')">
        Cancel
      </button>
      <button 
        type="button" 
        class="btn btn-danger"
        :disabled="deletingResource"
        @click="handleDeleteConfirmed"
      >
        <span>{{ deletingResource ? '⏳ Deleting...' : '🗑️ Delete Permanently' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
