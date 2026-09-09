<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    percent: number
    label?: string
    size?: number
    strokeWidth?: number
    color?: 'auto' | 'emerald' | 'amber' | 'rose' | 'cyan' | 'violet'
    showValue?: boolean
    unit?: string
    disabled?: boolean
    disabledText?: string
  }>(),
  {
    percent: 0,
    label: '',
    size: 48,
    strokeWidth: 3.5,
    color: 'auto',
    showValue: true,
    unit: '%',
    disabled: false,
    disabledText: '—',
  }
)

const clampedPercent = computed(() => {
  if (isNaN(props.percent) || props.percent < 0) return 0
  return Math.min(100, Math.round(props.percent * 10) / 10)
})

const strokeColor = computed(() => {
  if (props.disabled) return 'var(--text-muted, #64748b)'
  if (props.color && props.color !== 'auto') {
    switch (props.color) {
      case 'emerald': return '#10b981'
      case 'amber': return '#f59e0b'
      case 'rose': return '#f43f5e'
      case 'cyan': return '#06b6d4'
      case 'violet': return '#8b5cf6'
    }
  }
  const val = clampedPercent.value
  if (val >= 80) return '#f43f5e' // Overload Red
  if (val >= 60) return '#f59e0b' // Warning Amber
  return '#10b981' // Healthy Green
})

const glowColor = computed(() => {
  if (props.disabled) return 'transparent'
  const val = clampedPercent.value
  if (val >= 80) return 'rgba(244, 63, 94, 0.4)'
  if (val >= 60) return 'rgba(245, 158, 11, 0.3)'
  return 'rgba(16, 185, 129, 0.25)'
})
</script>

<template>
  <div
    class="circular-gauge-wrapper"
    :class="{ 'is-disabled': disabled }"
    :style="{ width: `${size}px` }"
  >
    <div class="gauge-container" :style="{ width: `${size}px`, height: `${size}px` }">
      <svg
        viewBox="0 0 36 36"
        class="circular-gauge"
        :class="{
          'is-critical': clampedPercent >= 80 && !disabled,
          'is-disabled': disabled
        }"
      >
        <!-- Background Track -->
        <path
          class="gauge-bg"
          :stroke-width="strokeWidth"
          d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
        />
        <!-- Active Progress Arc -->
        <path
          class="gauge-fill"
          :stroke-width="strokeWidth"
          :stroke="strokeColor"
          :stroke-dasharray="disabled ? '0, 100' : `${clampedPercent}, 100`"
          :style="{ filter: `drop-shadow(0 0 4px ${glowColor})` }"
          d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
        />
        <!-- Center Value -->
        <text
          v-if="showValue"
          x="18"
          :y="disabled ? 21.5 : 20.5"
          class="gauge-text"
          :class="{ 'is-disabled-val': disabled }"
          :fill="strokeColor"
        >
          {{ disabled ? disabledText : `${Math.round(clampedPercent)}${unit}` }}
        </text>
      </svg>
    </div>
    <span v-if="label" class="gauge-label" :class="{ 'gauge-label-disabled': disabled }">{{ label }}</span>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/ui/common.css';
</style>
