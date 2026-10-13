<script setup lang="ts">
import { computed } from 'vue'

export interface PercentageBarProps {
  value?: number
  percentage?: number
  threshold?: number
  height?: number | string
  variant?: 'spectrum' | 'cyan' | 'emerald' | 'amber' | 'rose' | 'violet' | string
  class?: string | Record<string, boolean | undefined | null> | (string | Record<string, boolean | undefined | null>)[]
}

const props = withDefaults(defineProps<PercentageBarProps>(), {
  threshold: undefined,
  height: 3,
  variant: 'spectrum',
})

const resolvedValue = computed<number>(() => {
  const raw = props.value !== undefined ? props.value : (props.percentage !== undefined ? props.percentage : 0)
  if (Number.isNaN(raw) || raw < 0) return 0
  return Math.min(100, raw)
})

const barHeight = computed<string>(() => {
  if (typeof props.height === 'number') return `${props.height}px`
  return props.height || '3px'
})

// Semantic fill thresholds:
// < 70%: Emerald (#10b981)
// 70% – 84.9%: Amber (#f59e0b)
// >= 85%: Rose (#f43f5e)
const fillColor = computed<string>(() => {
  if (props.variant && props.variant !== 'spectrum') {
    switch (props.variant) {
      case 'cyan': return '#06b6d4'
      case 'emerald': return '#10b981'
      case 'amber': return '#f59e0b'
      case 'rose': return '#f43f5e'
      case 'violet': return '#8b5cf6'
      default: return props.variant
    }
  }

  const pct = resolvedValue.value
  const thresh = props.threshold

  if (thresh !== undefined) {
    if (pct >= thresh) return '#f43f5e'
    if (pct >= thresh * 0.85) return '#f59e0b'
    return '#10b981'
  }

  if (pct >= 85) return '#f43f5e'
  if (pct >= 70) return '#f59e0b'
  return '#10b981'
})
</script>

<template>
  <div
    class="percentage-bar-track"
    :class="props.class"
    role="progressbar"
    :aria-valuenow="Math.round(resolvedValue)"
    aria-valuemin="0"
    aria-valuemax="100"
    :style="{ height: barHeight }"
  >
    <div
      class="percentage-bar-fill"
      :style="{
        width: `${resolvedValue}%`,
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
