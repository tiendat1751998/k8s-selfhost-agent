<script setup lang="ts">
import { computed } from 'vue'
import { useHelmCatalog } from '../composables/useHelmCatalog'
import { sanitizeHelmRelease } from '../composables/useHelm'
import HelmChartsGrid from '../components/helm/HelmChartsGrid.vue'
import HelmReleasesTable from '../components/helm/HelmReleasesTable.vue'
import HelmInstallDrawer from '../components/helm/HelmInstallDrawer.vue'
import HelmReleaseDetailsDrawer from '../components/helm/HelmReleaseDetailsDrawer.vue'
import HelmRepositoryModal from '../components/helm/HelmRepositoryModal.vue'
import HelmMobileCards from '../components/helm/HelmMobileCards.vue'
import HelmActionModals from '../components/helm/HelmActionModals.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'

const {
  loading, error, toastMessage, activeTab, clusters, selectedCluster, namespaces, selectedNamespace,
  releases, releaseSearch, releaseStatusFilter, showDetailDrawer, selectedRelease, loadingReleaseDetail,
  activeDrawerTab, releaseHistory, loadingHistory, valuesCopied, manifestCopied,
  showUpgradeModal, upgradeTarget, upgradeForm, upgrading,
  showRollbackModal, rollbackTarget, rollbackRevision, rollbackHistoryList, loadingRollbackHistory, rollingBack,
  showUninstallModal, uninstallTarget, uninstalling,
  charts, chartSearch, selectedRepoFilter, selectedCategoryTag, loadingCharts,
  showInstallModal, selectedChartForInstall, installStep, installForm, installing,
  repos, loadingRepos, showAddRepoModal, addRepoForm, addingRepo, updatingAllRepos, showRemoveRepoModal, repoToRemove, removingRepo,
  repoPresets, categoryTags,
  totalReleasesCount, deployedRate, filteredReleases, filteredCharts,
  fetchReleases, refreshActiveTab, onChartSearchInput,
  openReleaseDetail, openUpgradeModal, handleUpgradeRelease, openRollbackModal, handleRollbackRelease, promptUninstall, handleUninstallRelease,
  openInstallWizard, handleInstallChart, openAddRepoModal, applyRepoPreset, handleAddRepo, handleUpdateAllRepos, promptRemoveRepo, handleRemoveRepo,
  copyToClipboard, downloadAsFile,
} = useHelmCatalog()

const searchQuery = computed({
  get() {
    if (activeTab.value === 'charts') return chartSearch.value
    return releaseSearch.value
  },
  set(val: string) {
    if (activeTab.value === 'charts') {
      chartSearch.value = val
      onChartSearchInput()
    } else {
      releaseSearch.value = val
    }
  }
})

const searchPlaceholder = computed(() => {
  if (activeTab.value === 'releases') return 'Filter releases...'
  if (activeTab.value === 'charts') return 'Search charts...'
  return 'Search repos...'
})

const cleanFilteredReleases = computed(() => {
  return filteredReleases.value.map(r => sanitizeHelmRelease(r))
})

const cleanSelectedRelease = computed(() => {
  return selectedRelease.value ? sanitizeHelmRelease(selectedRelease.value) : null
})

const filteredRepos = computed(() => {
  if (!searchQuery.value || activeTab.value !== 'repos') return repos.value
  const q = searchQuery.value.toLowerCase()
  return repos.value.filter(r => r.name.toLowerCase().includes(q) || (r.url && r.url.toLowerCase().includes(q)))
})
</script>

<template>
  <div class="helm-catalog-layout helm-view animate-fade-in">
    <!-- Toast Notification -->
    <Transition name="toast-slide">
      <div v-if="toastMessage" class="cyber-toast" :class="`toast-${toastMessage.type}`">
        <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="sm" class="toast-icon" />
        <span class="toast-text">{{ toastMessage.text }}</span>
      </div>
    </Transition>

    <!-- Unified 42px Sleek Enterprise Toolbar (Desktop Only) -->
    <div class="helm-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="Helm Enterprise Toolbar">
      <!-- Left: Segmented capsule tabs -->
      <div class="toolbar-capsule-pills" role="tablist" aria-label="Helm Catalog Sections">
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'releases'"
          class="capsule-pill font-mono"
          :class="{ active: activeTab === 'releases' }"
          @click="activeTab = 'releases'"
        >
          <BaseIcon name="anchor" size="xs" class="capsule-icon" />
          <span>Releases</span>
          <span class="capsule-count">({{ releases.length }})</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'charts'"
          class="capsule-pill font-mono"
          :class="{ active: activeTab === 'charts' }"
          @click="activeTab = 'charts'"
        >
          <BaseIcon name="package" size="xs" class="capsule-icon" />
          <span>Chart Catalog</span>
          <span class="capsule-count">({{ charts.length }})</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'repos'"
          class="capsule-pill font-mono"
          :class="{ active: activeTab === 'repos' }"
          @click="activeTab = 'repos'"
        >
          <BaseIcon name="database" size="xs" class="capsule-icon" />
          <span>Repositories</span>
          <span class="capsule-count">({{ repos.length }})</span>
        </button>
      </div>

      <!-- Center-Left: Standard 30px Capsule Pill search input -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="searchPlaceholder"
          class="toolbar-search-input"
          aria-label="Filter or search Helm resources"
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

      <!-- Center-Right: Contextual filter -->
      <div class="toolbar-context-filter">
        <select
          v-if="activeTab === 'releases'"
          v-model="releaseStatusFilter"
          class="toolbar-select font-mono"
          aria-label="Filter Releases by Status"
        >
          <option value="all">All Statuses ({{ releases.length }})</option>
          <option value="deployed">Deployed</option>
          <option value="failed">Failed</option>
          <option value="pending">Pending</option>
          <option value="superseded">Superseded</option>
        </select>
        <select
          v-else-if="activeTab === 'charts'"
          v-model="selectedRepoFilter"
          class="toolbar-select font-mono"
          aria-label="Filter Charts by Repository"
        >
          <option value="all">All Repositories ({{ repos.length }})</option>
          <option v-for="repo in repos" :key="repo.name" :value="repo.name">
            {{ repo.name }}
          </option>
        </select>
      </div>

      <!-- Right: Action buttons -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="toolbar-btn btn-primary"
          title="Add Helm Repository"
          @click="openAddRepoModal"
        >
          <BaseIcon name="plus" size="xs" />
          <span>Add Repo</span>
        </button>
        <button
          type="button"
          class="toolbar-btn btn-secondary toolbar-btn-icon"
          :disabled="loading || loadingCharts || loadingRepos"
          title="Refresh view"
          aria-label="Refresh view"
          @click="refreshActiveTab"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': loading || loadingCharts || loadingRepos }" />
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<=640px) -->
    <div class="helm-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="anchor" size="sm" /> Helm ({{ releases.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button type="button" class="btn-icon-cmd" title="Add Repository" aria-label="Add Repository" @click="openAddRepoModal">
          <BaseIcon name="plus" size="xs" />
        </button>
        <button type="button" class="btn-icon-cmd" :disabled="loading || loadingCharts || loadingRepos" title="Refresh" aria-label="Refresh" @click="refreshActiveTab">
          <BaseIcon name="refresh" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<=640px) -->
    <div class="helm-micro-telemetry mobile-only font-mono" role="status" aria-label="Helm Micro Telemetry">
      <span class="tel-item tel-rel"><BaseIcon name="anchor" size="xs" /> {{ totalReleasesCount }} rel</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-ok"><BaseIcon name="shield" size="xs" /> {{ deployedRate }} ok</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-charts"><BaseIcon name="package" size="xs" /> {{ charts.length }} charts</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-repos"><BaseIcon name="database" size="xs" /> {{ repos.length }} repos</span>
    </div>

    <!-- Slim Mobile Cluster & Namespace Pill Row (<=640px) -->
    <div class="helm-mobile-cluster-bar helm-mobile-selectors mobile-only">
      <div class="pill-select-wrap">
        <BaseIcon name="anchor" size="xs" class="pill-prefix" />
        <select v-model="selectedCluster" class="pill-select" aria-label="Target Cluster">
          <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">{{ c.name || c.id }}</option>
          <option v-if="clusters.length === 0" value="primary-cluster">primary-cluster</option>
        </select>
      </div>
      <div class="pill-select-wrap">
        <BaseIcon name="grid" size="xs" class="pill-prefix" />
        <select v-model="selectedNamespace" class="pill-select" aria-label="Namespace Scope">
          <option value="all">all namespaces</option>
          <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">{{ ns.name }}</option>
        </select>
      </div>
    </div>

    <!-- Mobile Section Tabs Navigation (<768px) -->
    <nav class="catalog-tabs-bar mobile-only glass-panel" aria-label="Helm Catalog Sections">
      <div class="tabs-list">
        <button type="button" class="tab-btn" :class="{ active: activeTab === 'releases' }" @click="activeTab = 'releases'">
          <BaseIcon name="anchor" size="xs" class="tab-icon" /><span class="tab-label">Releases</span><span class="tab-counter">{{ releases.length }}</span>
        </button>
        <button type="button" class="tab-btn" :class="{ active: activeTab === 'charts' }" @click="activeTab = 'charts'">
          <BaseIcon name="package" size="xs" class="tab-icon" /><span class="tab-label">Charts</span><span class="tab-counter">{{ charts.length }}</span>
        </button>
        <button type="button" class="tab-btn" :class="{ active: activeTab === 'repos' }" @click="activeTab = 'repos'">
          <BaseIcon name="database" size="xs" class="tab-icon" /><span class="tab-label">Repos</span><span class="tab-counter">{{ repos.length }}</span>
        </button>
      </div>
    </nav>

    <!-- Tab 1: Releases -->
    <section v-if="activeTab === 'releases'" class="tab-content">
      <div class="desktop-only">
        <HelmReleasesTable
          :releases="cleanFilteredReleases"
          :loading="loading"
          :error="error"
          :search="releaseSearch"
          :status-filter="releaseStatusFilter"
          :selected-cluster="selectedCluster"
          @update:search="releaseSearch = $event"
          @update:status-filter="releaseStatusFilter = $event"
          @open-detail="openReleaseDetail"
          @inspect="openReleaseDetail"
          @upgrade="openUpgradeModal"
          @rollback="openRollbackModal"
          @uninstall="promptUninstall"
          @retry="fetchReleases"
          @browse-charts="activeTab = 'charts'"
        />
      </div>
      <div class="mobile-only">
        <HelmMobileCards
          active-tab="releases"
          :releases="cleanFilteredReleases"
          :charts="charts"
          :repos="repos"
          :loading="loading"
          @open-detail="openReleaseDetail"
          @upgrade="openUpgradeModal"
          @rollback="openRollbackModal"
          @uninstall="promptUninstall"
        />
      </div>
    </section>

    <!-- Tab 2: Chart Catalog -->
    <section v-if="activeTab === 'charts'" class="tab-content">
      <div class="desktop-only">
        <HelmChartsGrid
          :charts="filteredCharts"
          :repos="repos"
          :loading="loadingCharts"
          :search="chartSearch"
          :selected-repo="selectedRepoFilter"
          :selected-category="selectedCategoryTag"
          :category-tags="categoryTags"
          @update:search="chartSearch = $event"
          @update:selected-repo="selectedRepoFilter = $event"
          @update:selected-category="selectedCategoryTag = $event"
          @search-input="onChartSearchInput"
          @install="openInstallWizard"
          @add-repo="openAddRepoModal"
        />
      </div>
      <div class="mobile-only">
        <HelmMobileCards
          active-tab="charts"
          :releases="filteredReleases"
          :charts="filteredCharts"
          :repos="repos"
          :loading="loadingCharts"
          @install-chart="openInstallWizard"
        />
      </div>
    </section>

    <!-- Tab 3: Repositories -->
    <section v-if="activeTab === 'repos'" class="tab-content">
      <div class="repos-toolbar glass-panel">
        <div class="repos-title-wrap">
          <h3 class="section-title">Configured Helm Repositories</h3>
          <p class="section-subtitle">Manage upstream OCI and HTTP chart registry sources</p>
        </div>
        <div class="repos-actions">
          <button type="button" class="btn-cyber btn-outline-cyan" :disabled="updatingAllRepos" @click="handleUpdateAllRepos">
            <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': updatingAllRepos }" />
            <span>{{ updatingAllRepos ? 'Updating Indexes...' : 'Update All Repos' }}</span>
          </button>
          <button type="button" class="btn-cyber btn-primary" @click="openAddRepoModal">
            <span>+ Add Repository</span>
          </button>
        </div>
      </div>

      <div class="data-table-container glass-panel">
        <div v-if="loadingRepos" class="loading-state">
          <div class="cyber-spinner"></div>
          <p class="font-mono text-muted">Loading Helm repository indexes...</p>
        </div>
        <div v-else-if="repos.length === 0" class="empty-state">
          <BaseIcon name="database" size="xl" class="empty-icon" />
          <h4 class="empty-title">No Repositories Configured</h4>
          <p class="empty-desc">Add your first Helm repository to start discovering charts.</p>
          <button type="button" class="btn-cyber btn-primary" @click="openAddRepoModal">+ Add Repository</button>
        </div>
        <div v-else class="table-scroll-wrapper desktop-only">
          <table class="cyber-table">
            <thead>
              <tr><th>Repository Name</th><th>Repository URL</th><th class="text-right">Actions</th></tr>
            </thead>
            <tbody>
              <tr v-for="repo in filteredRepos" :key="repo.name">
                <td>
                  <div class="repo-name-cell">
                    <BaseIcon name="database" size="xs" class="repo-icon" />
                    <strong class="font-mono text-primary">{{ repo.name }}</strong>
                  </div>
                </td>
                <td>
                  <div class="repo-url-cell">
                    <a :href="repo.url" target="_blank" rel="noopener noreferrer" class="repo-link font-mono">{{ repo.url }}</a>
                    <button type="button" class="btn-copy-icon" title="Copy URL" @click="copyToClipboard(repo.url, 'url')">
                      <BaseIcon name="copy" size="xs" />
                    </button>
                  </div>
                </td>
                <td class="text-right actions-cell">
                  <div class="action-btn-group">
                    <button type="button" class="btn-row-action btn-action-upgrade" title="Update index" @click="handleUpdateAllRepos">
                      <BaseIcon name="refresh" size="xs" /> <span>Update</span>
                    </button>
                    <button type="button" class="btn-row-action btn-action-delete" title="Remove repository" @click="promptRemoveRepo(repo)">
                      <BaseIcon name="trash" size="xs" /> <span>Remove</span>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mobile-only">
          <HelmMobileCards
            active-tab="repos"
            :releases="filteredReleases"
            :charts="filteredCharts"
            :repos="filteredRepos"
            :loading="loadingRepos"
            @copy-url="copyToClipboard($event, 'url')"
            @remove-repo="promptRemoveRepo"
          />
        </div>
      </div>
    </section>

    <!-- Drawers & Modals -->
    <HelmReleaseDetailsDrawer
      :show="showDetailDrawer"
      :release="cleanSelectedRelease"
      :loading-detail="loadingReleaseDetail"
      :active-drawer-tab="activeDrawerTab"
      :release-history="releaseHistory"
      :loading-history="loadingHistory"
      :values-copied="valuesCopied"
      :manifest-copied="manifestCopied"
      @close="showDetailDrawer = false"
      @update:active-drawer-tab="activeDrawerTab = $event"
      @upgrade="openUpgradeModal"
      @rollback="openRollbackModal"
      @uninstall="promptUninstall"
      @rollback-revision="rollbackRevision = $event; showRollbackModal = true"
      @copy-notes="copyToClipboard($event, 'url')"
      @copy-values="copyToClipboard($event, 'values')"
      @download-values="downloadAsFile"
      @copy-manifest="copyToClipboard($event, 'manifest')"
      @download-manifest="downloadAsFile"
    />

    <HelmInstallDrawer
      :show="showInstallModal"
      :chart="selectedChartForInstall"
      :namespaces="namespaces"
      :selected-cluster="selectedCluster"
      :install-step="installStep"
      :install-form="installForm"
      :installing="installing"
      @close="showInstallModal = false"
      @step-change="installStep = $event"
      @reset-values="selectedChartForInstall && openInstallWizard(selectedChartForInstall)"
      @install="handleInstallChart"
    />

    <HelmRepositoryModal
      :show="showAddRepoModal"
      :add-repo-form="addRepoForm"
      :adding-repo="addingRepo"
      :repo-presets="repoPresets"
      @close="showAddRepoModal = false"
      @apply-preset="applyRepoPreset"
      @add-repo="handleAddRepo"
    />

    <HelmActionModals
      :show-upgrade-modal="showUpgradeModal"
      :upgrade-target="upgradeTarget"
      :upgrade-form="upgradeForm"
      :upgrading="upgrading"
      :show-rollback-modal="showRollbackModal"
      :rollback-target="rollbackTarget"
      :rollback-revision="rollbackRevision"
      :rollback-history-list="rollbackHistoryList"
      :loading-rollback-history="loadingRollbackHistory"
      :rolling-back="rollingBack"
      :show-uninstall-modal="showUninstallModal"
      :uninstall-target="uninstallTarget"
      :uninstalling="uninstalling"
      :show-remove-repo-modal="showRemoveRepoModal"
      :repo-to-remove="repoToRemove"
      :removing-repo="removingRepo"
      @update:show-upgrade-modal="showUpgradeModal = $event"
      @update:show-rollback-modal="showRollbackModal = $event"
      @update:show-uninstall-modal="showUninstallModal = $event"
      @update:show-remove-repo-modal="showRemoveRepoModal = $event"
      @update:rollback-revision="rollbackRevision = $event"
      @upgrade="handleUpgradeRelease"
      @rollback="handleRollbackRelease"
      @uninstall="handleUninstallRelease"
      @remove-repo="handleRemoveRepo"
    />
  </div>
</template>

<style>
@import '../assets/styles/views/helm.css';
@import '../assets/styles/views/helm-catalog-grid.css';
@import '../assets/styles/components/helm-drawers.css';
</style>