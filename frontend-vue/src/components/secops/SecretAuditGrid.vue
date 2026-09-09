<script setup lang="ts">
import type { SecretAuditItem } from '../../composables/useDevSecOps'

interface Props {
  secrets: SecretAuditItem[]
  loading?: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  'rotateSecret': [secret: SecretAuditItem]
  'viewSecret': [secret: SecretAuditItem]
}>()

function getStatusBadgeClass(status: string): string {
  switch (status) {
    case 'EXPOSED': return 'badge-rose'
    case 'EXPIRING_SOON': return 'badge-amber'
    case 'EXPIRED': return 'badge-rose'
    default: return 'badge-emerald'
  }
}

function getSecretTypeClass(type: string): string {
  switch (type) {
    case 'TLS Certificate': return 'badge-cyan'
    case 'Vault Secret': return 'badge-violet'
    case 'API Key': return 'badge-amber'
    case 'JWT Token': return 'badge-rose'
    default: return 'badge-muted'
  }
}
</script>

<template>
  <div class="table-container glass-panel">
    <div class="table-header">
      <div>
        <h2 class="table-title">Exposed Secrets & TLS Certificates Governance Matrix</h2>
        <p class="table-subtitle">
          Continuous detection across pods, container crash logs, HashiCorp Vault dynamic leases, and External Secrets Operator (ESO)
        </p>
      </div>
      <span class="badge badge-violet">{{ secrets.length }} Monitored Credentials</span>
    </div>

    <div class="secrets-grid">
      <div
        v-for="secret in secrets"
        :key="secret.id"
        class="secret-card"
        :class="{
          'secret-status-exposed': secret.status === 'EXPOSED',
          'secret-status-expiring': secret.status === 'EXPIRING_SOON',
        }"
      >
        <div class="secret-card-top">
          <div class="secret-name" :title="secret.name">
            <span>🔐 {{ secret.name }}</span>
          </div>
          <span class="secret-type-badge badge" :class="getSecretTypeClass(secret.secret_type)">
            {{ secret.secret_type }}
          </span>
        </div>

        <div class="secret-meta">
          <div class="secret-source">
            <span>Namespace:</span>
            <span class="badge badge-violet font-mono" style="font-size: 10px;">{{ secret.namespace }}</span>
            <span class="text-muted">• {{ secret.source_resource }}</span>
          </div>

          <p class="desc-cell" style="font-size: 11.5px; margin: 4px 0;">
            {{ secret.exposure_detail }}
          </p>

          <div class="secret-val-masked font-mono">
            {{ secret.masked_value }}
          </div>
        </div>

        <div class="secret-footer">
          <div style="display: flex; align-items: center; gap: 6px;">
            <span class="badge" :class="getStatusBadgeClass(secret.status)">
              {{ secret.status.replace('_', ' ') }}
            </span>
            <span v-if="secret.days_until_expiry !== undefined" class="text-muted" style="font-size: 10.5px;">
              ({{ secret.days_until_expiry }}d remaining)
            </span>
          </div>

          <button
            class="btn btn-secondary btn-sm"
            :class="{ 'btn-patch': secret.status !== 'COMPLIANT' }"
            @click="emit('rotateSecret', secret)"
          >
            <span>{{ secret.status === 'COMPLIANT' ? '🔄 Resync' : '⚡ Rotate / Vault' }}</span>
          </button>
        </div>
      </div>

      <div v-if="secrets.length === 0" class="empty-table-cell" style="grid-column: 1 / -1;">
        <span class="empty-icon">🔐</span>
        <p>No exposed secrets or expiring TLS certificates detected in cluster namespaces.</p>
      </div>
    </div>
  </div>
</template>
