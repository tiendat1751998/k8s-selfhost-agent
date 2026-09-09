<script setup lang="ts">
import type { DetectedTool } from '../../api/ecosystem'
import type {
  HealthProbeResult,
  WebhookSyncRecord,
  ToolErrorLog
} from '../../composables/useEcosystem'

interface Props {
  show: boolean
  tool: DetectedTool | null
  probeResult: HealthProbeResult | null
  isProbing: boolean
  webhookHistory: WebhookSyncRecord[]
  errorLogs: ToolErrorLog[]
  getToolIcon: (tool: DetectedTool) => string
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'probe', tool: DetectedTool): void
  (e: 'sync', tool: DetectedTool): void
}>()
</script>

<template>
  <div v-if="show && tool" class="drawer-backdrop" @click.self="emit('close')">
    <div class="health-drawer">
      <!-- Drawer Header -->
      <div class="drawer-header">
        <div class="drawer-title-group">
          <span style="font-size: 24px;">{{ getToolIcon(tool) }}</span>
          <div>
            <h2>{{ tool.name }}</h2>
            <span class="category-badge">{{ tool.category.toUpperCase() }}</span>
          </div>
        </div>
        <button class="modal-close-btn" @click="emit('close')">✕</button>
      </div>

      <!-- Latency & Health Probe Section -->
      <div class="drawer-section">
        <div class="drawer-section-title">
          <span>📡 Latency Probe & Diagnostics</span>
          <button
            class="table-btn btn-health"
            :disabled="isProbing"
            @click="emit('probe', tool)"
          >
            <span :class="{ 'spin-anim': isProbing }">🔄</span>
            <span>{{ isProbing ? 'Probing...' : 'Run Probe Now' }}</span>
          </button>
        </div>

        <div v-if="probeResult" class="probe-metrics-grid">
          <div class="probe-metric-box">
            <span class="probe-metric-lbl">Response Latency</span>
            <span
              class="probe-metric-val font-mono"
              :style="{ color: probeResult.latencyMs < 100 ? '#34d399' : probeResult.latencyMs < 500 ? '#fbbf24' : '#f43f5e' }"
            >
              {{ probeResult.latencyMs }} ms
            </span>
          </div>
          <div class="probe-metric-box">
            <span class="probe-metric-lbl">HTTP Status</span>
            <span class="probe-metric-val font-mono" style="color: #38bdf8;">
              {{ probeResult.statusCode }}
            </span>
          </div>
          <div class="probe-metric-box">
            <span class="probe-metric-lbl">DNS Lookup</span>
            <span class="probe-metric-val font-mono" style="color: #a855f7;">
              {{ probeResult.dnsResolveMs }} ms
            </span>
          </div>
          <div class="probe-metric-box">
            <span class="probe-metric-lbl">TLS Expiry</span>
            <span class="probe-metric-val font-mono" style="color: #10b981;">
              {{ probeResult.tlsExpiryDays ? `${probeResult.tlsExpiryDays} days` : 'Valid' }}
            </span>
          </div>
        </div>

        <div v-if="probeResult" class="detail-row" style="margin-top: 6px; font-size: 12px; color: #94a3b8;">
          <span>{{ probeResult.message }}</span>
        </div>
      </div>

      <!-- Webhook Sync History Section -->
      <div class="drawer-section">
        <div class="drawer-section-title">
          <span>🔄 Webhook Sync Heartbeats</span>
          <button
            class="table-btn btn-sync"
            @click="emit('sync', tool)"
          >
            ⚡ Trigger Sync
          </button>
        </div>

        <div v-if="webhookHistory.length > 0" class="webhook-list">
          <div
            v-for="wh in webhookHistory"
            :key="wh.id"
            class="webhook-item"
          >
            <div class="webhook-item-header">
              <span class="webhook-event font-mono">{{ wh.event }}</span>
              <span
                class="status-pill"
                :class="wh.status === 'success' ? 'pill-healthy' : 'pill-degraded'"
              >
                {{ wh.status.toUpperCase() }} ({{ wh.durationMs }}ms)
              </span>
            </div>
            <div style="font-size: 11.5px; color: #94a3b8;">
              {{ wh.details }} · <span class="font-mono text-muted">{{ wh.payloadSize }}</span>
            </div>
          </div>
        </div>
        <div v-else class="text-muted" style="font-size: 12px; padding: 12px; text-align: center;">
          No recent webhook sync activities recorded.
        </div>
      </div>

      <!-- Error Logs Stream Section -->
      <div class="drawer-section">
        <div class="drawer-section-title">
          <span>📋 Error & Discovery Logs</span>
        </div>

        <div v-if="errorLogs.length > 0" class="error-logs-terminal">
          <div
            v-for="log in errorLogs"
            :key="log.id"
            class="log-line"
          >
            <span class="log-time">{{ new Date(log.timestamp).toLocaleTimeString() }}</span>
            <span :class="`log-level-${log.level}`">[{{ log.level.toUpperCase() }}]</span>
            <span class="log-msg">[{{ log.source }}] {{ log.message }}</span>
          </div>
        </div>
        <div v-else class="text-muted" style="font-size: 12px; padding: 12px; text-align: center;">
          No error events logged. Component operating within normal parameters.
        </div>
      </div>
    </div>
  </div>
</template>
