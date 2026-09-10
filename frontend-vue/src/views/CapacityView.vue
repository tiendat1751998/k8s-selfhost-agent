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

const {
  forecasts,
  loading,
  statusMessage,
  nodesHeadroom,
  clusterSaturation,
  daysToExhaustion,
  binPackingEfficiency,
  safeHeadroom,
  recommendations,
  fetchCapacityData,
  handleRecordForecast,
  addPolicy,
  rebalanceNode,
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

function handleRebalanceFromDrawer(nodeId: string) {
  rebalanceNode(nodeId)
}
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
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Forecasts' }}</span>
        </button>
        <button class="btn btn-secondary" @click="showPolicyModal = true">
          <span>⚙️ Capacity Policy</span>
        </button>
        <button class="btn btn-primary" @click="showRecordModal = true">
          <span>+ Record Checkpoint</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<768px) -->
    <div class="capacity-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">📈 Capacity ({{ forecasts.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Record Checkpoint"
          aria-label="Record Checkpoint"
          @click="showRecordModal = true"
        >
          <span>➕</span>
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh Forecasts"
          aria-label="Refresh Forecasts"
          @click="() => fetchCapacityData()"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="capacity-micro-telemetry mobile-only font-mono" role="status" aria-label="Capacity Micro Telemetry">
      <span class="tel-item tel-sat">📈 {{ clusterSaturation?.value || '0%' }} sat</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-runway">⏳ {{ daysToExhaustion?.value || '--' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-pack">📦 {{ binPackingEfficiency?.value || '0%' }} pack</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-hdrm">🛡️ {{ safeHeadroom?.value || '0%' }} hdrm</span>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Metrics HUD Grid (Top 4 Metrics) -->
    <CapacityHudCards
      class="desktop-only"
      :saturation="clusterSaturation"
      :exhaustion="daysToExhaustion"
      :bin-packing="binPackingEfficiency"
      :headroom="safeHeadroom"
    />

    <!-- Resource Forecast Predictive Trend Chart (Desktop Full linear regression) -->
    <ResourceForecastChart
      class="desktop-only"
      :forecasts="forecasts"
    />

    <!-- Bespoke Mobile SVG Forecast Trend Card (Mobile Adaption) -->
    <CapacityMobileTrendCard
      class="mobile-only"
      :forecasts="forecasts"
      :exhaustion="daysToExhaustion"
    />

    <!-- Node Headroom Matrix: Desktop Table -->
    <div class="desktop-only-wrapper">
      <NodeHeadroomTable
        :nodes="nodesHeadroom"
        @rebalance="rebalanceNode"
        @inspect="handleInspectNode"
      />
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
      <CapacityMobileCards
        :nodes="nodesHeadroom"
        @rebalance="rebalanceNode"
        @inspect="handleInspectNode"
      />
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
          <div class="rec-icon">{{ rec.icon }}</div>
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
      @rebalance="handleRebalanceFromDrawer"
    />

    <!-- Policy Modal -->
    <AddCapacityPolicyModal
      v-if="showPolicyModal"
      @close="showPolicyModal = false"
      @save="addPolicy"
    />

    <!-- Record Checkpoint Modal -->
    <div v-if="showRecordModal" class="modal-overlay" @click.self="showRecordModal = false">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge badge-cyan">CAPACITY CHECKPOINT</span>
            <h3 class="modal-title">Record Capacity Forecast Data</h3>
          </div>
          <button class="modal-close" @click="showRecordModal = false">✕</button>
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
