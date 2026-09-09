<script setup lang="ts">
import { useRouter } from 'vue-router'
import type { SettingsFormState, CategoryKey } from '../../composables/useSettings'
import type { TOTPStatusResponse } from '../../api/auth'

const router = useRouter()

defineProps<{
  form: SettingsFormState
  saving: boolean
  isDirty: boolean
  totpStatus: TOTPStatusResponse | null
  loadingTotpStatus: boolean
}>()

const emit = defineEmits<{
  (e: 'save', category: CategoryKey): void
  (e: 'reset', category: CategoryKey): void
  (e: 'openDisable2FA'): void
}>()
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in">
    <div class="card-header">
      <div class="card-title-group">
        <div>
          <h2 class="card-title">Security & Zero-Trust Access Policy</h2>
          <p class="card-subtitle">
            Configure MFA mandates, JWT session durations, credential complexity, and API rate limiting policies.
          </p>
        </div>
        <span v-if="isDirty" class="dirty-indicator-pill">● Unsaved Changes</span>
      </div>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        @click="emit('reset', 'security')"
      >
        <span>↺ Reset Defaults</span>
      </button>
    </div>

    <form class="settings-form" @submit.prevent="emit('save', 'security')">
      <!-- Session and Password Policy Row -->
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label" for="sec-jwt-duration">
            <span>JWT Session Duration (Hours)</span>
            <span class="required">*</span>
          </label>
          <p class="field-desc">Maximum valid duration of operator access tokens before mandatory refresh.</p>
          <input
            id="sec-jwt-duration"
            v-model.number="form.jwt_session_duration_hours"
            type="number"
            min="1"
            max="72"
            class="input-glass form-input"
            required
          />
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="sec-timeout">
            <span>Inactivity Timeout (Minutes)</span>
            <span class="required">*</span>
          </label>
          <p class="field-desc">Idle duration before automatic UI locking (5 to 1440 minutes).</p>
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
          <p class="field-desc">Minimum character count required for tenant credentials.</p>
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

      <!-- Rate Limiting Policy Row -->
      <div class="form-group toggle-group">
        <div class="toggle-info">
          <span class="toggle-label">API Gateway Rate Limiting & DoS Shield</span>
          <p class="field-desc">
            Enforces token bucket rate limiting on cluster endpoints to prevent resource exhaustion.
          </p>
        </div>
        <label class="toggle-switch">
          <input v-model="form.rate_limit_enabled" type="checkbox" />
          <span class="toggle-slider"></span>
        </label>
      </div>

      <div v-if="form.rate_limit_enabled" class="form-row animate-fade-in">
        <div class="form-group flex-1">
          <label class="form-label" for="sec-rate-limit">
            <span>Max Requests / Min (Per Client IP)</span>
          </label>
          <input
            id="sec-rate-limit"
            v-model.number="form.rate_limit_requests_per_min"
            type="number"
            min="60"
            max="60000"
            class="input-glass form-input"
          />
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="sec-rate-burst">
            <span>Burst Request Capacity</span>
          </label>
          <input
            id="sec-rate-burst"
            v-model.number="form.rate_limit_burst"
            type="number"
            min="10"
            max="5000"
            class="input-glass form-input"
          />
        </div>
      </div>

      <div class="form-group">
        <label class="form-label" for="sec-ip-allowlist">
          <span>Trusted CIDR IP Allowlist</span>
        </label>
        <p class="field-desc">Comma-separated IPv4/IPv6 CIDRs exempt from aggressive rate throttling (e.g. 10.0.0.0/8, 192.168.1.0/24).</p>
        <input
          id="sec-ip-allowlist"
          v-model="form.ip_allowlist"
          type="text"
          class="input-glass form-input font-mono"
          placeholder="10.244.0.0/16, 172.16.0.0/12"
        />
      </div>

      <!-- Mandatory 2FA Toggle -->
      <div class="form-group toggle-group">
        <div class="toggle-info">
          <span class="toggle-label">Require Multi-Factor Authentication (2FA) for All Operators</span>
          <p class="field-desc">
            Mandates TOTP authenticator verification or WebAuthn hardware tokens for all cluster tenants.
          </p>
        </div>
        <label class="toggle-switch">
          <input v-model="form.require_2fa" type="checkbox" />
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
              Protect your individual operator credentials with Google Authenticator, Authy, or RFC 6238 compliant TOTP apps.
            </p>
          </div>
          <div class="user-2fa-badge-wrap">
            <span v-if="loadingTotpStatus" class="spinner spinner-sm"></span>
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
                  Active and verified {{ totpStatus.verified_at ? 'on ' + new Date(totpStatus.verified_at).toLocaleDateString() : 'for this account' }}. Recovery codes active.
                </p>
              </div>
            </div>
            <div class="user-2fa-actions">
              <button
                type="button"
                class="btn btn-secondary btn-sm btn-danger-outline"
                @click="emit('openDisable2FA')"
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
                  Add a time-based one-time password authenticator to secure your cluster management operations.
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

      <!-- Zero-Trust Notice -->
      <div class="security-info-box">
        <div class="info-box-header">
          <span>🔒 Zero-Trust Enforcement Notice</span>
        </div>
        <p>
          Modifications to security policies are applied across all active tenant sessions immediately upon saving.
          Token invalidation triggers seamless re-authentication without loss of background telemetry.
        </p>
      </div>

      <!-- Form Actions -->
      <div class="form-actions">
        <span class="field-desc">Security policies apply to all authenticated operator roles and service accounts.</span>
        <button type="submit" class="btn btn-primary" :disabled="saving">
          <span v-if="saving" class="spinner spinner-sm"></span>
          <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Security Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
