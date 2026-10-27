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
import HelmCatalogToolbar from '../components/helm/HelmCatalogToolbar.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'

const {
  loading, error, toastMessage, activeTab, clusters, selectedCluster, namespaces, selectedNamespace,
  releases, releaseSearch, releaseStatusFilter, showDetailDrawer, selectedRelease, loadingReleaseDetail,
  activeDrawerTab, releaseHistory, loadingHistory, valuesCopied, manifestCopied,
  showUpgradeModal, upgradeTarget, upgradeForm, upgrading,
  showRollbackModal, rollbackTarget, rollbackRevision, rollbackHistoryList, loadingRollbackHistory, rollingBack,
  showUninstallModal, uninstallTarget, uninstalling,
  charts, chartSearch, selectedRepoFilter, selectedCategoryTag, loadingCharts,
  showInstallModal, selectedChartForInstall, installStep, installForm, installing, installError,
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
      <div v-if="toastMessage" class="helm-toast glass-panel" :class="`toast-${toastMessage.type}`">
        <BaseIcon :name="toastMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="sm" class="toast-icon" />
        <span class="toast-text">{{ toastMessage.text }}</span>
      </div>
    </Transition>

    <HelmCatalogToolbar
      v-model:active-tab="activeTab"
      v-model:search-query="searchQuery"
      v-model:release-status-filter="releaseStatusFilter"
      v-model:selected-repo-filter="selectedRepoFilter"
      v-model:selected-cluster="selectedCluster"
      v-model:selected-namespace="selectedNamespace"
      :releases-count="releases.length"
      :charts-count="charts.length"
      :repos-count="repos.length"
      :total-releases-count="totalReleasesCount"
      :deployed-rate="deployedRate"
      :search-placeholder="searchPlaceholder"
      :repos="repos"
      :loading="loading || loadingCharts || loadingRepos"
      :clusters="clusters"
      :namespaces="namespaces"
      @add-repo="openAddRepoModal"
      @refresh="refreshActiveTab"
    />

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
          <button type="button" class="btn btn-secondary" :disabled="updatingAllRepos" @click="handleUpdateAllRepos">
            <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': updatingAllRepos }" />
            <span>{{ updatingAllRepos ? 'Updating Indexes...' : 'Update All Repos' }}</span>
          </button>
          <button type="button" class="btn btn-primary" @click="openAddRepoModal">
            <span>+ Add Repository</span>
          </button>
        </div>
      </div>

      <div class="data-table-container glass-panel">
        <div v-if="loadingRepos" class="loading-state">
          <div class="helm-spinner"></div>
          <p class="font-mono text-muted">Loading Helm repository indexes...</p>
        </div>
        <div v-else-if="repos.length === 0" class="empty-state">
          <BaseIcon name="database" size="xl" class="empty-icon" />
          <h4 class="empty-title">No Repositories Configured</h4>
          <p class="empty-desc">Add your first Helm repository to start discovering charts.</p>
          <button type="button" class="btn btn-primary" @click="openAddRepoModal">+ Add Repository</button>
        </div>
        <div v-else class="table-scroll-wrapper desktop-only">
          <table class="helm-table data-table">
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
                    <button type="button" class="btn-row-action btn-action-upgrade" title="Sync repository index" :disabled="updatingAllRepos" @click="handleUpdateAllRepos">
                      <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': updatingAllRepos }" /> <span>Sync</span>
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
      :install-error="installError"
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

<style scoped>
@import '../assets/styles/views/helm.css';
@import '../assets/styles/views/helm-catalog-grid.css';
@import '../assets/styles/components/helm-drawers.css';
</style>