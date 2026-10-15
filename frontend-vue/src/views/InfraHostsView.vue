<script setup lang="ts">
import { ref } from 'vue'
import { useInfraHosts } from '../composables/useInfraHosts'
import HostControlsBar from '../components/hosts/HostControlsBar.vue'
import HostsGrid from '../components/hosts/HostsGrid.vue'
import HostsTable from '../components/hosts/HostsTable.vue'
import HostsMobileCards from '../components/hosts/HostsMobileCards.vue'
import HostAddEditModal from '../components/hosts/HostAddEditModal.vue'
import HostDetailDrawer from '../components/hosts/HostDetailDrawer.vue'
import HostDeleteModal from '../components/hosts/HostDeleteModal.vue'
import HostFleetTelemetryPanel from '../components/hosts/HostFleetTelemetryPanel.vue'
import InfraHostsHeaderBar from '../components/hosts/InfraHostsHeaderBar.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import NodeStorageFencingModal from '../components/overview/drawer/NodeStorageFencingModal.vue'
import type { FencingExecutionReport } from '../api/dr'

const showMobileSearch = ref(false)
const showTelemetry = ref(false)
const showFencingModal = ref(false)
const fencingTargetNode = ref('')
const fencingTargetCluster = ref('default')

function openFencingModal(nodeName: string, clusterId = 'default') {
  fencingTargetNode.value = nodeName
  fencingTargetCluster.value = clusterId
  showFencingModal.value = true
}

function handleFenced(report: FencingExecutionReport) {
  toastMessage.value = {
    type: 'success',
    text: `Storage fencing & pod failover completed for ${report.node_name} in ${report.duration_ms ?? 0}ms.`
  }
  fetchHosts()
}

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
    <!-- Mobile Command Bar & Fencing Alert Banner -->
    <InfraHostsHeaderBar
      v-model:show-mobile-search="showMobileSearch"
      v-model:search-query="searchQuery"
      v-model:show-telemetry="showTelemetry"
      v-model:selected-type-filter="selectedTypeFilter"
      v-model:selected-status-filter="selectedStatusFilter"
      :filtered-hosts-count="filteredHosts.length"
      :loading="loading"
      :hosts="hosts"
      :online-hosts-count="onlineHostsCount"
      :offline-hosts-count="offlineHostsCount"
      :avg-latency="avgLatency"
      :type-counts="typeCounts"
      :host-type-definitions="hostTypeDefinitions"
      @add-host="openAddHostModal"
      @refresh="fetchHosts"
      @open-fencing="openFencingModal($event, hosts.find(h => h.status !== 'connected')?.tenant_id || 'default')"
    />


    <!-- Unified 38px Controls Toolbar (Desktop Only) -->
    <div class="desktop-only">
      <HostControlsBar
        v-model:search-query="searchQuery"
        v-model:selected-type-filter="selectedTypeFilter"
        v-model:selected-status-filter="selectedStatusFilter"
        v-model:selected-label-filter="selectedLabelFilter"
        v-model:view-mode="viewMode"
        v-model:show-telemetry="showTelemetry"
        :total-hosts="totalHosts"
        :connected-hosts="connectedHosts"
        :disconnected-hosts="disconnectedHosts"
        :error-hosts="errorHosts"
        :type-counts="typeCounts"
        :available-labels="availableLabels"
        :host-type-definitions="hostTypeDefinitions"
        :loading="loading"
        @refresh="fetchHosts"
        @add-host="openAddHostModal"
      />
    </div>

    <!-- Fleet Live Telemetry (Collapsible, Defaulted to Collapsed) -->
    <HostFleetTelemetryPanel
      :show-telemetry="showTelemetry"
      :total-hosts="totalHosts"
      :connected-hosts="connectedHosts"
      :disconnected-hosts="disconnectedHosts"
      :error-hosts="errorHosts"
      :type-counts="typeCounts"
      :online-hosts-count="onlineHostsCount"
      :avg-latency="avgLatency"
    />

    <!-- Loading State -->
    <div v-if="loading && hosts.length === 0" class="loading-state glass-panel">
      <div class="spinner"></div>
      <span>Querying infrastructure fleet registry...</span>
    </div>

    <!-- Empty State (No hosts registered in fleet) -->
    <div v-else-if="hosts.length === 0" class="empty-state glass-panel">
      <div class="empty-icon-wrap">
        <BaseIcon name="server" size="xl" />
      </div>
      <h3 class="empty-title">No infrastructure hosts registered</h3>
      <p class="empty-desc text-muted">Register your first compute node to begin monitoring fleet health and telemetry.</p>
      <button class="btn btn-primary" style="margin-top: 12px;" @click="openAddHostModal">
        <span class="font-bold">+</span>
        <span>Register First Host</span>
      </button>
    </div>

    <!-- Empty Filter State (Hosts exist, but filter yields 0) -->
    <div v-else-if="filteredHosts.length === 0" class="empty-state glass-panel">
      <div class="empty-icon-wrap">
        <BaseIcon name="search" size="xl" />
      </div>
      <h3 class="empty-title">No compute hosts found</h3>
      <p class="empty-desc text-muted">No hosts match the selected search and filter criteria.</p>
    </div>

    <!-- Live Hosts Content -->
    <div v-else>
      <!-- Mobile Stream (< 640px) -->
      <div class="mobile-only">
        <HostsMobileCards
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
          @fencing="openFencingModal($event.name, $event.tenant_id || 'default')"
        />
      </div>

      <!-- Desktop Grid View -->
      <div v-if="viewMode === 'grid'" class="desktop-only animate-fade-in">
        <HostsGrid
          :hosts="filteredHosts"
          :host-test-results="hostTestResults"
          :testing-host-id="testingHostId"
          :get-host-type-meta="getHostTypeMeta"
          :format-date="formatDate"
          :format-uptime="formatUptime"
          @select="openHostDrawer"
          @test="handleTestHost"
          @edit="openEditHostModal"
          @delete="promptDeleteHost"
          @copy="copyToClipboard"
        />
      </div>

      <!-- Desktop Table View -->
      <div v-else-if="viewMode === 'table'" class="desktop-only animate-fade-in">
        <HostsTable
          :hosts="filteredHosts"
          :total-count="hosts.length"
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
          @add-host="openAddHostModal"
        />
      </div>
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

    <!-- Dead Node Recovery & Zero-Data-Loss Storage Fencing Modal -->
    <NodeStorageFencingModal
      v-model:show="showFencingModal"
      :node-name="fencingTargetNode"
      :cluster-id="fencingTargetCluster"
      @fenced="handleFenced"
    />
  </div>
</template>

<style>
@import '../assets/styles/views/infra-hosts.css';
</style>

<style scoped>
.empty-icon-wrap { font-size: 32px; margin-bottom: 12px; color: #64748b; }
:deep(.pill-count) { margin-left: 4px; }
</style>


