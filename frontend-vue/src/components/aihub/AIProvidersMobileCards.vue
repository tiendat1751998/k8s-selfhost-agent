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
  (e: 'openMetrics', provider: AIProvider): void
  (e: 'remove', name: string): void
  (e: 'register'): void
}>()

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
  <div class="mobile-stream-wrap animate-fade-in">
    <div v-if="providers.length > 0" class="mobile-cards-stream">
      <div v-for="p in providers" :key="p.name" class="mobile-card-item glass-panel">
        <div class="mobile-card-main">
          <StatusBadge :status="p.status || 'healthy'" size="sm" />
          <div class="mobile-card-info">
            <div class="mobile-card-title">
              {{ p.name }}
              <span v-if="p.default" class="badge badge-cyan" style="font-size: 8px; padding: 1px 4px;">DEF</span>
            </div>
            <div class="mobile-card-sub font-mono">
              <span class="text-cyan">{{ p.model }}</span>
              <span>•</span>
              <span class="text-muted">{{ p.type }}</span>
            </div>
          </div>
        </div>

        <div class="mobile-card-telemetry">
          <div class="provider-latency-dial">
            <span class="latency-pulse-dot" :style="{ backgroundColor: getLatencyColor(p.name, p), color: getLatencyColor(p.name, p) }"></span>
            <span class="font-mono" :style="{ color: getLatencyColor(p.name, p), fontSize: '11px', fontWeight: '700' }">
              {{ healthResults[p.name]?.latency || p.latency || '--' }}
            </span>
          </div>
        </div>

        <div class="mobile-card-actions">
          <button 
            class="mobile-btn-icon" 
            :disabled="probingName === p.name"
            title="Probe Health"
            @click="emit('probe', p.name)"
          >
            <span>{{ probingName === p.name ? '⏳' : '⚡' }}</span>
          </button>
          <button 
            class="mobile-btn-icon" 
            title="Metrics & Quota"
            @click="emit('openMetrics', p)"
          >
            <span>📊</span>
          </button>
          <button 
            class="mobile-btn-icon" 
            title="Test in Console"
            @click="emit('testInConsole', p.name)"
          >
            <span>💬</span>
          </button>
          <button 
            class="mobile-btn-icon btn-danger-crimson" 
            title="Remove Provider"
            @click="emit('remove', p.name)"
          >
            <span>🗑</span>
          </button>
        </div>
      </div>
    </div>

    <div v-else class="empty-state-box glass-panel">
      <span class="empty-icon">🔌</span>
      <h3 class="empty-title">No AI Gateways</h3>
      <p class="empty-desc">Register your first AI provider endpoint.</p>
      <button class="btn btn-primary btn-sm" @click="emit('register')">
        <span>+ Register Provider</span>
      </button>
    </div>
  </div>
</template>
