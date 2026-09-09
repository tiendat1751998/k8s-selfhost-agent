<script setup lang="ts">
interface Props {
  linesStreamed: number
  errorRate: number
  bufferSaturation: number
  bufferSize: number
  maxBufferSize: number
  latency: number
  isConnected: boolean
  isPaused: boolean
}

defineProps<Props>()
</script>

<template>
  <section class="log-hud-grid" aria-label="Log stream performance telemetry">
    <!-- Card 1: Lines Streamed -->
    <div class="log-hud-card">
      <div class="log-hud-top">
        <div class="log-hud-title-group">
          <span class="log-hud-icon">⚡</span>
          <span class="log-hud-label">Lines Streamed</span>
        </div>
        <span
          class="log-hud-badge"
          :class="isPaused ? 'badge-warn text-amber' : 'badge-info text-cyan'"
        >
          {{ isPaused ? 'STREAM PAUSED' : 'ACTIVE INGEST' }}
        </span>
      </div>
      <div class="log-hud-value-row">
        <span class="log-hud-value font-mono">{{ linesStreamed.toLocaleString() }}</span>
        <span class="log-hud-subtext font-mono">events ingested</span>
      </div>
      <div class="log-hud-progress-bg">
        <div
          class="log-hud-progress-fill"
          style="width: 100%;"
          :style="{ backgroundColor: isPaused ? '#f59e0b' : '#38bdf8' }"
        ></div>
      </div>
    </div>

    <!-- Card 2: Error Rate -->
    <div class="log-hud-card">
      <div class="log-hud-top">
        <div class="log-hud-title-group">
          <span class="log-hud-icon">🚨</span>
          <span class="log-hud-label">Error Rate</span>
        </div>
        <span
          class="log-hud-badge"
          :class="errorRate === 0 ? 'badge-info text-emerald' : (errorRate > 5 ? 'badge-error text-rose' : 'badge-warn text-amber')"
        >
          {{ errorRate === 0 ? 'NOMINAL' : (errorRate > 5 ? 'CRITICAL' : 'ELEVATED') }}
        </span>
      </div>
      <div class="log-hud-value-row">
        <span
          class="log-hud-value font-mono"
          :class="errorRate === 0 ? 'text-emerald' : (errorRate > 5 ? 'text-rose' : 'text-amber')"
        >
          {{ errorRate }}%
        </span>
        <span class="log-hud-subtext font-mono">severity ratio</span>
      </div>
      <div class="log-hud-progress-bg">
        <div
          class="log-hud-progress-fill"
          :style="{
            width: `${Math.min(100, errorRate * 5)}%`,
            backgroundColor: errorRate > 5 ? '#f43f5e' : (errorRate > 0 ? '#f59e0b' : '#10b981')
          }"
        ></div>
      </div>
    </div>

    <!-- Card 3: Buffer Saturation -->
    <div class="log-hud-card">
      <div class="log-hud-top">
        <div class="log-hud-title-group">
          <span class="log-hud-icon">💾</span>
          <span class="log-hud-label">Buffer Saturation</span>
        </div>
        <span class="log-hud-badge badge-info text-cyan">
          RING BUFFER
        </span>
      </div>
      <div class="log-hud-value-row">
        <span class="log-hud-value font-mono">{{ bufferSaturation }}%</span>
        <span class="log-hud-subtext font-mono">{{ bufferSize }} / {{ maxBufferSize }}</span>
      </div>
      <div class="log-hud-progress-bg">
        <div
          class="log-hud-progress-fill"
          :style="{
            width: `${bufferSaturation}%`,
            backgroundColor: bufferSaturation > 90 ? '#f59e0b' : '#38bdf8'
          }"
        ></div>
      </div>
    </div>

    <!-- Card 4: Connection Latency -->
    <div class="log-hud-card">
      <div class="log-hud-top">
        <div class="log-hud-title-group">
          <span class="log-hud-icon">🌐</span>
          <span class="log-hud-label">Connection Latency</span>
        </div>
        <span
          class="pulse-dot"
          :class="!isConnected ? 'pulse-dot-rose' : (isPaused ? 'pulse-dot-amber' : 'pulse-dot-emerald')"
        ></span>
      </div>
      <div class="log-hud-value-row">
        <span
          class="log-hud-value font-mono"
          :class="isConnected ? 'text-emerald' : 'text-rose'"
        >
          {{ isConnected ? (latency > 0 ? `${latency}ms` : '--') : 'OFFLINE' }}
        </span>
        <span class="log-hud-subtext font-mono">
          {{ isConnected ? 'WebSocket <50ms' : 'Disconnected' }}
        </span>
      </div>
      <div class="log-hud-progress-bg">
        <div
          class="log-hud-progress-fill"
          :style="{
            width: isConnected ? '100%' : '0%',
            backgroundColor: isConnected ? '#10b981' : '#f43f5e'
          }"
        ></div>
      </div>
    </div>
  </section>
</template>
