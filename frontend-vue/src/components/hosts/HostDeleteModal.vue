<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { ComputeHost } from '../../api/compute'

defineProps<{
  show: boolean
  host: ComputeHost | null
  deleting: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'confirm'): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Decommission Infrastructure Host"
    subtitle="Confirm removal of host endpoint from fleet registry"
    max-width="500px"
    @update:show="emit('update:show', $event)"
  >
    <div v-if="host" class="confirm-dialog-content">
      <div class="confirm-alert alert-danger">
        <span class="alert-icon">🚨</span>
        <div>
          <strong>Are you sure you want to remove this host?</strong>
          <p class="alert-desc" style="margin-top: 6px;">
            Host <strong class="text-rose font-mono">{{ host.name }}</strong> ({{ host.endpoint }}) will be permanently disconnected from the multi-host fleet registry.
          </p>
        </div>
      </div>

      <div class="modal-actions" style="margin-top: 20px;">
        <button type="button" class="btn btn-secondary" @click="emit('update:show', false)">Cancel</button>
        <button type="button" class="btn btn-danger" :disabled="deleting" @click="emit('confirm')">
          <span>{{ deleting ? '⏳ Removing...' : '🗑️ Confirm Decommission' }}</span>
        </button>
      </div>
    </div>
  </ModalDrawer>
</template>
