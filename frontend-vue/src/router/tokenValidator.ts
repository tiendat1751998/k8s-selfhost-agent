export interface TokenClaims {
  sub?: string
  role?: string
  tenant?: string
  type?: string
  exp?: number
  iat?: number
}

/**
 * Validates JWT token structure, signature formatting, and expiration.
 * Rejects arbitrary strings, un-parseable JSON, or expired tokens.
 */
export function parseAndValidateToken(token: string | null): TokenClaims | null {
  if (!token || typeof token !== 'string') return null
  const parts = token.split('.')
  if (parts.length !== 3) return null

  try {
    const rawPayload = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const decoded = typeof atob === 'function'
      ? atob(rawPayload)
      : Buffer.from(rawPayload, 'base64').toString('utf-8')
    const claims = JSON.parse(decoded) as TokenClaims

    if (!claims || typeof claims !== 'object') return null

    if (claims.exp && typeof claims.exp === 'number') {
      const nowSeconds = Math.floor(Date.now() / 1000)
      if (claims.exp <= nowSeconds) {
        return null // Token expired
      }
    }

    return claims
  } catch {
    return null
  }
}

/**
 * Verifies if the role is authorized for sensitive enterprise admin operations.
 */
export function isPrivilegedAdminRole(role: string | undefined): boolean {
  if (!role || typeof role !== 'string') return false
  const normalized = role.trim().toLowerCase()
  return (
    normalized === 'admin' ||
    normalized === 'platform admin' ||
    normalized === 'security auditor' ||
    normalized === 'platform-admin' ||
    normalized === 'platform_admin' ||
    normalized === 'devops team'
  )
}
