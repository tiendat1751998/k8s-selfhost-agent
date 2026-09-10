<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>OPERATIONAL RUNBOOKS & PLAYBOOKS</span>
        </div>
        <h1 class="view-title">Standard Operating Procedures & Execution Catalog</h1>
        <p class="view-desc">
          Structured runbook library with interactive step execution, automated diagnostics commands, and disaster recovery playbooks.
        </p>
      </div>

      <div class="header-actions">
        <div class="view-switcher glass-panel" style="display: inline-flex; padding: 3px; gap: 4px;">
          <button 
            class="btn btn-sm" 
            :class="viewMode === 'grid' ? 'btn-primary' : 'btn-ghost'"
            @click="viewMode = 'grid'"
          >
            <span>⊞ Grid</span>
          </button>
          <button 
            class="btn btn-sm" 
            :class="viewMode === 'table' ? 'btn-primary' : 'btn-ghost'"
            @click="viewMode = 'table'"
          >
            <span>≡ Table</span>
          </button>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="fetchRunbooks">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>+ Create Runbook</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<768px) -->
    <div class="runbooks-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">📖 Runbooks ({{ filteredRunbooks.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Create Runbook"
          aria-label="Create Runbook"
          @click="showCreateModal = true"
        >
          <span>➕</span>
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh Runbooks"
          aria-label="Refresh Runbooks"
          @click="fetchRunbooks"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="runbooks-micro-telemetry mobile-only font-mono" role="status" aria-label="Runbooks Micro Telemetry">
      <span class="tel-item tel-rbooks">📖 {{ runbooks.length }} rbooks</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-cats">🗂️ {{ categoriesCount }} cats</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-steps">🪜 {{ totalStepsCount }} steps</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-live">⚡ Live CLI</span>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="dismissStatus">✕</button>
    </div>

    <!-- Metrics HUD Grid -->
    <div class="metrics-grid desktop-metrics desktop-only">
      <MetricCard
        title="Cataloged Runbooks"
        :value="runbooks.length"
        badge="AVAILABLE"
        badge-color="cyan"
        subtitle="Standard operating playbooks"
        icon="📖"
      />
      <MetricCard
        title="Operational Categories"
        :value="categoriesCount"
        badge="ORGANIZED"
        badge-color="violet"
        subtitle="Incident, DR, Security & DB"
        icon="🗂️"
      />
      <MetricCard
        title="Total Procedure Steps"
        :value="totalStepsCount"
        badge="STEPS"
        badge-color="emerald"
        subtitle="Automated & verified instructions"
        icon="🪜"
      />
      <MetricCard
        title="Execution Engine"
        value="LIVE CLI"
        badge="READY"
        badge-color="emerald"
        subtitle="One-click diagnostic run"
        icon="⚡"
      />
    </div>

    <!-- Category Filter Bar -->
    <div class="filter-bar glass-panel">
      <div class="filter-group">
        <span class="filter-label">Category:</span>
        <button 
          v-for="cat in categoryList" 
          :key="cat"
          class="filter-pill"
          :class="{ 'filter-active': activeCategory === cat }"
          @click="selectCategory(cat)"
        >
          <span>{{ cat }}</span>
        </button>
      </div>

      <div class="search-box">
        <input 
          v-model="searchQuery" 
          type="text" 
          placeholder="Search runbook title, tags, or steps..." 
          class="input-glass" 
          style="width: 260px;" 
        />
      </div>
    </div>

    <!-- Desktop Grid View -->
    <RunbooksGrid
      v-if="viewMode === 'grid'"
      class="desktop-only-grid"
      :runbooks="filteredRunbooks"
      :executing-id="executingId"
      :get-category-icon="getCategoryIcon"
      :format-date="formatDate"
      @execute="openExecuteModal"
      @inspect="openInspectDrawer"
      @edit="openEditModal"
      @delete="handleDeleteRunbook"
      @create="showCreateModal = true"
    />

    <!-- Desktop Table View -->
    <RunbooksTable
      v-else-if="viewMode === 'table'"
      class="desktop-only-table"
      :runbooks="filteredRunbooks"
      :executing-id="executingId"
      :get-category-icon="getCategoryIcon"
      :format-date="formatDate"
      @execute="openExecuteModal"
      @inspect="openInspectDrawer"
      @edit="openEditModal"
      @delete="handleDeleteRunbook"
    />

    <!-- Mobile Card Stream -->
    <RunbooksMobileCards
      class="mobile-only-stream"
      :runbooks="filteredRunbooks"
      :executing-id="executingId"
      :get-category-icon="getCategoryIcon"
      :format-date="formatDate"
      @execute="openExecuteModal"
      @inspect="openInspectDrawer"
      @edit="openEditModal"
      @delete="handleDeleteRunbook"
    />

    <!-- Modals & Drawers -->
    <CreateRunbookModal
      :show="showCreateModal || showEditModal"
      :is-edit="showEditModal"
      :runbook="editingRunbook || newRunbook"
      :tag-input="tagInput"
      @close="showCreateModal = false; showEditModal = false"
      @save="showEditModal ? handleUpdateRunbook() : handleCreateRunbook()"
      @update:tag-input="tagInput = $event"
    />

    <ExecuteRunbookModal
      :show="showExecuteModal"
      :runbook="executingRunbook"
      :parameters="parameterSchemas"
      @close="showExecuteModal = false"
      @execute="payload => executingRunbook && handleExecuteRunbook(executingRunbook, payload)"
    />

    <RunbookExecutionDrawer
      :show="showExecutionDrawer"
      :runbook="selectedRunbook"
      :execution="activeExecution"
      :steps="parsedSteps"
      :completed-steps="completedSteps"
      :logs="executionLogs"
      @close="showExecutionDrawer = false"
      @toggle-step="toggleStep"
      @mark-all-complete="markAllStepsComplete"
      @execute-step="executeSingleStep"
      @copy-command="copyCommand"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import RunbooksGrid from '../components/runbooks/RunbooksGrid.vue'
import RunbooksTable from '../components/runbooks/RunbooksTable.vue'
import RunbooksMobileCards from '../components/runbooks/RunbooksMobileCards.vue'
import CreateRunbookModal from '../components/runbooks/CreateRunbookModal.vue'
import ExecuteRunbookModal from '../components/runbooks/ExecuteRunbookModal.vue'
import RunbookExecutionDrawer from '../components/runbooks/RunbookExecutionDrawer.vue'
import { useRunbooks } from '../composables/useRunbooks'
import '../assets/styles/views/runbooks.css'
import '../assets/styles/components/runbooks-drawers.css'

const {
  runbooks,
  loading,
  statusMessage,
  activeCategory,
  searchQuery,
  viewMode,
  selectedRunbook,
  executingRunbook,
  activeExecution,
  executingId,
  completedSteps,
  showCreateModal,
  showEditModal,
  showExecuteModal,
  showExecutionDrawer,
  editingRunbook,
  executionLogs,
  tagInput,
  newRunbook,
  categoryList,
  categoriesCount,
  totalStepsCount,
  filteredRunbooks,
  parsedSteps,
  parameterSchemas,
  fetchRunbooks,
  selectCategory,
  openInspectDrawer,
  openExecuteModal,
  openEditModal,
  toggleStep,
  markAllStepsComplete,
  handleExecuteRunbook,
  executeSingleStep,
  handleCreateRunbook,
  handleUpdateRunbook,
  handleDeleteRunbook,
  copyCommand,
  getCategoryIcon,
  formatDate,
  dismissStatus,
} = useRunbooks()

onMounted(() => {
  fetchRunbooks()
})
</script>
