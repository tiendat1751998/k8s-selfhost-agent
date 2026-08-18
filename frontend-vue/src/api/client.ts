// API Client with JWT Bearer Auth and Tenant Isolation

export interface ApiResponse<T> {
  data: T
  total?: number
  error?: string
}

class ApiClient {
  private baseUrl: string
  private token: string | null = null
  private tenantId: string = 'default'

  constructor() {
    this.baseUrl = import.meta.env.VITE_API_BASE_URL || '/api/v1'
    this.token = typeof window !== 'undefined' ? localStorage.getItem('k8s_token') : null
  }

  setToken(token: string | null): void {
    this.token = token
    if (typeof window !== 'undefined') {
      if (token) {
        localStorage.setItem('k8s_token', token)
      } else {
        localStorage.removeItem('k8s_token')
      }
    }
  }

  clearToken(): void {
    this.setToken(null)
    if (typeof window !== 'undefined') {
      localStorage.removeItem('k8s_user')
    }
  }

  getToken(): string | null {
    return this.token
  }

  setTenantId(tenantId: string): void {
    this.tenantId = tenantId
  }

  async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.baseUrl}${endpoint.startsWith('/') ? endpoint : `/${endpoint}`}`
    
    const headers: Record<string, string> = {
      'X-Tenant-ID': this.tenantId,
      ...(options.headers as Record<string, string> || {}),
    }

    if (!(options.body instanceof FormData) && !headers['Content-Type']) {
      headers['Content-Type'] = 'application/json'
    }

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    const response = await fetch(url, {
      ...options,
      headers,
    })

    if (response.status === 401) {
      this.clearToken()
      if (typeof window !== 'undefined') {
        localStorage.removeItem('k8s_token')
        localStorage.removeItem('k8s_user')
        if (!window.location.pathname.includes('/login')) {
          const currentPath = window.location.pathname + window.location.search
          window.location.href = `/login?redirect=${encodeURIComponent(currentPath)}`
        }
      }
      throw new Error('Unauthorized: Session expired or invalid credentials.')
    }

    if (!response.ok) {
      const errorBody = await response.text()
      throw new Error(`API Error ${response.status} (${response.statusText}): ${errorBody}`)
    }

    if (response.status === 204) {
      return {} as T
    }

    return response.json()
  }

  get<T>(endpoint: string, params?: Record<string, string | number>): Promise<T> {
    let query = ''
    if (params) {
      const sp = new URLSearchParams()
      for (const [k, v] of Object.entries(params)) {
        if (v !== undefined && v !== null && v !== '') {
          sp.append(k, String(v))
        }
      }
      query = `?${sp.toString()}`
    }
    return this.request<T>(`${endpoint}${query}`, { method: 'GET' })
  }

  post<T>(endpoint: string, body?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: body instanceof FormData ? body : (body ? JSON.stringify(body) : undefined),
    })
  }

  put<T>(endpoint: string, body?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PUT',
      body: body instanceof FormData ? body : (body ? JSON.stringify(body) : undefined),
    })
  }

  delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' })
  }
}

export const api = new ApiClient()
