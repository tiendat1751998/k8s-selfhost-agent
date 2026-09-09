<script setup lang="ts">
import type { SettingsFormState, CategoryKey, TestState } from '../../composables/useSettings'

const props = defineProps<{
  form: SettingsFormState
  saving: boolean
  isDirty: boolean
  integrationTests: Record<string, TestState>
}>()

const emit = defineEmits<{
  (e: 'save', category: CategoryKey): void
  (e: 'reset', category: CategoryKey): void
  (e: 'testService', key: string): void
}>()
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in">
    <div class="card-header">
      <div class="card-title-group">
        <div>
          <h2 class="card-title">DevOps & Toolchain Integrations</h2>
          <p class="card-subtitle">
            Connect external continuous delivery pipelines, vulnerability scanners, secrets vaults, and observability dashboards.
          </p>
        </div>
        <span v-if="isDirty" class="dirty-indicator-pill">● Unsaved Changes</span>
      </div>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        @click="emit('reset', 'integrations')"
      >
        <span>↺ Reset Defaults</span>
      </button>
    </div>

    <form class="settings-form" @submit.prevent="emit('save', 'integrations')">
      <div class="integrations-grid">
        <!-- ArgoCD -->
        <div class="integration-item glass-panel">
          <div class="integration-header">
            <div class="integration-title-group">
              <div class="integration-icon">🐙</div>
              <div>
                <h3 class="integration-name">ArgoCD Continuous Delivery</h3>
                <p class="integration-desc">GitOps synchronization and application lifecycle management.</p>
              </div>
            </div>
            <div class="integration-status">
              <span v-if="integrationTests.argocd_url?.testing" class="badge badge-amber">⏳ Testing...</span>
              <span v-else-if="integrationTests.argocd_url?.result?.reachable" class="badge badge-emerald">
                ✓ HTTP {{ integrationTests.argocd_url.result.status_code }} ({{ integrationTests.argocd_url.result.latency_ms }}ms)
              </span>
              <span v-else-if="integrationTests.argocd_url?.result && !integrationTests.argocd_url?.result?.reachable" class="badge badge-rose">
                ✗ Unreachable ({{ integrationTests.argocd_url.result.latency_ms }}ms)
              </span>
              <span v-else-if="integrationTests.argocd_url?.error" class="badge badge-rose">
                ✗ {{ integrationTests.argocd_url.error }}
              </span>
              <span v-else-if="form.argocd_url" class="badge badge-cyan">CONFIGURED</span>
              <span v-else class="badge badge-muted">NOT CONFIGURED</span>
            </div>
          </div>

          <div class="integration-input-row">
            <input
              v-model="form.argocd_url"
              type="url"
              class="input-glass form-input flex-1"
              placeholder="https://argocd.internal.company.com"
            />
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="integrationTests.argocd_url?.testing || !form.argocd_url"
              @click="emit('testService', 'argocd_url')"
            >
              <span>{{ integrationTests.argocd_url?.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
            </button>
          </div>
        </div>

        <!-- Trivy -->
        <div class="integration-item glass-panel">
          <div class="integration-header">
            <div class="integration-title-group">
              <div class="integration-icon">🛡️</div>
              <div>
                <h3 class="integration-name">Trivy Security Scanner</h3>
                <p class="integration-desc">Container image vulnerability & IaC misconfiguration scanner.</p>
              </div>
            </div>
            <div class="integration-status">
              <span v-if="integrationTests.trivy_url?.testing" class="badge badge-amber">⏳ Testing...</span>
              <span v-else-if="integrationTests.trivy_url?.result?.reachable" class="badge badge-emerald">
                ✓ HTTP {{ integrationTests.trivy_url.result.status_code }} ({{ integrationTests.trivy_url.result.latency_ms }}ms)
              </span>
              <span v-else-if="integrationTests.trivy_url?.result && !integrationTests.trivy_url?.result?.reachable" class="badge badge-rose">
                ✗ Unreachable ({{ integrationTests.trivy_url.result.latency_ms }}ms)
              </span>
              <span v-else-if="integrationTests.trivy_url?.error" class="badge badge-rose">
                ✗ {{ integrationTests.trivy_url.error }}
              </span>
              <span v-else-if="form.trivy_url" class="badge badge-cyan">CONFIGURED</span>
              <span v-else class="badge badge-muted">NOT CONFIGURED</span>
            </div>
          </div>

          <div class="integration-input-row">
            <input
              v-model="form.trivy_url"
              type="url"
              class="input-glass form-input flex-1"
              placeholder="http://trivy.security.svc:4954"
            />
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="integrationTests.trivy_url?.testing || !form.trivy_url"
              @click="emit('testService', 'trivy_url')"
            >
              <span>{{ integrationTests.trivy_url?.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
            </button>
          </div>
        </div>

        <!-- HashiCorp Vault -->
        <div class="integration-item glass-panel">
          <div class="integration-header">
            <div class="integration-title-group">
              <div class="integration-icon">🔐</div>
              <div>
                <h3 class="integration-name">HashiCorp Vault</h3>
                <p class="integration-desc">Centralized secret storage, PKI encryption, and dynamic lease manager.</p>
              </div>
            </div>
            <div class="integration-status">
              <span v-if="integrationTests.vault_url?.testing" class="badge badge-amber">⏳ Testing...</span>
              <span v-else-if="integrationTests.vault_url?.result?.reachable" class="badge badge-emerald">
                ✓ HTTP {{ integrationTests.vault_url.result.status_code }} ({{ integrationTests.vault_url.result.latency_ms }}ms)
              </span>
              <span v-else-if="integrationTests.vault_url?.result && !integrationTests.vault_url?.result?.reachable" class="badge badge-rose">
                ✗ Unreachable ({{ integrationTests.vault_url.result.latency_ms }}ms)
              </span>
              <span v-else-if="integrationTests.vault_url?.error" class="badge badge-rose">
                ✗ {{ integrationTests.vault_url.error }}
              </span>
              <span v-else-if="form.vault_url" class="badge badge-cyan">CONFIGURED</span>
              <span v-else class="badge badge-muted">NOT CONFIGURED</span>
            </div>
          </div>

          <div class="integration-input-row">
            <input
              v-model="form.vault_url"
              type="url"
              class="input-glass form-input flex-1"
              placeholder="https://vault.internal.company.com:8200"
            />
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="integrationTests.vault_url?.testing || !form.vault_url"
              @click="emit('testService', 'vault_url')"
            >
              <span>{{ integrationTests.vault_url?.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
            </button>
          </div>
        </div>

        <!-- Grafana Observability -->
        <div class="integration-item glass-panel">
          <div class="integration-header">
            <div class="integration-title-group">
              <div class="integration-icon">📊</div>
              <div>
                <h3 class="integration-name">Grafana Observability</h3>
                <p class="integration-desc">Time-series visual metrics, dashboards, and Loki log exploration.</p>
              </div>
            </div>
            <div class="integration-status">
              <span v-if="integrationTests.grafana_url?.testing" class="badge badge-amber">⏳ Testing...</span>
              <span v-else-if="integrationTests.grafana_url?.result?.reachable" class="badge badge-emerald">
                ✓ HTTP {{ integrationTests.grafana_url.result.status_code }} ({{ integrationTests.grafana_url.result.latency_ms }}ms)
              </span>
              <span v-else-if="integrationTests.grafana_url?.result && !integrationTests.grafana_url?.result?.reachable" class="badge badge-rose">
                ✗ Unreachable ({{ integrationTests.grafana_url.result.latency_ms }}ms)
              </span>
              <span v-else-if="integrationTests.grafana_url?.error" class="badge badge-rose">
                ✗ {{ integrationTests.grafana_url.error }}
              </span>
              <span v-else-if="form.grafana_url" class="badge badge-cyan">CONFIGURED</span>
              <span v-else class="badge badge-muted">NOT CONFIGURED</span>
            </div>
          </div>

          <div class="integration-input-row">
            <input
              v-model="form.grafana_url"
              type="url"
              class="input-glass form-input flex-1"
              placeholder="https://grafana.internal.company.com"
            />
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="integrationTests.grafana_url?.testing || !form.grafana_url"
              @click="emit('testService', 'grafana_url')"
            >
              <span>{{ integrationTests.grafana_url?.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div class="form-actions">
        <span class="field-desc">Service connectivity checks evaluate network routing and token exchange.</span>
        <button type="submit" class="btn btn-primary" :disabled="saving">
          <span v-if="saving" class="spinner spinner-sm"></span>
          <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Integration Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
