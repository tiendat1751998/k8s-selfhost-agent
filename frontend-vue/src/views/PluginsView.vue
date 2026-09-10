<template>
  <div class="plugins-page">
    <!-- Desktop Header -->
    <header class="page-header glass-panel desktop-only">
      <div class="header-content">
        <div class="title-group">
          <div class="icon-bubble"><BaseIcon name="plug" size="md" /></div>
          <div>
            <div class="badge-row">
              <span class="badge badge-cyan">RUNTIME EXTENSIBILITY</span>
              <span class="badge badge-emerald">HEADLAMP COMPLIANT</span>
              <span class="badge badge-indigo">WASM SANDBOX {{ wasmSandboxStatus.isolationMode.toUpperCase() }}</span>
            </div>
            <h1 class="page-title">Frontend Plugin Hub</h1>
            <p class="page-subtitle">
              Extend platform dashboards, workload views, and telemetry graphs dynamically using isolated custom JavaScript/WASM bundles.
            </p>
          </div>
        </div>
        <div class="header-actions">
          <button class="btn btn-secondary" @click="refreshPlugins" :disabled="loading">
            <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" /> Refresh
          </button>
          <button class="btn btn-primary" @click="openRegisterModal">
            <BaseIcon name="plus" size="xs" /> Register Plugin
          </button>
        </div>
      </div>

      <!-- Stats Overview Cards -->
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-icon"><BaseIcon name="box" size="sm" /></div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.total }}</div>
            <div class="stat-label">Total Plugins</div>
          </div>
        </div>
        <div class="stat-card stat-card-active">
          <div class="stat-icon"><BaseIcon name="zap" size="sm" /></div>
          <div class="stat-info">
            <div class="stat-value text-emerald">{{ stats.enabled }}</div>
            <div class="stat-label">Active & Enabled</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon"><BaseIcon name="pause" size="sm" /></div>
          <div class="stat-info">
            <div class="stat-value text-muted">{{ stats.disabled }}</div>
            <div class="stat-label">Disabled</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon"><BaseIcon name="tag" size="sm" /></div>
          <div class="stat-info">
            <div class="stat-value text-cyan">{{ categoryCount }}</div>
            <div class="stat-label">Categories</div>
          </div>
        </div>
      </div>
    </header>

    <!-- Mobile 44px Command Bar (<768px) -->
    <div class="plugins-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title"><BaseIcon name="plug" size="xs" /> Plugins ({{ stats.enabled }}/{{ plugins.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Install Plugin"
          aria-label="Install Plugin"
          @click="openRegisterModal"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Sync Registry"
          aria-label="Sync Registry"
          @click="refreshPlugins"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="plugins-micro-telemetry mobile-only font-mono" role="status" aria-label="Plugins Micro Telemetry">
      <span class="tel-item tel-installed"><BaseIcon name="plug" size="xs" /> {{ stats.total }} Installed</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-active"><BaseIcon name="zap" size="xs" /> {{ stats.enabled }} Active</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-verified"><BaseIcon name="shield" size="xs" /> {{ wasmSandboxStatus.isolationMode === 'wasm-wasi' ? 'WASM Verified' : 'Verified' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-available"><BaseIcon name="box" size="xs" /> {{ categoryCount }} Available</span>
    </div>

    <!-- Filters & Search Toolbar -->
    <div class="toolbar glass-panel">
      <div class="search-box">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search plugins by name, description, author, or permissions..."
          class="search-input"
        />
        <button v-if="searchQuery" class="clear-btn" @click="searchQuery = ''"><BaseIcon name="x" size="xs" /></button>
      </div>

      <div class="filter-controls">
        <div class="category-tabs">
          <button
            v-for="cat in categories"
            :key="cat.value"
            class="tab-btn"
            :class="{ active: selectedCategory === cat.value }"
            @click="selectedCategory = cat.value"
          >
            <span>{{ cat.icon }}</span> {{ cat.label }}
          </button>
        </div>

        <div class="status-filters">
          <!-- Permission Scopes Filter -->
          <select v-if="availablePermissionScopes.length > 0" v-model="selectedScope" class="select-input">
            <option value="all">All Scopes</option>
            <option v-for="scope in availablePermissionScopes" :key="scope" :value="scope">
              Scope: {{ scope }}
            </option>
          </select>

          <!-- Status Filter -->
          <select v-model="selectedStatus" class="select-input">
            <option value="all">All Status ({{ plugins.length }})</option>
            <option value="enabled">Enabled Only ({{ stats.enabled }})</option>
            <option value="disabled">Disabled Only ({{ stats.disabled }})</option>
          </select>

          <!-- View Mode Toggle (Desktop only) -->
          <div class="view-toggle-group desktop-only">
            <button
              class="view-toggle-btn"
              :class="{ active: viewMode === 'grid' }"
              @click="viewMode = 'grid'"
              title="Grid View"
            >
              ⊞ Grid
            </button>
            <button
              class="view-toggle-btn"
              :class="{ active: viewMode === 'table' }"
              @click="viewMode = 'table'"
              title="Table View"
            >
              Table
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Loading & Error States -->
    <div v-if="loading && plugins.length === 0" class="loading-state glass-panel">
      <div class="spinner"></div>
      <p>Loading plugin registry...</p>
    </div>

    <div v-else-if="error" class="error-banner glass-panel">
      <BaseIcon name="alert-triangle" size="xs" class="error-icon" />
      <div class="error-msg"><strong>Error:</strong> {{ error }}</div>
      <button class="btn btn-sm btn-secondary" @click="refreshPlugins">Retry</button>
    </div>

    <!-- Desktop Empty Filter State -->
    <div v-else-if="filteredPlugins.length === 0 && plugins.length > 0" class="empty-state glass-panel desktop-only">
      <div class="empty-icon"><BaseIcon name="plug" size="lg" /></div>
      <h3>No plugins match current filters</h3>
      <p>Try adjusting your search terms, category tabs, or permission scope filter.</p>
      <div class="empty-actions">
        <button class="btn btn-secondary" @click="resetFilters">Reset Filters</button>
      </div>
    </div>

    <!-- Desktop / Tablet View Component (Grid or Table) -->
    <div v-if="filteredPlugins.length > 0 || plugins.length === 0" class="desktop-only">
      <PluginsGrid
        v-if="viewMode === 'grid'"
        :plugins="filteredPlugins"
        :starter-presets="starterPresets"
        :toggling-id="togglingId"
        :loading="loading"
        :installing-preset="installingPreset"
        :category-badge-class="categoryBadgeClass"
        :get-runtime-status-label="getRuntimeStatusLabel"
        :get-runtime-status-class="getRuntimeStatusClass"
        @toggle="togglePlugin"
        @configure="openConfigModal"
        @edit="openEditModal"
        @delete="confirmDelete"
        @test-bundle="testBundleLoad"
        @install-preset="installPreset"
      />

      <PluginsTable
        v-else-if="viewMode === 'table'"
        :plugins="filteredPlugins"
        :toggling-id="togglingId"
        :category-badge-class="categoryBadgeClass"
        :get-runtime-status-label="getRuntimeStatusLabel"
        :get-runtime-status-class="getRuntimeStatusClass"
        @toggle="togglePlugin"
        @configure="openConfigModal"
        @inspect="openEditModal"
        @uninstall="confirmDelete"
      />
    </div>

    <!-- Mobile High-Density Stream Cards Component (<768px) -->
    <div class="mobile-only">
      <PluginsMobileCards
        :plugins="filteredPlugins"
        :toggling-id="togglingId"
        :category-badge-class="categoryBadgeClass"
        :get-runtime-status-label="getRuntimeStatusLabel"
        :get-runtime-status-class="getRuntimeStatusClass"
        @toggle="togglePlugin"
        @configure="openConfigModal"
        @edit="openEditModal"
        @delete="confirmDelete"
        @install="openRegisterModal"
      />
    </div>

    <!-- Modals -->
    <InstallPluginModal
      :show="showFormModal"
      :is-editing="isEditing"
      :form="form"
      v-model:permissions-raw="formPermissionsRaw"
      :submitting="formSubmitting"
      :error="formError"
      @close="closeFormModal"
      @submit="submitForm"
    />

    <PluginConfigModal
      :show="showConfigModal"
      :plugin="activeConfigPlugin"
      :config-pairs="configPairs"
      :saving="configSaving"
      :error="configSaveError"
      @close="closeConfigModal"
      @save="saveConfig"
      @add-pair="addConfigPair"
      @remove-pair="removeConfigPair"
    />

    <!-- Operation Toast Feedback -->
    <div v-if="toastMessage" class="test-feedback-toast glass-panel" :class="toastMessage.type">
      <div class="toast-header">
        <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" /> <span>{{ toastMessage.type === 'success' ? 'Success' : 'Action Notice' }}</span>
        <button class="toast-close" @click="toastMessage = null"><BaseIcon name="x" size="xs" /></button>
      </div>
      <div class="toast-body">
        {{ toastMessage.text }}
      </div>
    </div>

    <!-- Test Feedback Toast -->
    <div v-if="testResult" class="test-feedback-toast glass-panel" :class="testResult.status">
      <div class="toast-header">
        <BaseIcon :name="testResult.status === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" /> <span>{{ testResult.status === 'success' ? 'Bundle Loaded' : 'Bundle Test Failed' }}</span>
        <button class="toast-close" @click="testResult = null"><BaseIcon name="x" size="xs" /></button>
      </div>
      <div class="toast-body">
        <strong>{{ testResult.pluginName }}:</strong> {{ testResult.message }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { usePlugins } from '../composables/usePlugins'
import PluginsGrid from '../components/plugins/PluginsGrid.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import PluginsTable from '../components/plugins/PluginsTable.vue'
import PluginsMobileCards from '../components/plugins/PluginsMobileCards.vue'
import PluginConfigModal from '../components/plugins/PluginConfigModal.vue'
import InstallPluginModal from '../components/plugins/InstallPluginModal.vue'

const {
  plugins, stats, loading, error, toastMessage, viewMode, categoryCount, starterPresets, categories,
  availablePermissionScopes, wasmSandboxStatus, searchQuery, selectedCategory,
  selectedStatus, selectedScope, filteredPlugins, togglingId, installingPreset,
  testResult, testBundleLoad, getRuntimeStatusLabel, getRuntimeStatusClass,
  refreshPlugins, resetFilters, categoryBadgeClass, togglePlugin, installPreset, confirmDelete,
  showFormModal, isEditing, form, formPermissionsRaw, formSubmitting, formError,
  openRegisterModal, openEditModal, closeFormModal, submitForm,
  showConfigModal, activeConfigPlugin, configPairs, configSaving, configSaveError,
  openConfigModal, closeConfigModal, addConfigPair, removeConfigPair, saveConfig,
} = usePlugins()
</script>

<style>
@import '../assets/styles/views/plugins.css';
</style>