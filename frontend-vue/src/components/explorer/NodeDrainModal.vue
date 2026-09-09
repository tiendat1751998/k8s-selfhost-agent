<script setup lang="ts">
import { ref, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { k8sApi, type K8sResource } from '../../api/k8s'

const props = defineProps<{
  show: boolean
  node: K8sResource | null
  cluster: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'drained', node: K8sResource): void
  (e: 'toast', msg: string, type?: 'success' | 'error'): void
}>()

const drainOptions = ref({
  gracePeriodSeconds: 30,
  ignoreDaemonSets: true,
  deleteEmptyDirData: false,
  force: false,
})
const drainingNode = ref(false)

watch(
  () => props.show,
  (isOpen) => {
    if (isOpen) {
      drainOptions.value = {
        gracePeriodSeconds: 30,
        ignoreDaemonSets: true,
        deleteEmptyDirData: false,
        force: false,
      }
    }
  }
)

async function handleDrainNodeConfirm() {
  const name = props.node?.metadata?.name
  if (!name || !props.cluster) return
  drainingNode.value = true
  try {
    await k8sApi.drainNode(props.cluster, name, drainOptions.value)
    emit('toast', 'Node ' + name + ' drain request submitted successfully', 'success')
    if (props.node) emit('drained', props.node)
    emit('close')
  } catch (err: unknown) {
    emit('toast', err instanceof Error ? err.message : 'Failed to drain node', 'error')
  } finally {
    drainingNode.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Drain Kubernetes Node"
    :subtitle="`Safely evict all pods from ${node?.metadata?.name || ''}`"
    max-width="540px"
    @close="emit('close')"
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
      <button type="button" class="btn btn-secondary" :disabled="drainingNode" @click="emit('close')">
        Cancel
      </button>
      <button 
        type="button" 
        class="btn btn-danger"
        :disabled="drainingNode"
        @click="handleDrainNodeConfirm"
      >
        <span>{{ drainingNode ? '⏳ Draining...' : '🧹 Confirm Drain' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
