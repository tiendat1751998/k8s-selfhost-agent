<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { HelmRelease } from '../../api/helm'
import { parseHelmChart, getFormattedReleaseChart, getFormattedReleaseDescription } from '../../composables/useHelm'

defineProps<{
  releases: HelmRelease[]
  loading: boolean
  error: string | null
  search: string
  statusFilter: string
  selectedCluster: string
}>()

const emit = defineEmits<{
  (e: 'update:search', val: string): void
  (e: 'update:statusFilter', val: string): void
  (e: 'openDetail', rel: HelmRelease): void
  (e: 'upgrade', rel: HelmRelease): void
  (e: 'rollback', rel: HelmRelease): void
  (e: 'uninstall', rel: HelmRelease): void
  (e: 'retry'): void
  (e: 'browseCharts'): void
}>()

function getStatusType(status: string): string {
  const s = (status || '').toLowerCase()
  if (s.includes('deploy') || s === 'active' || s === 'success') return 'deployed'
  if (s.includes('fail') || s.includes('error')) return 'failed'
  if (s.includes('pend') || s.includes('upgrad') || s.includes('install')) return 'pending'
  if (s.includes('super') || s.includes('uninst')) return 'superseded'
  return 'unknown'
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
    const days = Math.floor(Math.abs(diff) / (1000 * 60 * 60 * 24))
    return days < 30 ? `${days}d ago` : d.toLocaleDateString()
  } catch {
    return dateStr
  }
}
</script>

<template>
  <div class="helm-releases-table-container">
    <!-- Table Filter Bar -->
    <div class="table-toolbar glass-panel">
      <div class="toolbar-search">
        <span class="search-icon">🔍</span>
        <input
          :value="search"
          type="text"
          placeholder="Filter releases by name, chart or namespace..."
          class="input-glass search-input"
          @input="emit('update:search', ($event.target as HTMLInputElement).value)"
        />
        <button
          v-if="search"
          type="button"
          class="btn-clear"
          @click="emit('update:search', '')"
        >
          ✕
        </button>
      </div>

      <div class="toolbar-filters">
        <div class="filter-group">
          <span class="filter-label">Status:</span>
          <select
            :value="statusFilter"
            class="input-glass select-sm"
            @change="emit('update:statusFilter', ($event.target as HTMLSelectElement).value)"
          >
            <option value="all">All Statuses</option>
            <option value="deployed">Deployed</option>
            <option value="failed">Failed</option>
            <option value="pending">Pending</option>
            <option value="superseded">Superseded</option>
          </select>
        </div>
      </div>
    </div>

    <!-- Data Table Container -->
    <div class="data-table-container glass-panel">
      <div v-if="loading" class="loading-state">
        <div class="cyber-spinner"></div>
        <p class="font-mono text-muted">Retrieving Helm releases from {{ selectedCluster }}...</p>
      </div>

      <div v-else-if="error" class="error-state">
        <span class="error-icon">⚠️</span>
        <h4 class="error-title">Failed to load releases</h4>
        <p class="error-desc">{{ error }}</p>
        <button type="button" class="btn-cyber btn-primary btn-sm" @click="emit('retry')">
          Retry Connection
        </button>
      </div>

      <div v-else-if="releases.length === 0" class="empty-state">
        <span class="empty-icon">⛵</span>
        <h4 class="empty-title">No Helm Releases Found</h4>
        <p class="empty-desc">
          No releases match your current filters on cluster <code class="text-gold">{{ selectedCluster }}</code>.
        </p>
        <button
          type="button"
          class="btn-cyber btn-primary"
          @click="emit('browseCharts')"
        >
          Browse Chart Catalog & Install
        </button>
      </div>

      <div v-else class="table-scroll-wrapper">
        <table class="cyber-table">
          <thead>
            <tr>
              <th>Release Name</th>
              <th>Chart Package</th>
              <th>Version</th>
              <th>Namespace</th>
              <th>Status</th>
              <th>Revision</th>
              <th>Updated</th>
              <th class="text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="rel in releases"
              :key="`${rel.namespace}/${rel.name}`"
              class="table-row-interactive"
            >
              <!-- Release Name -->
              <td>
                <div class="release-name-cell" @click="emit('openDetail', rel)">
                  <span class="release-icon">⛵</span>
                  <div>
                    <span class="release-name-text">{{ rel.name }}</span>
                    <span v-if="getFormattedReleaseDescription(rel) || rel.description" class="release-desc-sub">
                      {{ getFormattedReleaseDescription(rel) || rel.description }}
                    </span>
                  </div>
                </div>
              </td>

              <!-- Chart -->
              <td>
                <span class="font-mono text-cyan chart-cell">Chart: {{ getFormattedReleaseChart(rel) }}</span>
              </td>

              <!-- Version & App Version -->
              <td>
                <div class="version-cell">
                  <span class="badge-tag badge-cyan">{{ parseHelmChart(rel.chart, rel.description, rel.version).version || rel.version || rel.appVersion || 'v1.0.0' }}</span>
                  <span v-if="rel.app_version || rel.appVersion" class="app-version-sub font-mono">
                    app: {{ rel.app_version || rel.appVersion }}
                  </span>
                </div>
              </td>

              <!-- Namespace -->
              <td>
                <span class="ns-badge font-mono">🏷️ {{ rel.namespace }}</span>
              </td>

              <!-- Status Badge -->
              <td>
                <StatusBadge :status="getStatusType(rel.status)" :label="rel.status" size="sm" />
              </td>

              <!-- Revision -->
              <td>
                <span class="revision-pill font-mono">rev {{ rel.revision || 1 }}</span>
              </td>

              <!-- Updated -->
              <td>
                <span class="text-muted font-mono font-xs">{{ formatReleaseDate(rel.updated) }}</span>
              </td>

              <!-- Row Actions with Labeled Buttons -->
              <td class="text-right actions-cell">
                <div class="action-btn-group">
                  <button
                    type="button"
                    class="btn-row-action btn-action-detail"
                    title="Inspect Release Values & Details"
                    @click="emit('openDetail', rel)"
                  >
                    <span>🔍 Values</span>
                  </button>
                  <button
                    type="button"
                    class="btn-row-action btn-action-upgrade"
                    title="Upgrade Release"
                    @click="emit('upgrade', rel)"
                  >
                    <span>🔄 Upgrade</span>
                  </button>
                  <button
                    type="button"
                    class="btn-row-action btn-action-rollback"
                    title="Rollback Release Revision"
                    @click="emit('rollback', rel)"
                  >
                    <span>⏪ Rollback</span>
                  </button>
                  <button
                    type="button"
                    class="btn-row-action btn-action-delete"
                    title="Uninstall Release"
                    @click="emit('uninstall', rel)"
                  >
                    <span>🗑 Uninstall</span>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/helm.css';
</style>
