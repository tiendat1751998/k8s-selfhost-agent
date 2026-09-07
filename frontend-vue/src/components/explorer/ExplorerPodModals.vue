<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import PodLogViewer from '../k8s/PodLogViewer.vue'
import PodTerminal from '../k8s/PodTerminal.vue'
import type { K8sResource } from '../../domain/explorer'

defineProps<{
  showLogs: boolean
  logsPod: K8sResource | null
  showTerminal: boolean
  terminalPod: K8sResource | null
  cluster: string
}>()

defineEmits<{
  (e: 'close-logs'): void
  (e: 'close-terminal'): void
}>()
</script>

<template>
  <div>
    <ModalDrawer
      :show="showLogs"
      mode="modal"
      :title="`Pod Logs: ${logsPod?.metadata?.name || ''}`"
      :subtitle="`Namespace: ${logsPod?.metadata?.namespace || 'default'} • Cluster: ${cluster}`"
      max-width="960px"
      @close="$emit('close-logs')"
    >
      <div v-if="logsPod" class="logs-modal-body">
        <PodLogViewer :cluster="cluster" :namespace="logsPod.metadata?.namespace || 'default'" :pod="logsPod.metadata?.name || ''" />
      </div>
      <template #footer><button type="button" class="btn btn-secondary" @click="$emit('close-logs')">Close</button></template>
    </ModalDrawer>

    <ModalDrawer
      :show="showTerminal"
      mode="modal"
      :title="`Pod Exec Console: ${terminalPod?.metadata?.name || ''}`"
      :subtitle="`Namespace: ${terminalPod?.metadata?.namespace || 'default'} • Cluster: ${cluster}`"
      max-width="960px"
      @close="$emit('close-terminal')"
    >
      <div v-if="terminalPod" class="terminal-modal-body">
        <PodTerminal :cluster="cluster" :namespace="terminalPod.metadata?.namespace || 'default'" :pod="terminalPod.metadata?.name || ''" />
      </div>
      <template #footer><button type="button" class="btn btn-secondary" @click="$emit('close-terminal')">Close</button></template>
    </ModalDrawer>
  </div>
</template>
