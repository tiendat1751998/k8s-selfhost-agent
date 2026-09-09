<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTestHistoryItem, HostTypeDefinition } from '../../types/hosts'

defineProps<{
  show: boolean
  host: ComputeHost | null
  hostTestResult?: HostTestResult
  hostTestHistories?: HostTestHistoryItem[]
  testingHostId: string | null
  getHostTypeMeta: (type?: string) => HostTypeDefinition
  formatDate: (d?: string) => string
  formatUptime: (seconds?: number) => string
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'test', host: ComputeHost): void
  (e: 'edit', host: ComputeHost): void
  (e: 'delete', host: ComputeHost): void
  (e: 'copy', text: string): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    :title="host ? `${getHostTypeMeta(host.host_type).icon} ${host.name}` : 'Host Details'"
    subtitle="Complete host specifications, connectivity history, and operational telemetry"
    max-width="640px"
    @update:show="emit('update:show', $event)"
  >
    <div v-if="host" class="drawer-content-body font-mono">
      <!-- Top Status Card -->
      <div class="drawer-hero-card glass-panel">
        <div class="hero-header">
          <div>
            <h2 class="hero-name">{{ host.name }}</h2>
            <span class="hero-id text-muted">{{ host.id }}</span>
          </div>
          <div class="hero-badges">
            <span class="type-badge" :class="getHostTypeMeta(host.host_type).badgeClass">
              {{ getHostTypeMeta(host.host_type).label }}
            </span>
            <StatusBadge :status="host.status || 'connected'" size="md" />
          </div>
        </div>

        <div class="hero-endpoint-row">
          <span class="text-cyan">{{ host.endpoint }}</span>
          <button class="btn-copy-mini" title="Copy endpoint" @click="emit('copy', host.endpoint)">📋</button>
        </div>
      </div>

      <!-- Specifications Grid -->
      <div class="drawer-section">
        <h3 class="section-title">SPECIFICATIONS & SECURITY</h3>
        <div class="spec-grid glass-panel">
          <div class="spec-cell">
            <span class="spec-lbl">HOST TYPE</span>
            <span class="spec-val text-primary">{{ (host.host_type || 'agent').toUpperCase() }}</span>
          </div>
          <div class="spec-cell">
            <span class="spec-lbl">TLS AUTH (mTLS)</span>
            <span class="spec-val" :class="host.tls_enabled ? 'text-emerald' : 'text-muted'">
              {{ host.tls_enabled ? 'ENABLED' : 'DISABLED' }}
            </span>
          </div>
          <div class="spec-cell">
            <span class="spec-lbl">API VERSION</span>
            <span class="spec-val">{{ host.api_version || 'auto-negotiate' }}</span>
          </div>
          <div class="spec-cell">
            <span class="spec-lbl">TENANT ID</span>
            <span class="spec-val">{{ host.tenant_id || 'default-tenant' }}</span>
          </div>
          <div class="spec-cell">
            <span class="spec-lbl">REGISTERED AT</span>
            <span class="spec-val">{{ formatDate(host.created_at) }}</span>
          </div>
          <div class="spec-cell">
            <span class="spec-lbl">LAST HEALTH CHECK</span>
            <span class="spec-val">{{ formatDate(host.last_health_check) }}</span>
          </div>
        </div>
      </div>

      <!-- Live System Telemetry (if agent info available) -->
      <div v-if="hostTestResult?.agent_info" class="drawer-section">
        <h3 class="section-title">LIVE SYSTEM TELEMETRY</h3>
        <div class="metrics-agent-box glass-panel">
          <div class="telemetry-item">
            <span class="telemetry-lbl">HOSTNAME:</span>
            <span class="telemetry-val font-bold text-cyan">{{ hostTestResult.agent_info?.hostname }}</span>
          </div>
          <div class="telemetry-item">
            <span class="telemetry-lbl">OS / DISTRO:</span>
            <span class="telemetry-val">{{ hostTestResult.agent_info?.os_distro || hostTestResult.agent_info?.os }} ({{ hostTestResult.agent_info?.arch }})</span>
          </div>
          <div v-if="hostTestResult.agent_info?.kernel_version" class="telemetry-item">
            <span class="telemetry-lbl">KERNEL:</span>
            <span class="telemetry-val font-mono">{{ hostTestResult.agent_info?.kernel_version }}</span>
          </div>
          <div class="telemetry-item">
            <span class="telemetry-lbl">UPTIME:</span>
            <span class="telemetry-val text-emerald">{{ formatUptime(hostTestResult.agent_info?.uptime || hostTestResult.agent_info?.uptime_seconds) }}</span>
          </div>
        </div>
      </div>

      <!-- Placement Labels -->
      <div class="drawer-section">
        <h3 class="section-title">PLACEMENT LABELS</h3>
        <div v-if="host.labels && Object.keys(host.labels).length > 0" class="labels-container glass-panel">
          <div v-for="(val, key) in host.labels" :key="key" class="label-chip">
            <span class="chip-key text-cyan">{{ key }}</span>
            <span class="chip-eq">=</span>
            <span class="chip-val text-primary">{{ val }}</span>
          </div>
        </div>
        <div v-else class="text-muted text-sm glass-panel" style="padding: 12px;">
          No placement labels assigned to this host.
        </div>
      </div>

      <!-- Connection History (Last 10 tests) -->
      <div class="drawer-section">
        <div class="section-header-flex">
          <h3 class="section-title">RECENT TEST HISTORY</h3>
          <button
            class="btn btn-secondary btn-xs"
            :disabled="testingHostId === host.id"
            @click="emit('test', host)"
          >
            <span>{{ testingHostId === host.id ? '⏳ Testing...' : '⚡ Test Now' }}</span>
          </button>
        </div>

        <div v-if="hostTestHistories && hostTestHistories.length > 0" class="history-list glass-panel">
          <div
            v-for="(hist, hIdx) in hostTestHistories"
            :key="hIdx"
            class="history-row"
            :class="hist.status === 'ok' ? 'hist-pass' : 'hist-fail'"
          >
            <div class="hist-left">
              <span class="hist-dot"></span>
              <span>{{ hist.status === 'ok' ? 'CONNECTED' : 'ERROR' }}</span>
              <span class="hist-lat font-bold">⚡ {{ hist.latency_ms }}ms</span>
            </div>
            <span class="hist-time text-muted">{{ formatDate(hist.timestamp.toISOString()) }}</span>
          </div>
        </div>
        <div v-else class="text-muted text-sm glass-panel" style="padding: 12px;">
          No test runs recorded during this session. Click "Test Now" to probe endpoint.
        </div>
      </div>

      <!-- Drawer Footer Actions -->
      <div class="drawer-footer-actions">
        <button class="btn btn-secondary" @click="emit('edit', host)">
          <span>✏️ Edit Host</span>
        </button>
        <button class="btn btn-danger-outline" @click="emit('delete', host)">
          <span>🗑️ Delete Host</span>
        </button>
      </div>
    </div>
  </ModalDrawer>
</template>
