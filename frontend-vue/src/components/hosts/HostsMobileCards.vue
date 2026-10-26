<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition, HostHardwareMetrics } from '../../types/hosts'
import { formatDate as defaultFormatDate, formatHostLastSeen, resolveHostHardwareMetrics } from '../../types/hosts'

const props = defineProps<{
  hosts: ComputeHost[]
  hostHardwareMap?: Record<string, HostHardwareMetrics>
  hostTestResults: Record<string, HostTestResult>
  testingHostId: string | null
  getHostTypeMeta: (type?: string) => HostTypeDefinition
  getLatencyBadgeClass: (latency?: number) => string
  formatDate?: (d?: string) => string
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

const emit = defineEmits<{
  (e: 'test', host: ComputeHost): void
  (e: 'edit', host: ComputeHost): void
  (e: 'delete', host: ComputeHost): void
  (e: 'select', host: ComputeHost): void
  (e: 'fencing', host: ComputeHost): void
}>()

function formatHostDate(d?: string): string {
  if (props.formatDate) return props.formatDate(d)
  return defaultFormatDate(d)
}

function getMobileRowActions(host: ComputeHost): ActionItem[] {
  const items: ActionItem[] = [
    { id: 'select', action: 'select', label: 'View Details', icon: 'server' },
    { id: 'edit', action: 'edit', label: 'Edit Host', icon: 'edit' }
  ]
  if (host.status !== 'connected') {
    items.push({
      id: 'fencing',
      action: 'fencing',
      label: 'Safe Storage Fencing',
      icon: 'shield',
      variant: 'warning'
    })
  }
  items.push({
    id: 'delete',
    action: 'delete',
    label: 'Delete Host',
    icon: 'trash',
    variant: 'danger',
    isDestructive: true
  })
  return items
}

function handleMobileAction(action: string, host: ComputeHost) {
  if (action === 'select') emit('select', host)
  else if (action === 'edit') emit('edit', host)
  else if (action === 'fencing') emit('fencing', host)
  else if (action === 'delete') emit('delete', host)
}

function getHw(host: ComputeHost): HostHardwareMetrics {
  return props.hostHardwareMap?.[host.id] || resolveHostHardwareMetrics(host, undefined, props.hostTestResults[host.id]?.agent_info)
}

function getSaturationClass(pct: number): string {
  return pct >= 85 ? 'm-meter-critical' : pct >= 70 ? 'm-meter-warning' : 'm-meter-healthy'
}
</script>

<template>
  <div class="mobile-hosts-container">
    <div
      v-if="hosts.length > pageSize"
      class="mobile-pagination-bar font-mono"
    >
      <span class="text-muted">Showing {{ startIndex + 1 }}–{{ endIndex }} of {{ hosts.length }}</span>
      <div class="m-pagination-controls">
        <button
          type="button"
          class="btn btn-secondary btn-xs"
          :disabled="currentPage <= 1"
          @click="prevPage"
        >
          Prev
        </button>
        <span>{{ currentPage }} / {{ totalPages }}</span>
        <button
          type="button"
          class="btn btn-secondary btn-xs"
          :disabled="currentPage >= totalPages"
          @click="nextPage"
        >
          Next
        </button>
      </div>
    </div>

    <div class="mobile-hosts-stream animate-fade-in">
      <div
        v-for="host in paginatedHosts"
        :key="host.id"
        class="mobile-host-card"
        @click="emit('select', host)"
      >
        <!-- Row 1: Left has 6px status dot + Host Name + Type Chip; Right has Probe button + ActionDropdown -->
        <div class="m-card-header-row">
          <div class="m-card-header-left">
            <span
              class="m-status-dot"
              :class="host.status === 'connected' ? 'dot-connected' : 'dot-disconnected'"
              :aria-label="host.status === 'connected' ? 'Connected' : 'Disconnected'"
            ></span>
            <span class="m-host-name font-semibold" :title="host.name">
              {{ host.name }}
            </span>
            <span class="m-type-chip font-mono" :class="getHostTypeMeta(host.host_type).badgeClass">
              {{ getHostTypeMeta(host.host_type).label }}
            </span>
          </div>
          <div class="m-card-header-right" @click.stop>
            <button
              type="button"
              class="btn-m-probe"
              :disabled="testingHostId === host.id"
              title="Probe connection"
              @click.stop="emit('test', host)"
            >
              <BaseIcon
                :name="testingHostId === host.id ? 'refresh' : 'zap'"
                size="xs"
                :class="{ 'animate-spin': testingHostId === host.id }"
              />
              <span>{{ testingHostId === host.id ? 'Probing' : 'Probe' }}</span>
            </button>
            <ActionDropdown
              :items="getMobileRowActions(host)"
              size="xs"
              trigger-title="Host actions"
              @select="handleMobileAction($event, host)"
            />
          </div>
        </div>

        <!-- Row 2: Left has Monospace Endpoint; Right has Tabular Latency or Last Seen -->
        <div class="m-card-meta-row">
          <div class="m-meta-info-left">
            <span
              class="m-endpoint font-mono"
              :title="host.endpoint || host.labels?.['kubernetes.io/hostname'] || host.name"
            >
              {{ host.endpoint || host.labels?.['kubernetes.io/hostname'] || host.name }}
            </span>
          </div>

          <div class="m-meta-info-right">
            <span
              v-if="hostTestResults[host.id]"
              class="m-latency font-mono"
              :class="getLatencyBadgeClass(hostTestResults[host.id].latency_ms)"
            >
              <template v-if="hostTestResults[host.id].latency_ms > 0">
                {{ hostTestResults[host.id].latency_ms }}ms
              </template>
              <template v-else>
                --
              </template>
            </span>
            <span v-else class="m-last-check text-muted">
              {{ formatHostLastSeen(host, formatHostDate, true) }}
            </span>
          </div>
        </div>

        <!-- Row 3: Compact Inline CPU % · RAM % · DISK % Mini-bars (Mobile Parity) -->
        <div class="m-card-telemetry-row font-mono">
          <div class="m-telemetry-meter">
            <span class="m-meter-lbl">CPU</span>
            <div class="m-meter-track">
              <div
                class="m-meter-fill"
                :class="getSaturationClass(getHw(host).cpu_percent)"
                :style="{ width: `${Math.min(100, Math.max(0, getHw(host).cpu_percent))}%` }"
              ></div>
            </div>
            <span class="m-meter-val" :class="getSaturationClass(getHw(host).cpu_percent)">
              {{ getHw(host).cpu_percent.toFixed(0) }}%
            </span>
          </div>

          <div class="m-telemetry-meter">
            <span class="m-meter-lbl">RAM</span>
            <div class="m-meter-track">
              <div
                class="m-meter-fill"
                :class="getSaturationClass(getHw(host).memory_percent)"
                :style="{ width: `${Math.min(100, Math.max(0, getHw(host).memory_percent))}%` }"
              ></div>
            </div>
            <span class="m-meter-val" :class="getSaturationClass(getHw(host).memory_percent)">
              {{ getHw(host).memory_percent.toFixed(0) }}%
            </span>
          </div>

          <div class="m-telemetry-meter">
            <span class="m-meter-lbl">DISK</span>
            <div class="m-meter-track">
              <div
                class="m-meter-fill"
                :class="getSaturationClass(getHw(host).disk_percent)"
                :style="{ width: `${Math.min(100, Math.max(0, getHw(host).disk_percent))}%` }"
              ></div>
            </div>
            <span class="m-meter-val" :class="getSaturationClass(getHw(host).disk_percent)">
              {{ getHw(host).disk_percent.toFixed(0) }}%
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mobile-hosts-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.mobile-pagination-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 10px;
  margin-bottom: 6px;
  font-size: 11px;
}
.m-pagination-controls {
  display: flex;
  gap: 6px;
  align-items: center;
}
.mobile-hosts-stream {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.mobile-host-card {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  min-height: 68px;
  height: auto;
  padding: 12px;
  border-radius: 8px;
  background: var(--bg-card);
  border: 1px solid var(--color-border-subtle);
  cursor: pointer;
  box-sizing: border-box;
  width: 100%;
  transition: background 0.15s ease, border-color 0.15s ease;
}
.mobile-host-card:hover {
  background: var(--bg-elevated);
  border-color: var(--color-border-strong);
}
.mobile-host-card:active {
  transform: scale(0.99);
}
.m-card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  min-width: 0;
}
.m-card-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
}
.m-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  display: inline-block;
}
.m-status-dot.dot-connected { background: var(--color-status-online); }
.m-status-dot.dot-disconnected { background: var(--color-status-offline); }
.m-host-name {
  font-family: var(--font-sans);
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  line-height: 1.2;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: -0.01em;
}
.m-type-chip {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
  flex-shrink: 0;
  white-space: nowrap;
  text-transform: uppercase;
  color: var(--color-badge-agent-text, #38bdf8);
  background: var(--color-badge-agent-bg, rgba(56, 189, 248, 0.15));
  border: 1px solid var(--color-badge-agent-border, rgba(56, 189, 248, 0.40));
}
.m-type-chip.badge-k8s { color: var(--color-badge-k8s-text); background: var(--color-badge-k8s-bg); border-color: var(--color-badge-k8s-border); }
.m-type-chip.badge-agent { color: var(--color-badge-agent-text); background: var(--color-badge-agent-bg); border-color: var(--color-badge-agent-border); }
.m-type-chip.badge-docker { color: var(--color-badge-docker-text); background: var(--color-badge-docker-bg); border-color: var(--color-badge-docker-border); }
.m-type-chip.badge-purple { color: var(--color-badge-metrics-text); background: var(--color-badge-metrics-bg); border-color: var(--color-badge-metrics-border); }
.m-card-header-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.btn-m-probe {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 28px;
  padding: 0 10px;
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 500;
  border-radius: 6px;
  background: var(--color-surface-inset);
  border: 1px solid var(--color-border-subtle);
  color: var(--text-primary);
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-m-probe:hover:not(:disabled) {
  background: var(--bg-elevated);
  color: var(--text-primary);
  border-color: var(--color-border-strong);
}
.btn-m-probe:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.m-card-meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  width: 100%;
  min-width: 0;
}
.m-meta-info-left {
  display: flex;
  align-items: center;
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
}
.m-endpoint {
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 500;
  color: var(--color-endpoint-syntax, #58a6ff);
  line-height: 1.3;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.m-meta-info-right {
  display: flex;
  align-items: center;
  font-size: 11px;
  flex-shrink: 0;
  white-space: nowrap;
}
.m-latency {
  font-weight: 500;
}
.m-last-check {
  font-family: var(--font-sans);
  font-size: 11px;
  font-weight: 400;
  color: var(--text-muted);
  line-height: 1.3;
}
.m-card-telemetry-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  padding: 6px 8px;
  background: var(--color-surface-inset);
  border: 1px solid var(--color-hairline);
  border-radius: 6px;
  width: 100%;
  box-sizing: border-box;
}
.m-telemetry-meter {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}
.m-meter-lbl {
  font-size: 9.5px;
  font-weight: 700;
  color: var(--text-muted);
  flex-shrink: 0;
}
.m-meter-track {
  flex: 1 1 auto;
  height: 4px;
  background: var(--color-surface-card);
  border-radius: 999px;
  overflow: hidden;
  border: 1px solid var(--color-border-subtle);
  min-width: 20px;
}
.m-meter-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.3s ease;
}
.m-meter-fill.m-meter-healthy { background: var(--color-status-online); }
.m-meter-fill.m-meter-warning { background: var(--color-status-warning); }
.m-meter-fill.m-meter-critical { background: var(--color-status-offline); }

.m-meter-val {
  font-size: 10px;
  font-weight: 700;
  flex-shrink: 0;
  min-width: 26px;
  text-align: right;
  padding: 0 3px;
  border-radius: 3px;
  line-height: 1.2;
}

.m-meter-val.m-meter-healthy {
  background: var(--color-status-online-bg);
  border: 1px solid var(--color-status-online-border);
  color: var(--color-status-online);
}
.m-meter-val.m-meter-warning {
  background: var(--color-status-warning-bg);
  border: 1px solid var(--color-status-warning-border);
  color: var(--color-status-warning);
}
.m-meter-val.m-meter-critical {
  background: var(--color-status-offline-bg);
  border: 1px solid var(--color-status-offline-border);
  color: var(--color-status-offline);
}
</style>
