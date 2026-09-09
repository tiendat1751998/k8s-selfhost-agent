<script setup lang="ts">
import { ref, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { K8sResource } from '../../api/k8s'
import { IconScale } from './icons'

const props = defineProps<{
  show: boolean
  target: K8sResource | null
  scaling?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm', replicas: number): void
}>()

const scaleReplicasCount = ref(1)

watch(
  () => props.target,
  (newTarget) => {
    if (newTarget) {
      const spec = newTarget.spec as { replicas?: number } | undefined
      const status = newTarget.status as { replicas?: number } | undefined
      scaleReplicasCount.value = spec?.replicas ?? status?.replicas ?? 1
    }
  },
  { immediate: true }
)
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="`Scale ${target?.kind || 'Workload'}`"
    :subtitle="`Resource: ${target?.metadata?.name || ''} (${target?.metadata?.namespace || 'default'})`"
    max-width="480px"
    @close="$emit('close')"
  >
    <div class="scale-modal-body">
      <p class="text-muted font-small">
        Adjust the desired number of pod replicas. The Kubernetes controller will automatically adjust pods.
      </p>

      <div class="scale-stepper-wrap">
        <label class="form-label">Desired Replicas</label>
        <div class="stepper-input-group">
          <button 
            type="button" 
            class="btn-stepper"
            :disabled="scaleReplicasCount <= 0"
            @click="scaleReplicasCount = Math.max(0, scaleReplicasCount - 1)"
          >
            -
          </button>
          <input 
            v-model.number="scaleReplicasCount" 
            type="number" 
            min="0" 
            max="100" 
            class="input-glass font-mono scale-num-input" 
          />
          <button 
            type="button" 
            class="btn-stepper"
            @click="scaleReplicasCount = scaleReplicasCount + 1"
          >
            +
          </button>
        </div>
        <input 
          v-model.number="scaleReplicasCount" 
          type="range" 
          min="0" 
          max="20" 
          class="scale-range-slider" 
        />
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="scaling" @click="$emit('close')">
        Cancel
      </button>
      <button 
        type="button" 
        class="btn btn-primary"
        :disabled="scaling"
        @click="$emit('confirm', scaleReplicasCount)"
      >
        <IconScale :size="13" />
        <span>{{ scaling ? 'Scaling...' : 'Apply Scale' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
