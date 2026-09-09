<template>
  <div class="view-container audit-view">
    <!-- Desktop View Header -->
    <div class="audit-header desktop-header desktop-only">
      <div>
        <div class="audit-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>CONTINUOUS SECURITY & CLUSTER AUDIT</span>
        </div>
        <h1 class="audit-title">Enterprise Audit Trail & Governance</h1>
        <p class="audit-desc">
          Cryptographically signed mutation logs: tracking <span class="highlight">who</span>,
          <span class="highlight">what</span>, <span class="highlight">when</span>, client origin IP,
          and zero-trust policy enforcement across all cluster tenants.
        </p>
      </div>

      <div class="audit-header-actions">
        <span class="audit-live-badge" :class="{ 'live-active': isLiveTailing }">
          <span>{{ isLiveTailing ? '● STREAMING' : '○ STANDBY' }}</span>
        </span>
        <button class="btn btn-secondary" :disabled="isLoading" @click="fetchLogs">
          <span>{{ isLoading ? '⏳ Syncing...' : '🔄 Refresh Trail' }}</span>
        </button>
        <button class="btn btn-primary" :disabled="isTriggeringScan" @click="triggerAuditScan">
          <span>{{ isTriggeringScan ? '⚡ Scanning...' : '⚡ Trigger Audit Scan' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="audit-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🛡️ Audit Trail ({{ filteredLogs.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="isTriggeringScan"
          title="Trigger Audit Scan"
          aria-label="Trigger Audit Scan"
          @click="triggerAuditScan"
        >
          <span>⚡</span>
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="isLoading"
          title="Refresh Trail"
          aria-label="Refresh Trail"
          @click="fetchLogs"
        >
          <span>🔄</span>
        </button>
        <button
          class="btn-icon-cmd"
          :class="{ active: isLiveTailing }"
          title="Toggle Live Tail"
          aria-label="Toggle Live Tail"
          @click="toggleLiveTail"
        >
          <span>🔴</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="audit-micro-telemetry mobile-only font-mono" role="status" aria-label="Audit Micro Telemetry">
      <span class="tel-item tel-events">🛡️ {{ filteredLogs.length }} evts</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-signed">⚡ {{ kpiMetrics.signedPercentage }}% signed</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-viol">⚠️ {{ kpiMetrics.securityViolations }} viol</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-trust">🔒 Zero-Trust</span>
    </div>

    <!-- Notification Banner -->
    <div
      v-if="statusMessage"
      class="audit-banner animate-fade-in"
      :class="statusMessage.type === 'success' ? 'banner-success' : 'banner-error'"
    >
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- 1. HUD Metric Cards -->
    <AuditHudCards class="desktop-only" :metrics="kpiMetrics" :loading="isLoading" />

    <!-- 2. Filter & Query Toolbar -->
    <AuditFilterToolbar
      v-model:search-query="searchQuery"
      v-model:selected-action-type="selectedActionType"
      v-model:selected-severity="selectedSeverity"
      v-model:selected-actor="selectedActor"
      v-model:date-range="dateRange"
      :unique-actors="uniqueActors"
      :action-types="actionTypeFilters"
      :is-live-tailing="isLiveTailing"
      @toggle-live-tail="toggleLiveTail"
      @export-json="exportToJson"
      @export-csv="exportToCsv"
      @reset-filters="resetFilters"
    />

    <!-- 3. Desktop Audit Trail Table -->
    <div class="audit-desktop-view">
      <AuditTrailTable
        :logs="filteredLogs"
        :loading="isLoading"
        :error="error"
        @select-payload="openPayloadDrawer"
      />
    </div>

    <!-- 4. High-Density Mobile Card Stream (~65px/item, 0 horizontal scroll) -->
    <div class="audit-mobile-view">
      <AuditMobileCards
        :logs="filteredLogs"
        :loading="isLoading"
        @select-payload="openPayloadDrawer"
      />
    </div>

    <!-- 5. Audit Event Payload Drawer & JSON Inspector -->
    <AuditPayloadDrawer
      :event="selectedEvent"
      :open="isDrawerOpen"
      @close="closePayloadDrawer"
    />
  </div>
</template>

<script setup lang="ts">
import '../assets/styles/views/audit.css'
import { useAuditLogs } from '../composables/useAuditLogs'
import AuditHudCards from '../components/audit/AuditHudCards.vue'
import AuditTrailTable from '../components/audit/AuditTrailTable.vue'
import AuditFilterToolbar from '../components/audit/AuditFilterToolbar.vue'
import AuditMobileCards from '../components/audit/AuditMobileCards.vue'
import AuditPayloadDrawer from '../components/audit/AuditPayloadDrawer.vue'

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
