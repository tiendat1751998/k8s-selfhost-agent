<template>
  <div class="plugins-page">
    <!-- Header Section -->
    <header class="page-header glass-panel">
      <div class="header-content">
        <div class="title-group">
          <div class="icon-bubble">🧩</div>
          <div>
            <div class="badge-row">
              <span class="badge badge-cyan">RUNTIME EXTENSIBILITY</span>
              <span class="badge badge-emerald">HEADLAMP COMPLIANT</span>
            </div>
            <h1 class="page-title">Frontend Plugin Hub</h1>
            <p class="page-subtitle">
              Extend platform dashboards, workload views, and telemetry graphs dynamically using custom JavaScript bundles.
            </p>
          </div>
        </div>
        <div class="header-actions">
          <button class="btn btn-secondary" @click="refreshPlugins" :disabled="loading">
            <span :class="{ 'spin-icon': loading }">🔄</span> Refresh
          </button>
          <button class="btn btn-primary" @click="openRegisterModal">
            <span>✨</span> Register Plugin
          </button>
        </div>
      </div>

      <!-- Stats Overview Cards -->
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-icon">📦</div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.total }}</div>
            <div class="stat-label">Total Plugins</div>
          </div>
        </div>
        <div class="stat-card stat-card-active">
          <div class="stat-icon">⚡</div>
          <div class="stat-info">
            <div class="stat-value text-emerald">{{ stats.enabled }}</div>
            <div class="stat-label">Active & Enabled</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon">⏸️</div>
          <div class="stat-info">
            <div class="stat-value text-muted">{{ stats.disabled }}</div>
            <div class="stat-label">Disabled</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon">🏷️</div>
          <div class="stat-info">
            <div class="stat-value text-cyan">{{ categoryCount }}</div>
            <div class="stat-label">Categories</div>
          </div>
        </div>
      </div>
    </header>

    <!-- Starter Templates Section (Quick Install) -->
    <section class="templates-section glass-panel" v-if="plugins.length === 0 && !loading">
      <div class="section-title-row">
        <h3>🚀 Starter Plugin Catalog</h3>
        <span class="section-hint">Click any preset to install and activate instantly</span>
      </div>
      <div class="templates-grid">
        <div 
          v-for="tpl in starterPresets" 
          :key="tpl.name" 
          class="template-card"
          @click="installPreset(tpl)"
        >
          <div class="tpl-icon">{{ tpl.icon }}</div>
          <div class="tpl-info">
            <h4>{{ tpl.name }} <span class="badge" :class="categoryBadgeClass(tpl.category || 'devtools')">{{ tpl.category }}</span></h4>
            <p>{{ tpl.description }}</p>
          </div>
          <button class="btn btn-sm btn-primary" :disabled="installingPreset">
            + Install
          </button>
        </div>
      </div>
    </section>

    <!-- Filters & Search Toolbar -->
    <div class="toolbar glass-panel">
      <div class="search-box">
        <span class="search-icon">🔍</span>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search plugins by name, description, author, or permissions..."
          class="search-input"
        />
        <button v-if="searchQuery" class="clear-btn" @click="searchQuery = ''">✕</button>
      </div>

      <div class="filter-controls">
        <div class="category-tabs">
          <button
            v-for="cat in categories"
            :key="cat.value"
            class="tab-btn"
            :class="{ active: selectedCategory === cat.value }"
            @click="selectedCategory = cat.value"
          >
            <span>{{ cat.icon }}</span> {{ cat.label }}
          </button>
        </div>

        <div class="status-filters">
          <select v-model="selectedStatus" class="select-input">
            <option value="all">All Status ({{ plugins.length }})</option>
            <option value="enabled">Enabled Only ({{ stats.enabled }})</option>
            <option value="disabled">Disabled Only ({{ stats.disabled }})</option>
          </select>
        </div>
      </div>
    </div>

    <!-- Loading & Error States -->
    <div v-if="loading && plugins.length === 0" class="loading-state glass-panel">
      <div class="spinner"></div>
      <p>Loading plugin registry...</p>
    </div>

    <div v-else-if="error" class="error-banner glass-panel">
      <span class="error-icon">⚠️</span>
      <div class="error-msg">
        <strong>Error:</strong> {{ error }}
      </div>
      <button class="btn btn-sm btn-secondary" @click="refreshPlugins">Retry</button>
    </div>

    <!-- Empty State -->
    <div v-else-if="filteredPlugins.length === 0" class="empty-state glass-panel">
      <div class="empty-icon">🧩</div>
      <h3>No plugins found</h3>
      <p v-if="searchQuery || selectedCategory !== 'all' || selectedStatus !== 'all'">
        Try adjusting your filters or search terms.
      </p>
      <p v-else>
        No plugins registered yet. Create your first runtime plugin extension!
      </p>
      <div class="empty-actions">
        <button class="btn btn-primary" @click="openRegisterModal">
          Register New Plugin
        </button>
        <button 
          v-if="searchQuery || selectedCategory !== 'all' || selectedStatus !== 'all'"
          class="btn btn-secondary" 
          @click="resetFilters"
        >
          Reset Filters
        </button>
      </div>
    </div>

    <!-- Plugins Grid -->
    <div v-else class="plugins-grid">
      <div
        v-for="p in filteredPlugins"
        :key="p.id"
        class="plugin-card glass-panel"
        :class="{ 'plugin-disabled': !p.enabled }"
      >
        <!-- Card Header -->
        <div class="card-header">
          <div class="plugin-identity">
            <div class="plugin-avatar">{{ p.icon || '🧩' }}</div>
            <div>
              <div class="plugin-name-row">
                <h3 class="plugin-name">{{ p.name }}</h3>
                <span class="version-tag">v{{ p.version }}</span>
              </div>
              <div class="plugin-meta-row">
                <span class="badge" :class="categoryBadgeClass(p.category)">
                  {{ p.category }}
                </span>
                <span v-if="p.author" class="author-tag">by {{ p.author }}</span>
              </div>
            </div>
          </div>

          <!-- Switch Toggle -->
          <div class="toggle-wrapper" :title="p.enabled ? 'Click to Disable' : 'Click to Enable'">
            <label class="switch">
              <input
                type="checkbox"
                :checked="p.enabled"
                :disabled="togglingId === p.id"
                @change="togglePlugin(p)"
              />
              <span class="slider round"></span>
            </label>
          </div>
        </div>

        <!-- Description -->
        <p class="plugin-desc">
          {{ p.description || 'No description provided.' }}
        </p>

        <!-- Entry Point URL -->
        <div class="entrypoint-box" v-if="p.entry_point">
          <span class="entry-icon">🔗</span>
          <span class="entry-url" :title="p.entry_point">{{ p.entry_point }}</span>
        </div>

        <!-- Permissions Chips -->
        <div class="permissions-container" v-if="p.permissions && p.permissions.length > 0">
          <span class="perm-label">Permissions:</span>
          <div class="perm-chips">
            <span
              v-for="perm in p.permissions"
              :key="perm"
              class="perm-chip"
            >
              {{ perm }}
            </span>
          </div>
        </div>

        <!-- Card Footer -->
        <div class="card-footer">
          <div class="footer-left">
            <span class="config-count" @click="openConfigModal(p)">
              ⚙️ {{ Object.keys(p.config || {}).length }} config keys
            </span>
            <span 
              class="runtime-status-pill"
              :class="getRuntimeStatusClass(p.id)"
            >
              {{ getRuntimeStatusLabel(p.id) }}
            </span>
          </div>

          <div class="footer-actions">
            <button 
              class="btn-icon" 
              title="Test bundle load" 
              @click="testBundleLoad(p)"
            >
              ⚡
            </button>
            <button 
              class="btn-icon" 
              title="Configure Settings" 
              @click="openConfigModal(p)"
            >
              ⚙️
            </button>
            <button 
              class="btn-icon" 
              title="Edit Plugin" 
              @click="openEditModal(p)"
            >
              ✏️
            </button>
            <button 
              class="btn-icon btn-danger-icon" 
              title="Delete Plugin" 
              @click="confirmDelete(p)"
            >
              🗑️
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Register / Edit Modal -->
    <div v-if="showFormModal" class="modal-backdrop" @click.self="closeFormModal">
      <div class="modal-card glass-panel">
        <div class="modal-header">
          <div class="modal-title-row">
            <span class="modal-icon">{{ isEditing ? '✏️' : '✨' }}</span>
            <h3>{{ isEditing ? 'Edit Plugin Extension' : 'Register New Plugin' }}</h3>
          </div>
          <button class="modal-close" @click="closeFormModal">✕</button>
        </div>

        <form @submit.prevent="submitForm" class="modal-form">
          <div class="form-grid">
            <div class="form-group">
              <label>Plugin Name <span class="required">*</span></label>
              <input
                v-model="form.name"
                type="text"
                required
                placeholder="e.g. k8s-cost-analyzer"
                class="text-input"
              />
            </div>

            <div class="form-group">
              <label>Version</label>
              <input
                v-model="form.version"
                type="text"
                placeholder="1.0.0"
                class="text-input"
              />
            </div>

            <div class="form-group">
              <label>Category</label>
              <select v-model="form.category" class="select-input">
                <option value="monitoring">📊 Monitoring</option>
                <option value="security">🛡️ Security</option>
                <option value="devtools">🛠️ Developer Tools</option>
                <option value="integration">🔌 Integration</option>
              </select>
            </div>

            <div class="form-group">
              <label>Icon (Emoji or Icon Name)</label>
              <input
                v-model="form.icon"
                type="text"
                placeholder="e.g. 📊, ⚡, 🛡️"
                class="text-input"
              />
            </div>

            <div class="form-group full-width">
              <label>Author / Organization</label>
              <input
                v-model="form.author"
                type="text"
                placeholder="e.g. DevOps Core Team"
                class="text-input"
              />
            </div>

            <div class="form-group full-width">
              <label>JS Bundle Entry Point URL</label>
              <input
                v-model="form.entry_point"
                type="text"
                placeholder="https://cdn.example.com/plugins/bundle.js"
                class="text-input font-mono"
              />
              <span class="input-hint">Remote or relative URL to the JavaScript module bundle</span>
            </div>

            <div class="form-group full-width">
              <label>Description</label>
              <textarea
                v-model="form.description"
                rows="3"
                placeholder="Describe what this plugin does..."
                class="textarea-input"
              ></textarea>
            </div>

            <div class="form-group full-width">
              <label>Permissions (comma separated)</label>
              <input
                v-model="formPermissionsRaw"
                type="text"
                placeholder="k8s:read, metrics:read, exec:pods"
                class="text-input"
              />
              <span class="input-hint">e.g. k8s:read, metrics:read, logs:read, notifications:write</span>
            </div>
          </div>

          <div v-if="formError" class="form-error-msg">
            ⚠️ {{ formError }}
          </div>

          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeFormModal">
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" :disabled="formSubmitting">
              {{ formSubmitting ? 'Saving...' : (isEditing ? 'Save Changes' : 'Register Plugin') }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Config Editor Modal -->
    <div v-if="showConfigModal && activeConfigPlugin" class="modal-backdrop" @click.self="closeConfigModal">
      <div class="modal-card glass-panel config-modal">
        <div class="modal-header">
          <div class="modal-title-row">
            <span class="modal-icon">⚙️</span>
            <div>
              <h3>Plugin Configuration</h3>
              <p class="modal-subtitle">{{ activeConfigPlugin.name }} (v{{ activeConfigPlugin.version }})</p>
            </div>
          </div>
          <button class="modal-close" @click="closeConfigModal">✕</button>
        </div>

        <div class="config-modal-body">
          <div class="config-intro">
            Key-value configuration passed into runtime plugin hooks at boot.
          </div>

          <div class="config-table-container">
            <table class="config-table" v-if="configPairs.length > 0">
              <thead>
                <tr>
                  <th>Config Key</th>
                  <th>Value</th>
                  <th class="col-action"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(pair, idx) in configPairs" :key="idx">
                  <td>
                    <input
                      v-model="pair.key"
                      type="text"
                      placeholder="e.g. api_url"
                      class="text-input font-mono"
                    />
                  </td>
                  <td>
                    <input
                      v-model="pair.value"
                      type="text"
                      placeholder="e.g. https://api.endpoint.internal"
                      class="text-input"
                    />
                  </td>
                  <td>
                    <button class="btn-icon btn-danger-icon" @click="removeConfigPair(idx)">✕</button>
                  </td>
                </tr>
              </tbody>
            </table>
            <div v-else class="config-empty">
              No configuration variables set. Click below to add key-value pairs.
            </div>
          </div>

          <button class="btn btn-sm btn-secondary add-pair-btn" @click="addConfigPair">
            + Add Config Variable
          </button>

          <div v-if="configSaveError" class="form-error-msg">
            ⚠️ {{ configSaveError }}
          </div>
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" @click="closeConfigModal">
            Cancel
          </button>
          <button type="button" class="btn btn-primary" :disabled="configSaving" @click="saveConfig">
            {{ configSaving ? 'Saving...' : 'Save Configuration' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Test Bundle Feedback Toast / Modal -->
    <div v-if="testResult" class="test-feedback-toast glass-panel" :class="testResult.status">
      <div class="toast-header">
        <span>{{ testResult.status === 'success' ? '✅ Bundle Loaded' : '⚠️ Bundle Test Failed' }}</span>
        <button class="toast-close" @click="testResult = null">✕</button>
      </div>
      <div class="toast-body">
        <strong>{{ testResult.pluginName }}:</strong> {{ testResult.message }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { pluginsApi, type Plugin, type PluginStats, type CreatePluginDTO } from '../api/plugins'
import { pluginLoader } from '../services/pluginLoader'

// State
const plugins = ref<Plugin[]>([])
const stats = ref<PluginStats>({
  total: 0,
  enabled: 0,
  disabled: 0,
  by_category: {},
})
const loading = ref(false)
const error = ref<string | null>(null)
const searchQuery = ref('')
const selectedCategory = ref('all')
const selectedStatus = ref('all')
const togglingId = ref<string | null>(null)
const installingPreset = ref(false)

// Starter Catalog Presets & Active Extension Plugins
const starterPresets: CreatePluginDTO[] = [
  {
    name: 'Trivy Security Scanner',
    version: '1.4.2',
    category: 'security',
    icon: '🛡️',
    author: 'Aqua Security & Platform SRE',
    description: 'Injects live CVE vulnerability analysis, SBOM component graphs, and image scan metrics directly into pod detail drawers.',
    entry_point: 'https://cdn.jsdelivr.net/npm/@k8s-plugins/trivy-scanner/dist/index.js',
    permissions: ['audit:read', 'k8s:read', 'cve:read'],
    config: { scanner_endpoint: 'http://trivy.security:4954', auto_scan_on_mount: 'true', min_severity: 'HIGH' },
  },
  {
    name: 'Grafana Dashboard Embed',
    version: '2.1.0',
    category: 'monitoring',
    icon: '📈',
    author: 'Grafana Labs Community',
    description: 'Seamlessly embeds contextual Grafana latency, throughput, and CPU/memory panels into deployment and service views.',
    entry_point: 'https://cdn.jsdelivr.net/npm/@k8s-plugins/grafana-embed/dist/index.js',
    permissions: ['metrics:read', 'dashboards:view'],
    config: { grafana_url: 'http://grafana.monitoring:3000', theme: 'dark', refresh_interval: '15s' },
  },
  {
    name: 'Prometheus AlertBridge',
    version: '1.2.0',
    category: 'monitoring',
    icon: '🔥',
    author: 'Prometheus Authors',
    description: 'Connects Alertmanager active firings, alert routing rules, and Prometheus SLI breach warnings to real-time notification toasts.',
    entry_point: 'https://cdn.jsdelivr.net/npm/@k8s-plugins/alert-bridge/dist/index.js',
    permissions: ['alerts:read', 'notifications:send'],
    config: { alertmanager_url: 'http://alertmanager:9093', poll_interval_sec: '10', dedup_window: '60s' },
  },
  {
    name: 'Vector Log Processor',
    version: '1.1.5',
    category: 'devtools',
    icon: '📜',
    author: 'Timber.io & Vector OSS',
    description: 'High-throughput client-side log parsing, regex highlighting, and structured JSON stream extraction for container logs.',
    entry_point: 'https://cdn.jsdelivr.net/npm/@k8s-plugins/vector-logs/dist/index.js',
    permissions: ['logs:read', 'k8s:read'],
    config: { buffer_lines: '5000', ansi_highlighting: 'true', filter_heartbeats: 'true' },
  },
]

// Category Filter Options
const categories = [
  { value: 'all', label: 'All Categories', icon: '🌐' },
  { value: 'monitoring', label: 'Monitoring', icon: '📊' },
  { value: 'security', label: 'Security', icon: '🛡️' },
  { value: 'devtools', label: 'DevTools', icon: '🛠️' },
  { value: 'integration', label: 'Integration', icon: '🔌' },
]

// Modal & Form State
const showFormModal = ref(false)
const isEditing = ref(false)
const editingId = ref<string | null>(null)
const formSubmitting = ref(false)
const formError = ref<string | null>(null)

const form = ref<{
  name: string
  version: string
  category: string
  author: string
  icon: string
  entry_point: string
  description: string
  enabled: boolean
}>({
  name: '',
  version: '1.0.0',
  category: 'devtools',
  author: '',
  icon: '🧩',
  entry_point: '',
  description: '',
  enabled: true,
})
const formPermissionsRaw = ref('')

// Config Modal State
const showConfigModal = ref(false)
const activeConfigPlugin = ref<Plugin | null>(null)
const configPairs = ref<{ key: string; value: string }[]>([])
const configSaving = ref(false)
const configSaveError = ref<string | null>(null)

// Test Result State
const testResult = ref<{
  pluginName: string
  status: 'success' | 'error'
  message: string
} | null>(null)

// Computed
const categoryCount = computed(() => {
  return Object.keys(stats.value.by_category || {}).length || 3
})

const filteredPlugins = computed(() => {
  return plugins.value.filter((p) => {
    // Category filter
    if (selectedCategory.value !== 'all' && p.category !== selectedCategory.value) {
      return false
    }
    // Status filter
    if (selectedStatus.value === 'enabled' && !p.enabled) return false
    if (selectedStatus.value === 'disabled' && p.enabled) return false

    // Search query
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      const matchName = p.name.toLowerCase().includes(q)
      const matchDesc = (p.description || '').toLowerCase().includes(q)
      const matchAuthor = (p.author || '').toLowerCase().includes(q)
      const matchPerms = (p.permissions || []).some((perm) => perm.toLowerCase().includes(q))
      if (!matchName && !matchDesc && !matchAuthor && !matchPerms) {
        return false
      }
    }

    return true
  })
})

// Methods
async function loadData() {
  loading.value = true
  error.value = null
  try {
    const [list, statData] = await Promise.all([
      pluginsApi.list(),
      pluginsApi.getStats().catch(() => null),
    ])
    plugins.value = Array.isArray(list) ? list : []

    const enabledCount = plugins.value.filter(p => p.enabled).length
    const disabledCount = plugins.value.filter(p => !p.enabled).length
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
}

function categoryBadgeClass(cat?: string): string {
  switch (cat) {
    case 'monitoring':
      return 'badge-cyan'
    case 'security':
      return 'badge-rose'
    case 'devtools':
      return 'badge-indigo'
    case 'integration':
      return 'badge-amber'
    default:
      return 'badge-subtle'
  }
}

async function togglePlugin(p: Plugin) {
  togglingId.value = p.id
  const targetState = !p.enabled
  try {
    const updated = await pluginsApi.toggle(p.id, targetState)
    p.enabled = updated.enabled
    
    // Sync stats
    if (p.enabled) {
      stats.value.enabled++
      stats.value.disabled = Math.max(0, stats.value.disabled - 1)
      if (p.entry_point) {
        pluginLoader.loadPlugin(p)
      }
    } else {
      stats.value.disabled++
      stats.value.enabled = Math.max(0, stats.value.enabled - 1)
      pluginLoader.unloadPlugin(p.id)
    }
  } catch (err: any) {
    alert(`Failed to toggle plugin: ${err.message}`)
  } finally {
    togglingId.value = null
  }
}

async function installPreset(preset: CreatePluginDTO) {
  installingPreset.value = true
  try {
    await pluginsApi.create(preset)
    await loadData()
  } catch (err: any) {
    alert(`Failed to install preset: ${err.message}`)
  } finally {
    installingPreset.value = false
  }
}

// Form Handlers
function openRegisterModal() {
  isEditing.value = false
  editingId.value = null
  form.value = {
    name: '',
    version: '1.0.0',
    category: 'devtools',
    author: '',
    icon: '🧩',
    entry_point: '',
    description: '',
    enabled: true,
  }
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

  const perms = formPermissionsRaw.value
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)

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

// Delete Handler
async function confirmDelete(p: Plugin) {
  if (confirm(`Are you sure you want to delete plugin "${p.name}"?`)) {
    try {
      await pluginsApi.delete(p.id)
      pluginLoader.unloadPlugin(p.id)
      await loadData()
    } catch (err: any) {
      alert(`Failed to delete plugin: ${err.message}`)
    }
  }
}

// Config Modal Handlers
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
    if (k) {
      configMap[k] = pair.value
    }
  }

  try {
    const updated = await pluginsApi.update(activeConfigPlugin.value.id, {
      name: activeConfigPlugin.value.name,
      config: configMap,
    })
    activeConfigPlugin.value.config = updated.config
    const idx = plugins.value.findIndex((p) => p.id === updated.id)
    if (idx !== -1) {
      plugins.value[idx] = updated
    }
    closeConfigModal()
  } catch (err: any) {
    configSaveError.value = err.message || 'Failed to save configuration'
  } finally {
    configSaving.value = false
  }
}

// Runtime Testing
async function testBundleLoad(p: Plugin) {
  testResult.value = null
  if (!p.entry_point) {
    testResult.value = {
      pluginName: p.name,
      status: 'error',
      message: 'No Entry Point URL configured for this plugin.',
    }
    return
  }

  const success = await pluginLoader.loadPlugin(p)
  if (success) {
    testResult.value = {
      pluginName: p.name,
      status: 'success',
      message: 'JS module fetched and registered into platform runtime context.',
    }
  } else {
    const rt = pluginLoader.getRuntimeStatus(p.id)
    testResult.value = {
      pluginName: p.name,
      status: 'error',
      message: rt?.error || 'Failed to load JS module.',
    }
  }
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

onMounted(() => {
  loadData()
  pluginLoader.initGlobalHook()
})
</script>

<style scoped>
.plugins-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 24px;
  max-width: 1600px;
  margin: 0 auto;
  width: 100%;
}

/* Page Header */
.page-header {
  padding: 24px 28px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  border-radius: 16px;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
  flex-wrap: wrap;
}

.title-group {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.icon-bubble {
  font-size: 32px;
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(6, 182, 212, 0.12);
  border: 1px solid rgba(6, 182, 212, 0.25);
  border-radius: 14px;
}

.badge-row {
  display: flex;
  gap: 8px;
  margin-bottom: 6px;
}

.page-title {
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--text-primary);
  margin-bottom: 4px;
}

.page-subtitle {
  color: var(--text-secondary);
  font-size: 13.5px;
  max-width: 720px;
  line-height: 1.5;
}

.header-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
}

.stat-icon {
  font-size: 24px;
}

.stat-value {
  font-size: 22px;
  font-weight: 700;
  font-family: var(--font-mono);
  line-height: 1.2;
}

.stat-label {
  font-size: 11.5px;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 600;
}

/* Templates Section */
.templates-section {
  padding: 20px 24px;
}

.section-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-title-row h3 {
  font-size: 16px;
  font-weight: 600;
}

.section-hint {
  font-size: 12px;
  color: var(--text-muted);
}

.templates-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 14px;
}

.template-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.template-card:hover {
  background: rgba(6, 182, 212, 0.05);
  border-color: var(--accent-cyan);
  transform: translateY(-1px);
}

.tpl-icon {
  font-size: 28px;
}

.tpl-info {
  flex: 1;
  min-width: 0;
}

.tpl-info h4 {
  font-size: 13.5px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 2px;
}

.tpl-info p {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Toolbar */
.toolbar {
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.search-box {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 14px;
  font-size: 15px;
  color: var(--text-muted);
}

.search-input {
  width: 100%;
  padding: 10px 36px 10px 40px;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border-medium);
  border-radius: 10px;
  color: var(--text-primary);
  font-size: 13.5px;
  font-family: var(--font-sans);
  outline: none;
  transition: border-color 0.2s;
}

.search-input:focus {
  border-color: var(--accent-cyan);
}

.clear-btn {
  position: absolute;
  right: 12px;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 14px;
}

.filter-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.category-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
  background: rgba(255, 255, 255, 0.02);
  color: var(--text-secondary);
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tab-btn:hover {
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-primary);
}

.tab-btn.active {
  background: rgba(6, 182, 212, 0.15);
  border-color: var(--accent-cyan);
  color: var(--accent-cyan);
  font-weight: 600;
}

/* Plugins Grid */
.plugins-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
  gap: 18px;
}

.plugin-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  border-radius: 14px;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
}

.plugin-card:hover {
  border-color: var(--border-accent);
  box-shadow: 0 12px 32px -8px rgba(0, 0, 0, 0.6);
  transform: translateY(-2px);
}

.plugin-card.plugin-disabled {
  opacity: 0.7;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.plugin-identity {
  display: flex;
  gap: 12px;
  align-items: center;
}

.plugin-avatar {
  font-size: 26px;
  width: 46px;
  height: 46px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-medium);
  border-radius: 12px;
}

.plugin-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.plugin-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.version-tag {
  font-size: 11px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.05);
  padding: 1px 6px;
  border-radius: 4px;
}

.plugin-meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 3px;
}

.author-tag {
  font-size: 11.5px;
  color: var(--text-muted);
}

.plugin-desc {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.45;
  flex: 1;
}

.entrypoint-box {
  display: flex;
  align-items: center;
  gap: 6px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 11.5px;
  font-family: var(--font-mono);
  color: var(--text-muted);
}

.entry-url {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.permissions-container {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.perm-label {
  font-size: 11px;
  text-transform: uppercase;
  color: var(--text-muted);
  font-weight: 600;
  letter-spacing: 0.04em;
}

.perm-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.perm-chip {
  font-size: 10.5px;
  font-family: var(--font-mono);
  background: rgba(99, 102, 241, 0.1);
  border: 1px solid rgba(99, 102, 241, 0.25);
  color: #a5b4fc;
  padding: 2px 7px;
  border-radius: 4px;
}

/* Card Footer */
.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 12px;
  border-top: 1px solid var(--border-subtle);
}

.footer-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.config-count {
  font-size: 11.5px;
  color: var(--text-muted);
  cursor: pointer;
  transition: color 0.15s;
}

.config-count:hover {
  color: var(--accent-cyan);
}

.runtime-status-pill {
  font-size: 10.5px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
  text-transform: uppercase;
}

.status-ready {
  background: rgba(148, 163, 184, 0.1);
  color: var(--text-muted);
}

.status-active {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
}

.status-loading {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
}

.status-error {
  background: rgba(244, 63, 94, 0.15);
  color: #fb7185;
}

.footer-actions {
  display: flex;
  gap: 6px;
}

.btn-icon {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 13px;
  transition: all 0.15s ease;
}

.btn-icon:hover {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text-primary);
  border-color: var(--border-medium);
}

.btn-danger-icon:hover {
  background: rgba(244, 63, 94, 0.15);
  border-color: rgba(244, 63, 94, 0.4);
  color: #f43f5e;
}

/* Switch Toggle UI */
.switch {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 22px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(255, 255, 255, 0.15);
  transition: 0.25s;
  border-radius: 22px;
}

.slider:before {
  position: absolute;
  content: '';
  height: 16px;
  width: 16px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: 0.25s;
  border-radius: 50%;
}

input:checked + .slider {
  background-color: var(--accent-emerald);
}

input:checked + .slider:before {
  transform: translateX(18px);
}

/* Buttons */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.15s ease;
}

.btn-primary {
  background: var(--grad-cyan);
  color: #020617;
}

.btn-primary:hover {
  filter: brightness(1.1);
  box-shadow: 0 4px 14px -3px rgba(6, 182, 212, 0.5);
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.05);
  border-color: var(--border-medium);
  color: var(--text-primary);
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.1);
}

.btn-sm {
  padding: 4px 10px;
  font-size: 11.5px;
}

/* Badges */
.badge {
  display: inline-block;
  padding: 2px 7px;
  border-radius: 4px;
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.badge-cyan {
  background: rgba(6, 182, 212, 0.12);
  color: #22d3ee;
  border: 1px solid rgba(6, 182, 212, 0.3);
}

.badge-emerald {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.badge-rose {
  background: rgba(244, 63, 94, 0.12);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.3);
}

.badge-indigo {
  background: rgba(99, 102, 241, 0.12);
  color: #818cf8;
  border: 1px solid rgba(99, 102, 241, 0.3);
}

.badge-amber {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.badge-subtle {
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-secondary);
}

/* Modals */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.modal-card {
  width: 100%;
  max-width: 640px;
  max-height: 90vh;
  overflow-y: auto;
  padding: 24px;
  border-radius: 16px;
  background: #0d131f;
  border: 1px solid var(--border-medium);
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.8);
}

.config-modal {
  max-width: 740px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--border-subtle);
}

.modal-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.modal-icon {
  font-size: 22px;
}

.modal-header h3 {
  font-size: 18px;
  font-weight: 700;
}

.modal-subtitle {
  font-size: 12px;
  color: var(--text-muted);
}

.modal-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 18px;
  cursor: pointer;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.full-width {
  grid-column: 1 / -1;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.required {
  color: #f43f5e;
}

.text-input,
.select-input,
.textarea-input {
  width: 100%;
  padding: 9px 12px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 13px;
  font-family: var(--font-sans);
  outline: none;
}

.text-input:focus,
.select-input:focus,
.textarea-input:focus {
  border-color: var(--accent-cyan);
}

.font-mono {
  font-family: var(--font-mono);
}

.input-hint {
  font-size: 11px;
  color: var(--text-muted);
}

.form-error-msg {
  padding: 10px;
  background: rgba(244, 63, 94, 0.1);
  border: 1px solid rgba(244, 63, 94, 0.3);
  border-radius: 8px;
  color: #fb7185;
  font-size: 12.5px;
  margin-top: 14px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
}

/* Config Table */
.config-modal-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.config-intro {
  font-size: 13px;
  color: var(--text-secondary);
}

.config-table-container {
  max-height: 320px;
  overflow-y: auto;
}

.config-table {
  width: 100%;
  border-collapse: collapse;
}

.config-table th {
  text-align: left;
  font-size: 11.5px;
  text-transform: uppercase;
  color: var(--text-muted);
  padding: 8px;
  border-bottom: 1px solid var(--border-subtle);
}

.config-table td {
  padding: 6px 8px;
}

.col-action {
  width: 40px;
}

.config-empty {
  text-align: center;
  padding: 24px;
  color: var(--text-muted);
  font-size: 13px;
}

.add-pair-btn {
  align-self: flex-start;
}

/* Feedback Toast */
.test-feedback-toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  width: 360px;
  padding: 14px 18px;
  border-radius: 12px;
  z-index: 1100;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.7);
  animation: slideUp 0.25s ease-out;
}

@keyframes slideUp {
  from {
    transform: translateY(20px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

.test-feedback-toast.success {
  border: 1px solid rgba(16, 185, 129, 0.4);
  background: rgba(6, 32, 22, 0.95);
}

.test-feedback-toast.error {
  border: 1px solid rgba(244, 63, 94, 0.4);
  background: rgba(36, 8, 14, 0.95);
}

.toast-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 4px;
}

.toast-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
}

.toast-body {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}

/* Helpers */
.text-emerald {
  color: #34d399;
}
.text-cyan {
  color: #22d3ee;
}
.text-muted {
  color: var(--text-muted);
}

.spin-icon {
  display: inline-block;
  animation: spin 1s infinite linear;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

.loading-state,
.empty-state,
.error-banner {
  padding: 48px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.empty-icon {
  font-size: 48px;
}

.empty-actions {
  display: flex;
  gap: 12px;
  margin-top: 12px;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid rgba(6, 182, 212, 0.2);
  border-top-color: var(--accent-cyan);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@media (max-width: 768px) {
  .plugins-page {
    padding: 16px;
  }
  .page-header {
    padding: 18px 16px;
  }
  .header-content {
    flex-direction: column;
    align-items: stretch;
    gap: 16px;
  }
  .header-actions {
    display: flex !important;
    flex-direction: row !important;
    width: 100% !important;
    gap: 8px !important;
    flex-wrap: wrap !important;
  }
  .header-actions .btn,
  .header-actions button,
  .header-actions a {
    flex: 1 1 calc(50% - 4px) !important;
    min-width: 0 !important;
    padding: 7px 10px !important;
    font-size: 11.5px !important;
    white-space: nowrap !important;
    justify-content: center !important;
  }
  .header-actions > :last-child:nth-child(odd) {
    flex: 1 1 100% !important;
  }
  .metrics-grid,
  .grid-metrics,
  .stats-grid,
  .plugin-stats,
  .ai-stats {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 8px !important;
  }
  :deep(.metric-card),
  .metric-card,
  :deep(.stat-card),
  .stat-card,
  .hud-card,
  :deep(.hud-card) {
    padding: 10px 12px !important;
  }
  .toolbar {
    padding: 14px 16px;
  }
  .filter-controls {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }
  .category-tabs {
    overflow-x: auto;
    flex-wrap: nowrap;
    -webkit-overflow-scrolling: touch;
    width: 100%;
    padding-bottom: 4px;
  }
  .status-filters,
  .status-filters .select-input {
    width: 100%;
  }
  .plugins-grid,
  .templates-grid {
    grid-template-columns: 1fr;
  }
  .form-grid {
    grid-template-columns: 1fr;
  }
  .modal-card,
  .config-modal {
    width: 100%;
    max-width: 100%;
    padding: 18px 16px;
  }
  .test-feedback-toast {
    left: 16px;
    right: 16px;
    width: auto;
  }
}

@media (max-width: 640px) {
  .header-actions {
    display: flex !important;
    flex-direction: row !important;
    width: 100% !important;
    gap: 8px !important;
    flex-wrap: wrap !important;
  }

  .header-actions .btn,
  .header-actions button,
  .header-actions a {
    flex: 1 1 calc(50% - 4px) !important;
    min-width: 0 !important;
    padding: 7px 10px !important;
    font-size: 11.5px !important;
    white-space: nowrap !important;
    justify-content: center !important;
  }

  .header-actions > :last-child:nth-child(odd) {
    flex: 1 1 100% !important;
  }

  .metrics-grid,
  .grid-metrics,
  .stats-grid,
  .plugin-stats,
  .ai-stats {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 8px !important;
  }

  :deep(.metric-card),
  .metric-card,
  :deep(.stat-card),
  .stat-card,
  .hud-card,
  :deep(.hud-card) {
    padding: 10px 12px !important;
  }

  :deep(.stat-card .stat-value),
  .stat-card .stat-value {
    font-size: 18px;
  }

  :deep(.stat-card .stat-label),
  .stat-card .stat-label {
    font-size: 10px;
  }

  .templates-section {
    padding: 16px;
  }
  .template-card {
    flex-direction: column;
    align-items: flex-start;
  }
  .template-card .btn {
    width: 100%;
    justify-content: center;
  }
  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  .toggle-wrapper {
    align-self: flex-start;
  }
  .card-footer {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }
  .footer-actions {
    justify-content: flex-end;
  }
  .modal-footer {
    flex-direction: column;
    width: 100%;
  }
  .modal-footer .btn {
    width: 100%;
    justify-content: center;
  }
}
</style>
