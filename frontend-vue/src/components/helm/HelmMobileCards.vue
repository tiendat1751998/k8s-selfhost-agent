<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { HelmRelease, HelmChart, HelmRepo } from '../../api/helm'
import { getFormattedReleaseChart } from '../../composables/useHelm'

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
  <div class="helm-mobile-cards-stream">
    <!-- Releases Cards Stream: High-Density Stream (~75-85px) -->
    <div v-if="activeTab === 'releases'" class="mobile-cards-list">
      <div
        v-for="rel in releases"
        :key="`${rel.namespace}/${rel.name}`"
        class="mobile-card mobile-release-dense-card glass-panel"
        role="button"
        tabindex="0"
        @click="emit('openDetail', rel)"
        @keydown.enter="emit('openDetail', rel)"
      >
        <!-- Row 1: Helm icon + Release name + Status badge -->
        <div class="card-row-1">
          <div class="card-name-group">
            <BaseIcon name="anchor" size="xs" class="card-helm-icon" />
            <span class="card-release-name font-semibold">{{ rel.name }}</span>
          </div>
          <StatusBadge :status="getStatusType(rel.status)" :label="rel.status" size="sm" />
        </div>

        <!-- Row 2: Chart name & version + Namespace pill + Revision + Quick action + More -->
        <div class="card-row-2">
          <div class="card-meta-pills">
            <span class="pill-chart font-mono" :title="getFormattedReleaseChart(rel)">{{ getFormattedReleaseChart(rel) }}</span>
            <span class="pill-ns font-mono">{{ rel.namespace }}</span>
            <span class="pill-rev font-mono">rev {{ rel.revision || 1 }}</span>
          </div>
          <div class="card-quick-actions" @click.stop>
            <button
              type="button"
              class="btn-m-quick btn-m-values"
              title="Inspect Values & Configuration"
              aria-label="Inspect Values"
              @click.stop="emit('openDetail', rel)"
            >
              <BaseIcon name="sliders" size="xs" /> <span>Values</span>
            </button>
            <button
              type="button"
              class="btn-m-more"
              title="More Actions (Upgrade, Rollback, Uninstall)"
              aria-label="More Actions"
              @click.stop="emit('openDetail', rel)"
            >
              <span>⋯</span>
            </button>
          </div>
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
            <BaseIcon :name="getChartIcon(chart)" size="sm" class="card-icon" />
            <div>
              <strong class="card-title">{{ chart.name }}</strong>
              <span class="card-subtitle font-mono text-muted"><BaseIcon name="database" size="xs" /> {{ chart.repo }} • v{{ chart.version }}</span>
            </div>
          </div>
        </div>
        <p class="card-desc">{{ chart.description || 'Cloud-native Helm chart package.' }}</p>
        <button
          type="button"
          class="btn-cyber btn-primary btn-sm w-full"
          @click="emit('installChart', chart)"
        >
          <BaseIcon name="play" size="xs" /> <span>Install Chart</span>
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
            <BaseIcon name="database" size="xs" class="card-icon" />
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
            <BaseIcon name="copy" size="xs" /> Copy URL
          </button>
          <button
            type="button"
            class="btn-cyber btn-danger btn-xs"
            @click="emit('removeRepo', repo)"
          >
            <BaseIcon name="trash" size="xs" /> Remove
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/helm.css';
</style>
