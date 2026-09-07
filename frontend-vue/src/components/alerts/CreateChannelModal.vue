<script setup lang="ts">
import { ref } from 'vue'
import type { ChannelConfig } from '../../api/management'
import ModalDrawer from '../ui/ModalDrawer.vue'

defineProps<{
  show: boolean
  isSubmitting: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'submit', data: { name: string; type: string; config: ChannelConfig; enabled: boolean }): void
}>()

const newChannel = ref({
  name: '',
  type: 'slack',
  webhook_url: '',
  email: '',
  chat_id: '',
  enabled: true
})

function submit() {
  if (!newChannel.value.name) return
  const configObj: Record<string, unknown> = {}
  if (newChannel.value.type === 'slack') configObj.webhook_url = newChannel.value.webhook_url
  if (newChannel.value.type === 'webhook') configObj.webhook_url = newChannel.value.webhook_url
  if (newChannel.value.type === 'email') configObj.recipients = [newChannel.value.email]
  if (newChannel.value.type === 'telegram') configObj.chat_id = newChannel.value.chat_id

  emit('submit', {
    name: newChannel.value.name,
    type: newChannel.value.type,
    config: configObj,
    enabled: true
  })
}
</script>

<template>
  <ModalDrawer
    :show="show"
    title="Connect Notification Channel"
    subtitle="Register Slack, Telegram, SMTP Email, or PagerDuty Webhooks."
    @update:show="emit('update:show', $event)"
  >
    <form @submit.prevent="submit" class="form-layout">
      <div class="form-group">
        <label>Channel Name</label>
        <input 
          v-model="newChannel.name" 
          type="text" 
          placeholder="e.g. SRE Slack Alerts #k8s-prod" 
          class="input-glass" 
          required 
        />
      </div>

      <div class="form-group">
        <label>Integration Type</label>
        <select v-model="newChannel.type" class="input-glass">
          <option value="slack">Slack Webhook</option>
          <option value="telegram">Telegram Bot</option>
          <option value="email">Corporate SMTP Email</option>
          <option value="webhook">Custom HTTPS Webhook / PagerDuty</option>
        </select>
      </div>

      <div v-if="newChannel.type === 'slack'" class="form-group">
        <label>Slack Incoming Webhook URL</label>
        <input 
          v-model="newChannel.webhook_url" 
          type="url" 
          placeholder="https://hooks.slack.com/services/..." 
          class="input-glass" 
          required 
        />
      </div>

      <div v-if="newChannel.type === 'email'" class="form-group">
        <label>Recipient Email Address</label>
        <input 
          v-model="newChannel.email" 
          type="email" 
          placeholder="sre-oncall@enterprise.io" 
          class="input-glass" 
          required 
        />
      </div>

      <div v-if="newChannel.type === 'telegram'" class="form-group">
        <label>Telegram Chat ID</label>
        <input 
          v-model="newChannel.chat_id" 
          type="text" 
          placeholder="-10029384920" 
          class="input-glass" 
          required 
        />
      </div>

      <div v-if="newChannel.type === 'webhook'" class="form-group">
        <label>Webhook URL</label>
        <input 
          v-model="newChannel.webhook_url" 
          type="url" 
          placeholder="https://hooks.example.com/..." 
          class="input-glass" 
          required 
        />
      </div>
    </form>

    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
      <button class="btn btn-primary" :disabled="isSubmitting" @click="submit">
        {{ isSubmitting ? 'Registering...' : 'Connect Channel' }}
      </button>
    </template>
  </ModalDrawer>
</template>
