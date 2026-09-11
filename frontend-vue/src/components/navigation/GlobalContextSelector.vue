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
    id: activeClusterId.value || 'staging-k8s', name: activeClusterId.value || 'staging-k8s', status: 'healthy'
  }
})

const clusterStatusDotClass = computed(() => {
  const s = (currentCluster.value.status || '').toLowerCase()
  if (['healthy', 'ready', 'active', 'online', 'ok', 'running'].includes(s)) return 'dot-emerald'
  if (['degraded', 'warning', 'pending'].includes(s)) return 'dot-amber dot-pulse'
  if (['offline', 'failed', 'critical', 'down'].includes(s)) return 'dot-rose'
  return 'dot-slate'
})

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

function onMenuKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') { e.preventDefault(); closeDropdown(true); return }
  const count = activeDropdown.value === 'cluster' ? filteredClusters.value.length : filteredNamespaces.value.length
  if (count === 0) return
  if (e.key === 'ArrowDown') { e.preventDefault(); focusedIndex.value = (focusedIndex.value + 1) % count }
  else if (e.key === 'ArrowUp') { e.preventDefault(); focusedIndex.value = (focusedIndex.value - 1 + count) % count }
  else if (e.key === 'Enter') {
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
      ref="clusterBtnRef" type="button" class="context-pill"
      :class="{ 'is-active': activeDropdown === 'cluster' }" aria-haspopup="listbox"
      :aria-expanded="activeDropdown === 'cluster'" title="Cluster Context: Click to switch" @click="openDropdown('cluster')"
    >
      <BaseIcon name="anchor" size="xs" class="pill-icon text-cyan" />
      <span class="pill-text font-mono">{{ currentCluster.name || currentCluster.id }}</span>
      <span class="status-dot" :class="clusterStatusDotClass" />
      <svg class="pill-caret" width="8" height="6" viewBox="0 0 8 6" fill="none"><path d="M1 1.5L4 4.5L7 1.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/></svg>
    </button>
    <div class="pill-divider" />
    <button
      ref="namespaceBtnRef" type="button" class="context-pill"
      :class="{ 'is-active': activeDropdown === 'namespace' }" aria-haspopup="listbox"
      :aria-expanded="activeDropdown === 'namespace'" title="Namespace Context: Click to switch" @click="openDropdown('namespace')"
    >
      <BaseIcon name="layers" size="xs" class="pill-icon text-muted" />
      <span class="pill-text font-mono">{{ activeNamespaceDisplay }}</span>
      <svg class="pill-caret" width="8" height="6" viewBox="0 0 8 6" fill="none"><path d="M1 1.5L4 4.5L7 1.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/></svg>
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
        </div>
        <div class="context-menu-items" role="presentation">
          <template v-if="activeDropdown === 'cluster'">
            <button
              v-for="(c, idx) in filteredClusters" :key="c.id" type="button" class="context-item"
              :class="{ 'is-selected': c.id === activeClusterId, 'is-focused': idx === focusedIndex }"
              role="option" :aria-selected="c.id === activeClusterId" @mouseenter="focusedIndex = idx" @click="selectCluster(c.id)"
            >
              <div class="item-main">
                <span class="status-dot" :class="c.status === 'healthy' ? 'dot-emerald' : 'dot-amber'" />
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

<style scoped>
.global-context-selector {
  display: inline-flex; align-items: center; height: 32px;
  background: rgba(255, 255, 255, 0.04); border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.12));
  border-radius: 6px; padding: 2px; gap: 2px;
}
.context-pill {
  display: inline-flex; align-items: center; gap: 6px; height: 26px; padding: 0 8px;
  background: transparent; border: 1px solid transparent; border-radius: 4px;
  color: var(--text-primary, #f1f5f9); cursor: pointer; font-size: 11px; transition: all 0.15s ease;
}
.context-pill:hover, .context-pill.is-active { background: rgba(255, 255, 255, 0.08); border-color: rgba(255, 255, 255, 0.1); }
.pill-text { max-width: 110px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pill-divider { width: 1px; height: 16px; background: var(--border-subtle, rgba(255, 255, 255, 0.1)); }
.pill-caret { opacity: 0.5; transition: transform 0.15s ease; }
.context-pill.is-active .pill-caret { transform: rotate(180deg); }
@media (max-width: 1366px) { .pill-text { max-width: 80px; } }
@media (max-width: 768px) { .pill-text { max-width: 75px; } }
</style>

<style>
.global-context-menu {
  position: fixed !important; z-index: 9999; background: #0f172a;
  border: 1px solid rgba(255, 255, 255, 0.12); border-radius: 8px;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.6); padding: 6px;
  display: flex; flex-direction: column; gap: 4px; box-sizing: border-box;
}
.context-menu-header {
  display: flex; align-items: center; gap: 6px; padding: 4px 8px;
  background: rgba(255, 255, 255, 0.05); border-radius: 4px; border: 1px solid rgba(255, 255, 255, 0.08);
}
.context-search-input { flex: 1; background: transparent; border: none; outline: none; color: #f1f5f9; font-size: 11px; }
.context-menu-items { max-height: 220px; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.context-item {
  display: flex; align-items: center; justify-content: space-between; width: 100%;
  padding: 6px 8px; background: transparent; border: none; border-radius: 4px;
  color: #94a3b8; font-size: 11px; cursor: pointer; text-align: left;
}
.context-item:hover, .context-item.is-focused { background: rgba(255, 255, 255, 0.08); color: #fff; }
.context-item.is-selected { color: #38bdf8; font-weight: 600; }
.item-main { display: flex; align-items: center; gap: 6px; }
.item-region { font-size: 10px; color: #64748b; }
.context-empty { padding: 8px; text-align: center; font-size: 11px; color: #64748b; }
</style>
