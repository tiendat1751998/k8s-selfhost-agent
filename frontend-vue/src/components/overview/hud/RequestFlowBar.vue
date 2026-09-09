<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  isLiveWs?: boolean
  effectiveHttpRps: number
  httpQueuedReqs?: number
  httpErrorRate?: number
}

const props = withDefaults(defineProps<Props>(), {
  isLiveWs: false,
  httpQueuedReqs: 0,
  httpErrorRate: 0,
})

const isFlowIdle = computed(() => props.effectiveHttpRps <= 0)
const flowHealthColor = computed<'emerald' | 'amber' | 'rose'>(() => {
  if (props.httpErrorRate >= 5) return 'rose'
  if (props.httpQueuedReqs > 0 || props.httpErrorRate > 0) return 'amber'
  return 'emerald'
})

const flowStatusClass = computed(() => {
  return `flow-theme-${flowHealthColor.value}`
})

const flowStatusLabel = computed(() => {
  if (isFlowIdle.value) return 'INGRESS IDLE'
  if (props.httpErrorRate >= 5) return 'HIGH ERROR RATE'
  if (props.httpQueuedReqs > 0) return 'TRAFFIC QUEUED'
  return 'LIVE REQUEST FLOW'
})

const flowAnimationDuration = computed(() => {
  const rps = props.effectiveHttpRps
  if (rps <= 0) return 6.0
  return Math.max(0.4, Math.min(3.5, 3.0 / Math.pow(Math.max(1, rps), 0.45)))
})
</script>

<template>
  <section
    class="request-flow-bar glass-panel smooth-value"
    :class="flowStatusClass"
    :style="{ '--flow-duration': `${flowAnimationDuration}s` }"
  >
    <!-- Animated Background Stream & Subtle Track -->
    <div class="flow-track-background">
      <div class="flow-laser-line"></div>
      <div class="flow-stream" :class="{ 'flow-paused': isFlowIdle }">
        <div class="flow-dot dot-1"></div>
        <div class="flow-dot dot-2"></div>
        <div class="flow-dot dot-3"></div>
        <div class="flow-dot dot-4"></div>
        <div class="flow-dot dot-5"></div>
        <div class="flow-dot dot-6"></div>
        <div class="flow-dot dot-7"></div>
        <div class="flow-dot dot-8"></div>
      </div>
    </div>

    <div class="flow-bar-content">
      <!-- Animated Glyphs & Status Tag -->
      <div class="flow-indicator-group">
        <div class="flow-glyphs" :class="{ 'flow-paused': isFlowIdle }">
          <span class="flow-glyph-dot dot-a">●</span>
          <span class="flow-glyph-dot dot-b">●</span>
          <span class="flow-glyph-dot dot-c">●</span>
          <span class="flow-glyph-arrow">→</span>
        </div>
        <span class="flow-badge" :class="`flow-badge-${flowHealthColor}`">
          <span class="flow-status-dot" :class="{ 'pulse-active': !isFlowIdle }"></span>
          {{ flowStatusLabel }}
        </span>
      </div>

      <div class="flow-stream-hint font-mono">
        <span class="stream-pulse-tail"></span>
      </div>
    </div>
  </section>
</template>

<style scoped>
@import '../../../assets/styles/views/overview.css';
</style>