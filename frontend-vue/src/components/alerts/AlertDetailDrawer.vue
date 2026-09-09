<script setup lang="ts">
import type { AlertHistory } from '../../api/management'
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  show: boolean
  alert: AlertHistory | null
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'acknowledge', alert: AlertHistory): void
  (e: 'silence', alert: AlertHistory, mins: number): void
  (e: 'resolve', alert: AlertHistory): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    title="Alert Telemetry & Incident Runbook"
    subtitle="Detailed anomaly telemetry, metric timeline, and operational runbook linkage."
    @update:show="emit('update:show', $event)"
  >
    <div v-if="alert" class="detail-drawer-content">
      <div class="flex justify-between items-center pb-3 border-b border-[var(--border-subtle)]">
        <div>
          <span class="font-mono text-cyan text-sm font-bold">{{ alert.ID }}</span>
          <div class="text-sm font-semibold text-white mt-1">{{ alert.Message }}</div>
        </div>
        <StatusBadge 
          :status="alert.Status === 'firing' ? 'danger' : alert.Status === 'acknowledged' ? 'warning' : 'healthy'" 
          :label="alert.Status.toUpperCase()" 
        />
      </div>

      <div class="telemetry-graph-card">
        <div class="graph-header">
          <span class="text-cyan">Metric Telemetry Breach Graph</span>
          <span class="font-mono text-rose">Breach Value: {{ alert.Value }}</span>
        </div>
        <svg class="telemetry-svg" viewBox="0 0 300 80" preserveAspectRatio="none">
          <line x1="0" y1="35" x2="300" y2="35" stroke="#f43f5e" stroke-dasharray="4 4" stroke-width="1.5" />
          <text x="5" y="30" fill="#f43f5e" font-size="9" font-family="monospace">THRESHOLD (85%)</text>
          <path 
            d="M 0 65 Q 40 60, 80 58 T 160 55 T 220 50 T 260 20 L 300 15" 
            fill="none" 
            stroke="#38bdf8" 
            stroke-width="2.5" 
          />
          <circle cx="300" cy="15" r="5" fill="#f43f5e" />
          <circle cx="300" cy="15" r="9" fill="none" stroke="#f43f5e" opacity="0.5" />
        </svg>
      </div>

      <div class="runbook-box">
        <div>
          <div class="runbook-title">📖 SOP-RUNBOOK: {{ alert.RuleID || 'K8S-ANOMALY-TRIAGE' }}</div>
          <p class="runbook-desc">
            Standard Operating Procedure for handling anomaly breaches on cluster workloads.
          </p>
        </div>
        <button 
          class="btn btn-secondary btn-sm whitespace-nowrap"
          type="button"
          @click="emit('acknowledge', alert)"
        >
          <span>Runbook SOP →</span>
        </button>
      </div>

      <div>
        <h4 class="text-xs font-bold text-muted uppercase tracking-wider mb-3">Incident Timeline</h4>
        <div class="timeline-track">
          <div class="timeline-node done">
            <span class="timeline-title">Anomaly Threshold Breached</span>
            <span class="timeline-time">{{ new Date(alert.CreatedAt).toLocaleString() }}</span>
          </div>
          <div class="timeline-node done">
            <span class="timeline-title">Prometheus Alertmanager Dispatched to Channels</span>
            <span class="timeline-time">{{ new Date(alert.CreatedAt).toLocaleTimeString() }}</span>
          </div>
          <div class="timeline-node" :class="{ done: alert.Status === 'acknowledged' || alert.Status === 'resolved' }">
            <span class="timeline-title">
              {{ alert.AcknowledgedBy ? `Acknowledged by ${alert.AcknowledgedBy}` : 'Awaiting SRE Acknowledgement' }}
            </span>
            <span v-if="alert.UpdatedAt" class="timeline-time">{{ new Date(alert.UpdatedAt).toLocaleTimeString() }}</span>
          </div>
          <div class="timeline-node" :class="{ done: alert.Status === 'resolved' }">
            <span class="timeline-title">
              {{ alert.Status === 'resolved' ? 'Resolved & Restored to Healthy State' : 'Remediation Pending' }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <div class="flex justify-between items-center w-full" v-if="alert">
        <button class="btn btn-secondary btn-sm" type="button" @click="close">Close</button>
        <div class="flex gap-2">
          <button 
            v-if="alert.Status === 'firing'" 
            class="btn btn-secondary btn-sm" 
            @click="emit('silence', alert, 60)"
          >
            <span>🔕 Silence 1h</span>
          </button>
          <button 
            v-if="alert.Status === 'firing'" 
            class="btn btn-cyan btn-sm" 
            @click="emit('acknowledge', alert)"
          >
            <span>✓ Acknowledge</span>
          </button>
          <button 
            v-if="alert.Status !== 'resolved'" 
            class="btn btn-primary btn-sm" 
            @click="emit('resolve', alert)"
          >
            <span>⚡ Resolve</span>
          </button>
        </div>
      </div>
    </template>
  </ModalDrawer>
</template>
