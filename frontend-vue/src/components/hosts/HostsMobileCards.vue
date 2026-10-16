<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition } from '../../types/hosts'
import { formatDate as defaultFormatDate, formatHostLastSeen } from '../../types/hosts'

const props = defineProps<{
  hosts: ComputeHost[]
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

function prevPage() {
  if (currentPage.value > 1) currentPage.value--
}
function nextPage() {
  if (currentPage.value < totalPages.value) currentPage.value++
}

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
              class="btn-m-probe font-mono"
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

          <div class="m-meta-info-right font-mono">
            <span
              v-if="hostTestResults[host.id]"
              class="m-latency"
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
  gap: 6px;
}
.mobile-host-card {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 6px;
  min-height: 62px;
  height: auto;
  padding: 10px 12px;
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
  gap: 6px;
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
}
.m-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
  display: inline-block;
}
.m-status-dot.dot-connected {
  background: var(--color-status-online, #3fb950);
}
.m-status-dot.dot-disconnected {
  background: var(--color-status-offline, #f85149);
}
.m-host-name {
  font-size: 13.5px;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: -0.01em;
}
.m-type-chip {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  flex-shrink: 0;
  white-space: nowrap;
  text-transform: uppercase;
}
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
  height: 24px;
  padding: 0 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 4px;
  background: var(--bg-elevated);
  border: 1px solid var(--color-border-strong);
  color: var(--text-primary);
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-m-probe:hover:not(:disabled) {
  background: var(--color-surface-elevated, #222f46);
  color: var(--text-primary, #f8fafc);
  border-color: var(--color-border-strong, #384b6e);
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
  font-family: var(--font-mono, monospace);
  font-size: 11.5px;
  color: var(--color-endpoint-syntax, #58a6ff);
  font-weight: 500;
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
  font-size: 10.5px;
}
</style>
