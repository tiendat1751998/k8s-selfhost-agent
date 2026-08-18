import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api/client'

export interface UserInfo {
  id: string
  role: string
  tenant_id: string
  email?: string
}

interface LoginResponse {
  token: string
  user: {
    id: string
    role: string
    tenant_id: string
  }
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

  if (token.value) {
    api.setToken(token.value)
  }
  if (user.value?.tenant_id) {
    api.setTenantId(user.value.tenant_id)
  }
  
  const loading = ref(false)
  const error = ref<string | null>(null)

  const isAuthenticated = computed(() => !!token.value)

  async function login(email: string, password: string): Promise<UserInfo> {
    loading.value = true
    error.value = null
    try {
      const res = await api.post<LoginResponse>('/auth/login', { email, password })
      token.value = res.token
      user.value = {
        ...res.user,
        email,
      }
      api.setToken(res.token)
      if (res.user.tenant_id) {
        api.setTenantId(res.user.tenant_id)
      }
      if (typeof window !== 'undefined') {
        localStorage.setItem('k8s_user', JSON.stringify(user.value))
      }
      return user.value
    } catch (err: any) {
      error.value = err.message || 'Login failed'
      throw err
    } finally {
      loading.value = false
    }
  }

  function logout() {
    token.value = null
    user.value = null
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
    isAuthenticated,
    login,
    logout,
  }
})
