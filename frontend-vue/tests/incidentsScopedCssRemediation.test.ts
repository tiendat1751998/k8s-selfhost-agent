import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface IncidentRecord {
  id: string
  pod_name: string
  namespace: string
  type: string
  severity: 'P1' | 'P2' | 'P3'
  status: 'firing' | 'investigating' | 'mitigated' | 'resolved'
  timestamp: string
}

export function formatIncidentCardDetails(inc: IncidentRecord): {
  title: string
  namespace: string
  typeBadge: string
  severityColor: string
} {
  const severityColorMap: Record<string, string> = {
    P1: '#f43f5e', // rose
    P2: '#f59e0b', // amber
    P3: '#3b82f6'  // blue
  }

  return {
    title: inc.pod_name,
    namespace: inc.namespace,
    typeBadge: inc.type.toUpperCase(),
    severityColor: severityColorMap[inc.severity] || '#64748b'
  }
}

export function filterIncidents(
  incidents: IncidentRecord[],
  filters: { namespace?: string; severity?: string; query?: string }
): IncidentRecord[] {
  return incidents.filter(inc => {
    if (filters.namespace && filters.namespace !== 'all' && inc.namespace !== filters.namespace) {
      return false
    }
    if (filters.severity && filters.severity !== 'all' && inc.severity !== filters.severity) {
      return false
    }
    if (filters.query) {
      const q = filters.query.trim().toLowerCase()
      const matchesPod = inc.pod_name.toLowerCase().includes(q)
      const matchesType = inc.type.toLowerCase().includes(q)
      const matchesNs = inc.namespace.toLowerCase().includes(q)
      if (!matchesPod && !matchesType && !matchesNs) return false
    }
    return true
  })
}

export function resolveIncidentDisplayMode(viewportWidth: number): 'mobile-cards' | 'desktop-table' {
  return viewportWidth <= 768 ? 'mobile-cards' : 'desktop-table'
}

describe('Incidents UI Scoped CSS Remediation & Responsive Card Layout Specifications', () => {
  const sampleIncidents: IncidentRecord[] = [
    {
      id: 'inc-01',
      pod_name: 'payment-gateway-8f9d-1',
      namespace: 'finance',
      type: 'OOMKilled',
      severity: 'P1',
      status: 'firing',
      timestamp: '2026-10-15T08:00:00Z'
    },
    {
      id: 'inc-02',
      pod_name: 'auth-service-5c6e-2',
      namespace: 'core',
      type: 'CrashLoopBackOff',
      severity: 'P2',
      status: 'investigating',
      timestamp: '2026-10-15T08:15:00Z'
    },
    {
      id: 'inc-03',
      pod_name: 'inventory-worker-2b1a-0',
      namespace: 'logistics',
      type: 'ProbeFailure',
      severity: 'P3',
      status: 'mitigated',
      timestamp: '2026-10-15T08:30:00Z'
    }
  ]

  it('formatIncidentCardDetails derives card properties accurately', () => {
    const details = formatIncidentCardDetails(sampleIncidents[0])
    assert.equal(details.title, 'payment-gateway-8f9d-1')
    assert.equal(details.namespace, 'finance')
    assert.equal(details.typeBadge, 'OOMKILLED')
    assert.equal(details.severityColor, '#f43f5e')
  })

  it('filterIncidents filters correctly by namespace, severity, and search query', () => {
    const p1Only = filterIncidents(sampleIncidents, { severity: 'P1' })
    assert.equal(p1Only.length, 1)
    assert.equal(p1Only[0].id, 'inc-01')

    const coreNs = filterIncidents(sampleIncidents, { namespace: 'core' })
    assert.equal(coreNs.length, 1)
    assert.equal(coreNs[0].id, 'inc-02')

    const querySearch = filterIncidents(sampleIncidents, { query: 'inventory' })
    assert.equal(querySearch.length, 1)
    assert.equal(querySearch[0].id, 'inc-03')

    const noMatches = filterIncidents(sampleIncidents, { query: 'nonexistent-service' })
    assert.equal(noMatches.length, 0)
  })

  it('resolveIncidentDisplayMode maps viewports to mobile cards or desktop table', () => {
    assert.equal(resolveIncidentDisplayMode(375), 'mobile-cards')
    assert.equal(resolveIncidentDisplayMode(768), 'mobile-cards')
    assert.equal(resolveIncidentDisplayMode(1024), 'desktop-table')
    assert.equal(resolveIncidentDisplayMode(1440), 'desktop-table')
  })
})
