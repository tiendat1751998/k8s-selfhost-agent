<script setup lang="ts">
import { ref } from 'vue'
import { authApi } from '../../api/auth'
import { useAuthStore } from '../../stores/authStore'
import { validatePasswordChange } from './passwordChangeUtils'
import BaseIcon from '../ui/BaseIcon.vue'

const authStore = useAuthStore()

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const isSubmitting = ref(false)
const errorMessage = ref<string | null>(null)
const successMessage = ref<string | null>(null)

async function handleSubmit() {
  errorMessage.value = null
  successMessage.value = null

  const validation = validatePasswordChange(currentPassword.value, newPassword.value, confirmPassword.value)
  if (!validation.valid) {
    errorMessage.value = validation.error
    return
  }

  isSubmitting.value = true
  try {
    const res = await authApi.changePassword({
      current_password: currentPassword.value,
      new_password: newPassword.value,
    })
    successMessage.value = res?.message || 'Password updated successfully. Please use your new password on next login.'
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    if (authStore.user) {
      authStore.user.requires_password_change = false
    }
  } catch (err: unknown) {
    errorMessage.value = err instanceof Error ? err.message : 'Password update failed. Verify current password.'
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in password-card">
    <div class="card-header">
      <div>
        <h3 class="card-title">Change Account Password</h3>
        <p class="card-subtitle">
          Update your credentials for enterprise cluster authentication and web console access.
        </p>
      </div>
    </div>

    <!-- Alert when password change is required -->
    <div
      v-if="authStore.user?.requires_password_change"
      class="password-alert alert-banner-warning animate-fade-in"
      role="alert"
    >
      <BaseIcon name="alert-triangle" size="xs" />
      <span>Password change required: Please update your temporary or expired credentials to continue.</span>
    </div>

    <!-- Feedback messages -->
    <div v-if="errorMessage" class="password-alert alert-banner-danger animate-fade-in" role="alert">
      <BaseIcon name="alert-triangle" size="xs" />
      <span>{{ errorMessage }}</span>
    </div>

    <div v-if="successMessage" class="password-alert alert-banner-success animate-fade-in" role="status">
      <BaseIcon name="check-circle" size="xs" />
      <span>{{ successMessage }}</span>
    </div>

    <form class="settings-form" @submit.prevent="handleSubmit">
      <div class="form-group">
        <label class="form-label" for="current-password">Current Password</label>
        <input
          id="current-password"
          v-model="currentPassword"
          type="password"
          required
          autocomplete="current-password"
          class="input-glass form-input"
          placeholder="Enter current account password"
          :disabled="isSubmitting"
        />
      </div>

      <div class="form-row-2">
        <div class="form-group flex-1">
          <label class="form-label" for="new-password">New Password (min 10 characters)</label>
          <input
            id="new-password"
            v-model="newPassword"
            type="password"
            required
            minlength="10"
            autocomplete="new-password"
            class="input-glass form-input"
            placeholder="At least 10 characters"
            :disabled="isSubmitting"
          />
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="confirm-password">Confirm New Password</label>
          <input
            id="confirm-password"
            v-model="confirmPassword"
            type="password"
            required
            minlength="10"
            autocomplete="new-password"
            class="input-glass form-input"
            placeholder="Re-enter new password"
            :disabled="isSubmitting"
          />
        </div>
      </div>

      <div class="card-footer-actions">
        <button
          type="submit"
          class="btn btn-primary btn-sm"
          :disabled="isSubmitting || !currentPassword || !newPassword || !confirmPassword"
        >
          <BaseIcon :name="isSubmitting ? 'clock' : 'lock'" size="xs" />
          <span>{{ isSubmitting ? 'Updating Password...' : 'Update Password' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.password-card { margin-top: 16px; }
.password-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 12.5px;
  margin-bottom: 14px;
}
.alert-banner-warning {
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.35);
  color: #fbbf24;
}
.alert-banner-danger {
  background: rgba(225, 29, 72, 0.12);
  border: 1px solid rgba(225, 29, 72, 0.35);
  color: #f43f5e;
}
.alert-banner-success {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.35);
  color: #34d399;
}
.form-row-2 {
  display: flex;
  gap: 16px;
}
.card-footer-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
@media (max-width: 640px) {
  .form-row-2 { flex-direction: column; gap: 10px; }
}
</style>
