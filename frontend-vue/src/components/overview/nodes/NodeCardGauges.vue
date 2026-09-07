<script setup lang="ts">
import { computed } from 'vue'
import type { NodeMetrics } from '../../../api/overview'
import CircularGauge from '../../ui/CircularGauge.vue'

const props = defineProps<{ node: NodeMetrics }>()

const isDisabled = computed(() =>
  props.node.status === 'down' || props.node.status === 'offline' || props.node.status === 'disconnected' || props.node.memory_total === 0
)
</script>

<template>
  <div class="node-gauges-cluster">
    <div class="gauge-col">
      <CircularGauge :percent="node.cpu_percent" label="CPU" :size="52" :strokeWidth="3.5" :disabled="isDisabled" />
    </div>
    <div class="gauge-col">
      <CircularGauge :percent="node.memory_percent" label="RAM" :size="52" :strokeWidth="3.5" :disabled="isDisabled" />
    </div>
    <div class="gauge-col">
      <CircularGauge :percent="node.disk_percent" label="DISK" :size="52" :strokeWidth="3.5" :disabled="isDisabled" />
    </div>
  </div>
</template>

<style scoped>
.node-gauges-cluster {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 10px 6px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(0, 0, 0, 0.18);
  border-radius: 10px;
}
.gauge-col { display: flex; flex-direction: column; align-items: center; }
</style>
