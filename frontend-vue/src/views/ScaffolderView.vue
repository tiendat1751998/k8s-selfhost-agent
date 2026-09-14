<script setup lang="ts">
import { ref, computed } from 'vue'
import { useScaffolder } from '../composables/useScaffolder'
import ScaffolderTemplatesGrid from '../components/scaffolder/ScaffolderTemplatesGrid.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import ScaffolderStepWizard from '../components/scaffolder/ScaffolderStepWizard.vue'
import ScaffolderLogsDrawer from '../components/scaffolder/ScaffolderLogsDrawer.vue'
import ScaffolderMobileCards from '../components/scaffolder/ScaffolderMobileCards.vue'
import RegisterTemplateModal from '../components/scaffolder/RegisterTemplateModal.vue'

const {
  loading,
  rendering,
  saving,
  deleting,
  toastMessage,
  templates,
  searchQuery,

  showWizardModal,
  currentStep,
  activeTemplate,
  formVariables,
  repoConfig,
  cicdConfig,
  registerInCatalog,
  ownerTeam,
  ownerEmail,

  showLogsDrawer,
  renderResult,
  activeOutputTab,
  copySuccess,
  logs,
  isDryRun,

  showCustomModal,
  customModalMode,
  customTemplate,

  loadTemplates,
  openWizard,
  previewTemplate,
  closeWizard,
  nextStep,
  prevStep,
  goToStep,
  triggerScaffoldJob,
  copyToClipboard,
  downloadFile,
  openCustomTemplateModal,
  addVariableToCustomTemplate,
  removeVariableFromCustomTemplate,
  handleSaveCustomTemplate,
  handleDeleteCustomTemplate,
  resetFilters,
} = useScaffolder()

interface CategoryOptionItem {
  key: string
  label: string
  icon?: string
}

const filterCategories: CategoryOptionItem[] = [
  { key: 'all', label: 'All', icon: 'sparkles' },
  { key: 'backend', label: 'Backend', icon: 'server' },
  { key: 'frontend', label: 'Frontend', icon: 'globe' },
  { key: 'fullstack', label: 'Fullstack', icon: 'layers' },
  { key: 'k8s', label: 'K8s', icon: 'box' },
  { key: 'devops', label: 'DevOps', icon: 'cpu' },
  { key: 'ai', label: 'AI', icon: 'zap' },
]

const activeCategory = ref('all')

const filteredTemplates = computed(() => {
  return templates.value.filter(t => {
    // 1. Category Filter
    if (activeCategory.value !== 'all') {
      const cat = activeCategory.value.toLowerCase()
      const tCat = (t.category || '').toLowerCase()
      const tTags = (t.tags || []).map(tag => tag.toLowerCase())
      const tName = (t.name || '').toLowerCase()
      const tDesc = (t.description || '').toLowerCase()

      let matches = false
      if (tCat === cat || tTags.includes(cat)) {
        matches = true
      } else if (cat === 'backend') {
        matches = tCat === 'api' || tCat === 'worker' || tCat === 'database' || tTags.some(tg => ['backend', 'api', 'rest', 'grpc', 'golang', 'python', 'go', 'node'].includes(tg))
      } else if (cat === 'frontend') {
        matches = tCat === 'web' || tTags.some(tg => ['frontend', 'web', 'ui', 'react', 'vue', 'nginx'].includes(tg))
      } else if (cat === 'fullstack') {
        matches = tCat === 'fullstack' || tTags.includes('fullstack')
      } else if (cat === 'k8s') {
        matches = tCat === 'k8s' || tTags.includes('k8s') || tTags.includes('kubernetes') || tName.includes('k8s') || tDesc.includes('k8s')
      } else if (cat === 'devops') {
        matches = tCat === 'devops' || tTags.some(tg => ['devops', 'docker', 'helm', 'ingress', 'proxy'].includes(tg))
      } else if (cat === 'ai') {
        matches = tCat === 'ai' || tTags.some(tg => ['ai', 'llm', 'ml', 'openai'].includes(tg)) || tName.includes('ai') || tDesc.includes('ai')
      }

      if (!matches) return false
    }

    // 2. Search Query Filter (filters templates by title, description, tags)
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.trim().toLowerCase()
      const matchName = t.name.toLowerCase().includes(q)
      const matchDesc = t.description.toLowerCase().includes(q)
      const matchFramework = t.framework.toLowerCase().includes(q)
      const matchTags = t.tags && t.tags.some(tag => tag.toLowerCase().includes(q))
      if (!matchName && !matchDesc && !matchFramework && !matchTags) return false
    }

    return true
  })
})

const handleResetFilters = () => {
  activeCategory.value = 'all'
  searchQuery.value = ''
  resetFilters()
}

const handleDrawerCopy = async () => {
  let content = ''
  if (activeOutputTab.value === 'logs') {
    content = logs.value.map(l => `[${l.timestamp}] [${l.level.toUpperCase()}] ${l.message}`).join('\n')
  } else if (renderResult.value) {
    if (activeOutputTab.value === 'yaml') content = renderResult.value.rendered_yaml || ''
    else if (activeOutputTab.value === 'compose') content = renderResult.value.rendered_compose || ''
    else if (activeOutputTab.value === 'helm') content = renderResult.value.rendered_helm || ''
  }
  if (content) {
    await copyToClipboard(content)
  }
}

const handleDrawerDownload = () => {
  let content = ''
  let filename = `${activeTemplate.value?.name || 'scaffold-output'}.txt`
  if (activeOutputTab.value === 'logs') {
    content = logs.value.map(l => `[${l.timestamp}] [${l.level.toUpperCase()}] ${l.message}`).join('\n')
    filename = `${activeTemplate.value?.name || 'scaffold'}-logs.txt`
  } else if (renderResult.value) {
    if (activeOutputTab.value === 'yaml') {
      content = renderResult.value.rendered_yaml || ''
      filename = `${activeTemplate.value?.name || 'manifest'}.yaml`
    } else if (activeOutputTab.value === 'compose') {
      content = renderResult.value.rendered_compose || ''
      filename = 'docker-compose.yml'
    } else if (activeOutputTab.value === 'helm') {
      content = renderResult.value.rendered_helm || ''
      filename = 'values.yaml'
    }
  }
  if (content) {
    downloadFile(content, filename)
  }
}
</script>

<template>
  <div class="scaffolder-view">
    <!-- Toast Notification -->
    <transition name="toast">
      <div v-if="toastMessage" :class="['toast-banner', `toast-${toastMessage.type}`]">
        <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" class="toast-icon" />
        <span>{{ toastMessage.text }}</span>
      </div>
    </transition>

    <!-- Mobile 40-44px Command Bar (<768px) -->
    <div class="scaffolder-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="layers" size="xs" /> Scaffolder ({{ filteredTemplates.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          type="button"
          class="btn-icon-cmd"
          title="Register Template"
          aria-label="Register Template"
          @click="openCustomTemplateModal('create')"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          type="button"
          class="btn-icon-cmd"
          title="Sync Templates"
          aria-label="Sync Templates"
          :disabled="loading"
          @click="loadTemplates"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': loading }" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="scaffolder-micro-telemetry mobile-only font-mono" role="status" aria-label="Scaffolder Micro Telemetry">
      <span class="tel-item tel-tmpl"><BaseIcon name="layers" size="xs" /> {{ filteredTemplates.length }} Templates</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-deploy"><BaseIcon name="zap" size="xs" /> {{ rendering ? 'Executing' : 'Ready' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-verified"><BaseIcon name="shield" size="xs" /> Verified</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-cats"><BaseIcon name="folder" size="xs" /> {{ filterCategories.length }} Categories</span>
    </div>

    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="scaffolder-toolbar-sleek glass-panel">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search templates, tags..."
          class="toolbar-search-input"
          aria-label="Search templates by title, description, tags"
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

      <!-- Category Filter Pills (Desktop) -->
      <div class="toolbar-category-pills desktop-only" role="tablist" aria-label="Template categories">
        <button
          v-for="cat in filterCategories"
          :key="cat.key"
          type="button"
          role="tab"
          :aria-selected="activeCategory === cat.key"
          class="toolbar-pill-btn"
          :class="{ active: activeCategory === cat.key }"
          @click="activeCategory = cat.key"
        >
          <BaseIcon v-if="cat.icon" :name="cat.icon" size="xs" />
          <span>{{ cat.label }}</span>
        </button>
      </div>

      <!-- Category Filter Dropdown (Tablet / Mobile Fallback) -->
      <select
        v-model="activeCategory"
        class="toolbar-select category-dropdown"
        aria-label="Filter templates by category"
      >
        <option v-for="cat in filterCategories" :key="cat.key" :value="cat.key">
          {{ cat.label }}
        </option>
      </select>

      <!-- Inline compact template count font-mono -->
      <div class="toolbar-kpi-strip font-mono desktop-only" role="status" aria-label="Template count">
        <span class="kpi-badge font-mono">{{ filteredTemplates.length }} Templates</span>
      </div>

      <!-- Action Buttons: + Custom Template & Refresh -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="btn btn-primary toolbar-btn"
          title="Create Custom Template"
          aria-label="Create Custom Template"
          @click="openCustomTemplateModal('create')"
        >
          <BaseIcon name="plus" size="xs" />
          <span>Custom Template</span>
        </button>
        <button
          type="button"
          class="btn btn-secondary toolbar-btn"
          title="Refresh Templates"
          aria-label="Refresh Templates"
          :disabled="loading"
          @click="loadTemplates"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': loading }" />
          <span>Refresh</span>
        </button>
      </div>
    </div>

    <!-- Desktop Grid Gallery View -->
    <div class="desktop-view">
      <ScaffolderTemplatesGrid
        :templates="filteredTemplates"
        :loading="loading"
        :deleting="deleting"
        @deploy="openWizard"
        @preview="previewTemplate"
        @edit="openCustomTemplateModal('edit', $event)"
        @delete="handleDeleteCustomTemplate"
        @reset-filters="handleResetFilters"
      />
    </div>

    <!-- Mobile Card Stream View (Touch-Optimized) -->
    <div class="mobile-view">
      <ScaffolderMobileCards
        :templates="filteredTemplates"
        :loading="loading"
        :deleting="deleting"
        @deploy="openWizard"
        @preview="previewTemplate"
        @edit="openCustomTemplateModal('edit', $event)"
        @delete="handleDeleteCustomTemplate"
        @reset-filters="handleResetFilters"
      />
    </div>

    <!-- Multi-Step Wizard Modal -->
    <ScaffolderStepWizard
      :show="showWizardModal"
      :active-template="activeTemplate"
      :current-step="currentStep"
      :form-variables="formVariables"
      :repo-config="repoConfig"
      :cicd-config="cicdConfig"
      :register-in-catalog="registerInCatalog"
      :owner-team="ownerTeam"
      :owner-email="ownerEmail"
      :rendering="rendering"
      @close="closeWizard"
      @update:current-step="goToStep"
      @update:register-in-catalog="registerInCatalog = $event"
      @update:owner-team="ownerTeam = $event"
      @update:owner-email="ownerEmail = $event"
      @next-step="nextStep"
      @prev-step="prevStep"
      @submit="triggerScaffoldJob"
    />

    <!-- Logs & Dry-Run Manifest Drawer -->
    <ScaffolderLogsDrawer
      :show="showLogsDrawer"
      :active-template="activeTemplate"
      :render-result="renderResult"
      :active-output-tab="activeOutputTab"
      :copy-success="copySuccess"
      :logs="logs"
      :is-dry-run="isDryRun"
      @close="showLogsDrawer = false"
      @update:active-output-tab="activeOutputTab = $event"
      @copy="handleDrawerCopy"
      @download="handleDrawerDownload"
    />

    <!-- Custom Template Registration Modal -->
    <RegisterTemplateModal
      :show="showCustomModal"
      :mode="customModalMode"
      :template-form="customTemplate"
      :saving="saving"
      @close="showCustomModal = false"
      @save="handleSaveCustomTemplate"
      @add-variable="addVariableToCustomTemplate"
      @remove-variable="removeVariableFromCustomTemplate"
    />
  </div>
</template>

<style>
@import '../assets/styles/views/scaffolder.css';
</style>
