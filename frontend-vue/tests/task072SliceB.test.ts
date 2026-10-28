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
const { resolveHostHardwareMetrics, computeFleetHardwareAggregates } = await import('../src/types/hosts.ts')
const { deduplicateCharts } = await import('../src/composables/useHelmCatalog.ts')
const { getChartIcon, REPO_PRESETS, CATEGORY_TAGS, buildInstallPayload } = await import('../src/composables/helmCatalogConstants.ts')
const { formatContainerName, formatImageName } = await import('../src/utils/dockerFormat.ts')

describe('Task 072 — Subtask 72B Behavioral Regression Suite (LES-012)', () => {
  describe('Criterion 1: Workload Manifest Generation Behavioral Truthfulness', () => {
    it('generates systemd unit specification for daemon workloads', () => {
      const daemonApp = {
        name: 'k8s-agent',
        type: 'daemon' as const,
        namespace: 'edge-host-01',
        port: 9100,
        env: 'production',
        image: 'local-binary',
        replicas: 1,
        target: 'local-host',
        cpu: '150m',
        memory: '256Mi'
      }
      const manifest = generateYamlManifest(daemonApp as any)
      assert.match(manifest, /\[Unit\]/)
      assert.match(manifest, /Description=.*Daemon Service/)
      assert.match(manifest, /\[Service\]/)
      assert.match(manifest, /ExecStart=\/usr\/local\/bin\/k8s-agent/)
      assert.match(manifest, /Restart=always/)
      assert.match(manifest, /\[Install\]/)
      assert.match(manifest, /WantedBy=multi-user\.target/)
      assert.doesNotMatch(manifest, /apiVersion:\s*apps\/v1/)
    })

    it('generates Kubernetes Deployment YAML for standard pod workloads', () => {
      const k8sApp = {
        name: 'frontend-nginx',
        type: 'deployment' as const,
        namespace: 'production',
        port: 80,
        env: 'prod',
        image: 'nginx:1.25-alpine',
        replicas: 3,
        target: 'cluster-a',
        cpu: '500m',
        memory: '512Mi'
      }
      const manifest = generateYamlManifest(k8sApp as any)
      assert.match(manifest, /apiVersion:\s*apps\/v1/)
      assert.match(manifest, /kind:\s*Deployment/)
      assert.match(manifest, /metadata:\s*[\s\S]*name:\s*frontend-nginx/)
      assert.match(manifest, /namespace:\s*production/)
      assert.match(manifest, /replicas:\s*3/)
      assert.match(manifest, /image:\s*nginx:1\.25-alpine/)
      assert.doesNotMatch(manifest, /\[Unit\]/)
    })
  })

  describe('Criterion 2: Workload Replica & Container Formatting Truthfulness', () => {
    it('returns truthful unit label for daemon apps', () => {
      const activeDaemon = {
        name: 'telemetry-agent',
        type: 'daemon' as const,
        namespace: 'node-01',
        image: 'telemetry:1.0',
        status: 'healthy' as const,
        replicas: 1,
        readyReplicas: 1
      }
      const activeCells = computeReplicaCells(activeDaemon)
      assert.equal(activeCells.label, '1 / 1 Unit Up')
      assert.equal(activeCells.blocks[0].state, 'online')

      const stoppedDaemon = {
        ...activeDaemon,
        status: 'degraded' as const,
        readyReplicas: 0
      }
      const stoppedCells = computeReplicaCells(stoppedDaemon)
      assert.equal(stoppedCells.label, '0 / 1 Unit Down')
      assert.equal(stoppedCells.blocks[0].state, 'offline')
    })

    it('formats container names and avoids docker :latest on systemd services', () => {
      const formatted = formatContainerName('k8s-agent.service')
      assert.equal(formatted.serviceName, 'k8s-agent.service')
      assert.equal(formatted.fullName, 'k8s-agent.service')

      const img = formatImageName('redis:7-alpine')
      assert.equal(img.display, 'redis:7-alpine')
      assert.equal(img.baseImage, 'redis:7-alpine')
    })
  })

  describe('Criterion 3: Host Blade Hardware Calculations', () => {
    it('computes hardware utilization and memory ratios correctly', () => {
      const host = {
        id: 'host-1',
        name: 'edge-node-alpha',
        endpoint: 'http://192.168.1.10:9100',
        host_type: 'agent' as const,
        tls_enabled: false,
        status: 'connected' as const,
        labels: {},
        created_at: '2026-10-20T00:00:00Z'
      }
      const metric = {
        node_id: 'host-1',
        node_name: 'edge-node-alpha',
        role: 'worker',
        status: 'ready',
        cpu_percent: 45.2,
        load_average: [0.85, 0.92, 0.78],
        memory_used: 23512399872,
        memory_total: 34359738368,
        memory_percent: 68.4,
        disk_used: 55000000000,
        disk_total: 100000000000,
        disk_percent: 55.0
      }
      const metrics = resolveHostHardwareMetrics(host as any, metric as any)
      assert.equal(metrics.cpu_percent, 45.2)
      assert.equal(metrics.memory_percent, 68.4)
      assert.deepEqual(metrics.load_average, [0.85, 0.92, 0.78])
      assert.equal(metrics.is_live, true)
    })

    it('aggregates fleet hardware totals accurately', () => {
      const hosts = [
        {
          id: 'h1',
          name: 'blade-1',
          endpoint: 'http://192.168.1.1:9100',
          host_type: 'agent' as const,
          tls_enabled: false,
          status: 'connected' as const,
          labels: {},
          created_at: '2026-10-20T00:00:00Z'
        },
        {
          id: 'h2',
          name: 'blade-2',
          endpoint: 'http://192.168.1.2:9100',
          host_type: 'agent' as const,
          tls_enabled: false,
          status: 'disconnected' as const,
          labels: {},
          created_at: '2026-10-20T00:00:00Z'
        }
      ]
      const hwMap = {
        h1: resolveHostHardwareMetrics(hosts[0] as any, {
          node_id: 'h1',
          node_name: 'blade-1',
          role: 'manager',
          status: 'ready',
          cpu_percent: 20,
          memory_percent: 40,
          memory_used: 6871947673,
          memory_total: 17179869184,
          disk_percent: 30
        } as any),
        h2: resolveHostHardwareMetrics(hosts[1] as any, undefined)
      }
      const aggregates = computeFleetHardwareAggregates(hosts as any, hwMap)
      assert.equal(aggregates.totalBladeCount, 2)
      assert.equal(aggregates.onlineBladeCount, 1)
      assert.equal(aggregates.fleetCpuAvgPct, 20)
    })
  })

  describe('Criterion 4: Helm Client Deduplication and Icon Sanitization', () => {
    it('deduplicates releases keeping the first occurrence per repo:name', () => {
      const charts = [
        { name: 'ingress-nginx', repo: 'bitnami', version: '4.8.0', description: 'Ingress' },
        { name: 'ingress-nginx', repo: 'bitnami', version: '4.7.1', description: 'Old Ingress' },
        { name: 'ingress-nginx', repo: 'kubernetes', version: '4.8.0', description: 'K8s Ingress' }
      ]
      const deduped = deduplicateCharts(charts as any)
      assert.equal(deduped.length, 2)
      assert.equal(deduped[0].version, '4.8.0')
      assert.equal(deduped[1].repo, 'kubernetes')
    })

    it('ensures all Helm icons and presets use BaseIcon keys with zero emojis', () => {
      for (const preset of REPO_PRESETS) {
        assert.ok(!/[\u{1F300}-\u{1FAFF}]/u.test(preset.name), `Preset ${preset.name} contains emoji`)
      }
      for (const cat of CATEGORY_TAGS) {
        assert.ok(!/[\u{1F300}-\u{1FAFF}]/u.test(cat.label), `Category label ${cat.label} contains emoji`)
        assert.ok(!/[\u{1F300}-\u{1FAFF}]/u.test(cat.icon), `Category icon ${cat.icon} contains emoji`)
        assert.ok(!/[\u{1F300}-\u{1FAFF}]/u.test(getChartIcon([cat.id])), `Chart icon for ${cat.id} contains emoji`)
      }
    })

    it('builds valid install payloads for Helm deployment', () => {
      const chart = { name: 'redis', repo: 'bitnami', version: '17.0.1' }
      const form = {
        releaseName: 'prod-redis',
        namespace: 'databases',
        createNamespace: true,
        version: '17.0.1',
        values: 'replicaCount: 3\n',
        loadingDefaultValues: false
      }
      const payload = buildInstallPayload(chart as any, form)
      assert.equal(payload.chart, 'redis')
      assert.equal(payload.releaseName, 'prod-redis')
      assert.equal(payload.namespace, 'databases')
      assert.equal(payload.values, 'replicaCount: 3\n')
    })
  })
})
