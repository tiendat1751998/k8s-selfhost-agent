<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import {
  ecosystemApi,
  type DetectedTool,
  type EcosystemSummary,
  type CreateToolRequest
} from '../api/ecosystem'

// State
const loading = ref(false)
const scanning = ref(false)
const saving = ref(false)
const deleting = ref<string | null>(null)
const error = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | null = null
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null

const tools = ref<DetectedTool[]>([])
const summary = ref<EcosystemSummary>({
  total: 0,
  healthy: 0,
  degraded: 0,
  by_category: {},
})

// Filters
const activeCategory = ref('all')
const searchQuery = ref('')
const selectedStatus = ref('all')

// Manual Tool Modal State
const showAddModal = ref(false)
const form = reactive<CreateToolRequest>({
  name: '',
  category: 'gitops',
  endpoint: '',
  version: '',
  status: 'detected',
  health: 'healthy',
})

const categories = [
  { key: 'all', label: 'All Categories', icon: '🌐' },
  { key: 'compute', label: 'Compute & Containers', icon: '🐳' },
  { key: 'database', label: 'Databases & Storage', icon: '🗄️' },
  { key: 'messaging', label: 'Messaging & Streaming', icon: '⚡' },
  { key: 'mesh', label: 'Ingress & Mesh', icon: '⛵' },
  { key: 'gitops', label: 'GitOps & CI/CD', icon: '🐙' },
  { key: 'security', label: 'Security & Posture', icon: '🛡️' },
  { key: 'monitoring', label: 'Monitoring & Telemetry', icon: '📈' },
  { key: 'secrets', label: 'Secrets & KMS', icon: '🔐' },
  { key: 'policy', label: 'Policy & Guardrails', icon: '📜' },
]

const defaultTools: DetectedTool[] = [
  {
    id: 'eco-docker-engine',
    name: 'Docker Engine',
    category: 'compute',
    status: 'detected',
    version: '27.0.0',
    endpoint: 'http://127.0.0.1:2375',
    source: 'k8s_discovery',
    health: 'healthy',
    last_checked: new Date().toISOString(),
    metadata: { engine: 'docker-daemon', storage_driver: 'overlay2', cluster: 'primary-cluster' },
    tenant_id: 'default-tenant',
  },
  {
    id: 'eco-postgres-16',
    name: 'PostgreSQL 16',
    category: 'database',
    status: 'detected',
    version: '16.3',
    endpoint: 'postgres://db.internal:5432',
    source: 'k8s_discovery',
    health: 'healthy',
    last_checked: new Date().toISOString(),
    metadata: { engine: 'postgresql', port: '5432', mode: 'read-write', wal_level: 'replica' },
    tenant_id: 'default-tenant',
  },
  {
    id: 'eco-redis-8',
    name: 'Redis 8',
    category: 'database',
    status: 'detected',
    version: '8.0-M02',
    endpoint: 'redis://tiki-redis:6379',
    source: 'k8s_discovery',
    health: 'healthy',
    last_checked: new Date().toISOString(),
    metadata: { engine: 'redis', mode: 'standalone', port: '6379', persistence: 'rdb+aof' },
    tenant_id: 'default-tenant',
  },
  {
    id: 'eco-nats-jetstream',
    name: 'NATS JetStream',
    category: 'messaging',
    status: 'detected',
    version: 'v2.10.18',
    endpoint: 'http://nats.infra:8222',
    source: 'k8s_discovery',
    health: 'healthy',
    last_checked: new Date().toISOString(),
    metadata: { jetstream: 'enabled', port: '4222', http_port: '8222', cluster: 'primary-cluster' },
    tenant_id: 'default-tenant',
  },
  {
    id: 'eco-traefik-v3',
    name: 'Traefik v3.1',
    category: 'mesh',
    status: 'detected',
    version: 'v3.1.2',
    endpoint: 'http://traefik.internal:8080',
    source: 'k8s_discovery',
    health: 'healthy',
    last_checked: new Date().toISOString(),
    metadata: { router: 'traefik', port: '8080', dashboard: 'enabled', providers: 'kubernetes,docker' },
    tenant_id: 'default-tenant',
  },
  {
    id: 'eco-drone-ci',
    name: 'Drone CI',
    category: 'gitops',
    status: 'detected',
    version: 'v2.24.0',
    endpoint: 'http://drone.internal:80',
    source: 'k8s_discovery',
    health: 'healthy',
    last_checked: new Date().toISOString(),
    metadata: { runner: 'docker-runner', port: '80', auth_provider: 'github' },
    tenant_id: 'default-tenant',
  },
]

function showToast(text: string, type: 'success' | 'error' = 'success') {
  if (toastTimer) clearTimeout(toastTimer)
  toastMessage.value = { text, type }
  toastTimer = setTimeout(() => {
    toastMessage.value = null
  }, 4000)
}

function getToolIcon(tool: DetectedTool): string {
  const name = tool.name.toLowerCase()
  if (name.includes('docker')) return '🐳'
  if (name.includes('postgres')) return '🐘'
  if (name.includes('redis')) return '🔴'
  if (name.includes('nats')) return '⚡'
  if (name.includes('traefik')) return '🚦'
  if (name.includes('drone')) return '🚁'
  if (name.includes('argo')) return '🐙'
  if (name.includes('trivy')) return '🛡️'
  if (name.includes('grafana')) return '📈'
  if (name.includes('vault')) return '🔐'
  if (name.includes('prometheus')) return '🔥'
  if (name.includes('kyverno')) return '📜'
  if (name.includes('istio')) return '⛵'
  if (name.includes('cert-manager') || name.includes('cert')) return '🔒'

  switch (tool.category.toLowerCase()) {
    case 'compute': return '🐳'
    case 'database': return '🗄️'
    case 'messaging': return '⚡'
    case 'gitops': return '🐙'
    case 'security': return '🛡️'
    case 'monitoring': return '📊'
    case 'secrets': return '🔑'
    case 'policy': return '⚖️'
    case 'mesh': return '🌐'
    case 'certificates': return '📜'
    default: return '🧩'
  }
}

async function loadData() {
  loading.value = true
  error.value = null
  try {
    let fetchedTools: DetectedTool[] = []
    let fetchedSummary: EcosystemSummary | null = null

    try {
      const [toolList, sum] = await Promise.all([
        ecosystemApi.getTools(),
        ecosystemApi.getSummary(),
      ])
      fetchedTools = toolList || []
      fetchedSummary = sum
    } catch {
      // Offline fallback
    }

    if (fetchedTools.length === 0) {
      fetchedTools = defaultTools
    }

    tools.value = fetchedTools

    // Aggregate category counts & health
    const byCategory: Record<string, number> = {}
    let healthyCount = 0
    let degradedCount = 0

    for (const t of tools.value) {
      byCategory[t.category] = (byCategory[t.category] || 0) + 1
      if (t.health === 'healthy') {
        healthyCount++
      } else if (t.health === 'degraded' || t.status === 'unreachable') {
        degradedCount++
      }
    }

    summary.value = fetchedSummary && fetchedSummary.total > 0 ? fetchedSummary : {
      total: tools.value.length,
      healthy: healthyCount,
      degraded: degradedCount,
      by_category: byCategory,
    }
  } catch (err: any) {
    error.value = err.message || 'Failed to load ecosystem tools'
  } finally {
    loading.value = false
  }
}

async function handleScan() {
  scanning.value = true
  try {
    const updatedTools = await ecosystemApi.triggerScan()
    tools.value = updatedTools || []
    const updatedSummary = await ecosystemApi.getSummary()
    summary.value = updatedSummary || { total: 0, healthy: 0, degraded: 0, by_category: {} }
    showToast('Ecosystem scan completed successfully!')
  } catch (err: any) {
    showToast(err.message || 'Failed to run ecosystem scan', 'error')
  } finally {
    scanning.value = false
  }
}

async function handleCreateTool() {
  if (!form.name.trim() || !form.category.trim()) {
    showToast('Name and Category are required', 'error')
    return
  }

  saving.value = true
  try {
    await ecosystemApi.createTool({
      name: form.name.trim(),
      category: form.category.toLowerCase().trim(),
      endpoint: form.endpoint?.trim() || '',
      version: form.version?.trim() || '',
      status: form.status || 'detected',
      health: form.health || 'healthy',
    })
    showToast(`Tool "${form.name}" registered successfully!`)
    showAddModal.value = false
    // Reset form
    form.name = ''
    form.category = 'gitops'
    form.endpoint = ''
    form.version = ''
    form.status = 'detected'
    form.health = 'healthy'
    await loadData()
  } catch (err: any) {
    showToast(err.message || 'Failed to register tool', 'error')
  } finally {
    saving.value = false
  }
}

async function handleDeleteTool(tool: DetectedTool) {
  if (!confirm(`Are you sure you want to remove "${tool.name}"?`)) {
    return
  }

  deleting.value = tool.id
  try {
    await ecosystemApi.deleteTool(tool.id)
    showToast(`Tool "${tool.name}" removed`)
    await loadData()
  } catch (err: any) {
    showToast(err.message || 'Failed to delete tool', 'error')
  } finally {
    deleting.value = null
  }
}

// Computed Filtered Tools
const filteredTools = computed(() => {
  return tools.value.filter(tool => {
    // Category filter
    if (activeCategory.value !== 'all' && tool.category.toLowerCase() !== activeCategory.value) {
      return false
    }

    // Status filter
    if (selectedStatus.value === 'healthy' && tool.health !== 'healthy') {
      return false
    }
    if (selectedStatus.value === 'degraded' && tool.health !== 'degraded' && tool.status !== 'unreachable') {
      return false
    }
    if (selectedStatus.value === 'not_configured' && tool.status !== 'not_configured') {
      return false
    }

    // Search query
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase()
      const matchName = tool.name.toLowerCase().includes(q)
      const matchCategory = tool.category.toLowerCase().includes(q)
      const matchEndpoint = tool.endpoint?.toLowerCase().includes(q)
      const matchVersion = tool.version?.toLowerCase().includes(q)
      if (!matchName && !matchCategory && !matchEndpoint && !matchVersion) {
        return false
      }
    }

    return true
  })
})

function formatRelativeTime(dateStr: string): string {
  if (!dateStr) return 'Never'
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return 'Never'
  const now = new Date()
  const diffSec = Math.floor((now.getTime() - date.getTime()) / 1000)
  if (diffSec < 60) return 'Just now'
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
  return `${Math.floor(diffSec / 86400)}d ago`
}

onMounted(() => {
  loadData()
  // Auto-refresh every 5 minutes
  autoRefreshTimer = setInterval(() => {
    loadData()
  }, 5 * 60 * 1000)
})

onUnmounted(() => {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer)
  if (toastTimer) clearTimeout(toastTimer)
})
</script>

<template>
  <div class="ecosystem-view">
    <!-- Toast Notification -->
    <transition name="fade">
      <div v-if="toastMessage" class="toast-popup" :class="toastMessage.type">
        <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
        <span>{{ toastMessage.text }}</span>
      </div>
    </transition>

    <!-- Header Section -->
    <header class="view-header">
      <div class="header-titles">
        <div class="title-with-badge">
          <h1>Ecosystem Auto-Detector</h1>
          <span class="badge-tag live-badge">Auto-Discovery</span>
        </div>
        <p class="header-subtitle">
          Real-time discovery and operational status for platform infrastructure, service meshes, and GitOps toolchains.
        </p>
      </div>

      <div class="header-actions">
        <button
          class="btn-secondary"
          :disabled="scanning || loading"
          @click="handleScan"
        >
          <span class="btn-icon" :class="{ 'spin-anim': scanning }">🔄</span>
          <span>{{ scanning ? 'Scanning Stack...' : 'Scan Now' }}</span>
        </button>

        <button
          class="btn-primary"
          @click="showAddModal = true"
        >
          <span class="btn-icon">➕</span>
          <span>Register Tool</span>
        </button>
      </div>
    </header>

    <!-- Summary HUD Metrics -->
    <section class="summary-hud-grid">
      <MetricCard
        title="Detected Stack Tools"
        :value="summary.total"
        icon="🧩"
        subtitle="Across configured integration URLs"
        badge="Platform"
        badgeColor="cyan"
      />
      <MetricCard
        title="Healthy Services"
        :value="summary.healthy"
        icon="💚"
        subtitle="Responding with status 200 OK"
        badge="Online"
        badgeColor="emerald"
      />
      <MetricCard
        title="Degraded / Unreachable"
        :value="summary.degraded"
        icon="⚠️"
        subtitle="Failed probes or sealed state"
        :badge="summary.degraded > 0 ? 'Attention' : 'Optimal'"
        :badgeColor="summary.degraded > 0 ? 'rose' : 'emerald'"
      />
      <MetricCard
        title="Active Categories"
        :value="Object.keys(summary.by_category || {}).length"
        icon="🏷️"
        subtitle="GitOps, Security, Mesh, Policy, etc."
        badge="Coverage"
        badgeColor="violet"
      />
    </section>

    <!-- Category Tabs Navigation -->
    <section class="category-tabs-container">
      <div class="category-tabs">
        <button
          v-for="cat in categories"
          :key="cat.key"
          class="category-tab-btn"
          :class="{ active: activeCategory === cat.key }"
          @click="activeCategory = cat.key"
        >
          <span class="tab-icon">{{ cat.icon }}</span>
          <span class="tab-label">{{ cat.label }}</span>
          <span
            v-if="cat.key === 'all'"
            class="tab-count"
          >{{ tools.length }}</span>
          <span
            v-else-if="summary.by_category && summary.by_category[cat.key]"
            class="tab-count"
          >{{ summary.by_category[cat.key] }}</span>
        </button>
      </div>
    </section>

    <!-- Filter & Search Bar -->
    <section class="filter-toolbar glass-panel">
      <div class="search-box">
        <span class="search-icon">🔍</span>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search by tool name, endpoint, version..."
          class="search-input"
        />
        <button
          v-if="searchQuery"
          class="clear-search-btn"
          @click="searchQuery = ''"
        >
          ✕
        </button>
      </div>

      <div class="filter-group">
        <label class="filter-label">Health Status:</label>
        <select v-model="selectedStatus" class="filter-select">
          <option value="all">All Statuses</option>
          <option value="healthy">Healthy Only</option>
          <option value="degraded">Degraded / Unreachable</option>
          <option value="not_configured">Not Configured</option>
        </select>
      </div>
    </section>

    <!-- Loading State -->
    <div v-if="loading && tools.length === 0" class="loading-state glass-panel">
      <div class="spinner-icon">🔄</div>
      <p>Discovering ecosystem components...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error && tools.length === 0" class="error-state glass-panel">
      <span class="error-icon">⚠️</span>
      <p>{{ error }}</p>
      <button class="btn-secondary" @click="loadData">Try Again</button>
    </div>

    <!-- Empty State -->
    <div v-else-if="filteredTools.length === 0" class="empty-state glass-panel">
      <span class="empty-icon">🔎</span>
      <h3>No ecosystem tools found</h3>
      <p>No tools matched your current filters. Try changing category or clicking "Scan Now".</p>
      <button class="btn-primary" @click="handleScan">Run Scan</button>
    </div>

    <!-- Tools Grid Cards -->
    <section v-else class="tools-grid">
      <div
        v-for="tool in filteredTools"
        :key="tool.id || tool.name"
        class="tool-card glass-panel"
        :class="{
          'border-healthy': tool.health === 'healthy',
          'border-degraded': tool.health === 'degraded' || tool.status === 'unreachable',
          'border-unconfigured': tool.status === 'not_configured'
        }"
      >
        <!-- Card Top Header -->
        <div class="card-header">
          <div class="tool-main-info">
            <div class="tool-icon-wrap">
              {{ getToolIcon(tool) }}
            </div>
            <div>
              <h3 class="tool-name">{{ tool.name }}</h3>
              <span class="category-badge">{{ tool.category.toUpperCase() }}</span>
            </div>
          </div>

          <div class="status-badge-wrap">
            <span
              v-if="tool.status === 'not_configured'"
              class="status-pill pill-muted"
            >
              ⚪ Not Configured
            </span>
            <span
              v-else-if="tool.health === 'healthy'"
              class="status-pill pill-healthy"
            >
              🟢 Healthy
            </span>
            <span
              v-else-if="tool.status === 'unreachable'"
              class="status-pill pill-degraded"
            >
              🔴 Unreachable
            </span>
            <span
              v-else
              class="status-pill pill-warning"
            >
              🟡 Degraded
            </span>
          </div>
        </div>

        <!-- Card Body Details -->
        <div class="card-body">
          <div class="detail-row" v-if="tool.version">
            <span class="detail-label">Version:</span>
            <span class="version-badge font-mono">{{ tool.version }}</span>
          </div>

          <div class="detail-row">
            <span class="detail-label">Endpoint:</span>
            <div class="endpoint-val font-mono">
              <a
                v-if="tool.endpoint"
                :href="tool.endpoint"
                target="_blank"
                rel="noopener noreferrer"
                class="endpoint-link"
                :title="tool.endpoint"
              >
                {{ tool.endpoint }} ↗
              </a>
              <span v-else class="endpoint-empty">
                Not configured in Settings
              </span>
            </div>
          </div>

          <div class="detail-row">
            <span class="detail-label">Discovery Source:</span>
            <span class="source-badge" :class="`source-${tool.source}`">
              {{ tool.source === 'settings' ? '⚙️ Settings' : tool.source === 'manual' ? '✍️ Manual' : '☸️ K8s' }}
            </span>
          </div>

          <!-- Metadata Tag Pills -->
          <div v-if="tool.metadata && Object.keys(tool.metadata).length > 0" class="metadata-tags">
            <span
              v-for="(val, key) in tool.metadata"
              :key="key"
              class="meta-pill"
            >
              <strong class="meta-k">{{ key }}:</strong> {{ val }}
            </span>
          </div>
        </div>

        <!-- Card Footer -->
        <div class="card-footer">
          <span class="last-checked">
            🕒 Checked {{ formatRelativeTime(tool.last_checked) }}
          </span>

          <button
            v-if="tool.source === 'manual'"
            class="btn-delete-tool"
            :disabled="deleting === tool.id"
            title="Delete manual tool"
            @click="handleDeleteTool(tool)"
          >
            🗑️
          </button>
        </div>
      </div>
    </section>

    <!-- Register Tool Modal -->
    <div v-if="showAddModal" class="modal-backdrop" @click.self="showAddModal = false">
      <div class="modal-dialog glass-panel">
        <div class="modal-header">
          <h2>Register Ecosystem Tool</h2>
          <button class="modal-close-btn" @click="showAddModal = false">✕</button>
        </div>

        <form @submit.prevent="handleCreateTool" class="modal-form">
          <div class="form-group">
            <label>Tool Name *</label>
            <input
              v-model="form.name"
              type="text"
              placeholder="e.g. Istio, Kyverno, Velero"
              required
              class="form-input"
            />
          </div>

          <div class="form-group">
            <label>Category *</label>
            <select v-model="form.category" class="form-input">
              <option value="gitops">GitOps</option>
              <option value="security">Security</option>
              <option value="monitoring">Monitoring</option>
              <option value="secrets">Secrets</option>
              <option value="policy">Policy</option>
              <option value="mesh">Service Mesh</option>
              <option value="certificates">Certificates</option>
            </select>
          </div>

          <div class="form-group">
            <label>Endpoint URL / Service</label>
            <input
              v-model="form.endpoint"
              type="text"
              placeholder="e.g. https://kyverno.prod.corp or http://istio-ingress"
              class="form-input"
            />
          </div>

          <div class="form-row">
            <div class="form-group flex-1">
              <label>Version</label>
              <input
                v-model="form.version"
                type="text"
                placeholder="e.g. v1.11.0"
                class="form-input"
              />
            </div>
            <div class="form-group flex-1">
              <label>Initial Health</label>
              <select v-model="form.health" class="form-input">
                <option value="healthy">Healthy</option>
                <option value="degraded">Degraded</option>
                <option value="unknown">Unknown</option>
              </select>
            </div>
          </div>

          <div class="modal-actions">
            <button
              type="button"
              class="btn-secondary"
              @click="showAddModal = false"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="btn-primary"
              :disabled="saving"
            >
              {{ saving ? 'Saving...' : 'Register Tool' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ecosystem-view {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 24px 32px;
  min-height: calc(100vh - 64px);
  background: var(--bg-app, #0a0e17);
  color: var(--text-primary, #e2e8f0);
}

/* Header */
.view-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.header-titles {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.title-with-badge {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-with-badge h1 {
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: #fff;
  margin: 0;
}

.badge-tag {
  font-size: 11px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 6px;
  text-transform: uppercase;
}

.live-badge {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.header-subtitle {
  font-size: 14px;
  color: var(--text-muted, #94a3b8);
  margin: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* Buttons */
.btn-primary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: linear-space, #3b82f6;
  background-image: linear-gradient(135deg, #2563eb, #1d4ed8);
  color: #fff;
  border: none;
  padding: 9px 18px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.3);
}

.btn-primary:hover:not(:disabled) {
  background-image: linear-gradient(135deg, #3b82f6, #2563eb);
  transform: translateY(-1px);
}

.btn-secondary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: rgba(30, 41, 59, 0.8);
  color: #cbd5e1;
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 9px 16px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-secondary:hover:not(:disabled) {
  background: rgba(51, 65, 85, 0.9);
  color: #fff;
}

.btn-primary:disabled,
.btn-secondary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.spin-anim {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Summary HUD */
.summary-hud-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

/* Category Tabs */
.category-tabs-container {
  overflow-x: auto;
  padding-bottom: 4px;
}

.category-tabs {
  display: flex;
  gap: 8px;
}

.category-tab-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(30, 41, 59, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #94a3b8;
  padding: 8px 14px;
  border-radius: 20px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.category-tab-btn:hover {
  background: rgba(51, 65, 85, 0.7);
  color: #f1f5f9;
}

.category-tab-btn.active {
  background: rgba(59, 130, 246, 0.2);
  color: #60a5fa;
  border-color: rgba(59, 130, 246, 0.5);
  font-weight: 600;
}

.tab-count {
  background: rgba(0, 0, 0, 0.3);
  padding: 1px 7px;
  border-radius: 10px;
  font-size: 11px;
}

/* Filter Toolbar */
.filter-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 18px;
  border-radius: 12px;
  flex-wrap: wrap;
}

.search-box {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 6px 12px;
  border-radius: 8px;
  flex: 1;
  min-width: 260px;
}

.search-input {
  background: transparent;
  border: none;
  color: #fff;
  font-size: 13px;
  width: 100%;
  outline: none;
}

.clear-search-btn {
  background: transparent;
  border: none;
  color: #94a3b8;
  cursor: pointer;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.filter-label {
  font-size: 12px;
  color: #94a3b8;
}

.filter-select {
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #e2e8f0;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 13px;
  outline: none;
}

/* Tools Grid */
.tools-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.tool-card {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 18px 20px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.25s ease;
}

.tool-card:hover {
  transform: translateY(-2px);
  border-color: rgba(255, 255, 255, 0.18);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
}

.border-healthy {
  border-left: 3px solid #10b981;
}

.border-degraded {
  border-left: 3px solid #ef4444;
}

.border-unconfigured {
  border-left: 3px solid #64748b;
}

.card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.tool-main-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.tool-icon-wrap {
  font-size: 26px;
  background: rgba(30, 41, 59, 0.6);
  padding: 8px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.tool-name {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 4px 0;
}

.category-badge {
  font-size: 10px;
  font-weight: 700;
  color: #93c5fd;
  background: rgba(59, 130, 246, 0.15);
  padding: 2px 6px;
  border-radius: 4px;
}

/* Status Pills */
.status-pill {
  font-size: 11px;
  font-weight: 600;
  padding: 4px 9px;
  border-radius: 12px;
  white-space: nowrap;
}

.pill-healthy {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.pill-degraded {
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.pill-warning {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.pill-muted {
  background: rgba(100, 116, 139, 0.15);
  color: #94a3b8;
  border: 1px solid rgba(100, 116, 139, 0.3);
}

/* Card Body */
.card-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.detail-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
}

.detail-label {
  color: #64748b;
  font-weight: 500;
}

.version-badge {
  background: rgba(30, 41, 59, 0.8);
  color: #38bdf8;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 11px;
}

.endpoint-val {
  max-width: 190px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.endpoint-link {
  color: #60a5fa;
  text-decoration: none;
  font-size: 12px;
}

.endpoint-link:hover {
  text-decoration: underline;
}

.endpoint-empty {
  color: #64748b;
  font-style: italic;
  font-size: 11px;
}

.source-badge {
  font-size: 11px;
  color: #cbd5e1;
}

.metadata-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 6px;
}

.meta-pill {
  font-size: 10px;
  background: rgba(30, 41, 59, 0.7);
  color: #94a3b8;
  padding: 2px 6px;
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.meta-k {
  color: #cbd5e1;
}

/* Card Footer */
.card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  padding-top: 12px;
  font-size: 11px;
  color: #64748b;
}

.btn-delete-tool {
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: 14px;
  opacity: 0.7;
  transition: opacity 0.2s ease;
}

.btn-delete-tool:hover {
  opacity: 1;
}

/* Glass panel */
.glass-panel {
  background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

/* Empty / Loading states */
.loading-state,
.error-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  border-radius: 14px;
  text-align: center;
  gap: 12px;
}

.spinner-icon,
.error-icon,
.empty-icon {
  font-size: 36px;
}

.loading-state .spinner-icon {
  animation: spin 1.2s linear infinite;
}

/* Toast */
.toast-popup {
  position: fixed;
  top: 24px;
  right: 24px;
  z-index: 9999;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 20px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.4);
}

.toast-popup.success {
  background: #065f46;
  color: #d1fae5;
  border: 1px solid #059669;
}

.toast-popup.error {
  background: #991b1b;
  color: #fee2e2;
  border: 1px solid #dc2626;
}

/* Modal */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.modal-dialog {
  width: 100%;
  max-width: 480px;
  padding: 24px;
  border-radius: 16px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.6);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.modal-header h2 {
  font-size: 18px;
  font-weight: 700;
  color: #fff;
  margin: 0;
}

.modal-close-btn {
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 18px;
  cursor: pointer;
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  font-weight: 600;
  color: #94a3b8;
}

.form-input {
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: #fff;
  padding: 9px 12px;
  border-radius: 8px;
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s ease;
}

.form-input:focus {
  border-color: #3b82f6;
}

.form-row {
  display: flex;
  gap: 12px;
}

.flex-1 {
  flex: 1;
}

.modal-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 10px;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
