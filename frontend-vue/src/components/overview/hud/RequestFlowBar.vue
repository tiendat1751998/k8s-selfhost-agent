<script setup lang="ts">
import { computed } from 'vue'
import BaseIcon from '../../ui/BaseIcon.vue'

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

const flowStatusLabel = computed(() => {
  if (isFlowIdle.value) return 'INGRESS IDLE'
  if (props.httpErrorRate >= 5) return 'HIGH ERROR RATE'
  if (props.httpQueuedReqs > 0) return 'TRAFFIC QUEUED'
  return 'OPTIMAL'
})

const flowAnimationDuration = computed(() => {
  const rps = props.effectiveHttpRps
  if (rps <= 0) return 6.0
  return Math.max(0.4, Math.min(3.5, 3.0 / Math.pow(Math.max(1, rps), 0.45)))
})
</script>

<template>
  <section
    class="request-flow-bar glass-panel"
    :class="`flow-theme-${flowHealthColor}`"
    :style="{ '--flow-duration': `${flowAnimationDuration}s` }"
    aria-label="Live Request Telemetry Strip"
  >
    <div class="flow-track-background" aria-hidden="true">
      <div class="flow-laser-line"></div>
      <div class="flow-stream" :class="{ 'flow-paused': isFlowIdle }">
        <div class="flow-beam"></div>
      </div>
    </div>

    <div class="flow-bar-content">
      <div class="flow-left-group">
        <BaseIcon name="activity" size="xs" class="flow-icon" />
        <span class="flow-title font-mono">LIVE REQUEST FLOW</span>
        <span class="flow-status-dot" :class="[`dot-${flowHealthColor}`, { 'pulse-active': !isFlowIdle }]" />
      </div>

      <div class="flow-metrics-group font-mono">
        <div class="metric-item">
          <span class="metric-val text-emerald">{{ effectiveHttpRps }}</span>
          <span class="metric-unit">req/s</span>
        </div>
        <span class="metric-divider">/</span>
        <div class="metric-item">
          <span class="metric-val" :class="httpQueuedReqs > 0 ? 'text-amber' : 'text-slate'">{{ httpQueuedReqs }}</span>
          <span class="metric-unit">queued</span>
        </div>
        <span class="metric-divider">/</span>
        <div class="metric-item">
          <span class="metric-val" :class="httpErrorRate >= 5 ? 'text-rose' : httpErrorRate > 0 ? 'text-amber' : 'text-slate'">
            {{ Number(httpErrorRate).toFixed(1) }}%
          </span>
          <span class="metric-unit">err</span>
        </div>
      </div>

      <div class="flow-right-group">
        <span class="flow-badge font-mono" :class="`flow-badge-${flowHealthColor}`">
          {{ flowStatusLabel }}
        </span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.request-flow-bar {
  position: relative;
  overflow: hidden;
  height: 34px;
  padding: 0 12px;
  display: flex;
  align-items: center;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  transition: border-color 0.25s ease;
  margin: 10px 0;
}
.flow-theme-emerald { border-color: rgba(16, 185, 129, 0.25); }
.flow-theme-amber { border-color: rgba(245, 158, 11, 0.35); }
.flow-theme-rose { border-color: rgba(244, 63, 94, 0.35); }
.flow-track-background { position: absolute; inset: 0; pointer-events: none; overflow: hidden; }
.flow-laser-line {
  position: absolute; top: 50%; left: 0; right: 0; height: 1px; transform: translateY(-50%);
  background: linear-gradient(90deg, transparent 0%, rgba(255, 255, 255, 0.04) 20%, rgba(255, 255, 255, 0.08) 50%, rgba(255, 255, 255, 0.04) 80%, transparent 100%);
}
.flow-stream { position: absolute; inset: 0; width: 100%; height: 100%; animation: flowBeamAnimation var(--flow-duration, 2.5s) linear infinite; }
.flow-paused { animation-play-state: paused !important; opacity: 0.15; }
.flow-beam {
  position: absolute; top: 50%; transform: translateY(-50%); width: 60px; height: 2px; border-radius: 9999px;
  background: linear-gradient(90deg, transparent, currentColor, transparent); opacity: 0.6;
}
.flow-theme-emerald .flow-beam { color: #10b981; }
.flow-theme-amber .flow-beam { color: #f59e0b; }
.flow-theme-rose .flow-beam { color: #f43f5e; }
@keyframes flowBeamAnimation { 0% { transform: translateX(-100%); } 100% { transform: translateX(200%); } }
.flow-bar-content { position: relative; z-index: 2; display: flex; align-items: center; justify-content: space-between; width: 100%; }
.flow-left-group { display: flex; align-items: center; gap: 8px; }
.flow-icon { color: var(--text-secondary, #94a3b8); }
.flow-title { font-size: 11px; font-weight: 700; color: var(--text-secondary, #94a3b8); letter-spacing: 0.04em; text-transform: uppercase; }
.flow-status-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.dot-emerald { background: #10b981; box-shadow: 0 0 6px #10b981; }
.dot-amber { background: #f59e0b; box-shadow: 0 0 6px #f59e0b; }
.dot-rose { background: #f43f5e; box-shadow: 0 0 6px #f43f5e; }
.pulse-active { animation: pulseDot 2s infinite ease-in-out; }
@keyframes pulseDot { 0%, 100% { opacity: 0.6; } 50% { opacity: 1; } }
.flow-metrics-group { display: flex; align-items: center; gap: 12px; font-size: 12px; }
.metric-item { display: inline-flex; align-items: baseline; gap: 4px; }
.metric-val { font-weight: 700; font-variant-numeric: tabular-nums; }
.metric-unit { font-size: 10px; color: var(--text-muted, #64748b); text-transform: uppercase; }
.metric-divider { color: rgba(255, 255, 255, 0.15); font-size: 11px; }
.flow-right-group { display: flex; align-items: center; }
.flow-badge { display: inline-flex; align-items: center; padding: 1px 7px; border-radius: 4px; font-size: 9.5px; font-weight: 700; letter-spacing: 0.04em; text-transform: uppercase; }
.flow-badge-emerald { background: rgba(16, 185, 129, 0.12); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.25); }
.flow-badge-amber { background: rgba(245, 158, 11, 0.12); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.25); }
.flow-badge-rose { background: rgba(244, 63, 94, 0.12); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.25); }
.font-mono { font-family: var(--font-mono, monospace); }
.text-emerald { color: #10b981; }
.text-amber { color: #f59e0b; }
.text-rose { color: #f43f5e; }
.text-slate { color: #94a3b8; }
@media (max-width: 640px) {
  .metric-divider { display: none; }
  .flow-metrics-group { gap: 8px; font-size: 11px; }
  .flow-title { display: none; }
}
</style>