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
.request-flow-bar {
  position: relative;
  overflow: hidden;
  height: 36px;
  padding: 0 14px;
  display: flex;
  align-items: center;
  border-radius: 8px;
  background: var(--bg-card, #161f30);
  border: 1px solid var(--color-hairline, rgba(255, 255, 255, 0.08));
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
  transition: border-color 0.25s ease, background-color 0.25s ease;
}

.request-flow-bar.flow-theme-emerald {
  border-color: rgba(16, 185, 129, 0.25);
}

.request-flow-bar.flow-theme-amber {
  border-color: rgba(245, 158, 11, 0.35);
}

.request-flow-bar.flow-theme-rose {
  border-color: rgba(244, 63, 94, 0.35);
}

.flow-track-background {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}

.flow-laser-line {
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  height: 1px;
  transform: translateY(-50%);
  background: linear-gradient(90deg, transparent 0%, rgba(255, 255, 255, 0.04) 20%, rgba(255, 255, 255, 0.08) 50%, rgba(255, 255, 255, 0.04) 80%, transparent 100%);
}

.flow-stream {
  position: absolute;
  inset: 0;
  width: 200%;
  height: 100%;
  display: flex;
  align-items: center;
  animation: flowParticleStream var(--flow-duration, 2.5s) linear infinite;
}

.flow-paused {
  animation-play-state: paused !important;
  opacity: 0.15;
}

.flow-dot {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 4px;
  height: 4px;
  border-radius: 50%;
}

.flow-theme-emerald .flow-dot {
  background: #10b981;
  opacity: 0.5;
}

.flow-theme-amber .flow-dot {
  background: #f59e0b;
  opacity: 0.5;
}

.flow-theme-rose .flow-dot {
  background: #f43f5e;
  opacity: 0.5;
}

.flow-dot.dot-1 { left: 0%; opacity: 0.15; }
.flow-dot.dot-2 { left: 12.5%; opacity: 0.3; }
.flow-dot.dot-3 { left: 25%; opacity: 0.5; }
.flow-dot.dot-4 { left: 37.5%; opacity: 0.35; }
.flow-dot.dot-5 { left: 50%; opacity: 0.15; }
.flow-dot.dot-6 { left: 62.5%; opacity: 0.3; }
.flow-dot.dot-7 { left: 75%; opacity: 0.5; }
.flow-dot.dot-8 { left: 87.5%; opacity: 0.35; }

@keyframes flowParticleStream {
  0% {
    transform: translateX(-50%);
  }
  100% {
    transform: translateX(0%);
  }
}

.flow-bar-content {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.flow-indicator-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.flow-glyphs {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 9px;
}

.flow-theme-emerald .flow-glyphs { color: #10b981; }
.flow-theme-amber .flow-glyphs { color: #f59e0b; }
.flow-theme-rose .flow-glyphs { color: #f43f5e; }

.flow-glyph-dot {
  animation: glyphPulse var(--flow-duration, 2.5s) ease-in-out infinite;
  opacity: 0.5;
}

.flow-glyph-dot.dot-a { animation-delay: 0s; }
.flow-glyph-dot.dot-b { animation-delay: calc(var(--flow-duration, 2.5s) * 0.25); }
.flow-glyph-dot.dot-c { animation-delay: calc(var(--flow-duration, 2.5s) * 0.5); }

.flow-glyph-arrow {
  font-size: 11px;
  font-weight: 700;
  margin-left: 2px;
  opacity: 0.7;
}

@keyframes glyphPulse {
  0%, 100% {
    opacity: 0.3;
    transform: scale(0.9);
  }
  50% {
    opacity: 0.85;
    transform: scale(1.05);
  }
}

.flow-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.flow-badge-emerald {
  background: rgba(16, 185, 129, 0.1);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.2);
}

.flow-badge-amber {
  background: rgba(245, 158, 11, 0.1);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.2);
}

.flow-badge-rose {
  background: rgba(244, 63, 94, 0.1);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.2);
}

.flow-status-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
}

.pulse-active {
  animation: pulseDot 2s infinite ease-in-out;
}

@keyframes pulseDot {
  0%, 100% { opacity: 0.6; }
  50% { opacity: 1; }
}

.flow-stream-hint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted, #64748b);
  user-select: none;
}

.stream-pulse-tail {
  width: 20px;
  height: 2px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 1px;
}

.font-mono {
  font-family: var(--font-mono, monospace);
}

@media (max-width: 768px) {
  .flow-stream-hint {
    display: none !important;
  }
}

@media (max-width: 640px) {
  .request-flow-bar {
    height: 32px;
    padding: 0 10px;
  }

  .flow-glyphs {
    display: none;
  }
}
</style>