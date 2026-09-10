<script setup lang="ts">
import { computed } from 'vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { ComputeHost } from '../../api/compute'

const props = defineProps<{
  host: ComputeHost
  testResult?: any
  isTesting?: boolean
}>()

const emit = defineEmits<{
  (e: 'select', host: ComputeHost): void
  (e: 'test', host: ComputeHost): void
  (e: 'edit', host: ComputeHost): void
  (e: 'delete', host: ComputeHost): void
  (e: 'copy', text: string): void
}>()

const runtimeBadge = computed(() => {
  const env = (props.host.runtime_environment || props.host.host_type || '').toLowerCase()
  if (env.includes('k8s') || env.includes('kube')) {
    return { label: 'K8S', class: 'badge-slate text-muted' }
  }
  if (env.includes('docker') || env.includes('container')) {
    return { label: 'DOCKER', class: 'badge-slate text-muted' }
  }
  return { label: 'BARE-METAL', class: 'badge-slate text-muted' }
})

const isDatabaseRole = computed(() => {
  const role = (props.host.host_role || props.host.labels?.role || '').toLowerCase()
  return role === 'database' || role === 'db'
})

function formatDate(d?: string) {
  if (!d) return 'Never'
  try {
    const dt = new Date(d)
    const diffSec = Math.floor((Date.now() - dt.getTime()) / 1000)
    if (diffSec < 60) return 'Just now'
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
    return dt.toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}

function formatUptime(seconds?: number): string {
  if (!seconds || seconds <= 0) return '0s'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}
</script>

<template>
  <div class="host-card glass-panel" :class="`host-card-${host.host_type || 'agent'}`">
    <div class="card-top">
      <div class="card-title-group" @click="emit('select', host)">
        <span class="card-type-icon">💹</span>
        <div class="card-name-wrap">
          <h3 class="card-host-name">{{ host.name }}</h3>
          <span class="card-host-id font-mono text-muted">{{ host.id }}</span>
        </div>
      </div>

      <div class="card-badges">
        <span class="type-badge font-mono" :class="runtimeBadge.class">
          {{ runtimeBadge.label }}
        </span>
        <span v-if="isDatabaseRole" class="role-badge font-mono badge-slate text-muted">
          DATABASE
        </span>
        <StatusBadge :status="host.status || 'connected'" size="sm" />
      </div>
    </div>

    <div class="card-endpoint-box">
      <span class="endpoint-text font-mono text-cyan" :title="host.endpoint">{{ host.endpoint }}</span>
      <button class="btn-copy-sm" title="Copy endpoint URL" @click.stop="emit('copy', host.endpoint)">📋</button>
    </div>

    <div class="card-meta-grid font-mono">
      <div class="meta-item">
        <span class="meta-lbl">LAST SEEN:</span>
        <span class="meta-val text-primary">{{ formatDate(host.last_health_check || host.created_at) }}</span>
      </div>

      <div class="meta-item">
        <span class="meta-lbl">SECURITY:</span>
        <span class="meta-val" :class="host.tls_enabled ? 'text-emerald' : 'text-muted'">
          {{ host.tls_enabled ? 'mTLS' : 'Standard' }}
        </span>
      </div>

      <div v-if="testResult" class="meta-item-full test-result-bar animate-fade-in" :class="testResult.status === 'ok' ? 'test-pass' : 'test-fail'">
        <div class="test-top">
          <span>⚡ Latency: <strong>{{ testResult.latency_ms }}ms</strong></span>
          <span class="test-status-tag">{{ (testResult.status || '').toUpperCase() }}</span>
        </div>
        <div v-if="testResult.agent_info" class="agent-telemetry-mini">
          <span v-if="testResult.agent_info?.hostname">💻 {{ testResult.agent_info?.hostname }}</span>
          <span v-if="testResult.agent_info?.os_distro || testResult.agent_info?.os">
            🐧 {{ testResult.agent_info?.os_distro || testResult.agent_info?.os }} ({{ testResult.agent_info?.arch }})
          </span>
          <span v-if="testResult.agent_info?.uptime || testResult.agent_info?.uptime_seconds">
            ⏱ {{ formatUptime(testResult.agent_info?.uptime || testResult.agent_info?.uptime_seconds) }}
          </span>
        </div>
      </div>
    </div>

    <div v-if="host.labels && Object.keys(host.labels).length > 0" class="card-labels-list">
      <span v-for="(val, key) in host.labels" :key="key" class="host-tag font-mono" :title="`key}=${val}`">
        {{ key }}={{ val }}
      </span>
    </div>

    <div class="card-footer">
      <button class="btn btn-secondary btn-xs" :disabled="isTesting" @click="emit('test', host)">
        <span>{{ isTesting ? '⃳ Testing...' : '⚡ Test Connection' }}</span>
      </button>

      <div class="footer-btn-group">
        <button class="btn btn-secondary btn-xs" title="Edit Host" @click="emit('edit', host)">
          <span>✏️ Edit</span>
        </button>
        <button class="btn btn-danger-outline btn-xs" title="Delete Host" @click="emit('delete', host)">
          <span>[🗑 Delete]</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/infra-hosts.css';
</style>
