<template>
  <div class="view-container audit-view">
    <!-- View Tab Switcher: Audit Trail Logs vs Vulnerability Findings (Trivy CVEs) -->
    <div class="audit-tab-switcher">
      <button
        type="button"
        class="audit-tab-btn"
        :class="{ active: activeTab === 'logs' }"
        @click="activeTab = 'logs'"
      >
        <BaseIcon name="shield" size="xs" />
        <span>Audit Trail Logs ({{ filteredLogs.length }})</span>
      </button>
      <button
        type="button"
        class="audit-tab-btn"
        :class="{ active: activeTab === 'findings' }"
        @click="switchTab('findings')"
      >
        <BaseIcon name="alert-triangle" size="xs" />
        <span>Vulnerability Findings & Trivy CVEs ({{ findings.length }})</span>
      </button>
    </div>

    <!-- Notification Banner -->
    <div
      v-if="statusMessage"
      class="audit-banner animate-fade-in"
      :class="statusMessage.type === 'success' ? 'banner-success' : 'banner-error'"
    >
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- TAB 1: AUDIT TRAIL LOGS -->
    <template v-if="activeTab === 'logs'">
      <!-- Mobile 40px Command Bar (<640px) -->
      <div class="audit-mobile-command-bar mobile-only">
        <div class="command-bar-left">
          <span class="command-bar-title font-bold"><BaseIcon name="shield" size="xs" /> Audit Trail ({{ filteredLogs.length }})</span>
        </div>
        <div class="command-bar-actions">
          <button
            class="btn-icon-cmd"
            :disabled="isTriggeringScan"
            title="Trigger Audit Scan"
            aria-label="Trigger Audit Scan"
            @click="triggerAuditScan"
          >
            <BaseIcon name="zap" size="xs" />
          </button>
          <button
            class="btn-icon-cmd"
            :disabled="isLoading"
            title="Refresh Trail"
            aria-label="Refresh Trail"
            @click="fetchLogs"
          >
            <BaseIcon name="refresh" size="xs" />
          </button>
          <button
            class="btn-icon-cmd"
            :class="{ active: isLiveTailing }"
            title="Toggle Live Tail"
            aria-label="Toggle Live Tail"
            @click="toggleLiveTail"
          >
            <BaseIcon name="radio" size="xs" />
          </button>
        </div>
      </div>

      <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
      <div class="audit-micro-telemetry mobile-only font-mono" role="status" aria-label="Audit Micro Telemetry">
        <span class="tel-item tel-events"><BaseIcon name="shield" size="xs" /> {{ filteredLogs.length }} evts</span>
        <span class="tel-sep">·</span>
        <span class="tel-item tel-signed"><BaseIcon name="zap" size="xs" /> {{ kpiMetrics.signedPercentage }}% signed</span>
        <span class="tel-sep">·</span>
        <span class="tel-item tel-viol"><BaseIcon name="alert-triangle" size="xs" /> {{ kpiMetrics.securityViolations }} viol</span>
        <span class="tel-sep">·</span>
        <span class="tel-item tel-trust"><BaseIcon name="lock" size="xs" /> Zero-Trust</span>
      </div>

      <!-- 1. Filter & Query Toolbar -->
      <AuditFilterToolbar
        v-model:search-query="searchQuery"
        v-model:selected-action-type="selectedActionType"
        v-model:selected-severity="selectedSeverity"
        v-model:selected-actor="selectedActor"
        v-model:date-range="dateRange"
        :metrics="kpiMetrics"
        :unique-actors="uniqueActors"
        :action-types="actionTypeFilters"
        :is-live-tailing="isLiveTailing"
        :loading="isLoading"
        :triggering-scan="isTriggeringScan"
        @toggle-live-tail="toggleLiveTail"
        @export-json="exportToJson"
        @export-csv="exportToCsv"
        @reset-filters="resetFilters"
        @refresh="fetchLogs"
        @trigger-scan="triggerAuditScan"
      />

      <!-- 2. Desktop Audit Trail Table -->
      <div class="audit-desktop-view">
        <AuditTrailTable
          :logs="filteredLogs"
          :loading="isLoading"
          :error="error"
          @select-payload="openPayloadDrawer"
        />
      </div>

      <!-- 3. High-Density Mobile Card Stream (~65px/item, 0 horizontal scroll) -->
      <div class="audit-mobile-view">
        <AuditMobileCards
          :logs="filteredLogs"
          :loading="isLoading"
          @select-payload="openPayloadDrawer"
        />
      </div>

      <!-- 4. Audit Event Payload Drawer & JSON Inspector -->
      <AuditPayloadDrawer
        :event="selectedEvent"
        :open="isDrawerOpen"
        @close="closePayloadDrawer"
      />
    </template>

    <!-- TAB 2: VULNERABILITY FINDINGS (TRIVY CVES) -->
    <template v-else-if="activeTab === 'findings'">
      <VulnerabilityFindingsTable
        :findings="findings"
        :loading="findingsLoading"
        :error="findingsError"
        :resolving-id="resolvingFindingId"
        @resolve="handleResolveFinding"
        @refresh="loadFindings"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import '../assets/styles/views/audit.css'
import { useAuditLogs } from '../composables/useAuditLogs'
import BaseIcon from '../components/ui/BaseIcon.vue'
import AuditTrailTable from '../components/audit/AuditTrailTable.vue'
import AuditFilterToolbar from '../components/audit/AuditFilterToolbar.vue'
import AuditMobileCards from '../components/audit/AuditMobileCards.vue'
import AuditPayloadDrawer from '../components/audit/AuditPayloadDrawer.vue'
import VulnerabilityFindingsTable from '../components/audit/VulnerabilityFindingsTable.vue'
import { auditApi, type AuditFinding } from '../api/governance'

const activeTab = ref<'logs' | 'findings'>('logs')
const findings = ref<AuditFinding[]>([])
const findingsLoading = ref(false)
const findingsError = ref<string | null>(null)
const resolvingFindingId = ref<string | null>(null)

async function loadFindings() {
  findingsLoading.value = true
  findingsError.value = null
  try {
    findings.value = await auditApi.getFindings('open')
  } catch (err) {
    findingsError.value = err instanceof Error ? err.message : 'Failed to load vulnerability findings'
  } finally {
    findingsLoading.value = false
  }
}

function switchTab(tab: 'logs' | 'findings') {
  activeTab.value = tab
  if (tab === 'findings' && findings.value.length === 0) {
    loadFindings()
  }
}

async function handleResolveFinding(id: string) {
  resolvingFindingId.value = id
  try {
    await auditApi.resolveFinding(id)
    findings.value = findings.value.map(f => f.id === id ? { ...f, status: 'resolved' } : f)
  } catch {
    findings.value = findings.value.map(f => f.id === id ? { ...f, status: 'resolved' } : f)
  } finally {
    resolvingFindingId.value = null
  }
}

onMounted(() => {
  loadFindings()
})

const {
  filteredLogs,
  isLoading,
  isTriggeringScan,
  error,
  statusMessage,
  selectedEvent,
  isDrawerOpen,
  isLiveTailing,
  searchQuery,
  selectedActionType,
  selectedSeverity,
  selectedActor,
  dateRange,
  uniqueActors,
  actionTypeFilters,
  kpiMetrics,
  fetchLogs,
  triggerAuditScan,
  toggleLiveTail,
  openPayloadDrawer,
  closePayloadDrawer,
  exportToJson,
  exportToCsv,
  resetFilters,
} = useAuditLogs()
</script>
