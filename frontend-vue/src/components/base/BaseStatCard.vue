<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
  icon?: string
  iconClass?: string
}>()
</script>

<template>
  <div class="stat-card">
    <div v-if="icon" class="stat-icon" :class="iconClass">{{ icon }}</div>
    <div class="stat-content">
      <div class="stat-value">{{ value }}</div>
      <div class="stat-label">{{ label }}</div>
    </div>
  </div>
</template>

<style scoped>
.stat-card {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-md) var(--space-lg);
  background: var(--color-surface-card);
  border-radius: var(--rounded-xl);
  border: 1px solid var(--color-hairline);
}
.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--rounded-lg);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
  background: var(--color-surface-elevated);
}
.stat-content {
  display: flex;
  flex-direction: column;
}
.stat-value {
  font-family: var(--font-number);
  font-size: var(--text-number-display);
  font-weight: 700;
  line-height: 1.1;
  color: var(--color-on-dark);
  font-variant-numeric: tabular-nums;
}
.stat-label {
  font-size: var(--text-caption);
  font-weight: 500;
  color: var(--color-muted);
  text-transform: uppercase;
}

@media (max-width: 640px) {
  .stat-card {
    padding: 12px 14px;
    gap: var(--space-xs, 8px);
  }

  .stat-icon {
    width: 32px;
    height: 32px;
    font-size: 15px;
  }

  .stat-value {
    font-size: 22px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }

  .stat-label {
    font-size: 10.5px;
    font-weight: 600;
  }
}
</style>
