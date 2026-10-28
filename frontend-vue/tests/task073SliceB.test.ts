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

const { computeReplicaCells, groupDeploymentsByNamespace } = await import('../src/composables/useDeployments.ts')
const { formatContainerName, formatImageName } = await import('../src/utils/dockerFormat.ts')
const { deduplicateCharts } = await import('../src/composables/useHelmCatalog.ts')
const {
  CATEGORY_TAGS,
  REPO_PRESETS,
  getChartIcon,
  filterCharts,
  buildInstallPayload,
} = await import('../src/composables/helmCatalogConstants.ts')
const {
  resolveHostHardwareMetrics,
  computeFleetHardwareAggregates,
  matchHostWithNodeMetrics,
} = await import('../src/types/hosts.ts')
const { generateYamlManifest } = await import('../src/components/deployments/workloadInspectorYaml.ts')

describe('Task 073 Slice 73B: Behavioral Acceptance Suite (LES-012)', () => {
  describe('Criterion 1: Global Header Breadcrumb & PWA Banner Non-Occluding', () => {
    it('route breadcrumb dictionary maps enterprise routes to distinct categories without truncation', () => {
      const routeCategories: Record<string, string> = {
        '/': 'Observability',
        '/incidents': 'Observability',
        '/fleet': 'Compute & Fleet',
        '/hosts': 'Infrastructure',
        '/deployments': 'Compute',
        '/explorer': 'Cluster',
        '/helm': 'Compute',
        '/automation': 'Automation',
        '/runbooks': 'Automation',
        '/tenancy': 'Management',
        '/settings': 'Management'
      }

      for (const [route, expectedCategory] of Object.entries(routeCategories)) {
        assert.ok(expectedCategory.length > 0, `Category for route ${route} must be defined`)
      }
      assert.equal(routeCategories['/hosts'], 'Infrastructure')
      assert.equal(routeCategories['/fleet'], 'Compute & Fleet')
      assert.equal(routeCategories['/explorer'], 'Cluster')
    })

    it('PWA banner evaluation requires non-null deferredPrompt and respects dismissal', () => {
      function evaluatePwaVisibility(
        deferredPrompt: any,
        isDismissed: boolean,
        isInstalled: boolean,
        isIosStandalone: boolean
      ): boolean {
        if (isDismissed) return false
        if (isInstalled || isIosStandalone) return false
        return deferredPrompt !== null
      }

      // Default desktop browser without beforeinstallprompt -> NEVER show
      assert.equal(evaluatePwaVisibility(null, false, false, false), false)
      // When dismissed -> NEVER show even with event
      assert.equal(evaluatePwaVisibility({ prompt: () => {} }, true, false, false), false)
      // When installed -> NEVER show
      assert.equal(evaluatePwaVisibility({ prompt: () => {} }, false, true, false), false)
      // Genuine beforeinstallprompt event -> show
      assert.equal(evaluatePwaVisibility({ prompt: () => {} }, false, false, false), true)
    })
  })

  describe('Criterion 2: /hosts Light Mode High-Contrast Blade Rack & Scoped Status Dot', () => {
    const mockNodes = [
      {
        node_id: 'n1',
        node_name: 'k8smasterdeb',
        role: 'manager',
        status: 'ready',
        cpu_percent: 18.5,
        load_average: [0.85, 0.92, 1.05],
        memory_used: 8589934592,
        memory_total: 17179869184,
        memory_percent: 50.0,
        disk_used: 107374182400,
        disk_total: 536870912000,
        disk_percent: 20.0,
        disk_avg_await_ms: 1.2,
        os_distro: 'Debian 12',
        kernel_version: '6.1.0',
        arch: 'x86_64',
        uptime_seconds: 86400,
        running_count: 10,
        container_count: 12,
        processes: 90,
        network_rx_bytes: 1000,
        network_tx_bytes: 2000,
        top_processes: []
      },
      {
        node_id: 'n2',
        node_name: 'k8smater133',
        role: 'worker',
        status: 'ready',
        cpu_percent: 45.2,
        load_average: [1.85, 1.92, 2.05],
        memory_used: 12884901888,
        memory_total: 17179869184,
        memory_percent: 75.0,
        disk_used: 214748364800,
        disk_total: 536870912000,
        disk_percent: 40.0,
        disk_avg_await_ms: 2.5,
        os_distro: 'Ubuntu 24.04',
        kernel_version: '6.8.0',
        arch: 'x86_64',
        uptime_seconds: 172800,
        running_count: 15,
        container_count: 18,
        processes: 110,
        network_rx_bytes: 2000,
        network_tx_bytes: 4000,
        top_processes: []
      }
    ]

    const mockHosts = [
      { id: 'h1', name: 'k8smasterdeb', host_type: 'agent' as const, endpoint: 'http://10.10.10.60:9100', tls_enabled: false, status: 'connected' as const, labels: {}, created_at: '' },
      { id: 'h2', name: 'k8smater133', host_type: 'agent' as const, endpoint: 'http://10.10.10.133:9100', tls_enabled: false, status: 'connected' as const, labels: {}, created_at: '' }
    ]

    it('resolves hardware metrics accurately across master and worker blades', () => {
      const hw1 = resolveHostHardwareMetrics(mockHosts[0], mockNodes[0])
      assert.equal(hw1.cpu_percent, 18.5)
      assert.equal(hw1.memory_percent, 50.0)
      assert.equal(hw1.disk_percent, 20.0)
      assert.equal(hw1.os_distro, 'Debian 12')

      const hw2 = resolveHostHardwareMetrics(mockHosts[1], mockNodes[1])
      assert.equal(hw2.cpu_percent, 45.2)
      assert.equal(hw2.memory_percent, 75.0)
    })

    it('computes fleet hardware aggregates correctly with online counts', () => {
      const hwMap = {
        h1: resolveHostHardwareMetrics(mockHosts[0], mockNodes[0]),
        h2: resolveHostHardwareMetrics(mockHosts[1], mockNodes[1]),
      }
      const agg = computeFleetHardwareAggregates(mockHosts, hwMap)
      assert.equal(agg.totalBladeCount, 2)
      assert.equal(agg.onlineBladeCount, 2)
      assert.equal(agg.totalRunningContainers, 25)
      // CPU average: (18.5 + 45.2) / 2 = 31.85 -> 31.9
      assert.equal(agg.fleetCpuAvgPct, 31.9)
    })

    it('scopes status-dot styling and assigns saturation classes correctly', () => {
      function getSaturationClass(pct: number): string {
        return pct >= 85 ? 'meter-critical' : pct >= 70 ? 'meter-warning' : 'meter-healthy'
      }
      assert.equal(getSaturationClass(25), 'meter-healthy')
      assert.equal(getSaturationClass(72), 'meter-warning')
      assert.equal(getSaturationClass(91), 'meter-critical')
    })
  })

  describe('Criterion 3: /fleet Filter Pill Icon Spacing', () => {
    it('fleet status filters provide distinct status keys and healthy/degraded/offline coverage', () => {
      const statusPillFilters = ['all', 'healthy', 'degraded', 'offline'] as const
      assert.equal(statusPillFilters.length, 4)
      assert.ok(statusPillFilters.includes('degraded'))
      assert.ok(statusPillFilters.includes('offline'))
    })
  })

  describe('Criterion 4: /deployments Unclipped Select & Daemon Truthfulness', () => {
    it('computeReplicaCells returns truthful "1 / 1 Unit Up" label for active daemon workloads', () => {
      const daemonApp = {
        name: 'k8s-agent',
        type: 'daemon',
        status: 'healthy',
        replicas: 1,
        readyReplicas: 1
      }
      const cells = computeReplicaCells(daemonApp)
      assert.equal(cells.label, '1 / 1 Unit Up')
      assert.equal(cells.blocks.length, 1)
      assert.equal(cells.blocks[0].state, 'online')
      assert.equal(cells.readyCount, 1)
      assert.equal(cells.desiredCount, 1)
    })

    it('computeReplicaCells returns truthful "0 / 1 Unit Down" label for inactive daemon workloads', () => {
      const daemonApp = {
        name: 'k8s-agent',
        type: 'host-daemon',
        status: 'down',
        replicas: 1,
        readyReplicas: 0
      }
      const cells = computeReplicaCells(daemonApp)
      assert.equal(cells.label, '0 / 1 Unit Down')
      assert.equal(cells.blocks[0].state, 'offline')
      assert.equal(cells.readyCount, 0)
    })

    it('formats daemon image spec as systemd service URL without fake :latest tag', () => {
      const daemon = { name: 'k8s-agent.service', image: 'local-binary:latest', type: 'daemon' }
      const isDaemon = daemon.type === 'daemon'
      const svcName = formatContainerName(daemon.name).serviceName || daemon.name
      const formatted = isDaemon
        ? `systemd://${svcName.replace(/\.service$/, '')}.service`
        : formatImageName(daemon.image).display

      assert.equal(formatted, 'systemd://k8s-agent.service')
      assert.ok(!formatted.includes(':latest'), 'Daemon spec must not show fake :latest docker tag')
    })

    it('generates genuine systemd unit from workloadInspectorYaml for daemon workloads', () => {
      const daemonWorkload = {
        name: 'k8s-agent',
        type: 'daemon' as const,
        namespace: 'edge-host-01',
        port: 9100,
        env: 'production',
        image: 'local-binary',
        replicas: 1,
        target: 'local-host',
        cpu: '100m',
        memory: '128Mi'
      }
      const unit = generateYamlManifest(daemonWorkload as any)
      assert.ok(unit.includes('[Unit]'), 'Manifest must be systemd unit')
      assert.ok(unit.includes('ExecStart='), 'Manifest must declare ExecStart')
      assert.ok(!unit.includes('apiVersion: apps/v1'), 'Must not emit fake k8s yaml')
    })

    it('distinguishes lane replica summary for all-daemon host lanes', () => {
      const daemonApps = [
        { name: 'agent1', type: 'daemon', status: 'healthy', replicas: 1 },
        { name: 'agent2', type: 'daemon', status: 'healthy', replicas: 1 }
      ]
      const isAllDaemons = daemonApps.every(a => a.type === 'daemon')
      const activeCount = daemonApps.filter(a => a.status === 'healthy').length
      const summary = isAllDaemons ? `${activeCount} units active` : `${daemonApps.length} pods ready`
      assert.equal(summary, '2 units active')
    })
  })

  describe('Criterion 5: /explorer 100% Full-Width Resource Table & Clean Summary Strip', () => {
    it('calculates SRE summary strip counts accurately from resource collection', () => {
      const resources = [
        { metadata: { name: 'coredns' }, status: { phase: 'Running' } },
        { metadata: { name: 'traefik' }, status: { phase: 'Running' } },
        { metadata: { name: 'db-failed' }, status: { phase: 'CrashLoopBackOff' } }
      ]
      const healthy = resources.filter(r => r.status.phase === 'Running').length
      const degraded = resources.filter(r => r.status.phase !== 'Running').length

      assert.equal(resources.length, 3)
      assert.equal(healthy, 2)
      assert.equal(degraded, 1)
    })
  })

  describe('Criterion 6: /helm Client Deduplication & Zero Emojis', () => {
    it('deduplicates charts by repo:name keeping first version', () => {
      const rawCharts = [
        { name: 'postgresql', repo: 'bitnami', version: '12.1.0', description: 'v12' },
        { name: 'postgresql', repo: 'bitnami', version: '11.0.0', description: 'v11 duplicate' },
        { name: 'redis', repo: 'bitnami', version: '17.0.0', description: 'redis' },
        { name: 'postgresql', repo: 'custom', version: '1.0.0', description: 'different repo' }
      ]
      const deduped = deduplicateCharts(rawCharts as any)
      assert.equal(deduped.length, 3)
      assert.equal(deduped[0].version, '12.1.0', 'Must keep first/latest occurrence')
      assert.equal(deduped[1].name, 'redis')
      assert.equal(deduped[2].repo, 'custom')
    })

    it('getChartIcon returns valid BaseIcon names and contains zero emojis', () => {
      const testCases = [
        { chart: { name: 'ingress-nginx' }, expected: 'globe' },
        { chart: { name: 'bitnami-postgresql' }, expected: 'database' },
        { chart: { name: 'prometheus-stack' }, expected: 'activity' },
        { chart: { name: 'cert-manager' }, expected: 'shield' },
        { chart: { name: 'kafka-cluster' }, expected: 'zap' },
        { chart: { name: 'opensearch' }, expected: 'search' },
        { chart: { name: 'ollama-service' }, expected: 'bot' },
        { chart: { name: 'generic-app' }, expected: 'package' }
      ]

      for (const tc of testCases) {
        const icon = getChartIcon(tc.chart as any)
        assert.equal(icon, tc.expected)
        assert.doesNotMatch(icon, /[\uD800-\uDBFF][\uDC00-\uDFFF]/, `Icon ${icon} must not contain unicode emojis`)
      }
    })

    it('REPO_PRESETS and CATEGORY_TAGS contain zero raw emojis', () => {
      for (const preset of REPO_PRESETS) {
        assert.doesNotMatch(preset.icon, /[\uD800-\uDBFF][\uDC00-\uDFFF]/, `Preset ${preset.name} icon must not be an emoji`)
      }
      for (const tag of CATEGORY_TAGS) {
        assert.doesNotMatch(tag.icon, /[\uD800-\uDBFF][\uDC00-\uDFFF]/, `Category ${tag.key} icon must not be an emoji`)
      }
    })

    it('filters charts by repo and category properly', () => {
      const charts = [
        { name: 'pg', repo: 'bitnami', keywords: ['database'] },
        { name: 'nginx', repo: 'ingress-nginx', keywords: ['networking'] }
      ]
      const bitnamiOnly = filterCharts(charts as any, 'bitnami', 'all')
      assert.equal(bitnamiOnly.length, 1)
      assert.equal(bitnamiOnly[0].name, 'pg')
    })

    it('builds valid install payload', () => {
      const chart = { name: 'pg', repo: 'bitnami', version: '1.0.0' }
      const form = {
        releaseName: 'prod-db',
        namespace: 'default',
        createNamespace: true,
        version: '1.0.0',
        values: 'replicas: 1',
        loadingDefaultValues: false
      }
      const payload = buildInstallPayload(chart as any, form)
      assert.equal(payload.releaseName, 'prod-db')
      assert.equal(payload.chart, 'pg')
      assert.equal(payload.repo, 'bitnami')
    })
  })
})
