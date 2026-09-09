<script setup lang="ts">
import { ref, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import PodLogViewer from '../k8s/PodLogViewer.vue'
import type { K8sResource } from '../../api/k8s'
import { getPodContainers } from '../../composables/useK8sExplorer'

const props = defineProps<{
  show: boolean
  pod: K8sResource | null
  cluster: string
  container?: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const selectedPodContainer = ref<string>('')

watch(
  () => [props.pod, props.container],
  () => {
    if (props.pod) {
      const containers = getPodContainers(props.pod)
      selectedPodContainer.value = props.container || (containers.length > 0 ? containers[0].name : '')
    }
  },
  { immediate: true }
)

function getPodContainerNames(pod: K8sResource | null): string[] {
  if (!pod) return []
  return getPodContainers(pod).map(c => c.name)
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="`Pod Logs: ${pod?.metadata?.name || ''}`"
    :subtitle="`Namespace: ${pod?.metadata?.namespace || 'default'} • Cluster: ${cluster}`"
    max-width="960px"
    @close="emit('close')"
  >
    <div v-if="pod" class="pod-logs-wrapper">
      <div v-if="getPodContainerNames(pod).length > 1" class="container-select-bar">
        <label class="form-label">Select Container:</label>
        <div class="container-tabs">
          <button
            v-for="cName in getPodContainerNames(pod)"
            :key="cName"
            type="button"
            class="container-tab-btn"
            :class="{ 'is-active': selectedPodContainer === cName }"
            @click="selectedPodContainer = cName"
          >
            {{ cName }}
          </button>
        </div>
      </div>

      <PodLogViewer
        :cluster="cluster"
        :namespace="pod.metadata?.namespace || 'default'"
        :pod="pod.metadata?.name || ''"
        :container="selectedPodContainer"
      />
    </div>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
