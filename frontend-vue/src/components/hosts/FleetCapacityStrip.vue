<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import type { FleetHardwareAggregates } from '../../types/hosts'

defineProps<{
  aggregates: FleetHardwareAggregates
  loading?: boolean
}>()

function getSaturationClass(pct: number): string {
  if (pct >= 85) return 'saturation-critical'
  if (pct >= 70) return 'saturation-warning'
  return 'saturation-healthy'
}
</script>

<template>
  <div class="fleet-capacity-strip glass-panel font-mono" role="region" aria-label="Fleet Hardware Capacity">
    <!-- 1. Connected Physical Blades -->
    <div class="capacity-metric-card">
      <div class="metric-icon-wrap icon-blades">
        <BaseIcon name="server" size="sm" />
      </div>
      <div class="metric-info">
        <div class="metric-top-row">
          <span class="metric-label">PHYSICAL BLADES</span>
          <span class="metric-tag status-pill-connected">
            <span class="pill-dot emerald"></span>
            CONNECTED
          </span>
        </div>
        <div class="metric-primary-val">
          <span class="val-number">{{ aggregates.onlineBladeCount }}</span>
          <span class="val-sub">/ {{ aggregates.totalBladeCount }}</span>
        </div>
        <div class="metric-subtext">Active rack servers</div>
      </div>
    </div>

    <!-- 2. Fleet CPU Load -->
    <div class="capacity-metric-card">
      <div class="metric-icon-wrap icon-cpu">
        <BaseIcon name="cpu" size="sm" />
      </div>
      <div class="metric-info">
        <div class="metric-top-row">
          <span class="metric-label">FLEET CPU LOAD</span>
          <span class="metric-pct" :class="getSaturationClass(aggregates.fleetCpuAvgPct)">
            {{ aggregates.fleetCpuAvgPct.toFixed(1) }}%
          </span>
        </div>
        <div class="micro-meter-track">
          <div
            class="micro-meter-fill"
            :class="getSaturationClass(aggregates.fleetCpuAvgPct)"
            :style="{ width: `${Math.min(100, Math.max(0, aggregates.fleetCpuAvgPct))}%` }"
          ></div>
        </div>
        <div class="metric-subtext">Cluster compute average</div>
      </div>
    </div>

    <!-- 3. Fleet Memory Allocation -->
    <div class="capacity-metric-card">
      <div class="metric-icon-wrap icon-mem">
        <BaseIcon name="activity" size="sm" />
      </div>
      <div class="metric-info">
        <div class="metric-top-row">
          <span class="metric-label">RAM ALLOCATION</span>
          <span class="metric-pct" :class="getSaturationClass(aggregates.fleetMemPct)">
            {{ aggregates.fleetMemPct.toFixed(1) }}%
          </span>
        </div>
        <div class="micro-meter-track">
          <div
            class="micro-meter-fill"
            :class="getSaturationClass(aggregates.fleetMemPct)"
            :style="{ width: `${Math.min(100, Math.max(0, aggregates.fleetMemPct))}%` }"
          ></div>
        </div>
        <div class="metric-subtext">
          {{ aggregates.fleetMemUsedGiB }} / {{ aggregates.fleetMemTotalGiB }} GiB
        </div>
      </div>
    </div>

    <!-- 4. Fleet Storage Capacity -->
    <div class="capacity-metric-card">
      <div class="metric-icon-wrap icon-disk">
        <BaseIcon name="database" size="sm" />
      </div>
      <div class="metric-info">
        <div class="metric-top-row">
          <span class="metric-label">STORAGE CAPACITY</span>
          <span class="metric-pct" :class="getSaturationClass(aggregates.fleetDiskPct)">
            {{ aggregates.fleetDiskPct.toFixed(1) }}%
          </span>
        </div>
        <div class="micro-meter-track">
          <div
            class="micro-meter-fill"
            :class="getSaturationClass(aggregates.fleetDiskPct)"
            :style="{ width: `${Math.min(100, Math.max(0, aggregates.fleetDiskPct))}%` }"
          ></div>
        </div>
        <div class="metric-subtext">
          {{ aggregates.fleetDiskUsedGiB }} / {{ aggregates.fleetDiskTotalGiB }} GiB
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fleet-capacity-strip {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  padding: 12px 14px;
  background: var(--bg-card);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--rounded-lg, 8px);
  box-sizing: border-box;
  width: 100%;
}

.capacity-metric-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 10px;
  background: var(--color-surface-inset);
  border: 1px solid var(--color-hairline);
  border-radius: var(--rounded-md, 6px);
  min-width: 0;
}

.metric-icon-wrap {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background: var(--color-surface-card);
  border: 1px solid var(--color-border-subtle);
  color: var(--text-primary);
}

.icon-blades { color: var(--color-primary); }
.icon-cpu { color: var(--color-badge-agent-text); }
.icon-mem { color: var(--color-badge-metrics-text); }
.icon-disk { color: var(--color-warning); }

.metric-info {
  display: flex;
  flex-direction: column;
  gap: 3px;
  flex: 1 1 auto;
  min-width: 0;
}

.metric-top-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.metric-label {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  white-space: nowrap;
}

.metric-tag {
  font-size: 9.5px;
  padding: 1px 6px;
  border-radius: 4px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-weight: 600;
  line-height: 1.2;
}

.status-pill-connected {
  background: var(--color-status-online-bg);
  border: 1px solid var(--color-status-online-border);
  color: var(--color-status-online);
}

.pill-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
}
.pill-dot.emerald { background: var(--color-status-online); }

.metric-primary-val {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.val-number {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1.1;
}

.val-sub {
  font-size: 12px;
  color: var(--text-muted);
}

.metric-pct {
  font-size: 11px;
  font-weight: 700;
}

.micro-meter-track {
  width: 100%;
  height: 5px;
  background: var(--color-surface-card);
  border-radius: 999px;
  overflow: hidden;
  margin-top: 3px;
  margin-bottom: 2px;
  border: 1px solid var(--color-border-subtle);
}

.micro-meter-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.3s ease;
}

.saturation-healthy {
  background: var(--color-status-online);
  color: var(--color-status-online);
}
.saturation-warning {
  background: var(--color-status-warning);
  color: var(--color-status-warning);
}
.saturation-critical {
  background: var(--color-status-offline);
  color: var(--color-status-offline);
}

.metric-subtext {
  font-size: 10.5px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 1024px) {
  .fleet-capacity-strip {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .fleet-capacity-strip {
    grid-template-columns: 1fr;
    padding: 10px;
    gap: 8px;
  }
}
</style>
