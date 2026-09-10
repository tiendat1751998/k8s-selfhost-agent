<script setup lang="ts">
import { ref } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
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
  <div class="view-container animate-fade-in">
    <!-- Desktop Header (>=768px) -->
    <div class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>PROGRESSIVE DELIVERY & GOVERNANCE PIPELINE</span>
        </div>
        <h1 class="view-title">Multi-Stage Release Promotions</h1>
        <p class="view-desc">
          Automated gate approvals and progressive environment promotion pipeline across Dev ➔ QA ➔ Staging ➔ Production.
        </p>
      </div>

      <div class="header-actions">
        <!-- Segmented View Mode Toggle: [ 📑 Table ] [ 🔀 Pipeline ] -->
        <div class="segmented-control font-mono">
          <button
            class="segmented-btn"
            :class="{ active: viewMode === 'table' }"
            @click="viewMode = 'table'"
            title="Table View"
          >
            <span>📑 Table</span>
          </button>
          <button
            class="segmented-btn"
            :class="{ active: viewMode === 'pipeline' }"
            @click="viewMode = 'pipeline'"
            title="Pipeline View"
          >
            <span>🔀 Pipeline</span>
          </button>
        </div>

        <button class="btn btn-secondary" :disabled="loading || loadingServices" @click="refreshAll">
          <span>{{ loading || loadingServices ? '⏳ Querying...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="openCreateModal">
          <span>+ Request Promotion</span>
        </button>
      </div>
    </div>

    <!-- Mobile 44px Command Bar (<768px) -->
    <div class="promotions-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🚀 Promotions ({{ promotions.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button class="btn-icon-cmd" title="Request Promotion" aria-label="Request Promotion" @click="openCreateModal">
          <span>➕</span>
        </button>
        <button class="btn-icon-cmd" :disabled="loading || loadingServices" title="Refresh" aria-label="Refresh" @click="refreshAll">
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="promotions-micro-telemetry mobile-only font-mono" role="status" aria-label="Promotions Micro Telemetry">
      <span class="tel-item tel-pend">⏳ {{ pendingCount }} pend</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-act">🚀 {{ approvedCount }} act</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-done">✅ {{ completedCount }} done</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-rej">🛑 {{ rejectedCount }} rej</span>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- Metric HUD (Desktop only >=768px) -->
    <div class="metrics-grid desktop-metrics desktop-only">
      <MetricCard
        title="Pending Approvals"
        :value="pendingCount"
        subtitle="Promotion requests awaiting review"
        icon="⏳"
        badge="GATE"
        :badge-color="pendingCount > 0 ? 'amber' : 'emerald'"
        :trend="pendingCount > 0 ? 'Review Required' : 'All Clear'"
        :trend-type="pendingCount > 0 ? 'neutral' : 'positive'"
      />
      <MetricCard
        title="Active Promoting"
        :value="approvedCount"
        subtitle="Canary rollout & staging verification"
        icon="🚀"
        badge="ROLLOUT"
        badge-color="cyan"
        trend="In-Flight Verification"
        trend-type="positive"
      />
      <MetricCard
        title="Completed Releases"
        :value="completedCount"
        subtitle="Successfully promoted to destination"
        icon="✅"
        badge="SHIPPED"
        badge-color="emerald"
        trend="Continuous Delivery"
        trend-type="positive"
      />
      <MetricCard
        title="Rejected / Aborted"
        :value="rejectedCount"
        subtitle="Failed quality gates or security review"
        icon="🛑"
        badge="REJECTED"
        :badge-color="rejectedCount > 0 ? 'rose' : 'emerald'"
      />
    </div>

    <!-- Desktop Pipeline Board OR Table (Mutually exclusive on desktop, suppressed on mobile) -->
    <div v-if="viewMode === 'pipeline'" class="desktop-only pipeline-grid-container animate-fade-in">
      <PromotionPipelinesGrid
        :environments="environments"
        :promotions="promotions"
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
        :promotions="promotions"
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

.metrics-grid.desktop-only {
  display: grid !important;
  grid-template-columns: repeat(4, 1fr) !important;
}
@media (min-width: 768px) and (max-width: 1023.98px) {
  .metrics-grid.desktop-only {
    grid-template-columns: repeat(2, 1fr) !important;
  }
}
@media (max-width: 767.98px) {
  .metrics-grid.desktop-only {
    display: none !important;
  }
}
</style>
