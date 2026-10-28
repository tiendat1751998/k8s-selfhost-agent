<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition, HostHardwareMetrics } from '../../types/hosts'
import { resolveHostHardwareMetrics } from '../../types/hosts'

const props = defineProps<{
  hosts: ComputeHost[]
  hostHardwareMap?: Record<string, HostHardwareMetrics>
  hostTestResults: Record<string, HostTestResult>
  testingHostId: string | null
  getHostTypeMeta: (type?: string) => HostTypeDefinition
  formatDate: (d?: string) => string
  formatUptime: (seconds?: number) => string
}>()

const emit = defineEmits<{
  (e: 'test', host: ComputeHost): void
  (e: 'edit', host: ComputeHost): void
  (e: 'delete', host: ComputeHost): void
  (e: 'select', host: ComputeHost): void
  (e: 'copy', text: string): void
}>()

const pageSize = 48
const currentPage = ref(1)
const totalPages = computed(() => Math.max(1, Math.ceil(props.hosts.length / pageSize)))

watch(() => props.hosts.length, () => {
  if (currentPage.value > totalPages.value) currentPage.value = 1
})

const startIndex = computed(() => (currentPage.value - 1) * pageSize)
const endIndex = computed(() => Math.min(startIndex.value + pageSize, props.hosts.length))
const paginatedHosts = computed(() => {
  if (props.hosts.length <= pageSize) return props.hosts
  return props.hosts.slice(startIndex.value, endIndex.value)
})

function prevPage() { if (currentPage.value > 1) currentPage.value-- }
function nextPage() { if (currentPage.value < totalPages.value) currentPage.value++ }

function getHw(host: ComputeHost): HostHardwareMetrics {
  return props.hostHardwareMap?.[host.id] || resolveHostHardwareMetrics(host, undefined, props.hostTestResults[host.id]?.agent_info)
}

function getLatency(hostId: string): number { return props.hostTestResults[hostId]?.latency_ms ?? 0 }
function getSaturationClass(pct: number): string {
  return pct >= 85 ? 'meter-critical' : pct >= 70 ? 'meter-warning' : 'meter-healthy'
}
function bytesToGiB(b: number): string { return (b / (1024 * 1024 * 1024)).toFixed(1) }

function getSystemSpecLine(host: ComputeHost, hw: HostHardwareMetrics): string {
  const parts: string[] = []
  if (hw.os_distro) parts.push(hw.os_distro)
  if (hw.kernel_version) parts.push(hw.kernel_version)
  if (hw.arch) parts.push(hw.arch)
  if (hw.uptime_seconds > 0) parts.push(`Up ${props.formatUptime(hw.uptime_seconds)}`)
  return parts.length > 0 ? parts.join(' · ') : host.endpoint
}

function getHostActions(): ActionItem[] {
  return [
    { id: 'edit', action: 'edit', label: 'Edit Blade', icon: 'edit' },
    { id: 'copy', action: 'copy', label: 'Copy Endpoint', icon: 'copy' },
    { id: 'delete', action: 'delete', label: 'Decommission Host', icon: 'trash', variant: 'danger', isDestructive: true }
  ]
}

function handleDropdownAction(action: string, host: ComputeHost) {
  if (action === 'edit') emit('edit', host)
  else if (action === 'copy') emit('copy', host.endpoint)
  else if (action === 'delete') emit('delete', host)
}
</script>

<template>
  <div class="blade-rack-container font-mono">
    <!-- Optional Pagination for ultra-dense fleets -->
    <div v-if="hosts.length > pageSize" class="blade-pagination">
      <span class="text-xs text-muted">Showing {{ startIndex + 1 }}–{{ endIndex }} of {{ hosts.length }} blades</span>
      <div class="pagination-controls">
        <button type="button" class="btn btn-secondary btn-xs" :disabled="currentPage <= 1" @click="prevPage">Prev</button>
        <span class="text-xs font-bold">{{ currentPage }} / {{ totalPages }}</span>
        <button type="button" class="btn btn-secondary btn-xs" :disabled="currentPage >= totalPages" @click="nextPage">Next</button>
      </div>
    </div>

    <!-- Physical Hardware Blade Rack Grid -->
    <div class="blade-rack-grid animate-fade-in" role="list" aria-label="Physical Hardware Blade Fabric">
      <div
        v-for="host in paginatedHosts"
        :key="host.id"
        class="blade-card"
        :class="[
          `blade-rail-${host.status === 'connected' || host.status === 'ok' ? 'online' : 'offline'}`,
          { 'blade-card-testing': testingHostId === host.id }
        ]"
        role="listitem"
      >
        <!-- Left Physical Chassis Indicator Rail -->
        <div class="chassis-status-rail" :title="`Status: ${host.status || 'unknown'}`"></div>

        <div class="blade-body">
          <!-- Blade Top Row: Identity, Status, Latency & Role/Labels -->
          <div class="blade-header-row">
            <div class="blade-identity" @click="emit('select', host)">
              <div class="chassis-icon">
                <BaseIcon name="server" size="sm" />
              </div>
              <div class="blade-titles">
                <div class="blade-name-line">
                  <h3 class="blade-hostname" :title="host.name">{{ host.name }}</h3>
                  <span
                    class="blade-status-pill"
                    :class="host.status === 'connected' || host.status === 'ok' ? 'pill-online' : 'pill-offline'"
                  >
                    <span class="status-dot"></span>
                    {{ host.status === 'connected' || host.status === 'ok' ? 'CONNECTED' : 'OFFLINE' }}
                  </span>
                  <span class="blade-latency-badge" :title="getLatency(host.id) > 0 ? `Latency: ${getLatency(host.id)}ms` : 'Unprobed'">
                    <BaseIcon name="activity" size="xs" />
                    {{ getLatency(host.id) > 0 ? `${getLatency(host.id)}ms` : '--' }}
                  </span>
                </div>
                <!-- Real OS System Spec Line -->
                <div class="blade-spec-line text-muted" :title="getSystemSpecLine(host, getHw(host))">
                  <span>{{ getSystemSpecLine(host, getHw(host)) }}</span>
                </div>
              </div>
            </div>

            <div class="blade-chips">
              <span class="blade-role-chip" :class="getHostTypeMeta(host.host_type).badgeClass">
                {{ host.host_role || getHostTypeMeta(host.host_type).label }}
              </span>
              <template v-if="host.labels">
                <span
                  v-for="(val, key) in host.labels"
                  :key="key"
                  class="blade-tag-chip"
                  :title="`${key}=${val}`"
                >
                  {{ key }}={{ val }}
                </span>
              </template>
            </div>
          </div>

          <!-- 3 Horizontal Hardware Saturation Meters -->
          <div class="blade-telemetry-meters">
            <!-- 1. CPU Saturation -->
            <div class="telemetry-bar-row">
              <div class="meter-head">
                <span class="meter-name">CPU</span>
                <span class="meter-val" :class="getSaturationClass(getHw(host).cpu_percent)">
                  {{ getHw(host).cpu_percent.toFixed(1) }}%
                </span>
                <span class="meter-extra text-muted">
                  Load 1m: {{ getHw(host).load_average[0]?.toFixed(2) ?? '0.00' }}
                </span>
              </div>
              <div class="meter-track">
                <div
                  class="meter-fill"
                  :class="getSaturationClass(getHw(host).cpu_percent)"
                  :style="{ width: `${Math.min(100, Math.max(0, getHw(host).cpu_percent))}%` }"
                ></div>
              </div>
            </div>

            <!-- 2. RAM Saturation -->
            <div class="telemetry-bar-row">
              <div class="meter-head">
                <span class="meter-name">RAM</span>
                <span class="meter-val" :class="getSaturationClass(getHw(host).memory_percent)">
                  {{ getHw(host).memory_percent.toFixed(1) }}%
                </span>
                <span class="meter-extra text-muted">
                  {{ bytesToGiB(getHw(host).memory_used) }} / {{ bytesToGiB(getHw(host).memory_total) }} GiB
                </span>
              </div>
              <div class="meter-track">
                <div
                  class="meter-fill"
                  :class="getSaturationClass(getHw(host).memory_percent)"
                  :style="{ width: `${Math.min(100, Math.max(0, getHw(host).memory_percent))}%` }"
                ></div>
              </div>
            </div>

            <!-- 3. DISK Saturation -->
            <div class="telemetry-bar-row">
              <div class="meter-head">
                <span class="meter-name">DISK</span>
                <span class="meter-val" :class="getSaturationClass(getHw(host).disk_percent)">
                  {{ getHw(host).disk_percent.toFixed(1) }}%
                </span>
                <span class="meter-extra text-muted">
                  {{ bytesToGiB(getHw(host).disk_used) }} / {{ bytesToGiB(getHw(host).disk_total) }} GiB
                  <template v-if="getHw(host).disk_avg_await_ms > 0">
                    · {{ getHw(host).disk_avg_await_ms.toFixed(1) }}ms await
                  </template>
                </span>
              </div>
              <div class="meter-track">
                <div
                  class="meter-fill"
                  :class="getSaturationClass(getHw(host).disk_percent)"
                  :style="{ width: `${Math.min(100, Math.max(0, getHw(host).disk_percent))}%` }"
                ></div>
              </div>
            </div>
          </div>

          <!-- Blade Footer Strip: Workloads & Primary Actions -->
          <div class="blade-footer-strip">
            <div class="footer-workload-info text-muted">
              <template v-if="getHw(host).is_live">
                <span class="workload-stat">
                  <BaseIcon name="box" size="xs" />
                  <strong>{{ getHw(host).running_count }}</strong> containers
                </span>
                <span class="workload-sep">·</span>
                <span class="workload-stat">
                  <strong>{{ getHw(host).processes }}</strong> procs
                </span>
                <template v-if="getHw(host).top_processes && getHw(host).top_processes.length > 0">
                  <span class="workload-sep">·</span>
                  <span class="workload-top-proc text-primary" :title="getHw(host).top_processes[0].command_line">
                    top: {{ getHw(host).top_processes[0].name }} ({{ getHw(host).top_processes[0].cpu_percent.toFixed(1) }}%)
                  </span>
                </template>
              </template>
              <template v-else>
                <span class="endpoint-mono text-muted" :title="host.endpoint">{{ host.endpoint }}</span>
              </template>
            </div>

            <div class="footer-actions-group">
              <!-- Inline Action 1: Inspect -->
              <button
                type="button"
                class="btn btn-secondary btn-xs inline-blade-btn"
                title="Inspect Blade Telemetry"
                @click="emit('select', host)"
              >
                <BaseIcon name="server" size="xs" />
                <span>Inspect</span>
              </button>

              <!-- Inline Action 2: Ping -->
              <button
                type="button"
                class="btn btn-secondary btn-xs inline-blade-btn"
                :disabled="testingHostId === host.id"
                title="Probe Host Latency"
                @click="emit('test', host)"
              >
                <BaseIcon
                  :name="testingHostId === host.id ? 'refresh' : 'activity'"
                  size="xs"
                  :class="{ 'animate-spin': testingHostId === host.id }"
                />
                <span>{{ testingHostId === host.id ? 'Pinging' : 'Ping' }}</span>
              </button>

              <!-- Context Menu [...] -->
              <ActionDropdown
                :items="getHostActions()"
                size="xs"
                trigger-title="Blade options"
                @select="handleDropdownAction($event, host)"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Inherited via InfraHostsView.vue: assets/styles/views/infra-hosts.css -> infra-hosts-blade.css */
</style>
