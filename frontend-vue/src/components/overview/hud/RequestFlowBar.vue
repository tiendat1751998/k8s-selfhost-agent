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
    <!-- Animated Background Stream & Laser Track -->
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
  height: 38px;
  padding: 0 16px;
  display: flex;
  align-items: center;
  border-radius: 10px;
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.85) 0%, rgba(30, 41, 59, 0.75) 100%);
  border: 1px solid rgba(56, 189, 248, 0.2);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.25), 0 0 14px rgba(6, 182, 212, 0.06);
  transition: border-color 0.4s ease, box-shadow 0.4s ease;
}
.request-flow-bar.flow-theme-emerald {
  border-color: rgba(16, 185, 129, 0.35);
  box-shadow: 0 0 18px rgba(16, 185, 129, 0.1), 0 4px 16px rgba(0, 0, 0, 0.25);
}
.request-flow-bar.flow-theme-amber {
  border-color: rgba(245, 158, 11, 0.45);
  box-shadow: 0 0 18px rgba(245, 158, 11, 0.14), 0 4px 16px rgba(0, 0, 0, 0.25);
}
.request-flow-bar.flow-theme-rose {
  border-color: rgba(244, 63, 94, 0.55);
  box-shadow: 0 0 22px rgba(244, 63, 94, 0.18), 0 4px 16px rgba(0, 0, 0, 0.25);
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
  background: linear-gradient(90deg, transparent 0%, rgba(56, 189, 248, 0.08) 20%, rgba(56, 189, 248, 0.18) 50%, rgba(56, 189, 248, 0.08) 80%, transparent 100%);
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
  opacity: 0.25;
}
.flow-dot {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 6px;
  height: 6px;
  border-radius: 50%;
  filter: blur(0.6px);
}
.flow-theme-emerald .flow-dot {
  background: #10b981;
  box-shadow: 0 0 10px #10b981, 0 0 20px rgba(16, 185, 129, 0.6);
}
.flow-theme-amber .flow-dot {
  background: #f59e0b;
  box-shadow: 0 0 10px #f59e0b, 0 0 20px rgba(245, 158, 11, 0.6);
}
.flow-theme-rose .flow-dot {
  background: #f43f5e;
  box-shadow: 0 0 10px #f43f5e, 0 0 20px rgba(244, 63, 94, 0.6);
}
.flow-dot.dot-1 { left: 0%; opacity: 0.3; }
.flow-dot.dot-2 { left: 12.5%; opacity: 0.6; }
.flow-dot.dot-3 { left: 25%; opacity: 0.9; }
.flow-dot.dot-4 { left: 37.5%; opacity: 0.7; }
.flow-dot.dot-5 { left: 50%; opacity: 0.3; }
.flow-dot.dot-6 { left: 62.5%; opacity: 0.6; }
.flow-dot.dot-7 { left: 75%; opacity: 0.9; }
.flow-dot.dot-8 { left: 87.5%; opacity: 0.7; }
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
  gap: 12px;
}
.flow-glyphs {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
}
.flow-theme-emerald .flow-glyphs { color: #10b981; }
.flow-theme-amber .flow-glyphs { color: #f59e0b; }
.flow-theme-rose .flow-glyphs { color: #f43f5e; }
.flow-glyph-dot {
  animation: glyphPulse var(--flow-duration, 2.5s) ease-in-out infinite;
}
.flow-glyph-dot.dot-a { animation-delay: 0s; }
.flow-glyph-dot.dot-b { animation-delay: calc(var(--flow-duration, 2.5s) * 0.25); }
.flow-glyph-dot.dot-c { animation-delay: calc(var(--flow-duration, 2.5s) * 0.5); }
.flow-glyph-arrow {
  font-size: 14px;
  font-weight: 800;
  margin-left: 2px;
}
@keyframes glyphPulse {
  0%, 100% {
    opacity: 0.35;
    transform: scale(0.85);
  }
  50% {
    opacity: 1;
    transform: scale(1.2);
    filter: drop-shadow(0 0 6px currentColor);
  }
}
.flow-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 9px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}
.flow-badge-emerald {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}
.flow-badge-amber {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}
.flow-badge-rose {
  background: rgba(244, 63, 94, 0.15);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.3);
}
.flow-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}	.pulse-active {
  animation: pulseDot 1.5s infinite;
}
@keyframes pulseDot {
  0% { transform: scale(0.95); opacity: 0.8; }
  50% { transform: scale(1.3); opacity: 1; filter: drop-shadow(0 0 4px currentColor); }
  100% { transform: scale(0.95); opacity: 0.8; }
}
.flow-stream-hint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.06em;
  color: rgba(148, 163, 184, 0.5);
  user-select: none;
}
.stream-pulse-tail {
  width: 24px;
  height: 2px;
  background: linear-gradient(90deg, transparent, rgba(56, 189, 248, 0.4));
  border-radius: 1px;
}
.font-mono {
  font-family: var(--font-mono, monospace);
}
.stream-text {
  display: none;
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

  .flow-badge {
    display: none;
  }
}
</style>