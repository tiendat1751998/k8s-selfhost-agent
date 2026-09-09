<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { HelmRelease, HelmReleaseDetail, HelmRevisionHistory } from '../../api/helm'

const props = defineProps<{
  show: boolean
  release: HelmReleaseDetail | null
  loadingDetail: boolean
  activeDrawerTab: 'overview' | 'values' | 'manifest' | 'history'
  releaseHistory: HelmRevisionHistory[]
  loadingHistory: boolean
  valuesCopied: boolean
  manifestCopied: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'close'): void
  (e: 'update:activeDrawerTab', val: 'overview' | 'values' | 'manifest' | 'history'): void
  (e: 'upgrade', rel: HelmRelease): void
  (e: 'rollback', rel: HelmRelease): void
  (e: 'uninstall', rel: HelmRelease): void
  (e: 'rollbackRevision', revision: number): void
  (e: 'copyNotes', text: string): void
  (e: 'copyValues', text: string): void
  (e: 'downloadValues', filename: string, content: string): void
  (e: 'copyManifest', text: string): void
  (e: 'downloadManifest', filename: string, content: string): void
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
    const days = Math.floor(Math.abs(Date.now() - d.getTime()) / (1000 * 60 * 60 * 24))
    return days < 30 ? `${days}d ago` : d.toLocaleDateString()
  } catch {
    return dateStr
  }
}

function getValuesString(): string {
  if (!props.release?.values) return ''
  return typeof props.release.values === 'string'
    ? props.release.values
    : JSON.stringify(props.release.values, null, 2)
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    placement="right"
    max-width="780px"
    :title="release ? `Release: ${release.name}` : 'Release Detail'"
    :subtitle="release ? `${release.namespace} • ${release.chart}` : ''"
    @close="emit('close'); emit('update:show', false)"
  >
    <div v-if="release" class="drawer-inner-content">
      <!-- Status Header Card -->
      <div class="release-header-card glass-panel">
        <div class="header-card-row">
          <div class="release-title-block">
            <span class="title-icon">⛵</span>
            <div>
              <h3 class="release-hero-title">{{ release.name }}</h3>
              <span class="font-mono font-xs text-muted">
                Namespace: <strong class="text-cyan">{{ release.namespace }}</strong> • Revision: <strong class="text-gold">#{{ release.revision || 1 }}</strong>
              </span>
            </div>
          </div>
          <StatusBadge :status="getStatusType(release.status)" :label="release.status" size="md" />
        </div>

        <div class="header-details-grid">
          <div class="detail-metric">
            <span class="metric-lbl">Chart Package</span>
            <span class="metric-val font-mono text-cyan">{{ release.chart }}</span>
          </div>
          <div class="detail-metric">
            <span class="metric-lbl">Chart Version</span>
            <span class="metric-val font-mono text-gold">{{ release.version || 'v1.0.0' }}</span>
          </div>
          <div class="detail-metric">
            <span class="metric-lbl">App Version</span>
            <span class="metric-val font-mono text-primary">{{ release.app_version || release.appVersion || 'latest' }}</span>
          </div>
          <div class="detail-metric">
            <span class="metric-lbl">Last Updated</span>
            <span class="metric-val font-mono text-muted">{{ formatReleaseDate(release.updated) }}</span>
          </div>
        </div>

        <div class="header-drawer-actions">
          <button
            type="button"
            class="btn-cyber btn-primary btn-sm"
            @click="emit('upgrade', release)"
          >
            🔄 Upgrade Release
          </button>
          <button
            type="button"
            class="btn-cyber btn-secondary btn-sm"
            @click="emit('rollback', release)"
          >
            ⏪ Rollback Revision
          </button>
          <button
            type="button"
            class="btn-cyber btn-danger btn-sm"
            @click="emit('uninstall', release)"
          >
            🗑️ Uninstall
          </button>
        </div>
      </div>

      <!-- Drawer Navigation Subtabs -->
      <div class="drawer-tabs-bar">
        <button
          type="button"
          class="subtab-btn"
          :class="{ active: activeDrawerTab === 'overview' }"
          @click="emit('update:activeDrawerTab', 'overview')"
        >
          <span>📜 Notes & Overview</span>
        </button>
        <button
          type="button"
          class="subtab-btn"
          :class="{ active: activeDrawerTab === 'values' }"
          @click="emit('update:activeDrawerTab', 'values')"
        >
          <span>⚙️ Values (YAML)</span>
        </button>
        <button
          type="button"
          class="subtab-btn"
          :class="{ active: activeDrawerTab === 'manifest' }"
          @click="emit('update:activeDrawerTab', 'manifest')"
        >
          <span>📄 K8s Manifest</span>
        </button>
        <button
          type="button"
          class="subtab-btn"
          :class="{ active: activeDrawerTab === 'history' }"
          @click="emit('update:activeDrawerTab', 'history')"
        >
          <span>⏱️ Revision History</span>
        </button>
      </div>

      <!-- Subtab 1: Notes & Overview -->
      <div v-if="activeDrawerTab === 'overview'" class="drawer-tab-pane">
        <div class="notes-panel glass-panel">
          <div class="notes-panel-header">
            <h4 class="font-mono text-cyan font-sm">RELEASE NOTES & ACCESS GUIDANCE</h4>
            <button
              v-if="release.notes"
              type="button"
              class="btn-cyber btn-outline-cyan btn-xs"
              @click="emit('copyNotes', release.notes)"
            >
              📋 Copy Notes
            </button>
          </div>
          <pre v-if="release.notes" class="cyber-code-block">{{ release.notes }}</pre>
          <p v-else class="text-muted font-mono font-xs text-center py-4">
            No release notes provided by the chart maintainer.
          </p>
        </div>
      </div>

      <!-- Subtab 2: Values (YAML) -->
      <div v-if="activeDrawerTab === 'values'" class="drawer-tab-pane">
        <div class="values-panel glass-panel">
          <div class="panel-actions-row">
            <span class="font-mono font-xs text-muted">Active Helm Values (User & Computed)</span>
            <div class="btn-group-sm">
              <button
                type="button"
                class="btn-cyber btn-outline-cyan btn-xs"
                @click="emit('copyValues', getValuesString())"
              >
                <span>{{ valuesCopied ? '✓ Copied!' : '📋 Copy YAML' }}</span>
              </button>
              <button
                type="button"
                class="btn-cyber btn-secondary btn-xs"
                @click="emit('downloadValues', `${release.name}-values.yaml`, getValuesString())"
              >
                <span>💾 Download</span>
              </button>
            </div>
          </div>
          <pre class="cyber-code-block yaml-display">{{ getValuesString() || '# No values configured' }}</pre>
        </div>
      </div>

      <!-- Subtab 3: Manifest -->
      <div v-if="activeDrawerTab === 'manifest'" class="drawer-tab-pane">
        <div class="manifest-panel glass-panel">
          <div class="panel-actions-row">
            <span class="font-mono font-xs text-muted">Synthesized Kubernetes Manifests</span>
            <div class="btn-group-sm">
              <button
                type="button"
                class="btn-cyber btn-outline-cyan btn-xs"
                @click="emit('copyManifest', release.manifest || '')"
              >
                <span>{{ manifestCopied ? '✓ Copied!' : '📋 Copy Manifest' }}</span>
              </button>
              <button
                type="button"
                class="btn-cyber btn-secondary btn-xs"
                @click="emit('downloadManifest', `${release.name}-manifest.yaml`, release.manifest || '')"
              >
                <span>💾 Download</span>
              </button>
            </div>
          </div>
          <pre class="cyber-code-block yaml-display">{{ release.manifest || '# No manifest data available' }}</pre>
        </div>
      </div>

      <!-- Subtab 4: Revision History -->
      <div v-if="activeDrawerTab === 'history'" class="drawer-tab-pane">
        <div v-if="loadingHistory" class="loading-state">
          <div class="cyber-spinner"></div>
          <p class="font-mono text-muted">Retrieving revision timeline...</p>
        </div>

        <div v-else-if="releaseHistory.length === 0" class="empty-state">
          <span class="empty-icon">⏱️</span>
          <p class="font-mono text-muted">No revision history found.</p>
        </div>

        <div v-else class="history-table-wrapper glass-panel">
          <table class="cyber-table table-compact">
            <thead>
              <tr>
                <th>Revision</th>
                <th>Chart</th>
                <th>App Version</th>
                <th>Status</th>
                <th>Updated</th>
                <th class="text-right">Action</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="hist in releaseHistory"
                :key="hist.revision"
                :class="{ 'row-highlight': hist.revision === release.revision }"
              >
                <td>
                  <span class="revision-pill font-mono">#{{ hist.revision }}</span>
                  <span v-if="hist.revision === release.revision" class="current-tag">CURRENT</span>
                </td>
                <td><span class="font-mono text-cyan">{{ hist.chart }}</span></td>
                <td><span class="font-mono text-muted">{{ hist.app_version || hist.appVersion || '—' }}</span></td>
                <td><StatusBadge :status="getStatusType(hist.status)" :label="hist.status" size="sm" /></td>
                <td><span class="font-mono font-xs text-muted">{{ formatReleaseDate(hist.updated) }}</span></td>
                <td class="text-right">
                  <button
                    v-if="hist.revision !== release.revision"
                    type="button"
                    class="btn-cyber btn-secondary btn-xs"
                    @click="emit('rollbackRevision', hist.revision)"
                  >
                    ⏪ Rollback
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/helm.css';
</style>
