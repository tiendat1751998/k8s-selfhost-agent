<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import type { HelmRepo } from '../../api/helm'

interface ClusterItem {
  id?: string
  name?: string
}

interface NamespaceItem {
  name: string
}

defineProps<{
  activeTab: 'releases' | 'charts' | 'repos'
  releasesCount: number
  chartsCount: number
  reposCount: number
  totalReleasesCount: number
  deployedRate: string
  searchQuery: string
  searchPlaceholder: string
  releaseStatusFilter: string
  selectedRepoFilter: string
  repos: HelmRepo[]
  loading: boolean
  clusters: ClusterItem[]
  selectedCluster: string
  namespaces: NamespaceItem[]
  selectedNamespace: string
}>()

const emit = defineEmits<{
  (e: 'update:activeTab', tab: 'releases' | 'charts' | 'repos'): void
  (e: 'update:searchQuery', query: string): void
  (e: 'update:releaseStatusFilter', filter: string): void
  (e: 'update:selectedRepoFilter', filter: string): void
  (e: 'update:selectedCluster', cluster: string): void
  (e: 'update:selectedNamespace', ns: string): void
  (e: 'addRepo'): void
  (e: 'refresh'): void
}>()
</script>

<template>
  <div class="helm-catalog-toolbar-wrapper">
    <!-- Unified 42px Sleek Enterprise Toolbar (Desktop Only) -->
    <div class="helm-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="Helm Enterprise Toolbar">
      <!-- Left: Segmented capsule tabs -->
      <div class="toolbar-capsule-pills" role="tablist" aria-label="Helm Catalog Sections">
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'releases'"
          class="capsule-pill font-mono"
          :class="{ active: activeTab === 'releases' }"
          @click="emit('update:activeTab', 'releases')"
        >
          <BaseIcon name="anchor" size="xs" class="capsule-icon" />
          <span>Releases</span>
          <span class="capsule-count">({{ releasesCount }})</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'charts'"
          class="capsule-pill font-mono"
          :class="{ active: activeTab === 'charts' }"
          @click="emit('update:activeTab', 'charts')"
        >
          <BaseIcon name="package" size="xs" class="capsule-icon" />
          <span>Chart Catalog</span>
          <span class="capsule-count">({{ chartsCount }})</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'repos'"
          class="capsule-pill font-mono"
          :class="{ active: activeTab === 'repos' }"
          @click="emit('update:activeTab', 'repos')"
        >
          <BaseIcon name="database" size="xs" class="capsule-icon" />
          <span>Repositories</span>
          <span class="capsule-count">({{ reposCount }})</span>
        </button>
      </div>

      <!-- Center-Left: Standard 30px Capsule Pill search input -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          :value="searchQuery"
          type="text"
          :placeholder="searchPlaceholder"
          class="toolbar-search-input"
          aria-label="Filter or search Helm resources"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="clear-input-btn"
          aria-label="Clear search"
          @click="emit('update:searchQuery', '')"
        >
          <BaseIcon name="x" size="xs" />
        </button>
      </div>

      <!-- Center-Right: Contextual filter -->
      <div class="toolbar-context-filter">
        <select
          v-if="activeTab === 'releases'"
          :value="releaseStatusFilter"
          class="toolbar-select font-mono"
          aria-label="Filter Releases by Status"
          @change="emit('update:releaseStatusFilter', ($event.target as HTMLSelectElement).value)"
        >
          <option value="all">All Statuses ({{ releasesCount }})</option>
          <option value="deployed">Deployed</option>
          <option value="failed">Failed</option>
          <option value="pending">Pending</option>
          <option value="superseded">Superseded</option>
        </select>
        <select
          v-else-if="activeTab === 'charts'"
          :value="selectedRepoFilter"
          class="toolbar-select font-mono"
          aria-label="Filter Charts by Repository"
          @change="emit('update:selectedRepoFilter', ($event.target as HTMLSelectElement).value)"
        >
          <option value="all">All Repositories ({{ reposCount }})</option>
          <option v-for="repo in repos" :key="repo.name" :value="repo.name">
            {{ repo.name }}
          </option>
        </select>
      </div>

      <!-- Right: Action buttons -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="toolbar-btn btn-primary"
          title="Add Helm Repository"
          @click="emit('addRepo')"
        >
          <BaseIcon name="plus" size="xs" />
          <span>Add Repo</span>
        </button>
        <button
          type="button"
          class="toolbar-btn btn-secondary toolbar-btn-icon"
          :disabled="loading"
          title="Refresh view"
          aria-label="Refresh view"
          @click="emit('refresh')"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': loading }" />
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<=640px) -->
    <div class="helm-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="anchor" size="sm" /> Helm ({{ releasesCount }})</span>
      </div>
      <div class="command-bar-actions">
        <button type="button" class="btn-icon-cmd" title="Add Repository" aria-label="Add Repository" @click="emit('addRepo')">
          <BaseIcon name="plus" size="xs" />
        </button>
        <button type="button" class="btn-icon-cmd" :disabled="loading" title="Refresh" aria-label="Refresh" @click="emit('refresh')">
          <BaseIcon name="refresh" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<=640px) -->
    <div class="helm-micro-telemetry mobile-only font-mono" role="status" aria-label="Helm Micro Telemetry">
      <span class="tel-item tel-rel"><BaseIcon name="anchor" size="xs" /> {{ totalReleasesCount }} rel</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-ok"><BaseIcon name="shield" size="xs" /> {{ deployedRate }} ok</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-charts"><BaseIcon name="package" size="xs" /> {{ chartsCount }} charts</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-repos"><BaseIcon name="database" size="xs" /> {{ reposCount }} repos</span>
    </div>

    <!-- Slim Mobile Cluster & Namespace Pill Row (<=640px) -->
    <div class="helm-mobile-cluster-bar helm-mobile-selectors mobile-only">
      <div class="pill-select-wrap">
        <BaseIcon name="anchor" size="xs" class="pill-prefix" />
        <select
          :value="selectedCluster"
          class="pill-select"
          aria-label="Target Cluster"
          @change="emit('update:selectedCluster', ($event.target as HTMLSelectElement).value)"
        >
          <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">{{ c.name || c.id }}</option>
          <option v-if="clusters.length === 0" value="primary-cluster">primary-cluster</option>
        </select>
      </div>
      <div class="pill-select-wrap">
        <BaseIcon name="grid" size="xs" class="pill-prefix" />
        <select
          :value="selectedNamespace"
          class="pill-select"
          aria-label="Namespace Scope"
          @change="emit('update:selectedNamespace', ($event.target as HTMLSelectElement).value)"
        >
          <option value="all">all namespaces</option>
          <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">{{ ns.name }}</option>
        </select>
      </div>
    </div>

    <!-- Mobile Section Tabs Navigation (<768px) -->
    <nav class="catalog-tabs-bar mobile-only glass-panel" aria-label="Helm Catalog Sections">
      <div class="tabs-list">
        <button type="button" class="tab-btn" :class="{ active: activeTab === 'releases' }" @click="emit('update:activeTab', 'releases')">
          <BaseIcon name="anchor" size="xs" class="tab-icon" /><span class="tab-label">Releases</span><span class="tab-counter">{{ releasesCount }}</span>
        </button>
        <button type="button" class="tab-btn" :class="{ active: activeTab === 'charts' }" @click="emit('update:activeTab', 'charts')">
          <BaseIcon name="package" size="xs" class="tab-icon" /><span class="tab-label">Charts</span><span class="tab-counter">{{ chartsCount }}</span>
        </button>
        <button type="button" class="tab-btn" :class="{ active: activeTab === 'repos' }" @click="emit('update:activeTab', 'repos')">
          <BaseIcon name="database" size="xs" class="tab-icon" /><span class="tab-label">Repos</span><span class="tab-counter">{{ reposCount }}</span>
        </button>
      </div>
    </nav>
  </div>
</template>
