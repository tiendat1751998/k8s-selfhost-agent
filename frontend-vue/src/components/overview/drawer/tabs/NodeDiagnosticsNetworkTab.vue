<script setup lang="ts">
import { computed } from 'vue'
import type { NodeMetrics } from '../../../../api/overview'
import { getUtilizationColor, formatBytes, formatIoRate } from './nodeDiagnosticsUtils'

interface Props {
  node: NodeMetrics
}

const props = defineProps<Props>()

const filteredNodeInterfaces = computed(() => {
  return props.node.network_interfaces || []
})

const filteredNodeDiskDevices = computed(() => {
  return props.node.disk_devices || []
})
</script>

<template>
  <div>
    <!-- Network Interfaces Section (Compact 4-Card Row) -->
    <div class="network-interfaces-section" v-if="filteredNodeInterfaces.length > 0">
      <div class="hud-section-header">
        <h4 class="hud-section-heading">
          <span class="heading-icon">📡</span> Network Interface Throughput
        </h4>
        <span class="badge badge-cyan font-mono">{{ filteredNodeInterfaces.length }} {{ filteredNodeInterfaces.length === 1 ? 'interface' : 'interfaces' }}</span>
      </div>
      <div class="interfaces-grid">
        <div
          v-for="iface in filteredNodeInterfaces"
          :key="iface.name"
          class="iface-card glass-panel"
        >
          <div class="iface-top">
            <div class="iface-name-group">
              <span class="iface-icon">🌐</span>
              <span class="iface-name font-mono" :title="iface.name">{{ iface.name }}</span>
            </div>
            <span class="iface-nic-tag font-mono">NIC</span>
          </div>
          <div class="iface-rates-row font-mono">
            <span class="iface-rate-item text-cyan" :title="'Download Rate (Rx): ' + formatIoRate(iface.rx_bytes_per_sec)">
              <span class="rate-arrow">↓</span> {{ formatIoRate(iface.rx_bytes_per_sec) }}
            </span>
            <span class="iface-rate-sep">·</span>
            <span class="iface-rate-item text-purple" :title="'Upload Rate (Tx): ' + formatIoRate(iface.tx_bytes_per_sec)">
              <span class="rate-arrow">↑</span> {{ formatIoRate(iface.tx_bytes_per_sec) }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Disk I/O & Block Device Telemetry Section -->
    <div class="disk-io-section mt-4">
      <div class="hud-section-header">
        <h4 class="hud-section-heading">
          <span class="heading-icon">💽</span> Disk I/O & Block Device Telemetry
        </h4>
        <span class="badge badge-indigo font-mono" v-if="filteredNodeDiskDevices.length > 0">
          {{ filteredNodeDiskDevices.length }} {{ filteredNodeDiskDevices.length === 1 ? 'device' : 'devices' }}
        </span>
        <span class="badge badge-slate font-mono" v-else>
          HOST TELEMETRY
        </span>
      </div>

      <!-- Summary Strip -->
      <div class="disk-summary-strip glass-panel">
        <div class="disk-summary-item" title="Physical Disk Read Throughput">
          <span class="disk-summary-label">📖 READ THROUGHPUT</span>
          <span class="disk-summary-val font-mono text-cyan font-bold">
            {{ formatBytes(node.disk_read_bytes_per_sec || 0) }}/s
          </span>
        </div>
        <div class="disk-summary-item" title="Physical Disk Write Throughput">
          <span class="disk-summary-label">✍️ WRITE THROUGHPUT</span>
          <span class="disk-summary-val font-mono text-purple font-bold">
            {{ formatBytes(node.disk_write_bytes_per_sec || 0) }}/s
          </span>
        </div>
        <div class="disk-summary-item" title="Total Physical Read & Write IOPS">
          <span class="disk-summary-label">⚡ TOTAL IOPS</span>
          <span class="disk-summary-val font-mono text-emerald font-bold">
            {{ (node.disk_read_iops || 0).toFixed(0) }} R · {{ (node.disk_write_iops || 0).toFixed(0) }} W
          </span>
        </div>
        <div class="disk-summary-item" title="Average Total Latency Per I/O Operation (await)">
          <span class="disk-summary-label">⏱️ AVG LATENCY (await)</span>
          <span
            class="disk-summary-val font-mono font-bold"
            :class="(node.disk_avg_await_ms || 0) > 30 ? 'text-rose' : (node.disk_avg_await_ms || 0) > 10 ? 'text-amber' : 'text-emerald'"
          >
            {{ (node.disk_avg_await_ms || 0).toFixed(1) }} ms
          </span>
        </div>
        <div class="disk-summary-item" title="Maximum Block Device I/O Saturation Percentage">
          <span class="disk-summary-label">📊 MAX %UTIL</span>
          <span
            class="disk-summary-val font-mono font-bold"
            :class="`text-${getUtilizationColor(node.disk_max_io_util_pct || 0)}`"
          >
            {{ (node.disk_max_io_util_pct || 0).toFixed(1) }}%
          </span>
        </div>
      </div>

      <!-- Block Devices Table / Micro-Cards -->
      <div class="disk-devices-wrapper" v-if="filteredNodeDiskDevices.length > 0">
        <div class="disk-devices-grid">
          <div
            v-for="dev in filteredNodeDiskDevices"
            :key="dev.device_name"
            class="disk-device-card glass-panel"
          >
            <div class="dev-card-header">
              <div class="dev-name-group">
                <span class="dev-icon">💾</span>
                <span class="dev-name font-mono font-bold" :title="dev.device_name">{{ dev.device_name }}</span>
              </div>
              <span
                class="dev-badge font-mono font-bold"
                :class="dev.is_root_device ? 'badge-root' : 'badge-part'"
                :title="dev.is_root_device ? 'Root Partition / Primary Device' : 'Storage Device / Partition'"
              >
                {{ dev.is_root_device ? 'ROOT' : 'PART' }}
              </span>
            </div>

            <div class="dev-metrics-grid font-mono">
              <div class="dev-metric-col" title="Read Throughput Rate">
                <span class="dev-metric-label">READ RATE</span>
                <span class="dev-metric-val text-cyan font-bold">📖 {{ formatIoRate(dev.read_bytes_per_sec) }}</span>
              </div>
              <div class="dev-metric-col" title="Write Throughput Rate">
                <span class="dev-metric-label">WRITE RATE</span>
                <span class="dev-metric-val text-purple font-bold">✍️ {{ formatIoRate(dev.write_bytes_per_sec) }}</span>
              </div>
              <div class="dev-metric-col" title="IOPS (Read / Write)">
                <span class="dev-metric-label">IOPS (R/W)</span>
                <span class="dev-metric-val text-emerald font-bold">{{ dev.read_iops.toFixed(0) }} R / {{ dev.write_iops.toFixed(0) }} W</span>
              </div>
              <div class="dev-metric-col" title="Average Wait Latency (await)">
                <span class="dev-metric-label">AWAIT LATENCY</span>
                <span
                  class="dev-metric-val font-bold"
                  :class="dev.avg_wait_ms > 30 ? 'text-rose' : dev.avg_wait_ms > 10 ? 'text-amber' : 'text-emerald'"
                >
                  {{ dev.avg_wait_ms.toFixed(1) }} ms
                </span>
              </div>
              <div class="dev-metric-col" title="Average Request Size">
                <span class="dev-metric-label">AVG REQ SZ</span>
                <span class="dev-metric-val text-slate font-bold">{{ dev.avg_req_size_kb.toFixed(1) }} KB</span>
              </div>
              <div class="dev-metric-col" title="Current Queue Depth">
                <span class="dev-metric-label">QUEUE DEPTH</span>
                <span
                  class="dev-metric-val font-bold"
                  :class="dev.current_queue_depth > 10 ? 'text-rose' : dev.current_queue_depth > 4 ? 'text-amber' : 'text-slate'"
                >
                  {{ dev.current_queue_depth }}
                </span>
              </div>
            </div>

            <!-- %util Progress Track -->
            <div class="dev-util-row">
              <div class="dev-util-label-group">
                <span class="dev-util-label">DEVICE I/O UTILIZATION (%UTIL)</span>
                <span
                  class="dev-util-pct font-mono font-bold"
                  :class="`text-${getUtilizationColor(dev.io_utilization_pct || 0)}`"
                >
                  {{ (dev.io_utilization_pct || 0).toFixed(1) }}%
                </span>
              </div>
              <div class="hw-progress-track">
                <div
                  class="hw-progress-fill smooth-bar"
                  :class="`bg-${getUtilizationColor(dev.io_utilization_pct || 0)}`"
                  :style="{ width: `${Math.min(100, Math.max(0, dev.io_utilization_pct || 0))}%` }"
                ></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Fallback when no devices reported -->
      <div class="disk-fallback-panel glass-panel" v-else>
        <div class="fallback-content">
          <span class="fallback-icon">💾</span>
          <div class="fallback-info">
            <span class="fallback-title font-mono">Block Device Breakdown Unavailable</span>
            <span class="fallback-subtitle text-muted">
              Displaying host-level aggregate storage metrics. Individual block device breakdown is available when reported by node agent.
            </span>
          </div>
          <div class="fallback-rates font-mono">
            <span class="rate-pill text-cyan">📖 {{ formatIoRate(node.disk_read_bytes_per_sec || 0) }} Read</span>
            <span class="rate-pill text-purple">✍️ {{ formatIoRate(node.disk_write_bytes_per_sec || 0) }} Write</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../../../assets/styles/components/node-diagnostics.css';
</style>
