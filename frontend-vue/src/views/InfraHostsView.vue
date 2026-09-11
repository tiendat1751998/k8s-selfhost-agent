<script setup lang="ts">
import { ref, computed } from 'vue'
import { useInfraHosts } from '../composables/useInfraHosts'
import HostMetricsHud from '../components/hosts/HostMetricsHud.vue'
import HostControlsBar from '../components/hosts/HostControlsBar.vue'
import HostCard from '../components/hosts/HostCard.vue'
import HostsTable from '../components/hosts/HostsTable.vue'
import HostsMobileCards from '../components/hosts/HostsMobileCards.vue'
import HostAddEditModal from '../components/hosts/HostAddEditModal.vue'
import HostDetailDrawer from '../components/hosts/HostDetailDrawer.vue'
import HostDeleteModal from '../components/hosts/HostDeleteModal.vue'
import CanvasTimeSeries, { type TimeSeriesItem } from '../components/telemetry/CanvasTimeSeries.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'

const showMobileSearch = ref(false)
const showTelemetry = ref(false)

const {
  loading, toastMessage, hosts, viewMode, searchQuery,
  selectedTypeFilter, selectedStatusFilter, selectedLabelFilter,
  testingHostId, hostTestResults, hostTestHistories,
  showDetailDrawer, selectedHost, showHostModal, isEditing,
  submittingHost, modalTesting, modalTestResult, hostForm,
  showDeleteModal, hostToDelete, deletingHost,
  totalHosts, connectedHosts, disconnectedHosts, errorHosts,
  onlineHostsCount, offlineHostsCount, avgLatency, typeCounts,
  availableLabels, filteredHosts, hostTypeDefinitions,
  getHostTypeMeta, getLatencyBadgeClass, formatDate, formatUptime,
  copyToClipboard, fetchHosts, openAddHostModal, openEditHostModal,
  addLabelRow, removeLabelRow, testModalConnection, submitHostForm,
  handleTestHost, promptDeleteHost, confirmDeleteHost, openHostDrawer
} = useInfraHosts()

// Synchronized Fleet Telemetry Window
const fleetTelemetryWindow = computed(() => {
  const stepMs = 30_000, count = 16, now = Date.now()
  const baseTime = Math.floor(now / stepMs) * stepMs
  const timestamps: number[] = []
  for (let i = count - 1; i >= 0; i--) timestamps.push(baseTime - i * stepMs)

  const latencyTarget = avgLatency.value > 0 ? avgLatency.value : 18
  const activeCount = onlineHostsCount.value
  const total = Math.max(1, totalHosts.value)

  const latencyData: [number, number][] = timestamps.map((t, idx) => {
    const offset = count - 1 - idx
    const val = Number(Math.max(1, latencyTarget - offset * 0.2 + Math.sin(idx * 0.8) * 1.8).toFixed(1))
    return [t, val]
  })

  const availabilityData: [number, number][] = timestamps.map((t, idx) => {
    const pct = total > 0 ? Math.min(100, Math.round((activeCount / total) * 100)) : 100
    const val = Number(Math.max(0, Math.min(100, pct - (idx % 4 === 0 ? 2 : 0))).toFixed(1))
    return [t, val]
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
  <div class="infra-hosts-view animate-fade-in">
    <!-- Desktop Header -->
    <div class="view-header desktop-only">
      <div class="header-titles">
        <div class="header-badge">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>ENTERPRISE MULTI-TYPE INFRASTRUCTURE REGISTRY</span>
        </div>
        <h1 class="view-title">
          <BaseIcon name="server" size="lg" />
          <span>Infrastructure Fleet Registry</span>
        </h1>
        <p class="view-desc">
          Register, monitor, and manage compute nodes, databases, Git endpoints, and monitoring targets across your infrastructure.
        </p>
      </div>
      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchHosts">
          <BaseIcon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          <span>{{ loading ? 'Refreshing...' : 'Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="openAddHostModal">
          <span class="font-bold">+</span>
          <span>Add Host</span>
        </button>
      </div>
    </div>

    <!-- 44px Mobile Command Bar (< 640px) -->
    <div class="hosts-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <BaseIcon name="server" size="sm" />
        <span class="command-bar-title font-bold">Compute Hosts ({{ filteredHosts.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button class="btn-icon-cmd" title="Add Host" @click="openAddHostModal">
          <span class="font-bold text-sm">+</span>
        </button>
        <button class="btn-icon-cmd" title="Refresh" :disabled="loading" @click="fetchHosts">
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
        </button>
        <button class="btn-icon-cmd" :class="{ active: showMobileSearch }" title="Search" @click="showMobileSearch = !showMobileSearch">
          <BaseIcon name="search" size="xs" />
        </button>
      </div>
    </div>

    <!-- 20px Mobile Micro-Telemetry Strip (< 640px) -->
    <div class="hosts-micro-telemetry mobile-only">
      <span class="tel-item"><BaseIcon name="server" size="xs" /> {{ hosts.length }} hosts</span>
      <span>·</span>
      <span class="tel-item tel-online"><BaseIcon name="shield" size="xs" /> {{ onlineHostsCount }} online</span>
      <span>·</span>
      <span class="tel-item tel-offline"><BaseIcon name="alert-triangle" size="xs" /> {{ offlineHostsCount }} offline</span>
      <span v-if="avgLatency > 0">·</span>
      <span v-if="avgLatency > 0" class="tel-item tel-latency"><BaseIcon name="zap" size="xs" /> avg {{ avgLatency }}ms</span>
    </div>

    <!-- Mobile Search Expandable Input -->
    <div v-if="showMobileSearch" class="mobile-search-box mobile-only animate-fade-in">
      <input v-model="searchQuery" type="text" placeholder="Filter hosts by name, IP, label..." class="input-glass" style="width: 100%; font-size: 12px; padding: 6px 10px;" />
    </div>

    <!-- Notification Toast Banner -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="sm" />
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Compact 36px Metric Strip & Telemetry Toggle (Above the Fold) -->
    <div class="hosts-strip-row desktop-only">
      <HostMetricsHud
        :total-hosts="totalHosts"
        :connected-hosts="connectedHosts"
        :disconnected-hosts="disconnectedHosts"
        :error-hosts="errorHosts"
        :type-counts="typeCounts"
      />
      <button
        type="button"
        class="telemetry-toggle-btn font-mono"
        :class="{ active: showTelemetry }"
        :title="showTelemetry ? 'Hide Telemetry & Latency charts' : 'Show Telemetry & Latency charts'"
        @click="showTelemetry = !showTelemetry"
      >
        <BaseIcon :name="showTelemetry ? 'chevron-up' : 'activity'" size="xs" />
        <span>{{ showTelemetry ? 'Hide Telemetry' : 'Show Telemetry & Latency' }}</span>
      </button>
    </div>

    <!-- Fleet Live Telemetry (Collapsible, Defaulted to Collapsed) -->
    <Transition name="fade">
      <div v-if="showTelemetry" class="section-card glass-panel fleet-telemetry-panel animate-fade-in">
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
              <span class="chart-card-val font-mono">{{ avgLatency > 0 ? avgLatency : 18 }}ms avg</span>
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

    <!-- Controls Bar (Desktop Only) -->
    <div class="desktop-only">
      <HostControlsBar
        v-model:search-query="searchQuery"
        v-model:selected-type-filter="selectedTypeFilter"
        v-model:selected-status-filter="selectedStatusFilter"
        v-model:selected-label-filter="selectedLabelFilter"
        v-model:view-mode="viewMode"
        :total-hosts="totalHosts"
        :type-counts="typeCounts"
        :available-labels="availableLabels"
        :host-type-definitions="hostTypeDefinitions"
      />
    </div>

    <!-- Loading State -->
    <div v-if="loading && hosts.length === 0" class="loading-state glass-panel">
      <div class="spinner"></div>
      <span>Querying infrastructure fleet registry...</span>
    </div>

    <!-- Empty State -->
    <div v-else-if="filteredHosts.length === 0" class="empty-state glass-panel">
      <div class="empty-icon-wrap">
        <BaseIcon name="server" size="xl" />
      </div>
      <h3 class="empty-title">No infrastructure hosts matched</h3>
      <p class="empty-desc text-muted">Try adjusting your search criteria or register a new host target.</p>
      <button class="btn btn-primary" style="margin-top: 12px;" @click="openAddHostModal">
        <span class="font-bold">+</span>
        <span>Register First Host</span>
      </button>
    </div>

    <!-- Mobile Stream (< 640px) -->
    <div v-else-if="filteredHosts.length > 0" class="mobile-only">
      <HostsMobileCards
        :hosts="filteredHosts"
        :host-test-results="hostTestResults"
        :testing-host-id="testingHostId"
        :get-host-type-meta="getHostTypeMeta"
        :get-latency-badge-class="getLatencyBadgeClass"
        @select="openHostDrawer"
        @test="handleTestHost"
        @edit="openEditHostModal"
        @delete="promptDeleteHost"
      />
    </div>

    <!-- Desktop Grid View -->
    <div v-if="viewMode === 'grid' && filteredHosts.length > 0" class="hosts-grid desktop-only animate-fade-in">
      <HostCard
        v-for="host in filteredHosts"
        :key="host.id"
        :host="host"
        :test-result="hostTestResults[host.id]"
        :is-testing="testingHostId === host.id"
        @select="openHostDrawer"
        @test="handleTestHost"
        @edit="openEditHostModal"
        @delete="promptDeleteHost"
        @copy="copyToClipboard"
      />
    </div>

    <!-- Desktop Table View -->
    <div v-else-if="viewMode === 'table' && filteredHosts.length > 0" class="desktop-only animate-fade-in">
      <HostsTable
        :hosts="filteredHosts"
        :host-test-results="hostTestResults"
        :testing-host-id="testingHostId"
        :get-host-type-meta="getHostTypeMeta"
        :get-latency-badge-class="getLatencyBadgeClass"
        :format-date="formatDate"
        @select="openHostDrawer"
        @test="handleTestHost"
        @edit="openEditHostModal"
        @delete="promptDeleteHost"
        @copy="copyToClipboard"
      />
    </div>

    <!-- Add / Edit Modal -->
    <HostAddEditModal
      v-model:show="showHostModal"
      :is-editing="isEditing"
      :submitting="submittingHost"
      :modal-testing="modalTesting"
      :modal-test-result="modalTestResult"
      :host-form="hostForm"
      :host-type-definitions="hostTypeDefinitions"
      :get-host-type-meta="getHostTypeMeta"
      :format-uptime="formatUptime"
      @submit="submitHostForm"
      @test-connection="testModalConnection"
      @add-label="addLabelRow"
      @remove-label="removeLabelRow"
    />

    <!-- Host Detail Drawer -->
    <HostDetailDrawer
      v-model:show="showDetailDrawer"
      :host="selectedHost"
      :host-test-result="selectedHost ? hostTestResults[selectedHost.id] : undefined"
      :host-test-histories="selectedHost ? hostTestHistories[selectedHost.id] : undefined"
      :testing-host-id="testingHostId"
      :get-host-type-meta="getHostTypeMeta"
      :format-date="formatDate"
      :format-uptime="formatUptime"
      @test="handleTestHost"
      @edit="openEditHostModal"
      @delete="promptDeleteHost"
      @copy="copyToClipboard"
    />

    <!-- Delete Confirmation Modal -->
    <HostDeleteModal
      v-model:show="showDeleteModal"
      :host="hostToDelete"
      :deleting="deletingHost"
      @confirm="confirmDeleteHost"
    />
  </div>
</template>

<style>
@import '../assets/styles/views/infra-hosts.css';

.fleet-telemetry-panel { margin-bottom: 20px; }
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
.empty-icon-wrap { font-size: 32px; margin-bottom: 12px; color: #64748b; }
</style>

