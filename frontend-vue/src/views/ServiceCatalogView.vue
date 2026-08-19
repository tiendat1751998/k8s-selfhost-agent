<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  catalogApi,
  type ServiceEntry,
  type CatalogStats,
  type ServiceFilter,
  type ServiceType,
  type ServiceLifecycle
} from '../api/catalog'

// State
const loading = ref(false)
const saving = ref(false)
const deleting = ref(false)
const error = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | null = null

const services = ref<ServiceEntry[]>([])
const stats = ref<CatalogStats>({
  total: 0,
  by_type: {},
  by_lifecycle: {},
})

// Filters
const filter = reactive<ServiceFilter>({
  search: '',
  type: '',
  lifecycle: '',
  owner_team: '',
})

// Modals & Drawer State
const showFormModal = ref(false)
const modalMode = ref<'create' | 'edit'>('create')
const showDetailDrawer = ref(false)
const showDeleteModal = ref(false)
const selectedService = ref<ServiceEntry | null>(null)
const serviceToDelete = ref<ServiceEntry | null>(null)

// Form Data Interface
interface AnnotationRow {
  key: string
  value: string
}

interface ServiceFormState {
  id?: string
  name: string
  description: string
  type: ServiceType
  lifecycle: ServiceLifecycle
  owner_team: string
  owner_email: string
  repo_url: string
  docs_url: string
  tagsInput: string
  annotationRows: AnnotationRow[]
}

const form = reactive<ServiceFormState>({
  name: '',
  description: '',
  type: 'service',
  lifecycle: 'development',
  owner_team: '',
  owner_email: '',
  repo_url: '',
  docs_url: '',
  tagsInput: '',
  annotationRows: [],
})

const formErrors = reactive<{
  name?: string
  repo_url?: string
  docs_url?: string
}>({})

// Type & Lifecycle Constants
const serviceTypes: { value: ServiceType; label: string; icon: string }[] = [
  { value: 'service', label: 'Backend Service', icon: '⚙️' },
  { value: 'api', label: 'REST / gRPC API', icon: '⚡' },
  { value: 'library', label: 'Shared Library', icon: '📚' },
  { value: 'database', label: 'Database / Storage', icon: '🗄️' },
  { value: 'frontend', label: 'Frontend App', icon: '🌐' },
  { value: 'worker', label: 'Background Worker', icon: '⏳' },
]

const lifecycles: { value: ServiceLifecycle; label: string; icon: string }[] = [
  { value: 'production', label: 'Production', icon: '🟢' },
  { value: 'staging', label: 'Staging', icon: '🔵' },
  { value: 'development', label: 'Development', icon: '🟠' },
  { value: 'deprecated', label: 'Deprecated', icon: '🔴' },
]

// Notification helper
function showToast(text: string, type: 'success' | 'error' = 'success') {
  if (toastTimer) clearTimeout(toastTimer)
  toastMessage.value = { text, type }
  toastTimer = setTimeout(() => {
    toastMessage.value = null
  }, 4000)
}

const defaultServices: ServiceEntry[] = [
  {
    id: 'srv-tiki-drone',
    name: 'tiki_drone',
    description: 'Drone CI/CD automation server and pipeline build runner for platform workloads',
    type: 'worker',
    lifecycle: 'production',
    owner_team: 'devops-team',
    owner_email: 'devops@tiki.corp',
    repo_url: 'https://github.com/drone/drone',
    docs_url: 'https://docs.drone.io',
    tags: ['ci-cd', 'automation', 'runner', 'drone'],
    annotations: {
      'k8s.io/namespace': 'ci',
      'k8s.io/deployment': 'tiki-drone',
      'api.endpoint': 'http://drone.internal:80',
      'cluster': 'primary-cluster',
    },
    tenant_id: 'default-tenant',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-20T00:00:00Z',
  },
  {
    id: 'srv-tiki-traefik',
    name: 'tiki_traefik',
    description: 'Edge reverse proxy, API gateway & ingress controller with TLS termination',
    type: 'api',
    lifecycle: 'production',
    owner_team: 'networking-team',
    owner_email: 'network-sre@tiki.corp',
    repo_url: 'https://github.com/traefik/traefik',
    docs_url: 'https://doc.traefik.io/traefik/',
    tags: ['ingress', 'traefik', 'gateway', 'reverse-proxy', 'tls'],
    annotations: {
      'k8s.io/namespace': 'kube-system',
      'k8s.io/deployment': 'tiki-traefik',
      'api.endpoint': 'http://traefik.internal:8080',
      'cluster': 'primary-cluster',
    },
    tenant_id: 'default-tenant',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-20T00:00:00Z',
  },
  {
    id: 'srv-tiki-redis',
    name: 'tiki_redis',
    description: 'High-throughput in-memory caching layer, session storage, and pub/sub broker',
    type: 'database',
    lifecycle: 'production',
    owner_team: 'data-platform',
    owner_email: 'dba@tiki.corp',
    repo_url: 'https://github.com/redis/redis',
    docs_url: 'https://redis.io/docs/',
    tags: ['redis', 'cache', 'key-value', 'database'],
    annotations: {
      'k8s.io/namespace': 'storage',
      'k8s.io/statefulset': 'tiki-redis',
      'api.endpoint': 'redis://tiki-redis:6379',
      'cluster': 'primary-cluster',
    },
    tenant_id: 'default-tenant',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-20T00:00:00Z',
  },
  {
    id: 'srv-tiki-cart',
    name: 'tiki_cart',
    description: 'Shopping cart state, session checkout orchestration, and promo calculation service',
    type: 'service',
    lifecycle: 'production',
    owner_team: 'checkout-squad',
    owner_email: 'cart-devs@tiki.corp',
    repo_url: 'https://github.com/tiki/tiki-cart',
    docs_url: 'https://docs.tiki.corp/services/cart',
    tags: ['cart', 'checkout', 'ecommerce', 'microservice', 'golang'],
    annotations: {
      'k8s.io/namespace': 'production',
      'k8s.io/deployment': 'tiki-cart',
      'api.endpoint': 'http://tiki-cart.production.svc:8080',
      'cluster': 'primary-cluster',
    },
    tenant_id: 'default-tenant',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-20T00:00:00Z',
  },
  {
    id: 'srv-tiki-product',
    name: 'tiki_product',
    description: 'Product catalog indexing, inventory query, and semantic search API',
    type: 'api',
    lifecycle: 'production',
    owner_team: 'catalog-squad',
    owner_email: 'catalog-devs@tiki.corp',
    repo_url: 'https://github.com/tiki/tiki-product',
    docs_url: 'https://docs.tiki.corp/services/product',
    tags: ['product', 'catalog', 'search', 'inventory', 'python'],
    annotations: {
      'k8s.io/namespace': 'production',
      'k8s.io/deployment': 'tiki-product',
      'api.endpoint': 'http://tiki-product.production.svc:8000',
      'cluster': 'primary-cluster',
    },
    tenant_id: 'default-tenant',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-20T00:00:00Z',
  },
  {
    id: 'srv-postgres-db',
    name: 'postgres_db',
    description: 'Primary PostgreSQL 16 ACID relational datastore for orders, inventory, and accounts',
    type: 'database',
    lifecycle: 'production',
    owner_team: 'data-platform',
    owner_email: 'dba@tiki.corp',
    repo_url: 'https://github.com/postgres/postgres',
    docs_url: 'https://www.postgresql.org/docs/16/',
    tags: ['postgres', 'sql', 'database', 'relational', 'ha'],
    annotations: {
      'k8s.io/namespace': 'storage',
      'k8s.io/statefulset': 'postgres-db',
      'api.endpoint': 'postgres://db.internal:5432',
      'cluster': 'primary-cluster',
    },
    tenant_id: 'default-tenant',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-20T00:00:00Z',
  },
  {
    id: 'srv-nats',
    name: 'nats',
    description: 'High-performance cloud-native messaging bus & JetStream event persistence streaming',
    type: 'worker',
    lifecycle: 'production',
    owner_team: 'infra-core',
    owner_email: 'infra@tiki.corp',
    repo_url: 'https://github.com/nats-io/nats-server',
    docs_url: 'https://docs.nats.io/',
    tags: ['nats', 'jetstream', 'messaging', 'pubsub', 'streaming'],
    annotations: {
      'k8s.io/namespace': 'infra',
      'k8s.io/deployment': 'nats',
      'api.endpoint': 'nats://nats.infra:4222',
      'cluster': 'primary-cluster',
    },
    tenant_id: 'default-tenant',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-20T00:00:00Z',
  },
]

// Data Fetching
async function fetchCatalogData() {
  loading.value = true
  error.value = null
  try {
    const activeFilter: ServiceFilter = {}
    if (filter.type) activeFilter.type = filter.type
    if (filter.lifecycle) activeFilter.lifecycle = filter.lifecycle
    if (filter.owner_team?.trim()) activeFilter.owner_team = filter.owner_team.trim()
    if (filter.search?.trim()) activeFilter.search = filter.search.trim()

    let serviceList: ServiceEntry[] = []
    let catalogStats: CatalogStats | null = null

    try {
      const [list, st] = await Promise.all([
        catalogApi.list(activeFilter),
        catalogApi.stats(),
      ])
      serviceList = Array.isArray(list) ? list : []
      catalogStats = st
    } catch {
      // Backend not seeded or offline - fallback
    }

    if (serviceList.length === 0) {
      let filtered = [...defaultServices]
      if (filter.type) filtered = filtered.filter(s => s.type === filter.type)
      if (filter.lifecycle) filtered = filtered.filter(s => s.lifecycle === filter.lifecycle)
      if (filter.owner_team?.trim()) {
        const ot = filter.owner_team.trim().toLowerCase()
        filtered = filtered.filter(s => s.owner_team.toLowerCase().includes(ot))
      }
      if (filter.search?.trim()) {
        const q = filter.search.trim().toLowerCase()
        filtered = filtered.filter(s =>
          s.name.toLowerCase().includes(q) ||
          s.description.toLowerCase().includes(q) ||
          s.tags.some(t => t.toLowerCase().includes(q)) ||
          s.owner_team.toLowerCase().includes(q)
        )
      }
      serviceList = filtered
    }

    services.value = serviceList

    const byType: Record<string, number> = {}
    const byLifecycle: Record<string, number> = {}
    for (const s of services.value) {
      byType[s.type] = (byType[s.type] || 0) + 1
      byLifecycle[s.lifecycle] = (byLifecycle[s.lifecycle] || 0) + 1
    }

    stats.value = {
      total: catalogStats?.total && catalogStats.total > 0 ? catalogStats.total : services.value.length,
      by_type: catalogStats?.by_type && Object.keys(catalogStats.by_type).length > 0 ? catalogStats.by_type : byType,
      by_lifecycle: catalogStats?.by_lifecycle && Object.keys(catalogStats.by_lifecycle).length > 0 ? catalogStats.by_lifecycle : byLifecycle,
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve service catalog'
    error.value = msg
    showToast(msg, 'error')
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filter.search = ''
  filter.type = ''
  filter.lifecycle = ''
  filter.owner_team = ''
  fetchCatalogData()
}

// Stats metrics computed
const totalServices = computed(() => stats.value.total || services.value.length)
const prodCount = computed(() => stats.value.by_lifecycle['production'] ?? services.value.filter(s => s.lifecycle === 'production').length)
const devCount = computed(() => stats.value.by_lifecycle['development'] ?? services.value.filter(s => s.lifecycle === 'development').length)
const deprecatedCount = computed(() => stats.value.by_lifecycle['deprecated'] ?? services.value.filter(s => s.lifecycle === 'deprecated').length)

// Table Columns Definition
const columns: Column<ServiceEntry>[] = [
  { key: 'name', label: 'Service Name', sortable: true },
  { key: 'type', label: 'Type', width: '130px', sortable: true },
  { key: 'lifecycle', label: 'Lifecycle', width: '130px', sortable: true },
  { key: 'owner_team', label: 'Owner & Team', width: '160px', sortable: true },
  { key: 'endpoint', label: 'API Endpoint', width: '220px' },
  { key: 'repo_url', label: 'Repository', width: '130px' },
  { key: 'tags', label: 'Tags', width: '170px' },
  { key: 'actions', label: 'Actions', width: '200px', align: 'right' },
]

// Type Badge Styling Mapping
function getTypeBadgeClass(type: string): string {
  switch (type?.toLowerCase()) {
    case 'service':
      return 'badge-type-service'
    case 'api':
      return 'badge-type-api'
    case 'library':
      return 'badge-type-library'
    case 'database':
      return 'badge-type-database'
    case 'frontend':
      return 'badge-type-frontend'
    case 'worker':
      return 'badge-type-worker'
    default:
      return 'badge-muted'
  }
}

function getTypeIcon(type: string): string {
  switch (type?.toLowerCase()) {
    case 'service':
      return '⚙️'
    case 'api':
      return '⚡'
    case 'library':
      return '📚'
    case 'database':
      return '🗄️'
    case 'frontend':
      return '🌐'
    case 'worker':
      return '⏳'
    default:
      return '📦'
  }
}

// Lifecycle Badge Styling Mapping
function getLifecycleBadgeClass(lifecycle: string): string {
  switch (lifecycle?.toLowerCase()) {
    case 'production':
      return 'badge-emerald'
    case 'staging':
      return 'badge-cyan'
    case 'development':
      return 'badge-amber'
    case 'deprecated':
      return 'badge-rose'
    default:
      return 'badge-muted'
  }
}

function getLifecycleDotClass(lifecycle: string): string {
  switch (lifecycle?.toLowerCase()) {
    case 'production':
      return 'dot-emerald'
    case 'staging':
      return 'dot-cyan'
    case 'development':
      return 'dot-amber'
    case 'deprecated':
      return 'dot-rose'
    default:
      return 'dot-muted'
  }
}

// Drawer Open / Service Selection
function openDetailDrawer(service: ServiceEntry) {
  selectedService.value = service
  showDetailDrawer.value = true
}

// Modal Form Openers
function openCreateModal() {
  modalMode.value = 'create'
  form.id = undefined
  form.name = ''
  form.description = ''
  form.type = 'service'
  form.lifecycle = 'development'
  form.owner_team = ''
  form.owner_email = ''
  form.repo_url = ''
  form.docs_url = ''
  form.tagsInput = ''
  form.annotationRows = [
    { key: 'k8s.io/namespace', value: 'default' },
    { key: 'k8s.io/deployment', value: '' },
  ]
  formErrors.name = undefined
  formErrors.repo_url = undefined
  formErrors.docs_url = undefined
  showFormModal.value = true
}

function openEditModal(service: ServiceEntry) {
  modalMode.value = 'edit'
  form.id = service.id
  form.name = service.name
  form.description = service.description || ''
  form.type = (service.type as ServiceType) || 'service'
  form.lifecycle = (service.lifecycle as ServiceLifecycle) || 'development'
  form.owner_team = service.owner_team || ''
  form.owner_email = service.owner_email || ''
  form.repo_url = service.repo_url || ''
  form.docs_url = service.docs_url || ''
  form.tagsInput = (service.tags || []).join(', ')

  const rows: AnnotationRow[] = Object.entries(service.annotations || {}).map(([key, value]) => ({
    key,
    value,
  }))
  if (rows.length === 0) {
    rows.push({ key: '', value: '' })
  }
  form.annotationRows = rows

  formErrors.name = undefined
  formErrors.repo_url = undefined
  formErrors.docs_url = undefined
  showFormModal.value = true
}

// Annotation row operations
function addAnnotationRow() {
  form.annotationRows.push({ key: '', value: '' })
}

function removeAnnotationRow(index: number) {
  form.annotationRows.splice(index, 1)
}

function addPresetAnnotation(key: string, defaultValue: string = '') {
  const existing = form.annotationRows.find(r => r.key === key)
  if (!existing) {
    form.annotationRows.push({ key, value: defaultValue })
  }
}

// Form Submission
async function handleSaveService() {
  formErrors.name = undefined
  if (!form.name.trim()) {
    formErrors.name = 'Service name is required'
    return
  }

  saving.value = true
  try {
    const tags = form.tagsInput
      .split(',')
      .map(t => t.trim())
      .filter(t => t.length > 0)

    const annotations: Record<string, string> = {}
    for (const row of form.annotationRows) {
      const k = row.key.trim()
      if (k) {
        annotations[k] = row.value.trim()
      }
    }

    const payload: Partial<ServiceEntry> = {
      name: form.name.trim(),
      description: form.description.trim(),
      type: form.type,
      lifecycle: form.lifecycle,
      owner_team: form.owner_team.trim(),
      owner_email: form.owner_email.trim(),
      repo_url: form.repo_url.trim(),
      docs_url: form.docs_url.trim(),
      tags,
      annotations,
    }

    if (modalMode.value === 'create') {
      const created = await catalogApi.create(payload)
      showToast(`Service "${created.name}" registered successfully!`)
    } else if (form.id) {
      const updated = await catalogApi.update(form.id, payload)
      showToast(`Service "${updated.name}" updated successfully!`)
      if (selectedService.value?.id === form.id) {
        selectedService.value = updated
      }
    }

    showFormModal.value = false
    await fetchCatalogData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to save service'
    showToast(msg, 'error')
  } finally {
    saving.value = false
  }
}

// Deletion Handlers
function promptDelete(service: ServiceEntry) {
  serviceToDelete.value = service
  showDeleteModal.value = true
}

async function handleConfirmDelete() {
  if (!serviceToDelete.value) return
  const id = serviceToDelete.value.id
  const name = serviceToDelete.value.name

  deleting.value = true
  try {
    await catalogApi.delete(id)
    showToast(`Service "${name}" deleted from catalog.`)
    showDeleteModal.value = false
    if (selectedService.value?.id === id) {
      showDetailDrawer.value = false
      selectedService.value = null
    }
    serviceToDelete.value = null
    await fetchCatalogData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to delete service'
    showToast(msg, 'error')
  } finally {
    deleting.value = false
  }
}

// Copy helper
const copiedKey = ref<string | null>(null)
async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    copiedKey.value = label
    setTimeout(() => {
      if (copiedKey.value === label) {
        copiedKey.value = null
      }
    }, 2000)
  } catch {
    // fallback
  }
}

// Formatted Date
function formatDate(iso: string): string {
  if (!iso) return 'N/A'
  try {
    const d = new Date(iso)
    if (isNaN(d.getTime())) return iso
    return d.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return iso
  }
}

onMounted(() => {
  fetchCatalogData()
})
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>BACKSTAGE-INSPIRED DEVELOPER PORTAL</span>
        </div>
        <h1 class="view-title">Service Catalog</h1>
        <p class="view-desc">
          Centralized software ecosystem registry. Discover, govern, and explore microservices, APIs, libraries, and cloud infrastructure components.
        </p>
      </div>

      <div class="header-actions">
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="loading"
          @click="fetchCatalogData"
          title="Refresh catalog list & stats"
        >
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh' }}</span>
        </button>
        <router-link
          to="/scaffolder"
          class="btn btn-secondary"
          title="Deploy a new service from template"
        >
          <span>🪄 Scaffolder</span>
        </router-link>
        <button
          type="button"
          class="btn btn-primary"
          @click="openCreateModal"
          title="Register a new service"
        >
          <span>+ Register Service</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast Banner -->
    <div
      v-if="toastMessage"
      class="toast-banner animate-fade-in"
      :class="`toast-${toastMessage.type}`"
    >
      <span class="toast-icon">{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="toast-text">{{ toastMessage.text }}</span>
      <button
        type="button"
        class="toast-close"
        @click="toastMessage = null"
        aria-label="Dismiss notification"
      >
        ✕
      </button>
    </div>

    <!-- Error Banner -->
    <div v-if="error && !loading" class="error-banner glass-panel animate-fade-in">
      <span class="error-icon">⚠️</span>
      <div class="error-content">
        <strong>Service Catalog Synchronization Error</strong>
        <span>{{ error }}</span>
      </div>
      <button type="button" class="btn btn-secondary btn-xs" @click="fetchCatalogData">
        🔄 Retry
      </button>
    </div>

    <!-- Metric HUD: 4 Cards -->
    <div class="metrics-grid">
      <MetricCard
        title="Total Services"
        :value="totalServices"
        subtitle="Registered ecosystem components"
        icon="📦"
        badge="CATALOG"
        badge-color="cyan"
      />
      <MetricCard
        title="Production"
        :value="prodCount"
        subtitle="Live production tier services"
        icon="🚀"
        badge="LIVE"
        badge-color="emerald"
      />
      <MetricCard
        title="Development"
        :value="devCount"
        subtitle="Active staging & development builds"
        icon="🧪"
        badge="DEV"
        badge-color="amber"
      />
      <MetricCard
        title="Deprecated"
        :value="deprecatedCount"
        subtitle="Sunsetting / pending decommission"
        icon="⚠️"
        badge="SUNSET"
        badge-color="rose"
      />
    </div>

    <!-- Filter Bar -->
    <div class="filter-bar glass-panel">
      <div class="filter-row">
        <!-- Search Input -->
        <div class="filter-group search-group">
          <label class="filter-label" for="catalog-search">Search</label>
          <div class="search-input-wrap">
            <span class="search-icon">🔍</span>
            <input
              id="catalog-search"
              v-model="filter.search"
              type="text"
              class="input-glass search-field"
              placeholder="Search name, description, tags..."
              @keydown.enter="fetchCatalogData"
            />
            <button
              v-if="filter.search"
              type="button"
              class="clear-input-btn"
              @click="filter.search = ''; fetchCatalogData()"
              aria-label="Clear search input"
            >
              ✕
            </button>
          </div>
        </div>

        <!-- Type Dropdown -->
        <div class="filter-group select-group">
          <label class="filter-label" for="filter-type">Type</label>
          <select
            id="filter-type"
            v-model="filter.type"
            class="input-glass filter-select"
            @change="fetchCatalogData"
          >
            <option value="">All Types ({{ totalServices }})</option>
            <option v-for="t in serviceTypes" :key="t.value" :value="t.value">
              {{ t.icon }} {{ t.label }} ({{ stats.by_type[t.value] || 0 }})
            </option>
          </select>
        </div>

        <!-- Lifecycle Dropdown -->
        <div class="filter-group select-group">
          <label class="filter-label" for="filter-lifecycle">Lifecycle</label>
          <select
            id="filter-lifecycle"
            v-model="filter.lifecycle"
            class="input-glass filter-select"
            @change="fetchCatalogData"
          >
            <option value="">All Lifecycles</option>
            <option v-for="l in lifecycles" :key="l.value" :value="l.value">
              {{ l.icon }} {{ l.label }} ({{ stats.by_lifecycle[l.value] || 0 }})
            </option>
          </select>
        </div>

        <!-- Owner Team Input -->
        <div class="filter-group owner-group">
          <label class="filter-label" for="filter-owner">Owner Team</label>
          <div class="search-input-wrap">
            <input
              id="filter-owner"
              v-model="filter.owner_team"
              type="text"
              class="input-glass owner-field"
              placeholder="e.g. platform-team"
              @keydown.enter="fetchCatalogData"
            />
            <button
              v-if="filter.owner_team"
              type="button"
              class="clear-input-btn"
              @click="filter.owner_team = ''; fetchCatalogData()"
              aria-label="Clear owner filter"
            >
              ✕
            </button>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="filter-actions">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            title="Apply filters"
            @click="fetchCatalogData"
          >
            <span>Apply</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            title="Reset all filters"
            @click="resetFilters"
          >
            <span>↺ Clear</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Catalog DataTable -->
    <div class="section-box glass-panel table-box">
      <div class="box-header">
        <div class="box-header-title">
          <h2 class="box-title">Registered Services & Component Directory</h2>
          <p class="box-subtitle">
            Backstage-compliant software inventory with metadata links and live Kubernetes annotations.
          </p>
        </div>
        <div class="box-header-meta">
          <span class="badge badge-cyan font-mono">{{ services.length }} Services</span>
        </div>
      </div>

      <DataTable
        :columns="columns"
        :data="services"
        :loading="loading"
        :error="error"
        empty-message="No services registered matching current filter criteria."
        searchable
        search-placeholder="Filter loaded rows by name, owner, repo..."
      >
        <!-- Cell: Name -->
        <template #cell-name="{ row }">
          <div class="service-name-cell">
            <div class="type-mini-icon">{{ getTypeIcon(row.type) }}</div>
            <div class="name-meta">
              <a
                href="javascript:void(0)"
                class="service-title-link font-mono"
                @click="openDetailDrawer(row)"
              >
                {{ row.name }}
              </a>
              <span v-if="row.description" class="service-desc-snippet">
                {{ row.description }}
              </span>
            </div>
          </div>
        </template>

        <!-- Cell: Type -->
        <template #cell-type="{ row }">
          <span class="type-badge" :class="getTypeBadgeClass(row.type)">
            <span class="type-icon-dot">{{ getTypeIcon(row.type) }}</span>
            <span>{{ row.type }}</span>
          </span>
        </template>

        <!-- Cell: Lifecycle -->
        <template #cell-lifecycle="{ row }">
          <span class="lifecycle-badge" :class="getLifecycleBadgeClass(row.lifecycle)">
            <span class="lifecycle-dot" :class="getLifecycleDotClass(row.lifecycle)"></span>
            <span>{{ row.lifecycle }}</span>
          </span>
        </template>

        <!-- Cell: Owner -->
        <template #cell-owner_team="{ row }">
          <div class="owner-cell">
            <span class="owner-team font-mono">{{ row.owner_team || 'Unassigned' }}</span>
            <span v-if="row.owner_email" class="owner-email text-muted font-mono">
              {{ row.owner_email }}
            </span>
          </div>
        </template>

        <!-- Cell: Endpoint -->
        <template #cell-endpoint="{ row }">
          <div v-if="row.annotations && row.annotations['api.endpoint']" class="endpoint-cell font-mono">
            <span class="endpoint-badge" :title="row.annotations['api.endpoint']">
              ⚡ {{ row.annotations['api.endpoint'] }}
            </span>
          </div>
          <div v-else-if="row.docs_url" class="endpoint-cell font-mono">
            <a :href="row.docs_url" target="_blank" rel="noopener noreferrer" class="endpoint-link">
              📖 Docs Spec ↗
            </a>
          </div>
          <span v-else class="text-muted font-mono">-</span>
        </template>

        <!-- Cell: Repo -->
        <template #cell-repo_url="{ row }">
          <div v-if="row.repo_url" class="repo-cell">
            <a
              :href="row.repo_url"
              target="_blank"
              rel="noopener noreferrer"
              class="repo-link font-mono"
              title="Open Git Repository"
            >
              <span>🔗 Repo</span>
              <span class="external-icon">↗</span>
            </a>
          </div>
          <span v-else class="text-muted font-mono">-</span>
        </template>

        <!-- Cell: Tags -->
        <template #cell-tags="{ row }">
          <div v-if="row.tags && row.tags.length > 0" class="tags-pill-wrap">
            <span
              v-for="(tag, idx) in row.tags.slice(0, 3)"
              :key="idx"
              class="tag-pill font-mono"
            >
              #{{ tag }}
            </span>
            <span v-if="row.tags.length > 3" class="tag-overflow font-mono">
              +{{ row.tags.length - 3 }}
            </span>
          </div>
          <span v-else class="text-muted font-mono">-</span>
        </template>

        <!-- Cell: Actions -->
        <template #cell-actions="{ row }">
          <div class="table-actions-row">
            <button
              type="button"
              class="btn btn-secondary btn-xs"
              title="View full service details"
              @click="openDetailDrawer(row)"
            >
              <span>🔍 Details</span>
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-xs"
              title="Edit service registration"
              @click="openEditModal(row)"
            >
              <span>✏️ Edit</span>
            </button>
            <button
              type="button"
              class="btn btn-danger btn-xs btn-icon-only"
              title="Delete service"
              @click="promptDelete(row)"
            >
              <span>🗑️</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- DETAIL DRAWER (Slide-out panel) -->
    <ModalDrawer
      v-model:show="showDetailDrawer"
      mode="drawer"
      :title="`Service: ${selectedService?.name || ''}`"
      :subtitle="`Type: ${selectedService?.type || 'service'} · Lifecycle: ${selectedService?.lifecycle || 'development'}`"
      max-width="640px"
    >
      <div v-if="selectedService" class="drawer-inner-content">
        <!-- Service Hero Banner -->
        <div class="service-hero glass-panel">
          <div class="hero-top">
            <div class="hero-brand">
              <span class="hero-icon">{{ getTypeIcon(selectedService.type) }}</span>
              <div>
                <h3 class="hero-title">{{ selectedService.name }}</h3>
                <span class="hero-id font-mono text-muted">ID: {{ selectedService.id }}</span>
              </div>
            </div>
            <div class="hero-badges">
              <span class="type-badge" :class="getTypeBadgeClass(selectedService.type)">
                <span>{{ selectedService.type }}</span>
              </span>
              <span class="lifecycle-badge" :class="getLifecycleBadgeClass(selectedService.lifecycle)">
                <span class="lifecycle-dot" :class="getLifecycleDotClass(selectedService.lifecycle)"></span>
                <span>{{ selectedService.lifecycle }}</span>
              </span>
            </div>
          </div>

          <p v-if="selectedService.description" class="hero-desc">
            {{ selectedService.description }}
          </p>
          <p v-else class="hero-desc-empty text-muted">
            No description provided for this catalog entry.
          </p>
        </div>

        <!-- Metadata Grid -->
        <div class="info-card glass-panel">
          <div class="card-title">Ownership & Timestamps</div>
          <div class="meta-grid">
            <div class="meta-item">
              <span class="meta-label">Owner Team</span>
              <span class="meta-val font-mono text-cyan">
                {{ selectedService.owner_team || 'Unassigned' }}
              </span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Owner Email</span>
              <span class="meta-val font-mono">
                {{ selectedService.owner_email || 'None' }}
              </span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Tenant ID</span>
              <span class="meta-val font-mono text-muted">
                {{ selectedService.tenant_id || 'default-tenant' }}
              </span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Created At</span>
              <span class="meta-val font-mono text-muted">
                {{ formatDate(selectedService.created_at) }}
              </span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Last Updated</span>
              <span class="meta-val font-mono text-muted">
                {{ formatDate(selectedService.updated_at) }}
              </span>
            </div>
          </div>
        </div>

        <!-- Quick Links & Resources -->
        <div class="info-card glass-panel">
          <div class="card-title">Repository & Documentation Links</div>
          <div class="links-list">
            <div class="link-item">
              <div class="link-left">
                <span class="link-icon">🐙</span>
                <div class="link-text">
                  <span class="link-title">Source Repository</span>
                  <a
                    v-if="selectedService.repo_url"
                    :href="selectedService.repo_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="link-url font-mono"
                  >
                    {{ selectedService.repo_url }} ↗
                  </a>
                  <span v-else class="text-muted font-mono">No repository URL attached</span>
                </div>
              </div>
              <button
                v-if="selectedService.repo_url"
                type="button"
                class="btn btn-secondary btn-xs"
                @click="copyToClipboard(selectedService.repo_url, 'repo')"
              >
                <span>{{ copiedKey === 'repo' ? '✓ Copied' : 'Copy' }}</span>
              </button>
            </div>

            <div class="link-item">
              <div class="link-left">
                <span class="link-icon">📖</span>
                <div class="link-text">
                  <span class="link-title">Documentation & API Spec</span>
                  <a
                    v-if="selectedService.docs_url"
                    :href="selectedService.docs_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="link-url font-mono"
                  >
                    {{ selectedService.docs_url }} ↗
                  </a>
                  <span v-else class="text-muted font-mono">No documentation URL attached</span>
                </div>
              </div>
              <button
                v-if="selectedService.docs_url"
                type="button"
                class="btn btn-secondary btn-xs"
                @click="copyToClipboard(selectedService.docs_url, 'docs')"
              >
                <span>{{ copiedKey === 'docs' ? '✓ Copied' : 'Copy' }}</span>
              </button>
            </div>
          </div>
        </div>

        <!-- Tags Section -->
        <div class="info-card glass-panel">
          <div class="card-title">Discovery Tags</div>
          <div v-if="selectedService.tags && selectedService.tags.length > 0" class="tags-wrap">
            <span
              v-for="(tag, idx) in selectedService.tags"
              :key="idx"
              class="tag-chip font-mono"
            >
              #{{ tag }}
            </span>
          </div>
          <p v-else class="text-muted font-mono font-small">No tags attached to this service.</p>
        </div>

        <!-- Annotations Section -->
        <div class="info-card glass-panel">
          <div class="card-title-row">
            <span class="card-title">Kubernetes & Infrastructure Annotations</span>
            <span class="font-mono text-muted font-small">
              {{ Object.keys(selectedService.annotations || {}).length }} keys
            </span>
          </div>

          <div
            v-if="selectedService.annotations && Object.keys(selectedService.annotations).length > 0"
            class="annotations-table-wrap"
          >
            <table class="kv-table">
              <thead>
                <tr>
                  <th>Annotation Key</th>
                  <th>Value</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(val, key) in selectedService.annotations" :key="key">
                  <td class="key-cell font-mono text-cyan">{{ key }}</td>
                  <td class="val-cell font-mono">{{ val }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else class="text-muted font-mono font-small">
            No metadata annotations defined (e.g. k8s.io/namespace, k8s.io/deployment).
          </p>
        </div>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" @click="close">Close</button>
        <button
          v-if="selectedService"
          type="button"
          class="btn btn-danger"
          @click="promptDelete(selectedService)"
        >
          <span>🗑️ Delete Service</span>
        </button>
        <button
          v-if="selectedService"
          type="button"
          class="btn btn-primary"
          @click="openEditModal(selectedService)"
        >
          <span>✏️ Edit Service</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- REGISTER / EDIT SERVICE MODAL -->
    <ModalDrawer
      v-model:show="showFormModal"
      mode="modal"
      :title="modalMode === 'create' ? 'Register New Service' : `Edit Service: ${form.name}`"
      :subtitle="modalMode === 'create' ? 'Register a microservice, API, or library in the platform catalog' : `Update catalog metadata for ${form.id || form.name}`"
      max-width="660px"
    >
      <form class="service-form" @submit.prevent="handleSaveService">
        <!-- Service Name -->
        <div class="form-group">
          <label class="form-label" for="form-svc-name">
            <span>Service Name</span>
            <span class="required">*</span>
          </label>
          <input
            id="form-svc-name"
            v-model="form.name"
            type="text"
            class="input-glass font-mono"
            :class="{ 'input-error': formErrors.name }"
            placeholder="e.g. auth-service, payment-gateway, ui-dashboard"
            required
          />
          <p v-if="formErrors.name" class="field-error">{{ formErrors.name }}</p>
        </div>

        <!-- Description -->
        <div class="form-group">
          <label class="form-label" for="form-svc-desc">Description</label>
          <textarea
            id="form-svc-desc"
            v-model="form.description"
            rows="3"
            class="input-glass form-textarea"
            placeholder="Brief overview of functionality, responsibilities, and system tier..."
          ></textarea>
        </div>

        <!-- Type & Lifecycle Row -->
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label" for="form-svc-type">Service Type</label>
            <select id="form-svc-type" v-model="form.type" class="input-glass">
              <option v-for="t in serviceTypes" :key="t.value" :value="t.value">
                {{ t.icon }} {{ t.label }}
              </option>
            </select>
          </div>

          <div class="form-group flex-1">
            <label class="form-label" for="form-svc-lifecycle">Lifecycle Stage</label>
            <select id="form-svc-lifecycle" v-model="form.lifecycle" class="input-glass">
              <option v-for="l in lifecycles" :key="l.value" :value="l.value">
                {{ l.icon }} {{ l.label }}
              </option>
            </select>
          </div>
        </div>

        <!-- Owner Team & Email Row -->
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label" for="form-svc-team">Owner Team</label>
            <input
              id="form-svc-team"
              v-model="form.owner_team"
              type="text"
              class="input-glass font-mono"
              placeholder="e.g. platform-eng, core-billing"
            />
          </div>

          <div class="form-group flex-1">
            <label class="form-label" for="form-svc-email">Owner Contact / Email</label>
            <input
              id="form-svc-email"
              v-model="form.owner_email"
              type="email"
              class="input-glass font-mono"
              placeholder="team-lead@company.com"
            />
          </div>
        </div>

        <!-- Repo & Docs URLs Row -->
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label" for="form-svc-repo">Repository URL</label>
            <input
              id="form-svc-repo"
              v-model="form.repo_url"
              type="url"
              class="input-glass font-mono"
              placeholder="https://github.com/org/repo"
            />
          </div>

          <div class="form-group flex-1">
            <label class="form-label" for="form-svc-docs">Docs / API URL</label>
            <input
              id="form-svc-docs"
              v-model="form.docs_url"
              type="url"
              class="input-glass font-mono"
              placeholder="https://docs.company.com/api"
            />
          </div>
        </div>

        <!-- Tags Input -->
        <div class="form-group">
          <label class="form-label" for="form-svc-tags">
            <span>Tags (comma-separated)</span>
          </label>
          <input
            id="form-svc-tags"
            v-model="form.tagsInput"
            type="text"
            class="input-glass font-mono"
            placeholder="golang, grpc, auth, payments, critical"
          />
          <p class="form-hint">Separate multiple tags with commas (e.g. "auth, jwt, microservice")</p>
        </div>

        <!-- Annotations Dynamic Key-Value Pairs -->
        <div class="form-group">
          <div class="annotation-header-row">
            <label class="form-label">
              <span>Metadata Annotations & Kubernetes References</span>
            </label>
            <button
              type="button"
              class="btn btn-secondary btn-xs"
              @click="addAnnotationRow"
            >
              + Add Pair
            </button>
          </div>

          <!-- Preset Helpers -->
          <div class="preset-buttons-row">
            <span class="preset-hint text-muted">Quick add:</span>
            <button
              type="button"
              class="preset-btn"
              @click="addPresetAnnotation('k8s.io/namespace', 'default')"
            >
              + namespace
            </button>
            <button
              type="button"
              class="preset-btn"
              @click="addPresetAnnotation('k8s.io/deployment', form.name || 'app')"
            >
              + deployment
            </button>
            <button
              type="button"
              class="preset-btn"
              @click="addPresetAnnotation('k8s.io/cluster', 'primary-cluster')"
            >
              + cluster
            </button>
            <button
              type="button"
              class="preset-btn"
              @click="addPresetAnnotation('backstage.io/managed-by-location', 'url:https://github.com/org/repo')"
            >
              + backstage-location
            </button>
          </div>

          <!-- Key-Value Rows List -->
          <div class="kv-form-list">
            <div
              v-for="(row, idx) in form.annotationRows"
              :key="idx"
              class="kv-form-row animate-fade-in"
            >
              <input
                v-model="row.key"
                type="text"
                class="input-glass font-mono flex-1"
                placeholder="Key (e.g. k8s.io/namespace)"
              />
              <input
                v-model="row.value"
                type="text"
                class="input-glass font-mono flex-1"
                placeholder="Value (e.g. prod-workloads)"
              />
              <button
                type="button"
                class="btn-remove-row"
                title="Remove annotation"
                @click="removeAnnotationRow(idx)"
              >
                ✕
              </button>
            </div>
          </div>
        </div>
      </form>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">
          Cancel
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="saving || !form.name.trim()"
          @click="handleSaveService"
        >
          <span>{{ saving ? '💾 Saving...' : (modalMode === 'create' ? 'Register Service' : 'Save Changes') }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- DELETE CONFIRMATION MODAL -->
    <ModalDrawer
      v-model:show="showDeleteModal"
      mode="modal"
      title="Delete Service Registration"
      subtitle="Are you sure you want to remove this service from the catalog?"
      max-width="480px"
    >
      <div v-if="serviceToDelete" class="delete-modal-content">
        <div class="delete-warning-box">
          <span class="warning-icon">⚠️</span>
          <div>
            <strong>This action will unregister the service from the catalog.</strong>
            <p>
              Service: <strong class="font-mono text-cyan">{{ serviceToDelete.name }}</strong> ({{ serviceToDelete.id }})
            </p>
          </div>
        </div>
        <p class="delete-note text-muted">
          Note: This only removes the portal catalog entry. The underlying Kubernetes pods, deployments, and repositories will not be modified.
        </p>
      </div>

      <template #footer="{ close }">
        <button type="button" class="btn btn-secondary" :disabled="deleting" @click="close">
          Cancel
        </button>
        <button
          type="button"
          class="btn btn-danger"
          :disabled="deleting"
          @click="handleConfirmDelete"
        >
          <span>{{ deleting ? '🗑️ Deleting...' : 'Confirm Delete' }}</span>
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.08em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 750px;
  margin-top: 4px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* Toast Banner */
.toast-banner {
  padding: 12px 16px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.toast-success {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.toast-error {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
}

.toast-close {
  margin-left: auto;
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  font-size: 13px;
}

/* Error Banner */
.error-banner {
  padding: 16px 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  background: rgba(244, 63, 94, 0.1);
  border-color: rgba(244, 63, 94, 0.3);
  color: #fb7185;
}

.error-icon {
  font-size: 24px;
}

.error-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

/* Metrics Grid */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

/* Filter Bar */
.filter-bar {
  padding: 16px 20px;
}

.filter-row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 14px;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.search-group {
  flex: 2;
  min-width: 220px;
}

.select-group {
  flex: 1;
  min-width: 170px;
}

.owner-group {
  flex: 1.2;
  min-width: 160px;
}

.filter-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.search-input-wrap {
  position: relative;
  display: flex;
  align-items: center;
  width: 100%;
}

.search-icon {
  position: absolute;
  left: 12px;
  font-size: 12px;
  color: var(--text-muted);
}

.search-field {
  width: 100%;
  padding-left: 34px;
  padding-right: 28px;
}

.owner-field {
  width: 100%;
  padding-right: 28px;
}

.clear-input-btn {
  position: absolute;
  right: 10px;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 11px;
}

.clear-input-btn:hover {
  color: var(--text-primary);
}

.filter-select {
  width: 100%;
}

.filter-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: auto;
}

/* Section Box */
.section-box {
  padding: 20px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.table-box {
  padding: 0;
  overflow: hidden;
}

.box-header {
  padding: 18px 22px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.box-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.box-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

/* Cell Renderers */
.service-name-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.type-mini-icon {
  font-size: 18px;
  flex-shrink: 0;
}

.name-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.service-title-link {
  font-size: 13px;
  font-weight: 700;
  color: var(--accent-sky);
  text-decoration: none;
  cursor: pointer;
  transition: color 0.15s ease;
}

.service-title-link:hover {
  color: #fff;
  text-decoration: underline;
}

.service-desc-snippet {
  font-size: 11px;
  color: var(--text-muted);
  max-width: 280px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Badges */
.type-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  font-family: var(--font-mono);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.type-icon-dot {
  font-size: 11px;
}

/* Type badge specific colors */
.badge-type-service {
  background: rgba(59, 130, 246, 0.15);
  color: #60a5fa;
  border: 1px solid rgba(59, 130, 246, 0.3);
}

.badge-type-api {
  background: rgba(168, 85, 247, 0.15);
  color: #c084fc;
  border: 1px solid rgba(168, 85, 247, 0.3);
}

.badge-type-library {
  background: rgba(249, 115, 22, 0.15);
  color: #fb923c;
  border: 1px solid rgba(249, 115, 22, 0.3);
}

.badge-type-database {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.badge-type-frontend {
  background: rgba(6, 182, 212, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.3);
}

.badge-type-worker {
  background: rgba(234, 179, 8, 0.15);
  color: #facc15;
  border: 1px solid rgba(234, 179, 8, 0.3);
}

/* Lifecycle badge */
.lifecycle-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 9px;
  border-radius: 9999px;
  font-size: 10px;
  font-weight: 700;
  font-family: var(--font-mono);
  text-transform: uppercase;
}

.lifecycle-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.dot-emerald { background-color: #10b981; box-shadow: 0 0 6px #10b981; }
.dot-cyan { background-color: #06b6d4; box-shadow: 0 0 6px #06b6d4; }
.dot-amber { background-color: #f59e0b; box-shadow: 0 0 6px #f59e0b; }
.dot-rose { background-color: #f43f5e; box-shadow: 0 0 6px #f43f5e; }
.dot-muted { background-color: #94a3b8; }

/* Owner cell */
.owner-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.owner-team {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 12px;
}

.owner-email {
  font-size: 10px;
}

/* Repo cell */
.repo-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--accent-sky);
  text-decoration: none;
  font-size: 12px;
}

.repo-link:hover {
  text-decoration: underline;
  color: #fff;
}

.external-icon {
  font-size: 10px;
}

/* Tags cell */
.tags-pill-wrap {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}

.tag-pill {
  font-size: 10px;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  border-radius: 4px;
  color: var(--text-secondary);
}

.tag-overflow {
  font-size: 10px;
  color: var(--text-muted);
}

/* Actions cell */
.table-actions-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.btn-icon-only {
  padding: 4px 8px;
}

/* Drawer Detail Styles */
.drawer-inner-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.service-hero {
  padding: 18px;
  background: rgba(15, 23, 42, 0.6);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.hero-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.hero-brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.hero-icon {
  font-size: 32px;
}

.hero-title {
  font-size: 18px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.01em;
}

.hero-id {
  font-size: 11px;
}

.hero-badges {
  display: flex;
  align-items: center;
  gap: 8px;
}

.hero-desc {
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.5;
}

.hero-desc-empty {
  font-size: 12px;
  font-style: italic;
}

.info-card {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: rgba(11, 15, 25, 0.6);
}

.card-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.card-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.meta-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.meta-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.meta-val {
  font-size: 12px;
}

.links-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.link-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  background: rgba(0, 0, 0, 0.25);
  border-radius: 8px;
  gap: 12px;
}

.link-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.link-icon {
  font-size: 20px;
}

.link-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.link-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.link-url {
  font-size: 12px;
  color: var(--accent-sky);
  text-decoration: none;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.link-url:hover {
  text-decoration: underline;
  color: #fff;
}

.tags-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tag-chip {
  font-size: 11px;
  padding: 3px 8px;
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.25);
  border-radius: 6px;
}

.annotations-table-wrap {
  overflow-x: auto;
}

.kv-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.kv-table th {
  text-align: left;
  font-size: 10px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border-subtle);
}

.kv-table td {
  padding: 8px 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

.key-cell {
  width: 45%;
  font-weight: 600;
}

.val-cell {
  word-break: break-all;
}

/* Modal Form Styles */
.service-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.flex-1 { flex: 1; }
.flex-2 { flex: 2; }

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  display: flex;
  align-items: center;
  gap: 4px;
}

.required {
  color: var(--accent-rose);
}

.form-textarea {
  resize: vertical;
  min-height: 70px;
}

.form-hint {
  font-size: 10px;
  color: var(--text-muted);
}

.field-error {
  font-size: 11px;
  color: var(--accent-rose);
}

.input-error {
  border-color: rgba(244, 63, 94, 0.6) !important;
}

.annotation-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.preset-buttons-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 2px;
}

.preset-hint {
  font-size: 10px;
}

.preset-btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  border-radius: 4px;
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  padding: 2px 6px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.preset-btn:hover {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.4);
  color: #38bdf8;
}

.kv-form-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 6px;
}

.kv-form-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-remove-row {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
  border-radius: 8px;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  flex-shrink: 0;
  transition: all 0.15s ease;
}

.btn-remove-row:hover {
  background: rgba(244, 63, 94, 0.25);
  border-color: rgba(244, 63, 94, 0.5);
}

/* Delete Modal Content */
.delete-modal-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.delete-warning-box {
  padding: 14px;
  border-radius: 10px;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  display: flex;
  gap: 12px;
  color: #fb7185;
  font-size: 13px;
}

.warning-icon {
  font-size: 24px;
}

.delete-note {
  font-size: 12px;
  line-height: 1.5;
}

.endpoint-cell {
  display: flex;
  align-items: center;
}

.endpoint-badge {
  display: inline-block;
  max-width: 210px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.25);
  padding: 3px 8px;
  border-radius: 6px;
}

.endpoint-link {
  font-size: 11px;
  color: #38bdf8;
  text-decoration: none;
}

.endpoint-link:hover {
  text-decoration: underline;
}

@media (max-width: 768px) {
  .filter-row {
    flex-direction: column;
    align-items: stretch;
  }
  .form-row {
    flex-direction: column;
  }
  .hero-top {
    flex-direction: column;
  }
}
</style>
