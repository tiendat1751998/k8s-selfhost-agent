export interface PasswordValidationResult {
  valid: boolean
  error: string | null
}

export function validatePasswordChange(
  currentPassword?: string,
  newPassword?: string,
  confirmPassword?: string
): PasswordValidationResult {
  if (!currentPassword || !currentPassword.trim()) {
    return { valid: false, error: 'Current password is required' }
  }
  if (!newPassword || newPassword.length < 10) {
    return { valid: false, error: 'New password must be at least 10 characters in length' }
  }
  if (newPassword !== confirmPassword) {
    return { valid: false, error: 'New password and confirmation do not match' }
  }
  if (newPassword === currentPassword) {
    return { valid: false, error: 'New password cannot be identical to current password' }
  }
  return { valid: true, error: null }
}
