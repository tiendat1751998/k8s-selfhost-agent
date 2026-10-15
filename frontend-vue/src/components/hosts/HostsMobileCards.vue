<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import StatusBadge from '../ui/StatusBadge.vue'
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
</script>

<template>
  <div class="mobile-hosts-container">
    <div v-if="hosts.length > pageSize" class="mobile-pagination-bar font-mono" style="display: flex; justify-content: space-between; align-items: center; padding: 6px 10px; margin-bottom: 8px; font-size: 11px;">
      <span class="text-muted">Showing {{ startIndex + 1 }}–{{ endIndex }} of {{ hosts.length }}</span>
      <div style="display: flex; gap: 6px; align-items: center;">
        <button type="button" class="btn btn-secondary btn-xs" :disabled="currentPage <= 1" @click="prevPage">Prev</button>
        <span>{{ currentPage }} / {{ totalPages }}</span>
        <button type="button" class="btn btn-secondary btn-xs" :disabled="currentPage >= totalPages" @click="nextPage">Next</button>
      </div>
    </div>

    <div class="mobile-hosts-stream animate-fade-in">
      <div
        v-for="host in paginatedHosts"
        :key="host.id"
        class="mobile-host-card glass-panel"
        @click="emit('select', host)"
      >
        <!-- Row 1: Left has Icon + Host Name + Type Chip; Right has ONLY StatusBadge -->
        <div class="m-card-header-row">
          <div class="m-card-header-left">
            <span class="m-card-icon">
              <BaseIcon :name="getHostTypeMeta(host.host_type).icon" size="xs" />
            </span>
            <span class="m-host-name font-bold text-white" :title="host.name">
              {{ host.name }}
            </span>
            <span class="m-type-chip badge-slate font-mono text-xs">
              {{ getHostTypeMeta(host.host_type).label }}
            </span>
          </div>
          <div class="m-card-header-right" @click.stop>
            <StatusBadge :status="host.status || 'connected'" size="sm" />
          </div>
        </div>

        <!-- Row 2: Left has Endpoint + Telemetry/LastSeen; Right has Action Buttons -->
        <div class="m-card-meta-row">
          <div class="m-meta-info-left">
            <div class="m-meta-endpoint-group">
              <BaseIcon name="server" size="xs" class="text-muted flex-shrink-0" />
              <span
                class="m-endpoint font-mono text-cyan"
                :title="host.endpoint || host.labels?.['kubernetes.io/hostname'] || host.name"
              >
                {{ host.endpoint || host.labels?.['kubernetes.io/hostname'] || host.name }}
              </span>
            </div>
            <span
              v-if="hostTestResults[host.id]"
              class="m-latency font-mono"
              :class="getLatencyBadgeClass(hostTestResults[host.id].latency_ms)"
            >
              <template v-if="hostTestResults[host.id].latency_ms > 0">
                <BaseIcon name="zap" size="xs" />
                <span>{{ hostTestResults[host.id].latency_ms }}ms</span>
              </template>
              <template v-else>
                <span>--</span>
              </template>
            </span>
            <span v-else class="m-last-check font-mono text-muted">
              {{ formatHostLastSeen(host, formatHostDate, true) }}
            </span>
          </div>

          <div class="m-meta-actions-right" @click.stop>
            <button
              type="button"
              class="btn-m-action btn-m-test btn-m-compact"
              :disabled="testingHostId === host.id"
              title="Test Connection"
              @click.stop="emit('test', host)"
            >
              <BaseIcon
                :name="testingHostId === host.id ? 'refresh' : 'zap'"
                size="xs"
                :class="{ 'animate-spin': testingHostId === host.id }"
              />
              <span class="btn-m-text">{{ testingHostId === host.id ? 'Testing' : 'Test' }}</span>
            </button>
            <button
              v-if="host.status !== 'connected'"
              type="button"
              class="btn-m-action btn-m-fence btn-m-compact"
              title="Safe Storage Fencing"
              @click.stop="emit('fencing', host)"
            >
              <BaseIcon name="shield" size="xs" />
              <span class="btn-m-text">Fence</span>
            </button>
            <button
              type="button"
              class="btn-m-action btn-m-icon-only btn-m-edit"
              title="Edit Host"
              @click.stop="emit('edit', host)"
            >
              <BaseIcon name="edit" size="xs" />
            </button>
            <button
              type="button"
              class="btn-m-action btn-m-icon-only btn-m-delete"
              title="Delete Host"
              @click.stop="emit('delete', host)"
            >
              <BaseIcon name="trash" size="xs" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>



<style scoped>
/* Inherited via InfraHostsView.vue: assets/styles/views/infra-hosts.css */
</style>
