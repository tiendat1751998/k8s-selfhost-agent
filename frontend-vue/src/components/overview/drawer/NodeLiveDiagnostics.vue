<script setup lang="ts">
import { computed } from 'vue'
import type { NodeMetrics, SystemOverview, TpsSnapshot } from '../../../api/overview'
import NodeDiagnosticsOverviewTab from './tabs/NodeDiagnosticsOverviewTab.vue'
import NodeDiagnosticsNetworkTab from './tabs/NodeDiagnosticsNetworkTab.vue'
import NodeDiagnosticsProcessesTab from './tabs/NodeDiagnosticsProcessesTab.vue'

interface Props {
  node: NodeMetrics
  overview?: SystemOverview | null
  tpsData?: TpsSnapshot | null
}

const props = withDefaults(defineProps<Props>(), {
  overview: null,
  tpsData: null,
})

const isNodeOffline = computed(() => {
  if (!props.node) return true
  const s = (props.node.status || '').toLowerCase().trim()
  if (['down', 'offline', 'disconnected', 'unreachable'].includes(s)) return true
  if (!props.node.memory_total || props.node.memory_total <= 0) return true
  return false
})
</script>

<template>
  <div class="drawer-tab-content">
    <!-- 1. Hardware Saturation Telemetry HUD -->
    <NodeDiagnosticsOverviewTab
      :node="node"
      :isNodeOffline="isNodeOffline"
    />

    <!-- 2. Network Interface & Disk Block Devices -->
    <NodeDiagnosticsNetworkTab
      :node="node"
    />

    <!-- 3. Top Resource-Consuming Apps & Processes -->
    <NodeDiagnosticsProcessesTab
      :node="node"
      :overview="overview"
      :tpsData="tpsData"
    />
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/node-diagnostics.css';
</style>
