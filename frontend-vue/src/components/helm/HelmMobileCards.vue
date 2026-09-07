<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { HelmRelease, HelmChart, HelmRepo } from '../../api/helm'
import { getFormattedReleaseChart, getFormattedReleaseDescription } from '../../composables/useHelm'

defineProps<{
  activeTab: 'releases' | 'charts' | 'repos'
  releases: HelmRelease[]
  charts: HelmChart[]
  repos: HelmRepo[]
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'openDetail', rel: HelmRelease): void
  (e: 'upgrade', rel: HelmRelease): void
  (e: 'rollback', rel: HelmRelease): void
  (e: 'uninstall', rel: HelmRelease): void
  (e: 'installChart', chart: HelmChart): void
  (e: 'removeRepo', repo: HelmRepo): void
  (e: 'copyUrl', url: string): void
}>()

function getStatusType(status: string): string {
  const s = (status || '').toLowerCase()
  if (s.includes('deploy') || s === 'active' || s === 'success') return 'deployed'
  if (s.includes('fail') || s.includes('error')) return 'failed'
  if (s.includes('pend') || s.includes('upgrad') || s.includes('install')) return 'pending'
  if (s.includes('super') || s.includes('uninst')) return 'superseded'
  return 'unknown'
}

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

function formatReleaseDate(dateStr?: string): string {
  if (!dateStr) return '—'
  try {
    const d = new Date(dateStr)
    if (isNaN(d.getTime())) return dateStr
    const diff = Date.now() - d.getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 1) return 'Just now'
    if (mins < 60) return `${mins}m ago`
    const hours = Math.floor(mins / 60)
    if (hours < 24) return `${hours}h ago`
    return d.toLocaleDateString()
  } catch {
    return dateStr
  }
}
</script>

<template>
  <div class="helm-mobile-cards-stream">
    <!-- Releases Cards Stream -->
    <div v-if="activeTab === 'releases'" class="mobile-cards-list">
      <div
        v-for="rel in releases"
        :key="`${rel.namespace}/${rel.name}`"
        class="mobile-card glass-panel"
      >
        <div class="card-top-row" @click="emit('openDetail', rel)">
          <div class="card-identity">
            <span class="card-icon">⛵</span>
            <div class="card-identity-text">
              <strong class="card-title font-mono">{{ rel.name }}</strong>
              <span class="card-subtitle font-mono text-cyan">Chart: {{ getFormattedReleaseChart(rel) }}</span>
              <p v-if="getFormattedReleaseDescription(rel)" class="card-release-desc font-xs text-muted">
                Description: {{ getFormattedReleaseDescription(rel) }}
              </p>
            </div>
          </div>
          <StatusBadge :status="getStatusType(rel.status)" :label="rel.status" size="sm" />
        </div>

        <div class="card-meta-row font-mono font-xs">
          <span class="ns-tag">🏷️ {{ rel.namespace }}</span>
          <span class="rev-tag">rev {{ rel.revision || 1 }}</span>
          <span class="text-muted">{{ formatReleaseDate(rel.updated) }}</span>
        </div>

        <!-- 4 Labeled Touch Action Buttons (with crimson red for uninstall) -->
        <div class="card-actions-grid">
          <button
            type="button"
            class="btn-m-action btn-m-values"
            @click="emit('openDetail', rel)"
          >
            <span>🔍 Values</span>
          </button>
          <button
            type="button"
            class="btn-m-action btn-m-upgrade"
            @click="emit('upgrade', rel)"
          >
            <span>🔄 Upgrade</span>
          </button>
          <button
            type="button"
            class="btn-m-action btn-m-rollback"
            @click="emit('rollback', rel)"
          >
            <span>⏪ Rollback</span>
          </button>
          <button
            type="button"
            class="btn-m-action btn-m-delete"
            @click="emit('uninstall', rel)"
          >
            <span>🗑 Uninstall</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Charts Cards Stream -->
    <div v-else-if="activeTab === 'charts'" class="mobile-cards-list">
      <div
        v-for="chart in charts"
        :key="`${chart.repo}/${chart.name}`"
        class="mobile-card glass-panel"
      >
        <div class="card-top-row">
          <div class="card-identity">
            <span class="card-icon">{{ getChartIcon(chart) }}</span>
            <div>
              <strong class="card-title">{{ chart.name }}</strong>
              <span class="card-subtitle font-mono text-muted">🗄️ {{ chart.repo }} • v{{ chart.version }}</span>
            </div>
          </div>
        </div>
        <p class="card-desc">{{ chart.description || 'Cloud-native Helm chart package.' }}</p>
        <button
          type="button"
          class="btn-cyber btn-primary btn-sm w-full"
          @click="emit('installChart', chart)"
        >
          <span>🚀 Install Chart</span>
        </button>
      </div>
    </div>

    <!-- Repos Cards Stream -->
    <div v-else-if="activeTab === 'repos'" class="mobile-cards-list">
      <div
        v-for="repo in repos"
        :key="repo.name"
        class="mobile-card glass-panel"
      >
        <div class="card-top-row">
          <div class="card-identity">
            <span class="card-icon">🗄️</span>
            <div>
              <strong class="card-title font-mono">{{ repo.name }}</strong>
              <span class="card-subtitle font-mono text-cyan">{{ repo.url }}</span>
            </div>
          </div>
        </div>
        <div class="card-actions-row">
          <button
            type="button"
            class="btn-cyber btn-secondary btn-xs"
            @click="emit('copyUrl', repo.url)"
          >
            📋 Copy URL
          </button>
          <button
            type="button"
            class="btn-cyber btn-danger btn-xs"
            @click="emit('removeRepo', repo)"
          >
            🗑️ Remove
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.helm-mobile-cards-stream {
  display: none;
}
@media (max-width: 768px) {
  .helm-mobile-cards-stream {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
}
.mobile-cards-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.mobile-card {
  padding: 14px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.card-top-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 8px;
}
.card-identity {
  display: flex;
  align-items: center;
  gap: 10px;
}
.card-icon {
  font-size: 20px;
}
.card-title {
  font-size: 14px;
  color: var(--text-primary);
  display: block;
}
.card-identity-text {
  min-width: 0;
  flex: 1;
}
.card-subtitle {
  font-size: 11px;
  display: block;
}
.card-release-desc {
  font-size: 11px;
  color: var(--text-secondary, #94a3b8);
  margin: 3px 0 0 0;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  text-overflow: ellipsis;
  word-break: break-word;
}
.card-meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.ns-tag {
  color: var(--text-secondary);
}
.rev-tag {
  background: rgba(255, 255, 255, 0.06);
  padding: 1px 5px;
  border-radius: 4px;
  color: var(--text-secondary);
}
.card-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.card-actions-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 6px;
  margin-top: 4px;
}
.btn-m-action {
  min-height: 32px;
  padding: 6px 6px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  text-align: center;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-secondary);
  cursor: pointer;
  white-space: nowrap;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}
.btn-m-values:hover {
  border-color: #38bdf8;
  color: #38bdf8;
}
.btn-m-upgrade:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
}
.btn-m-rollback:hover {
  border-color: #fbbf24;
  color: #fbbf24;
}
.btn-m-delete {
  border-color: rgba(244, 63, 94, 0.4);
  color: #f43f5e;
  background: rgba(244, 63, 94, 0.08);
}
.btn-m-delete:hover {
  border-color: #f43f5e;
  background: rgba(244, 63, 94, 0.22);
  color: #fff;
}
.card-actions-row {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
@media (max-width: 480px) {
  .card-actions-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }
}
.w-full {
  width: 100%;
}
</style>
