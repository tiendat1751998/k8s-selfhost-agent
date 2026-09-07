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
.helm-charts-grid-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.catalog-filters-panel {
  padding: 14px 18px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.search-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.catalog-search-box {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 280px;
}
.search-icon {
  position: absolute;
  left: 12px;
  font-size: 14px;
  pointer-events: none;
  opacity: 0.6;
}
.catalog-search-input {
  width: 100%;
  padding: 10px 36px 10px 36px;
  border-radius: 8px;
  font-size: 14px;
}
.btn-clear {
  position: absolute;
  right: 10px;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
}
.repo-filter-box {
  display: flex;
  align-items: center;
  gap: 8px;
}
.filter-label {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}
.select-repo {
  padding: 9px 12px;
  border-radius: 8px;
  font-size: 13px;
  min-width: 180px;
}
.category-chips-row {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
}
.chip-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  transition: all var(--transition-fast);
}
.chip-btn:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.08);
}
.chip-btn.active {
  background: rgba(252, 213, 53, 0.12);
  border-color: rgba(252, 213, 53, 0.4);
  color: var(--color-primary);
}
.charts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}
.chart-card {
  padding: 20px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 14px;
  transition: transform var(--transition-normal), border-color var(--transition-normal);
}
.chart-card:hover {
  transform: translateY(-2px);
  border-color: rgba(252, 213, 53, 0.35);
}
.chart-card-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.chart-avatar {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
}
.chart-icon-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.chart-fallback-icon {
  font-size: 22px;
}
.chart-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow: hidden;
}
.chart-repo-tag {
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  text-transform: uppercase;
}
.chart-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.chart-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.45;
  margin: 0 0 10px 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.chart-badges-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
.chart-keywords {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}
.keyword-pill {
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.04);
  padding: 1px 6px;
  border-radius: 4px;
}
.btn-install-chart {
  width: 100%;
  justify-content: center;
  padding: 9px 14px;
}
.loading-state, .empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  text-align: center;
  gap: 12px;
  border-radius: 14px;
}
.cyber-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid rgba(252, 213, 53, 0.15);
  border-top-color: var(--color-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
.empty-icon {
  font-size: 38px;
}
.empty-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}
.empty-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 420px;
  margin: 0;
}
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
