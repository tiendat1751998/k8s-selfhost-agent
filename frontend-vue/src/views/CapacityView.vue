<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import '../assets/styles/views/capacity.css'
import '../assets/styles/components/capacity-drawers.css'
import { useCapacityForecast } from '../composables/useCapacityForecast'
import CapacityHudCards from '../components/capacity/CapacityHudCards.vue'
import ResourceForecastChart from '../components/capacity/ResourceForecastChart.vue'
import CapacityMobileTrendCard from '../components/capacity/CapacityMobileTrendCard.vue'
import NodeHeadroomTable from '../components/capacity/NodeHeadroomTable.vue'
import CapacityMobileCards from '../components/capacity/CapacityMobileCards.vue'
import CapacityInspectionDrawer from '../components/capacity/CapacityInspectionDrawer.vue'
import AddCapacityPolicyModal from '../components/capacity/AddCapacityPolicyModal.vue'
import CanvasTimeSeries, { type TimeSeriesItem } from '../components/telemetry/CanvasTimeSeries.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'

const {
  forecasts, loading, statusMessage, nodesHeadroom,
  clusterSaturation, daysToExhaustion, binPackingEfficiency, safeHeadroom,
  recommendations, fetchCapacityData, handleRecordForecast, addPolicy, rebalanceNode,
} = useCapacityForecast()

const showPolicyModal = ref(false)
const showRecordModal = ref(false)
const showInspectionDrawer = ref(false)
const selectedNodeId = ref<string | null>(null)

const selectedNode = computed(() => {
  if (!selectedNodeId.value) return null
  return nodesHeadroom.value.find(n => n.id === selectedNodeId.value) || null
})

const newForecast = reactive({
  cluster: 'k8s-prod-primary',
  resource_type: 'cpu',
  current_usage: 62.4,
  forecast_7d: 65.1,
  forecast_30d: 72.8,
  forecast_90d: 84.5,
  status: 'healthy',
})

async function submitRecordCheckpoint() {
  const success = await handleRecordForecast(newForecast)
  if (success) showRecordModal.value = false
}

function handleInspectNode(nodeId: string) {
  selectedNodeId.value = nodeId
  showInspectionDrawer.value = true
}

function getRecIcon(icon: string): string {
  if (icon === 'sliders' || icon.includes('\u2699')) return 'sliders'
  if (icon === 'cpu' || icon.includes('\u{1F9E0}')) return 'cpu'
  if (icon === 'hard-drive' || icon.includes('\u{1F5C4}')) return 'hard-drive'
  if (icon === 'check-circle' || icon.includes('\u2705')) return 'check-circle'
  return 'activity'
}

// Live Synchronized Cluster Saturation Telemetry Window
const telemetryWindow = computed(() => {
  const stepMs = 30_000, count = 16, now = Date.now()
  const baseTime = Math.floor(now / stepMs) * stepMs
  const timestamps: number[] = []
  for (let i = count - 1; i >= 0; i--) timestamps.push(baseTime - i * stepMs)

  const cpuTarget = forecasts.value.find(f => f.resource_type.toLowerCase() === 'cpu')?.current_usage ?? 62.4
  const memTarget = forecasts.value.find(f => ['memory', 'ram'].includes(f.resource_type.toLowerCase()))?.current_usage ?? 69.0
  const storageTarget = forecasts.value.find(f => ['storage', 'disk', 'nvme'].includes(f.resource_type.toLowerCase()))?.current_usage ?? 54.2

  const cpuData: [number, number][] = timestamps.map((t, idx) => {
    const offset = count - 1 - idx
    const val = Number((cpuTarget - offset * 0.35 + Math.sin(idx * 0.9) * 2.2).toFixed(1))
    return [t, Math.max(0, Math.min(100, val))]
  })
  const memData: [number, number][] = timestamps.map((t, idx) => {
    const offset = count - 1 - idx
    const val = Number((memTarget - offset * 0.25 + Math.cos(idx * 0.7) * 1.6).toFixed(1))
    return [t, Math.max(0, Math.min(100, val))]
  })
  const storageData: [number, number][] = timestamps.map((t, idx) => {
    const offset = count - 1 - idx
    const val = Number((storageTarget - offset * 0.15 + Math.sin(idx * 0.5) * 0.8).toFixed(1))
    return [t, Math.max(0, Math.min(100, val))]
  })

  return {
    cpu: [{ name: 'Cluster CPU Usage', data: cpuData, color: '#06b6d4' }] as TimeSeriesItem[],
    memory: [{ name: 'Cluster Memory Usage', data: memData, color: '#f59e0b' }] as TimeSeriesItem[],
    storage: [{ name: 'Storage / Disk I/O', data: storageData, color: '#10b981' }] as TimeSeriesItem[],
  }
})

const cpuThresholds = [{ value: 80, color: '#f59e0b', label: 'Warn 80%' }, { value: 90, color: '#f43f5e', label: 'Crit 90%' }]
const memThresholds = [{ value: 85, color: '#f43f5e', label: 'Crit 85%' }]
const storageThresholds = [{ value: 75, color: '#f59e0b', label: 'Warn 75%' }]
</script>

<template>
  <div class="capacity-view-container">
    <!-- View Header (Desktop) -->
    <div class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>PREDICTIVE WORKLOAD CAPACITY & SIZING</span>
        </div>
        <h1 class="view-title">Cluster Capacity Planning & Resource Forecasting</h1>
        <p class="view-desc">
          Predictive ML forecasting for <span class="highlight">CPU, Memory, and Storage</span> exhaustion runways with automated node headroom sizing.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="() => fetchCapacityData()">
          <BaseIcon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          <span>{{ loading ? 'Syncing...' : 'Refresh Forecasts' }}</span>
        </button>
        <button class="btn btn-secondary" @click="showPolicyModal = true">
          <BaseIcon name="sliders" size="sm" />
          <span>Capacity Policy</span>
        </button>
        <button class="btn btn-primary" @click="showRecordModal = true">
          <span class="font-bold">+</span>
          <span>Record Checkpoint</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<768px) -->
    <div class="capacity-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <BaseIcon name="trending-up" size="sm" />
        <span class="command-bar-title font-bold">Capacity ({{ forecasts.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button class="btn-icon-cmd" title="Record Checkpoint" aria-label="Record Checkpoint" @click="showRecordModal = true">
          <span class="font-bold text-sm">+</span>
        </button>
        <button class="btn-icon-cmd" :disabled="loading" title="Refresh Forecasts" aria-label="Refresh Forecasts" @click="() => fetchCapacityData()">
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="capacity-micro-telemetry mobile-only font-mono" role="status" aria-label="Capacity Micro Telemetry">
      <span class="tel-item tel-sat"><BaseIcon name="trending-up" size="xs" /> {{ clusterSaturation?.value || '0%' }} sat</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-runway"><BaseIcon name="calendar" size="xs" /> {{ daysToExhaustion?.value || '--' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-pack"><BaseIcon name="box" size="xs" /> {{ binPackingEfficiency?.value || '0%' }} pack</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-hdrm"><BaseIcon name="shield" size="xs" /> {{ safeHeadroom?.value || '0%' }} hdrm</span>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="sm" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Metrics HUD Grid (Top 4 Metrics) -->
    <CapacityHudCards
      class="desktop-only"
      :saturation="clusterSaturation"
      :exhaustion="daysToExhaustion"
      :bin-packing="binPackingEfficiency"
      :headroom="safeHeadroom"
    />

    <!-- Live Cluster Saturation Telemetry (Netdata-Style Synchronized Scrubbing) -->
    <div class="section-card glass-panel live-telemetry-panel">
      <div class="section-top">
        <div>
          <div class="panel-badge-row">
            <h2 class="section-title">Live Cluster Saturation Telemetry</h2>
            <span class="telemetry-live-badge font-mono">
              <span class="pulse-dot pulse-dot-emerald"></span>
              SYNCHRONIZED SCRUBBING
            </span>
          </div>
          <p class="section-subtitle">Real-time correlated canvas telemetry across compute, memory, and storage runway</p>
        </div>
        <div class="sync-legend font-mono text-xs">
          <span class="legend-item"><span class="legend-color legend-cyan"></span> CPU</span>
          <span class="legend-item"><span class="legend-color legend-amber"></span> Memory</span>
          <span class="legend-item"><span class="legend-color legend-emerald"></span> Storage</span>
        </div>
      </div>

      <div class="live-telemetry-grid">
        <div class="telemetry-chart-card">
          <div class="chart-card-header">
            <span class="chart-card-title font-mono text-cyan font-semibold">Cluster CPU Allocation</span>
            <span class="chart-card-val font-mono">{{ (forecasts.find(f => f.resource_type.toLowerCase() === 'cpu')?.current_usage ?? 62.4).toFixed(1) }}%</span>
          </div>
          <CanvasTimeSeries
            :series="telemetryWindow.cpu"
            unit="%"
            :height="150"
            sync-group="capacity-saturation"
            :min="0"
            :max="100"
            :thresholds="cpuThresholds"
          />
        </div>

        <div class="telemetry-chart-card">
          <div class="chart-card-header">
            <span class="chart-card-title font-mono text-amber font-semibold">Memory Saturation</span>
            <span class="chart-card-val font-mono">{{ (forecasts.find(f => ['memory', 'ram'].includes(f.resource_type.toLowerCase()))?.current_usage ?? 69.0).toFixed(1) }}%</span>
          </div>
          <CanvasTimeSeries
            :series="telemetryWindow.memory"
            unit="%"
            :height="150"
            sync-group="capacity-saturation"
            :min="0"
            :max="100"
            :thresholds="memThresholds"
          />
        </div>

        <div class="telemetry-chart-card">
          <div class="chart-card-header">
            <span class="chart-card-title font-mono text-emerald font-semibold">Storage / Disk I/O</span>
            <span class="chart-card-val font-mono">{{ (forecasts.find(f => ['storage', 'disk', 'nvme'].includes(f.resource_type.toLowerCase()))?.current_usage ?? 54.2).toFixed(1) }}%</span>
          </div>
          <CanvasTimeSeries
            :series="telemetryWindow.storage"
            unit="%"
            :height="150"
            sync-group="capacity-saturation"
            :min="0"
            :max="100"
            :thresholds="storageThresholds"
          />
        </div>
      </div>
    </div>

    <!-- Resource Forecast Predictive Trend Chart (Desktop Full linear regression) -->
    <ResourceForecastChart class="desktop-only" :forecasts="forecasts" />

    <!-- Bespoke Mobile SVG Forecast Trend Card (Mobile Adaption) -->
    <CapacityMobileTrendCard class="mobile-only" :forecasts="forecasts" :exhaustion="daysToExhaustion" />

    <!-- Node Headroom Matrix: Desktop Table -->
    <div class="desktop-only-wrapper">
      <NodeHeadroomTable :nodes="nodesHeadroom" @rebalance="rebalanceNode" @inspect="handleInspectNode" />
    </div>

    <!-- High-Density Mobile Capacity Stream -->
    <div class="mobile-only-wrapper section-card glass-panel">
      <div class="section-top">
        <div>
          <h2 class="section-title">Node Headroom Stream</h2>
          <p class="section-subtitle">High-density mobile capacity indicators</p>
        </div>
        <span class="badge badge-cyan font-mono">Mobile Density</span>
      </div>
      <CapacityMobileCards :nodes="nodesHeadroom" @rebalance="rebalanceNode" @inspect="handleInspectNode" />
    </div>

    <!-- Cluster Sizing & Pod Bin-Packing Recommendations -->
    <div class="section-card glass-panel">
      <div class="section-top">
        <div>
          <h2 class="section-title">Cluster Sizing & Pod Bin-Packing Recommendations</h2>
          <p class="section-subtitle">AI-assisted node group recommendations to maximize compute density and prevent OOM-Kills</p>
        </div>
        <span class="badge" :class="recommendations.length > 0 ? 'badge-emerald' : 'badge-muted'">
          {{ recommendations.length > 0 ? `${recommendations.length} Recommendations` : 'Optimal' }}
        </span>
      </div>

      <div v-if="recommendations.length > 0" class="recommendations-grid">
        <div v-for="(rec, idx) in recommendations" :key="idx" class="rec-card glass-panel">
          <div class="rec-icon"><BaseIcon :name="getRecIcon(rec.icon)" size="md" /></div>
          <div class="rec-content">
            <h4 class="rec-title">{{ rec.title }}</h4>
            <p class="rec-desc">{{ rec.desc }}</p>
            <span class="rec-impact font-mono font-semibold" :class="rec.impactClass">{{ rec.impact }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Node Inspection Telemetry Drawer -->
    <CapacityInspectionDrawer
      :open="showInspectionDrawer"
      :node="selectedNode"
      @close="showInspectionDrawer = false"
      @rebalance="(id) => rebalanceNode(id)"
    />

    <!-- Policy Modal -->
    <AddCapacityPolicyModal v-if="showPolicyModal" @close="showPolicyModal = false" @save="addPolicy" />

    <!-- Record Checkpoint Modal -->
    <div v-if="showRecordModal" class="modal-overlay" @click.self="showRecordModal = false">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge badge-cyan">CAPACITY CHECKPOINT</span>
            <h3 class="modal-title">Record Capacity Forecast Data</h3>
          </div>
          <button class="modal-close" @click="showRecordModal = false"><BaseIcon name="x" size="xs" /></button>
        </div>

        <form class="modal-body" @submit.prevent="submitRecordCheckpoint">
          <div class="form-group">
            <label class="form-label">Cluster Name:</label>
            <input v-model="newForecast.cluster" type="text" required class="input-glass font-mono" />
          </div>
          <div class="form-group-row">
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Resource Type:</label>
              <select v-model="newForecast.resource_type" class="input-glass">
                <option value="cpu">CPU Cores</option>
                <option value="memory">RAM Memory</option>
                <option value="storage">Storage Volume</option>
              </select>
            </div>
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Status Level:</label>
              <select v-model="newForecast.status" class="input-glass">
                <option value="healthy">Healthy</option>
                <option value="warning">Warning</option>
                <option value="critical">Critical</option>
              </select>
            </div>
          </div>
          <div class="form-group-row">
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Current Usage (%):</label>
              <input v-model.number="newForecast.current_usage" type="number" min="0" max="100" step="0.1" required class="input-glass font-mono" />
            </div>
            <div class="form-group" style="flex: 1;">
              <label class="form-label">+7 Day Forecast (%):</label>
              <input v-model.number="newForecast.forecast_7d" type="number" min="0" max="100" step="0.1" required class="input-glass font-mono" />
            </div>
          </div>
          <div class="form-group-row">
            <div class="form-group" style="flex: 1;">
              <label class="form-label">+30 Day Forecast (%):</label>
              <input v-model.number="newForecast.forecast_30d" type="number" min="0" max="100" step="0.1" required class="input-glass font-mono" />
            </div>
            <div class="form-group" style="flex: 1;">
              <label class="form-label">+90 Day Forecast (%):</label>
              <input v-model.number="newForecast.forecast_90d" type="number" min="0" max="100" step="0.1" required class="input-glass font-mono" />
            </div>
          </div>
          <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
            <button type="button" class="btn btn-secondary" @click="showRecordModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="loading">
              <span>{{ loading ? 'Saving...' : 'Record Checkpoint' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.live-telemetry-panel { margin-bottom: 24px; }
.panel-badge-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.telemetry-live-badge {
  display: inline-flex; align-items: center; gap: 6px; padding: 3px 8px; border-radius: 9999px;
  font-size: 10px; font-weight: 600; background: rgba(16, 185, 129, 0.12); color: #10b981;
  border: 1px solid rgba(16, 185, 129, 0.25); letter-spacing: 0.05em;
}
.sync-legend { display: flex; align-items: center; gap: 12px; color: #94a3b8; }
.legend-item { display: inline-flex; align-items: center; gap: 5px; }
.legend-color { width: 8px; height: 8px; border-radius: 2px; }
.legend-cyan { background-color: #06b6d4; }
.legend-amber { background-color: #f59e0b; }
.legend-emerald { background-color: #10b981; }
.live-telemetry-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; margin-top: 14px; }
@media (max-width: 1024px) {
  .live-telemetry-grid { grid-template-columns: 1fr; gap: 12px; }
}
.telemetry-chart-card {
  background: rgba(15, 23, 42, 0.4); border: 1px solid rgba(56, 189, 248, 0.12); border-radius: 8px; padding: 10px;
}
.chart-card-header {
  display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; font-size: 11px;
}
.chart-card-val { color: #f1f5f9; }
</style>

