<script setup lang="ts">
defineProps<{
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
      <button type="button" class="btn btn-secondary btn-sm" @click="emit('reset', 'notifications')">
        <span>↺ Reset Defaults</span>
      </button>
    </div>

    <form class="settings-form" @submit.prevent="emit('save', 'notifications')">
      <div class="form-group toggle-group">
        <div class="toggle-info">
          <span id="lbl-smtp-toggle" class="toggle-label">Enable SMTP Outbound Email Relay</span>
          <p class="field-desc">
            Allows the platform to send alert digests, scheduled PDF reports, and critical event notifications.
          </p>
        </div>
        <label class="toggle-switch">
          <input id="toggle-smtp" v-model="form.smtp_enabled" type="checkbox" aria-labelledby="lbl-smtp-toggle" />
          <span class="toggle-slider"></span>
        </label>
      </div>

      <div class="form-row">
        <div class="form-group flex-2">
          <label class="form-label" for="smtp-host" :class="{ 'label-disabled': !form.smtp_enabled }">
            <span>SMTP Host</span>
          </label>
          <p class="field-desc">FQDN or IP address of your mail transport agent (e.g. smtp.sendgrid.net).</p>
          <input
            id="smtp-host"
            v-model="form.smtp_host"
            type="text"
            class="input-glass form-input"
            :disabled="!form.smtp_enabled"
            placeholder="smtp.mailgun.org or smtp.office365.com"
          />
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="smtp-port" :class="{ 'label-disabled': !form.smtp_enabled }">
            <span>SMTP Port</span>
          </label>
          <p class="field-desc">Port for TLS/STARTTLS dispatch (typically 587 or 465).</p>
          <input
            id="smtp-port"
            v-model.number="form.smtp_port"
            type="number"
            class="input-glass form-input"
            :disabled="!form.smtp_enabled"
            placeholder="587"
          />
        </div>
      </div>

      <div class="form-group">
        <label class="form-label" for="webhook-url">
          <span>Incident Webhook URL</span>
        </label>
        <p class="field-desc">
          HTTP POST endpoint for Slack, Discord, PagerDuty, or Microsoft Teams webhook integrations.
        </p>
        <input
          id="webhook-url"
          v-model="form.webhook_url"
          type="url"
          class="input-glass form-input"
          placeholder="https://hooks.slack.com/services/T00/B00/XXXXX"
        />
      </div>

      <div class="form-actions">
        <button type="submit" class="btn btn-primary" :disabled="saving">
          <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Notification Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
