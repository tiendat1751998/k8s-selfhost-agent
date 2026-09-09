<script setup lang="ts">
import MetricCard from '../components/ui/MetricCard.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import ServiceCatalogTable from '../components/catalog/ServiceCatalogTable.vue'
import ServiceCatalogGrid from '../components/catalog/ServiceCatalogGrid.vue'
import ServiceCatalogMobileCards from '../components/catalog/ServiceCatalogMobileCards.vue'
import ServiceDetailDrawer from '../components/catalog/ServiceDetailDrawer.vue'
import RegisterServiceModal from '../components/catalog/RegisterServiceModal.vue'
import { useServiceCatalog } from '../composables/useServiceCatalog'

const {
  loading, deleting, saving, error, toastMessage, services, stats, viewMode,
  filter, showFormModal, modalMode, showDetailDrawer, showDeleteModal,
  selectedService, serviceToDelete, copiedKey, form, formErrors,
  serviceTypes, lifecycles, columns, totalServices, prodCount, devCount, deprecatedCount,
  selectedServiceDependencies, showMobileFilters, activeFilterCount,
  fetchCatalogData, resetFilters,
  getTypeBadgeClass, getTypeIcon, getLifecycleBadgeClass, getLifecycleDotClass,
  openDetailDrawer, openCreateModal, openEditModal, addAnnotationRow,
  removeAnnotationRow, addPresetAnnotation, handleSaveService, promptDelete,
  handleConfirmDelete, handleDeploy, handleConfig, copyToClipboard, formatDate
} = useServiceCatalog()
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Desktop View Header -->
    <div class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>BACKSTAGE-INSPIRED DEVELOPER PORTAL</span>
        </div>
        <h1 class="view-title">Service Catalog</h1>
        <p class="view-desc">
          Centralized software ecosystem registry. Discover, govern, and explore microservices, APIs, libraries, and cloud infrastructure components.
        </p>
      </div>

      <div class="header-actions">
        <!-- View Mode Switcher -->
        <div class="view-mode-toggle" title="Switch layout display">
          <button type="button" class="mode-btn" :class="{ active: viewMode === 'table' }" @click="viewMode = 'table'" title="Table View">📋 Table</button>
          <button type="button" class="mode-btn" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'" title="Grid View">▦ Grid</button>
          <button type="button" class="mode-btn" :class="{ active: viewMode === 'mobile' }" @click="viewMode = 'mobile'" title="Stream View">📱 Stream</button>
        </div>

        <button type="button" class="btn btn-secondary" :disabled="loading" @click="fetchCatalogData" title="Refresh catalog list & stats">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh' }}</span>
        </button>
        <router-link to="/scaffolder" class="btn btn-secondary" title="Deploy a new service from template">
          <span>🪄 Scaffolder</span>
        </router-link>
        <button type="button" class="btn btn-primary" @click="openCreateModal" title="Register a new service">
          <span>+ Register Service</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="catalog-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">📦 Catalog ({{ totalServices }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          type="button"
          class="btn-icon-cmd"
          title="Register Service"
          aria-label="Register Service"
          @click="openCreateModal"
        >
          <span>➕</span>
        </button>
        <button
          type="button"
          class="btn-icon-cmd"
          title="Refresh"
          aria-label="Refresh"
          :disabled="loading"
          @click="fetchCatalogData"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="catalog-micro-telemetry mobile-only font-mono" role="status" aria-label="Catalog Micro Telemetry">
      <span class="tel-item tel-total">📦 {{ totalServices }} svcs</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-prod">🚀 {{ prodCount }} prod</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-dev">🧪 {{ devCount }} dev</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-depr">⚠️ {{ deprecatedCount }} sunset</span>
    </div>

    <!-- Notification Toast Banner -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span class="toast-icon">{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="toast-text">{{ toastMessage.text }}</span>
      <button type="button" class="toast-close" @click="toastMessage = null" aria-label="Dismiss">✕</button>
    </div>

    <!-- Metric HUD (Desktop Only) -->
    <div class="metrics-grid desktop-metrics desktop-only">
      <MetricCard title="Total Services" :value="totalServices" subtitle="Registered ecosystem components" icon="📦" badge="CATALOG" badge-color="cyan" />
      <MetricCard title="Production" :value="prodCount" subtitle="Live production tier services" icon="🚀" badge="LIVE" badge-color="emerald" />
      <MetricCard title="Development" :value="devCount" subtitle="Active staging & development builds" icon="🧪" badge="DEV" badge-color="amber" />
      <MetricCard title="Deprecated" :value="deprecatedCount" subtitle="Sunsetting / pending decommission" icon="⚠️" badge="SUNSET" badge-color="rose" />
    </div>

    <!-- Filter Bar: Compact 32px Bar on Mobile, Full on Desktop -->
    <div class="filter-bar glass-panel" :class="{ 'mobile-filters-open': showMobileFilters }">
      <div class="filter-row">
        <!-- Search Input -->
        <div class="filter-group search-group">
          <label class="filter-label desktop-only" for="catalog-search">Search</label>
          <div class="search-input-wrap">
            <span class="search-icon">🔍</span>
            <input
              id="catalog-search"
              v-model="filter.search"
              type="text"
              class="input-glass search-field"
              placeholder="Search name, description, tags..."
              @keydown.enter="fetchCatalogData"
            />
            <button
              v-if="filter.search"
              type="button"
              class="clear-input-btn"
              @click="filter.search = ''; fetchCatalogData()"
              aria-label="Clear search"
            >
              ✕
            </button>
          </div>
        </div>

        <!-- Mobile Filter Toggle Button (32px standard height) -->
        <button
          type="button"
          class="btn btn-secondary btn-sm mobile-only btn-filter-toggle"
          :class="{ active: showMobileFilters || activeFilterCount > 0 }"
          @click="showMobileFilters = !showMobileFilters"
          aria-label="Toggle filter options"
        >
          <span>🌪️ Filters</span>
          <span v-if="activeFilterCount > 0" class="badge-filter-count">{{ activeFilterCount }}</span>
        </button>

        <!-- Secondary Filter Controls (Inline on Desktop, Expandable on Mobile) -->
        <div class="filter-secondary-group" :class="{ 'is-open': showMobileFilters }">
          <div class="filter-group select-group">
            <label class="filter-label" for="filter-type">Type</label>
            <select id="filter-type" v-model="filter.type" class="input-glass filter-select" @change="fetchCatalogData">
              <option value="">All Types ({{ totalServices }})</option>
              <option v-for="t in serviceTypes" :key="t.value" :value="t.value">{{ t.icon }} {{ t.label }} ({{ stats.by_type[t.value] || 0 }})</option>
            </select>
          </div>

          <div class="filter-group select-group">
            <label class="filter-label" for="filter-lifecycle">Lifecycle</label>
            <select id="filter-lifecycle" v-model="filter.lifecycle" class="input-glass filter-select" @change="fetchCatalogData">
              <option value="">All Lifecycles</option>
              <option v-for="l in lifecycles" :key="l.value" :value="l.value">{{ l.icon }} {{ l.label }} ({{ stats.by_lifecycle[l.value] || 0 }})</option>
            </select>
          </div>

          <div class="filter-group owner-group">
            <label class="filter-label" for="filter-owner">Owner Team</label>
            <div class="search-input-wrap">
              <input id="filter-owner" v-model="filter.owner_team" type="text" class="input-glass owner-field" placeholder="e.g. platform-team" @keydown.enter="fetchCatalogData" />
              <button v-if="filter.owner_team" type="button" class="clear-input-btn" @click="filter.owner_team = ''; fetchCatalogData()" aria-label="Clear owner">✕</button>
            </div>
          </div>

          <div class="filter-actions">
            <button type="button" class="btn btn-secondary btn-sm" title="Apply filters" @click="fetchCatalogData"><span>Apply</span></button>
            <button type="button" class="btn btn-secondary btn-sm" title="Reset all filters" @click="resetFilters"><span>↺ Clear</span></button>
          </div>
        </div>
      </div>
    </div>

    <!-- Desktop Presentation Views -->
    <div class="desktop-view">
      <ServiceCatalogTable
        v-if="viewMode === 'table'"
        :services="services"
        :columns="columns"
        :loading="loading"
        :error="error"
        :get-type-icon="getTypeIcon"
        :get-type-badge-class="getTypeBadgeClass"
        :get-lifecycle-badge-class="getLifecycleBadgeClass"
        :get-lifecycle-dot-class="getLifecycleDotClass"
        @open-detail="openDetailDrawer"
        @deploy="handleDeploy"
        @config="handleConfig"
        @delete="promptDelete"
      />

      <ServiceCatalogGrid
        v-else-if="viewMode === 'grid'"
        :services="services"
        :loading="loading"
        :get-type-icon="getTypeIcon"
        :get-type-badge-class="getTypeBadgeClass"
        :get-lifecycle-badge-class="getLifecycleBadgeClass"
        :get-lifecycle-dot-class="getLifecycleDotClass"
        @open-detail="openDetailDrawer"
        @deploy="handleDeploy"
        @config="handleConfig"
        @delete="promptDelete"
      />

      <ServiceCatalogMobileCards
        v-else
        :services="services"
        :loading="loading"
        :get-type-icon="getTypeIcon"
        :get-type-badge-class="getTypeBadgeClass"
        :get-lifecycle-badge-class="getLifecycleBadgeClass"
        :get-lifecycle-dot-class="getLifecycleDotClass"
        @open-detail="openDetailDrawer"
        @deploy="handleDeploy"
        @config="handleConfig"
        @delete="promptDelete"
      />
    </div>

    <!-- Mobile Presentation View: Touch-Optimized Stream (<768px) -->
    <div class="mobile-view">
      <ServiceCatalogMobileCards
        :services="services"
        :loading="loading"
        :get-type-icon="getTypeIcon"
        :get-type-badge-class="getTypeBadgeClass"
        :get-lifecycle-badge-class="getLifecycleBadgeClass"
        :get-lifecycle-dot-class="getLifecycleDotClass"
        @open-detail="openDetailDrawer"
        @deploy="handleDeploy"
        @config="handleConfig"
        @delete="promptDelete"
      />
    </div>

    <!-- DETAIL DRAWER -->
    <ServiceDetailDrawer
      v-model:show="showDetailDrawer"
      :service="selectedService"
      :dependencies="selectedServiceDependencies"
      :copied-key="copiedKey"
      :get-type-icon="getTypeIcon"
      :get-type-badge-class="getTypeBadgeClass"
      :get-lifecycle-badge-class="getLifecycleBadgeClass"
      :get-lifecycle-dot-class="getLifecycleDotClass"
      :format-date="formatDate"
      @edit="openEditModal"
      @delete="promptDelete"
      @deploy="handleDeploy"
      @copy="copyToClipboard"
    />

    <!-- REGISTER / EDIT SERVICE MODAL -->
    <RegisterServiceModal
      v-model:show="showFormModal"
      :mode="modalMode"
      :form="form"
      :form-errors="formErrors"
      :saving="saving"
      :service-types="serviceTypes"
      :lifecycles="lifecycles"
      @save="handleSaveService"
      @add-annotation="addAnnotationRow"
      @remove-annotation="removeAnnotationRow"
      @preset-annotation="addPresetAnnotation"
    />

    <!-- DELETE CONFIRMATION MODAL -->
    <ModalDrawer
      v-model:show="showDeleteModal"
      mode="modal"
      title="Delete Service Registration"
      subtitle="Are you sure you want to remove this service from the catalog?"
      max-width="480px"
    >
      <div v-if="serviceToDelete" class="delete-modal-content">
        <div class="delete-warning-box">
          <span class="warning-icon">⚠️</span>
          <div>
            <strong>This action will unregister the service from the catalog.</strong>
            <p>
              Service: <strong class="font-mono text-cyan">{{ serviceToDelete.name }}</strong> ({{ serviceToDelete.id }})
            </p>
          </div>
        </div>
        <p class="delete-note text-muted">
          Note: This only removes the portal catalog entry. The underlying Kubernetes pods, deployments, and repositories will not be modified.
        </p>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="deleting" @click="close">Cancel</button>
        <button type="button" class="btn btn-danger-crimson" :disabled="deleting" @click="handleConfirmDelete">
          <span>{{ deleting ? '🗑️ Deleting...' : 'Confirm Delete' }}</span>
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style>
@import '../assets/styles/views/catalog.css';
</style>
