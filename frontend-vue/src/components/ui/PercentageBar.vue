<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  percentage: number
  threshold?: number
  height?: number | string
  variant?: 'spectrum' | 'cyan' | 'emerald' | 'amber' | 'rose' | 'violet' | string
}

const props = withDefaults(defineProps<Props>(), {
  threshold: undefined,
  height: 3,
  variant: 'spectrum',
})

const clampedPercentage = computed(() => {
  if (isNaN(props.percentage) || props.percentage < 0) return 0
  return Math.min(100, props.percentage)
})

const barHeight = computed(() => {
  if (typeof props.height === 'number') return `${props.height}px`
  return props.height
})

// Semantic fill spectrum:
// < 70%: Muted Cyan / Emerald (#06b6d4 / #10b981)
// 70% – 84.9%: Warm Amber (#f59e0b)
// >= 85%: Rose Coral (#ef4444)
const fillColor = computed(() => {
  if (props.variant && props.variant !== 'spectrum') {
    switch (props.variant) {
      case 'cyan': return '#06b6d4'
      case 'emerald': return '#10b981'
      case 'amber': return '#f59e0b'
      case 'rose': return '#ef4444'
      case 'violet': return '#8b5cf6'
      default: return props.variant
    }
  }

  const pct = clampedPercentage.value
  const thresh = props.threshold

  if (thresh !== undefined) {
    if (pct >= thresh) return '#ef4444'
    if (pct >= thresh * 0.85) return '#f59e0b'
    return '#06b6d4'
  }

  if (pct >= 85) return '#ef4444'
  if (pct >= 70) return '#f59e0b'
  return '#06b6d4'
})
</script>

<template>
  <div
    class="percentage-bar-track"
    role="progressbar"
    :aria-valuenow="Math.round(clampedPercentage)"
    aria-valuemin="0"
    aria-valuemax="100"
    :style="{ height: barHeight }"
  >
    <div
      class="percentage-bar-fill"
      :style="{
        width: `${clampedPercentage}%`,
        backgroundColor: fillColor,
      }"
    />
  </div>
</template>

<style scoped>
.percentage-bar-track {
  width: 100%;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
  position: relative;
}

.percentage-bar-fill {
  height: 100%;
  border-radius: 9999px;
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1), background-color 0.3s ease;
}
</style>
