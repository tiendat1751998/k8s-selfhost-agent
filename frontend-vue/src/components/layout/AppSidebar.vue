<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/authStore'
import { navGroups, type NavItem } from '../../config/navigation'
import BaseIcon from '../ui/BaseIcon.vue'

const STORAGE_KEY = 'k8s_sidebar_collapsed'

const props = defineProps<{
  mobileOpen: boolean
  selectedTenant: string
  tenants: { id: string; name: string }[]
  userInitials: string
  collapsed?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:selectedTenant', val: string): void
  (e: 'update:collapsed', val: boolean): void
  (e: 'tenantChange'): void
  (e: 'logout'): void
  (e: 'closeMobile'): void
  (e: 'open-zerotrust'): void
}>()

function handleOpenZeroTrust() {
  emit('open-zerotrust')
}

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

function getInitialCollapseState(): boolean {
  if (typeof window === 'undefined') return false
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved !== null) {
    return saved === 'true'
  }
  return false
}

const isCollapsed = ref<boolean>(getInitialCollapseState())

watch(() => props.collapsed, (newVal) => {
  if (newVal !== undefined && newVal !== isCollapsed.value) {
    isCollapsed.value = newVal
  }
})

function toggleCollapse() {
  isCollapsed.value = !isCollapsed.value
  try {
    localStorage.setItem(STORAGE_KEY, String(isCollapsed.value))
  } catch {
    // quota/sandboxing fallback
  }
  emit('update:collapsed', isCollapsed.value)
}

function handleResize() {
  if (typeof window === 'undefined') return
  if (window.innerWidth > 1024 && props.mobileOpen) {
    emit('closeMobile')
  }
}

onMounted(() => {
  emit('update:collapsed', isCollapsed.value)
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})

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

// Hover Tooltip for Collapsed Rail
const hoveredItem = ref<{ item: NavItem; top: number } | null>(null)

function handleItemMouseEnter(item: NavItem, event: MouseEvent) {
  if (!isCollapsed.value) return
  const target = event.currentTarget as HTMLElement | null
  if (!target) return
  const rect = target.getBoundingClientRect()
  hoveredItem.value = {
    item,
    top: rect.top + rect.height / 2,
  }
}

function handleItemMouseLeave() {
  hoveredItem.value = null
}

function handleItemClick() {
  hoveredItem.value = null
  emit('closeMobile')
}
</script>

<template>
  <aside class="sidebar" :class="{ 'sidebar-open': mobileOpen, 'is-collapsed': isCollapsed }">
    <!-- Brand Header & Collapse Toggle -->
    <div class="sidebar-brand-header">
      <div
        class="brand-container"
        role="button"
        tabindex="0"
        aria-label="Go to Overview"
        :title="isCollapsed ? 'K8SCONTROL — Go to Overview' : undefined"
        @click="navigateHome"
        @keydown.enter="navigateHome"
        @keydown.space.prevent="navigateHome"
      >
        <div class="brand-icon-wrapper">
          <div class="brand-icon">
            <BaseIcon name="anchor" size="lg" />
          </div>
          <div class="brand-glow"></div>
        </div>
        <div class="brand-info">
          <div class="brand-title">K8S<span>CONTROL</span></div>
          <div class="brand-subtitle">Enterprise Hybrid Platform</div>
        </div>
      </div>

      <!-- Close button for mobile/tablet drawer -->
      <button
        v-if="mobileOpen"
        class="sidebar-close-btn"
        @click="emit('closeMobile')"
        aria-label="Close navigation menu"
      >
        <BaseIcon name="x" size="sm" />
      </button>

      <!-- Sleek Collapse Toggle Button -->
      <button
        class="sidebar-toggle-btn"
        @click="toggleCollapse"
        aria-label="Toggle sidebar collapse"
        :title="isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
      >
        <span class="toggle-icon">
          <BaseIcon :name="isCollapsed ? 'chevron-right' : 'chevron-left'" size="xs" />
        </span>
      </button>
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
    <nav class="nav-menu" @scroll.passive="handleItemMouseLeave">
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
          <span class="nav-section-title">
            <span class="nav-section-icon">
              <BaseIcon :name="group.icon" size="xs" />
            </span>
            <span>{{ group.label }}</span>
          </span>
          <span class="nav-section-caret">
            <BaseIcon :name="collapsedSections[group.key] ? 'chevron-right' : 'chevron-down'" size="xs" />
          </span>
        </div>

        <!-- Section Navigation Items -->
        <div v-show="isCollapsed || !collapsedSections[group.key]" class="nav-items-container animate-fade-in">
          <router-link
            v-for="item in group.items"
            :key="item.path"
            :to="item.path"
            class="nav-item"
            :class="{ 'nav-active': route.path === item.path || (item.path === '/deployments' && route.path === '/workloads') }"
            :aria-label="item.name"
            @mouseenter="handleItemMouseEnter(item, $event)"
            @mouseleave="handleItemMouseLeave"
            @click="handleItemClick"
          >
            <div class="nav-icon" aria-hidden="true">
              <BaseIcon :name="item.icon" size="sm" />
            </div>
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
          <span class="drawer-tenant-icon" aria-hidden="true">
            <BaseIcon name="layers" size="sm" />
          </span>
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
            <span class="logout-icon" aria-hidden="true">
              <BaseIcon name="lock" size="sm" />
            </span>
            <span>Sign Out</span>
          </button>
        </div>
      </div>

      <div
        class="telemetry-card"
        role="button"
        tabindex="0"
        aria-label="Open ZeroTrust KMS & Dual-Sync Attestation"
        title="ZeroTrust KMS & Dual-Sync Attestation — Click to inspect"
        @click="handleOpenZeroTrust"
        @keydown.enter="handleOpenZeroTrust"
        @keydown.space.prevent="handleOpenZeroTrust"
      >
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

  <!-- Collapsed Rail Floating Tooltip -->
  <Teleport to="body">
    <Transition name="tooltip-fade">
      <div
        v-if="isCollapsed && hoveredItem"
        class="sidebar-rail-tooltip"
        :style="{ top: `${hoveredItem.top}px` }"
        role="tooltip"
      >
        <span class="rail-tooltip-arrow" aria-hidden="true"></span>
        <span class="rail-tooltip-name">{{ hoveredItem.item.name }}</span>
        <span v-if="hoveredItem.item.sub" class="rail-tooltip-sub">{{ hoveredItem.item.sub }}</span>
        <span v-if="hoveredItem.item.badge" class="rail-tooltip-badge">{{ hoveredItem.item.badge }}</span>
      </div>
    </Transition>
  </Teleport>
</template>
