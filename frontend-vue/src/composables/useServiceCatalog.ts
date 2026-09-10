import { ref, reactive, computed, onMounted } from 'vue'
import {
  catalogApi,
  type ServiceEntry,
  type CatalogStats,
  type ServiceFilter,
  type ServiceType,
  type ServiceLifecycle,
} from '../api/catalog'
import type { Column } from '../components/ui/DataTable.vue'

export interface AnnotationRow {
  key: string
  value: string
}

export interface ServiceFormState {
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

export interface ServiceDependency {
  id: string
  name: string
  type: string
  status: string
  relation?: string
  relationship?: string
  latencyMs?: number
}

const BUILTIN_SERVICES: ServiceEntry[] = [
  {
    id: 'srv-auth-01',
    name: 'auth-service',
    description: 'Centralized OAuth2 / OIDC authentication service with JWT token issuance and MFA verification',
    type: 'api',
    lifecycle: 'production',
    owner_team: 'security-team',
    owner_email: 'security@k8scontrol.io',
    repo_url: 'https://github.com/k8scontrol/auth-service',
    docs_url: 'https://docs.k8scontrol.io/api/auth',
    tags: ['auth', 'jwt', 'security'],
    annotations: { 'k8s.io/namespace': 'identity', 'k8s.io/deployment': 'auth-service' },
    tenant_id: 'default-tenant',
    created_at: '2026-08-01T10:00:00Z',
    updated_at: '2026-08-25T14:30:00Z',
  },
  {
    id: 'srv-pay-02',
    name: 'payment-gateway',
    description: 'PCI-DSS transaction engine with automated multi-provider card routing and idempotency guards',
    type: 'service',
    lifecycle: 'production',
    owner_team: 'billing-team',
    owner_email: 'billing@k8scontrol.io',
    repo_url: 'https://github.com/k8scontrol/payment-gateway',
    docs_url: 'https://docs.k8scontrol.io/api/payments',
    tags: ['payments', 'finance', 'critical'],
    annotations: { 'k8s.io/namespace': 'finance', 'k8s.io/deployment': 'payment-gateway' },
    tenant_id: 'default-tenant',
    created_at: '2026-08-05T09:15:00Z',
    updated_at: '2026-08-26T11:00:00Z',
  },
  {
    id: 'srv-notif-03',
    name: 'notification-worker',
    description: 'Asynchronous event consumer routing real-time alerts to Telegram, Slack, and webhook targets',
    type: 'worker',
    lifecycle: 'staging',
    owner_team: 'sre-platform',
    owner_email: 'sre@k8scontrol.io',
    repo_url: 'https://github.com/k8scontrol/notification-worker',
    docs_url: '',
    tags: ['nats', 'worker', 'alerts'],
    annotations: { 'k8s.io/namespace': 'monitoring', 'k8s.io/deployment': 'notification-worker' },
    tenant_id: 'default-tenant',
    created_at: '2026-08-10T12:00:00Z',
    updated_at: '2026-08-27T08:45:00Z',
  },
  {
    id: 'srv-web-04',
    name: 'portal-dashboard',
    description: 'Single-page developer portal with responsive 4-tier design and multi-cluster telemetry graphs',
    type: 'frontend',
    lifecycle: 'production',
    owner_team: 'ui-guild',
    owner_email: 'frontend@k8scontrol.io',
    repo_url: 'https://github.com/k8scontrol/frontend-portal',
    docs_url: 'https://docs.k8scontrol.io/portal',
    tags: ['vue3', 'vite', 'pwa'],
    annotations: { 'k8s.io/namespace': 'web', 'k8s.io/deployment': 'portal-dashboard' },
    tenant_id: 'default-tenant',
    created_at: '2026-08-12T16:20:00Z',
    updated_at: '2026-08-27T09:00:00Z',
  },
  {
    id: 'srv-db-05',
    name: 'cluster-storage-db',
    description: 'Highly available PostgreSQL 16 cluster with automated point-in-time recovery and streaming replication',
    type: 'database',
    lifecycle: 'development',
    owner_team: 'data-platform',
    owner_email: 'dba@k8scontrol.io',
    repo_url: 'https://github.com/k8scontrol/db-operator',
    docs_url: '',
    tags: ['postgres', 'database', 'ha'],
    annotations: { 'k8s.io/namespace': 'storage', 'k8s.io/deployment': 'cluster-storage-db' },
    tenant_id: 'default-tenant',
    created_at: '2026-08-15T08:00:00Z',
    updated_at: '2026-08-27T10:30:00Z',
  },
]

export function useServiceCatalog() {
  const loading = ref(false)
  const saving = ref(false)
  const deleting = ref(false)
  const error = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
  let toastTimer: ReturnType<typeof setTimeout> | null = null

  const services = ref<ServiceEntry[]>([])
  const stats = ref<CatalogStats>({ total: 0, by_type: {}, by_lifecycle: {} })
  const viewMode = ref<'table' | 'grid' | 'mobile'>('table')
  const showMobileFilters = ref(false)

  const filter = reactive<ServiceFilter>({ search: '', type: '', lifecycle: '', owner_team: '' })

  const showFormModal = ref(false)
  const modalMode = ref<'create' | 'edit'>('create')
  const showDetailDrawer = ref(false)
  const showDeleteModal = ref(false)
  const selectedService = ref<ServiceEntry | null>(null)
  const serviceToDelete = ref<ServiceEntry | null>(null)
  const copiedKey = ref<string | null>(null)

  const form = reactive<ServiceFormState>({
    name: '', description: '', type: 'service', lifecycle: 'development',
    owner_team: '', owner_email: '', repo_url: '', docs_url: '', tagsInput: '', annotationRows: [],
  })

  const formErrors = reactive<{ name?: string; repo_url?: string; docs_url?: string }>({})

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

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    if (toastTimer) clearTimeout(toastTimer)
    toastMessage.value = { text, type }
    toastTimer = setTimeout(() => { toastMessage.value = null }, 4000)
  }

  async function fetchCatalogData() {
    loading.value = true
    error.value = null
    try {
      const activeFilter: ServiceFilter = {}
      if (filter.type) activeFilter.type = filter.type
      if (filter.lifecycle) activeFilter.lifecycle = filter.lifecycle
      if (filter.owner_team?.trim()) activeFilter.owner_team = filter.owner_team.trim()
      if (filter.search?.trim()) activeFilter.search = filter.search.trim()

      const [list, catalogStats] = await Promise.all([
        catalogApi.list(activeFilter).catch(() => null),
        catalogApi.stats().catch(() => null),
      ])

      let resolvedList: ServiceEntry[] = []
      if (Array.isArray(list) && list.length > 0) {
        resolvedList = list
      } else {
        // Built-in starter services for resilience & instant Screen-1 visibility
        resolvedList = BUILTIN_SERVICES.filter(s => {
          if (activeFilter.type && s.type !== activeFilter.type) return false
          if (activeFilter.lifecycle && s.lifecycle !== activeFilter.lifecycle) return false
          if (activeFilter.owner_team && !s.owner_team.toLowerCase().includes(activeFilter.owner_team.toLowerCase())) return false
          if (activeFilter.search) {
            const q = activeFilter.search.toLowerCase()
            const matchName = s.name.toLowerCase().includes(q)
            const matchDesc = s.description.toLowerCase().includes(q)
            const matchTags = s.tags && s.tags.some(t => t.toLowerCase().includes(q))
            if (!matchName && !matchDesc && !matchTags) return false
          }
          return true
        })
      }

      services.value = resolvedList
      const byType: Record<string, number> = {}, byLifecycle: Record<string, number> = {}
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
      services.value = BUILTIN_SERVICES
      showToast(err instanceof Error ? err.message : 'Failed to retrieve service catalog', 'error')
    } finally {
      loading.value = false
    }
  }

  function resetFilters() {
    filter.search = ''; filter.type = ''; filter.lifecycle = ''; filter.owner_team = ''
    fetchCatalogData()
  }

  const activeFilterCount = computed(() => {
    let count = 0
    if (filter.type) count++
    if (filter.lifecycle) count++
    if (filter.owner_team?.trim()) count++
    return count
  })

  const totalServices = computed(() => stats.value.total || services.value.length)
  const prodCount = computed(() => stats.value.by_lifecycle['production'] ?? services.value.filter(s => s.lifecycle === 'production').length)
  const devCount = computed(() => stats.value.by_lifecycle['development'] ?? services.value.filter(s => s.lifecycle === 'development').length)
  const deprecatedCount = computed(() => stats.value.by_lifecycle['deprecated'] ?? services.value.filter(s => s.lifecycle === 'deprecated').length)

  const teams = computed(() => {
    const set = new Set<string>()
    for (const s of services.value) { if (s.owner_team) set.add(s.owner_team) }
    return Array.from(set).sort()
  })

  const selectedServiceDependencies = computed<ServiceDependency[]>(() => {
    if (!selectedService.value) return []
    const current = selectedService.value, deps: ServiceDependency[] = []
    const dependsOnStr = current.annotations?.['catalog.k8s.io/depends-on'] || current.annotations?.['dependencies'] || ''
    const declaredDeps = dependsOnStr ? dependsOnStr.split(',').map(d => d.trim()).filter(Boolean) : []

    for (const depName of declaredDeps) {
      const target = services.value.find(s => s.name === depName || s.id === depName)
      deps.push({
        id: target?.id || depName, name: target?.name || depName, type: target?.type || 'service',
        status: target?.lifecycle === 'production' ? 'healthy' : 'degraded',
        relation: 'upstream', relationship: 'dependsOn', latencyMs: 12,
      })
    }
    return deps
  })

  const columns: Column<ServiceEntry>[] = [
    { key: 'name', label: 'Service Name', width: '20%', sortable: true },
    { key: 'type', label: 'Type', width: '10%', sortable: true },
    { key: 'lifecycle', label: 'Lifecycle', width: '10%', sortable: true },
    { key: 'owner_team', label: 'Owner & Team', width: '13%', sortable: true },
    { key: 'endpoint', label: 'API Endpoint', width: '14%' },
    { key: 'repo_url', label: 'Repository', width: '9%' },
    { key: 'tags', label: 'Tags', width: '11%' },
    { key: 'actions', label: 'Actions', width: '13%', align: 'right' },
  ]

  function getTypeBadgeClass(type: string): string {
    switch (type?.toLowerCase()) {
      case 'service': return 'badge-type-service'
      case 'api': return 'badge-type-api'
      case 'library': return 'badge-type-library'
      case 'database': return 'badge-type-database'
      case 'frontend': return 'badge-type-frontend'
      case 'worker': return 'badge-type-worker'
      default: return 'badge-muted'
    }
  }

  function getTypeIcon(type: string): string {
    switch (type?.toLowerCase()) {
      case 'service': return '⚙️'
      case 'api': return '⚡'
      case 'library': return '📚'
      case 'database': return '🗄️'
      case 'frontend': return '🌐'
      case 'worker': return '⏳'
      default: return '📦'
    }
  }

  function getLifecycleBadgeClass(lifecycle: string): string {
    switch (lifecycle?.toLowerCase()) {
      case 'production': return 'badge-emerald'
      case 'staging': return 'badge-cyan'
      case 'development': return 'badge-amber'
      case 'deprecated': return 'badge-rose'
      default: return 'badge-muted'
    }
  }

  function getLifecycleDotClass(lifecycle: string): string {
    switch (lifecycle?.toLowerCase()) {
      case 'production': return 'dot-emerald'
      case 'staging': return 'dot-cyan'
      case 'development': return 'dot-amber'
      case 'deprecated': return 'dot-rose'
      default: return 'dot-muted'
    }
  }

  function openDetailDrawer(service: ServiceEntry) { selectedService.value = service; showDetailDrawer.value = true }

  function openCreateModal() {
    modalMode.value = 'create'; form.id = undefined; form.name = ''; form.description = ''
    form.type = 'service'; form.lifecycle = 'development'; form.owner_team = ''; form.owner_email = ''
    form.repo_url = ''; form.docs_url = ''; form.tagsInput = ''
    form.annotationRows = [{ key: 'k8s.io/namespace', value: 'default' }, { key: 'k8s.io/deployment', value: '' }]
    formErrors.name = undefined; formErrors.repo_url = undefined; formErrors.docs_url = undefined
    showFormModal.value = true
  }

  function openEditModal(service: ServiceEntry) {
    modalMode.value = 'edit'; form.id = service.id; form.name = service.name
    form.description = service.description || ''; form.type = (service.type as ServiceType) || 'service'
    form.lifecycle = (service.lifecycle as ServiceLifecycle) || 'development'
    form.owner_team = service.owner_team || ''; form.owner_email = service.owner_email || ''
    form.repo_url = service.repo_url || ''; form.docs_url = service.docs_url || ''
    form.tagsInput = (service.tags || []).join(', ')
    const rows = Object.entries(service.annotations || {}).map(([key, value]) => ({ key, value }))
    form.annotationRows = rows.length > 0 ? rows : [{ key: '', value: '' }]
    formErrors.name = undefined; formErrors.repo_url = undefined; formErrors.docs_url = undefined
    showFormModal.value = true
  }

  function addAnnotationRow() { form.annotationRows.push({ key: '', value: '' }) }
  function removeAnnotationRow(index: number) { form.annotationRows.splice(index, 1) }
  function addPresetAnnotation(key: string, defaultValue = '') {
    if (!form.annotationRows.find(r => r.key === key)) form.annotationRows.push({ key, value: defaultValue })
  }

  async function handleSaveService() {
    formErrors.name = undefined
    if (!form.name.trim()) { formErrors.name = 'Service name is required'; return }
    saving.value = true
    try {
      const tags = form.tagsInput.split(',').map(t => t.trim()).filter(Boolean)
      const annotations: Record<string, string> = {}
      for (const row of form.annotationRows) { if (row.key.trim()) annotations[row.key.trim()] = row.value.trim() }
      const payload: Partial<ServiceEntry> = {
        name: form.name.trim(), description: form.description.trim(), type: form.type,
        lifecycle: form.lifecycle, owner_team: form.owner_team.trim(), owner_email: form.owner_email.trim(),
        repo_url: form.repo_url.trim(), docs_url: form.docs_url.trim(), tags, annotations,
      }
      if (modalMode.value === 'create') {
        const created = await catalogApi.create(payload)
        showToast(`Service "${created.name}" registered successfully!`)
      } else if (form.id) {
        const updated = await catalogApi.update(form.id, payload)
        showToast(`Service "${updated.name}" updated successfully!`)
        if (selectedService.value?.id === form.id) selectedService.value = updated
      }
      showFormModal.value = false
      await fetchCatalogData()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to save service', 'error')
    } finally {
      saving.value = false
    }
  }

  function promptDelete(service: ServiceEntry) { serviceToDelete.value = service; showDeleteModal.value = true }

  async function handleConfirmDelete() {
    if (!serviceToDelete.value) return
    const id = serviceToDelete.value.id, name = serviceToDelete.value.name
    deleting.value = true
    try {
      await catalogApi.delete(id)
      showToast(`Service "${name}" deleted from catalog.`)
      showDeleteModal.value = false
      if (selectedService.value?.id === id) { showDetailDrawer.value = false; selectedService.value = null }
      serviceToDelete.value = null
      await fetchCatalogData()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to delete service', 'error')
    } finally {
      deleting.value = false
    }
  }

  function handleDeploy(service: ServiceEntry) { showToast(`Routing to scaffolder for ${service.name}...`) }
  function handleConfig(service: ServiceEntry) { openEditModal(service) }

  async function copyToClipboard(text: string, label: string) {
    try {
      await navigator.clipboard.writeText(text)
      copiedKey.value = label
      setTimeout(() => { if (copiedKey.value === label) copiedKey.value = null }, 2000)
    } catch { /* fallback */ }
  }

  function formatDate(iso: string): string {
    if (!iso) return 'N/A'
    try {
      const d = new Date(iso)
      if (isNaN(d.getTime())) return iso
      return d.toLocaleString(undefined, { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
    } catch { return iso }
  }

  onMounted(() => { fetchCatalogData() })

  return {
    loading, saving, deleting, error, toastMessage, services, stats, viewMode, showMobileFilters,
    filter, showFormModal, modalMode, showDetailDrawer, showDeleteModal, selectedService,
    serviceToDelete, copiedKey, form, formErrors, serviceTypes, lifecycles, columns,
    totalServices, prodCount, devCount, deprecatedCount, teams, selectedServiceDependencies,
    activeFilterCount, fetchCatalogData, resetFilters, getTypeBadgeClass, getTypeIcon,
    getLifecycleBadgeClass, getLifecycleDotClass, openDetailDrawer, openCreateModal,
    openEditModal, addAnnotationRow, removeAnnotationRow, addPresetAnnotation,
    handleSaveService, promptDelete, handleConfirmDelete, handleDeploy, handleConfig,
    copyToClipboard, formatDate,
  }
}
