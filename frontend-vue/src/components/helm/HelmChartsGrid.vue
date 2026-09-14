<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { HelmChart, HelmRepo } from '../../api/helm'

const props = defineProps<{
  charts: HelmChart[]
  repos: HelmRepo[]
  loading: boolean
  search: string
  selectedRepo: string
  selectedCategory: string
  categoryTags: Array<{ key: string; label: string; icon: string }>
}>()

const pageSize = ref<number>(24)
const currentPage = ref<number>(1)

const totalPages = computed(() => Math.max(1, Math.ceil(props.charts.length / pageSize.value)))

const paginatedCharts = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return props.charts.slice(start, start + pageSize.value)
})

watch([() => props.search, () => props.selectedRepo, () => props.selectedCategory], () => {
  currentPage.value = 1
})

watch(() => props.charts.length, () => {
  if (currentPage.value > totalPages.value) {
    currentPage.value = 1
  }
})

const emit = defineEmits<{
  (e: 'update:search', val: string): void
  (e: 'update:selectedRepo', val: string): void
  (e: 'update:selectedCategory', val: string): void
  (e: 'searchInput'): void
  (e: 'install', chart: HelmChart): void
  (e: 'addRepo'): void
}>()

function getChartIcon(chart: HelmChart): string {
  const name = (chart.name || '').toLowerCase()
  if (name.includes('nginx') || name.includes('ingress') || name.includes('traefik')) return 'globe'
  if (name.includes('postgres') || name.includes('mysql') || name.includes('mariadb') || name.includes('redis') || name.includes('mongo')) return 'database'
  if (name.includes('prom') || name.includes('grafana') || name.includes('loki') || name.includes('metric')) return 'activity'
  if (name.includes('cert') || name.includes('vault') || name.includes('auth') || name.includes('keycloak')) return 'shield'
  if (name.includes('kafka') || name.includes('rabbit') || name.includes('queue') || name.includes('nats')) return 'zap'
  if (name.includes('elastic') || name.includes('search') || name.includes('opensearch')) return 'search'
  if (name.includes('ai') || name.includes('ollama') || name.includes('vllm') || name.includes('llm')) return 'bot'
  return 'package'
}
</script>

<template>
  <div class="helm-charts-grid-container">
    <!-- Search & Filters Toolbar -->
    <div class="catalog-filters-panel glass-panel">
      <div class="search-row">
        <div class="catalog-search-box">
          <BaseIcon name="search" size="xs" class="search-icon" />
          <input
            :value="search"
            type="text"
            placeholder="Search Helm charts (e.g. nginx, redis, postgres, prometheus)..."
            class="input-glass catalog-search-input"
            @input="emit('update:search', ($event.target as HTMLInputElement).value); emit('searchInput')"
          />
          <button
            v-if="search"
            type="button"
            class="btn-clear"
            @click="emit('update:search', ''); emit('searchInput')"
          >
            <BaseIcon name="x" size="xs" />
          </button>
        </div>

        <div class="repo-filter-box">
          <label class="filter-label">Repository:</label>
          <select
            :value="selectedRepo"
            class="input-glass select-repo"
            @change="emit('update:selectedRepo', ($event.target as HTMLSelectElement).value)"
          >
            <option value="all">All Repositories</option>
            <option v-for="repo in repos" :key="repo.name" :value="repo.name">
              {{ repo.name }}
            </option>
          </select>
        </div>
      </div>

      <!-- Quick Category Chips -->
      <div class="category-chips-row">
        <button
          v-for="cat in categoryTags"
          :key="cat.key"
          type="button"
          class="chip-btn"
          :class="{ active: selectedCategory === cat.key }"
          @click="emit('update:selectedCategory', cat.key)"
        >
          <span class="chip-icon">{{ cat.icon }}</span>
          <span class="chip-label">{{ cat.label }}</span>
        </button>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-state glass-panel">
      <div class="cyber-spinner"></div>
      <p class="font-mono text-muted">Searching & indexing Helm chart repositories...</p>
    </div>

    <!-- Empty State -->
    <div v-else-if="charts.length === 0" class="empty-state glass-panel">
      <BaseIcon name="package" size="xl" class="empty-icon" />
      <h4 class="empty-title">No Charts Found</h4>
      <p class="empty-desc">
        No charts match your search query or filter. Try searching for other keywords or adding new Helm repositories.
      </p>
      <button type="button" class="btn-cyber btn-primary" @click="emit('addRepo')">
        + Add Repository
      </button>
    </div>

    <!-- Charts Grid -->
    <div v-else class="charts-grid">
      <div
        v-for="chart in paginatedCharts"
        :key="`${chart.repo}/${chart.name}`"
        class="chart-card glass-panel glass-panel-glow"
      >
        <div class="chart-card-header">
          <div class="chart-avatar">
            <img
              v-if="chart.icon && chart.icon.startsWith('http')"
              :src="chart.icon"
              :alt="chart.name"
              class="chart-icon-img"
              @error="($event.target as HTMLElement).style.display = 'none'"
            />
            <BaseIcon :name="getChartIcon(chart)" size="md" class="chart-fallback-icon" />
          </div>
          <div class="chart-meta">
            <div class="chart-repo-tag">
              <span class="repo-badge"><BaseIcon name="database" size="xs" /> {{ chart.repo }}</span>
            </div>
            <h3 class="chart-title" :title="chart.name">{{ chart.name }}</h3>
          </div>
        </div>

        <div class="chart-card-body">
          <p class="chart-desc">
            {{ chart.description || 'Cloud-native application packaged as a standardized Helm chart.' }}
          </p>

          <div class="chart-badges-row">
            <span class="badge-tag badge-cyan font-mono">v{{ chart.version }}</span>
            <span v-if="chart.appVersion || chart.app_version" class="badge-tag badge-violet font-mono">
              app: {{ chart.appVersion || chart.app_version }}
            </span>
          </div>

          <!-- Keywords Chips -->
          <div v-if="chart.keywords && chart.keywords.length > 0" class="chart-keywords">
            <span
              v-for="kw in chart.keywords.slice(0, 3)"
              :key="kw"
              class="keyword-pill font-xs font-mono"
            >
              #{{ kw }}
            </span>
          </div>
        </div>

        <div class="chart-card-footer">
          <button
            type="button"
            class="btn-cyber btn-primary btn-install-chart"
            @click="emit('install', chart)"
          >
            <BaseIcon name="play" size="xs" /> <span>Install Chart</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Pagination Controls -->
    <div v-if="!loading && charts.length > 0" class="catalog-pagination-bar glass-panel font-mono">
      <div class="pagination-info">
        <span>Showing {{ ((currentPage - 1) * pageSize) + 1 }}-{{ Math.min(currentPage * pageSize, charts.length) }} of {{ charts.length }} charts</span>
        <div class="page-size-selector">
          <label class="font-xs text-muted">Per page:</label>
          <select v-model.number="pageSize" class="input-glass select-page-size" @change="currentPage = 1">
            <option :value="24">24</option>
            <option :value="48">48</option>
          </select>
        </div>
      </div>

      <div class="pagination-controls">
        <button
          type="button"
          class="btn-cyber btn-secondary btn-sm"
          :disabled="currentPage <= 1"
          @click="currentPage--"
        >
          <BaseIcon name="chevron-left" size="xs" /> <span>Prev</span>
        </button>
        <span class="page-indicator">Page {{ currentPage }} of {{ totalPages }}</span>
        <button
          type="button"
          class="btn-cyber btn-secondary btn-sm"
          :disabled="currentPage >= totalPages"
          @click="currentPage++"
        >
          <span>Next</span> <BaseIcon name="chevron-right" size="xs" />
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/helm-catalog-grid.css';
</style>
