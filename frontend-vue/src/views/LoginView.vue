<template>
  <div class="login-page">
    <div class="login-background-glow" aria-hidden="true"></div>

    <!-- Mobile 40-44px Command Bar (<768px) -->
    <header class="login-mobile-command-bar mobile-only" role="banner">
      <div class="command-bar-left">
        <span class="command-bar-icon">🔐</span>
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
            <div class="brand-icon">⎈</div>
            <div class="brand-glow"></div>
          </div>
          <h1 class="brand-title">K8S<span>CONTROL</span></h1>
          <p class="brand-subtitle">Enterprise Hybrid Control Plane</p>
        </div>

        <!-- Environment Badge -->
        <div class="env-pill">
          <span class="pulse-dot pulse-dot-emerald"></span>
          <span class="env-text">Air-Gapped ZeroTrust Enforced</span>
          <span class="env-chip">TLS v1.3</span>
        </div>

        <!-- Error Box -->
        <div v-if="errorMessage" class="error-banner animate-fade-in" role="alert">
          <span class="error-icon">⚠️</span>
          <span class="error-text">{{ errorMessage }}</span>
        </div>

        <!-- Steps Transition -->
        <Transition name="step-fade" mode="out-in">
          <!-- STEP 1: Email & Password + Enterprise SSO Gateway -->
          <LoginFormCard
            v-if="step === 'credentials'"
            key="step-creds"
            v-model:email="email"
            v-model:password="password"
            v-model:remember-me="rememberMe"
            :loading="isLoading"
            @submit="handleCredentialsSubmit"
            @sso-login="handleSsoLogin"
          />

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

        <!-- Security Disclaimer Badges -->
        <div class="security-disclaimer-badges" role="complementary" aria-label="Security Disclaimers">
          <div class="sec-badge" title="FIPS 140-3 Cryptographic Boundary">
            <span class="sec-badge-icon">🛡️</span>
            <span class="sec-badge-text">FIPS 140-3 Enforced</span>
          </div>
          <div class="sec-badge" title="ZeroTrust Multi-Factor Identity Gate">
            <span class="sec-badge-icon">🔒</span>
            <span class="sec-badge-text">ZeroTrust MFA Gate</span>
          </div>
          <div class="sec-badge" title="Immutable SOC2 Audit Stream">
            <span class="sec-badge-icon">📋</span>
            <span class="sec-badge-text">SOC2 Audit Stream</span>
          </div>
        </div>

        <!-- Footer Info -->
        <footer class="login-footer">
          <span>Dual-Sync DR • Trivy Gate • Real-Time Stream</span>
        </footer>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import '../assets/styles/views/login.css'
import '../assets/styles/components/login-sso.css'
import { useLoginAuth } from '../composables/useLoginAuth'
import LoginBrandingHero from '../components/auth/LoginBrandingHero.vue'
import LoginFormCard from '../components/auth/LoginFormCard.vue'
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
  handleSsoLogin,
  switchToRecovery,
  switchToTotp,
  backToCredentials,
  onTotpInput,
} = useLoginAuth()
</script>
