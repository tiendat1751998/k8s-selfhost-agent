<script setup lang="ts">
import { ref, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import PodTerminal from '../k8s/PodTerminal.vue'
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
    :title="`Terminal Exec: ${pod?.metadata?.name || ''}`"
    :subtitle="`Namespace: ${pod?.metadata?.namespace || 'default'} • Cluster: ${cluster}`"
    max-width="1000px"
    @close="emit('close')"
  >
    <div v-if="pod" class="pod-terminal-wrapper">
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

      <PodTerminal
        :cluster="cluster"
        :namespace="pod.metadata?.namespace || 'default'"
        :pod="pod.metadata?.name || ''"
        :container="selectedPodContainer"
      />
    </div>
  </ModalDrawer>
</template>

<style scoped>
.pod-terminal-wrapper {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.container-select-bar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.container-tabs {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.container-tab-btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #94a3b8;
  font-family: var(--font-mono);
  font-size: 0.78rem;
  padding: 4px 10px;
  border-radius: 6px;
  cursor: pointer;
}

.container-tab-btn.is-active {
  background: rgba(6, 182, 212, 0.2);
  border-color: #06b6d4;
  color: #fff;
}
</style>
