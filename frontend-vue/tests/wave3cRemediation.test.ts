import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
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

describe('Wave 3C Remediation & Honest Frontend Specifications', () => {
  describe('1. Explorer Diagnostic Banner & API Degradation', () => {
    it('parses k8s_degraded and degraded_reason from explorer response', async () => {
      const { parseExplorerResponse } = await import('../src/composables/useClusterExplorer.ts')
      assert.ok(parseExplorerResponse, 'parseExplorerResponse helper must be exported')
      
      const degradedRes = {
        data: [],
        total: 0,
        k8s_degraded: true,
        degraded_reason: 'kube-apiserver (:6443) connection refused on primary-cluster'
      }
      const parsed = parseExplorerResponse(degradedRes)
      assert.equal(parsed.isDegraded, true)
      assert.equal(parsed.degradedReason, 'kube-apiserver (:6443) connection refused on primary-cluster')
      assert.equal(parsed.items.length, 0)
    })

    it('defaults degradedReason when k8s_degraded is true but degraded_reason is omitted', async () => {
      const { parseExplorerResponse } = await import('../src/composables/useClusterExplorer.ts')
      const res = { data: [], total: 0, k8s_degraded: true }
      const parsed = parseExplorerResponse(res)
      assert.equal(parsed.isDegraded, true)
      assert.ok(parsed.degradedReason.includes(':6443'), 'Default reason should reference :6443')
    })
  })

  describe('2. Overview Node IP & Honest Container KPI & Spacing', () => {
    it('extracts real IP from node.address (http://10.10.10.60:9100 -> 10.10.10.60)', async () => {
      const { extractNodeIp } = await import('../src/components/overview/overviewMobileUtils.ts')
      
      const node60 = { address: 'http://10.10.10.60:9100', node_name: 'k8smasterdeb' }
      const node133 = { address: 'http://10.10.10.133:9100', node_name: 'k8smater133' }
      const node80 = { endpoint: 'http://10.10.10.80:9100', node_name: 'k8sworker' }
      const nodeEmpty = { node_name: 'unknown-host' }

      assert.equal(extractNodeIp(node60 as any), '10.10.10.60')
      assert.equal(extractNodeIp(node133 as any), '10.10.10.133')
      assert.equal(extractNodeIp(node80 as any), '10.10.10.80')
      assert.equal(extractNodeIp(nodeEmpty as any), '--')
    })

    it('determines honest container status: 0 PODS + K8S :6443 OFFLINE when total is 0 but nodes are online', async () => {
      const { resolveContainerKpi } = await import('../src/components/overview/overviewMobileUtils.ts')

      // Case 1: 0 containers, 3 healthy nodes online (offline k8s :6443)
      const kpiOffline = resolveContainerKpi(0, 0, 3)
      assert.equal(kpiOffline.primaryText, '0 PODS')
      assert.equal(kpiOffline.badgeText, 'K8S :6443 OFFLINE')
      assert.equal(kpiOffline.badgeVariant, 'badge-amber')

      // Case 2: 12 containers running out of 12 total
      const kpiRunning = resolveContainerKpi(12, 12, 3)
      assert.equal(kpiRunning.primaryText, '12/12')
      assert.equal(kpiRunning.badgeText, 'RUNNING')
      assert.equal(kpiRunning.badgeVariant, 'badge-cyan')

      // Case 3: 0 containers and 0 nodes online
      const kpiNone = resolveContainerKpi(0, 0, 0)
      assert.equal(kpiNone.primaryText, '0/0')
      assert.equal(kpiNone.badgeText, 'STOPPED')
    })
  })

  describe('3. Incidents API 200 { available: false } & Debug Toggle', () => {
    it('normalizeReport returns null when available is false', async () => {
      const { normalizeReport } = await import('../src/api/incidents.ts')

      const unavailableRes = { available: false, message: 'No RCA generated for incident' }
      assert.equal(normalizeReport(unavailableRes), null, 'normalizeReport must return null when available is false')

      const validReport = { id: 'rca-1', incident_id: 'inc-1', root_cause: 'OOMKilled', available: true }
      const norm = normalizeReport(validReport)
      assert.ok(norm)
      assert.equal(norm?.root_cause, 'OOMKilled')
    })

    it('normalizePR returns null when available is false or gitops_configured is false', async () => {
      const { normalizePR } = await import('../src/api/incidents.ts')

      const unavailablePR = { available: false, gitops_configured: false }
      assert.equal(normalizePR(unavailablePR), null, 'normalizePR must return null when available is false')

      const unconfiguredPR = { gitops_configured: false }
      assert.equal(normalizePR(unconfiguredPR), null, 'normalizePR must return null when gitops_configured is false')

      const validPR = { id: 'pr-10', incident_id: 'inc-1', title: 'fix: OOM', available: true }
      const norm = normalizePR(validPR)
      assert.ok(norm)
      assert.equal(norm?.title, 'fix: OOM')
    })

    it('isSimulationAllowed only returns true when route query debug === "1"', async () => {
      const { isSimulationAllowed } = await import('../src/composables/useIncidents.ts')

      assert.equal(isSimulationAllowed({ debug: '1' }), true)
      assert.equal(isSimulationAllowed({ debug: 'true' }), false)
      assert.equal(isSimulationAllowed({}), false)
      assert.equal(isSimulationAllowed(undefined), false)
    })
  })

  describe('4. Settings Honest KMS & Password Change Contract', () => {
    it('resolveKmsProviderTelemetry displays Local AES-256-GCM Envelope Encryption (Software KMS) when hardware_backed is false', async () => {
      const { resolveKmsProviderTelemetry } = await import('../src/components/layout/zeroTrustUtils.ts')

      const softwareKms = resolveKmsProviderTelemetry({
        provider: 'local-aes256-gcm',
        hardware_backed: false,
      })
      assert.equal(softwareKms.title, 'Local AES-256-GCM Envelope Encryption (Software KMS)')
      assert.equal(softwareKms.enclaveIsolation, 'Software Keystore (App Enclave)')
      assert.notEqual(softwareKms.title, 'Hardware Security Module (HSM PKCS#11) / Vault KMS Provider')
    })

    it('SettingsPasswordCard validates new password minimum length and matching confirmation', async () => {
      const { validatePasswordChange } = await import('../src/components/settings/passwordChangeUtils.ts')

      const shortRes = validatePasswordChange('curPass123', 'short', 'short')
      assert.equal(shortRes.valid, false)
      assert.ok(shortRes.error?.includes('10 characters'), 'New password must require at least 10 characters')

      const mismatchRes = validatePasswordChange('curPass123', 'newSecretPassword123', 'diffSecretPassword123')
      assert.equal(mismatchRes.valid, false)
      assert.ok(mismatchRes.error?.includes('match'), 'Passwords must match')

      const validRes = validatePasswordChange('curPass123', 'newSecretPassword123', 'newSecretPassword123')
      assert.equal(validRes.valid, true)
      assert.equal(validRes.error, null)
    })
  })
})
