<script setup lang="ts">
import type { AlertChannel } from '../../api/management'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  channels: AlertChannel[]
}>()

const emit = defineEmits<{
  (e: 'test', channelName: string): void
}>()
</script>

<template>
  <div>
    <div v-if="channels.length === 0" class="empty-list glass-panel">
      No delivery channels configured yet. Click "+ Add Channel" above to configure Slack, Telegram, Email, or Webhooks.
    </div>
    <div v-else class="channels-grid">
      <div v-for="chan in channels" :key="chan.ID" class="channel-card glass-panel">
        <div class="channel-card-header">
          <div class="chan-title-wrap">
            <span class="chan-icon">
              {{ chan.Type === 'slack' ? '💬' : chan.Type === 'telegram' ? '✈️' : chan.Type === 'email' ? '✉️' : '🔗' }}
            </span>
            <div>
              <h3 class="chan-name">{{ chan.Name }}</h3>
              <small class="chan-type font-mono uppercase text-cyan">{{ chan.Type }} integration</small>
            </div>
          </div>
          <StatusBadge :status="chan.Enabled ? 'healthy' : 'standby'" :label="chan.Enabled ? 'CONNECTED' : 'DISABLED'" size="sm" />
        </div>

        <div class="channel-body">
          <div v-if="chan.Config?.webhook_url" class="cstat-row">
            <span class="cstat-key">Webhook Target:</span>
            <span class="cstat-val font-mono text-muted">https://hooks.slack.com/...</span>
          </div>
          <div v-if="chan.Config?.channel" class="cstat-row">
            <span class="cstat-key">Channel:</span>
            <span class="cstat-val font-mono text-cyan">{{ chan.Config.channel }}</span>
          </div>
          <div v-if="chan.Config?.recipients" class="cstat-row">
            <span class="cstat-key">Recipients:</span>
            <span class="cstat-val font-mono text-muted">{{ chan.Config.recipients.join(', ') }}</span>
          </div>
          <div v-if="chan.Config?.endpoint" class="cstat-row">
            <span class="cstat-key">Endpoint:</span>
            <span class="cstat-val font-mono text-muted">{{ chan.Config.endpoint }}</span>
          </div>
        </div>

        <div class="channel-footer">
          <button class="btn btn-secondary btn-sm" @click="emit('test', chan.Name)">
            <span>⚡ Test Dispatch</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
