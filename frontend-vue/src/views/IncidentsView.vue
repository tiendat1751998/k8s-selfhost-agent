<script setup lang="ts">
import { ref } from 'vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import IncidentDetailPane from '../components/incidents/IncidentDetailPane.vue'
import IncidentSimulationModal from '../components/incidents/IncidentSimulationModal.vue'
import IncidentCreatePrModal from '../components/incidents/IncidentCreatePrModal.vue'
import IncidentMicroTelemetry from '../components/incidents/IncidentMicroTelemetry.vue'
import { useIncidents } from '../composables/useIncidents'
import type { Incident } from '../api/compute'

const {
  loading,
  actionLoading,
  toastMessage,
  incidents,
  filterSeverity,
  filterStatus,
  searchQuery,
  selectedIncident,
  selectedReport,
  activePR,
  loadingReport,
  reportError,
  showPRModal,
  showSimulateModal,
  showCreateModal,
  createForm,
  handleCreateIncident,
  prForm,
  filteredIncidents,
  totalIncidents,
  criticalCount,
  analyzingCount,
  resolvedCount,
  countAll,
  countOpen,
  countInProgress,
  countResolved,
  fetchIncidents,
  selectIncident,
  triggerAIAnalysis,
  handleSimulateIncident,
  openCreatePRModal,
  handleCreatePR,
  handleMergePR
} = useIncidents()

function resetFilters() {
  filterStatus.value = 'all'
  filterSeverity.value = 'all'
  searchQuery.value = ''
}

function openDeclareModal() {
  createForm.value = {
    pod_name: '',
    namespace: 'default',
    cluster_name: 'prod-us-east-1',
    type: 'CrashLoopBackOff',
    severity: 'critical',
    message: ''
  }
  showCreateModal.value = true
}

function submitDeclareIncident() {
  if (!createForm.value.pod_name.trim()) return
  handleCreateIncident(createForm.value)
}

// Mobile Ergonomics States
const showMobileDetail = ref(false)
const showMobileSearch = ref(false)

function handleCardSelect(inc: Incident) {
  selectIncident(inc)
  showMobileDetail.value = true
}

function formatRelativeTime(dateStr?: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return dateStr
  const diffSec = Math.floor((Date.now() - date.getTime()) / 1000)
  if (diffSec < 45) return 'Just now'
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
  return `${Math.floor(diffSec / 86400)}d ago`
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" />
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="incidents-toolbar-sleek glass-panel desktop-only">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter pod, cluster, ns, error..."
          class="toolbar-search-input"
          aria-label="Filter incidents by pod, cluster, namespace, or error"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="clear-input-btn"
          aria-label="Clear search"
          @click="searchQuery = ''"
        >
          <BaseIcon name="x" size="xs" />
        </button>
      </div>

      <!-- Status tabs / pills with count badges -->
      <div class="toolbar-status-pills" role="tablist" aria-label="Filter incidents by status">
        <button
          v-for="tab in [
            { key: 'all', label: 'All', count: countAll },
            { key: 'open', label: 'Open', count: countOpen },
            { key: 'in_progress', label: 'In Progress', count: countInProgress },
            { key: 'resolved', label: 'Resolved', count: countResolved }
          ]"
          :key="tab.key"
          type="button"
          role="tab"
          :aria-selected="filterStatus === tab.key"
          class="toolbar-pill-btn"
          :class="{ active: filterStatus === tab.key }"
          @click="filterStatus = tab.key"
        >
          {{ tab.label }} ({{ tab.count }})
        </button>
      </div>

      <!-- Severity filter dropdown -->
      <select
        v-model="filterSeverity"
        class="toolbar-select severity-dropdown"
        aria-label="Filter incidents by severity"
      >
        <option value="all">All Severities</option>
        <option value="critical">Critical</option>
        <option value="high">High</option>
        <option value="medium">Medium</option>
        <option value="low">Low</option>
      </select>

      <!-- Inline compact KPI badge strip font-mono -->
      <div class="toolbar-kpi-strip font-mono" role="status" aria-label="Incident metrics summary">
        <span class="kpi-live-dot"></span>
        <span class="kpi-live-text">{{ totalIncidents }} Incidents ({{ criticalCount }} Critical · {{ resolvedCount }} Resolved)</span>
      </div>

      <!-- Action buttons -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="toolbar-btn btn-primary"
          aria-label="Declare Operational Incident"
          @click="openDeclareModal"
        >
          <BaseIcon name="plus" size="xs" />
          <span>Declare Incident</span>
        </button>
        <button
          type="button"
          class="toolbar-btn btn-secondary"
          :disabled="loading"
          @click="fetchIncidents"
        >
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" :class="{ 'spin-animate': loading }" />
          <span>{{ loading ? 'Querying...' : 'Refresh Feed' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile Sleek 40px Command Bar -->
    <div class="mobile-command-bar mobile-only">
      <div class="mobile-command-title">
        <span class="mobile-title-icon"><BaseIcon name="alert-triangle" size="xs" /></span>
        <span class="mobile-title-text">Incidents ({{ filteredIncidents.length }})</span>
      </div>
      <div class="mobile-command-actions">
        <button
          class="mobile-action-btn"
          title="Declare Incident"
          aria-label="Declare Incident"
          @click="openDeclareModal"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          class="mobile-action-btn"
          title="Simulate Test Incident (Demo Mode)"
          aria-label="Simulate Test Incident"
          @click="showSimulateModal = true"
        >
          <BaseIcon name="zap" size="xs" />
        </button>
        <button
          class="mobile-action-btn"
          :disabled="loading"
          title="Refresh Feed"
          aria-label="Refresh Feed"
          @click="fetchIncidents"
        >
          <BaseIcon name="refresh" size="xs" />
        </button>
        <button
          class="mobile-action-btn"
          :class="{ active: showMobileSearch }"
          title="Search and Filter"
          aria-label="Toggle Search"
          @click="showMobileSearch = !showMobileSearch"
        >
          <BaseIcon name="search" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile Micro-Telemetry Strip (20px) -->
    <IncidentMicroTelemetry
      class="mobile-only"
      :total-incidents="totalIncidents"
      :critical-count="criticalCount"
      :analyzing-count="analyzingCount"
      :resolved-count="resolvedCount"
    />

    <!-- Mobile Collapsible Search & Filter Bar -->
    <div v-if="showMobileSearch" class="mobile-filter-bar mobile-only animate-fade-in">
      <div class="status-tab-group mobile-status-tabs">
        <button
          v-for="tab in [
            { key: 'all', label: 'All', count: countAll },
            { key: 'open', label: 'Open', count: countOpen },
            { key: 'in_progress', label: 'In Progress', count: countInProgress },
            { key: 'resolved', label: 'Resolved', count: countResolved }
          ]"
          :key="tab.key"
          class="status-tab-btn"
          :class="{ active: filterStatus === tab.key }"
          @click="filterStatus = tab.key"
        >
          {{ tab.label }} ({{ tab.count }})
        </button>
      </div>
      <div class="mobile-filter-inputs">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter pod, cluster, ns..."
          class="search-mini"
        />
        <select v-model="filterSeverity" class="select-mini">
          <option value="all">All Severities</option>
          <option value="critical">Critical</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </select>
      </div>
    </div>

    <!-- Specialized Split-Pane Incident Inspector -->
    <div class="split-pane-layout" :class="{ 'mobile-showing-detail': showMobileDetail }">
      <!-- Left Pane: Incident Feed List -->
      <div class="left-pane glass-panel">
        <div class="pane-header desktop-only">
          <div class="pane-title-wrap">
            <span class="pane-icon"><BaseIcon name="alert-triangle" size="xs" /></span>
            <h2 class="pane-title">Incident Queue ({{ filteredIncidents.length }})</h2>
          </div>
        </div>

        <div class="incident-list">
          <!-- Empty State: Filter Mismatch (incidents exist, but none match filters) -->
          <div v-if="filteredIncidents.length === 0 && incidents.length > 0" class="empty-list filter-mismatch-empty">
            <div class="empty-icon"><BaseIcon name="search" size="lg" /></div>
            <div class="empty-title">No Matching Incidents</div>
            <p class="empty-desc">
              No incidents match the active filters (Status: {{ filterStatus }}, Severity: {{ filterSeverity }})
            </p>
            <button class="btn-slate-primary empty-simulate-btn" @click="resetFilters">
              <BaseIcon name="refresh" size="xs" /> <span>Reset Filters</span>
            </button>
          </div>

          <!-- Empty State: Truly No Incidents in Cluster Feed -->
          <div v-else-if="filteredIncidents.length === 0" class="empty-list">
            <div class="empty-icon"><BaseIcon name="shield" size="lg" /></div>
            <div class="empty-title">No Incidents Detected</div>
            <p class="empty-desc">
              Cluster telemetry is nominal. Inject a test anomaly scenario to evaluate autonomous AI diagnostics and GitOps remediation.
            </p>
            <button class="btn-slate-primary empty-simulate-btn" @click="showSimulateModal = true">
              <BaseIcon name="zap" size="xs" /> <span>Inject Test Incident (Demo Mode)</span>
            </button>
          </div>

          <!-- High-Density Incident Cards (~68-76px height) -->
          <div
            v-for="inc in filteredIncidents"
            :key="inc.id"
            class="incident-card-item"
            :class="{ 'incident-item-active': selectedIncident?.id === inc.id, [`border-sev-${inc.severity}`]: true }"
            @click="handleCardSelect(inc)"
          >
            <div class="card-item-top">
              <div class="item-title-group">
                <span class="item-pod" :title="inc.pod_name">{{ inc.pod_name }}</span>
                <span class="item-ns font-mono">{{ inc.namespace }}</span>
              </div>
              <div class="card-badges">
                <StatusBadge :status="inc.severity" size="sm" />
                <StatusBadge :status="inc.status" size="sm" />
              </div>
            </div>

            <div class="card-item-bot">
              <span class="type-badge font-mono">{{ inc.type }}</span>
              <span class="time-stamp-clean font-mono" :title="inc.created_at">
                {{ formatRelativeTime(inc.created_at) }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Right Pane: Active Incident Inspector & AI Remediation Engine Component -->
      <div class="right-pane-wrapper">
        <div class="mobile-back-bar mobile-only">
          <button class="btn-slate mobile-back-btn" @click="showMobileDetail = false">
            <BaseIcon name="x" size="xs" /> <span>Back to Incidents</span>
          </button>
        </div>
        <IncidentDetailPane
          :selected-incident="selectedIncident"
          :selected-report="selectedReport"
          :active-p-r="activePR"
          :loading-report="loadingReport"
          :report-error="reportError"
          :action-loading="actionLoading"
          @analyze="triggerAIAnalysis"
          @open-pr-modal="openCreatePRModal"
          @merge-pr="handleMergePR"
        />
      </div>
    </div>

    <!-- Declare Operational Incident Modal -->
    <ModalDrawer
      v-model:show="showCreateModal"
      mode="modal"
      title="Declare Operational Incident"
      subtitle="Log an active Kubernetes workload anomaly for autonomous AI RCA and remediation"
      max-width="560px"
    >
      <form class="modal-form" @submit.prevent="submitDeclareIncident">
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Pod / Workload Name</label>
            <input
              v-model="createForm.pod_name"
              type="text"
              class="toolbar-search-input font-mono"
              placeholder="e.g. checkout-api-7b9c6f8d"
              required
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Namespace</label>
            <input
              v-model="createForm.namespace"
              type="text"
              class="toolbar-search-input font-mono"
              placeholder="e.g. ecommerce"
              required
            />
          </div>
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Cluster</label>
            <input
              v-model="createForm.cluster_name"
              type="text"
              class="toolbar-search-input font-mono"
              placeholder="e.g. prod-us-east-1"
              required
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Anomaly Type</label>
            <input
              v-model="createForm.type"
              type="text"
              class="toolbar-search-input font-mono"
              placeholder="e.g. CrashLoopBackOff"
              required
            />
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Severity Level</label>
          <select v-model="createForm.severity" class="toolbar-select">
            <option value="critical">Critical (P1 Outage)</option>
            <option value="high">High (P2 Degradation)</option>
            <option value="medium">Medium (P3 Warning)</option>
            <option value="low">Low (P4 Info)</option>
          </select>
        </div>

        <div class="form-group">
          <label class="form-label">Incident Symptoms / Failure Log</label>
          <textarea
            v-model="createForm.message"
            rows="3"
            class="toolbar-search-input font-mono form-textarea"
            placeholder="Describe symptoms, container exit code, or error output..."
          ></textarea>
        </div>
      </form>

      <template #footer="{ close }">
        <button type="button" class="btn-slate" @click="close">Cancel</button>
        <button
          type="button"
          class="btn-slate-primary"
          :disabled="actionLoading === 'create-incident' || !createForm.pod_name.trim()"
          @click="submitDeclareIncident"
        >
          <BaseIcon :name="actionLoading === 'create-incident' ? 'clock' : 'alert-triangle'" size="xs" />
          <span>{{ actionLoading === 'create-incident' ? 'Declaring...' : 'Declare Incident' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Create PR Modal Component -->
    <IncidentCreatePrModal
      v-model:show="showPRModal"
      :action-loading="actionLoading"
      :form="prForm"
      @submit="handleCreatePR"
    />

    <!-- Incident Simulation Modal Component -->
    <IncidentSimulationModal
      v-model:show="showSimulateModal"
      :action-loading="actionLoading"
      @simulate="handleSimulateIncident"
    />
  </div>
</template>

<style scoped>
@import '../assets/styles/views/incidents.css';
</style>
