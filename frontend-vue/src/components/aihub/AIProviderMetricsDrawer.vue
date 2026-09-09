<script setup lang="ts">
import { computed } from 'vue'
import type { AIProvider } from '../../api/management'
import type { ProviderQuota, LatencyTelemetry, FallbackRoute } from '../../composables/useAIProviderHub'
import ModalDrawer from '../ui/ModalDrawer.vue'
import CircularGauge from '../ui/CircularGauge.vue'
import StatusBadge from '../ui/StatusBadge.vue'

const props = defineProps<{
  show: boolean
  provider: AIProvider | null
  quota?: ProviderQuota
  telemetry?: LatencyTelemetry
  fallbackRoute?: FallbackRoute
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'updateRoute', partial: Partial<FallbackRoute>): void
}>()

const quotaPercent = computed(() => {
  if (!props.quota || !props.quota.maxTokens) return 25
  return Math.min(100, Math.round((props.quota.usedTokens / props.quota.maxTokens) * 100))
})

const maxLatencyHistory = computed(() => {
  if (!props.telemetry?.history?.length) return 100
  const max = Math.max(...props.telemetry.history.map(h => h.latencyMs))
  return Math.max(max, 50)
})

function getBarHeight(latencyMs: number) {
  return Math.max(10, Math.min(100, Math.round((latencyMs / maxLatencyHistory.value) * 100)))
}

function getBarColor(latencyMs: number) {
  if (latencyMs < 50) return '#10b981'
  if (latencyMs < 120) return '#f59e0b'
  return '#f43f5e'
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    placement="right"
    maxWidth="560px"
    :title="provider ? `Metrics: ${provider.name}` : 'Provider Telemetry & Quota'"
    subtitle="Real-time LLM gateway token consumption, cost modeling, and latency telemetry."
    @update:show="emit('update:show', $event)"
  >
    <div v-if="provider" class="metrics-drawer-content animate-fade-in">
      <!-- Provider Metadata Banner -->
      <div class="drawer-section">
        <div class="pstat-row">
          <span class="drawer-section-title">Gateway Target</span>
          <StatusBadge :status="provider.status || 'healthy'" />
        </div>
        <div class="drawer-stats-row">
          <div class="drawer-stat-item">
            <span class="drawer-stat-label">Model</span>
            <span class="drawer-stat-val text-cyan">{{ provider.model }}</span>
          </div>
          <div class="drawer-stat-item">
            <span class="drawer-stat-label">Architecture</span>
            <span class="drawer-stat-val text-violet">{{ provider.type }}</span>
          </div>
          <div class="drawer-stat-item">
            <span class="drawer-stat-label">Current Latency</span>
            <span class="drawer-stat-val text-emerald">{{ telemetry?.currentMs || 14 }}ms</span>
          </div>
        </div>
      </div>

      <!-- Token Quotas & Gauges -->
      <div class="drawer-section">
        <span class="drawer-section-title">Token Quota & Usage Matrix</span>
        <div style="display: flex; align-items: center; gap: 20px; margin: 8px 0;">
          <CircularGauge 
            :percent="quotaPercent" 
            :size="72"
            :strokeWidth="5"
            :color="quotaPercent > 80 ? 'rose' : quotaPercent > 60 ? 'amber' : 'cyan'" 
          />
          <div style="display: flex; flex-direction: column; gap: 4px; flex: 1;">
            <div class="pstat-row">
              <span class="text-muted">Quota Used:</span>
              <span class="font-mono text-cyan" style="font-weight: 700;">
                {{ ((quota?.usedTokens || 0) / 1000).toLocaleString() }}k / {{ ((quota?.maxTokens || 1000000) / 1000).toLocaleString() }}k tokens
              </span>
            </div>
            <div class="pstat-row">
              <span class="text-muted">Prompt vs Output:</span>
              <span class="font-mono text-secondary">
                {{ ((quota?.promptTokens || 0) / 1000).toFixed(0) }}k in / {{ ((quota?.completionTokens || 0) / 1000).toFixed(0) }}k out
              </span>
            </div>
            <div class="pstat-row">
              <span class="text-muted">Rate Limits:</span>
              <span class="font-mono text-muted">{{ quota?.rpmLimit || 500 }} RPM • {{ (quota?.tpmLimit || 100000) / 1000 }}k TPM</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Cost Modeling Breakdown -->
      <div class="drawer-section">
        <span class="drawer-section-title">Cost Breakdown & Budget</span>
        <div class="drawer-stats-row">
          <div class="drawer-stat-item">
            <span class="drawer-stat-label">Spend (Current Month)</span>
            <span class="drawer-stat-val text-emerald">${{ (quota?.costSpentUsd || 0).toFixed(3) }}</span>
          </div>
          <div class="drawer-stat-item">
            <span class="drawer-stat-label">Prompt / 1k Tokens</span>
            <span class="drawer-stat-val font-mono">${{ (quota?.costPer1kPrompt || 0).toFixed(4) }}</span>
          </div>
          <div class="drawer-stat-item">
            <span class="drawer-stat-label">Completion / 1k Tokens</span>
            <span class="drawer-stat-val font-mono">${{ (quota?.costPer1kCompletion || 0).toFixed(4) }}</span>
          </div>
        </div>
      </div>

      <!-- Latency Sparkline Graph -->
      <div class="drawer-section">
        <div class="pstat-row">
          <span class="drawer-section-title">Latency Telemetry (Probes)</span>
          <span class="font-mono text-muted" style="font-size: 11px;">
            P95: {{ telemetry?.p95Ms || 22 }}ms | P99: {{ telemetry?.p99Ms || 35 }}ms
          </span>
        </div>
        <div class="latency-sparkline-wrap">
          <div 
            v-for="(point, idx) in telemetry?.history || []" 
            :key="idx" 
            class="latency-bar-col"
            :title="`${point.timestamp}: ${point.latencyMs}ms`"
          >
            <div 
              class="latency-bar" 
              :style="{ 
                height: `${getBarHeight(point.latencyMs)}%`, 
                backgroundColor: getBarColor(point.latencyMs) 
              }"
            ></div>
          </div>
        </div>
      </div>

      <!-- Active Fallback Routing -->
      <div class="drawer-section" style="border-bottom: none;">
        <span class="drawer-section-title">Active Failover Routing</span>
        <div class="routing-rule-box font-mono" style="font-size: 11.5px;">
          <div class="pstat-row">
            <span class="text-muted">Routing Policy:</span>
            <span class="text-cyan font-bold">{{ fallbackRoute?.mode === 'auto' ? 'Dynamic Autonomous Circuit' : 'Static Fallback' }}</span>
          </div>
          <div class="pstat-row">
            <span class="text-muted">Target Secondary:</span>
            <span class="text-emerald">{{ fallbackRoute?.fallbackProviderName || 'Local Ollama Mesh' }}</span>
          </div>
          <div class="pstat-row">
            <span class="text-muted">Timeout Trigger:</span>
            <span>{{ fallbackRoute?.timeoutMs || 2500 }}ms ({{ fallbackRoute?.circuitThresholdFailures || 3 }} consecutive errors)</span>
          </div>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Close</button>
    </template>
  </ModalDrawer>
</template>
