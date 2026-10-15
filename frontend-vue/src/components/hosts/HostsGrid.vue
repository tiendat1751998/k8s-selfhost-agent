<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition } from '../../types/hosts'

const props = defineProps<{
  hosts: ComputeHost[]
  hostTestResults: Record<string, HostTestResult>
  testingHostId: string | null
  getHostTypeMeta: (type?: string) => HostTypeDefinition
  formatDate: (d?: string) => string
  formatUptime: (seconds?: number) => string
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
  (e: 'copy', text: string): void
}>()
</script>

<template>
  <div class="hosts-grid-container">
    <div v-if="hosts.length > pageSize" class="pagination-bar glass-panel font-mono" style="display: flex; justify-content: space-between; align-items: center; padding: 8px 12px; margin-bottom: 12px; border-radius: 6px;">
      <span class="text-xs text-muted">Showing {{ startIndex + 1 }}–{{ endIndex }} of {{ hosts.length }} hosts</span>
      <div class="pagination-actions" style="display: flex; gap: 8px; align-items: center;">
        <button type="button" class="btn btn-secondary btn-xs" :disabled="currentPage <= 1" @click="prevPage">
          Prev
        </button>
        <span class="page-indicator text-xs font-bold">{{ currentPage }} / {{ totalPages }}</span>
        <button type="button" class="btn btn-secondary btn-xs" :disabled="currentPage >= totalPages" @click="nextPage">
          Next
        </button>
      </div>
    </div>

    <div class="hosts-grid animate-fade-in">
      <div
        v-for="host in paginatedHosts"
        :key="host.id"
        class="host-card glass-panel"
      :class="`host-card-${getHostTypeMeta(host.host_type).color}`"
    >
      <div class="card-top">
        <div class="card-title-group" @click="emit('select', host)">
          <span class="card-type-icon"><BaseIcon :name="getHostTypeMeta(host.host_type).icon" size="sm" /></span>
          <div class="card-name-wrap">
            <h3 class="card-host-name">{{ host.name }}</h3>
            <span class="card-host-id font-mono text-muted">{{ host.id }}</span>
          </div>
        </div>
        <div class="card-badges">
          <span class="type-badge font-mono" :class="getHostTypeMeta(host.host_type).badgeClass">
            {{ getHostTypeMeta(host.host_type).label }}
          </span>
          <StatusBadge :status="host.status || 'connected'" size="sm" />
        </div>
      </div>

      <div class="card-endpoint-box">
        <span class="endpoint-text font-mono text-secondary" :title="host.endpoint">{{ host.endpoint }}</span>
        <button class="btn-copy-sm" title="Copy endpoint URL" @click="emit('copy', host.endpoint)">
          <BaseIcon name="copy" size="xs" />
        </button>
      </div>

      <div class="card-meta-grid font-mono">
        <div class="meta-item">
          <span class="meta-lbl">LAST SEEN:</span>
          <span class="meta-val text-primary">{{ formatDate(host.last_health_check || host.created_at) }}</span>
        </div>
        <div class="meta-item">
          <span class="meta-lbl">SECURITY:</span>
          <span class="meta-val" :class="host.tls_enabled ? 'text-emerald' : 'text-muted'">
            <BaseIcon :name="host.tls_enabled ? 'lock' : 'shield'" size="xs" /> {{ host.tls_enabled ? 'mTLS' : 'Standard' }}
          </span>
        </div>
        <div v-if="hostTestResults[host.id]" class="meta-item-full test-result-bar animate-fade-in" :class="hostTestResults[host.id].status === 'ok' ? 'test-pass' : 'test-fail'">
          <div class="test-top">
            <span v-if="hostTestResults[host.id].latency_ms > 0"><BaseIcon name="activity" size="xs" /> Latency: <strong>{{ hostTestResults[host.id].latency_ms }}ms</strong></span>
            <span v-else><BaseIcon name="activity" size="xs" /> Latency: <strong class="text-muted">--</strong></span>
            <span class="test-status-tag">{{ hostTestResults[host.id].status.toUpperCase() }}</span>
          </div>
          <div v-if="hostTestResults[host.id].agent_info" class="agent-telemetry-mini">
            <span v-if="hostTestResults[host.id].agent_info?.hostname"><BaseIcon name="tag" size="xs" /> {{ hostTestResults[host.id].agent_info?.hostname }}</span>
            <span v-if="hostTestResults[host.id].agent_info?.os_distro || hostTestResults[host.id].agent_info?.os">
              <BaseIcon name="terminal" size="xs" /> {{ hostTestResults[host.id].agent_info?.os_distro || hostTestResults[host.id].agent_info?.os }} ({{ hostTestResults[host.id].agent_info?.arch }})
            </span>
            <span v-if="hostTestResults[host.id].agent_info?.uptime || hostTestResults[host.id].agent_info?.uptime_seconds">
              <BaseIcon name="clock" size="xs" /> {{ formatUptime(hostTestResults[host.id].agent_info?.uptime || hostTestResults[host.id].agent_info?.uptime_seconds) }}
            </span>
          </div>
        </div>
      </div>

      <div v-if="host.labels && Object.keys(host.labels).length > 0" class="card-labels-list">
        <span v-for="(val, key) in host.labels" :key="key" class="host-tag font-mono" :title="`${key}=${val}`">
          {{ key }}={{ val }}
        </span>
      </div>

      <div class="card-footer">
        <button class="btn btn-secondary btn-xs" :disabled="testingHostId === host.id" @click="emit('test', host)">
          <BaseIcon :name="testingHostId === host.id ? 'refresh' : 'activity'" size="xs" :class="{ 'animate-spin': testingHostId === host.id }" />
          <span>{{ testingHostId === host.id ? 'Testing...' : 'Test Connection' }}</span>
        </button>
        <div class="footer-btn-group">
          <button class="btn btn-secondary btn-xs" title="Edit Host" @click="emit('edit', host)">
            <BaseIcon name="edit" size="xs" />
            <span>Edit</span>
          </button>
          <button class="btn btn-danger-outline btn-xs" title="Delete Host" @click="emit('delete', host)">
            <BaseIcon name="trash" size="xs" />
            <span>Delete</span>
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

