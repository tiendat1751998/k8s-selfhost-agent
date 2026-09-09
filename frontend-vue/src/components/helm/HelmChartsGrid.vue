<script setup lang="ts">
import type { HelmChart, HelmRepo } from '../../api/helm'

defineProps<{
  charts: HelmChart[]
  repos: HelmRepo[]
  loading: boolean
  search: string
  selectedRepo: string
  selectedCategory: string
  categoryTags: Array<{ key: string; label: string; icon: string }>
}>()

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
  if (name.includes('nginx') || name.includes('ingress') || name.includes('traefik')) return '🌐'
  if (name.includes('postgres') || name.includes('mysql') || name.includes('mariadb') || name.includes('redis') || name.includes('mongo')) return '🗄️'
  if (name.includes('prom') || name.includes('grafana') || name.includes('loki') || name.includes('metric')) return '📊'
  if (name.includes('cert') || name.includes('vault') || name.includes('auth') || name.includes('keycloak')) return '🛡️'
  if (name.includes('kafka') || name.includes('rabbit') || name.includes('queue') || name.includes('nats')) return '⚡'
  if (name.includes('elastic') || name.includes('search') || name.includes('opensearch')) return '🔍'
  if (name.includes('ai') || name.includes('ollama') || name.includes('vllm') || name.includes('llm')) return '🤖'
  return '📦'
}
</script>

<template>
  <div class="helm-charts-grid-container">
    <!-- Search & Filters Toolbar -->
    <div class="catalog-filters-panel glass-panel">
      <div class="search-row">
        <div class="catalog-search-box">
          <span class="search-icon">🔍</span>
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
            ✕
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
              🗄️ {{ repo.name }}
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
      <span class="empty-icon">📦</span>
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
        v-for="chart in charts"
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
            <span class="chart-fallback-icon">{{ getChartIcon(chart) }}</span>
          </div>
          <div class="chart-meta">
            <div class="chart-repo-tag">
              <span class="repo-badge">🗄️ {{ chart.repo }}</span>
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
            <span>🚀 Install Chart</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/helm.css';
</style>
