<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

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

      <!-- Card Actions Bar -->
      <div class="card-actions-bar">
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
