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
    return { label: '❨️ K8S', class: 'badge-blue' }
  }
  if (env.includes('docker') || env.includes('container')) {
    return { label: '🐱 DOCKER', class: 'badge-cyan' }
  }
  return { label: '💻 BARE-METAL', class: 'badge-purple' }
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
          [ {{ runtimeBadge.label }} ]
        </span>
        <span v-if="isDatabaseRole" class="role-badge font-mono badge-amber">
          [ 🔩 DATABASE ]
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
          {{ host.tls_enabled ? '🔴 mTLS' : '🔥 Standard' }}
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
          <span>п／／ Delete</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.host-card { padding: 14px; border-radius: 12px; display: flex; flex-direction: column; gap: 10px; background: rgba(11, 15, 25, 0.7); border: 1px solid var(--border-subtle); transition: all 0.2s ease; }
.host-card:hover { border-color: var(--border-medium); }
.card-top { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; }
.card-title-group { display: flex; align-items: center; gap: 10px; cursor: pointer; flex: 1; min-width: 0; }
.card-type-icon { font-size: 20px; }
.card-name-wrap { display: flex; flex-direction: column; min-width: 0; }
.card-host-name { font-size: 14px; font-weight: 700; color: #fff; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.card-host-id { font-size: 10.5px; }
.card-badges { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.type-badge { font-size: 10px; font-weight: 700; padding: 2px 6px; border-radius: 4px; }
.badge-blue { background: rgba(59, 130, 246, 0.15); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.3); }
.badge-cyan { background: rgba(6, 182, 212, 0.15); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.3); }
.badge-purple { background: rgba(168, 85, 247, 0.15); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.3); }
.role-badge { font-size: 10px; font-weight: 700; padding: 2px 6px; border-radius: 4px; }
.badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.card-endpoint-box { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 5px 8px; border-radius: 6px; background: rgba(0, 0, 0, 0.3); border: 1px solid var(--border-subtle); }
.endpoint-text { font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.btn-copy-sm { background: none; border: none; cursor: pointer; padding: 0; font-size: 12px; }
.card-meta-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px; background: rgba(0, 0, 0, 0.2); padding: 8px; border-radius: 8px; }
.meta-item { display: flex;
  flex-direction: column; gap: 2px; }
.meta-item-full { grid-column: 1 / -1; }
.meta-lbl { font-size: 9.5px; color: var(--text-muted); }
.meta-val { font-size: 11.5px; font-weight: 600; }
.test-result-bar { padding: 6px 8px; border-radius: 6px; display: flex; flex-direction: column; gap: 4px; font-size: 11px; }
.test-pass { background: rgba(16, 185, 129, 0.1); border: 1px solid rgba(16, 185, 129, 0.25); color: #34d399; }
.test-fail { background: rgba(244, 63, 94, 0.1); border: 1px solid rgba(244, 63, 94, 0.25); color: #fb7185; }
.test-top { display: flex; justify-content: space-between; align-items: center; }
.test-status-tag { font-size: 9.5px; font-weight: 700; padding: 1px 4px; border-radius: 3px; background: rgba(0,0,0,0.3); }
-agent-telemetry-mini { display: flex; flex-wrap: wrap; gap: 6px; font-size: 10px; color: var(--text-secondary); }
.card-labels-list { display: flex; flex-wrap: wrap; gap: 4px; }
.host-tag { font-size: 10px; background: rgba(255, 255, 255, 0.05); padding: 2px 6px; border-radius: 4px; border: 1px solid var(--border-subtle); color: var(--text-secondary); }
.card-footer { display: flex; justify-content: space-between; align-items: center; gap: 8px; border-top: 1px solid var(--border-subtle); padding-top: 8px; }
.footer-btn-group { display: flex; gap: 6px; }
.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.btn-danger-outline { border: 1px solid rgba(244, 63, 94, 0.4); color: #fb7185; background: transparent; }
.btn-danger-outline:hover { background: rgba(244, 63, 94, 0.15); }
</style>
