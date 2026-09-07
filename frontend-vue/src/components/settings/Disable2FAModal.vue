<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'

defineProps<{
  show: boolean
  password?: string
  totpCode?: string
  disabling: boolean
  error: string | null
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'update:password', value: string): void
  (e: 'update:totpCode', value: string): void
  (e: 'confirm'): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    title="Disable Two-Factor Authentication"
    max-width="480px"
    @close="emit('update:show', false)"
  >
    <div class="disable-2fa-form">
      <div v-if="error" class="modal-error-banner">
        {{ error }}
      </div>

      <p class="field-desc">
        Disabling 2FA reduces your account security. Please verify your current account password and TOTP code to proceed.
      </p>

      <div class="form-group">
        <label class="form-label" for="disable-password">Current Password</label>
        <input
          type="password"
          class="form-input"
          id="disable-password" autocomplete="current-password" placeholder="Enter current password"
          :value="password"
          @input="emit('update:password', ($event.target as HTMLInputElement).value)"
        />
      </div>

      <div class="form-group">
        <label class="form-label" for="disable-totp-code">Current TOTP Code (6 Digits)</label>
        <input
          type="text"
          id="disable-totp-code" inputmode="numeric" pattern="[0-9]*" class="form-input totp-input-mini"
          placeholder="000000"
          maxlength="6"
          :value="totpCode"
          @input="emit('update:totpCode', ($event.target as HTMLInputElement).value)"
        />
      </div>

      <div class="modal-form-actions">
        <button
          type="button"
          class="btn btn-secondary"
          @click="emit('update:show', false)"
        >
          Cancel
        </button>
        <button
          type="button"
          class="btn btn-danger"
          :disabled="disabling || !password || !totpCode"
          @click="emit('confirm')"
        >
          <span v-if="disabling" class="spinner spinner-sm"></span>
          <span v-else>Disable 2FA</span>
        </button>
      </div>
    </div>
  </ModalDrawer>
</template>
