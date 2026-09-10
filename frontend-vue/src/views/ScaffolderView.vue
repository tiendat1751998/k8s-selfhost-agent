<script setup lang="ts">
import { useScaffolder } from '../composables/useScaffolder'
import ScaffolderTemplatesGrid from '../components/scaffolder/ScaffolderTemplatesGrid.vue'
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
  selectedCategory,
  searchQuery,
  filteredTemplates,
  categories,

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
        <span class="toast-icon">{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
        <span>{{ toastMessage.text }}</span>
      </div>
    </transition>

    <!-- Mobile 40-44px Command Bar (<768px) -->
    <div class="scaffolder-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🏗️ Scaffolder ({{ templates.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          type="button"
          class="btn-icon-cmd"
          title="Register Template"
          aria-label="Register Template"
          @click="openCustomTemplateModal('create')"
        >
          <span>➕</span>
        </button>
        <button
          type="button"
          class="btn-icon-cmd"
          title="Sync Templates"
          aria-label="Sync Templates"
          :disabled="loading"
          @click="loadTemplates"
        >
          <span :class="{ 'spin-anim': loading }">🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="scaffolder-micro-telemetry mobile-only font-mono" role="status" aria-label="Scaffolder Micro Telemetry">
      <span class="tel-item tel-tmpl">🏗️ {{ templates.length }} Templates</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-deploy">⚡ {{ rendering ? 'Executing' : 'Ready' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-verified">🛡️ Verified</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-cats">📁 {{ categories.length }} Categories</span>
    </div>

    <!-- Header & Hero Section (Desktop View) -->
    <header class="page-header glass-panel">
      <div class="header-content desktop-header desktop-only">
        <div class="header-left">
          <div class="header-icon-badge">🏗️</div>
          <div>
            <h1 class="header-title">Application Scaffolder</h1>
            <p class="header-sub">
              1-Click Deploy & Template Engine • Generates Cloud-Native K8s Manifests, Helm Charts & Compose Files
            </p>
          </div>
        </div>
        <div class="header-actions">
          <button class="btn btn-primary" @click="openCustomTemplateModal('create')">
            <span class="btn-icon">➕</span> Create Template
          </button>
          <button class="btn btn-secondary" :disabled="loading" @click="loadTemplates">
            <span class="btn-icon" :class="{ 'spin-anim': loading }">🔄</span> Refresh
          </button>
        </div>
      </div>

      <!-- Controls: Category Filter Tabs + Search -->
      <div class="filter-bar">
        <div class="category-tabs">
          <button
            v-for="cat in categories"
            :key="cat.key"
            :class="['tab-btn', { active: selectedCategory === cat.key }]"
            @click="selectedCategory = cat.key"
          >
            <span class="tab-icon">{{ cat.icon }}</span>
            <span class="tab-label">{{ cat.label }}</span>
          </button>
        </div>

        <div class="search-box">
          <span class="search-icon">🔍</span>
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search templates, frameworks, tags..."
            class="search-input"
          />
          <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''">✕</button>
        </div>
      </div>
    </header>

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
        @reset-filters="resetFilters"
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
        @reset-filters="resetFilters"
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
