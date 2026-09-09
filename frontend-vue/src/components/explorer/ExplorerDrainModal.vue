<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { K8sResource, DrainOptions } from '../../domain/explorer'
import { IconDrain } from './icons'

defineProps<{
  show: boolean
  targetNode: K8sResource | null
  draining?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm', options: DrainOptions): void
}>()

const drainOptions = ref<DrainOptions>({
  gracePeriodSeconds: 30,
  ignoreDaemonSets: true,
  deleteEmptyDirData: false,
  force: false,
})
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Drain Kubernetes Node"
    :subtitle="`Safely evict all pods from ${targetNode?.metadata?.name || ''}`"
    max-width="540px"
    @close="$emit('close')"
  >
    <div class="drain-modal-body">
      <p class="text-muted font-small">
        Draining will cordon the node and evict all pods. Configure eviction parameters:
      </p>

      <div class="form-group">
        <label class="form-label">Grace Period (seconds)</label>
        <input 
          v-model.number="drainOptions.gracePeriodSeconds" 
          type="number" 
          class="input-glass font-mono" 
          min="0" 
          max="3600" 
        />
      </div>

      <div class="checkbox-group">
        <label class="checkbox-label">
          <input v-model="drainOptions.ignoreDaemonSets" type="checkbox" />
          <span>Ignore DaemonSet-managed pods (Recommended)</span>
        </label>

        <label class="checkbox-label">
          <input v-model="drainOptions.deleteEmptyDirData" type="checkbox" />
          <span>Delete local data in emptyDir volumes</span>
        </label>

        <label class="checkbox-label">
          <input v-model="drainOptions.force" type="checkbox" />
          <span>Force eviction (continue if pods are unmanaged)</span>
        </label>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="draining" @click="$emit('close')">
        Cancel
      </button>
      <button 
        type="button" 
        class="btn btn-danger"
        :disabled="draining"
        @click="$emit('confirm', drainOptions)"
      >
        <IconDrain :size="13" />
        <span>{{ draining ? 'Draining...' : 'Confirm Drain' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
