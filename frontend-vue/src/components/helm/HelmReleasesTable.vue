<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import BaseIcon from '../ui/BaseIcon.vue'
import type { HelmRelease } from '../../api/helm'
import { parseHelmChart, getFormattedReleaseChart, getFormattedReleaseDescription } from '../../composables/useHelm'

defineProps<{
  releases: HelmRelease[]
  loading: boolean
  error: string | null
  search?: string
  statusFilter?: string
  selectedCluster?: string
}>()

const emit = defineEmits<{
  (e: 'update:search', val: string): void
  (e: 'update:statusFilter', val: string): void
  (e: 'openDetail', rel: HelmRelease): void
  (e: 'inspect', rel: HelmRelease): void
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
  if (!dateStr) return '-'
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

function handleInspect(rel: HelmRelease) {
  emit('inspect', rel)
  emit('openDetail', rel)
}

function getActionItems(rel: HelmRelease): ActionItem[] {
  const isPending = (rel.status || '').toLowerCase().includes('pending')
  return [
    { id: 'upgrade', label: 'Upgrade Release', icon: 'refresh', disabled: isPending },
    { id: 'rollback', label: 'Rollback Revision', icon: 'rotate-ccw', disabled: isPending },
    { id: 'sep', label: '', separator: true },
    { id: 'uninstall', label: 'Uninstall Release', icon: 'trash', variant: 'danger', disabled: isPending },
  ]
}

function handleRowAction(actionId: string, rel: HelmRelease) {
  if (actionId === 'upgrade') emit('upgrade', rel)
  else if (actionId === 'rollback') emit('rollback', rel)
  else if (actionId === 'uninstall') emit('uninstall', rel)
}
</script>

<template>
  <div class="helm-releases-table-container">
    <!-- Data Table Container (Clean Enterprise Card, mounts directly below unified toolbar) -->
    <div class="data-table-container glass-panel">
      <div v-if="loading" class="loading-state">
        <div class="cyber-spinner"></div>
        <p class="font-mono text-muted">Retrieving Helm releases from {{ selectedCluster || 'cluster' }}...</p>
      </div>

      <div v-else-if="error" class="error-state">
        <BaseIcon name="alert-triangle" size="xs" class="error-icon" />
        <h4 class="error-title">Failed to load releases</h4>
        <p class="error-desc">{{ error }}</p>
        <button type="button" class="btn-cyber btn-primary btn-sm" @click="emit('retry')">
          Retry Connection
        </button>
      </div>

      <div v-else-if="releases.length === 0" class="empty-state">
        <BaseIcon name="anchor" size="xl" class="empty-icon" />
        <h4 class="empty-title">No Helm Releases Found</h4>
        <p class="empty-desc">
          No releases match your current filters on cluster <code class="text-primary">{{ selectedCluster || 'current' }}</code>.
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
                <div class="release-name-cell" @click="handleInspect(rel)">
                  <BaseIcon name="anchor" size="xs" class="release-icon" />
                  <div class="release-info-col">
                    <span class="release-name-text release-title-strong">{{ rel.name }}</span>
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
                  <span class="version-pill font-mono">{{ parseHelmChart(rel.chart, rel.description, rel.version).version || rel.version || rel.appVersion || 'v1.0.0' }}</span>
                  <span v-if="rel.app_version || rel.appVersion" class="app-version-sub font-mono">
                    app: {{ rel.app_version || rel.appVersion }}
                  </span>
                </div>
              </td>

              <!-- Namespace -->
              <td>
                <span class="ns-badge font-mono"><BaseIcon name="grid" size="xs" /> {{ rel.namespace }}</span>
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

              <!-- Standardized Row Actions: 1 Inline Button + ActionDropdown [ â‹¯ ] -->
              <td class="text-right actions-cell">
                <div class="action-btn-group">
                  <button
                    type="button"
                    class="btn btn-xs btn-secondary"
                    title="Inspect Release Values"
                    @click="handleInspect(rel)"
                  >
                    <BaseIcon name="search" size="xs" />
                    <span>Values</span>
                  </button>
                  <ActionDropdown
                    :items="getActionItems(rel)"
                    size="xs"
                    trigger-title="Release Actions"
                    @select="handleRowAction($event, rel)"
                    @action="handleRowAction($event, rel)"
                  />
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

.table-scroll-wrapper,
table.cyber-table {
  width: 100%;
}

.release-desc-sub {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}

.release-title-strong,
.release-name-text {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}
</style>