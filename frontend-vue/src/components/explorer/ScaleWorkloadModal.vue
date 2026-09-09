<script setup lang="ts">
import { ref, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { k8sApi, type K8sResource, type ResourceKind } from '../../api/k8s'

const props = defineProps<{
  show: boolean
  target: K8sResource | null
  cluster: string
  selectedKind: ResourceKind
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'scaled', resource: K8sResource, count: number): void
  (e: 'toast', msg: string, type?: 'success' | 'error'): void
}>()

const scaleReplicasCount = ref(1)
const scalingResource = ref(false)

watch(
  () => [props.show, props.target],
  () => {
    if (props.target) {
      const spec = props.target.spec as { replicas?: number } | undefined
      const status = props.target.status as { replicas?: number } | undefined
      scaleReplicasCount.value = spec?.replicas ?? status?.replicas ?? 1
    }
  },
  { immediate: true }
)

async function handleScaleConfirm() {
  if (!props.target || !props.cluster) return
  const r = props.target
  const name = r.metadata?.name || ''
  const ns = r.metadata?.namespace || 'default'
  const kind = (r.kind || '').toLowerCase()
  scalingResource.value = true
  try {
    if (kind === 'statefulset' || props.selectedKind === 'statefulsets') {
      await k8sApi.scaleStatefulSet(props.cluster, name, ns, scaleReplicasCount.value)
    } else {
      await k8sApi.scaleDeployment(props.cluster, name, ns, scaleReplicasCount.value)
    }
    emit('toast', `Scaled ${r.kind}/${name} to ${scaleReplicasCount.value} replicas`)
    emit('scaled', r, scaleReplicasCount.value)
    emit('close')
  } catch (err: unknown) {
    emit('toast', err instanceof Error ? err.message : 'Failed to scale workload', 'error')
  } finally {
    scalingResource.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="`Scale ${target?.kind || 'Workload'}`"
    :subtitle="`Resource: ${target?.metadata?.name || ''} (${target?.metadata?.namespace || 'default'})`"
    max-width="480px"
    @close="emit('close')"
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
      <button type="button" class="btn btn-secondary" :disabled="scalingResource" @click="emit('close')">
        Cancel
      </button>
      <button 
        type="button" 
        class="btn btn-primary"
        :disabled="scalingResource"
        @click="handleScaleConfirm"
      >
        <span>{{ scalingResource ? '⏳ Scaling...' : '⚡ Apply Scale' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
