<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition } from '../../types/hosts'

defineProps<{
  hosts: ComputeHost[]
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
</script>

<template>
  <div class="hosts-grid animate-fade-in">
    <div
      v-for="host in hosts"
      :key="host.id"
      class="host-card glass-panel"
      :class="`host-card-${getHostTypeMeta(host.host_type).color}`"
    >
      <div class="card-top">
        <div class="card-title-group" @click="emit('select', host)">
          <span class="card-type-icon">{{ getHostTypeMeta(host.host_type).icon }}</span>
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
        <span class="endpoint-text font-mono text-cyan" :title="host.endpoint">{{ host.endpoint }}</span>
        <button class="btn-copy-sm" title="Copy endpoint URL" @click="emit('copy', host.endpoint)">📋</button>
      </div>

      <div class="card-meta-grid font-mono">
        <div class="meta-item">
          <span class="meta-lbl">LAST SEEN:</span>
          <span class="meta-val text-primary">{{ formatDate(host.last_health_check || host.created_at) }}</span>
        </div>
        <div class="meta-item">
          <span class="meta-lbl">SECURITY:</span>
          <span class="meta-val" :class="host.tls_enabled ? 'text-emerald' : 'text-muted'">
            {{ host.tls_enabled ? '🔒 mTLS' : '🔓 Standard' }}
          </span>
        </div>
        <div v-if="hostTestResults[host.id]" class="meta-item-full test-result-bar animate-fade-in" :class="hostTestResults[host.id].status === 'ok' ? 'test-pass' : 'test-fail'">
          <div class="test-top">
            <span v-if="hostTestResults[host.id].latency_ms > 0">⚡ Latency: <strong>{{ hostTestResults[host.id].latency_ms }}ms</strong></span>
            <span v-else>⚡ Latency: <strong class="text-muted">--</strong></span>
            <span class="test-status-tag">{{ hostTestResults[host.id].status.toUpperCase() }}</span>
          </div>
          <div v-if="hostTestResults[host.id].agent_info" class="agent-telemetry-mini">
            <span v-if="hostTestResults[host.id].agent_info?.hostname">🏷️ {{ hostTestResults[host.id].agent_info?.hostname }}</span>
            <span v-if="hostTestResults[host.id].agent_info?.os_distro || hostTestResults[host.id].agent_info?.os">
              💻 {{ hostTestResults[host.id].agent_info?.os_distro || hostTestResults[host.id].agent_info?.os }} ({{ hostTestResults[host.id].agent_info?.arch }})
            </span>
            <span v-if="hostTestResults[host.id].agent_info?.uptime || hostTestResults[host.id].agent_info?.uptime_seconds">
              ⏱️ {{ formatUptime(hostTestResults[host.id].agent_info?.uptime || hostTestResults[host.id].agent_info?.uptime_seconds) }}
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
          <span>{{ testingHostId === host.id ? '⏳ Testing...' : '⚡ Test Connection' }}</span>
        </button>
        <div class="footer-btn-group">
          <button class="btn btn-secondary btn-xs" title="Edit Host" @click="emit('edit', host)">
            <span>✏️ Edit</span>
          </button>
          <button class="btn btn-danger-outline btn-xs" title="Delete Host" @click="emit('delete', host)">
            <span>🗑️ Delete</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
