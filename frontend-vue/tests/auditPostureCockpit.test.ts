import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { register } from 'node:module'

register('data:text/javascript,' + encodeURIComponent(`
export async function resolve(specifier, context, nextResolve) {
  try {
    return await nextResolve(specifier, context);
  } catch (err) {
    if (specifier.startsWith('.')) {
      try {
        return await nextResolve(specifier + '.ts', context);
      } catch {}
    }
    throw err;
  }
}
`))

const {
  computeSeverityDistribution,
  filterComplianceViolations,
  filterAuditFindings,
  calculateFrameworkSummary,
  getScoreRingColor,
} = await import('../src/composables/useCompliance.ts')

import type { ComplianceFramework, ComplianceViolation, AuditFinding } from '../src/api/governance.ts'

describe('Task T068-2.2: Security Posture, Compliance Scorecards & Forensic Cockpit', () => {
  const mockFrameworks: ComplianceFramework[] = [
    {
      id: 'cis-k8s',
      name: 'CIS Kubernetes Benchmark v1.8',
      total_checks: 124,
      passed_checks: 104,
      failed_checks: 20,
      score: 83.87,
      last_scan_at: '2026-10-25T14:30:00Z',
    },
    {
      id: 'nsa-cisa',
      name: 'NSA/CISA Hardening Guidance',
      total_checks: 68,
      passed_checks: 62,
      failed_checks: 6,
      score: 91.18,
      last_scan_at: '2026-10-25T14:35:00Z',
    },
    {
      id: 'soc2',
      name: 'SOC 2 Type II Controls',
      total_checks: 50,
      passed_checks: 48,
      failed_checks: 2,
      score: 96.0,
      last_scan_at: '2026-10-25T14:40:00Z',
    },
  ]

  const mockViolations: ComplianceViolation[] = [
    {
      id: 'viol-1',
      framework_id: 'cis-k8s',
      severity: 'critical',
      policy: 'Privileged containers forbidden in production',
      resource: 'kube-proxy-daemonset',
      namespace: 'kube-system',
      cluster: 'primary',
      message: 'Container runs with securityContext.privileged: true',
      resolved: false,
      detected_at: '2026-10-25T10:00:00Z',
    },
    {
      id: 'viol-2',
      framework_id: 'cis-k8s',
      severity: 'high',
      policy: 'Root file system must be read-only',
      resource: 'ingress-nginx-controller',
      namespace: 'ingress',
      cluster: 'primary',
      message: 'Container allows write access to root filesystem',
      resolved: false,
      detected_at: '2026-10-25T10:05:00Z',
    },
    {
      id: 'viol-3',
      framework_id: 'nsa-cisa',
      severity: 'medium',
      policy: 'Default service account tokens should not be automounted',
      resource: 'metrics-server',
      namespace: 'kube-system',
      cluster: 'primary',
      message: 'automountServiceAccountToken is true on default SA',
      resolved: false,
      detected_at: '2026-10-25T10:10:00Z',
    },
    {
      id: 'viol-4',
      framework_id: 'soc2',
      severity: 'low',
      policy: 'Namespace missing owner contact label',
      resource: 'staging-app',
      namespace: 'staging',
      cluster: 'primary',
      message: 'Label app.kubernetes.io/managed-by is absent',
      resolved: false,
      detected_at: '2026-10-25T10:15:00Z',
    },
  ]

  const mockFindings: AuditFinding[] = [
    {
      id: 'CVE-2026-1945',
      category: 'cve_vulnerability',
      severity: 'critical',
      description: 'Buffer overflow in glibc libresolv resolver',
      remediation: 'Upgrade to debian bookworm package libc6 2.36-9+deb12u8',
      status: 'open',
      detected_at: '2026-10-25T12:00:00Z',
    },
    {
      id: 'CVE-2026-3821',
      category: 'cve_vulnerability',
      severity: 'high',
      description: 'Denial of service in Envoy HTTP/2 parser',
      remediation: 'Upgrade ingress image to v1.11.2',
      status: 'open',
      detected_at: '2026-10-25T12:05:00Z',
    },
    {
      id: 'CVE-2026-0012',
      category: 'cve_vulnerability',
      severity: 'critical',
      description: 'Resolved defect in container image',
      remediation: 'Patched',
      status: 'resolved',
      detected_at: '2026-10-20T00:00:00Z',
      resolved_at: '2026-10-21T00:00:00Z',
    },
  ]

  it('computeSeverityDistribution aggregates open violations and CVEs accurately', () => {
    const dist = computeSeverityDistribution(mockViolations, mockFindings)

    // Violations: 1 crit, 1 high, 1 med, 1 low
    // Findings (open only): 1 crit, 1 high, 0 med, 0 low (resolved CVE-2026-0012 ignored)
    // Totals: Critical = 2, High = 2, Medium = 1, Low = 1 -> Total = 6
    assert.strictEqual(dist.critical, 2)
    assert.strictEqual(dist.high, 2)
    assert.strictEqual(dist.medium, 1)
    assert.strictEqual(dist.low, 1)
    assert.strictEqual(dist.total, 6)

    // Percentages: 2/6 = 33.3%, 2/6 = 33.3%, 1/6 = 16.7%, 1/6 = 16.7%
    assert.strictEqual(Math.round(dist.criticalPct), 33)
    assert.strictEqual(Math.round(dist.highPct), 33)
    assert.strictEqual(Math.round(dist.mediumPct), 17)
    assert.strictEqual(Math.round(dist.lowPct), 17)
  })

  it('computeSeverityDistribution handles zero open defects safely without NaN or division by zero', () => {
    const emptyDist = computeSeverityDistribution([], [])
    assert.strictEqual(emptyDist.critical, 0)
    assert.strictEqual(emptyDist.high, 0)
    assert.strictEqual(emptyDist.medium, 0)
    assert.strictEqual(emptyDist.low, 0)
    assert.strictEqual(emptyDist.total, 0)
    assert.strictEqual(emptyDist.criticalPct, 0)
    assert.strictEqual(emptyDist.highPct, 0)
    assert.strictEqual(emptyDist.mediumPct, 0)
    assert.strictEqual(emptyDist.lowPct, 0)
    assert.strictEqual(emptyDist.isClean, true)
  })

  it('calculateFrameworkSummary computes aggregate score and check counts', () => {
    const summary = calculateFrameworkSummary(mockFrameworks)
    // Total checks: 124 + 68 + 50 = 242
    assert.strictEqual(summary.totalChecks, 242)
    // Passed checks: 104 + 62 + 48 = 214
    assert.strictEqual(summary.passedChecks, 214)
    // Failed checks: 20 + 6 + 2 = 28
    assert.strictEqual(summary.failedChecks, 28)
    // Average score: (83.87 + 91.18 + 96.0) / 3 = 90.35
    assert.strictEqual(summary.averageScore.toFixed(2), '90.35')
  })

  it('getScoreRingColor assigns high-contrast semantic color tokens based on threshold', () => {
    assert.strictEqual(getScoreRingColor(95), 'var(--color-status-online, #10b981)')
    assert.strictEqual(getScoreRingColor(80), 'var(--color-status-warning, #f59e0b)')
    assert.strictEqual(getScoreRingColor(60), 'var(--color-status-offline, #f43f5e)')
  })

  it('filterComplianceViolations filters by framework_id, severity, and search query', () => {
    // 1. By Framework
    const cisViolations = filterComplianceViolations(mockViolations, 'cis-k8s', 'ALL', '')
    assert.strictEqual(cisViolations.length, 2)
    assert.ok(cisViolations.every(v => v.framework_id === 'cis-k8s'))

    // 2. By Severity
    const critViolations = filterComplianceViolations(mockViolations, '', 'CRITICAL', '')
    assert.strictEqual(critViolations.length, 1)
    assert.strictEqual(critViolations[0].id, 'viol-1')

    // 3. By Search Query
    const queryResults = filterComplianceViolations(mockViolations, '', 'ALL', 'nginx')
    assert.strictEqual(queryResults.length, 1)
    assert.strictEqual(queryResults[0].resource, 'ingress-nginx-controller')

    // 4. Combined
    const combined = filterComplianceViolations(mockViolations, 'cis-k8s', 'HIGH', 'root')
    assert.strictEqual(combined.length, 1)
    assert.strictEqual(combined[0].id, 'viol-2')
  })

  it('filterAuditFindings filters open container CVE findings by severity and query', () => {
    // Filter critical open CVEs
    const critCves = filterAuditFindings(mockFindings, 'CRITICAL', '')
    assert.strictEqual(critCves.length, 1)
    assert.strictEqual(critCves[0].id, 'CVE-2026-1945')

    // Filter by package/description search
    const glibcSearch = filterAuditFindings(mockFindings, 'ALL', 'glibc')
    assert.strictEqual(glibcSearch.length, 1)
    assert.strictEqual(glibcSearch[0].id, 'CVE-2026-1945')
  })

  it('enforces strict architectural limits: AuditView <= 350 lines, PostureHeader < 280 lines, all files < 500 lines', () => {
    const targetFiles = [
      '../src/views/AuditView.vue',
      '../src/components/compliance/CompliancePostureHeader.vue',
      '../src/components/compliance/ComplianceControlsTable.vue',
      '../src/components/compliance/ComplianceFrameworksGrid.vue',
      '../src/components/compliance/ComplianceMobileCards.vue',
      '../src/components/compliance/ComplianceViolationDrawer.vue',
      '../src/components/audit/AuditTrailTable.vue',
      '../src/components/audit/AuditFilterToolbar.vue',
      '../src/components/audit/VulnerabilityFindingsTable.vue',
      '../src/components/audit/AuditPayloadDrawer.vue',
      '../src/composables/useCompliance.ts',
      '../src/assets/styles/views/audit.css',
      '../src/assets/styles/views/compliance.css',
      '../src/assets/styles/components/audit-drawers.css',
    ]

    const currentDir = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([a-zA-Z]:)/, '$1'))
    for (const relPath of targetFiles) {
      const fullPath = path.resolve(currentDir, relPath)
      if (fs.existsSync(fullPath)) {
        const content = fs.readFileSync(fullPath, 'utf8')
        const lines = content.split('\n').length
        assert.ok(lines < 500, `File ${relPath} exceeds 500 lines (current: ${lines})`)
        if (relPath.includes('AuditView.vue')) {
          assert.ok(lines <= 350, `AuditView.vue must be <= 350 lines (current: ${lines})`)
        }
        if (relPath.includes('CompliancePostureHeader.vue')) {
          assert.ok(lines < 280, `CompliancePostureHeader.vue must be < 280 lines (current: ${lines})`)
        }
      }
    }
  })
})
