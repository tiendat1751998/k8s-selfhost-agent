import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Frontend Client-Side Security & Performance Optimization Specification', () => {
  const tokenValidatorPath = path.join(rootDir, 'src', 'router', 'tokenValidator.ts')
  const routerPath = path.join(rootDir, 'src', 'router', 'index.ts')
  const useInfraHostsPath = path.join(rootDir, 'src', 'composables', 'useInfraHosts.ts')
  const useTenancyRbacPath = path.join(rootDir, 'src', 'composables', 'useTenancyRbac.ts')
  const useSettingsPath = path.join(rootDir, 'src', 'composables', 'useSettings.ts')
  const useDeploymentsPath = path.join(rootDir, 'src', 'composables', 'useDeployments.ts')
  const useK8sExplorerPath = path.join(rootDir, 'src', 'composables', 'useK8sExplorer.ts')
  const useTOTPSetupPath = path.join(rootDir, 'src', 'composables', 'useTOTPSetup.ts')
  const useAgentMeshPath = path.join(rootDir, 'src', 'composables', 'useAgentMesh.ts')
  const fleetViewPath = path.join(rootDir, 'src', 'views', 'FleetView.vue')
  const infraHostsViewPath = path.join(rootDir, 'src', 'views', 'InfraHostsView.vue')
  const incidentsViewPath = path.join(rootDir, 'src', 'views', 'IncidentsView.vue')
  const dataTablePath = path.join(rootDir, 'src', 'components', 'ui', 'DataTable.vue')
  const explorerTablePath = path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerResourceTable.vue')
  const auditTablePath = path.join(rootDir, 'src', 'components', 'audit', 'AuditTrailTable.vue')
  const topologySectionPath = path.join(rootDir, 'src', 'components', 'overview', 'OverviewTopologySection.vue')
  const canvasTimeSeriesPath = path.join(rootDir, 'src', 'components', 'telemetry', 'CanvasTimeSeries.vue')
  const densityMatrixPath = path.join(rootDir, 'src', 'components', 'overview', 'matrix', 'ClusterDensityMatrix.vue')

  it('1. SEC-14: tokenValidator accepts platform_admin and router navigation guard prevents localStorage role escalation', async () => {
    const { isPrivilegedAdminRole } = await import('../src/router/tokenValidator.ts')
    assert.strictEqual(isPrivilegedAdminRole('platform_admin'), true, 'platform_admin must be recognized as privileged admin')
    assert.strictEqual(isPrivilegedAdminRole('PLATFORM_ADMIN'), true, 'platform_admin case-insensitive must match')

    const routerContent = fs.readFileSync(routerPath, 'utf-8')
    // Guard must NOT overwrite claims.role when claims.role is present
    assert.ok(
      routerContent.includes('!activeRole') || routerContent.includes('claims.role ||'),
      'Router must use claims.role from JWT as authoritative and not overwrite it with localStorage'
    )
    assert.strictEqual(
      routerContent.includes('if (parsed.role) activeRole = parsed.role') && !routerContent.includes('!activeRole'),
      false,
      'Router must not unconditionally override activeRole from localStorage if claims.role exists'
    )
  })

  it('2. PERF-14 & PERF-18: timer cleanup in useInfraHosts and feedback timers with onUnmounted across composables & views', () => {
    const infraContent = fs.readFileSync(useInfraHostsPath, 'utf-8')
    assert.ok(
      infraContent.includes('finally {') && infraContent.includes('clearTimeout(timer)'),
      'useInfraHosts probe fetch must clear timer inside a finally block'
    )
    assert.ok(
      infraContent.includes('onUnmounted') && (infraContent.includes('toastTimer') || infraContent.includes('clearTimeout')),
      'useInfraHosts must clean up toast timer in onUnmounted'
    )

    const tenancyContent = fs.readFileSync(useTenancyRbacPath, 'utf-8')
    assert.ok(
      tenancyContent.includes('onUnmounted') && tenancyContent.includes('feedbackTimer'),
      'useTenancyRbac must clean up feedbackTimer in onUnmounted'
    )

    const settingsContent = fs.readFileSync(useSettingsPath, 'utf-8')
    assert.ok(
      settingsContent.includes('onUnmounted') && settingsContent.includes('messageTimer'),
      'useSettings must clean up messageTimer in onUnmounted'
    )

    const deploymentsContent = fs.readFileSync(useDeploymentsPath, 'utf-8')
    assert.ok(
      deploymentsContent.includes('onUnmounted') && deploymentsContent.includes('toastTimer'),
      'useDeployments must track toast timer and clear in onUnmounted'
    )

    const fleetContent = fs.readFileSync(fleetViewPath, 'utf-8')
    assert.ok(
      fleetContent.includes('onUnmounted') && fleetContent.includes('toastTimer'),
      'FleetView must track toast timer and clear in onUnmounted'
    )

    const explorerContent = fs.readFileSync(useK8sExplorerPath, 'utf-8')
    assert.ok(
      explorerContent.includes('onUnmounted') && explorerContent.includes('toastTimer'),
      'useK8sExplorer must track toast timer and clear in onUnmounted'
    )

    const totpContent = fs.readFileSync(useTOTPSetupPath, 'utf-8')
    assert.ok(
      totpContent.includes('onUnmounted') && (totpContent.includes('secretCopiedTimer') || totpContent.includes('allCodesCopiedTimer') || totpContent.includes('clearTimeout')),
      'useTOTPSetup must clean up copy feedback timers in onUnmounted'
    )
  })

  it('3. PERF-15 & Orphan Wiring: DataTable row keys, capped lists, and VirtualNodeGrid wiring', () => {
    const dataTableContent = fs.readFileSync(dataTablePath, 'utf-8')
    assert.strictEqual(
      dataTableContent.includes(':key="idx"'),
      false,
      'DataTable row v-for must use a stable key instead of raw index :key="idx"'
    )
    assert.ok(
      dataTableContent.includes('getRowKey') || dataTableContent.includes('row.id') || dataTableContent.includes('row.name'),
      'DataTable must use stable row key logic'
    )

    const explorerContent = fs.readFileSync(explorerTablePath, 'utf-8')
    assert.ok(
      explorerContent.includes('slice(0, 100)') || explorerContent.includes('cappedResources') || explorerContent.includes('props.resources.slice'),
      'ExplorerResourceTable must cap rendered items to avoid freezing on 1,000+ resources'
    )

    const auditContent = fs.readFileSync(auditTablePath, 'utf-8')
    assert.ok(
      auditContent.includes('slice(0, 100)') || auditContent.includes('cappedLogs') || auditContent.includes('props.logs.slice'),
      'AuditTrailTable must cap rendered items to avoid freezing on high-cardinality logs'
    )

    const incidentsContent = fs.readFileSync(incidentsViewPath, 'utf-8')
    assert.ok(
      incidentsContent.includes('slice(0, 100)') || incidentsContent.includes('cappedIncidents') || incidentsContent.includes('filteredIncidents.slice'),
      'IncidentsView must cap rendered DOM incident cards'
    )

    const topologyContent = fs.readFileSync(topologySectionPath, 'utf-8')
    assert.ok(
      topologyContent.includes('VirtualNodeGrid'),
      'OverviewTopologySection must wire VirtualNodeGrid component'
    )
  })

  it('4. PERF-16: InfraHostsView unscoped css import and child components duplicate css elimination', () => {
    const viewContent = fs.readFileSync(infraHostsViewPath, 'utf-8')
    assert.ok(
      viewContent.includes("<style>\n@import '../assets/styles/views/infra-hosts.css';\n</style>") ||
      viewContent.includes("<style>\r\n@import '../assets/styles/views/infra-hosts.css';\r\n</style>") ||
      viewContent.includes("import '../assets/styles/views/infra-hosts.css'") ||
      (viewContent.includes('<style>') && viewContent.includes('infra-hosts.css')),
      'InfraHostsView must import infra-hosts.css unscoped so child components inherit classes without duplication'
    )

    const hostsDir = path.join(rootDir, 'src', 'components', 'hosts')
    const hostFiles = fs.readdirSync(hostsDir).filter(f => f.endsWith('.vue'))
    for (const f of hostFiles) {
      const content = fs.readFileSync(path.join(hostsDir, f), 'utf-8')
      assert.strictEqual(
        content.includes("@import '../../assets/styles/views/infra-hosts.css';"),
        false,
        `${f} must not duplicate @import of infra-hosts.css in its scoped style block`
      )
    }
  })

  it('5. PERF-17: useAgentMesh terminalLogs watcher and Canvas/Matrix mousemove RAF throttling', () => {
    const meshContent = fs.readFileSync(useAgentMeshPath, 'utf-8')
    assert.strictEqual(
      meshContent.includes('watch(\n    terminalLogs,') || meshContent.includes('watch(\r\n    terminalLogs,'),
      false,
      'useAgentMesh must not watch terminalLogs with deep: true'
    )
    assert.ok(
      meshContent.includes('terminalLogs.value.length'),
      'useAgentMesh must watch () => terminalLogs.value.length'
    )

    const canvasContent = fs.readFileSync(canvasTimeSeriesPath, 'utf-8')
    assert.ok(
      canvasContent.includes('requestAnimationFrame') && canvasContent.includes('cancelAnimationFrame') && (canvasContent.includes('mouseMoveRafId') || canvasContent.includes('pendingMouse')),
      'CanvasTimeSeries must throttle mousemove with requestAnimationFrame and clean up on unmount'
    )

    const matrixContent = fs.readFileSync(densityMatrixPath, 'utf-8')
    assert.ok(
      matrixContent.includes('requestAnimationFrame') && matrixContent.includes('cancelAnimationFrame') && (matrixContent.includes('mouseMoveRafId') || matrixContent.includes('pendingMouseMove')),
      'ClusterDensityMatrix must throttle mousemove with requestAnimationFrame and clean up on unmount'
    )
  })

  it('6. Architectural Modularity & Line Count Constraints (< 500 lines per file, views <= 350, components < 300)', () => {
    const views = [fleetViewPath, infraHostsViewPath, incidentsViewPath]
    for (const v of views) {
      const lines = fs.readFileSync(v, 'utf-8').split('\n').length
      assert.ok(lines <= 350, `View ${path.basename(v)} (${lines} lines) must be <= 350 lines`)
    }

    const components = [
      dataTablePath,
      explorerTablePath,
      auditTablePath,
      topologySectionPath,
      canvasTimeSeriesPath,
      densityMatrixPath
    ]
    for (const c of components) {
      const lines = fs.readFileSync(c, 'utf-8').split('\n').length
      assert.ok(lines < 300, `Component ${path.basename(c)} (${lines} lines) must be < 300 lines`)
    }

    const composables = [
      useInfraHostsPath,
      useTenancyRbacPath,
      useSettingsPath,
      useDeploymentsPath,
      useK8sExplorerPath,
      useTOTPSetupPath,
      useAgentMeshPath
    ]
    for (const comp of composables) {
      const lines = fs.readFileSync(comp, 'utf-8').split('\n').length
      assert.ok(lines < 500, `Composable ${path.basename(comp)} (${lines} lines) must be < 500 lines`)
    }
  })
})
