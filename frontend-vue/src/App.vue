<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from './stores/authStore'
import { useBackupStore } from './stores/backupStore'
import { useSecurityStore } from './stores/securityStore'
import { useLogStore } from './stores/logStore'
import { useAppStore } from './stores/app'
import { useAlertStore } from './stores/alertStore'
import { api } from './api/client'
import { tenancyApi } from './api/management'
import { overviewApi } from './api/overview'
import ModalDrawer from './components/ui/ModalDrawer.vue'
import TopHudAlertBell from './components/layout/TopHudAlertBell.vue'
import AlertCenterModal from './components/overview/alerts/AlertCenterModal.vue'
import PwaInstallBanner from './components/common/PwaInstallBanner.vue'
import { usePwaInstall } from './registerServiceWorker'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const backupStore = useBackupStore()
const securityStore = useSecurityStore()
const logStore = useLogStore()
const appStore = useAppStore()
const alertStore = useAlertStore()
const { isInstallable, promptInstall } = usePwaInstall()

// State
const selectedTenant = ref('default-tenant')
const showCommandPalette = ref(false)
const searchQuery = ref('')
const mobileSidebarOpen = ref(false)

watch(() => route.path, () => {
  mobileSidebarOpen.value = false
})

// Collapsible Navigation Section States
const collapsedSections = ref<Record<string, boolean>>({
  observability: false,
  compute: false,
  governance: false,
  automation: false,
  management: false,
})

function toggleSection(sectionKey: string) {
  collapsedSections.value[sectionKey] = !collapsedSections.value[sectionKey]
}

// Tenancy
interface TenantOption {
  id: string
  name: string
}

const tenants = ref<TenantOption[]>([
  { id: 'default-tenant', name: 'default-tenant' }
])

async function loadTenants() {
  try {
    const orgs = await tenancyApi.getOrganizations()
    if (orgs && orgs.length > 0) {
      tenants.value = orgs.map(o => ({ id: o.id, name: o.name || o.id }))
      if (!tenants.value.some(t => t.id === selectedTenant.value)) {
        selectedTenant.value = tenants.value[0].id
      }
    } else {
      tenants.value = [{ id: 'default-tenant', name: 'default-tenant' }]
      selectedTenant.value = 'default-tenant'
    }
  } catch {
    tenants.value = [{ id: 'default-tenant', name: 'default-tenant' }]
    selectedTenant.value = 'default-tenant'
  }
  api.setTenantId(selectedTenant.value)
}

function handleTenantChange() {
  api.setTenantId(selectedTenant.value)
}

// 5 Structured Navigation Groups
const navGroups = [
  {
    key: 'observability',
    label: 'OBSERVABILITY & OPS',
    icon: '📊',
    items: [
      { path: '/', name: 'Overview', icon: '🌐', sub: 'Fleet Health & Mesh' },
      { path: '/incidents', name: 'Incidents & RCA', icon: '🚨', sub: 'Autonomous Triage' },
      { path: '/agents', name: 'AI SRE Agents', icon: '🤖', sub: 'Continuous Audit' },
      { path: '/slo', name: 'SLOs & Budgets', icon: '🎯', sub: 'PromQL SLI Tracking' },
      { path: '/logs', name: 'Real-Time Logs', icon: '📜', sub: 'Live WebSocket Stream', badge: 'LIVE' },
    ]
  },
  {
    key: 'compute',
    label: 'COMPUTE & FLEET',
    icon: '🌐',
    items: [
      { path: '/fleet', name: 'Fleet Clusters', icon: '☸️', sub: 'Multi-Region Mesh' },
      { path: '/hosts', name: 'Infrastructure Hosts', icon: '🖥️', sub: 'Compute & Database Nodes' },
      { path: '/deployments', name: 'Deployments & Apps', icon: '🚀', sub: 'Canary, Blue-Green & Workloads' },
      { path: '/promotions', name: 'Promotions', icon: '🔄', sub: 'Dev → Stage → Prod' },
      { path: '/explorer', name: 'Cluster Explorer', icon: '🔍', sub: 'Live CRD & Pod Trees' },
    ]
  },
  {
    key: 'governance',
    label: 'GOVERNANCE & SECURITY',
    icon: '🛡️',
    items: [
      { path: '/audit', name: 'Audit & CVEs', icon: '🛡️', sub: 'Trivy & IaC Scanner' },
      { path: '/compliance', name: 'Compliance & CIS', icon: '📋', sub: 'SOC2 / CIS Level 2' },
      { path: '/drift', name: 'Config Drift', icon: '⚡', sub: 'GitOps Reconciliation' },
      { path: '/backup', name: 'Disaster Recovery', icon: '📦', sub: 'Dual-Sync NVMe + S3' },
    ]
  },
  {
    key: 'automation',
    label: 'AUTOMATION & FINOPS',
    icon: '⚡',
    items: [
      { path: '/automation', name: 'Automation Rules', icon: '⚙️', sub: 'Event-Driven SRE' },
      { path: '/runbooks', name: 'SRE Runbooks', icon: '📖', sub: 'One-Click Remediation' },
      { path: '/cost', name: 'Cost Optimization', icon: '💰', sub: 'FinOps Node Bin-Packing' },
      { path: '/capacity', name: 'Capacity Forecast', icon: '📈', sub: 'ARIMA Time-Series' },
    ]
  },
  {
    key: 'management',
    label: 'MANAGEMENT',
    icon: '🏢',
    items: [
      { path: '/tenancy', name: 'Tenancy & RBAC', icon: '🏢', sub: 'Org Isolation Matrix' },
      { path: '/ai-hub', name: 'AI Provider Hub', icon: '🧠', sub: 'LLM Gateway & Breakers' },
      { path: '/changes', name: 'Change Requests', icon: '📝', sub: 'RFC Approvals & Windows' },
      { path: '/alerts', name: 'Alerts & Channels', icon: '🔥', sub: 'Slack, Telegram, Email' },
      { path: '/reports', name: 'Reports Center', icon: '📊', sub: 'Executive SOC2 & Cost' },
      { path: '/catalog', name: 'Service Catalog', icon: '📚', sub: 'Developer Portal & APIs' },
      { path: '/scaffolder', name: 'Scaffolder Templates', icon: '🪄', sub: '1-Click App Deployment' },
      { path: '/ecosystem', name: 'Ecosystem Tools', icon: '⚡', sub: 'Auto-Detected Stack' },
      { path: '/plugins', name: 'Plugin Hub', icon: '🧩', sub: 'JS Extension Runtime' },
      { path: '/settings', name: 'System Settings', icon: '🔧', sub: 'MetalLB & KEDA Nodes' },
    ]
  }
]

// All routes flattened for Command Palette Search
const allSearchableRoutes = computed(() => {
  const list: { path: string; name: string; icon: string; group: string; sub: string }[] = []
  for (const group of navGroups) {
    for (const item of group.items) {
      list.push({
        path: item.path,
        name: item.name,
        icon: item.icon,
        group: group.label,
        sub: item.sub,
      })
    }
  }
  return list
})

const filteredSearchResults = computed(() => {
  if (!searchQuery.value.trim()) return allSearchableRoutes.value
  const q = searchQuery.value.toLowerCase().trim()
  return allSearchableRoutes.value.filter(item =>
    item.name.toLowerCase().includes(q) ||
    item.group.toLowerCase().includes(q) ||
    item.sub.toLowerCase().includes(q) ||
    item.path.toLowerCase().includes(q)
  )
})

function navigateTo(path: string) {
  showCommandPalette.value = false
  searchQuery.value = ''
  router.push(path)
}

function handleGlobalKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    showCommandPalette.value = !showCommandPalette.value
  }
}

let globalTelemetryInterval: ReturnType<typeof setInterval> | null = null

async function syncGlobalTelemetry() {
  if (!authStore.isAuthenticated) return
  try {
    const data = await overviewApi.getOverview()
    if (data) {
      appStore.setMetrics(data)
      alertStore.syncAlerts(data.alerts || [])
    }
  } catch {
    // Background telemetry fallback
  }
}

onMounted(() => {
  if (authStore.isAuthenticated) {
    backupStore.fetchAll().catch(() => {})
    securityStore.fetchAll().catch(() => {})
    loadTenants().catch(() => {})
    syncGlobalTelemetry().catch(() => {})
  }
  if (typeof window !== 'undefined') {
    window.addEventListener('keydown', handleGlobalKeydown)
  }
  globalTelemetryInterval = setInterval(() => {
    if (authStore.isAuthenticated) {
      syncGlobalTelemetry().catch(() => {})
    }
  }, 10000)
})

onUnmounted(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('keydown', handleGlobalKeydown)
  }
  if (globalTelemetryInterval) {
    clearInterval(globalTelemetryInterval)
    globalTelemetryInterval = null
  }
})

const isLoginPage = computed(() => route.path === '/login')

// Mesh & Node Health Real-time Calculations
const downNodes = computed(() => {
  const nodes = appStore.latestMetrics?.nodes || []
  return nodes.filter(n => n.status?.toLowerCase() === 'down' || n.status?.toLowerCase() === 'offline')
})

const downNodeCount = computed(() => downNodes.value.length)

const clusterMeshStatus = computed(() => {
  if (downNodeCount.value > 0) return 'DEGRADED'
  if (securityStore.error || backupStore.error || alertStore.hasCriticalAlerts) return 'DEGRADED'
  return 'HEALTHY'
})

const meshTooltip = computed(() => {
  if (downNodeCount.value > 0) {
    const names = downNodes.value.map(n => n.node_name || n.node_id).join(', ')
    return `Mesh Status: DEGRADED — ${downNodeCount.value} node(s) offline (${names}). Cluster cross-node communication impaired.`
  }
  if (securityStore.error || backupStore.error) {
    return `Mesh Status: DEGRADED — Security audit or backup synchronization encountered error.`
  }
  if (alertStore.hasCriticalAlerts) {
    return `Mesh Status: DEGRADED — Critical resource alerts active in cluster.`
  }
  const total = appStore.latestMetrics?.total_nodes || appStore.latestMetrics?.nodes?.length || 0
  const healthy = appStore.latestMetrics?.healthy_nodes || total
  return `Mesh Status: HEALTHY — WireGuard/eBPF encrypted mesh operational across ${healthy}/${total || 'all'} connected nodes.`
})

// Latency & Stream Real-time Calculations
const streamStatusText = computed(() => {
  return logStore.isConnected ? '<50ms' : 'POLLING'
})

const latencyTooltip = computed(() => {
  if (logStore.isConnected) {
    return 'Telemetry Stream: Live WebSocket connection active (<50ms real-time stream latency). Continuous bidirectional events.'
  }
  return 'Telemetry Stream: HTTP 5s Polling Fallback active. WebSocket stream reconnecting in background.'
})

function handleLogout() {
  authStore.logout()
  router.push('/login')
}

function handleNavigateToHost(nodeNameOrId: string) {
  alertStore.closeAlertCenter()
  router.push({ path: '/hosts', query: { search: nodeNameOrId } })
}
</script>

<template>
  <div v-if="isLoginPage" class="login-container">
    <RouterView />
  </div>

  <div v-else class="app-layout">
    <!-- Backdrop for mobile drawer -->
    <div 
      v-if="mobileSidebarOpen" 
      class="sidebar-backdrop" 
      @click="mobileSidebarOpen = false"
    ></div>

    <!-- Enterprise Multi-Group Sidebar -->
    <aside class="sidebar" :class="{ 'sidebar-open': mobileSidebarOpen }">
      <!-- Brand Logo -->
      <div class="brand-container" @click="router.push('/')">
        <div class="brand-icon-wrapper">
          <div class="brand-icon">⎈</div>
          <div class="brand-glow"></div>
        </div>
        <div class="brand-info">
          <div class="brand-title">K8S<span>CONTROL</span></div>
          <div class="brand-subtitle">Enterprise Hybrid Platform</div>
        </div>
      </div>

      <!-- Live Environment & Tenant Scope Pill -->
      <div class="env-pill">
        <span class="pulse-dot pulse-dot-emerald"></span>
        <div class="env-pill-text">
          <span class="env-mesh">Air-Gapped Mesh</span>
          <span class="env-cluster">{{ selectedTenant }}</span>
        </div>
        <span class="env-chip">v2.4</span>
      </div>

      <!-- Navigation Menu with 5 Collapsible Groups -->
      <nav class="nav-menu">
        <div v-for="group in navGroups" :key="group.key" class="nav-group-wrapper">
          <!-- Section Header / Accordion Toggle -->
          <div class="nav-section-header" @click="toggleSection(group.key)">
            <span class="section-title">
              <span class="section-icon">{{ group.icon }}</span>
              <span>{{ group.label }}</span>
            </span>
            <span class="section-caret" :class="{ 'caret-collapsed': collapsedSections[group.key] }">
              ▾
            </span>
          </div>

          <!-- Section Navigation Items -->
          <div v-show="!collapsedSections[group.key]" class="nav-items-container animate-fade-in">
            <router-link 
              v-for="item in group.items" 
              :key="item.path" 
              :to="item.path" 
              class="nav-item" 
              :class="{ 'nav-active': route.path === item.path || (item.path === '/deployments' && route.path === '/workloads') }"
            >
              <div class="nav-icon">{{ item.icon }}</div>
              <div class="nav-label">
                <span>{{ item.name }}</span>
                <small>{{ item.sub }}</small>
              </div>
              <div v-if="item.badge" class="nav-badge-live">{{ item.badge }}</div>
            </router-link>
          </div>
        </div>
      </nav>

      <!-- Sidebar Bottom System Telemetry -->
      <div class="sidebar-footer">
        <div class="telemetry-card">
          <div class="telemetry-row">
            <span class="telemetry-key">ZeroTrust KMS</span>
            <span class="telemetry-val text-emerald">ARMED</span>
          </div>
          <div class="telemetry-row">
            <span class="telemetry-key">Dual-Sync Target</span>
            <span class="telemetry-val text-cyan font-mono">NVMe + S3</span>
          </div>
          <div class="telemetry-progress">
            <div class="progress-bar" style="width: 100%;"></div>
          </div>
        </div>
      </div>
    </aside>

    <!-- Main Wrapper -->
    <div class="main-wrapper">
      <!-- Enterprise Top Navigation / HUD -->
      <header class="top-hud">
        <div class="hud-left">
          <!-- Mobile Menu Toggle Button -->
          <button 
            class="mobile-menu-btn" 
            @click="mobileSidebarOpen = !mobileSidebarOpen" 
            aria-label="Toggle navigation menu"
          >
            <span>☰</span>
          </button>

          <div class="breadcrumb-nav">
            <span class="breadcrumb-root">Enterprise Console</span>
            <span class="breadcrumb-sep">/</span>
            <span class="breadcrumb-active">{{ route.name?.toString().toUpperCase() || 'DASHBOARD' }}</span>
          </div>

          <!-- Command Palette Search Button -->
          <button class="command-search-btn" @click="showCommandPalette = true">
            <span class="search-ico">🔍</span>
            <span class="search-text">Search commands, routes, models...</span>
            <kbd class="kbd-badge">Ctrl K</kbd>
          </button>
        </div>

        <div class="hud-right">
          <!-- Tenant Selector Dropdown -->
          <div class="tenant-selector-wrap" title="Workspace / Multi-Tenant Organization">
            <span class="tenant-icon">🏢</span>
            <select v-model="selectedTenant" @change="handleTenantChange" class="tenant-select font-mono" title="Select active workspace tenant">
              <option v-for="t in tenants" :key="t.id" :value="t.id">
                {{ t.name }}
              </option>
            </select>
          </div>

          <!-- Mesh Status -->
          <div class="hud-stat hud-stat-interactive" :title="meshTooltip">
            <span 
              class="pulse-dot" 
              :class="clusterMeshStatus === 'HEALTHY' ? 'pulse-dot-emerald' : (downNodeCount > 0 ? 'pulse-dot-rose' : 'pulse-dot-amber')"
            ></span>
            <span class="stat-label">Mesh:</span>
            <span 
              class="stat-value" 
              :class="clusterMeshStatus === 'HEALTHY' ? 'text-emerald' : (downNodeCount > 0 ? 'text-rose font-bold' : 'text-amber')"
            >
              {{ downNodeCount > 0 ? `DEGRADED (${downNodeCount} Down)` : clusterMeshStatus }}
            </span>
          </div>

          <!-- Latency -->
          <div class="hud-stat hud-stat-interactive" :title="latencyTooltip">
            <span class="stat-label">Latency:</span>
            <span class="stat-value font-mono" :class="logStore.isConnected ? 'text-emerald font-semibold' : 'text-cyan'">
              {{ streamStatusText }}
            </span>
          </div>

          <!-- Compact PWA Install Button -->
          <button 
            v-if="isInstallable" 
            class="pwa-install-hud-btn" 
            @click="promptInstall"
            title="Install K8sControl PWA App"
            aria-label="Install App"
          >
            <span class="pwa-hud-icon">📲</span>
            <span class="pwa-hud-label font-mono">INSTALL</span>
          </button>

          <!-- Top HUD Global Alert Bell & Dropdown Toast -->
          <TopHudAlertBell />

          <!-- User Profile & Sign Out -->
          <div v-if="authStore.user" class="hud-user">
            <span class="user-role font-mono">{{ authStore.user.role || 'ADMIN' }}</span>
            <button class="btn btn-secondary btn-sm" title="Sign Out" @click="handleLogout">
              <span>Sign Out</span>
            </button>
          </div>
        </div>
      </header>

      <!-- Main Page Content -->
      <main class="page-container animate-fade-in">
        <RouterView />
      </main>
    </div>

    <!-- COMMAND PALETTE MODAL (Ctrl+K) -->
    <ModalDrawer
      v-model:show="showCommandPalette"
      title="Enterprise Command Palette"
      subtitle="Quick search across all 20+ platform routes, AI models, and resources."
      max-width="640px"
    >
      <div class="command-palette-content">
        <div class="palette-input-wrap">
          <span class="palette-search-icon">🔍</span>
          <input 
            v-model="searchQuery" 
            type="text" 
            placeholder="Type a route, view, or category..." 
            class="input-glass palette-search-input"
            autofocus 
          />
          <span v-if="searchQuery" class="palette-clear" @click="searchQuery = ''">✕</span>
        </div>

        <div class="palette-results">
          <div 
            v-for="item in filteredSearchResults" 
            :key="item.path" 
            class="palette-item"
            :class="{ 'palette-item-active': route.path === item.path }"
            @click="navigateTo(item.path)"
          >
            <div class="p-item-icon">{{ item.icon }}</div>
            <div class="p-item-info">
              <div class="p-item-title-row">
                <span class="p-item-name">{{ item.name }}</span>
                <span class="p-item-group font-mono">{{ item.group }}</span>
              </div>
              <small class="p-item-sub">{{ item.sub }}</small>
            </div>
            <span class="p-item-path font-mono">{{ item.path }}</span>
          </div>

          <div v-if="filteredSearchResults.length === 0" class="palette-empty">
            <span>No matching commands or routes found for "{{ searchQuery }}"</span>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="palette-footer-tips font-mono">
          <span>Navigate with click • <kbd>Esc</kbd> to close</span>
        </div>
      </template>
    </ModalDrawer>

    <!-- GLOBAL ALERT CENTER & TELEMETRY DIAGNOSTICS MODAL -->
    <AlertCenterModal
      :show="alertStore.showAlertCenterModal"
      :activeAlerts="alertStore.activeAlerts"
      :mutedAlertsList="alertStore.mutedAlertsList"
      :hasCriticalAlerts="alertStore.hasCriticalAlerts"
      @close="alertStore.closeAlertCenter"
      @mute-alert="alertStore.muteAlert"
      @unmute-alert="alertStore.unmuteAlert"
      @mute-all="alertStore.muteAll"
      @unmute-all="alertStore.unmuteAll"
      @dismiss-alert="alertStore.dismissAlert"
      @navigate-to-host="handleNavigateToHost"
    />

    <!-- Mobile Bottom Navigation Bar (Docked) -->
    <nav class="mobile-bottom-bar" aria-label="Mobile Navigation">
      <router-link to="/" class="mobile-nav-tab" :class="{ 'mobile-tab-active': route.path === '/' }">
        <span class="tab-icon">📊</span>
        <span class="tab-label font-mono">Overview</span>
      </router-link>
      <router-link to="/agents" class="mobile-nav-tab" :class="{ 'mobile-tab-active': route.path.startsWith('/agents') }">
        <span class="tab-icon">🤖</span>
        <span class="tab-label font-mono">Agents</span>
      </router-link>
      <router-link to="/fleet" class="mobile-nav-tab" :class="{ 'mobile-tab-active': route.path.startsWith('/fleet') }">
        <span class="tab-icon">☸️</span>
        <span class="tab-label font-mono">Fleet</span>
      </router-link>
      <router-link to="/deployments" class="mobile-nav-tab" :class="{ 'mobile-tab-active': route.path.startsWith('/deployments') }">
        <span class="tab-icon">🚀</span>
        <span class="tab-label font-mono">Deploy</span>
      </router-link>
      <button 
        class="mobile-nav-tab mobile-nav-btn" 
        :class="{ 'mobile-tab-active': alertStore.showAlertCenterModal }" 
        @click="alertStore.showAlertCenterModal = true"
        aria-label="Open Alert Center"
      >
        <div class="tab-icon-wrap">
          <span class="tab-icon">🔔</span>
          <span v-if="alertStore.activeAlerts.length > 0" class="mobile-badge-count font-mono">
            {{ alertStore.activeAlerts.length > 99 ? '99+' : alertStore.activeAlerts.length }}
          </span>
        </div>
        <span class="tab-label font-mono">Alerts</span>
      </button>
    </nav>

    <!-- PWA Install Floating Banner -->
    <PwaInstallBanner />
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  width: 100vw;
  min-height: 100vh;
  background-color: var(--bg-app);
}

/* Sidebar */
.sidebar {
  width: 290px;
  min-width: 290px;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  height: 100vh;
  position: sticky;
  top: 0;
  z-index: 50;
  overflow: hidden;
}

.brand-container {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border-subtle);
  cursor: pointer;
  transition: background 0.15s ease;
}

.brand-container:hover {
  background: rgba(255, 255, 255, 0.02);
}

.brand-icon-wrapper {
  position: relative;
  width: 38px;
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.brand-icon {
  width: 38px;
  height: 38px;
  background: var(--grad-cyan);
  color: #fff;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: bold;
  box-shadow: 0 4px 15px rgba(6, 182, 212, 0.4);
  position: relative;
  z-index: 2;
}

.brand-glow {
  position: absolute;
  width: 38px;
  height: 38px;
  background: var(--accent-cyan);
  filter: blur(10px);
  opacity: 0.5;
}

.brand-title {
  font-size: 15px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: #fff;
}

.brand-title span {
  color: var(--accent-cyan);
  margin-left: 2px;
}

.brand-subtitle {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 500;
}

.env-pill {
  margin: 12px 16px;
  padding: 8px 12px;
  background: rgba(16, 185, 129, 0.08);
  border: 1px solid rgba(16, 185, 129, 0.2);
  border-radius: 10px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.env-pill-text {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.env-mesh {
  font-size: 11px;
  font-weight: 700;
  color: #34d399;
}

.env-cluster {
  font-size: 10px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.env-chip {
  font-size: 10px;
  background: rgba(16, 185, 129, 0.2);
  color: #a7f3d0;
  padding: 2px 6px;
  border-radius: 6px;
  font-weight: 700;
  font-family: var(--font-mono);
}

/* Nav Menu */
.nav-menu {
  padding: 8px 12px;
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.nav-group-wrapper {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  cursor: pointer;
  user-select: none;
  border-radius: 6px;
  transition: background 0.15s ease;
}

.nav-section-header:hover {
  background: rgba(255, 255, 255, 0.03);
}

.section-title {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.06em;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.section-icon {
  font-size: 12px;
}

.section-caret {
  font-size: 11px;
  color: var(--text-muted);
  transition: transform 0.2s ease;
}

.caret-collapsed {
  transform: rotate(-90deg);
}

.nav-items-container {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 10px;
  text-decoration: none;
  color: var(--text-secondary);
  transition: all 0.15s ease;
  border: 1px solid transparent;
}

.nav-item:hover {
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-primary);
}

.nav-active {
  background: linear-gradient(90deg, rgba(6, 182, 212, 0.14) 0%, rgba(6, 182, 212, 0.03) 100%);
  border-color: rgba(6, 182, 212, 0.3);
  color: #fff;
}

.nav-icon {
  font-size: 16px;
}

.nav-label {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.nav-label span {
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-label small {
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-badge-live {
  font-size: 9px;
  font-weight: 800;
  background: rgba(244, 63, 94, 0.15);
  color: #fb7185;
  padding: 2px 6px;
  border-radius: 9999px;
  font-family: var(--font-mono);
  animation: pulse 1.5s infinite;
}

.sidebar-footer {
  padding: 14px 16px;
  border-top: 1px solid var(--border-subtle);
  background: rgba(11, 15, 25, 0.95);
}

.telemetry-card {
  background: rgba(16, 24, 40, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 10px 12px;
}

.telemetry-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 11px;
  margin-bottom: 4px;
}

.telemetry-key {
  color: var(--text-muted);
}

.telemetry-val {
  font-weight: 600;
}

.telemetry-progress {
  width: 100%;
  height: 4px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
  margin-top: 6px;
}

.progress-bar {
  height: 100%;
  background: var(--grad-cyan);
}

/* Top HUD */
.main-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--bg-app);
}

.top-hud {
  height: 60px;
  background: rgba(11, 15, 25, 0.85);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 40;
}

.hud-left {
  display: flex;
  align-items: center;
  gap: 18px;
}

.breadcrumb-nav {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.breadcrumb-root {
  color: var(--text-muted);
}

.breadcrumb-sep {
  color: var(--border-medium);
}

.breadcrumb-active {
  color: #fff;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.command-search-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  cursor: pointer;
  color: var(--text-muted);
  font-size: 12px;
  transition: all 0.15s ease;
}

.command-search-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  border-color: rgba(6, 182, 212, 0.4);
  color: var(--text-primary);
}

.search-ico {
  font-size: 12px;
}

.search-text {
  font-size: 12px;
}

.kbd-badge {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid var(--border-subtle);
  border-radius: 4px;
  padding: 1px 6px;
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-secondary);
}

.hud-right {
  display: flex;
  align-items: center;
  gap: 18px;
}

.tenant-selector-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 4px 10px;
}

.tenant-icon {
  font-size: 14px;
}

.tenant-select {
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 600;
  outline: none;
  cursor: pointer;
}

.tenant-select option {
  background: var(--bg-sidebar);
  color: #fff;
}

.hud-stat {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.hud-stat-interactive {
  cursor: help;
  padding: 4px 8px;
  border-radius: 6px;
  transition: all 0.15s ease;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid transparent;
}

.hud-stat-interactive:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: var(--border-subtle);
}

.stat-label {
  color: var(--text-muted);
}

.stat-value {
  font-weight: 600;
}

.hud-user {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-left: 12px;
  border-left: 1px solid var(--border-subtle);
}

.user-role {
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  background: rgba(6, 182, 212, 0.1);
  border: 1px solid rgba(6, 182, 212, 0.25);
  padding: 2px 8px;
  border-radius: 6px;
}

.page-container {
  flex: 1;
  padding: 28px;
  overflow-y: auto;
}

/* Command Palette Modal Content */
.command-palette-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.palette-input-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.palette-search-icon {
  position: absolute;
  left: 14px;
  font-size: 14px;
  color: var(--text-muted);
}

.palette-search-input {
  width: 100%;
  padding: 12px 38px 12px 42px;
  font-size: 14px;
}

.palette-clear {
  position: absolute;
  right: 14px;
  cursor: pointer;
  color: var(--text-muted);
}

.palette-results {
  max-height: 380px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.palette-item {
  padding: 10px 14px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  gap: 14px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.palette-item:hover, .palette-item-active {
  background: rgba(6, 182, 212, 0.12);
  border-color: rgba(6, 182, 212, 0.4);
}

.p-item-icon {
  font-size: 20px;
}

.p-item-info {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.p-item-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.p-item-name {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.p-item-group {
  font-size: 10px;
  color: var(--accent-sky);
  background: rgba(56, 189, 248, 0.1);
  padding: 1px 6px;
  border-radius: 4px;
}

.p-item-sub {
  font-size: 11px;
  color: var(--text-muted);
}

.p-item-path {
  font-size: 11px;
  color: var(--text-muted);
}

.palette-empty {
  padding: 30px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
}

.palette-footer-tips {
  font-size: 11px;
  color: var(--text-muted);
}

.btn-sm {
  padding: 6px 12px;
  font-size: 12px;
}

.text-emerald { color: #34d399; }
.text-amber { color: #fbbf24; }
.text-cyan { color: #38bdf8; }
.font-mono { font-family: var(--font-mono); }

.login-container {
  width: 100vw;
  min-height: 100vh;
  display: flex;
}

/* Responsive Styles */
.mobile-menu-btn {
  display: none;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  color: var(--text-primary);
  width: 36px;
  height: 36px;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  cursor: pointer;
  flex-shrink: 0;
  transition: all 0.15s ease;
}

.mobile-menu-btn:hover {
  background: rgba(255, 255, 255, 0.1);
}

.sidebar-backdrop {
  display: none;
}

/* PWA HUD Install Button */
.pwa-install-hud-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: rgba(6, 182, 212, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 8px;
  padding: 5px 9px;
  color: var(--accent-sky);
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 0 10px rgba(6, 182, 212, 0.15);
}

.pwa-install-hud-btn:hover {
  background: rgba(6, 182, 212, 0.22);
  border-color: var(--accent-sky);
  box-shadow: 0 0 14px rgba(56, 189, 248, 0.35);
  transform: translateY(-1px);
}

.pwa-hud-icon {
  font-size: 13px;
}

.pwa-hud-label {
  letter-spacing: 0.05em;
}

/* Mobile Bottom Navigation Bar */
.mobile-bottom-bar {
  display: none;
}

@media (max-width: 1024px) {
  .mobile-menu-btn {
    display: flex;
  }

  .sidebar {
    position: fixed;
    top: 0;
    left: 0;
    height: 100vh;
    transform: translateX(-100%);
    transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    z-index: 100;
    box-shadow: 0 0 35px rgba(0, 0, 0, 0.85);
  }

  .sidebar.sidebar-open {
    transform: translateX(0);
  }

  .sidebar-backdrop {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
    z-index: 99;
  }

  .page-container {
    padding: 16px;
  }

  .command-search-btn .search-text {
    display: none;
  }

  .hud-stat {
    display: none;
  }
}

@media (max-width: 640px) {
  .top-hud {
    padding: 0 10px;
    gap: 8px;
  }

  .hud-left {
    gap: 8px;
    flex-shrink: 0;
  }

  .hud-right {
    gap: 8px;
    min-width: 0;
  }

  .breadcrumb-nav {
    display: none;
  }

  .command-search-btn {
    padding: 6px 10px;
    gap: 0;
  }

  .command-search-btn .kbd-badge {
    display: none;
  }

  .pwa-install-hud-btn .pwa-hud-label {
    display: none;
  }

  .pwa-install-hud-btn {
    padding: 5px 6px;
  }

  .tenant-selector-wrap {
    max-width: 120px;
    padding: 4px 6px;
    gap: 4px;
    flex-shrink: 1;
    min-width: 0;
  }

  .tenant-select {
    max-width: 80px;
    text-overflow: ellipsis;
    white-space: nowrap;
    overflow: hidden;
    font-size: 11px;
  }

  .hud-user {
    padding-left: 6px;
    gap: 6px;
  }

  .user-role {
    display: none;
  }

  .page-container {
    padding: 12px 8px calc(68px + var(--sab)) 8px;
  }

  /* Docked Mobile Bottom Navigation Bar */
  .mobile-bottom-bar {
    display: flex;
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: calc(56px + var(--sab));
    padding-bottom: var(--sab);
    background: rgba(7, 9, 14, 0.94);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    border-top: 1px solid var(--border-medium);
    z-index: 90;
    align-items: center;
    justify-content: space-around;
    box-shadow: 0 -4px 20px rgba(0, 0, 0, 0.5);
  }

  .mobile-nav-tab {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 3px;
    color: var(--text-muted);
    text-decoration: none;
    height: 100%;
    background: transparent;
    border: none;
    cursor: pointer;
    transition: all 0.15s ease;
    padding: 4px 0;
  }

  .mobile-nav-tab:hover,
  .mobile-nav-tab.mobile-tab-active {
    color: var(--accent-sky);
  }

  .mobile-nav-tab.mobile-tab-active .tab-icon {
    transform: scale(1.1);
    filter: drop-shadow(0 0 6px rgba(56, 189, 248, 0.5));
  }

  .tab-icon {
    font-size: 18px;
    line-height: 1;
    transition: transform 0.15s ease, filter 0.15s ease;
  }

  .tab-label {
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 0.02em;
  }

  .tab-icon-wrap {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .mobile-badge-count {
    position: absolute;
    top: -4px;
    right: -10px;
    background: #f43f5e;
    color: #fff;
    font-size: 9px;
    font-weight: 700;
    padding: 0 4px;
    border-radius: 8px;
    min-width: 14px;
    height: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 0 6px rgba(244, 63, 94, 0.6);
  }
}
</style>
