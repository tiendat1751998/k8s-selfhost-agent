<script setup lang="ts">
import type { AIProvider } from '../../api/management'
import type { ProviderQuota } from '../../composables/useAIProviderHub'
import StatusBadge from '../ui/StatusBadge.vue'

const props = defineProps<{
  providers: AIProvider[]
  probingName: string | null
  healthResults: Record<string, { status: string; latency?: string; error?: string }>
  quotas?: Record<string, ProviderQuota>
}>()

const emit = defineEmits<{
  (e: 'probe', name: string): void
  (e: 'testInConsole', name: string): void
  (e: 'remove', name: string): void
  (e: 'openMetrics', provider: AIProvider): void
  (e: 'register'): void
}>()

function isHealthy(p: AIProvider) {
  const probe = props.healthResults[p.name]
  const s = (probe?.status || p.status || '').toLowerCase()
  return s === 'healthy' || s === 'ready' || s === 'active' || s === 'ok' || s === ''
}

function getQuotaPercent(name: string) {
  const q = props.quotas?.[name]
  if (!q || !q.maxTokens) return 25
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
  <div class="ai-providers-grid-wrap animate-fade-in">
    <div v-if="providers.length > 0" class="providers-grid">
      <div v-for="p in providers" :key="p.name" class="provider-card glass-panel">
        <div class="provider-card-header">
          <div class="provider-info-wrap">
            <div class="provider-title-row">
              <span class="provider-name">{{ p.name }}</span>
              <span v-if="p.default" class="badge badge-cyan">DEFAULT</span>
            </div>
            <small class="provider-endpoint font-mono">{{ p.endpoint }}</small>
          </div>
          <StatusBadge :status="p.status || 'healthy'" />
        </div>

        <div class="provider-body">
          <div class="pstat-row">
            <span class="pstat-label">Model Target:</span>
            <span class="pstat-val font-mono text-cyan">{{ p.model }}</span>
          </div>
          <div class="pstat-row">
            <span class="pstat-label">Backend Type:</span>
            <span class="pstat-val font-mono">{{ p.type }}</span>
          </div>
          <div class="pstat-row">
            <span class="pstat-label">Live Latency:</span>
            <div class="provider-latency-dial">
              <span class="latency-pulse-dot" :style="{ backgroundColor: getLatencyColor(p.name, p), color: getLatencyColor(p.name, p) }"></span>
              <span class="pstat-val font-mono" :style="{ color: getLatencyColor(p.name, p) }">
                {{ healthResults[p.name]?.latency || p.latency || '--' }}
              </span>
            </div>
          </div>
          <div class="pstat-row">
            <span class="pstat-label">Circuit Breaker:</span>
            <span class="pstat-val" :class="isHealthy(p) ? 'text-emerald' : 'text-amber'">
              {{ isHealthy(p) ? 'CLOSED (ARMED)' : 'DEGRADED' }}
            </span>
          </div>

          <!-- Token Quota Bar -->
          <div v-if="quotas?.[p.name]" class="provider-quota-bar-wrap">
            <div class="quota-bar-labels">
              <span>Token Quota ({{ getQuotaPercent(p.name) }}%)</span>
              <span class="font-mono">{{ (quotas[p.name].usedTokens / 1000).toFixed(0) }}k / {{ (quotas[p.name].maxTokens / 1000).toFixed(0) }}k</span>
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
        </div>

        <div class="provider-footer">
          <button 
            class="btn btn-secondary btn-sm" 
            :disabled="probingName === p.name"
            @click="emit('probe', p.name)"
          >
            <span>{{ probingName === p.name ? 'Probing...' : '⚡ Probe Health' }}</span>
          </button>
          <button class="btn btn-secondary btn-sm" @click="emit('openMetrics', p)">
            <span>📊 Metrics</span>
          </button>
          <button class="btn btn-secondary btn-sm" @click="emit('testInConsole', p.name)">
            <span>Test</span>
          </button>
          <button class="btn btn-secondary btn-sm btn-danger-crimson" title="Remove" @click="emit('remove', p.name)">
            <span>Remove</span>
          </button>
        </div>
      </div>
    </div>

    <div v-else class="empty-state-box glass-panel">
      <span class="empty-icon">🔌</span>
      <h3 class="empty-title">No Active AI Providers Registered</h3>
      <p class="empty-desc">Connect a local Ollama instance or external model endpoint to activate the AI SRE mesh.</p>
      <button class="btn btn-primary btn-sm" @click="emit('register')">
        <span>+ Register Custom Provider</span>
      </button>
    </div>
  </div>
</template>
