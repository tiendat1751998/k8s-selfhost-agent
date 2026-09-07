<script setup lang="ts">
import type { NodeMetrics, TpsSnapshot } from '../../../api/overview'
import ModalDrawer from '../../ui/ModalDrawer.vue'
import TrafficGeoDistribution, { type TrendPoint } from './deepdive/TrafficGeoDistribution.vue'
import TrafficTopologyGraph from './deepdive/TrafficTopologyGraph.vue'
import TrafficEndpointsTable, { type DeepDiveServiceItem } from './deepdive/TrafficEndpointsTable.vue'

export type { TrendPoint, DeepDiveServiceItem }

interface Props {
  show: boolean
  trendHistory: TrendPoint[]
  nodes: NodeMetrics[]
  tpsData?: TpsSnapshot | null
  dockerContainers?: any[]
}

withDefaults(defineProps<Props>(), {
  show: false,
  tpsData: null,
  dockerContainers: () => [],
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:show', value: boolean): void
}>()

function handleClose() {
  emit('close')
  emit('update:show', false)
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    maxWidth="1140px"
    title="⚡ Traffic & Telemetry Deep-Dive"
    subtitle="High-resolution time-series saturation curves and multi-service traffic contributor breakdown"
    @close="handleClose"
  >
    <div class="deep-dive-body">
      <!-- SECTION 1: Summary Statistics & Distribution -->
      <TrafficGeoDistribution
        :trendHistory="trendHistory"
        :nodes="nodes"
        :tpsData="tpsData"
      />

      <!-- SECTION 2: High-Resolution Telemetry Overlay Graph -->
      <TrafficTopologyGraph
        :trendHistory="trendHistory"
        :tpsData="tpsData"
      />

      <!-- SECTION 3: Service & Workload Contributors Breakdown -->
      <TrafficEndpointsTable
        :nodes="nodes"
        :tpsData="tpsData"
        :dockerContainers="dockerContainers"
      />
    </div>

    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">
        <span>Close Deep-Dive</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../../assets/styles/components/deep-dive-traffic.css';
</style>