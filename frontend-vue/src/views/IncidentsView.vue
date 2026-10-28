<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import BaseIcon from '../components/ui/BaseIcon.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import IncidentDetailPane from '../components/incidents/IncidentDetailPane.vue'
import IncidentSimulationModal from '../components/incidents/IncidentSimulationModal.vue'
import IncidentCreatePrModal from '../components/incidents/IncidentCreatePrModal.vue'
import CreateIncidentModal from '../components/incidents/CreateIncidentModal.vue'
import RcaTimelineModal from '../components/incidents/RcaTimelineModal.vue'
import IncidentsHeaderBar from '../components/incidents/IncidentsHeaderBar.vue'
import { useIncidents, isSimulationAllowed } from '../composables/useIncidents'
import type { Incident } from '../api/compute'

const route = useRoute()
const allowSimulate = computed(() => isSimulationAllowed(route.query as Record<string, unknown>))

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
  showRcaModal,
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
  selectedBlastRadius,
  rcaTimelineEvents,
  fetchIncidents,
  selectIncident,
  triggerAIAnalysis,
  handleSimulateIncident,
  handleMitigateIncident,
  handleResolveIncident,
  handleAcknowledgeIncident,
  handleExportPostmortem,
  openCreatePRModal,
  handleCreatePR,
  handleMergePR,
  openRcaTimeline
} = useIncidents()

const cappedIncidents = computed(() => {
  return filteredIncidents.value.length > 100 ? filteredIncidents.value.slice(0, 100) : filteredIncidents.value
})

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
    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" />
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <IncidentsHeaderBar
      v-model:search-query="searchQuery"
      v-model:filter-status="filterStatus"
      v-model:filter-severity="filterSeverity"
      v-model:show-mobile-search="showMobileSearch"
      :count-all="countAll"
      :count-open="countOpen"
      :count-in-progress="countInProgress"
      :count-resolved="countResolved"
      :total-incidents="totalIncidents"
      :critical-count="criticalCount"
      :analyzing-count="analyzingCount"
      :resolved-count="resolvedCount"
      :filtered-count="filteredIncidents.length"
      :loading="loading"
      :allow-simulate="allowSimulate"
      @declare-incident="showCreateModal = true"
      @simulate-incident="showSimulateModal = true"
      @refresh="fetchIncidents"
    />

    <!-- SRE Incident Blast-Radius & MTTR Command Strip -->
    <div class="sre-incident-command-strip glass-panel" role="region" aria-label="SRE Incident Command & Blast Radius Strip">
      <div class="strip-item">
        <span class="strip-label font-mono">BLAST RADIUS</span>
        <span class="strip-val font-mono" :class="selectedBlastRadius?.riskLevel?.includes('Critical') ? 'text-rose' : 'text-cyan'">
          <BaseIcon name="target" size="xs" /> {{ selectedBlastRadius?.riskLevel || (countOpen === 0 ? 'Contained (0 Active)' : 'Nominal Scope') }}
          <template v-if="selectedBlastRadius">({{ selectedBlastRadius.impactScore }}/100)</template>
        </span>
      </div>
      <span class="strip-divider font-mono">|</span>
      <div class="strip-item">
        <span class="strip-label font-mono">SCOPE</span>
        <span class="strip-val font-mono text-amber truncate">
          {{ selectedIncident ? `${selectedIncident.cluster_name}/${selectedIncident.namespace}/${selectedIncident.pod_name}` : 'No active incident selected' }}
        </span>
      </div>
      <span class="strip-divider font-mono">|</span>
      <div class="strip-item">
        <span class="strip-label font-mono">HEALTH</span>
        <span class="strip-val font-mono text-emerald">
          {{ selectedBlastRadius?.nodeHealth || (countOpen === 0 ? 'Nominal (Resolved)' : 'Nominal Workload Health') }}
        </span>
      </div>
      <span class="strip-divider font-mono">|</span>
      <div class="strip-item">
        <span class="strip-label font-mono">TRIAGE MTTR</span>
        <span class="strip-val font-mono text-cyan">
          {{ criticalCount }} P1 Active · {{ countInProgress }} Mitigating · {{ countResolved }} Cleared
        </span>
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
              All cluster workloads and node telemetry are currently nominal. Real incidents detected by log & metrics daemons will stream here in real time.
            </p>
            <button v-if="allowSimulate" class="btn-slate-primary empty-simulate-btn" @click="showSimulateModal = true">
              <BaseIcon name="zap" size="xs" /> <span>Inject Test Incident (Debug Mode)</span>
            </button>
          </div>

          <!-- High-Density Incident Cards (~68-76px height) -->
          <div
            v-for="inc in cappedIncidents"
            :key="inc.id"
            class="incident-card-item"
            :class="{ 'incident-item-active': selectedIncident?.id === inc.id, [`border-sev-${inc.severity}`]: true }"
            @click="handleCardSelect(inc)"
          >
            <div class="card-item-top">
              <span class="item-pod" :title="inc.pod_name">{{ inc.pod_name }}</span>
              <div class="card-badges">
                <StatusBadge :status="inc.severity" size="sm" />
                <StatusBadge :status="inc.status" size="sm" />
              </div>
            </div>

            <div class="card-item-bot">
              <div class="card-bot-left">
                <span class="type-badge font-mono">{{ inc.type }}</span>
                <span class="item-ns font-mono">{{ inc.namespace }}</span>
              </div>
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
          @acknowledge="handleAcknowledgeIncident"
          @export-postmortem="handleExportPostmortem"
          @inspect-timeline="openRcaTimeline"
          @mitigate="handleMitigateIncident"
          @resolve="handleResolveIncident"
          @open-pr-modal="openCreatePRModal"
          @merge-pr="handleMergePR"
        />
      </div>
    </div>

    <!-- Create Incident Modal Component -->
    <CreateIncidentModal
      v-model:show="showCreateModal"
      :action-loading="actionLoading"
      @create="handleCreateIncident"
    />

    <!-- Interactive RCA Chronology & Reasoning Timeline Modal -->
    <RcaTimelineModal
      v-model:show="showRcaModal"
      :incident="selectedIncident"
      :report="selectedReport"
      :active-pr="activePR"
      :events="rcaTimelineEvents"
    />

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

.sre-incident-command-strip {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 14px;
  border-radius: 8px;
  background: var(--bg-card);
  border: 1px solid var(--color-border-subtle);
  font-size: 12px;
  margin-bottom: 2px;
  flex-wrap: wrap;
}

.strip-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.strip-label {
  color: var(--text-muted);
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.strip-val {
  font-size: 11.5px;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.strip-divider {
  color: var(--color-border-subtle);
  user-select: none;
}
</style>
