import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import {
  helmApi,
  type HelmRelease,
  type HelmReleaseDetail,
  type HelmRevisionHistory,
  type HelmRepo,
  type HelmChart,
  type UpgradeReleaseRequest,
} from '../api/helm'
import { fleetApi, type Cluster } from '../api/fleet'
import { k8sApi, type K8sNamespace } from '../api/k8s'
import {
  REPO_PRESETS,
  CATEGORY_TAGS,
  type ActiveTab,
  type ActiveDrawerTab,
  type ToastMessage,
  type InstallFormData,
  downloadAsFile,
  formatReleaseDate,
  getChartIcon,
  getStatusType,
  getErrorMessage,
  filterReleases,
  filterCharts,
  computeReleaseStats,
  populateUpgradeForm,
  initInstallWizardForm,
  buildInstallPayload,
} from './helmCatalogConstants'

export { REPO_PRESETS, CATEGORY_TAGS, type ActiveTab, type ActiveDrawerTab }

export function useHelmCatalog() {
  const route = useRoute()
  const loading = ref(false), error = ref<string | null>(null), toastMessage = ref<ToastMessage | null>(null)
  let toastTimer: ReturnType<typeof setTimeout> | null = null

  const activeTab = ref<ActiveTab>('releases')
  const clusters = ref<Cluster[]>([]), selectedCluster = ref<string>('primary-cluster')
  const namespaces = ref<K8sNamespace[]>([]), selectedNamespace = ref<string>('all')

  // Releases State
  const releases = ref<HelmRelease[]>([]), releaseSearch = ref(''), releaseStatusFilter = ref('all')
  const showDetailDrawer = ref(false), loadingReleaseDetail = ref(false)
  const selectedRelease = ref<HelmReleaseDetail | null>(null), activeDrawerTab = ref<ActiveDrawerTab>('overview')
  const releaseHistory = ref<HelmRevisionHistory[]>([]), loadingHistory = ref(false)
  const valuesCopied = ref(false), manifestCopied = ref(false)

  // Upgrade / Rollback / Uninstall State
  const showUpgradeModal = ref(false), upgrading = ref(false), upgradeTarget = ref<HelmRelease | null>(null)
  const upgradeForm = reactive<UpgradeReleaseRequest>({ chart: '', repo: '', version: '', values: '', resetValues: false, reuseValues: true })
  const showRollbackModal = ref(false), rollingBack = ref(false), loadingRollbackHistory = ref(false)
  const rollbackTarget = ref<HelmRelease | null>(null), rollbackRevision = ref<number | null>(null)
  const rollbackHistoryList = ref<HelmRevisionHistory[]>([])
  const showUninstallModal = ref(false), uninstalling = ref(false), uninstallTarget = ref<HelmRelease | null>(null)

  // Charts & Wizard State
  const charts = ref<HelmChart[]>([]), chartSearch = ref(''), loadingCharts = ref(false)
  const selectedRepoFilter = ref('all'), selectedCategoryTag = ref('all')
  let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null
  const showInstallModal = ref(false), installing = ref(false), installStep = ref<1 | 2 | 3>(1)
  const selectedChartForInstall = ref<HelmChart | null>(null)
  const installForm = reactive<InstallFormData>({ releaseName: '', namespace: 'default', createNamespace: true, version: '', values: '', loadingDefaultValues: false })

  // Repositories State
  const repos = ref<HelmRepo[]>([]), loadingRepos = ref(false)
  const showAddRepoModal = ref(false), addingRepo = ref(false), updatingAllRepos = ref(false)
  const addRepoForm = reactive({ name: '', url: '' })
  const showRemoveRepoModal = ref(false), removingRepo = ref(false), repoToRemove = ref<HelmRepo | null>(null)

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    if (toastTimer) clearTimeout(toastTimer)
    toastMessage.value = { text, type }
    toastTimer = setTimeout(() => { toastMessage.value = null }, 4000)
  }

  async function loadClusters() {
    try {
      const list = (await fleetApi.list()) || []
      clusters.value = list
      if (list.length && (!selectedCluster.value || !list.some(c => (c.name || c.id) === selectedCluster.value))) {
        selectedCluster.value = list[0].name || list[0].id
      }
    } catch { clusters.value = [] }
  }

  async function loadNamespaces() {
    if (!selectedCluster.value) return (namespaces.value = [])
    try { namespaces.value = (await k8sApi.listNamespaces(selectedCluster.value)) || [] }
    catch { namespaces.value = [] }
  }

  async function fetchReleases() {
    if (!selectedCluster.value) return
    loading.value = true; error.value = null
    try {
      const ns = selectedNamespace.value !== 'all' ? selectedNamespace.value : undefined
      releases.value = (await helmApi.listReleases(selectedCluster.value, ns)) || []
    } catch (err: unknown) {
      error.value = getErrorMessage(err, 'Failed to fetch Helm releases')
      releases.value = []
    } finally { loading.value = false }
  }

  async function fetchCharts(keyword?: string) {
    loadingCharts.value = true
    try { charts.value = (await helmApi.searchCharts(keyword)) || [] }
    catch (err: unknown) { showToast(getErrorMessage(err, 'Failed to search Helm charts'), 'error'); charts.value = [] }
    finally { loadingCharts.value = false }
  }

  async function fetchRepos() {
    loadingRepos.value = true
    try { repos.value = (await helmApi.listRepos()) || [] }
    catch (err: unknown) { showToast(getErrorMessage(err, 'Failed to fetch Helm repositories'), 'error'); repos.value = [] }
    finally { loadingRepos.value = false }
  }

  async function refreshActiveTab() {
    if (activeTab.value === 'releases') await fetchReleases()
    else if (activeTab.value === 'charts') await fetchCharts(chartSearch.value)
    else if (activeTab.value === 'repos') await fetchRepos()
  }

  function onChartSearchInput() {
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
    searchDebounceTimer = setTimeout(() => fetchCharts(chartSearch.value), 350)
  }

  const releaseStats = computed(() => computeReleaseStats(releases.value))
  const totalReleasesCount = computed(() => releaseStats.value.total)
  const deployedReleasesCount = computed(() => releaseStats.value.deployed)
  const failedReleasesCount = computed(() => releaseStats.value.failed)
  const deployedRate = computed(() => releaseStats.value.rate)
  const filteredReleases = computed(() => filterReleases(releases.value, releaseStatusFilter.value, releaseSearch.value))
  const filteredCharts = computed(() => filterCharts(charts.value, selectedRepoFilter.value, selectedCategoryTag.value))

  async function openReleaseDetail(release: HelmRelease) {
    showDetailDrawer.value = true; activeDrawerTab.value = 'overview'; loadingReleaseDetail.value = true
    selectedRelease.value = { ...release }; valuesCopied.value = false; manifestCopied.value = false
    try { selectedRelease.value = await helmApi.getRelease(selectedCluster.value, release.name, release.namespace) }
    catch (err: unknown) { showToast(getErrorMessage(err, 'Could not fetch full release details'), 'error') }
    finally { loadingReleaseDetail.value = false }
    loadReleaseHistory(release)
  }

  async function loadReleaseHistory(release: HelmRelease) {
    loadingHistory.value = true
    try { releaseHistory.value = (await helmApi.getReleaseHistory(selectedCluster.value, release.name, release.namespace)) || [] }
    catch { releaseHistory.value = [] }
    finally { loadingHistory.value = false }
  }

  function openUpgradeModal(release: HelmRelease) {
    upgradeTarget.value = release
    populateUpgradeForm(upgradeForm, release)
    showUpgradeModal.value = true
  }

  async function handleUpgradeRelease() {
    if (!upgradeTarget.value) return
    upgrading.value = true
    try {
      await helmApi.upgradeRelease(selectedCluster.value, upgradeTarget.value.name, { ...upgradeForm, namespace: upgradeTarget.value.namespace }, upgradeTarget.value.namespace)
      showToast(`Release ${upgradeTarget.value.name} upgraded successfully!`)
      showUpgradeModal.value = false; await fetchReleases()
      if (showDetailDrawer.value && selectedRelease.value?.name === upgradeTarget.value.name) await openReleaseDetail(upgradeTarget.value)
    } catch (err: unknown) { showToast(getErrorMessage(err, 'Upgrade failed'), 'error') }
    finally { upgrading.value = false }
  }

  async function openRollbackModal(release: HelmRelease) {
    rollbackTarget.value = release; rollbackRevision.value = null
    showRollbackModal.value = true; loadingRollbackHistory.value = true
    try {
      const hist = (await helmApi.getReleaseHistory(selectedCluster.value, release.name, release.namespace)) || []
      rollbackHistoryList.value = hist
      if (hist.length > 1) rollbackRevision.value = hist[hist.length - 2]?.revision ?? null
    } catch { rollbackHistoryList.value = [] }
    finally { loadingRollbackHistory.value = false }
  }

  async function handleRollbackRelease() {
    if (!rollbackTarget.value || rollbackRevision.value === null) return showToast('Please select a revision to rollback to', 'error')
    rollingBack.value = true
    try {
      await helmApi.rollbackRelease(selectedCluster.value, rollbackTarget.value.name, rollbackRevision.value, rollbackTarget.value.namespace)
      showToast(`Release ${rollbackTarget.value.name} rolled back to revision ${rollbackRevision.value}!`)
      showRollbackModal.value = false; await fetchReleases()
      if (showDetailDrawer.value && selectedRelease.value?.name === rollbackTarget.value.name) await openReleaseDetail(rollbackTarget.value)
    } catch (err: unknown) { showToast(getErrorMessage(err, 'Rollback failed'), 'error') }
    finally { rollingBack.value = false }
  }

  function promptUninstall(release: HelmRelease) { uninstallTarget.value = release; showUninstallModal.value = true }

  async function handleUninstallRelease() {
    if (!uninstallTarget.value) return
    uninstalling.value = true
    try {
      await helmApi.uninstallRelease(selectedCluster.value, uninstallTarget.value.name, uninstallTarget.value.namespace)
      showToast(`Release ${uninstallTarget.value.name} uninstalled successfully!`)
      showUninstallModal.value = false
      if (showDetailDrawer.value && selectedRelease.value?.name === uninstallTarget.value.name) showDetailDrawer.value = false
      await fetchReleases()
    } catch (err: unknown) { showToast(getErrorMessage(err, 'Uninstall failed'), 'error') }
    finally { uninstalling.value = false }
  }

  async function openInstallWizard(chart: HelmChart) {
    selectedChartForInstall.value = chart; installStep.value = 1
    Object.assign(installForm, initInstallWizardForm(chart, selectedNamespace.value))
    showInstallModal.value = true
    try {
      const valRes = await helmApi.getChartValues(chart.repo, chart.name, chart.version)
      installForm.values = valRes?.values || `# Default values for ${chart.name}\n`
    } catch {
      installForm.values = `# Custom values for ${chart.name}\n`
    } finally {
      installForm.loadingDefaultValues = false
    }
  }

  async function handleInstallChart() {
    if (!selectedChartForInstall.value || !installForm.releaseName.trim() || !installForm.namespace.trim()) {
      return showToast('Release name and namespace are required', 'error')
    }
    installing.value = true
    try {
      await helmApi.installRelease(selectedCluster.value, buildInstallPayload(selectedChartForInstall.value, installForm))
      showToast(`Chart ${selectedChartForInstall.value.name} installed as ${installForm.releaseName}!`)
      showInstallModal.value = false; activeTab.value = 'releases'; await fetchReleases()
    } catch (err: unknown) { showToast(getErrorMessage(err, 'Installation failed'), 'error') }
    finally { installing.value = false }
  }

  function openAddRepoModal() { addRepoForm.name = ''; addRepoForm.url = ''; showAddRepoModal.value = true }
  function applyRepoPreset(preset: { name: string; url: string }) { addRepoForm.name = preset.name; addRepoForm.url = preset.url }
  function promptRemoveRepo(repo: HelmRepo) { repoToRemove.value = repo; showRemoveRepoModal.value = true }

  async function handleAddRepo() {
    if (!addRepoForm.name.trim() || !addRepoForm.url.trim()) return showToast('Repository name and URL required', 'error')
    addingRepo.value = true
    try {
      await helmApi.addRepo(addRepoForm.name.trim(), addRepoForm.url.trim())
      showToast(`Repository ${addRepoForm.name} added successfully!`)
      showAddRepoModal.value = false; await Promise.all([fetchRepos(), fetchCharts()])
    } catch (err: unknown) { showToast(getErrorMessage(err, 'Failed to add repository'), 'error') }
    finally { addingRepo.value = false }
  }

  async function handleUpdateAllRepos() {
    updatingAllRepos.value = true
    try {
      await helmApi.updateRepos()
      showToast('All Helm repositories successfully synchronized!')
      await Promise.all([fetchRepos(), fetchCharts()])
    } catch (err: unknown) { showToast(getErrorMessage(err, 'Repository sync failed'), 'error') }
    finally { updatingAllRepos.value = false }
  }

  async function handleRemoveRepo() {
    if (!repoToRemove.value) return
    removingRepo.value = true
    try {
      await helmApi.removeRepo(repoToRemove.value.name)
      showToast(`Repository ${repoToRemove.value.name} removed successfully!`)
      showRemoveRepoModal.value = false; await Promise.all([fetchRepos(), fetchCharts()])
    } catch (err: unknown) { showToast(getErrorMessage(err, 'Failed to remove repository'), 'error') }
    finally { removingRepo.value = false }
  }

  function copyToClipboard(text: string, type: 'values' | 'manifest' | 'url') {
    if (!navigator?.clipboard) return
    navigator.clipboard.writeText(text).then(() => {
      if (type === 'values') { valuesCopied.value = true; setTimeout(() => (valuesCopied.value = false), 2500) }
      else if (type === 'manifest') { manifestCopied.value = true; setTimeout(() => (manifestCopied.value = false), 2500) }
      else { showToast('Copied to clipboard!') }
    })
  }

  watch(selectedCluster, async () => { await loadNamespaces(); if (activeTab.value === 'releases') await fetchReleases() })
  watch(selectedNamespace, async () => { if (activeTab.value === 'releases') await fetchReleases() })
  watch(activeTab, async (newTab) => {
    if (newTab === 'releases' && releases.value.length === 0) await fetchReleases()
    else if (newTab === 'charts' && charts.value.length === 0) await fetchCharts()
    else if (newTab === 'repos' && repos.value.length === 0) await fetchRepos()
  })

  onMounted(async () => {
    if (route.query.cluster && typeof route.query.cluster === 'string') selectedCluster.value = route.query.cluster
    if (route.query.namespace && typeof route.query.namespace === 'string') selectedNamespace.value = route.query.namespace
    if (route.query.tab && ['releases', 'charts', 'repos'].includes(route.query.tab as string)) activeTab.value = route.query.tab as ActiveTab

    await loadClusters()
    await loadNamespaces()
    await Promise.all([fetchReleases(), fetchCharts(), fetchRepos()])
  })

  onUnmounted(() => {
    if (toastTimer) clearTimeout(toastTimer)
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  })

  return {
    loading, error, toastMessage, activeTab, clusters, selectedCluster, namespaces, selectedNamespace,
    releases, releaseSearch, releaseStatusFilter, showDetailDrawer, selectedRelease, loadingReleaseDetail,
    activeDrawerTab, releaseHistory, loadingHistory, valuesCopied, manifestCopied,
    showUpgradeModal, upgradeTarget, upgradeForm, upgrading,
    showRollbackModal, rollbackTarget, rollbackRevision, rollbackHistoryList, loadingRollbackHistory, rollingBack,
    showUninstallModal, uninstallTarget, uninstalling,
    charts, chartSearch, selectedRepoFilter, selectedCategoryTag, loadingCharts,
    showInstallModal, selectedChartForInstall, installStep, installForm, installing,
    repos, loadingRepos, showAddRepoModal, addRepoForm, addingRepo, updatingAllRepos, showRemoveRepoModal, repoToRemove, removingRepo,
    repoPresets: REPO_PRESETS, categoryTags: CATEGORY_TAGS,
    totalReleasesCount, deployedReleasesCount, failedReleasesCount, deployedRate, filteredReleases, filteredCharts,
    showToast, loadClusters, loadNamespaces, fetchReleases, fetchCharts, fetchRepos, refreshActiveTab, onChartSearchInput,
    openReleaseDetail, loadReleaseHistory, openUpgradeModal, handleUpgradeRelease, openRollbackModal, handleRollbackRelease,
    promptUninstall, handleUninstallRelease, openInstallWizard, handleInstallChart, openAddRepoModal, applyRepoPreset,
    handleAddRepo, handleUpdateAllRepos, promptRemoveRepo, handleRemoveRepo, copyToClipboard, downloadAsFile,
    formatReleaseDate, getChartIcon, getStatusType,
  }
}
