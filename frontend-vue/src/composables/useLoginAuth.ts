import { ref, computed, nextTick, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/authStore'
import { api } from '../api/client'

export type LoginStep = 'credentials' | 'totp' | 'recovery'

export interface UseLoginAuthOptions {
  defaultRedirect?: string
}

export function useLoginAuth(options: UseLoginAuthOptions = {}) {
  const router = useRouter()
  const route = useRoute()
  const authStore = useAuthStore()

  // 1. Form and navigation state
  const step = ref<LoginStep>('credentials')
  const email = ref('')
  const password = ref('')
  const totpCode = ref('')
  const recoveryCode = ref('')
  const tenantId = ref('default')
  const rememberMe = ref(false)
  const errorMessage = ref('')

  // Template refs for input focus
  const totpInputRef = ref<HTMLInputElement | null>(null)
  const recoveryInputRef = ref<HTMLInputElement | null>(null)

  // 2. Tenant Resolution
  function resolveTenant(): string {
    const queryTenant = (route.query.tenant || route.query.tenant_id) as string | undefined
    if (queryTenant && queryTenant.trim()) {
      return queryTenant.trim()
    }
    if (typeof window !== 'undefined') {
      const savedTenant = localStorage.getItem('k8s_tenant_id')
      if (savedTenant && savedTenant.trim()) {
        return savedTenant.trim()
      }
      const host = window.location.hostname
      const parts = host.split('.')
      if (parts.length > 2 && parts[0] !== 'www') {
        return parts[0]
      }
    }
    return 'default'
  }

  function setTenant(newTenant: string): void {
    tenantId.value = newTenant
    api.setTenantId(newTenant)
    if (typeof window !== 'undefined') {
      localStorage.setItem('k8s_tenant_id', newTenant)
    }
  }

  // 3. Remembered Credentials & Session Initialization
  function initRememberedCredentials(): void {
    if (typeof window === 'undefined') return
    const rememberedEmail = localStorage.getItem('k8s_remembered_email')
    if (rememberedEmail) {
      email.value = rememberedEmail
      rememberMe.value = true
    }
  }

  function persistRememberedCredentials(): void {
    if (typeof window === 'undefined') return
    if (rememberMe.value && email.value) {
      localStorage.setItem('k8s_remembered_email', email.value.trim())
    } else {
      localStorage.removeItem('k8s_remembered_email')
    }
  }

  // 4. Session Persistence & Redirect Path
  function getRedirectPath(): string {
    const redirect = route.query.redirect as string | undefined
    if (redirect && redirect.startsWith('/') && !redirect.startsWith('/login')) {
      return redirect
    }
    return options.defaultRedirect || '/'
  }

  // 5. Error formatting
  function formatAuthError(err: unknown, fallback: string): string {
    if (err instanceof Error) {
      const msg = err.message
      if (msg.includes('Failed to fetch') || msg.includes('NetworkError')) {
        return 'Network connection failed. Verify cluster connectivity and TLS configuration.'
      }
      if (msg.toLowerCase().includes('rate limit') || msg.includes('429')) {
        return 'Too many login attempts. Please wait 60 seconds before retrying.'
      }
      return msg
    }
    return fallback
  }

  function clearError(): void {
    errorMessage.value = ''
  }

  // 6. Action Handlers
  async function handleCredentialsSubmit(): Promise<void> {
    if (!email.value.trim() || !password.value) return
    clearError()

    try {
      persistRememberedCredentials()
      api.setTenantId(tenantId.value)
      const res = await authStore.login(email.value.trim(), password.value)

      if (res.mfaRequired) {
        step.value = 'totp'
        totpCode.value = ''
        await nextTick()
        totpInputRef.value?.focus()
      } else {
        await router.push(getRedirectPath())
      }
    } catch (err: unknown) {
      errorMessage.value = formatAuthError(
        err,
        'Authentication failed. Please verify your credentials.'
      )
    }
  }

  async function handleTotpSubmit(): Promise<void> {
    if (totpCode.value.length !== 6) return
    clearError()

    try {
      await authStore.verifyMFA(totpCode.value)
      await router.push(getRedirectPath())
    } catch (err: unknown) {
      errorMessage.value = formatAuthError(
        err,
        'Invalid 2FA code. Please verify your authenticator time synchronization.'
      )
    }
  }

  function onTotpInput(e: Event | string): void {
    const val = typeof e === 'string' ? e : ((e.target as HTMLInputElement)?.value || '')
    totpCode.value = val.replace(/\D/g, '').slice(0, 6)
    if (totpCode.value.length === 6) {
      handleTotpSubmit()
    }
  }

  async function handleRecoverySubmit(): Promise<void> {
    const code = recoveryCode.value.trim()
    if (!code) return
    clearError()

    try {
      await authStore.verifyRecovery(code)
      await router.push(getRedirectPath())
    } catch (err: unknown) {
      errorMessage.value = formatAuthError(
        err,
        'Invalid recovery code. Please check your emergency backup list.'
      )
    }
  }

  async function switchToRecovery(): Promise<void> {
    clearError()
    step.value = 'recovery'
    recoveryCode.value = ''
    await nextTick()
    recoveryInputRef.value?.focus()
  }

  async function switchToTotp(): Promise<void> {
    clearError()
    step.value = 'totp'
    totpCode.value = ''
    await nextTick()
    totpInputRef.value?.focus()
  }

  function backToCredentials(): void {
    clearError()
    step.value = 'credentials'
    authStore.cancelMFA()
  }

  function resetForm(): void {
    step.value = 'credentials'
    password.value = ''
    totpCode.value = ''
    recoveryCode.value = ''
    clearError()
    authStore.cancelMFA()
  }

  function handleSsoLogin(provider: string): void {
    clearError()
    if (typeof window !== 'undefined') {
      const redirect = getRedirectPath()
      const baseUrl = import.meta.env.VITE_API_BASE_URL || '/api/v1'
      const target = baseUrl + "/auth/sso/" + provider + "?redirect=" + encodeURIComponent(redirect) + "&tenant=" + encodeURIComponent(tenantId.value)
      window.location.href = target
    }
  }

  // 7. Computed helpers
  const isLoading = computed(() => authStore.loading)
  const canSubmitCredentials = computed(() => !isLoading.value && !!email.value.trim() && !!password.value)
  const canSubmitTotp = computed(() => !isLoading.value && totpCode.value.length === 6)
  const canSubmitRecovery = computed(() => !isLoading.value && !!recoveryCode.value.trim())

  onMounted(() => {
    tenantId.value = resolveTenant()
    api.setTenantId(tenantId.value)
    initRememberedCredentials()
  })

  return {
    step,
    email,
    password,
    totpCode,
    recoveryCode,
    tenantId,
    rememberMe,
    errorMessage,
    totpInputRef,
    recoveryInputRef,
    isLoading,
    canSubmitCredentials,
    canSubmitTotp,
    canSubmitRecovery,
    resolveTenant,
    setTenant,
    clearError,
    handleCredentialsSubmit,
    handleTotpSubmit,
    handleSsoLogin,
    onTotpInput,
    handleRecoverySubmit,
    switchToRecovery,
    switchToTotp,
    backToCredentials,
    resetForm,
    getRedirectPath,
  }
}
