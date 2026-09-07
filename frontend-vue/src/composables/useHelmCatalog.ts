import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import {
  helmApi,
  type HelmRelease,
  type HelmReleaseDetail,
  type HelmRevisionHistory,
  type HelmRepo,
  type HelmChart,
  type InstallReleaseRequest,
  type UpgradeReleaseRequest,
} from '../api/helm'
import { fleetApi, type Cluster } from '../api/fleet'
import { k8sApi, type K8sNamespace } from '../api/k8s'

export type ActiveTab = 'releases' | 'charts' | 'repos'
export type ActiveDrawerTab = 'overview' | 'values' | 'manifest' | 'history'

export const REPO_PRESETS = [
  { name: 'bitnami', url: 'https://charts.bitnami.com/bitnami', icon: '🍱' },
  { name: 'ingress-nginx', url: 'https://kubernetes.github.io/ingress-nginx', icon: '🌐' },
  { name: 'prometheus-community', url: 'https://prometheus-community.github.io/helm-charts', icon: '📊' },
  { name: 'grafana', url: 'https://grafana.github.io/helm-charts', icon: '📈' },
  { name: 'jetstack', url: 'https://charts.jetstack.io', icon: '🔒' },
  { name: 'traefik', url: 'https://traefik.github.io/charts', icon: '🚦' },
  { name: 'hashicorp', url: 'https://helm.releases.hashicorp.com', icon: '🔷' },
]

export const CATEGORY_TAGS = [
  { key: 'all', label: 'All Categories', icon: '✨' },
  { key: 'database', label: 'Databases', icon: '🗄️' },
  { key: 'networking', label: 'Ingress & Mesh', icon: '🌐' },
  { key: 'monitoring', label: 'Observability', icon: '📊' },
  { key: 'security', label: 'Security & Auth', icon: '🛡️' },
  { key: 'storage', label: 'Storage & Backup', icon: '📦' },
  { key: 'web', label: 'Web & APIs', icon: '⚡' },
]

export function useHelmCatalog() {
  const route = useRoute()
  const loading = ref(false)
  const error = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
  let toastTimer: ReturnType<typeof setTimeout> | null = null

  const activeTab = ref<ActiveTab>('releases')
  const clusters = ref<Cluster[]>([])
  const selectedCluster = ref<string>('primary-cluster')
  const namespaces = ref<K8sNamespace[]>([])
  const selectedNamespace = ref<string>('all')

  // Releases State
  const releases = ref<HelmRelease[]>([])
  const releaseSearch = ref('')
  const releaseStatusFilter = ref('all')
  const showDetailDrawer = ref(false)
  const selectedRelease = ref<HelmReleaseDetail | null>(null)
  const loadingReleaseDetail = ref(false)
  const activeDrawerTab = ref<ActiveDrawerTab>('overview')
  const releaseHistory = ref<HelmRevisionHistory[]>([])
  const loadingHistory = ref(false)
  const valuesCopied = ref(false)
  const manifestCopied = ref(false)

  // Upgrade / Rollback / Uninstall State
  const showUpgradeModal = ref(false)
  const upgradeTarget = ref<HelmRelease | null>(null)
  const upgradeForm = reactive<UpgradeReleaseRequest>({ chart: '', repo: '', version: '', values: '', resetValues: false, reuseValues: true })
  const upgrading = ref(false)

  const showRollbackModal = ref(false)
  const rollbackTarget = ref<HelmRelease | null>(null)
  const rollbackRevision = ref<number | null>(null)
  const rollbackHistoryList = ref<HelmRevisionHistory[]>([])
  const loadingRollbackHistory = ref(false)
  const rollingBack = ref(false)

  const showUninstallModal = ref(false)
  const uninstallTarget = ref<HelmRelease | null>(null)
  const uninstalling = ref(false)

  // Charts State
  const charts = ref<HelmChart[]>([])
  const chartSearch = ref('')
  const selectedRepoFilter = ref('all')
  const selectedCategoryTag = ref('all')
  const loadingCharts = ref(false)
  let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

  // Install Wizard State
  const showInstallModal = ref(false)
  const selectedChartForInstall = ref<HelmChart | null>(null)
  const installStep = ref<1 | 2 | 3>(1)
  const installForm = reactive({
    releaseName: '',
    namespace: 'default',
    createNamespace: true,
    version: '',
    values: '',
    loadingDefaultValues: false,
  })
  const installing = ref(false)

  // Repositories State
  const repos = ref<HelmRepo[]>([])
  const loadingRepos = ref(false)
  const showAddRepoModal = ref(false)
  const addRepoForm = reactive({ name: '', url: '' })
  const addingRepo = ref(false)
  const updatingAllRepos = ref(false)
  const showRemoveRepoModal = ref(false)
  const repoToRemove = ref<HelmRepo | null>(null)
  const removingRepo = ref(false)

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    if (toastTimer) clearTimeout(toastTimer)
    toastMessage.value = { text, type }
    toastTimer = setTimeout(() => { toastMessage.value = null }, 4000)
  }

  async function loadClusters() {
    try {
      const list = await fleetApi.list()
      clusters.value = list && list.length > 0 ? list : []
      if (list?.length && (!selectedCluster.value || !list.some(c => (c.name || c.id) === selectedCluster.value))) {
        selectedCluster.value = list[0].name || list[0].id
      }
    } catch {
      clusters.value = []
    }
  }

  async function loadNamespaces() {
    if (!selectedCluster.value) return (namespaces.value = [])
    try {
      const nsList = await k8sApi.listNamespaces(selectedCluster.value)
      namespaces.value = Array.isArray(nsList) ? nsList : []
    } catch {
      namespaces.value = []
    }
  }

  async function fetchReleases() {
    if (!selectedCluster.value) return
    loading.value = true
    error.value = null
    try {
      const ns = selectedNamespace.value !== 'all' ? selectedNamespace.value : undefined
      const list = await helmApi.listReleases(selectedCluster.value, ns)
      releases.value = Array.isArray(list) ? list : []
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch Helm releases'
      releases.value = []
    } finally {
      loading.value = false
    }
  }

  async function fetchCharts(keyword?: string) {
    loadingCharts.value = true
    try {
      const list = await helmApi.searchCharts(keyword)
      charts.value = Array.isArray(list) ? list : []
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to search Helm charts', 'error')
      charts.value = []
    } finally {
      loadingCharts.value = false
    }
  }

  async function fetchRepos() {
    loadingRepos.value = true
    try {
      const list = await helmApi.listRepos()
      repos.value = Array.isArray(list) ? list : []
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to fetch Helm repositories', 'error')
      repos.value = []
    } finally {
      loadingRepos.value = false
    }
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

  const totalReleasesCount = computed(() => releases.value.length)
  const deployedReleasesCount = computed(() => releases.value.filter(r => (r.status || '').toLowerCase().includes('deploy')).length)
  const failedReleasesCount = computed(() => releases.value.filter(r => (r.status || '').toLowerCase().includes('fail') || (r.status || '').toLowerCase().includes('error')).length)
  const deployedRate = computed(() => releases.value.length === 0 ? '100%' : `${Math.round((deployedReleasesCount.value / releases.value.length) * 100)}%`)

  const filteredReleases = computed(() => {
    let list = [...releases.value]
    if (releaseStatusFilter.value !== 'all') {
      const s = releaseStatusFilter.value.toLowerCase()
      list = list.filter(r => (r.status || '').toLowerCase().includes(s))
    }
    if (releaseSearch.value.trim()) {
      const q = releaseSearch.value.toLowerCase().trim()
      list = list.filter(r => (r.name || '').toLowerCase().includes(q) || (r.chart || '').toLowerCase().includes(q) || (r.namespace || '').toLowerCase().includes(q) || (r.description || '').toLowerCase().includes(q))
    }
    return list
  })

  const filteredCharts = computed(() => {
    let list = [...charts.value]
    if (selectedRepoFilter.value !== 'all') list = list.filter(c => c.repo === selectedRepoFilter.value)
    if (selectedCategoryTag.value !== 'all') {
      const tag = selectedCategoryTag.value.toLowerCase()
      list = list.filter(c => (c.name || '').toLowerCase().includes(tag) || (c.description || '').toLowerCase().includes(tag) || (c.keywords || []).some(k => k.toLowerCase().includes(tag)))
    }
    return list
  })

  async function openReleaseDetail(release: HelmRelease) {
    showDetailDrawer.value = true
    activeDrawerTab.value = 'overview'
    loadingReleaseDetail.value = true
    selectedRelease.value = { ...release }
    valuesCopied.value = false
    manifestCopied.value = false
    try {
      selectedRelease.value = await helmApi.getRelease(selectedCluster.value, release.name, release.namespace)
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Could not fetch full release details', 'error')
    } finally {
      loadingReleaseDetail.value = false
    }
    loadReleaseHistory(release)
  }

  async function loadReleaseHistory(release: HelmRelease) {
    loadingHistory.value = true
    try {
      const hist = await helmApi.getReleaseHistory(selectedCluster.value, release.name, release.namespace)
      releaseHistory.value = Array.isArray(hist) ? hist : []
    } catch {
      releaseHistory.value = []
    } finally {
      loadingHistory.value = false
    }
  }

  function openUpgradeModal(release: HelmRelease) {
    upgradeTarget.value = release
    upgradeForm.chart = release.chart || ''
    upgradeForm.repo = (release.chart || '').includes('/') ? release.chart.split('/')[0] : ''
    upgradeForm.version = release.version || ''
    upgradeForm.namespace = release.namespace || 'default'
    upgradeForm.resetValues = false
    upgradeForm.reuseValues = true
    upgradeForm.values = typeof release.values === 'string' ? release.values : ''
    showUpgradeModal.value = true
  }

  async function handleUpgradeRelease() {
    if (!upgradeTarget.value) return
    upgrading.value = true
    try {
      await helmApi.upgradeRelease(selectedCluster.value, upgradeTarget.value.name, { ...upgradeForm, namespace: upgradeTarget.value.namespace }, upgradeTarget.value.namespace)
      showToast(`Release ${upgradeTarget.value.name} upgraded successfully!`)
      showUpgradeModal.value = false
      await fetchReleases()
      if (showDetailDrawer.value && selectedRelease.value?.name === upgradeTarget.value.name) await openReleaseDetail(upgradeTarget.value)
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Upgrade failed', 'error')
    } finally {
      upgrading.value = false
    }
  }

  async function openRollbackModal(release: HelmRelease) {
    rollbackTarget.value = release
    rollbackRevision.value = null
    showRollbackModal.value = true
    loadingRollbackHistory.value = true
    try {
      const hist = await helmApi.getReleaseHistory(selectedCluster.value, release.name, release.namespace)
      rollbackHistoryList.value = Array.isArray(hist) ? hist : []
      if (rollbackHistoryList.value.length > 1) rollbackRevision.value = rollbackHistoryList.value[rollbackHistoryList.value.length - 2]?.revision ?? null
    } catch {
      rollbackHistoryList.value = []
    } finally {
      loadingRollbackHistory.value = false
    }
  }

  async function handleRollbackRelease() {
    if (!rollbackTarget.value || rollbackRevision.value === null) return showToast('Please select a revision to rollback to', 'error')
    rollingBack.value = true
    try {
      await helmApi.rollbackRelease(selectedCluster.value, rollbackTarget.value.name, rollbackRevision.value, rollbackTarget.value.namespace)
      showToast(`Release ${rollbackTarget.value.name} rolled back to revision ${rollbackRevision.value}!`)
      showRollbackModal.value = false
      await fetchReleases()
      if (showDetailDrawer.value && selectedRelease.value?.name === rollbackTarget.value.name) await openReleaseDetail(rollbackTarget.value)
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Rollback failed', 'error')
    } finally {
      rollingBack.value = false
    }
  }

  function promptUninstall(release: HelmRelease) {
    uninstallTarget.value = release
    showUninstallModal.value = true
  }

  async function handleUninstallRelease() {
    if (!uninstallTarget.value) return
    uninstalling.value = true
    try {
      await helmApi.uninstallRelease(selectedCluster.value, uninstallTarget.value.name, uninstallTarget.value.namespace)
      showToast(`Release ${uninstallTarget.value.name} uninstalled successfully!`)
      showUninstallModal.value = false
      if (showDetailDrawer.value && selectedRelease.value?.name === uninstallTarget.value.name) showDetailDrawer.value = false
      await fetchReleases()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Uninstall failed', 'error')
    } finally {
      uninstalling.value = false
    }
  }

  async function openInstallWizard(chart: HelmChart) {
    selectedChartForInstall.value = chart
    installStep.value = 1
    const sanitizedName = chart.name.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/^-+|-+$/g, '')
    installForm.releaseName = `${sanitizedName}-${Date.now().toString(36)}`
    installForm.namespace = selectedNamespace.value !== 'all' ? selectedNamespace.value : 'default'
    installForm.createNamespace = true
    installForm.version = chart.version
    installForm.values = ''
    installForm.loadingDefaultValues = true
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
      const payload: InstallReleaseRequest = {
        releaseName: installForm.releaseName.trim(),
        chart: selectedChartForInstall.value.name,
        repo: selectedChartForInstall.value.repo,
        version: installForm.version || selectedChartForInstall.value.version,
        namespace: installForm.namespace.trim(),
        values: installForm.values,
        createNamespace: installForm.createNamespace,
      }
      await helmApi.installRelease(selectedCluster.value, payload)
      showToast(`Chart ${selectedChartForInstall.value.name} installed as ${installForm.releaseName}!`)
      showInstallModal.value = false
      activeTab.value = 'releases'
      await fetchReleases()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Installation failed', 'error')
    } finally {
      installing.value = false
    }
  }

  function openAddRepoModal() {
    addRepoForm.name = ''
    addRepoForm.url = ''
    showAddRepoModal.value = true
  }

  function applyRepoPreset(preset: { name: string; url: string }) {
    addRepoForm.name = preset.name
    addRepoForm.url = preset.url
  }

  async function handleAddRepo() {
    if (!addRepoForm.name.trim() || !addRepoForm.url.trim()) return showToast('Repository name and URL required', 'error')
    addingRepo.value = true
    try {
      await helmApi.addRepo(addRepoForm.name.trim(), addRepoForm.url.trim())
      showToast(`Repository ${addRepoForm.name} added successfully!`)
      showAddRepoModal.value = false
      await Promise.all([fetchRepos(), fetchCharts()])
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to add repository', 'error')
    } finally {
      addingRepo.value = false
    }
  }

  async function handleUpdateAllRepos() {
    updatingAllRepos.value = true
    try {
      await helmApi.updateRepos()
      showToast('All Helm repositories successfully synchronized!')
      await Promise.all([fetchRepos(), fetchCharts()])
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Repository sync failed', 'error')
    } finally {
      updatingAllRepos.value = false
    }
  }

  function promptRemoveRepo(repo: HelmRepo) {
    repoToRemove.value = repo
    showRemoveRepoModal.value = true
  }

  async function handleRemoveRepo() {
    if (!repoToRemove.value) return
    removingRepo.value = true
    try {
      await helmApi.removeRepo(repoToRemove.value.name)
      showToast(`Repository ${repoToRemove.value.name} removed successfully!`)
      showRemoveRepoModal.value = false
      await Promise.all([fetchRepos(), fetchCharts()])
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to remove repository', 'error')
    } finally {
      removingRepo.value = false
    }
  }

  function copyToClipboard(text: string, type: 'values' | 'manifest' | 'url') {
    if (!navigator?.clipboard) return
    navigator.clipboard.writeText(text).then(() => {
      if (type === 'values') {
        valuesCopied.value = true
        setTimeout(() => (valuesCopied.value = false), 2500)
      } else if (type === 'manifest') {
        manifestCopied.value = true
        setTimeout(() => (manifestCopied.value = false), 2500)
      } else {
        showToast('Copied to clipboard!')
      }
    })
  }

  function downloadAsFile(filename: string, content: string) {
    const blob = new Blob([content], { type: 'text/yaml;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  function formatReleaseDate(dateStr?: string): string {
    if (!dateStr) return '—'
    try {
      const d = new Date(dateStr)
      if (isNaN(d.getTime())) return dateStr
      const diff = Date.now() - d.getTime()
      const mins = Math.floor(diff / 60000)
      if (mins < 1) return 'Just now'
      if (mins < 60) return `${mins}m ago`
      const hours = Math.floor(mins / 60)
      if (hours < 24) return `${hours}h ago`
      const days = Math.floor(Math.abs(diff) / (1000 * 60 * 60 * 24))
      return days < 30 ? `${days}d ago` : d.toLocaleDateString()
    } catch {
      return dateStr
    }
  }

  function getChartIcon(chart: HelmChart): string {
    const name = (chart.name || '').toLowerCase()
    if (name.includes('nginx') || name.includes('ingress') || name.includes('traefik')) return '🌐'
    if (name.includes('postgres') || name.includes('mysql') || name.includes('mariadb') || name.includes('redis') || name.includes('mongo')) return '🗄️'
    if (name.includes('prom') || name.includes('grafana') || name.includes('loki') || name.includes('metric')) return '📊'
    if (name.includes('cert') || name.includes('vault') || name.includes('auth') || name.includes('keycloak')) return '🛡️'
    if (name.includes('kafka') || name.includes('rabbit') || name.includes('queue') || name.includes('nats')) return '⚡'
    if (name.includes('elastic') || name.includes('search') || name.includes('opensearch')) return '🔍'
    if (name.includes('ai') || name.includes('ollama') || name.includes('vllm') || name.includes('llm')) return '🤖'
    return '📦'
  }

  function getStatusType(status: string): string {
    const s = (status || '').toLowerCase()
    if (s.includes('deploy') || s === 'active' || s === 'success') return 'deployed'
    if (s.includes('fail') || s.includes('error')) return 'failed'
    if (s.includes('pend') || s.includes('upgrad') || s.includes('install')) return 'pending'
    if (s.includes('super') || s.includes('uninst')) return 'superseded'
    return 'unknown'
  }

  watch(selectedCluster, async () => {
    await loadNamespaces()
    if (activeTab.value === 'releases') await fetchReleases()
  })

  watch(selectedNamespace, async () => {
    if (activeTab.value === 'releases') await fetchReleases()
  })

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
