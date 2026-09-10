<script setup lang="ts">
import { computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import { useHelmCatalog } from '../composables/useHelmCatalog'
import { sanitizeHelmRelease } from '../composables/useHelm'
import HelmChartsGrid from '../components/helm/HelmChartsGrid.vue'
import HelmReleasesTable from '../components/helm/HelmReleasesTable.vue'
import HelmInstallDrawer from '../components/helm/HelmInstallDrawer.vue'
import HelmReleaseDetailsDrawer from '../components/helm/HelmReleaseDetailsDrawer.vue'
import HelmRepositoryModal from '../components/helm/HelmRepositoryModal.vue'
import HelmMobileCards from '../components/helm/HelmMobileCards.vue'
import HelmActionModals from '../components/helm/HelmActionModals.vue'

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
  totalReleasesCount, deployedReleasesCount, failedReleasesCount, deployedRate, filteredReleases, filteredCharts,
  fetchReleases, refreshActiveTab, onChartSearchInput,
  openReleaseDetail, openUpgradeModal, handleUpgradeRelease, openRollbackModal, handleRollbackRelease, promptUninstall, handleUninstallRelease,
  openInstallWizard, handleInstallChart, openAddRepoModal, applyRepoPreset, handleAddRepo, handleUpdateAllRepos, promptRemoveRepo, handleRemoveRepo,
  copyToClipboard, downloadAsFile,
} = useHelmCatalog()

const cleanFilteredReleases = computed(() => {
  return filteredReleases.value.map(r => sanitizeHelmRelease(r))
})

const cleanSelectedRelease = computed(() => {
  return selectedRelease.value ? sanitizeHelmRelease(selectedRelease.value) : null
})
</script>

<template>
  <div class="helm-catalog-layout animate-fade-in">
    <!-- Toast Notification -->
    <Transition name="toast-slide">
      <div v-if="toastMessage" class="cyber-toast" :class="`toast-${toastMessage.type}`">
        <span class="toast-icon">{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
        <span class="toast-text">{{ toastMessage.text }}</span>
      </div>
    </Transition>

    <!-- Page Header (Desktop only) -->
    <header class="page-header desktop-header glass-panel glass-panel-glow">
      <div class="header-main">
        <div class="title-group">
          <div class="badge-title-row">
            <span class="pulse-dot pulse-dot-gold"></span>
            <span class="cyber-tag">HELM ENGINE v3</span>
          </div>
          <h1 class="page-title title-full">Helm Application Catalog</h1>
          <h1 class="page-title title-compact">Helm Catalog</h1>
          <p class="page-subtitle">Browse repositories, deploy pre-packaged cloud-native charts & manage lifecycle releases</p>
        </div>

        <div class="header-controls">
          <div class="control-box">
            <label class="control-label">Target Cluster</label>
            <select v-model="selectedCluster" class="input-glass header-select cluster-select">
              <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">☸️ {{ c.name || c.id }}</option>
              <option v-if="clusters.length === 0" value="primary-cluster">☸️ primary-cluster</option>
            </select>
          </div>

          <div class="control-box">
            <label class="control-label">Namespace Scope</label>
            <select v-model="selectedNamespace" class="input-glass header-select ns-select">
              <option value="all">🌐 All Namespaces</option>
              <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">🏷️ {{ ns.name }}</option>
            </select>
          </div>

          <div class="header-actions-group">
            <button type="button" class="btn-cyber btn-primary" @click="openAddRepoModal"><span>+ Add Repo</span></button>
            <button type="button" class="btn-cyber btn-secondary" :disabled="loading || loadingCharts || loadingRepos" title="Refresh current view" @click="refreshActiveTab">
              <span>🔄 Refresh</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Metrics Row -->
      <div class="metrics-grid">
        <MetricCard title="Total Releases" :value="totalReleasesCount" :subtitle="`${deployedReleasesCount} Deployed • ${failedReleasesCount} Failed`" icon="⛵" badge="Live" badge-color="emerald" />
        <MetricCard title="Deployed Health" :value="deployedRate" subtitle="Successful rollout ratio" icon="🟢" :trend="failedReleasesCount > 0 ? `${failedReleasesCount} Degraded` : '100% Healthy'" :trend-type="failedReleasesCount > 0 ? 'negative' : 'positive'" />
        <MetricCard title="Catalog Charts" :value="charts.length" subtitle="Available packages across repos" icon="📦" badge="Searchable" badge-color="cyan" />
        <MetricCard title="Helm Repos" :value="repos.length" subtitle="Configured chart repositories" icon="🗄️" badge="Sync Ready" badge-color="violet" />
      </div>
    </header>

    <!-- Mobile 40px Command Bar (<=640px) -->
    <div class="helm-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">⛵ Helm ({{ releases.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button type="button" class="btn-icon-cmd" title="Add Repository" aria-label="Add Repository" @click="openAddRepoModal">
          <span>➕</span>
        </button>
        <button type="button" class="btn-icon-cmd" :disabled="loading || loadingCharts || loadingRepos" title="Refresh" aria-label="Refresh" @click="refreshActiveTab">
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<=640px) -->
    <div class="helm-micro-telemetry mobile-only font-mono" role="status" aria-label="Helm Micro Telemetry">
      <span class="tel-item tel-rel">⛵ {{ totalReleasesCount }} rel</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-ok">🛡️ {{ deployedRate }} ok</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-charts">📦 {{ charts.length }} charts</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-repos">🗄️ {{ repos.length }} repos</span>
    </div>

    <!-- Slim Mobile Cluster & Namespace Pill Row (<=640px) -->
    <div class="helm-mobile-cluster-bar helm-mobile-selectors mobile-only">
      <div class="pill-select-wrap">
        <span class="pill-prefix">☸️</span>
        <select v-model="selectedCluster" class="pill-select" aria-label="Target Cluster">
          <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">{{ c.name || c.id }}</option>
          <option v-if="clusters.length === 0" value="primary-cluster">primary-cluster</option>
        </select>
      </div>
      <div class="pill-select-wrap">
        <span class="pill-prefix">🏷️</span>
        <select v-model="selectedNamespace" class="pill-select" aria-label="Namespace Scope">
          <option value="all">all namespaces</option>
          <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">{{ ns.name }}</option>
        </select>
      </div>
    </div>

    <!-- Navigation Tabs Bar -->
    <nav class="catalog-tabs-bar glass-panel" aria-label="Helm Catalog Sections">
      <div class="tabs-list">
        <button type="button" class="tab-btn" :class="{ active: activeTab === 'releases' }" @click="activeTab = 'releases'">
          <span class="tab-icon">⛵</span><span class="tab-label">Releases</span><span class="tab-counter">{{ releases.length }}</span>
        </button>
        <button type="button" class="tab-btn" :class="{ active: activeTab === 'charts' }" @click="activeTab = 'charts'">
          <span class="tab-icon">📦</span><span class="tab-label">Chart Catalog</span><span class="tab-counter">{{ charts.length }}</span>
        </button>
        <button type="button" class="tab-btn" :class="{ active: activeTab === 'repos' }" @click="activeTab = 'repos'">
          <span class="tab-icon">🗄️</span><span class="tab-label">Repositories</span><span class="tab-counter">{{ repos.length }}</span>
        </button>
      </div>

      <div class="tabs-extra">
        <button v-if="activeTab === 'repos'" type="button" class="btn-cyber btn-outline-cyan btn-sm" :disabled="updatingAllRepos" @click="handleUpdateAllRepos">
          <span :class="{ 'spin-anim': updatingAllRepos }">🔄</span><span>{{ updatingAllRepos ? 'Syncing Repos...' : 'Update All Repos' }}</span>
        </button>
        <span v-else class="text-muted font-mono font-xs">Cluster: <strong class="text-gold">{{ selectedCluster }}</strong></span>
      </div>
    </nav>

    <!-- Tab 1: Releases -->
    <section v-if="activeTab === 'releases'" class="tab-content">
      <div class="desktop-only">
        <HelmReleasesTable
          :releases="cleanFilteredReleases" :loading="loading" :error="error" :search="releaseSearch"
          :status-filter="releaseStatusFilter" :selected-cluster="selectedCluster"
          @update:search="releaseSearch = $event" @update:status-filter="releaseStatusFilter = $event"
          @open-detail="openReleaseDetail" @upgrade="openUpgradeModal" @rollback="openRollbackModal"
          @uninstall="promptUninstall" @retry="fetchReleases" @browse-charts="activeTab = 'charts'"
        />
      </div>
      <div class="mobile-only">
        <HelmMobileCards
          active-tab="releases" :releases="cleanFilteredReleases" :charts="charts" :repos="repos" :loading="loading"
          @open-detail="openReleaseDetail" @upgrade="openUpgradeModal" @rollback="openRollbackModal" @uninstall="promptUninstall"
        />
      </div>
    </section>

    <!-- Tab 2: Chart Catalog -->
    <section v-if="activeTab === 'charts'" class="tab-content">
      <div class="desktop-only">
        <HelmChartsGrid
          :charts="filteredCharts" :repos="repos" :loading="loadingCharts" :search="chartSearch"
          :selected-repo="selectedRepoFilter" :selected-category="selectedCategoryTag" :category-tags="categoryTags"
          @update:search="chartSearch = $event" @update:selected-repo="selectedRepoFilter = $event"
          @update:selected-category="selectedCategoryTag = $event" @search-input="onChartSearchInput"
          @install="openInstallWizard" @add-repo="openAddRepoModal"
        />
      </div>
      <div class="mobile-only">
        <HelmMobileCards active-tab="charts" :releases="filteredReleases" :charts="filteredCharts" :repos="repos" :loading="loadingCharts" @install-chart="openInstallWizard" />
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
            <span :class="{ 'spin-anim': updatingAllRepos }">🔄</span><span>{{ updatingAllRepos ? 'Updating Indexes...' : 'Update All Repos' }}</span>
          </button>
          <button type="button" class="btn-cyber btn-primary" @click="openAddRepoModal"><span>+ Add Repository</span></button>
        </div>
      </div>

      <div class="data-table-container glass-panel">
        <div v-if="loadingRepos" class="loading-state">
          <div class="cyber-spinner"></div>
          <p class="font-mono text-muted">Loading Helm repository indexes...</p>
        </div>
        <div v-else-if="repos.length === 0" class="empty-state">
          <span class="empty-icon">🗄️</span>
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
              <tr v-for="repo in repos" :key="repo.name">
                <td><div class="repo-name-cell"><span class="repo-icon">🗄️</span><strong class="font-mono text-primary">{{ repo.name }}</strong></div></td>
                <td>
                  <div class="repo-url-cell">
                    <a :href="repo.url" target="_blank" rel="noopener noreferrer" class="repo-link font-mono">{{ repo.url }} ↗</a>
                    <button type="button" class="btn-copy-icon" title="Copy URL" @click="copyToClipboard(repo.url, 'url')">📋</button>
                  </div>
                </td>
                <td class="text-right actions-cell">
                  <div class="action-btn-group">
                    <button type="button" class="btn-row-action btn-action-upgrade" title="Update index" @click="handleUpdateAllRepos"><span>🔄 Update</span></button>
                    <button type="button" class="btn-row-action btn-action-delete" title="Remove repository" @click="promptRemoveRepo(repo)"><span>🗑️ Remove</span></button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mobile-only">
          <HelmMobileCards active-tab="repos" :releases="filteredReleases" :charts="filteredCharts" :repos="repos" :loading="loadingRepos" @copy-url="copyToClipboard($event, 'url')" @remove-repo="promptRemoveRepo" />
        </div>
      </div>
    </section>

    <!-- Drawers & Modals -->
    <HelmReleaseDetailsDrawer
      :show="showDetailDrawer" :release="cleanSelectedRelease" :loading-detail="loadingReleaseDetail"
      :active-drawer-tab="activeDrawerTab" :release-history="releaseHistory" :loading-history="loadingHistory"
      :values-copied="valuesCopied" :manifest-copied="manifestCopied"
      @close="showDetailDrawer = false" @update:active-drawer-tab="activeDrawerTab = $event"
      @upgrade="openUpgradeModal" @rollback="openRollbackModal" @uninstall="promptUninstall"
      @rollback-revision="rollbackRevision = $event; showRollbackModal = true"
      @copy-notes="copyToClipboard($event, 'url')" @copy-values="copyToClipboard($event, 'values')"
      @download-values="downloadAsFile" @copy-manifest="copyToClipboard($event, 'manifest')" @download-manifest="downloadAsFile"
    />

    <HelmInstallDrawer
      :show="showInstallModal" :chart="selectedChartForInstall" :namespaces="namespaces" :selected-cluster="selectedCluster"
      :install-step="installStep" :install-form="installForm" :installing="installing"
      @close="showInstallModal = false" @step-change="installStep = $event"
      @reset-values="selectedChartForInstall && openInstallWizard(selectedChartForInstall)" @install="handleInstallChart"
    />

    <HelmRepositoryModal
      :show="showAddRepoModal" :add-repo-form="addRepoForm" :adding-repo="addingRepo" :repo-presets="repoPresets"
      @close="showAddRepoModal = false" @apply-preset="applyRepoPreset" @add-repo="handleAddRepo"
    />

    <HelmActionModals
      :show-upgrade-modal="showUpgradeModal" :upgrade-target="upgradeTarget" :upgrade-form="upgradeForm" :upgrading="upgrading"
      :show-rollback-modal="showRollbackModal" :rollback-target="rollbackTarget" :rollback-revision="rollbackRevision"
      :rollback-history-list="rollbackHistoryList" :loading-rollback-history="loadingRollbackHistory" :rolling-back="rollingBack"
      :show-uninstall-modal="showUninstallModal" :uninstall-target="uninstallTarget" :uninstalling="uninstalling"
      :show-remove-repo-modal="showRemoveRepoModal" :repo-to-remove="repoToRemove" :removing-repo="removingRepo"
      @update:show-upgrade-modal="showUpgradeModal = $event" @update:show-rollback-modal="showRollbackModal = $event"
      @update:show-uninstall-modal="showUninstallModal = $event" @update:show-remove-repo-modal="showRemoveRepoModal = $event"
      @update:rollback-revision="rollbackRevision = $event"
      @upgrade="handleUpgradeRelease" @rollback="handleRollbackRelease" @uninstall="handleUninstallRelease" @remove-repo="handleRemoveRepo"
    />
  </div>
</template>

<style>
@import '../assets/styles/views/helm.css';
@import '../assets/styles/views/helm-catalog-grid.css';
@import '../assets/styles/components/helm-drawers.css';
</style>
