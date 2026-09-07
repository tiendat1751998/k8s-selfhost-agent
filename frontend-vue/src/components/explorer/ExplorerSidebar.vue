<script setup lang="ts">
import type { Cluster } from '../../api/compute'
import type { K8sNamespace, ResourceKind } from '../../api/k8s'
import type { KindCategory } from '../../composables/useK8sExplorer'

defineProps<{
  clusters: Cluster[]
  selectedCluster: string
  namespaces: K8sNamespace[]
  selectedNamespace: string
  selectedKind: ResourceKind
  kindSearchQuery: string
  filteredKindCategories: KindCategory[]
  isMobileSidebarOpen: boolean
}>()

const emit = defineEmits<{
  (e: 'update:selectedCluster', val: string): void
  (e: 'update:selectedNamespace', val: string): void
  (e: 'update:kindSearchQuery', val: string): void
  (e: 'update:isMobileSidebarOpen', val: boolean): void
  (e: 'select-kind', kind: ResourceKind): void
  (e: 'import-cluster'): void
  (e: 'new-namespace'): void
}>()
</script>

<template>
  <aside class="explorer-sidebar glass-panel" :class="{ 'mobile-sidebar-open': isMobileSidebarOpen }">
    <div class="sidebar-header">
      <div class="sidebar-brand">
        <span class="pulse-beacon"></span>
        <div class="brand-text-col">
          <span class="sidebar-title">K8s Explorer</span>
          <span class="sidebar-sub font-mono">18 Resources</span>
        </div>
      </div>
      <button type="button" class="mobile-close-btn" aria-label="Close Resource Tree" @click="emit('update:isMobileSidebarOpen', false)">✕</button>
    </div>

    <!-- Quick Resource Filter Search -->
    <div class="sidebar-search-box">
      <span class="sidebar-search-icon">
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/></svg>
      </span>
      <input
        :value="kindSearchQuery"
        type="text"
        placeholder="Filter 18 kinds (e.g. pod, pvc)..."
        class="input-glass sidebar-search-input font-mono"
        @input="emit('update:kindSearchQuery', ($event.target as HTMLInputElement).value)"
      />
      <button v-if="kindSearchQuery" type="button" class="sidebar-search-clear" title="Clear filter" @click="emit('update:kindSearchQuery', '')">✕</button>
    </div>

    <!-- Target Cluster Selector -->
    <div class="sidebar-section">
      <div class="section-heading-row">
        <label class="section-heading">Cluster Target</label>
        <button type="button" class="pill-action-btn" title="Import Cluster" @click="emit('import-cluster')">+ Import</button>
      </div>
      <select
        v-if="clusters.length > 0"
        :value="selectedCluster"
        class="input-glass select-full cluster-select font-mono"
        @change="emit('update:selectedCluster', ($event.target as HTMLSelectElement).value)"
      >
        <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">🌐 {{ c.name || c.id }}</option>
      </select>
      <div v-else class="cluster-empty-box">
        <span class="font-mono text-muted font-small">{{ selectedCluster || 'primary-cluster' }} (offline)</span>
      </div>
    </div>

    <!-- Target Namespace Selector -->
    <div class="sidebar-section">
      <div class="section-heading-row">
        <label class="section-heading">Namespace Scope</label>
        <button type="button" class="pill-action-btn" title="Create namespace" @click="emit('new-namespace')">+ New</button>
      </div>
      <select
        :value="selectedNamespace"
        class="input-glass select-full ns-select font-mono"
        @change="emit('update:selectedNamespace', ($event.target as HTMLSelectElement).value)"
      >
        <option value="all">🌐 All Namespaces</option>
        <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">📁 {{ ns.name }}</option>
      </select>
    </div>

    <!-- Resource Kinds Tree -->
    <div class="sidebar-tree custom-sidebar-scroll">
      <div v-if="filteredKindCategories.length === 0" class="sidebar-no-results font-mono">
        <span>No matching resource kind</span>
        <button type="button" class="btn-link font-xs" @click="emit('update:kindSearchQuery', '')">Reset filter</button>
      </div>
      <div v-for="category in filteredKindCategories" :key="category.title" class="tree-category">
        <div class="category-title">
          <span class="category-name">{{ category.title }}</span>
          <span class="category-badge">{{ category.items.length }}</span>
        </div>
        <div class="category-items">
          <button
            v-for="item in category.items"
            :key="item.kind"
            type="button"
            class="tree-item-btn"
            :class="{ 'is-active': selectedKind === item.kind }"
            @click="emit('select-kind', item.kind)"
          >
            <span class="active-indicator"></span>
            <span class="item-label font-mono">{{ item.label }}</span>
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.explorer-sidebar {
  width: 270px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  max-height: calc(100vh - 100px);
  background: rgba(11, 15, 25, 0.85);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  box-shadow: 0 10px 30px -10px rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(20px);
  z-index: 20;
}

.custom-sidebar-scroll {
  overflow-y: auto;
  padding-right: 4px;
  max-height: calc(100vh - 340px);
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.sidebar-brand { display: flex; align-items: center; gap: 10px; }
.pulse-beacon {
  width: 10px; height: 10px; border-radius: 50%;
  background-color: #06b6d4;
  box-shadow: 0 0 12px #06b6d4;
}

.sidebar-title { font-weight: 800; font-size: 0.95rem; color: #fff; text-transform: uppercase; }
.sidebar-sub { font-size: 0.68rem; color: #06b6d4; }
.mobile-close-btn { display: none; background: none; border: none; color: #94a3b8; font-size: 1.1rem; cursor: pointer; }

.sidebar-search-box { position: relative; display: flex; align-items: center; }
.sidebar-search-icon { position: absolute; left: 10px; color: #64748b; }
.sidebar-search-input { width: 100%; padding: 7px 28px 7px 30px; font-size: 0.76rem; }
.sidebar-search-clear { position: absolute; right: 8px; background: none; border: none; color: #64748b; cursor: pointer; }

.sidebar-section { display: flex; flex-direction: column; gap: 6px; }
.section-heading-row { display: flex; align-items: center; justify-content: space-between; }
.section-heading { font-size: 0.7rem; font-weight: 700; color: #64748b; text-transform: uppercase; font-family: var(--font-mono); }
.pill-action-btn { background: rgba(6, 182, 212, 0.08); border: 1px solid rgba(6, 182, 212, 0.25); color: #38bdf8; font-size: 0.68rem; padding: 2px 8px; border-radius: 9999px; cursor: pointer; }

.tree-category { display: flex; flex-direction: column; gap: 3px; margin-bottom: 8px; }
.category-title { display: flex; justify-content: space-between; font-size: 0.68rem; font-weight: 700; color: #64748b; text-transform: uppercase; padding: 4px 6px; font-family: var(--font-mono); }
.category-badge { background: rgba(255, 255, 255, 0.05); padding: 1px 5px; border-radius: 4px; font-size: 0.65rem; }
.category-items { display: flex; flex-direction: column; gap: 2px; }

.tree-item-btn {
  position: relative; display: flex; align-items: center; gap: 8px; padding: 6px 12px;
  background: none; border: 1px solid transparent; border-radius: 8px; color: #94a3b8;
  cursor: pointer; text-align: left; font-size: 0.8rem;
}
.tree-item-btn:hover { background: rgba(255, 255, 255, 0.04); color: #f1f5f9; }
.tree-item-btn.is-active { background: rgba(6, 182, 212, 0.12); border-color: rgba(6, 182, 212, 0.3); color: #38bdf8; font-weight: 600; }

@media (max-width: 1024px) {
  .explorer-sidebar {
    position: fixed; top: 0; left: 0; bottom: 0; width: 290px; max-height: 100vh;
    transform: translateX(-105%); transition: transform 0.3s ease; z-index: 1000;
  }
  .explorer-sidebar.mobile-sidebar-open { transform: translateX(0); }
  .mobile-close-btn { display: flex; }
}
</style>
