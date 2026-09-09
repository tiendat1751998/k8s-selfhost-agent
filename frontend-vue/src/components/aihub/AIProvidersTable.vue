<script setup lang="ts">
import type { AIProvider } from '../../api/management'
import type { ProviderQuota, LatencyTelemetry, FallbackRoute } from '../../composables/useAIProviderHub'
import StatusBadge from '../ui/StatusBadge.vue'

const props = defineProps<{
  providers: AIProvider[]
  probingName: string | null
  healthResults: Record<string, { status: string; latency?: string; error?: string }>
  quotas?: Record<string, ProviderQuota>
  telemetries?: Record<string, LatencyTelemetry>
  fallbackRoutes?: Record<string, FallbackRoute>
}>()

const emit = defineEmits<{
  (e: 'probe', name: string): void
  (e: 'testInConsole', name: string): void
  (e: 'openRouting', provider: AIProvider): void
  (e: 'openMetrics', provider: AIProvider): void
  (e: 'remove', name: string): void
  (e: 'register'): void
}>()

function isHealthy(p: AIProvider) {
  const probe = props.healthResults[p.name]
  const s = (probe?.status || p.status || '').toLowerCase()
  return s === 'healthy' || s === 'ready' || s === 'active' || s === 'ok' || s === ''
}

function getQuotaPercent(name: string) {
  const q = props.quotas?.[name]
  if (!q || !q.maxTokens) return 30
  return Math.min(100, Math.round((q.usedTokens / q.maxTokens) * 100))
}

function getLatencyColor(name: string, p: AIProvider) {
  const raw = props.healthResults[name]?.latency || p.latency
  if (!raw) return '#94a3b8'
  const ms = parseInt(raw)
  if (isNaN(ms) || ms <= 0) return '#94a3b8'
  if (ms < 50) return '#10b981'
  if (ms < 150) return '#f59e0b'
  return '#f43f5e'
}
</script>

<template>
  <div class="ai-table-wrap animate-fade-in">
    <div v-if="providers.length > 0" class="ai-table-container glass-panel">
      <table class="ai-gateway-table">
        <thead>
          <tr>
            <th>Provider & Gateway</th>
            <th>Target Model</th>
            <th>Live Latency</th>
            <th>Token Quota</th>
            <th>Fallback Routing</th>
            <th>Circuit Status</th>
            <th style="text-align: right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in providers" :key="p.name">
            <!-- Provider Info -->
            <td>
              <div class="table-provider-cell">
                <div class="table-provider-name">
                  <span>{{ p.name }}</span>
                  <span v-if="p.default" class="badge badge-cyan" style="font-size: 9px; padding: 1px 4px;">DEFAULT</span>
                </div>
                <span class="font-mono text-muted" style="font-size: 10px;">{{ p.endpoint }}</span>
              </div>
            </td>

            <!-- Model Info -->
            <td>
              <div class="table-provider-cell">
                <span class="font-mono text-cyan" style="font-weight: 600;">{{ p.model }}</span>
                <span class="badge badge-violet" style="font-size: 9px; width: fit-content;">{{ p.type }}</span>
              </div>
            </td>

            <!-- Latency Dial -->
            <td>
              <div class="provider-latency-dial">
                <span class="latency-pulse-dot" :style="{ backgroundColor: getLatencyColor(p.name, p), color: getLatencyColor(p.name, p) }"></span>
                <span class="font-mono" :style="{ color: getLatencyColor(p.name, p), fontWeight: '700' }">
                  {{ healthResults[p.name]?.latency || p.latency || '--' }}
                </span>
              </div>
            </td>

            <!-- Quota Bar -->
            <td style="min-width: 150px;">
              <div class="provider-quota-bar-wrap">
                <div class="quota-bar-labels">
                  <span>{{ getQuotaPercent(p.name) }}%</span>
                  <span class="font-mono" v-if="quotas?.[p.name]">
                    {{ (quotas[p.name].usedTokens / 1000).toFixed(0) }}k / {{ (quotas[p.name].maxTokens / 1000).toFixed(0) }}k
                  </span>
                </div>
                <div class="quota-progress-track">
                  <div 
                    class="quota-progress-fill" 
                    :style="{ 
                      width: `${getQuotaPercent(p.name)}%`, 
                      background: getQuotaPercent(p.name) > 85 ? '#f43f5e' : getQuotaPercent(p.name) > 60 ? '#f59e0b' : '#06b6d4' 
                    }"
                  ></div>
                </div>
              </div>
            </td>

            <!-- Fallback Route -->
            <td>
              <div class="table-routing-route font-mono">
                <span class="text-secondary">Primary</span>
                <span class="text-cyan">➔</span>
                <span class="text-muted">{{ fallbackRoutes?.[p.name]?.fallbackProviderName || 'Ollama Local' }}</span>
              </div>
            </td>

            <!-- Circuit Breaker -->
            <td>
              <StatusBadge :status="p.status || (isHealthy(p) ? 'healthy' : 'warning')" size="sm" />
            </td>

            <!-- Action Buttons -->
            <td>
              <div class="table-actions-cell">
                <button 
                  class="btn btn-secondary btn-sm" 
                  title="Test in Console"
                  @click="emit('testInConsole', p.name)"
                >
                  <span>⚡ Test</span>
                </button>
                <button 
                  class="btn btn-secondary btn-sm" 
                  title="Routing Configuration"
                  @click="emit('openRouting', p)"
                >
                  <span>⚙️ Routing</span>
                </button>
                <button 
                  class="btn btn-secondary btn-sm" 
                  title="View Token Quota & Cost"
                  @click="emit('openMetrics', p)"
                >
                  <span>📊 Quota</span>
                </button>
                <button 
                  class="btn btn-secondary btn-sm btn-danger-crimson" 
                  title="Remove Provider"
                  @click="emit('remove', p.name)"
                >
                  <span>🗑 Remove</span>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="empty-state-box glass-panel">
      <span class="empty-icon">🔌</span>
      <h3 class="empty-title">No Active AI Providers</h3>
      <p class="empty-desc">Register provider endpoints to build your resilient LLM gateway mesh.</p>
      <button class="btn btn-primary btn-sm" @click="emit('register')">
        <span>+ Register Custom Provider</span>
      </button>
    </div>
  </div>
</template>
