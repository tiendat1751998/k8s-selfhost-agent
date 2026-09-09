<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/authStore'
import { navGroups } from '../../config/navigation'

defineProps<{
  mobileOpen: boolean
  selectedTenant: string
  tenants: { id: string; name: string }[]
  userInitials: string
}>()

const emit = defineEmits<{
  (e: 'update:selectedTenant', val: string): void
  (e: 'tenantChange'): void
  (e: 'logout'): void
  (e: 'closeMobile'): void
}>()

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

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

function onTenantChange(e: Event) {
  const target = e.target as HTMLSelectElement
  emit('update:selectedTenant', target.value)
  emit('tenantChange')
}

function navigateHome() {
  emit('closeMobile')
  router.push('/')
}
</script>

<template>
  <aside class="sidebar" :class="{ 'sidebar-open': mobileOpen }">
    <!-- Brand Logo -->
    <div
      class="brand-container"
      role="button"
      tabindex="0"
      aria-label="Go to Overview"
      @click="navigateHome"
      @keydown.enter="navigateHome"
      @keydown.space.prevent="navigateHome"
    >
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
        <div
          class="nav-section-header"
          role="button"
          tabindex="0"
          :aria-expanded="!collapsedSections[group.key]"
          :aria-label="`Toggle ${group.label} navigation section`"
          @click="toggleSection(group.key)"
          @keydown.enter="toggleSection(group.key)"
          @keydown.space.prevent="toggleSection(group.key)"
        >
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
            @click="emit('closeMobile')"
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
      <!-- Mobile Drawer Tenant Switcher & User Profile -->
      <div class="drawer-mobile-meta">
        <div class="drawer-tenant-row">
          <span class="drawer-tenant-icon" aria-hidden="true">🏢</span>
          <select
            :value="selectedTenant"
            @change="onTenantChange"
            class="drawer-tenant-select font-mono"
            aria-label="Select workspace tenant"
          >
            <option v-for="t in tenants" :key="t.id" :value="t.id">
              {{ t.name }}
            </option>
          </select>
        </div>
        <div v-if="authStore.user" class="drawer-user-row">
          <div class="drawer-user-badge">
            <span class="user-avatar">{{ userInitials }}</span>
            <span class="user-role font-mono">{{ authStore.user.role || 'ADMIN' }}</span>
          </div>
          <button class="drawer-logout-btn" title="Sign Out" aria-label="Sign Out" @click="emit('logout')">
            <span class="logout-icon" aria-hidden="true">🚪</span>
            <span>Sign Out</span>
          </button>
        </div>
      </div>

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
</template>
