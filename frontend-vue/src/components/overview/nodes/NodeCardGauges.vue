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
@import '../../../assets/styles/views/overview.css';
</style>
