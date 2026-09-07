<script setup lang="ts">
import MetricCard from '../components/ui/MetricCard.vue'
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
        <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
        <span>{{ toastMessage.text }}</span>
      </div>
    </transition>

    <!-- Header Section -->
    <header class="view-header">
      <div class="header-titles">
        <div class="title-with-badge">
          <h1>Ecosystem Auto-Detector</h1>
          <span class="badge-tag live-badge">Auto-Discovery</span>
        </div>
        <p class="header-subtitle">
          Real-time discovery and operational status for platform infrastructure, service meshes, and GitOps toolchains.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn-secondary" :disabled="scanning || loading" @click="handleScan">
          <span class="btn-icon" :class="{ 'spin-anim': scanning }">🔄</span>
          <span>{{ scanning ? 'Scanning Stack...' : 'Scan Now' }}</span>
        </button>
        <button class="btn-primary" @click="openConnectModal()">
          <span class="btn-icon">➕</span>
          <span>Register Tool</span>
        </button>
      </div>
    </header>

    <!-- Summary HUD Metrics -->
    <section class="summary-hud-grid">
      <MetricCard
        title="Detected Stack Tools"
        :value="summary.total"
        icon="🧩"
        subtitle="Across configured integration URLs"
        badge="Platform"
        badgeColor="cyan"
      />
      <MetricCard
        title="Healthy Services"
        :value="summary.healthy"
        icon="💚"
        subtitle="Responding with status 200 OK"
        badge="Online"
        badgeColor="emerald"
      />
      <MetricCard
        title="Degraded / Unreachable"
        :value="summary.degraded"
        icon="⚠️"
        subtitle="Failed probes or sealed state"
        :badge="summary.degraded > 0 ? 'Attention' : 'Optimal'"
        :badgeColor="summary.degraded > 0 ? 'rose' : 'emerald'"
      />
      <MetricCard
        title="Active Categories"
        :value="Object.keys(summary.by_category || {}).length"
        icon="🏷️"
        subtitle="GitOps, Security, Mesh, Policy, etc."
        badge="Coverage"
        badgeColor="violet"
      />
    </section>

    <!-- Category Tabs Navigation -->
    <section class="category-tabs-container">
      <div class="category-tabs">
        <button
          v-for="cat in categories"
          :key="cat.key"
          class="category-tab-btn"
          :class="{ active: activeCategory === cat.key }"
          @click="activeCategory = cat.key"
        >
          <span class="tab-icon">{{ cat.icon }}</span>
          <span class="tab-label">{{ cat.label }}</span>
          <span v-if="cat.key === 'all'" class="tab-count">{{ tools.length }}</span>
          <span v-else-if="summary.by_category && summary.by_category[cat.key]" class="tab-count">
            {{ summary.by_category[cat.key] }}
          </span>
        </button>
      </div>
    </section>

    <!-- Filter & Search Bar -->
    <section class="filter-toolbar glass-panel">
      <div class="search-box">
        <span class="search-icon">🔍</span>
        <input v-model="searchQuery" type="text" placeholder="Search by tool name, endpoint, version..." class="search-input" />
        <button v-if="searchQuery" class="clear-search-btn" @click="searchQuery = ''">✕</button>
      </div>

      <div class="filter-group">
        <label class="filter-label">Health Status:</label>
        <select v-model="selectedStatus" class="filter-select">
          <option value="all">All Statuses</option>
          <option value="healthy">Healthy Only</option>
          <option value="degraded">Degraded / Unreachable</option>
          <option value="not_configured">Not Configured</option>
        </select>

        <div class="view-mode-toggle">
          <button class="toggle-btn" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'">Grid</button>
          <button class="toggle-btn" :class="{ active: viewMode === 'table' }" @click="viewMode = 'table'">Table</button>
        </div>
      </div>
    </section>

    <!-- Loading State -->
    <div v-if="loading && tools.length === 0" class="loading-state glass-panel">
      <div class="spinner-icon">🔄</div>
      <p>Discovering ecosystem components...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error && tools.length === 0" class="error-state glass-panel">
      <span class="error-icon">⚠️</span>
      <p>{{ error }}</p>
      <button class="btn-secondary" @click="loadData">Try Again</button>
    </div>

    <!-- Empty State -->
    <div v-else-if="filteredTools.length === 0" class="empty-state glass-panel">
      <span class="empty-icon">🔎</span>
      <h3>No ecosystem tools found</h3>
      <p>No tools matched your current filters. Try changing category or clicking "Scan Now".</p>
      <button class="btn-primary" @click="handleScan">Run Scan</button>
    </div>

    <!-- Data Presentation -->
    <template v-else>
      <EcosystemGrid
        v-if="viewMode === 'grid'"
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

      <EcosystemTable
        v-else
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
</style>
