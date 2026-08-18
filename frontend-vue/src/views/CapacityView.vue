<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>PREDICTIVE WORKLOAD CAPACITY & SIZING</span>
        </div>
        <h1 class="view-title">Cluster Capacity Planning & Resource Forecasting</h1>
        <p class="view-desc">
          Predictive machine learning forecasting for <span class="highlight">CPU, Memory, and NVMe Storage</span> exhaustion runways with automated node pool sizing recommendations.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchCapacityData">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Forecasts' }}</span>
        </button>
        <button class="btn btn-primary" @click="showRecordModal = true">
          <span>+ Record Forecast Checkpoint</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Metrics HUD Grid -->
    <div class="metrics-grid">
      <MetricCard
        title="CPU Capacity Runway"
        value="> 180 Days"
        trend="+12% Headroom"
        trend-type="positive"
        badge="HEALTHY"
        badge-color="emerald"
        subtitle="Current peak compute utilization: 64%"
        icon="⚡"
      />
      <MetricCard
        title="RAM Saturation Forecast"
        value="78 Days"
        trend="Growth Trend +4.2%/mo"
        trend-type="neutral"
        badge="MONITOR"
        badge-color="amber"
        subtitle="Estimated saturation by Q4"
        icon="🧠"
      />
      <MetricCard
        title="Storage Volume Headroom"
        value="4.8 TB"
        trend="72% Available"
        trend-type="positive"
        badge="NVMe OPTIMAL"
        badge-color="cyan"
        subtitle="Total 6.8 TB Provisioned PVCs"
        icon="💾"
      />
      <MetricCard
        title="Scaling Action"
        value="+1 Node Req"
        badge="RECOMMENDED"
        badge-color="violet"
        subtitle="Pre-scale worker pool before Black Friday"
        icon="📈"
      />
    </div>

    <!-- Forecast Progression Cards Grid -->
    <div class="section-card glass-panel">
      <div class="section-top">
        <div>
          <h2 class="section-title">Resource Projection Checkpoints (7d / 30d / 90d)</h2>
          <p class="section-subtitle">Predictive trajectory of hardware resources based on rolling 30-day telemetry trends</p>
        </div>
      </div>

      <div v-if="forecasts.length > 0" class="forecasts-grid">
        <div v-for="fc in forecasts" :key="fc.id" class="forecast-card glass-panel glass-panel-glow">
          <div class="forecast-top">
            <div class="resource-icon-box">{{ getResourceIcon(fc.resource_type) }}</div>
            <div class="forecast-meta">
              <h3 class="resource-title">{{ fc.resource_type.toUpperCase() }} ALLOCATION</h3>
              <span class="cluster-tag font-mono text-muted">Cluster: {{ fc.cluster || 'k8s-prod-primary' }}</span>
            </div>
            <StatusBadge :status="fc.status" :label="fc.status.toUpperCase()" size="sm" />
          </div>

          <!-- Current Gauge Bar -->
          <div class="forecast-current">
            <div class="gauge-label-row">
              <span class="gauge-k font-mono">Current Usage:</span>
              <span class="gauge-v font-mono font-bold" :class="getUsageColorText(fc.current_usage)">{{ fc.current_usage.toFixed(1) }}%</span>
            </div>
            <div class="gauge-bar-bg">
              <div 
                class="gauge-bar-fill" 
                :style="{ width: `${Math.min(100, fc.current_usage)}%` }"
                :class="getUsageColorBg(fc.current_usage)"
              ></div>
            </div>
          </div>

          <!-- 7d / 30d / 90d Projection Matrix -->
          <div class="trajectory-matrix font-mono">
            <div class="matrix-item">
              <span class="matrix-label">+7 Days</span>
              <span class="matrix-val">{{ fc.forecast_7d.toFixed(1) }}%</span>
            </div>
            <div class="matrix-item">
              <span class="matrix-label">+30 Days</span>
              <span class="matrix-val text-cyan">{{ fc.forecast_30d.toFixed(1) }}%</span>
            </div>
            <div class="matrix-item">
              <span class="matrix-label">+90 Days</span>
              <span class="matrix-val" :class="fc.forecast_90d >= 85 ? 'text-rose font-bold' : 'text-amber'">
                {{ fc.forecast_90d.toFixed(1) }}%
              </span>
            </div>
          </div>

          <div class="forecast-footer font-mono text-muted">
            <span>Exhaustion: <strong class="text-primary">{{ fc.exhaustion_at ? formatDate(fc.exhaustion_at) : '> 12 Months' }}</strong></span>
            <span>Recorded: {{ formatDate(fc.recorded_at) }}</span>
          </div>
        </div>
      </div>

      <div v-else-if="!loading" class="empty-state-box glass-panel">
        <span class="empty-icon">📈</span>
        <h3 class="empty-title">No Predictive Forecast Checkpoints</h3>
        <p class="empty-desc">No machine learning capacity models or historical checkpoints recorded yet. Record a new checkpoint or wait for the telemetry daemon to compute trends.</p>
        <button class="btn btn-primary btn-sm" @click="showRecordModal = true">
          <span>+ Record Forecast Checkpoint</span>
        </button>
      </div>
    </div>

    <!-- Cluster Sizing & Scaling Recommendations -->
    <div class="section-card glass-panel">
      <div class="section-top">
        <div>
          <h2 class="section-title">Cluster Sizing & Pod Bin-Packing Recommendations</h2>
          <p class="section-subtitle">AI-assisted node group recommendations to maximize compute density and prevent OOM-Kills</p>
        </div>
        <span class="badge badge-emerald">Optimal Recommendations</span>
      </div>

      <div class="recommendations-grid">
        <div class="rec-card glass-panel">
          <div class="rec-icon">⚙️</div>
          <div class="rec-content">
            <h4 class="rec-title">Horizontal Node Pool Auto-Scaling</h4>
            <p class="rec-desc">
              Adjust minimum worker node count from <strong>3 → 4 nodes</strong> in node-pool-prod to accommodate the 30-day forecast increase.
            </p>
            <span class="rec-impact text-cyan font-mono font-semibold">Prevents pod scheduling throttles during peak traffic</span>
          </div>
        </div>

        <div class="rec-card glass-panel">
          <div class="rec-icon">📦</div>
          <div class="rec-content">
            <h4 class="rec-title">Pod Request Right-Sizing</h4>
            <p class="rec-desc">
              Lower memory requests on <strong>staging-sandbox</strong> namespaces from 2.0GiB to 512MiB based on 90-day historical p99 utilization.
            </p>
            <span class="rec-impact text-emerald font-mono font-semibold">Frees up 6.0 GiB assignable memory capacity</span>
          </div>
        </div>

        <div class="rec-card glass-panel">
          <div class="rec-icon">🗄️</div>
          <div class="rec-content">
            <h4 class="rec-title">Storage Compaction & PVC Pruning</h4>
            <p class="rec-desc">
              Enable automated ZFS/NVMe volume snapshots deduplication to extend NVMe drive exhaustion past 24 months.
            </p>
            <span class="rec-impact text-violet font-mono font-semibold">Recovers approximately 850 GB unreferenced space</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Record Capacity Forecast -->
    <div v-if="showRecordModal" class="modal-overlay" @click.self="showRecordModal = false">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge badge-cyan">CAPACITY CHECKPOINT</span>
            <h3 class="modal-title">Record Capacity Forecast Data</h3>
          </div>
          <button class="modal-close" @click="showRecordModal = false">✕</button>
        </div>

        <form class="modal-body" @submit.prevent="handleRecordForecast">
          <div class="form-group">
            <label class="form-label">Cluster Name:</label>
            <input v-model="newForecast.cluster" type="text" required class="input-glass" placeholder="k8s-prod-primary" />
          </div>

          <div class="form-group-row">
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Resource Type:</label>
              <select v-model="newForecast.resource_type" class="input-glass">
                <option value="cpu">CPU Cores</option>
                <option value="memory">RAM Memory</option>
                <option value="storage">NVMe Storage</option>
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
              <span>{{ loading ? 'Saving...' : 'Record Forecast' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import {
  capacityApi,
  type CapacityForecast,
} from '../api/governance'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'

const forecasts = ref<CapacityForecast[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)
const showRecordModal = ref(false)

const newForecast = reactive({
  cluster: 'k8s-prod-primary',
  resource_type: 'cpu',
  current_usage: 62.4,
  forecast_7d: 65.1,
  forecast_30d: 72.8,
  forecast_90d: 84.5,
  status: 'healthy',
})

onMounted(() => {
  fetchCapacityData()
})

async function fetchCapacityData() {
  loading.value = true
  error.value = null
  try {
    const data = await capacityApi.getForecasts()
    forecasts.value = data || []
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load capacity forecasts'
    error.value = msg
    forecasts.value = []
  } finally {
    loading.value = false
  }
}

async function handleRecordForecast() {
  loading.value = true
  statusMessage.value = null
  try {
    await capacityApi.recordForecast(newForecast)
    statusMessage.value = {
      type: 'success',
      text: `Forecast checkpoint for ${newForecast.resource_type.toUpperCase()} recorded successfully.`,
    }
    showRecordModal.value = false
    await fetchCapacityData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to record forecast'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    loading.value = false
  }
}

function getResourceIcon(type: string): string {
  const t = (type || '').toLowerCase()
  if (t.includes('cpu')) return '⚡'
  if (t.includes('mem')) return '🧠'
  if (t.includes('stor')) return '💾'
  return '📦'
}

function getUsageColorText(val: number): string {
  if (val >= 85) return 'text-rose'
  if (val >= 70) return 'text-amber'
  return 'text-emerald'
}

function getUsageColorBg(val: number): string {
  if (val >= 85) return 'bg-rose'
  if (val >= 70) return 'bg-amber'
  return 'bg-emerald'
}

function formatDate(d: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString()
  } catch {
    return d
  }
}
</script>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 16px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 820px;
  margin-top: 4px;
}

.highlight {
  color: #fff;
  font-weight: 600;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.status-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 18px;
  border-radius: 12px;
  font-size: 13px;
}

.banner-success {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.banner-error {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fda4af;
}

.banner-icon {
  font-size: 16px;
}

.banner-text {
  flex: 1;
  font-weight: 500;
}

.banner-close {
  background: none;
  border: none;
  color: inherit;
  font-size: 14px;
  cursor: pointer;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.section-card {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 20px;
}

.section-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 12px;
}

.section-title {
  font-size: 16px;
  color: #fff;
}

.section-subtitle {
  font-size: 12px;
  color: var(--text-muted);
}

.forecasts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.forecast-card {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.forecast-top {
  display: flex;
  align-items: center;
  gap: 12px;
}

.resource-icon-box {
  width: 42px;
  height: 42px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.forecast-meta {
  flex: 1;
  min-width: 0;
}

.resource-title {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.cluster-tag {
  font-size: 10px;
}

.forecast-current {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.gauge-label-row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}

.gauge-bar-bg {
  width: 100%;
  height: 8px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
}

.gauge-bar-fill {
  height: 100%;
  border-radius: 9999px;
  transition: width 0.3s ease;
}

.bg-emerald { background: var(--accent-emerald); }
.bg-amber { background: var(--accent-amber); }
.bg-rose { background: var(--accent-rose); }
.text-emerald { color: #34d399; }
.text-amber { color: #fbbf24; }
.text-rose { color: #fb7185; }

.trajectory-matrix {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 8px;
  background: rgba(0, 0, 0, 0.25);
  padding: 10px;
  border-radius: 8px;
  text-align: center;
}

.matrix-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.matrix-label {
  font-size: 10px;
  color: var(--text-muted);
}

.matrix-val {
  font-size: 13px;
  font-weight: 700;
}

.forecast-footer {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  padding-top: 8px;
  border-top: 1px solid var(--border-subtle);
}

.recommendations-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 14px;
}

.rec-card {
  padding: 16px;
  display: flex;
  gap: 14px;
  align-items: flex-start;
}

.rec-icon {
  font-size: 24px;
}

.rec-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.rec-title {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.rec-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}

.rec-impact {
  font-size: 11px;
  margin-top: 4px;
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 20px;
}

.modal-card {
  width: 100%;
  max-width: 560px;
  background: var(--bg-sidebar);
  border: 1px solid var(--border-medium);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.7);
}

.modal-header {
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  border-bottom: 1px solid var(--border-subtle);
}

.modal-title-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.modal-title {
  font-size: 18px;
  color: #fff;
}

.modal-close {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 18px;
  cursor: pointer;
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group-row {
  display: flex;
  gap: 12px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.modal-footer {
  padding: 16px 20px;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  border-top: 1px solid var(--border-subtle);
}

.empty-state-box {
  padding: 36px 20px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.empty-icon {
  font-size: 32px;
}

.empty-title {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
}

.empty-desc {
  font-size: 12px;
  color: var(--text-muted);
  max-width: 480px;
  margin-bottom: 6px;
}
</style>
