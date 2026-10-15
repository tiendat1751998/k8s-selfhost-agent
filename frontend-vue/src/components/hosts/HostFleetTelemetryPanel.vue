<script setup lang="ts">
import { computed } from 'vue'
import HostMetricsHud from './HostMetricsHud.vue'
import CanvasTimeSeries, { type TimeSeriesItem } from '../telemetry/CanvasTimeSeries.vue'

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

const fleetTelemetryWindow = computed(() => {
  const stepMs = 30_000, count = 16, now = Date.now()
  const baseTime = Math.floor(now / stepMs) * stepMs
  const timestamps: number[] = []
  for (let i = count - 1; i >= 0; i--) timestamps.push(baseTime - i * stepMs)

  const activeCount = props.onlineHostsCount
  const total = Math.max(1, props.totalHosts)
  const isFleetOnline = activeCount > 0
  const latencyTarget = isFleetOnline && props.avgLatency > 0 ? props.avgLatency : 0

  const latencyData: [number, number][] = timestamps.map((t) => {
    return [t, latencyTarget]
  })

  const availabilityPct = total > 0 ? Math.min(100, Math.round((activeCount / total) * 100)) : 0
  const availabilityData: [number, number][] = timestamps.map((t) => {
    return [t, availabilityPct]
  })

  return {
    latency: [{ name: 'Fleet Probe Latency', data: latencyData, color: '#06b6d4' }] as TimeSeriesItem[],
    availability: [{ name: 'Host Availability', data: availabilityData, color: '#10b981' }] as TimeSeriesItem[],
  }
})

const latencyThresholds = [
  { value: 50, color: '#f59e0b', label: 'Warn 50ms' },
  { value: 100, color: '#f43f5e', label: 'Crit 100ms' },
]

const availabilityThresholds = [
  { value: 95, color: '#f59e0b', label: 'SLA 95%' },
]
</script>

<template>
  <Transition name="fade">
    <div v-if="showTelemetry" class="section-card glass-panel fleet-telemetry-panel animate-fade-in">
      <div class="hosts-telemetry-hud-strip">
        <HostMetricsHud
          :total-hosts="totalHosts"
          :connected-hosts="connectedHosts"
          :disconnected-hosts="disconnectedHosts"
          :error-hosts="errorHosts"
          :type-counts="typeCounts"
        />
      </div>
      <div class="section-top">
        <div>
          <div class="panel-badge-row">
            <h2 class="section-title">Fleet Telemetry & Probe Latency</h2>
            <span class="telemetry-live-badge font-mono">
              <span class="pulse-dot pulse-dot-cyan"></span>
              SYNCHRONIZED SCRUBBING
            </span>
          </div>
          <p class="section-subtitle">Real-time probe round-trip latency and active node connectivity</p>
        </div>
        <div class="sync-legend font-mono text-xs">
          <span class="legend-item"><span class="legend-color legend-cyan"></span> Probe Latency</span>
          <span class="legend-item"><span class="legend-color legend-emerald"></span> Availability</span>
        </div>
      </div>

      <div class="fleet-telemetry-grid">
        <div class="telemetry-chart-card">
          <div class="chart-card-header">
            <span class="chart-card-title font-mono text-cyan font-semibold">Fleet Probe Latency (RTT)</span>
            <span class="chart-card-val font-mono">{{ onlineHostsCount > 0 && avgLatency > 0 ? `${avgLatency}ms avg` : '--' }}</span>
          </div>
          <CanvasTimeSeries
            :series="fleetTelemetryWindow.latency"
            unit="ms"
            :height="130"
            sync-group="fleet-telemetry"
            :min="0"
            :thresholds="latencyThresholds"
          />
        </div>

        <div class="telemetry-chart-card">
          <div class="chart-card-header">
            <span class="chart-card-title font-mono text-emerald font-semibold">Fleet Availability SLA</span>
            <span class="chart-card-val font-mono">{{ onlineHostsCount }}/{{ totalHosts }} online</span>
          </div>
          <CanvasTimeSeries
            :series="fleetTelemetryWindow.availability"
            unit="%"
            :height="130"
            sync-group="fleet-telemetry"
            :min="0"
            :max="100"
            :thresholds="availabilityThresholds"
          />
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.fleet-telemetry-panel { margin-bottom: 16px; }
.hosts-telemetry-hud-strip { margin-bottom: 12px; }
.section-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 8px;
}
.section-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  margin: 0;
}
.section-subtitle {
  font-size: 11px;
  color: #94a3b8;
  margin: 2px 0 0 0;
}
.panel-badge-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.telemetry-live-badge {
  display: inline-flex; align-items: center; gap: 6px; padding: 3px 8px; border-radius: 9999px;
  font-size: 10px; font-weight: 600; background: rgba(6, 182, 212, 0.12); color: #06b6d4;
  border: 1px solid rgba(6, 182, 212, 0.25); letter-spacing: 0.05em;
}
.sync-legend { display: flex; align-items: center; gap: 12px; color: #94a3b8; }
.legend-item { display: inline-flex; align-items: center; gap: 5px; }
.legend-color { width: 8px; height: 8px; border-radius: 2px; }
.legend-cyan { background-color: #06b6d4; }
.legend-emerald { background-color: #10b981; }
.fleet-telemetry-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 14px; margin-top: 14px; }
@media (max-width: 768px) {
  .fleet-telemetry-grid { grid-template-columns: 1fr; gap: 12px; }
}
.telemetry-chart-card {
  background: rgba(15, 23, 42, 0.4); border: 1px solid rgba(56, 189, 248, 0.12); border-radius: 8px; padding: 10px;
}
.chart-card-header {
  display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; font-size: 11px;
}
.chart-card-val { color: #f1f5f9; }
</style>

