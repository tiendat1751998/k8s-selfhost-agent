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
  (e: 'testService', key: string, customUrl?: string): void
}>()

const retentionOptions = [7, 14, 30, 90, 180, 365]
const scrapeIntervalOptions = [5, 10, 15, 30, 60]
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in">
    <div class="card-header">
      <div class="card-title-group">
        <div>
          <h2 class="card-title">Telemetry, Metrics & Observability</h2>
          <p class="card-subtitle">
            Configure Prometheus scraping, Loki log retention, Alertmanager webhooks, and SMTP relay dispatch.
          </p>
        </div>
        <span v-if="isDirty" class="dirty-indicator-pill">● Unsaved Changes</span>
      </div>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        @click="emit('reset', 'telemetry')"
      >
        <span>↺ Reset Defaults</span>
      </button>
    </div>

    <form class="settings-form" @submit.prevent="emit('save', 'telemetry')">
      <!-- Prometheus Configuration -->
      <div class="integration-item glass-panel">
        <div class="integration-header">
          <div class="integration-title-group">
            <div class="integration-icon">📈</div>
            <div>
              <h3 class="integration-name">Prometheus Metric Collector</h3>
              <p class="integration-desc">High-resolution cluster compute, memory, and TPS time-series telemetry.</p>
            </div>
          </div>
          <div class="integration-status">
            <span v-if="integrationTests.prometheus_endpoint?.testing" class="badge badge-amber">⏳ Testing...</span>
            <span v-else-if="integrationTests.prometheus_endpoint?.result?.reachable" class="badge badge-emerald">
              ✓ HTTP {{ integrationTests.prometheus_endpoint.result.status_code }} ({{ integrationTests.prometheus_endpoint.result.latency_ms }}ms)
            </span>
            <span v-else-if="integrationTests.prometheus_endpoint?.error" class="badge badge-rose">
              ✗ {{ integrationTests.prometheus_endpoint.error }}
            </span>
            <span v-else-if="form.prometheus_endpoint" class="badge badge-cyan">CONFIGURED</span>
            <span v-else class="badge badge-muted">NOT CONFIGURED</span>
          </div>
        </div>

        <div class="form-row">
          <div class="form-group flex-2">
            <label class="form-label" for="prom-endpoint">Prometheus Endpoint URL</label>
            <div class="integration-input-row">
              <input
                id="prom-endpoint"
                v-model="form.prometheus_endpoint"
                type="url"
                class="input-glass form-input flex-1"
                placeholder="http://prometheus-k8s.monitoring.svc:9090"
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="integrationTests.prometheus_endpoint?.testing || !form.prometheus_endpoint"
                @click="emit('testService', 'prometheus_endpoint')"
              >
                <span>{{ integrationTests.prometheus_endpoint?.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
              </button>
            </div>
          </div>

          <div class="form-group flex-1">
            <label class="form-label" for="prom-scrape">Scrape Interval (Seconds)</label>
            <select id="prom-scrape" v-model.number="form.prometheus_scrape_interval_sec" class="input-glass form-select">
              <option v-for="sec in scrapeIntervalOptions" :key="sec" :value="sec">
                ⚡ {{ sec }}s Interval ({{ sec <= 10 ? 'High Fidelity' : 'Standard' }})
              </option>
            </select>
          </div>
        </div>
      </div>

      <!-- Loki Log Stream Configuration -->
      <div class="integration-item glass-panel">
        <div class="integration-header">
          <div class="integration-title-group">
            <div class="integration-icon">📜</div>
            <div>
              <h3 class="integration-name">Grafana Loki Log Gateway</h3>
              <p class="integration-desc">Centralized pod stdout/stderr log stream aggregator and indexer.</p>
            </div>
          </div>
          <div class="integration-status">
            <span v-if="integrationTests.loki_endpoint?.testing" class="badge badge-amber">⏳ Testing...</span>
            <span v-else-if="integrationTests.loki_endpoint?.result?.reachable" class="badge badge-emerald">
              ✓ HTTP {{ integrationTests.loki_endpoint.result.status_code }} ({{ integrationTests.loki_endpoint.result.latency_ms }}ms)
            </span>
            <span v-else-if="integrationTests.loki_endpoint?.error" class="badge badge-rose">
              ✗ {{ integrationTests.loki_endpoint.error }}
            </span>
            <span v-else-if="form.loki_endpoint" class="badge badge-cyan">CONFIGURED</span>
            <span v-else class="badge badge-muted">NOT CONFIGURED</span>
          </div>
        </div>

        <div class="form-row">
          <div class="form-group flex-2">
            <label class="form-label" for="loki-endpoint">Loki Gateway Endpoint</label>
            <div class="integration-input-row">
              <input
                id="loki-endpoint"
                v-model="form.loki_endpoint"
                type="url"
                class="input-glass form-input flex-1"
                placeholder="http://loki-gateway.logging.svc:3100"
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="integrationTests.loki_endpoint?.testing || !form.loki_endpoint"
                @click="emit('testService', 'loki_endpoint')"
              >
                <span>{{ integrationTests.loki_endpoint?.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
              </button>
            </div>
          </div>

          <div class="form-group flex-1">
            <label class="form-label" for="loki-retention">Log Retention Period (Days)</label>
            <select id="loki-retention" v-model.number="form.loki_retention_days" class="input-glass form-select">
              <option v-for="days in retentionOptions" :key="days" :value="days">
                📦 {{ days }} Days (Auto-Rotate)
              </option>
            </select>
          </div>
        </div>
      </div>

      <!-- Alertmanager Configuration -->
      <div class="integration-item glass-panel">
        <div class="integration-header">
          <div class="integration-title-group">
            <div class="integration-icon">🚨</div>
            <div>
              <h3 class="integration-name">Prometheus Alertmanager</h3>
              <p class="integration-desc">Incident grouping, deduplication, and notification dispatch bus.</p>
            </div>
          </div>
          <div class="integration-status">
            <span v-if="integrationTests.alertmanager_endpoint?.testing" class="badge badge-amber">⏳ Testing...</span>
            <span v-else-if="integrationTests.alertmanager_endpoint?.result?.reachable" class="badge badge-emerald">
              ✓ HTTP {{ integrationTests.alertmanager_endpoint.result.status_code }} ({{ integrationTests.alertmanager_endpoint.result.latency_ms }}ms)
            </span>
            <span v-else-if="integrationTests.alertmanager_endpoint?.error" class="badge badge-rose">
              ✗ {{ integrationTests.alertmanager_endpoint.error }}
            </span>
            <span v-else-if="form.alertmanager_endpoint" class="badge badge-cyan">CONFIGURED</span>
            <span v-else class="badge badge-muted">NOT CONFIGURED</span>
          </div>
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label" for="alertm-endpoint">Alertmanager Endpoint</label>
            <div class="integration-input-row">
              <input
                id="alertm-endpoint"
                v-model="form.alertmanager_endpoint"
                type="url"
                class="input-glass form-input flex-1"
                placeholder="http://alertmanager.monitoring.svc:9093"
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="integrationTests.alertmanager_endpoint?.testing || !form.alertmanager_endpoint"
                @click="emit('testService', 'alertmanager_endpoint')"
              >
                <span>{{ integrationTests.alertmanager_endpoint?.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
              </button>
            </div>
          </div>

          <div class="form-group flex-1">
            <label class="form-label" for="alertm-webhook">Outbound Webhook Dispatch URL</label>
            <input
              id="alertm-webhook"
              v-model="form.alertmanager_webhook_url"
              type="url"
              class="input-glass form-input"
              placeholder="https://hooks.slack.com/services/T000/B000/XXXX"
            />
          </div>
        </div>
      </div>

      <!-- Outbound SMTP Relay -->
      <div class="form-group toggle-group">
        <div class="toggle-info">
          <span class="toggle-label">Enable SMTP Outbound Email Relay</span>
          <p class="field-desc">
            Allows the platform to send alert digests, scheduled reports, and critical event notifications.
          </p>
        </div>
        <label class="toggle-switch">
          <input v-model="form.smtp_enabled" type="checkbox" />
          <span class="toggle-slider"></span>
        </label>
      </div>

      <div v-if="form.smtp_enabled" class="form-row animate-fade-in">
        <div class="form-group flex-2">
          <label class="form-label" for="smtp-host">SMTP Host FQDN</label>
          <input
            id="smtp-host"
            v-model="form.smtp_host"
            type="text"
            class="input-glass form-input"
            placeholder="smtp.sendgrid.net or smtp.mailgun.org"
          />
        </div>
        <div class="form-group flex-1">
          <label class="form-label" for="smtp-port">SMTP Port</label>
          <input
            id="smtp-port"
            v-model.number="form.smtp_port"
            type="number"
            class="input-glass form-input"
            placeholder="587"
          />
        </div>
      </div>

      <!-- Form Actions -->
      <div class="form-actions">
        <span class="field-desc">Scrape metrics and log rotation policies synchronize to cluster daemons automatically.</span>
        <button type="submit" class="btn btn-primary" :disabled="saving">
          <span v-if="saving" class="spinner spinner-sm"></span>
          <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Telemetry Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
