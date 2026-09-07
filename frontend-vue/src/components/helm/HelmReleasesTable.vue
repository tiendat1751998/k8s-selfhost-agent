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
.helm-releases-table-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.table-toolbar {
  padding: 14px 18px;
  border-radius: 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.toolbar-search {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  max-width: 450px;
}
.search-icon {
  position: absolute;
  left: 12px;
  font-size: 14px;
  pointer-events: none;
  opacity: 0.6;
}
.search-input {
  width: 100%;
  padding: 8px 32px 8px 34px;
  border-radius: 8px;
  font-size: 13px;
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
.toolbar-filters {
  display: flex;
  align-items: center;
  gap: 12px;
}
.filter-group {
  display: flex;
  align-items: center;
  gap: 6px;
}
.filter-label {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}
.select-sm {
  padding: 6px 10px;
  font-size: 12px;
  border-radius: 6px;
}
.data-table-container {
  border-radius: 14px;
  overflow: hidden;
  padding: 4px;
}
.table-scroll-wrapper {
  overflow-x: auto;
}
.cyber-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 13px;
}
.cyber-table th {
  padding: 12px 16px;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid var(--color-hairline);
  font-family: var(--font-mono);
  white-space: nowrap;
}
.cyber-table td {
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  vertical-align: middle;
}
.table-row-interactive {
  transition: background var(--transition-fast);
}
.table-row-interactive:hover {
  background: rgba(255, 255, 255, 0.03);
}
.release-name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
}
.release-icon {
  font-size: 18px;
}
.release-name-text {
  font-weight: 700;
  color: var(--text-primary);
  font-family: var(--font-mono);
}
.release-name-cell:hover .release-name-text {
  color: var(--color-primary);
}
.release-desc-sub {
  display: block;
  font-size: 11px;
  color: var(--text-muted);
  max-width: 260px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.chart-cell {
  font-weight: 600;
}
.version-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.app-version-sub {
  font-size: 10px;
  color: var(--text-muted);
}
.ns-badge {
  font-size: 12px;
  color: var(--text-secondary);
}
.revision-pill {
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.06);
  font-size: 11px;
  color: var(--text-secondary);
}
.action-btn-group {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.btn-row-action {
  min-height: 32px;
  padding: 5px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all var(--transition-fast);
  white-space: nowrap;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.btn-row-action:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.1);
}
.btn-action-detail:hover {
  border-color: rgba(6, 182, 212, 0.5);
  color: #38bdf8;
}
.btn-action-upgrade:hover {
  border-color: rgba(252, 213, 53, 0.5);
  color: var(--color-primary);
}
.btn-action-rollback:hover {
  border-color: rgba(245, 158, 11, 0.5);
  color: #fbbf24;
}
.btn-action-delete {
  border-color: rgba(244, 63, 94, 0.4);
  color: #f43f5e;
  background: rgba(244, 63, 94, 0.08);
}
.btn-action-delete:hover {
  border-color: #f43f5e;
  background: rgba(244, 63, 94, 0.22);
  color: #fff;
}
.loading-state, .empty-state, .error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  text-align: center;
  gap: 12px;
}
.cyber-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid rgba(252, 213, 53, 0.15);
  border-top-color: var(--color-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
.empty-icon, .error-icon {
  font-size: 38px;
}
.empty-title, .error-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}
.error-title {
  color: #fb7185;
}
.empty-desc, .error-desc {
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
