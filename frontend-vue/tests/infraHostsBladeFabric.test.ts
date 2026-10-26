import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {
  matchHostWithNodeMetrics,
  resolveHostHardwareMetrics,
  computeFleetHardwareAggregates,
  type HostHardwareMetrics,
} from '../src/types/hosts.ts'
import type { ComputeHost, NodeMetrics, ProcessMetric } from '../src/api/hosts.ts'

describe('Task T068-1.1: Physical Hardware Blade Fabric Telemetry Join Tests', () => {
  const mockNodeMetrics: NodeMetrics[] = [
    {
      node_id: 'node-60-uuid',
      node_name: 'k8smasterdeb',
      role: 'manager',
      status: 'ready',
      cpu_percent: 14.8,
      load_average: [0.85, 0.92, 1.05],
      memory_used: 12884901888, // 12 GiB
      memory_total: 34359738368, // 32 GiB
      memory_percent: 37.5,
      disk_used: 107374182400, // 100 GiB
      disk_total: 536870912000, // 500 GiB
      disk_percent: 20.0,
      disk_avg_await_ms: 1.2,
      os_distro: 'Debian GNU/Linux 12 (bookworm)',
      kernel_version: '6.1.0-21-amd64',
      arch: 'x86_64',
      uptime_seconds: 1209600, // 14 days
      running_count: 18,
      container_count: 22,
      processes: 142,
      network_rx_bytes: 1048576,
      network_tx_bytes: 2097152,
      top_processes: [
        {
          pid: 1024,
          name: 'kube-apiserver',
          command_line: '/usr/local/bin/kube-apiserver',
          user: 'root',
          cpu_percent: 6.4,
          memory_bytes: 1073741824,
          memory_percent: 3.1,
          read_bytes_per_sec: 1024,
          write_bytes_per_sec: 2048,
          state: 'R'
        }
      ]
    },
    {
      node_id: 'node-133-uuid',
      node_name: 'k8smater133',
      role: 'worker',
      status: 'ready',
      cpu_percent: 42.1,
      load_average: [2.15, 1.88, 1.45],
      memory_used: 25769803776, // 24 GiB
      memory_total: 34359738368, // 32 GiB
      memory_percent: 75.0,
      disk_used: 322122547200, // 300 GiB
      disk_total: 536870912000, // 500 GiB
      disk_percent: 60.0,
      disk_avg_await_ms: 3.8,
      os_distro: 'Ubuntu 24.04 LTS',
      kernel_version: '6.8.0-31-generic',
      arch: 'x86_64',
      uptime_seconds: 604800, // 7 days
      running_count: 12,
      container_count: 14,
      processes: 98,
      network_rx_bytes: 524288,
      network_tx_bytes: 1048576,
      top_processes: []
    }
  ]

  it('matches ComputeHost with NodeMetrics by exact name', () => {
    const host: ComputeHost = {
      id: 'host-1',
      name: 'k8smasterdeb',
      host_type: 'agent',
      endpoint: 'http://10.10.10.60:9100',
      tls_enabled: false,
      status: 'connected',
      labels: {},
      created_at: '2026-10-20T00:00:00Z'
    }

    const matched = matchHostWithNodeMetrics(host, mockNodeMetrics)
    assert.ok(matched)
    assert.strictEqual(matched?.node_name, 'k8smasterdeb')
    assert.strictEqual(matched?.cpu_percent, 14.8)
  })

  it('matches ComputeHost with NodeMetrics by endpoint IP', () => {
    const host: ComputeHost = {
      id: 'host-2',
      name: 'worker-primary',
      host_type: 'agent',
      endpoint: 'http://10.10.10.133:9100',
      tls_enabled: false,
      status: 'connected',
      labels: {},
      created_at: '2026-10-20T00:00:00Z'
    }

    const matched = matchHostWithNodeMetrics(host, [
      ...mockNodeMetrics,
      {
        ...mockNodeMetrics[1],
        node_id: '10.10.10.133',
        node_name: 'agent-10.10.10.133'
      }
    ])
    assert.ok(matched)
    assert.strictEqual(matched?.node_id, '10.10.10.133')
  })

  it('matches ComputeHost with NodeMetrics by kubernetes.io/hostname label', () => {
    const host: ComputeHost = {
      id: 'custom-host-id',
      name: 'Physical Blade 01',
      host_type: 'agent',
      endpoint: 'tcp://192.168.1.50:9100',
      tls_enabled: true,
      status: 'connected',
      labels: {
        'kubernetes.io/hostname': 'k8smasterdeb'
      },
      created_at: '2026-10-20T00:00:00Z'
    }

    const matched = matchHostWithNodeMetrics(host, mockNodeMetrics)
    assert.ok(matched)
    assert.strictEqual(matched?.node_name, 'k8smasterdeb')
  })

  it('resolves HostHardwareMetrics from NodeMetrics with complete fidelity', () => {
    const host: ComputeHost = {
      id: 'host-1',
      name: 'k8smasterdeb',
      host_type: 'agent',
      endpoint: 'http://10.10.10.60:9100',
      tls_enabled: false,
      status: 'connected',
      labels: {},
      created_at: '2026-10-20T00:00:00Z'
    }

    const hw = resolveHostHardwareMetrics(host, mockNodeMetrics[0])
    assert.strictEqual(hw.cpu_percent, 14.8)
    assert.deepStrictEqual(hw.load_average, [0.85, 0.92, 1.05])
    assert.strictEqual(hw.memory_used, 12884901888)
    assert.strictEqual(hw.memory_total, 34359738368)
    assert.strictEqual(hw.memory_percent, 37.5)
    assert.strictEqual(hw.disk_used, 107374182400)
    assert.strictEqual(hw.disk_total, 536870912000)
    assert.strictEqual(hw.disk_percent, 20.0)
    assert.strictEqual(hw.disk_avg_await_ms, 1.2)
    assert.strictEqual(hw.os_distro, 'Debian GNU/Linux 12 (bookworm)')
    assert.strictEqual(hw.kernel_version, '6.1.0-21-amd64')
    assert.strictEqual(hw.arch, 'x86_64')
    assert.strictEqual(hw.uptime_seconds, 1209600)
    assert.strictEqual(hw.running_count, 18)
    assert.strictEqual(hw.container_count, 22)
    assert.strictEqual(hw.processes, 142)
    assert.strictEqual(hw.top_processes.length, 1)
    assert.strictEqual(hw.top_processes[0].name, 'kube-apiserver')
    assert.strictEqual(hw.is_live, true)
  })

  it('handles missing / offline metrics gracefully with zero values and no NaN', () => {
    const host: ComputeHost = {
      id: 'host-offline',
      name: 'k8soffline',
      host_type: 'agent',
      endpoint: 'http://10.10.10.80:9100',
      tls_enabled: false,
      status: 'disconnected',
      labels: {},
      created_at: '2026-10-20T00:00:00Z'
    }

    const hw = resolveHostHardwareMetrics(host, undefined)
    assert.strictEqual(hw.cpu_percent, 0)
    assert.strictEqual(isNaN(hw.cpu_percent), false)
    assert.strictEqual(hw.memory_percent, 0)
    assert.strictEqual(isNaN(hw.memory_percent), false)
    assert.strictEqual(hw.disk_percent, 0)
    assert.strictEqual(isNaN(hw.disk_percent), false)
    assert.strictEqual(hw.load_average[0], 0)
    assert.strictEqual(hw.is_live, false)
  })

  it('computes real fleet hardware aggregates accurately', () => {
    const hosts: ComputeHost[] = [
      {
        id: 'h1',
        name: 'k8smasterdeb',
        host_type: 'agent',
        endpoint: 'http://10.10.10.60:9100',
        tls_enabled: false,
        status: 'connected',
        labels: {},
        created_at: '2026-10-20T00:00:00Z'
      },
      {
        id: 'h2',
        name: 'k8smater133',
        host_type: 'agent',
        endpoint: 'http://10.10.10.133:9100',
        tls_enabled: false,
        status: 'connected',
        labels: {},
        created_at: '2026-10-20T00:00:00Z'
      },
      {
        id: 'h3',
        name: 'k8soffline',
        host_type: 'agent',
        endpoint: 'http://10.10.10.80:9100',
        tls_enabled: false,
        status: 'disconnected',
        labels: {},
        created_at: '2026-10-20T00:00:00Z'
      }
    ]

    const hwMap: Record<string, HostHardwareMetrics> = {
      h1: resolveHostHardwareMetrics(hosts[0], mockNodeMetrics[0]),
      h2: resolveHostHardwareMetrics(hosts[1], mockNodeMetrics[1]),
      h3: resolveHostHardwareMetrics(hosts[2], undefined)
    }

    const aggregates = computeFleetHardwareAggregates(hosts, hwMap)

    // CPU avg: (14.8 + 42.1) / 2 = 28.45 -> 28.5%
    assert.strictEqual(aggregates.fleetCpuAvgPct, 28.5)
    assert.strictEqual(aggregates.fleetMemUsedBytes, 12884901888 + 25769803776)
    assert.strictEqual(aggregates.fleetMemTotalBytes, 34359738368 + 34359738368)
    assert.strictEqual(aggregates.fleetDiskUsedBytes, 107374182400 + 322122547200)
    assert.strictEqual(aggregates.fleetDiskTotalBytes, 536870912000 + 536870912000)
    assert.strictEqual(aggregates.totalRunningContainers, 18 + 12)
    assert.strictEqual(aggregates.onlineBladeCount, 2)
    assert.strictEqual(aggregates.totalBladeCount, 3)
  })

  it('verifies strict line limits (< 500 lines, BladeRack < 300, Drawer < 220) on all target files', () => {
    const targetFiles = [
      '../src/types/hosts.ts',
      '../src/composables/useInfraHosts.ts',
      '../src/views/InfraHostsView.vue',
      '../src/components/hosts/HostBladeRackGrid.vue',
      '../src/components/hosts/FleetCapacityStrip.vue',
      '../src/components/hosts/HostControlsBar.vue',
      '../src/components/hosts/HostsMobileCards.vue',
      '../src/components/hosts/HostDetailDrawer.vue',
      '../src/assets/styles/views/infra-hosts-blade.css'
    ]

    const currentDir = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([a-zA-Z]:)/, '$1'))
    for (const relPath of targetFiles) {
      const fullPath = path.resolve(currentDir, relPath)
      if (fs.existsSync(fullPath)) {
        const content = fs.readFileSync(fullPath, 'utf8')
        const lines = content.split('\n').length
        assert.ok(
          lines < 500,
          `File ${relPath} exceeds 500 lines (current: ${lines})`
        )
        if (relPath.includes('HostBladeRackGrid.vue')) {
          assert.ok(lines < 300, `HostBladeRackGrid.vue must be < 300 lines (current: ${lines})`)
        }
        if (relPath.includes('HostDetailDrawer.vue')) {
          assert.ok(lines < 220, `HostDetailDrawer.vue must be < 220 lines (current: ${lines})`)
        }
      }
    }
  })
})
