<script setup lang="ts">
import { ref } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
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
    <!-- Desktop Header -->
    <div class="view-header desktop-only">
      <div>
        <h1 class="view-title">Kubernetes Incident Command Center</h1>
        <p class="view-desc">
          Live anomaly stream, AI Root Cause Analysis (RCA) reasoning engine, and GitOps pull request diff visualizer.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn-slate-primary" @click="showSimulateModal = true">
          <span>⚡ Simulate Incident</span>
        </button>
        <button class="btn-slate" :disabled="loading" @click="fetchIncidents">
          <span>{{ loading ? '⏳ Querying...' : '🔄 Refresh Feed' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile Sleek 40px Command Bar -->
    <div class="mobile-command-bar mobile-only">
      <div class="mobile-command-title">
        <span class="mobile-title-icon">🚨</span>
        <span class="mobile-title-text">Incidents ({{ filteredIncidents.length }})</span>
      </div>
      <div class="mobile-command-actions">
        <button
          class="mobile-action-btn"
          title="Simulate Incident"
          aria-label="Simulate Incident"
          @click="showSimulateModal = true"
        >
          <span>⚡</span>
        </button>
        <button
          class="mobile-action-btn"
          :disabled="loading"
          title="Refresh Feed"
          aria-label="Refresh Feed"
          @click="fetchIncidents"
        >
          <span>🔄</span>
        </button>
        <button
          class="mobile-action-btn"
          :class="{ active: showMobileSearch }"
          title="Search and Filter"
          aria-label="Toggle Search"
          @click="showMobileSearch = !showMobileSearch"
        >
          <span>🔍</span>
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
          class="status-tab-btn"
          :class="{ active: filterStatus === 'all' }"
          @click="filterStatus = 'all'"
        >
          All ({{ countAll }})
        </button>
        <button
          class="status-tab-btn"
          :class="{ active: filterStatus === 'open' }"
          @click="filterStatus = 'open'"
        >
          Open ({{ countOpen }})
        </button>
        <button
          class="status-tab-btn"
          :class="{ active: filterStatus === 'in_progress' }"
          @click="filterStatus = 'in_progress'"
        >
          In Progress ({{ countInProgress }})
        </button>
        <button
          class="status-tab-btn"
          :class="{ active: filterStatus === 'resolved' }"
          @click="filterStatus = 'resolved'"
        >
          Resolved ({{ countResolved }})
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

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- Desktop Metric HUD (4 cards, hidden on mobile) -->
    <div class="metrics-grid desktop-only">
      <MetricCard
        title="Detected Incidents"
        :value="totalIncidents"
        subtitle="Total cluster anomalies logged"
        icon="⚡"
        badge="TELEMETRY"
        badge-color="cyan"
      />
      <MetricCard
        title="Critical Severity"
        :value="criticalCount"
        subtitle="Workloads requiring urgent fix"
        icon="🔥"
        badge="HIGH PRIORITY"
        :badge-color="criticalCount > 0 ? 'rose' : 'emerald'"
        :trend="criticalCount > 0 ? 'Action Required' : 'Zero Critical'"
        :trend-type="criticalCount > 0 ? 'negative' : 'positive'"
      />
      <MetricCard
        title="Active Remediation"
        :value="analyzingCount"
        subtitle="AI reasoning & PR synthesis in progress"
        icon="🤖"
        badge="AI AGENT"
        badge-color="violet"
        trend="Autonomous Pipeline"
        trend-type="positive"
      />
      <MetricCard
        title="Resolved & Verified"
        :value="resolvedCount"
        subtitle="Incidents successfully healed"
        icon="🛡️"
        badge="CLOSED"
        badge-color="emerald"
        trend="Auto-Remediated"
        trend-type="positive"
      />
    </div>

    <!-- Specialized Split-Pane Incident Inspector -->
    <div class="split-pane-layout" :class="{ 'mobile-showing-detail': showMobileDetail }">
      <!-- Left Pane: Incident Feed List -->
      <div class="left-pane glass-panel">
        <div class="pane-header desktop-only">
          <div class="pane-title-wrap">
            <span class="pane-icon">🚨</span>
            <h2 class="pane-title">Incident Queue ({{ filteredIncidents.length }})</h2>
          </div>
          <div class="status-tab-group">
            <button
              class="status-tab-btn"
              :class="{ active: filterStatus === 'all' }"
              @click="filterStatus = 'all'"
            >
              All ({{ countAll }})
            </button>
            <button
              class="status-tab-btn"
              :class="{ active: filterStatus === 'open' }"
              @click="filterStatus = 'open'"
            >
              Open ({{ countOpen }})
            </button>
            <button
              class="status-tab-btn"
              :class="{ active: filterStatus === 'in_progress' }"
              @click="filterStatus = 'in_progress'"
            >
              In Progress ({{ countInProgress }})
            </button>
            <button
              class="status-tab-btn"
              :class="{ active: filterStatus === 'resolved' }"
              @click="filterStatus = 'resolved'"
            >
              Resolved ({{ countResolved }})
            </button>
          </div>
          <div class="filter-controls">
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Filter pod, cluster..."
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

        <div class="incident-list">
          <!-- Empty State: Filter Mismatch (incidents exist, but none match filters) -->
          <div v-if="filteredIncidents.length === 0 && incidents.length > 0" class="empty-list filter-mismatch-empty">
            <div class="empty-icon">🔍</div>
            <div class="empty-title">No Matching Incidents</div>
            <p class="empty-desc">
              No incidents match the active filters (Status: {{ filterStatus }}, Severity: {{ filterSeverity }})
            </p>
            <button class="btn-slate-primary empty-simulate-btn" @click="resetFilters">
              <span>↺ Reset Filters</span>
            </button>
          </div>

          <!-- Empty State: Truly No Incidents in Cluster Feed -->
          <div v-else-if="filteredIncidents.length === 0" class="empty-list">
            <div class="empty-icon">🛡️</div>
            <div class="empty-title">No Incidents Detected</div>
            <p class="empty-desc">
              Cluster telemetry is nominal. Inject a test anomaly scenario to evaluate autonomous AI diagnostics and GitOps remediation.
            </p>
            <button class="btn-slate-primary empty-simulate-btn" @click="showSimulateModal = true">
              <span>⚡ Inject Test Incident (Demo Mode)</span>
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
            <span>✕ Back to Incidents</span>
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

<style>
@import '../assets/styles/views/incidents.css';
</style>
