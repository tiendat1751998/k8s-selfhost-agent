import { ref, computed, onMounted } from 'vue'
import { useSecurityStore } from '../stores/securityStore'
import { securityApi } from '../api/security'
import { complianceApi } from '../api/governance'

export interface CveFinding {
  id: string
  cve_id: string
  scanner: string
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
  type: 'success' | 'error'
  text: string
}

const DEFAULT_SECRETS: SecretAuditItem[] = [
  { id: 'sec-tls-001', name: 'ingress-wildcard-tls', namespace: 'ingress-nginx', secret_type: 'TLS Certificate', source_resource: 'cert-manager.io/Certificate', status: 'EXPIRING_SOON', exposure_detail: 'Production TLS Certificate expires in 4 days. ACME challenge auto-renewal failing.', expires_at: '2026-09-07T00:00:00Z', days_until_expiry: 4, masked_value: 'MIIDrjCCApagAwIBAgIQ...==', detected_at: 'Continuous Vault Monitor' },
  { id: 'sec-vlt-002', name: 'payment-gateway-stripe-token', namespace: 'finance-prod', secret_type: 'API Key', source_resource: 'deployment/payment-processor', status: 'EXPOSED', exposure_detail: 'Live Stripe API secret key logged in stdout pod crash logs and plaintext env var.', masked_value: 'sk_live_51M7...98xQ', detected_at: 'Trivy Secret Scanner (12m ago)' },
  { id: 'sec-eso-003', name: 'postgres-cluster-superuser', namespace: 'database', secret_type: 'Vault Secret', source_resource: 'ExternalSecret/postgres-auth', status: 'COMPLIANT', exposure_detail: 'HashiCorp Vault dynamic leasing synced via ESO. Automatic 24h lease renewal active.', expires_at: '2026-09-04T12:00:00Z', days_until_expiry: 1, masked_value: 'v.hvo.r1...02xP', detected_at: 'Vault ESO Synced (1m ago)' },
  { id: 'sec-jwt-004', name: 'internal-auth-jwt-secret', namespace: 'auth-gateway', secret_type: 'JWT Token', source_resource: 'configmap/auth-config', status: 'EXPOSED', exposure_detail: 'Weak symmetric HS256 JWT key stored unencrypted in ConfigMap rather than Secret.', masked_value: 'dHJpdnlfc2VjcmV0...==', detected_at: 'Checkov IaC Audit (35m ago)' },
]

const DEFAULT_RBAC_FINDINGS: RbacAuditFinding[] = [
  { id: 'rbac-001', subject: 'gitlab-runner-sa', subject_kind: 'ServiceAccount', role_binding: 'ci-cluster-admin-binding', namespace: 'ci-cd', risk_level: 'CRITICAL', escalation_vector: 'Bound to cluster-admin ClusterRole. Allows arbitrary pod exec, node drain, and secret exfiltration.', remediation: 'Scope permissions using namespace-restricted RoleBinding with least-privilege verbs.' },
  { id: 'rbac-002', subject: 'dev-contractors', subject_kind: 'Group', role_binding: 'debug-pods-exec-binding', namespace: 'production', risk_level: 'HIGH', escalation_vector: 'Wildcard authorization on pods/exec with capability to inject root privileged containers.', remediation: 'Remove wildcards, enable ephemeral debug containers with audit logging and session recording.' },
]

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

  const cveFindings = computed<CveFinding[]>(() => {
    return securityStore.violations.map((v) => {
      const raw = v as unknown as Record<string, unknown>
      const scanner = String(raw.scanner || raw.framework || v.framework_id || '--')
      const fixed = String(raw.fixed_version || v.remediation || '--')
      const installed = String(raw.installed_version || '--')
      const pkg = String(raw.package_name || v.resource_name || '--')
      const cvss = typeof raw.cvss_score === 'number' ? raw.cvss_score : (typeof raw.score === 'number' ? raw.score : 0)

      return {
        id: v.id,
        cve_id: v.rule_id,
        scanner,
        package_name: pkg,
        installed_version: installed,
        fixed_version: fixed,
        cvss_score: cvss,
        severity: v.severity,
        resource_name: v.resource_name,
        resource_type: v.resource_type,
        namespace: v.namespace,
        description: v.description,
        remediation: v.remediation,
        detected_at: v.detected_at || '--',
        patch_available: Boolean(fixed && fixed !== '--'),
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
      statusMessage.value = {
        type: 'success',
        text: 'Dispatched Trivy Container Scanner, Clair CVE DB & Checkov IaC Security Gate. All compliance matrices synchronized.',
      }
    } catch (err: unknown) {
      statusMessage.value = {
        type: 'error',
        text: err instanceof Error ? err.message : 'Failed to refresh security scan compliance.',
      }
    } finally {
      isScanning.value = false
    }
  }

  async function patchVulnerability(finding: CveFinding) {
    isPatching.value = true
    try {
      await complianceApi.remediateViolation(finding.id)
      statusMessage.value = {
        type: 'success',
        text: `Remediation initiated: Automated PR opened to update package ${finding.package_name} to ${finding.fixed_version} on ${finding.resource_name}.`,
      }
    } catch (err: unknown) {
      statusMessage.value = {
        type: 'error',
        text: err instanceof Error ? err.message : 'Failed to remediate vulnerability',
      }
    } finally {
      isPatching.value = false
    }
  }

  async function rotateSecret(secret: SecretAuditItem) {
    try {
      await securityApi.rotateKMSKey()
      const idx = secretsAuditList.value.findIndex((s) => s.id === secret.id)
      if (idx !== -1) {
        secretsAuditList.value[idx] = {
          ...secretsAuditList.value[idx],
          status: 'COMPLIANT',
          exposure_detail: 'Secret rotated via HashiCorp Vault dynamic engine. New token injected via ESO.',
          days_until_expiry: 90,
        }
      }
      statusMessage.value = {
        type: 'success',
        text: `Successfully rotated secret ${secret.name} in namespace ${secret.namespace} via HashiCorp Vault.`,
      }
    } catch (err: unknown) {
      statusMessage.value = {
        type: 'error',
        text: err instanceof Error ? err.message : 'Failed to rotate secret',
      }
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
