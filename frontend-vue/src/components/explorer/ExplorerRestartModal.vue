<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { K8sResource } from '../../api/k8s'

defineProps<{
  show: boolean
  target: K8sResource | null
  restarting?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm', target: K8sResource): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="`Rolling Restart ${target?.kind || 'Workload'}`"
    :subtitle="`Trigger rolling rollout for ${target?.metadata?.name || ''}`"
    max-width="480px"
    @close="$emit('close')"
  >
    <div class="restart-modal-body">
      <div class="restart-warning-banner font-mono">
        <span class="warning-icon">🔄</span>
        <div>
          <strong class="text-white">{{ target?.kind }}/{{ target?.metadata?.name }}</strong>
          <div class="text-muted font-small">Namespace: {{ target?.metadata?.namespace || 'default' }}</div>
        </div>
      </div>
      <p class="restart-desc text-muted font-small">
        A rollout restart will trigger Kubernetes to sequentially terminate and recreate all pods for this workload with zero downtime.
      </p>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="restarting" @click="$emit('close')">
        Cancel
      </button>
      <button
        type="button"
        class="btn btn-primary btn-amber-glow"
        :disabled="restarting || !target"
        @click="target && emit('confirm', target)"
      >
        <span>{{ restarting ? 'Restarting...' : 'Confirm Restart' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
.restart-modal-body {
  padding: 4px 0;
}
.restart-warning-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 8px;
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.25);
}
.warning-icon {
  font-size: 1.4rem;
}
.restart-desc {
  margin-top: 12px;
  line-height: 1.5;
}
.btn-amber-glow {
  background: rgba(245, 158, 11, 0.2);
  border-color: #f59e0b;
  color: #fbbf24;
}
.btn-amber-glow:hover:not(:disabled) {
  background: #f59e0b;
  color: #000;
}
</style>
