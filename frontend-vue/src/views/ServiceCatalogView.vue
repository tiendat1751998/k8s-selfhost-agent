<script setup lang="ts">
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import ServiceCatalogTable from '../components/catalog/ServiceCatalogTable.vue'
import ServiceCatalogGrid from '../components/catalog/ServiceCatalogGrid.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import ServiceCatalogMobileCards from '../components/catalog/ServiceCatalogMobileCards.vue'
import ServiceDetailDrawer from '../components/catalog/ServiceDetailDrawer.vue'
import RegisterServiceModal from '../components/catalog/RegisterServiceModal.vue'
import { useServiceCatalog } from '../composables/useServiceCatalog'

const {
  loading, deleting, saving, error, toastMessage, services, stats, viewMode,
  filter, showFormModal, modalMode, showDetailDrawer, showDeleteModal,
  selectedService, serviceToDelete, copiedKey, form, formErrors,
  serviceTypes, lifecycles, columns, totalServices, prodCount, devCount, teams,
  selectedServiceDependencies, showMobileFilters, activeFilterCount,
  fetchCatalogData, resetFilters,
  getTypeBadgeClass, getTypeIcon, getLifecycleBadgeClass, getLifecycleDotClass,
  openDetailDrawer, openCreateModal, openEditModal, addAnnotationRow,
  removeAnnotationRow, addPresetAnnotation, handleSaveService, promptDelete,
  handleConfirmDelete, handleDeploy, handleConfig, copyToClipboard, formatDate
} = useServiceCatalog()

let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null
function handleSearchInput() {
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  searchDebounceTimer = setTimeout(() => {
    fetchCatalogData()
  }, 250)
}

function handleClearSearch() {
  filter.search = ''
  fetchCatalogData()
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Mobile 40-44px Command Bar (<768px) -->
    <div class="catalog-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="box" size="xs" /> Catalog ({{ totalServices }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          type="button"
          class="btn-icon-cmd"
          title="Register Service"
          aria-label="Register Service"
          @click="openCreateModal"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          type="button"
          class="btn-icon-cmd"
          title="Sync / Refresh"
          aria-label="Sync"
          :disabled="loading"
          @click="fetchCatalogData"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="catalog-micro-telemetry mobile-only font-mono" role="status" aria-label="Catalog Micro Telemetry">
      <span class="tel-item tel-total"><BaseIcon name="box" size="xs" /> {{ totalServices }} Services</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-prod"><BaseIcon name="check-circle" size="xs" /> {{ prodCount }} Healthy</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-api"><BaseIcon name="zap" size="xs" /> {{ stats.by_type['api'] || 0 }} APIs</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-teams"><BaseIcon name="users" size="xs" /> {{ teams.length }} Teams</span>
    </div>

    <!-- Notification Toast Banner -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="toast-icon" />
      <span class="toast-text">{{ toastMessage.text }}</span>
      <button type="button" class="toast-close" @click="toastMessage = null" aria-label="Dismiss"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Unified 38px Enterprise Catalog Toolbar -->
    <div class="catalog-toolbar-sleek glass-panel">
      <!-- Search input with search icon and clear button (filters by name, owner team, tags) -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          id="catalog-search"
          v-model="filter.search"
          type="text"
          class="input-glass toolbar-search-input"
          placeholder="Search name, owner team, tags..."
          @input="handleSearchInput"
          @keydown.enter="fetchCatalogData"
        />
        <button
          v-if="filter.search"
          type="button"
          class="clear-input-btn"
          aria-label="Clear search"
          @click="handleClearSearch"
        >
          <BaseIcon name="x" size="xs" />
        </button>
      </div>

      <!-- Type select dropdown (All Types, Service, Library, API, etc.) -->
      <select
        id="filter-type"
        v-model="filter.type"
        class="input-glass toolbar-select desktop-only"
        aria-label="Filter by Type"
        @change="fetchCatalogData"
      >
        <option value="">All Types ({{ totalServices }})</option>
        <option v-for="t in serviceTypes" :key="t.value" :value="t.value">
          {{ t.label }} ({{ stats.by_type[t.value] || 0 }})
        </option>
      </select>

      <!-- Lifecycle select dropdown (All Lifecycles, Production, Development, Deprecated) -->
      <select
        id="filter-lifecycle"
        v-model="filter.lifecycle"
        class="input-glass toolbar-select desktop-only"
        aria-label="Filter by Lifecycle"
        @change="fetchCatalogData"
      >
        <option value="">All Lifecycles</option>
        <option v-for="l in lifecycles" :key="l.value" :value="l.value">
          {{ l.label }} ({{ stats.by_lifecycle[l.value] || 0 }})
        </option>
      </select>

      <!-- Mobile Filter Toggle Button (<768px) -->
      <button
        type="button"
        class="btn btn-secondary btn-sm mobile-only btn-filter-toggle"
        :class="{ active: showMobileFilters || activeFilterCount > 0 }"
        aria-label="Toggle filter options"
        @click="showMobileFilters = !showMobileFilters"
      >
        <BaseIcon name="filter" size="xs" /> <span>Filters</span>
        <span v-if="activeFilterCount > 0" class="badge-filter-count">{{ activeFilterCount }}</span>
      </button>

      <!-- Inline compact KPI badge strip font-mono: e.g. {{ totalServices }} Services ({{ prodCount }} Prod · {{ devCount }} Dev) -->
      <div class="toolbar-kpi-strip font-mono desktop-only" role="status" aria-label="Catalog summary metrics">
        <span class="kpi-badge font-mono">{{ totalServices }} Services ({{ prodCount }} Prod · {{ devCount }} Dev)</span>
      </div>

      <!-- Right: View Mode Toggle + Action Buttons -->
      <div class="toolbar-actions-group">
        <!-- View mode toggle (Table / Grid) -->
        <div class="view-mode-toggle desktop-only" title="Switch layout display">
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'table' }"
            title="Table View"
            @click="viewMode = 'table'"
          >
            <BaseIcon name="file-text" size="xs" /> <span>Table</span>
          </button>
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'grid' }"
            title="Grid View"
            @click="viewMode = 'grid'"
          >
            <BaseIcon name="grid" size="xs" /> <span>Grid</span>
          </button>
        </div>

        <router-link
          to="/scaffolder"
          class="btn btn-secondary toolbar-btn desktop-only"
          title="Deploy a new service from template"
        >
          <BaseIcon name="sparkles" size="xs" /> <span>Scaffolder</span>
        </router-link>

        <button
          type="button"
          class="btn btn-primary toolbar-btn desktop-only"
          title="Register a new service"
          @click="openCreateModal"
        >
          <span>+ Register Service</span>
        </button>

        <button
          type="button"
          class="btn btn-secondary toolbar-btn"
          :disabled="loading"
          title="Refresh catalog list & stats"
          @click="fetchCatalogData"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
          <span class="desktop-only">{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile Secondary Filters Drawer (Expandable on <768px) -->
    <div v-if="showMobileFilters" class="mobile-filter-drawer glass-panel mobile-only animate-fade-in">
      <div class="mobile-filter-row">
        <label class="filter-label" for="mobile-filter-type">Type</label>
        <select
          id="mobile-filter-type"
          v-model="filter.type"
          class="input-glass toolbar-select"
          @change="fetchCatalogData"
        >
          <option value="">All Types ({{ totalServices }})</option>
          <option v-for="t in serviceTypes" :key="t.value" :value="t.value">
            {{ t.label }} ({{ stats.by_type[t.value] || 0 }})
          </option>
        </select>
      </div>

      <div class="mobile-filter-row">
        <label class="filter-label" for="mobile-filter-lifecycle">Lifecycle</label>
        <select
          id="mobile-filter-lifecycle"
          v-model="filter.lifecycle"
          class="input-glass toolbar-select"
          @change="fetchCatalogData"
        >
          <option value="">All Lifecycles</option>
          <option v-for="l in lifecycles" :key="l.value" :value="l.value">
            {{ l.label }} ({{ stats.by_lifecycle[l.value] || 0 }})
          </option>
        </select>
      </div>

      <div class="mobile-filter-actions">
        <button type="button" class="btn btn-secondary btn-sm" @click="resetFilters"><span>↺ Clear</span></button>
        <button type="button" class="btn btn-primary btn-sm" @click="showMobileFilters = false"><span>Done</span></button>
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
          <BaseIcon name="alert-triangle" size="xs" class="warning-icon" />
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
          <BaseIcon :name="deleting ? 'clock' : 'trash'" size="xs" /> <span>{{ deleting ? 'Deleting...' : 'Confirm Delete' }}</span>
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style>
@import '../assets/styles/views/catalog.css';
@import '../assets/styles/components/catalog-drawers.css';
</style>
