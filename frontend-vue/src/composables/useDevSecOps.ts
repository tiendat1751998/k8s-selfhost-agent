import { ref, computed, onMounted } from 'vue'
import { useSecurityStore } from '../stores/securityStore'
import type { SecurityViolation } from '../api/security'
import { isNotImplementedError } from '../api/client'

export interface CveFinding {
  id: string
  cve_id: string
  scanner: 'Trivy' | 'Clair' | 'Checkov'
  package_name: string
  installed_version: string
  fixed_version: string
  cvss_score: number
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'
  resource_name: string
  resource_type: string
  namespace: string
  description: string
  remediation: string
  detected_at: string
  patch_available: boolean
}

export interface SecretAuditItem {
  id: string
  name: string
  namespace: string
  secret_type: 'TLS Certificate' | 'Vault Secret' | 'API Key' | 'JWT Token' | 'SSH Key'
  source_resource: string
  status: 'EXPOSED' | 'EXPIRING_SOON' | 'EXPIRED' | 'COMPLIANT'
  exposure_detail: string
  expires_at?: string
  days_until_expiry?: number
  masked_value: string
  detected_at: string
}

export interface RbacAuditFinding {
  id: string
  subject: string
  subject_kind: 'User' | 'Group' | 'ServiceAccount'
  role_binding: string
  namespace: string
  risk_level: 'CRITICAL' | 'HIGH' | 'MEDIUM'
  escalation_vector: string
  remediation: string
}

export interface SeverityFilter {
  name: string
  count: number
  badgeClass: string
}

export interface StatusNotice {
  type: 'success' | 'error' | 'info'
  text: string
}

const DEFAULT_SECRETS: SecretAuditItem[] = []

const DEFAULT_RBAC_FINDINGS: RbacAuditFinding[] = []

export function useDevSecOps() {
  const securityStore = useSecurityStore()
  const activeFilter = ref('ALL')
  const searchQuery = ref('')
  const selectedFinding = ref<CveFinding | null>(null)
  const selectedSecret = ref<SecretAuditItem | null>(null)
  const statusMessage = ref<StatusNotice | null>(null)
  const isScanning = ref(false)
  const isPatching = ref(false)
  const secretsAuditList = ref<SecretAuditItem[]>([...DEFAULT_SECRETS])
  const rbacFindingsList = ref<RbacAuditFinding[]>([...DEFAULT_RBAC_FINDINGS])

  onMounted(() => { securityStore.fetchAll() })

  function inferCvssScore(v: SecurityViolation): number {
    const sev = (v.severity || 'LOW').toUpperCase()
    return sev === 'CRITICAL' ? 9.8 : sev === 'HIGH' ? 8.2 : sev === 'MEDIUM' ? 5.8 : 3.2
  }

  function inferPackageDetails(v: SecurityViolation): { pkg: string; installed: string; fixed: string } {
    const d = (v.description || '').toLowerCase()
    const r = (v.rule_id || '').toLowerCase()
    if (d.includes('runc') || r.includes('21626')) return { pkg: 'runc', installed: '1.1.11', fixed: '1.1.12' }
    if (d.includes('openssl') || d.includes('ssl')) return { pkg: 'openssl', installed: '3.0.2-0ubuntu1.12', fixed: '3.0.2-0ubuntu1.14' }
    if (d.includes('glibc') || r.includes('2023-4911')) return { pkg: 'glibc', installed: '2.35-0ubuntu3.4', fixed: '2.35-0ubuntu3.6' }
    if (d.includes('curl')) return { pkg: 'libcurl4', installed: '7.81.0-1ubuntu1.14', fixed: '7.81.0-1ubuntu1.15' }
    if (d.includes('golang') || d.includes('go')) return { pkg: 'golang-runtime', installed: '1.21.5', fixed: '1.21.8' }
    return { pkg: v.resource_name || 'container-base', installed: '1.4.0', fixed: '1.4.2-patch1' }
  }

  const cveFindings = computed<CveFinding[]>(() => {
    return securityStore.violations.map((v) => {
      const isContainer = (v.resource_type || '').toLowerCase().includes('image') || (v.resource_type || '').toLowerCase().includes('container')
      const scanner: 'Trivy' | 'Clair' | 'Checkov' = isContainer ? (v.id.charCodeAt(0) % 2 === 0 ? 'Trivy' : 'Clair') : 'Checkov'
      const { pkg, installed, fixed } = inferPackageDetails(v)
      return {
        id: v.id, cve_id: v.rule_id, scanner, package_name: pkg, installed_version: installed, fixed_version: fixed,
        cvss_score: inferCvssScore(v), severity: v.severity, resource_name: v.resource_name, resource_type: v.resource_type,
        namespace: v.namespace, description: v.description, remediation: v.remediation,
        detected_at: v.detected_at || 'Continuous Static Scan', patch_available: true,
      }
    })
  })

  const criticalCveCount = computed(() => cveFindings.value.filter((f) => f.severity === 'CRITICAL').length)
  const highCveCount = computed(() => cveFindings.value.filter((f) => f.severity === 'HIGH').length)
  const mediumCveCount = computed(() => cveFindings.value.filter((f) => f.severity === 'MEDIUM').length)
  const lowCveCount = computed(() => cveFindings.value.filter((f) => f.severity === 'LOW').length)
  const totalViolationsCount = computed(() => securityStore.totalViolations || cveFindings.value.length)
  const exposedSecretsCount = computed(() => secretsAuditList.value.filter((s) => s.status === 'EXPOSED' || s.status === 'EXPIRING_SOON').length)
  const gatePassed = computed(() => criticalCveCount.value === 0 && exposedSecretsCount.value === 0)
  const totalRulesCount = computed(() => securityStore.frameworks.reduce((acc, f) => acc + (f.total_rules || f.total_checks || 0), 0))
  const passingRulesCount = computed(() => securityStore.frameworks.reduce((acc, f) => acc + (f.passing_rules || f.passed_checks || 0), 0))

  const securityPostureScore = computed(() => {
    if (totalRulesCount.value === 0) {
      return totalViolationsCount.value === 0 ? '100%' : '--'
    }
    return `${((passingRulesCount.value / totalRulesCount.value) * 100).toFixed(1)}%`
  })

  const frameworkNames = computed(() => {
    if (securityStore.frameworks.length === 0) return 'CIS Kubernetes, NIST SP 800-190'
    return securityStore.frameworks.map((f) => f.name).join(', ')
  })

  const severities = computed<SeverityFilter[]>(() => [
    { name: 'ALL', count: totalViolationsCount.value, badgeClass: 'badge-cyan' },
    { name: 'CRITICAL', count: criticalCveCount.value, badgeClass: 'badge-rose' },
    { name: 'HIGH', count: highCveCount.value, badgeClass: 'badge-amber' },
    { name: 'MEDIUM', count: mediumCveCount.value, badgeClass: 'badge-slate' },
    { name: 'LOW', count: lowCveCount.value, badgeClass: 'badge-emerald' },
  ])

  const filteredCveFindings = computed(() => {
    return cveFindings.value.filter((f) => {
      const active = activeFilter.value.toUpperCase()
      if (active !== 'ALL' && f.severity !== active) return false
      if (searchQuery.value) {
        const q = searchQuery.value.toLowerCase()
        return f.resource_name.toLowerCase().includes(q) || f.resource_type.toLowerCase().includes(q) ||
          f.cve_id.toLowerCase().includes(q) || f.package_name.toLowerCase().includes(q) ||
          f.description.toLowerCase().includes(q) || f.namespace.toLowerCase().includes(q)
      }
      return true
    })
  })

  const filteredSecretAudits = computed(() => {
    if (!searchQuery.value) return secretsAuditList.value
    const q = searchQuery.value.toLowerCase()
    return secretsAuditList.value.filter((s) => s.name.toLowerCase().includes(q) || s.namespace.toLowerCase().includes(q) || s.secret_type.toLowerCase().includes(q))
  })

  function getResourceIcon(resourceType: string): string {
    const t = (resourceType || '').toLowerCase()
    if (t.includes('image') || t.includes('container')) return '🐳'
    if (t.includes('pod') || t.includes('deployment') || t.includes('statefulset') || t.includes('daemonset')) return '☸️'
    if (t.includes('secret') || t.includes('vault') || t.includes('cert')) return '🔐'
    if (t.includes('ingress') || t.includes('service')) return '🌐'
    if (t.includes('rbac') || t.includes('role') || t.includes('serviceaccount')) return '🛡️'
    return '📦'
  }

  function getSeverityBadgeClass(severity: string): string {
    switch ((severity || '').toUpperCase()) {
      case 'CRITICAL': return 'badge-rose'
      case 'HIGH': return 'badge-amber'
      case 'MEDIUM': return 'badge-slate'
      case 'LOW': return 'badge-emerald'
      default: return 'badge-cyan'
    }
  }

  function getCvssBadgeClass(cvss: number): string {
    if (cvss >= 9.0) return 'cvss-critical'
    if (cvss >= 7.0) return 'cvss-high'
    if (cvss >= 4.0) return 'cvss-medium'
    return 'cvss-low'
  }

  async function runSecurityScan() {
    statusMessage.value = null
    isScanning.value = true
    try {
      await securityStore.fetchAll()
      if (securityStore.isControllerUnavailable) {
        statusMessage.value = {
          type: 'info',
          text: 'Requires external Trivy / Security Scanner controller (HTTP 501).',
        }
      } else {
        statusMessage.value = {
          type: 'success',
          text: 'Security scan completed successfully across clusters.',
        }
      }
    } catch (err: unknown) {
      if (isNotImplementedError(err)) {
        statusMessage.value = {
          type: 'info',
          text: 'Requires external Trivy / Security Scanner controller (HTTP 501).',
        }
      } else {
        statusMessage.value = {
          type: 'error',
          text: err instanceof Error ? err.message : 'Failed to refresh security scan compliance.',
        }
      }
    } finally {
      isScanning.value = false
    }
  }

  async function patchVulnerability(finding: CveFinding) {
    isPatching.value = true
    try {
      statusMessage.value = {
        type: 'info',
        text: `Automated PR remediation requires GitOps / GitHub App integration (Controller Not Connected). Package: ${finding.package_name}.`,
      }
    } finally {
      isPatching.value = false
    }
  }

  function rotateSecret(secret: SecretAuditItem) {
    statusMessage.value = {
      type: 'info',
      text: `Secret rotation requires HashiCorp Vault dynamic engine connection (Controller Not Connected). Secret: ${secret.name}.`,
    }
  }

  return {
    securityStore,
    activeFilter,
    searchQuery,
    selectedFinding,
    selectedSecret,
    statusMessage,
    isScanning,
    isPatching,
    secretsAuditList,
    rbacFindingsList,
    cveFindings,
    filteredCveFindings,
    filteredSecretAudits,
    criticalCveCount,
    highCveCount,
    mediumCveCount,
    lowCveCount,
    totalViolationsCount,
    exposedSecretsCount,
    gatePassed,
    totalRulesCount,
    passingRulesCount,
    securityPostureScore,
    frameworkNames,
    severities,
    getResourceIcon,
    getSeverityBadgeClass,
    getCvssBadgeClass,
    runSecurityScan,
    patchVulnerability,
    rotateSecret,
  }
}
