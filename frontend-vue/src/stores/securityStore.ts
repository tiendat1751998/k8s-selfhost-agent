import { defineStore } from 'pinia'
import { ref } from 'vue'
import { securityApi, type ComplianceFramework, type SecurityViolation } from '../api/security'

export const useSecurityStore = defineStore('security', () => {
  const frameworks = ref<ComplianceFramework[]>([])
  const violations = ref<SecurityViolation[]>([])
  const totalViolations = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchAll(severity?: string) {
    loading.value = true
    error.value = null
    try {
      const [f, v] = await Promise.all([
        securityApi.getFrameworks(),
        securityApi.getViolations(severity),
      ])
      frameworks.value = f
      violations.value = v.items
      totalViolations.value = v.total
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch security data'
    } finally {
      loading.value = false
    }
  }

  return {
    frameworks,
    violations,
    totalViolations,
    loading,
    error,
    fetchAll,
  }
})
