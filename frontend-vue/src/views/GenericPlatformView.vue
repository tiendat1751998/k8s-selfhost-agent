<script setup lang="ts">
import '../assets/styles/views/generic.css'
import { useGenericPlatform } from '../composables/useGenericPlatform'
import MetricCard from '../components/ui/MetricCard.vue'
import GenericResourceTable from '../components/platform/GenericResourceTable.vue'
import GenericResourceDrawer from '../components/platform/GenericResourceDrawer.vue'
import GenericMobileCards from '../components/platform/GenericMobileCards.vue'

const {
  currentFeature,
  filteredItems,
  paginatedItems,
  dynamicColumns,
  searchQuery,
  statusFilter,
  sortKey,
  sortOrder,
  currentPage,
  pageSize,
  totalPages,
  toggleSort,
  isDrawerOpen,
  inspectedItem,
  inspectedYaml,
  inspectedJson,
  inspectItem,
  closeDrawer,
  dispatchAction,
  isSyncing,
  toastMessage
} = useGenericPlatform()
</script>

<template>
  <div class="generic-page animate-fade-in">
    <!-- Action Toast Notification -->
    <div
      v-if="toastMessage"
      class="feedback-toast"
      :class="toastMessage.type === 'success' ? 'toast-success' : 'toast-error'"
      role="alert"
    >
      <span>{{ toastMessage.type === 'success' ? '⚡' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
    </div>

    <!-- Page Header -->
    <header class="page-header">
      <div class="header-titles">
        <div class="header-badge">
          <span class="badge badge-cyan">{{ currentFeature.category }}</span>
          <span class="badge badge-emerald">{{ currentFeature.badge }}</span>
        </div>
        <h1 class="page-title">{{ currentFeature.title }}</h1>
        <p class="page-desc">{{ currentFeature.desc }}</p>
      </div>

      <div class="header-actions">
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="isSyncing"
          @click="dispatchAction('configure_rules')"
        >
          <span>⚙️ Configure Rules</span>
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="isSyncing"
          @click="dispatchAction('autonomous_sync')"
        >
          <span>{{ isSyncing ? '🔄 Syncing...' : '⚡ Run Autonomous Sync' }}</span>
        </button>
      </div>
    </header>

    <!-- Metrics Summary Grid -->
    <section class="metrics-grid" aria-label="Platform Telemetry Metrics">
      <MetricCard
        v-for="m in currentFeature.metrics"
        :key="m.title"
        :title="m.title"
        :value="m.value"
        :trend="m.trend"
        :trendType="m.dir === 'up' || m.dir === 'down' ? 'positive' : 'neutral'"
      />
    </section>

    <!-- Telemetry & Resource Management Panel -->
    <main class="content-panel glass-panel">
      <div class="panel-header">
        <div class="p-title-wrap">
          <span class="pulse-dot pulse-dot-emerald"></span>
          <h3>Active Subsystem Telemetry & Managed Entities</h3>
        </div>

        <div class="panel-controls">
          <div class="search-input-wrap">
            <span class="search-icon-inline" aria-hidden="true">🔍</span>
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Search resources, tags, IDs..."
              class="search-field"
              aria-label="Filter resources"
            />
          </div>

          <select
            v-model="statusFilter"
            class="status-select"
            aria-label="Filter by status"
          >
            <option value="all">All Statuses</option>
            <option value="healthy">Healthy</option>
            <option value="active">Active</option>
            <option value="warning">Warning</option>
            <option value="error">Error</option>
          </select>
        </div>
      </div>

      <!-- Desktop & Tablet: Dynamic Unstructured Columns Table -->
      <GenericResourceTable
        :items="paginatedItems"
        :columns="dynamicColumns"
        :sort-key="sortKey"
        :sort-order="sortOrder"
        :selected-id="inspectedItem?.id"
        @sort="toggleSort"
        @inspect="inspectItem"
        @action="dispatchAction"
      />

      <!-- Mobile: High-Density Stream (~65px/item, 0 horizontal scroll) -->
      <GenericMobileCards
        :items="paginatedItems"
        :selected-id="inspectedItem?.id"
        @inspect="inspectItem"
        @action="dispatchAction"
      />

      <!-- Pagination Bar -->
      <footer v-if="filteredItems.length > pageSize" class="pagination-bar">
        <span>
          Showing {{ ((currentPage - 1) * pageSize) + 1 }} - {{ Math.min(currentPage * pageSize, filteredItems.length) }} of {{ filteredItems.length }}
        </span>
        <div class="pagination-actions">
          <button
            type="button"
            class="btn btn-xs btn-secondary"
            :disabled="currentPage <= 1"
            @click="currentPage--"
          >
            ‹ Prev
          </button>
          <span class="font-mono">Page {{ currentPage }} / {{ totalPages }}</span>
          <button
            type="button"
            class="btn btn-xs btn-secondary"
            :disabled="currentPage >= totalPages"
            @click="currentPage++"
          >
            Next ›
          </button>
        </div>
      </footer>
    </main>

    <!-- YAML/JSON Live Inspection Drawer -->
    <GenericResourceDrawer
      :is-open="isDrawerOpen"
      :item="inspectedItem"
      :yaml-content="inspectedYaml"
      :json-content="inspectedJson"
      @close="closeDrawer"
      @action="dispatchAction"
    />
  </div>
</template>