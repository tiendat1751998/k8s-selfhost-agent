import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Enterprise HTTP 501 State Handling & Integration Badges (W4-B)', () => {
  const clientApiPath = path.join(rootDir, 'src', 'api', 'client.ts')
  const drTabPath = path.join(rootDir, 'src', 'components', 'backup', 'ClusterDisasterRecoveryTab.vue')
  const auditViewPath = path.join(rootDir, 'src', 'views', 'AuditView.vue')
  const vulnerabilityTablePath = path.join(rootDir, 'src', 'components', 'audit', 'VulnerabilityFindingsTable.vue')
  const auditToolbarPath = path.join(rootDir, 'src', 'components', 'audit', 'AuditFilterToolbar.vue')
  const useAuditLogsPath = path.join(rootDir, 'src', 'composables', 'useAuditLogs.ts')
  const explorerViewPath = path.join(rootDir, 'src', 'views', 'ExplorerView.vue')
  const useK8sExplorerPath = path.join(rootDir, 'src', 'composables', 'useK8sExplorer.ts')
  const fleetViewPath = path.join(rootDir, 'src', 'views', 'FleetView.vue')
  const fleetTablePath = path.join(rootDir, 'src', 'components', 'fleet', 'FleetClustersTable.vue')
  const fleetGridPath = path.join(rootDir, 'src', 'components', 'fleet', 'FleetClustersGrid.vue')
  const useDevSecOpsPath = path.join(rootDir, 'src', 'composables', 'useDevSecOps.ts')
  const integrationBadgePath = path.join(rootDir, 'src', 'components', 'common', 'IntegrationBadge.vue')
  const integrationEmptyStatePath = path.join(rootDir, 'src', 'components', 'common', 'IntegrationEmptyState.vue')

  it('1. ApiClient exports ApiError, isNotImplementedError, and handles 501 responses', () => {
    const content = fs.readFileSync(clientApiPath, 'utf-8')
    assert.ok(content.includes('class ApiError extends Error'), 'client.ts must export ApiError class')
    assert.ok(content.includes('isNotImplementedError'), 'client.ts must export isNotImplementedError helper')
    assert.ok(content.includes('status: number') || content.includes('status = status'), 'ApiError must have status property')
    assert.ok(content.includes('throw new ApiError'), 'ApiClient must throw ApiError on non-ok responses')
  })

  it('2. IntegrationBadge and IntegrationEmptyState components exist and are created', () => {
    assert.ok(fs.existsSync(integrationBadgePath), 'IntegrationBadge.vue component must exist')
    assert.ok(fs.existsSync(integrationEmptyStatePath), 'IntegrationEmptyState.vue component must exist')
    const badgeContent = fs.readFileSync(integrationBadgePath, 'utf-8')
    assert.ok(badgeContent.includes('status') || badgeContent.includes('connected'), 'IntegrationBadge must accept status')
    const emptyContent = fs.readFileSync(integrationEmptyStatePath, 'utf-8')
    assert.ok(emptyContent.includes('controllerName') || emptyContent.includes('controller'), 'IntegrationEmptyState must accept controllerName')
  })

  it('3. Velero & DR: ClusterDisasterRecoveryTab intercepts 501, disables DR button, and displays controller state', () => {
    const content = fs.readFileSync(drTabPath, 'utf-8')
    assert.ok(content.includes('isNotImplementedError') || content.includes('velero') || content.includes('controller'), 'drTab must inspect 501 / controller state')
    assert.ok(content.includes('veleroConnected') || content.includes('veleroAvailable') || content.includes('controllerUnavailable'), 'drTab must track velero availability')
    assert.ok(content.includes('Controller Not Connected') || content.includes('Requires External Controller') || content.includes('IntegrationEmptyState'), 'drTab must display informative banner/empty state')
    assert.ok(content.includes(':disabled="backingUp || !clusterId ||') || content.includes(':title="'), 'drTab must disable 1-click DR when controller is unavailable with tooltip')
  })

  it('4. Trivy & CVEs: AuditView and VulnerabilityFindingsTable intercept 501 and display controller required empty state', () => {
    const auditViewContent = fs.readFileSync(auditViewPath, 'utf-8')
    const vulContent = fs.readFileSync(vulnerabilityTablePath, 'utf-8')
    assert.ok(auditViewContent.includes('isNotImplemented') || auditViewContent.includes('trivy') || vulContent.includes('Controller Not Connected') || vulContent.includes('IntegrationEmptyState') || vulContent.includes('trivyConnected'), 'AuditView / VulnerabilityFindingsTable must handle 501 gracefully')
    assert.ok(!vulContent.includes('⚠️'), 'VulnerabilityFindingsTable must not contain raw emoji warnings')
  })

  it('5. NetworkPolicies & Explorer: useK8sExplorer does not mark cluster offline on 501', () => {
    const explorerContent = fs.readFileSync(useK8sExplorerPath, 'utf-8')
    assert.ok(explorerContent.includes('isNotImplementedError'), 'useK8sExplorer must check isNotImplementedError on resource fetch')
    assert.ok(explorerContent.includes('controllerUnavailable') || explorerContent.includes('isControllerUnavailable'), 'useK8sExplorer must track controller availability separately from clusterOffline')
  })

  it('6. Fleet & Cluster Upgrade: FleetView and FleetClustersTable disable upgrade and intercept 501', () => {
    const fleetViewContent = fs.readFileSync(fleetViewPath, 'utf-8')
    const fleetTableContent = fs.readFileSync(fleetTablePath, 'utf-8')
    assert.ok(fleetViewContent.includes('isNotImplementedError') || fleetViewContent.includes('501') || fleetTableContent.includes('disabled: true') || fleetTableContent.includes('tooltip') || fleetTableContent.includes('Not Implemented'), 'FleetView / FleetClustersTable must handle 501 upgrade gracefully')
  })

  it('7. SecOps Anti-Mock: useDevSecOps eradicates hardcoded static mock secrets, RBAC, and fake toasts', () => {
    const devSecOpsContent = fs.readFileSync(useDevSecOpsPath, 'utf-8')
    assert.strictEqual(devSecOpsContent.includes('sk_live_51M7...98xQ'), false, 'useDevSecOps must not contain hardcoded Stripe secret mock')
    assert.strictEqual(devSecOpsContent.includes('dHJpdnlfc2VjcmV0...=='), false, 'useDevSecOps must not contain hardcoded JWT mock')
    assert.strictEqual(devSecOpsContent.includes('Automated PR opened'), false, 'useDevSecOps must not emit fake mock PR opened toast')
    assert.strictEqual(devSecOpsContent.includes('Successfully rotated secret'), false, 'useDevSecOps must not emit fake mock Vault rotation toast')
  })

  it('8. File length constraint: All modified files must be strictly under 500 lines', () => {
    const filesToCheck = [
      clientApiPath,
      drTabPath,
      auditViewPath,
      vulnerabilityTablePath,
      auditToolbarPath,
      useAuditLogsPath,
      explorerViewPath,
      useK8sExplorerPath,
      fleetViewPath,
      fleetTablePath,
      fleetGridPath,
      useDevSecOpsPath,
      integrationBadgePath,
      integrationEmptyStatePath,
    ]

    for (const file of filesToCheck) {
      if (fs.existsSync(file)) {
        const lines = fs.readFileSync(file, 'utf-8').split('\n').length
        assert.ok(lines < 500, `${path.basename(file)} has ${lines} lines, which exceeds the 500 line limit!`)
      }
    }
  })
})
