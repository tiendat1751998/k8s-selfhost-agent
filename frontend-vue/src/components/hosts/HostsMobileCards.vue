<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition } from '../../types/hosts'
import { formatDate as defaultFormatDate } from '../../types/hosts'

const props = defineProps<{
  hosts: ComputeHost[]
  hostTestResults: Record<string, HostTestResult>
  testingHostId: string | null
  getHostTypeMeta: (type?: string) => HostTypeDefinition
  getLatencyBadgeClass: (latency?: number) => string
  formatDate?: (d?: string) => string
}>()

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
  <div class="mobile-hosts-stream animate-fade-in">
    <div
      v-for="host in hosts"
      :key="host.id"
      class="mobile-host-card glass-panel"
      @click="emit('select', host)"
    >
      <!-- Header Row: Type icon + Host Name + Role Chip on Left, StatusBadge on Right -->
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
        <div class="m-card-header-right">
          <StatusBadge :status="host.status || 'connected'" size="sm" />
        </div>
      </div>

      <!-- Meta Row: Endpoint / IP on Left, Latency or Last Check on Right -->
      <div class="m-card-meta-row">
        <div class="m-meta-endpoint-group">
          <BaseIcon name="server" size="xs" class="text-muted flex-shrink-0" />
          <span
            class="m-endpoint font-mono text-cyan"
            :title="host.endpoint || host.labels?.['kubernetes.io/hostname'] || host.name"
          >
            {{ host.endpoint || host.labels?.['kubernetes.io/hostname'] || host.name }}
          </span>
        </div>
        <div class="m-meta-telemetry-group">
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
            Last check: {{ formatHostDate(host.last_health_check) }}
          </span>
        </div>
      </div>

      <!-- Footer Action Bar: Standardized, Comfortable Touch Buttons -->
      <div class="m-card-footer-row" @click.stop>
        <button
          type="button"
          class="btn-m-action btn-m-test"
          :disabled="testingHostId === host.id"
          title="Test Connection"
          @click.stop="emit('test', host)"
        >
          <BaseIcon
            :name="testingHostId === host.id ? 'refresh' : 'zap'"
            size="xs"
            :class="{ 'animate-spin': testingHostId === host.id }"
          />
          <span>{{ testingHostId === host.id ? 'Testing' : 'Test' }}</span>
        </button>
        <button
          type="button"
          class="btn-m-action btn-m-edit"
          title="Edit Host"
          @click.stop="emit('edit', host)"
        >
          <BaseIcon name="edit" size="xs" />
          <span>Edit</span>
        </button>
        <button
          type="button"
          class="btn-m-action btn-m-delete"
          title="Delete Host"
          @click.stop="emit('delete', host)"
        >
          <BaseIcon name="trash" size="xs" />
          <span>Delete</span>
        </button>
        <button
          v-if="host.status !== 'connected'"
          type="button"
          class="btn-m-action btn-m-fence"
          title="Safe Storage Fencing"
          @click.stop="emit('fencing', host)"
        >
          <BaseIcon name="shield" size="xs" />
          <span>Fence</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/infra-hosts.css';
</style>
