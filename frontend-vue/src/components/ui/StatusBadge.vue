<script setup lang="ts">
import { computed } from 'vue'

export type StatusVariant = 'emerald' | 'amber' | 'rose' | 'slate' | 'cyan'

const props = defineProps<{
  status: string
  label?: string
  size?: 'sm' | 'md'
}>()

const EMERALD_STATUSES = new Set([
  'ready', 'healthy', 'active', 'online', 'connected', 'ok', 'live', 'running', 'deployed', 'success',
  'pass', 'resolved', 'completed', 'verified', 'armed'
])
const AMBER_STATUSES = new Set([
  'warning', 'pending', 'degraded', 'in_progress', 'inprogress', 'standby', 'polling', 'open', 'promoting'
])
const ROSE_STATUSES = new Set([
  'failed', 'critical', 'error', 'offline', 'unhealthy', 'disconnected', 'down', 'danger', 'rejected', 'blocked', 'mismatch'
])
const SLATE_STATUSES = new Set([
  'cordoned', 'draining', 'terminating', 'disabled', 'unknown'
])
const CYAN_STATUSES = new Set([
  'info', 'analyzing', 'remediating', 'idle', 'draft', 'generating'
])

const normalizedStatus = computed<StatusVariant>(() => {
  if (!props.status) return 'slate'
  const lower = props.status.toLowerCase().trim()
  if (EMERALD_STATUSES.has(lower)) return 'emerald'
  if (AMBER_STATUSES.has(lower)) return 'amber'
  if (ROSE_STATUSES.has(lower)) return 'rose'
  if (SLATE_STATUSES.has(lower)) return 'slate'
  if (CYAN_STATUSES.has(lower)) return 'cyan'
  return 'slate'
})

const isPulsing = computed(() => normalizedStatus.value === 'amber')
</script>

<template>
  <span 
    class="status-badge" 
    :class="[`badge-${normalizedStatus}`, size === 'sm' ? 'badge-sm' : 'badge-md']"
  >
    <span class="status-dot" :class="[`dot-${normalizedStatus}`, { 'dot-pulse': isPulsing }]"></span>
    <span class="status-text">{{ label || status }}</span>
  </span>
</template>

<style scoped>
@import '../../assets/styles/components/ui/common.css';
</style>
