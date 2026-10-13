import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Task 065: Dead Node Recovery & Zero-Data-Loss Storage Fencing UI', () => {
  const drApiPath = path.join(rootDir, 'src', 'api', 'dr.ts')
  const modalComponentPath = path.join(rootDir, 'src', 'components', 'overview', 'drawer', 'NodeStorageFencingModal.vue')
  const cssPath = path.join(rootDir, 'src', 'assets', 'styles', 'components', 'node-fencing.css')
  const infraHostsViewPath = path.join(rootDir, 'src', 'views', 'InfraHostsView.vue')

  it('1. dr.ts exports getFencingPreflight, executeNodeFencing and required types', () => {
    assert.ok(fs.existsSync(drApiPath), 'dr.ts must exist')
    const content = fs.readFileSync(drApiPath, 'utf-8')
    assert.ok(content.includes('export interface NodeFencingPreflight'), 'must export NodeFencingPreflight')
    assert.ok(content.includes('export interface FencingExecutionPayload'), 'must export FencingExecutionPayload')
    assert.ok(content.includes('export interface FencingExecutionReport'), 'must export FencingExecutionReport')
    assert.ok(content.includes('export async function getFencingPreflight'), 'must export getFencingPreflight')
    assert.ok(content.includes('export async function executeNodeFencing'), 'must export executeNodeFencing')
    assert.ok(content.includes('is_single_worker_cluster'), 'preflight must contain is_single_worker_cluster')
    assert.ok(content.includes('confirm_power_isolated'), 'payload must contain confirm_power_isolated')
  })

  it('2. NodeStorageFencingModal.vue exists, meets 5 stages and stays under 450 lines', () => {
    assert.ok(fs.existsSync(modalComponentPath), 'NodeStorageFencingModal.vue must exist')
    const content = fs.readFileSync(modalComponentPath, 'utf-8')
    const lines = content.split('\n').length
    assert.ok(lines < 450, `NodeStorageFencingModal.vue must be < 450 lines (got ${lines})`)

    // Stage 1 check
    assert.ok(content.includes('Probable Cause') || content.includes('probable_cause') || content.includes('RCA'), 'Must show RCA analysis')
    assert.ok(content.includes('is_single_worker_cluster') || content.includes('Single Worker Cluster'), 'Must handle 1M+1W single worker topology alert')
    assert.ok(content.includes('Emergency Untaint Master') || content.includes('emergency_untaint_master'), 'Must have emergency untaint master toggle')

    // Stage 2 check: STONITH
    assert.ok(content.includes('confirm_power_isolated') || content.includes('confirmPowerIsolated'), 'Must have STONITH power isolation confirmation')

    // Stage 3 check: Stepper
    assert.ok(content.includes('out-of-service') || content.includes('Cordon') || content.includes('VolumeAttachment'), 'Must have fencing stepper')

    // Stage 4 check: Pod Eviction
    assert.ok(content.includes('evict') || content.includes('Eviction'), 'Must display pod eviction stage')

    // Stage 5 check: Post-Recovery Summary
    assert.ok(content.includes('Copy RCA Report') || content.includes('copyReport'), 'Must have copy RCA report action')
    // Reviewer Fix: Anti-trapped UI resets stage.value = 2 on error so confirmation screen remains visible
    assert.ok(content.includes('stage.value = 2'), 'Must reset stage.value = 2 in error catch block')
  })

  it('3. node-fencing.css exists and stays under 350 lines', () => {
    assert.ok(fs.existsSync(cssPath), 'node-fencing.css must exist')
    const content = fs.readFileSync(cssPath, 'utf-8')
    const lines = content.split('\n').length
    assert.ok(lines < 350, `node-fencing.css must be < 350 lines (got ${lines})`)
  })

  it('4. InfraHostsView.vue integrates NodeStorageFencingModal and stays under 500 lines', () => {
    assert.ok(fs.existsSync(infraHostsViewPath), 'InfraHostsView.vue must exist')
    const content = fs.readFileSync(infraHostsViewPath, 'utf-8')
    const lines = content.split('\n').length
    assert.ok(lines < 500, `InfraHostsView.vue must be < 500 lines (got ${lines})`)
    assert.ok(content.includes('NodeStorageFencingModal'), 'InfraHostsView.vue must import/use NodeStorageFencingModal')
    assert.ok(content.includes('Safe Remediation') || content.includes('Fence'), 'InfraHostsView.vue must have Safe Remediation / Fence action')

    // Reviewer Fix: No hardcoded 'default' in mobile micro-telemetry fencing trigger
    assert.ok(
      content.includes("hosts.find(h => h.status !== 'connected')?.tenant_id || 'default'"),
      "InfraHostsView must resolve tenant_id dynamically for fencing modal"
    )
  })

  it('5. dr.ts executeNodeRescue uses clean route /k8s/{clusterId}/dr/nodes/{nodeName}/rescue without api/v1 prefix', () => {
    const content = fs.readFileSync(drApiPath, 'utf-8')
    assert.ok(
      content.includes('`/k8s/${cid}/dr/nodes/${node}/rescue`') || content.includes('`/k8s/${clusterId}/dr/nodes/${nodeName}/rescue`'),
      'executeNodeRescue must use /k8s/${clusterId}/dr/nodes/${nodeName}/rescue'
    )
    assert.ok(!content.includes('/api/v1/clusters/'), 'executeNodeRescue must NOT have /api/v1 prefix')
  })

  it('6. dr.ts getFencingPreflight normalizes probe_report, dynamic_rca, capacity_plan, circuit_breaker', () => {
    const content = fs.readFileSync(drApiPath, 'utf-8')
    // probe_report normalization
    assert.ok(content.includes('classification'), 'probe_report must map state/classification')
    assert.ok(content.includes('ping_reachable'), 'probe_report must map ping_reachable')
    assert.ok(content.includes('kubelet_responsive'), 'probe_report must map kubelet_responsive')
    assert.ok(content.includes('containerd_responsive'), 'probe_report must map containerd_responsive')

    // capacity_plan normalization
    assert.ok(content.includes('headroom_ram_percent'), 'capacity_plan must compute headroom_ram_percent')
    assert.ok(content.includes('headroom_cpu_percent'), 'capacity_plan must compute headroom_cpu_percent')

    // circuit_breaker normalization
    assert.ok(content.includes('eviction_frozen'), 'circuit_breaker must map eviction_frozen')
    assert.ok(content.includes('freeze_reason'), 'circuit_breaker must map freeze_reason')
  })

  it('7. NodeFencingRescueSection.vue handleRescue checks cancel_fencing, success, recovered_to_ready', () => {
    const rescueComponentPath = path.join(rootDir, 'src', 'components', 'overview', 'drawer', 'NodeFencingRescueSection.vue')
    assert.ok(fs.existsSync(rescueComponentPath), 'NodeFencingRescueSection.vue must exist')
    const content = fs.readFileSync(rescueComponentPath, 'utf-8')
    assert.ok(
      content.includes('cancel_fencing'),
      'handleRescue must check cancel_fencing'
    )
    assert.ok(
      content.includes('res.cancel_fencing || res.success || res.recovered_to_ready') ||
      content.includes('cancel_fencing || res.success || res.recovered_to_ready'),
      'handleRescue must check cancel_fencing || res.success || res.recovered_to_ready'
    )
  })
})
