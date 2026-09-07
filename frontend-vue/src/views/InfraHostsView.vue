<script setup lang="ts">
import { ref } from 'vue'
import { useInfraHosts } from '../composables/useInfraHosts'
import HostMetricsHud from '../components/hosts/HostMetricsHud.vue'
import HostControlsBar from '../components/hosts/HostControlsBar.vue'
import HostCard from '../components/hosts/HostCard.vue'
import HostsTable from '../components/hosts/HostsTable.vue'
import HostsMobileCards from '../components/hosts/HostsMobileCards.vue'
import HostAddEditModal from '../components/hosts/HostAddEditModal.vue'
import HostDetailDrawer from '../components/hosts/HostDetailDrawer.vue'
import HostDeleteModal from '../components/hosts/HostDeleteModal.vue'

const showMobileSearch = ref(false)

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
        <h1 class="view-title">🖥️ Infrastructure Fleet Registry</h1>
        <p class="view-desc">
          Register, monitor, and manage compute nodes, databases, Git endpoints, and monitoring targets across your infrastructure.
        </p>
      </div>
      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchHosts">
          <span>{{ loading ? '⏳ Refreshing...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="openAddHostModal">
          <span>➕ Add Host</span>
        </button>
      </div>
    </div>

    <!-- 44px Mobile Command Bar (< 640px) -->
    <div class="hosts-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title">🖥️ Compute Hosts ({{ filteredHosts.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button class="btn-icon-cmd" title="Add Host" @click="openAddHostModal">➕</button>
        <button class="btn-icon-cmd" title="Refresh" :disabled="loading" @click="fetchHosts">🔄</button>
        <button class="btn-icon-cmd" :class="{ active: showMobileSearch }" title="Search" @click="showMobileSearch = !showMobileSearch">🔍</button>
      </div>
    </div>

    <!-- 20px Mobile Micro-Telemetry Strip (< 640px) -->
    <div class="hosts-micro-telemetry mobile-only">
      <span class="tel-item">🖥️ {{ hosts.length }} hosts</span>
      <span>·</span>
      <span class="tel-item tel-online">🛡️ {{ onlineHostsCount }} online</span>
      <span>·</span>
      <span class="tel-item tel-offline">⚠️ {{ offlineHostsCount }} offline</span>
      <span v-if="avgLatency > 0">·</span>
      <span v-if="avgLatency > 0" class="tel-item tel-latency">⚡ avg {{ avgLatency }}ms</span>
    </div>

    <!-- Mobile Search Expandable Input -->
    <div v-if="showMobileSearch" class="mobile-search-box mobile-only animate-fade-in">
      <input v-model="searchQuery" type="text" placeholder="Filter hosts by name, IP, label..." class="input-glass" style="width: 100%; font-size: 12px; padding: 6px 10px;" />
    </div>

    <!-- Notification Toast Banner -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- 4-Card KPI Metric HUD (Desktop Only) -->
    <div class="desktop-only">
      <HostMetricsHud
        :total-hosts="totalHosts"
        :connected-hosts="connectedHosts"
        :disconnected-hosts="disconnectedHosts"
        :error-hosts="errorHosts"
        :type-counts="typeCounts"
      />
    </div>

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
      <span class="empty-icon">🖥️</span>
      <h3 class="empty-title">No infrastructure hosts matched</h3>
      <p class="empty-desc text-muted">Try adjusting your search criteria or register a new host target.</p>
      <button class="btn btn-primary" style="margin-top: 12px;" @click="openAddHostModal">
        <span>➕ Register First Host</span>
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
</style>
