<template>
  <div class="plugins-page plugins-view">
    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="plugins-toolbar-sleek glass-panel">
      <!-- Search input with search icon and clear button (filters by name, id, author, description) -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          id="plugin-search"
          v-model="searchQuery"
          type="text"
          class="toolbar-search-input"
          placeholder="Search name, id, author, description..."
          aria-label="Search plugins"
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

      <!-- Scope filter dropdown (All Scopes, UI, API, Core, Custom) -->
      <select
        id="filter-scope"
        v-model="selectedScope"
        class="toolbar-select desktop-only"
        aria-label="Filter by Scope"
      >
        <option value="all">All Scopes</option>
        <option value="ui">UI</option>
        <option value="api">API</option>
        <option value="core">Core</option>
        <option value="custom">Custom</option>
        <template v-for="scope in availablePermissionScopes" :key="scope">
          <option
            v-if="!['ui', 'api', 'core', 'custom'].includes(scope.toLowerCase())"
            :value="scope"
          >
            {{ scope.toUpperCase() }}
          </option>
        </template>
      </select>

      <!-- Status filter dropdown (All Statuses, Active, Disabled) -->
      <select
        id="filter-status"
        v-model="selectedStatus"
        class="toolbar-select desktop-only"
        aria-label="Filter by Status"
      >
        <option value="all">All Statuses</option>
        <option value="active">Active</option>
        <option value="disabled">Disabled</option>
      </select>

      <!-- Inline compact KPI badge strip font-mono: {{ plugins.length }} Plugins ({{ activeCount }} Active · {{ installedCount }} Installed) -->
      <div class="toolbar-kpi-strip font-mono desktop-only" role="status" aria-label="Plugin count metrics">
        <span class="kpi-badge font-mono">{{ plugins.length }} Plugins ({{ activeCount }} Active · {{ installedCount }} Installed)</span>
      </div>

      <!-- Action buttons: View mode toggle, Refresh, + Register Plugin (primary button) -->
      <div class="toolbar-actions-group">
        <!-- View mode toggle (Grid / Table) -->
        <div class="view-mode-toggle desktop-only" title="Switch layout display">
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'grid' }"
            title="Grid View"
            aria-label="Grid View"
            @click="viewMode = 'grid'"
          >
            <BaseIcon name="grid" size="xs" /> <span>Grid</span>
          </button>
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'table' }"
            title="Table View"
            aria-label="Table View"
            @click="viewMode = 'table'"
          >
            <BaseIcon name="file-text" size="xs" /> <span>Table</span>
          </button>
        </div>

        <button
          type="button"
          class="btn btn-secondary toolbar-btn desktop-only"
          title="Refresh plugins"
          aria-label="Refresh plugins"
          :disabled="loading"
          @click="refreshPlugins"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" />
          <span>Refresh</span>
        </button>

        <button
          type="button"
          class="btn btn-primary toolbar-btn"
          title="Register a new plugin"
          aria-label="+ Register Plugin"
          @click="openRegisterModal"
        >
          <BaseIcon name="plus" size="xs" /> <span>+ Register Plugin</span>
        </button>
      </div>
    </div>

    <!-- Mobile 44px Command Bar (<768px) -->
    <div class="plugins-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title"><BaseIcon name="plug" size="xs" /> Plugins ({{ activeCount }}/{{ installedCount }})</span>
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
      <span class="tel-item tel-installed"><BaseIcon name="plug" size="xs" /> {{ installedCount }} Installed</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-active"><BaseIcon name="zap" size="xs" /> {{ activeCount }} Active</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-verified"><BaseIcon name="shield" size="xs" /> {{ wasmSandboxStatus.isolationMode === 'wasm-wasi' ? 'WASM Verified' : 'Verified' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-available"><BaseIcon name="box" size="xs" /> {{ categoryCount }} Available</span>
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
      <p>Try adjusting your search terms, scope, or status filters.</p>
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
  plugins, loading, error, toastMessage, viewMode, categoryCount, starterPresets,
  availablePermissionScopes, wasmSandboxStatus, searchQuery,
  selectedStatus, selectedScope, filteredPlugins, togglingId, installingPreset,
  activeCount, installedCount,
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