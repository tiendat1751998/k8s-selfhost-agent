import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Task 015: Workload Logs Drawer & Log Stream Integration Specification', () => {
  const k8sTsPath = path.join(rootDir, 'src', 'api', 'k8s.ts')
  const workloadLogsTabPath = path.join(rootDir, 'src', 'components', 'deployments', 'WorkloadLogsTab.vue')
  const logStreamViewPath = path.join(rootDir, 'src', 'views', 'LogStreamView.vue')
  const logTargetTreePath = path.join(rootDir, 'src', 'components', 'logs', 'LogTargetTree.vue')
  const mockServerJsPath = path.join(rootDir, 'scripts', 'mock-preview-server.js')

  it('1. k8s.ts defines getWorkloadLogs API function', () => {
    assert.ok(fs.existsSync(k8sTsPath), 'k8s.ts must exist')
    const k8sContent = fs.readFileSync(k8sTsPath, 'utf-8')

    assert.match(k8sContent, /async\s+getWorkloadLogs\s*\(/, 'k8s.ts must define getWorkloadLogs method')
    assert.match(k8sContent, /\/workloads\//, 'getWorkloadLogs must call /workloads/ endpoint')
    assert.match(k8sContent, /pods:\s*string\[\]/, 'getWorkloadLogs return type must include pods array')
  })

  it('2. WorkloadLogsTab.vue uses getWorkloadLogs and eradicates fake fallback strings', () => {
    assert.ok(fs.existsSync(workloadLogsTabPath), 'WorkloadLogsTab.vue must exist')
    const tabContent = fs.readFileSync(workloadLogsTabPath, 'utf-8')

    // Must call getWorkloadLogs
    assert.match(tabContent, /getWorkloadLogs/, 'WorkloadLogsTab.vue must call k8sApi.getWorkloadLogs')

    // Must NOT contain hardcoded fake fallback strings
    assert.doesNotMatch(tabContent, /\[INFO\] Kubernetes Workload:/, 'Must eradicate fake fallback string [INFO] Kubernetes Workload:')
    assert.doesNotMatch(tabContent, /Direct pod log streaming: Open Live Logs Explorer/, 'Must eradicate fake string Direct pod log streaming')
    assert.doesNotMatch(tabContent, /Container log stream connected for/, 'Must eradicate fake string Container log stream connected for')

    // Must NOT do client-side listResources pods prefix matching
    assert.doesNotMatch(tabContent, /listResources\(\s*cluster\s*,\s*['"]pods['"]/, 'Must eliminate client-side listResources pods scanning')

    // Must provide Pod Selector with All Pods (Aggregated) (in tab or extracted toolbar)
    const toolbarPath = path.join(rootDir, 'src', 'components', 'deployments', 'WorkloadLogToolbar.vue')
    const toolbarContent = fs.existsSync(toolbarPath) ? fs.readFileSync(toolbarPath, 'utf-8') : ''
    assert.ok(
      tabContent.includes('All Pods (Aggregated)') || toolbarContent.includes('All Pods (Aggregated)'),
      'WorkloadLogsTab.vue or WorkloadLogToolbar.vue must provide All Pods (Aggregated) option'
    )

    // Must enforce strictly < 300 lines
    const lineCount = tabContent.split('\n').length
    assert.ok(lineCount < 300, `WorkloadLogsTab.vue length (${lineCount}) must be strictly < 300 lines`)
  })

  it('3. LogStreamView.vue binds route queries (app, pod, namespace) and stays <= 350 lines', () => {
    assert.ok(fs.existsSync(logStreamViewPath), 'LogStreamView.vue must exist')
    const viewContent = fs.readFileSync(logStreamViewPath, 'utf-8')

    assert.match(viewContent, /route\.query\.app/, 'LogStreamView.vue must inspect route.query.app')
    assert.match(viewContent, /route\.query\.pod/, 'LogStreamView.vue must inspect route.query.pod')
    assert.match(viewContent, /route\.query\.namespace/, 'LogStreamView.vue must inspect route.query.namespace')

    const lineCount = viewContent.split('\n').length
    assert.ok(lineCount <= 350, `LogStreamView.vue length (${lineCount}) must be strictly <= 350 lines`)
  })

  it('4. LogTargetTree.vue provides Kubernetes Workloads section and stays < 500 lines', () => {
    assert.ok(fs.existsSync(logTargetTreePath), 'LogTargetTree.vue must exist')
    const treeContent = fs.readFileSync(logTargetTreePath, 'utf-8')

    assert.match(treeContent, /Kubernetes Workloads/, 'LogTargetTree.vue must render Kubernetes Workloads section')

    const lineCount = treeContent.split('\n').length
    assert.ok(lineCount < 500, `LogTargetTree.vue length (${lineCount}) must be strictly < 500 lines`)
  })

  it('5. mock-preview-server.js is completely eradicated and k8s API defines workload endpoints', () => {
    assert.ok(!fs.existsSync(mockServerJsPath), 'mock-preview-server.js must be completely deleted to prevent fake telemetry')
    const k8sContent = fs.readFileSync(k8sTsPath, 'utf-8')
    assert.match(k8sContent, /getWorkloadLogs/, 'k8sApi must define authentic getWorkloadLogs')
  })
})
