import { api } from './client'

export interface LoginRequest {
  email: string
  password: string
}

export interface UserInfo {
  id: string
  email?: string
  role: string
  tenant_id: string
  mfa_enabled?: boolean
}

export interface LoginResponse {
  token?: string
  user?: UserInfo
  mfa_required?: boolean
  partial_token?: string
}

export interface VerifyMFARequest {
  partial_token: string
  code: string
}

export interface VerifyRecoveryRequest {
  partial_token: string
  code: string
}

export interface TOTPSetupResponse {
  secret: string
  qr_uri: string
}

export interface TOTPVerifyResponse {
  recovery_codes: string[]
  message?: string
}

export interface TOTPStatusResponse {
  enabled: boolean
  verified_at: string | null
}

export const authApi = {
  login: (data: LoginRequest) => api.post<LoginResponse>('/auth/login', data),
  verifyMFA: (data: VerifyMFARequest) => api.post<LoginResponse>('/auth/verify-mfa', data),
  verifyRecovery: (data: VerifyRecoveryRequest) => api.post<LoginResponse>('/auth/recovery/verify', data),
  refresh: () => api.post<{ token: string }>('/auth/refresh'),
  logout: () => api.post('/auth/logout'),
  setupTOTP: () => api.post<TOTPSetupResponse>('/auth/totp/setup'),
  verifyTOTPSetup: (code: string) => api.post<TOTPVerifyResponse>('/auth/totp/verify-setup', { code }),
  getTOTPStatus: () => api.get<TOTPStatusResponse>('/auth/totp/status'),
  disableTOTP: (password: string, code: string) => api.post<{ message: string }>('/auth/totp/disable', { password, code }),
}
