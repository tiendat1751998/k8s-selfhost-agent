<script setup lang="ts">
import { ref } from 'vue'
import '../assets/styles/views/drift.css'
import '../assets/styles/components/drift-drawers.css'
import { useDriftDetection } from '../composables/useDriftDetection'
import DriftHudCards from '../components/drift/DriftHudCards.vue'
import DriftResourcesTable from '../components/drift/DriftResourcesTable.vue'
import DriftMobileCards from '../components/drift/DriftMobileCards.vue'
import DriftDiffDrawer from '../components/drift/DriftDiffDrawer.vue'

const isFilterOpen = ref(false)

const {
  filteredDrifts,
  loading,
  error,
  activeStatus,
  clusterFilter,
  resolvingId,
  selectedDrift,
  diffViewMode,
  statusMessage,
  remediatedTodayCount,
  driftedCount,
  criticalCount,
  gitReposTracked,
  statusFilters,
  fetchDriftData,
  handleReconcile,
  handleBatchReconcile,
  toggleDriftSuppression,
  openDiff,
  closeDiff,
  setDiffViewMode,
} = useDriftDetection()
</script>

<template>
  <div class="view-container">
    <!-- Desktop View Header -->
    <header class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>GITOPS CONTINUOUS RECONCILIATION</span>
        </div>
        <h1 class="view-title">Configuration Drift Detection & Auto-Reconcile</h1>
        <p class="view-desc">
          Continuous cryptographic state comparison between Git repository manifests (<span class="highlight">Desired State</span>) and Kubernetes live etcd runtime (<span class="highlight">Actual State</span>).
        </p>
      </div>

      <div class="header-actions">
        <button 
          v-if="criticalCount > 0"
          class="btn btn-warning" 
          :disabled="loading" 
          title="Auto-reconcile all critical drifted workloads"
          @click="handleBatchReconcile(true)"
        >
          <span>⚡ Sync Critical ({{ criticalCount }})</span>
        </button>
        <button 
          class="btn btn-secondary" 
          :disabled="loading" 
          @click="fetchDriftData"
        >
          <span>{{ loading ? '⏳ Scanning...' : '🔄 Scan Cluster Drift' }}</span>
        </button>
      </div>
    </header>

    <!-- Mobile 44px Command Bar (<768px) -->
    <div class="drift-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🎯 Drift ({{ filteredDrifts.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-cmd-filter"
          :class="{ active: isFilterOpen }"
          title="Toggle Filters"
          aria-label="Toggle Filters"
          @click="isFilterOpen = !isFilterOpen"
        >
          <span>⚙️ Filters</span>
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Sync Critical"
          aria-label="Sync Critical"
          @click="handleBatchReconcile(true)"
        >
          <span>⚡</span>
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Scan Drift"
          aria-label="Scan Drift"
          @click="fetchDriftData"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="drift-micro-telemetry mobile-only font-mono" role="status" aria-label="Drift Micro Telemetry">
      <span class="tel-item tel-drifted">🎯 {{ driftedCount }} drift</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-crit">🔥 {{ criticalCount }} crit</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-sync">✅ {{ remediatedTodayCount }} sync</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-repos">📦 {{ gitReposTracked }} repos</span>
    </div>

    <!-- Mobile Collapsible Filter Drawer (<768px) -->
    <transition name="accordion">
      <div v-if="isFilterOpen" class="drift-mobile-filter-drawer mobile-only glass-panel animate-fade-in">
        <div class="mobile-filter-content">
          <div class="mobile-filter-section">
            <span class="mobile-filter-title">Filter Status:</span>
            <div class="mobile-filter-pills">
              <button 
                v-for="st in statusFilters" 
                :key="st.key"
                class="filter-pill"
                :class="[st.badgeClass, { 'filter-active': activeStatus === st.key }]"
                @click="activeStatus = st.key"
              >
                <span>{{ st.label }} ({{ st.count }})</span>
              </button>
            </div>
          </div>
          <div class="mobile-filter-section">
            <span class="mobile-filter-title">Cluster:</span>
            <select v-model="clusterFilter" class="input-glass filter-select-mobile" @change="fetchDriftData">
              <option value="">All Clusters</option>
              <option value="primary">primary</option>
              <option value="edge-node-01">edge-node-01</option>
            </select>
          </div>
        </div>
      </div>
    </transition>

    <!-- Notification Banner -->
    <div 
      v-if="statusMessage" 
      class="status-banner animate-fade-in" 
      :class="'banner-' + statusMessage.type"
      role="alert"
    >
      <span class="banner-icon">
        {{ statusMessage.type === 'success' ? '✅' : statusMessage.type === 'info' ? 'ℹ️' : '⚠️' }}
      </span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" aria-label="Dismiss alert" @click="statusMessage = null">✕</button>
    </div>

    <!-- Metric HUD Cards -->
    <DriftHudCards
      class="desktop-only"
      :drifted-count="driftedCount"
      :critical-count="criticalCount"
      :remediated-today-count="remediatedTodayCount"
      :git-repos-tracked="gitReposTracked"
    />

    <!-- Desktop Filter Bar -->
    <div class="filter-bar glass-panel desktop-only">
      <div class="filter-group">
        <span class="filter-label">Filter Status:</span>
        <button 
          v-for="st in statusFilters" 
          :key="st.key"
          class="filter-pill"
          :class="[st.badgeClass, { 'filter-active': activeStatus === st.key }]"
          @click="activeStatus = st.key"
        >
          <span>{{ st.label }} ({{ st.count }})</span>
        </button>
      </div>

      <div class="filter-group">
        <span class="filter-label">Cluster:</span>
        <select v-model="clusterFilter" class="input-glass filter-select" @change="fetchDriftData">
          <option value="">All Clusters</option>
          <option value="primary">primary</option>
          <option value="edge-node-01">edge-node-01</option>
        </select>
      </div>
    </div>

    <!-- Workloads View: Desktop Table -->
    <div class="drift-desktop-view">
      <DriftResourcesTable
        :drifts="filteredDrifts"
        :loading="loading"
        :error="error"
        :resolving-id="resolvingId"
        @inspect="openDiff"
        @sync="handleReconcile"
        @suppress="toggleDriftSuppression"
      />
    </div>

    <!-- Workloads View: Mobile Cards Stream (~65px/item, 0 horizontal scroll) -->
    <div class="drift-mobile-view">
      <DriftMobileCards
        :drifts="filteredDrifts"
        :loading="loading"
        :resolving-id="resolvingId"
        @inspect="openDiff"
        @sync="handleReconcile"
      />
    </div>

    <!-- Diff Inspection Drawer / Modal -->
    <DriftDiffDrawer
      :drift="selectedDrift"
      :resolving="resolvingId === selectedDrift?.id"
      :diff-mode="diffViewMode"
      @close="closeDiff"
      @reconcile="handleReconcile"
      @suppress="toggleDriftSuppression"
      @update:diff-mode="setDiffViewMode"
    />
  </div>
</template>
