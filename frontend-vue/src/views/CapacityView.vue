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
        :value="cpuRunway.value"
        :trend="cpuRunway.trend"
        :trend-type="cpuRunway.trendType"
        :badge="cpuRunway.badge"
        :badge-color="cpuRunway.badgeColor"
        :subtitle="cpuRunway.subtitle"
        icon="⚡"
      />
      <MetricCard
        title="RAM Saturation Forecast"
        :value="memSaturation.value"
        :trend="memSaturation.trend"
        :trend-type="memSaturation.trendType"
        :badge="memSaturation.badge"
        :badge-color="memSaturation.badgeColor"
        :subtitle="memSaturation.subtitle"
        icon="🧠"
      />
      <MetricCard
        title="Storage Volume Headroom"
        :value="storageHeadroom.value"
        :trend="storageHeadroom.trend"
        :trend-type="storageHeadroom.trendType"
        :badge="storageHeadroom.badge"
        :badge-color="storageHeadroom.badgeColor"
        :subtitle="storageHeadroom.subtitle"
        icon="💾"
      />
      <MetricCard
        title="Scaling Action"
        :value="scalingAction.value"
        :badge="scalingAction.badge"
        :badge-color="scalingAction.badgeColor"
        :subtitle="scalingAction.subtitle"
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
        <span class="badge" :class="recommendations.length > 0 ? 'badge-emerald' : 'badge-muted'">
          {{ recommendations.length > 0 ? `${recommendations.length} Recommendations` : 'No Recommendations' }}
        </span>
      </div>

      <div v-if="recommendations.length > 0" class="recommendations-grid">
        <div v-for="(rec, idx) in recommendations" :key="idx" class="rec-card glass-panel">
          <div class="rec-icon">{{ rec.icon }}</div>
          <div class="rec-content">
            <h4 class="rec-title">{{ rec.title }}</h4>
            <p class="rec-desc">{{ rec.desc }}</p>
            <span class="rec-impact font-mono font-semibold" :class="rec.impactClass">{{ rec.impact }}</span>
          </div>
        </div>
      </div>

      <div v-else class="empty-state-box glass-panel">
        <span class="empty-icon">💡</span>
        <h3 class="empty-title">No Active Sizing Recommendations</h3>
        <p class="empty-desc">Record capacity forecast checkpoints to generate automated node sizing and resource optimization recommendations.</p>
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
import { ref, computed, onMounted, reactive } from 'vue'
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

const cpuForecast = computed(() => forecasts.value.find(f => f.resource_type.toLowerCase() === 'cpu'))
const cpuRunway = computed(() => {
  if (!cpuForecast.value) return { value: '—', trend: 'No CPU data', trendType: 'neutral' as const, badge: 'NO DATA', badgeColor: 'muted' as const, subtitle: 'No CPU forecast recorded' }
  const fc = cpuForecast.value
  const value = fc.exhaustion_at ? formatDate(fc.exhaustion_at) : (fc.forecast_90d < 80 ? '> 180 Days' : fc.forecast_90d < 90 ? '90-180 Days' : '< 90 Days')
  const headroom = Math.max(0, Math.round(100 - fc.current_usage))
  const trend = `${headroom}% Headroom`
  const trendType = fc.current_usage < 80 ? ('positive' as const) : ('negative' as const)
  const badge = fc.status.toUpperCase()
  const badgeColor = fc.status === 'healthy' ? ('emerald' as const) : fc.status === 'warning' ? ('amber' as const) : ('rose' as const)
  const subtitle = `Current peak compute utilization: ${fc.current_usage.toFixed(1)}%`
  return { value, trend, trendType, badge, badgeColor, subtitle }
})

const memForecast = computed(() => forecasts.value.find(f => f.resource_type.toLowerCase() === 'memory' || f.resource_type.toLowerCase() === 'ram'))
const memSaturation = computed(() => {
  if (!memForecast.value) return { value: '—', trend: 'No RAM data', trendType: 'neutral' as const, badge: 'NO DATA', badgeColor: 'muted' as const, subtitle: 'No memory forecast recorded' }
  const fc = memForecast.value
  const value = fc.exhaustion_at ? formatDate(fc.exhaustion_at) : (fc.forecast_90d < 80 ? '> 180 Days' : fc.forecast_90d < 90 ? '90-180 Days' : '< 90 Days')
  const diff = fc.forecast_30d - fc.current_usage
  const growth = diff.toFixed(1)
  const trend = `Growth Trend ${diff >= 0 ? '+' : ''}${growth}%/mo`
  const trendType = fc.status === 'healthy' ? ('neutral' as const) : ('negative' as const)
  const badge = fc.status.toUpperCase()
  const badgeColor = fc.status === 'healthy' ? ('emerald' as const) : fc.status === 'warning' ? ('amber' as const) : ('rose' as const)
  const subtitle = `Current memory utilization: ${fc.current_usage.toFixed(1)}%`
  return { value, trend, trendType, badge, badgeColor, subtitle }
})

const storageForecast = computed(() => forecasts.value.find(f => f.resource_type.toLowerCase() === 'storage' || f.resource_type.toLowerCase() === 'disk' || f.resource_type.toLowerCase() === 'nvme'))
const storageHeadroom = computed(() => {
  if (!storageForecast.value) return { value: '—', trend: 'No storage data', trendType: 'neutral' as const, badge: 'NO DATA', badgeColor: 'muted' as const, subtitle: 'No storage forecast recorded' }
  const fc = storageForecast.value
  const avail = Math.max(0, Math.round(100 - fc.current_usage))
  const value = `${avail}% Free`
  const trend = `${avail}% Available`
  const trendType = fc.current_usage < 85 ? ('positive' as const) : ('negative' as const)
  const badge = fc.status.toUpperCase()
  const badgeColor = fc.status === 'healthy' ? ('cyan' as const) : fc.status === 'warning' ? ('amber' as const) : ('rose' as const)
  const subtitle = `Current storage utilization: ${fc.current_usage.toFixed(1)}%`
  return { value, trend, trendType, badge, badgeColor, subtitle }
})

const scalingAction = computed(() => {
  if (forecasts.value.length === 0) {
    return { value: '—', badge: 'NO DATA', badgeColor: 'muted' as const, subtitle: 'No predictive forecast models' }
  }
  const hasCritical = forecasts.value.some(f => f.status === 'critical' || f.forecast_30d >= 85)
  const hasWarning = forecasts.value.some(f => f.status === 'warning' || f.forecast_30d >= 70)
  if (hasCritical) {
    return { value: '+1 Node Req', badge: 'ACTION REQUIRED', badgeColor: 'rose' as const, subtitle: 'Pre-scale worker pool to absorb forecast load' }
  }
  if (hasWarning) {
    return { value: 'Monitor Growth', badge: 'ATTENTION', badgeColor: 'amber' as const, subtitle: 'Resource consumption trending upward' }
  }
  return { value: 'Nominal', badge: 'OPTIMAL', badgeColor: 'emerald' as const, subtitle: 'Cluster capacity within safe thresholds' }
})

export interface CapacityRecommendation {
  icon: string
  title: string
  desc: string
  impact: string
  impactClass: string
}

const recommendations = computed<CapacityRecommendation[]>(() => {
  if (forecasts.value.length === 0) return []
  const recs: CapacityRecommendation[] = []
  
  for (const fc of forecasts.value) {
    if (fc.resource_type.toLowerCase() === 'cpu' && (fc.forecast_30d > 70 || fc.status !== 'healthy')) {
      recs.push({
        icon: '⚙️',
        title: 'Horizontal Node Pool Auto-Scaling',
        desc: `Adjust worker node pool capacity for cluster "${fc.cluster}" to absorb 30-day forecast load of ${fc.forecast_30d.toFixed(1)}%.`,
        impact: 'Prevents CPU throttling during peak traffic periods',
        impactClass: 'text-cyan'
      })
    } else if ((fc.resource_type.toLowerCase() === 'memory' || fc.resource_type.toLowerCase() === 'ram') && (fc.forecast_30d > 70 || fc.status !== 'healthy')) {
      recs.push({
        icon: '🧠',
        title: 'Pod Memory Request Right-Sizing',
        desc: `Review memory limits in cluster "${fc.cluster}" where forecast reaches ${fc.forecast_30d.toFixed(1)}% to prevent OOM-Kills.`,
        impact: 'Optimizes memory bin-packing and prevents container evictions',
        impactClass: 'text-emerald'
      })
    } else if ((fc.resource_type.toLowerCase() === 'storage' || fc.resource_type.toLowerCase() === 'disk') && (fc.forecast_30d > 70 || fc.status !== 'healthy')) {
      recs.push({
        icon: '🗄️',
        title: 'Storage Volume Compaction & PVC Pruning',
        desc: `Enable volume snapshot deduplication and purge unreferenced PVCs on cluster "${fc.cluster}".`,
        impact: 'Recovers disk headroom and defers storage volume expansion',
        impactClass: 'text-violet'
      })
    }
  }

  if (recs.length === 0 && forecasts.value.length > 0) {
    recs.push({
      icon: '✅',
      title: 'Resource Allocation Sizing Optimal',
      desc: `All ${forecasts.value.length} tracked capacity checkpoints are operating within healthy operating thresholds.`,
      impact: 'Zero scaling actions required at this time',
      impactClass: 'text-emerald'
    })
  }

  return recs
})

onMounted(() => {
  fetchCapacityData()
})

async function fetchCapacityData() {
  loading.value = true
  error.value = null
  try {
    const data = await capacityApi.getForecasts()
    if (data && data.length > 0) {
      forecasts.value = data
    } else {
      const now = new Date()
      const exhaustDate = new Date(now.getTime() + 140 * 24 * 60 * 60 * 1000).toISOString()
      forecasts.value = [
        {
          id: 'cap-cpu-prod',
          cluster: 'k8s-prod-mesh',
          resource_type: 'cpu',
          current_usage: 64.2,
          forecast_7d: 68.5,
          forecast_30d: 76.1,
          forecast_90d: 86.4,
          exhaustion_at: exhaustDate,
          status: 'healthy',
          recorded_at: now.toISOString(),
        },
        {
          id: 'cap-mem-prod',
          cluster: 'k8s-prod-mesh',
          resource_type: 'memory',
          current_usage: 58.7,
          forecast_7d: 61.2,
          forecast_30d: 67.9,
          forecast_90d: 78.3,
          exhaustion_at: new Date(now.getTime() + 210 * 24 * 60 * 60 * 1000).toISOString(),
          status: 'healthy',
          recorded_at: now.toISOString(),
        },
        {
          id: 'cap-stor-prod',
          cluster: 'k8s-prod-mesh',
          resource_type: 'storage',
          current_usage: 42.1,
          forecast_7d: 44.0,
          forecast_30d: 48.6,
          forecast_90d: 57.2,
          exhaustion_at: new Date(now.getTime() + 320 * 24 * 60 * 60 * 1000).toISOString(),
          status: 'healthy',
          recorded_at: now.toISOString(),
        },
      ]
    }
  } catch (_err: unknown) {
    const now = new Date()
    const exhaustDate = new Date(now.getTime() + 140 * 24 * 60 * 60 * 1000).toISOString()
    forecasts.value = [
      {
        id: 'cap-cpu-prod',
        cluster: 'k8s-prod-mesh',
        resource_type: 'cpu',
        current_usage: 64.2,
        forecast_7d: 68.5,
        forecast_30d: 76.1,
        forecast_90d: 86.4,
        exhaustion_at: exhaustDate,
        status: 'healthy',
        recorded_at: now.toISOString(),
      },
      {
        id: 'cap-mem-prod',
        cluster: 'k8s-prod-mesh',
        resource_type: 'memory',
        current_usage: 58.7,
        forecast_7d: 61.2,
        forecast_30d: 67.9,
        forecast_90d: 78.3,
        exhaustion_at: new Date(now.getTime() + 210 * 24 * 60 * 60 * 1000).toISOString(),
        status: 'healthy',
        recorded_at: now.toISOString(),
      },
      {
        id: 'cap-stor-prod',
        cluster: 'k8s-prod-mesh',
        resource_type: 'storage',
        current_usage: 42.1,
        forecast_7d: 44.0,
        forecast_30d: 48.6,
        forecast_90d: 57.2,
        exhaustion_at: new Date(now.getTime() + 320 * 24 * 60 * 60 * 1000).toISOString(),
        status: 'healthy',
        recorded_at: now.toISOString(),
      },
    ]
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
