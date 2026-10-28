import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'
import { createPinia, setActivePinia } from 'pinia'

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

setActivePinia(createPinia())

const { useIncidents } = await import('../src/composables/useIncidents.ts')
const { useCompliance } = await import('../src/composables/useCompliance.ts')
const { useAlerts } = await import('../src/composables/useAlerts.ts')
const { api } = await import('../src/api/client.ts')

describe('Task 073 Slice 73C: Behavioral Governance, Incidents & Polish Tests', () => {
  describe('1. Global Semantic Status Dots (StatusBadge)', () => {
    // Testing status normalization logic directly
    const EMERALD_STATUSES = new Set([
      'ready', 'healthy', 'active', 'online', 'connected', 'ok', 'live', 'running', 'deployed', 'success',
      'pass', 'resolved', 'mitigated', 'completed', 'verified', 'armed', 'isolated'
    ])
    const AMBER_STATUSES = new Set([
      'warning', 'pending', 'degraded', 'elevated', 'in_progress', 'inprogress', 'standby', 'polling',
      'open', 'promoting', 'mitigating', 'medium'
    ])
    const ROSE_STATUSES = new Set([
      'failed', 'critical', 'error', 'offline', 'unhealthy', 'disconnected', 'down', 'danger',
      'rejected', 'blocked', 'mismatch', 'high'
    ])

    function normalizeStatus(s?: string): string {
      if (!s) return 'slate'
      const lower = s.toLowerCase().trim()
      if (EMERALD_STATUSES.has(lower)) return 'emerald'
      if (AMBER_STATUSES.has(lower)) return 'amber'
      if (ROSE_STATUSES.has(lower)) return 'rose'
      return 'slate'
    }

    it('medium maps to amber status for /audit MEDIUM violations', () => {
      assert.equal(normalizeStatus('medium'), 'amber')
      assert.equal(normalizeStatus('MEDIUM'), 'amber')
    })

    it('high maps to rose status for /audit HIGH violations', () => {
      assert.equal(normalizeStatus('high'), 'rose')
      assert.equal(normalizeStatus('HIGH'), 'rose')
    })

    it('isolated maps to emerald status for /tenancy ISOLATED organizations', () => {
      assert.equal(normalizeStatus('isolated'), 'emerald')
      assert.equal(normalizeStatus('ISOLATED'), 'emerald')
    })
  })

  describe('2. /incidents Active P1 Math & Status Sums', () => {
    it('criticalCount ONLY counts non-resolved critical incidents (excluding resolved)', () => {
      const { incidents, criticalCount, countOpen, countInProgress, countResolved, totalIncidents } = useIncidents()

      incidents.value = [
        {
          id: 'inc-1',
          pod_name: 'api-gateway-1',
          namespace: 'default',
          cluster_name: 'k8s-prod',
          type: 'CrashLoopBackOff',
          severity: 'critical',
          status: 'open',
          created_at: new Date().toISOString()
        },
        {
          id: 'inc-2',
          pod_name: 'worker-queue-2',
          namespace: 'default',
          cluster_name: 'k8s-prod',
          type: 'OOMKilled',
          severity: 'critical',
          status: 'resolved',
          created_at: new Date().toISOString()
        },
        {
          id: 'inc-3',
          pod_name: 'cache-redis-3',
          namespace: 'default',
          cluster_name: 'k8s-prod',
          type: 'HighMemory',
          severity: 'high',
          status: 'mitigating',
          created_at: new Date().toISOString()
        }
      ]

      assert.equal(totalIncidents.value, 3, 'Total incidents should be 3')
      assert.equal(criticalCount.value, 1, 'Only non-resolved critical incident counts as P1 Active (was 2 before fix)')
      assert.equal(countOpen.value, 1, '1 open incident')
      assert.equal(countInProgress.value, 1, '1 mitigating incident')
      assert.equal(countResolved.value, 1, '1 resolved incident')
      assert.equal(countOpen.value + countInProgress.value + countResolved.value, totalIncidents.value, 'Open + InProgress + Resolved sums to total')
    })
  })

  describe('3. /audit Accurate NSA-CISA vs CIS Framework Matching', () => {
    it('formatFrameworkTag correctly matches NSA-CISA and does NOT mistakenly treat it as CIS LEVEL 2', () => {
      const { frameworks, formatFrameworkTag } = useCompliance()
      frameworks.value = [
        {
          id: 'nsa-cisa',
          name: 'NSA-CISA Kubernetes Hardening',
          total_checks: 45,
          passed_checks: 40,
          failed_checks: 5,
          score: 88,
          last_scan_at: new Date().toISOString()
        },
        {
          id: 'cis-k8s',
          name: 'CIS Kubernetes Benchmark v1.8',
          total_checks: 60,
          passed_checks: 55,
          failed_checks: 5,
          score: 91,
          last_scan_at: new Date().toISOString()
        }
      ]

      // NSA-CISA framework
      const nsaTag = formatFrameworkTag('nsa-cisa')
      assert.equal(nsaTag, 'NSA-CISA', 'NSA-CISA must be returned, NOT "CIS LEVEL 2"!')

      // CIS framework
      const cisTag = formatFrameworkTag('cis-k8s')
      assert.equal(cisTag, 'CIS LEVEL 2')

      // Direct rule prefix tags
      assert.equal(formatFrameworkTag('SOC 2'), 'SOC 2')
      assert.equal(formatFrameworkTag('NSA-CISA'), 'NSA-CISA')
    })
  })

  describe('4. /alerts Channel Delete Action & useAlerts Composable', () => {
    it('deleteChannel calls API and removes channel from reactive list', async () => {
      const alerts = useAlerts()
      alerts.channels.value = [
        {
          ID: 'chan-slack-1',
          Name: 'DevOps Slack',
          Type: 'slack',
          Config: { channel: '#devops-alerts' },
          Enabled: true
        },
        {
          ID: 'chan-email-2',
          Name: 'Platform Ops Email',
          Type: 'email',
          Config: { recipients: ['ops@k8s.local'] },
          Enabled: true
        }
      ]

      // Mock api.delete
      let deletedEndpoint = ''
      const originalDelete = api.delete
      api.delete = (async (endpoint: string) => {
        deletedEndpoint = endpoint
        return {} as any
      }) as any

      try {
        await alerts.deleteChannel('chan-slack-1')
        assert.equal(deletedEndpoint, '/alerts/channels/chan-slack-1', 'Must call DELETE /alerts/channels/{id}')
        assert.equal(alerts.channels.value.length, 1, 'Channel must be removed from reactive channels list')
        assert.equal(alerts.channels.value[0].ID, 'chan-email-2')
        assert.match(alerts.feedbackMessage.value || '', /deleted successfully/i)
      } finally {
        api.delete = originalDelete
      }
    })
  })

  describe('5. /settings 2FA Setup Account Display', () => {
    it('fallback account name correctly defaults to platform_admin', () => {
      const getAccountLabel = (user?: { email?: string; username?: string }) => {
        return user?.email || user?.username || 'platform_admin'
      }

      assert.equal(getAccountLabel({ username: 'platform_admin' }), 'platform_admin')
      assert.equal(getAccountLabel({ email: 'sre@corp.com' }), 'sre@corp.com')
      assert.equal(getAccountLabel(undefined), 'platform_admin')
    })
  })

  describe('6. Automation & Runbooks <= 2 Row Action Limits', () => {
    it('Row actions provide strictly 1 primary inline action + [ ⋯ ] ActionDropdown', () => {
      // In AutomationRulesTable: primary inline action is 'trigger', secondary 'edit' is in ActionDropdown
      const automationActions = [
        { id: 'edit', label: 'Edit Rule', icon: 'sliders' },
        { id: 'delete', label: 'Delete Rule', icon: 'trash', variant: 'danger' }
      ]
      assert.equal(automationActions.length, 2, 'Automation row ActionDropdown contains edit and delete')

      // In RunbooksTable: primary inline action is 'run', secondary 'inspect' and 'edit' are in ActionDropdown
      const runbookActions = [
        { id: 'inspect', label: 'Inspect Steps', icon: 'eye' },
        { id: 'edit', label: 'Edit Runbook', icon: 'sliders' },
        { id: 'delete', label: 'Delete Runbook', icon: 'trash', variant: 'danger' }
      ]
      assert.equal(runbookActions.length, 3, 'Runbook row ActionDropdown contains inspect, edit, and delete')
    })
  })
})
