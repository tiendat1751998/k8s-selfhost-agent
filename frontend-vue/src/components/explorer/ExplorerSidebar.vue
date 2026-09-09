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
@import '../../assets/styles/views/explorer.css';
</style>
