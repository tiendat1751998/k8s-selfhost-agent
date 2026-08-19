<template>
  <div class="setup-container animate-fade-in">
    <!-- Header -->
    <div class="view-header">
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
    </div>

    <!-- Stepper Navigation -->
    <div class="stepper-card glass-panel">
      <div class="stepper">
        <div
          v-for="(s, index) in steps"
          :key="s.id"
          class="step-item"
          :class="{
            'step-active': currentStep === s.id,
            'step-completed': currentStepIndex > index,
            'step-disabled': currentStepIndex < index
          }"
        >
          <div class="step-circle">
            <span v-if="currentStepIndex > index">✓</span>
            <span v-else>{{ index + 1 }}</span>
          </div>
          <div class="step-label-group">
            <span class="step-title">{{ s.title }}</span>
            <span class="step-desc">{{ s.desc }}</span>
          </div>
          <div v-if="index < steps.length - 1" class="step-line"></div>
        </div>
      </div>
    </div>

    <!-- Error Banner -->
    <div v-if="errorMessage" class="error-banner animate-fade-in" role="alert">
      <span class="error-icon">⚠️</span>
      <div class="error-content">
        <span class="error-text">{{ errorMessage }}</span>
      </div>
      <button class="banner-close-btn" @click="errorMessage = ''">✕</button>
    </div>

    <!-- Step Content Area -->
    <div class="wizard-card glass-panel">
      <Transition name="step-slide" mode="out-in">
        <!-- STEP 1: Introduction -->
        <div v-if="currentStep === 'intro'" key="intro" class="wizard-step-content">
          <div class="step-hero">
            <div class="hero-icon-wrap">
              <span class="hero-icon">🛡️</span>
            </div>
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
                <div class="app-links">
                  <span class="store-tag">iOS App Store</span>
                  <span class="store-tag">Google Play Store</span>
                </div>
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
            <button class="btn btn-secondary" @click="router.push('/settings')">
              <span>Cancel</span>
            </button>
            <button class="btn btn-primary" :disabled="loading" @click="startSetup">
              <span v-if="loading" class="spinner"></span>
              <span>{{ loading ? 'Initializing Key...' : 'Get Started →' }}</span>
            </button>
          </div>
        </div>

        <!-- STEP 2: Scan QR Code -->
        <div v-else-if="currentStep === 'scan'" key="scan" class="wizard-step-content">
          <div class="step-hero">
            <h2 class="hero-title">Scan the QR Code</h2>
            <p class="hero-text">
              Open your authenticator app and scan the QR code below, or manually enter the Base32 security key.
            </p>
          </div>

          <div class="qr-showcase">
            <div class="qr-box-wrapper">
              <div v-if="qrImageSrc" class="qr-image-container">
                <img :src="qrImageSrc" alt="TOTP QR Code" class="qr-img" />
              </div>
              <div v-else class="qr-loading-placeholder">
                <span class="spinner"></span>
                <span>Generating QR Code...</span>
              </div>
            </div>

            <div class="manual-secret-box">
              <span class="manual-label">Can't scan the QR code? Enter this key manually:</span>
              <div class="secret-display-pill">
                <code class="secret-key font-mono">{{ secretKey || '...' }}</code>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm copy-btn"
                  @click="copySecretToClipboard"
                >
                  <span>{{ secretCopied ? '✓ Copied' : '📋 Copy Key' }}</span>
                </button>
              </div>
              <p class="secret-hint">
                Account: <strong>{{ authStore.user?.email || 'admin@k8s.local' }}</strong> | Type: <strong>Time-based (30s)</strong>
              </p>
            </div>
          </div>

          <div class="step-actions">
            <button class="btn btn-secondary" @click="currentStep = 'intro'">
              <span>← Back</span>
            </button>
            <button class="btn btn-primary" @click="goToVerify">
              <span>I've Scanned the QR Code →</span>
            </button>
          </div>
        </div>

        <!-- STEP 3: Verify Code -->
        <div v-else-if="currentStep === 'verify'" key="verify" class="wizard-step-content">
          <div class="step-hero">
            <div class="hero-icon-wrap">
              <span class="hero-icon">🔢</span>
            </div>
            <h2 class="hero-title">Verify Authenticator Code</h2>
            <p class="hero-text">
              Enter the 6-digit code currently shown in your authenticator app to confirm correct synchronization.
            </p>
          </div>

          <form class="verify-form" @submit.prevent="verifyAndEnable">
            <div class="totp-input-box">
              <label for="wizard-totp-code" class="form-label">6-Digit Code</label>
              <input
                id="wizard-totp-code"
                ref="verifyInputRef"
                v-model="verificationCode"
                type="text"
                inputmode="numeric"
                pattern="[0-9]*"
                maxlength="6"
                placeholder="000000"
                required
                autocomplete="one-time-code"
                class="input-glass form-input totp-input"
                :disabled="loading"
                @input="onVerificationInput"
              />
              <span class="verify-hint">Codes refresh automatically every 30 seconds.</span>
            </div>

            <div class="step-actions">
              <button type="button" class="btn btn-secondary" :disabled="loading" @click="currentStep = 'scan'">
                <span>← Back to QR Code</span>
              </button>
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="loading || verificationCode.length !== 6"
              >
                <span v-if="loading" class="spinner"></span>
                <span>{{ loading ? 'Verifying...' : 'Verify & Enable 2FA' }}</span>
              </button>
            </div>
          </form>
        </div>

        <!-- STEP 4: Recovery Codes -->
        <div v-else-if="currentStep === 'recovery'" key="recovery" class="wizard-step-content">
          <div class="step-hero">
            <div class="hero-icon-wrap alert-icon-wrap">
              <span class="hero-icon">📦</span>
            </div>
            <h2 class="hero-title">Save Your Emergency Recovery Codes</h2>
            <p class="hero-text">
              If you lose access to your authenticator app, these one-time recovery codes are the <strong>only way</strong> to access your account.
            </p>
          </div>

          <div class="alert-box-warning">
            <span class="alert-icon">⚠️</span>
            <div class="alert-text">
              <strong>CRITICAL:</strong> These backup codes will <u>NEVER</u> be displayed again. Store them in a secure password manager or encrypted drive.
            </div>
          </div>

          <!-- Codes Grid -->
          <div class="recovery-codes-grid">
            <div
              v-for="(code, idx) in recoveryCodes"
              :key="idx"
              class="recovery-code-card"
            >
              <span class="code-number">#{{ idx + 1 }}</span>
              <code class="code-value font-mono">{{ code }}</code>
            </div>
          </div>

          <div class="recovery-actions-bar">
            <button class="btn btn-secondary btn-sm" @click="copyAllCodes">
              <span>{{ allCodesCopied ? '✓ All Codes Copied' : '📋 Copy All Codes' }}</span>
            </button>
            <button class="btn btn-secondary btn-sm" @click="downloadCodesAsFile">
              <span>💾 Download as Text File (.txt)</span>
            </button>
          </div>

          <div class="acknowledgement-group">
            <label class="custom-checkbox-label">
              <input v-model="hasSavedCodes" type="checkbox" class="custom-checkbox" />
              <span class="checkbox-text">
                I confirm that I have saved these 10 recovery codes in a secure, encrypted location.
              </span>
            </label>
          </div>

          <div class="step-actions">
            <button
              class="btn btn-primary"
              :disabled="!hasSavedCodes"
              @click="currentStep = 'success'"
            >
              <span>Complete Setup →</span>
            </button>
          </div>
        </div>

        <!-- STEP 5: Success -->
        <div v-else-if="currentStep === 'success'" key="success" class="wizard-step-content">
          <div class="step-hero">
            <div class="hero-icon-wrap success-icon-wrap">
              <span class="hero-icon">✅</span>
            </div>
            <h2 class="hero-title">Two-Factor Authentication is Active!</h2>
            <p class="hero-text">
              Your account is now secured with TOTP multi-factor authentication. You will be asked for a verification code upon subsequent sign-ins.
            </p>
          </div>

          <div class="success-features-card">
            <div class="feature-row">
              <span class="feature-icon">🛡️</span>
              <div class="feature-info">
                <h4>Cluster Perimeter Hardened</h4>
                <p>Brute-force and credential leak vectors mitigated.</p>
              </div>
            </div>
            <div class="feature-row">
              <span class="feature-icon">🔑</span>
              <div class="feature-info">
                <h4>Recovery Codes Ready</h4>
                <p>10 single-use emergency bypass codes active.</p>
              </div>
            </div>
            <div class="feature-row">
              <span class="feature-icon">🔄</span>
              <div class="feature-info">
                <h4>Auto-Refreshed Sessions</h4>
                <p>Secure HttpOnly token rotation active across all endpoints.</p>
              </div>
            </div>
          </div>

          <div class="step-actions">
            <button class="btn btn-primary" @click="router.push('/settings')">
              <span>Return to Settings</span>
            </button>
          </div>
        </div>
      </Transition>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import QRCode from 'qrcode'
import { authApi } from '../api/auth'
import { useAuthStore } from '../stores/authStore'

type WizardStep = 'intro' | 'scan' | 'verify' | 'recovery' | 'success'

interface StepConfig {
  id: WizardStep
  title: string
  desc: string
}

const router = useRouter()
const authStore = useAuthStore()

const steps: StepConfig[] = [
  { id: 'intro', title: '1. Overview', desc: 'Requirements' },
  { id: 'scan', title: '2. Scan QR', desc: 'Authenticator App' },
  { id: 'verify', title: '3. Verify', desc: '6-digit code' },
  { id: 'recovery', title: '4. Backup Codes', desc: 'Save offline' },
  { id: 'success', title: '5. Finished', desc: '2FA Active' },
]

const currentStep = ref<WizardStep>('intro')
const loading = ref(false)
const errorMessage = ref('')

const secretKey = ref('')
const qrUri = ref('')
const qrImageSrc = ref('')
const secretCopied = ref(false)

const verificationCode = ref('')
const verifyInputRef = ref<HTMLInputElement | null>(null)

const recoveryCodes = ref<string[]>([])
const hasSavedCodes = ref(false)
const allCodesCopied = ref(false)

const currentStepIndex = computed(() => {
  return steps.findIndex(s => s.id === currentStep.value)
})

async function startSetup() {
  loading.value = true
  errorMessage.value = ''
  try {
    const res = await authApi.setupTOTP()
    secretKey.value = res.secret
    qrUri.value = res.qr_uri

    // Generate local QR Code data URL
    try {
      qrImageSrc.value = await QRCode.toDataURL(res.qr_uri, {
        width: 260,
        margin: 2,
        color: {
          dark: '#000000',
          light: '#ffffff',
        },
      })
    } catch {
      // Fallback: Use google charts QR if local generation has issue
      qrImageSrc.value = `https://chart.googleapis.com/chart?cht=qr&chs=260x260&chl=${encodeURIComponent(res.qr_uri)}`
    }

    currentStep.value = 'scan'
  } catch (err: unknown) {
    errorMessage.value = err instanceof Error ? err.message : 'Failed to initialize TOTP setup. Please try again.'
  } finally {
    loading.value = false
  }
}

async function copySecretToClipboard() {
  if (!secretKey.value) return
  try {
    await navigator.clipboard.writeText(secretKey.value)
    secretCopied.value = true
    setTimeout(() => {
      secretCopied.value = false
    }, 2500)
  } catch {
    errorMessage.value = 'Clipboard access denied. Please copy the code manually.'
  }
}

async function goToVerify() {
  verificationCode.value = ''
  currentStep.value = 'verify'
  errorMessage.value = ''
  await nextTick()
  verifyInputRef.value?.focus()
}

function onVerificationInput(e: Event) {
  const target = e.target as HTMLInputElement
  verificationCode.value = target.value.replace(/\D/g, '').slice(0, 6)
  if (verificationCode.value.length === 6) {
    verifyAndEnable()
  }
}

async function verifyAndEnable() {
  if (verificationCode.value.length !== 6) return
  loading.value = true
  errorMessage.value = ''

  try {
    const res = await authApi.verifyTOTPSetup(verificationCode.value)
    recoveryCodes.value = res.recovery_codes || []
    currentStep.value = 'recovery'
    hasSavedCodes.value = false
  } catch (err: unknown) {
    errorMessage.value = err instanceof Error ? err.message : 'Verification failed. Please check that your authenticator code is accurate and device time is in sync.'
  } finally {
    loading.value = false
  }
}

async function copyAllCodes() {
  if (!recoveryCodes.value.length) return
  const text = recoveryCodes.value.join('\n')
  try {
    await navigator.clipboard.writeText(text)
    allCodesCopied.value = true
    setTimeout(() => {
      allCodesCopied.value = false
    }, 2500)
  } catch {
    errorMessage.value = 'Failed to copy to clipboard.'
  }
}

function downloadCodesAsFile() {
  if (!recoveryCodes.value.length) return
  const email = authStore.user?.email || 'user'
  const dateStr = new Date().toISOString().slice(0, 10)
  const content = [
    '==================================================',
    'K8S CONTROL PLANE - TWO-FACTOR RECOVERY CODES',
    '==================================================',
    `Account: ${email}`,
    `Generated Date: ${new Date().toUTCString()}`,
    '',
    'Each recovery code can be used ONCE to access your account',
    'if you lose your TOTP authenticator device.',
    'Keep this document in an encrypted, offline location.',
    '',
    'RECOVERY CODES:',
    ...recoveryCodes.value.map((c, i) => `[${i + 1}]  ${c}`),
    '',
    '==================================================',
  ].join('\r\n')

  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `k8s-control-recovery-codes-${dateStr}.txt`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
</script>

<style scoped>
.setup-container {
  max-width: 860px;
  margin: 0 auto;
  padding: 32px 24px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.back-nav-btn {
  margin-bottom: 12px;
}

.title-with-badge {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-title {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.025em;
  color: var(--text-primary);
}

.badge {
  font-size: 11px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 6px;
  font-family: var(--font-mono);
}

.badge-cyan {
  background: rgba(6, 182, 212, 0.15);
  color: var(--accent-cyan);
  border: 1px solid rgba(6, 182, 212, 0.3);
}

.page-subtitle {
  font-size: 13px;
  color: var(--text-secondary);
  margin-top: 4px;
}

/* Stepper */
.stepper-card {
  padding: 18px 24px;
}

.stepper {
  display: flex;
  align-items: center;
  justify-content: space-between;
  position: relative;
}

.step-item {
  display: flex;
  align-items: center;
  gap: 10px;
  position: relative;
  flex: 1;
}

.step-circle {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-medium);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
  flex-shrink: 0;
  transition: all 0.2s;
}

.step-label-group {
  display: flex;
  flex-direction: column;
}

.step-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
}

.step-desc {
  font-size: 10px;
  color: var(--text-muted);
}

.step-line {
  flex: 1;
  height: 2px;
  background: var(--border-subtle);
  margin: 0 12px;
}

/* Stepper States */
.step-item.step-active .step-circle {
  background: var(--accent-cyan);
  border-color: var(--accent-cyan);
  color: #000;
  box-shadow: 0 0 14px rgba(6, 182, 212, 0.5);
}

.step-item.step-active .step-title {
  color: var(--accent-cyan);
}

.step-item.step-completed .step-circle {
  background: rgba(16, 185, 129, 0.2);
  border-color: var(--accent-emerald);
  color: var(--accent-emerald);
}

.step-item.step-completed .step-title {
  color: var(--text-primary);
}

.step-item.step-completed .step-line {
  background: rgba(16, 185, 129, 0.4);
}

/* Error Banner */
.error-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.35);
  border-radius: 12px;
  color: #fda4af;
  font-size: 13px;
}

.banner-close-btn {
  background: none;
  border: none;
  color: #fda4af;
  cursor: pointer;
  margin-left: auto;
  font-size: 14px;
}

/* Wizard Card */
.wizard-card {
  padding: 36px 32px;
  min-height: 400px;
}

.wizard-step-content {
  display: flex;
  flex-direction: column;
  gap: 28px;
}

.step-hero {
  text-align: center;
  max-width: 600px;
  margin: 0 auto;
}

.hero-icon-wrap {
  width: 64px;
  height: 64px;
  border-radius: 20px;
  background: rgba(6, 182, 212, 0.1);
  border: 1px solid rgba(6, 182, 212, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 16px;
  font-size: 32px;
}

.alert-icon-wrap {
  background: rgba(245, 158, 11, 0.15);
  border-color: rgba(245, 158, 11, 0.4);
}

.success-icon-wrap {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.4);
}

.hero-title {
  font-size: 20px;
  font-weight: 800;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.hero-text {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.6;
}

/* Info Cards Grid */
.info-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}

.info-card {
  padding: 18px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--border-subtle);
  border-radius: 14px;
  display: flex;
  gap: 14px;
}

.info-card-icon {
  font-size: 24px;
  flex-shrink: 0;
}

.info-card-body h3 {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 4px;
}

.info-card-body p {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
}

.app-links {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}

.store-tag {
  font-size: 10px;
  padding: 2px 8px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  color: var(--text-secondary);
  font-weight: 600;
}

/* QR Showcase */
.qr-showcase {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
}

.qr-box-wrapper {
  padding: 16px;
  background: #ffffff;
  border-radius: 18px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5), 0 0 20px rgba(6, 182, 212, 0.2);
}

.qr-img {
  width: 240px;
  height: 240px;
  display: block;
}

.qr-loading-placeholder {
  width: 240px;
  height: 240px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #000;
  font-size: 12px;
}

.manual-secret-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  max-width: 500px;
  text-align: center;
}

.manual-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.secret-display-pill {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid var(--border-medium);
  padding: 8px 14px;
  border-radius: 12px;
}

.secret-key {
  font-size: 15px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 2px;
}

.secret-hint {
  font-size: 11px;
  color: var(--text-muted);
}

/* Verify Form */
.verify-form {
  max-width: 380px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.totp-input-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.totp-input {
  width: 100%;
  text-align: center;
  font-family: var(--font-mono);
  font-size: 26px;
  font-weight: 700;
  letter-spacing: 10px;
  padding: 12px;
  color: var(--accent-cyan);
  border-color: rgba(6, 182, 212, 0.4);
}

.totp-input:focus {
  border-color: var(--accent-cyan);
  box-shadow: 0 0 20px rgba(6, 182, 212, 0.3);
}

.verify-hint {
  font-size: 11px;
  color: var(--text-muted);
}

/* Recovery Codes Grid */
.alert-box-warning {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 18px;
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.35);
  border-radius: 12px;
  color: #fcd34d;
  font-size: 13px;
}

.recovery-codes-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.recovery-code-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 18px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
}

.code-number {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 600;
}

.code-value {
  font-size: 15px;
  font-weight: 700;
  color: var(--accent-amber);
  letter-spacing: 2px;
}

.recovery-actions-bar {
  display: flex;
  gap: 12px;
  justify-content: center;
}

.acknowledgement-group {
  margin-top: 8px;
  padding: 14px 18px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px dashed var(--border-medium);
  border-radius: 12px;
}

.custom-checkbox-label {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
}

.custom-checkbox {
  width: 18px;
  height: 18px;
  accent-color: var(--accent-cyan);
  cursor: pointer;
}

.checkbox-text {
  font-size: 13px;
  color: var(--text-primary);
  font-weight: 500;
}

/* Success Card */
.success-features-card {
  max-width: 540px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 20px;
  background: rgba(16, 185, 129, 0.05);
  border: 1px solid rgba(16, 185, 129, 0.2);
  border-radius: 16px;
}

.feature-row {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.feature-icon {
  font-size: 20px;
  flex-shrink: 0;
  margin-top: 2px;
}

.feature-info h4 {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-primary);
}

.feature-info p {
  font-size: 12px;
  color: var(--text-muted);
}

/* Step Actions */
.step-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 12px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 20px;
}

.spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  display: inline-block;
  vertical-align: middle;
  margin-right: 6px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.step-slide-enter-active,
.step-slide-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.step-slide-enter-from {
  opacity: 0;
  transform: translateX(12px);
}

.step-slide-leave-to {
  opacity: 0;
  transform: translateX(-12px);
}

@media (max-width: 768px) {
  .stepper {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  .step-line {
    display: none;
  }
  .recovery-codes-grid {
    grid-template-columns: 1fr;
  }
}
</style>
