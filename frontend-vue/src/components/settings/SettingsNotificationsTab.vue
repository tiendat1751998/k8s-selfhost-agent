<script setup lang="ts">
import { ref } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import { settingsApi, type IntegrationTestResult } from '../../api/settings'

const props = defineProps<{
  form: {
    smtp_enabled: boolean
    smtp_host: string
    smtp_port: number
    webhook_url: string
  }
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'save', category: 'notifications'): void
  (e: 'reset', category: 'notifications'): void
}>()

const testing = ref(false)
const testResult = ref<IntegrationTestResult | null>(null)
const testError = ref<string | null>(null)

async function handleTestNotification() {
  if (!props.form.webhook_url) {
    testError.value = 'Please provide an Incident Webhook URL before testing integration.'
    return
  }
  testing.value = true
  testResult.value = null
  testError.value = null
  try {
    const res = await settingsApi.testIntegration(props.form.webhook_url)
    testResult.value = res
  } catch (err: unknown) {
    testError.value = err instanceof Error ? err.message : 'Webhook dispatch test failed'
  } finally {
    testing.value = false
  }
}
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in">
    <div class="card-header">
      <div>
        <h2 class="card-title">Notifications & Alert Routing</h2>
        <p class="card-subtitle">
          Set up outbound SMTP mail relay and incoming webhook endpoints for real-time cluster incident dispatch.
        </p>
      </div>
    </div>

    <form class="settings-form" @submit.prevent="emit('save', 'notifications')">
      <!-- Row 1: Enable SMTP Toggle -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="toggle-smtp">Enable SMTP Outbound Email Relay</label>
          <p class="setting-desc">
            Allows the platform to send alert digests, scheduled PDF reports, and critical event notifications.
          </p>
        </div>
        <div class="setting-control-col">
          <label class="toggle-switch">
            <input id="toggle-smtp" v-model="form.smtp_enabled" type="checkbox" aria-label="Enable SMTP Relay" />
            <span class="toggle-slider"></span>
          </label>
        </div>
      </div>

      <!-- Row 2: SMTP Host -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="smtp-host" :class="{ 'label-disabled': !form.smtp_enabled }">SMTP Host</label>
          <p class="setting-desc">FQDN or IP address of your mail transport agent (e.g. smtp.sendgrid.net).</p>
        </div>
        <div class="setting-control-col">
          <input
            id="smtp-host"
            v-model="form.smtp_host"
            type="text"
            class="input-glass form-input input-compact-lg"
            :disabled="!form.smtp_enabled"
            placeholder="smtp.mailgun.org or smtp.office365.com"
          />
        </div>
      </div>

      <!-- Row 3: SMTP Port -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="smtp-port" :class="{ 'label-disabled': !form.smtp_enabled }">SMTP Port</label>
          <p class="setting-desc">Port for TLS/STARTTLS dispatch (typically 587 or 465).</p>
        </div>
        <div class="setting-control-col">
          <input
            id="smtp-port"
            v-model.number="form.smtp_port"
            type="number"
            min="1"
            max="65535"
            class="input-glass form-input"
            style="width: 100px;"
            :disabled="!form.smtp_enabled"
            placeholder="587"
          />
        </div>
      </div>

      <!-- Row 4: Incident Webhook URL -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="webhook-url">Incident Webhook URL</label>
          <p class="setting-desc">
            HTTP POST endpoint for Slack, Discord, PagerDuty, or Microsoft Teams webhook integrations.
          </p>
        </div>
        <div class="setting-control-col">
          <input
            id="webhook-url"
            v-model="form.webhook_url"
            type="url"
            class="input-glass form-input input-compact-lg"
            placeholder="https://hooks.slack.com/services/T00/B00/XXXXX"
          />
        </div>
      </div>

      <!-- Test Feedback Status -->
      <div v-if="testResult" class="test-feedback-banner test-success font-mono text-xs animate-fade-in">
        <BaseIcon name="check-circle" size="xs" />
        <span>Integration test passed! Status: {{ testResult.status_code }}, Latency: {{ testResult.latency_ms.toFixed(1) }}ms</span>
      </div>
      <div v-if="testError" class="test-feedback-banner test-error font-mono text-xs animate-fade-in">
        <BaseIcon name="alert-triangle" size="xs" />
        <span>Integration test failed: {{ testError }}</span>
      </div>

      <!-- Card Actions Bar -->
      <div class="card-actions-bar">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="testing || !form.webhook_url"
          @click="handleTestNotification"
        >
          <BaseIcon :name="testing ? 'refresh' : 'send'" size="xs" :class="{ 'animate-spin': testing }" />
          <span>{{ testing ? 'Sending Test...' : 'Send Test Notification' }}</span>
        </button>
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('reset', 'notifications')">
          <BaseIcon name="rotate-ccw" size="xs" /> <span>Reset Defaults</span>
        </button>
        <button type="submit" class="btn btn-primary btn-sm" :disabled="saving">
          <BaseIcon name="hard-drive" size="xs" /> <span>{{ saving ? 'Saving Changes...' : 'Save Notification Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.test-feedback-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  margin-top: 14px;
}
.test-success {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid var(--color-success, #10b981);
  color: var(--color-success, #10b981);
}
.test-error {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid var(--color-danger, #f43f5e);
  color: var(--color-danger, #f43f5e);
}
</style>
