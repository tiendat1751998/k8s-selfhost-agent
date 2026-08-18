<script setup lang="ts">
defineProps<{
  title: string
  value: string | number
  subtitle?: string
  icon?: string
  trend?: string
  trendType?: 'positive' | 'negative' | 'neutral'
  badge?: string
  badgeColor?: 'emerald' | 'amber' | 'rose' | 'cyan' | 'violet'
}>()
</script>

<template>
  <div class="metric-card glass-panel glass-panel-glow">
    <div class="metric-header">
      <div class="metric-title-group">
        <span v-if="icon" class="metric-icon">{{ icon }}</span>
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
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  border-radius: 14px;
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
  font-size: 16px;
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
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--text-primary);
  font-family: var(--font-sans);
}

.metric-trend {
  font-size: 12px;
  font-weight: 600;
}

.trend-positive { color: #34d399; }
.trend-negative { color: #fb7185; }
.trend-neutral { color: var(--text-muted); }

.metric-footer {
  font-size: 11px;
  color: var(--text-muted);
  border-top: 1px solid var(--border-subtle);
  padding-top: 8px;
}
</style>
