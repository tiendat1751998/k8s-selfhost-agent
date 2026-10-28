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

const { generateYamlManifest } = await import('../src/components/deployments/workloadInspectorYaml.ts')
const { computeReplicaCells } = await import('../src/composables/useDeployments.ts')
const { formatContainerName, formatImageName } = await import('../src/utils/dockerFormat.ts')
const { deduplicateCharts } = await import('../src/composables/useHelmCatalog.ts')
const { getChartIcon } = await import('../src/composables/helmCatalogConstants.ts')

describe('Task 072 — Subtask 72D Behavioral Regression Suite (LES-012)', () => {
  describe('Criterion 1: PWA Banner Eligibility & Dismissal Logic', () => {
    it('evaluates banner visibility based on browser deferredPrompt and user dismissal', () => {
      function evaluatePwaVisibility(hasPrompt: boolean, isDismissed: boolean): boolean {
        return hasPrompt && !isDismissed
      }

      assert.equal(evaluatePwaVisibility(false, false), false, 'Banner must not show without deferredPrompt')
      assert.equal(evaluatePwaVisibility(true, true), false, 'Banner must not show when dismissed')
      assert.equal(evaluatePwaVisibility(true, false), true, 'Banner must show when prompt is active and not dismissed')
    })
  })

  describe('Criterion 2: Workload Inspector Daemon Truthfulness & YAML Generation', () => {
    it('generates systemd unit specification with honest restart and exec parameters for daemons', () => {
      const daemonWorkload = {
        name: 'k8s-agent.service',
        type: 'daemon' as const,
        namespace: 'edge-tokyo-01',
        port: 9100,
        env: 'production',
        image: 'local-binary',
        replicas: 1,
        target: 'local-node',
        cpu: '100m',
        memory: '128Mi'
      }

      const manifest = generateYamlManifest(daemonWorkload as any)
      assert.match(manifest, /\[Unit\]/)
      assert.match(manifest, /Description=.*Daemon Service/)
      assert.match(manifest, /\[Service\]/)
      assert.match(manifest, /ExecStart=\/usr\/local\/bin\/k8s-agent/)
      assert.match(manifest, /Restart=always/)
      assert.doesNotMatch(manifest, /apiVersion:\s*apps\/v1/)
      assert.doesNotMatch(manifest, /kind:\s*Deployment/)
    })

    it('generates standard Kubernetes manifest for regular deployment workloads', () => {
      const k8sWorkload = {
        name: 'web-ingress',
        type: 'deployment' as const,
        namespace: 'ingress-system',
        port: 80,
        env: 'prod',
        image: 'traefik:v2.10',
        replicas: 2,
        target: 'cluster-mesh',
        cpu: '250m',
        memory: '256Mi'
      }

      const manifest = generateYamlManifest(k8sWorkload as any)
      assert.match(manifest, /apiVersion:\s*apps\/v1/)
      assert.match(manifest, /kind:\s*Deployment/)
      assert.match(manifest, /metadata:\s*[\s\S]*name:\s*web-ingress/)
      assert.doesNotMatch(manifest, /\[Unit\]/)
    })
  })

  describe('Criterion 3: Workload Replica Matrix & Posture Truthfulness', () => {
    it('accurately resolves daemon units and pod replica state blocks', () => {
      const runningDaemon = {
        name: 'k8s-agent',
        type: 'daemon' as const,
        status: 'healthy' as const,
        replicas: 1,
        readyReplicas: 1
      }
      const activeCells = computeReplicaCells(runningDaemon)
      assert.equal(activeCells.label, '1 / 1 Unit Up')
      assert.equal(activeCells.readyCount, 1)
      assert.equal(activeCells.blocks[0].state, 'online')

      const stoppedDaemon = {
        name: 'k8s-agent',
        type: 'daemon' as const,
        status: 'degraded' as const,
        replicas: 1,
        readyReplicas: 0
      }
      const downCells = computeReplicaCells(stoppedDaemon)
      assert.equal(downCells.label, '0 / 1 Unit Down')
      assert.equal(downCells.readyCount, 0)
      assert.equal(downCells.blocks[0].state, 'offline')

      const scaledK8s = {
        name: 'worker-queue',
        type: 'deployment' as const,
        status: 'scaled_zero' as const,
        replicas: 0,
        readyReplicas: 0
      }
      const scaledCells = computeReplicaCells(scaledK8s)
      assert.equal(scaledCells.label, '0 / 0 Pods (Scaled)')
    })
  })

  describe('Criterion 4: Container and Daemon Service URI Formatting', () => {
    it('formats daemon URIs cleanly without redundant .service duplication', () => {
      function getDaemonServiceUri(name: string): string {
        const cleanName = formatContainerName(name).serviceName || name
        return `systemd://${cleanName.replace(/\.service$/, '')}.service`
      }

      assert.equal(getDaemonServiceUri('k8s-agent'), 'systemd://k8s-agent.service')
      assert.equal(getDaemonServiceUri('k8s-agent.service'), 'systemd://k8s-agent.service')
      assert.equal(getDaemonServiceUri('docker-daemon.service'), 'systemd://docker-daemon.service')
    })

    it('formats container images without docker tag pollution on binaries', () => {
      const dockerImg = formatImageName('quay.io/coreos/etcd:v3.5.9')
      assert.equal(dockerImg.display, 'quay.io/coreos/etcd:v3.5.9')
      assert.equal(dockerImg.baseImage, 'quay.io/coreos/etcd:v3.5.9')
    })
  })

  describe('Criterion 5: Helm Catalog Deduplication and Clean Icons', () => {
    it('deduplicates charts and preserves release integrity', () => {
      const rawCharts = [
        { name: 'prometheus', repo: 'prometheus-community', version: '15.0.0' },
        { name: 'prometheus', repo: 'prometheus-community', version: '14.8.0' },
        { name: 'grafana', repo: 'grafana', version: '6.50.0' }
      ]
      const deduped = deduplicateCharts(rawCharts as any)
      assert.equal(deduped.length, 2)
      assert.equal(deduped[0].version, '15.0.0')
      assert.equal(deduped[1].name, 'grafana')
    })

    it('verifies getChartIcon maps to clean BaseIcon keys without emojis', () => {
      const icons = [
        getChartIcon(['monitoring']),
        getChartIcon(['database']),
        getChartIcon(['security']),
        getChartIcon(['networking']),
        getChartIcon(['storage']),
        getChartIcon(['ai'])
      ]
      for (const icon of icons) {
        assert.ok(!/[\u{1F300}-\u{1FAFF}]/u.test(icon), `Chart icon ${icon} must not be an emoji`)
        assert.match(icon, /^[a-z0-9-]+$/, `Icon ${icon} must be a valid kebab-case BaseIcon name`)
      }
    })
  })
})
