<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  showTelemetry: boolean
  totalHosts: number
  connectedHosts: number
  disconnectedHosts: number
  errorHosts: number
  typeCounts: Record<string, number>
  onlineHostsCount: number
  avgLatency: number
}>()

const availabilityPct = computed(() => {
  if (props.totalHosts <= 0) return 0
  return Math.min(100, Math.round((props.onlineHostsCount / props.totalHosts) * 100))
})
</script>

<template>
  <Transition name="fade">
    <section
      v-if="showTelemetry"
      class="fleet-diagnostics-panel animate-fade-in"
      aria-label="Fleet Diagnostics & Telemetry"
    >
      <div class="diagnostics-header">
        <div class="diagnostics-title-group">
          <h2 class="diagnostics-title">Fleet Diagnostics</h2>
          <span class="diagnostics-subtitle">Real-time connectivity, latency benchmarks, and protocol health</span>
        </div>
      </div>

      <div class="diagnostics-grid">
        <!-- 1. Fleet Availability -->
        <div class="diagnostic-card diag-col">
          <div class="diag-card-header">
            <span class="diag-card-label">Fleet Availability</span>
            <span
              class="diag-status-dot"
              :class="availabilityPct >= 95 ? 'dot-emerald' : availabilityPct > 0 ? 'dot-amber' : 'dot-rose'"
              aria-hidden="true"
            ></span>
          </div>
          <div class="diag-card-value font-mono">
            {{ availabilityPct }}<span class="diag-unit">%</span>
          </div>
          <div class="diag-card-sub text-muted">
            {{ onlineHostsCount }} of {{ totalHosts }} reachable
          </div>
        </div>

        <!-- 2. Measured Probe Latency -->
        <div class="diagnostic-card diag-col">
          <div class="diag-card-header">
            <span class="diag-card-label">Probe Latency (RTT)</span>
            <span
              class="diag-status-dot"
              :class="avgLatency > 0 && avgLatency < 100 ? 'dot-emerald' : avgLatency >= 100 ? 'dot-amber' : 'dot-slate'"
              aria-hidden="true"
            ></span>
          </div>
          <div class="diag-card-value font-mono">
            {{ avgLatency > 0 ? `${avgLatency}ms` : '--' }}
          </div>
          <div class="diag-card-sub text-muted">
            {{ avgLatency > 0 ? 'Active HTTP/TCP probe' : 'Run probe to measure' }}
          </div>
        </div>

        <!-- 3. Degraded / Unreachable -->
        <div class="diagnostic-card diag-col">
          <div class="diag-card-header">
            <span class="diag-card-label">Degraded / Unreachable</span>
            <span
              class="diag-status-dot"
              :class="(disconnectedHosts + errorHosts) === 0 ? 'dot-emerald' : 'dot-rose'"
              aria-hidden="true"
            ></span>
          </div>
          <div class="diag-card-value font-mono" :class="{ 'val-danger': (disconnectedHosts + errorHosts) > 0 }">
            {{ disconnectedHosts + errorHosts }}
          </div>
          <div class="diag-card-sub text-muted">
            {{ errorHosts }} error · {{ disconnectedHosts }} offline
          </div>
        </div>

        <!-- 4. Protocol Distribution -->
        <div class="diagnostic-card diag-col">
          <div class="diag-card-header">
            <span class="diag-card-label">Protocol Fleet</span>
            <span class="diag-status-dot dot-slate" aria-hidden="true"></span>
          </div>
          <div class="diag-card-value font-mono">
            {{ (typeCounts['k8s'] || 0) + (typeCounts['agent'] || 0) + (typeCounts['docker'] || 0) }}
          </div>
          <div class="diag-card-sub text-muted">
            {{ typeCounts['k8s'] || 0 }} K8s · {{ typeCounts['agent'] || 0 }} Agent · {{ typeCounts['docker'] || 0 }} Docker
          </div>
        </div>
      </div>
    </section>
  </Transition>
</template>

<style scoped>
.fleet-diagnostics-panel {
  background: var(--bg-card);
  border: 1px solid var(--color-border-subtle);
  border-radius: 8px;
  padding: 14px 16px;
  margin: 0;
  box-shadow: none;
}

.diagnostics-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.diagnostics-title-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.diagnostics-title {
  font-family: var(--font-sans);
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
  letter-spacing: -0.01em;
}

.diagnostics-subtitle {
  font-family: var(--font-sans);
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.3;
}

.diagnostics-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

.diagnostic-card {
  background: var(--color-surface-inset);
  border: 1px solid var(--color-border-subtle);
  border-radius: 6px;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.diag-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.diag-card-label {
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.diag-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-emerald {
  background-color: var(--color-status-online, #3fb950);
}

.dot-amber {
  background-color: var(--color-status-warning, #f59e0b);
}

.dot-rose {
  background-color: var(--color-status-offline, #f85149);
}

.dot-slate {
  background-color: var(--text-muted, #94a3b8);
}

.diag-card-value {
  font-family: var(--font-mono);
  font-size: 20px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1.15;
  letter-spacing: -0.02em;
}

.diag-unit {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary, #9ca3af);
  margin-left: 2px;
}

.val-danger {
  color: var(--color-status-offline, #f85149);
}

.diag-card-sub {
  font-family: var(--font-sans);
  font-size: 11px;
  font-weight: 400;
  line-height: 1.3;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 640px) {
  .fleet-diagnostics-panel {
    padding: 12px;
    margin: 0;
  }
  .diagnostics-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }
  .diagnostic-card {
    padding: 8px 10px;
    gap: 4px;
  }
  .diag-card-value {
    font-size: 18px;
  }
  .diag-card-label {
    font-size: 11px;
  }
  .diag-card-sub {
    font-size: 10px;
  }
}
</style>
