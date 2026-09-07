import { ref, computed, onMounted } from 'vue'
import {
  complianceApi,
  auditApi,
  type ComplianceFramework,
  type ComplianceViolation,
  type AuditRun,
} from '../api/governance'

export type ComplianceStandard = 'ALL' | 'CIS Benchmark' | 'NIST SP 800-53' | 'PCI-DSS' | 'SOC 2' | 'HIPAA'
export type SeverityFilter = 'ALL' | 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'
export type AuditRunStatus = 'idle' | 'pending' | 'running' | 'completed' | 'failed'

export interface ComplianceControlItem extends ComplianceViolation {
  [key: string]: unknown
}

export const COMPLIANCE_STANDARDS: ComplianceStandard[] = [
  'ALL',
  'CIS Benchmark',
  'NIST SP 800-53',
  'PCI-DSS',
  'SOC 2',
  'HIPAA',
]

export function useCompliance() {
  const frameworks = ref<ComplianceFramework[]>([])
  const violations = ref<ComplianceViolation[]>([])
  const totalViolations = ref(0)
  const loading = ref(false)
  const isScanning = ref(false)
  const error = ref<string | null>(null)

  const selectedFrameworkId = ref<string>('')
  const selectedStandard = ref<ComplianceStandard>('ALL')
  const activeSeverity = ref<SeverityFilter>('ALL')
  const searchQuery = ref<string>('')

  const auditStatus = ref<AuditRunStatus>('idle')
  const latestRun = ref<AuditRun | null>(null)
  const selectedViolation = ref<ComplianceViolation | null>(null)
  const modalMode = ref<'inspect' | 'remediate' | null>(null)

  onMounted(() => {
    fetchComplianceData()
  })

  async function fetchComplianceData() {
    loading.value = true
    error.value = null
    try {
      const [fwData, viData, runData] = await Promise.all([
        complianceApi.getFrameworks(),
        complianceApi.getViolations(activeSeverity.value === 'ALL' ? undefined : activeSeverity.value.toLowerCase()),
        auditApi.getLatestRun().catch(() => null),
      ])
      frameworks.value = fwData || []
      violations.value = viData?.data || []
      totalViolations.value = viData?.total || 0
      latestRun.value = runData
      if (runData) {
        auditStatus.value = runData.status
      }
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to load compliance data'
      frameworks.value = []
      violations.value = []
      totalViolations.value = 0
    } finally {
      loading.value = false
    }
  }

  async function triggerScan() {
    if (isScanning.value) return
    isScanning.value = true
    auditStatus.value = 'running'
    error.value = null
    try {
      await auditApi.triggerRun()
      await fetchComplianceData()
      auditStatus.value = 'completed'
    } catch (err: unknown) {
      auditStatus.value = 'failed'
      error.value = err instanceof Error ? err.message : 'Failed to trigger compliance scan'
    } finally {
      isScanning.value = false
    }
  }

  function matchesStandard(fwNameOrTag: string, standard: ComplianceStandard): boolean {
    if (standard === 'ALL') return true
    const normalized = fwNameOrTag.toLowerCase()
    if (standard === 'CIS Benchmark') return normalized.includes('cis')
    if (standard === 'NIST SP 800-53') return normalized.includes('nist')
    if (standard === 'PCI-DSS') return normalized.includes('pci')
    if (standard === 'SOC 2') return normalized.includes('soc')
    if (standard === 'HIPAA') return normalized.includes('hipaa')
    return false
  }

  function selectFramework(id: string) {
    selectedFrameworkId.value = selectedFrameworkId.value === id ? '' : id
  }

  function selectStandard(std: ComplianceStandard) {
    selectedStandard.value = std
    if (std !== 'ALL') {
      const matched = frameworks.value.find(f => matchesStandard(f.name, std))
      if (matched) selectedFrameworkId.value = matched.id
    } else {
      selectedFrameworkId.value = ''
    }
  }

  function getSelectedFrameworkName(): string {
    const fw = frameworks.value.find(f => f.id === selectedFrameworkId.value)
    return fw ? fw.name : selectedFrameworkId.value
  }

  const overallScore = computed(() => {
    const list = frameworks.value
    if (list.length === 0) return 0
    return list.reduce((acc, f) => acc + f.score, 0) / list.length
  })

  const passingControlsCount = computed(() => frameworks.value.reduce((acc, f) => acc + f.passed_checks, 0))
  const totalControlsCount = computed(() => frameworks.value.reduce((acc, f) => acc + f.total_checks, 0))
  const criticalViolationsCount = computed(() => violations.value.filter(v => v.severity.toLowerCase() === 'critical').length)
  const highViolationsCount = computed(() => violations.value.filter(v => v.severity.toLowerCase() === 'high').length)
  const mediumViolationsCount = computed(() => violations.value.filter(v => v.severity.toLowerCase() === 'medium').length)
  const lowViolationsCount = computed(() => violations.value.filter(v => v.severity.toLowerCase() === 'low').length)

  const severityFilters = computed(() => [
    { key: 'ALL' as const, label: 'All Severities', count: violations.value.length, badgeClass: 'badge-cyan' },
    { key: 'CRITICAL' as const, label: 'Critical', count: criticalViolationsCount.value, badgeClass: 'badge-rose' },
    { key: 'HIGH' as const, label: 'High', count: highViolationsCount.value, badgeClass: 'badge-amber' },
    { key: 'MEDIUM' as const, label: 'Medium', count: mediumViolationsCount.value, badgeClass: 'badge-violet' },
    { key: 'LOW' as const, label: 'Low', count: lowViolationsCount.value, badgeClass: 'badge-emerald' },
  ])

  const filteredFrameworks = computed(() => {
    if (selectedStandard.value === 'ALL') return frameworks.value
    return frameworks.value.filter(f => matchesStandard(f.name, selectedStandard.value))
  })

  const filteredViolations = computed(() => {
    return violations.value.filter(v => {
      if (selectedFrameworkId.value && v.framework_id !== selectedFrameworkId.value) return false
      if (selectedStandard.value !== 'ALL') {
        const fw = frameworks.value.find(f => f.id === v.framework_id)
        if (!matchesStandard(fw ? fw.name : v.framework_id, selectedStandard.value)) return false
      }
      if (activeSeverity.value !== 'ALL' && v.severity.toUpperCase() !== activeSeverity.value) return false
      if (searchQuery.value.trim()) {
        const q = searchQuery.value.toLowerCase()
        return (
          v.policy.toLowerCase().includes(q) ||
          v.resource.toLowerCase().includes(q) ||
          (v.namespace || '').toLowerCase().includes(q) ||
          (v.message || '').toLowerCase().includes(q)
        )
      }
      return true
    })
  })

  function inspectControl(v: ComplianceViolation) {
    selectedViolation.value = v
    modalMode.value = 'inspect'
  }

  function remediateControl(v: ComplianceViolation) {
    selectedViolation.value = v
    modalMode.value = 'remediate'
  }

  function closeModal() {
    selectedViolation.value = null
    modalMode.value = null
  }

  function exportRemediationReport(format: 'markdown' | 'json' = 'markdown') {
    const timestamp = new Date().toISOString()
    const targetViolations = filteredViolations.value
    const isJson = format === 'json'

    let content = ''
    if (isJson) {
      content = JSON.stringify({
        generated_at: timestamp,
        aggregate_score: overallScore.value,
        total_violations: targetViolations.length,
        violations: targetViolations,
      }, null, 2)
    } else {
      content = '# Compliance Remediation Playbook\n' +
        'Generated: ' + timestamp + '\n' +
        'Aggregate Posture Score: ' + overallScore.value.toFixed(1) + '%\n' +
        'Total Impacted Controls: ' + targetViolations.length + '\n\n' +
        '## Actionable Findings\n\n'
      targetViolations.forEach((v, idx) => {
        content += '### ' + (idx + 1) + '. [' + v.severity.toUpperCase() + '] ' + v.policy + '\n' +
          '- **Resource:** `' + v.resource + '` (Namespace: `' + (v.namespace || 'default') + '` | Cluster: `' + (v.cluster || 'primary') + '`)\n' +
          '- **Finding:** ' + v.message + '\n' +
          '- **Framework ID:** `' + v.framework_id + '`\n' +
          '- **Remediation Steps:** Review manifest in Git, configure security context, reconcile drift.\n\n'
      })
    }

    const blob = new Blob([content], { type: isJson ? 'application/json' : 'text/markdown' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'compliance-remediation-' + new Date().toISOString().slice(0, 10) + '.' + (isJson ? 'json' : 'md')
    a.click()
    URL.revokeObjectURL(url)
  }

  function getScoreBadgeClass(score: number): string {
    if (score >= 90) return 'badge-emerald'
    if (score >= 75) return 'badge-amber'
    return 'badge-rose'
  }

  function getProgressColorClass(score: number): string {
    if (score >= 90) return 'bg-emerald'
    if (score >= 75) return 'bg-amber'
    return 'bg-rose'
  }

  function formatFrameworkTag(tag: string): string {
    if (!tag) return 'POLICY'
    const fw = frameworks.value.find(f => f.id === tag)
    if (fw) {
      if (fw.name.includes('CIS')) return 'CIS LEVEL 2'
      if (fw.name.includes('SOC 2') || fw.name.includes('SOC2')) return 'SOC 2'
      if (fw.name.includes('HIPAA')) return 'HIPAA'
      if (fw.name.includes('PCI')) return 'PCI-DSS'
      if (fw.name.includes('NIST')) return 'NIST SP 800-53'
      return fw.name.toUpperCase()
    }
    return tag.replace(/-/g, ' ').toUpperCase()
  }

  function formatDate(d: string): string {
    if (!d) return '-'
    try {
      return new Date(d).toLocaleDateString()
    } catch {
      return d
    }
  }

  return {
    frameworks,
    violations,
    totalViolations,
    loading,
    isScanning,
    error,
    selectedFrameworkId,
    selectedStandard,
    activeSeverity,
    searchQuery,
    auditStatus,
    latestRun,
    selectedViolation,
    modalMode,
    overallScore,
    passingControlsCount,
    totalControlsCount,
    criticalViolationsCount,
    highViolationsCount,
    mediumViolationsCount,
    lowViolationsCount,
    severityFilters,
    filteredFrameworks,
    filteredViolations,
    fetchComplianceData,
    triggerScan,
    selectFramework,
    selectStandard,
    getSelectedFrameworkName,
    inspectControl,
    remediateControl,
    closeModal,
    exportRemediationReport,
    getScoreBadgeClass,
    getProgressColorClass,
    formatFrameworkTag,
    formatDate,
  }
}
