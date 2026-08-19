import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api/client'
import { authApi, type UserInfo as AuthUserInfo } from '../api/auth'

export type UserInfo = AuthUserInfo

export interface LoginResult {
  mfaRequired: boolean
  user?: UserInfo | null
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(typeof window !== 'undefined' ? localStorage.getItem('k8s_token') : null)
  
  const savedUser = typeof window !== 'undefined' ? localStorage.getItem('k8s_user') : null
  let initialUser: UserInfo | null = null
  if (savedUser) {
    try {
      initialUser = JSON.parse(savedUser)
    } catch {
      initialUser = null
      if (typeof window !== 'undefined') {
        localStorage.removeItem('k8s_user')
      }
    }
  }
  const user = ref<UserInfo | null>(initialUser)
  const mfaRequired = ref(false)
  const partialToken = ref('')
  const refreshTimer = ref<number | null>(null)
  
  const loading = ref(false)
  const error = ref<string | null>(null)

  const isAuthenticated = computed(() => !!token.value)

  function setAuth(newToken: string, userInfo: UserInfo) {
    token.value = newToken
    user.value = userInfo
    api.setToken(newToken)
    if (userInfo.tenant_id) {
      api.setTenantId(userInfo.tenant_id)
    }
    if (typeof window !== 'undefined') {
      localStorage.setItem('k8s_token', newToken)
      localStorage.setItem('k8s_user', JSON.stringify(userInfo))
    }
    startRefreshTimer()
  }

  function startRefreshTimer() {
    if (refreshTimer.value) {
      clearInterval(refreshTimer.value)
      refreshTimer.value = null
    }
    if (typeof window === 'undefined') return

    // Auto-refresh token every 13 minutes (access token lifetime is 15 minutes)
    refreshTimer.value = window.setInterval(async () => {
      try {
        const res = await authApi.refresh()
        if (res.token) {
          token.value = res.token
          api.setToken(res.token)
          localStorage.setItem('k8s_token', res.token)
        }
      } catch {
        logout()
      }
    }, 13 * 60 * 1000)
  }

  if (token.value) {
    api.setToken(token.value)
    if (user.value?.tenant_id) {
      api.setTenantId(user.value.tenant_id)
    }
    startRefreshTimer()
  }

  async function login(email: string, password: string): Promise<LoginResult> {
    loading.value = true
    error.value = null
    try {
      const res = await authApi.login({ email, password })
      if (res.mfa_required) {
        mfaRequired.value = true
        partialToken.value = res.partial_token || ''
        return { mfaRequired: true }
      }

      if (res.token && res.user) {
        const fullUser: UserInfo = {
          ...res.user,
          email,
        }
        setAuth(res.token, fullUser)
        return { mfaRequired: false, user: fullUser }
      }

      throw new Error('Invalid login response from server')
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Login failed'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function verifyMFA(code: string): Promise<UserInfo> {
    loading.value = true
    error.value = null
    try {
      const res = await authApi.verifyMFA({
        partial_token: partialToken.value,
        code,
      })
      if (!res.token || !res.user) {
        throw new Error('MFA verification did not return valid session token.')
      }
      mfaRequired.value = false
      const fullUser: UserInfo = {
        ...res.user,
        email: user.value?.email || undefined,
      }
      setAuth(res.token, fullUser)
      partialToken.value = ''
      return fullUser
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'MFA verification failed'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function verifyRecovery(code: string): Promise<UserInfo> {
    loading.value = true
    error.value = null
    try {
      const res = await authApi.verifyRecovery({
        partial_token: partialToken.value,
        code,
      })
      if (!res.token || !res.user) {
        throw new Error('Recovery code verification did not return valid session token.')
      }
      mfaRequired.value = false
      const fullUser: UserInfo = {
        ...res.user,
        email: user.value?.email || undefined,
      }
      setAuth(res.token, fullUser)
      partialToken.value = ''
      return fullUser
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Recovery code verification failed'
      throw err
    } finally {
      loading.value = false
    }
  }

  function cancelMFA() {
    mfaRequired.value = false
    partialToken.value = ''
    error.value = null
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch {
      // Ignore network errors on logout
    }
    if (refreshTimer.value) {
      clearInterval(refreshTimer.value)
      refreshTimer.value = null
    }
    token.value = null
    user.value = null
    mfaRequired.value = false
    partialToken.value = ''
    api.clearToken()
    if (typeof window !== 'undefined') {
      localStorage.removeItem('k8s_user')
      localStorage.removeItem('k8s_token')
    }
  }

  return {
    token,
    user,
    loading,
    error,
    mfaRequired,
    partialToken,
    isAuthenticated,
    login,
    verifyMFA,
    verifyRecovery,
    cancelMFA,
    setAuth,
    startRefreshTimer,
    logout,
  }
})

