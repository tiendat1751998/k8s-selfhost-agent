<script setup lang="ts">
import EcosystemGrid from '../components/ecosystem/EcosystemGrid.vue'
import EcosystemTable from '../components/ecosystem/EcosystemTable.vue'
import EcosystemMobileCards from '../components/ecosystem/EcosystemMobileCards.vue'
import ConnectIntegrationModal from '../components/ecosystem/ConnectIntegrationModal.vue'
import IntegrationHealthDrawer from '../components/ecosystem/IntegrationHealthDrawer.vue'
import { useEcosystem } from '../composables/useEcosystem'

const {
  loading, scanning, saving, deleting, syncing, error, toastMessage, viewMode,
  tools, summary, activeCategory, searchQuery, selectedStatus, categories, presets,
  showConnectModal, connectForm, showHealthDrawer, selectedToolForHealth,
  healthProbeResult, isProbing, webhookHistory, errorLogs, filteredTools,
  getToolIcon, formatRelativeTime, loadData, handleScan, openConnectModal,
  closeConnectModal, handleCreateTool, handleDeleteTool, handleSyncWebhook,
  openHealthDrawer, closeHealthDrawer, runHealthProbe
} = useEcosystem()
</script>

<template>
  <div class="ecosystem-view">
    <!-- Toast Notification -->
    <transition name="fade">
      <div v-if="toastMessage" class="toast-popup" :class="toastMessage.type">
        <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="sm" />
        <span>{{ toastMessage.text }}</span>
      </div>
    </transition>

    <!-- Mobile Command Bar (<768px) -->
    <div class="ecosystem-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="globe" size="sm" /> Ecosystem ({{ summary.total }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="scanning || loading"
          title="Sync"
          aria-label="Sync"
          @click="handleScan"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': scanning }" />
        </button>
        <button
          class="btn-icon-cmd"
          title="Connect"
          aria-label="Connect"
          @click="openConnectModal()"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile Micro-Telemetry Strip (<768px) -->
    <div class="ecosystem-micro-telemetry mobile-only font-mono" role="status" aria-label="Ecosystem Micro Telemetry">
      <span class="tel-item tel-total"><BaseIcon name="globe" size="xs" /> {{ summary.total }} Total</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-connected"><BaseIcon name="check-circle" size="xs" /> {{ summary.healthy }} Connected</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-latency"><BaseIcon name="zap" size="xs" /> {{ healthProbeResult?.latencyMs ? `${healthProbeResult.latencyMs}ms` : '<45ms' }} Latency</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-issues"><BaseIcon name="alert-triangle" size="xs" /> {{ summary.degraded }} Issues</span>
    </div>

    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="ecosystem-toolbar-sleek glass-panel">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search tools, endpoints, versions..."
          class="toolbar-search-input"
          aria-label="Search tools, endpoints, versions"
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

      <!-- Status select dropdown (All Statuses, Healthy, Degraded, Unknown) -->
      <select
        v-model="selectedStatus"
        class="toolbar-select"
        aria-label="Filter by health status"
      >
        <option value="all">All Statuses</option>
        <option value="healthy">Healthy</option>
        <option value="degraded">Degraded</option>
        <option value="not_configured">Unknown</option>
      </select>

      <!-- Inline compact KPI badge strip font-mono -->
      <div class="toolbar-kpi-strip font-mono desktop-only" role="status" aria-label="Ecosystem summary metrics">
        <span class="kpi-badge font-mono">{{ summary.total }} Tools ({{ summary.healthy }} Healthy · {{ summary.degraded }} Issues)</span>
      </div>

      <!-- Right: View Mode Toggle & Action Buttons -->
      <div class="toolbar-actions-group">
        <!-- View mode toggle (Table / Grid) -->
        <div class="view-mode-toggle desktop-only" title="Switch layout display">
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'table' }"
            title="Table View"
            aria-label="Table View"
            @click="viewMode = 'table'"
          >
            <BaseIcon name="file-text" size="xs" />
            <span>Table</span>
          </button>
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'grid' }"
            title="Grid View"
            aria-label="Grid View"
            @click="viewMode = 'grid'"
          >
            <BaseIcon name="grid" size="xs" />
            <span>Grid</span>
          </button>
        </div>

        <!-- Action buttons: Scan Now and + Register Tool -->
        <button
          type="button"
          class="btn-toolbar btn-secondary desktop-only"
          :disabled="scanning || loading"
          title="Run discovery scan"
          @click="handleScan"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': scanning }" />
          <span>{{ scanning ? 'Scanning...' : 'Scan Now' }}</span>
        </button>

        <button
          type="button"
          class="btn-toolbar btn-primary desktop-only"
          title="Register new ecosystem integration"
          @click="openConnectModal()"
        >
          <BaseIcon name="plus" size="xs" />
          <span>Register Tool</span>
        </button>
      </div>
    </div>

    <!-- Category Filter Pills Navigation Bar -->
    <section class="category-pills-bar category-tabs-container">
      <div class="category-pills category-tabs">
        <button
          v-for="cat in categories"
          :key="cat.key"
          type="button"
          class="category-pill-btn category-tab-btn"
          :class="{ active: activeCategory === cat.key }"
          @click="activeCategory = cat.key"
        >
          <BaseIcon :name="cat.icon" size="xs" class="pill-icon" />
          <span class="pill-label tab-label">{{ cat.label }}</span>
          <span v-if="cat.key === 'all'" class="pill-count tab-count">{{ tools.length }}</span>
          <span v-else-if="summary.by_category && summary.by_category[cat.key]" class="pill-count tab-count">
            {{ summary.by_category[cat.key] }}
          </span>
        </button>
      </div>
    </section>

    <!-- Loading State -->
    <div v-if="loading && tools.length === 0" class="loading-state glass-panel">
      <BaseIcon name="refresh" size="md" class="spin-anim" />
      <p>Discovering ecosystem components...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error && tools.length === 0" class="error-state glass-panel">
      <BaseIcon name="alert-triangle" size="md" class="error-icon" />
      <p>{{ error }}</p>
      <button class="btn-secondary" @click="loadData">Try Again</button>
    </div>

    <!-- Empty State -->
    <div v-else-if="filteredTools.length === 0" class="empty-state glass-panel">
      <BaseIcon name="search" size="xl" class="empty-icon" />
      <h3>No ecosystem tools found</h3>
      <p>No tools matched your current filters. Try changing category or clicking "Scan Now".</p>
      <button class="btn-primary" @click="handleScan">Run Scan</button>
    </div>

    <!-- Data Presentation -->
    <template v-else>
      <EcosystemTable
        v-if="viewMode === 'table'"
        class="desktop-only"
        :tools="filteredTools"
        :deleting-id="deleting"
        :syncing-id="syncing"
        :get-tool-icon="getToolIcon"
        :format-relative-time="formatRelativeTime"
        @inspect-health="openHealthDrawer"
        @configure="openConnectModal"
        @sync="handleSyncWebhook"
        @delete="handleDeleteTool"
      />

      <EcosystemGrid
        v-else
        class="desktop-only"
        :tools="filteredTools"
        :deleting-id="deleting"
        :syncing-id="syncing"
        :get-tool-icon="getToolIcon"
        :format-relative-time="formatRelativeTime"
        @inspect-health="openHealthDrawer"
        @configure="openConnectModal"
        @sync="handleSyncWebhook"
        @delete="handleDeleteTool"
      />

      <EcosystemMobileCards
        class="mobile-only"
        :tools="filteredTools"
        :deleting-id="deleting"
        :syncing-id="syncing"
        :get-tool-icon="getToolIcon"
        :format-relative-time="formatRelativeTime"
        @inspect-health="openHealthDrawer"
        @configure="openConnectModal"
        @sync="handleSyncWebhook"
        @delete="handleDeleteTool"
      />
    </template>

    <!-- Connect / Configure Modal -->
    <ConnectIntegrationModal
      :show="showConnectModal"
      :form="connectForm"
      :saving="saving"
      :presets="presets"
      @close="closeConnectModal"
      @submit="handleCreateTool"
      @select-preset="openConnectModal"
    />

    <!-- Health Inspection Drawer -->
    <IntegrationHealthDrawer
      :show="showHealthDrawer"
      :tool="selectedToolForHealth"
      :probe-result="healthProbeResult"
      :is-probing="isProbing"
      :webhook-history="webhookHistory"
      :error-logs="errorLogs"
      :get-tool-icon="getToolIcon"
      @close="closeHealthDrawer"
      @probe="runHealthProbe"
      @sync="handleSyncWebhook"
    />
  </div>
</template>

<style>
@import '../assets/styles/views/ecosystem.css';
@import '../assets/styles/components/ecosystem-drawers.css';
</style>
