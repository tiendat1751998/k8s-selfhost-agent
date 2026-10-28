<template>
  <div class="login-page">
    <div class="login-background-glow" aria-hidden="true"></div>

    <!-- Mobile 40-44px Command Bar (<768px) -->
    <header class="login-mobile-command-bar mobile-only" role="banner">
      <div class="command-bar-left">
        <BaseIcon name="lock" size="sm" class="command-bar-icon" />
        <span class="command-bar-title font-bold">Enterprise Access</span>
      </div>
      <div class="command-bar-right">
        <span class="command-bar-badge font-mono">TLS v1.3</span>
      </div>
    </header>

    <main class="login-container">
      <!-- Left-side branding / status mesh hero banner -->
      <LoginBrandingHero />

      <!-- Auth Form Card -->
      <div class="login-card glass-panel animate-fade-in">
        <!-- Mobile-only Brand Header (visible when hero banner is hidden on narrow screens) -->
        <div class="mobile-brand">
          <div class="brand-icon-wrapper">
            <div class="brand-icon"><BaseIcon name="anchor" size="lg" /></div>
            <div class="brand-glow"></div>
          </div>
          <h1 class="brand-title">K8S<span>CONTROL</span></h1>
          <p class="brand-subtitle">Enterprise Hybrid Control Plane</p>
        </div>

        <!-- Status Pill -->
        <div class="env-pill">
          <span class="pulse-dot pulse-dot-emerald"></span>
          <span class="env-text">Zero-Trust Control Plane</span>
        </div>

        <!-- Error Box -->
        <div v-if="errorMessage" class="error-banner animate-fade-in" role="alert">
          <BaseIcon name="alert-triangle" size="sm" class="error-icon" />
          <span class="error-text">{{ errorMessage }}</span>
        </div>

        <!-- Steps Transition -->
        <Transition name="step-fade" mode="out-in">
          <!-- STEP 1: Email & Password -->
          <form
            v-if="step === 'credentials'"
            key="step-creds"
            class="login-form"
            @submit.prevent="handleCredentialsSubmit"
          >
            <div class="form-group">
              <label for="username" class="form-label">Email / Username</label>
              <input
                id="username"
                name="username"
                v-model="email"
                type="text"
                required
                autocomplete="username"
                aria-label="Username or Email"
                placeholder="user@k8s.local"
                class="input-glass form-input"
                :disabled="isLoading"
              />
            </div>

            <div class="form-group">
              <label for="password" class="form-label">Password</label>
              <input
                id="password"
                name="password"
                v-model="password"
                type="password"
                required
                autocomplete="current-password"
                aria-label="Password"
                placeholder="••••••••••••"
                class="input-glass form-input"
                :disabled="isLoading"
              />
            </div>

            <div class="form-row-remember">
              <label for="rememberMe" class="checkbox-label">
                <input
                  id="rememberMe"
                  name="rememberMe"
                  type="checkbox"
                  v-model="rememberMe"
                  class="form-checkbox"
                  :disabled="isLoading"
                />
                <span>Remember credentials</span>
              </label>
            </div>

            <button
              type="submit"
              class="btn btn-primary login-btn"
              :disabled="isLoading || !email || !password"
            >
              <span v-if="isLoading" class="spinner"></span>
              <span>{{ isLoading ? 'Authenticating...' : 'Sign In to Console' }}</span>
            </button>
          </form>

          <!-- STEP 2: TOTP / Recovery MFA -->
          <MFAVerificationStep
            v-else
            key="step-mfa"
            :mode="step === 'recovery' ? 'recovery' : 'totp'"
            v-model:totp-code="totpCode"
            v-model:recovery-code="recoveryCode"
            :loading="isLoading"
            @submit-totp="handleTotpSubmit"
            @submit-recovery="handleRecoverySubmit"
            @switch-to-recovery="switchToRecovery"
            @switch-to-totp="switchToTotp"
            @back="backToCredentials"
            @totp-input="onTotpInput"
          />
        </Transition>

        <!-- Footer Info -->
        <div class="login-footer">© 2026 K8sControl • Enterprise Hybrid Control Plane</div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import '../assets/styles/views/login.css'
import { useLoginAuth } from '../composables/useLoginAuth'
import LoginBrandingHero from '../components/auth/LoginBrandingHero.vue'
import MFAVerificationStep from '../components/auth/MFAVerificationStep.vue'

const {
  step,
  email,
  password,
  totpCode,
  recoveryCode,
  rememberMe,
  errorMessage,
  isLoading,
  handleCredentialsSubmit,
  handleTotpSubmit,
  handleRecoverySubmit,
  switchToRecovery,
  switchToTotp,
  backToCredentials,
  onTotpInput,
} = useLoginAuth()
</script>
