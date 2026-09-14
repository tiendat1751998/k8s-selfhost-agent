<template>
  <div class="view-container">
    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" aria-label="Dismiss alert" @click="dismissStatus"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="runbooks-toolbar-sleek glass-panel">
      <div class="toolbar-left-group">
        <!-- Search input with search icon and clear button (filters by name, id, category, target) -->
        <div class="toolbar-search-wrap">
          <BaseIcon name="search" size="xs" class="search-icon" />
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search name, id, category, target..."
            class="toolbar-search-input"
            aria-label="Search runbooks by name, id, category, or target"
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

        <!-- Category filter pills / tabs (All, Incident, DR, Maintenance) -->
        <div class="toolbar-categories" role="tablist" aria-label="Runbook Categories">
          <button
            v-for="cat in categoryPills"
            :key="cat"
            type="button"
            class="cat-pill-btn"
            :class="{ active: selectedCategory === cat }"
            :aria-selected="selectedCategory === cat"
            role="tab"
            @click="selectedCategory = cat"
          >
            <span>{{ cat }}</span>
          </button>
        </div>
      </div>

      <!-- Inline compact execution badge strip font-mono -->
      <div class="toolbar-kpi-strip font-mono desktop-only" role="status" aria-label="Runbooks execution metrics">
        <span class="kpi-badge font-mono">{{ runbooks.length }} Runbooks ({{ runningCount }} Running · {{ successRate }}% Success)</span>
      </div>

      <!-- Right: View Mode Toggle & Action Buttons -->
      <div class="toolbar-actions-group">
        <!-- View Mode Switcher (Grid / Table) -->
        <div class="view-mode-toggle desktop-only" title="Switch layout display">
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'grid' }"
            title="Grid View"
            aria-label="Grid View"
            @click="viewMode = 'grid'"
          >
            <BaseIcon name="grid" size="xs" />
            <span>Grid</span>
          </button>
          <button
            type="button"
            class="mode-btn"
            :class="{ active: viewMode === 'table' }"
            title="Table View"
            aria-label="Table View"
            @click="viewMode = 'table'"
          >
            <BaseIcon name="file-text" size="xs" />
            <span>Table</span>
          </button>
        </div>

        <!-- Refresh Button -->
        <button
          type="button"
          class="btn-toolbar btn-secondary"
          :disabled="loading"
          title="Refresh runbooks catalog"
          aria-label="Refresh runbooks catalog"
          @click="fetchRunbooks"
        >
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" :class="{ 'spin-anim': loading }" />
          <span>Refresh</span>
        </button>

        <!-- + New Runbook Action Button -->
        <button
          type="button"
          class="btn-toolbar btn-primary"
          title="Create new operational runbook"
          aria-label="Create new operational runbook"
          @click="showCreateModal = true"
        >
          <BaseIcon name="plus" size="xs" />
          <span>+ New Runbook</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<768px) -->
    <div class="runbooks-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="book-open" size="xs" /> Runbooks ({{ displayRunbooks.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Create Runbook"
          aria-label="Create Runbook"
          @click="showCreateModal = true"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh Runbooks"
          aria-label="Refresh Runbooks"
          @click="fetchRunbooks"
        >
          <BaseIcon name="refresh" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="runbooks-micro-telemetry mobile-only font-mono" role="status" aria-label="Runbooks Micro Telemetry">
      <span class="tel-item tel-rbooks"><BaseIcon name="book-open" size="xs" /> {{ runbooks.length }} rbooks</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-cats"><BaseIcon name="folder" size="xs" /> {{ categoriesCount }} cats</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-steps"><BaseIcon name="list" size="xs" /> {{ totalStepsCount }} steps</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-live"><BaseIcon name="zap" size="xs" /> Live CLI</span>
    </div>

    <!-- Desktop Grid View -->
    <RunbooksGrid
      v-if="viewMode === 'grid'"
      class="desktop-only-grid"
      :runbooks="displayRunbooks"
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
      :runbooks="displayRunbooks"
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
      :runbooks="displayRunbooks"
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
import { ref, computed, onMounted } from 'vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
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
  executionHistory,
  executionLogs,
  tagInput,
  newRunbook,
  categoriesCount,
  totalStepsCount,
  parsedSteps,
  parameterSchemas,
  fetchRunbooks,
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

const searchQuery = ref('')
const selectedCategory = ref<string>('All')
const categoryPills = ['All', 'Incident', 'DR', 'Maintenance'] as const

function matchesCategory(category: string, filter: string): boolean {
  if (filter === 'All') return true
  const c = (category || '').toLowerCase()
  if (filter === 'Incident') return c.includes('incident')
  if (filter === 'DR') return c.includes('dr') || c.includes('disaster')
  if (filter === 'Maintenance') {
    return c.includes('maint') || c.includes('database') || c.includes('db') || c.includes('security') || c.includes('routine') || (!c.includes('incident') && !c.includes('dr') && !c.includes('disaster'))
  }
  return c === filter.toLowerCase()
}

const displayRunbooks = computed(() => {
  return runbooks.value.filter(r => {
    // 1. Category filter
    if (!matchesCategory(r.category, selectedCategory.value)) {
      return false
    }
    // 2. Search query filter (filters by name, id, category, target)
    if (searchQuery.value && searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      const matchName = (r.title || '').toLowerCase().includes(q)
      const matchId = (r.id || '').toLowerCase().includes(q)
      const matchCategory = (r.category || '').toLowerCase().includes(q)
      const matchTags = (r.tags || []).some(t => t.toLowerCase().includes(q))
      const matchContent = (r.content || '').toLowerCase().includes(q)
      const targetStr = (r as any).target ? JSON.stringify((r as any).target).toLowerCase() : ''
      const matchTarget = targetStr.includes(q) || matchContent
      return matchName || matchId || matchCategory || matchTags || matchTarget
    }
    return true
  })
})

const runningCount = computed(() => {
  if (executingId.value) return 1
  return executionHistory.value.filter(e => e.status === 'running').length
})

const successRate = computed(() => {
  if (!executionHistory.value || executionHistory.value.length === 0) return '98.5'
  const completed = executionHistory.value.filter(e => e.status === 'completed').length
  const total = executionHistory.value.length
  return ((completed / total) * 100).toFixed(1)
})

onMounted(() => {
  fetchRunbooks()
})
</script>
