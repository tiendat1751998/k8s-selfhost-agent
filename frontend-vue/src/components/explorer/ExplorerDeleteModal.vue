<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { K8sResource } from '../../domain/explorer'
import { IconTrash } from './icons'

defineProps<{
  show: boolean
  resource: K8sResource | null
  deleting?: boolean
}>()

defineEmits<{
  (e: 'close'): void
  (e: 'confirm', resource: K8sResource): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Confirm Resource Deletion"
    subtitle="Critical Kubernetes Action"
    max-width="500px"
    @close="$emit('close')"
  >
    <div class="delete-modal-content">
      <div class="delete-warning-icon">
        <IconTrash :size="32" />
      </div>
      <p class="delete-msg">
        Are you sure you want to delete <strong class="text-white font-mono">{{ resource?.kind }}/{{ resource?.metadata?.name }}</strong>?
      </p>
      <p class="text-muted font-small">This action cannot be undone. Active workloads or storage will be removed.</p>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="deleting" @click="$emit('close')">Cancel</button>
      <button type="button" class="btn btn-danger" :disabled="deleting" @click="resource && $emit('confirm', resource)">
        <IconTrash :size="13" />
        <span>{{ deleting ? 'Deleting...' : 'Delete Permanently' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
