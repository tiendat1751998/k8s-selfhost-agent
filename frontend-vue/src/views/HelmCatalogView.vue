<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
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

const route = useRoute()

// Async loading & error states
const loading = ref(false)
const error = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | null = null

// Navigation Tabs
type ActiveTab = 'releases' | 'charts' | 'repos'
const activeTab = ref<ActiveTab>('releases')

// Cluster & Namespace Selection State
const clusters = ref<Cluster[]>([])
const selectedCluster = ref<string>('primary-cluster')
const namespaces = ref<K8sNamespace[]>([])
const selectedNamespace = ref<string>('all')

// Tab 1: Releases State
const releases = ref<HelmRelease[]>([])
const releaseSearch = ref('')
const releaseStatusFilter = ref('all')
const showDetailDrawer = ref(false)
const selectedRelease = ref<HelmReleaseDetail | null>(null)
const loadingReleaseDetail = ref(false)
const activeDrawerTab = ref<'overview' | 'values' | 'manifest' | 'history'>('overview')
const releaseHistory = ref<HelmRevisionHistory[]>([])
const loadingHistory = ref(false)
const valuesCopied = ref(false)
const manifestCopied = ref(false)

// Upgrade Release State
const showUpgradeModal = ref(false)
const upgradeTarget = ref<HelmRelease | null>(null)
const upgradeForm = reactive<UpgradeReleaseRequest>({
  chart: '',
  repo: '',
  version: '',
  values: '',
  resetValues: false,
  reuseValues: true,
})
const upgrading = ref(false)

// Rollback Release State
const showRollbackModal = ref(false)
const rollbackTarget = ref<HelmRelease | null>(null)
const rollbackRevision = ref<number | null>(null)
const rollbackHistoryList = ref<HelmRevisionHistory[]>([])
const loadingRollbackHistory = ref(false)
const rollingBack = ref(false)

// Uninstall Release State
const showUninstallModal = ref(false)
const uninstallTarget = ref<HelmRelease | null>(null)
const uninstallConfirmText = ref('')
const uninstalling = ref(false)

// Tab 2: Charts State
const charts = ref<HelmChart[]>([])
const chartSearch = ref('')
const selectedRepoFilter = ref('all')
const selectedCategoryTag = ref('all')
const loadingCharts = ref(false)
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

// Install Chart Wizard State
const showInstallModal = ref(false)
const selectedChartForInstall = ref<HelmChart | null>(null)
const installStep = ref<1 | 2 | 3>(1)
const installForm = reactive<{
  releaseName: string
  namespace: string
  createNamespace: boolean
  version: string
  values: string
  loadingDefaultValues: boolean
}>({
  releaseName: '',
  namespace: 'default',
  createNamespace: true,
  version: '',
  values: '',
  loadingDefaultValues: false,
})
const installing = ref(false)

// Tab 3: Repositories State
const repos = ref<HelmRepo[]>([])
const loadingRepos = ref(false)
const showAddRepoModal = ref(false)
const addRepoForm = reactive({
  name: '',
  url: '',
})
const addingRepo = ref(false)
const updatingAllRepos = ref(false)
const showRemoveRepoModal = ref(false)
const repoToRemove = ref<HelmRepo | null>(null)
const removingRepo = ref(false)
// Repo Presets
const repoPresets = [
  { name: 'bitnami', url: 'https://charts.bitnami.com/bitnami', icon: '🍱' },
  { name: 'ingress-nginx', url: 'https://kubernetes.github.io/ingress-nginx', icon: '🌐' },
  { name: 'prometheus-community', url: 'https://prometheus-community.github.io/helm-charts', icon: '📊' },
  { name: 'grafana', url: 'https://grafana.github.io/helm-charts', icon: '📈' },
  { name: 'jetstack', url: 'https://charts.jetstack.io', icon: '🔒' },
  { name: 'traefik', url: 'https://traefik.github.io/charts', icon: '🚦' },
  { name: 'hashicorp', url: 'https://helm.releases.hashicorp.com', icon: '🔷' },
]

// Quick category chips
const categoryTags = [
  { key: 'all', label: 'All Categories', icon: '✨' },
  { key: 'database', label: 'Databases', icon: '🗄️' },
  { key: 'networking', label: 'Ingress & Mesh', icon: '🌐' },
  { key: 'monitoring', label: 'Observability', icon: '📊' },
  { key: 'security', label: 'Security & Auth', icon: '🛡️' },
  { key: 'storage', label: 'Storage & Backup', icon: '📦' },
  { key: 'web', label: 'Web & APIs', icon: '⚡' },
]

// Toast notification helper
function showToast(text: string, type: 'success' | 'error' = 'success') {
  if (toastTimer) clearTimeout(toastTimer)
  toastMessage.value = { text, type }
  toastTimer = setTimeout(() => {
    toastMessage.value = null
  }, 4000)
}

// Cluster & Namespace Loading
async function loadClusters() {
  try {
    const list = await fleetApi.list()
    if (list && list.length > 0) {
      clusters.value = list
      if (!selectedCluster.value || !list.some(c => (c.name || c.id) === selectedCluster.value)) {
        selectedCluster.value = list[0].name || list[0].id
      }
    } else {
      clusters.value = []
      if (!selectedCluster.value) {
        selectedCluster.value = 'primary-cluster'
      }
    }
  } catch {
    clusters.value = []
    if (!selectedCluster.value) {
      selectedCluster.value = 'primary-cluster'
    }
  }
}

async function loadNamespaces() {
  if (!selectedCluster.value) {
    namespaces.value = []
    return
  }
  try {
    const nsList = await k8sApi.listNamespaces(selectedCluster.value)
    namespaces.value = Array.isArray(nsList) ? nsList : []
  } catch {
    namespaces.value = []
  }
}

// Data Fetching: Releases, Charts, Repos
async function fetchReleases() {
  if (!selectedCluster.value) return
  loading.value = true
  error.value = null
  try {
    const ns = selectedNamespace.value !== 'all' ? selectedNamespace.value : undefined
    const list = await helmApi.listReleases(selectedCluster.value, ns)
    releases.value = Array.isArray(list) ? list : []
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to fetch Helm releases'
    error.value = msg
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
    const msg = err instanceof Error ? err.message : 'Failed to search Helm charts'
    showToast(msg, 'error')
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
    const msg = err instanceof Error ? err.message : 'Failed to fetch Helm repositories'
    showToast(msg, 'error')
    repos.value = []
  } finally {
    loadingRepos.value = false
  }
}

async function refreshActiveTab() {
  if (activeTab.value === 'releases') {
    await fetchReleases()
  } else if (activeTab.value === 'charts') {
    await fetchCharts(chartSearch.value)
  } else if (activeTab.value === 'repos') {
    await fetchRepos()
  }
}

function onChartSearchInput() {
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  searchDebounceTimer = setTimeout(() => {
    fetchCharts(chartSearch.value)
  }, 350)
}
// Computed Metrics & Filtered Lists
const totalReleasesCount = computed(() => releases.value.length)
const deployedReleasesCount = computed(() =>
  releases.value.filter(r => (r.status || '').toLowerCase().includes('deploy')).length
)
const failedReleasesCount = computed(() =>
  releases.value.filter(r => (r.status || '').toLowerCase().includes('fail') || (r.status || '').toLowerCase().includes('error')).length
)
const deployedRate = computed(() => {
  if (releases.value.length === 0) return '100%'
  const pct = Math.round((deployedReleasesCount.value / releases.value.length) * 100)
  return `${pct}%`
})

const filteredReleases = computed(() => {
  let list = [...releases.value]

  if (releaseStatusFilter.value !== 'all') {
    const s = releaseStatusFilter.value.toLowerCase()
    list = list.filter(r => (r.status || '').toLowerCase().includes(s))
  }

  if (releaseSearch.value.trim()) {
    const q = releaseSearch.value.toLowerCase().trim()
    list = list.filter(r =>
      (r.name || '').toLowerCase().includes(q) ||
      (r.chart || '').toLowerCase().includes(q) ||
      (r.namespace || '').toLowerCase().includes(q) ||
      (r.description || '').toLowerCase().includes(q)
    )
  }

  return list
})

const filteredCharts = computed(() => {
  let list = [...charts.value]

  if (selectedRepoFilter.value !== 'all') {
    list = list.filter(c => c.repo === selectedRepoFilter.value)
  }

  if (selectedCategoryTag.value !== 'all') {
    const tag = selectedCategoryTag.value.toLowerCase()
    list = list.filter(c => {
      const matchName = (c.name || '').toLowerCase().includes(tag)
      const matchDesc = (c.description || '').toLowerCase().includes(tag)
      const matchKeywords = (c.keywords || []).some(k => k.toLowerCase().includes(tag))
      return matchName || matchDesc || matchKeywords
    })
  }

  return list
})

// Release Actions
async function openReleaseDetail(release: HelmRelease) {
  showDetailDrawer.value = true
  activeDrawerTab.value = 'overview'
  loadingReleaseDetail.value = true
  selectedRelease.value = { ...release }
  valuesCopied.value = false
  manifestCopied.value = false

  try {
    const detail = await helmApi.getRelease(selectedCluster.value, release.name, release.namespace)
    selectedRelease.value = detail
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Could not fetch full release details'
    showToast(msg, 'error')
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
    await helmApi.upgradeRelease(
      selectedCluster.value,
      upgradeTarget.value.name,
      {
        chart: upgradeForm.chart,
        repo: upgradeForm.repo,
        version: upgradeForm.version,
        values: upgradeForm.values,
        namespace: upgradeTarget.value.namespace,
        resetValues: upgradeForm.resetValues,
        reuseValues: upgradeForm.reuseValues,
      },
      upgradeTarget.value.namespace
    )
    showToast(`Release ${upgradeTarget.value.name} upgraded successfully!`)
    showUpgradeModal.value = false
    await fetchReleases()
    if (showDetailDrawer.value && selectedRelease.value?.name === upgradeTarget.value.name) {
      await openReleaseDetail(upgradeTarget.value)
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Upgrade failed'
    showToast(msg, 'error')
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
    if (rollbackHistoryList.value.length > 1) {
      rollbackRevision.value = rollbackHistoryList.value[rollbackHistoryList.value.length - 2]?.revision ?? null
    }
  } catch {
    rollbackHistoryList.value = []
  } finally {
    loadingRollbackHistory.value = false
  }
}

async function handleRollbackRelease() {
  if (!rollbackTarget.value || rollbackRevision.value === null) {
    showToast('Please select a revision to rollback to', 'error')
    return
  }
  rollingBack.value = true
  try {
    await helmApi.rollbackRelease(
      selectedCluster.value,
      rollbackTarget.value.name,
      rollbackRevision.value,
      rollbackTarget.value.namespace
    )
    showToast(`Release ${rollbackTarget.value.name} rolled back to revision ${rollbackRevision.value}!`)
    showRollbackModal.value = false
    await fetchReleases()
    if (showDetailDrawer.value && selectedRelease.value?.name === rollbackTarget.value.name) {
      await openReleaseDetail(rollbackTarget.value)
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Rollback failed'
    showToast(msg, 'error')
  } finally {
    rollingBack.value = false
  }
}

function promptUninstall(release: HelmRelease) {
  uninstallTarget.value = release
  uninstallConfirmText.value = ''
  showUninstallModal.value = true
}

async function handleUninstallRelease() {
  if (!uninstallTarget.value) return
  uninstalling.value = true
  try {
    await helmApi.uninstallRelease(
      selectedCluster.value,
      uninstallTarget.value.name,
      uninstallTarget.value.namespace
    )
    showToast(`Release ${uninstallTarget.value.name} uninstalled successfully!`)
    showUninstallModal.value = false
    if (showDetailDrawer.value && selectedRelease.value?.name === uninstallTarget.value.name) {
      showDetailDrawer.value = false
    }
    await fetchReleases()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Uninstall failed'
    showToast(msg, 'error')
  } finally {
    uninstalling.value = false
  }
}
// Chart Install Wizard
async function openInstallWizard(chart: HelmChart) {
  selectedChartForInstall.value = chart
  installStep.value = 1
  const sanitizedName = chart.name.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/^-+|-+$/g, '')
  installForm.releaseName = `${sanitizedName}-${Math.floor(100 + Math.random() * 900)}`
  installForm.namespace = selectedNamespace.value !== 'all' ? selectedNamespace.value : 'default'
  installForm.createNamespace = true
  installForm.version = chart.version
  installForm.values = ''
  installForm.loadingDefaultValues = true
  showInstallModal.value = true

  try {
    const valRes = await helmApi.getChartValues(chart.repo, chart.name, chart.version)
    installForm.values = valRes?.values || '# Default values for ' + chart.name + '\n'
  } catch {
    installForm.values = `# Custom values for ${chart.name}\n`
  } finally {
    installForm.loadingDefaultValues = false
  }
}

async function handleInstallChart() {
  if (!selectedChartForInstall.value) return
  if (!installForm.releaseName.trim()) {
    showToast('Please enter a release name', 'error')
    return
  }
  if (!installForm.namespace.trim()) {
    showToast('Please enter a target namespace', 'error')
    return
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
    const msg = err instanceof Error ? err.message : 'Installation failed'
    showToast(msg, 'error')
  } finally {
    installing.value = false
  }
}

// Repository Management
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
  if (!addRepoForm.name.trim()) {
    showToast('Repository name is required', 'error')
    return
  }
  if (!addRepoForm.url.trim()) {
    showToast('Repository URL is required', 'error')
    return
  }

  addingRepo.value = true
  try {
    await helmApi.addRepo(addRepoForm.name.trim(), addRepoForm.url.trim())
    showToast(`Repository ${addRepoForm.name} added successfully!`)
    showAddRepoModal.value = false
    await fetchRepos()
    await fetchCharts()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to add repository'
    showToast(msg, 'error')
  } finally {
    addingRepo.value = false
  }
}

async function handleUpdateAllRepos() {
  updatingAllRepos.value = true
  try {
    await helmApi.updateRepos()
    showToast('All Helm repositories successfully synchronized!')
    await fetchRepos()
    await fetchCharts()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Repository sync failed'
    showToast(msg, 'error')
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
    await fetchRepos()
    await fetchCharts()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to remove repository'
    showToast(msg, 'error')
  } finally {
    removingRepo.value = false
  }
}

// Utilities
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

function daysBetween(d1: Date, d2: Date): number {
  return Math.floor(Math.abs(d2.getTime() - d1.getTime()) / (1000 * 60 * 60 * 24))
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
    const days = Math.floor(daysBetween(d, new Date()))
    if (days < 30) return `${days}d ago`
    return d.toLocaleDateString()
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

// Watchers & Lifecycle
watch(selectedCluster, async () => {
  await loadNamespaces()
  if (activeTab.value === 'releases') {
    await fetchReleases()
  }
})

watch(selectedNamespace, async () => {
  if (activeTab.value === 'releases') {
    await fetchReleases()
  }
})

watch(activeTab, async (newTab) => {
  if (newTab === 'releases' && releases.value.length === 0) {
    await fetchReleases()
  } else if (newTab === 'charts' && charts.value.length === 0) {
    await fetchCharts()
  } else if (newTab === 'repos' && repos.value.length === 0) {
    await fetchRepos()
  }
})

onMounted(async () => {
  if (route.query.cluster && typeof route.query.cluster === 'string') {
    selectedCluster.value = route.query.cluster
  }
  if (route.query.namespace && typeof route.query.namespace === 'string') {
    selectedNamespace.value = route.query.namespace
  }
  if (route.query.tab && ['releases', 'charts', 'repos'].includes(route.query.tab as string)) {
    activeTab.value = route.query.tab as ActiveTab
  }

  await loadClusters()
  await loadNamespaces()
  await Promise.all([
    fetchReleases(),
    fetchCharts(),
    fetchRepos(),
  ])
})

onUnmounted(() => {
  if (toastTimer) clearTimeout(toastTimer)
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
})
</script>
<template>
  <div class="helm-catalog-layout animate-fade-in">
    <!-- Toast Notification -->
    <Transition name="toast-slide">
      <div 
        v-if="toastMessage" 
        class="cyber-toast" 
        :class="`toast-${toastMessage.type}`"
      >
        <span class="toast-icon">{{ toastMessage.type === 'success' ? '✓' : '⚠️' }}</span>
        <span class="toast-text">{{ toastMessage.text }}</span>
      </div>
    </Transition>

    <!-- Page Header with Cyber Hud Controls -->
    <header class="page-header glass-panel glass-panel-glow">
      <div class="header-main">
        <div class="title-group">
          <div class="badge-title-row">
            <span class="pulse-dot pulse-dot-gold"></span>
            <span class="cyber-tag">HELM ENGINE v3</span>
          </div>
          <h1 class="page-title title-full">Helm Application Catalog</h1>
          <h1 class="page-title title-compact">Helm Catalog</h1>
          <p class="page-subtitle">
            Browse repositories, deploy pre-packaged cloud-native charts & manage lifecycle releases
          </p>
        </div>

        <div class="header-controls">
          <!-- Cluster Selector -->
          <div class="control-box">
            <label class="control-label">Target Cluster</label>
            <select v-model="selectedCluster" class="input-glass header-select cluster-select">
              <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">
                🌐 {{ c.name || c.id }}
              </option>
              <option v-if="clusters.length === 0" value="primary-cluster">🌐 primary-cluster</option>
            </select>
          </div>

          <!-- Namespace Selector -->
          <div class="control-box">
            <label class="control-label">Namespace Scope</label>
            <select v-model="selectedNamespace" class="input-glass header-select ns-select">
              <option value="all">🌐 All Namespaces</option>
              <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">
                🏷️ {{ ns.name }}
              </option>
            </select>
          </div>

          <!-- Quick Action Buttons -->
          <div class="header-actions-group">
            <button 
              type="button" 
              class="btn-cyber btn-primary"
              @click="openAddRepoModal"
            >
              <span>+ Add Repo</span>
            </button>
            <button 
              type="button" 
              class="btn-cyber btn-secondary"
              :disabled="loading || loadingCharts || loadingRepos"
              title="Refresh current view"
              @click="refreshActiveTab"
            >
              <span>🔄 Refresh</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Metrics Row (2x2 on Mobile) -->
      <div class="metrics-grid">
        <MetricCard
          title="Total Releases"
          :value="totalReleasesCount"
          :subtitle="`${deployedReleasesCount} Deployed • ${failedReleasesCount} Failed`"
          icon="⛵"
          badge="Live"
          badge-color="emerald"
        />
        <MetricCard
          title="Deployed Health"
          :value="deployedRate"
          subtitle="Successful rollout ratio"
          icon="🟢"
          :trend="failedReleasesCount > 0 ? `${failedReleasesCount} Degraded` : '100% Healthy'"
          :trend-type="failedReleasesCount > 0 ? 'negative' : 'positive'"
        />
        <MetricCard
          title="Catalog Charts"
          :value="charts.length"
          subtitle="Available packages across repos"
          icon="📦"
          badge="Searchable"
          badge-color="cyan"
        />
        <MetricCard
          title="Helm Repos"
          :value="repos.length"
          subtitle="Configured chart repositories"
          icon="🗄️"
          badge="Sync Ready"
          badge-color="violet"
        />
      </div>
    </header>

    <!-- Navigation Tabs -->
    <nav class="catalog-tabs-bar glass-panel" aria-label="Helm Catalog Sections">
      <div class="tabs-list">
        <button
          type="button"
          class="tab-btn"
          :class="{ active: activeTab === 'releases' }"
          @click="activeTab = 'releases'"
        >
          <span class="tab-icon">⛵</span>
          <span class="tab-label">Releases</span>
          <span class="tab-counter">{{ releases.length }}</span>
        </button>
        <button
          type="button"
          class="tab-btn"
          :class="{ active: activeTab === 'charts' }"
          @click="activeTab = 'charts'"
        >
          <span class="tab-icon">📦</span>
          <span class="tab-label">Chart Catalog</span>
          <span class="tab-counter">{{ charts.length }}</span>
        </button>
        <button
          type="button"
          class="tab-btn"
          :class="{ active: activeTab === 'repos' }"
          @click="activeTab = 'repos'"
        >
          <span class="tab-icon">🗄️</span>
          <span class="tab-label">Repositories</span>
          <span class="tab-counter">{{ repos.length }}</span>
        </button>
      </div>

      <div class="tabs-extra">
        <button 
          v-if="activeTab === 'repos'" 
          type="button" 
          class="btn-cyber btn-outline-cyan btn-sm"
          :disabled="updatingAllRepos"
          @click="handleUpdateAllRepos"
        >
          <span :class="{ 'spin-anim': updatingAllRepos }">🔄</span>
          <span>{{ updatingAllRepos ? 'Syncing Repos...' : 'Update All Repos' }}</span>
        </button>
        <span v-else class="text-muted font-mono font-xs">
          Cluster: <strong class="text-gold">{{ selectedCluster }}</strong>
        </span>
      </div>
    </nav>

    <!-- TAB 1: RELEASES -->
    <section v-if="activeTab === 'releases'" class="tab-content">
      <!-- Table Filter Bar -->
      <div class="table-toolbar glass-panel">
        <div class="toolbar-search">
          <span class="search-icon">🔍</span>
          <input
            v-model="releaseSearch"
            type="text"
            placeholder="Filter releases by name, chart or namespace..."
            class="input-glass search-input"
          />
          <button 
            v-if="releaseSearch" 
            type="button" 
            class="btn-clear" 
            @click="releaseSearch = ''"
          >
            ✕
          </button>
        </div>

        <div class="toolbar-filters">
          <div class="filter-group">
            <span class="filter-label">Status:</span>
            <select v-model="releaseStatusFilter" class="input-glass select-sm">
              <option value="all">All Statuses</option>
              <option value="deployed">Deployed</option>
              <option value="failed">Failed</option>
              <option value="pending">Pending</option>
              <option value="superseded">Superseded</option>
            </select>
          </div>
        </div>
      </div>

      <!-- Releases Table -->
      <div class="data-table-container glass-panel">
        <div v-if="loading" class="loading-state">
          <div class="cyber-spinner"></div>
          <p class="font-mono text-muted">Retrieving Helm releases from {{ selectedCluster }}...</p>
        </div>

        <div v-else-if="error" class="error-state">
          <span class="error-icon">⚠️</span>
          <h4 class="error-title">Failed to load releases</h4>
          <p class="error-desc">{{ error }}</p>
          <button type="button" class="btn-cyber btn-primary btn-sm" @click="fetchReleases">
            Retry Connection
          </button>
        </div>

        <div v-else-if="filteredReleases.length === 0" class="empty-state">
          <span class="empty-icon">⛵</span>
          <h4 class="empty-title">No Helm Releases Found</h4>
          <p class="empty-desc">
            No releases match your current filters on cluster <code class="text-gold">{{ selectedCluster }}</code>.
          </p>
          <button 
            type="button" 
            class="btn-cyber btn-primary"
            @click="activeTab = 'charts'"
          >
            Browse Chart Catalog & Install
          </button>
        </div>

        <div v-else class="table-scroll-wrapper">
          <table class="cyber-table">
            <thead>
              <tr>
                <th>Release Name</th>
                <th>Chart Package</th>
                <th>Version</th>
                <th>Namespace</th>
                <th>Status</th>
                <th>Revision</th>
                <th>Updated</th>
                <th class="text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr 
                v-for="rel in filteredReleases" 
                :key="`${rel.namespace}/${rel.name}`"
                class="table-row-interactive"
              >
                <!-- Release Name -->
                <td>
                  <div class="release-name-cell" @click="openReleaseDetail(rel)">
                    <span class="release-icon">⛵</span>
                    <div>
                      <span class="release-name-text">{{ rel.name }}</span>
                      <span v-if="rel.description" class="release-desc-sub">{{ rel.description }}</span>
                    </div>
                  </div>
                </td>

                <!-- Chart -->
                <td>
                  <span class="font-mono text-cyan chart-cell">{{ rel.chart }}</span>
                </td>

                <!-- Version & App Version -->
                <td>
                  <div class="version-cell">
                    <span class="badge-tag badge-cyan">{{ rel.version || rel.appVersion || 'v1.0.0' }}</span>
                    <span v-if="rel.app_version || rel.appVersion" class="app-version-sub font-mono">
                      app: {{ rel.app_version || rel.appVersion }}
                    </span>
                  </div>
                </td>

                <!-- Namespace -->
                <td>
                  <span class="ns-badge font-mono">🏷️ {{ rel.namespace }}</span>
                </td>

                <!-- Status Badge -->
                <td>
                  <StatusBadge :status="getStatusType(rel.status)" :label="rel.status" size="sm" />
                </td>

                <!-- Revision -->
                <td>
                  <span class="revision-pill font-mono">rev {{ rel.revision || 1 }}</span>
                </td>

                <!-- Updated -->
                <td>
                  <span class="text-muted font-mono font-xs">{{ formatReleaseDate(rel.updated) }}</span>
                </td>

                <!-- Row Actions -->
                <td class="text-right actions-cell">
                  <div class="action-btn-group">
                    <button
                      type="button"
                      class="btn-row-action btn-action-detail"
                      title="Inspect Release Details & History"
                      @click="openReleaseDetail(rel)"
                    >
                      <span>🔍 Details</span>
                    </button>
                    <button
                      type="button"
                      class="btn-row-action btn-action-upgrade"
                      title="Upgrade Release"
                      @click="openUpgradeModal(rel)"
                    >
                      <span>🔄 Upgrade</span>
                    </button>
                    <button
                      type="button"
                      class="btn-row-action btn-action-rollback"
                      title="Rollback Release Revision"
                      @click="openRollbackModal(rel)"
                    >
                      <span>⏪ Rollback</span>
                    </button>
                    <button
                      type="button"
                      class="btn-row-action btn-action-delete"
                      title="Uninstall Release"
                      @click="promptUninstall(rel)"
                    >
                      <span>🗑️</span>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
    <!-- TAB 2: CHART CATALOG -->
    <section v-if="activeTab === 'charts'" class="tab-content">
      <!-- Search & Filters Toolbar -->
      <div class="catalog-filters-panel glass-panel">
        <div class="search-row">
          <div class="catalog-search-box">
            <span class="search-icon">🔍</span>
            <input
              v-model="chartSearch"
              type="text"
              placeholder="Search Helm charts (e.g. nginx, redis, postgres, prometheus)..."
              class="input-glass catalog-search-input"
              @input="onChartSearchInput"
            />
            <button 
              v-if="chartSearch" 
              type="button" 
              class="btn-clear" 
              @click="chartSearch = ''; fetchCharts()"
            >
              ✕
            </button>
          </div>

          <div class="repo-filter-box">
            <label class="filter-label">Repository:</label>
            <select v-model="selectedRepoFilter" class="input-glass select-repo">
              <option value="all">All Repositories</option>
              <option v-for="repo in repos" :key="repo.name" :value="repo.name">
                🗄️ {{ repo.name }}
              </option>
            </select>
          </div>
        </div>

        <!-- Quick Category Chips -->
        <div class="category-chips-row">
          <button
            v-for="cat in categoryTags"
            :key="cat.key"
            type="button"
            class="chip-btn"
            :class="{ active: selectedCategoryTag === cat.key }"
            @click="selectedCategoryTag = cat.key"
          >
            <span class="chip-icon">{{ cat.icon }}</span>
            <span class="chip-label">{{ cat.label }}</span>
          </button>
        </div>
      </div>

      <!-- Charts Grid -->
      <div v-if="loadingCharts" class="loading-state glass-panel">
        <div class="cyber-spinner"></div>
        <p class="font-mono text-muted">Searching & indexing Helm chart repositories...</p>
      </div>

      <div v-else-if="filteredCharts.length === 0" class="empty-state glass-panel">
        <span class="empty-icon">📦</span>
        <h4 class="empty-title">No Charts Found</h4>
        <p class="empty-desc">
          No charts match your search query or filter. Try searching for other keywords or adding new Helm repositories.
        </p>
        <button type="button" class="btn-cyber btn-primary" @click="openAddRepoModal">
          + Add Repository
        </button>
      </div>

      <div v-else class="charts-grid">
        <div
          v-for="chart in filteredCharts"
          :key="`${chart.repo}/${chart.name}`"
          class="chart-card glass-panel glass-panel-glow"
        >
          <div class="chart-card-header">
            <div class="chart-avatar">
              <img 
                v-if="chart.icon && chart.icon.startsWith('http')" 
                :src="chart.icon" 
                :alt="chart.name" 
                class="chart-icon-img"
                @error="($event.target as HTMLElement).style.display = 'none'"
              />
              <span class="chart-fallback-icon">{{ getChartIcon(chart) }}</span>
            </div>
            <div class="chart-meta">
              <div class="chart-repo-tag">
                <span class="repo-badge">🗄️ {{ chart.repo }}</span>
              </div>
              <h3 class="chart-title" :title="chart.name">{{ chart.name }}</h3>
            </div>
          </div>

          <div class="chart-card-body">
            <p class="chart-desc">
              {{ chart.description || 'Cloud-native application packaged as a standardized Helm chart.' }}
            </p>

            <div class="chart-badges-row">
              <span class="badge-tag badge-cyan font-mono">v{{ chart.version }}</span>
              <span v-if="chart.appVersion || chart.app_version" class="badge-tag badge-violet font-mono">
                app: {{ chart.appVersion || chart.app_version }}
              </span>
            </div>

            <!-- Keywords Chips -->
            <div v-if="chart.keywords && chart.keywords.length > 0" class="chart-keywords">
              <span 
                v-for="kw in chart.keywords.slice(0, 3)" 
                :key="kw" 
                class="keyword-pill font-xs font-mono"
              >
                #{{ kw }}
              </span>
            </div>
          </div>

          <div class="chart-card-footer">
            <button
              type="button"
              class="btn-cyber btn-primary btn-install-chart"
              @click="openInstallWizard(chart)"
            >
              <span>🚀 Install Chart</span>
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- TAB 3: REPOSITORIES -->
    <section v-if="activeTab === 'repos'" class="tab-content">
      <div class="repos-toolbar glass-panel">
        <div class="repos-title-wrap">
          <h3 class="section-title">Configured Helm Repositories</h3>
          <p class="section-subtitle">Manage upstream OCI and HTTP chart registry sources</p>
        </div>
        <div class="repos-actions">
          <button 
            type="button" 
            class="btn-cyber btn-outline-cyan"
            :disabled="updatingAllRepos"
            @click="handleUpdateAllRepos"
          >
            <span :class="{ 'spin-anim': updatingAllRepos }">🔄</span>
            <span>{{ updatingAllRepos ? 'Updating Indexes...' : 'Update All Repos' }}</span>
          </button>
          <button 
            type="button" 
            class="btn-cyber btn-primary"
            @click="openAddRepoModal"
          >
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
          <span class="empty-icon">🗄️</span>
          <h4 class="empty-title">No Repositories Configured</h4>
          <p class="empty-desc">
            Add your first Helm repository (such as Bitnami, NGINX, or Prometheus) to start discovering charts.
          </p>
          <button type="button" class="btn-cyber btn-primary" @click="openAddRepoModal">
            + Add Repository
          </button>
        </div>

        <div v-else class="table-scroll-wrapper">
          <table class="cyber-table">
            <thead>
              <tr>
                <th>Repository Name</th>
                <th>Repository URL</th>
                <th class="text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="repo in repos" :key="repo.name">
                <!-- Repo Name -->
                <td>
                  <div class="repo-name-cell">
                    <span class="repo-icon">🗄️</span>
                    <div>
                      <strong class="font-mono text-primary">{{ repo.name }}</strong>
                    </div>
                  </div>
                </td>

                <!-- Repo URL -->
                <td>
                  <div class="repo-url-cell">
                    <a :href="repo.url" target="_blank" rel="noopener noreferrer" class="repo-link font-mono">
                      {{ repo.url }} ↗
                    </a>
                    <button 
                      type="button" 
                      class="btn-copy-icon" 
                      title="Copy URL"
                      @click="copyToClipboard(repo.url, 'url')"
                    >
                      📋
                    </button>
                  </div>
                </td>

                <!-- Actions -->
                <td class="text-right actions-cell">
                  <div class="action-btn-group">
                    <button
                      type="button"
                      class="btn-row-action btn-action-upgrade"
                      title="Update repository index"
                      @click="handleUpdateAllRepos"
                    >
                      <span>🔄 Update</span>
                    </button>
                    <button
                      type="button"
                      class="btn-row-action btn-action-delete"
                      title="Remove repository"
                      @click="promptRemoveRepo(repo)"
                    >
                      <span>🗑️ Remove</span>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
    <!-- ============================================================= -->
    <!-- MODAL / DRAWER: RELEASE DETAILS -->
    <!-- ============================================================= -->
    <ModalDrawer
      :show="showDetailDrawer"
      mode="drawer"
      placement="right"
      max-width="780px"
      :title="selectedRelease ? `Release: ${selectedRelease.name}` : 'Release Detail'"
      :subtitle="selectedRelease ? `${selectedRelease.namespace} • ${selectedRelease.chart}` : ''"
      @close="showDetailDrawer = false"
    >
      <div v-if="selectedRelease" class="drawer-inner-content">
        <!-- Status Header Card -->
        <div class="release-header-card glass-panel">
          <div class="header-card-row">
            <div class="release-title-block">
              <span class="title-icon">⛵</span>
              <div>
                <h3 class="release-hero-title">{{ selectedRelease.name }}</h3>
                <span class="font-mono font-xs text-muted">
                  Namespace: <strong class="text-cyan">{{ selectedRelease.namespace }}</strong> • Revision: <strong class="text-gold">#{{ selectedRelease.revision || 1 }}</strong>
                </span>
              </div>
            </div>
            <StatusBadge :status="getStatusType(selectedRelease.status)" :label="selectedRelease.status" size="md" />
          </div>

          <div class="header-details-grid">
            <div class="detail-metric">
              <span class="metric-lbl">Chart Package</span>
              <span class="metric-val font-mono text-cyan">{{ selectedRelease.chart }}</span>
            </div>
            <div class="detail-metric">
              <span class="metric-lbl">Chart Version</span>
              <span class="metric-val font-mono text-gold">{{ selectedRelease.version || 'v1.0.0' }}</span>
            </div>
            <div class="detail-metric">
              <span class="metric-lbl">App Version</span>
              <span class="metric-val font-mono text-primary">{{ selectedRelease.app_version || selectedRelease.appVersion || 'latest' }}</span>
            </div>
            <div class="detail-metric">
              <span class="metric-lbl">Last Updated</span>
              <span class="metric-val font-mono text-muted">{{ formatReleaseDate(selectedRelease.updated) }}</span>
            </div>
          </div>

          <div class="header-drawer-actions">
            <button 
              type="button" 
              class="btn-cyber btn-primary btn-sm"
              @click="openUpgradeModal(selectedRelease)"
            >
              🔄 Upgrade Release
            </button>
            <button 
              type="button" 
              class="btn-cyber btn-secondary btn-sm"
              @click="openRollbackModal(selectedRelease)"
            >
              ⏪ Rollback Revision
            </button>
            <button 
              type="button" 
              class="btn-cyber btn-danger btn-sm"
              @click="promptUninstall(selectedRelease)"
            >
              🗑️ Uninstall
            </button>
          </div>
        </div>

        <!-- Drawer Navigation Subtabs -->
        <div class="drawer-tabs-bar">
          <button
            type="button"
            class="subtab-btn"
            :class="{ active: activeDrawerTab === 'overview' }"
            @click="activeDrawerTab = 'overview'"
          >
            <span>📜 Notes & Overview</span>
          </button>
          <button
            type="button"
            class="subtab-btn"
            :class="{ active: activeDrawerTab === 'values' }"
            @click="activeDrawerTab = 'values'"
          >
            <span>⚙️ Values (YAML)</span>
          </button>
          <button
            type="button"
            class="subtab-btn"
            :class="{ active: activeDrawerTab === 'manifest' }"
            @click="activeDrawerTab = 'manifest'"
          >
            <span>📄 K8s Manifest</span>
          </button>
          <button
            type="button"
            class="subtab-btn"
            :class="{ active: activeDrawerTab === 'history' }"
            @click="activeDrawerTab = 'history'"
          >
            <span>⏱️ Revision History</span>
          </button>
        </div>

        <!-- Subtab 1: Notes & Overview -->
        <div v-if="activeDrawerTab === 'overview'" class="drawer-tab-pane">
          <div class="notes-panel glass-panel">
            <div class="notes-panel-header">
              <h4 class="font-mono text-cyan font-sm">RELEASE NOTES & ACCESS GUIDANCE</h4>
              <button
                v-if="selectedRelease.notes"
                type="button"
                class="btn-cyber btn-outline-cyan btn-xs"
                @click="copyToClipboard(selectedRelease.notes, 'url')"
              >
                📋 Copy Notes
              </button>
            </div>
            <pre v-if="selectedRelease.notes" class="cyber-code-block">{{ selectedRelease.notes }}</pre>
            <p v-else class="text-muted font-mono font-xs text-center py-4">
              No release notes provided by the chart maintainer.
            </p>
          </div>
        </div>

        <!-- Subtab 2: Values (YAML) -->
        <div v-if="activeDrawerTab === 'values'" class="drawer-tab-pane">
          <div class="values-panel glass-panel">
            <div class="panel-actions-row">
              <span class="font-mono font-xs text-muted">Active Helm Values (User & Computed)</span>
              <div class="btn-group-sm">
                <button
                  type="button"
                  class="btn-cyber btn-outline-cyan btn-xs"
                  @click="copyToClipboard(typeof selectedRelease.values === 'string' ? selectedRelease.values : JSON.stringify(selectedRelease.values, null, 2), 'values')"
                >
                  <span>{{ valuesCopied ? '✓ Copied!' : '📋 Copy YAML' }}</span>
                </button>
                <button
                  type="button"
                  class="btn-cyber btn-secondary btn-xs"
                  @click="downloadAsFile(`${selectedRelease.name}-values.yaml`, typeof selectedRelease.values === 'string' ? selectedRelease.values : JSON.stringify(selectedRelease.values, null, 2))"
                >
                  <span>💾 Download</span>
                </button>
              </div>
            </div>
            <pre class="cyber-code-block yaml-display">{{ typeof selectedRelease.values === 'string' ? selectedRelease.values : JSON.stringify(selectedRelease.values, null, 2) || '# No values configured' }}</pre>
          </div>
        </div>

        <!-- Subtab 3: Manifest -->
        <div v-if="activeDrawerTab === 'manifest'" class="drawer-tab-pane">
          <div class="manifest-panel glass-panel">
            <div class="panel-actions-row">
              <span class="font-mono font-xs text-muted">Synthesized Kubernetes Manifests</span>
              <div class="btn-group-sm">
                <button
                  type="button"
                  class="btn-cyber btn-outline-cyan btn-xs"
                  @click="copyToClipboard(selectedRelease.manifest || '', 'manifest')"
                >
                  <span>{{ manifestCopied ? '✓ Copied!' : '📋 Copy Manifest' }}</span>
                </button>
                <button
                  type="button"
                  class="btn-cyber btn-secondary btn-xs"
                  @click="downloadAsFile(`${selectedRelease.name}-manifest.yaml`, selectedRelease.manifest || '')"
                >
                  <span>💾 Download</span>
                </button>
              </div>
            </div>
            <pre class="cyber-code-block yaml-display">{{ selectedRelease.manifest || '# No manifest data available' }}</pre>
          </div>
        </div>

        <!-- Subtab 4: Revision History -->
        <div v-if="activeDrawerTab === 'history'" class="drawer-tab-pane">
          <div v-if="loadingHistory" class="loading-state">
            <div class="cyber-spinner"></div>
            <p class="font-mono text-muted">Retrieving revision timeline...</p>
          </div>

          <div v-else-if="releaseHistory.length === 0" class="empty-state">
            <span class="empty-icon">⏱️</span>
            <p class="font-mono text-muted">No revision history found.</p>
          </div>

          <div v-else class="history-table-wrapper glass-panel">
            <table class="cyber-table table-compact">
              <thead>
                <tr>
                  <th>Revision</th>
                  <th>Chart</th>
                  <th>App Version</th>
                  <th>Status</th>
                  <th>Updated</th>
                  <th class="text-right">Action</th>
                </tr>
              </thead>
              <tbody>
                <tr 
                  v-for="hist in releaseHistory" 
                  :key="hist.revision"
                  :class="{ 'row-highlight': hist.revision === selectedRelease.revision }"
                >
                  <td>
                    <span class="revision-pill font-mono">#{{ hist.revision }}</span>
                    <span v-if="hist.revision === selectedRelease.revision" class="current-tag">CURRENT</span>
                  </td>
                  <td><span class="font-mono text-cyan">{{ hist.chart }}</span></td>
                  <td><span class="font-mono text-muted">{{ hist.app_version || hist.appVersion || '—' }}</span></td>
                  <td><StatusBadge :status="getStatusType(hist.status)" :label="hist.status" size="sm" /></td>
                  <td><span class="font-mono font-xs text-muted">{{ formatReleaseDate(hist.updated) }}</span></td>
                  <td class="text-right">
                    <button
                      v-if="hist.revision !== selectedRelease.revision"
                      type="button"
                      class="btn-cyber btn-secondary btn-xs"
                      @click="rollbackRevision = hist.revision; showRollbackModal = true"
                    >
                      ⏪ Rollback
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </ModalDrawer>

    <!-- ============================================================= -->
    <!-- MODAL: INSTALL CHART WIZARD -->
    <!-- ============================================================= -->
    <ModalDrawer
      :show="showInstallModal"
      mode="modal"
      max-width="740px"
      :title="selectedChartForInstall ? `Install Chart: ${selectedChartForInstall.name}` : 'Install Chart'"
      :subtitle="selectedChartForInstall ? `Repository: ${selectedChartForInstall.repo} • Version: ${selectedChartForInstall.version}` : ''"
      @close="showInstallModal = false"
    >
      <div v-if="selectedChartForInstall" class="install-wizard-container">
        <!-- Wizard Step Indicators -->
        <div class="wizard-steps-header">
          <div class="step-badge" :class="{ active: installStep >= 1 }">
            <span class="step-num">1</span>
            <span class="step-name">Release & Scope</span>
          </div>
          <div class="step-connector" :class="{ active: installStep >= 2 }"></div>
          <div class="step-badge" :class="{ active: installStep >= 2 }">
            <span class="step-num">2</span>
            <span class="step-name">Custom Values</span>
          </div>
          <div class="step-connector" :class="{ active: installStep === 3 }"></div>
          <div class="step-badge" :class="{ active: installStep === 3 }">
            <span class="step-num">3</span>
            <span class="step-name">Review & Deploy</span>
          </div>
        </div>

        <!-- Step 1: Release Configuration -->
        <div v-if="installStep === 1" class="wizard-step-pane">
          <div class="form-grid">
            <div class="form-group">
              <label class="form-label">Release Name <span class="text-rose">*</span></label>
              <input
                v-model="installForm.releaseName"
                type="text"
                class="input-glass"
                placeholder="e.g. my-production-nginx"
              />
              <span class="form-hint">Lowercase alphanumeric characters and dashes only</span>
            </div>

            <div class="form-group">
              <label class="form-label">Chart Version</label>
              <input
                v-model="installForm.version"
                type="text"
                class="input-glass"
                placeholder="e.g. 1.2.0"
              />
            </div>

            <div class="form-group">
              <label class="form-label">Target Namespace <span class="text-rose">*</span></label>
              <select v-model="installForm.namespace" class="input-glass">
                <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">
                  🏷️ {{ ns.name }}
                </option>
                <option v-if="!namespaces.some(n => n.name === 'default')" value="default">
                  🏷️ default
                </option>
              </select>
            </div>

            <div class="form-group checkbox-group">
              <label class="cyber-checkbox-label">
                <input
                  v-model="installForm.createNamespace"
                  type="checkbox"
                  class="cyber-checkbox"
                />
                <span>Create namespace if it does not exist</span>
              </label>
            </div>
          </div>

          <div class="modal-footer-actions">
            <button type="button" class="btn-cyber btn-secondary" @click="showInstallModal = false">
              Cancel
            </button>
            <button 
              type="button" 
              class="btn-cyber btn-primary"
              :disabled="!installForm.releaseName.trim() || !installForm.namespace.trim()"
              @click="installStep = 2"
            >
              Next: Configure Values →
            </button>
          </div>
        </div>

        <!-- Step 2: Values Editor -->
        <div v-if="installStep === 2" class="wizard-step-pane">
          <div class="values-editor-container">
            <div class="editor-header-bar">
              <span class="font-mono font-xs text-muted">values.yaml Configuration</span>
              <button
                type="button"
                class="btn-cyber btn-secondary btn-xs"
                :disabled="installForm.loadingDefaultValues"
                @click="openInstallWizard(selectedChartForInstall)"
              >
                <span>Reset to Default Values</span>
              </button>
            </div>

            <textarea
              v-model="installForm.values"
              class="cyber-textarea yaml-editor"
              rows="12"
              placeholder="# Enter custom Helm values in YAML syntax..."
            ></textarea>
          </div>

          <div class="modal-footer-actions">
            <button type="button" class="btn-cyber btn-secondary" @click="installStep = 1">
              ← Back
            </button>
            <button type="button" class="btn-cyber btn-primary" @click="installStep = 3">
              Next: Review & Install →
            </button>
          </div>
        </div>

        <!-- Step 3: Review & Install -->
        <div v-if="installStep === 3" class="wizard-step-pane">
          <div class="summary-card glass-panel">
            <h4 class="summary-title font-mono text-cyan">INSTALLATION SUMMARY</h4>
            <div class="summary-grid">
              <div class="summary-item">
                <span class="summary-label">Target Cluster:</span>
                <span class="summary-val text-gold font-mono">{{ selectedCluster }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Release Name:</span>
                <span class="summary-val text-primary font-mono">{{ installForm.releaseName }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Namespace:</span>
                <span class="summary-val text-cyan font-mono">{{ installForm.namespace }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Chart:</span>
                <span class="summary-val font-mono">{{ selectedChartForInstall.repo }}/{{ selectedChartForInstall.name }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Version:</span>
                <span class="summary-val font-mono text-gold">v{{ installForm.version }}</span>
              </div>
            </div>
          </div>

          <div class="modal-footer-actions">
            <button type="button" class="btn-cyber btn-secondary" @click="installStep = 2">
              ← Back
            </button>
            <button
              type="button"
              class="btn-cyber btn-primary"
              :disabled="installing"
              @click="handleInstallChart"
            >
              <span :class="{ 'spin-anim': installing }">🚀</span>
              <span>{{ installing ? 'Deploying Chart...' : 'Confirm & Deploy Release' }}</span>
            </button>
          </div>
        </div>
      </div>
    </ModalDrawer>
    <!-- ============================================================= -->
    <!-- MODAL: UPGRADE RELEASE -->
    <!-- ============================================================= -->
    <ModalDrawer
      :show="showUpgradeModal"
      mode="modal"
      max-width="680px"
      :title="upgradeTarget ? `Upgrade Release: ${upgradeTarget.name}` : 'Upgrade Release'"
      :subtitle="upgradeTarget ? `Namespace: ${upgradeTarget.namespace} • Current: ${upgradeTarget.chart}` : ''"
      @close="showUpgradeModal = false"
    >
      <div v-if="upgradeTarget" class="modal-form-body">
        <div class="form-grid">
          <div class="form-group">
            <label class="form-label">Target Version</label>
            <input
              v-model="upgradeForm.version"
              type="text"
              class="input-glass"
              placeholder="e.g. 1.3.0"
            />
          </div>

          <div class="form-group checkbox-group">
            <label class="cyber-checkbox-label">
              <input
                v-model="upgradeForm.reuseValues"
                type="checkbox"
                class="cyber-checkbox"
              />
              <span>Reuse existing release values</span>
            </label>
          </div>

          <div class="form-group full-width">
            <label class="form-label">Custom Values Override (YAML)</label>
            <textarea
              v-model="upgradeForm.values"
              class="cyber-textarea"
              rows="8"
              placeholder="# Optional YAML values to merge/override..."
            ></textarea>
          </div>
        </div>

        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="showUpgradeModal = false">
            Cancel
          </button>
          <button
            type="button"
            class="btn-cyber btn-primary"
            :disabled="upgrading"
            @click="handleUpgradeRelease"
          >
            <span :class="{ 'spin-anim': upgrading }">🔄</span>
            <span>{{ upgrading ? 'Upgrading Release...' : 'Deploy Upgrade' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- ============================================================= -->
    <!-- MODAL: ROLLBACK RELEASE -->
    <!-- ============================================================= -->
    <ModalDrawer
      :show="showRollbackModal"
      mode="modal"
      max-width="580px"
      :title="rollbackTarget ? `Rollback Release: ${rollbackTarget.name}` : 'Rollback Release'"
      :subtitle="rollbackTarget ? `Current Revision: #${rollbackTarget.revision || 1}` : ''"
      @close="showRollbackModal = false"
    >
      <div v-if="rollbackTarget" class="modal-form-body">
        <div v-if="loadingRollbackHistory" class="loading-state">
          <div class="cyber-spinner"></div>
          <p class="font-mono text-muted">Fetching revision history...</p>
        </div>

        <div v-else class="form-grid">
          <div class="form-group full-width">
            <label class="form-label">Select Revision to Restore <span class="text-rose">*</span></label>
            <select v-model="rollbackRevision" class="input-glass">
              <option 
                v-for="h in rollbackHistoryList" 
                :key="h.revision" 
                :value="h.revision"
              >
                Revision #{{ h.revision }} — {{ h.chart }} ({{ h.status }}) [{{ formatReleaseDate(h.updated) }}]
              </option>
            </select>
          </div>

          <div class="alert-box alert-warning full-width">
            <span>⚠️ Rollback will revert cluster resources to the state defined in Revision #{{ rollbackRevision || '?' }}.</span>
          </div>
        </div>

        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="showRollbackModal = false">
            Cancel
          </button>
          <button
            type="button"
            class="btn-cyber btn-warning"
            :disabled="rollingBack || rollbackRevision === null"
            @click="handleRollbackRelease"
          >
            <span :class="{ 'spin-anim': rollingBack }">⏪</span>
            <span>{{ rollingBack ? 'Rolling back...' : 'Confirm Rollback' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- ============================================================= -->
    <!-- MODAL: UNINSTALL RELEASE CONFIRMATION -->
    <!-- ============================================================= -->
    <ModalDrawer
      :show="showUninstallModal"
      mode="modal"
      max-width="520px"
      title="Confirm Release Uninstall"
      subtitle="Danger Zone: Resource Removal"
      @close="showUninstallModal = false"
    >
      <div v-if="uninstallTarget" class="modal-form-body">
        <div class="danger-warning-box">
          <span class="warning-icon">🔥</span>
          <p>
            You are about to uninstall <strong class="text-rose font-mono">{{ uninstallTarget.name }}</strong> in namespace <strong class="text-cyan font-mono">{{ uninstallTarget.namespace }}</strong>. All Kubernetes workloads, services, and associated resources managed by this Helm release will be deleted.
          </p>
        </div>

        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="showUninstallModal = false">
            Cancel
          </button>
          <button
            type="button"
            class="btn-cyber btn-danger"
            :disabled="uninstalling"
            @click="handleUninstallRelease"
          >
            <span>{{ uninstalling ? 'Uninstalling...' : 'Uninstall Release' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- ============================================================= -->
    <!-- MODAL: ADD REPOSITORY -->
    <!-- ============================================================= -->
    <ModalDrawer
      :show="showAddRepoModal"
      mode="modal"
      max-width="600px"
      title="Add Helm Repository"
      subtitle="Register an HTTP or OCI chart repository"
      @close="showAddRepoModal = false"
    >
      <div class="modal-form-body">
        <div class="form-grid">
          <div class="form-group">
            <label class="form-label">Repository Name <span class="text-rose">*</span></label>
            <input
              v-model="addRepoForm.name"
              type="text"
              class="input-glass"
              placeholder="e.g. bitnami"
            />
          </div>

          <div class="form-group">
            <label class="form-label">Repository URL <span class="text-rose">*</span></label>
            <input
              v-model="addRepoForm.url"
              type="url"
              class="input-glass"
              placeholder="https://charts.bitnami.com/bitnami"
            />
          </div>

          <!-- Popular Presets -->
          <div class="form-group full-width">
            <label class="form-label text-muted">Quick Presets:</label>
            <div class="presets-chips">
              <button
                v-for="p in repoPresets"
                :key="p.name"
                type="button"
                class="preset-chip-btn"
                @click="applyRepoPreset(p)"
              >
                <span>{{ p.icon }}</span>
                <span>{{ p.name }}</span>
              </button>
            </div>
          </div>
        </div>

        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="showAddRepoModal = false">
            Cancel
          </button>
          <button
            type="button"
            class="btn-cyber btn-primary"
            :disabled="addingRepo || !addRepoForm.name.trim() || !addRepoForm.url.trim()"
            @click="handleAddRepo"
          >
            <span :class="{ 'spin-anim': addingRepo }">➕</span>
            <span>{{ addingRepo ? 'Adding & Indexing...' : 'Add Repository' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- ============================================================= -->
    <!-- MODAL: REMOVE REPOSITORY CONFIRMATION -->
    <!-- ============================================================= -->
    <ModalDrawer
      :show="showRemoveRepoModal"
      mode="modal"
      max-width="480px"
      title="Remove Helm Repository"
      subtitle="Unregister chart repository"
      @close="showRemoveRepoModal = false"
    >
      <div v-if="repoToRemove" class="modal-form-body">
        <p class="text-body font-mono">
          Are you sure you want to remove repository <strong class="text-rose">{{ repoToRemove.name }}</strong> ({{ repoToRemove.url }})?
        </p>

        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="showRemoveRepoModal = false">
            Cancel
          </button>
          <button
            type="button"
            class="btn-cyber btn-danger"
            :disabled="removingRepo"
            @click="handleRemoveRepo"
          >
            <span>{{ removingRepo ? 'Removing...' : 'Remove Repository' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>
  </div>
</template>
<style scoped>
.helm-catalog-layout {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 24px;
  max-width: 1600px;
  margin: 0 auto;
  min-height: 100vh;
}

/* Cyber Header & Metrics */
.page-header {
  padding: 24px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.header-main {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
  flex-wrap: wrap;
}

.title-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.badge-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pulse-dot-gold {
  background: var(--color-primary);
  box-shadow: 0 0 10px var(--color-primary);
  width: 8px;
  height: 8px;
  border-radius: 50%;
  animation: pulse-glow 2s infinite ease-in-out;
}

.cyber-tag {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--color-primary);
  font-family: var(--font-mono);
  text-transform: uppercase;
}

.page-title {
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--text-primary);
  margin: 0;
}

.title-compact {
  display: none;
}

.page-subtitle {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0;
}

.header-controls {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  flex-wrap: wrap;
}

.control-box {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.control-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.header-select {
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 13px;
  min-width: 170px;
}

.header-actions-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

/* Navigation Tabs Bar */
.catalog-tabs-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  border-radius: 12px;
}

.tabs-list {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: 8px;
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-secondary);
  font-weight: 600;
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.tab-btn:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.04);
}

.tab-btn.active {
  color: var(--color-primary);
  background: rgba(252, 213, 53, 0.08);
  border-color: rgba(252, 213, 53, 0.25);
}

.tab-counter {
  background: rgba(255, 255, 255, 0.08);
  color: inherit;
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 9999px;
  font-family: var(--font-mono);
}

.tab-btn.active .tab-counter {
  background: rgba(252, 213, 53, 0.2);
}

.tabs-extra {
  display: flex;
  align-items: center;
}

/* Tab Content & Toolbars */
.tab-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.table-toolbar, .repos-toolbar, .catalog-filters-panel {
  padding: 14px 18px;
  border-radius: 12px;
}

.table-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.toolbar-search {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  max-width: 450px;
}

.search-icon {
  position: absolute;
  left: 12px;
  font-size: 14px;
  pointer-events: none;
  opacity: 0.6;
}

.search-input {
  width: 100%;
  padding: 8px 32px 8px 34px;
  border-radius: 8px;
  font-size: 13px;
}

.btn-clear {
  position: absolute;
  right: 10px;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: 12px;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.filter-label {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.select-sm {
  padding: 6px 10px;
  font-size: 12px;
  border-radius: 6px;
}
/* Cyber Data Table */
.data-table-container {
  border-radius: 14px;
  overflow: hidden;
  padding: 4px;
}

.table-scroll-wrapper {
  overflow-x: auto;
}

.cyber-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 13px;
}

.cyber-table th {
  padding: 12px 16px;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid var(--color-hairline);
  font-family: var(--font-mono);
  white-space: nowrap;
}

.cyber-table td {
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  vertical-align: middle;
}

.table-row-interactive {
  transition: background var(--transition-fast);
}

.table-row-interactive:hover {
  background: rgba(255, 255, 255, 0.03);
}

.release-name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
}

.release-icon {
  font-size: 18px;
}

.release-name-text {
  font-weight: 700;
  color: var(--text-primary);
  font-family: var(--font-mono);
}

.release-name-cell:hover .release-name-text {
  color: var(--color-primary);
}

.release-desc-sub {
  display: block;
  font-size: 11px;
  color: var(--text-muted);
  max-width: 260px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chart-cell {
  font-weight: 600;
}

.version-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.app-version-sub {
  font-size: 10px;
  color: var(--text-muted);
}

.ns-badge {
  font-size: 12px;
  color: var(--text-secondary);
}

.revision-pill {
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.06);
  font-size: 11px;
  color: var(--text-secondary);
}

.action-btn-group {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.btn-row-action {
  padding: 5px 9px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.btn-row-action:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.1);
}

.btn-action-detail:hover {
  border-color: rgba(6, 182, 212, 0.5);
  color: #38bdf8;
}

.btn-action-upgrade:hover {
  border-color: rgba(252, 213, 53, 0.5);
  color: var(--color-primary);
}

.btn-action-rollback:hover {
  border-color: rgba(245, 158, 11, 0.5);
  color: #fbbf24;
}

.btn-action-delete:hover {
  border-color: rgba(244, 63, 94, 0.5);
  color: #fb7185;
}

/* Chart Catalog Tab Styling */
.catalog-filters-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.search-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.catalog-search-box {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 280px;
}

.catalog-search-input {
  width: 100%;
  padding: 10px 36px 10px 36px;
  border-radius: 8px;
  font-size: 14px;
}

.repo-filter-box {
  display: flex;
  align-items: center;
  gap: 8px;
}

.select-repo {
  padding: 9px 12px;
  border-radius: 8px;
  font-size: 13px;
  min-width: 180px;
}

.category-chips-row {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
}

.chip-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  transition: all var(--transition-fast);
}

.chip-btn:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.08);
}

.chip-btn.active {
  background: rgba(252, 213, 53, 0.12);
  border-color: rgba(252, 213, 53, 0.4);
  color: var(--color-primary);
}

.charts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.chart-card {
  padding: 20px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 14px;
  transition: transform var(--transition-normal), border-color var(--transition-normal);
}

.chart-card:hover {
  transform: translateY(-2px);
  border-color: rgba(252, 213, 53, 0.35);
}

.chart-card-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.chart-avatar {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
}

.chart-icon-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.chart-fallback-icon {
  font-size: 22px;
}

.chart-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow: hidden;
}

.repo-badge {
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  text-transform: uppercase;
}

.chart-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chart-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.45;
  margin: 0 0 10px 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.chart-badges-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.chart-keywords {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.keyword-pill {
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.04);
  padding: 1px 6px;
  border-radius: 4px;
}

.btn-install-chart {
  width: 100%;
  justify-content: center;
  padding: 9px 14px;
}
/* Repositories Tab */
.repos-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.section-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.section-subtitle {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0;
}

.repos-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.repo-name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.repo-icon {
  font-size: 18px;
}

.repo-url-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.repo-link {
  color: #38bdf8;
  text-decoration: none;
  font-size: 12px;
}

.repo-link:hover {
  text-decoration: underline;
}

.btn-copy-icon {
  background: transparent;
  border: none;
  cursor: pointer;
  opacity: 0.6;
  transition: opacity var(--transition-fast);
}

.btn-copy-icon:hover {
  opacity: 1;
}

/* Release Detail Drawer Content */
.drawer-inner-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.release-header-card {
  padding: 18px 20px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.header-card-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.release-title-block {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-icon {
  font-size: 26px;
}

.release-hero-title {
  font-size: 18px;
  font-weight: 800;
  color: var(--text-primary);
  margin: 0;
  font-family: var(--font-mono);
}

.header-details-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  padding-top: 10px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.detail-metric {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.metric-lbl {
  font-size: 10px;
  color: var(--text-muted);
  text-transform: uppercase;
  font-family: var(--font-mono);
}

.metric-val {
  font-size: 12px;
  font-weight: 600;
}

.header-drawer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 8px;
}

.drawer-tabs-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  border-bottom: 1px solid var(--color-hairline);
  padding-bottom: 8px;
  overflow-x: auto;
}

.subtab-btn {
  padding: 6px 12px;
  border-radius: 6px;
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}

.subtab-btn:hover {
  color: var(--text-primary);
}

.subtab-btn.active {
  color: var(--color-primary);
  background: rgba(252, 213, 53, 0.1);
  border-color: rgba(252, 213, 53, 0.25);
}

.drawer-tab-pane {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.notes-panel, .values-panel, .manifest-panel, .history-table-wrapper {
  padding: 16px;
  border-radius: 10px;
}

.notes-panel-header, .panel-actions-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.btn-group-sm {
  display: flex;
  align-items: center;
  gap: 6px;
}

.cyber-code-block {
  background: #090c10;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 12px 14px;
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: #c9d1d9;
  line-height: 1.5;
  overflow-x: auto;
  max-height: 480px;
  white-space: pre-wrap;
}

.yaml-display {
  white-space: pre;
}

.table-compact th, .table-compact td {
  padding: 8px 10px;
}

.row-highlight {
  background: rgba(252, 213, 53, 0.05);
}

.current-tag {
  font-size: 9px;
  font-family: var(--font-mono);
  background: var(--color-primary);
  color: #000;
  font-weight: 800;
  padding: 1px 4px;
  border-radius: 3px;
  margin-left: 6px;
}

/* Install Wizard Styles */
.install-wizard-container {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.wizard-steps-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: rgba(255, 255, 255, 0.02);
  border-radius: 10px;
}

.step-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 600;
}

.step-badge.active {
  color: var(--color-primary);
}

.step-num {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.08);
  font-size: 11px;
}

.step-badge.active .step-num {
  background: var(--color-primary);
  color: #000;
  font-weight: 800;
}

.step-connector {
  flex: 1;
  height: 2px;
  background: rgba(255, 255, 255, 0.08);
  margin: 0 10px;
}

.step-connector.active {
  background: var(--color-primary);
}

.wizard-step-pane {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group.full-width {
  grid-column: span 2;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.form-hint {
  font-size: 10px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.checkbox-group {
  grid-column: span 2;
  justify-content: center;
}

.cyber-checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-secondary);
  cursor: pointer;
}

.cyber-checkbox {
  accent-color: var(--color-primary);
}

.values-editor-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.editor-header-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.cyber-textarea {
  width: 100%;
  padding: 10px 12px;
  background: #090c10;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.45;
  resize: vertical;
}

.summary-card {
  padding: 16px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.summary-title {
  margin: 0;
  font-size: 12px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}

.summary-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.summary-label {
  font-size: 11px;
  color: var(--text-muted);
}

.summary-val {
  font-size: 13px;
  font-weight: 600;
}

.modal-footer-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
  padding-top: 14px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.presets-chips {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.preset-chip-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.preset-chip-btn:hover {
  background: rgba(252, 213, 53, 0.1);
  border-color: rgba(252, 213, 53, 0.3);
  color: var(--color-primary);
}

.danger-warning-box {
  padding: 14px;
  border-radius: 8px;
  background: rgba(244, 63, 94, 0.1);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
  font-size: 13px;
  line-height: 1.5;
}

.alert-box {
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 12px;
}

.alert-warning {
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.3);
  color: #fbbf24;
}

/* Common UI Components & Helpers */
.loading-state, .empty-state, .error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  text-align: center;
  gap: 12px;
}

.cyber-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid rgba(252, 213, 53, 0.15);
  border-top-color: var(--color-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.empty-icon, .error-icon {
  font-size: 38px;
}

.empty-title, .error-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.empty-desc, .error-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 420px;
  margin: 0;
}

.error-title {
  color: #fb7185;
}

.cyber-toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 9999;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 18px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(12px);
}

.toast-success {
  background: rgba(16, 185, 129, 0.95);
  color: #fff;
  border: 1px solid #34d399;
}

.toast-error {
  background: rgba(244, 63, 94, 0.95);
  color: #fff;
  border: 1px solid #fb7185;
}

.toast-slide-enter-active, .toast-slide-leave-active {
  transition: all 0.25s ease;
}
.toast-slide-enter-from, .toast-slide-leave-to {
  opacity: 0;
  transform: translateY(20px);
}

.spin-anim {
  display: inline-block;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes pulse-glow {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.85); }
}

/* Responsive Breakpoints */
@media (max-width: 1024px) {
  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .helm-catalog-layout {
    padding: 12px;
    gap: 14px;
  }

  .page-header {
    padding: 16px;
  }

  .title-full {
    display: none;
  }

  .title-compact {
    display: block;
    font-size: 20px;
  }

  .header-controls {
    flex-direction: column;
    align-items: stretch;
    width: 100%;
  }

  .header-select {
    min-width: 100%;
  }

  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
  }

  .catalog-tabs-bar {
    padding: 6px;
    overflow-x: auto;
  }

  .tabs-list {
    flex: 1;
  }

  .tabs-extra {
    display: none;
  }

  .header-details-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .form-group.full-width {
    grid-column: span 1;
  }
}
</style>
