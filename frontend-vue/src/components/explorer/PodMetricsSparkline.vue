<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    cpuMillicores: number
    memoryBytes: number
    cpuLimit?: number
    memoryLimit?: number
    compact?: boolean
  }>(),
  {
    cpuLimit: undefined,
    memoryLimit: undefined,
    compact: false,
  }
)

const hasCpu = computed(() => typeof props.cpuMillicores === 'number' && props.cpuMillicores > 0)
const hasMemory = computed(() => typeof props.memoryBytes === 'number' && props.memoryBytes > 0)

const formattedCpu = computed(() => {
  if (!hasCpu.value) return '--'
  if (props.cpuMillicores >= 1000) return `${(props.cpuMillicores / 1000).toFixed(1)} cores`
  return `${Math.round(props.cpuMillicores)}m`
})

const formattedMemory = computed(() => {
  if (!hasMemory.value) return '--'
  const mib = props.memoryBytes / (1024 * 1024)
  if (mib >= 1024) return `${(mib / 1024).toFixed(1)} GiB`
  return `${Math.round(mib)} MiB`
})

const cpuPercent = computed(() => {
  if (!hasCpu.value) return 0
  const limit = props.cpuLimit && props.cpuLimit > 0 ? props.cpuLimit : 1000
  return Math.min(Math.round((props.cpuMillicores / limit) * 100), 100)
})

const memoryPercent = computed(() => {
  if (!hasMemory.value) return 0
  const limit = props.memoryLimit && props.memoryLimit > 0 ? props.memoryLimit : 512 * 1024 * 1024
  return Math.min(Math.round((props.memoryBytes / limit) * 100), 100)
})

function getColorClass(pct: number): string {
  if (pct >= 85) return 'color-crimson'
  if (pct >= 70) return 'color-amber'
  return 'color-emerald'
}
</script>

<template>
  <div class="pod-metrics-sparkline font-mono" :class="{ 'is-compact': compact }">
    <div class="metric-row">
      <span class="metric-lbl">CPU</span>
      <div class="bar-container">
        <svg v-if="hasCpu" class="spark-bar" viewBox="0 0 100 6" preserveAspectRatio="none">
          <rect x="0" y="0" width="100" height="6" rx="3" class="bar-bg" />
          <rect x="0" y="0" :width="cpuPercent" height="6" rx="3" :class="getColorClass(cpuPercent)" />
        </svg>
        <span v-else class="empty-bar">--</span>
      </div>
      <span class="metric-val" :class="hasCpu ? getColorClass(cpuPercent) : 'text-muted'">{{ formattedCpu }}</span>
    </div>

    <div class="metric-row">
      <span class="metric-lbl">RAM</span>
      <div class="bar-container">
        <svg v-if="hasMemory" class="spark-bar" viewBox="0 0 100 6" preserveAspectRatio="none">
          <rect x="0" y="0" width="100" height="6" rx="3" class="bar-bg" />
          <rect x="0" y="0" :width="memoryPercent" height="6" rx="3" :class="getColorClass(memoryPercent)" />
        </svg>
        <span v-else class="empty-bar">--</span>
      </div>
      <span class="metric-val" :class="hasMemory ? getColorClass(memoryPercent) : 'text-muted'">{{ formattedMemory }}</span>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
