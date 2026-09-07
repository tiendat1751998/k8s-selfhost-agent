<script setup lang="ts">
defineProps<{
  linesStreamed: number
  errorRate: number
  bufferSize: number
  maxBufferSize: number
  latency: number
  isConnected: boolean
  isPaused: boolean
}>()
</script>

<template>
  <div class="log-telemetry-strip font-mono" role="status" aria-label="Stream Telemetry Strip">
    <!-- Desktop Telemetry Bar -->
    <div class="desktop-telemetry">
      <div class="telemetry-item">
        <span class="pulse-dot" :class="!isConnected ? 'pulse-dot-rose' : (isPaused ? 'pulse-dot-amber' : 'pulse-dot-emerald')"></span>
        <span class="telemetry-text" :class="isConnected ? (isPaused ? 'text-amber' : 'text-emerald') : 'text-rose'">
          {{ !isConnected ? 'OFFLINE' : (isPaused ? 'PAUSED' : 'LIVE STREAM') }}
        </span>
      </div>

      <span class="telemetry-sep">|</span>
      <div class="telemetry-item">
        <span class="telemetry-icon">⚡</span>
        <span class="telemetry-label">Ingested:</span>
        <span class="telemetry-val text-cyan">{{ linesStreamed.toLocaleString() }}</span>
        <span class="telemetry-unit">events</span>
      </div>

      <span class="telemetry-sep">|</span>
      <div class="telemetry-item">
        <span class="telemetry-icon">🚨</span>
        <span class="telemetry-label">Error Rate:</span>
        <span class="telemetry-val" :class="errorRate === 0 ? 'text-emerald' : (errorRate > 5 ? 'text-rose' : 'text-amber')">
          {{ errorRate }}%
        </span>
      </div>

      <span class="telemetry-sep">|</span>
      <div class="telemetry-item">
        <span class="telemetry-icon">💾</span>
        <span class="telemetry-label">Buffer:</span>
        <span class="telemetry-val text-slate">{{ bufferSize }}/{{ maxBufferSize }}</span>
        <div class="mini-gauge">
          <div class="mini-gauge-fill" :style="{ width: `${Math.min(100, Math.round((bufferSize / maxBufferSize) * 100))}%` }"></div>
        </div>
      </div>

      <span class="telemetry-sep">|</span>
      <div class="telemetry-item">
        <span class="telemetry-icon">🌐</span>
        <span class="telemetry-label">Latency:</span>
        <span class="telemetry-val text-emerald">{{ isConnected ? (latency > 0 ? `${latency}ms` : '<50ms') : '--' }}</span>
      </div>
    </div>

    <!-- Mobile Ultra-Compact Micro-Telemetry -->
    <div class="mobile-micro-telemetry">
      <span>⚡ {{ linesStreamed.toLocaleString() }} ev</span>
      <span class="sep">·</span>
      <span :class="errorRate === 0 ? 'text-emerald' : (errorRate > 5 ? 'text-rose' : 'text-amber')">🚨 {{ errorRate }}% err</span>
      <span class="sep">·</span>
      <span class="text-slate">💾 {{ bufferSize }}/{{ maxBufferSize }}</span>
      <span class="sep">·</span>
      <span class="text-emerald">{{ isConnected ? (latency > 0 ? `${latency}ms` : '<50ms') : 'OFF' }}</span>
    </div>
  </div>
</template>

<style scoped>
.log-telemetry-strip { height: 32px; display: flex; align-items: center; padding: 0 12px; background: rgba(11, 15, 25, 0.85); border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 6px; font-size: 11px; }
.desktop-telemetry { display: flex; align-items: center; gap: 10px; width: 100%; overflow-x: auto; white-space: nowrap; }
.mobile-micro-telemetry { display: none; }
.telemetry-item { display: inline-flex; align-items: center; gap: 5px; }
.telemetry-icon { font-size: 11px; line-height: 1; }
.telemetry-label { color: #64748b; font-weight: 600; }
.telemetry-val { font-weight: 700; }
.telemetry-unit { color: #475569; font-size: 10px; }
.telemetry-sep { color: rgba(255, 255, 255, 0.1); user-select: none; }
.mini-gauge { width: 36px; height: 4px; background: rgba(255, 255, 255, 0.08); border-radius: 2px; overflow: hidden; margin-left: 2px; }
.mini-gauge-fill { height: 100%; background: #38bdf8; border-radius: 2px; transition: width 0.2s ease; }

@media (max-width: 640px) {
  .log-telemetry-strip { height: 20px; font-size: 10px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; border: none; background: transparent; padding: 0 4px; justify-content: center; }
  .desktop-telemetry { display: none; }
  .mobile-micro-telemetry { display: flex; align-items: center; justify-content: center; gap: 4px; width: 100%; color: #94a3b8; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .mobile-micro-telemetry .sep { color: rgba(255, 255, 255, 0.25); margin: 0 1px; }
}
</style>
