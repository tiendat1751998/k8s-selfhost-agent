<script setup lang="ts">
import type { ConnectorPreset, ConnectFormData } from '../../composables/useEcosystem'

interface Props {
  show: boolean
  form: ConnectFormData
  saving: boolean
  presets: ConnectorPreset[]
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit'): void
  (e: 'selectPreset', preset: ConnectorPreset): void
}>()
</script>

<template>
  <div v-if="show" class="modal-backdrop" @click.self="emit('close')">
    <div class="modal-dialog glass-panel">
      <div class="modal-header">
        <h2>Connect Ecosystem Integration</h2>
        <button class="modal-close-btn" @click="emit('close')">✕</button>
      </div>

      <!-- Quick Preset Selectors -->
      <div class="preset-selector-section">
        <div class="preset-title">Quick Connect Presets:</div>
        <div class="preset-chips">
          <button
            v-for="p in presets"
            :key="p.id"
            type="button"
            class="preset-chip"
            @click="emit('selectPreset', p)"
          >
            <span>{{ p.icon }}</span>
            <span>{{ p.name }}</span>
          </button>
        </div>
      </div>

      <form class="modal-form" @submit.prevent="emit('submit')">
        <div class="form-group">
          <label>Integration Name *</label>
          <input
            v-model="form.name"
            type="text"
            placeholder="e.g. ArgoCD, Prometheus, HashiCorp Vault"
            required
            class="form-input"
          />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label>Category *</label>
            <select v-model="form.category" class="form-input">
              <option value="gitops">GitOps & CI/CD</option>
              <option value="monitoring">Monitoring & Telemetry</option>
              <option value="secrets">Secrets & KMS</option>
              <option value="compute">Compute & Control Plane</option>
              <option value="mesh">Service Mesh & Ingress</option>
              <option value="security">Security & Posture</option>
              <option value="policy">Policy & Guardrails</option>
              <option value="database">Database & Storage</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label>Version</label>
            <input
              v-model="form.version"
              type="text"
              placeholder="e.g. v2.10.4"
              class="form-input"
            />
          </div>
        </div>

        <div class="form-group">
          <label>Endpoint URL / Service URI *</label>
          <input
            v-model="form.endpoint"
            type="text"
            placeholder="https://argocd.corp.internal or http://prometheus.monitoring.svc:9090"
            class="form-input font-mono"
          />
        </div>

        <div class="form-group">
          <label>API Token / Secret Key (Optional)</label>
          <input
            v-model="form.apiToken"
            type="password"
            placeholder="Bearer token or API key for webhook probes"
            class="form-input font-mono"
          />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label>Initial Health</label>
            <select v-model="form.health" class="form-input">
              <option value="healthy">Healthy</option>
              <option value="degraded">Degraded</option>
              <option value="unknown">Unknown</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label>Sync Interval</label>
            <select v-model="form.syncInterval" class="form-input">
              <option value="1m">1 Minute</option>
              <option value="5m">5 Minutes</option>
              <option value="15m">15 Minutes</option>
              <option value="1h">1 Hour</option>
            </select>
          </div>
        </div>

        <div class="modal-actions">
          <button
            type="button"
            class="btn-secondary"
            @click="emit('close')"
          >
            Cancel
          </button>
          <button
            type="submit"
            class="btn-primary"
            :disabled="saving"
          >
            {{ saving ? 'Connecting...' : 'Save & Connect' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
