<script setup lang="ts">
import { useRouter } from 'vue-router'
import type { TOTPStatusResponse } from '../../api/auth'

const router = useRouter()

defineProps<{
  form: {
    session_timeout_minutes: number
    password_min_length: number
    require_2fa: boolean
  }
  saving: boolean
  totpStatus: TOTPStatusResponse | null
  loadingTotpStatus: boolean
}>()

const emit = defineEmits<{
  (e: 'save', category: 'security'): void
  (e: 'reset', category: 'security'): void
  (e: 'openDisable2FA'): void
  (e: 'openDisable2fa'): void
}>()
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in">
    <div class="card-header">
      <div>
        <h2 class="card-title">Security & Access Policy</h2>
        <p class="card-subtitle">
          Configure session lifetime thresholds, credential complexity minimums, and MFA mandates.
        </p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" @click="emit('reset', 'security')">
        <span>↺ Reset Defaults</span>
      </button>
    </div>

    <form class="settings-form" @submit.prevent="emit('save', 'security')">
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label" for="sec-timeout">
            <span>Session Timeout (Minutes)</span>
            <span class="required">*</span>
          </label>
          <p class="field-desc">Inactivity duration before automatic token invalidation (5 to 1440 minutes).</p>
          <input
            id="sec-timeout"
            v-model.number="form.session_timeout_minutes"
            type="number"
            min="5"
            max="1440"
            class="input-glass form-input"
            required
          />
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="sec-pass-len">
            <span>Minimum Password Length</span>
            <span class="required">*</span>
          </label>
          <p class="field-desc">Minimum character count required for tenant user credentials.</p>
          <input
            id="sec-pass-len"
            v-model.number="form.password_min_length"
            type="number"
            min="6"
            max="64"
            class="input-glass form-input"
            required
          />
        </div>
      </div>

      <div class="form-group toggle-group">
        <div class="toggle-info">
          <span id="lbl-require-2fa" class="toggle-label">Require Multi-Factor Authentication (2FA)</span>
          <p class="field-desc">
            Mandate TOTP authenticator verification or WebAuthn hardware tokens for all cluster operators.
          </p>
        </div>
        <label class="toggle-switch">
          <input id="toggle-require-2fa" v-model="form.require_2fa" type="checkbox" aria-labelledby="lbl-require-2fa" />
          <span class="toggle-slider"></span>
        </label>
      </div>

      <!-- Personal Operator 2FA Card -->
      <div class="user-2fa-section">
        <div class="section-divider"></div>
        <div class="user-2fa-header">
          <div class="user-2fa-info">
            <h3 class="subsection-title">🔐 Personal Two-Factor Authentication (TOTP)</h3>
            <p class="field-desc">
              Protect your individual operator credentials with Google Authenticator or RFC 6238 compliant TOTP apps.
            </p>
          </div>
          <div class="user-2fa-badge-wrap">
            <span v-if="loadingTotpStatus" class="spinner"></span>
            <span v-else-if="totpStatus?.enabled" class="badge badge-emerald">
              ✅ 2FA ACTIVE
            </span>
            <span v-else class="badge badge-amber">
              ⚠️ NOT CONFIGURED
            </span>
          </div>
        </div>

        <div class="user-2fa-card">
          <div v-if="totpStatus?.enabled" class="user-2fa-active-view">
            <div class="status-detail">
              <span class="status-icon">🛡️</span>
              <div>
                <span class="status-title">Two-Factor Authentication is Enabled</span>
                <p class="status-sub">
                  Active and verified {{ totpStatus.verified_at ? 'on ' + new Date(totpStatus.verified_at).toLocaleDateString() : 'for this account' }}. 10 single-use emergency recovery codes active.
                </p>
              </div>
            </div>
            <div class="user-2fa-actions">
              <button
                type="button"
                class="btn btn-secondary btn-sm btn-danger-outline"
                @click="emit('openDisable2FA'); emit('openDisable2fa')"
              >
                <span>Disable 2FA</span>
              </button>
            </div>
          </div>

          <div v-else class="user-2fa-inactive-view">
            <div class="status-detail">
              <span class="status-icon">🔒</span>
              <div>
                <span class="status-title">Two-Factor Authentication is Not Enabled</span>
                <p class="status-sub">
                  Add a time-based one-time password authenticator to secure your cluster operations.
                </p>
              </div>
            </div>
            <div class="user-2fa-actions">
              <button
                type="button"
                class="btn btn-primary btn-sm"
                @click="router.push('/settings/2fa-setup')"
              >
                <span>Enable 2FA Wizard →</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="security-info-box">
        <div class="info-box-header">
          <span>🔒 Zero-Trust Enforcement Notice</span>
        </div>
        <p>
          Modifications to security policies are applied across all active tenant sessions immediately upon saving.
          Token invalidation triggers seamless re-authentication without loss of background telemetry.
        </p>
      </div>

      <div class="form-actions">
        <button type="submit" class="btn btn-primary" :disabled="saving">
          <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Security Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
