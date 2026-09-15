<script setup lang="ts">
import { ref } from 'vue'
import { usePromotions } from '../composables/usePromotions'
import PromotionPipelinesGrid from '../components/promotions/PromotionPipelinesGrid.vue'
import PromotionsTable from '../components/promotions/PromotionsTable.vue'
import PromotionsMobileCards from '../components/promotions/PromotionsMobileCards.vue'
import TriggerPromotionModal from '../components/promotions/TriggerPromotionModal.vue'
import PromotionDiffDrawer from '../components/promotions/PromotionDiffDrawer.vue'

const viewMode = ref<'table' | 'pipeline'>('table')

const {
  loading,
  loadingServices,
  error,
  actionLoading,
  toastMessage,
  promotions,
  statusFilter,
  promotionSearchQuery,
  filteredPromotions,
  runningServices,
  environments,
  serviceSearchQuery,
  isServiceDropdownOpen,
  showCreateModal,
  isDiffDrawerOpen,
  selectedPromotionForDiff,
  newPromotion,
  selectedService,
  filteredServices,
  pendingCount,
  approvedCount,
  completedCount,
  rejectedCount,
  selectService,
  clearServiceSearch,
  onSearchInput,
  onSearchFocus,
  refreshAll,
  openCreateModal,
  openDiffDrawer,
  handleCreatePromotion,
  handleApprove,
  handleReject,
  handleComplete,
  handleRollback,
  handleAbort
} = usePromotions()
</script>

<template>
  <div class="view-container promotions-view animate-fade-in">
    <!-- Desktop Header (>=768px) -->
    <div class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>PROGRESSIVE DELIVERY & GOVERNANCE PIPELINE</span>
        </div>
        <h1 class="view-title">Multi-Stage Release Promotions</h1>
        <p class="view-desc">
          Automated gate approvals and progressive environment promotion pipeline across Dev &rarr; QA &rarr; Staging &rarr; Production.
        </p>
      </div>
    </div>

    <!-- Unified 38px Sleek Desktop Toolbar (>=768px) -->
    <div class="promotions-toolbar-sleek glass-panel desktop-only">
      <!-- Left: Standardized 30px Capsule Pill search input -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="promotionSearchQuery"
          type="text"
          class="toolbar-search-input"
          placeholder="Search promotions..."
        />
        <button
          v-if="promotionSearchQuery"
          type="button"
          class="clear-input-btn"
          title="Clear search"
          aria-label="Clear search"
          @click="promotionSearchQuery = ''"
        >
          &times;
        </button>
      </div>

      <!-- Center: 1-Click status filter pills with live counts -->
      <div class="toolbar-status-pills">
        <button
          type="button"
          class="status-pill"
          :class="{ active: statusFilter === 'all' }"
          @click="statusFilter = 'all'"
        >
          All <span class="pill-count">{{ promotions.length }}</span>
        </button>
        <button
          type="button"
          class="status-pill status-pill-pending"
          :class="{ active: statusFilter === 'pending' }"
          @click="statusFilter = 'pending'"
        >
          <span class="pill-dot amber"></span>
          Pending <span class="pill-count">{{ pendingCount }}</span>
        </button>
        <button
          type="button"
          class="status-pill status-pill-active"
          :class="{ active: statusFilter === 'active' }"
          @click="statusFilter = 'active'"
        >
          <span class="pill-dot cyan"></span>
          Active <span class="pill-count">{{ approvedCount }}</span>
        </button>
        <button
          type="button"
          class="status-pill status-pill-completed"
          :class="{ active: statusFilter === 'completed' }"
          @click="statusFilter = 'completed'"
        >
          <span class="pill-dot emerald"></span>
          Completed <span class="pill-count">{{ completedCount }}</span>
        </button>
        <button
          type="button"
          class="status-pill status-pill-rejected"
          :class="{ active: statusFilter === 'rejected' }"
          @click="statusFilter = 'rejected'"
        >
          <span class="pill-dot rose"></span>
          Rejected <span class="pill-count">{{ rejectedCount }}</span>
        </button>
      </div>

      <!-- Right: View Mode Toggle, Refresh, + Request Promotion CTA -->
      <div class="toolbar-actions-group">
        <div class="segmented-control font-mono">
          <button
            type="button"
            class="segmented-btn"
            :class="{ active: viewMode === 'table' }"
            @click="viewMode = 'table'"
            title="Table View"
          >
            <BaseIcon name="file-text" size="xs" /> <span>Table</span>
          </button>
          <button
            type="button"
            class="segmented-btn"
            :class="{ active: viewMode === 'pipeline' }"
            @click="viewMode = 'pipeline'"
            title="Pipeline View"
          >
            <BaseIcon name="git-branch" size="xs" /> <span>Pipeline</span>
          </button>
        </div>

        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="loading || loadingServices"
          @click="refreshAll"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading || loadingServices }" />
          <span>{{ loading || loadingServices ? 'Querying...' : 'Refresh' }}</span>
        </button>

        <button
          type="button"
          class="btn btn-primary btn-sm"
          @click="openCreateModal"
        >
          <span>+ Request Promotion</span>
        </button>
      </div>
    </div>

    <!-- Mobile 44px Command Bar (<768px) -->
    <div class="promotions-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="play" size="sm" /> Promotions ({{ promotions.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button class="btn-icon-cmd" title="Request Promotion" aria-label="Request Promotion" @click="openCreateModal">
          <BaseIcon name="plus" size="xs" />
        </button>
        <button class="btn-icon-cmd" :disabled="loading || loadingServices" title="Refresh" aria-label="Refresh" @click="refreshAll">
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="promotions-micro-telemetry mobile-only font-mono" role="status" aria-label="Promotions Micro Telemetry">
      <span class="tel-item tel-pend"><BaseIcon name="clock" size="xs" /> {{ pendingCount }} pend</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-act"><BaseIcon name="play" size="xs" /> {{ approvedCount }} act</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-done"><BaseIcon name="check-circle" size="xs" /> {{ completedCount }} done</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-rej"><BaseIcon name="x-circle" size="xs" /> {{ rejectedCount }} rej</span>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="sm" />
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null" aria-label="Close"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Desktop Pipeline Board OR Table (Mutually exclusive on desktop, suppressed on mobile) -->
    <div v-if="viewMode === 'pipeline'" class="desktop-only pipeline-grid-container animate-fade-in">
      <PromotionPipelinesGrid
        :environments="environments"
        :promotions="filteredPromotions"
        :action-loading="actionLoading"
        @approve="handleApprove"
        @reject="handleReject"
        @complete="handleComplete"
        @diff="openDiffDrawer"
        @rollback="handleRollback"
        @request-promotion="openCreateModal"
      />
    </div>

    <div v-else-if="viewMode === 'table'" class="desktop-only desktop-only-table animate-fade-in">
      <PromotionsTable
        :promotions="filteredPromotions"
        :loading="loading"
        :error="error"
        :action-loading="actionLoading"
        @approve="handleApprove"
        @reject="handleReject"
        @complete="handleComplete"
        @diff="openDiffDrawer"
        @rollback="handleRollback"
        @abort="handleAbort"
      />
    </div>

    <!-- Mobile Touch-Optimized Promotion Card Stream (<768px) -->
    <div class="mobile-only mobile-stream-wrapper">
      <PromotionsMobileCards
        :promotions="promotions"
        :loading="loading"
        :action-loading="actionLoading"
        @approve="handleApprove"
        @reject="handleReject"
        @complete="handleComplete"
        @diff="openDiffDrawer"
        @rollback="handleRollback"
        @abort="handleAbort"
        @request-promotion="openCreateModal"
      />
    </div>

    <!-- Trigger Promotion Modal -->
    <TriggerPromotionModal
      v-model:show="showCreateModal"
      :running-services="runningServices"
      :loading-services="loadingServices"
      :action-loading="actionLoading"
      :new-promotion="newPromotion"
      :service-search-query="serviceSearchQuery"
      :is-service-dropdown-open="isServiceDropdownOpen"
      :filtered-services="filteredServices"
      :selected-service="selectedService"
      @update:service-search-query="val => serviceSearchQuery = val"
      @update:is-service-dropdown-open="val => isServiceDropdownOpen = val"
      @select-service="selectService"
      @clear-service-search="clearServiceSearch"
      @search-input="onSearchInput"
      @search-focus="onSearchFocus"
      @submit="handleCreatePromotion"
    />

    <!-- Promotion Diff & Gate Inspector Drawer -->
    <PromotionDiffDrawer
      :show="isDiffDrawerOpen"
      :promotion="selectedPromotionForDiff"
      :action-loading="actionLoading"
      @update:show="val => isDiffDrawerOpen = val"
      @approve="handleApprove"
      @reject="handleReject"
      @complete="handleComplete"
      @rollback="handleRollback"
    />
  </div>
</template>

<style>
@import '../assets/styles/views/promotions.css';
</style>
