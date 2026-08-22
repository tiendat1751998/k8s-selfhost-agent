<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  promotionsApi,
  dockerApi,
  type Promotion,
  type Environment,
  type CreatePromotionPayload,
  type ComputeHost
} from '../api/compute'
import { useAuthStore } from '../stores/authStore'

const authStore = useAuthStore()

export interface RunningServiceOption {
  id: string
  name: string
  image: string
  type: 'swarm' | 'container' | 'k8s'
  host: string // e.g. 'k8smater (10.10.10.133)' or compute host name & IP
  hostEndpoint?: string // e.g. '10.10.10.133'
  status?: string
  replicas?: number
  ports?: string[]
}

const loading = ref(false)
const loadingServices = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const promotions = ref<Promotion[]>([])
const runningServices = ref<RunningServiceOption[]>([])

// Searchable Combobox State
const serviceSearchQuery = ref('')
const isServiceDropdownOpen = ref(false)
const comboboxRef = ref<HTMLElement | null>(null)

function getDefaultRequester(): string {
  return authStore.user?.email || authStore.user?.role || 'admin'
}

// Create Promotion Modal
const showCreateModal = ref(false)
const newPromotion = ref<CreatePromotionPayload>({
  service: '',
  version: '',
  from_env: 'dev',
  to_env: 'qa',
  requester: getDefaultRequester()
})

const selectedService = computed(() => {
  return runningServices.value.find(s => s.name === newPromotion.value.service)
})

const filteredServices = computed(() => {
  const q = serviceSearchQuery.value.trim().toLowerCase()
  if (!q) return runningServices.value
  return runningServices.value.filter(s =>
    s.name.toLowerCase().includes(q) ||
    s.image.toLowerCase().includes(q) ||
    s.host.toLowerCase().includes(q) ||
    s.type.toLowerCase().includes(q) ||
    (s.hostEndpoint && s.hostEndpoint.toLowerCase().includes(q)) ||
    (s.ports && s.ports.some(p => p.toLowerCase().includes(q)))
  )
})

function selectService(svc: RunningServiceOption) {
  newPromotion.value.service = svc.name
  serviceSearchQuery.value = svc.name
  isServiceDropdownOpen.value = false
}

function clearServiceSearch() {
  serviceSearchQuery.value = ''
  newPromotion.value.service = ''
  isServiceDropdownOpen.value = true
}

function onSearchInput() {
  isServiceDropdownOpen.value = true
  const q = serviceSearchQuery.value.trim().toLowerCase()
  const matched = runningServices.value.find(s => s.name.toLowerCase() === q)
  if (matched) {
    newPromotion.value.service = matched.name
  } else {
    newPromotion.value.service = serviceSearchQuery.value.trim()
  }
}

function onSearchFocus() {
  isServiceDropdownOpen.value = true
}

function handleClickOutside(e: MouseEvent) {
  if (comboboxRef.value && !comboboxRef.value.contains(e.target as Node)) {
    isServiceDropdownOpen.value = false
  }
}

async function fetchPromotions() {
  loading.value = true
  error.value = null
  try {
    const res = await promotionsApi.list()
    promotions.value = Array.isArray(res.data) ? res.data : []
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve promotions'
    error.value = msg
    promotions.value = []
  } finally {
    loading.value = false
  }
}

async function loadServices() {
  loadingServices.value = true
  try {
    const [svcRes, contRes, hostsRes] = await Promise.allSettled([
      dockerApi.getServices(),
      dockerApi.getContainers(),
      dockerApi.listHosts()
    ])

    const hostsList: ComputeHost[] = (hostsRes.status === 'fulfilled' && Array.isArray(hostsRes.value))
      ? hostsRes.value
      : []

    // Format host name and endpoint helper
    const formatHostDisplay = (host: ComputeHost): string => {
      const cleanEndpoint = (host.endpoint || '').replace(/^tcp:\/\/|^https?:\/\//, '').replace(/:\d+$/, '')
      return cleanEndpoint ? `${host.name} (${cleanEndpoint})` : (host.name || 'Unknown Host')
    }

    const primaryHost = hostsList.find(h => h.host_type === 'docker' || h.status === 'connected') || hostsList[0]
    const defaultHostDisplay = primaryHost ? formatHostDisplay(primaryHost) : 'k8smater (10.10.10.133)'
    const defaultHostEndpoint = primaryHost?.endpoint ? primaryHost.endpoint.replace(/^tcp:\/\/|^https?:\/\//, '').replace(/:\d+$/, '') : '10.10.10.133'

    const servicesList: RunningServiceOption[] = []
    const seenNames = new Set<string>()

    // 1. Live Docker Swarm services (e.g. tiki_traefik, tiki_redis, my-nginx)
    const swarmNames: string[] = []
    if (svcRes.status === 'fulfilled' && Array.isArray(svcRes.value)) {
      for (const s of svcRes.value) {
        if (s.name && !seenNames.has(s.name)) {
          seenNames.add(s.name)
          swarmNames.push(s.name)
          servicesList.push({
            id: s.id || s.name,
            name: s.name,
            image: s.image || 'unknown:latest',
            type: 'swarm',
            host: `${defaultHostDisplay} [Swarm Cluster]`,
            hostEndpoint: defaultHostEndpoint,
            status: (s.replicas ?? 0) > 0 ? 'running' : 'stopped',
            replicas: s.replicas ?? 1,
            ports: Array.isArray(s.ports) ? s.ports : []
          })
        }
      }
    }

    // 2. Live Standalone Docker containers (e.g. postgres_db, nats, registry)
    if (contRes.status === 'fulfilled' && Array.isArray(contRes.value)) {
      for (const c of contRes.value) {
        const cleanName = (c.name || '').replace(/^\//, '')
        if (!cleanName) continue
        // Skip swarm replica tasks (e.g. tiki_traefik.1.xxxx, my-nginx.1.xxx)
        const isSwarmTask = swarmNames.some(sName =>
          cleanName.startsWith(sName + '.') || cleanName.startsWith(sName + '_')
        )
        if (isSwarmTask || seenNames.has(cleanName)) continue

        seenNames.add(cleanName)

        // Parse ports from container
        let portsList: string[] = []
        if (Array.isArray((c as any).ports)) {
          portsList = (c as any).ports.map((p: any) =>
            typeof p === 'string' ? p : `${p.PublicPort || p.public_port || ''}:${p.PrivatePort || p.private_port || ''}/${p.Type || p.type || 'tcp'}`
          ).filter(Boolean)
        } else if (typeof (c as any).ports === 'string' && (c as any).ports) {
          portsList = [(c as any).ports]
        }

        const isRunning = (c.state || '').toLowerCase() === 'running' || (c.status || '').toLowerCase().includes('up')

        servicesList.push({
          id: c.id || cleanName,
          name: cleanName,
          image: c.image || 'unknown:latest',
          type: 'container',
          host: defaultHostDisplay,
          hostEndpoint: defaultHostEndpoint,
          status: c.state || c.status || 'running',
          replicas: isRunning ? 1 : 0,
          ports: portsList
        })
      }
    }

    runningServices.value = servicesList

    // Set initial selection if available
    if (servicesList.length > 0 && !newPromotion.value.service) {
      newPromotion.value.service = servicesList[0].name
      serviceSearchQuery.value = servicesList[0].name
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load running services'
    console.error('Failed to load active services:', msg)
  } finally {
    loadingServices.value = false
  }
}

async function refreshAll() {
  await Promise.all([
    fetchPromotions(),
    loadServices()
  ])
}

function openCreateModal() {
  if (runningServices.value.length > 0 && !newPromotion.value.service) {
    newPromotion.value.service = runningServices.value[0].name
  }
  if (newPromotion.value.service) {
    serviceSearchQuery.value = newPromotion.value.service
  }
  isServiceDropdownOpen.value = false
  newPromotion.value.requester = getDefaultRequester()
  showCreateModal.value = true
}

onMounted(async () => {
  document.addEventListener('click', handleClickOutside)
  await refreshAll()
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

const pendingCount = computed(() => promotions.value.filter(p => p.status === 'pending').length)
const approvedCount = computed(() => promotions.value.filter(p => p.status === 'approved' || p.status === 'promoting').length)
const completedCount = computed(() => promotions.value.filter(p => p.status === 'completed').length)
const rejectedCount = computed(() => promotions.value.filter(p => p.status === 'rejected' || p.status === 'failed').length)

// Pipeline Board Stages
const environments: Environment[] = ['dev', 'qa', 'staging', 'production']

function getPromotionsForEnv(env: Environment) {
  return promotions.value.filter(p => p.to_env === env)
}

const columns: Column<Promotion>[] = [
  { key: 'service', label: 'Service Name', sortable: true },
  { key: 'version', label: 'Target Version', width: '130px', sortable: true },
  { key: 'from_env', label: 'Source', width: '110px', sortable: true },
  { key: 'to_env', label: 'Destination', width: '110px', sortable: true },
  { key: 'requester', label: 'Requested By', width: '140px', sortable: true },
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'created_at', label: 'Requested At', width: '160px', sortable: true },
  { key: 'actions', label: 'Approvals', width: '220px', align: 'right' },
]

function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

async function handleCreatePromotion() {
  if (!newPromotion.value.service || !newPromotion.value.version) {
    showToast('Service name and target version are required', 'error')
    return
  }
  if (newPromotion.value.from_env === newPromotion.value.to_env) {
    showToast('Source and destination environments must differ', 'error')
    return
  }

  actionLoading.value = 'create'
  try {
    await promotionsApi.create(newPromotion.value)
    showToast(`Promotion request for ${newPromotion.value.service} created!`)
    showCreateModal.value = false
    newPromotion.value.version = ''
    if (runningServices.value.length > 0) {
      newPromotion.value.service = runningServices.value[0].name
      serviceSearchQuery.value = runningServices.value[0].name
    } else {
      newPromotion.value.service = ''
      serviceSearchQuery.value = ''
    }
    await fetchPromotions()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Promotion request failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleApprove(p: Promotion) {
  actionLoading.value = p.id
  try {
    await promotionsApi.approve(p.id)
    showToast(`Promotion for ${p.service} approved! Ready for deployment.`)
    await fetchPromotions()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Approval failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleReject(p: Promotion) {
  actionLoading.value = p.id
  try {
    await promotionsApi.reject(p.id)
    showToast(`Promotion for ${p.service} rejected.`)
    await fetchPromotions()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Rejection failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleComplete(p: Promotion) {
  actionLoading.value = p.id
  try {
    await promotionsApi.complete(p.id)
    showToast(`Promotion for ${p.service} completed successfully in ${p.to_env}!`)
    await fetchPromotions()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Completion failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

function formatDate(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>PROGRESSIVE DELIVERY & GOVERNANCE PIPELINE</span>
        </div>
        <h1 class="view-title">Multi-Stage Release Promotions</h1>
        <p class="view-desc">
          Automated gate approvals and progressive environment promotion pipeline across Dev ➔ QA ➔ Staging ➔ Production.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading || loadingServices" @click="refreshAll">
          <span>{{ loading || loadingServices ? '⏳ Querying...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="openCreateModal">
          <span>+ Request Promotion</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- Metric HUD -->
    <div class="metrics-grid">
      <MetricCard
        title="Pending Approvals"
        :value="pendingCount"
        subtitle="Promotion requests awaiting review"
        icon="⏳"
        badge="GATE"
        :badge-color="pendingCount > 0 ? 'amber' : 'emerald'"
        :trend="pendingCount > 0 ? 'Review Required' : 'All Clear'"
        :trend-type="pendingCount > 0 ? 'neutral' : 'positive'"
      />
      <MetricCard
        title="Active Promoting"
        :value="approvedCount"
        subtitle="Canary rollout & staging verification"
        icon="🚀"
        badge="ROLLOUT"
        badge-color="cyan"
        trend="In-Flight Verification"
        trend-type="positive"
      />
      <MetricCard
        title="Completed Releases"
        :value="completedCount"
        subtitle="Successfully promoted to destination"
        icon="✅"
        badge="SHIPPED"
        badge-color="emerald"
        trend="Continuous Delivery"
        trend-type="positive"
      />
      <MetricCard
        title="Rejected / Aborted"
        :value="rejectedCount"
        subtitle="Failed quality gates or security review"
        icon="🛑"
        badge="REJECTED"
        :badge-color="rejectedCount > 0 ? 'rose' : 'emerald'"
      />
    </div>

    <!-- Visual Pipeline Board (Dev ➔ QA ➔ Staging ➔ Prod) -->
    <div class="section-box glass-panel">
      <div class="box-header">
        <div>
          <h2 class="box-title">Visual Release Pipeline Board</h2>
          <p class="box-subtitle">Progressive gate stages with active release artifacts</p>
        </div>
      </div>

      <div class="pipeline-board">
        <div v-for="env in environments" :key="env" class="pipeline-column glass-panel">
          <div class="column-header">
            <div class="stage-badge" :class="`stage-${env}`">
              {{ env.toUpperCase() }}
            </div>
            <span class="stage-count font-mono">{{ getPromotionsForEnv(env).length }} Releases</span>
          </div>

          <div class="column-body">
            <div v-if="getPromotionsForEnv(env).length === 0" class="stage-empty">
              <span>No promotion requests found for {{ env }}. Click '+ Request Promotion' to submit a release.</span>
            </div>

            <div 
              v-for="p in getPromotionsForEnv(env)" 
              :key="p.id" 
              class="promotion-card glass-panel"
              :class="`p-status-${p.status}`"
            >
              <div class="p-card-top">
                <span class="p-service">{{ p.service }}</span>
                <StatusBadge :status="p.status" size="sm" />
              </div>

              <div class="p-version font-mono">
                <span>{{ p.from_env }} ➔ </span>
                <span class="text-cyan">{{ p.version }}</span>
              </div>

              <div class="p-meta font-mono text-muted">
                <span>By: {{ p.requester }}</span>
              </div>

              <!-- Quick Actions -->
              <div v-if="p.status === 'pending'" class="p-card-actions">
                <button class="btn btn-secondary btn-xs" @click="handleReject(p)">✕ Reject</button>
                <button class="btn btn-primary btn-xs" @click="handleApprove(p)">✓ Approve</button>
              </div>
              <div v-else-if="p.status === 'approved' || p.status === 'promoting'" class="p-card-actions">
                <button class="btn btn-primary btn-xs" @click="handleComplete(p)">Complete Rollout ➔</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Promotions History Data Table -->
    <div class="section-box glass-panel table-box">
      <div class="box-header" style="padding: 18px 22px; border-bottom: 1px solid var(--border-subtle);">
        <div>
          <h2 class="box-title">Promotion Request Audit Table</h2>
          <p class="box-subtitle">Full historical ledger of promotion approvals and audit trail</p>
        </div>
      </div>

      <DataTable
        :columns="columns"
        :data="promotions"
        :loading="loading"
        :error="error"
        empty-message="No promotion requests found. Click '+ Request Promotion' to submit a new release for environment gating."
        searchable
        search-placeholder="Search promotions by service, requester, or version..."
      >
        <template #cell-service="{ row }">
          <span class="font-mono text-cyan" style="font-weight: 700;">{{ row.service }}</span>
        </template>

        <template #cell-version="{ row }">
          <span class="font-mono text-emerald">{{ row.version }}</span>
        </template>

        <template #cell-from_env="{ row }">
          <span class="env-pill-tag font-mono">{{ row.from_env }}</span>
        </template>

        <template #cell-to_env="{ row }">
          <span class="env-pill-tag font-mono text-cyan">{{ row.to_env }}</span>
        </template>

        <template #cell-status="{ row }">
          <StatusBadge :status="row.status" size="sm" />
        </template>

        <template #cell-created_at="{ row }">
          <span class="font-mono text-muted">{{ formatDate(row.created_at) }}</span>
        </template>

        <template #cell-actions="{ row }">
          <div class="table-actions-row">
            <button 
              v-if="row.status === 'pending'"
              class="btn btn-secondary btn-xs" 
              :disabled="actionLoading === row.id"
              @click="handleReject(row)"
            >
              <span>Reject</span>
            </button>
            <button 
              v-if="row.status === 'pending'"
              class="btn btn-primary btn-xs" 
              :disabled="actionLoading === row.id"
              @click="handleApprove(row)"
            >
              <span>Approve</span>
            </button>
            <button 
              v-if="row.status === 'approved' || row.status === 'promoting'"
              class="btn btn-primary btn-xs" 
              :disabled="actionLoading === row.id"
              @click="handleComplete(row)"
            >
              <span>Complete</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Request Promotion Modal -->
    <ModalDrawer
      v-model:show="showCreateModal"
      mode="modal"
      title="Request Environment Promotion"
      subtitle="Submit a workload version promotion request for gatekeeper review"
      max-width="560px"
    >
      <div class="modal-form">
        <div class="form-group">
          <div class="form-label-row">
            <label class="form-label">Service Name</label>
            <span v-if="runningServices.length > 0" class="service-count-pill font-mono">
              {{ runningServices.length }} Workload{{ runningServices.length === 1 ? '' : 's' }} Available
            </span>
          </div>

          <div v-if="loadingServices" class="input-loading-hint font-mono">
            <span>⏳ Fetching running services & host placement topology...</span>
          </div>

          <!-- Searchable Interactive Service Combobox -->
          <div v-else ref="comboboxRef" class="combobox-wrapper">
            <div class="combobox-input-box">
              <input 
                v-model="serviceSearchQuery" 
                type="text" 
                class="input-glass combobox-input font-mono"
                placeholder="🔍 Type to search service name, image, or host server..."
                autocomplete="off"
                @focus="onSearchFocus"
                @input="onSearchInput"
                @keydown.escape="isServiceDropdownOpen = false"
              />
              <button
                v-if="serviceSearchQuery"
                type="button"
                class="combobox-btn combobox-clear"
                title="Clear selection"
                @click.stop="clearServiceSearch"
              >
                ✕
              </button>
              <button
                type="button"
                class="combobox-btn combobox-toggle"
                title="Toggle service list"
                @click.stop="isServiceDropdownOpen = !isServiceDropdownOpen"
              >
                <span class="dropdown-chevron" :class="{ 'chevron-open': isServiceDropdownOpen }">▾</span>
              </button>
            </div>

            <!-- Real-time Filtered Workloads Dropdown -->
            <div 
              v-if="isServiceDropdownOpen" 
              class="combobox-dropdown glass-panel"
            >
              <div v-if="filteredServices.length === 0" class="combobox-empty font-mono">
                <span>⚠️ No workloads matching "{{ serviceSearchQuery }}" found</span>
              </div>

              <div
                v-for="svc in filteredServices"
                :key="svc.id + '-' + svc.name"
                class="combobox-option"
                :class="{ 'is-selected': svc.name === newPromotion.service }"
                @click="selectService(svc)"
              >
                <!-- Line 1: Workload Name + Type Badge + Status indicator (🟢 Replicas) -->
                <div class="option-line-1">
                  <span class="option-name font-mono">{{ svc.name }}</span>
                  <div class="option-tags">
                    <span 
                      class="workload-type-badge font-mono"
                      :class="svc.type === 'swarm' ? 'badge-swarm' : (svc.type === 'k8s' ? 'badge-k8s' : 'badge-docker')"
                    >
                      {{ svc.type === 'swarm' ? 'Swarm' : (svc.type === 'k8s' ? 'K8s' : 'Container') }}
                    </span>
                    <span 
                      class="option-status-pill font-mono"
                      :class="svc.status === 'running' || (svc.replicas ?? 0) > 0 ? 'status-active' : 'status-inactive'"
                    >
                      <span class="status-dot"></span>
                      <span>{{ svc.replicas ?? 1 }} Rep</span>
                    </span>
                  </div>
                </div>

                <!-- Line 2: 🖥️ Server: {host} | 📦 Image: {image} | 🔌 Ports: {ports} -->
                <div class="option-line-2 font-mono">
                  <span class="option-meta-part">
                    <span class="meta-label">🖥️ Server:</span>
                    <span class="meta-val text-host">{{ svc.host }}</span>
                  </span>
                  <span class="meta-bar">|</span>
                  <span class="option-meta-part">
                    <span class="meta-label">📦 Image:</span>
                    <span class="meta-val text-cyan">{{ svc.image }}</span>
                  </span>
                  <span class="meta-bar">|</span>
                  <span class="option-meta-part">
                    <span class="meta-label">🔌 Ports:</span>
                    <span class="meta-val text-amber">{{ svc.ports && svc.ports.length > 0 ? svc.ports.join(', ') : 'None' }}</span>
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Upgraded Currently Running Image & Placement Preview Card -->
        <div v-if="selectedService" class="placement-preview-card glass-panel">
          <div class="placement-header">
            <div class="placement-title font-mono">
              <span class="placement-icon">🌐</span>
              <span>WORKLOAD TOPOLOGY & PLACEMENT</span>
            </div>
            <span class="active-badge font-mono" :class="selectedService.type === 'swarm' ? 'badge-swarm' : (selectedService.type === 'k8s' ? 'badge-k8s' : 'badge-docker')">
              {{ selectedService.type === 'swarm' ? 'Docker Swarm Service' : (selectedService.type === 'k8s' ? 'Kubernetes Workload' : 'Standalone Container') }}
            </span>
          </div>

          <div class="placement-details-grid font-mono">
            <div class="placement-grid-row">
              <span class="placement-row-key">🖥️ Host Server:</span>
              <span class="placement-row-val text-host font-bold">{{ selectedService.host }}</span>
            </div>

            <div class="placement-grid-row">
              <span class="placement-row-key">📦 Active Running Image:</span>
              <span class="placement-row-val text-cyan font-bold">{{ selectedService.image }}</span>
            </div>

            <div class="placement-grid-row">
              <span class="placement-row-key">⚙️ Replicas & Ports:</span>
              <span class="placement-row-val">
                <span class="text-emerald font-bold">{{ selectedService.replicas ?? 1 }} Replica(s)</span>
                <span class="placement-sep">•</span>
                <span class="text-muted">Ports:</span>
                <span class="text-amber">{{ selectedService.ports && selectedService.ports.length > 0 ? selectedService.ports.join(', ') : 'None' }}</span>
              </span>
            </div>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Target Release Version / Image Tag</label>
          <input 
            v-model="newPromotion.version" 
            type="text" 
            :placeholder="selectedService ? `e.g. ${selectedService.image}` : 'e.g. redis:8-alpine, nginx:1.27-alpine'" 
            class="input-glass font-mono" 
          />
          <span class="form-hint">Specify the target Docker image tag or release version to promote</span>
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Source Stage</label>
            <select v-model="newPromotion.from_env" class="input-glass">
              <option value="dev">DEV (Development)</option>
              <option value="qa">QA (Automated Tests)</option>
              <option value="staging">STAGING (Pre-Prod)</option>
            </select>
          </div>

          <div class="form-group flex-1">
            <label class="form-label">Destination Stage</label>
            <select v-model="newPromotion.to_env" class="input-glass">
              <option value="qa">QA (Automated Tests)</option>
              <option value="staging">STAGING (Pre-Prod)</option>
              <option value="production">PRODUCTION (Live Users)</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Requester ID / Name</label>
          <input v-model="newPromotion.requester" type="text" class="input-glass font-mono" />
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button 
          class="btn btn-primary" 
          :disabled="actionLoading === 'create' || !newPromotion.service || !newPromotion.version" 
          @click="handleCreatePromotion"
        >
          <span>{{ actionLoading === 'create' ? 'Submitting...' : 'Submit Promotion Request ➔' }}</span>
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
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

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

.pipeline-board {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}

@media (max-width: 1024px) {
  .pipeline-board {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .pipeline-board {
    grid-template-columns: 1fr;
  }
}

.pipeline-column {
  background: rgba(11, 15, 25, 0.5);
  border-radius: 14px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 240px;
}

.column-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 10px;
}

.stage-badge {
  font-size: 11px;
  font-weight: 800;
  padding: 3px 8px;
  border-radius: 6px;
  letter-spacing: 0.05em;
}

.stage-dev { background: rgba(56, 189, 248, 0.15); color: #38bdf8; }
.stage-qa { background: rgba(139, 92, 246, 0.15); color: #a78bfa; }
.stage-staging { background: rgba(245, 158, 11, 0.15); color: #fbbf24; }
.stage-production { background: rgba(16, 185, 129, 0.15); color: #34d399; }

.stage-count {
  font-size: 11px;
  color: var(--text-muted);
}

.column-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
}

.stage-empty {
  padding: 24px 8px;
  text-align: center;
  font-size: 11px;
  color: var(--text-muted);
}

.promotion-card {
  padding: 12px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: rgba(15, 23, 42, 0.7);
}

.p-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.p-service {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.p-version {
  font-size: 11px;
}

.p-meta {
  font-size: 10px;
}

.p-card-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  margin-top: 4px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 6px;
}

.env-pill-tag {
  font-size: 11px;
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
}

.table-actions-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
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

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.font-mono { font-family: var(--font-mono); }
.font-bold { font-weight: 700; }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }

.form-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.service-count-pill {
  font-size: 10px;
  color: var(--accent-cyan);
  background: rgba(6, 182, 212, 0.12);
  border: 1px solid rgba(6, 182, 212, 0.25);
  padding: 1px 6px;
  border-radius: 4px;
}

.combobox-wrapper {
  position: relative;
  width: 100%;
}

.combobox-input-box {
  position: relative;
  display: flex;
  align-items: center;
  width: 100%;
}

.combobox-input {
  width: 100%;
  padding-right: 64px;
}

.combobox-btn {
  position: absolute;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px 6px;
  border-radius: 4px;
  transition: all 0.15s ease;
}

.combobox-btn:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.08);
}

.combobox-clear {
  right: 32px;
  font-size: 11px;
}

.combobox-toggle {
  right: 8px;
  font-size: 14px;
}

.dropdown-chevron {
  display: inline-block;
  transition: transform 0.2s ease;
}

.chevron-open {
  transform: rotate(180deg);
}

.combobox-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  max-height: 250px;
  overflow-y: auto;
  z-index: 60;
  background: rgba(11, 15, 25, 0.97);
  border: 1px solid var(--border-medium);
  border-radius: 10px;
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.7);
  padding: 4px;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.combobox-empty {
  padding: 16px 12px;
  text-align: center;
  font-size: 11px;
  color: var(--text-muted);
}

.combobox-option {
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s ease;
  border: 1px solid transparent;
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: rgba(15, 23, 42, 0.4);
}

.combobox-option:hover {
  background: rgba(56, 189, 248, 0.12);
  border-color: rgba(56, 189, 248, 0.3);
}

.combobox-option.is-selected {
  background: rgba(6, 182, 212, 0.16);
  border-color: rgba(6, 182, 212, 0.45);
}

.option-line-1 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.option-name {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.option-tags {
  display: flex;
  align-items: center;
  gap: 6px;
}

.workload-type-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
}

.option-status-pill {
  font-size: 10px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 6px;
  border-radius: 4px;
}

.status-active {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.status-inactive {
  background: rgba(148, 163, 184, 0.15);
  color: #94a3b8;
  border: 1px solid rgba(148, 163, 184, 0.3);
}

.status-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
}

.option-line-2 {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  font-size: 11px;
  color: var(--text-muted);
}

.option-meta-part {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.meta-label {
  color: var(--text-muted);
}

.meta-val {
  color: var(--text-secondary);
}

.meta-bar {
  color: rgba(255, 255, 255, 0.15);
  font-size: 10px;
}

.text-host {
  color: #38bdf8;
  font-weight: 600;
}

.text-amber {
  color: #fbbf24;
}

.active-badge {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 700;
}

.badge-swarm {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.3);
}

.badge-docker {
  background: rgba(168, 85, 247, 0.15);
  color: #c084fc;
  border: 1px solid rgba(168, 85, 247, 0.3);
}

.badge-k8s {
  background: rgba(59, 130, 246, 0.15);
  color: #60a5fa;
  border: 1px solid rgba(59, 130, 246, 0.3);
}

.placement-preview-card {
  padding: 12px 14px;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(56, 189, 248, 0.25);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.placement-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.placement-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.06em;
  display: flex;
  align-items: center;
  gap: 6px;
}

.placement-icon {
  font-size: 13px;
}

.placement-details-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 11px;
}

.placement-grid-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
  word-break: break-all;
}

.placement-row-key {
  color: var(--text-muted);
  font-weight: 600;
  min-width: 140px;
}

.placement-row-val {
  color: var(--text-primary);
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.placement-sep {
  color: rgba(255, 255, 255, 0.2);
}

.form-hint {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 2px;
}

.input-loading-hint {
  font-size: 12px;
  color: var(--text-muted);
  padding: 8px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
}
</style>
