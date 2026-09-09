<script setup lang="ts">
defineProps<{
  status: string
  label?: string
  size?: 'sm' | 'md'
}>()

function normalizeStatus(s: string) {
  if (!s) return 'unknown'
  const lower = s.toLowerCase()
  if (['healthy', 'active', 'connected', 'online', 'ready', 'live', 'ok', 'pass', 'success', 'resolved', 'deployed', 'completed', 'verified', 'running', 'armed'].includes(lower)) {
    return 'emerald'
  }
  if (['warning', 'pending', 'standby', 'polling', 'inprogress', 'in_progress', 'promoting', 'open', 'degraded'].includes(lower)) {
    return 'amber'
  }
  if (['critical', 'danger', 'failed', 'error', 'offline', 'disconnected', 'rejected', 'blocked', 'mismatch', 'down', 'unhealthy'].includes(lower)) {
    return 'rose'
  }
  if (['info', 'analyzing', 'remediating', 'generating', 'idle', 'draft'].includes(lower)) {
    return 'cyan'
  }
  return 'violet'
}
</script>

<template>
  <span 
    class="status-badge" 
    :class="[`badge-${normalizeStatus(status)}`, size === 'sm' ? 'badge-sm' : 'badge-md']"
  >
    <span class="status-dot" :class="`dot-${normalizeStatus(status)}`"></span>
    <span class="status-text">{{ label || status }}</span>
  </span>
</template>

<style scoped>
@import '../../assets/styles/components/ui/common.css';
</style>
