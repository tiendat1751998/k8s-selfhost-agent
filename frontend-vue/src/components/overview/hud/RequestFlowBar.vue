<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  isLiveWs?: boolean
  effectiveHttpRps: number
  httpActiveConns?: number
  httpQueuedReqs?: number
  httpErrorRate?: number
  clusterAvgLatencyMs?: number
}

const props = withDefaults(defineProps<Props>(), {
  isLiveWs: false,
  httpActiveConns: 0,
  httpQueuedReqs: 0,
  httpErrorRate: 0,
  clusterAvgLatencyMs: 2.4,
})

defineEmits<{
  (e: 'open-deep-dive'): void
}>()

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
    <!-- Animated Background Stream -->
    <div class="flow-track-background">
      <div class="flow-stream" :class="{ 'flow-paused': isFlowIdle }">
        <div class="flow-dot dot-1"></div>
        <div class="flow-dot dot-2"></div>
        <div class="flow-dot dot-3"></div>
        <div class="flow-dot dot-4"></div>
        <div class="flow-dot dot-5"></div>
        <div class="flow-dot dot-6"></div>
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

      <!-- Live Metrics Items -->
      <div class="flow-metrics-group">
        <div class="flow-metric-item">
          <span class="flow-metric-icon">🌐</span>
          <span class="flow-metric-value font-mono">{{ httpActiveConns.toLocaleString() }}</span>
          <span class="flow-metric-label">active connections</span>
        </div>

        <div class="flow-metric-divider"></div>

        <div class="flow-metric-item" :class="{ 'metric-warning': httpQueuedReqs > 0 }">
          <span class="flow-metric-icon">⏳</span>
          <span class="flow-metric-value font-mono">{{ httpQueuedReqs }}</span>
          <span class="flow-metric-label">queued</span>
        </div>

        <div class="flow-metric-divider"></div>

        <div class="flow-metric-item">
          <span class="flow-metric-icon">⚡</span>
          <span class="flow-metric-value font-mono text-cyan">{{ effectiveHttpRps >= 100 ? Math.round(effectiveHttpRps).toLocaleString() : (effectiveHttpRps > 0 ? effectiveHttpRps.toFixed(1) : '0.0') }}</span>
          <span class="flow-metric-label">req/s</span>
        </div>

        <div class="flow-metric-divider"></div>

        <div class="flow-metric-item">
          <span class="flow-metric-icon">⏱️</span>
          <span class="flow-metric-value font-mono text-violet">{{ clusterAvgLatencyMs > 0 ? clusterAvgLatencyMs.toFixed(1) : '2.4' }}ms</span>
          <span class="flow-metric-label">latency</span>
        </div>

        <div class="flow-metric-divider"></div>

        <div class="flow-metric-item" :class="httpErrorRate >= 5 ? 'metric-critical' : httpErrorRate > 0 ? 'metric-warning' : ''">
          <span class="flow-metric-icon">{{ httpErrorRate >= 5 ? '❌' : httpErrorRate > 0 ? '⚠️' : '🛡️' }}</span>
          <span class="flow-metric-value font-mono">{{ httpErrorRate.toFixed(1) }}%</span>
          <span class="flow-metric-label">errors</span>
        </div>

        <div class="flow-metric-divider"></div>

        <button
          type="button"
          class="btn-flow-deepdive font-mono"
          @click="$emit('open-deep-dive')"
          title="Open cluster telemetry deep-dive modal"
        >
          <span>🔍 Deep-Dive</span>
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.request-flow-bar {
  position: relative;
  overflow: hidden;
  padding: 10px 18px;
  border-radius: 12px;
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.85) 0%, rgba(30, 41, 59, 0.75) 100%);
  border: 1px solid rgba(56, 189, 248, 0.2);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3), 0 0 16px rgba(6, 182, 212, 0.08);
  transition: border-color 0.5s ease, box-shadow 0.5s ease;
}

/* Status Themes */
.request-flow-bar.flow-theme-emerald {
  border-color: rgba(16, 185, 129, 0.35);
  box-shadow: 0 0 20px rgba(16, 185, 129, 0.12), 0 4px 20px rgba(0, 0, 0, 0.3);
}

.request-flow-bar.flow-theme-amber {
  border-color: rgba(245, 158, 11, 0.45);
  box-shadow: 0 0 20px rgba(245, 158, 11, 0.15), 0 4px 20px rgba(0, 0, 0, 0.3);
}

.request-flow-bar.flow-theme-rose {
  border-color: rgba(244, 63, 94, 0.55);
  box-shadow: 0 0 25px rgba(244, 63, 94, 0.2), 0 4px 20px rgba(0, 0, 0, 0.3);
}

/* Particle / Laser Track in Background */
.flow-track-background {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  pointer-events: none;
  overflow: hidden;
}

.flow-stream {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
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
  filter: blur(1px);
}

.flow-theme-emerald .flow-dot {
  background: #10b981;
  box-shadow: 0 0 12px #10b981, 0 0 24px rgba(16, 185, 129, 0.6);
}
.flow-theme-amber .flow-dot {
  background: #f59e0b;
  box-shadow: 0 0 12px #f59e0b, 0 0 24px rgba(245, 158, 11, 0.6);
}
.flow-theme-rose .flow-dot {
  background: #f43f5e;
  box-shadow: 0 0 12px #f43f5e, 0 0 24px rgba(244, 63, 94, 0.6);
}

.flow-dot.dot-1 { left: 0%; opacity: 0.2; }
.flow-dot.dot-2 { left: 20%; opacity: 0.5; }
.flow-dot.dot-3 { left: 40%; opacity: 0.8; }
.flow-dot.dot-4 { left: 60%; opacity: 0.9; }
.flow-dot.dot-5 { left: 80%; opacity: 0.6; }
.flow-dot.dot-6 { left: 100%; opacity: 0.2; }

@keyframes flowParticleStream {
  0% {
    transform: translateX(-30%);
  }
  100% {
    transform: translateX(30%);
  }
}

/* Content Layout */
.flow-bar-content {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
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
}

.pulse-active {
  animation: pulseDot 1.5s infinite;
}

@keyframes pulseDot {
  0% { transform: scale(0.95); opacity: 0.8; }
  50% { transform: scale(1.3); opacity: 1; filter: drop-shadow(0 0 4px currentColor); }
  100% { transform: scale(0.95); opacity: 0.8; }
}

.flow-metrics-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.flow-metric-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 8px;
  padding: 5px 11px;
  transition: all 0.2s ease;
}

.flow-metric-item:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.12);
}

.flow-metric-icon {
  font-size: 13px;
}

.flow-metric-value {
  font-weight: 700;
  font-size: 13px;
  color: var(--text-primary, #f8fafc);
}

.flow-metric-label {
  color: var(--text-secondary, #94a3b8);
  font-size: 11px;
}

.flow-metric-item.metric-warning {
  background: rgba(245, 158, 11, 0.08);
  border-color: rgba(245, 158, 11, 0.25);
}

.flow-metric-item.metric-warning .flow-metric-value,
.flow-metric-item.metric-warning .flow-metric-label {
  color: #f59e0b;
}

.flow-metric-item.metric-critical {
  background: rgba(244, 63, 94, 0.08);
  border-color: rgba(244, 63, 94, 0.25);
}

.flow-metric-item.metric-critical .flow-metric-value,
.flow-metric-item.metric-critical .flow-metric-label {
  color: #f43f5e;
}

.flow-metric-divider {
  width: 1px;
  height: 14px;
  background: rgba(255, 255, 255, 0.12);
}

.btn-flow-deepdive {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.3);
  color: #38bdf8;
  padding: 5px 12px;
  border-radius: 8px;
  font-size: 11.5px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-flow-deepdive:hover {
  background: rgba(56, 189, 248, 0.22);
  border-color: #38bdf8;
  transform: translateY(-1px);
  box-shadow: 0 0 12px rgba(56, 189, 248, 0.25);
}

.font-mono {
  font-family: var(--font-mono, monospace);
}

.text-cyan { color: #06b6d4; }
.text-violet { color: #8b5cf6; }

@media (max-width: 700px) {
  .flow-bar-content {
    flex-direction: column;
    align-items: flex-start;
  }
  .flow-metrics-group {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
