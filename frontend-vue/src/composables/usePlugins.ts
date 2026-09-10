import { ref, computed, onMounted } from 'vue'
import { pluginsApi, type Plugin, type PluginStats, type CreatePluginDTO } from '../api/plugins'
import { pluginLoader } from '../services/pluginLoader'

export interface WasmSandboxStatus {
  activeSandboxes: number
  isolationMode: 'v8-isolate' | 'wasm-wasi' | 'strict'
  memoryUsageMb: number
  hotReloadEnabled: boolean
}

export function usePlugins() {
  const plugins = ref<Plugin[]>([])
  const stats = ref<PluginStats>({ total: 0, enabled: 0, disabled: 0, by_category: {} })
  const loading = ref(false)
  const error = ref<string | null>(null)
  const viewMode = ref<'grid' | 'table'>('grid')

  const searchQuery = ref('')
  const selectedCategory = ref('all')
  const selectedStatus = ref('all')
  const selectedScope = ref('all')
  const togglingId = ref<string | null>(null)
  const installingPreset = ref(false)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    toastMessage.value = { text, type }
    setTimeout(() => {
      if (toastMessage.value?.text === text) {
        toastMessage.value = null
      }
    }, 4000)
  }

  const wasmSandboxStatus = ref<WasmSandboxStatus>({
    activeSandboxes: 0,
    isolationMode: 'wasm-wasi',
    memoryUsageMb: 12.4,
    hotReloadEnabled: true,
  })

  const testResult = ref<{ pluginName: string; status: 'success' | 'error'; message: string } | null>(null)

  const starterPresets: CreatePluginDTO[] = [
    { name: 'Trivy Security Scanner', version: '1.4.2', category: 'security', icon: '🛡️', author: 'Aqua Security & Platform SRE', description: 'Injects live CVE vulnerability analysis, SBOM component graphs, and image scan metrics directly into pod detail drawers.', entry_point: 'https://cdn.jsdelivr.net/npm/@k8s-plugins/trivy-scanner/dist/index.js', permissions: ['audit:read', 'k8s:read', 'cve:read'], config: { scanner_endpoint: 'http://trivy.security:4954', auto_scan_on_mount: 'true', min_severity: 'HIGH' } },
    { name: 'Grafana Dashboard Embed', version: '2.1.0', category: 'monitoring', icon: '📈', author: 'Grafana Labs Community', description: 'Seamlessly embeds contextual Grafana latency, throughput, and CPU/memory panels into deployment and service views.', entry_point: 'https://cdn.jsdelivr.net/npm/@k8s-plugins/grafana-embed/dist/index.js', permissions: ['metrics:read', 'dashboards:view'], config: { grafana_url: 'http://grafana.monitoring:3000', theme: 'dark', refresh_interval: '15s' } },
    { name: 'Prometheus AlertBridge', version: '1.2.0', category: 'monitoring', icon: '🔥', author: 'Prometheus Authors', description: 'Connects Alertmanager active firings, alert routing rules, and Prometheus SLI breach warnings to real-time notification toasts.', entry_point: 'https://cdn.jsdelivr.net/npm/@k8s-plugins/alert-bridge/dist/index.js', permissions: ['alerts:read', 'notifications:send'], config: { alertmanager_url: 'http://alertmanager:9093', poll_interval_sec: '10', dedup_window: '60s' } },
    { name: 'Vector Log Processor', version: '1.1.5', category: 'devtools', icon: '📜', author: 'Timber.io & Vector OSS', description: 'High-throughput client-side log parsing, regex highlighting, and structured JSON stream extraction for container logs.', entry_point: 'https://cdn.jsdelivr.net/npm/@k8s-plugins/vector-logs/dist/index.js', permissions: ['logs:read', 'k8s:read'], config: { buffer_lines: '5000', ansi_highlighting: 'true', filter_heartbeats: 'true' } },
  ]

  const categories = [
    { value: 'all', label: 'All Categories', icon: '🌐' },
    { value: 'monitoring', label: 'Monitoring', icon: '📊' },
    { value: 'security', label: 'Security', icon: '🛡️' },
    { value: 'devtools', label: 'DevTools', icon: '🛠️' },
    { value: 'integration', label: 'Integration', icon: '🔌' },
  ]

  const showFormModal = ref(false)
  const isEditing = ref(false)
  const editingId = ref<string | null>(null)
  const formSubmitting = ref(false)
  const formError = ref<string | null>(null)
  const form = ref({ name: '', version: '1.0.0', category: 'devtools', author: '', icon: '🧩', entry_point: '', description: '', enabled: true })
  const formPermissionsRaw = ref('')

  const showConfigModal = ref(false)
  const activeConfigPlugin = ref<Plugin | null>(null)
  const configPairs = ref<{ key: string; value: string }[]>([])
  const configSaving = ref(false)
  const configSaveError = ref<string | null>(null)

  const categoryCount = computed(() => Object.keys(stats.value.by_category || {}).length || 3)

  const availablePermissionScopes = computed(() => {
    const scopeSet = new Set<string>()
    for (const p of plugins.value) {
      for (const perm of p.permissions || []) {
        const scope = perm.split(':')[0]
        if (scope) scopeSet.add(scope)
      }
    }
    return Array.from(scopeSet).sort()
  })

  const filteredPlugins = computed(() => {
    return plugins.value.filter((p) => {
      if (selectedCategory.value !== 'all' && p.category !== selectedCategory.value) return false
      if (selectedStatus.value === 'enabled' && !p.enabled) return false
      if (selectedStatus.value === 'disabled' && p.enabled) return false
      if (selectedScope.value !== 'all') {
        const hasScope = (p.permissions || []).some((perm) => perm.startsWith(selectedScope.value + ':') || perm === selectedScope.value)
        if (!hasScope) return false
      }
      if (searchQuery.value.trim()) {
        const q = searchQuery.value.toLowerCase().trim()
        const matchName = p.name.toLowerCase().includes(q)
        const matchDesc = (p.description || '').toLowerCase().includes(q)
        const matchAuthor = (p.author || '').toLowerCase().includes(q)
        const matchPerms = (p.permissions || []).some((perm) => perm.toLowerCase().includes(q))
        if (!matchName && !matchDesc && !matchAuthor && !matchPerms) return false
      }
      return true
    })
  })

  async function loadData() {
    loading.value = true
    error.value = null
    try {
      const [list, statData] = await Promise.all([pluginsApi.list(), pluginsApi.getStats().catch(() => null)])
      plugins.value = Array.isArray(list) ? list : []
      const enabledCount = plugins.value.filter((p) => p.enabled).length
      const disabledCount = plugins.value.filter((p) => !p.enabled).length
      const byCategory: Record<string, number> = {}
      for (const p of plugins.value) {
        byCategory[p.category] = (byCategory[p.category] || 0) + 1
      }
      stats.value = statData && statData.total > 0 ? statData : {
        total: plugins.value.length,
        enabled: enabledCount,
        disabled: disabledCount,
        by_category: byCategory,
      }
      wasmSandboxStatus.value.activeSandboxes = enabledCount
    } catch (err: any) {
      error.value = err?.message || 'Failed to load plugins'
      plugins.value = []
    } finally {
      loading.value = false
    }
  }

  async function refreshPlugins() {
    await loadData()
  }

  function resetFilters() {
    searchQuery.value = ''
    selectedCategory.value = 'all'
    selectedStatus.value = 'all'
    selectedScope.value = 'all'
  }

  function categoryBadgeClass(cat?: string): string {
    switch (cat) {
      case 'monitoring': return 'badge-cyan'
      case 'security': return 'badge-rose'
      case 'devtools': return 'badge-indigo'
      case 'integration': return 'badge-amber'
      default: return 'badge-subtle'
    }
  }

  async function togglePlugin(p: Plugin) {
    togglingId.value = p.id
    const targetState = !p.enabled
    try {
      const updated = await pluginsApi.toggle(p.id, targetState)
      p.enabled = updated.enabled
      if (p.enabled) {
        stats.value.enabled++
        stats.value.disabled = Math.max(0, stats.value.disabled - 1)
        if (p.entry_point) pluginLoader.loadPlugin(p)
      } else {
        stats.value.disabled++
        stats.value.enabled = Math.max(0, stats.value.enabled - 1)
        pluginLoader.unloadPlugin(p.id)
      }
      wasmSandboxStatus.value.activeSandboxes = stats.value.enabled
    } catch (err: any) {
      const msg = err.response?.data?.message || err.message || 'Unknown error'
      showToast(`Failed to toggle plugin: ${msg}`, 'error')
      error.value = `Failed to toggle plugin: ${msg}`
    } finally {
      togglingId.value = null
    }
  }

  async function installPreset(preset: CreatePluginDTO) {
    installingPreset.value = true
    try {
      await pluginsApi.create(preset)
      showToast(`Plugin "${preset.name}" installed successfully!`, 'success')
      await loadData()
    } catch (err: any) {
      const msg = err.response?.data?.message || err.message || 'Unknown error'
      showToast(`Failed to install preset: ${msg}`, 'error')
      error.value = `Failed to install preset: ${msg}`
    } finally {
      installingPreset.value = false
    }
  }

  async function testBundleLoad(p: Plugin) {
    testResult.value = null
    if (!p.entry_point) {
      testResult.value = { pluginName: p.name, status: 'error', message: 'No Entry Point URL configured.' }
      return
    }
    const success = await pluginLoader.loadPlugin(p)
    if (success) {
      testResult.value = { pluginName: p.name, status: 'success', message: 'JS module fetched and registered into runtime.' }
    } else {
      const rt = pluginLoader.getRuntimeStatus(p.id)
      testResult.value = { pluginName: p.name, status: 'error', message: rt?.error || 'Failed to load JS module.' }
    }
  }

  async function hotReloadPlugin(p: Plugin) {
    pluginLoader.unloadPlugin(p.id)
    await testBundleLoad(p)
  }

  function getRuntimeStatusLabel(id: string): string {
    const rt = pluginLoader.getRuntimeStatus(id)
    if (!rt) return 'Ready'
    if (rt.status === 'active') return 'Active'
    if (rt.status === 'loading') return 'Loading...'
    if (rt.status === 'error') return 'Load Error'
    return 'Ready'
  }

  function getRuntimeStatusClass(id: string): string {
    const rt = pluginLoader.getRuntimeStatus(id)
    if (!rt) return 'status-ready'
    if (rt.status === 'active') return 'status-active'
    if (rt.status === 'loading') return 'status-loading'
    if (rt.status === 'error') return 'status-error'
    return 'status-ready'
  }

  function openRegisterModal() {
    isEditing.value = false
    editingId.value = null
    form.value = { name: '', version: '1.0.0', category: 'devtools', author: '', icon: '🧩', entry_point: '', description: '', enabled: true }
    formPermissionsRaw.value = 'k8s:read'
    formError.value = null
    showFormModal.value = true
  }

  function openEditModal(p: Plugin) {
    isEditing.value = true
    editingId.value = p.id
    form.value = {
      name: p.name,
      version: p.version || '1.0.0',
      category: p.category || 'devtools',
      author: p.author || '',
      icon: p.icon || '🧩',
      entry_point: p.entry_point || '',
      description: p.description || '',
      enabled: p.enabled,
    }
    formPermissionsRaw.value = (p.permissions || []).join(', ')
    formError.value = null
    showFormModal.value = true
  }

  function closeFormModal() {
    showFormModal.value = false
    formSubmitting.value = false
    formError.value = null
  }

  async function submitForm() {
    if (!form.value.name.trim()) {
      formError.value = 'Plugin name is required'
      return
    }
    formSubmitting.value = true
    formError.value = null
    const perms = formPermissionsRaw.value.split(',').map((s) => s.trim()).filter(Boolean)
    const payload: CreatePluginDTO = {
      name: form.value.name.trim(),
      version: form.value.version.trim() || '1.0.0',
      category: form.value.category,
      author: form.value.author.trim(),
      icon: form.value.icon.trim() || '🧩',
      entry_point: form.value.entry_point.trim(),
      description: form.value.description.trim(),
      permissions: perms,
    }
    try {
      if (isEditing.value && editingId.value) {
        await pluginsApi.update(editingId.value, payload)
      } else {
        await pluginsApi.create(payload)
      }
      closeFormModal()
      await loadData()
    } catch (err: any) {
      formError.value = err.message || 'Operation failed'
    } finally {
      formSubmitting.value = false
    }
  }

  async function confirmDelete(p: Plugin) {
    if (confirm(`Are you sure you want to delete plugin "${p.name}"?`)) {
      try {
        await pluginsApi.delete(p.id)
        pluginLoader.unloadPlugin(p.id)
        showToast(`Plugin "${p.name}" deleted successfully`, 'success')
        await loadData()
      } catch (err: any) {
        const msg = err.response?.data?.message || err.message || 'Unknown error'
        showToast(`Failed to delete plugin: ${msg}`, 'error')
        error.value = `Failed to delete plugin: ${msg}`
      }
    }
  }

  function openConfigModal(p: Plugin) {
    activeConfigPlugin.value = p
    const entries = Object.entries(p.config || {})
    configPairs.value = entries.map(([key, value]) => ({ key, value }))
    configSaveError.value = null
    showConfigModal.value = true
  }

  function closeConfigModal() {
    showConfigModal.value = false
    activeConfigPlugin.value = null
    configPairs.value = []
  }

  function addConfigPair() {
    configPairs.value.push({ key: '', value: '' })
  }

  function removeConfigPair(idx: number) {
    configPairs.value.splice(idx, 1)
  }

  async function saveConfig() {
    if (!activeConfigPlugin.value) return
    configSaving.value = true
    configSaveError.value = null
    const configMap: Record<string, string> = {}
    for (const pair of configPairs.value) {
      const k = pair.key.trim()
      if (k) configMap[k] = pair.value
    }
    try {
      const updated = await pluginsApi.update(activeConfigPlugin.value.id, {
        name: activeConfigPlugin.value.name,
        config: configMap,
      })
      activeConfigPlugin.value.config = updated.config
      const idx = plugins.value.findIndex((p) => p.id === updated.id)
      if (idx !== -1) plugins.value[idx] = updated
      closeConfigModal()
    } catch (err: any) {
      configSaveError.value = err.message || 'Failed to save configuration'
    } finally {
      configSaving.value = false
    }
  }

  onMounted(() => {
    loadData()
    pluginLoader.initGlobalHook()
  })

  return {
    plugins, stats, loading, error, toastMessage, showToast, viewMode, categoryCount, starterPresets, categories,
    availablePermissionScopes, wasmSandboxStatus, searchQuery, selectedCategory,
    selectedStatus, selectedScope, filteredPlugins, togglingId, installingPreset,
    testResult, testBundleLoad, hotReloadPlugin, getRuntimeStatusLabel, getRuntimeStatusClass,
    loadData, refreshPlugins, resetFilters, categoryBadgeClass, togglePlugin, installPreset, confirmDelete,
    showFormModal, isEditing, editingId, form, formPermissionsRaw, formSubmitting, formError,
    openRegisterModal, openEditModal, closeFormModal, submitForm,
    showConfigModal, activeConfigPlugin, configPairs, configSaving, configSaveError,
    openConfigModal, closeConfigModal, addConfigPair, removeConfigPair, saveConfig,
  }
}
