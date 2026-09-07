<script setup lang="ts">
import { ref } from 'vue'
import type { CveFinding, SecretAuditItem } from '../../composables/useDevSecOps'

interface Props {
  findings: CveFinding[]
  secrets: SecretAuditItem[]
  getResourceIcon: (type: string) => string
  getSeverityBadgeClass: (sev: string) => string
  getCvssBadgeClass: (score: number) => string
}

defineProps<Props>()

const emit = defineEmits<{
  'viewFinding': [finding: CveFinding]
  'patchFinding': [finding: CveFinding]
  'viewSecret': [secret: SecretAuditItem]
  'rotateSecret': [secret: SecretAuditItem]
}>()

const activeTab = ref<'cves' | 'secrets'>('cves')

function getBorderClass(severity: string): string {
  switch ((severity || '').toUpperCase()) {
    case 'CRITICAL': return 'border-critical'
    case 'HIGH': return 'border-high'
    case 'MEDIUM': return 'border-medium'
    default: return 'border-low'
  }
}

function getSecretBorderClass(status: string): string {
  switch (status) {
    case 'EXPOSED': return 'border-critical'
    case 'EXPIRING_SOON': return 'border-high'
    default: return 'border-low'
  }
}
</script>

<template>
  <div class="secops-mobile-stream">
    <!-- Mobile Segmented Tabs -->
    <div style="display: flex; gap: 8px; margin-bottom: 6px;">
      <button
        class="filter-pill"
        :class="{ 'filter-active': activeTab === 'cves' }"
        style="flex: 1; text-align: center;"
        @click="activeTab = 'cves'"
      >
        <span>Vulnerabilities ({{ findings.length }})</span>
      </button>
      <button
        class="filter-pill"
        :class="{ 'filter-active': activeTab === 'secrets' }"
        style="flex: 1; text-align: center;"
        @click="activeTab = 'secrets'"
      >
        <span>Secrets & Certs ({{ secrets.length }})</span>
      </button>
    </div>

    <!-- CVEs Stream -->
    <template v-if="activeTab === 'cves'">
      <div
        v-for="finding in findings"
        :key="finding.id"
        class="secops-mobile-card glass-panel"
        :class="getBorderClass(finding.severity)"
        @click="emit('viewFinding', finding)"
      >
        <div class="mobile-card-main">
          <div class="mobile-card-row-1">
            <span>{{ getResourceIcon(finding.resource_type) }}</span>
            <span class="mobile-card-title">{{ finding.resource_name }}</span>
            <span class="badge" :class="getSeverityBadgeClass(finding.severity)" style="font-size: 9.5px; padding: 1px 5px;">
              {{ finding.severity?.substring(0, 4) }}
            </span>
          </div>
          <div class="mobile-card-row-2">
            <span class="font-mono text-cyan">{{ finding.cve_id }}</span>
            <span class="text-muted">•</span>
            <span class="text-muted">{{ finding.package_name }} {{ finding.installed_version }}</span>
          </div>
        </div>

        <div class="mobile-card-actions">
          <span class="cvss-badge" :class="getCvssBadgeClass(finding.cvss_score)">
            {{ finding.cvss_score.toFixed(1) }}
          </span>
          <button
            class="btn btn-secondary btn-sm"
            style="padding: 4px 8px; font-size: 11px;"
            @click.stop="emit('viewFinding', finding)"
          >
            <span>🔍</span>
          </button>
          <button
            class="btn btn-sm btn-patch"
            style="padding: 4px 8px; font-size: 11px;"
            @click.stop="emit('patchFinding', finding)"
          >
            <span>🛡️</span>
          </button>
        </div>
      </div>

      <div v-if="findings.length === 0" class="empty-table-cell" style="padding: 24px !important;">
        <span class="empty-icon">🛡️</span>
        <p style="font-size: 12px;">No vulnerabilities matching current filters.</p>
      </div>
    </template>

    <!-- Secrets & Certs Stream -->
    <template v-else>
      <div
        v-for="secret in secrets"
        :key="secret.id"
        class="secops-mobile-card glass-panel"
        :class="getSecretBorderClass(secret.status)"
        @click="emit('rotateSecret', secret)"
      >
        <div class="mobile-card-main">
          <div class="mobile-card-row-1">
            <span>🔐</span>
            <span class="mobile-card-title">{{ secret.name }}</span>
            <span
              class="badge"
              :class="secret.status === 'EXPOSED' ? 'badge-rose' : secret.status === 'EXPIRING_SOON' ? 'badge-amber' : 'badge-emerald'"
              style="font-size: 9.5px; padding: 1px 5px;"
            >
              {{ secret.status.replace('_', ' ') }}
            </span>
          </div>
          <div class="mobile-card-row-2">
            <span class="badge badge-violet" style="font-size: 9px; padding: 0 4px;">{{ secret.namespace }}</span>
            <span class="text-muted font-mono">{{ secret.masked_value }}</span>
          </div>
        </div>

        <div class="mobile-card-actions">
          <button
            class="btn btn-secondary btn-sm"
            :class="{ 'btn-patch': secret.status !== 'COMPLIANT' }"
            style="padding: 4px 8px; font-size: 11px;"
            @click.stop="emit('rotateSecret', secret)"
          >
            <span>{{ secret.status === 'COMPLIANT' ? '🔄' : '⚡' }}</span>
          </button>
        </div>
      </div>

      <div v-if="secrets.length === 0" class="empty-table-cell" style="padding: 24px !important;">
        <span class="empty-icon">🔐</span>
        <p style="font-size: 12px;">No secrets or certificates flagged.</p>
      </div>
    </template>
  </div>
</template>
