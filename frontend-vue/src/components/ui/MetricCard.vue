<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  title: string
  value: string | number
  subtitle?: string
  icon?: string
  trend?: string
  trendType?: 'positive' | 'negative' | 'neutral'
  badge?: string
  badgeColor?: 'emerald' | 'amber' | 'rose' | 'cyan' | 'violet' | 'muted'
}>()

const iconMap: Record<string, string> = {
  alert: '⚠️',
  warning: '⚠️',
  fire: '🔥',
  danger: '🚨',
  critical: '🚨',
  ok: '✅',
  success: '✅',
  check: '✔️',
  info: 'ℹ️',
  clock: '⏱️',
  shield: '🛡️',
  cpu: '⚡',
  ram: '🧠',
  disk: '💾',
  network: '🌐'
}

const resolvedIcon = computed(() => {
  if (!props.icon) return ''
  const trimmed = props.icon.trim()
  const key = trimmed.toLowerCase()
  return iconMap[key] || trimmed
})
</script>

<template>
  <div class="metric-card glass-panel">
    <div class="metric-header">
      <div class="metric-title-group">
        <span v-if="resolvedIcon" class="metric-icon" aria-hidden="true">{{ resolvedIcon }}</span>
        <span class="metric-title">{{ title }}</span>
      </div>
      <span v-if="badge" class="metric-badge" :class="`badge-${badgeColor || 'cyan'}`">
        {{ badge }}
      </span>
    </div>

    <div class="metric-body">
      <div class="metric-value">{{ value }}</div>
      <div v-if="trend" class="metric-trend" :class="`trend-${trendType || 'neutral'}`">
        {{ trend }}
      </div>
    </div>

    <div v-if="subtitle" class="metric-footer">
      <span class="metric-subtitle">{{ subtitle }}</span>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/ui/common.css';
</style>
