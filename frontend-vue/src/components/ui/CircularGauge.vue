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
  }
)

const clampedPercent = computed(() => {
  if (isNaN(props.percent) || props.percent < 0) return 0
  return Math.min(100, Math.round(props.percent * 10) / 10)
})

const strokeColor = computed(() => {
  if (props.disabled) return 'var(--text-muted)'
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
  <div class="circular-gauge-wrapper" :style="{ width: `${size}px` }">
    <div class="gauge-container" :style="{ width: `${size}px`, height: `${size}px` }">
      <svg
        viewBox="0 0 36 36"
        class="circular-gauge"
        :class="{ 'is-critical': clampedPercent >= 80 && !disabled }"
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
          :stroke-dasharray="`${clampedPercent}, 100`"
          :style="{ filter: `drop-shadow(0 0 4px ${glowColor})` }"
          d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
        />
        <!-- Center Value -->
        <text
          v-if="showValue"
          x="18"
          y="20.5"
          class="gauge-text"
          :fill="strokeColor"
        >
          {{ Math.round(clampedPercent) }}{{ unit }}
        </text>
      </svg>
    </div>
    <span v-if="label" class="gauge-label">{{ label }}</span>
  </div>
</template>

<style scoped>
.circular-gauge-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.gauge-container {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

.circular-gauge {
  width: 100%;
  height: 100%;
  transform: rotate(0deg);
}

.gauge-bg {
  fill: none;
  stroke: rgba(255, 255, 255, 0.08);
}

.gauge-fill {
  fill: none;
  stroke-linecap: round;
  transition: stroke-dasharray 0.6s cubic-bezier(0.4, 0, 0.2, 1), stroke 0.4s ease;
}

.gauge-text {
  font-family: var(--font-mono);
  font-size: 8px;
  font-weight: 700;
  text-anchor: middle;
  transition: fill 0.3s ease;
}

.gauge-label {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.is-critical .gauge-fill {
  animation: gauge-pulse 2s infinite ease-in-out;
}

@keyframes gauge-pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.65;
  }
}
</style>
