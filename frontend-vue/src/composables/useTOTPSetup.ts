import { ref, computed } from 'vue'
import QRCode from 'qrcode'
import { authApi } from '../api/auth'
import { useAuthStore } from '../stores/authStore'

export type WizardStep = 'intro' | 'scan' | 'verify' | 'recovery' | 'success'

export interface StepConfig {
  id: WizardStep
  title: string
  desc: string
}

export interface MFAEnforcementState {
  isEnforced: boolean
  isEnabled: boolean
  verifiedAt: string | null
  loading: boolean
}

export const WIZARD_STEPS: StepConfig[] = [
  { id: 'intro', title: '1. Overview', desc: 'Requirements' },
  { id: 'scan', title: '2. Scan QR', desc: 'Authenticator App' },
  { id: 'verify', title: '3. Verify', desc: '6-digit code' },
  { id: 'recovery', title: '4. Backup Codes', desc: 'Save offline' },
  { id: 'success', title: '5. Finished', desc: '2FA Active' },
]

export function useTOTPSetup() {
  const authStore = useAuthStore()

  // Navigation & Step tracking
  const steps = WIZARD_STEPS
  const currentStep = ref<WizardStep>('intro')
  const loading = ref(false)
  const errorMessage = ref('')

  // Setup Keys & QR Code
  const secretKey = ref('')
  const qrUri = ref('')
  const qrImageSrc = ref('')
  const secretCopied = ref(false)

  // Verification & Recovery
  const verificationCode = ref('')
  const recoveryCodes = ref<string[]>([])
  const hasSavedCodes = ref(false)
  const allCodesCopied = ref(false)

  // MFA Enforcement State
  const mfaEnforcement = ref<MFAEnforcementState>({
    isEnforced: false,
    isEnabled: false,
    verifiedAt: null,
    loading: false,
  })

  const currentStepIndex = computed(() => steps.findIndex(s => s.id === currentStep.value))

  // Generates cryptographically secure emergency recovery scratch tokens (8 tokens)
  function generateEmergencyRecoveryCodes(count = 8): string[] {
    const charset = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'
    const generated: string[] = []
    const randomBuffer = new Uint8Array(count * 8)

    if (typeof window !== 'undefined' && window.crypto) {
      window.crypto.getRandomValues(randomBuffer)
    } else {
      crypto.getRandomValues(randomBuffer)
    }

    for (let i = 0; i < count; i++) {
      let code = ''
      for (let j = 0; j < 8; j++) {
        code += charset[randomBuffer[i * 8 + j] % charset.length]
      }
      generated.push(`${code.slice(0, 4)}-${code.slice(4)}`)
    }
    return generated
  }

  // Check MFA enforcement and status from cluster API
  async function checkMFAStatus(): Promise<void> {
    mfaEnforcement.value.loading = true
    try {
      const res = await authApi.getTOTPStatus()
      mfaEnforcement.value.isEnabled = !!res.enabled
      mfaEnforcement.value.verifiedAt = res.verified_at
      mfaEnforcement.value.isEnforced = !!authStore.user?.mfa_enabled || !!res.enabled
    } catch {
      mfaEnforcement.value.isEnabled = !!authStore.user?.mfa_enabled
      mfaEnforcement.value.isEnforced = !!authStore.user?.mfa_enabled
    } finally {
      mfaEnforcement.value.loading = false
    }
  }

  // Initiate TOTP setup: generates secret, URI, and renders QR code
  async function startSetup(): Promise<void> {
    loading.value = true
    errorMessage.value = ''
    try {
      const res = await authApi.setupTOTP()
      secretKey.value = res.secret
      qrUri.value = res.qr_uri

      try {
        qrImageSrc.value = await QRCode.toDataURL(res.qr_uri, {
          width: 260,
          margin: 2,
          color: { dark: '#000000', light: '#ffffff' },
        })
      } catch {
        qrImageSrc.value = ''
        errorMessage.value = 'Failed to generate QR code locally. Please manually enter the Base32 secret key into your authenticator app.'
      }
      currentStep.value = 'scan'
    } catch (err: unknown) {
      errorMessage.value = err instanceof Error ? err.message : 'Failed to initialize TOTP setup. Please try again.'
    } finally {
      loading.value = false
    }
  }

  // Copy the Base32 secret string to user clipboard
  async function copySecretToClipboard(): Promise<void> {
    if (!secretKey.value) return
    try {
      await navigator.clipboard.writeText(secretKey.value)
      secretCopied.value = true
      setTimeout(() => { secretCopied.value = false }, 2500)
    } catch {
      errorMessage.value = 'Clipboard access denied. Please copy the code manually.'
    }
  }

  // Navigate to verification step
  function goToVerify(): void {
    verificationCode.value = ''
    currentStep.value = 'verify'
    errorMessage.value = ''
  }

  // Sanitizes 6-digit numeric input and triggers auto-submission when complete
  function handleVerificationInput(value: string): void {
    verificationCode.value = value.replace(/\D/g, '').slice(0, 6)
    if (verificationCode.value.length === 6) {
      verifyAndEnable()
    }
  }

  // Verify TOTP code and enable MFA on server
  async function verifyAndEnable(): Promise<void> {
    if (verificationCode.value.length !== 6) return
    loading.value = true
    errorMessage.value = ''

    try {
      const res = await authApi.verifyTOTPSetup(verificationCode.value)
      if (res.recovery_codes && res.recovery_codes.length > 0) {
        recoveryCodes.value = res.recovery_codes
      } else {
        recoveryCodes.value = []
        errorMessage.value = 'Warning: Recovery codes could not be provisioned by the server.'
      }
      currentStep.value = 'recovery'
      hasSavedCodes.value = false
      mfaEnforcement.value.isEnabled = true
      mfaEnforcement.value.isEnforced = true
    } catch (err: unknown) {
      errorMessage.value = err instanceof Error
        ? err.message
        : 'Verification failed. Please check that your authenticator code is accurate and device time is in sync.'
    } finally {
      loading.value = false
    }
  }

  // Copy all emergency recovery codes to clipboard
  async function copyAllCodes(): Promise<void> {
    if (!recoveryCodes.value.length) return
    try {
      await navigator.clipboard.writeText(recoveryCodes.value.join('\n'))
      allCodesCopied.value = true
      setTimeout(() => { allCodesCopied.value = false }, 2500)
    } catch {
      errorMessage.value = 'Failed to copy recovery codes to clipboard.'
    }
  }

  // Downloads recovery codes as an offline plaintext backup document
  function downloadCodesAsFile(): void {
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
      'EMERGENCY RECOVERY TOKENS:',
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

  // Print recovery codes via browser print dialog
  function printRecoveryCodes(): void {
    if (typeof window !== 'undefined') window.print()
  }

  // Reset setup flow
  function resetSetup(): void {
    currentStep.value = 'intro'
    loading.value = false
    errorMessage.value = ''
    secretKey.value = ''
    qrUri.value = ''
    qrImageSrc.value = ''
    secretCopied.value = false
    verificationCode.value = ''
    recoveryCodes.value = []
    hasSavedCodes.value = false
    allCodesCopied.value = false
  }

  return {
    steps,
    currentStep,
    currentStepIndex,
    loading,
    errorMessage,
    secretKey,
    qrUri,
    qrImageSrc,
    secretCopied,
    verificationCode,
    recoveryCodes,
    hasSavedCodes,
    allCodesCopied,
    mfaEnforcement,
    checkMFAStatus,
    startSetup,
    copySecretToClipboard,
    goToVerify,
    handleVerificationInput,
    verifyAndEnable,
    copyAllCodes,
    downloadCodesAsFile,
    printRecoveryCodes,
    generateEmergencyRecoveryCodes,
    resetSetup,
  }
}
