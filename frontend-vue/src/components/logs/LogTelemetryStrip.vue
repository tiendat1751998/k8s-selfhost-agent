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
@import '../../assets/styles/views/logstream.css';
</style>
