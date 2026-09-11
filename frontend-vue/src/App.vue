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
import AppSidebar from './components/layout/AppSidebar.vue'
import ZeroTrustDrawer from './components/layout/ZeroTrustDrawer.vue'
import AppCommandPalette from './components/layout/AppCommandPalette.vue'
import TopHudAlertBell from './components/layout/TopHudAlertBell.vue'
import AlertCenterModal from './components/overview/alerts/AlertCenterModal.vue'
import AppMobileNav from './components/layout/AppMobileNav.vue'
import PwaInstallBanner from './components/common/PwaInstallBanner.vue'
import BaseIcon from './components/ui/BaseIcon.vue'
import GlobalContextSelector from './components/navigation/GlobalContextSelector.vue'
import { navGroups } from './config/navigation'
import '@/assets/styles/layout/app-shell.css'

const routeBreadcrumbs: Record<string, { category: string; title: string }> = {
  '/': { category: 'Observability', title: 'Fleet Overview' },
  '/incidents': { category: 'Observability', title: 'Incidents & RCA' },
  '/slo': { category: 'Observability', title: 'SLOs & Budgets' },
  '/logs': { category: 'Observability', title: 'Real-Time Logs' },
  '/fleet': { category: 'Compute & Fleet', title: 'Fleet Clusters' },
  '/hosts': { category: 'Infrastructure', title: 'Hosts' },
  '/deployments': { category: 'Compute', title: 'Deployments & Apps' },
  '/promotions': { category: 'Delivery', title: 'Promotions' },
  '/explorer': { category: 'Cluster', title: 'Cluster Explorer' },
  '/helm': { category: 'Compute', title: 'Helm Catalog' },
  '/audit': { category: 'Governance', title: 'Audit & CVEs' },
  '/compliance': { category: 'Governance', title: 'Compliance & CIS' },
  '/drift': { category: 'Governance', title: 'Config Drift' },
  '/backup': { category: 'Governance', title: 'Disaster Recovery' },
  '/automation': { category: 'Automation', title: 'Automation Rules' },
  '/runbooks': { category: 'Automation', title: 'SRE Runbooks' },
  '/cost': { category: 'FinOps', title: 'Cost Optimization' },
  '/capacity': { category: 'Compute', title: 'Capacity Planning' },
  '/tenancy': { category: 'Management', title: 'Tenancy & RBAC' },
  '/ai-hub': { category: 'Management', title: 'AI Provider Hub' },
  '/changes': { category: 'Management', title: 'Change Requests' },
  '/alerts': { category: 'Management', title: 'Alerts & Channels' },
  '/reports': { category: 'Management', title: 'Reports Center' },
  '/catalog': { category: 'Developer Portal', title: 'Service Catalog' },
  '/scaffolder': { category: 'Developer Portal', title: 'Scaffolder Templates' },
  '/ecosystem': { category: 'Management', title: 'Ecosystem Tools' },
  '/plugins': { category: 'Management', title: 'Plugin Hub' },
  '/settings': { category: 'Management', title: 'System Settings' },
}

const currentBreadcrumb = computed(() => {
  const p = route.path
  if (routeBreadcrumbs[p]) return routeBreadcrumbs[p]
  for (const group of navGroups) {
    const item = group.items.find(i => i.path === p || (i.path !== '/' && p.startsWith(i.path)))
    if (item) {
      const cat = group.label.split('&')[0].trim()
      const formatted = cat.charAt(0).toUpperCase() + cat.slice(1).toLowerCase()
      return { category: formatted, title: item.name }
    }
  }
  return { category: 'Platform', title: (route.name as string) || 'Dashboard' }
})

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const backupStore = useBackupStore()
const securityStore = useSecurityStore()
const logStore = useLogStore()
const appStore = useAppStore()
const alertStore = useAlertStore()

const userInitials = computed(() => {
  const identifier = authStore.user?.email || authStore.user?.role || 'AD'
  return identifier.slice(0, 2).toUpperCase()
})

// State
const selectedTenant = ref('default-tenant')
const showCommandPalette = ref(false)
const mobileSidebarOpen = ref(false)
const isSidebarCollapsed = ref(false)
const showZeroTrustDrawer = ref(false)

watch(() => route.path, () => {
  mobileSidebarOpen.value = false
})

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
      if (authStore.user?.tenant_id && tenants.value.some(t => t.id === authStore.user?.tenant_id)) {
        selectedTenant.value = authStore.user.tenant_id
      } else if (!tenants.value.some(t => t.id === selectedTenant.value)) {
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

watch(() => authStore.isAuthenticated, (authed) => {
  if (authed) {
    loadTenants().catch(() => {})
    backupStore.fetchAll().catch(() => {})
    securityStore.fetchAll().catch(() => {})
    syncGlobalTelemetry().catch(() => {})
  }
})

onMounted(() => {
  if (authStore.isAuthenticated) {
    backupStore.fetchAll().catch(() => {})
    securityStore.fetchAll().catch(() => {})
    loadTenants().catch(() => {})
    syncGlobalTelemetry().catch(() => {})
  }
  globalTelemetryInterval = setInterval(() => {
    if (authStore.isAuthenticated) {
      syncGlobalTelemetry().catch(() => {})
    }
  }, 10000)
})

onUnmounted(() => {
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

const streamStatusText = computed(() => {
  return logStore.isConnected ? '<50ms' : 'POLLING'
})

const latencyTooltip = computed(() => {
  if (logStore.isConnected) {
    return 'Telemetry Stream: Live WebSocket connection active (<50ms real-time stream latency). Continuous bidirectional events.'
  }
  return 'Telemetry Stream: HTTP 5s Polling Fallback active. WebSocket stream reconnecting in background.'
})

const systemStatus = computed(() => {
  if (downNodeCount.value > 0) {
    return {
      label: `${downNodeCount.value} Down`,
      compactLabel: `${downNodeCount.value}`,
      word: ' Down',
      fullLabel: `${downNodeCount.value} Nodes Down`,
      dotClass: 'pulse-dot-rose',
      textClass: 'text-rose font-bold'
    }
  }
  if (clusterMeshStatus.value === 'DEGRADED') {
    return {
      label: 'Degraded',
      compactLabel: 'Degraded',
      word: '',
      fullLabel: 'Degraded',
      dotClass: 'pulse-dot-amber',
      textClass: 'text-amber'
    }
  }
  return {
    label: 'Operational',
    compactLabel: 'Operational',
    word: '',
    fullLabel: 'Operational',
    dotClass: 'pulse-dot-emerald',
    textClass: 'text-emerald'
  }
})

const systemStatusTooltip = computed(() => {
  return `${meshTooltip.value} | Telemetry: ${streamStatusText.value} (${latencyTooltip.value})`
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
    <AppSidebar
      :mobile-open="mobileSidebarOpen"
      v-model:collapsed="isSidebarCollapsed"
      v-model:selected-tenant="selectedTenant"
      :tenants="tenants"
      :user-initials="userInitials"
      @tenant-change="handleTenantChange"
      @logout="handleLogout"
      @close-mobile="mobileSidebarOpen = false"
      @open-zerotrust="showZeroTrustDrawer = true"
    />

    <!-- Main Wrapper -->
    <div class="main-wrapper" :class="{ 'sidebar-collapsed': isSidebarCollapsed }">
      <!-- Enterprise Top Navigation / HUD -->
      <header class="top-hud">
        <!-- Left: Mobile menu & Screen Breadcrumb -->
        <div class="hud-left">
          <!-- Mobile Menu Toggle Button -->
          <button
            class="mobile-menu-btn"
            @click="mobileSidebarOpen = !mobileSidebarOpen"
            aria-label="Toggle navigation menu"
          >
            <BaseIcon name="menu" size="sm" />
          </button>

          <!-- Mobile Brand Logo / Title (visible on mobile only) -->
          <div
            class="hud-mobile-brand"
            role="button"
            tabindex="0"
            @click="router.push('/')"
            @keydown.enter="router.push('/')"
            title="K8SCONTROL Enterprise Platform"
            aria-label="K8SCONTROL Enterprise Platform Overview"
          >
            <BaseIcon name="anchor" size="md" class="hud-mobile-brand-icon" aria-hidden="true" />
            <span class="hud-mobile-brand-title">K8S<span>CONTROL</span></span>
          </div>

          <!-- Dynamic Screen Title & Category Breadcrumb -->
          <nav class="hud-breadcrumb" aria-label="Screen location breadcrumb">
            <span class="bc-cat font-mono">{{ currentBreadcrumb.category }}</span>
            <span class="bc-sep font-mono">/</span>
            <span class="bc-title">{{ currentBreadcrumb.title }}</span>
          </nav>
        </div>

        <!-- Center: Cluster / Namespace Scope Dropdowns & Tenant -->
        <div class="hud-center">
          <!-- Global Context Selector: Cluster & Namespace (Tasks 021 & 015) -->
          <GlobalContextSelector />

          <!-- Sleek Workspace / Tenant Selector -->
          <div class="tenant-selector-wrap" title="Workspace / Multi-Tenant Organization">
            <span class="tenant-icon" aria-hidden="true">
              <BaseIcon name="layers" size="xs" />
            </span>
            <select v-model="selectedTenant" @change="handleTenantChange" class="tenant-select font-mono" aria-label="Select active workspace tenant">
              <option v-for="t in tenants" :key="t.id" :value="t.id">
                {{ t.name }}
              </option>
            </select>
            <span class="tenant-chevron" aria-hidden="true">
              <BaseIcon name="chevron-down" size="xs" />
            </span>
          </div>
        </div>

        <!-- Right: Search, Cluster health, Alerts, User Profile -->
        <div class="hud-right">
          <!-- Desktop Quick Search Trigger (125px compact) -->
          <button class="command-search-btn desktop-search" @click="showCommandPalette = true" aria-label="Quick search (Ctrl+K)">
            <span class="search-ico" aria-hidden="true">
              <BaseIcon name="search" size="xs" />
            </span>
            <span class="search-text">Search...</span>
            <kbd class="kbd-badge">Ctrl K</kbd>
          </button>

          <!-- Mobile Quick Search Trigger (Mobile only) -->
          <button class="mobile-search-btn" @click="showCommandPalette = true" aria-label="Quick search (Ctrl+K)" title="Quick search (Ctrl+K)">
            <span class="search-ico" aria-hidden="true">
              <BaseIcon name="search" size="xs" />
            </span>
          </button>

          <!-- Consolidated System Telemetry Pill (Clickable) -->
          <div
            class="hud-status-pill"
            :title="`${systemStatusTooltip} — Click to inspect down nodes`"
            role="button"
            tabindex="0"
            aria-label="Inspect down nodes in Hosts view"
            aria-live="polite"
            @click="router.push('/hosts?status=offline')"
            @keydown.enter="router.push('/hosts?status=offline')"
            @keydown.space.prevent="router.push('/hosts?status=offline')"
          >
            <span class="pulse-dot" :class="systemStatus.dotClass"></span>
            <span class="status-label" :class="systemStatus.textClass">
              <span class="status-num">{{ systemStatus.compactLabel }}</span>
              <span v-if="systemStatus.word" class="status-word">{{ systemStatus.word }}</span>
            </span>
          </div>

          <!-- Top HUD Global Alert Bell & Dropdown Toast -->
          <TopHudAlertBell />

          <!-- Unified User Profile & Sign Out -->
          <div v-if="authStore.user" class="hud-user">
            <div class="user-profile-badge" :title="`User: ${authStore.user.email || authStore.user.role || 'Admin'} (${authStore.user.role || 'ADMIN'})`">
              <span class="user-avatar">{{ userInitials }}</span>
              <span class="user-role font-mono">{{ authStore.user.role || 'ADMIN' }}</span>
            </div>
            <button class="hud-logout-btn" title="Sign Out" aria-label="Sign Out" @click="handleLogout">
              <span class="logout-icon" aria-hidden="true">
                <BaseIcon name="lock" size="xs" />
              </span>
            </button>
          </div>
        </div>
      </header>

      <!-- Main Page Content -->
      <main class="page-container main-content animate-fade-in" :class="{ 'route-logs': route.path.startsWith('/logs'), 'route-hosts': route.path.startsWith('/hosts') }">
        <RouterView />
      </main>
    </div>

    <!-- Command Palette Modal (Ctrl+K) -->
    <AppCommandPalette v-model:show="showCommandPalette" />

    <!-- Global Alert Center & Telemetry Diagnostics Modal -->
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
    <AppMobileNav />

    <!-- ZeroTrust KMS & Dual-Sync Attestation Drawer -->
    <ZeroTrustDrawer v-model:show="showZeroTrustDrawer" />

    <!-- PWA Install Floating Banner -->
    <PwaInstallBanner />
  </div>
</template>
