<template>
  <div class="setup-container animate-fade-in">
    <!-- Header -->
    <header class="view-header">
      <div class="header-left">
        <button class="btn btn-secondary btn-sm back-nav-btn" @click="router.push('/settings')">
          <span>← Back to Settings</span>
        </button>
        <div class="title-with-badge">
          <h1 class="page-title">Two-Factor Authentication Setup</h1>
          <span class="badge badge-cyan">TOTP RFC 6238</span>
        </div>
        <p class="page-subtitle">
          Enhance cluster control-plane security by binding a time-based one-time password (TOTP) authenticator.
        </p>
      </div>
    </header>

    <!-- Desktop Step Wizard Navigation -->
    <TOTPStepWizard :current-step="setup.currentStep.value" />

    <!-- Error Banner Alert -->
    <div v-if="setup.errorMessage.value" class="error-banner animate-fade-in" role="alert">
      <span class="error-icon" aria-hidden="true">⚠️</span>
      <div class="error-content">
        <span class="error-text">{{ setup.errorMessage.value }}</span>
      </div>
      <button class="banner-close-btn" aria-label="Dismiss error" @click="setup.errorMessage.value = ''">✕</button>
    </div>

    <!-- Wizard Card -->
    <main class="wizard-card glass-panel">
      <Transition name="step-slide" mode="out-in">
        <!-- STEP 1: Overview -->
        <div v-if="setup.currentStep.value === 'intro'" key="intro" class="wizard-step-content">
          <div class="step-hero">
            <div class="hero-icon-wrap"><span class="hero-icon">🛡️</span></div>
            <h2 class="hero-title">Protect Your Operator Account</h2>
            <p class="hero-text">
              Two-factor authentication adds an essential second layer of defense. In addition to your password, you will be prompted for a temporary 6-digit verification code whenever signing in.
            </p>
          </div>
          <div class="info-cards-grid">
            <div class="info-card">
              <div class="info-card-icon">📱</div>
              <div class="info-card-body">
                <h3>Supported Authenticator Apps</h3>
                <p>Google Authenticator, Microsoft Authenticator, 1Password, Bitwarden, or Authy.</p>
                <div class="app-links"><span class="store-tag">iOS App Store</span><span class="store-tag">Google Play Store</span></div>
              </div>
            </div>
            <div class="info-card">
              <div class="info-card-icon">⚡</div>
              <div class="info-card-body">
                <h3>Air-Gapped Ready</h3>
                <p>TOTP relies strictly on offline cryptographic HMAC-SHA1 time synchronization (RFC 6238). No external telemetry required.</p>
              </div>
            </div>
          </div>
          <div class="step-actions">
            <button class="btn btn-secondary" @click="router.push('/settings')"><span>Cancel</span></button>
            <button class="btn btn-primary" :disabled="setup.loading.value" @click="setup.startSetup()">
              <span v-if="setup.loading.value" class="spinner" aria-hidden="true"></span>
              <span>{{ setup.loading.value ? 'Initializing Key...' : 'Get Started →' }}</span>
            </button>
          </div>
        </div>

        <!-- STEP 2: Scan QR Code -->
        <div v-else-if="setup.currentStep.value === 'scan'" key="scan" class="wizard-step-content">
          <div class="step-hero">
            <h2 class="hero-title">Scan the QR Code</h2>
            <p class="hero-text">Open your authenticator app and scan the QR code below, or manually enter the Base32 security key.</p>
          </div>
          <QRCodeDisplay
            :qr-image-src="setup.qrImageSrc.value"
            :secret-key="setup.secretKey.value"
            :secret-copied="setup.secretCopied.value"
            :account-email="authStore.user?.email"
            @copy-secret="setup.copySecretToClipboard()"
          />
          <div class="step-actions">
            <button class="btn btn-secondary" @click="setup.currentStep.value = 'intro'"><span>← Back</span></button>
            <button class="btn btn-primary" @click="handleGoToVerify"><span>I've Scanned the QR Code →</span></button>
          </div>
        </div>

        <!-- STEP 3: Verify Code -->
        <div v-else-if="setup.currentStep.value === 'verify'" key="verify" class="wizard-step-content">
          <div class="step-hero">
            <div class="hero-icon-wrap"><span class="hero-icon">🔢</span></div>
            <h2 class="hero-title">Verify Authenticator Code</h2>
            <p class="hero-text">Enter the 6-digit code currently shown in your authenticator app to confirm correct synchronization.</p>
          </div>
          <form class="verify-form" @submit.prevent="setup.verifyAndEnable()">
            <div class="totp-input-box">
              <label for="wizard-totp-code" class="form-label">6-Digit Code</label>
              <input
                id="wizard-totp-code"
                ref="verifyInputRef"
                :value="setup.verificationCode.value"
                type="text"
                inputmode="numeric"
                pattern="[0-9]*"
                maxlength="6"
                placeholder="000000"
                required
                autocomplete="one-time-code"
                class="input-glass form-input totp-input"
                :disabled="setup.loading.value"
                @input="setup.handleVerificationInput(($event.target as HTMLInputElement).value)"
              />
              <span class="verify-hint">Codes refresh automatically every 30 seconds.</span>
            </div>
            <div class="step-actions">
              <button type="button" class="btn btn-secondary" :disabled="setup.loading.value" @click="setup.currentStep.value = 'scan'">
                <span>← Back to QR Code</span>
              </button>
              <button type="submit" class="btn btn-primary" :disabled="setup.loading.value || setup.verificationCode.value.length !== 6">
                <span v-if="setup.loading.value" class="spinner" aria-hidden="true"></span>
                <span>{{ setup.loading.value ? 'Verifying...' : 'Verify & Enable 2FA' }}</span>
              </button>
            </div>
          </form>
        </div>

        <!-- STEP 4: Recovery Codes -->
        <div v-else-if="setup.currentStep.value === 'recovery'" key="recovery" class="wizard-step-content">
          <div class="step-hero">
            <div class="hero-icon-wrap alert-icon-wrap"><span class="hero-icon">📦</span></div>
            <h2 class="hero-title">Save Your Emergency Recovery Codes</h2>
            <p class="hero-text">If you lose access to your authenticator app, these one-time recovery codes are the <strong>only way</strong> to access your account.</p>
          </div>
          <RecoveryCodesCard
            v-model:has-saved-codes="setup.hasSavedCodes.value"
            :recovery-codes="setup.recoveryCodes.value"
            :all-codes-copied="setup.allCodesCopied.value"
            @copy-codes="setup.copyAllCodes()"
            @download-txt="setup.downloadCodesAsFile()"
            @print="setup.printRecoveryCodes()"
          />
          <div class="step-actions">
            <button class="btn btn-primary" :disabled="!setup.hasSavedCodes.value" @click="setup.currentStep.value = 'success'">
              <span>Complete Setup →</span>
            </button>
          </div>
        </div>

        <!-- STEP 5: Success -->
        <div v-else-if="setup.currentStep.value === 'success'" key="success" class="wizard-step-content">
          <div class="step-hero">
            <div class="hero-icon-wrap success-icon-wrap"><span class="hero-icon">✅</span></div>
            <h2 class="hero-title">Two-Factor Authentication is Active!</h2>
            <p class="hero-text">Your account is now secured with TOTP multi-factor authentication. You will be asked for a verification code upon subsequent sign-ins.</p>
          </div>
          <div class="success-features-card">
            <div class="feature-row">
              <span class="feature-icon">🛡️</span>
              <div class="feature-info"><h4>Cluster Perimeter Hardened</h4><p>Brute-force and credential leak vectors mitigated.</p></div>
            </div>
            <div class="feature-row">
              <span class="feature-icon">🔑</span>
              <div class="feature-info"><h4>Recovery Codes Ready</h4><p>Emergency single-use scratch tokens active.</p></div>
            </div>
            <div class="feature-row">
              <span class="feature-icon">🔄</span>
              <div class="feature-info"><h4>Auto-Refreshed Sessions</h4><p>Secure HttpOnly token rotation active across all endpoints.</p></div>
            </div>
          </div>
          <div class="step-actions">
            <button class="btn btn-primary" @click="router.push('/settings')"><span>Return to Settings</span></button>
          </div>
        </div>
      </Transition>
    </main>

    <!-- Mobile Compact Setup Cards (< 640px) -->
    <TOTPMobileCards
      v-if="setup.currentStep.value !== 'success'"
      class="mobile-only"
      :current-step="setup.currentStep.value"
      :is-enforced="setup.mfaEnforcement.value.isEnforced"
      @select-step="(step) => setup.currentStep.value = step"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/authStore'
import { useTOTPSetup } from '../composables/useTOTPSetup'
import TOTPStepWizard from '../components/totp/TOTPStepWizard.vue'
import QRCodeDisplay from '../components/totp/QRCodeDisplay.vue'
import RecoveryCodesCard from '../components/totp/RecoveryCodesCard.vue'
import TOTPMobileCards from '../components/totp/TOTPMobileCards.vue'
import '../assets/styles/views/totp.css'

const router = useRouter()
const authStore = useAuthStore()
const verifyInputRef = ref<HTMLInputElement | null>(null)
const setup = useTOTPSetup()

async function handleGoToVerify() {
  setup.goToVerify()
  await nextTick()
  verifyInputRef.value?.focus()
}

onMounted(() => {
  setup.checkMFAStatus()
})
</script>
