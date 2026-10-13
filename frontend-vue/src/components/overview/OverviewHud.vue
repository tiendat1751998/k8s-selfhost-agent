<script setup lang="ts">
import OverviewHud from './hud/OverviewHud.vue'
import type { SystemOverview, NodeMetrics } from '../../api/overview'
import type { TrendPoint } from '../../composables/useOverviewDashboard'

interface Props {
  overview: SystemOverview
  runningContainers: number
  totalContainers: number
  peakCpuNode?: NodeMetrics | null
  clusterUsedMemBytes: number
  clusterTotalMemBytes: number
  clusterUsedDiskBytes: number
  clusterTotalDiskBytes: number
  trendHistory?: TrendPoint[]
}

defineProps<Props>()
</script>

<template>
  <OverviewHud v-bind="$props" />
</template>
