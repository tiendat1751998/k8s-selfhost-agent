<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import { useGlobalContext } from '../../composables/useGlobalContext'

const {
  activeClusterId, activeNamespace, availableClusters,
  availableNamespaces, setCluster, setNamespace, loadClusters
} = useGlobalContext()

type DropdownType = 'cluster' | 'namespace' | null
const activeDropdown = ref<DropdownType>(null)
const searchQuery = ref('')
const focusedIndex = ref(0)
watch(searchQuery, () => { focusedIndex.value = 0 })
const clusterBtnRef = ref<HTMLButtonElement | null>(null)
const namespaceBtnRef = ref<HTMLButtonElement | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)
const menuRef = ref<HTMLDivElement | null>(null)
const menuPos = ref({ top: 0, left: 0, width: 230 })

const currentCluster = computed(() => {
  return availableClusters.value.find(c => c.id === activeClusterId.value) || {
    id: activeClusterId.value || 'local',
    name: activeClusterId.value || 'local',
    status: 'unknown',
    health_status: 'unknown',
  }
})

function getClusterDotClass(status?: string, healthStatus?: string): string {
  const hs = (healthStatus || '').toLowerCase().trim()
  const s = (status || '').toLowerCase().trim()

  if (['degraded', 'warning', 'pending', 'elevated'].includes(hs)) return 'dot-amber dot-pulse'
  if (['offline', 'disconnected', 'unhealthy', 'error', 'failed', 'critical', 'down'].includes(hs)) return 'dot-rose'
  if (hs === 'unknown') return 'dot-slate'

  if (['degraded', 'warning', 'pending', 'elevated'].includes(s)) return 'dot-amber dot-pulse'
  if (['offline', 'failed', 'critical', 'down', 'error', 'disconnected', 'unhealthy'].includes(s)) return 'dot-rose'

  if (['healthy', 'ready', 'active', 'online', 'ok', 'running', 'connected'].includes(hs)) return 'dot-emerald'
  if (['healthy', 'ready', 'active', 'online', 'ok', 'running', 'connected'].includes(s)) return 'dot-emerald'

  return 'dot-slate'
}

const clusterStatusDotClass = computed(() => getClusterDotClass(currentCluster.value.status, currentCluster.value.health_status))

const activeNamespaceDisplay = computed(() => (!activeNamespace.value || activeNamespace.value === 'all') ? 'All Namespaces' : activeNamespace.value)

const filteredClusters = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return q ? availableClusters.value.filter(c => (c.name + c.id + (c.region || '')).toLowerCase().includes(q)) : availableClusters.value
})

const filteredNamespaces = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return q ? availableNamespaces.value.filter(ns => (ns === 'all' ? 'all namespaces' : ns).toLowerCase().includes(q)) : availableNamespaces.value
})

function updateMenuPos() {
  const btn = activeDropdown.value === 'cluster' ? clusterBtnRef.value : namespaceBtnRef.value
  if (!btn) return
  const r = btn.getBoundingClientRect()
  const vw = window.innerWidth, width = 230
  let left = r.left
  if (left + width > vw - 12) left = vw - width - 12
  if (left < 12) left = 12
  menuPos.value = { top: Math.round(r.bottom + 4), left: Math.round(left), width }
}

function openDropdown(type: DropdownType) {
  if (activeDropdown.value === type) { closeDropdown(); return }
  activeDropdown.value = type
  searchQuery.value = ''
  focusedIndex.value = 0
  nextTick(() => { updateMenuPos(); searchInputRef.value?.focus() })
}

function closeDropdown(refocus = false) {
  const prev = activeDropdown.value
  activeDropdown.value = null
  searchQuery.value = ''
  focusedIndex.value = 0
  if (refocus) {
    if (prev === 'cluster') clusterBtnRef.value?.focus()
    else if (prev === 'namespace') namespaceBtnRef.value?.focus()
  }
}

function selectCluster(id: string) { setCluster(id); closeDropdown(true) }
function selectNamespace(ns: string) { setNamespace(ns); closeDropdown(true) }

function scrollFocusedIntoView(index: number) {
  nextTick(() => {
    const el = menuRef.value?.querySelectorAll('.context-item')[index] as HTMLElement | undefined
    el?.scrollIntoView({ block: 'nearest' })
  })
}

function onMenuKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') { e.preventDefault(); closeDropdown(true); return }
  const count = activeDropdown.value === 'cluster' ? filteredClusters.value.length : filteredNamespaces.value.length
  if (count === 0) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    focusedIndex.value = (focusedIndex.value + 1) % count
    scrollFocusedIntoView(focusedIndex.value)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    focusedIndex.value = (focusedIndex.value - 1 + count) % count
    scrollFocusedIntoView(focusedIndex.value)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (activeDropdown.value === 'cluster') {
      const target = filteredClusters.value[focusedIndex.value]
      if (target) selectCluster(target.id)
    } else if (activeDropdown.value === 'namespace') {
      const target = filteredNamespaces.value[focusedIndex.value]
      if (target) selectNamespace(target)
    }
  }
}

function onDocClick(e: MouseEvent) {
  if (!activeDropdown.value) return
  const t = e.target as Node | null
  if (menuRef.value?.contains(t) || clusterBtnRef.value?.contains(t) || namespaceBtnRef.value?.contains(t)) return
  closeDropdown()
}

onMounted(() => {
  loadClusters().catch(() => {})
  document.addEventListener('click', onDocClick, true)
  window.addEventListener('resize', updateMenuPos)
})

onUnmounted(() => {
  document.removeEventListener('click', onDocClick, true)
  window.removeEventListener('resize', updateMenuPos)
})
</script>

<template>
  <div class="global-context-selector">
    <button
      ref="clusterBtnRef"
      type="button"
      class="context-pill cluster-pill"
      :class="{ 'is-active': activeDropdown === 'cluster' }"
      aria-haspopup="listbox"
      :aria-expanded="activeDropdown === 'cluster'"
      title="Cluster Context: Click to switch"
      @click="openDropdown('cluster')"
    >
      <span class="status-dot" :class="clusterStatusDotClass" />
      <span class="pill-prefix">Cluster</span>
      <span class="pill-text">{{ currentCluster.name || currentCluster.id }}</span>
      <BaseIcon name="chevron-down" size="xs" class="pill-caret" />
    </button>
    <span class="pill-divider" aria-hidden="true"></span>
    <button
      ref="namespaceBtnRef"
      type="button"
      class="context-pill namespace-pill"
      :class="{ 'is-active': activeDropdown === 'namespace' }"
      aria-haspopup="listbox"
      :aria-expanded="activeDropdown === 'namespace'"
      title="Namespace Context: Click to switch"
      @click="openDropdown('namespace')"
    >
      <span class="pill-prefix">NS</span>
      <span class="pill-text">{{ activeNamespaceDisplay }}</span>
      <BaseIcon name="chevron-down" size="xs" class="pill-caret" />
    </button>
    <Teleport to="body">
      <div
        v-if="activeDropdown" ref="menuRef" class="global-context-menu"
        :style="{ top: `${menuPos.top}px`, left: `${menuPos.left}px`, width: `${menuPos.width}px` }" role="listbox"
      >
        <div class="context-menu-header">
          <BaseIcon name="search" size="xs" class="search-ico" />
          <input
            ref="searchInputRef" v-model="searchQuery" type="text" class="context-search-input"
            :placeholder="activeDropdown === 'cluster' ? 'Search clusters...' : 'Search namespaces...'" @keydown="onMenuKeydown"
          />
          <span class="context-count-badge font-mono">
            {{ activeDropdown === 'cluster' ? filteredClusters.length : filteredNamespaces.length }}
          </span>
        </div>
        <div class="context-menu-items" role="presentation">
          <template v-if="activeDropdown === 'cluster'">
            <button
              v-for="(c, idx) in filteredClusters" :key="c.id" type="button" class="context-item"
              :class="{ 'is-selected': c.id === activeClusterId, 'is-focused': idx === focusedIndex }"
              role="option" :aria-selected="c.id === activeClusterId" @mouseenter="focusedIndex = idx" @click="selectCluster(c.id)"
            >
              <div class="item-main">
                <span class="status-dot" :class="getClusterDotClass(c.status, c.health_status)" />
                <span class="item-name font-mono">{{ c.name || c.id }}</span>
              </div>
              <span v-if="c.region" class="item-region">{{ c.region }}</span>
            </button>
            <div v-if="filteredClusters.length === 0" class="context-empty">No clusters found</div>
          </template>
          <template v-else-if="activeDropdown === 'namespace'">
            <button
              v-for="(ns, idx) in filteredNamespaces" :key="ns" type="button" class="context-item"
              :class="{ 'is-selected': ns === activeNamespace, 'is-focused': idx === focusedIndex }"
              role="option" :aria-selected="ns === activeNamespace" @mouseenter="focusedIndex = idx" @click="selectNamespace(ns)"
            >
              <BaseIcon name="layers" size="xs" class="item-icon" />
              <span class="item-name font-mono">{{ ns === 'all' ? 'All Namespaces' : ns }}</span>
            </button>
            <div v-if="filteredNamespaces.length === 0" class="context-empty">No namespaces found</div>
          </template>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style src="../../assets/styles/components/global-context-selector.css"></style>
