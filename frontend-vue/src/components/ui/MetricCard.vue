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
.metric-card {
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  border-radius: 12px;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.metric-card:hover {
  border-color: var(--border-medium);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
}

.metric-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.metric-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.metric-icon {
  font-size: 15px;
  line-height: 1;
}

.metric-title {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-secondary);
}

.metric-badge {
  padding: 2px 8px;
  border-radius: 9999px;
  font-size: 10px;
  font-weight: 700;
  font-family: var(--font-mono);
}

.metric-body {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.metric-value {
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--text-primary);
  font-family: var(--font-sans);
  font-variant-numeric: tabular-nums;
}

.metric-trend {
  font-size: 12px;
  font-weight: 600;
}

.trend-positive { color: #10b981; }
.trend-negative { color: #f43f5e; }
.trend-neutral { color: var(--text-muted); }

.metric-footer {
  font-size: 11px;
  color: var(--text-muted);
  border-top: 1px solid var(--border-subtle);
  padding-top: 8px;
}

@media (max-width: 640px) {
  .metric-card {
    padding: 10px 12px;
    gap: 6px;
  }

  .metric-title {
    font-size: 10px;
  }

  .metric-value {
    font-size: 18px;
  }

  .metric-footer {
    font-size: 10px;
    padding-top: 6px;
  }
}
</style>